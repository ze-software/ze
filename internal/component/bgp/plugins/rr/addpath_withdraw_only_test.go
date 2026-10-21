// Design: docs/architecture/wire/nlri.md -- ADD-PATH framing of withdrawn routes
// RFC: rfc/short/rfc7911.md -- RFC7911-3-1, the Path Identifier in withdrawn routes

package rr

import (
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// rrAddPathUpdate builds a received ipv4/unicast UPDATE from a peer that
// negotiated ADD-PATH, with withdrawn and nlri in the body fields and attrs as
// the attribute section.
func rrAddPathUpdate(t *testing.T, withdrawn, attrs, nlri []byte) *bgptypes.RawMessage {
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

// TestWithdrawOnlyUpdateKeepsPathIdentifier announces 10.0.0.0/8 with Path
// Identifier 7, then withdraws it in an UPDATE that holds only the Withdrawn
// Routes field.
//
// VALIDATES: the route reflector's withdrawal map is empty after the withdrawal.
// PREVENTS: the ADD-PATH state read from AttrsWire, which is nil for an UPDATE
// with no attribute section, so the withdrawal was walked without its Path
// Identifier and the route stayed in the map, to be withdrawn again when the
// peer goes down.
func TestWithdrawOnlyUpdateKeepsPathIdentifier(t *testing.T) {
	t.Parallel()

	const peer = "192.0.2.1"
	// RFC 7911 Section 3: Path Identifier (4 octets), Length (1 octet), Prefix.
	route := []byte{0, 0, 0, 7, 8, 10}
	origin := []byte{0x40, 1, 1, 0}
	rr := &routeReflector{withdrawals: make(map[string]map[string]withdrawalInfo)}

	rr.updateWithdrawalMapWire(peer, rrAddPathUpdate(t, nil, origin, route))
	if got := len(rr.withdrawals[peer]); got != 1 {
		t.Fatalf("withdrawal map holds %d routes after the announcement, want 1: %v", got, rr.withdrawals[peer])
	}
	rr.updateWithdrawalMapWire(peer, rrAddPathUpdate(t, route, nil, nil))
	if got := len(rr.withdrawals[peer]); got != 0 {
		t.Fatalf("withdrawal map holds %d routes after the withdraw-only UPDATE, want 0: %v", got, rr.withdrawals[peer])
	}
}

// TestWithdrawOnePathKeepsTheOther announces 10.0.0.0/8 with Path Identifiers
// 1 and 2 from one ADD-PATH peer, then withdraws Path Identifier 1.
//
// VALIDATES: the withdrawal map still holds Path Identifier 2, as the native
// NLRI [00000002][8][10] with ADD-PATH framing, which is what the peer-down
// withdrawal sends.
// PREVENTS: walkUnicastNLRIs discarding the Path Identifier, so both paths
// shared the key "ipv4/unicast|10.0.0.0/8": withdrawing path 1 deleted it, and
// when the peer went down path 2 was never withdrawn from the other peers.
// RFC 7911 Section 5 makes the identifier part of the route: "a new
// advertisement for a given address prefix and a given Path Identifier replaces
// a previous advertisement for the same address prefix and Path Identifier".
func TestWithdrawOnePathKeepsTheOther(t *testing.T) {
	t.Parallel()

	const peer = "192.0.2.1"
	path1 := []byte{0, 0, 0, 1, 8, 10}
	path2 := []byte{0, 0, 0, 2, 8, 10}
	origin := []byte{0x40, 1, 1, 0}
	rr := &routeReflector{withdrawals: make(map[string]map[string]withdrawalInfo)}

	rr.updateWithdrawalMapWire(peer, rrAddPathUpdate(t, nil, origin, append(append([]byte{}, path1...), path2...)))
	if got := len(rr.withdrawals[peer]); got != 2 {
		t.Fatalf("withdrawal map holds %d routes after announcing two paths, want 2: %v", got, rr.withdrawals[peer])
	}
	rr.updateWithdrawalMapWire(peer, rrAddPathUpdate(t, path1, nil, nil))
	entries := rr.withdrawals[peer]
	if len(entries) != 1 {
		t.Fatalf("withdrawal map holds %d routes after withdrawing path 1, want 1 (path 2): %v", len(entries), entries)
	}
	want := withdrawalInfo{Family: "ipv4/unicast", Prefix: "00000002080a", WireForm: true, AddPath: true}
	for key, got := range entries {
		if got != want {
			t.Fatalf("withdrawal map entry %q = %+v, want %+v", key, got, want)
		}
	}
}
