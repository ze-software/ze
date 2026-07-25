package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/family"
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

// fakeFilterRawInfo is a filterRawInfo stub that reports a fixed raw flag and
// records the plugin/filter names it was queried with.
type fakeFilterRawInfo struct {
	raw       bool
	gotPlugin string
	gotFilter string
}

func (f *fakeFilterRawInfo) FilterInfo(pluginName, filterName string) ([]string, bool) {
	f.gotPlugin, f.gotFilter = pluginName, filterName
	return nil, f.raw
}

// TestDefaultOriginateRejectsRawFilter verifies a filter declared raw=true is
// rejected as a default-originate gate: the synthetic default route has no wire
// bytes, so a raw filter would evaluate empty hex and decide on nothing.
//
// VALIDATES: L119 -- raw filters bound to default-originate-filter must not
// silently gate on empty hex; fail-closed instead.
// PREVENTS: a raw filter accepting/rejecting the default route based on an empty
// raw payload, silently emitting or suppressing 0.0.0.0/0.
func TestDefaultOriginateRejectsRawFilter(t *testing.T) {
	info := &fakeFilterRawInfo{raw: true}
	rejected := defaultOriginateRejectsRawFilter(info, "policy:raw-thing", "192.0.2.1")
	assert.True(t, rejected, "a raw=true filter must be rejected for default-originate")
	assert.Equal(t, "policy", info.gotPlugin, "ref must be split on ':' before the raw lookup")
	assert.Equal(t, "raw-thing", info.gotFilter)
}

// TestDefaultOriginateAllowsNonRawFilter verifies a text (raw=false) filter is
// not blocked by the raw guard and proceeds to the normal dry-run.
//
// VALIDATES: L119 -- the raw guard only blocks raw filters, leaving text gates working.
// PREVENTS: the raw guard over-reaching and disabling legitimate text filters.
func TestDefaultOriginateAllowsNonRawFilter(t *testing.T) {
	info := &fakeFilterRawInfo{raw: false}
	rejected := defaultOriginateRejectsRawFilter(info, "policy:text-gate", "192.0.2.1")
	assert.False(t, rejected, "a text filter must not be blocked by the raw guard")
}

// TestDefaultOriginateRawGuardIgnoresMalformedRef verifies the raw guard leaves a
// ref without a ':' alone (the caller's colon check already fails it closed), and
// does not perform a lookup on a bogus split.
//
// VALIDATES: L119 -- the raw guard defers malformed refs to the existing check.
// PREVENTS: double-handling / a lookup with an empty filter name.
func TestDefaultOriginateRawGuardIgnoresMalformedRef(t *testing.T) {
	info := &fakeFilterRawInfo{raw: true}
	rejected := defaultOriginateRejectsRawFilter(info, "no-colon", "192.0.2.1")
	assert.False(t, rejected, "malformed ref must be left to the caller's colon check")
	assert.Empty(t, info.gotPlugin, "no lookup should happen on a malformed ref")
}
