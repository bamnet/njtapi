// Package njtapi provides an library to access NJTransit Train data.
//
// A valid username and password is required to call the NJTransit API.
// To  register, head on over to https://datasource.njtransit.com/.
//
// This library makes opinionated decisions about how data should be sanitized
// and modeled. As a result, it does not provide a 1:1 mapping of all of
// features and fields included in the API spec.
package njtapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client stores connection info needed talking to the NJTransit API.
//
// By default a Client talks to the legacy train data API using the username
// and password given to NewClient. Some methods, such as Alerts, use
// NJTransit's RailData API instead and need RailData credentials, supplied
// with the WithRailData option.
//
// A Client is safe for concurrent use.
type Client struct {
	httpClient *http.Client
	baseURL    string
	username   string
	password   string
	location   *time.Location

	// railData is nil unless WithRailData was supplied.
	railData *railData
}

// Option configures optional features of a Client.
type Option func(*options)

// options collects the settings made by Option values.
type options struct {
	railDataUsername string
	railDataPassword string
	railDataURL      string
}

// WithRailData supplies credentials for NJTransit's RailData API, which
// methods such as Alerts need.
//
// RailData credentials are issued separately from the username and password
// passed to NewClient, and neither set works on the other API.
func WithRailData(username, password string) Option {
	return func(o *options) {
		o.railDataUsername = username
		o.railDataPassword = password
	}
}

// WithRailDataURL overrides the root URL of the RailData API, which defaults
// to https://raildata.njtransit.com/api/. Individual APIs such as GTFSRT are
// requested below it. This is mostly useful for tests and test environments.
// It has no effect without WithRailData.
func WithRailDataURL(baseURL string) Option {
	return func(o *options) {
		o.railDataURL = baseURL
	}
}

// applyOptions applies opts to c.
func (c *Client) applyOptions(opts []Option) *Client {
	o := options{railDataURL: railDataBaseURL}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.railDataUsername != "" || o.railDataPassword != "" {
		c.railData = &railData{
			httpClient: c.httpClient,
			baseURL:    o.railDataURL,
			username:   o.railDataUsername,
			password:   o.railDataPassword,
		}
	}
	return c
}

// ErrUnexpectedStatus is returned when the API returns a non-2xx HTTP status code.
var ErrUnexpectedStatus = errors.New("unexpected HTTP status")

// APIError captures a non-2xx HTTP response from the API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, http.StatusText(e.StatusCode))
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func (e *APIError) Unwrap() error {
	return ErrUnexpectedStatus
}

// NewClient constructs a new client to talk to the NJTransit API.
//
// baseURL: The root URL that the API is exposed on.
// username / password: Authentication credentials for calling the API.
// opts: Optional features, such as WithRailData.
func NewClient(baseURL, username, password string, opts ...Option) *Client {
	return NewClientWithLocation(baseURL, username, password, defaultLocation(), opts...)
}

// NewClientWithLocation constructs a new client with a custom timezone location.
// If the provided location is nil, it falls back to UTC.
func NewClientWithLocation(baseURL, username, password string, loc *time.Location, opts ...Option) *Client {
	if loc == nil {
		loc = time.UTC
	}
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		username:   username,
		password:   password,
		location:   loc,
	}
	return c.applyOptions(opts)
}

// NewCustomClient uses the supplied `http.Client` when talking to the API.
// This can be useful if you need to supply a custom timeout, proxy server, etc.
//
// See `NewClient` for a description of the rest of the parameters.
func NewCustomClient(c *http.Client, baseURL, username, password string, opts ...Option) *Client {
	client := &Client{
		httpClient: c,
		baseURL:    baseURL,
		username:   username,
		password:   password,
		location:   defaultLocation(),
	}
	return client.applyOptions(opts)
}

func defaultLocation() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
}

// fetch retrieves data from an API endpoint.
func (c *Client) fetch(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}

	u = u.JoinPath(endpoint)
	q := u.Query()
	q.Set("username", c.username)
	q.Set("password", c.password)
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		const maxBodyLen = 1024
		errBody := string(body)
		if len(errBody) > maxBodyLen {
			errBody = errBody[:maxBodyLen] + "..."
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       errBody,
		}
	}

	return body, nil
}
