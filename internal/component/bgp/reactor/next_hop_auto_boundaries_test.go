// Design: docs/architecture/bgp/structural-forwarding.md -- forwarded Auto next-hop boundaries
// Related: next_hop_auto_relay_test.go -- receive-to-socket fixtures for both relay paths
// Related: forward_next_hop.go -- the effective next-hop scope and withholding decisions
package reactor

import (
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A multihop internal recipient keeps a usable Global under Auto, withdraws
// that advertised route when only a Link-Local remains, and can receive the
// same Link-Local input through explicit Self. Socket bytes on each relay path
// distinguish the conditional refusal from dropping every internal route.
// RFC 4271 Section 5.1.3 and draft-ietf-idr-linklocal-capability Section 4.
func TestNextHopAutoInternalMultihopUsesOnlyUsableNextHop(t *testing.T) {
	tc := nextHopAutoFamilies()[1]
	for _, native := range []bool{false, true} {
		t.Run(nextHopAutoRailName(native), func(t *testing.T) {
			f := nextHopAutoFixture(t, tc, false)
			// Capability 77 permits the Link-Local-only form in principle.
			// Its absence would mask the off-link scope rule with a separate
			// capability refusal, even if the off-link guard were removed.
			for _, peer := range []*Peer{f.source, f.destination} {
				peer.negotiated.Load().LinkLocalNextHop = true
			}
			f.destination.settings.PeerAS = f.destination.settings.LocalAS
			f.destination.settings.ExportPolicy = setExportNextHopAuto(f.destination.settings.ExportPolicy)
			f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
			rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
			updates := nextHopAutoUpdates(t, f)
			require.Len(t, updates, 1)
			hop, announced, withdrawn := nextHopAutoFields(t, tc, updates[0])
			require.Equal(t, tc.received.AsSlice(), hop, "Auto preserves a usable Global toward iBGP")
			require.Equal(t, tc.prefix, announced)
			require.Empty(t, withdrawn)

			f.conn.reset()
			linkLocal := tc
			linkLocal.received = netip.MustParseAddr("fe80::1")
			rfc4271ProofRelay(t, f, nextHopAutoBody(t, linkLocal, tc.prefix, nil), native)
			updates = nextHopAutoUpdates(t, f)
			require.Len(t, updates, 1)
			hop, announced, withdrawn = nextHopAutoFields(t, tc, updates[0])
			require.Empty(t, hop)
			require.Empty(t, announced)
			require.Equal(t, tc.prefix, withdrawn, "the previously advertised route is withdrawn")

			f.conn.reset()
			f.destination.settings.ExportPolicy = setExportNextHopSelf(f.destination.settings.ExportPolicy)
			f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
			rfc4271ProofRelay(t, f, nextHopAutoBody(t, linkLocal, tc.prefix, nil), native)
			updates = nextHopAutoUpdates(t, f)
			require.Len(t, updates, 1)
			hop, announced, withdrawn = nextHopAutoFields(t, tc, updates[0])
			require.Equal(t, tc.self.AsSlice(), hop, "explicit Self supplies a usable Global")
			require.Equal(t, tc.prefix, announced)
			require.Empty(t, withdrawn)
		})
	}
}

// Auto chooses the existing Self encoding: the configured Link-Local is appended
// only when the recipient and this speaker's Global share a connected subnet.
// The off-link survivor still carries its Global, rather than losing the route.
// RFC 2545 Section 3; RFC 4271 Section 5.1.3.
func TestNextHopAutoIPv6PairFollowsConnectedSubnet(t *testing.T) {
	tc := nextHopAutoFamilies()[1]
	linkLocal := netip.MustParseAddr("fe80::254")
	for _, direct := range []bool{false, true} {
		for _, native := range []bool{false, true} {
			t.Run(nextHopAutoCaseName(tc, direct, native), func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, direct)
				f.destination.settings.ExportPolicy = setExportNextHopAuto(f.destination.settings.ExportPolicy)
				f.destination.settings.LinkLocal = linkLocal
				f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
				updates := nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, announced, withdrawn := nextHopAutoFields(t, tc, updates[0])
				want := tc.self.AsSlice()
				if direct {
					want = append(want, linkLocal.AsSlice()...)
				}
				require.Equal(t, want, hop)
				require.Equal(t, tc.prefix, announced)
				require.Empty(t, withdrawn)
			})
		}
	}
}

// An Auto destination that loses its local address emits the production warning
// at the default Warn level and identifies the affected peer. A successful
// advertisement first proves that the same path previously had usable output.
// RFC 4271 Section 5.1.3; next-hop-auto AC-4.
func TestNextHopAutoMissingOwnAddressWarnsForThePeer(t *testing.T) {
	const withholdLine = "withholding route: next-hop self is configured and the session has no local address"
	sink := &syncBuffer{}
	previous := fwdLogger
	fwdLogger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	t.Cleanup(func() { fwdLogger = previous })
	tc := nextHopAutoFamilies()[0]
	for _, native := range []bool{false, true} {
		t.Run(nextHopAutoRailName(native), func(t *testing.T) {
			before := strings.Count(sink.String(), withholdLine)
			f := nextHopAutoFixture(t, tc, true)
			f.destination.settings.ExportPolicy = setExportNextHopAuto(f.destination.settings.ExportPolicy)
			f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
			rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
			updates := nextHopAutoUpdates(t, f)
			require.Len(t, updates, 1)
			hop, announced, withdrawn := nextHopAutoFields(t, tc, updates[0])
			require.Equal(t, tc.self.AsSlice(), hop)
			require.Equal(t, tc.prefix, announced)
			require.Empty(t, withdrawn)
			require.Equal(t, before, strings.Count(sink.String(), withholdLine))

			f.conn.reset()
			f.destination.settings.LocalAddress = netip.Addr{}
			f.destination.session.transport.Store(nil)
			f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
			rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
			updates = nextHopAutoUpdates(t, f)
			require.Len(t, updates, 1)
			hop, announced, withdrawn = nextHopAutoFields(t, tc, updates[0])
			require.Empty(t, hop)
			require.Empty(t, announced)
			require.Equal(t, tc.prefix, withdrawn)
			logged := sink.String()
			require.Equal(t, before+1, strings.Count(logged, withholdLine))
			line := logged[strings.LastIndex(logged, "level="):]
			require.Contains(t, line, "level=WARN")
			require.Contains(t, line, "peer="+f.destination.Settings().Address.String())
			require.Contains(t, line, `rfc="RFC 4271 Section 5.1.3"`)
		})
	}
}
