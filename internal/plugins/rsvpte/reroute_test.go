// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- make-before-break reroute tests
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: AC-7 -- make-before-break signals a new LSP (new LSP_ID, SE-style)
// before tearing down the old one; the old LSP survives until the new RESV
// arrives, then is removed.
func TestEngineMakeBeforeBreak(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.1", nil)

	// Establish the original ingress LSP (LSP_ID 1) and bring it up.
	oldKey := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	old, _ := e.table.GetOrCreate(oldKey)
	old.Role = RoleIngress
	old.Bandwidth = 1e8
	old.NextHop = netip.MustParseAddr("10.0.0.5")
	old.PSB = &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: oldKey.TunnelEndpoint, TunnelID: oldKey.TunnelID, ExtTunnelID: oldKey.ExtTunnelID},
		SenderTemplate: senderTemplateIPv4{SenderAddr: oldKey.SenderAddr, LSPID: oldKey.LSPID},
		SenderTSpec:    FlowSpec{TokenRate: 1e8},
	}
	old.setState(LSPStateUp)

	// Reroute along a new explicit path.
	newERO := []eroHop{{Address: netip.MustParsePrefix("10.0.0.6/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}}
	newKey, ok := e.reroute(oldKey, newERO)
	require.True(t, ok)
	assert.Equal(t, uint16(2), newKey.LSPID, "new LSP uses the next LSP_ID")
	// RFC requirement: RFC3209-6-1 positive -- the make-before-break replacement keeps the old LSP's SESSION (Tunnel Endpoint, Tunnel ID, Extended Tunnel ID) and differs only in LSP ID (reroute sets newKey.LSPID = oldKey.LSPID+1, reroute.go:45-46).
	assert.Equal(t, oldKey.TunnelEndpoint, newKey.TunnelEndpoint, "same tunnel endpoint (SESSION)")
	assert.Equal(t, oldKey.TunnelID, newKey.TunnelID, "same tunnel ID (SESSION)")
	assert.Equal(t, oldKey.ExtTunnelID, newKey.ExtTunnelID, "same extended tunnel ID (SESSION)")
	assert.NotEqual(t, oldKey.LSPID, newKey.LSPID, "only the LSP ID differs")

	// A PATH was sent for the new LSP; both LSPs exist (make-before-break).
	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "reroute sends a PATH for the new LSP")
	assert.Equal(t, uint16(2), path.SenderTemplate.LSPID)
	require.Len(t, path.ERO, 2)
	_, oldStillThere := e.table.Get(oldKey)
	assert.True(t, oldStillThere, "old LSP still up until new one is established")

	newLSP, _ := e.table.Get(newKey)
	require.NotNil(t, newLSP.Replaces)
	assert.Equal(t, oldKey, *newLSP.Replaces)

	// The new LSP's RESV arrives: new comes up, old is torn down.
	rsb := &resvStateBlock{
		Session: sessionIPv4{TunnelEndpoint: newKey.TunnelEndpoint, TunnelID: newKey.TunnelID, ExtTunnelID: newKey.ExtTunnelID},
		Label:   labelObject{Label: 17000},
		Style:   StyleSharedExplicit,
	}
	filter := senderTemplateIPv4{SenderAddr: newKey.SenderAddr, LSPID: newKey.LSPID}
	resv := buildResv(rsb, filter, DefaultRefreshPeriod, netip.MustParseAddr("10.0.0.6"))
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.6"), Payload: resv})

	up, ok := e.table.Get(newKey)
	require.True(t, ok)
	assert.Equal(t, LSPStateUp, up.State)
	assert.Equal(t, uint32(17000), up.OutLabel)
	_, oldGone := e.table.Get(oldKey)
	assert.False(t, oldGone, "old LSP torn down after new one is up")

	// A PathTear was emitted for the old LSP.
	tear, _, ok := ft.lastByType(MsgTypePathTear)
	require.True(t, ok, "old LSP torn down with a PathTear")
	assert.Equal(t, uint16(1), tear.SenderTemplate.LSPID)
}

// VALIDATES: B1 wiring -- reconfiguring an up tunnel with a changed ERO triggers
// make-before-break through setupTunnel (the production entry point), proving
// Reroute is reachable from a user action and not dead code.
func TestSetupTunnelEROChangeReroutes(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.1", nil)
	cfg := rsvpteConfig{RouterID: netip.MustParseAddr("10.0.0.1"), RefreshPeriod: DefaultRefreshPeriod}
	tc := tunnelConfig{
		Name:        "t1",
		Destination: netip.MustParseAddr("10.0.0.9"),
		TunnelID:    1,
		Bandwidth:   1e8,
		ERO:         []eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}},
	}
	setupTunnel(slogutil.DiscardLogger(), e.table, tc, cfg, e)

	key := lspKey{
		TunnelEndpoint: tc.Destination, TunnelID: 1,
		ExtTunnelID: addrToUint32(cfg.RouterID), SenderAddr: cfg.RouterID, LSPID: 1,
	}
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	lsp.mu.Lock()
	lsp.setState(LSPStateUp)
	lsp.mu.Unlock()

	// Reconfigure with a different explicit path.
	tc.ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.6/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}}
	setupTunnel(slogutil.DiscardLogger(), e.table, tc, cfg, e)

	newKey := key
	newKey.LSPID = 2
	newLSP, ok := e.table.Get(newKey)
	require.True(t, ok, "ERO change starts make-before-break with a new LSP_ID")
	require.NotNil(t, newLSP.Replaces)
	assert.Equal(t, key, *newLSP.Replaces)

	p, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	assert.Equal(t, uint16(2), p.SenderTemplate.LSPID, "PATH sent for the replacement LSP")
}
