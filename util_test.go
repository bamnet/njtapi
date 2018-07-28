package njtapi

import (
	"testing"
	"time"
)

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
