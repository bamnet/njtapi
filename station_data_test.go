package njtapi

import (
	"context"
	"fmt"
	"log"
)

func ExampleClient_StationData() {
	// Update these values.
	username := "your username"
	password := "your password"

	client := NewClient("http://njttraindata_tst.njtransit.com:8090/njttraindata.asmx/", username, password)
	station, err := client.StationData(context.Background(), "NY")
	if err != nil {
		log.Fatalf("StationData() error: %v", err)
	}
	for _, departures := range station.Departures {
		fmt.Printf("Train to %s at %s", departures.Destination, departures.ScheduledDepartureDate)
	}
}
