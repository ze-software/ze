package rs

import (
	"strings"
	"testing"

	rsyang "github.com/ze-software/ze/internal/component/bgp/plugins/rs/yang"
	bgpyang "github.com/ze-software/ze/internal/component/bgp/yang"
)

// TestRSConfigOwnedByPlugin asserts the route-server per-peer config leaves are
// owned by the rs plugin's YANG (augmenting core), not defined in core BGP YANG,
// so deleting this plugin removes them from the schema together with the
// forwarding capability -- restoring the "delete the folder, the feature
// vanishes" invariant for the config surface as well as the code path.
//
// VALIDATES: P1 AC-3 -- rs-client and rs-fast-path live in ze-rs-conf and are
// absent from ze-bgp-conf; they attach to peers via augment.
// PREVENTS: a regression that re-adds either leaf to core BGP, restoring the
// split ownership the invariant closes.
func TestRSConfigOwnedByPlugin(t *testing.T) {
	for _, leaf := range []string{"rs-client", "rs-fast-path"} {
		if !strings.Contains(rsyang.ZeRsConfYANG, "leaf "+leaf) {
			t.Errorf("plugin YANG ze-rs-conf is missing leaf %q -- config ownership not moved", leaf)
		}
		if strings.Contains(bgpyang.ZeBGPConfYANG, "leaf "+leaf) {
			t.Errorf("core YANG ze-bgp-conf still defines leaf %q -- must be plugin-owned", leaf)
		}
	}
	// The leaves must be contributed by augmenting core, not a stranded grouping.
	if !strings.Contains(rsyang.ZeRsConfYANG, "augment") {
		t.Error("plugin YANG ze-rs-conf has no augment: rs leaves would not attach to peer config")
	}
}
