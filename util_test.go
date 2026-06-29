package njtapi

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestParseError(t *testing.T) {
	inner := errors.New("bad value")
	e := &ParseError{Field: "TIME", Value: "garbage", Err: inner}
	want := "parsing field \"TIME\" value \"garbage\": bad value"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(e, inner) {
		t.Errorf("expected errors.Is(e, inner) to be true")
	}
}

func TestParseTime(t *testing.T) {
	for _, r := range []struct {
		timestamp string
		time      time.Time
	}{
		{"28-Jul-2018 12:01:07 AM", time.Date(2018, 7, 28, 0, 1, 7, 0, tz)},
		{"28-Jul-2018 01:05:30 PM", time.Date(2018, 7, 28, 13, 5, 30, 0, tz)},
	} {
		if got, _ := parseTime(r.timestamp); got != r.time {
			t.Errorf("parseTime(%s) got %v want %v", r.timestamp, got, r.time)
		}
	}
}

func TestParseDegrees(t *testing.T) {
	for _, r := range []struct {
		input   string
		want    float64
		wantErr bool
	}{
		{" 40.7347 ", 40.7347, false},
		{"-74.0311", -74.0311, false},
		{"abc", 0, true},
		{"", 0, true},
	} {
		got, err := parseDegrees(r.input)
		if (err != nil) != r.wantErr {
			t.Errorf("parseDegrees(%q) error status got %v wantErr %v", r.input, err != nil, r.wantErr)
		}
		if got != r.want {
			t.Errorf("parseDegrees(%q) got %v want %v", r.input, got, r.want)
		}
	}
}

func TestParseLatLng(t *testing.T) {
	for _, r := range []struct {
		lat     string
		lng     string
		want    *LatLng
		wantErr bool
	}{
		{"40.7347", "-74.0311", &LatLng{Lat: 40.7347, Lng: -74.0311}, false},
		{" ", " ", nil, false},
		{"", "", nil, false},
		{"40.7347", " ", nil, false},
		{"40.7347", "abc", nil, true},
		{"abc", "-74.0311", nil, true},
	} {
		got, err := parseLatLng(r.lat, r.lng)
		if (err != nil) != r.wantErr {
			t.Errorf("parseLatLng(%q, %q) error status got %v wantErr %v", r.lat, r.lng, err != nil, r.wantErr)
		}
		if diff := cmp.Diff(r.want, got); diff != "" {
			t.Errorf("parseLatLng(%q, %q) mismatch (-want +got):\n%s", r.lat, r.lng, diff)
		}
	}
}
