// RFC: rfc/short/rfc9494.md -- Long-Lived Graceful Restart requirement bindings.
//
// These tests bind RFC 9494 MUST-level requirements to the producing functions in
// this package: capability declaration (gr_llgr.go), capability decode (gr_llgr.go)
// and the LLST state machine (gr_state.go).

package gr

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// configWithLLST is a peer config carrying both restart-time and long-lived-stale-time.
const configWithLLST = `{"bgp":{"peer":{"192.168.1.1":{"session":{"capability":{"graceful-restart":{"restart-time":120,"long-lived-stale-time":3600}}}}}}}`

// configGROnly is a peer config carrying graceful-restart without long-lived-stale-time.
const configGROnly = `{"bgp":{"peer":{"192.168.1.1":{"session":{"capability":{"graceful-restart":{"restart-time":120}}}}}}}`

// TestRFC9494_LLGRCapDeclaredWithGRCap verifies the LLGR capability is only ever
// declared together with the Graceful Restart capability.
//
// VALIDATES: a config that enables LLGR also declares code 64.
// PREVENTS: advertising code 71 alone, which a peer must disregard.
//
// RFC requirement: RFC9494-3.1-1 positive -- LLGR (code 71) is derived from the same
// "graceful-restart" capability container that produces GR (code 64): parseLLGRCapValue
// (internal/component/bgp/plugins/gr/gr_llgr.go:124) reads grData from capMap["graceful-restart"]
// and parseGRCapValue (internal/component/bgp/plugins/gr/gr.go:643) reads the same key, so a
// peer declared with code 71 is always also declared with code 64.
func TestRFC9494_LLGRCapDeclaredWithGRCap(t *testing.T) {
	t.Parallel()

	grCaps := extractGRCapabilities(configWithLLST)
	llgrCaps := extractLLGRCapabilities(configWithLLST)

	require.Len(t, llgrCaps, 1, "LLGR capability declared")
	assert.Equal(t, uint8(71), llgrCaps[0].Code)
	require.Len(t, grCaps, 1, "GR capability declared alongside LLGR")
	assert.Equal(t, uint8(64), grCaps[0].Code)
	assert.Equal(t, llgrCaps[0].Peers, grCaps[0].Peers, "both capabilities target the same peer")
}

// TestRFC9494_NoLLGRCapWithoutGRContainer verifies code 71 is not emitted when the
// graceful-restart container (the source of code 64) is absent.
//
// VALIDATES: a long-lived-stale-time outside the graceful-restart container declares nothing.
// PREVENTS: an LLGR-only declaration reaching the OPEN message.
//
// RFC requirement: RFC9494-3.1-1 negative -- with long-lived-stale-time placed outside the
// "graceful-restart" container, parseLLGRCapValue returns "" at its grData type assertion
// (internal/component/bgp/plugins/gr/gr_llgr.go:124-127), so neither code 71 nor code 64 is
// declared: there is no configuration shape that yields LLGR without GR.
func TestRFC9494_NoLLGRCapWithoutGRContainer(t *testing.T) {
	t.Parallel()

	jsonStr := `{"bgp":{"peer":{"192.168.1.1":{"session":{"capability":{"long-lived-stale-time":3600}}}}}}`

	assert.Empty(t, extractLLGRCapabilities(jsonStr), "no LLGR capability without the graceful-restart container")
	assert.Empty(t, extractGRCapabilities(jsonStr), "no GR capability either")
}

// TestRFC9494_FlagsReservedBitsZeroOnSend verifies the encoder writes only the F bit.
//
// VALIDATES: the per-family Flags octet is exactly 0x80.
// PREVENTS: garbage in the reserved bits of an advertised LLGR capability.
//
// RFC requirement: RFC9494-3.1-2 positive -- parseLLGRCapValue writes the literal 0x80 as the
// per-family Flags octet (internal/component/bgp/plugins/gr/gr_llgr.go:168), so bits 1-7 are
// zero on send.
func TestRFC9494_FlagsReservedBitsZeroOnSend(t *testing.T) {
	t.Parallel()

	caps := extractLLGRCapabilities(configWithLLST)
	require.Len(t, caps, 1)

	payload, err := hex.DecodeString(caps[0].Payload)
	require.NoError(t, err)
	require.Len(t, payload, 7, "one AFI/SAFI tuple")
	assert.Equal(t, byte(0x80), payload[3], "Flags octet carries the F bit and zero reserved bits")
}

// TestRFC9494_FlagsReservedBitsIgnoredOnReceive verifies reserved bits do not change decoding.
//
// VALIDATES: a Flags octet with every reserved bit set decodes exactly like 0x80.
// PREVENTS: rejecting or mis-reading a capability from a sender that sets reserved bits.
//
// RFC requirement: RFC9494-3.1-2 negative -- decodeLLGR masks the Flags octet with 0x80 and reads
// nothing else from it (internal/component/bgp/plugins/gr/gr_llgr.go:79), so a tuple with all
// reserved bits set yields the same ForwardState and LLST as the canonical encoding and is not
// treated as an error.
func TestRFC9494_FlagsReservedBitsIgnoredOnReceive(t *testing.T) {
	t.Parallel()

	canonical, err := decodeLLGR([]byte{0x00, 0x01, 0x01, 0x80, 0x00, 0x0E, 0x10})
	require.NoError(t, err)

	reservedSet, err := decodeLLGR([]byte{0x00, 0x01, 0x01, 0xFF, 0x00, 0x0E, 0x10})
	require.NoError(t, err, "reserved bits must not make the capability malformed")
	assert.Equal(t, canonical.Families, reservedSet.Families, "reserved bits are ignored on receive")

	// A clear F bit with reserved bits set still reads as F=0.
	fClear, err := decodeLLGR([]byte{0x00, 0x01, 0x01, 0x7F, 0x00, 0x0E, 0x10})
	require.NoError(t, err)
	require.Len(t, fClear.Families, 1)
	assert.False(t, fClear.Families[0].ForwardState, "reserved bits do not leak into the F bit")
	assert.Equal(t, uint32(3600), fClear.Families[0].LLST)
}

// TestRFC9494_LLGRNotEnabledByDefault verifies LLGR stays off until it is configured.
//
// VALIDATES: graceful-restart without long-lived-stale-time declares no LLGR capability,
// and the YANG leaf carries no default value.
// PREVENTS: LLGR turning itself on for every GR-enabled peer.
//
// RFC requirement: RFC9494-5-1 positive -- LLGR is off unless configured: parseLLGRCapValue
// returns "" when the long-lived-stale-time key is absent
// (internal/component/bgp/plugins/gr/gr_llgr.go:130-133), and the ze-graceful-restart YANG
// leaf long-lived-stale-time declares no default (unlike restart-time, which defaults to 120),
// so nothing supplies a value on the operator's behalf.
func TestRFC9494_LLGRNotEnabledByDefault(t *testing.T) {
	t.Parallel()

	assert.Empty(t, extractLLGRCapabilities(configGROnly), "no LLGR capability without explicit configuration")
	assert.Len(t, extractGRCapabilities(configGROnly), 1, "GR itself is configured")

	model := GetYANG()
	idx := strings.Index(model, "leaf long-lived-stale-time")
	require.Positive(t, idx, "YANG model declares the long-lived-stale-time leaf")
	end := strings.Index(model[idx:], "\n        }")
	require.Positive(t, end, "leaf block is delimited")
	leafBlock := model[idx : idx+end]
	assert.NotContains(t, leafBlock, "default", "long-lived-stale-time has no default value")
}

// TestRFC9494_LLGREnabledByExplicitConfig verifies the affirmative configuration works.
//
// VALIDATES: an explicit long-lived-stale-time declares code 71 with that LLST on the wire.
// PREVENTS: a broken extractor being mistaken for "off by default".
//
// RFC requirement: RFC9494-5-1 negative -- the contrast to the default state: with an explicit
// long-lived-stale-time, parseLLGRCapValue builds the 7-byte tuple and extractLLGRCapabilities
// emits the code-71 declaration (internal/component/bgp/plugins/gr/gr_llgr.go:158-177, :209-214),
// proving the absent declaration above comes from the missing configuration, not from a
// capability that can never be produced.
func TestRFC9494_LLGREnabledByExplicitConfig(t *testing.T) {
	t.Parallel()

	caps := extractLLGRCapabilities(configWithLLST)
	require.Len(t, caps, 1)

	payload, err := hex.DecodeString(caps[0].Payload)
	require.NoError(t, err)
	require.Len(t, payload, 7)
	llst := uint32(payload[4])<<16 | uint32(payload[5])<<8 | uint32(payload[6])
	assert.Equal(t, uint32(3600), llst, "configured LLST reaches the wire")
}

// TestRFC9494_NoLLSTExpiryAfterReestablish verifies a re-established session cancels the
// pending LLST deletion.
//
// VALIDATES: reconnecting inside the LLST window stops the per-family timer.
// PREVENTS: stale routes being deleted after the peer is back and resynchronizing.
//
// RFC requirement: RFC9494-4.2-3 negative -- the deletion is conditional on the session NOT
// being re-established: onSessionReestablished stops every LLST timer through
// stopLLSTTimersLocked (internal/component/bgp/plugins/gr/gr_state.go:194, :476-481), so
// handleLLSTExpired never runs and no family purge is requested.
func TestRFC9494_NoLLSTExpiryAfterReestablish(t *testing.T) {
	t.Parallel()

	familyExpired := &safeCollector{}
	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(peer string, fam family.Family, llst uint32) {}
	mgr.onLLGRFamilyExpired = func(peer string, fam family.Family) {
		familyExpired.add(fam.String())
	}

	llgrCap := &llgrPeerCap{
		Families: []llgrCapFamily{
			{Family: family.IPv4Unicast, ForwardState: true, LLST: 1},
		},
	}
	// restart-time=0 enters LLGR immediately, so the 1s LLST timer is armed now.
	mgr.onSessionDown(testPeer, testCap(0, famIPv4), llgrCap, false)
	require.True(t, mgr.peerActive(testPeer), "peer is in LLGR")

	// Session comes back before the LLST elapses.
	purged, wasInLLGR := mgr.onSessionReestablished(testPeer, testCap(120, famIPv4), llgrCap)
	assert.True(t, wasInLLGR, "reconnect happened during LLGR")
	assert.Empty(t, purged, "F bit set in both capabilities, nothing purged on reconnect")

	// Well past the original LLST: no family deletion may have fired.
	time.Sleep(1500 * time.Millisecond)
	assert.Empty(t, familyExpired.get(), "a re-established session cancels the LLST deletion")
}
