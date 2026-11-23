// Design: docs/architecture/bgp/egress-attribute-rules.md -- MED after ordinary selection.
// Related: rfc4271_selected_advertisement_test.go -- actual RIB and socket controls.
// Related: filter_answer_transport_test.go -- declared raw and MED policy answers.
// RFC: rfc/short/rfc4271.md -- Section 9.1.2.2 MED presence preservation before IBGP readvertisement.
package reactor

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC4271OrdinarySelectedMEDSurvivesRawEgressRemoval receives actual
// same-neighbor-AS (65010) candidates before connecting ordinary internal
// recipients. Both an all-eBGP pair and a mixed iBGP/eBGP pair prove that
// ordinary RIB MED comparison runs and that raw egress policy omitting MED
// cannot strip the selected MED before iBGP readvertisement, while a distinct
// no-MED control gains no MED, an explicit egress MED Set (7) wins, and an
// iBGP winner is withheld from internal peers by split horizon.
//
// VALIDATES: spec-rfc4271-med-ibgp-readvertisement AC-1 through AC-3 and AC-10.
// PREVENTS: post-selection raw egress replacement silently dropping MED on
// ordinary selected-route iBGP readvertisement without restricting MED
// comparison to eBGP-only routes.
// RFC 4271 Section 9.1.2.2: "If an implementation chooses to remove
// MULTI_EXIT_DISC, then the optional comparison on MULTI_EXIT_DISC, if performed,
// MUST be performed only among EBGP-learned routes."
// RFC requirement: RFC4271-9.1.2.2-2 positive -- when ordinary RIB route
// selection compares same-neighbor-AS (65010) candidates across mixed iBGP/eBGP
// and all-eBGP sources and readvertises the selected route over iBGP through a
// raw export policy that omits MED and rewrites a community, applyFactsMED
// restores the selected MED on the destination wire alongside the rewritten
// community.
// RFC requirement: RFC4271-9.1.2.2-2 negative -- on the same ordinary iBGP
// readvertisement path, a distinct selected route received without MED gains no
// MED after the raw export policy, a separate internal recipient's explicit
// egress MED Set (7) wins over the selected MED, and an iBGP-learned candidate
// winning at MED 10 over an eBGP candidate at MED 20 is withheld from internal
// recipients by iBGP split horizon.
// MUTATION: omit applyFactsMED's internal raw-absence restoration; the raw
// recipient's selected MED disappears while its rewritten community survives.
func TestRFC4271OrdinarySelectedMEDSurvivesRawEgressRemoval(t *testing.T) {
	t.Run("ebgp-same-neighbor-as", testOrdinarySelectedMEDAllEBGP)
	t.Run("mixed-ibgp-ebgp-same-neighbor-as", testOrdinarySelectedMEDMixedIBGPEBGP)
}

func testOrdinarySelectedMEDAllEBGP(t *testing.T) {
	policy := &ordinarySelectedMEDPolicy{}
	transport := &answerTransport{reads: []string{rpc.FilterFieldRaw, "med"},
		sets: []string{"med"}, answer: policy.answer}
	peers := ordinarySelectedMEDPeers(t, transport)
	a := lowLiveConnect(t, peers[0], 0x0a00fe01)
	b := lowLiveConnect(t, peers[1], 0x0a00fe02)
	subject := netip.MustParsePrefix("198.19.211.0/24")
	control := netip.MustParsePrefix("198.19.212.0/24")
	aHop, bHop := peers[0].Settings().Address, peers[1].Settings().Address

	// RFC 4271 Sections 4.3 and 9.1.2.2: both candidates enter over sockets.
	ownershipLiveSend(t, a, nil, ordinaryBoundaryMEDAttrs(aHop.String(), 30), subject)
	ownershipLiveSend(t, b, nil, ordinaryBoundaryMEDAttrs(bHop.String(), 20), subject)
	ordinarySelectedMEDStored(t, a, subject, 30)
	ordinarySelectedMEDStored(t, b, subject, 20)
	ordinarySelectedBest(t, subject, bHop.String())
	ordinaryBoundaryDrain(t, a)
	ordinaryBoundaryDrain(t, b)

	raw := lowLiveConnect(t, peers[2], 0x0a00fe03)
	set := lowLiveConnect(t, peers[3], 0x0a00fe04)
	ordinarySelectedWire(t, raw, subject, bHop.String())
	ordinarySelectedWire(t, set, subject, bHop.String())
	// A different useful route prevents an all-output refusal from passing.
	ownershipLiveSend(t, a, nil, ordinarySelectedAttrs(aHop.String(), 100), control)
	ordinarySelectedStored(t, a, control)
	ordinarySelectedBest(t, control, aHop.String())
	ordinarySelectedWire(t, raw, control, aHop.String())
	ordinarySelectedWire(t, set, control, aHop.String())
	ordinaryBoundaryDrain(t, a)
	ordinaryBoundaryCapture(t, raw)
	ordinaryBoundaryCapture(t, set)
	rawView, setView := ownershipLiveRead(t, raw), ownershipLiveRead(t, set)
	ordinarySelectedMEDWire(t, rawView.routes[subject], 20)
	ordinarySelectedMEDMarker(t, rawView.routes[subject])
	ordinarySelectedMEDWire(t, setView.routes[subject], 7)
	ordinarySelectedMEDMarker(t, rawView.routes[control])
	_, _, _, controlHasMED := attribute.AttrFind(rawView.routes[control], attribute.AttrMED)
	require.False(t, controlHasMED, "a selected source without MED must not gain one")
	ordinarySelectedMEDWire(t, setView.routes[control], 7)
	require.Positive(t, policy.initial.Load(), "the raw answer must actually omit received MED 20")

	// Change only A's MED; B remains received with the same MED 20.
	ownershipLiveSend(t, a, nil, ordinaryBoundaryMEDAttrs(aHop.String(), 10), subject)
	ordinarySelectedMEDStored(t, a, subject, 10)
	ordinarySelectedMEDStored(t, b, subject, 20)
	ordinarySelectedBest(t, subject, aHop.String())
	ordinarySelectedWire(t, raw, subject, aHop.String())
	ordinarySelectedWire(t, set, subject, aHop.String())
	ordinaryBoundaryDrain(t, a)
	ordinaryBoundaryCapture(t, raw)
	ordinaryBoundaryCapture(t, set)
	rawView, setView = ownershipLiveRead(t, raw), ownershipLiveRead(t, set)
	ordinarySelectedMEDWire(t, rawView.routes[subject], 10)
	ordinarySelectedMEDMarker(t, rawView.routes[subject])
	ordinarySelectedMEDWire(t, setView.routes[subject], 7)
	ordinarySelectedMEDMarker(t, rawView.routes[control])
	_, _, _, controlHasMED = attribute.AttrFind(rawView.routes[control], attribute.AttrMED)
	require.False(t, controlHasMED)
	ordinarySelectedMEDWire(t, setView.routes[control], 7)
	require.Positive(t, policy.changed.Load(), "the raw answer must actually omit received MED 10")
	require.Positive(t, transport.callsFor(peers[3].Settings().Address.String()),
		"the separate recipient's declared MED Set must execute")
	ownershipLiveEstablished(t, []*lowLivePeer{a, b, raw, set})
}

func testOrdinarySelectedMEDMixedIBGPEBGP(t *testing.T) {
	policy := &ordinarySelectedMEDPolicy{}
	transport := &answerTransport{reads: []string{rpc.FilterFieldRaw, "med"},
		sets: []string{"med"}, answer: policy.answer}
	settings := []*PeerSettings{
		acceptingLiveSettings(t, "192.0.2.215", 65000, 65000),
		acceptingLiveSettings(t, "192.0.2.216", 65000, 65010),
		acceptingLiveSettings(t, "192.0.2.217", 65000, 65000),
		acceptingLiveSettings(t, "192.0.2.218", 65000, 65000),
	}
	peers := startOrdinarySelectedMEDPeers(t, transport, settings)
	a := lowLiveConnect(t, peers[0], 0x0a00fe11)
	b := lowLiveConnect(t, peers[1], 0x0a00fe12)
	subject := netip.MustParsePrefix("198.19.213.0/24")
	control := netip.MustParsePrefix("198.19.214.0/24")
	fence := netip.MustParsePrefix("198.19.215.0/24")
	aHop, bHop := peers[0].Settings().Address, peers[1].Settings().Address

	// RFC 4271 Section 9.1.2.2: compare iBGP A and eBGP B sharing neighbor AS
	// 65010 before connecting recipients. A30/B20 selects B; A-only MED 10
	// selects A despite eBGP-over-iBGP preference; restored A30/B20 selects B.
	ownershipLiveSend(t, a, nil, ordinaryBoundaryMEDAttrs(aHop.String(), 30), subject)
	ownershipLiveSend(t, b, nil, ordinaryEBGPMEDAttrs(bHop, 65010, 20), subject)
	ordinarySelectedMEDStored(t, a, subject, 30)
	ordinarySelectedMEDStored(t, b, subject, 20)
	ordinarySelectedBest(t, subject, bHop.String())

	ownershipLiveSend(t, a, nil, ordinaryBoundaryMEDAttrs(aHop.String(), 10), subject)
	ordinarySelectedMEDStored(t, a, subject, 10)
	ordinarySelectedMEDStored(t, b, subject, 20)
	ordinarySelectedBest(t, subject, aHop.String())

	ownershipLiveSend(t, a, nil, ordinaryBoundaryMEDAttrs(aHop.String(), 30), subject)
	ordinarySelectedMEDStored(t, a, subject, 30)
	ordinarySelectedMEDStored(t, b, subject, 20)
	ordinarySelectedBest(t, subject, bHop.String())
	ordinaryBoundaryDrain(t, a)
	ordinaryBoundaryDrain(t, b)

	raw := lowLiveConnect(t, peers[2], 0x0a00fe13)
	set := lowLiveConnect(t, peers[3], 0x0a00fe14)
	ordinarySelectedWire(t, raw, subject, bHop.String())
	ordinarySelectedWire(t, set, subject, bHop.String())

	// The useful no-MED control comes from eBGP B: an iBGP A-origin control is
	// withheld from internal recipients by ordinary iBGP split horizon.
	noMEDControl := synthesizedNextHopAttrs(bHop, 65010)
	ownershipLiveSend(t, b, nil, noMEDControl, control)
	ordinarySelectedStored(t, b, control)
	ordinarySelectedBest(t, control, bHop.String())
	ordinarySelectedWire(t, raw, control, bHop.String())
	ordinarySelectedWire(t, set, control, bHop.String())
	ordinaryBoundaryDrain(t, b)
	ordinaryBoundaryCapture(t, raw)
	ordinaryBoundaryCapture(t, set)
	rawView, setView := ownershipLiveRead(t, raw), ownershipLiveRead(t, set)
	ordinarySelectedMEDWire(t, rawView.routes[subject], 20)
	ordinarySelectedMEDMarker(t, rawView.routes[subject])
	ordinarySelectedMEDWire(t, setView.routes[subject], 7)
	ordinarySelectedMEDMarker(t, rawView.routes[control])
	_, _, _, controlHasMED := attribute.AttrFind(rawView.routes[control], attribute.AttrMED)
	require.False(t, controlHasMED, "a selected eBGP source without MED must not gain one")
	ordinarySelectedMEDWire(t, setView.routes[control], 7)
	require.Positive(t, policy.initial.Load(), "the raw answer must actually omit received MED 20")

	// When iBGP A wins at MED 10, ordinary iBGP split horizon withdraws B's
	// prior advertisement and withholds A's route from both internal recipients.
	ownershipLiveSend(t, a, nil, ordinaryBoundaryMEDAttrs(aHop.String(), 10), subject)
	ordinarySelectedMEDStored(t, a, subject, 10)
	ordinarySelectedMEDStored(t, b, subject, 20)
	ordinarySelectedBest(t, subject, aHop.String())
	ordinaryBoundaryDrain(t, a)

	ownershipLiveSend(t, b, nil, noMEDControl, fence)
	ordinarySelectedStored(t, b, fence)
	ordinarySelectedBest(t, fence, bHop.String())
	ordinarySelectedWire(t, raw, fence, bHop.String())
	ordinarySelectedWire(t, set, fence, bHop.String())
	ordinaryBoundaryDrain(t, b)
	ordinaryBoundaryCapture(t, raw)
	ordinaryBoundaryCapture(t, set)
	rawView, setView = ownershipLiveRead(t, raw), ownershipLiveRead(t, set)
	ownershipLiveAbsent(t, rawView, subject)
	ownershipLiveWithdrawals(t, rawView, subject, 1)
	ownershipLiveAbsent(t, setView, subject)
	ownershipLiveWithdrawals(t, setView, subject, 1)
	ordinarySelectedMEDMarker(t, rawView.routes[fence])
	_, _, _, fenceHasMED := attribute.AttrFind(rawView.routes[fence], attribute.AttrMED)
	require.False(t, fenceHasMED)
	ordinarySelectedMEDWire(t, setView.routes[fence], 7)
	require.Positive(t, transport.callsFor(peers[3].Settings().Address.String()),
		"the separate recipient's declared MED Set must execute")
	ownershipLiveEstablished(t, []*lowLivePeer{a, b, raw, set})
}

func ordinaryEBGPMEDAttrs(hop netip.Addr, asn, med uint32) []byte {
	attrs := synthesizedNextHopAttrs(hop, asn)
	return append(attrs, makeAttr(0x80, byte(attribute.AttrMED), binary.BigEndian.AppendUint32(nil, med))...)
}

// ordinarySelectedMEDPeers configures the existing policy seam before start.
// The only inserted routes are independent terminal interior next-hop hosts;
// neither selected BGP destination is injected into the RIB by the fixture.
func ordinarySelectedMEDPeers(t *testing.T, transport *answerTransport) []*Peer {
	t.Helper()
	settings := []*PeerSettings{
		acceptingLiveSettings(t, "192.0.2.211", 65000, 65010),
		acceptingLiveSettings(t, "192.0.2.212", 65000, 65010),
		acceptingLiveSettings(t, "192.0.2.213", 65000, 65000),
		acceptingLiveSettings(t, "192.0.2.214", 65000, 65000),
	}
	return startOrdinarySelectedMEDPeers(t, transport, settings)
}

func startOrdinarySelectedMEDPeers(t *testing.T, transport *answerTransport, settings []*PeerSettings) []*Peer {
	t.Helper()
	settings[2].ExportPolicy = answerPolicy(t, routepolicy.FinalAccept, "remove-med")
	settings[3].ExportPolicy = answerPolicy(t, routepolicy.FinalAccept, "set-med")
	peers := lowLiveStart(t, lowLiveOptions{withoutRS: true,
		api: func(adapter *reactorAPIAdapter) plugin.ReactorLifecycle {
			adapter.r.filterTransportSeam = transport
			return adapter
		}}, settings...)
	loc := locrib.Default()
	require.NotNil(t, loc)
	protocol := redistevents.RegisterProtocol("ordinary-selected-med-test")
	for _, setting := range settings[:2] {
		host := netip.PrefixFrom(setting.Address, 32)
		loc.Insert(family.IPv4Unicast, host, locrib.Path{Source: protocol, Metric: 7})
		t.Cleanup(func() { loc.Remove(family.IPv4Unicast, host, protocol, 0) })
		require.True(t, igpcost.Resolve(loc, host.Addr()).Resolved)
	}
	for _, peer := range peers {
		require.False(t, peer.Settings().RSClient)
		require.False(t, peer.Settings().RouteReflectorClient)
	}
	return peers
}

// ordinarySelectedMEDPolicy is safe for concurrent filter calls. Its two
// atomic counters record successful raw replies, not attempted filter calls;
// all reply bytes belong to the current synchronous answer.
type ordinarySelectedMEDPolicy struct {
	initial atomic.Int64
	changed atomic.Int64
}

func (p *ordinarySelectedMEDPolicy) answer(input rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
	if input.Filter == "set-med" {
		return answerSet("med", "7")
	}
	if input.Filter != "remove-med" {
		return rpc.FilterUpdateOutput{Action: rpc.FilterReject, Error: "unknown ordinary MED fixture filter"}
	}
	var mods filterapi.ModAccumulator
	mods.Op(uint8(attribute.AttrMED), filterapi.AttrModSuppress, nil)
	mods.Op(uint8(attribute.AttrCommunity), filterapi.AttrModSet, ordinarySelectedMEDCommunity())
	rewritten, _, fail := buildModifiedPayload(input.Raw, &mods, attrModHandlersWithDefaults(), nil, nil)
	if fail.failed() {
		return rpc.FilterUpdateOutput{Action: rpc.FilterReject,
			Error: fmt.Sprintf("ordinary MED raw fixture rebuild failed: %v", fail)}
	}
	if rewritten == nil {
		return rpc.FilterUpdateOutput{Action: rpc.FilterReject, Error: "ordinary MED raw fixture produced no rewrite"}
	}
	if payloadMED(rewritten).present {
		return rpc.FilterUpdateOutput{Action: rpc.FilterReject, Error: "ordinary MED raw fixture retained MED"}
	}
	received := payloadMED(input.Raw)
	if received.present {
		if len(received.raw) != 4 {
			return rpc.FilterUpdateOutput{Action: rpc.FilterReject, Error: "ordinary MED raw fixture received malformed MED"}
		}
		metric := binary.BigEndian.Uint32(received.raw)
		if metric == 20 {
			p.initial.Add(1)
		}
		if metric == 10 {
			p.changed.Add(1)
		}
	}
	return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rewritten}
}

// ordinarySelectedMEDStored waits for the exact received MED, not just a
// nonempty candidate that could still carry its preceding revision.
func ordinarySelectedMEDStored(t *testing.T, source *lowLivePeer, prefix netip.Prefix, metric uint32) {
	t.Helper()
	address := prefix.Addr().As4()
	lowEventually(t, func() bool {
		attrs := extendedRecoveryStoredAttributes(source.peer.Settings().Address.String(), address[:3])
		_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMED)
		if !found {
			return false
		}
		if len(value) != 4 {
			return false
		}
		return binary.BigEndian.Uint32(value) == metric
	}, "ordinary received MED stored for "+prefix.String())
}

func ordinarySelectedMEDWire(t *testing.T, attrs []byte, metric uint32) {
	t.Helper()
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMED)
	require.True(t, found, "the actual ordinary recipient UPDATE must contain MED")
	require.Len(t, value, 4)
	require.Equal(t, metric, binary.BigEndian.Uint32(value))
}

func ordinarySelectedMEDMarker(t *testing.T, attrs []byte) {
	t.Helper()
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrCommunity)
	require.True(t, found, "the raw egress replacement's unrelated marker must survive")
	require.Equal(t, ordinarySelectedMEDCommunity(), value)
}

func ordinarySelectedMEDCommunity() []byte { return []byte{0xfd, 0xe8, 0x12, 0x34} }
