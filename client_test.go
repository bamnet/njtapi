package njtapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientDefaultTimeout(t *testing.T) {
	c := NewClient("http://example.com", "user", "pass")
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("NewClient() timeout = %v, want %v", c.httpClient.Timeout, 30*time.Second)
	}
}

func TestNewCustomClientNoDefaultTimeout(t *testing.T) {
	customHTTP := &http.Client{Timeout: 5 * time.Second}
	c := NewCustomClient(customHTTP, "http://example.com", "user", "pass")
	if c.httpClient.Timeout != 5*time.Second {
		t.Errorf("NewCustomClient() timeout = %v, want %v", c.httpClient.Timeout, 5*time.Second)
	}
}

func TestNewCustomClientZeroTimeout(t *testing.T) {
	customHTTP := &http.Client{}
	c := NewCustomClient(customHTTP, "http://example.com", "user", "pass")
	if c.httpClient.Timeout != 0 {
		t.Errorf("NewCustomClient() timeout = %v, want 0", c.httpClient.Timeout)
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("username") != "user" {
			t.Error("missing username")
		}
		if r.URL.Query().Get("password") != "pass" {
			t.Error("missing password")
		}
		_, _ = w.Write([]byte("<response>ok</response>"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user", "pass")
	body, err := c.fetch(context.Background(), "testEndpoint", nil)
	if err != nil {
		t.Fatalf("fetch() error = %v", err)
	}
	if string(body) != "<response>ok</response>" {
		t.Errorf("fetch() = %q, want %q", string(body), "<response>ok</response>")
	}
}
