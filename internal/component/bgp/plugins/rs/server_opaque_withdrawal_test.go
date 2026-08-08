// Design: server_inventory.go -- peer-down route inventory for the route server
// RFC: rfc/short/rfc9552.md -- Section 5.2: an unknown Link-State NLRI type is an
// opaque object that MUST be preserved and propagated, withdrawals included.

package rs

import (
	"encoding/hex"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
)

// bgplsUpdateBody is the UPDATE body (header stripped) of the MP_REACH_NLRI in
// test/plugin/rfc7606-54-bgpls-override-propagates.ci: ORIGIN, AS_PATH, then
// AFI 16388 / SAFI 71 with next-hop 1.1.1.1 and a 23-octet NLRI section holding
// two Link-State NLRIs -- type 1 (Node), which ze parses, and type 99, which ze
// does not.
const bgplsUpdateBody = "000000304001010040020602010000fde9800e2040044704010101010" +
	"00001000b020000000000000000000000630004deadbeef"

// The two Link-State NLRIs of that section, each framed by its own Type and
// Total NLRI Length (RFC 9552 Section 5.1).
const (
	bgplsNLRINode    = "0001000b0200000000000000000000"
	bgplsNLRIUnknown = "00630004deadbeef"
)

func bgplsRawMessage(t *testing.T) *bgptypes.RawMessage {
	t.Helper()
	body, err := hex.DecodeString(bgplsUpdateBody)
	require.NoError(t, err)

	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)

	return &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, WireUpdate: wu, AttrsWire: attrs}
}

// TestOpaqueNLRIRecordedAsSplitWireBytes verifies the inventory keeps the wire
// bytes of an NLRI ze cannot parse, one record per NLRI.
//
// VALIDATES: wireu.ParseNLRIs returns the WHOLE NLRI section as one opaque
// *nlri.WireNLRI for a family with no dedicated parser, and its String() is a
// size summary ("wire[bgp-ls/bgp-ls](23 bytes)") carrying none of the bytes.
// appendParsedRecords must record hex instead, and must split the section so
// each NLRI gets its own key.
// PREVENTS: a withdrawal-set key that names no route, and one key standing for
// every NLRI in an UPDATE (a later MP_UNREACH of one of them would miss it and
// leave the rest announced after the source peer goes down).
func TestOpaqueNLRIRecordedAsSplitWireBytes(t *testing.T) {
	records := extractWireNLRIRecords(bgplsRawMessage(t))
	require.NotNil(t, records)
	t.Cleanup(func() { returnNLRIRecords(records) })

	require.Len(t, *records, 2, "the 23-octet section holds two Link-State NLRIs")
	for _, rec := range *records {
		assert.True(t, rec.wireForm, "an unparsed NLRI is recorded in wire form")
		assert.Equal(t, actionAdd, rec.action)
		assert.Equal(t, "bgp-ls/bgp-ls", rec.familyName)
		assert.NotContains(t, rec.nlriStr, "wire[", "the size summary is not an NLRI")
	}
	assert.Equal(t, bgplsNLRINode, (*records)[0].nlriStr)
	assert.Equal(t, bgplsNLRIUnknown, (*records)[1].nlriStr)
}

// TestPeerDownWithdrawsOpaqueNLRIAsWireCommand verifies the peer-down
// withdrawal of a family with no text NLRI spelling goes out as "update hex",
// and that the command the route server produces parses.
//
// VALIDATES: sendBatchedWithdrawals picks the command form from the record, so
// a BGP-LS withdrawal reaches the wire parser instead of the text parser.
// PREVENTS: the regression this test was written for -- "update text nlri
// bgp-ls/bgp-ls del wire[bgp-ls/bgp-ls](23 bytes)", rejected with
// route.ErrFamilyNotSupported, which left every other route-server client
// holding the departed peer's Link-State routes forever.
func TestPeerDownWithdrawsOpaqueNLRIAsWireCommand(t *testing.T) {
	rs := newTestRouteServer(t)

	records := extractWireNLRIRecords(bgplsRawMessage(t))
	require.NotNil(t, records)
	rs.withdrawalMu.Lock()
	rs.applyNLRIRecords("10.0.0.1", *records)
	entries := rs.withdrawals["10.0.0.1"]
	rs.withdrawalMu.Unlock()
	returnNLRIRecords(records)
	require.Len(t, entries, 2)

	var mu sync.Mutex
	var commands []string
	rs.updateRouteHook = func(_, cmd string) {
		mu.Lock()
		commands = append(commands, cmd)
		mu.Unlock()
	}

	rs.sendBatchedWithdrawals("10.0.0.1", entries)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, commands, 1, "both NLRIs share one family and one form, so one batch")
	cmd := commands[0]

	assert.NotContains(t, cmd, "update text", "bgp-ls has no text NLRI spelling")
	assert.NotContains(t, cmd, "wire[", "the size summary must never reach a command")
	assert.True(t, strings.HasPrefix(cmd, "update hex nlri bgp-ls/bgp-ls "), "got %q", cmd)
	assert.Contains(t, cmd, "del "+bgplsNLRINode)
	assert.Contains(t, cmd, "del "+bgplsNLRIUnknown)

	// The command must PARSE, which is what the text form could not do. Drop
	// "update" and the encoding token, exactly as handleUpdate does.
	fields := strings.Fields(cmd)
	require.Greater(t, len(fields), 2)
	result, err := update.ParseUpdateWire(fields[2:], plugin.WireEncodingHex)
	require.NoError(t, err, "the route server's own withdrawal command must parse")
	require.Len(t, result.Groups, 1)

	group := result.Groups[0]
	assert.Equal(t, family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}, group.Family)
	assert.Empty(t, group.Announce)
	require.Len(t, group.Withdraw, 2, "both Link-State NLRIs are withdrawn, type 99 included")

	var got []string
	for _, n := range group.Withdraw {
		got = append(got, hex.EncodeToString(n.Bytes()))
	}
	assert.ElementsMatch(t, []string{bgplsNLRINode, bgplsNLRIUnknown}, got)
}
