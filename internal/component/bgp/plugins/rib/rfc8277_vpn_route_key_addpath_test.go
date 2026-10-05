package rib

import (
	"bytes"
	"net/netip"
	"testing"
)

// TestRFC8277VPNAddPathLabelsAreOneElection stores two paths of RD 0:1
// 10.0.0.0/8 on one ADD-PATH session, path 7 under label 100 with MED 10 and
// path 9 under label 101 with MED 20, then relabels path 9 and withdraws it
// with the Compatibility value 0x800000.
//
// VALIDATES: both paths are candidates of one election whichever path's NLRI
// starts the lookup, and MED 10 wins with its own label; a relabel of path 9
// replaces that path; the withdrawal removes path 9 and leaves path 7.
// PREVENTS: the label in the route key, which split the two paths into two
// routes, kept both labels of path 9, and let the withdrawal remove nothing.
//
// RFC requirement: RFC8277-3.1-1 positive -- two VPN paths of one RD and prefix on one ADD-PATH session, under different path identifiers and labels, are both candidates of the one selection, and the MED 10 path is selected with its own label whichever path's NLRI starts the lookup.
func TestRFC8277VPNAddPathLabelsAreOneElection(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	pe := netip.MustParseAddr("192.0.2.1")
	r.peerMeta[pe] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	peer := newOpaqueAddPathPeer(r, pe, vpnv4Family)

	path7 := framedKey(7, vpnv4NLRI(100, vpnRouteKeyRD, 0x0a))
	path9 := framedKey(9, vpnv4NLRI(101, vpnRouteKeyRD, 0x0a))
	peer.Insert(vpnv4Family, unicastAttrs([4]byte{10, 0, 0, 1}, 10, 100), path7)
	peer.Insert(vpnv4Family, unicastAttrs([4]byte{10, 0, 0, 1}, 20, 100), path9)

	for _, start := range [][]byte{path7, path9} {
		candidates := gatherCandidatesHeld(t, r, vpnv4Family, start, true)
		if len(candidates) != 2 {
			t.Fatalf("candidates from %x = %d, want 2: one route under two labels", start, len(candidates))
		}
		best := SelectBest(candidates)
		if best == nil || best.PathID != 7 || best.MED != 10 {
			t.Fatalf("best from %x = %+v, want path 7 MED 10", start, best)
		}
		if got := framedRouteNLRI(nil, best.Route, best.PathID, best.AddPath); !bytes.Equal(got, path7) {
			t.Fatalf("winner NLRI = %x, want path 7's own %x", got, path7)
		}
	}

	relabeled := framedKey(9, vpnv4NLRI(300, vpnRouteKeyRD, 0x0a))
	peer.Insert(vpnv4Family, unicastAttrs([4]byte{10, 0, 0, 1}, 20, 100), relabeled)
	if got := peer.FamilyLen(vpnv4Family); got != 2 {
		t.Fatalf("paths after relabeling path 9 = %d, want 2", got)
	}

	compat := framedKey(9, vpnv4NLRI(0, vpnRouteKeyRD, 0x0a))
	compat[5], compat[6], compat[7] = 0x80, 0x00, 0x00
	if !peer.Withdraw(vpnv4Family, compat) {
		t.Fatalf("withdrawal of path 9 with Compatibility 0x800000 removed nothing")
	}
	if _, ok := peer.Lookup(vpnv4Family, path7); !ok {
		t.Fatalf("path 7 lost when path 9 was withdrawn")
	}
	if got := peer.FamilyLen(vpnv4Family); got != 1 {
		t.Fatalf("paths after the withdrawal = %d, want 1", got)
	}
}
