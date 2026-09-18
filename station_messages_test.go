package njtapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func ExampleClient_StationMessages() {
	// Update these values.
	username := "your username"
	password := "your password"

	client := NewClient("http://njttraindata_tst.njtransit.com:8090/njttraindata.asmx/", username, password)
	msgs, err := client.StationMessages(context.Background(), "NY", "")
	if err != nil {
		log.Fatalf("StationMessages() error: %v", err)
	}
	for _, m := range msgs {
		fmt.Printf("%s: %s\n", m.PublishedAt, m.Text)
	}
}

func TestStationMessages(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("Error loading timezones: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("Error parsing request: %v", err)
		}
		if u, p := r.Form.Get("username"), r.Form.Get("password"); u != "username" || p != "pa$$word" {
			t.Errorf("Missing expected username & password: %v", r.Form)
		}
		// The endpoint rejects requests that omit a parameter, even an empty one.
		if _, ok := r.Form["trainLine"]; !ok {
			t.Errorf("Missing trainLine parameter: %v", r.Form)
		}
		switch r.Form.Get("station") {
		case "NY":
			http.ServeFile(w, r, "testdata/getStationMSG1.xml")
		case "":
			http.ServeFile(w, r, "testdata/getStationMSGEmpty.xml")
		case "BAD":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<STATION>
  <STATION_2CHAR>BAD</STATION_2CHAR>
  <BANNERMSGS>
    <MSG>
      <PubDate>not a date</PubDate>
      <MSGText>  Some text  </MSGText>
      <MSGID>1</MSGID>
      <MSGType>banner</MSGType>
      <MSGAgency>NJT</MSGAgency>
    </MSG>
  </BANNERMSGS>
</STATION>`))
		}
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")

	for _, r := range []struct {
		station    string
		want       []StationMessage
		wantErrors int
	}{
		{
			station: "NY",
			want: []StationMessage{
				{
					ID:          "2609171032561",
					Text:        "Hamilton & Princeton Junction Stations: Track 4 Unavailable ? Weekends from Friday, September 18, through Monday, October 5, 2026",
					Type:        "banner",
					Agency:      "NJT",
					PublishedAt: time.Date(2026, 9, 17, 10, 32, 35, 0, loc),
				},
				{
					ID:          "2609171044191",
					Text:        "North Jersey Coast Line: Possible Delays due to Track Maintenance ? Weekdays from Monday, September 28 until Friday, October 9, 2026",
					Type:        "banner",
					Agency:      "NJT",
					PublishedAt: time.Date(2026, 9, 17, 10, 43, 36, 0, loc),
				},
				{
					ID:          "2609171136382",
					Text:        "Temporary Rail Service Changes October 11, 2026 ? November 14, 2026* Portal North Bridge Enters Final Phase of Construction as Work Begins to Put the Second of Two Tracks into Service",
					Type:        "banner",
					Agency:      "NJT",
					PublishedAt: time.Date(2026, 9, 17, 11, 36, 11, 0, loc),
				},
			},
		},
		{
			station: "",
			want:    nil,
		},
		{
			station:    "BAD",
			want:       []StationMessage{{ID: "1", Text: "Some text", Type: "banner", Agency: "NJT"}},
			wantErrors: 1,
		},
	} {
		got, err := c.StationMessages(context.Background(), r.station, "")
		if err != nil {
			t.Errorf("StationMessages(%q) unexpected error: %v", r.station, err)
		}
		if diff := cmp.Diff(r.want, got, cmpopts.IgnoreFields(StationMessage{}, "ParseErrors")); diff != "" {
			t.Errorf("StationMessages(%q) mismatch (-want +got):\n%s", r.station, diff)
		}
		gotErrors := 0
		for _, m := range got {
			for _, e := range m.ParseErrors {
				var pe *ParseError
				if !errors.As(e, &pe) {
					t.Errorf("StationMessages(%q) expected *ParseError, got %T", r.station, e)
				}
				gotErrors++
			}
		}
		if gotErrors != r.wantErrors {
			t.Errorf("StationMessages(%q) got %d parse errors, want %d", r.station, gotErrors, r.wantErrors)
		}
	}
}

func TestStationMessagesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")
	if _, err := c.StationMessages(context.Background(), "NY", ""); err == nil {
		t.Error("StationMessages() expected error, got none.")
	}
}

func TestStationDataMessages(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("Error loading timezones: %v", err)
	}

	// getTrainScheduleXML carries the same BANNERMSGS element as
	// getStationMSGXML.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<STATION>
  <STATION_2CHAR>NY</STATION_2CHAR>
  <STATIONNAME>New York</STATIONNAME>
  <BANNERMSGS>
    <MSG>
      <PubDate>9/17/2026 10:43:36 AM</PubDate>
      <MSGText>North Jersey Coast Line: Possible Delays due to Track Maintenance</MSGText>
      <MSGID>2609171044191</MSGID>
      <MSGType>banner</MSGType>
      <MSGAgency>NJT</MSGAgency>
    </MSG>
  </BANNERMSGS>
  <ITEMS />
</STATION>`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")
	got, err := c.StationData(context.Background(), "NY")
	if err != nil {
		t.Fatalf("StationData() error: %v", err)
	}
	want := &Station{
		ID:         "NY",
		Name:       "New York",
		Departures: []StationTrain{},
		Messages: []StationMessage{{
			ID:          "2609171044191",
			Text:        "North Jersey Coast Line: Possible Delays due to Track Maintenance",
			Type:        "banner",
			Agency:      "NJT",
			PublishedAt: time.Date(2026, 9, 17, 10, 43, 36, 0, loc),
		}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("StationData() mismatch (-want +got):\n%s", diff)
	}
}
