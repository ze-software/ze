// Design: docs/architecture/core-design.md — the RIB plugin answers a route refresh
// RFC: rfc/short/rfc7313.md — Enhanced Route Refresh, Section 4
// Related: rfc7313_test.go — the marker-only view of the same producers
//
// RFC 7313 Section 4: "Before the speaker starts a route refresh that is either initiated
// locally, or in response to a "normal route refresh request" from the peer, the speaker
// MUST send a BoRR message. After the speaker completes the re-advertisement of the entire
// Adj-RIB-Out to the peer, it MUST send an EoRR message."
//
// The markers leave the RIB on the dispatch rail (dispatchPeerAction) and the routes on the
// update rail (updateRoute), so a test holding one hook sees half the sequence. These tests
// hold both hooks and write into ONE ordered log, so the position of each marker against
// the re-advertised routes is what they assert.

package rib

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	updatecmd "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// consumeRefreshCommand uses the production command consumers, not a second
// interpretation of text/hex wording. Socket delivery is separately exercised
// by plugin-refresh.ci and refresh-config-static.ci.
func consumeRefreshCommand(t *testing.T, command string) []bgptypes.NLRIGroup {
	t.Helper()
	groups := parseRefreshCommand(t, command)
	for _, group := range groups {
		require.NotNil(t, group.Wire)
	}
	return groups
}

// parseRefreshCommand accepts the real text and wire consumers without requiring
// one attribute representation. Callers assert the route behavior they need.
func parseRefreshCommand(t *testing.T, command string) []bgptypes.NLRIGroup {
	t.Helper()
	args := strings.Fields(command)
	require.GreaterOrEqual(t, len(args), 3)
	require.Equal(t, "update", args[0])
	var result *bgptypes.UpdateTextResult
	var err error
	switch args[1] {
	case "hex":
		result, err = updatecmd.ParseUpdateWire(args[2:], plugin.WireEncodingHex)
	case "b64":
		result, err = updatecmd.ParseUpdateWire(args[2:], plugin.WireEncodingB64)
	case "text":
		result, err = updatecmd.ParseUpdateText(args[2:])
	default:
		t.Fatalf("unsupported refresh command encoding %q", args[1])
	}
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result.EORFamilies, "refresh routes must not contain an End-of-RIB")
	require.NotEmpty(t, result.Groups)
	for _, group := range result.Groups {
		require.Empty(t, group.Withdraw, "refresh must advertise the retained route")
		require.NotEmpty(t, group.Announce)
	}
	return result.Groups
}

type refreshRouteIdentity struct {
	family family.Family
	prefix netip.Prefix
}

func consumedRefreshRoutes(t *testing.T, groups []bgptypes.NLRIGroup) []refreshRouteIdentity {
	t.Helper()
	var routes []refreshRouteIdentity
	for _, group := range groups {
		for _, route := range group.Announce {
			require.Equal(t, group.Family, route.Family())
			payload := make([]byte, route.Len())
			require.Equal(t, len(payload), route.WriteTo(payload, 0))
			prefix, ok := nlri.WirePrefixToKey(payload, group.Family)
			require.True(t, ok, "the consumer must receive a valid IP route")
			routes = append(routes, refreshRouteIdentity{family: group.Family, prefix: prefix.Masked()})
		}
	}
	return routes
}

// rfc7313SequencedRIB returns rfc7313ReadyRIB's manager, holding two IPv4 unicast routes
// and one IPv6 unicast route in the peer's Adj-RIB-Out, with markers and routes captured
// into one log in the order the RIB emitted them.
func rfc7313SequencedRIB(t *testing.T) (*RIBManager, *[]string) {
	t.Helper()
	r, _ := rfc7313ReadyRIB(t)
	addr := netip.MustParseAddr(rfc7313RefreshPeer)
	r.ribOut[addr][family.IPv6Unicast] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv6Unicast: {
			"2001:db8::/32": {MsgID: 3, Family: family.IPv6Unicast, Prefix: "2001:db8::/32", NextHop: "::1"},
		},
	})[family.IPv6Unicast]

	sequence := &[]string{}
	r.dispatchHook = func(cmd string) { *sequence = append(*sequence, "marker "+cmd) }
	r.updateHook = func(cmd string, _ map[string]any) { *sequence = append(*sequence, "route "+cmd) }
	return r, sequence
}

// requireBracketedReadvertisement asserts the whole RFC 7313 Section 4 sequence for the
// IPv4 unicast refresh: the BoRR first, then every IPv4 unicast route of the Adj-RIB-Out
// and nothing else, then the EoRR last.
func requireBracketedReadvertisement(t *testing.T, sequence []string) {
	t.Helper()
	require.GreaterOrEqual(t, len(sequence), 3, "BoRR, routes, EoRR: %q", sequence)

	borr := "marker request peer " + rfc7313RefreshPeer + " borr " + family.IPv4Unicast.String()
	eorr := "marker request peer " + rfc7313RefreshPeer + " eorr " + family.IPv4Unicast.String()
	require.Equal(t, borr, sequence[0], "the BoRR must precede the first re-advertised route")
	require.Equal(t, eorr, sequence[len(sequence)-1], "the EoRR must follow the last re-advertised route")

	var routes []refreshRouteIdentity
	for _, item := range sequence[1 : len(sequence)-1] {
		command, ok := strings.CutPrefix(item, "route ")
		require.True(t, ok, "only route delivery belongs between the markers: %q", item)
		routes = append(routes, consumedRefreshRoutes(t, consumeRefreshCommand(t, command))...)
	}
	require.ElementsMatch(t, []refreshRouteIdentity{
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")},
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.1.0/24")},
	}, routes, "the consumer receives the entire requested Adj-RIB-Out, and no other family")
}

// TestRFC7313BoRRRoutesEoRRInOneSequence drives both refresh producers and reads one
// ordered log of what each emitted.
//
// VALIDATES: RIBManager.handleRefresh (the JSON rail) and handleRefreshStructured (the
// DirectBridge rail), answering a normal route refresh request (subtype 0) from the peer,
// emit the BoRR before the first route, then both IPv4 unicast routes of the Adj-RIB-Out,
// then the EoRR after the last route.
// PREVENTS: a producer that re-advertises before its BoRR, or sends its EoRR before the
// re-advertisement completes: the marker-only unit (rfc7313_test.go) stays green on both,
// because the routes never enter its log.
//
// RFC requirement: RFC7313-4-1 positive -- in response to a normal route refresh request,
// the BoRR is sent before the first route of the re-advertisement, on both rails
// (rib.go handleRefresh, rib_structured.go handleRefreshStructured).
// RFC requirement: RFC7313-4-2 positive -- the EoRR is sent after the last route of the
// re-advertisement, which carries every route of the family's Adj-RIB-Out, on both rails.
func TestRFC7313BoRRRoutesEoRRInOneSequence(t *testing.T) {
	t.Run("handleRefresh", func(t *testing.T) {
		r, sequence := rfc7313SequencedRIB(t)

		r.handleRefresh(&Event{
			Message: &MessageInfo{Type: rpc.EventKindRefresh},
			Peer: mustMarshal(t, map[string]any{
				"local":  map[string]any{"address": "10.0.0.2", "as": uint32(65002)},
				"remote": map[string]any{"address": rfc7313RefreshPeer, "as": uint32(65001)},
			}),
			AFI:  family.AFIIPv4,
			SAFI: family.SAFIUnicast,
		})

		requireBracketedReadvertisement(t, *sequence)
	})

	t.Run("handleRefreshStructured", func(t *testing.T) {
		r, sequence := rfc7313SequencedRIB(t)

		// Route refresh wire: AFI (2 octets), message subtype (1), SAFI (1). Subtype 0.
		r.handleRefreshStructured(&rpc.StructuredEvent{
			PeerAddress: rfc7313RefreshPeer,
			RawMessage: &bgptypes.RawMessage{
				RawBytes: []byte{0x00, byte(family.AFIIPv4), 0x00, byte(family.SAFIUnicast)},
			},
		})

		requireBracketedReadvertisement(t, *sequence)
	})
}
