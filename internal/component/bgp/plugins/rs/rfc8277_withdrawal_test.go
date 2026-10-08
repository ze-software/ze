// Design: docs/architecture/wire/nlri.md -- withdrawal framing of a labeled route
// RFC: rfc/short/rfc8277.md -- RFC8277-2.4-1, the Compatibility field on receipt

package rs

import (
	"encoding/binary"
	"encoding/hex"
	"maps"
	"net/netip"
	"strconv"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

var rsLabeledIPv4 = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}

// rsLabeledPeer is the source peer of every labeled UPDATE these tests apply.
const rsLabeledPeer = "192.0.2.1"

// rsLabeledUpdate builds a received UPDATE whose one attribute is MP_REACH_NLRI
// (code 14, with next hop 192.0.2.1) or MP_UNREACH_NLRI (code 15) carrying nlris
// for ipv4/mpls-label.
func rsLabeledUpdate(t *testing.T, code byte, nlris []byte) *bgptypes.RawMessage {
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

// rsApplyLabeled extracts the records of one labeled UPDATE and applies them to
// the route server's withdrawal set for rsLabeledPeer, the way the forward
// path does.
func rsApplyLabeled(t *testing.T, rs *routeServer, code byte, nlris []byte) {
	t.Helper()

	records := extractWireNLRIRecords(rsLabeledUpdate(t, code, nlris))
	if records == nil {
		t.Fatal("no records extracted")
	}
	rs.applyNLRIRecords(rsLabeledPeer, records.records)
	returnNLRIRecords(records)
}

// TestLabeledWithdrawalLeavesTheRouteServerSet announces two labeled routes of
// one length, 10.0.0.0/8 under label 100 and 11.0.0.0/8 under label 101, then
// withdraws 10.0.0.0/8 with the Compatibility value RFC 8277 Section 2.4 says a
// sender SHOULD use.
//
// VALIDATES: the route server's withdrawal set keeps 11.0.0.0/8 alone, and a
// peer-down withdrawal would name it by the announcement's bytes.
// PREVENTS: the announcement keyed by its hex and the withdrawal keyed by its
// prefix, so the withdrawal never cancels the announcement and the peer-down
// withdraws a route the peer had already withdrawn.
//
// RFC requirement: RFC8277-2.4-1 positive -- a route server that saw 10.0.0.0/8 and 11.0.0.0/8 announced in ipv4/mpls-label removes only 10.0.0.0/8 from its withdrawal set when a withdrawal with Compatibility 0x800000 names it.
// RFC requirement: RFC8277-2.4-1 negative -- valid nonrecommended Compatibility cannot retain withdrawn IPv4/IPv6 labeled routes or remove prefix/ADD-PATH zero/17 siblings from the exact native route-server inventory.
func TestLabeledWithdrawalLeavesTheRouteServerSet(t *testing.T) {
	t.Parallel()
	rsLabeledCompatibilityMatrix(t)

	rs := &routeServer{withdrawals: make(map[string]map[withdrawalKey]withdrawalEntry)}
	peer := rsLabeledPeer

	// [Length 24+8][label 100, S set][10] and [Length 24+8][label 101, S set][11].
	rsApplyLabeled(t, rs, 14, []byte{32, 0x00, 0x06, 0x41, 10, 32, 0x00, 0x06, 0x51, 11})
	if got := len(rs.withdrawals[peer]); got != 2 {
		t.Fatalf("withdrawal set holds %d routes after the announcement, want 2: %v", got, rs.withdrawals[peer])
	}

	rsApplyLabeled(t, rs, 15, []byte{32, 0x80, 0x00, 0x00, 10})

	if got := len(rs.withdrawals[peer]); got != 1 {
		t.Fatalf("withdrawal set holds %d routes after withdrawing 10.0.0.0/8, want 1: %v", got, rs.withdrawals[peer])
	}
	for wk, entry := range rs.withdrawals[peer] {
		if wk.fam != rsLabeledIPv4 || wk.prefix.String() != "11.0.0.0/8" {
			t.Fatalf("withdrawal set keeps %v, want ipv4/mpls-label 11.0.0.0/8", wk)
		}
		if entry.wire != "200006510b" {
			t.Fatalf("11.0.0.0/8 would be withdrawn as %q, want the announcement 200006510b", entry.wire)
		}
	}
}

// TestLabeledRelabelReplacesTheRouteServerEntry announces 10.0.0.0/8 under
// label 100, announces it again under label 101, then withdraws it with a
// Compatibility field of zero.
//
// VALIDATES: the second announcement replaces the first (RFC 8277 Section 2.5),
// so the set holds one entry naming the label-101 bytes, and the withdrawal
// empties it whatever its Compatibility field holds.
// PREVENTS: one entry per label, which leaves the old label's route in the set
// after the withdrawal and withdraws it again on peer-down.
func TestLabeledRelabelReplacesTheRouteServerEntry(t *testing.T) {
	t.Parallel()

	rs := &routeServer{withdrawals: make(map[string]map[withdrawalKey]withdrawalEntry)}
	peer := rsLabeledPeer

	rsApplyLabeled(t, rs, 14, []byte{32, 0x00, 0x06, 0x41, 10})
	rsApplyLabeled(t, rs, 14, []byte{32, 0x00, 0x06, 0x51, 10})
	if got := len(rs.withdrawals[peer]); got != 1 {
		t.Fatalf("withdrawal set holds %d entries after relabeling 10.0.0.0/8, want 1: %v", got, rs.withdrawals[peer])
	}
	for _, entry := range rs.withdrawals[peer] {
		if entry.wire != "200006510a" {
			t.Fatalf("10.0.0.0/8 would be withdrawn as %q, want the label-101 announcement 200006510a", entry.wire)
		}
	}

	rsApplyLabeled(t, rs, 15, []byte{32, 0x00, 0x00, 0x00, 10})
	if got := len(rs.withdrawals[peer]); got != 0 {
		t.Fatalf("withdrawal set holds %d entries after withdrawing 10.0.0.0/8, want 0: %v", got, rs.withdrawals[peer])
	}
}

// rsLabeledCompatibilityMatrix requires the exact native survivors, not a count,
// after withdrawing prefix and ADD-PATH siblings with every Compatibility value.
// MUTATION: discard the Path Identifier in recordKey or interpret Compatibility as a label.
func rsLabeledCompatibilityMatrix(t *testing.T) {
	t.Helper()
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		fam := family.Family{AFI: afi, SAFI: family.SAFIMPLSLabel}
		for _, addPath := range []bool{false, true} {
			for _, compatibility := range []uint32{0x800000, 0, 0x000641, 0x123450, 0xffffff} {
				t.Run(fam.String()+"/addpath="+strconv.FormatBool(addPath)+"/compat="+strconv.FormatUint(uint64(compatibility), 16), func(t *testing.T) {
					rs := &routeServer{withdrawals: make(map[string]map[withdrawalKey]withdrawalEntry)}
					prefixes := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("11.0.0.0/8")}
					if afi == family.AFIIPv6 {
						prefixes = []netip.Prefix{netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001:db9::/32")}
					}
					routes := [][]byte{rsLabeledRoute(prefixes[0], addPath, 0, 100), rsLabeledRoute(prefixes[1], addPath, 0, 101)}
					if addPath {
						routes = append(routes, rsLabeledRoute(prefixes[0], true, 17, 102))
					}
					want := make(map[string]bool)
					for _, route := range routes {
						rsApplyLabeledMatrix(t, rs, fam, addPath, 14, route)
						want[hex.EncodeToString(route)] = addPath
					}
					rsAssertLabeledInventory(t, rs, fam, want)
					for _, route := range routes {
						withdrawn := append([]byte(nil), route...)
						offset := 1
						if addPath {
							offset += 4
						}
						withdrawn[offset], withdrawn[offset+1], withdrawn[offset+2] = byte(compatibility>>16), byte(compatibility>>8), byte(compatibility)
						rsApplyLabeledMatrix(t, rs, fam, addPath, 15, withdrawn)
						delete(want, hex.EncodeToString(route))
						rsAssertLabeledInventory(t, rs, fam, want)
					}
				})
			}
		}
	}
}

func rsLabeledRoute(prefix netip.Prefix, addPath bool, pathID, label uint32) []byte {
	var route []byte
	if addPath {
		route = binary.BigEndian.AppendUint32(route, pathID)
	}
	route = append(route, byte(prefix.Bits()+24), byte(label>>12), byte(label>>4), byte(label<<4)|1)
	return append(route, prefix.Addr().AsSlice()[:(prefix.Bits()+7)/8]...)
}

func rsAssertLabeledInventory(t *testing.T, rs *routeServer, fam family.Family, want map[string]bool) {
	t.Helper()
	got := make(map[string]bool)
	for key, entry := range rs.withdrawals[rsLabeledPeer] {
		if key.fam != fam {
			t.Fatalf("family = %v, want %v", key.fam, fam)
		}
		got[entry.wire] = entry.addPath
	}
	if len(rs.withdrawals[rsLabeledPeer]) != len(want) || !maps.Equal(got, want) {
		t.Fatalf("retained labeled routes = %v, want exactly %v", got, want)
	}
}

func rsApplyLabeledMatrix(t *testing.T, rs *routeServer, fam family.Family, addPath bool, code byte, routes []byte) {
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
	records := extractWireNLRIRecords(&bgptypes.RawMessage{RawBytes: body, AttrsWire: attrsWire, WireUpdate: wu})
	if records == nil {
		t.Fatal("labeled extraction returned no records")
	}
	defer returnNLRIRecords(records)
	rs.applyNLRIRecords(rsLabeledPeer, records.records)
}
