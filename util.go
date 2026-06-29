package njtapi

import (
	"fmt"
	"log"
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

var tz *time.Location

func init() {
	var err error
	// Hardcode the timezone to New York.
	// While New Jersey and New York are separate states,
	// New Jersey would never dream of using a different timezone
	// than New York without triggering some sort of proxy-war.
	tz, err = time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("unable to load timezone: %v", err)
	}
}

// Convert a timestamp returned from the API to a time.Time.
func parseTime(ts string) (time.Time, error) {
	return time.ParseInLocation("02-Jan-2006 03:04:05 PM", ts, tz)
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
