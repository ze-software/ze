// Design: docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md -- RFC 4090
// head-end request flags on the built PATH, and the RRO flags a PLR records,
// read from the RESV the engine sends rather than from rroProtectionFlags.
//
// VALIDATES: RFC 4090 Sections 4.4 and 5 at buildPath, handlePacket,
// handleLinkDown and sendResv.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requestedPath decodes the PATH buildPath encodes for a protected head-end
// PSB carrying the request pr.
func requestedPath(t *testing.T, pr *protectionRequest) *ParsedMessage {
	t.Helper()
	psb := protectedPSB()
	psb.Protection = pr
	msg, err := DecodeMessage(buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64))
	require.NoError(t, err)
	return msg
}

// RFC requirement: RFC4090-4.1-2 positive -- a head-end that desires one-to-one backup includes a FAST_REROUTE object in the PATH it builds, with the "one-to-one backup desired" flag (0x01) set.
func TestRFC4090OneToOneRequestInPath(t *testing.T) {
	msg := requestedPath(t, &protectionRequest{HopLimit: 16, SetupPrio: 7, HoldPrio: 7})
	require.True(t, msg.HasFastReroute, "PATH carries FAST_REROUTE")
	assert.Equal(t, uint8(0x01), msg.FastReroute.Flags&0x01, "one-to-one backup desired")
}

// RFC requirement: RFC4090-4.1-2 negative -- the FAST_REROUTE flags never name the method the head-end did not desire: a one-to-one request leaves "facility backup desired" (0x02) clear, and a facility request leaves "one-to-one backup desired" (0x01) clear.
func TestRFC4090RequestNeverNamesOtherMethod(t *testing.T) {
	oneToOne := requestedPath(t, &protectionRequest{HopLimit: 16, SetupPrio: 7, HoldPrio: 7})
	require.True(t, oneToOne.HasFastReroute)
	assert.Zero(t, oneToOne.FastReroute.Flags&0x02, "one-to-one request: facility flag clear")

	facility := requestedPath(t, &protectionRequest{Facility: true, HopLimit: 16, SetupPrio: 7, HoldPrio: 7})
	require.True(t, facility.HasFastReroute)
	assert.Equal(t, uint8(0x02), facility.FastReroute.Flags&0x02, "facility request: facility flag set")
	assert.Zero(t, facility.FastReroute.Flags&0x01, "facility request: one-to-one flag clear")
}

// RFC requirement: RFC4090-4.3-2 negative -- a head-end that does not desire node protection clears the "node protection desired" flag (0x10) in the SESSION_ATTRIBUTE of the PATH it builds, while still asking for local protection (0x01).
func TestRFC4090NodeDesiredClearedWhenNotDesired(t *testing.T) {
	msg := requestedPath(t, &protectionRequest{Facility: true, HopLimit: 16, SetupPrio: 7, HoldPrio: 7})
	require.True(t, msg.HasSessionAttr)
	assert.Equal(t, uint8(0x01), msg.SessionAttr.Flags&0x01, "local protection desired")
	assert.Zero(t, msg.SessionAttr.Flags&0x10, "node protection desired is clear")
}

// RFC requirement: RFC4090-4.4-7 negative -- a PLR that provides only link protection (a link request served by the bypass merging at the next hop) sets "local protection available" (0x01) and leaves "node protection" (0x08) clear in its RRO subobject.
func TestRFC4090LinkProtectionLeavesNodeBitClear(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))

	flags := relayedRROFlags(t, e, ft, psb, netip.MustParseAddr("10.0.0.3"), nil)
	assert.Equal(t, uint8(0x01), flags&0x01, "local protection available")
	assert.Zero(t, flags&0x08, "node protection clear: no node protection is provided")
}

// RFC requirement: RFC4090-4.4-2 positive -- during fast reroute the PLR updates its own RRO subobject in the RESV it builds from the stored reservation: after handleLinkDown repairs the LSP onto the bypass, the refreshed RESV carries both "local protection available" (0x01) and "local protection in use" (0x02).
func TestRFC4090RepairSetsInUseAndAvailable(t *testing.T) {
	e, ft, _ := plrEngine(t)
	ingress := netip.MustParseAddr("10.0.0.1")
	mp := netip.MustParseAddr("10.0.0.3")
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})
	bringBypassUp(t, e, 5000, netip.MustParseAddr("10.0.1.3"))
	before := relayedRROFlags(t, e, ft, psb, mp, nil)
	require.Zero(t, before&0x02, "fixture: no local repair yet")

	e.handleLinkDown("eth0")
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok, "the repaired LSP is kept")
	require.NoError(t, e.sendResv(lsp))

	rro := relayedResvRRO(t, ft)
	require.Equal(t, e.cfg().RouterID, rro[0].Address, "the PLR's own subobject")
	assert.Equal(t, uint8(0x03), rro[0].Flags&0x03, "available and in use")
}
