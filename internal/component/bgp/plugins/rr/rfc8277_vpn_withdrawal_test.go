// Design: docs/architecture/wire/nlri.md -- VPN withdrawal identity.

package rr

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestVPNWithdrawalLeavesExactReflectorInventory feeds native VPN UPDATEs
// through the registered wire parser and the route reflector's existing inventory.
// Equal-length routes differ only in RD, prefix, or negotiated Path Identifier.
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." Each withdrawal must remove exactly its announced route.
// MUTATION: key VPN records by WireNLRI.String, aliasing equal-length routes.
//
// RFC requirement: RFC8277-2.4-1 positive -- IPv4/IPv6 VPN withdrawals with Compatibility 0x800000 remove exactly the named route from the reflector inventory, retaining distinct RDs, prefixes and ADD-PATH identifiers including zero.
// RFC requirement: RFC8277-2.4-1 negative -- nonrecommended Compatibility values 0x000000, 0x000641, 0x123450 and 0xffffff neither leave the named VPN route retained nor remove routes with another RD, prefix or ADD-PATH identifier.
func TestVPNWithdrawalLeavesExactReflectorInventory(t *testing.T) {
	for _, fam := range []family.Family{vpn.IPv4VPN, vpn.IPv6VPN} {
		for _, addPath := range []bool{false, true} {
			for _, compatibility := range []uint32{0x800000, 0, 0x000641, 0x123450, 0xffffff} {
				name := fam.String() + "/addpath=" + strconv.FormatBool(addPath) + "/compat=" + strconv.FormatUint(uint64(compatibility), 16)
				t.Run(name, func(t *testing.T) {
					rr := &routeReflector{withdrawals: make(map[string]map[string]withdrawalInfo)}
					routes := [][]byte{
						rrVPNRoute(fam, addPath, 0, 1, 10, 0x000641),
						rrVPNRoute(fam, addPath, 0, 2, 10, 0x000651),
						rrVPNRoute(fam, addPath, 0, 1, 11, 0x000661),
					}
					if addPath {
						routes = append(routes, rrVPNRoute(fam, true, 17, 1, 10, 0x000671))
					}
					var announced []byte
					want := make(map[string]bool)
					for _, route := range routes {
						announced = append(announced, route...)
						want[hex.EncodeToString(route)] = addPath
					}
					rrApplyVPN(t, rr, fam, addPath, 14, announced)
					rrAssertVPNInventory(t, rr, fam, want)
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
						rrApplyVPN(t, rr, fam, addPath, 15, withdrawn)
						delete(want, hex.EncodeToString(route))
						rrAssertVPNInventory(t, rr, fam, want)
					}
				})
			}
		}
	}
}

func rrAssertVPNInventory(t *testing.T, rr *routeReflector, fam family.Family, want map[string]bool) {
	t.Helper()
	got := make(map[string]bool)
	for _, entry := range rr.withdrawals["192.0.2.1"] {
		if entry.Family != fam.String() {
			t.Fatalf("inventory family = %v, want %v", entry.Family, fam)
		}
		if !entry.WireForm {
			t.Fatalf("VPN inventory lost native framing: %+v", entry)
		}
		got[entry.Prefix] = entry.AddPath
	}
	if len(rr.withdrawals["192.0.2.1"]) != len(want) || !maps.Equal(got, want) {
		t.Fatalf("retained VPN routes = %v (%d entries), want exactly %v", got, len(rr.withdrawals["192.0.2.1"]), want)
	}
}

func rrApplyVPN(t *testing.T, rr *routeReflector, fam family.Family, addPath bool, code byte, routes []byte) {
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
	rr.updateWithdrawalMapWire("192.0.2.1", &bgptypes.RawMessage{RawBytes: body, AttrsWire: attrsWire, WireUpdate: wu})
}

// rrVPNRoute writes [Path ID?][Length][Label/Compatibility:3][RD:8][Prefix].
// IPv4 /8 and IPv6 /32 controls have equal lengths within each family.
func rrVPNRoute(fam family.Family, addPath bool, pathID uint32, rd, prefix byte, label uint32) []byte {
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

// TestVPNReflectorPeerDownEmitsOnlyRetainedNativeRoutes captures the actual
// peer-down RPC after a Compatibility-only withdrawal. The command and parsed
// NLRIs must name the two survivors, preserving ADD-PATH zero and nonzero IDs.
// RFC 8277 Section 2.4 requires matching prefixes and RFC 7911 identifiers.
// MUTATION: force the peer-down command to use text instead of native hex.
//
// RFC requirement: RFC8277-2.4-1 positive -- after receiving a VPN withdrawal with Compatibility 0x800000, reflector peer-down emits and reparses a native hex command naming only the two remaining IPv4/IPv6 VPN routes, with their original bytes and ADD-PATH framing.
func TestVPNReflectorPeerDownEmitsOnlyRetainedNativeRoutes(t *testing.T) {
	for _, fam := range []family.Family{vpn.IPv4VPN, vpn.IPv6VPN} {
		for _, addPath := range []bool{false, true} {
			t.Run(fam.String()+"/addpath="+strconv.FormatBool(addPath), func(t *testing.T) {
				bridge := rpc.NewDirectBridge()
				pluginEnd, engineEnd := net.Pipe()
				t.Cleanup(func() { _ = engineEnd.Close() })
				p := sdk.NewWithConn("rr-vpn-test", rpc.NewBridgedConn(pluginEnd, bridge))
				t.Cleanup(func() { _ = p.Close() })
				commands := make(chan string, 2)
				bridge.SetUpdateRouteSel(func(_ context.Context, _ *selector.Selector, command string, _ map[string]any) (uint32, uint32, error) {
					commands <- command
					return 0, 2, nil
				})
				bridge.SetReady()
				rr := &routeReflector{plugin: p, withdrawals: make(map[string]map[string]withdrawalInfo)}
				target := rrVPNRoute(fam, addPath, 0, 1, 10, 0x000641)
				first := rrVPNRoute(fam, addPath, 0, 2, 10, 0x000651)
				second := rrVPNRoute(fam, addPath, 17, 1, 11, 0x000661)
				for _, route := range [][]byte{target, first, second} {
					rrApplyVPN(t, rr, fam, addPath, 14, route)
				}
				rrApplyVPN(t, rr, fam, addPath, 15, rrVPNRoute(fam, addPath, 0, 1, 10, 0x800000))
				rr.handleStateDown("192.0.2.1")
				var command string
				select {
				case command = <-commands:
				case <-time.After(5 * time.Second):
					t.Fatal("peer-down did not emit a withdrawal command")
				}
				wantRoutes := []string{hex.EncodeToString(first), hex.EncodeToString(second)}
				slices.Sort(wantRoutes)
				wantCommand := "update hex nlri " + fam.String()
				if addPath {
					wantCommand += " addpath"
				}
				wantCommand += " del " + strings.Join(wantRoutes, " del ")
				if command != wantCommand {
					t.Fatalf("peer-down command = %q, want %q", command, wantCommand)
				}
				parsed, err := update.ParseUpdateWire(strings.Fields(command)[2:], plugin.WireEncodingHex)
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
