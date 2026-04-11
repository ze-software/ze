package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestDefaultOriginateFilterFailsClosedWithoutReactor verifies that the
// default-originate conditional filter fails closed when the peer has no
// reactor attached.
//
// VALIDATES: cmd-2 AC-7 guardrail -- a filter that cannot be evaluated
// must not silently originate the default route.
// PREVENTS: A missing reactor/API causing default routes to leak out
// unfiltered while the operator believes the filter is enforcing policy.
func TestDefaultOriginateFilterFailsClosedWithoutReactor(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	peer := NewPeer(settings)

	// No reactor attached -- fail-closed branch.
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	prefix := netip.MustParsePrefix("0.0.0.0/0")
	nh := netip.MustParseAddr("10.0.0.1")

	ok := peer.defaultOriginateFilterAccepts("policy:drop-all", fam, prefix, nh)
	assert.False(t, ok, "missing reactor must fail closed to prevent unfiltered origination")
}

// TestDefaultOriginateFilterFailsClosedOnMalformedRef verifies that a
// malformed filter reference (no "<plugin>:<filter>" colon) fails closed
// instead of being silently ignored.
//
// VALIDATES: cmd-2 AC-7 guardrail -- invalid config must not let a
// default route escape without filtering.
// PREVENTS: Typos in filter names ("drop" instead of "policy:drop")
// silently disabling the filter and originating the default.
func TestDefaultOriginateFilterFailsClosedOnMalformedRef(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01020304)
	peer := NewPeer(settings)
	// Attach a reactor so the nil-reactor branch is not taken.
	r := &Reactor{}
	peer.SetReactor(r)

	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	prefix := netip.MustParsePrefix("0.0.0.0/0")
	nh := netip.MustParseAddr("10.0.0.1")

	ok := peer.defaultOriginateFilterAccepts("missing-colon", fam, prefix, nh)
	assert.False(t, ok, "malformed filter ref must fail closed")
}
