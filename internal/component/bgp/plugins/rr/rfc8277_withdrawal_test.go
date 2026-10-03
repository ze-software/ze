// Design: docs/architecture/wire/nlri.md -- withdrawal framing of a labeled route
// RFC: rfc/short/rfc8277.md -- RFC8277-2.4-1, the Compatibility field on receipt

package rr

import (
	"slices"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

var rrLabeledIPv4 = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}

// rrLabeledUpdate builds a received UPDATE whose one attribute is MP_REACH_NLRI
// (code 14, with next hop 192.0.2.1) or MP_UNREACH_NLRI (code 15) carrying nlris
// for ipv4/mpls-label.
func rrLabeledUpdate(t *testing.T, code byte, nlris []byte) *bgptypes.RawMessage {
	t.Helper()

	value := []byte{0, 1, 4} // AFI 1, SAFI 4
	if code == 14 {
		value = append(value, 4, 192, 0, 2, 1, 0) // next hop length, next hop, reserved
	}
	value = append(value, nlris...)
	attrs := append([]byte{0x80, code, byte(len(value))}, value...)
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)

	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
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

// TestLabeledWithdrawalLeavesTheWithdrawalMap announces two labeled routes of
// one length, 10.0.0.0/8 under label 100 and 11.0.0.0/8 under label 101, then
// withdraws 10.0.0.0/8 with the Compatibility value RFC 8277 Section 2.4 says a
// sender SHOULD use.
//
// VALIDATES: the route reflector's withdrawal map keeps 11.0.0.0/8 alone, so a
// later peer-down withdraws only the route the peer still announces.
// PREVENTS: the withdrawal framed by the announcement reader, which walks
// 0x800000 as a label entry, errors, and leaves the withdrawn route in the map;
// and routes of one family keyed by their byte count, so one withdrawal
// removes every route of the same length.
//
// RFC requirement: RFC8277-2.4-1 positive -- a route reflector that saw 10.0.0.0/8 and 11.0.0.0/8 announced in ipv4/mpls-label removes only 10.0.0.0/8 from its withdrawal map when a withdrawal with Compatibility 0x800000 names it.
func TestLabeledWithdrawalLeavesTheWithdrawalMap(t *testing.T) {
	t.Parallel()

	rr := &routeReflector{withdrawals: make(map[string]map[string]withdrawalInfo)}
	const peer = "192.0.2.1"

	// [Length 24+8][label 100, S set][10] and [Length 24+8][label 101, S set][11].
	announce := []byte{32, 0x00, 0x06, 0x41, 10, 32, 0x00, 0x06, 0x51, 11}
	rr.updateWithdrawalMapWire(peer, rrLabeledUpdate(t, 14, announce))
	if got := len(rr.withdrawals[peer]); got != 2 {
		t.Fatalf("withdrawal map holds %d routes after the announcement, want 2: %v", got, rr.withdrawals[peer])
	}

	rr.updateWithdrawalMapWire(peer, rrLabeledUpdate(t, 15, []byte{32, 0x80, 0x00, 0x00, 10}))

	var left []string
	for _, info := range rr.withdrawals[peer] {
		left = append(left, info.Prefix)
	}
	slices.Sort(left)
	if len(left) != 1 || left[0] != "11.0.0.0/8" {
		t.Fatalf("withdrawal map holds %v after withdrawing 10.0.0.0/8, want [11.0.0.0/8]", left)
	}
	if info := rr.withdrawals[peer][rrLabeledIPv4.String()+"|11.0.0.0/8"]; info.Family != rrLabeledIPv4.String() {
		t.Fatalf("11.0.0.0/8 is not keyed by its family and prefix: %v", rr.withdrawals[peer])
	}
}
