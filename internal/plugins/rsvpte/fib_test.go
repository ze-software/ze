// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- forwarding acceptance gates signaling
package rsvpte

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/server"
	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
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

// replayWriter stands in for a data plane's label writer on the production
// bus: it consumes the batches addressed to the token it holds, records
// them and accepts each one.
type replayWriter struct {
	held    uint64
	batches []mplsfibevents.EntryBatch
}

func (w *replayWriter) handle(batch *mplsfibevents.EntryBatch) {
	if !batch.Addresses(w.held) {
		return
	}
	w.batches = append(w.batches, mplsfibevents.EntryBatch{Entries: slices.Clone(batch.Entries), ReplayID: batch.ReplayID})
	batch.Acknowledge(nil)
}

// TestRSVPTEReplayReappliesInstalledEntries proves RSVP-TE answers a writer's
// replay request with every entry a writer accepted and RSVP-TE has not
// removed, tagged with the request's token.
//
// VALIDATES: AC-25 -- RSVP-TE re-applies every installed entry tagged with
// the token; a removed entry is not re-applied; a writer holding no token
// (fib-kernel) does not receive the replay batch.
// METHOD: two accepting writers on the production bus, one holding a fresh
// token; busFIB installs a push, a swap, a backup stack and a pop, removes the
// swap, then answers a replay request emitted on the bus.
func TestRSVPTEReplayReappliesInstalledEntries(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	kernel := &replayWriter{}
	vpp := &replayWriter{held: mplsfibevents.NewReplayToken()}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, kernel.handle))
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, vpp.handle))
	fib := &busFIB{bus: bus}
	t.Cleanup(mplsfibevents.ReplayRequest.Subscribe(bus, func(req *replay.Request) {
		assert.NoError(t, fib.replay(req))
	}))

	nh := netip.MustParseAddr("10.0.0.3")
	fec := netip.MustParsePrefix("10.0.0.9/32")
	require.NoError(t, fib.programPush(fec, []uint32{18000}, nh, 0, 0))
	require.NoError(t, fib.programSwap(1001, 18001, nh, 0))
	require.NoError(t, fib.programBackup(1002, []uint32{19000, 18002}, nh, 0))
	require.NoError(t, fib.programPop(1003, fec, netip.Addr{}, 0))
	require.NoError(t, fib.removeSwap(1001))
	kernelBefore, vppBefore := len(kernel.batches), len(vpp.batches)

	_, err = mplsfibevents.ReplayRequest.Emit(bus, &replay.Request{ReplayID: vpp.held})
	require.NoError(t, err)

	require.Len(t, vpp.batches, vppBefore+1, "the asking writer receives one replay batch")
	assert.Len(t, kernel.batches, kernelBefore, "a writer that did not ask receives nothing")
	answer := vpp.batches[len(vpp.batches)-1]
	assert.Equal(t, vpp.held, answer.ReplayID)
	require.Len(t, answer.Entries, 3, "push, backup swap and pop; the removed swap is gone")
	byLabel := map[uint32]mplsfibevents.Entry{}
	for _, e := range answer.Entries {
		assert.Equal(t, mplsfibevents.ActionAdd, e.Action)
		assert.Equal(t, mplsSourceRSVPTE, e.Source)
		byLabel[e.InLabel] = e
	}
	assert.Equal(t, fec, byLabel[0].FEC, "the push")
	assert.Equal(t, []uint32{18000}, byLabel[0].OutLabels)
	assert.Equal(t, []uint32{19000, 18002}, byLabel[1002].OutLabels, "the backup stack keeps its order")
	assert.Equal(t, mplsfibevents.OpPop, byLabel[1003].Op)
	assert.Equal(t, fec, byLabel[1003].FEC)
	_, removed := byLabel[1001]
	assert.False(t, removed, "the removed swap is not re-applied")
}

// TestRSVPTEEgressPopCarriesFEC proves the egress pop names the session's
// tunnel endpoint as its FEC, so a writer that disposes of the payload
// itself can identify its network layer protocol.
//
// VALIDATES: AC-19 -- RSVP-TE's egress pop carries its session endpoint as
// FEC (RFC 3032 Section 2.2); the pop has no next hop.
func TestRSVPTEEgressPopCarriesFEC(t *testing.T) {
	e, _, fib := testEngine(t, "10.0.0.9", nil)
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		ERO:            []eroHop{{Address: netip.MustParsePrefix("10.0.0.9/32")}},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
	e.handlePacket(Packet{Src: psb.SenderTemplate.SenderAddr,
		Payload: buildPath(psb, psb.SenderTemplate.SenderAddr, defaultIPTTL)})

	require.Len(t, fib.popped, 1, "the egress programs one pop")
	require.Len(t, fib.popFECs, 1)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.9/32"), fib.popFECs[0], "the pop names the tunnel endpoint")
}
