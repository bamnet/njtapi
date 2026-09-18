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
	"time"
)

// RailDataGTFSRTURL is the base URL of NJTransit's RailData GTFS-realtime API.
const RailDataGTFSRTURL = "https://raildata.njtransit.com/api/GTFSRT/"

// ErrAuthenticationFailed is returned when the RailData API rejects the
// username and password supplied to a RailDataClient.
var ErrAuthenticationFailed = errors.New("raildata: authentication failed")

// RailDataClient talks to NJTransit's RailData GTFS-realtime API.
//
// RailData is a separate service from the one Client talks to: it has its own
// base URL and its own credentials, and authenticates with a token obtained
// from the username and password rather than sending them on every request.
// The client fetches a token on first use, reuses it across calls, and fetches
// a new one if the API reports that it is no longer valid.
//
// A RailDataClient is safe for concurrent use.
type RailDataClient struct {
	httpClient *http.Client
	baseURL    string
	username   string
	password   string
	location   *time.Location

	mu    sync.Mutex // guards token and serializes token requests
	token string
}

// NewRailDataClient constructs a new client to talk to the RailData
// GTFS-realtime API.
//
// baseURL: The root URL that the API is exposed on, usually RailDataGTFSRTURL.
// username / password: RailData credentials, which are distinct from the
// credentials used by Client.
func NewRailDataClient(baseURL, username, password string) *RailDataClient {
	return NewCustomRailDataClient(&http.Client{Timeout: 30 * time.Second}, baseURL, username, password)
}

// NewCustomRailDataClient uses the supplied `http.Client` when talking to the
// API. This can be useful if you need to supply a custom timeout, proxy
// server, etc.
//
// See `NewRailDataClient` for a description of the rest of the parameters.
func NewCustomRailDataClient(c *http.Client, baseURL, username, password string) *RailDataClient {
	return &RailDataClient{
		httpClient: c,
		baseURL:    baseURL,
		username:   username,
		password:   password,
		location:   defaultLocation(),
	}
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

// getToken returns the cached token, requesting a new one if none is cached.
func (c *RailDataClient) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" {
		return c.token, nil
	}

	body, err := c.post(ctx, "getToken", map[string]string{
		"username": c.username,
		"password": c.password,
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
	c.token = tr.UserToken
	return c.token, nil
}

// invalidateToken forgets token if it is still the cached one, so that the
// next getToken call requests a fresh token.
func (c *RailDataClient) invalidateToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == token {
		c.token = ""
	}
}

// fetch calls a token-authenticated endpoint. If the API rejects the token, a
// new token is requested and the call is retried once.
func (c *RailDataClient) fetch(ctx context.Context, endpoint string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		token, err := c.getToken(ctx)
		if err != nil {
			return nil, err
		}
		body, err := c.post(ctx, endpoint, map[string]string{"token": token})
		if err == nil {
			// Guard against an error reported with a 2xx status. Binary
			// GTFS-realtime feeds never start with '{'.
			if len(body) > 0 && body[0] == '{' {
				err = errorFromBody(http.StatusOK, body)
			} else {
				return body, nil
			}
		}
		if attempt > 0 || !isInvalidToken(err) {
			return nil, err
		}
		c.invalidateToken(token)
	}
}

// post sends fields as a multipart form to endpoint and returns the response
// body. Non-2xx responses are returned as an *APIError.
func (c *RailDataClient) post(ctx context.Context, endpoint string, fields map[string]string) ([]byte, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	u = u.JoinPath(endpoint)

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

	resp, err := c.httpClient.Do(req)
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
