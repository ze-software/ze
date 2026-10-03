// Design: docs/architecture/route-selection.md -- the next hop a forwarded route leaves with
// Related: peer_forward_facts.go -- precomputeNextHop and applyFactsNextHop, the next-hop-self rewrite and its withhold guard
// Related: rfc8950_reactor_a2_forward_test.go -- a2Forward and a2Parts, the harness

package reactor

import (
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
)

// a2ThirdPartyNextHop is the NEXT_HOP a2InlinePayload carries: 192.0.2.254, an
// address that is neither the forwarding speaker's nor the destination's, so
// toward an internal peer it is a third-party next hop.
var a2ThirdPartyNextHop = []byte{192, 0, 2, 254}

// TestRFC4271NextHopSelfDisablesThirdPartyNextHopOnTheWire drives RFC 4271
// Section 5.1.3: "A BGP speaker MUST be able to support the disabling
// advertisement of third party NEXT_HOP attributes in order to handle
// imperfectly bridged media."
//
// Method: a route learned from an external peer with NEXT_HOP 192.0.2.254 is
// forwarded on the general rail to two internal destinations in one fan-out.
// One is configured `next-hop self` with local address 10.0.0.254; the other
// keeps the default, under which an internal peer is sent the received next hop.
//
// VALIDATES: the next-hop-self destination is sent NEXT_HOP 10.0.0.254, so the
// third-party address does not reach it; the default destination in the same
// fan-out is sent 192.0.2.254, which shows the route did carry a third-party
// next hop and the setting is what removed it.
// PREVENTS: next-hop self armed in the forwarding facts but never reaching the
// NEXT_HOP the destination is sent.
//
// RFC requirement: RFC4271-5.1.3-3 positive -- an internal destination configured next-hop self with a local address is sent that local address as NEXT_HOP on the general forward rail, while a default internal destination in the same fan-out is sent the received third-party NEXT_HOP.
func TestRFC4271NextHopSelfDisablesThirdPartyNextHopOnTheWire(t *testing.T) {
	self := a2Dest(t, "192.0.2.61", 65000, netip.Addr{}, false)
	self.settings.NextHopMode = NextHopSelf
	self.refreshForwardFacts()
	passing := a2Dest(t, "192.0.2.62", 65000, netip.Addr{}, false)

	got := a2Forward(t, false, a2InlinePayload(), self, passing)

	toSelf, ok := got[netip.MustParseAddr("192.0.2.61")]
	require.True(t, ok, "the next-hop-self destination is owed the route")
	require.Equal(t, a2InlinePrefix, toSelf.nlri)
	require.Equal(t, []byte{10, 0, 0, 254}, toSelf.nextHop, "next-hop self replaces the third-party NEXT_HOP")

	toPassing, ok := got[netip.MustParseAddr("192.0.2.62")]
	require.True(t, ok, "the default destination is owed the route")
	require.Equal(t, a2InlinePrefix, toPassing.nlri)
	require.Equal(t, a2ThirdPartyNextHop, toPassing.nextHop, "the default internal destination is sent the received next hop")
}

// autoLocalSelfDest builds an internal destination configured `next-hop self`
// under `connection > local > ip auto`, which leaves LocalAddress unset. A valid
// connected is stored as its session's TCP local endpoint and the session is
// returned; a zero connected leaves the peer with no session, so no local
// address exists at all, and the returned session is nil.
//
// The session is attached only while the forwarding facts are built, which is
// when buildForwardFacts reads the endpoint. It is detached afterwards so the
// route-server rail's direct write (tryDirectWriteNoFlush) does not take the
// item for a session that has no connection; the facts snapshot keeps what it
// read.
func autoLocalSelfDest(t *testing.T, addr string, connected netip.Addr) (*Peer, *Session) {
	t.Helper()
	dest := a2Dest(t, addr, 65000, netip.Addr{}, false)
	dest.settings.LocalAddress = netip.Addr{} // local ip auto
	dest.settings.NextHopMode = NextHopSelf
	var session *Session
	if connected.IsValid() {
		session = NewSession(dest.settings)
		session.transport.Store(&sessionTransport{local: connected})
	}
	dest.session = session
	dest.refreshForwardFacts()
	dest.session = nil
	return dest, session
}

// TestRFC4271NextHopSelfWithAutoLocalAddressSendsTheConnectedEndpoint drives RFC
// 4271 Section 5.1.3: "A BGP speaker MUST be able to support the disabling
// advertisement of third party NEXT_HOP attributes in order to handle
// imperfectly bridged media."
//
// Method: an internal destination is configured `next-hop self` with `local ip
// auto`, so no local address is configured, and its session's TCP local
// endpoint is 10.0.0.77. A route learned from an external peer with the
// third-party NEXT_HOP 192.0.2.254 is forwarded to it beside a default internal
// destination, on the general rail and on the route-server rail.
//
// VALIDATES: on both rails the next-hop-self destination is sent NEXT_HOP
// 10.0.0.77, the connected endpoint, which is also what the announce rail's
// resolveNextHop answers for the same session; the default destination in the
// same fan-out is sent 192.0.2.254.
// PREVENTS: the forward rail passing the third-party NEXT_HOP because the local
// address was learned from the connection rather than typed.
//
// RFC requirement: RFC4271-5.1.3-3 positive -- with next-hop self and no configured local address, an internal destination is sent its session's connected local endpoint as NEXT_HOP on the general and route-server forward rails, the address resolveNextHop answers for that session, while a default internal destination in the same fan-out is sent the received third-party NEXT_HOP.
func TestRFC4271NextHopSelfWithAutoLocalAddressSendsTheConnectedEndpoint(t *testing.T) {
	connected := netip.MustParseAddr("10.0.0.77")
	for _, rs := range []bool{false, true} {
		self, session := autoLocalSelfDest(t, "192.0.2.64", connected)
		passing := a2Dest(t, "192.0.2.65", 65000, netip.Addr{}, false)

		announced, err := self.resolveNextHop(session, bgptypes.NewNextHopSelf(), family.IPv4Unicast)
		require.NoError(t, err)
		require.Equal(t, connected, announced, "the announce rail sends the connected endpoint")

		got := a2Forward(t, rs, a2InlinePayload(), self, passing)

		toSelf, ok := got[netip.MustParseAddr("192.0.2.64")]
		require.True(t, ok, "rs=%v: the next-hop-self destination is owed the route", rs)
		require.Equal(t, a2InlinePrefix, toSelf.nlri)
		require.Equal(t, connected.AsSlice(), toSelf.nextHop, "rs=%v: next-hop self is the connected endpoint", rs)

		toPassing, ok := got[netip.MustParseAddr("192.0.2.65")]
		require.True(t, ok, "rs=%v: the default destination is owed the route", rs)
		require.Equal(t, a2ThirdPartyNextHop, toPassing.nextHop, "rs=%v: the default destination is sent the received next hop", rs)
	}
}

// TestRFC4271NextHopSelfWithNoLocalAddressWithholdsTheRoute drives the refusal
// side of RFC 4271 Section 5.1.3 ("A BGP speaker MUST be able to support the
// disabling advertisement of third party NEXT_HOP attributes"): a disable that
// cannot be honored withholds the route rather than send the third-party next
// hop.
//
// Method: an internal destination is configured `next-hop self` with `local ip
// auto` and holds no session endpoint, so no address of this speaker exists for
// it. One UPDATE withdraws 198.51.100.0/24 and announces 192.0.2.0/24 with the
// third-party next hop 192.0.2.254; it is forwarded to that destination beside a
// default internal destination, on the general rail and on the route-server rail.
//
// VALIDATES: on both rails the next-hop-self destination is sent the withdrawal
// and no announcement, so the third-party next hop never reaches it; the default
// destination in the same fan-out is sent the announcement with 192.0.2.254.
// PREVENTS: next-hop self degrading in silence to passing the received next hop.
//
// RFC requirement: RFC4271-5.1.3-3 negative -- with next-hop self and no local address at all, the general and route-server forward rails send the destination the UPDATE's withdrawal and no announcement, while a default internal destination in the same fan-out is sent the announcement carrying the third-party next hop.
func TestRFC4271NextHopSelfWithNoLocalAddressWithholdsTheRoute(t *testing.T) {
	payload := a2Payload(a2ThirdPartyNextHop)
	for _, rs := range []bool{false, true} {
		withheld, _ := autoLocalSelfDest(t, "192.0.2.63", netip.Addr{})
		passing := a2Dest(t, "192.0.2.66", 65000, netip.Addr{}, false)

		got := a2Forward(t, rs, payload, withheld, passing)

		toWithheld, ok := got[netip.MustParseAddr("192.0.2.63")]
		require.True(t, ok, "rs=%v: the withdrawal half is still owed", rs)
		require.Equal(t, a2Withdrawn, toWithheld.withdrawn, "rs=%v: the withdrawal goes", rs)
		require.Empty(t, toWithheld.mpReach, "rs=%v: the announcement is withheld", rs)
		require.Empty(t, toWithheld.nlri, "rs=%v: the announcement is withheld", rs)

		toPassing, ok := got[netip.MustParseAddr("192.0.2.66")]
		require.True(t, ok, "rs=%v: the default destination is owed the route", rs)
		require.NotEmpty(t, toPassing.mpReach, "rs=%v: the default destination is sent the announcement", rs)
		require.Contains(t, string(toPassing.mpReach), string(a2ThirdPartyNextHop), "rs=%v: with the received next hop", rs)
	}
}

// TestRFC4271NextHopSelfWithheldRouteIsLoggedAtWarn proves the withhold of
// TestRFC4271NextHopSelfWithNoLocalAddressWithholdsTheRoute reaches the operator.
// A route withheld in silence reads as a peer that never sent it.
//
// Method: the forward logger is replaced by a text handler at Warn, the level
// ze.log runs at by default. The same next-hop-self destination with no local
// address, and a default destination beside it, receive one route on the
// general rail and on the route-server rail.
//
// VALIDATES: each rail writes exactly one WARN line for the withhold, naming the
// withheld destination and RFC 4271 Section 5.1.3; the default destination,
// which is sent the route, adds no line.
// PREVENTS: the withhold path losing its log line, or logging below the default
// level where no operator sees it.
func TestRFC4271NextHopSelfWithheldRouteIsLoggedAtWarn(t *testing.T) {
	const withholdLine = "withholding route: next-hop self is configured and the session has no local address"
	sink := &syncBuffer{}
	prev := fwdLogger
	fwdLogger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	t.Cleanup(func() { fwdLogger = prev })

	for _, rs := range []bool{false, true} {
		before := strings.Count(sink.String(), withholdLine)
		withheld, _ := autoLocalSelfDest(t, "192.0.2.67", netip.Addr{})
		passing := a2Dest(t, "192.0.2.68", 65000, netip.Addr{}, false)

		got := a2Forward(t, rs, a2Payload(a2ThirdPartyNextHop), withheld, passing)

		require.Empty(t, got[netip.MustParseAddr("192.0.2.67")].mpReach, "rs=%v: the announcement is withheld", rs)
		require.NotEmpty(t, got[netip.MustParseAddr("192.0.2.68")].mpReach, "rs=%v: the default destination is sent it", rs)

		logged := sink.String()
		require.Equal(t, before+1, strings.Count(logged, withholdLine), "rs=%v: one WARN line per withhold", rs)
		line := logged[strings.LastIndex(logged, "level="):]
		require.Contains(t, line, "level=WARN", "rs=%v: logged at the default level", rs)
		require.Contains(t, line, withholdLine, "rs=%v", rs)
		require.Contains(t, line, "peer=192.0.2.67", "rs=%v: the line names the withheld destination", rs)
		require.Contains(t, line, `rfc="RFC 4271 Section 5.1.3"`, "rs=%v: the line names the rule", rs)
	}
}
