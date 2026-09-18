module github.com/bamnet/njtapi

go 1.23

require (
	github.com/google/go-cmp v0.7.0
	google.golang.org/protobuf v1.36.12
)

retract v1.3.0 // Panics at init when linked with other GTFS Realtime bindings; use v1.3.1.
