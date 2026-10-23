// Design: docs/architecture/core-design.md — the RIB plugin answers a route refresh
// RFC: rfc/short/rfc7313.md — Enhanced Route Refresh, Section 4
// Related: rfc7313_sequence_test.go — the peer-initiated refresh, same helpers
//
// RFC 7313 Section 4: "Before the speaker starts a route refresh that is either initiated
// locally, or in response to a "normal route refresh request" from the peer, the speaker
// MUST send a BoRR message."
//
// `clear bgp rib out` (outboundResend) is the locally initiated refresh. These tests read
// the one ordered log of markers and routes it emits.

package rib

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// localResendFamilies splits an ordered marker and route log into one bracket per
// family, failing on any route outside a BoRR/EoRR pair or a pair that does not match.
// It answers the routes each bracket carried, keyed by the bracket's family.
func localResendFamilies(t *testing.T, sequence []string) map[string][]refreshRouteIdentity {
	t.Helper()
	borrPrefix := "marker request peer " + rfc7313RefreshPeer + " borr "
	eorrPrefix := "marker request peer " + rfc7313RefreshPeer + " eorr "
	brackets := make(map[string][]refreshRouteIdentity)
	open := ""
	for _, item := range sequence {
		if fam, ok := strings.CutPrefix(item, borrPrefix); ok {
			require.Empty(t, open, "a BoRR for %s opened inside the %s bracket: %q", fam, open, sequence)
			open = fam
			brackets[fam] = nil
			continue
		}
		if fam, ok := strings.CutPrefix(item, eorrPrefix); ok {
			require.Equal(t, open, fam, "the EoRR must close the family its BoRR opened: %q", sequence)
			open = ""
			continue
		}
		command, ok := strings.CutPrefix(item, "route ")
		require.True(t, ok, "unexpected item %q", item)
		if command == "update cursor done" {
			continue // The cursor reset around each resend carries no route.
		}
		// The first group of each resend carries full attributes, so the text
		// parser reads it; a delta it cannot read fails here, never silently.
		if rest, isCursor := strings.CutPrefix(command, "update cursor "); isCursor {
			command = "update text " + rest
		}
		require.NotEmpty(t, open, "a route left before its family's BoRR or after its EoRR: %q", sequence)
		brackets[open] = append(brackets[open], consumedRefreshRoutes(t, consumeRefreshCommand(t, command))...)
	}
	require.Empty(t, open, "the %s bracket was never closed by an EoRR: %q", open, sequence)
	return brackets
}

// TestLocalResendBracketsEachFamilyWithBoRRAndEoRR proves `clear bgp rib out` with no
// family sends, for every family of the Adj-RIB-Out, a BoRR, that family's routes and
// nothing else, then an EoRR.
//
// VALIDATES: outboundResend dispatches "request peer <addr> borr <family>" before and
// "request peer <addr> eorr <family>" after each family's resend. The engine sends the
// markers only to a peer that advertised Enhanced Route Refresh (sendRouteRefresh).
// PREVENTS: a locally initiated refresh leaving the peer no BoRR to mark stale against
// and no EoRR to purge what the Adj-RIB-Out no longer holds.
func TestLocalResendBracketsEachFamilyWithBoRRAndEoRR(t *testing.T) {
	r, sequence := rfc7313SequencedRIB(t)

	r.outboundResend(rfc7313RefreshPeer, "")

	brackets := localResendFamilies(t, *sequence)
	require.Len(t, brackets, 2, "one bracket per family of the Adj-RIB-Out: %q", *sequence)
	require.ElementsMatch(t, []refreshRouteIdentity{
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")},
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.1.0/24")},
	}, brackets[family.IPv4Unicast.String()])
	require.Equal(t, []refreshRouteIdentity{
		{family: family.IPv6Unicast, prefix: netip.MustParsePrefix("2001:db8::/32")},
	}, brackets[family.IPv6Unicast.String()])
}

// TestLocalResendOfOneFamilyBracketsOnlyThatFamily proves `clear bgp rib out <sel> family
// ipv4/unicast` sends the IPv4 unicast BoRR first, its routes, and the EoRR last, and
// nothing of the IPv6 unicast family the same Adj-RIB-Out holds.
func TestLocalResendOfOneFamilyBracketsOnlyThatFamily(t *testing.T) {
	r, sequence := rfc7313SequencedRIB(t)

	r.outboundResend(rfc7313RefreshPeer, family.IPv4Unicast.String())

	brackets := localResendFamilies(t, *sequence)
	require.Len(t, brackets, 1, "only the requested family is bracketed: %q", *sequence)
	require.ElementsMatch(t, []refreshRouteIdentity{
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")},
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.1.0/24")},
	}, brackets[family.IPv4Unicast.String()])
}

// TestLocalResendCarriesConfigRoutes proves `clear bgp rib out` resends a route the
// configuration announced. Peer-up replay skips a config-static entry because
// sendInitialRoutes owns it there, and a flush must not inherit that skip: the
// configured route is part of the entire Adj-RIB-Out the EoRR closes over.
func TestLocalResendCarriesConfigRoutes(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr(rfc7313RefreshPeer)
	r.peerUp[peer] = true
	body := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1, 24, 198, 51, 100}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)
	r.handleSentStructured(&rpc.StructuredEvent{
		PeerAddress: peer.String(),
		Meta:        map[string]any{"config-static": true},
		RawMessage:  &bgptypes.RawMessage{WireUpdate: wu, AttrsWire: attrs},
	})
	sequence := &[]string{}
	r.dispatchHook = func(cmd string) { *sequence = append(*sequence, "marker "+cmd) }
	r.updateHook = func(cmd string, _ map[string]any) { *sequence = append(*sequence, "route "+cmd) }

	r.outboundResend(rfc7313RefreshPeer, "")

	brackets := localResendFamilies(t, *sequence)
	require.Equal(t, []refreshRouteIdentity{
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("198.51.100.0/24")},
	}, brackets[family.IPv4Unicast.String()], "the configured route is resent inside the IPv4 bracket")
}

// TestLocalResendAfterAWithdrawalSendsNothing proves a route the engine withdrew
// leaves bgp-rib's Adj-RIB-Out copy, so `clear bgp rib out` does not bring it
// back. `clear bgp rib advertised` withdraws on the local withdraw rail, and the
// sent event of that withdrawal is what bgp-rib reads here.
func TestLocalResendAfterAWithdrawalSendsNothing(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr(rfc7313RefreshPeer)
	r.peerUp[peer] = true
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	announce := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1, 24, 198, 51, 100}
	wu := wireu.NewWireUpdate(announce, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)
	r.handleSentStructured(&rpc.StructuredEvent{
		PeerAddress: peer.String(),
		Meta:        map[string]any{"config-static": true},
		RawMessage:  &bgptypes.RawMessage{WireUpdate: wu, AttrsWire: attrs},
	})
	require.Len(t, r.collectGroupedRibOutRoutes(peer), 1, "the fixture holds the configured route")

	// RFC 4271 Section 4.3: withdrawn routes length 4, the route 198.51.100.0/24,
	// path attribute length 0.
	withdraw := wireu.NewWireUpdate([]byte{0, 4, 24, 198, 51, 100, 0, 0}, ctxID)
	r.handleSentStructured(&rpc.StructuredEvent{
		PeerAddress: peer.String(),
		RawMessage:  &bgptypes.RawMessage{WireUpdate: withdraw},
	})
	sequence := &[]string{}
	r.dispatchHook = func(cmd string) { *sequence = append(*sequence, "marker "+cmd) }
	r.updateHook = func(cmd string, _ map[string]any) { *sequence = append(*sequence, "route "+cmd) }

	r.outboundResend(rfc7313RefreshPeer, "")

	assert.Empty(t, *sequence, "a withdrawn route is not resent, and an empty family gets no bracket")
}
