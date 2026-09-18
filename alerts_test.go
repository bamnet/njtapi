package njtapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func ExampleClient_Alerts() {
	// Update these values. RailData credentials are separate from the legacy
	// API's username and password.
	username := "your RailData username"
	password := "your RailData password"

	client := NewClient("", "", "", WithRailData(username, password))
	alerts, err := client.Alerts(context.Background())
	if err != nil {
		log.Fatalf("Alerts() error: %v", err)
	}
	for _, a := range alerts {
		fmt.Printf("%s (routes %v)\n", a.Header, a.RouteIDs)
	}
}

// fakeRailData is a stand-in for the RailData APIs. Like the real service, it
// issues tokens per API and rejects a token issued by a different API.
type fakeRailData struct {
	t *testing.T

	mu          sync.Mutex
	tokenCalls  int
	alertsCalls int
	issued      int
	tokenAPI    map[string]string // token -> API that issued it
	// perAPITokenCalls counts getToken calls by API.
	perAPITokenCalls map[string]int

	// authenticate controls whether getToken accepts the credentials.
	authenticate bool
	// rejectToken reports whether getAlerts should treat a token as invalid.
	rejectToken func(token string) bool
	// alertsStatus, if non-zero, is returned by getAlerts instead of the feed.
	alertsStatus int
}

func (f *fakeRailData) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		f.t.Errorf("got %s request, want POST", r.Method)
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		f.t.Errorf("got Content-Type %q, want multipart/form-data", r.Header.Get("Content-Type"))
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		f.t.Errorf("Error parsing multipart form: %v", err)
	}
	if r.URL.RawQuery != "" {
		f.t.Errorf("Unexpected query string %q; credentials belong in the form", r.URL.RawQuery)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	api, endpoint, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	invalidToken := func() bool {
		token := r.FormValue("token")
		if token == "" || f.tokenAPI[token] != api || (f.rejectToken != nil && f.rejectToken(token)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errorMessage":"Invalid token."}`))
			return true
		}
		return false
	}
	switch {
	case (api == "GTFSRT" || api == "TrainData") && endpoint == "getToken":
		f.tokenCalls++
		if f.perAPITokenCalls == nil {
			f.perAPITokenCalls = map[string]int{}
		}
		f.perAPITokenCalls[api]++
		if u, p := r.FormValue("username"), r.FormValue("password"); u != "username" || p != "pa$$word" {
			f.t.Errorf("Missing expected username & password: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		if !f.authenticate {
			_, _ = w.Write([]byte(`{"Authenticated":"False","UserToken":""}`))
			return
		}
		f.issued++
		token := fmt.Sprintf("tok%d", f.issued)
		if f.tokenAPI == nil {
			f.tokenAPI = map[string]string{}
		}
		f.tokenAPI[token] = api
		fmt.Fprintf(w, `{"Authenticated":"True","UserToken":%q}`, token)
	case api == "TrainData" && endpoint == "getStationList":
		if invalidToken() {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	case api == "GTFSRT" && endpoint == "getAlerts":
		f.alertsCalls++
		if invalidToken() {
			return
		}
		if f.alertsStatus != 0 {
			w.WriteHeader(f.alertsStatus)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, "testdata/getAlerts.pb")
	default:
		f.t.Errorf("Unexpected request path %q", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestAlertsAuth(t *testing.T) {
	for _, tc := range []struct {
		name         string
		authenticate bool
		rejectToken  func(string) bool
		alertsStatus int
		calls        int // Alerts calls to make.

		wantErr         error // nil for success
		wantAnyErr      bool
		wantTokenCalls  int
		wantAlertsCalls int
	}{
		{
			name:            "token success",
			authenticate:    true,
			calls:           1,
			wantTokenCalls:  1,
			wantAlertsCalls: 1,
		},
		{
			name:           "token failure",
			authenticate:   false,
			calls:          1,
			wantErr:        ErrAuthenticationFailed,
			wantTokenCalls: 1,
		},
		{
			name:            "token reused across calls",
			authenticate:    true,
			calls:           2,
			wantTokenCalls:  1,
			wantAlertsCalls: 2,
		},
		{
			name:            "refresh on invalid token",
			authenticate:    true,
			rejectToken:     func(tok string) bool { return tok == "tok1" },
			calls:           2,
			wantTokenCalls:  2,
			wantAlertsCalls: 3,
		},
		{
			name:            "refresh only once",
			authenticate:    true,
			rejectToken:     func(string) bool { return true },
			calls:           1,
			wantAnyErr:      true,
			wantTokenCalls:  2,
			wantAlertsCalls: 2,
		},
		{
			name:            "other errors not retried",
			authenticate:    true,
			alertsStatus:    http.StatusServiceUnavailable,
			calls:           1,
			wantErr:         ErrUnexpectedStatus,
			wantTokenCalls:  1,
			wantAlertsCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRailData{t: t, authenticate: tc.authenticate, rejectToken: tc.rejectToken, alertsStatus: tc.alertsStatus}
			ts := httptest.NewServer(f)
			defer ts.Close()

			c := NewClient("", "", "", WithRailData("username", "pa$$word"), WithRailDataURL(ts.URL))
			for i := 0; i < tc.calls; i++ {
				got, err := c.Alerts(context.Background())
				switch {
				case tc.wantErr != nil || tc.wantAnyErr:
					if err == nil {
						t.Fatalf("Alerts() expected error, got none")
					}
					if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
						t.Errorf("Alerts() error = %v, want %v", err, tc.wantErr)
					}
					for _, secret := range []string{"tok1", "tok2", "pa$$word"} {
						if strings.Contains(err.Error(), secret) {
							t.Errorf("Alerts() error %q leaks %q", err, secret)
						}
					}
				case err != nil:
					t.Fatalf("Alerts() unexpected error: %v", err)
				case len(got) != 10:
					t.Errorf("Alerts() returned %d alerts, want 10", len(got))
				}
			}

			f.mu.Lock()
			defer f.mu.Unlock()
			if f.tokenCalls != tc.wantTokenCalls {
				t.Errorf("getToken called %d times, want %d", f.tokenCalls, tc.wantTokenCalls)
			}
			if f.alertsCalls != tc.wantAlertsCalls {
				t.Errorf("getAlerts called %d times, want %d", f.alertsCalls, tc.wantAlertsCalls)
			}
		})
	}
}

func TestAlertsConcurrentTokenReuse(t *testing.T) {
	f := &fakeRailData{t: t, authenticate: true}
	ts := httptest.NewServer(f)
	defer ts.Close()

	c := NewClient("", "", "", WithRailData("username", "pa$$word"), WithRailDataURL(ts.URL))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Alerts(context.Background()); err != nil {
				t.Errorf("Alerts() unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tokenCalls != 1 {
		t.Errorf("getToken called %d times, want 1", f.tokenCalls)
	}
}

func TestAlertsDecode(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("Error loading timezones: %v", err)
	}

	ts := httptest.NewServer(&fakeRailData{t: t, authenticate: true})
	defer ts.Close()

	c := NewClient("", "", "", WithRailData("username", "pa$$word"), WithRailDataURL(ts.URL))
	got, err := c.Alerts(context.Background())
	if err != nil {
		t.Fatalf("Alerts() error: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("Alerts() returned %d alerts, want 10", len(got))
	}
	byID := map[string]Alert{}
	for _, a := range got {
		byID[a.ID] = a
	}

	for _, want := range []Alert{
		{
			// A route-wide advisory; '?' stands in for an en dash.
			ID:          "2609180952194",
			Header:      "NJ TRANSIT Offers Rail Service between Secaucus Junction and MetLife Stadium for AC/DC Concert ? Friday, September 25, 2026",
			Description: "NJ TRANSIT Offers Rail Service between Secaucus Junction and MetLife Stadium for AC/DC Concert ? Friday, September 25, 2026",
			URL:         "https://www.njtransit.com/node/2171356",
			Cause:       "UNKNOWN_CAUSE",
			Effect:      "UNKNOWN_EFFECT",
			Severity:    "UNKNOWN_SEVERITY",
			ActivePeriods: []AlertPeriod{{
				Start: time.Date(2026, 9, 18, 9, 52, 1, 0, loc),
				End:   time.Date(2026, 9, 19, 9, 52, 1, 0, loc),
			}},
			Entities: []AlertEntity{{AgencyID: "NJT", RouteID: "10"}},
			RouteIDs: []string{"10"},
		},
		{
			// A single trip at a single stop; the route comes from the trip.
			ID:          "TRAIN_1613_63",
			Header:      "To be Announced",
			Description: "To be Announced",
			Cause:       "UNKNOWN_CAUSE",
			Effect:      "UNKNOWN_EFFECT",
			Severity:    "UNKNOWN_SEVERITY",
			ActivePeriods: []AlertPeriod{{
				Start: time.Date(2026, 9, 18, 11, 48, 29, 0, loc),
				End:   time.Date(2026, 9, 18, 13, 53, 29, 0, loc),
			}},
			Entities: []AlertEntity{{AgencyID: "NJT", RouteID: "13", StopID: "63", TripID: "2131689"}},
			RouteIDs: []string{"13"},
			StopIDs:  []string{"63"},
			TripIDs:  []string{"2131689"},
		},
	} {
		if diff := cmp.Diff(want, byID[want.ID]); diff != "" {
			t.Errorf("Alert %q mismatch (-want +got):\n%s", want.ID, diff)
		}
	}

	// A stop-level advisory listing many stations.
	stops := byID["78621"]
	if len(stops.StopIDs) != 32 || stops.StopIDs[0] != "168" || len(stops.RouteIDs) != 0 {
		t.Errorf("Alert 78621 got StopIDs %v, RouteIDs %v; want 32 stops starting with 168 and no routes", stops.StopIDs, stops.RouteIDs)
	}
}

func TestAlertsBadFeed(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "not protobuf", body: "\xff\xff\xff"},
		{name: "json error with 200", body: `{"errorMessage":"Something went wrong."}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/GTFSRT/getToken" {
					_, _ = w.Write([]byte(`{"Authenticated":"True","UserToken":"tok"}`))
					return
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()

			c := NewClient("", "", "", WithRailData("username", "pa$$word"), WithRailDataURL(ts.URL))
			if _, err := c.Alerts(context.Background()); err == nil {
				t.Error("Alerts() expected error, got none")
			}
		})
	}
}

func TestAlertsNotConfigured(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("Unexpected request to %q", r.URL.Path)
	}))
	defer ts.Close()

	for name, c := range map[string]*Client{
		"NewClient":                NewClient(ts.URL, "username", "password"),
		"NewCustomClient":          NewCustomClient(ts.Client(), ts.URL, "username", "password"),
		"URL without WithRailData": NewClient(ts.URL, "username", "password", WithRailDataURL(ts.URL)),
	} {
		if _, err := c.Alerts(context.Background()); !errors.Is(err, ErrRailDataNotConfigured) {
			t.Errorf("%s: Alerts() error = %v, want %v", name, err, ErrRailDataNotConfigured)
		}
	}
}

func TestNewCustomClientRailData(t *testing.T) {
	f := &fakeRailData{t: t, authenticate: true}
	ts := httptest.NewServer(f)
	defer ts.Close()

	c := NewCustomClient(ts.Client(), "", "", "", WithRailDataURL(ts.URL), WithRailData("username", "pa$$word"))
	got, err := c.Alerts(context.Background())
	if err != nil {
		t.Fatalf("Alerts() error: %v", err)
	}
	if len(got) != 10 {
		t.Errorf("Alerts() returned %d alerts, want 10", len(got))
	}
}

func TestRailDataTokensPerAPI(t *testing.T) {
	f := &fakeRailData{t: t, authenticate: true}
	ts := httptest.NewServer(f)
	defer ts.Close()

	c := NewClient("", "", "", WithRailData("username", "pa$$word"), WithRailDataURL(ts.URL))
	ctx := context.Background()
	// Alternate between two APIs. The fake rejects a token issued by the
	// other API, so a shared token would fail or force a refresh each time.
	for i := 0; i < 2; i++ {
		if _, err := c.railData.fetch(ctx, railDataGTFSRT, "getAlerts"); err != nil {
			t.Fatalf("GTFSRT fetch %d: unexpected error: %v", i, err)
		}
		if _, err := c.railData.fetch(ctx, "TrainData", "getStationList"); err != nil {
			t.Fatalf("TrainData fetch %d: unexpected error: %v", i, err)
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if diff := cmp.Diff(map[string]int{"GTFSRT": 1, "TrainData": 1}, f.perAPITokenCalls); diff != "" {
		t.Errorf("getToken calls by API mismatch (-want +got):\n%s", diff)
	}
	if f.alertsCalls != 2 {
		t.Errorf("getAlerts called %d times, want 2", f.alertsCalls)
	}
}
