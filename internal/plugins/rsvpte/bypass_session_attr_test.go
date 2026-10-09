// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- a bypass LSP's PATH carries a
// SESSION_ATTRIBUTE naming the bypass, with the default priorities and no flag set.
package rsvpte

import (
	"io"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBypassPathCarriesSessionAttribute signals a configured bypass and decodes the
// PATH it would send: the SESSION_ATTRIBUTE is present (C-Type 7), names the
// configured bypass, uses the default priorities of an ordinary tunnel and sets no
// flag, so the bypass asks nobody to protect it. freeRtr refuses a PATH that lacks
// the object, which is why the bypass carries it (owner decision 2026-10-09).
func TestBypassPathCarriesSessionAttribute(t *testing.T) {
	routerID := netip.MustParseAddr("10.0.0.3")
	mp := netip.MustParseAddr("10.0.14.14")
	cfg := rsvpteConfig{RouterID: routerID, RefreshPeriod: DefaultRefreshPeriod}
	bc := bypassConfig{
		Name:       "bypass-to-mp",
		MergePoint: mp,
		ERO:        []eroHop{{Address: netip.MustParsePrefix("10.0.15.15/32")}, {Address: netip.PrefixFrom(mp, 32)}},
	}
	table := newLSPTable()
	setupBypass(slog.New(slog.NewTextHandler(io.Discard, nil)), table, bc, cfg, nil)

	lsp, ok := table.Get(bypassKey(bc, routerID))
	require.True(t, ok, "the bypass LSP exists")
	require.NotNil(t, lsp.PSB)

	msg, err := DecodeMessage(buildPath(lsp.PSB, routerID, 64))
	require.NoError(t, err)
	require.True(t, msg.HasSessionAttr, "the bypass PATH carries a SESSION_ATTRIBUTE")
	require.GreaterOrEqual(t, len(msg.SessionAttrRaw), objHdrLen)
	assert.Equal(t, ClassSessionAttr, msg.SessionAttrRaw[2], "Class-Num 207")
	assert.Equal(t, CTypeSessionAttr, msg.SessionAttrRaw[3], "LSP_TUNNEL C-Type 7")
	assert.Equal(t, "bypass-to-mp", msg.SessionAttr.Name, "session name is the configured bypass name")
	assert.Equal(t, defaultLSPPriority, msg.SessionAttr.SetupPrio, "setup priority matches an ordinary tunnel's default")
	assert.Equal(t, defaultLSPPriority, msg.SessionAttr.HoldPrio, "holding priority matches an ordinary tunnel's default")
	assert.Zero(t, msg.SessionAttr.Flags, "no protection, label-recording or SE flag on the bypass itself")
	assert.False(t, msg.HasFastReroute, "a bypass requests no protection, so no FAST_REROUTE")
}
