// Design: docs/architecture/bgp/structural-forwarding.md -- scope of the selected MP field
// Related: next_hop_last_set_materializer_test.go -- last nonempty Set selection
// Related: ls71_pair_scope_test.go -- literal LS fields and immutable topology snapshots
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// Scope must judge the last field the materializer selects. Supply ordered Sets
// at that structural boundary, then send the resulting body through the real
// worker or native direct writer with received-cache provenance. This does not
// claim the current export API leaves several raw Sets in the relay accumulator:
// its library chain materializes an override before the relay records its ops.
// Literal LS71/72 fields distinguish missed off-link trimming from missed
// on-link growth, while legal-last, empty-last and fallback controls stay useful.
func TestNextHopScopeUsesMaterializedField(t *testing.T) {
	for _, rd := range []bool{false, true} {
		for _, native := range []bool{false, true} {
			for _, policy := range []bool{false, true} {
				for _, tc := range nextHopScopeSelections(t, rd) {
					name := nextHopScopeFamilyName(rd) + "/" + nextHopAutoRailName(native)
					name += "/received-base/" + tc.name
					if policy {
						name = nextHopScopeFamilyName(rd) + "/" + nextHopAutoRailName(native) + "/policy-base/" + tc.name
					}
					t.Run(name, func(t *testing.T) {
						topology := ls71PairTopologies()[0]
						if tc.offLink {
							topology = ls71PairTopologies()[1]
						}
						f := nextHopScopeFixture(t, rd, topology)
						body := nextHopScopeBody(t, rd, tc.received)
						original := bytes.Clone(body)
						var selectedOriginal [][]byte
						var mods filterapi.ModAccumulator
						for _, set := range tc.sets {
							selectedOriginal = append(selectedOriginal, bytes.Clone(set))
							mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, set)
						}
						cached, base := nextHopScopeBase(t, f, rd, body, policy)
						cachedOriginal := bytes.Clone(cached.WireUpdate.Payload())
						facts := f.destination.forwardFacts()
						require.NotNil(t, facts)
						baseNextHop := payloadNextHop(base.Payload())
						applyEgressNextHopScope(f.destination, facts, &mods, base.Payload(), baseNextHop.mpFamily)
						require.Equal(t, withholdNone, egressNextHopWithheld(f.destination, facts, &mods,
							baseNextHop, false, f.source.Settings().Address), "the selected field remains useful")
						output, _, failure := buildModifiedPayload(base.Payload(), &mods, f.r.attrModHandlers, nil, nil)
						require.Equal(t, modifyFailureNone, failure)
						if output == nil {
							output = base.Payload()
						}
						nextHopScopeWrite(t, f, cached, output, native)
						nextHopScopeAssert(t, f, rd, tc.want)
						require.Equal(t, original, body, "received or raw-policy base stays borrowed")
						require.Equal(t, cachedOriginal, cached.WireUpdate.Payload(), "cached source stays immutable")
						for i, set := range tc.sets {
							require.Equal(t, selectedOriginal[i], set, "policy operation buffers stay borrowed")
						}
					})
				}
			}
		}
	}
}

type nextHopScopeSelection struct {
	name           string
	received, want []byte
	sets           [][]byte
	offLink        bool
}

func nextHopScopeSelections(t *testing.T, rd bool) []nextHopScopeSelection {
	t.Helper()
	received := mustHex(t, "20010db8000100000000000000000009")
	owned := mustHex(t, "20010db800010000000000000000000a")
	other := mustHex(t, "20010db8000100000000000000000007")
	entityLL := mustHex(t, "fe800000000000000000000000000009")
	ownLL := mustHex(t, "fe800000000000000000000000000001")
	otherLL := mustHex(t, "fe800000000000000000000000000007")
	pair := append(bytes.Clone(received), entityLL...)
	ownPair := append(bytes.Clone(owned), ownLL...)
	otherPair := append(bytes.Clone(other), otherLL...)
	offTable := mustHex(t, "0000000000000000c0000207")
	if rd {
		pair = ls72Framed(received, entityLL)
		ownPair = ls72Framed(owned, ownLL)
		otherPair = ls72Framed(other, otherLL)
		received = ls72Framed(received, nil)
		owned = ls72Framed(owned, nil)
		other = ls72Framed(other, nil)
		// A decoded plain IPv6 global is outside LS72's declared RD widths.
		offTable = mustHex(t, "20010db8000100000000000000000007")
	}
	bad := []byte{0xfe}
	return []nextHopScopeSelection{
		{name: "off-link-received-pair", received: pair, want: received, offLink: true},
		{name: "off-link-valid-then-invalid-retains-received", received: pair, sets: [][]byte{other, bad}, want: received, offLink: true},
		{name: "off-link-valid-then-off-table-retains-received", received: pair, sets: [][]byte{other, offTable}, want: received, offLink: true},
		{name: "off-link-valid-then-empty", received: pair, sets: [][]byte{other, nil}, want: other, offLink: true},
		{name: "off-link-invalid-then-valid-pair", received: pair, sets: [][]byte{bad, otherPair}, want: other, offLink: true},
		{name: "on-link-owned-then-empty-grows-selected", received: received, sets: [][]byte{owned, nil}, want: ownPair},
		{name: "on-link-owned-then-invalid-grows-received", received: received, sets: [][]byte{owned, bad}, want: pair},
		{name: "on-link-owned-then-off-table-grows-received", received: received, sets: [][]byte{owned, offTable}, want: pair},
		{name: "on-link-invalid-then-owned-grows-selected", received: received, sets: [][]byte{bad, owned}, want: ownPair},
		{name: "on-link-pair-then-empty", received: received, sets: [][]byte{otherPair, nil}, want: otherPair},
		{name: "on-link-pair-then-invalid-retains-received-pair", received: pair, sets: [][]byte{otherPair, bad}, want: pair},
		{name: "on-link-last-valid-pair", received: pair, sets: [][]byte{owned, otherPair}, want: otherPair},
	}
}

// Full receipt-to-relay characterization keeps the reachable policy-base path
// apart from the structural multi-Set probe above. The actual export chain
// replaces a distinct received field once; its effective pair trims off-link
// and its speaker-owned single grows on-link before either send path writes it.
func TestNextHopScopeRelayCharacterization(t *testing.T) {
	for _, rd := range []bool{false, true} {
		for _, native := range []bool{false, true} {
			for _, policy := range []bool{false, true} {
				for _, offLink := range []bool{false, true} {
					name := nextHopScopeFamilyName(rd) + "/" + nextHopAutoRailName(native) + "/received-base"
					if policy {
						name = nextHopScopeFamilyName(rd) + "/" + nextHopAutoRailName(native) + "/policy-base"
					}
					if offLink {
						name += "/off-link"
					} else {
						name += "/on-link"
					}
					t.Run(name, func(t *testing.T) {
						topology := ls71PairTopologies()[0]
						if offLink {
							topology = ls71PairTopologies()[1]
						}
						f := nextHopScopeFixture(t, rd, topology)
						single := mustHex(t, "20010db800010000000000000000000a")
						pair := mustHex(t, "20010db800010000000000000000000afe800000000000000000000000000001")
						other := mustHex(t, "20010db8000100000000000000000007")
						if rd {
							single = ls72Framed(single, nil)
							pair = ls72Framed(pair[:16], pair[16:])
							other = ls72Framed(other, nil)
						}
						field, want := single, pair
						if offLink {
							field, want = pair, single
						}
						body := nextHopScopeBody(t, rd, field)
						if policy {
							ls71PairPolicy(t, f, body)
							body = nextHopScopeBody(t, rd, other)
						}
						original := bytes.Clone(body)
						rfc4271ProofRelay(t, f, body, native)
						nextHopScopeAssert(t, f, rd, want)
						require.Equal(t, original, body)
					})
				}
			}
		}
	}
}

func nextHopScopeFamilyName(rd bool) string {
	if rd {
		return "LS72"
	}
	return "LS71"
}

func nextHopScopeFixture(t *testing.T, rd bool, topology ls71PairTopology) *aigpReplayFixture {
	t.Helper()
	mode := ls71PairMode{name: "unchanged", mode: policychain.NextHopModeUnchanged, preserving: true}
	if rd {
		return ls72Fixture(t, mode, topology)
	}
	return ls71PairFixture(t, mode, topology)
}

func nextHopScopeBody(t *testing.T, rd bool, field []byte) []byte {
	t.Helper()
	if rd {
		return ls72Body(t, field, false)
	}
	return ls71PairBody(t, field, false)
}

func nextHopScopeAssert(t *testing.T, f *aigpReplayFixture, rd bool, field []byte) {
	t.Helper()
	if rd {
		ls72Assert(t, f, field, false, false)
		return
	}
	ls71PairAssert(t, f, field, false, false)
}

// The base is obtained from actual receipt and, in the policy arm, the same
// library-chain entry both forwarding rails call. Its raw answer is distinct
// from the received field, so skipping the chain cannot satisfy the wire oracle.
func nextHopScopeBase(t *testing.T, f *aigpReplayFixture, rd bool, body []byte, policy bool) (*ReceivedUpdate, *wireu.WireUpdate) {
	t.Helper()
	source := body
	if policy {
		ls71PairPolicy(t, f, body)
		other := mustHex(t, "20010db8000100000000000000000007")
		if rd {
			other = ls72Framed(other, nil)
		}
		source = nextHopScopeBody(t, rd, other)
	}
	id := f.receive(t, source)
	cached, present := f.r.recentUpdates.Get(id)
	require.True(t, present)
	if !policy {
		return cached, cached.WireUpdate
	}
	facts := f.destination.forwardFacts()
	require.NotNil(t, facts)
	session := policyview.ExportSession(facts.filterInfo, f.source.forwardFacts().filterInfo, true, filterapi.ExportExplicit)
	res := f.r.runEgressPolicyChain(facts.exportPolicy, facts.addrStr, facts.peerAS, facts.localAS, !facts.isEBGP,
		&session, cached.WireUpdate)
	require.True(t, res.accept)
	require.False(t, res.failed)
	require.False(t, res.withdraw)
	require.Empty(t, res.removals)
	require.NotNil(t, res.wireOverride)
	require.Equal(t, body, res.wireOverride.Payload())
	return cached, res.wireOverride
}

// Continue the structural scope/materializer boundary at the actual cached or
// native send entry. Both retain real source/session generation and provenance;
// the direct arm explicitly serves the source read-loop's deferred flush fence.
func nextHopScopeWrite(t *testing.T, f *aigpReplayFixture, cached *ReceivedUpdate, body []byte, native bool) {
	t.Helper()
	item := fwdItem{
		peer: f.destination, session: f.destination.session, authority: adjOutForwarded,
		rawBodies: [][]byte{body}, sourcePeerStr: cached.SourcePeerStr,
		sourceMessageID: cached.WireUpdate.MessageID(), receivedPeer: cached.receivedPeer,
		receivedGeneration: cached.receivedGeneration, sourcePolicyEpoch: cached.SourcePolicyEpoch,
		exportGeneration: f.destination.forwardFacts().exportGeneration,
	}
	owned := true
	defer func() {
		if owned {
			f.r.fwdPool.releaseItem(&item)
		}
	}()
	effective := wireu.NewWireUpdate(body, f.ctxID)
	require.NoError(t, prepareFwdProvenance(&item, cached.WireUpdate, effective, true))
	if native {
		handled, written, session := tryDirectWriteNoFlush(&item)
		require.True(t, handled, "the native send entry must own this established destination")
		require.True(t, written, "the native send entry must deliver its selected field")
		require.Same(t, f.destination.session, session)
		f.source.session.appendFwdDirty(f.destination, session)
		f.source.session.flushFwdDirty()
		require.Empty(t, f.source.session.fwdDirty)
		return
	}
	require.True(t, f.r.fwdPool.Dispatch(fwdKey{peerAddr: f.destination.Settings().PeerKey()}, &item))
	owned = false // Dispatch copied the item and transferred its private provenance.
	f.drain(t)
}
