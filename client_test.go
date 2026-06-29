package njtapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/getStationList.xml")
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")
	_, err := c.StationList(context.Background())
	if err != nil {
		t.Errorf("expected no error for 200 response, got: %v", err)
	}
}

func TestFetchNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("station not found"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")
	_, err := c.StationList(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}

	if !errors.Is(err, ErrUnexpectedStatus) {
		t.Errorf("expected ErrUnexpectedStatus, got: %v", err)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got: %T", err)
	}

	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected status code %d, got %d", http.StatusNotFound, apiErr.StatusCode)
	}
	if apiErr.Body != "station not found" {
		t.Errorf("expected body 'station not found', got '%s'", apiErr.Body)
	}
}

func TestFetchInternalServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")
	_, err := c.StationList(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}

	if !errors.Is(err, ErrUnexpectedStatus) {
		t.Errorf("expected ErrUnexpectedStatus, got: %v", err)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got: %T", err)
	}

	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, apiErr.StatusCode)
	}
}

func TestAPIError_Error(t *testing.T) {
	e := &APIError{StatusCode: 503, Body: "service unavailable"}
	want := "HTTP 503: service unavailable"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_ErrorEmptyBody(t *testing.T) {
	e := &APIError{StatusCode: 502, Body: ""}
	want := "HTTP 502: Bad Gateway"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestFetchEmptyBodyError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")
	_, err := c.StationList(context.Background())
	if err == nil {
		t.Fatal("expected error for 502 response, got nil")
	}

	if !errors.Is(err, ErrUnexpectedStatus) {
		t.Errorf("expected ErrUnexpectedStatus, got: %v", err)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got: %T", err)
	}

	if apiErr.Body != "" {
		t.Errorf("expected empty body, got '%s'", apiErr.Body)
	}
}

func TestFetchTruncatedErrorBody(t *testing.T) {
	longBody := make([]byte, 2000)
	for i := range longBody {
		longBody[i] = 'x'
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write(longBody)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")
	_, err := c.StationList(context.Background())
	if err == nil {
		t.Fatal("expected error for 502 response, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got: %T", err)
	}

	if len(apiErr.Body) > 1024+3 {
		t.Errorf("expected truncated body (max %d), got %d bytes", 1024+3, len(apiErr.Body))
	}
	if apiErr.Body[1024:] != "..." {
		t.Errorf("expected body to end with '...', got suffix %q", apiErr.Body[1020:])
	}
}