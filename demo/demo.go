// Package main demos functionality of the `njtapi` library.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/bamnet/njtapi"
)

var (
	baseURL  = flag.String("base_url", "", "NJTransit API base URL.")
	username = flag.String("username", "", "Username to authenticate with.")
	password = flag.String("password", "", "Password to authenticate with.")
)

func main() {
	flag.Parse()

	c := njtapi.NewClient(*baseURL, *username, *password)
	trains, err := c.VehicleData(context.Background())
	if err != nil {
		log.Fatalf("VehicleData() error: %v", err)
	}
	log.Printf("%+v", trains)
}
