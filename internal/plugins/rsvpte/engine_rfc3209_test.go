// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 3209 signaling proofs (fake transport)
// Related: engine_test.go -- fake transport and FIB, testEngine
//
// Each test drives the engine through handlePacket with a PATH or RESV built
// by the real encoders and asserts what the engine sends or programs.
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc3209PathPSB is a PATH from ingress 10.0.0.1 to egress 10.0.0.9 whose
// LABEL_REQUEST names IPv6 (0x86DD) so a copied L3PID is told from the 0x0800
// default every other test uses.
func rfc3209PathPSB(ero []eroHop) *pathStateBlock {
	return &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 3, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e6, TokenBucket: 1e6, PeakRate: 1e6},
		LabelRequest:   labelRequest{L3PID: 0x86DD},
		RefreshPeriod:  DefaultRefreshPeriod,
		ERO:            ero,
	}
}

// RFC requirement: RFC3209-4.2.4-1 positive — the egress that accepts a PATH carrying a LABEL_REQUEST answers it with a RESV that carries a LABEL object holding the label it allocated for that LSP.
func TestRFC3209EgressResvCarriesLabel(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", nil)
	path := buildPath(rfc3209PathPSB(nil), netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	resv, dst, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "the accepted LABEL_REQUEST is answered with a RESV")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), dst)
	require.True(t, resv.HasLabel, "the RESV carries a LABEL object")
	lsp, ok := e.table.Get(keyFromMessage(resv))
	require.True(t, ok)
	assert.Equal(t, lsp.InLabel, resv.Label.Label, "the LABEL is the label the egress allocated")
	assert.NotZero(t, resv.Label.Label)
}

// RFC requirement: RFC3209-4.2.4-3 positive — the ingress that sent a LABEL_REQUEST processes the LABEL of the answering RESV: it records the label as the LSP's out-label, programs the push and brings the LSP up.
func TestRFC3209IngressProcessesResvLabel(t *testing.T) {
	e, _, fib := testEngine(t, "10.0.0.1", nil)
	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 3,
		ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	lsp, _ := e.table.GetOrCreate(key)
	lsp.Role = RoleIngress
	lsp.setState(LSPStatePathSent)

	rsb := &resvStateBlock{
		Session: sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: key.TunnelID, ExtTunnelID: key.ExtTunnelID},
		Label:   labelObject{Label: 0x0ABCDE},
		Style:   StyleSharedExplicit,
	}
	filter := senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: key.LSPID}
	resv := buildResv(rsb, filter, DefaultRefreshPeriod, netip.MustParseAddr("10.0.0.9"))
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.9"), Payload: resv})

	got, _ := e.table.Get(key)
	assert.Equal(t, uint32(0x0ABCDE), got.OutLabel, "the received LABEL is the out-label")
	assert.Equal(t, LSPStateUp, got.State)
	require.Len(t, fib.pushed, 1, "the push is programmed with the received label")
	assert.Equal(t, netip.MustParsePrefix("10.0.0.9/32"), fib.pushed[0])
}

// RFC requirement: RFC3209-4.2.4-3 negative — a RESV that answers the LABEL_REQUEST without a LABEL object is refused: the out-label stays zero, the LSP stays path-sent and no push is programmed.
func TestRFC3209IngressRefusesResvWithoutLabel(t *testing.T) {
	e, _, fib := testEngine(t, "10.0.0.1", nil)
	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 3,
		ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	lsp, _ := e.table.GetOrCreate(key)
	lsp.Role = RoleIngress
	lsp.setState(LSPStatePathSent)

	session := sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: key.TunnelID, ExtTunnelID: key.ExtTunnelID}
	filter := senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: key.LSPID}
	raw := encodeMessage(MsgTypeResv, defaultIPTTL, []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: netip.MustParseAddr("10.0.0.9")}) },
		func(b []byte) int { return encodeTimeValues(b, timeValues{RefreshPeriod: 30000}) },
		func(b []byte) int { return encodeStyle(b, StyleSharedExplicit) },
		func(b []byte) int {
			return encodeFlowSpec(b, ClassFlowSpec, FlowSpec{TokenRate: 1e6, TokenBucket: 1e6, PeakRate: 1e6})
		},
		func(b []byte) int { return encodeSenderTemplate(b, filter) },
	})
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.9"), Payload: raw})

	got, _ := e.table.Get(key)
	assert.Zero(t, got.OutLabel, "no label was taken from a RESV that carried none")
	assert.Equal(t, LSPStatePathSent, got.State)
	assert.Empty(t, fib.pushed, "no push is programmed")
}

// RFC requirement: RFC3209-4.2.4-4 positive — a transit node relaying a PATH copies the received L3PID (0x86DD) into the LABEL_REQUEST of the PATH it forwards.
func TestRFC3209TransitCopiesL3PID(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.5", nil)
	ero := []eroHop{
		{Address: netip.MustParsePrefix("10.0.0.5/32")},
		{Address: netip.MustParsePrefix("10.0.0.9/32")},
	}
	path := buildPath(rfc3209PathPSB(ero), netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	fwd, dst, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "the transit relays the PATH")
	assert.Equal(t, netip.MustParseAddr("10.0.0.9"), dst)
	require.True(t, fwd.HasLabelRequest, "the relayed PATH carries a LABEL_REQUEST")
	assert.Equal(t, uint16(0x86DD), fwd.LabelRequest.L3PID, "the L3PID is the received one, not the local default")
}

// RFC requirement: RFC3209-4.3.4.1-1 positive — a node receiving a PATH with an ERO evaluates the first subobject: the one naming this node is consumed and the PATH is relayed to the node the second subobject names, with that subobject now first.
func TestRFC3209TransitEvaluatesFirstEROSubobject(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.5", nil)
	ero := []eroHop{
		{Address: netip.MustParsePrefix("10.0.0.5/32")},
		{Address: netip.MustParsePrefix("10.0.0.7/32")},
		{Address: netip.MustParsePrefix("10.0.0.9/32")},
	}
	path := buildPath(rfc3209PathPSB(ero), netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	fwd, dst, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "the transit relays the PATH")
	assert.Equal(t, netip.MustParseAddr("10.0.0.7"), dst, "the next hop is the second subobject")
	require.Len(t, fwd.ERO, 2)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.7/32"), fwd.ERO[0].Address)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.9/32"), fwd.ERO[1].Address)
	if _, _, sent := ft.lastByType(MsgTypePathErr); sent {
		t.Fatal("no PathErr for a well-formed ERO")
	}
}

// RFC requirement: RFC3209-4.3.4.1-1 negative — a transit PATH with no first ERO subobject to evaluate is not relayed: the node answers a PathErr with Routing Problem / Bad EXPLICIT_ROUTE object and installs no LSP.
func TestRFC3209TransitNoFirstEROSubobject(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.5", nil)
	path := buildPath(rfc3209PathPSB(nil), netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok, "a PATH with no ERO subobject is answered with a PathErr")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), dst)
	assert.Equal(t, ErrCodeRoutingProblem, perr.ErrorSpec.ErrorCode)
	assert.Equal(t, ErrValueBadEROObject, perr.ErrorSpec.ErrorValue)
	if _, _, sent := ft.lastByType(MsgTypePath); sent {
		t.Fatal("the PATH is not relayed")
	}
	assert.Zero(t, e.table.Len(), "no LSP state is installed")
}
