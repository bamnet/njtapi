# NJTAPI

[![GoDoc](https://godoc.org/github.com/bamnet/njtapi?status.svg)](https://godoc.org/github.com/bamnet/njtapi)
[![Build Status](https://github.com/bamnet/njtapi/actions/workflows/test.yaml/badge.svg)](https://github.com/bamnet/njtapi/actions)
[![codecov](https://codecov.io/gh/bamnet/njtapi/branch/master/graph/badge.svg)](https://codecov.io/gh/bamnet/njtapi)
[![Go Report Card](https://goreportcard.com/badge/github.com/bamnet/njtapi)](https://goreportcard.com/report/github.com/bamnet/njtapi)
[![GitHub](https://img.shields.io/github/license/bamnet/njtapi)](https://github.com/bamnet/njtapi/blob/master/LICENSE)

NJTAPI is a Go library for accessing data about NJTransit Trains. It wraps the NJTransit HTTP API.

Features include:

*  Timetables and statuses of departures from each station.
*  Train status including location and stops.
*  List of all the train stations in the system.

See the [GoDoc](https://godoc.org/github.com/bamnet/njtapi) for full details.

## Installation

```go
import "github.com/bamnet/njtapi"
```

## API Access

Register with the [NJTransit Developer Portal](https://datasource.njtransit.com)
to get a username and password needed to call the API.

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
	fmt.Println("Departures from New York Penn Station")
	for _, departures := range station.Departures {
		fmt.Printf("Train to %s at %s", departures.Destination, departures.ScheduledDepartureDate)
	}
}
```

## Demo

Run [demo.go](demo/demo.go) for a working demo using a command like:

```shell
go run demo/demo.go --base_url="http://njttraindata_tst.njtransit.com:8090/njttraindata.asmx/" --username=<USERNAME> --password=<PASSWORD>
```

Note: Both of the samples above point to a _testing_ api server, not the production one.
