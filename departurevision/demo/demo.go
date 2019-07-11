// Package main demos functionality of the `departurevision` library.
package main

import (
	"context"
	"flag"
	"log"

	dv "github.com/bamnet/njtapi/departurevision"
)

var (
	baseURL = flag.String("base_url", "http://dv.njtransit.com/mobile/tid-mobile.aspx", "NJTransit DepartureVision base URL.")
)

func main() {
	flag.Parse()

	c := dv.NewClient(*baseURL)
	trains, err := c.Departures(context.Background(), "NY")
	if err != nil {
		log.Fatalf("Departures(NY) error: %v", err)
	}
	log.Printf("%+v", trains)
}
