// Design: docs/architecture/wire/nlri.md -- Compatibility-free route identity.
// RFC naming: untagged -- Shared fixtures for the tagged RFC 8277 route-state carriers.
package rib

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"maps"
	"net/netip"
	"slices"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

func compatibilityRoute(fam family.Family, addPath bool, pathID uint32, rd byte, prefix netip.Prefix, label uint32) []byte {
	route := labeledNLRI(pathID, addPath, prefix, []uint32{label})
	if fam.SAFI != family.SAFIVPN {
		return route
	}
	offset := 0
	if addPath {
		offset = 4
	}
	route[offset] += 64
	out := append([]byte(nil), route[:offset+4]...)
	out = append(out, 0, 0, 0, 0, 0, 0, 0, rd)
	return append(out, route[offset+4:]...)
}

func compatibilityBody(fam family.Family, nextHop netip.Addr, med uint32, route []byte, withdraw bool) []byte {
	value := []byte{0, byte(fam.AFI), byte(fam.SAFI)}
	code := byte(15)
	var attrs []byte
	if !withdraw {
		code = 14
		var hop []byte
		if fam.SAFI == family.SAFIVPN {
			hop = make([]byte, 8)
		}
		hop = append(hop, nextHop.AsSlice()...)
		value = append(value, byte(len(hop)))
		value = append(value, hop...)
		value = append(value, 0)
		attrs = []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, 0x80, 4, 4}
		attrs = binary.BigEndian.AppendUint32(attrs, med)
	}
	value = append(value, route...)
	attrs = append(attrs, 0x80, code, byte(len(value)))
	attrs = append(attrs, value...)
	return append([]byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}, attrs...)
}

func compatibilityWithdraw(route []byte, addPath bool, value [3]byte) []byte {
	out := bytes.Clone(route)
	offset := 1
	if addPath {
		offset += 4
	}
	copy(out[offset:offset+3], value[:])
	return out
}

// compatibilityRouteState checks the exact surviving native routes after every
// real ingest withdrawal, including the replacement published to consumers.
// MUTATION: include Compatibility in the key, drop the Path Identifier, or
// publish Withdraw rather than Update while another path still carries the route.
func compatibilityRouteState(t *testing.T, safi family.SAFI, addPath bool) {
	t.Helper()
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		fam := family.Family{AFI: afi, SAFI: safi}
		prefix := netip.MustParsePrefix("10.0.0.0/8")
		sibling := netip.MustParsePrefix("11.0.0.0/8")
		hop := netip.MustParseAddr("10.0.0.1")
		if afi == family.AFIIPv6 {
			prefix = netip.MustParsePrefix("2001:db8::/32")
			sibling = netip.MustParsePrefix("2001:db9::/32")
			hop = netip.MustParseAddr("2001:db8::1")
		}
		for _, tc := range labeledWithdrawCompatibilities {
			t.Run(fam.String()+"/addpath="+strconv.FormatBool(addPath)+"/"+tc.name, func(t *testing.T) {
				bus := newTestEventBus()
				r := newTestRIBManagerWithBus(bus)
				peer := netip.MustParseAddr("192.0.2.21")
				r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
				ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: addPath}))
				if err != nil {
					t.Fatal(err)
				}
				routes := [][]byte{compatibilityRoute(fam, addPath, 0, 1, prefix, 100)}
				if addPath {
					routes = append(routes, compatibilityRoute(fam, true, 17, 1, prefix, 101))
				}
				routes = append(routes, compatibilityRoute(fam, addPath, 0, 1, sibling, 102))
				if safi == family.SAFIVPN {
					routes = append(routes, compatibilityRoute(fam, addPath, 0, 2, prefix, 103))
				}
				want := make(map[string]bool)
				for i, route := range routes {
					feedReceived(r, peer, ctxID, compatibilityBody(fam, hop, uint32(100+i), route, false))
					want[hex.EncodeToString(route)] = true
				}
				assertCompatibilityRoutes(t, r, peer, fam, addPath, want)
				for i, route := range routes {
					before := len(vpnBestChanges(bus, fam))
					feedReceived(r, peer, ctxID, compatibilityBody(fam, hop, 0, compatibilityWithdraw(route, addPath, tc.value), true))
					delete(want, hex.EncodeToString(route))
					assertCompatibilityRoutes(t, r, peer, fam, addPath, want)
					changes := vpnBestChanges(bus, fam)
					if len(changes) != before+1 {
						t.Fatalf("withdrawal published %d changes, want exactly one", len(changes)-before)
					}
					action := routeaction.Withdraw
					winner := route
					if addPath && i == 0 {
						action = routeaction.Update
						winner = routes[1]
					}
					assertCompatibilityChange(t, changes[before], fam, addPath, action, winner)
				}
			})
		}
	}
}

func assertCompatibilityRoutes(t *testing.T, r *RIBManager, peer netip.Addr, fam family.Family, addPath bool, want map[string]bool) {
	t.Helper()
	got := make(map[string]bool)
	r.bgpPeers[peer].IterateFamily(fam, func(route []byte, _ storage.RouteEntry) bool {
		if fam.SAFI == family.SAFIMPLSLabel {
			offset := 0
			if addPath {
				offset = 4
			}
			labels := pool.ResolveLabels(r.bgpPeers[peer].LookupLabels(fam, route))
			if len(labels) != 1 {
				t.Fatalf("survivor %x labels = %v, want one original label", route, labels)
			}
			wire := append([]byte(nil), route[:offset]...)
			wire = append(wire, route[offset]+24, byte(labels[0]>>12), byte(labels[0]>>4), byte(labels[0]<<4)|1)
			route = append(wire, route[offset+1:]...)
		}
		got[hex.EncodeToString(route)] = true
		return true
	})
	if !maps.Equal(got, want) {
		t.Fatalf("native surviving routes = %v, want exactly %v", got, want)
	}
}

func assertCompatibilityChange(t *testing.T, change ribevents.BestChangeEntry, fam family.Family, addPath bool, action routeaction.Action, route []byte) {
	t.Helper()
	if change.Action != action {
		t.Fatalf("action = %v, want %v", change.Action, action)
	}
	if change.AddPath != addPath {
		t.Fatalf("published ADD-PATH = %v, want %v", change.AddPath, addPath)
	}
	if addPath && change.PathID != binary.BigEndian.Uint32(route[:4]) {
		t.Fatalf("published PathID = %d, want %d", change.PathID, binary.BigEndian.Uint32(route[:4]))
	}
	if fam.SAFI == family.SAFIVPN {
		if !bytes.Equal(change.NLRI, route) {
			t.Fatalf("published native identity = %x, want %x", change.NLRI, route)
		}
		return
	}
	offset := 0
	if addPath {
		offset = 4
	}
	var address [16]byte
	copy(address[:], route[offset+4:])
	addr := netip.AddrFrom16(address)
	if fam.AFI == family.AFIIPv4 {
		addr = netip.AddrFrom4([4]byte(address[:4]))
	}
	want := netip.PrefixFrom(addr, int(route[offset])-24)
	if change.Prefix != want {
		t.Fatalf("published prefix = %v, want %v", change.Prefix, want)
	}
	if action == routeaction.Update {
		label := uint32(route[offset+1])<<12 | uint32(route[offset+2])<<4 | uint32(route[offset+3])>>4
		if !slices.Equal(change.Labels, []uint32{label}) {
			t.Fatalf("replacement labels = %v, want [%d]", change.Labels, label)
		}
	}
}

// compatibilityVPNPromotion withdraws the winner from one source and requires
// exactly the other source's native path, including its own Path Identifier.
func compatibilityVPNPromotion(t *testing.T) {
	t.Helper()
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		fam := family.Family{AFI: afi, SAFI: family.SAFIVPN}
		prefix := netip.MustParsePrefix("10.0.0.0/8")
		hopA := netip.MustParseAddr("10.0.0.1")
		hopB := netip.MustParseAddr("10.0.0.2")
		if afi == family.AFIIPv6 {
			prefix = netip.MustParsePrefix("2001:db8::/32")
			hopA = netip.MustParseAddr("2001:db8::1")
			hopB = netip.MustParseAddr("2001:db8::2")
		}
		for _, addPath := range []bool{false, true} {
			for _, tc := range labeledWithdrawCompatibilities {
				t.Run(fam.String()+"/addpath="+strconv.FormatBool(addPath)+"/"+tc.name, func(t *testing.T) {
					bus := newTestEventBus()
					r := newTestRIBManagerWithBus(bus)
					peA := netip.MustParseAddr("192.0.2.1")
					peB := netip.MustParseAddr("192.0.2.2")
					r.peerMeta[peA] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
					r.peerMeta[peB] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
					ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: addPath}))
					if err != nil {
						t.Fatal(err)
					}
					a := compatibilityRoute(fam, addPath, 0, 1, prefix, 100)
					b := compatibilityRoute(fam, addPath, 17, 1, prefix, 101)
					feedReceived(r, peA, ctxID, compatibilityBody(fam, hopA, 100, a, false))
					feedReceived(r, peB, ctxID, compatibilityBody(fam, hopB, 200, b, false))
					before := vpnBestChanges(bus, fam)
					if len(before) != 1 {
						t.Fatalf("initial changes = %d, want only A's add", len(before))
					}
					assertCompatibilityChange(t, before[0], fam, addPath, routeaction.Add, a)
					feedReceived(r, peA, ctxID, compatibilityBody(fam, hopA, 0, compatibilityWithdraw(a, addPath, tc.value), true))
					assertCompatibilityRoutes(t, r, peA, fam, addPath, map[string]bool{})
					assertCompatibilityRoutes(t, r, peB, fam, addPath, map[string]bool{hex.EncodeToString(b): true})
					changes := vpnBestChanges(bus, fam)
					if len(changes) != len(before)+1 {
						t.Fatalf("promotion changes = %d, want one", len(changes)-len(before))
					}
					last := changes[len(before)]
					assertCompatibilityChange(t, last, fam, addPath, routeaction.Update, b)
					if last.NextHop != hopB {
						t.Fatalf("replacement next hop = %v, want %v", last.NextHop, hopB)
					}
					if got := bestRecordCount(r, fam); got != 1 {
						t.Fatalf("best records = %d, want one replacement", got)
					}
				})
			}
		}
	}
}
