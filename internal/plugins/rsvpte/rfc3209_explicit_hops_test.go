// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 3209 Sections
// 4.3.3.1 and 4.3.4.2: strict-hop adjacency and the subobjects a transit may
// place ahead of the next abstract node, driven through a received PATH.
// Related: routing.go -- resolveExplicitPath, the producer under test.
//
// VALIDATES: a strict hop is reached only through its own or the preceding
// abstract node; a loose hop is expanded with the adjacent next hop.
// PREVENTS: a PATH forwarded toward a strict node through an outside node.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// explicitHopIngress is the previous hop of every PATH in this file.
var explicitHopIngress = netip.MustParseAddr("10.0.0.1")

// explicitHopTransit returns a transit engine with router ID routerID whose
// native route to the explicit target resolves to nextHop.
func explicitHopTransit(t *testing.T, routerID string, target netip.Prefix, nextHop netip.Addr) (*engine, *fakeTransport) {
	t.Helper()
	e, ft, _ := testEngine(t, routerID, nil)
	ft.routes = map[netip.Prefix]RouteInfo{target: {NextHop: nextHop, Lookup: nextHop, MTU: 1500}}
	return e, ft
}

// explicitHopPath returns the PATH the ingress sends, for a tunnel to endpoint
// along ero.
func explicitHopPath(endpoint netip.Addr, ero []eroHop) []byte {
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: endpoint, TunnelID: 11, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: explicitHopIngress, LSPID: 1},
		ERO:            ero,
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
	return buildPath(psb, explicitHopIngress, defaultIPTTL)
}

// requireBadStrictNode checks the transit answered with a PathErr Routing
// Problem / Bad strict node to the ingress and sent no PATH.
func requireBadStrictNode(t *testing.T, ft *fakeTransport) {
	t.Helper()
	assert.Zero(t, ft.countByType(MsgTypePath), "no PATH leaves toward the strict node")
	pathErr, dst, sent := ft.lastByType(MsgTypePathErr)
	require.True(t, sent, "the strict transition is refused with a PathErr")
	assert.Equal(t, explicitHopIngress, dst)
	assert.Equal(t, ErrCodeRoutingProblem, pathErr.ErrorSpec.ErrorCode)
	assert.Equal(t, ErrValueBadStrictNode, pathErr.ErrorSpec.ErrorValue)
}

// TestRFC3209StrictHopReachedDirectly sends a transit a PATH whose next
// subobject is a strict node the native route reaches directly.
//
// RFC requirement: RFC3209-4.3.3.1-1 positive -- a transit (10.0.0.5) whose native route to the strict node 10.0.0.9/32 has that node itself as next hop forwards the PATH on the wire straight to 10.0.0.9, with an ERO that names only the strict node, and sends no PathErr; resolveExplicitPath in routing.go.
// RFC 3209 Section 4.3.3.1: "The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node."
// The quote is read from rfc/full/rfc3209.txt.
func TestRFC3209StrictHopReachedDirectly(t *testing.T) {
	strict := netip.MustParsePrefix("10.0.0.9/32")
	e, ft := explicitHopTransit(t, "10.0.0.5", strict, strict.Addr())

	e.handlePacket(Packet{Src: explicitHopIngress, Payload: explicitHopPath(strict.Addr(),
		[]eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}, {Address: strict}})})

	path, dst, sent := ft.lastByType(MsgTypePath)
	require.True(t, sent, "the PATH is forwarded")
	assert.Equal(t, strict.Addr(), dst, "sent to the strict node itself")
	assert.Equal(t, []eroHop{{Address: strict}}, path.ERO)
	assert.Zero(t, ft.countByType(MsgTypePathErr))
}

// TestRFC3209StrictHopThroughOutsideNodeRefused sends the same PATH to a
// transit whose native route to the strict node goes through 10.0.0.7, a node
// in neither the strict node nor the transit's own abstract node.
//
// RFC requirement: RFC3209-4.3.3.1-1 negative -- a transit (10.0.0.5/32) whose native route to the strict node 10.0.0.9/32 goes through 10.0.0.7, a member of neither abstract node, sends no PATH and answers the ingress with a PathErr, Routing Problem (24) / Bad strict node (2); resolveExplicitPath in routing.go.
// RFC 3209 Section 4.3.3.1: "The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node."
// The quote is read from rfc/full/rfc3209.txt.
func TestRFC3209StrictHopThroughOutsideNodeRefused(t *testing.T) {
	strict := netip.MustParsePrefix("10.0.0.9/32")
	e, ft := explicitHopTransit(t, "10.0.0.5", strict, netip.MustParseAddr("10.0.0.7"))

	e.handlePacket(Packet{Src: explicitHopIngress, Payload: explicitHopPath(strict.Addr(),
		[]eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}, {Address: strict}})})

	requireBadStrictNode(t, ft)
}

// TestRFC3209LooseHopExpandedWithNextHop is the loose contrast of the refusal
// above: the same topology with the L bit set. RFC 3209 Section 4.3.4.1 step
// 5b lets the transit follow native routing toward a loose node and replace
// the first subobject with one containing the next hop it selected.
func TestRFC3209LooseHopExpandedWithNextHop(t *testing.T) {
	loose := netip.MustParsePrefix("10.0.0.9/32")
	through := netip.MustParseAddr("10.0.0.7")
	e, ft := explicitHopTransit(t, "10.0.0.5", loose, through)

	e.handlePacket(Packet{Src: explicitHopIngress, Payload: explicitHopPath(loose.Addr(),
		[]eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}, {Loose: true, Address: loose}})})

	path, dst, sent := ft.lastByType(MsgTypePath)
	require.True(t, sent, "a loose node may be reached through other nodes")
	assert.Equal(t, through, dst)
	assert.Equal(t, []eroHop{{Address: netip.PrefixFrom(through, 32)}, {Loose: true, Address: loose}}, path.ERO,
		"the adjacent next hop is placed before the remaining loose subobject")
	assert.Zero(t, ft.countByType(MsgTypePathErr))
}

// TestRFC3209InteriorHopStaysInAbstractNode sends a transit inside the
// abstract node 10.1.0.0/16 a PATH whose next strict node, 10.9.0.9/32, the
// native route reaches through 10.1.0.2, another member of that node.
//
// RFC requirement: RFC3209-4.3.4.2-1 positive -- a transit (10.1.0.1) inside the current abstract node 10.1.0.0/16, routing toward the strict node 10.9.0.9/32 through 10.1.0.2 inside the same abstract node, forwards the PATH to 10.1.0.2 with an ERO whose every subobject ahead of 10.9.0.9/32 denotes a subset of 10.1.0.0/16; resolveExplicitPath in routing.go.
// RFC 3209 Section 4.3.4.2: "Otherwise, if the node is a member of the abstract node for the first subobject, a series of subobjects MAY be inserted before the first subobject or MAY replace the first subobject.  Each subobject in this series MUST denote an abstract node that is a subset of the current abstract node."
// The quote is read from rfc/full/rfc3209.txt.
func TestRFC3209InteriorHopStaysInAbstractNode(t *testing.T) {
	current := netip.MustParsePrefix("10.1.0.0/16")
	strict := netip.MustParsePrefix("10.9.0.9/32")
	interior := netip.MustParseAddr("10.1.0.2")
	e, ft := explicitHopTransit(t, "10.1.0.1", strict, interior)

	e.handlePacket(Packet{Src: explicitHopIngress, Payload: explicitHopPath(strict.Addr(),
		[]eroHop{{Address: current}, {Address: strict}})})

	path, dst, sent := ft.lastByType(MsgTypePath)
	require.True(t, sent, "the PATH is forwarded inside the abstract node")
	assert.Equal(t, interior, dst)
	require.GreaterOrEqual(t, len(path.ERO), 2, "the strict node is still ahead")
	last := len(path.ERO) - 1
	assert.Equal(t, eroHop{Address: strict}, path.ERO[last])
	for _, hop := range path.ERO[:last] {
		assert.True(t, hop.Address.Bits() >= current.Bits() && current.Contains(hop.Address.Addr()),
			"subobject %s is a subset of %s", hop.Address, current)
	}
	assert.Zero(t, ft.countByType(MsgTypePathErr))
}

// TestRFC3209InteriorHopLeavingAbstractNodeRefused routes the same strict
// transition through 10.2.0.2, outside 10.1.0.0/16 and outside the strict node.
//
// RFC requirement: RFC3209-4.3.4.2-1 negative -- when the native route toward the strict node 10.9.0.9/32 leaves the current abstract node 10.1.0.0/16 through 10.2.0.2, the transit places no subobject outside 10.1.0.0/16 ahead of the strict node: it sends no PATH and answers the ingress with a PathErr, Routing Problem (24) / Bad strict node (2); resolveExplicitPath in routing.go.
// RFC 3209 Section 4.3.4.2: "Otherwise, if the node is a member of the abstract node for the first subobject, a series of subobjects MAY be inserted before the first subobject or MAY replace the first subobject.  Each subobject in this series MUST denote an abstract node that is a subset of the current abstract node."
// The quote is read from rfc/full/rfc3209.txt.
func TestRFC3209InteriorHopLeavingAbstractNodeRefused(t *testing.T) {
	strict := netip.MustParsePrefix("10.9.0.9/32")
	e, ft := explicitHopTransit(t, "10.1.0.1", strict, netip.MustParseAddr("10.2.0.2"))

	e.handlePacket(Packet{Src: explicitHopIngress, Payload: explicitHopPath(strict.Addr(),
		[]eroHop{{Address: netip.MustParsePrefix("10.1.0.0/16")}, {Address: strict}})})

	requireBadStrictNode(t, ft)
}
