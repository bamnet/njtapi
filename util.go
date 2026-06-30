package njtapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// LatLng models the latitude and longitude of an object.
type LatLng struct {
	Lat float64 // Latitude
	Lng float64 // Longitude
}

// ParseError is returned when a field value cannot be parsed.
type ParseError struct {
	Field string
	Value string
	Err   error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parsing field %q value %q: %v", e.Field, e.Value, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// parseTime converts a timestamp returned from the API to a time.Time.
func (c *Client) parseTime(ts string) (time.Time, error) {
	loc := time.UTC
	if c != nil && c.location != nil {
		loc = c.location
	}
	return time.ParseInLocation("02-Jan-2006 03:04:05 PM", ts, loc)
}

// Convert a lat or long string to an actual number.
func parseDegrees(degrees string) (float64, error) {
	trim := strings.TrimSpace(degrees)
	return strconv.ParseFloat(trim, 64)
}

// parseLatLng parses latitude and longitude strings and returns a LatLng object.
// If either coordinate is empty or whitespace-only, it returns nil, nil.
func parseLatLng(lat, lng string) (*LatLng, error) {
	lat = strings.TrimSpace(lat)
	lng = strings.TrimSpace(lng)
	if lat == "" || lng == "" {
		return nil, nil
	}
	lt, err := parseDegrees(lat)
	if err != nil {
		return nil, err
	}
	ln, err := parseDegrees(lng)
	if err != nil {
		return nil, err
	}
	return &LatLng{Lat: lt, Lng: ln}, nil
}
