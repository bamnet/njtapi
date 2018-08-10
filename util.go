package njtapi

import (
	"log"
	"time"
)

// LatLng models the latitude and longitude of an object.
type LatLng struct {
	Lat float64 // Latitude
	Lng float64 // Longitude
}

var tz *time.Location

func init() {
	var err error
	// Hardcode the timezone to New York.
	// While New Jersey and New York are seperate states,
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
