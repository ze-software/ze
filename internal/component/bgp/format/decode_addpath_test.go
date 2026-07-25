package format

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestFormatCapabilityAddPathSkipsInvalidMode pins the ADD-PATH capability
// decode display after the move to AddPathMode.Label().
//
// VALIDATES: formatCapability emits one entry per family carrying a valid RFC
// 7911 direction ("receive"/"send"/"send-receive") and drops families whose
// mode is AddPathNone or an out-of-range value.
// PREVENTS: the regression where a None or unknown mode produced a malformed
// "<family> " entry with an empty direction -- the former inline switch had no
// default arm, so an unrecognized mode left the rendered direction empty
// instead of skipping the family.
func TestFormatCapabilityAddPathSkipsInvalidMode(t *testing.T) {
	addPathCap := &capability.AddPath{Families: []capability.AddPathFamily{
		{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathReceive},
		// AddPathNone is not a valid negotiated mode -- skipped.
		{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathNone},
		// Out-of-range mode (corrupt/forward-incompatible peer) -- skipped.
		{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast, Mode: capability.AddPathMode(99)},
		{AFI: capability.AFIIPv6, SAFI: capability.SAFIMulticast, Mode: capability.AddPathBoth},
	}}

	got := formatCapability(addPathCap)

	want := []string{"ipv4/unicast receive", "ipv6/multicast send-receive"}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries (None and invalid skipped), got %d: %+v",
			len(want), len(got), got)
	}
	for i, w := range want {
		if got[i].Name != "addpath" {
			t.Errorf("entry %d: Name = %q, want addpath", i, got[i].Name)
		}
		if got[i].Value != w {
			t.Errorf("entry %d: Value = %q, want %q", i, got[i].Value, w)
		}
	}
}
