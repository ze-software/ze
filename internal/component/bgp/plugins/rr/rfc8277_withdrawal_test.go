// Design: docs/architecture/wire/nlri.md -- withdrawal framing of a labeled route
// RFC: rfc/short/rfc8277.md -- RFC8277-2.4-1, the Compatibility field on receipt

package rr

import (
	"encoding/binary"
	"encoding/hex"
	"maps"
	"net/netip"
	"slices"
	"strconv"
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
// RFC requirement: RFC8277-2.4-1 negative -- valid nonrecommended Compatibility cannot retain withdrawn IPv4/IPv6 labeled routes or remove prefix/ADD-PATH zero/17 siblings from the exact reflector inventory.
func TestLabeledWithdrawalLeavesTheWithdrawalMap(t *testing.T) {
	t.Parallel()
	rrLabeledCompatibilityMatrix(t)

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

// rrLabeledCompatibilityMatrix tracks prefix and Path Identifier siblings,
// checking every retained identity after each Compatibility-only withdrawal.
// MUTATION: key labeled inventory by the prefix alone, collapsing ADD-PATH siblings.
func rrLabeledCompatibilityMatrix(t *testing.T) {
	t.Helper()
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		fam := family.Family{AFI: afi, SAFI: family.SAFIMPLSLabel}
		for _, addPath := range []bool{false, true} {
			for _, compatibility := range []uint32{0x800000, 0, 0x000641, 0x123450, 0xffffff} {
				t.Run(fam.String()+"/addpath="+strconv.FormatBool(addPath)+"/compat="+strconv.FormatUint(uint64(compatibility), 16), func(t *testing.T) {
					rr := &routeReflector{withdrawals: make(map[string]map[string]withdrawalInfo)}
					prefixes := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("11.0.0.0/8")}
					if afi == family.AFIIPv6 {
						prefixes = []netip.Prefix{netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001:db9::/32")}
					}
					var routes [][]byte
					want := make(map[string]bool)
					for i, prefix := range prefixes {
						route := rrLabeledRoute(prefix, addPath, 0, uint32(100+i))
						routes = append(routes, route)
						want[rrLabeledInventoryIdentity(route, prefix, addPath)] = addPath
					}
					if addPath {
						route := rrLabeledRoute(prefixes[0], true, 17, 102)
						routes = append(routes, route)
						want[hex.EncodeToString(route)] = true
					}
					for _, route := range routes {
						rr.updateWithdrawalMapWire("192.0.2.1", rrLabeledMatrixUpdate(t, fam, addPath, 14, route))
					}
					rrAssertLabeledInventory(t, rr, fam, want)
					for i, route := range routes {
						withdrawn := append([]byte(nil), route...)
						offset := 1
						if addPath {
							offset += 4
						}
						withdrawn[offset], withdrawn[offset+1], withdrawn[offset+2] = byte(compatibility>>16), byte(compatibility>>8), byte(compatibility)
						rr.updateWithdrawalMapWire("192.0.2.1", rrLabeledMatrixUpdate(t, fam, addPath, 15, withdrawn))
						delete(want, rrLabeledInventoryIdentity(route, prefixes[i%2], addPath))
						rrAssertLabeledInventory(t, rr, fam, want)
					}
				})
			}
		}
	}
}

func rrLabeledRoute(prefix netip.Prefix, addPath bool, pathID, label uint32) []byte {
	var route []byte
	if addPath {
		route = binary.BigEndian.AppendUint32(route, pathID)
	}
	route = append(route, byte(prefix.Bits()+24), byte(label>>12), byte(label>>4), byte(label<<4)|1)
	return append(route, prefix.Addr().AsSlice()[:(prefix.Bits()+7)/8]...)
}

func rrLabeledInventoryIdentity(route []byte, prefix netip.Prefix, addPath bool) string {
	if addPath {
		return hex.EncodeToString(route)
	}
	return prefix.String()
}

func rrAssertLabeledInventory(t *testing.T, rr *routeReflector, fam family.Family, want map[string]bool) {
	t.Helper()
	got := make(map[string]bool)
	for _, info := range rr.withdrawals["192.0.2.1"] {
		if info.Family != fam.String() {
			t.Fatalf("family = %s, want %s", info.Family, fam)
		}
		if info.WireForm != info.AddPath {
			t.Fatalf("ADD-PATH survivor lacks native framing: %+v", info)
		}
		got[info.Prefix] = info.AddPath
	}
	if len(rr.withdrawals["192.0.2.1"]) != len(want) || !maps.Equal(got, want) {
		t.Fatalf("retained labeled routes = %v, want exactly %v", got, want)
	}
}

func rrLabeledMatrixUpdate(t *testing.T, fam family.Family, addPath bool, code byte, routes []byte) *bgptypes.RawMessage {
	t.Helper()
	value := []byte{0, byte(fam.AFI), byte(fam.SAFI)}
	if code == 14 {
		hop := netip.MustParseAddr("192.0.2.1")
		if fam.AFI == family.AFIIPv6 {
			hop = netip.MustParseAddr("2001:db8::1")
		}
		value = append(value, byte(len(hop.AsSlice())))
		value = append(value, hop.AsSlice()...)
		value = append(value, 0)
	}
	value = append(value, routes...)
	attrs := append([]byte{0x80, code, byte(len(value))}, value...)
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: addPath}))
	if err != nil {
		t.Fatal(err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	return &bgptypes.RawMessage{RawBytes: body, AttrsWire: attrsWire, WireUpdate: wu}
}
