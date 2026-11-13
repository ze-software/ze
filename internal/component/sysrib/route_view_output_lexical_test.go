// Design: docs/architecture/rib/unified-locrib.md -- owned forwarding values.
// Related: register_route_view_output.go -- bounded lexical row extraction.

package sysrib

import (
	"net/netip"
	"reflect"
	"strings"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// Distinct rich rows must survive lexical buffer reuse and caller wire reuse.
// A single-row control first proves the ordinary metadata fixture is valid.
func TestRouteViewOutputDecoderOwnsDistinctRows(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	redistevents.RegisterProtocol("static")
	control := routeViewOutputBatch()
	decoded, err := routeViewDecodeOutputBatch(mustOutputJSON(t, &control))
	if err != nil || !reflect.DeepEqual(decoded, control) {
		t.Fatalf("single-row metadata control failed: decoded=%+v error=%v", decoded, err)
	}
	batch := routeViewOutputBatch()
	batch.Changes = make([]sysribevents.BestChangeEntry, 3)
	prefixes := []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"}
	devices := []string{"a", "b", "c"}
	for i := range batch.Changes {
		entry := routeViewCloneOutputEntry(control.Changes[0])
		entry.Prefix = netip.MustParsePrefix(prefixes[i])
		entry.Interface = devices[i]
		entry.Metric += uint32(i)
		entry.Labels[0] += uint32(i)
		entry.ECMPPaths = make([]sysribevents.ECMPPath, sysribevents.MaxECMPPaths)
		for member := range entry.ECMPPaths {
			entry.ECMPPaths[member] = sysribevents.ECMPPath{
				NextHop: netip.MustParseAddr("198.51.100.2"),
				Interface: strings.Repeat(devices[i], 160),
				OnLink: true, Weight: uint8(member + 1),
				Labels: []uint32{100 + uint32(member), 200 + uint32(i)},
			}
		}
		batch.Changes[i] = entry
		if row := mustOutputJSON(t, &entry); len(row) <= 4096 {
			t.Fatalf("row %d did not exceed the lexical decoder's buffer: %d bytes", i, len(row))
		}
	}
	if err := routeViewValidateOutput(&batch); err != nil {
		t.Fatalf("rich rows violated the existing forwarding contract: %v", err)
	}
	wire := mustOutputJSON(t, &batch)
	decoded, err = routeViewDecodeOutputBatch(wire)
	if err != nil || !reflect.DeepEqual(decoded, batch) {
		t.Fatalf("lexical buffer reuse changed distinct metadata: decoded=%+v error=%v", decoded, err)
	}
	clear(wire)
	if !reflect.DeepEqual(decoded, batch) {
		t.Fatal("caller wire reuse changed decoded forwarding values")
	}
}
