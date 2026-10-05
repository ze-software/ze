// Design: docs/guide/graceful-restart.md -- received ownership drives sent LLGR lifecycle.
package rib

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// TestLLGRSentLifecycleOwnership checks received-generation ownership through
// community decoration and every purge path, including dispatched withdrawals.
func TestLLGRSentLifecycleOwnership(t *testing.T) {
	for _, tc := range []struct {
		name         string
		fam          family.Family
		wire, stored []byte
	}{
		{"ipv4", family.IPv4Unicast, []byte{24, 192, 0, 2}, []byte{24, 192, 0, 2}},
		{"ipv6", family.IPv6Unicast, []byte{32, 32, 1, 13, 184}, []byte{32, 32, 1, 13, 184}},
		{"labeled", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}, []byte{48, 0, 6, 65, 192, 0, 2}, []byte{24, 192, 0, 2}},
		{"evpn", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, nativeEVPNRoute(), nativeEVPNRoute()},
		{"mvpn", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN}, []byte{1, 12, 0, 0, 0, 0, 0, 0, 0, 1, 192, 0, 2, 1}, []byte{1, 12, 0, 0, 0, 0, 0, 0, 0, 1, 192, 0, 2, 1}},
		{"flowspec", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, []byte{5, 1, 24, 192, 0, 2}, []byte{5, 1, 24, 192, 0, 2}},
		{"rtc", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIRTC}, []byte{96, 0, 0, 0xfd, 0xe8, 0, 2, 0xfd, 0xe8, 0, 0, 0, 1}, []byte{96, 0, 0, 0xfd, 0xe8, 0, 2, 0xfd, 0xe8, 0, 0, 0, 1}},
	} {
		for _, operation := range []string{"purge", "no-llgr", "expiry", "release"} {
			t.Run(tc.name+"/"+operation, func(t *testing.T) {
				r := newTestRIBManager(t)
				source := netip.MustParseAddr("192.0.2.10")
				dest := netip.MustParseAddr("192.0.2.20")
				control := netip.MustParseAddr("192.0.2.21")
				peer := storage.NewPeerRIB(source.String())
				r.bgpPeers[source] = peer
				t.Cleanup(func() {
					peer.Release()
					for _, families := range r.ribOut {
						for _, routes := range families {
							for _, entry := range routes {
								entry.release()
							}
						}
					}
				})
				peer.SetAddPath(tc.fam, true)
				in := append([]byte{0, 0, 0, 7}, tc.stored...)
				attrs := nativeSentAttrs()
				if operation == "no-llgr" {
					attrs = appendAttr(attrs, 8, 0xc0, []byte{255, 255, 0, 7})
				}
				peer.Insert(tc.fam, attrs, in)
				peer.ModifyFamilyEntry(tc.fam, in, func(e *storage.RouteEntry) { e.MsgID = 91 })
				// Egress identifier zero is unrelated to ingress identifier seven.
				out := append([]byte{0, 0, 0, 0}, tc.wire...)
				event := nativeSentEvent(t, dest, tc.fam, out, attrs, true, false)
				event.RouteMeta["source-message-id"] = float64(91)
				r.handleSent(event)
				other := nativeSentEvent(t, control, tc.fam, out, attrs, true, false)
				other.RouteMeta["source-peer"] = "192.0.2.11"
				r.handleSent(other)
				key, ok := ribOutRouteKey(tc.fam, out, true)
				if !ok {
					t.Fatal("key rejected")
				}
				original := r.ribOut[control][tc.fam][key].AttrHandle
				if _, _, err := r.markStaleCommand([]string{source.String(), "0"}); err != nil {
					t.Fatal(err)
				}
				// A refreshed path of the same prefix and source is a different
				// owner even though ingress and egress path identifiers differ.
				freshIn := append([]byte{0, 0, 0, 8}, tc.stored...)
				peer.Insert(tc.fam, nativeSentAttrs(), freshIn)
				peer.ModifyFamilyEntry(tc.fam, freshIn, func(e *storage.RouteEntry) { e.MsgID = 92 })
				freshOut := append([]byte{0, 0, 0, 99}, tc.wire...)
				fresh := nativeSentEvent(t, dest, tc.fam, freshOut, nativeSentAttrs(), true, false)
				fresh.RouteMeta["source-message-id"] = float64(92)
				r.handleSent(fresh)
				if operation != "no-llgr" {
					if _, _, err := r.attachCommunityCommand([]string{source.String(), tc.fam.String(), "ffff0006"}); err != nil {
						t.Fatal(err)
					}
					entry := r.ribOut[dest][tc.fam][key]
					wire, err := pool.RibOut.Get(entry.AttrHandle)
					if err != nil {
						t.Fatal(err)
					}
					it := attribute.NewAttrIterator(wire)
					found := false
					for code, _, value, ok := it.Next(); ok; code, _, value, ok = it.Next() {
						if code == attribute.AttrCommunity && containsCommunity(value, []byte{255, 255, 0, 6}) {
							found = true
						}
					}
					if !found || entry.StaleLevel != 2 {
						t.Fatal("sent owned attributes did not enter LLGR")
					}
					wantAttrs := appendAttr(bytes.Clone(attrs), 8, 0xc0, []byte{255, 255, 0, 6})
					if !bytes.Equal(wire, wantAttrs) {
						t.Fatal("decoration changed unrelated wire attributes")
					}
					unchanged, err := pool.RibOut.Get(original)
					if err != nil || !bytes.Equal(unchanged, attrs) {
						t.Fatal("mutation damaged shared attributes")
					}
					// A stale replay snapshot must not roll back current attrs.
					event.RouteMeta["replay"] = true
					r.handleSent(event)
					delete(event.RouteMeta, "replay")
					if r.ribOut[dest][tc.fam][key].AttrHandle != entry.AttrHandle {
						t.Fatal("replay rolled back locally decorated attrs")
					}
				}
				removedHandle := r.ribOut[dest][tc.fam][key].AttrHandle
				withdrawals := make(map[string]int)
				r.updateHook = func(command string, meta map[string]any) {
					if meta["rib-lifecycle"] != true {
						t.Fatalf("withdrawal lacks lifecycle feedback guard: %s", command)
					}
					if meta[metaKeyReplay] != true {
						t.Fatalf("withdrawal lacks replay guard: %s", command)
					}
					withdrawals[command]++
				}
				switch operation {
				case "purge":
					_, _, _ = r.purgeStaleCommand([]string{source.String(), tc.fam.String()})
				case "no-llgr":
					_, _, _ = r.deleteWithCommunityCommand([]string{source.String(), tc.fam.String(), "ffff0007"})
				case "expiry":
					r.autoExpireStale(source, r.grState[source])
				case "release":
					r.retainRoutes(source.String(), nil, false)
					r.releaseRoutes(source.String())
				}
				// Pin the command's native bytes and explicit identifier zero
				// independently of the production withdrawal formatter.
				wantWithdraw := "update hex nlri " + tc.fam.String() + " addpath del " + hex.EncodeToString(out)
				wantFreshWithdraw := "update hex nlri " + tc.fam.String() + " addpath del " + hex.EncodeToString(freshOut)
				if key.Prefix.IsValid() {
					wantWithdraw = "update text nlri " + tc.fam.String() + " path-information 0 del " + key.Prefix.String()
					wantFreshWithdraw = "update text nlri " + tc.fam.String() + " path-information 99 del " + key.Prefix.String()
				}
				if withdrawals[wantWithdraw] != 1 {
					t.Fatalf("owned withdrawal %q dispatched %d times: %#v", wantWithdraw, withdrawals[wantWithdraw], withdrawals)
				}
				wantWrites := 1
				if operation == "release" {
					wantWrites = 2
					if withdrawals[wantFreshWithdraw] != 1 {
						t.Fatalf("released fresh withdrawal missing: %#v", withdrawals)
					}
				}
				if len(withdrawals) != wantWrites {
					t.Fatalf("withdrew an unowned route: %#v", withdrawals)
				}
				if _, remains := r.ribOut[dest][tc.fam][key]; remains {
					t.Fatal("purged received ownership survived in sent inventory")
				}
				wantSurvivors := 1
				if operation == "release" {
					wantSurvivors = 0
				}
				if len(r.ribOut[dest][tc.fam]) != wantSurvivors {
					t.Fatal("purge removed another path's refreshed ownership")
				}
				if operation != "no-llgr" {
					if _, err := pool.RibOut.Get(removedHandle); err == nil {
						t.Fatal("purged decorated attribute reference leaked")
					}
				}
				if len(r.ribOut[control][tc.fam]) != 1 {
					t.Fatal("purge removed another source's advertisement")
				}
				if wire, err := pool.RibOut.Get(original); err != nil || !bytes.Equal(wire, attrs) {
					t.Fatal("purge released another destination's shared attributes")
				}
				// Late replay feedback is not authority to recreate removed ownership.
				event.RouteMeta["replay"] = true
				r.handleSent(event)
				if _, remains := r.ribOut[dest][tc.fam][key]; remains {
					t.Fatal("late replay resurrected purged ownership")
				}
			})
		}
	}
}

// TestRetainRoutesLeavesUnretainedWithdrawalsToForwardingOwner separates the
// DOWN owners: RS withdraws families that were not retained; RIB owns later
// purge/expiry withdrawals only for the retained families. Both must prune
// their inventory, but retain-routes must not send a second DOWN withdrawal.
func TestRetainRoutesLeavesUnretainedWithdrawalsToForwardingOwner(t *testing.T) {
	testRetainRoutesSentOwnership(t, true)
}

// TestRetainRoutesLiveSourceWithdrawsPrunedFamilies exercises the registered
// public command while its source remains established. No subsequent DOWN
// forwarding owner exists, so the RIB must send the removed families itself.
func TestRetainRoutesLiveSourceWithdrawsPrunedFamilies(t *testing.T) {
	testRetainRoutesSentOwnership(t, false)
}

func testRetainRoutesSentOwnership(t *testing.T, onDown bool) {
	t.Helper()
	for _, tc := range []struct {
		name string
		fam  family.Family
		wire []byte
	}{
		{"ipv6", family.IPv6Unicast, []byte{32, 32, 1, 13, 184}},
		{"evpn", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, nativeEVPNRoute()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManager(t)
			source := netip.MustParseAddr("192.0.2.10")
			dest := netip.MustParseAddr("192.0.2.20")
			control := netip.MustParseAddr("192.0.2.21")
			peer := storage.NewPeerRIB(source.String())
			r.bgpPeers[source] = peer
			r.peerUp[source] = true
			t.Cleanup(func() {
				peer.Release()
				for _, families := range r.ribOut {
					for _, routes := range families {
						for _, entry := range routes {
							entry.release()
						}
					}
				}
			})
			attrs := nativeSentAttrs()
			v4 := []byte{24, 192, 0, 2}
			for _, route := range []struct {
				fam  family.Family
				wire []byte
			}{
				{family.IPv4Unicast, v4},
				{tc.fam, tc.wire},
			} {
				peer.Insert(route.fam, attrs, route.wire)
				peer.ModifyFamilyEntry(route.fam, route.wire, func(e *storage.RouteEntry) { e.MsgID = 91 })
				event := nativeSentEvent(t, dest, route.fam, route.wire, attrs, false, false)
				event.RouteMeta["source-message-id"] = float64(91)
				r.handleSent(event)
			}
			other := nativeSentEvent(t, control, tc.fam, tc.wire, attrs, false, false)
			other.RouteMeta["source-peer"] = "192.0.2.11"
			r.handleSent(other)
			var commands []string
			r.updateHook = func(command string, _ map[string]any) {
				commands = append(commands, command)
			}
			args := []string{source.String()}
			if onDown {
				args = append(args, "on-down")
			}
			args = append(args, family.IPv4Unicast.String())
			status, _, err := r.handleCommand("request bgp rib retain-routes", "*", args)
			if err != nil || status != statusDone {
				t.Fatalf("retain-routes failed: status %q, error %v", status, err)
			}
			if onDown && len(commands) != 0 {
				t.Fatalf("retain-routes duplicated forwarding-owner DOWN withdrawals: %v", commands)
			}
			if !onDown {
				want := "update hex nlri " + tc.fam.String() + " del " + hex.EncodeToString(tc.wire)
				if tc.fam == family.IPv6Unicast {
					want = "update text nlri ipv6/unicast del 2001:db8::/32"
				}
				if len(commands) != 1 || commands[0] != want {
					t.Fatalf("public retain-routes must withdraw pruned live family exactly once: got %v, want %q", commands, want)
				}
			}
			if len(r.ribOut[dest][tc.fam]) != 0 {
				t.Fatal("unretained sent family survived the DOWN decision")
			}
			if len(r.ribOut[dest][family.IPv4Unicast]) != 1 {
				t.Fatal("retained sent family was removed")
			}
			if len(r.ribOut[control][tc.fam]) != 1 {
				t.Fatal("another source's sent family was removed")
			}
		})
	}
}
