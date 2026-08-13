// VALIDATES: BestChangeEntry carries the FIB forwarding action across the
// cross-process event-bus rail, with the same json tag sysrib republishes.
// PREVENTS: the route type being dropped at the plugin process boundary, which
// would leave the forked-plugin deployment forwarding a prefix the in-process
// deployment discards.

package ribevents

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/routetype"
)

func TestBestChangeEntryRouteTypeRoundTrip(t *testing.T) {
	in := BestChangeEntry{
		Action:    BestChangeAdd,
		Prefix:    netip.MustParsePrefix("192.0.2.1/32"),
		NextHop:   netip.MustParseAddr("198.51.100.1"),
		RouteType: routetype.Blackhole,
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"route-type":6`) {
		t.Errorf("route-type absent or renamed in %s", data)
	}

	var out BestChangeEntry
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.RouteType != routetype.Blackhole {
		t.Errorf("RouteType = %v, want %v", out.RouteType, routetype.Blackhole)
	}
}

// An unstamped entry MUST NOT emit the key at all. Every existing producer
// leaves it zero, and a `"route-type":0` on the wire would be a new field for
// external plugin processes to decode.
func TestBestChangeEntryRouteTypeOmittedWhenUnset(t *testing.T) {
	data, err := json.Marshal(BestChangeEntry{
		Action: BestChangeAdd,
		Prefix: netip.MustParsePrefix("192.0.2.0/24"),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "route-type") {
		t.Errorf("unset route-type emitted in %s", data)
	}
}
