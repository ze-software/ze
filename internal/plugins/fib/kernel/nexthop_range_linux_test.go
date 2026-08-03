// VALIDATES: buildRichRoute carries a route's Metric and TableID into
// netlink.Route.Priority / .Table exactly, across the whole uint32 range this
// build can program, and refuses any value it cannot.
// PREVENTS: CodeQL go/incorrect-integer-conversion (alert 170) in its
// fib-kernel form. netlink.Route.Priority and .Table are Go ints and the
// encoder emits RTA_PRIORITY / RTA_TABLE only for positive values
// (vendor/github.com/vishvananda/netlink/route_linux.go:1058,1069), so on a
// 32-bit build a uint32 above MaxInt32 turns negative and the attribute is
// dropped without an error: a redistributed route silently lands in
// RT_TABLE_MAIN at the kernel default metric. Linux-only (netlink); runs under
// QEMU per ai/rules/platform-linux.md.

//go:build linux

package fibkernel

import (
	"math"
	"net/netip"
	"testing"
)

func TestBuildRichRouteIntFieldRange(t *testing.T) {
	for _, v := range []uint32{1, 200, math.MaxInt32, math.MaxUint32} {
		r := RichRoute{
			Prefix:  netip.MustParsePrefix("10.20.0.0/24"),
			NextHop: netip.MustParseAddr("10.0.0.2"),
			Metric:  v,
			TableID: v,
		}
		route, err := buildRichRoute(r)

		if uint64(v) > maxNetlinkInt {
			if err == nil {
				t.Errorf("value %d: accepted a value that does not fit in int", v)
			}
			continue
		}
		if err != nil {
			t.Errorf("value %d: unexpected error: %v", v, err)
			continue
		}
		if uint32(route.Priority) != v {
			t.Errorf("value %d: Priority = %d, want %d", v, route.Priority, v)
		}
		if uint32(route.Table) != v {
			t.Errorf("value %d: Table = %d, want %d", v, route.Table, v)
		}
	}
}
