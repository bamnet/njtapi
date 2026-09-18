package njtapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gtfs "github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"google.golang.org/protobuf/proto"
)

// TestRailDataTransportErrors covers failures before or while talking to the
// RailData API, none of which may leak the credentials or a token.
func TestRailDataTransportErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc // nil means the server is closed before the call.
		baseURL string           // overrides the test server's URL when set.
		wantErr error
	}{
		{
			name:    "server unreachable",
			handler: nil,
		},
		{
			name:    "invalid base URL",
			baseURL: "http://[::1]:namedport",
		},
		{
			name: "malformed getToken response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`not json, maybe a secret-token`))
			},
		},
		{
			name: "authenticated without a token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"Authenticated":"True","UserToken":""}`))
			},
			wantErr: ErrAuthenticationFailed,
		},
		{
			name: "getToken non-2xx",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
			},
			wantErr: ErrUnexpectedStatus,
		},
		{
			name: "response body cut short",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "100")
				_, _ = w.Write([]byte(`{"Authenticated"`))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(tc.handler)
			if tc.handler == nil {
				ts.Close()
			} else {
				defer ts.Close()
			}
			base := ts.URL
			if tc.baseURL != "" {
				base = tc.baseURL
			}

			c := NewClient("", "", "", WithRailData("username", "pa$$word"), WithRailDataURL(base))
			_, err := c.Alerts(context.Background())
			if err == nil {
				t.Fatal("Alerts() expected error, got none")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("Alerts() error = %v, want %v", err, tc.wantErr)
			}
			for _, secret := range []string{"pa$$word", "secret-token"} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("Alerts() error %q leaks %q", err, secret)
				}
			}
		})
	}
}

func TestRailDataUnauthorizedRefreshes(t *testing.T) {
	var tokenCalls, alertsCalls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/GTFSRT/getToken":
			tokenCalls++
			_, _ = w.Write([]byte(`{"Authenticated":"True","UserToken":"tok"}`))
		case "/GTFSRT/getAlerts":
			alertsCalls++
			if alertsCalls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.ServeFile(w, r, "testdata/getAlerts.pb")
		}
	}))
	defer ts.Close()

	c := NewClient("", "", "", WithRailData("username", "pa$$word"), WithRailDataURL(ts.URL))
	if _, err := c.Alerts(context.Background()); err != nil {
		t.Fatalf("Alerts() unexpected error: %v", err)
	}
	if tokenCalls != 2 || alertsCalls != 2 {
		t.Errorf("got %d getToken and %d getAlerts calls, want 2 and 2", tokenCalls, alertsCalls)
	}
}

func TestErrorFromBody(t *testing.T) {
	long := strings.Repeat("x", 2000)
	for _, tc := range []struct {
		name     string
		body     string
		wantBody string
	}{
		{name: "json error message", body: `{"errorMessage":"Invalid token."}`, wantBody: "Invalid token."},
		{name: "json without message", body: `{"other":"field"}`, wantBody: `{"other":"field"}`},
		{name: "plain text", body: "Service Unavailable", wantBody: "Service Unavailable"},
		{name: "long body truncated", body: long, wantBody: long[:1024] + "..."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := errorFromBody(http.StatusInternalServerError, []byte(tc.body))
			var ae *APIError
			if !errors.As(err, &ae) {
				t.Fatalf("errorFromBody() = %T, want *APIError", err)
			}
			if ae.StatusCode != http.StatusInternalServerError || ae.Body != tc.wantBody {
				t.Errorf("errorFromBody() = {%d, %q}, want {500, %q}", ae.StatusCode, ae.Body, tc.wantBody)
			}
		})
	}
}

func TestIsInvalidToken(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "not an APIError", err: errors.New("invalid token"), want: false},
		{name: "401", err: &APIError{StatusCode: http.StatusUnauthorized}, want: true},
		{name: "500 invalid token", err: &APIError{StatusCode: 500, Body: "Invalid token."}, want: true},
		{name: "500 other", err: &APIError{StatusCode: 500, Body: "Server error"}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isInvalidToken(tc.err); got != tc.want {
				t.Errorf("isInvalidToken(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestUnixTime(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		loc  *time.Location
		ts   uint64
		want time.Time
	}{
		{name: "unset", loc: ny, ts: 0, want: time.Time{}},
		{name: "client location", loc: ny, ts: 1789740360, want: time.Unix(1789740360, 0).In(ny)},
		{name: "no location falls back to UTC", loc: nil, ts: 1789740360, want: time.Unix(1789740360, 0).UTC()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{location: tc.loc}
			got := c.unixTime(tc.ts)
			if !got.Equal(tc.want) || got.Location().String() != tc.want.Location().String() {
				t.Errorf("unixTime(%d) = %v, want %v", tc.ts, got, tc.want)
			}
		})
	}
}

func TestTranslation(t *testing.T) {
	tr := func(lang, text string) *gtfs.TranslatedString_Translation {
		t := &gtfs.TranslatedString_Translation{Text: proto.String(text)}
		if lang != "" {
			t.Language = proto.String(lang)
		}
		return t
	}
	for _, tc := range []struct {
		name string
		in   *gtfs.TranslatedString
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "empty", in: &gtfs.TranslatedString{}, want: ""},
		{name: "untagged", in: &gtfs.TranslatedString{Translation: []*gtfs.TranslatedString_Translation{tr("", "hello")}}, want: "hello"},
		{name: "prefers english", in: &gtfs.TranslatedString{Translation: []*gtfs.TranslatedString_Translation{tr("es", "hola"), tr("en", "hello")}}, want: "hello"},
		{name: "en-US", in: &gtfs.TranslatedString{Translation: []*gtfs.TranslatedString_Translation{tr("fr", "bonjour"), tr("en-US", "hi")}}, want: "hi"},
		{name: "falls back to first", in: &gtfs.TranslatedString{Translation: []*gtfs.TranslatedString_Translation{tr("es", "hola"), tr("fr", "bonjour")}}, want: "hola"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := translation(tc.in); got != tc.want {
				t.Errorf("translation() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAlertsSkipsDeletedAndNonAlertEntities(t *testing.T) {
	feed := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{GtfsRealtimeVersion: proto.String("2.0")},
		Entity: []*gtfs.FeedEntity{
			{Id: proto.String("deleted"), IsDeleted: proto.Bool(true), Alert: &gtfs.Alert{}},
			{Id: proto.String("vehicle"), Vehicle: &gtfs.VehiclePosition{}},
			{Id: proto.String("kept"), Alert: &gtfs.Alert{
				HeaderText: &gtfs.TranslatedString{Translation: []*gtfs.TranslatedString_Translation{{Text: proto.String("Delays")}}},
			}},
		},
	}
	body, err := proto.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/GTFSRT/getToken" {
			_, _ = w.Write([]byte(`{"Authenticated":"True","UserToken":"tok"}`))
			return
		}
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	c := NewClient("", "", "", WithRailData("username", "pa$$word"), WithRailDataURL(ts.URL))
	got, err := c.Alerts(context.Background())
	if err != nil {
		t.Fatalf("Alerts() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "kept" || got[0].Header != "Delays" {
		t.Errorf("Alerts() = %+v, want only the %q alert", got, "kept")
	}
}
