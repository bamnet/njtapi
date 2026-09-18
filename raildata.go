package njtapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// railDataBaseURL is the root of NJTransit's RailData APIs. Each API lives
// under its own path (railDataGTFSRT, ...).
const railDataBaseURL = "https://raildata.njtransit.com/api/"

// RailData API paths, relative to the RailData base URL. Each API issues and
// accepts only its own tokens, so tokens are cached per API.
const (
	railDataGTFSRT = "GTFSRT"
)

// ErrRailDataNotConfigured is returned by methods that need the RailData API
// when the Client was constructed without WithRailData.
var ErrRailDataNotConfigured = errors.New("raildata: credentials not configured, see WithRailData")

// ErrAuthenticationFailed is returned when the RailData API rejects the
// username and password supplied with WithRailData.
var ErrAuthenticationFailed = errors.New("raildata: authentication failed")

// railData talks to NJTransit's RailData APIs.
//
// RailData authenticates with a token obtained from a username and password
// rather than sending them on every request. One login works on every RailData
// API, but each API issues its own tokens and rejects the others', so a token
// is fetched lazily and cached for each API separately. If an API reports that
// a token is no longer valid, a new one is fetched and the call retried once.
//
// A railData is safe for concurrent use.
type railData struct {
	httpClient *http.Client
	baseURL    string
	username   string
	password   string

	mu     sync.Mutex // guards tokens
	tokens map[string]*apiToken
}

// apiToken is the cached token for one RailData API.
type apiToken struct {
	mu    sync.Mutex // guards token and serializes token requests for the API
	token string
}

// tokenResponse is the JSON body returned by getToken.
type tokenResponse struct {
	Authenticated string `json:"Authenticated"`
	UserToken     string `json:"UserToken"`
}

// errorResponse is the JSON body the API returns alongside error statuses,
// e.g. {"errorMessage":"Invalid token."}.
type errorResponse struct {
	ErrorMessage string `json:"errorMessage"`
}

// tokenFor returns the token cache for api, creating it if needed.
func (r *railData) tokenFor(api string) *apiToken {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tokens == nil {
		r.tokens = map[string]*apiToken{}
	}
	t, ok := r.tokens[api]
	if !ok {
		t = &apiToken{}
		r.tokens[api] = t
	}
	return t
}

// getToken returns the cached token for api, requesting a new one if none is
// cached.
func (r *railData) getToken(ctx context.Context, api string) (string, error) {
	t := r.tokenFor(api)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" {
		return t.token, nil
	}

	body, err := r.post(ctx, api, "getToken", map[string]string{
		"username": r.username,
		"password": r.password,
	})
	if err != nil {
		return "", err
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		// Deliberately don't include the body: it may carry a token.
		return "", errors.New("raildata: malformed getToken response")
	}
	if !strings.EqualFold(tr.Authenticated, "true") || tr.UserToken == "" {
		return "", ErrAuthenticationFailed
	}
	t.token = tr.UserToken
	return t.token, nil
}

// invalidateToken forgets token if it is still the one cached for api, so
// that the next getToken call requests a fresh token.
func (r *railData) invalidateToken(api, token string) {
	t := r.tokenFor(api)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token == token {
		t.token = ""
	}
}

// fetch calls a token-authenticated endpoint of api. If the API rejects the
// token, a new token is requested and the call is retried once.
func (r *railData) fetch(ctx context.Context, api, endpoint string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		token, err := r.getToken(ctx, api)
		if err != nil {
			return nil, err
		}
		body, err := r.post(ctx, api, endpoint, map[string]string{"token": token})
		if err == nil {
			// Guard against an error reported with a 2xx status.
			var er errorResponse
			if len(body) == 0 || body[0] != '{' || json.Unmarshal(body, &er) != nil || er.ErrorMessage == "" {
				return body, nil
			}
			err = &APIError{StatusCode: http.StatusOK, Body: er.ErrorMessage}
		}
		if attempt > 0 || !isInvalidToken(err) {
			return nil, err
		}
		r.invalidateToken(api, token)
	}
}

// post sends fields as a multipart form to endpoint of api and returns the
// response body. Non-2xx responses are returned as an *APIError.
func (r *railData) post(ctx context.Context, api, endpoint string, fields map[string]string) ([]byte, error) {
	u, err := url.Parse(r.baseURL)
	if err != nil {
		return nil, err
	}
	u = u.JoinPath(api, endpoint)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errorFromBody(resp.StatusCode, body)
	}
	return body, nil
}

// errorFromBody builds an *APIError, preferring the API's JSON errorMessage
// over the raw body when one is present.
func errorFromBody(status int, body []byte) error {
	var er errorResponse
	if json.Unmarshal(body, &er) == nil && er.ErrorMessage != "" {
		return &APIError{StatusCode: status, Body: er.ErrorMessage}
	}
	const maxBodyLen = 1024
	errBody := string(body)
	if len(errBody) > maxBodyLen {
		errBody = errBody[:maxBodyLen] + "..."
	}
	return &APIError{StatusCode: status, Body: errBody}
}

// isInvalidToken reports whether err is the API rejecting a token. The API
// answers an invalid or expired token with HTTP 500 and
// {"errorMessage":"Invalid token."}.
func isInvalidToken(err error) bool {
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}
	if ae.StatusCode == http.StatusUnauthorized {
		return true
	}
	return strings.Contains(strings.ToLower(ae.Body), "invalid token")
}
