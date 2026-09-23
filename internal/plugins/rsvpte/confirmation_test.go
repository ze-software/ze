// Design: docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md -- reservation confirmation lifecycle
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirmationRequestForwardedOnce(t *testing.T) {
	e, ft, _ := plrEngine(t)
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	e.handlePacket(Packet{Src: psb.SenderTemplate.SenderAddr, Payload: buildPath(psb, psb.SenderTemplate.SenderAddr, 64)})
	mp := e.cfg().Bypasses[0].MergePoint
	rsb := &resvStateBlock{Session: psb.Session, FlowSpec: psb.SenderTSpec, Label: labelObject{Label: 18000},
		Style: StyleSharedExplicit, ResvConfirm: psb.Session.TunnelEndpoint}
	request := buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, mp)
	e.handlePacket(Packet{Src: mp, Payload: request})
	forwarded, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	assert.True(t, forwarded.HasResvConfirm)
	assert.Equal(t, rsb.ResvConfirm, forwarded.ResvConfirm)
	lsp, ok := e.table.Get(protectedKey())
	require.True(t, ok)
	require.NoError(t, e.sendResv(lsp))
	refresh, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	assert.False(t, refresh.HasResvConfirm, "periodic refresh must not repeat a confirmation request")

	before := ft.countByType(MsgTypeResv)
	e.handlePacket(Packet{Src: mp, Payload: request})
	assert.Equal(t, before, ft.countByType(MsgTypeResv), "an already sufficient reservation confirms locally")
	confirmed, _, ok := ft.lastByType(MsgTypeResvConf)
	require.True(t, ok)
	assert.Equal(t, rsb.ResvConfirm, confirmed.ResvConfirm)
	assert.Equal(t, psb.SenderTemplate, confirmed.FlowDescriptors[0].Filters[0].Filter)
	assert.Zero(t, confirmed.ErrorSpec.ErrorCode)
	assert.Zero(t, confirmed.ErrorSpec.ErrorValue)
	assert.Equal(t, e.cfg().RouterID, confirmed.ErrorSpec.ErrorNode)
}

func TestMergePointRestoresConfirmationIdentity(t *testing.T) {
	mp := netip.MustParseAddr("10.0.0.3")
	plr := netip.MustParseAddr("10.0.0.2")
	e, ft, _ := testEngine(t, mp.String(), nil)
	psb := protectedTransitPSB(&protectionRequest{Facility: true})
	psb.ERO = psb.ERO[1:]
	e.handlePacket(Packet{Src: psb.SenderTemplate.SenderAddr, Payload: buildPath(psb, plr, 64)})
	rsb := &resvStateBlock{Session: psb.Session, FlowSpec: psb.SenderTSpec, Label: labelObject{Label: 18000}, Style: StyleSharedExplicit}
	e.handlePacket(Packet{Src: psb.Session.TunnelEndpoint, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, psb.Session.TunnelEndpoint)})
	backup, err := backupPath(psb, mp, plr, plr, DefaultRefreshPeriod, 1500)
	require.NoError(t, err)
	e.handlePacket(Packet{Src: plr, Payload: buildPath(backup, plr, 64)})
	confirmation := &ParsedMessage{Session: psb.Session,
		ResvConfirm: psb.Session.TunnelEndpoint, Style: StyleSharedExplicit}
	descriptor := flowDescriptor{FlowSpec: psb.SenderTSpec,
		Filters: []reservationFilter{{Filter: backup.SenderTemplate}}}
	opaque := []byte{0, 8, 250, 1, 0xde, 0xad, 0xbe, 0xef}
	confirmation.ForwardObjects = [][]byte{opaque}
	raw := buildResvConf(confirmation, &descriptor, psb.SenderTemplate.SenderAddr)
	e.handlePacket(Packet{Src: plr, Payload: raw})
	sent := lastPathCarriage(t, ft, MsgTypeResvConf, psb.Session.TunnelID)
	forwarded := mustDecode(t, sent.payload)
	assert.Equal(t, psb.SenderTemplate, forwarded.FlowDescriptors[0].Filters[0].Filter)
	assert.Equal(t, psb.Session.TunnelEndpoint, sent.route.NextHop)
	assert.Zero(t, sent.route.TableID, "the merge point resumes the canonical downstream path")
	forwardedOpaque, present := objectBytes(sent.payload, opaque[2])
	require.True(t, present, "the opaque object survives identity translation")
	assert.Equal(t, opaque, forwardedOpaque, "identity translation must retain opaque forwarded objects")
	assert.Equal(t, uint8(63), forwarded.Header.TTL)
}
