package gtfsrt

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// TestNoSharedRegistryNames guards against the bindings reverting to the
// upstream proto package or file name, which panics at init in any program
// that also links another copy of the GTFS Realtime bindings.
func TestNoSharedRegistryNames(t *testing.T) {
	if got := File_njtapi_gtfs_realtime_proto.Package(); got == "transit_realtime" {
		t.Errorf("proto package = %q, must not be the upstream package", got)
	}
	for _, path := range []string{"gtfs-realtime.proto", "proto/gtfs-realtime.proto"} {
		if _, err := protoregistry.GlobalFiles.FindFileByPath(path); !errors.Is(err, protoregistry.NotFound) {
			t.Errorf("FindFileByPath(%q) = %v, want NotFound", path, err)
		}
	}
	name := protoreflect.FullName("transit_realtime.FeedMessage")
	if _, err := protoregistry.GlobalTypes.FindMessageByName(name); !errors.Is(err, protoregistry.NotFound) {
		t.Errorf("FindMessageByName(%q) = %v, want NotFound", name, err)
	}
}
