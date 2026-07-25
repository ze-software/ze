// Design: plan/spec-mpls-3-rsvp-te.md -- RESV refresh (AC-5) and ERO/RRO (AC-9) tests
package rsvpte

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

func (f *fakeTransport) countByType(msgType uint8) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, s := range f.sent {
		if msg, err := DecodeMessage(s.payload); err == nil && msg.Header.MsgType == msgType {
			n++
		}
	}
	return n
}

func egressTestPSB() *pathStateBlock {
	return &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
}

// VALIDATES: AC-9 -- the egress records its own address as the first RRO entry
// in the RESV it returns (RFC 3209 Section 4.4).
func TestEgressRecordsRRO(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", nil)
	path := buildPath(egressTestPSB(), netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	resv, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	require.True(t, resv.HasRRO, "egress RESV carries an RRO")
	require.NotEmpty(t, resv.RRO)
	assert.Equal(t, netip.MustParseAddr("10.0.0.9"), resv.RRO[0].Address, "egress records itself")
}

// VALIDATES: AC-9 -- the ingress head-end records the full path from the RESV's
// RRO, prepending itself, so `show rsvp-te session` can display it.
func TestIngressRecordsFullRRO(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.1", nil)
	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	lsp, _ := e.table.GetOrCreate(key)
	lsp.Role = RoleIngress
	lsp.setState(LSPStatePathSent)

	rsb := &resvStateBlock{
		Session: sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: key.TunnelID, ExtTunnelID: key.ExtTunnelID},
		Label:   labelObject{Label: 16050},
		Style:   StyleSharedExplicit,
		RRO:     []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.9")}},
	}
	filter := senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: key.LSPID}
	resv := buildResv(rsb, filter, DefaultRefreshPeriod, netip.MustParseAddr("10.0.0.9"))
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.9"), Payload: resv})

	got, _ := e.table.Get(key)
	require.NotNil(t, got.RSB)
	require.Len(t, got.RSB.RRO, 2, "ingress prepends itself to the downstream RRO")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), got.RSB.RRO[0].Address)
	assert.Equal(t, netip.MustParseAddr("10.0.0.9"), got.RSB.RRO[1].Address)
}

// VALIDATES: AC-5 -- the refresh loop re-sends a RESV upstream for an egress LSP
// so the reservation soft-state does not expire when no fresh PATH arrives.
func TestRefreshResendsResvForEgress(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", nil)
	path := buildPath(egressTestPSB(), netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	before := ft.countByType(MsgTypeResv)
	refreshPaths(slogutil.DiscardLogger(), e.table, e)
	after := ft.countByType(MsgTypeResv)
	assert.Greater(t, after, before, "refresh re-sends a RESV for the egress LSP")

	_, dst, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), dst, "RESV refresh goes upstream to PrevHop")
}

// VALIDATES: AC-9 -- show rsvp-te session reports the configured ERO and the
// recorded RRO for an LSP.
func TestShowSessionsIncludesEROAndRRO(t *testing.T) {
	table := newLSPTable()
	key := lspKey{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1}
	lsp, _ := table.GetOrCreate(key)
	lsp.Role = RoleIngress
	lsp.setState(LSPStateUp)
	lsp.PSB = &pathStateBlock{ERO: []eroHop{{Address: netip.MustParsePrefix("10.0.0.2/32")}}}
	lsp.RSB = &resvStateBlock{RRO: []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.1")}}}

	raw, err := json.Marshal(showSessions(table))
	require.NoError(t, err)
	s := string(raw)
	assert.Contains(t, s, `"ero":["10.0.0.2/32 strict"]`)
	assert.Contains(t, s, `"rro":["10.0.0.1"]`)
}
