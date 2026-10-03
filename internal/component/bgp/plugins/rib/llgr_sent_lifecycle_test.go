// Design: docs/guide/graceful-restart.md -- received ownership drives sent LLGR lifecycle.
package rib

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

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
				switch operation {
				case "purge":
					_, _, _ = r.purgeStaleCommand([]string{source.String(), tc.fam.String()})
				case "no-llgr":
					_, _, _ = r.deleteWithCommunityCommand([]string{source.String(), tc.fam.String(), "ffff0007"})
				case "expiry":
					r.autoExpireStale(source, r.grState[source])
				case "release":
					r.retainRoutes(source.String(), nil)
					r.releaseRoutes(source.String())
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
