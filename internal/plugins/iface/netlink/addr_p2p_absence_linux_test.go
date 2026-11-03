//go:build linux

// Design: docs/features/interfaces.md -- Exact-pair absence is not lookup failure.
// Related: addr_p2p_exact_linux_test.go -- Guarded-removal fixture.

package ifacenetlink

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"
)

// TestP2PAbsentSentinelPreventsDelete proves only a valid complete dump can
// report absence; malformed entries remain errors even after finding a match.
func TestP2PAbsentSentinelPreventsDelete(t *testing.T) {
	for _, tc := range []struct {
		name      string
		installed []netlink.Addr
		absent    bool
	}{
		{"empty", nil, true},
		{"different-peer", []netlink.Addr{*exactRemovalPair(t, "198.51.100.2/32", "192.0.2.1/24")}, true},
		{"invalid-dump", []netlink.Addr{{}}, false},
		{"invalid-after-match", []netlink.Addr{*exactRemovalPair(t, "198.51.100.2/32", "192.0.2.9/24"), {}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &exactRemovalFixture{
				request: exactRemovalPair(t, "198.51.100.2/32", "192.0.2.9/24"), installed: tc.installed,
			}
			target, ok := toDeviceAddress(f.request)
			if !ok {
				t.Fatal("valid request rejected")
			}
			err := removeAddressGuarded(f, "ppp7", target)
			if err == nil {
				t.Fatal("guard accepted an absent or invalid dump")
			}
			if errors.Is(err, errP2PPairAbsent) != tc.absent {
				t.Fatalf("absence classification = %v, want absent=%v", err, tc.absent)
			}
			if f.deleted != nil {
				t.Fatal("unresolved pair reached Delete")
			}
			if len(f.order) != 1 {
				t.Fatalf("operations = %v, want list only", f.order)
			}
			if f.order[0] != "list" {
				t.Fatalf("operation = %s", f.order[0])
			}
		})
	}
}
