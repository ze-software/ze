// Design: docs/architecture/core-design.md -- value-only protocol registry.

package redistevents

import (
	"maps"
	"reflect"
	"slices"
	"testing"
)

func TestPreserveForTestRestoresRegistry(t *testing.T) {
	t.Cleanup(PreserveForTest())
	ResetForTest()
	owned := RegisterProtocol("preserved-os-owner")
	plain := RegisterProtocol("preserved-plain")
	RegisterProducer(owned)
	RegisterOSInstalled(owned)
	wantEntries := slices.Clone(entries)
	wantNames := maps.Clone(byName)
	restore := PreserveForTest()
	RegisterProducer(plain)
	RegisterOSInstalled(plain)
	RegisterProtocol("temporary-protocol")
	ResetForTest()
	RegisterProtocol("replacement-protocol")
	restore()
	if !reflect.DeepEqual(entries, wantEntries) || !reflect.DeepEqual(byName, wantNames) {
		t.Fatalf("restoration lost registry values: entries=%+v names=%v", entries, byName)
	}
	if got := RegisterProtocol("after-restoration"); got != ProtocolID(len(wantEntries)) {
		t.Fatalf("restoration changed the next protocol identity: got %d want %d", got, len(wantEntries))
	}
	restore()
	if _, known := ProtocolIDOf("after-restoration"); !known {
		t.Fatal("a repeated restoration erased later registrations")
	}
}
