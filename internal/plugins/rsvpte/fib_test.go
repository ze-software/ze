// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- forwarding acceptance gates signaling
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/server"
	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

func TestForwardingAcceptancePrecedesReservation(t *testing.T) {
	for _, role := range []string{"ingress", "transit", "egress"} {
		t.Run(role, func(t *testing.T) {
			router := "10.0.0.2"
			if role == "ingress" {
				router = "10.0.0.1"
			}
			if role == "egress" {
				router = "10.0.0.9"
			}
			e, transport, accepted := testEngine(t, router, nil)
			psb := &pathStateBlock{
				Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 7, ExtTunnelID: 1},
				SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
				ERO:            []eroHop{{Address: netip.MustParsePrefix("10.0.0.2/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}},
				LabelRequest:   labelRequest{L3PID: 0x0800},
				RefreshPeriod:  DefaultRefreshPeriod,
			}
			if role == "egress" {
				psb.ERO = psb.ERO[len(psb.ERO)-1:]
			}
			path := buildPath(psb, psb.SenderTemplate.SenderAddr, defaultIPTTL)
			parsed, err := DecodeMessage(path)
			require.NoError(t, err)
			key := keyFromMessage(parsed)
			switch role {
			case "ingress":
				lsp, _ := e.table.GetOrCreate(key)
				lsp.Role, lsp.PSB = RoleIngress, psb
				lsp.setState(LSPStatePathSent)
			case "transit":
				e.handlePacket(Packet{Src: psb.SenderTemplate.SenderAddr, Payload: path})
			}
			// A transport delivery or published event is not native FIB acceptance.
			e.fib = &busFIB{}
			raw := buildResv(&resvStateBlock{Session: psb.Session, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit},
				psb.SenderTemplate, DefaultRefreshPeriod, psb.Session.TunnelEndpoint)
			packet := Packet{Src: psb.Session.TunnelEndpoint, Payload: raw}
			if role == "egress" {
				packet = Packet{Src: psb.SenderTemplate.SenderAddr, Payload: path}
			}
			e.handlePacket(packet)
			if lsp, ok := e.table.Get(key); ok {
				assert.NotEqual(t, LSPStateUp, lsp.State)
			}
			_, _, sent := transport.lastByType(MsgTypeResv)
			assert.False(t, sent, "an uninstalled label must not be advertised")
			e.fib = accepted
			e.handlePacket(packet)
			lsp, ok := e.table.Get(key)
			require.True(t, ok)
			assert.Equal(t, LSPStateUp, lsp.State)
			if role != "ingress" {
				_, _, sent = transport.lastByType(MsgTypeResv)
				assert.True(t, sent, "native acceptance permits the reservation")
			}
		})
	}
}

// TestSwapForwardingRequest retains the ordinary label-operation assertions.
// It makes no claim about native acceptance or the RFC 3209 Path MTU algorithm.
func TestSwapForwardingRequest(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	var entries []mplsfibevents.Entry
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, func(batch *mplsfibevents.EntryBatch) {
		entries = append(entries, batch.Entries...)
	}))
	fib := &busFIB{bus: bus}
	nextHop := netip.MustParseAddr("10.0.0.3")
	// Capturing the request does not acknowledge installation on an owner's behalf.
	require.Error(t, fib.programSwap(1001, 18000, nextHop, 0))
	require.Len(t, entries, 1)
	entry := entries[0]
	assert.Equal(t, mplsfibevents.ActionAdd, entry.Action)
	assert.Equal(t, mplsfibevents.OpSwap, entry.Op)
	assert.Equal(t, uint32(1001), entry.InLabel)
	assert.Equal(t, []uint32{18000}, entry.OutLabels)
	assert.Equal(t, nextHop, entry.NextHop)
	assert.Equal(t, mplsSourceRSVPTE, entry.Source)
}

// VALIDATES: a rejected transit-to-egress conversion cannot advertise the old
// swap label as an accepted pop when the same PATH arrives again.
func TestEgressConversionRetriesForwardingAcceptance(t *testing.T) {
	e, transport, accepted := testEngine(t, "10.0.0.2", nil)
	psb := &pathStateBlock{
		Session: sessionIPv4{
			TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 7, ExtTunnelID: 0x0a000001,
		},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		ERO: []eroHop{
			{Address: netip.MustParsePrefix("10.0.0.2/32")},
			{Address: netip.MustParsePrefix("10.0.0.9/32")},
		},
		LabelRequest:  labelRequest{L3PID: 0x0800},
		RefreshPeriod: DefaultRefreshPeriod,
	}
	path := Packet{Src: psb.SenderTemplate.SenderAddr,
		Payload: buildPath(psb, psb.SenderTemplate.SenderAddr, defaultIPTTL)}
	e.handlePacket(path)
	reservation := &resvStateBlock{
		Session: psb.Session, FlowSpec: psb.SenderTSpec,
		Label: labelObject{Label: 18000}, Style: StyleSharedExplicit,
	}
	e.handlePacket(Packet{Src: psb.Session.TunnelEndpoint,
		Payload: buildResv(reservation, psb.SenderTemplate, DefaultRefreshPeriod, psb.Session.TunnelEndpoint)})
	require.Len(t, accepted.swapped, 1, "the original transit forwarding was accepted")
	before := transport.countByType(MsgTypeResv)
	require.Equal(t, 1, before)

	transport.localAddresses = append(transport.localAddresses, psb.Session.TunnelEndpoint)
	e.fib = &busFIB{}
	for range 2 {
		e.handlePacket(path)
		assert.Equal(t, before, transport.countByType(MsgTypeResv),
			"a failed pop installation must not advertise a reservation on retry")
	}

	e.fib = accepted
	e.handlePacket(path)
	require.Len(t, accepted.popped, 1, "conversion still requires native pop acceptance")
	assert.Equal(t, before+1, transport.countByType(MsgTypeResv))
}
