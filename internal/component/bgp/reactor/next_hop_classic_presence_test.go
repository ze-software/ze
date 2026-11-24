// Design: docs/architecture/bgp/structural-forwarding.md -- classic presence after raw policy
// Related: forward_next_hop.go -- payloadNextHop and effectiveNextHop
// Related: ls71_pair_relay_test.go -- exact native mixed-refusal handback
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A raw policy base can carry classic NLRI without code 3. A configured IPv4
// replacement then creates code 3, so classic presence must survive parsing even
// when no original address existed. MP-only controls still ignore unused code 3.
func TestEffectiveNextHopClassicPresence(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.2")
	for _, tc := range []struct {
		name                string
		classic, code3, own bool
	}{
		{name: "classic-missing-code3", classic: true, own: true},
		{name: "classic-existing-code3", classic: true, code3: true, own: true},
		{name: "mp-only-missing-code3"},
		{name: "mp-only-unused-code3", code3: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := classicPresenceBody(t, tc.classic, tc.code3)
			original := bytes.Clone(body)
			base := payloadNextHop(body)
			facts := &peerForwardFacts{addr: peer, nhMode: nhModeSelf4, nhLegacy: [4]byte{192, 0, 2, 2}}
			var mods filterapi.ModAccumulator
			applyFactsNextHop(facts, &mods, base.mpFamily)
			applyNextHopFamily(facts, &mods, base, body)
			require.Equal(t, tc.own, egressNextHopIsPeerOwn(facts, &mods, base))
			wantGate := withholdNone
			if tc.own {
				wantGate = withholdPeerOwn
			}
			dest := NewPeer(&PeerSettings{Address: peer})
			require.Equal(t, wantGate, egressNextHopWithheld(dest, facts, &mods, base, false, netip.Addr{}))
			output, _, failure := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
			require.Equal(t, modifyFailureNone, failure)
			update, err := message.UnpackUpdate(output)
			require.NoError(t, err)
			_, _, hop, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
			require.True(t, found, "the materializer inserts the configured code 3")
			require.Equal(t, peer.AsSlice(), hop)
			require.Equal(t, tc.own, aigpNextHop(output).has(peer), "only classic NLRI makes code 3 a forwarding address")
			_, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
			require.True(t, found)
			require.Equal(t, classicPresenceReach(t), reach, "the zero-hop FlowSpec sibling survives")
			require.Equal(t, original, body)
		})
	}
}

// The received source is valid classic IPv4. Its real export policy introduces
// a zero-hop FlowSpec sibling and later omits code 3; a preceding next-hop:self
// repairs the attribute over the raw policy base. The selected recipient-owned
// repair must withdraw only classic NLRI, retain the useful FlowSpec rule, and
// recover both after correction.
func TestNextHopClassicPolicyRepairRecovery(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(nextHopAutoRailName(native), func(t *testing.T) {
			f, source, replacement := classicPresenceFixture(t)
			classicPresenceChangingPolicy(t, f, replacement)
			ls71PairRelay(t, f, source, native, false)
			classicPresenceAssert(t, f, netip.MustParseAddr("192.0.2.99"), false)

			f.conn.reset()
			*replacement = classicPresenceBody(t, true, false)
			classicPresenceSetRepairAddr(f, f.destination.settings.Address)
			ls71PairRelay(t, f, source, native, native)
			classicPresenceAssert(t, f, netip.Addr{}, true)

			f.conn.reset()
			classicPresenceSetRepairAddr(f, netip.MustParseAddr("192.0.2.99"))
			ls71PairRelay(t, f, source, native, false)
			classicPresenceAssert(t, f, netip.MustParseAddr("192.0.2.99"), false)
		})
	}
}

// Missing code 3 is not itself a refusal: an otherwise identical policy base
// repaired with a usable third-party address announces both exact sections.
func TestNextHopClassicPolicyUsefulRepair(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(nextHopAutoRailName(native), func(t *testing.T) {
			f, source, replacement := classicPresenceFixture(t)
			*replacement = classicPresenceBody(t, true, false)
			classicPresenceChangingPolicy(t, f, replacement)
			ls71PairRelay(t, f, source, native, false)
			classicPresenceAssert(t, f, netip.MustParseAddr("192.0.2.99"), false)
		})
	}
}

func classicPresenceFixture(t *testing.T) (*aigpReplayFixture, []byte, *[]byte) {
	t.Helper()
	tc := nextHopAutoFamilies()[0]
	f := nextHopAutoFixture(t, tc, true)
	classicPresenceSetRepairAddr(f, netip.MustParseAddr("192.0.2.99"))
	for _, peer := range []*Peer{f.source, f.destination} {
		peer.settings.AIGPSession = new(false)
		negotiateForwardTestFamilies(peer, family.IPv4Unicast, flowRedirectIPv4)
		peer.fwdFacts.Store(peer.buildForwardFacts())
	}
	replacement := classicPresenceBody(t, true, true)
	return f, nextHopAutoBody(t, tc, tc.prefix, nil), &replacement
}

func classicPresenceSetRepairAddr(f *aigpReplayFixture, addr netip.Addr) {
	f.destination.settings.LocalAddress = addr
	f.destination.session.transport.Store(&sessionTransport{local: addr, localString: addr.String()})
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
}

func classicPresenceChangingPolicy(t *testing.T, f *aigpReplayFixture, replacement *[]byte) {
	t.Helper()
	scope := f.destination.llScope.Load()
	f.r.orderedEgressSteps = buildOrderedEgressSteps()
	transport := &answerTransport{
		reads: []string{rpc.FilterFieldRaw},
		answer: func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
			return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: *replacement}
		},
	}
	f.r.filterTransportSeam = transport
	f.destination.settings.ExportPolicy = exportNextHopPolicy(t, []policychain.Member{
		{Name: policychain.BuiltinNextHopSelf},
		{Name: exportLibraryFilter, Plugin: answerPlugin},
	})
	f.destination.llScope.Store(scope)
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	t.Cleanup(func() { require.Positive(t, transport.calls(), "the real export policy must run") })
}

func classicPresenceReach(t *testing.T) []byte {
	t.Helper()
	// RFC 8955 framing: a length-five destination-prefix component for
	// 192.0.2.0/24, with no forwarding next hop and one reserved octet.
	return append([]byte{0, 1, 133, 0, 0}, mustHex(t, "050118c00002")...)
}

func classicPresenceBody(t *testing.T, classic, code3 bool) []byte {
	t.Helper()
	attrs := mixedAttrs(mixedAttr(14, classicPresenceReach(t)))
	if code3 {
		attrs = append(attrs, 0x40, 3, 4, 192, 0, 2, 9)
	}
	var prefix []byte
	if classic {
		prefix = mixedV4Legacy
	}
	return buildUpdatePayload(attrs, prefix)
}

func classicPresenceAssert(t *testing.T, f *aigpReplayFixture, hop netip.Addr, refused bool) {
	t.Helper()
	var classicAnnouncements, classicWithdrawals, flowAnnouncements int
	updates := nextHopAutoUpdates(t, f)
	require.NotEmpty(t, updates)
	for _, update := range updates {
		if len(update.NLRI) != 0 {
			classicAnnouncements++
			require.False(t, refused, "recipient-owned classic announcement escaped")
			require.Equal(t, mixedV4Legacy, update.NLRI, "only the policy's exact classic route leaves")
			_, _, value, present := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
			require.True(t, present)
			require.Equal(t, hop.AsSlice(), value)
		}
		if len(update.WithdrawnRoutes) != 0 {
			classicWithdrawals++
			require.True(t, refused)
			require.Equal(t, mixedV4Legacy, update.WithdrawnRoutes)
		}
		_, _, unreach, withdrawn := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
		require.False(t, withdrawn, "useful FlowSpec must never be withdrawn: %x", unreach)
		if _, _, reach, present := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI); present {
			flowAnnouncements++
			require.Equal(t, classicPresenceReach(t), reach, "exact useful zero-hop FlowSpec survivor")
		}
	}
	wantAnnouncements, wantWithdrawals := 1, 0
	if refused {
		wantAnnouncements, wantWithdrawals = 0, 1
	}
	require.Equal(t, wantAnnouncements, classicAnnouncements)
	require.Equal(t, wantWithdrawals, classicWithdrawals)
	require.Equal(t, 1, flowAnnouncements)
}
