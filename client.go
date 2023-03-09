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
	"io"
	"net/http"
	"net/url"
)

// Client stores connection info needed talking to the NJTransit API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	username   string
	password   string
}

// NewClient constructs a new client to talk to the NJTransit API.
//
// baseURL: The root URL that the API is exposed on.
// username / password: Authentication credentials for calling the API.
func NewClient(baseURL, username, password string) *Client {
	return &Client{&http.Client{}, baseURL, username, password}
}

// NewCustomClient uses the supplied `http.Client` when talking to the API.
// This can be useful if you need to supply a custom timeout, proxy server, etc.
//
// See `NewClient` for a description of the rest of the parameters.
func NewCustomClient(c *http.Client, baseURL, username, password string) *Client {
	return &Client{c, baseURL, username, password}
}

// fetch retrieves data from an API endpoint.
func (c *Client) fetch(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}

	u.Path = u.Path + endpoint
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

	return body, nil
}
