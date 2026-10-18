// Design: docs/architecture/wire/nlri.md -- VPN withdrawal identity.

package rs

import (
	"encoding/binary"
	"encoding/hex"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestVPNWithdrawalLeavesExactRouteServerInventory feeds native VPN UPDATEs
// through the registered wire parser and the route server's existing inventory.
// Equal-length routes differ only in RD, prefix, or negotiated Path Identifier.
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." Each withdrawal must remove exactly its announced route.
// MUTATION: key VPN records by the full NLRI, retaining the Compatibility bytes.
//
// RFC requirement: RFC8277-2.4-1 positive -- IPv4/IPv6 VPN withdrawals with Compatibility 0x800000 remove exactly the named route from the route-server inventory, retaining distinct RDs, prefixes and ADD-PATH identifiers including zero.
// RFC requirement: RFC8277-2.4-1 negative -- nonrecommended Compatibility values 0x000000, 0x000641, 0x123450 and 0xffffff neither leave the named VPN route retained nor remove routes with another RD, prefix or ADD-PATH identifier.
func TestVPNWithdrawalLeavesExactRouteServerInventory(t *testing.T) {
	for _, fam := range []family.Family{vpn.IPv4VPN, vpn.IPv6VPN} {
		for _, addPath := range []bool{false, true} {
			for _, compatibility := range []uint32{0x800000, 0, 0x000641, 0x123450, 0xffffff} {
				name := fam.String() + "/addpath=" + strconv.FormatBool(addPath) + "/compat=" + strconv.FormatUint(uint64(compatibility), 16)
				t.Run(name, func(t *testing.T) {
					rs := &routeServer{withdrawals: make(map[string]map[withdrawalKey]withdrawalEntry)}
					routes := [][]byte{
						rsVPNRoute(fam, addPath, 0, 1, 10, 0x000641),
						rsVPNRoute(fam, addPath, 0, 2, 10, 0x000651),
						rsVPNRoute(fam, addPath, 0, 1, 11, 0x000661),
					}
					if addPath {
						routes = append(routes, rsVPNRoute(fam, true, 17, 1, 10, 0x000671))
					}
					var announced []byte
					want := make(map[string]bool)
					for _, route := range routes {
						announced = append(announced, route...)
						want[hex.EncodeToString(route)] = addPath
					}
					rsApplyVPN(t, rs, fam, addPath, 14, announced)
					rsAssertVPNInventory(t, rs, fam, want)
					for _, route := range routes {
						withdrawn := append([]byte(nil), route...)
						offset := 1
						if addPath {
							offset += 4
						}
						withdrawn[offset] = byte(compatibility >> 16)
						withdrawn[offset+1] = byte(compatibility >> 8)
						withdrawn[offset+2] = byte(compatibility)
						// RFC 8277 Section 2.4: match RD, prefix and Path Identifier, not label.
						rsApplyVPN(t, rs, fam, addPath, 15, withdrawn)
						delete(want, hex.EncodeToString(route))
						rsAssertVPNInventory(t, rs, fam, want)
					}
				})
			}
		}
	}
}

func rsAssertVPNInventory(t *testing.T, rs *routeServer, fam family.Family, want map[string]bool) {
	t.Helper()
	got := make(map[string]bool)
	for key, entry := range rs.withdrawals[rsLabeledPeer] {
		if key.fam != fam {
			t.Fatalf("inventory family = %v, want %v", key.fam, fam)
		}
		if entry.wire == "" {
			if !key.wireForm {
				t.Fatalf("VPN inventory lost native framing: %+v", key)
			}
			got[key.nlriStr] = key.addPath
		} else {
			got[entry.wire] = entry.addPath
		}
	}
	if len(rs.withdrawals[rsLabeledPeer]) != len(want) || !maps.Equal(got, want) {
		t.Fatalf("retained VPN routes = %v (%d entries), want exactly %v", got, len(rs.withdrawals[rsLabeledPeer]), want)
	}
}

func rsApplyVPN(t *testing.T, rs *routeServer, fam family.Family, addPath bool, code byte, routes []byte) {
	t.Helper()
	value := []byte{0, byte(fam.AFI), byte(fam.SAFI)}
	if code == 14 {
		nextHop := make([]byte, 12)
		copy(nextHop[8:], []byte{192, 0, 2, 1})
		if fam.AFI == family.AFIIPv6 {
			nextHop = make([]byte, 24)
			copy(nextHop[8:], []byte{0x20, 1, 0x0d, 0xb8})
			nextHop[23] = 1
		}
		value = append(value, byte(len(nextHop)))
		value = append(value, nextHop...)
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
		t.Fatal("VPN extraction returned no records")
	}
	defer returnNLRIRecords(records)
	rs.applyNLRIRecords(rsLabeledPeer, records.records)
}

// rsVPNRoute writes [Path ID?][Length][Label/Compatibility:3][RD:8][Prefix].
// IPv4 /8 and IPv6 /32 controls have equal lengths within each family.
func rsVPNRoute(fam family.Family, addPath bool, pathID uint32, rd, prefix byte, label uint32) []byte {
	var route []byte
	if addPath {
		route = binary.BigEndian.AppendUint32(route, pathID)
	}
	bits := byte(96)
	if fam.AFI == family.AFIIPv6 {
		bits = 120
	}
	route = append(route, bits, byte(label>>16), byte(label>>8), byte(label), 0, 0, 0xfd, 0xe8, 0, 0, 0, rd)
	if fam.AFI == family.AFIIPv6 {
		return append(route, 0x20, 1, 0x0d, prefix)
	}
	return append(route, prefix)
}

// TestVPNRouteServerPeerDownEmitsOnlyRetainedNativeRoutes captures the existing
// peer-down sender after a Compatibility-only withdrawal. Exact command and
// parsed NLRI assertions keep the source's native bytes and ADD-PATH framing.
// RFC 8277 Section 2.4 requires matching prefixes and RFC 7911 identifiers.
// MUTATION: retain the withdrawn target by including Compatibility in its key.
//
// RFC requirement: RFC8277-2.4-1 positive -- after receiving a VPN withdrawal with Compatibility 0x800000, route-server peer-down emits and reparses a native hex command naming only the two remaining IPv4/IPv6 VPN routes, with their original bytes and ADD-PATH framing.
func TestVPNRouteServerPeerDownEmitsOnlyRetainedNativeRoutes(t *testing.T) {
	for _, fam := range []family.Family{vpn.IPv4VPN, vpn.IPv6VPN} {
		for _, addPath := range []bool{false, true} {
			t.Run(fam.String()+"/addpath="+strconv.FormatBool(addPath), func(t *testing.T) {
				rs := newTestRouteServer(t)
				target := rsVPNRoute(fam, addPath, 0, 1, 10, 0x000641)
				first := rsVPNRoute(fam, addPath, 0, 2, 10, 0x000651)
				second := rsVPNRoute(fam, addPath, 17, 1, 11, 0x000661)
				for _, route := range [][]byte{target, first, second} {
					rsApplyVPN(t, rs, fam, addPath, 14, route)
				}
				rsApplyVPN(t, rs, fam, addPath, 15, rsVPNRoute(fam, addPath, 0, 1, 10, 0x800000))
				var commands []string
				rs.updateRouteHook = func(_, command string) { commands = append(commands, command) }
				rs.sendBatchedWithdrawals(rsLabeledPeer, rs.withdrawals[rsLabeledPeer])
				if len(commands) != 1 {
					t.Fatalf("peer-down emitted %d commands, want 1", len(commands))
				}
				wantRoutes := []string{hex.EncodeToString(first), hex.EncodeToString(second)}
				slices.Sort(wantRoutes)
				wantCommand := "update hex nlri " + fam.String()
				if addPath {
					wantCommand += " addpath"
				}
				wantCommand += " del " + strings.Join(wantRoutes, " del ")
				if commands[0] != wantCommand {
					t.Fatalf("peer-down command = %q, want %q", commands[0], wantCommand)
				}
				parsed, err := update.ParseUpdateWire(strings.Fields(commands[0])[2:], plugin.WireEncodingHex)
				if err != nil {
					t.Fatalf("native withdrawal command rejected: %v", err)
				}
				if len(parsed.Groups) != 1 {
					t.Fatalf("withdrawal groups = %d, want 1", len(parsed.Groups))
				}
				group := parsed.Groups[0]
				if group.Family != fam || len(group.Announce) != 0 {
					t.Fatalf("wrong family or announcement in withdrawal: %+v", group)
				}
				var gotRoutes []string
				for _, route := range group.Withdraw {
					gotRoutes = append(gotRoutes, hex.EncodeToString(route.Bytes()))
				}
				slices.Sort(gotRoutes)
				if !slices.Equal(gotRoutes, wantRoutes) {
					t.Fatalf("parsed withdrawals = %v, want %v", gotRoutes, wantRoutes)
				}
			})
		}
	}
}
