// Design: docs/architecture/plugin/rib-storage-design.md -- best-path candidates
// Related: addpath_best_per_prefix_test.go -- the CIDR twins of these cases
// Related: vpn_bestchange_test.go -- vpnv4NLRI, the VPN-IPv4 key built here

package rib

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// opaqueFamilyCase is one non-CIDR family and the wire key of one of its
// routes, without a path identifier.
type opaqueFamilyCase struct {
	name string
	fam  family.Family
	key  []byte
}

// opaqueFamilyCases are the VPN and EVPN routes every case here drives. The
// VPN key is RFC 4364 Section 4.3.4 VPN-IPv4 10.0.0.0/8 under RD 0:1. The EVPN
// key is an RFC 7432 Section 7.2 MAC/IP Advertisement route: RD 0:1, a zero
// ESI and Ethernet Tag, MAC 00:00:5e:00:53:01, no IP address, label 100.
func opaqueFamilyCases() []opaqueFamilyCase {
	evpn := []byte{
		2, 33, // Route Type 2, length
		0, 0, 0, 0, 0, 0, 0, 1, // RD
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // ESI
		0, 0, 0, 0, // Ethernet Tag
		48, 0x00, 0x00, 0x5e, 0x00, 0x53, 0x01, // MAC Address Length, MAC Address
		0,                // IP Address Length
		0x00, 0x06, 0x41, // MPLS Label1 100
	}
	return []opaqueFamilyCase{
		{name: "vpnv4", fam: vpnv4Family, key: vpnv4NLRI(100, [8]byte{0, 0, 0, 0, 0, 0, 0, 1}, 0x0a)},
		{name: "evpn", fam: family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, key: evpn},
	}
}

// framedKey is key as an ADD-PATH session sends it: the 4-octet path
// identifier first (RFC 7911 Section 3).
func framedKey(pathID uint32, key []byte) []byte {
	framed := []byte{byte(pathID >> 24), byte(pathID >> 16), byte(pathID >> 8), byte(pathID)}
	return append(framed, key...)
}

// newOpaqueAddPathPeer registers an iBGP peer whose fam is stored with
// ADD-PATH.
func newOpaqueAddPathPeer(r *RIBManager, peer netip.Addr, fam family.Family) *storage.PeerRIB {
	r.peerMeta[peer] = &peerMetadata{PeerASN: 65000, LocalASN: 65000}
	rib := storage.NewPeerRIB(peer.String())
	rib.SetAddPath(fam, true)
	r.bgpPeers[peer] = rib
	return rib
}

// bestRecordCount counts the best-path records the RIB holds for fam.
func bestRecordCount(r *RIBManager, fam family.Family) int {
	total := 0
	for _, d := range r.bestPrev.shardDepth(fam) {
		total += d
	}
	return total
}

// TestAddPathOpaqueOneElectionPerRoute checks that two paths of one VPN or
// EVPN route on one ADD-PATH session are elected once.
//
// VALIDATES: AC-11, the VPN and EVPN twins of AC-1: path 7 (MED 10) and path 9
// (MED 20) are both candidates of one selection whichever path key the lookup
// starts from, one best record exists for the route, and the best-change names
// path 7 by its own framed key.
// PREVENTS: one election per path identifier because the identifier stayed in
// the opaque key, the same defect RFC 8277 Section 3.1 rules out for CIDR.
func TestAddPathOpaqueOneElectionPerRoute(t *testing.T) {
	for _, tc := range opaqueFamilyCases() {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManagerWithBus(newTestEventBus())
			rib := newOpaqueAddPathPeer(r, netip.MustParseAddr("192.0.2.31"), tc.fam)

			rib.Insert(tc.fam, unicastAttrs([4]byte{10, 0, 0, 1}, 10, 100), framedKey(7, tc.key))
			first, ok := r.checkBestPathChange(tc.fam, framedKey(7, tc.key), true, nil)
			require.True(t, ok)
			assert.Equal(t, framedKey(7, tc.key), first.NLRI, "the best-change names path 7's own key")
			assert.True(t, first.AddPath)
			assert.Equal(t, uint32(7), first.PathID)

			rib.Insert(tc.fam, unicastAttrs([4]byte{10, 0, 0, 2}, 20, 100), framedKey(9, tc.key))
			_, ok = r.checkBestPathChange(tc.fam, framedKey(9, tc.key), true, nil)
			assert.False(t, ok, "path 9 (MED 20) does not displace path 7 (MED 10)")

			for _, key := range [][]byte{framedKey(7, tc.key), framedKey(9, tc.key)} {
				assert.Len(t, gatherCandidatesHeld(r, tc.fam, key, true), 2, "both paths are candidates (key %x)", key)
			}
			assert.Equal(t, 1, bestRecordCount(r, tc.fam), "one best record for the route")
		})
	}
}

// TestAddPathOpaqueMixedModeMeetInOneElection checks that a VPN or EVPN route
// held by a peer stored without ADD-PATH and by a peer stored with it is one
// route with one election.
//
// VALIDATES: AC-8 twin for the opaque families: the plain peer's route
// (LOCAL_PREF 500) and the ADD-PATH peer's path 7 (LOCAL_PREF 100) are both
// candidates whichever session's key triggers, and the plain peer wins.
// PREVENTS: each peer being asked with the triggering session's wire key, so
// the four path-id octets keep the two sessions' routes from ever meeting.
func TestAddPathOpaqueMixedModeMeetInOneElection(t *testing.T) {
	for _, tc := range opaqueFamilyCases() {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManagerWithBus(newTestEventBus())
			addPathPeer := netip.MustParseAddr("192.0.2.31")
			plainPeer := netip.MustParseAddr("192.0.2.1")
			newOpaqueAddPathPeer(r, addPathPeer, tc.fam).Insert(tc.fam, unicastAttrs([4]byte{10, 0, 0, 1}, 10, 100), framedKey(7, tc.key))
			r.peerMeta[plainPeer] = &peerMetadata{PeerASN: 65000, LocalASN: 65000}
			r.bgpPeers[plainPeer] = storage.NewPeerRIB(plainPeer.String())
			r.bgpPeers[plainPeer].Insert(tc.fam, unicastAttrs([4]byte{10, 0, 0, 9}, 10, 500), tc.key)

			assert.Len(t, gatherCandidatesHeld(r, tc.fam, framedKey(7, tc.key), true), 2, "framed trigger")
			assert.Len(t, gatherCandidatesHeld(r, tc.fam, tc.key, false), 2, "plain trigger")

			best, ok := r.checkBestPathChange(tc.fam, framedKey(7, tc.key), true, nil)
			require.True(t, ok)
			assert.Equal(t, tc.key, best.NLRI, "the plain peer's route wins, named by its own key")
			assert.False(t, best.AddPath)
			_, ok = r.checkBestPathChange(tc.fam, tc.key, false, nil)
			assert.False(t, ok, "the plain trigger finds the same election and the same best")
			assert.Equal(t, 1, bestRecordCount(r, tc.fam), "one best record for the route")
		})
	}
}

// TestAddPathOpaqueWithdrawalKeepsOrPromotes checks withdrawal of each path of
// an ADD-PATH VPN or EVPN route.
//
// VALIDATES: AC-10 twin: withdrawing the non-best path 9 changes nothing;
// withdrawing the best path 7 promotes path 9, and withdrawing path 9 last
// withdraws the route, each change naming its path by its framed key.
// PREVENTS: a withdrawal judged by the withdrawn path id alone.
func TestAddPathOpaqueWithdrawalKeepsOrPromotes(t *testing.T) {
	for _, tc := range opaqueFamilyCases() {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManagerWithBus(newTestEventBus())
			rib := newOpaqueAddPathPeer(r, netip.MustParseAddr("192.0.2.31"), tc.fam)
			rib.Insert(tc.fam, unicastAttrs([4]byte{10, 0, 0, 7}, 10, 100), framedKey(7, tc.key))
			r.checkBestPathChange(tc.fam, framedKey(7, tc.key), true, nil)
			rib.Insert(tc.fam, unicastAttrs([4]byte{10, 0, 0, 9}, 20, 100), framedKey(9, tc.key))
			r.checkBestPathChange(tc.fam, framedKey(9, tc.key), true, nil)

			rib.Remove(tc.fam, framedKey(9, tc.key))
			_, ok := r.checkBestPathChange(tc.fam, framedKey(9, tc.key), true, nil)
			assert.False(t, ok, "withdrawing the non-best path keeps the best")

			rib.Insert(tc.fam, unicastAttrs([4]byte{10, 0, 0, 9}, 20, 100), framedKey(9, tc.key))
			r.checkBestPathChange(tc.fam, framedKey(9, tc.key), true, nil)
			rib.Remove(tc.fam, framedKey(7, tc.key))
			promoted, ok := r.checkBestPathChange(tc.fam, framedKey(7, tc.key), true, nil)
			require.True(t, ok)
			assert.Equal(t, ribevents.BestChangeUpdate, promoted.Action)
			assert.Equal(t, uint32(9), promoted.PathID)
			assert.True(t, bytes.Equal(framedKey(9, tc.key), promoted.NLRI), "promoted NLRI %x", promoted.NLRI)

			rib.Remove(tc.fam, framedKey(9, tc.key))
			gone, ok := r.checkBestPathChange(tc.fam, framedKey(9, tc.key), true, nil)
			require.True(t, ok)
			assert.Equal(t, ribevents.BestChangeWithdraw, gone.Action)
			assert.Equal(t, uint32(9), gone.PathID, "the withdraw names the path that was best")
			assert.Equal(t, framedKey(9, tc.key), gone.NLRI)
			assert.Equal(t, 0, bestRecordCount(r, tc.fam))
		})
	}
}
