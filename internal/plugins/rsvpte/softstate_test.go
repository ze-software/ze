// Design: plan/spec-mpls-3-rsvp-te.md -- RSVP-TE soft-state expiry (F8) test
package rsvpte

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// VALIDATES: F8 -- the refresh loop must NOT locally re-stamp a transit/egress
// PSB. That state is refreshed only by the incoming PATH it relays (RFC 2205
// Section 3.4); stamping it locally would stop the cleanup loop from ever
// expiring the LSP after the upstream stops refreshing, leaking the reservation
// and FIB state.
func TestRefreshDoesNotStampEgressPSB(t *testing.T) {
	// RFC requirement: RFC3209-2.5-1 negative -- soft-state depends on refresh: an LSP whose PATH refresh stops (its PSB is not re-stamped) becomes past its deadline and eligible for cleanup expiry (register.go:928-953), so state is not kept alive without refresh.
	e, _, _ := testEngine(t, "10.0.0.9", nil)
	path := buildPath(egressTestPSB(), netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	require.Equal(t, RoleEgress, lsp.Role)

	old := time.Now().Add(-time.Hour)
	lsp.mu.Lock()
	lsp.PSB.LastRefresh = old
	lsp.mu.Unlock()

	refreshPaths(slogutil.DiscardLogger(), e.table, e)

	lsp.mu.Lock()
	got := lsp.PSB.LastRefresh
	lsp.mu.Unlock()
	assert.True(t, got.Equal(old), "egress PSB must not be re-stamped by the refresh loop")

	// And it is therefore expirable: with a refresh multiplier of 1 the hour-old
	// PSB is past its deadline.
	expired := e.table.expiredPSBs(time.Now(), 1)
	assert.Contains(t, expired, key, "stale egress LSP must be eligible for cleanup")
}

// VALIDATES: a refresh tick re-sends soft-state signaling in BOTH directions -- an
// ingress LSP re-sends its PATH downstream and an egress/transit LSP re-sends its
// RESV upstream (RFC 2205 Section 3.7) -- so the reservation is maintained.
func TestRefreshResendsPathAndResv(t *testing.T) {
	// RFC requirement: RFC3209-2.5-1 positive -- refreshPaths re-sends a PATH for an ingress LSP and a RESV for an egress/transit LSP on a refresh tick (register.go:906-920), so both PATH and RESV state are periodically refreshed.
	e, ft, _ := testEngine(t, "10.0.0.5", nil)

	// Ingress LSP: a refresh must re-send its PATH downstream.
	ingressKey := lspKey{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, SenderAddr: netip.MustParseAddr("10.0.0.5"), LSPID: 1}
	ing, _ := e.table.GetOrCreate(ingressKey)
	ing.Role = RoleIngress
	ing.PSB = &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: ingressKey.TunnelEndpoint, TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: ingressKey.SenderAddr, LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
	ing.setState(LSPStateUp)

	// Egress LSP: a refresh must re-send its RESV upstream toward the PHOP.
	egressKey := lspKey{TunnelEndpoint: netip.MustParseAddr("10.0.0.5"), TunnelID: 2, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1}
	eg, _ := e.table.GetOrCreate(egressKey)
	eg.Role = RoleEgress
	eg.PrevHop = netip.MustParseAddr("10.0.0.1")
	eg.PSB = &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: egressKey.TunnelEndpoint, TunnelID: 2},
		SenderTemplate: senderTemplateIPv4{SenderAddr: egressKey.SenderAddr, LSPID: 1},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
	eg.RSB = &resvStateBlock{
		Session:  sessionIPv4{TunnelEndpoint: egressKey.TunnelEndpoint, TunnelID: 2},
		FlowSpec: FlowSpec{TokenRate: 1e8},
		Label:    labelObject{Label: 1000},
		Style:    StyleSharedExplicit,
	}
	eg.setState(LSPStateUp)

	refreshPaths(slogutil.DiscardLogger(), e.table, e)

	path, _, gotPath := ft.lastByType(MsgTypePath)
	require.True(t, gotPath, "refresh re-sends a PATH for the ingress LSP")
	assert.Equal(t, ingressKey.SenderAddr, path.SenderTemplate.SenderAddr, "the refreshed PATH is the ingress LSP's")

	resv, resvDst, gotResv := ft.lastByType(MsgTypeResv)
	require.True(t, gotResv, "refresh re-sends a RESV for the egress LSP")
	assert.Equal(t, eg.PrevHop, resvDst, "the refreshed RESV goes upstream to the PHOP")
	assert.Equal(t, uint32(1000), resv.Label.Label, "the refreshed RESV carries the egress label")
}
