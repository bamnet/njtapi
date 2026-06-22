# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
# Run all tests (with race detector and coverage)
go test -race -coverprofile=coverage.txt -covermode=atomic -v ./...

# Run a single test
go test -run TestFunctionName ./...

# Build
go build -v ./...

# Run the demo (requires NJTransit credentials)
go run demo/demo.go --base_url="http://njttraindata_tst.njtransit.com:8090/njttraindata.asmx/" --username=<USERNAME> --password=<PASSWORD>
```

## Architecture

This is a single-package Go library (`github.com/bamnet/njtapi`) that wraps the NJTransit HTTP API. The API returns XML responses, authenticated via username/password query parameters.

**Key design decisions:**
- The library makes opinionated decisions about data sanitization — it does not provide a 1:1 mapping of the API spec.
- All timestamps are hardcoded to `America/New_York` timezone (see `util.go`).
- Amtrak trains (IDs like "A123") are silently skipped in `StationData`.
- `VehicleData` deduplicates trains by ID, keeping the most recently modified entry.

**API endpoints and their methods:**

| Method | Endpoint | Returns |
|---|---|---|
| `StationData(ctx, stationID)` | `getTrainScheduleXML` | Departures from a station with per-train stop lists |
| `StationList(ctx)` | `getStationListXML` | All stations; enriched with aliases from `extra_stations.go` |
| `VehicleData(ctx)` | `getVehicleDataXML` | All active trains (location, delay, next stop) |
| `GetTrainMap(ctx, trainID)` | `getTrainMapXML` | Single train: location + track circuit only |
| `GetTrainStops(ctx, trainID)` | `getTrainStopListXML` | Single train: full stop list with connecting lines |

**`GetTrainMap` and `GetTrainStops` return partial `Train` objects** — the API endpoints expose different subsets of fields. See godoc comments on each method for which fields are populated.

**Tests use local XML fixtures** in `testdata/` rather than hitting the live API. Each `*_test.go` file reads the corresponding fixture file to drive tests.

**Station aliases** (`extra_stations.go`) exist because the NJTransit API returns inconsistent station names across endpoints. The `StationList` method merges these aliases into the `Station.Aliases` field.
