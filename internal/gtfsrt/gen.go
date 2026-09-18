// Package gtfsrt is a private copy of the GTFS Realtime protobuf bindings.
//
// It exists instead of a dependency on a public bindings module because the
// Go protobuf runtime panics at init when two packages register the same
// proto file or message names. A program using this library alongside any
// other GTFS Realtime bindings (proto package transit_realtime) would fail to
// start. See njtapi_gtfs_realtime.proto for what differs from upstream.
package gtfsrt

//go:generate protoc --proto_path=. --go_out=. --go_opt=paths=source_relative njtapi_gtfs_realtime.proto
