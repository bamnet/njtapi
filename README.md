# NJTAPI

[![GoDoc](https://godoc.org/github.com/bamnet/njtapi?status.svg)](https://godoc.org/github.com/bamnet/njtapi)
[![Build Status](https://travis-ci.com/bamnet/njtapi.svg?branch=master)](https://travis-ci.com/bamnet/njtapi)

NJTAPI provides a Go library for accessing data about NJTransit Trains via their HTTP API.

Features include:

*  Departure board style information for each train station.
*  Train status including location and stops.

## Installation

```go
import "github.com/bamnet/njtapi"
```

## Example Usage

```go
func main() {
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
```

Run [demo.go](demo/demo.go) for a working demo using a command like:

```shell
go run demo/demo.go --base_url="http://njttraindata_tst.njtransit.com:8090/njttraindata.asmx/" --username=<USERNAME> --password=<PASSWORD>
```

Note: Both of the samples above point to a _testing_ api server, not the production one.