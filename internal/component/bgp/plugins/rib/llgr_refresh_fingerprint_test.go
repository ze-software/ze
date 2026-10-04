// Design: docs/guide/graceful-restart.md -- fresh UPDATEs replace LLGR-stale attributes.
package rib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// TestLLGRRefreshReplacesMutatedAttributes replays the original received bytes
// after LLGR adds a community. A fingerprint hit must not preserve that mutation.
func TestLLGRRefreshReplacesMutatedAttributes(t *testing.T) {
	t.Parallel()
	r := setupGRTestRIB(t)
	peer := r.bgpPeers[netip.MustParseAddr("192.0.2.1")]
	for _, command := range []struct {
		name string
		args []string
	}{
		{"request bgp rib mark-stale", []string{"192.0.2.1", "0", "2"}},
		{"request bgp rib attach-community", []string{"192.0.2.1", "ipv4/unicast", "ffff0006"}},
	} {
		status, _, err := r.handleCommand(command.name, "*", command.args)
		if err != nil {
			t.Fatal(err)
		}
		if status != statusDone {
			t.Fatalf("%s: %s", command.name, status)
		}
	}
	nlri := []byte{24, 10, 0, 0}
	stale, found := peer.Lookup(family.IPv4Unicast, nlri)
	if !found {
		t.Fatal("retained route missing before refresh")
	}
	if !stale.GetBundle().HasCommunities() {
		t.Fatal("LLGR mutation did not attach its community")
	}
	peer.Insert(family.IPv4Unicast, concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop), nlri)
	fresh, found := peer.Lookup(family.IPv4Unicast, nlri)
	if !found {
		t.Fatal("fresh route missing")
	}
	if fresh.StaleLevel != storage.StaleLevelFresh {
		t.Fatalf("refreshed route level=%d", fresh.StaleLevel)
	}
	if fresh.GetBundle().HasCommunities() {
		t.Fatal("original-wire fingerprint kept LLGR_STALE on the fresh UPDATE")
	}
	control, found := peer.Lookup(family.IPv6Unicast, []byte{32, 0x20, 0x01, 0x0d, 0xb8})
	if !found {
		t.Fatal("unrefreshed control missing")
	}
	if control.StaleLevel != 2 {
		t.Fatalf("refresh changed unrelated family's stale level=%d", control.StaleLevel)
	}
}

// TestLLGRRefreshFingerprintStorageModes covers the direct, ADD-PATH and opaque
// fingerprint rails, with both raw-byte and parse-once insertion APIs.
func TestLLGRRefreshFingerprintStorageModes(t *testing.T) {
	// Keep storage-mode mutations of the shared attribute pools sequential.
	for _, tc := range []struct {
		name    string
		fam     family.Family
		addPath bool
		nlri    []byte
	}{
		{"direct", family.IPv4Unicast, false, []byte{24, 10, 0, 0}},
		{"add-path", family.IPv4Unicast, true, []byte{0, 0, 0, 7, 24, 10, 0, 0}},
		{"opaque", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, false, []byte{2, 3, 1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, parsed := range []bool{false, true} {
				peer := storage.NewPeerRIB("192.0.2.1")
				t.Cleanup(peer.Release)
				peer.SetAddPath(tc.fam, tc.addPath)
				attrs := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop)
				peer.Insert(tc.fam, attrs, tc.nlri)
				peer.MarkAllStale(2)
				r := newTestRIBManager(t)
				peer.ModifyFamilyAllKeyed(tc.fam, func(_ []byte, entry *storage.RouteEntry) {
					if entry.AttrFingerprint == 0 {
						t.Fatal("original wire did not populate fingerprint")
					}
					if !r.attachCommunity(entry, llgrStaleCommunity) {
						t.Fatal("community mutation failed")
					}
				})
				if parsed {
					entry, fingerprint, length, err := storage.ParseRouteEntry(attrs)
					if err != nil {
						t.Fatal(err)
					}
					peer.InsertEntry(tc.fam, entry, fingerprint, length, tc.nlri)
					entry.Release()
				} else {
					peer.Insert(tc.fam, attrs, tc.nlri)
				}
				fresh, found := peer.Lookup(tc.fam, tc.nlri)
				if !found {
					t.Fatal("refreshed route missing")
				}
				if fresh.GetBundle().HasCommunities() {
					t.Fatalf("parsed=%t: original wire retained locally attached community", parsed)
				}
				if fresh.StaleLevel != storage.StaleLevelFresh {
					t.Fatalf("parsed=%t: route remains stale", parsed)
				}
			}
		})
	}
}
