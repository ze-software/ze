// Design: docs/architecture/policyroute/netlink-int-field-truncation.md -- netlink int width

package policyroute

import (
	"math"
	"testing"
)

// VALIDATES: netlinkTableInt accepts a table ID exactly when it survives the
// conversion to the Go int netlink.Rule.Table is typed as, and returns the
// value unchanged when it does.
// PREVENTS: the per-architecture split silently narrowing the usable table
// range on the 64-bit targets Ze ships. netlinkint_linux_amd64.go and
// netlinkint_linux_arm64.go must convert every uint32 exactly; bounding them at
// MaxInt32 (as was briefly shipped for CodeQL alert 171) would make
// test/parse/netlink-int-field-range.ci a runtime failure instead of a config
// one.
//
// The assertion is the invariant, not a fixed outcome, because the answer
// genuinely differs on a 32-bit build: uint32(int(v)) round-trips on 64-bit, so
// "large value rejected" cannot be asserted portably.
func TestNetlinkTableIntMatchesBuildIntWidth(t *testing.T) {
	for _, v := range []uint32{0, 1, 254, 4000, math.MaxInt32, math.MaxInt32 + 1, math.MaxUint32} {
		fits := uint64(v) <= uint64(math.MaxInt)

		got, err := netlinkTableInt(v)
		if fits != (err == nil) {
			t.Errorf("netlinkTableInt(%d): fits in int = %v, accepted = %v (err=%v)", v, fits, err == nil, err)
			continue
		}
		if err == nil && uint32(got) != v {
			t.Errorf("netlinkTableInt(%d) = %d, want the value unchanged", v, got)
		}
	}
}
