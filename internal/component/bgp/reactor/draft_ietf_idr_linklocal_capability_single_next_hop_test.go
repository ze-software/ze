// Design: docs/architecture/bgp/structural-forwarding.md -- Link-Local-only egress scope
// Related: next_hop_auto_relay_test.go -- socket fixtures for both forwarding paths
// Related: forward_next_hop.go -- effective next-hop and off-link withholding
// RFC: rfc/short/draft-ietf-idr-linklocal-capability.md -- Section 4 item 3
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Section 4 item 3: "Link-Local IPv6 next hops MUST NOT be included."
// Each forwarding path has its own unit so its own recorded red must be observed.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 positive -- on the cached forwarding path, a received sixteen-octet Link-Local-only next hop under Unchanged is withheld from an ordinary multihop external peer with capability 77 and the prior prefix is withdrawn; usable Global and explicit-Self recovery remain advertised.
func TestDraftBareLinkLocalReceivedMultihopCached(t *testing.T) {
	singleNextHopReceivedMultihop(t, false)
}

// Section 4 item 3: "Link-Local IPv6 next hops MUST NOT be included."
// Each forwarding path has its own unit so its own recorded red must be observed.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 positive -- on the native forwarding path, a received sixteen-octet Link-Local-only next hop under Unchanged is withheld from an ordinary multihop external peer with capability 77 and the prior prefix is withdrawn; usable Global and explicit-Self recovery remain advertised.
func TestDraftBareLinkLocalReceivedMultihopNative(t *testing.T) {
	singleNextHopReceivedMultihop(t, true)
}

// Section 4 item 3: "Link-Local IPv6 next hops MUST NOT be included."
// Each forwarding path has its own unit so its own recorded red must be observed.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 positive -- on the cached forwarding path, a real export-library FilterModify.Raw replacement of Global input by a sixteen-octet Link-Local-only next hop is withheld from ordinary multihop eBGP with capability 77 and withdraws the prior prefix; valid Global policy output is advertised before and after.
func TestDraftBareLinkLocalPolicyMultihopCached(t *testing.T) {
	singleNextHopPolicyMultihop(t, false)
}

// Section 4 item 3: "Link-Local IPv6 next hops MUST NOT be included."
// Each forwarding path has its own unit so its own recorded red must be observed.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 positive -- on the native forwarding path, a real export-library FilterModify.Raw replacement of Global input by a sixteen-octet Link-Local-only next hop is withheld from ordinary multihop eBGP with capability 77 and withdraws the prior prefix; valid Global policy output is advertised before and after.
func TestDraftBareLinkLocalPolicyMultihopNative(t *testing.T) {
	singleNextHopPolicyMultihop(t, true)
}

// Section 4 item 3: "Link-Local IPv6 next hops MUST NOT be included."
// Each forwarding path has its own unit so its own recorded red must be observed.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 negative -- on the cached forwarding path, a directly attached ordinary external recipient with capability 77 receives the exact sixteen-octet Link-Local-only next hop and prefix from both received input and a real export-library raw policy replacement.
func TestDraftBareLinkLocalAttachedCached(t *testing.T) {
	singleNextHopAttached(t, false)
}

// Section 4 item 3: "Link-Local IPv6 next hops MUST NOT be included."
// Each forwarding path has its own unit so its own recorded red must be observed.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 negative -- on the native forwarding path, a directly attached ordinary external recipient with capability 77 receives the exact sixteen-octet Link-Local-only next hop and prefix from both received input and a real export-library raw policy replacement.
func TestDraftBareLinkLocalAttachedNative(t *testing.T) {
	singleNextHopAttached(t, true)
}

// The Global announcement first establishes a real Adj-RIB-Out entry. Its
// Link-Local-only replacement must withdraw that entry rather than retain it.
// Capability 77 prevents its independent refusal from masking the scope guard.
// MUTATION: bypass the borrowed-address return in egressNextHopLinkLocalOnlyOffLink.
func singleNextHopReceivedMultihop(t *testing.T, native bool) {
	t.Helper()
	tc := nextHopAutoFamilies()[1]
	f := singleNextHopExternalFixture(t, tc, false)
	f.destination.settings.NextHopMode = NextHopUnchanged
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
	hop, announced, withdrawn := singleNextHopSocketUpdate(t, f, tc)
	require.Equal(t, tc.received.AsSlice(), hop)
	require.Equal(t, tc.prefix, announced)
	require.Empty(t, withdrawn)

	f.conn.reset()
	bare := tc
	bare.received = netip.MustParseAddr("fe80::1")
	rfc4271ProofRelay(t, f, nextHopAutoBody(t, bare, tc.prefix, nil), native)
	hop, announced, withdrawn = singleNextHopSocketUpdate(t, f, tc)
	require.Empty(t, hop)
	require.Empty(t, announced)
	require.Equal(t, tc.prefix, withdrawn)

	f.conn.reset()
	f.destination.settings.NextHopMode = NextHopSelf
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	rfc4271ProofRelay(t, f, nextHopAutoBody(t, bare, tc.prefix, nil), native)
	hop, announced, withdrawn = singleNextHopSocketUpdate(t, f, tc)
	require.Equal(t, tc.self.AsSlice(), hop)
	require.Equal(t, tc.prefix, announced)
	require.Empty(t, withdrawn)
}

// A raw export-library reply is a real reachable producer of an effective
// Link-Local-only next hop. Config's explicit address leaf rejects that value,
// so the test does not create an impossible PeerSettings configuration.
// MUTATION: bypass the borrowed-address return, now judging the policy's base.
func singleNextHopPolicyMultihop(t *testing.T, native bool) {
	t.Helper()
	tc := nextHopAutoFamilies()[1]
	f := singleNextHopExternalFixture(t, tc, false)
	f.destination.settings.NextHopMode = NextHopUnchanged
	policy := tc
	policy.received = tc.self
	singleNextHopRawPolicy(t, f, nextHopAutoBody(t, policy, tc.prefix, nil))
	rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
	hop, announced, withdrawn := singleNextHopSocketUpdate(t, f, tc)
	require.Equal(t, tc.self.AsSlice(), hop, "the valid raw policy actually changes the received Global")
	require.Equal(t, tc.prefix, announced)
	require.Empty(t, withdrawn)

	f.conn.reset()
	policy.received = netip.MustParseAddr("fe80::254")
	singleNextHopRawPolicy(t, f, nextHopAutoBody(t, policy, tc.prefix, nil))
	rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
	hop, announced, withdrawn = singleNextHopSocketUpdate(t, f, tc)
	require.Empty(t, hop)
	require.Empty(t, announced)
	require.Equal(t, tc.prefix, withdrawn)

	f.conn.reset()
	policy.received = tc.self
	singleNextHopRawPolicy(t, f, nextHopAutoBody(t, policy, tc.prefix, nil))
	rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
	hop, announced, withdrawn = singleNextHopSocketUpdate(t, f, tc)
	require.Equal(t, tc.self.AsSlice(), hop)
	require.Equal(t, tc.prefix, announced)
	require.Empty(t, withdrawn)
}

// Section 4 item 2 permits the directly attached capability-77 recipient to
// receive the single Link-Local, whether borrowed or produced by export policy.
// MUTATION: bypass destOnLink's exemption in egressNextHopLinkLocalOnlyOffLink.
func singleNextHopAttached(t *testing.T, native bool) {
	t.Helper()
	tc := nextHopAutoFamilies()[1]
	for _, fromPolicy := range []bool{false, true} {
		name := "received"
		if fromPolicy {
			name = "policy"
		}
		t.Run(name, func(t *testing.T) {
			f := singleNextHopExternalFixture(t, tc, true)
			f.destination.settings.NextHopMode = NextHopUnchanged
			f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
			input := tc
			input.received = netip.MustParseAddr("fe80::1")
			want := input.received
			if fromPolicy {
				policy := tc
				policy.received = netip.MustParseAddr("fe80::254")
				singleNextHopRawPolicy(t, f, nextHopAutoBody(t, policy, tc.prefix, nil))
				input = tc
				want = policy.received
			}
			rfc4271ProofRelay(t, f, nextHopAutoBody(t, input, tc.prefix, nil), native)
			hop, announced, withdrawn := singleNextHopSocketUpdate(t, f, tc)
			require.Equal(t, want.AsSlice(), hop)
			require.Equal(t, tc.prefix, announced)
			require.Empty(t, withdrawn)
		})
	}
}

// Preserve the fixture's connected-prefix evidence: the library helper normally
// refreshes it from host interfaces while binding the real export policy chain.
func singleNextHopRawPolicy(t *testing.T, f *aigpReplayFixture, replacement []byte) {
	t.Helper()
	scope := f.destination.llScope.Load()
	exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: replacement}
	}, f.destination)
	f.destination.llScope.Store(scope)
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
}

// This fixture makes eBGP role, connected scope and capability-77 permission
// explicit, so each test's result depends on the scope condition it names.
func singleNextHopExternalFixture(t *testing.T, tc nextHopAutoFamily, direct bool) *aigpReplayFixture {
	t.Helper()
	f := nextHopAutoFixture(t, tc, direct)
	for _, peer := range []*Peer{f.source, f.destination} {
		peer.negotiated.Load().LinkLocalNextHop = true
	}
	require.True(t, f.destination.Settings().IsEBGP())
	require.False(t, f.destination.Settings().RSClient)
	require.Equal(t, direct, destOnLink(f.destination))
	return f
}

func singleNextHopSocketUpdate(t *testing.T, f *aigpReplayFixture, tc nextHopAutoFamily) (hop, announced, withdrawn []byte) {
	t.Helper()
	updates := nextHopAutoUpdates(t, f)
	require.Len(t, updates, 1, "the destination socket must contain exactly one UPDATE")
	return nextHopAutoFields(t, tc, updates[0])
}
