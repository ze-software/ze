// Design: docs/architecture/wire/attributes.md -- unknown transitive pass-along flags.
// Related: docs/architecture/plugin/rib-storage-design.md -- retained selected attributes.
// Related: partial_session_wire_test.go -- useful route and recognized attribute controls.
// Related: rfc4271_selected_boundaries_test.go -- ordinary real Session fixture.

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestUnknownTransitivePartialSurvivesOrdinarySessionWire sends an unrecognized
// transitive attribute over an eBGP Session and inspects stored and iBGP output
// bytes, initially and after a fresh destination Session reconnects without a
// new source UPDATE. Short and extended values arrive with Partial clear or set;
// all must leave with Partial set. Four recognized C0 attributes are software
// controls, independent of the unknown attribute's required outgoing Partial.
//
// RFC 4271 Section 5: "If a path with an unrecognized transitive optional
// attribute is accepted and passed to other BGP peers, then the unrecognized
// transitive optional attribute of that path MUST be passed, along with the path,
// to other BGP peers with the Partial bit in the Attribute Flags octet set to 1."
// This native Session boundary does not claim foreign acceptance or RS/RR paths.
// RFC requirement: RFC4271-5-3 positive -- accepted unrecognized transitive attribute 250 leaves ordinary eBGP-to-iBGP ASN4 Session wire with Partial set and its exact short or 300-octet value, for received Partial clear or set, including destination reconnect without a new source UPDATE.
func TestUnknownTransitivePartialSurvivesOrdinarySessionWire(t *testing.T) {
	require.False(t, attribute.AttributeCode(250).Recognized(), "the test code is unrecognized")
	long := make([]byte, 300)
	for i := range long {
		long[i] = byte(i % 256)
	}
	for _, tc := range []unknownPartialSessionWireCase{
		{"short_partial_clear", 0xc0, []byte{1, 2, 3}, netip.MustParsePrefix("198.19.231.0/24")},
		{"short_partial_set", 0xe0, []byte{1, 2, 3}, netip.MustParsePrefix("198.19.232.0/24")},
		{"extended_partial_clear", 0xc0, long, netip.MustParsePrefix("198.19.233.0/24")},
		{"extended_partial_set", 0xe0, long, netip.MustParsePrefix("198.19.234.0/24")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unknownPartialSessionWireCaseRun(t, tc)
		})
	}
}

// unknownPartialSessionWireCase holds immutable input for one isolated pair of
// real Sessions. Values remain live and unmodified through both output captures.
type unknownPartialSessionWireCase struct {
	name   string
	flags  attribute.AttributeFlags
	value  []byte
	prefix netip.Prefix
}

// unknownPartialSessionWireCaseRun follows ordinary ingress, selection and replay
// without supplying RIB state, source authority or a selected-route callback.
func unknownPartialSessionWireCaseRun(t *testing.T, tc unknownPartialSessionWireCase) {
	t.Helper()
	peers := ordinaryBoundaryPeers(t,
		acceptingLiveSettings(t, "192.0.2.225", 65000, 65010),
		acceptingLiveSettings(t, "192.0.2.226", 65000, 65000))
	source, destination := peers[0], peers[1]
	require.False(t, source.peer.IsIBGP())
	require.True(t, destination.peer.IsIBGP())
	for _, peer := range peers {
		negotiated := peer.peer.negotiated.Load()
		require.NotNil(t, negotiated)
		require.True(t, negotiated.ASN4, "both real sessions negotiate four-octet ASNs")
	}
	sourceSession := source.peer.currentSession()
	require.NotNil(t, sourceSession)
	attrs := synthesizedNextHopAttrs(netip.MustParseAddr("198.18.249.1"), 65010)
	for _, target := range partialSessionWireTargets() {
		attrs = append(attrs, makeAttr(0xc0, byte(target.code), target.value)...)
	}
	unknown := attribute.NewOpaqueAttribute(tc.flags, 250, tc.value)
	raw := make([]byte, attribute.AttrWireLen(unknown))
	// RFC 4271 Section 4.3: the existing writer selects the header width for
	// the bounded short or 300-octet value; check the actual input before send.
	require.Equal(t, len(raw), attribute.WriteAttrTo(unknown, raw, 0))
	requireUnknownPartialSessionWireAttribute(t, raw, tc.value, byte(tc.flags))
	attrs = append(attrs, raw...)
	// RFC 4271 Section 5: actual source socket ingress must establish both the
	// received unknown attribute and the four independent recognized controls.
	ownershipLiveSend(t, source, nil, attrs, tc.prefix)
	ordinarySelectedStored(t, source, tc.prefix)
	ordinarySelectedBest(t, tc.prefix, "198.18.249.1")
	ordinaryBoundaryDrain(t, source)
	address := tc.prefix.Addr().As4()
	stored := extendedRecoveryStoredAttributes(source.peer.Settings().Address.String(), address[:3])
	require.NotEmpty(t, stored, "the actual RIB retains the valid source candidate")
	requirePartialSessionWireTargets(t, stored, 0xc0)
	requireUnknownPartialSessionWireAttribute(t, stored, tc.value, 0xe0)
	ordinarySelectedWire(t, destination, tc.prefix, "198.18.249.1")
	ordinaryBoundaryCapture(t, destination)
	requireUnknownPartialSessionWireRoute(t, ownershipLiveRead(t, destination), tc)

	updates := source.peer.Stats().UpdatesReceived
	require.Positive(t, updates)
	previousSession := destination.peer.currentSession()
	require.NotNil(t, previousSession)
	identifier := destination.peer.RemoteRouterID()
	lowLiveDepart(t, destination)
	select {
	case <-destination.stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the departed destination capture did not stop")
	}
	// RFC 4271 Section 5: replay must pass the current stored path to a fresh
	// destination Session while the source remains up and sends no new UPDATE.
	reconnected := lowLiveReconnect(t, destination, identifier)
	replaySession := reconnected.peer.currentSession()
	require.NotNil(t, replaySession)
	require.NotSame(t, previousSession, replaySession)
	require.Equal(t, identifier, reconnected.peer.RemoteRouterID())
	ordinarySelectedWire(t, reconnected, tc.prefix, "198.18.249.1")
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, reconnected)
	requireUnknownPartialSessionWireRoute(t, ownershipLiveRead(t, reconnected), tc)
	require.Same(t, sourceSession, source.peer.currentSession())
	require.Equal(t, updates, source.peer.Stats().UpdatesReceived,
		"destination replay must not depend on a new source UPDATE")
	stored = extendedRecoveryStoredAttributes(source.peer.Settings().Address.String(), address[:3])
	require.NotEmpty(t, stored)
	requirePartialSessionWireTargets(t, stored, 0xc0)
	requireUnknownPartialSessionWireAttribute(t, stored, tc.value, 0xe0)
	ownershipLiveEstablished(t, []*lowLivePeer{source, reconnected})
}

// requireUnknownPartialSessionWireRoute retains the published useful NLRI,
// AS_PATH, next-hop, default LOCAL_PREF and four C0 controls on every observed
// announcement before checking that the unknown attribute also reaches the peer.
func requireUnknownPartialSessionWireRoute(t *testing.T, view ownershipLiveProjection, tc unknownPartialSessionWireCase) {
	t.Helper()
	requirePartialSessionWireRoute(t, view, tc.prefix, 0xc0)
	requireUnknownPartialSessionWireAttribute(t, view.routes[tc.prefix], tc.value, 0xe0)
	for _, attrs := range view.announcements[tc.prefix] {
		requireUnknownPartialSessionWireAttribute(t, attrs, tc.value, 0xe0)
	}
}

// requireUnknownPartialSessionWireAttribute checks exact useful bytes before
// flags. Extended Length is independently legal for short values and required
// for the 300-octet value; every other flag bit must match the expected class.
func requireUnknownPartialSessionWireAttribute(t *testing.T, attrs, value []byte, flags byte) {
	t.Helper()
	_, received, actual, found := attribute.AttrFind(attrs, 250)
	require.True(t, found, "unrecognized transitive attribute 250 survives")
	require.Equal(t, value, actual, "unrecognized attribute retains its exact value")
	if len(value) > 255 {
		require.NotZero(t, received&attribute.FlagExtLength, "300-octet value has extended length")
	}
	require.Equal(t, flags, byte(received)&^byte(attribute.FlagExtLength),
		"unrecognized attribute 250 has the required flags apart from header width")
}
