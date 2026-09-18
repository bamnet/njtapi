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

This is a single-package Go library (`github.com/bamnet/njtapi`) whose single public client type, **`Client`**, wraps two NJTransit HTTP APIs. Which API serves a method is an internal detail, not part of the public surface:

- **Legacy ASMX train data API** — XML responses, authenticated via username/password query parameters. Configured by the `NewClient` / `NewCustomClient` / `NewClientWithLocation` arguments.
- **RailData** (`railDataBaseURL`, `https://raildata.njtransit.com/api/`) — enabled with the functional option `WithRailData(username, password)`; `WithRailDataURL(base)` overrides the root for tests. The constructors take `...Option`, so new optional features should be added as options rather than new constructors or client types. RailData credentials are **separate** from the legacy ones; neither set works on the other API. Methods that need RailData return `ErrRailDataNotConfigured` without the option. RailData hosts several APIs under the root (`GTFSRT/`, `TrainData/`); only GTFSRT is used so far. Every call is a multipart form POST: `<api>/getToken` takes `username`/`password` and returns JSON `{"Authenticated":"True","UserToken":"..."}` (`"False"` and an empty token on bad credentials, mapped to `ErrAuthenticationFailed`); data calls take only `token`. An invalid token gets HTTP 500 with `{"errorMessage":"Invalid token."}`. **Tokens are per API**: one login works on every API, but a GTFSRT token is rejected by TrainData and vice versa. The unexported `railData` type (`raildata.go`) therefore keeps one lazily fetched token per API path, each under its own mutex (which also serializes that API's token requests), and on an invalid-token response fetches a new one and retries once. Tokens are never included in errors. The GTFSRT API also lists `isValidToken`, `getGTFS`, `getTripUpdates` and `getVehiclePositions` (swagger: `https://raildata.njtransit.com/swagger/GTFS/swagger.json`), which aren't wrapped yet. Don't capture `isValidToken` responses into fixtures: they include the account's organisation name.

**Key design decisions:**
- The library makes opinionated decisions about data sanitization — it does not provide a 1:1 mapping of the API spec.
- All timestamps are hardcoded to `America/New_York` timezone (see `util.go`).
- Amtrak trains (IDs like "A123") are silently skipped in `StationData`.
- `VehicleData` deduplicates trains by ID, keeping the most recently modified entry.

**API endpoints and their methods:**

| Method | Endpoint | Returns |
|---|---|---|
| `StationData(ctx, stationID)` | `getTrainScheduleXML` | Departures from a station with per-train stop lists (incl. per-stop `STOP_STATUS`) and station banner messages |
| `StationMessages(ctx, stationID, line)` | `getStationMSGXML` | Station banner/service messages |
| `StationList(ctx)` | `getStationListXML` | All stations; enriched with aliases from a local map in `StationList` |
| `VehicleData(ctx)` | `getVehicleDataXML` | All active trains (location, delay, next stop) |
| `GetTrainMap(ctx, trainID)` | `getTrainMapXML` | Single train: location + track circuit only |
| `GetTrainStops(ctx, trainID)` | `getTrainStopListXML` | Single train: full stop list with connecting lines |
| `Alerts(ctx)` | RailData `GTFSRT/getAlerts` (GTFS-rt, needs `WithRailData`) | Service alerts with header/description, cause/effect, active periods and informed routes/stops/trips (GTFS IDs) |

**Service alerts** are decoded with `gtfs-realtime-bindings` into the library-owned `Alert` type rather than exposing the protobuf types. NJTransit sends `cause`/`effect` as `UNKNOWN_*` only, informed entities carry GTFS `route_id`, `stop_id` or a `trip` (whose `route_id` fills `AlertEntity.RouteID`), and alert text has `?` where an en dash was; the text is left as sent.

**Station messages** come from the `BANNERMSGS>MSG` element, which has the same shape in `getTrainScheduleXML` and `getStationMSGXML` (`PubDate` as `9/17/2026 10:32:35 AM`, `MSGText`, `MSGID`, `MSGType`, `MSGAgency`). The API double-escapes message text (`&amp;amp;`), so it is HTML-unescaped once after XML decoding. Calling `getStationMSGXML` with an empty station returns an empty `<FULLSCREENMSGS />` root whose populated shape hasn't been observed, so it isn't parsed. `STOP_STATUS` values are free text (`OnTime`, `ON TIME`, `Late`, `Delayed`, `BOARDING`, `ALL ABOARD`, `STAND BY`, `2 HOURS LATE`, ...) and are only trimmed, not normalized.

**`GetTrainMap` and `GetTrainStops` return partial `Train` objects** — the API endpoints expose different subsets of fields. See godoc comments on each method for which fields are populated.

**Tests use local fixtures** in `testdata/` rather than hitting the live API (`getAlerts.pb` is a captured binary GTFS-rt alerts feed). Each `*_test.go` file reads the corresponding fixture file to drive tests.

**Station aliases** (a local map declared inside `StationList` in `station_data.go`) exist because the NJTransit API returns inconsistent station names across endpoints. The `StationList` method merges these aliases into the `Station.Aliases` field.
