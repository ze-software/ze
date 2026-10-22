// Design: docs/architecture/wire/nlri.md -- ADD-PATH framing of withdrawn routes
// RFC: rfc/short/rfc7911.md -- RFC7911-3-1, the Path Identifier in withdrawn routes

package rs

import (
	"encoding/hex"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestWithdrawOnlyUpdateKeepsPathIdentifier extracts the inventory records of
// an UPDATE that holds only the Withdrawn Routes field, withdrawing 10.0.0.0/8
// with Path Identifier 7, from a peer that negotiated ADD-PATH for ipv4/unicast.
//
// VALIDATES: one del record, for 10.0.0.0/8 with Path Identifier 7. Without
// the framing the six octets read as 0.0.0.0/0 three times and then 8.0.0.0/7.
// PREVENTS: the ADD-PATH state read from AttrsWire, which is nil for an UPDATE
// with no attribute section, so the withdrawal was walked without its Path
// Identifier and the route server kept the route in its inventory.
func TestWithdrawOnlyUpdateKeepsPathIdentifier(t *testing.T) {
	t.Parallel()

	// RFC 7911 Section 3: Path Identifier (4 octets), Length (1 octet), Prefix.
	withdrawn := []byte{0, 0, 0, 7, 8, 10}
	body := append([]byte{0, byte(len(withdrawn))}, withdrawn...)
	body = append(body, 0, 0) // Total Path Attribute Length 0, no NLRI.

	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wu.Attrs()
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	records := extractWireNLRIRecords(&bgptypes.RawMessage{RawBytes: body, AttrsWire: attrsWire, WireUpdate: wu})
	if records == nil {
		t.Fatal("no records extracted")
	}
	if len(records.records) != 1 {
		t.Fatalf("got %d records, want one withdrawal: %+v", len(records.records), records.records)
	}
	got := records.records[0]
	if got.action != actionDel {
		t.Fatalf("record action %q, want %q", got.action, actionDel)
	}
	if got.prefix != netip.MustParsePrefix("10.0.0.0/8") {
		t.Fatalf("record prefix %v, want 10.0.0.0/8", got.prefix)
	}
	if got.pathID != 7 {
		t.Fatalf("record Path Identifier %d, want 7", got.pathID)
	}
}

// rsAddPathUpdate builds a received ipv4/unicast UPDATE from a peer that
// negotiated ADD-PATH, with withdrawn and nlri in the body fields and attrs as
// the attribute section.
func rsAddPathUpdate(t *testing.T, withdrawn, attrs, nlri []byte) *bgptypes.RawMessage {
	t.Helper()

	body := append([]byte{0, byte(len(withdrawn))}, withdrawn...)
	body = append(body, 0, byte(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlri...)

	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wu.Attrs()
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	return &bgptypes.RawMessage{RawBytes: body, AttrsWire: attrsWire, WireUpdate: wu}
}

// TestWithdrawOnePathKeepsTheOther announces 10.0.0.0/8 with Path Identifiers
// 1 and 2 from one ADD-PATH peer, withdraws Path Identifier 1, then sends the
// peer-down withdrawals.
//
// VALIDATES: the inventory still holds Path Identifier 2 after the withdrawal,
// and the peer-down command withdraws exactly [00000002][8][10] as an ADD-PATH
// NLRI, in a command the update parser accepts.
// PREVENTS: appendUnicastRecords discarding the Path Identifier, so both paths
// collapsed into one inventory key: withdrawing path 1 deleted it, and when
// the peer went down path 2 stayed announced to every other client.
// RFC 7911 Section 5: "a new advertisement for a given address prefix and a
// given Path Identifier replaces a previous advertisement for the same address
// prefix and Path Identifier".
func TestWithdrawOnePathKeepsTheOther(t *testing.T) {
	rs := newTestRouteServer(t)
	path1 := []byte{0, 0, 0, 1, 8, 10}
	path2 := []byte{0, 0, 0, 2, 8, 10}
	origin := []byte{0x40, 1, 1, 0}

	apply := func(msg *bgptypes.RawMessage) {
		records := extractWireNLRIRecords(msg)
		if records == nil {
			t.Fatal("no records extracted")
		}
		rs.withdrawalMu.Lock()
		rs.applyNLRIRecords("10.0.0.1", records.records)
		rs.withdrawalMu.Unlock()
		returnNLRIRecords(records)
	}
	apply(rsAddPathUpdate(t, nil, origin, append(append([]byte{}, path1...), path2...)))
	if got := len(rs.withdrawals["10.0.0.1"]); got != 2 {
		t.Fatalf("inventory holds %d routes after announcing two paths, want 2: %v", got, rs.withdrawals["10.0.0.1"])
	}
	apply(rsAddPathUpdate(t, path1, nil, nil))
	entries := rs.withdrawals["10.0.0.1"]
	if len(entries) != 1 {
		t.Fatalf("inventory holds %d routes after withdrawing path 1, want 1 (path 2): %v", len(entries), entries)
	}

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
	if len(commands) != 1 {
		t.Fatalf("peer-down sent %d commands, want 1: %q", len(commands), commands)
	}
	const want = "update hex nlri ipv4/unicast addpath del 00000002080a"
	if commands[0] != want {
		t.Fatalf("peer-down command %q, want %q", commands[0], want)
	}
	fields := strings.Fields(commands[0])
	result, err := update.ParseUpdateWire(fields[2:], plugin.WireEncodingHex)
	if err != nil {
		t.Fatalf("the route server's own withdrawal command must parse: %v", err)
	}
	if len(result.Groups) != 1 || len(result.Groups[0].Withdraw) != 1 {
		t.Fatalf("parsed groups %+v, want one group withdrawing one NLRI", result.Groups)
	}
	if got := hex.EncodeToString(result.Groups[0].Withdraw[0].Bytes()); got != "00000002080a" {
		t.Fatalf("parsed withdrawal %s, want 00000002080a", got)
	}
}
