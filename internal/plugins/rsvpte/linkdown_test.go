// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- link-failure -> PathErr (AC-6) tests
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneIface configures a single rsvp-te interface, so admissionInterface resolves
// any next hop to it (the single-interface shortcut).
func oneIface(name string) func(*rsvpteConfig) {
	return func(c *rsvpteConfig) { c.Interfaces = []ifaceConfig{{Name: name}} }
}

func transitLSP(e *engine) lspKey {
	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	lsp, _ := e.table.GetOrCreate(key)
	lsp.Role = RoleTransit
	lsp.PrevHop = netip.MustParseAddr("10.0.0.1") // upstream (where PathErr goes)
	lsp.NextHop = netip.MustParseAddr("10.0.0.2") // downstream (the failing link)
	lsp.InLabel = 1001
	lsp.PSB = &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: 1},
	}
	lsp.setState(LSPStateUp)
	return key
}

// VALIDATES: AC-6 -- a transit node whose DOWNSTREAM link fails sends a PathErr
// upstream toward the head-end and tears the local LSP state.
func TestHandleLinkDownSendsPathErrAndTears(t *testing.T) {
	e, ft, fib := testEngine(t, "10.0.0.5", oneIface("eth0"))
	key := transitLSP(e)

	e.handleLinkDown("eth0")

	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok, "transit sends a PathErr on link-down")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), dst, "PathErr goes upstream toward the head-end")
	require.True(t, perr.HasErrorSpec)
	assert.Equal(t, ErrCodeRoutingProblem, perr.ErrorSpec.ErrorCode)
	assert.Equal(t, ErrValueNoRouteAvailable, perr.ErrorSpec.ErrorValue)

	_, exists := e.table.Get(key)
	assert.False(t, exists, "LSP torn down after link failure")
	require.Len(t, fib.removedSwap, 1, "transit swap entry withdrawn on link-down")
	assert.Equal(t, uint32(1001), fib.removedSwap[0])
}

// VALIDATES: F7 -- an INGRESS head-end LSP is matched by its downstream next hop
// (it never sets AdmissionIface), so a downstream link failure tears it down and
// reports a local path-err.
func TestHandleLinkDownIngressTornDown(t *testing.T) {
	e, _, fib := testEngine(t, "10.0.0.1", oneIface("eth0"))
	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	lsp, _ := e.table.GetOrCreate(key)
	lsp.Role = RoleIngress
	lsp.NextHop = netip.MustParseAddr("10.0.0.2") // downstream first hop
	lsp.PSB = &pathStateBlock{Session: sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: 1}}
	lsp.setState(LSPStateUp)

	e.handleLinkDown("eth0")

	_, exists := e.table.Get(key)
	assert.False(t, exists, "ingress LSP torn down on downstream link failure")
	require.Len(t, fib.removed, 1, "ingress push entry withdrawn on link-down")
	assert.Equal(t, netip.MustParsePrefix("10.0.0.9/32"), fib.removed[0])
}

// VALIDATES: AC-6 -- an interface-down event for an unrelated link leaves the
// LSP and its forwarding state intact.
func TestHandleLinkDownIgnoresUnrelatedIface(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.5", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", Prefix: netip.MustParsePrefix("10.0.0.0/24")},
			{Name: "eth1", Prefix: netip.MustParsePrefix("10.9.0.0/24")},
		}
	})
	key := transitLSP(e) // NextHop 10.0.0.2 is on eth0

	e.handleLinkDown("eth1")

	_, _, ok := ft.lastByType(MsgTypePathErr)
	assert.False(t, ok, "an unrelated link-down sends no PathErr")
	_, exists := e.table.Get(key)
	assert.True(t, exists, "LSP survives an unrelated link-down")
}
