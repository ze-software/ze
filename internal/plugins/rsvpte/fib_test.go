// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- forwarding acceptance gates signaling
package rsvpte

import (
	"errors"
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

// switchableLabelWriter accepts or refuses batches addressed to held based on
// liveErr (for live Apply) and replayErr (for ApplyReplay).
type switchableLabelWriter struct {
	held      uint64
	liveErr   error
	replayErr error
	batches   []mplsfibevents.EntryBatch
}

func (w *switchableLabelWriter) handle(batch *mplsfibevents.EntryBatch) {
	if batch == nil {
		return
	}
	if !batch.Addresses(w.held) {
		return
	}
	w.batches = append(w.batches, mplsfibevents.EntryBatch{
		Entries:  slices.Clone(batch.Entries),
		ReplayID: batch.ReplayID,
	})
	if replay.IsReplay(batch.ReplayID) {
		batch.Acknowledge(w.replayErr)
		return
	}
	batch.Acknowledge(w.liveErr)
}

// VALIDATES: spec-fib-dummy AC-18 -- a live refusal from mplsfib.Apply is
// returned to the engine AND recorded so rsvp-te-label-entries reports it
// under doctor-fib-mpls-no-writer with the writer's cause and updated fix.
// PREVENTS: RSVP-TE silently hiding a writer's live refusal from `show doctor`.
func TestRSVPTELiveRefusalIsRecorded(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	cause := errors.New("fib dummy programs no label, so this entry is not installed; delete fib dummy, or remove the label protocol")
	writer := &switchableLabelWriter{held: mplsfibevents.NewReplayToken(), liveErr: cause}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, writer.handle))
	fib := &busFIB{bus: bus}

	err = fib.programSwap(1001, 18001, netip.MustParseAddr("10.0.0.3"), 0)
	require.ErrorIs(t, err, cause)

	findings := fib.labelDoctorFindings()
	require.Len(t, findings, 1)
	assert.Equal(t, mplsfibevents.CodeDoctorNoWriter, findings[0].Code)
	assert.Equal(t, "error", findings[0].Severity)
	assert.Contains(t, findings[0].Message, "swap of in-label 1001")
	assert.Contains(t, findings[0].Message, cause.Error())
	assert.Contains(t, findings[0].Fix, "next refreshed")
	assert.Contains(t, findings[0].Fix, "next opens")
}

// VALIDATES: spec-fib-dummy AC-18, R-11 -- an accepted apply of the same slot
// clears a previously recorded live refusal.
// PREVENTS: stale live refusals lingering after recovery.
func TestRSVPTEAcceptedApplyClearsLiveRefusal(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	writer := &switchableLabelWriter{held: mplsfibevents.NewReplayToken(), liveErr: errors.New("writer refused")}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, writer.handle))
	fib := &busFIB{bus: bus}

	nh := netip.MustParseAddr("10.0.0.3")
	require.Error(t, fib.programSwap(1001, 18001, nh, 0))
	require.Len(t, fib.labelDoctorFindings(), 1)

	writer.liveErr = nil
	require.NoError(t, fib.programSwap(1001, 18001, nh, 0))
	assert.Empty(t, fib.labelDoctorFindings())
}

// VALIDATES: spec-fib-dummy AC-18 -- a successful replay of installed entries
// clears replay refusals whose recorded entry equals a replayed entry, while
// preserving a live refusal of an entry that is not installed.
// PREVENTS: busFIB.replay calling Reset() and wiping unrelated live refusals.
func TestRSVPTEReplaySuccessKeepsLiveRefusal(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	writer := &switchableLabelWriter{held: mplsfibevents.NewReplayToken()}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, writer.handle))
	fib := &busFIB{bus: bus}

	nh := netip.MustParseAddr("10.0.0.3")
	// Install swap 1001 cleanly.
	require.NoError(t, fib.programSwap(1001, 18001, nh, 0))

	// Refuse a replay of installed swap 1001.
	writer.replayErr = errors.New("replay refused")
	require.Error(t, fib.replay(&replay.Request{ReplayID: writer.held}))
	require.Len(t, fib.labelDoctorFindings(), 1)

	// Record a live refusal of uninstalled swap 1002.
	writer.liveErr = errors.New("live refused")
	require.Error(t, fib.programSwap(1002, 18002, nh, 0))
	require.Len(t, fib.labelDoctorFindings(), 2)

	// A successful replay re-sends swap 1001 (clearing its replay refusal) but
	// must keep the live refusal for uninstalled swap 1002.
	writer.replayErr = nil
	require.NoError(t, fib.replay(&replay.Request{ReplayID: writer.held}))
	findings := fib.labelDoctorFindings()
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Message, "swap of in-label 1002")
	assert.Contains(t, findings[0].Message, "live refused")
}

// VALIDATES: spec-fib-dummy AC-18, R-11 -- a refused replacement (programBackup)
// sharing an installed swap's slot survives a successful replay of the old
// installed swap because replay clears only when the recorded entry equals the
// replayed one.
// PREVENTS: slot-keyed clearing erasing a refused backup swap during replay.
func TestRSVPTEReplaySuccessKeepsRefusedReplacement(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	writer := &switchableLabelWriter{held: mplsfibevents.NewReplayToken()}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, writer.handle))
	fib := &busFIB{bus: bus}

	nh := netip.MustParseAddr("10.0.0.3")
	require.NoError(t, fib.programSwap(1001, 18001, nh, 0))

	writer.liveErr = errors.New("backup stack rejected")
	require.Error(t, fib.programBackup(1001, []uint32{19000, 18001}, nh, 0))
	require.Len(t, fib.labelDoctorFindings(), 1)

	// Replay re-sends the old accepted swap [18001]; the refused backup
	// [19000, 18001] on the same slot differs and MUST stay reported.
	require.NoError(t, fib.replay(&replay.Request{ReplayID: writer.held}))
	findings := fib.labelDoctorFindings()
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Message, "swap of in-label 1001")
	assert.Contains(t, findings[0].Message, "backup stack rejected")
}

// VALIDATES: spec-fib-dummy AC-18, AC-23, R-11 -- an installed push whose
// removal the writer refuses leaves installed (never replayed) and keeps its
// refused-removal finding across a successful replay of other entries.
// PREVENTS: replay re-installing a withdrawn push or erasing its removal error.
func TestRSVPTEReplaySuccessKeepsRefusedRemoval(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	writer := &switchableLabelWriter{held: mplsfibevents.NewReplayToken()}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, writer.handle))
	fib := &busFIB{bus: bus}

	nh := netip.MustParseAddr("10.0.0.3")
	fec := netip.MustParsePrefix("10.0.0.9/32")
	require.NoError(t, fib.programPush(fec, []uint32{18000}, nh, 0, 0))
	require.NoError(t, fib.programSwap(1001, 18001, nh, 0))

	writer.liveErr = errors.New("remove refused")
	require.Error(t, fib.removePush(fec, 0))

	before := len(writer.batches)
	require.NoError(t, fib.replay(&replay.Request{ReplayID: writer.held}))
	require.Len(t, writer.batches, before+1)
	replayed := writer.batches[len(writer.batches)-1]
	require.Len(t, replayed.Entries, 1, "withdrawn push must not be replayed")
	assert.Equal(t, uint32(1001), replayed.Entries[0].InLabel)

	findings := fib.labelDoctorFindings()
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Message, "may still hold")
	assert.Contains(t, findings[0].Message, "push for FEC 10.0.0.9/32")
	assert.Contains(t, findings[0].Message, "remove refused")
}

// VALIDATES: spec-fib-dummy AC-23 -- an accepted swap and push followed by a
// refused removeSwap leaves only the push in the replay batch, once.
// PREVENTS: a refused removal keeping a withdrawn swap in busFIB.installed.
func TestRSVPTEReplaySkipsWithdrawnEntry(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	writer := &switchableLabelWriter{held: mplsfibevents.NewReplayToken()}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, writer.handle))
	fib := &busFIB{bus: bus}

	nh := netip.MustParseAddr("10.0.0.3")
	fec := netip.MustParsePrefix("10.0.0.9/32")
	require.NoError(t, fib.programSwap(1001, 18001, nh, 0))
	require.NoError(t, fib.programPush(fec, []uint32{18000}, nh, 0, 0))

	writer.liveErr = errors.New("fib dummy programs no label")
	require.Error(t, fib.removeSwap(1001))

	before := len(writer.batches)
	require.NoError(t, fib.replay(&replay.Request{ReplayID: writer.held}))
	require.Len(t, writer.batches, before+1)
	replayed := writer.batches[len(writer.batches)-1]
	require.Len(t, replayed.Entries, 1, "only the push remains wanted")
	assert.Equal(t, mplsfibevents.OpPush, replayed.Entries[0].Op)
	assert.Equal(t, fec, replayed.Entries[0].FEC)
}

// VALIDATES: spec-fib-dummy AC-23, A-13 -- a transit Resv refused by the label
// writer is rejected with no entry in busFIB.installed, and the downstream
// neighbor's next Resv refresh installs the swap once the writer accepts.
// PREVENTS: a transit RSVP-TE reservation staying permanently down after a
// reload replaces fib-dummy with a real writer.
func TestRSVPTERefreshReappliesRefusedReservation(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	writer := &switchableLabelWriter{
		held:    mplsfibevents.NewReplayToken(),
		liveErr: errors.New("fib dummy programs no label"),
	}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, writer.handle))

	e, transport, _ := testEngine(t, "10.0.0.2", nil)
	fib := &busFIB{bus: bus}
	e.fib = fib

	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 11, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		ERO:            []eroHop{{Address: netip.MustParsePrefix("10.0.0.2/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
	pathBytes := buildPath(psb, psb.SenderTemplate.SenderAddr, defaultIPTTL)
	e.handlePacket(Packet{Src: psb.SenderTemplate.SenderAddr, Payload: pathBytes})

	parsed, err := DecodeMessage(pathBytes)
	require.NoError(t, err)
	key := keyFromMessage(parsed)

	resv := buildResv(&resvStateBlock{
		Session:  psb.Session,
		FlowSpec: psb.SenderTSpec,
		Label:    labelObject{Label: 18000},
		Style:    StyleSharedExplicit,
	}, psb.SenderTemplate, DefaultRefreshPeriod, psb.Session.TunnelEndpoint)
	resvPkt := Packet{Src: psb.Session.TunnelEndpoint, Payload: resv}

	// First Resv arrives while the writer refuses: reservation is rejected and
	// nothing enters installed.
	e.handlePacket(resvPkt)
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	assert.NotEqual(t, LSPStateUp, lsp.State)
	assert.False(t, lsp.Reserved)
	assert.Empty(t, fib.installed)
	assert.Equal(t, 0, transport.countByType(MsgTypeResv))
	require.Len(t, fib.labelDoctorFindings(), 1)

	// Writer starts accepting (e.g., fib-dummy stopped, fib-kernel running);
	// downstream's next Resv refresh programs the swap and brings the LSP up.
	writer.liveErr = nil
	e.handlePacket(resvPkt)
	lsp, ok = e.table.Get(key)
	require.True(t, ok)
	assert.Equal(t, LSPStateUp, lsp.State)
	assert.True(t, lsp.Reserved)
	assert.Len(t, fib.installed, 1)
	assert.Equal(t, 1, transport.countByType(MsgTypeResv))
	assert.Empty(t, fib.labelDoctorFindings())
}

// VALIDATES: spec-fib-dummy AC-23, A-13 -- an egress Path whose programPop the
// writer refuses sends PathErr upstream, removes the newly created LSP from
// e.table, leaves nothing in busFIB.installed, and the upstream neighbor's
// next Path refresh installs the pop and brings the LSP up once the writer
// accepts.
// PREVENTS: an egress RSVP-TE LSP staying permanently down after fib-dummy
// stops.
func TestRSVPTEPathRefreshReappliesRefusedEgressPop(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	writer := &switchableLabelWriter{
		held:    mplsfibevents.NewReplayToken(),
		liveErr: errors.New("fib dummy programs no label"),
	}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, writer.handle))

	e, transport, _ := testEngine(t, "10.0.0.9", nil)
	fib := &busFIB{bus: bus}
	e.fib = fib

	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 12, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		ERO:            []eroHop{{Address: netip.MustParsePrefix("10.0.0.9/32")}},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
	pathBytes := buildPath(psb, psb.SenderTemplate.SenderAddr, defaultIPTTL)
	parsed, err := DecodeMessage(pathBytes)
	require.NoError(t, err)
	key := keyFromMessage(parsed)
	pathPkt := Packet{Src: psb.SenderTemplate.SenderAddr, Payload: pathBytes}

	// First Path arrives while writer refuses: PathErr sent upstream, no LSP
	// kept in table, nothing in installed.
	e.handlePacket(pathPkt)
	_, exists := e.table.Get(key)
	assert.False(t, exists, "refused egress Path must not leave an LSP in the table")
	assert.Empty(t, fib.installed)
	assert.Equal(t, 1, transport.countByType(MsgTypePathErr))
	assert.Equal(t, 0, transport.countByType(MsgTypeResv))
	require.Len(t, fib.labelDoctorFindings(), 1)

	// Writer starts accepting; upstream's next Path refresh installs the pop
	// and brings the egress LSP up.
	writer.liveErr = nil
	e.handlePacket(pathPkt)
	lsp, exists := e.table.Get(key)
	require.True(t, exists)
	assert.Equal(t, LSPStateUp, lsp.State)
	assert.Len(t, fib.installed, 1)
	assert.Equal(t, 1, transport.countByType(MsgTypeResv))
	assert.Empty(t, fib.labelDoctorFindings())
}
