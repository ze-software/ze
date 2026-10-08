// Design: docs/architecture/plugin/rib-storage-design.md -- JSON received-route reconciliation.
// RFC naming: untagged -- supplementary JSON-to-RIB withdrawal reconciliation, not an independent whole-clause claim.
package rib

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"slices"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestJSONWithdrawalReconcilesNativeIdentity sends real formatted JSON UPDATE
// events through the parser and dispatcher. Compatibility bytes must affect
// neither removal nor the election, while RD and ADD-PATH boundaries survive.
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field MUST be ignored."
// The test varies those bytes while retaining each native withdrawal identity.
func TestJSONWithdrawalReconcilesNativeIdentity(t *testing.T) {
	for _, fam := range []family.Family{labeledFamily, vpnv4Family} {
		for _, addPath := range []bool{false, true} {
			mode := "base"
			if addPath {
				mode = "add-path-zero"
			}
			for _, compatibility := range labeledWithdrawCompatibilities {
				t.Run(fam.String()+"/"+mode+"/"+compatibility.name, func(t *testing.T) {
					testJSONWithdrawalReconcilesNativeIdentity(t, fam, addPath, compatibility.value)
				})
			}
		}
	}
}

// testJSONWithdrawalReconcilesNativeIdentity observes both the Adj-RIB-In and
// the published Loc-RIB decision after removing the preferred PE's path.
// RFC 8277 Section 2.4: "An explicit withdrawal in a SAFI-x UPDATE on a given BGP session not only withdraws the binding between the prefix and the label(s), it also withdraws the path to that prefix that was previously advertised in a SAFI-x UPDATE on that session."
// Both stored path removal and its resulting election are observed.
func testJSONWithdrawalReconcilesNativeIdentity(t *testing.T, fam family.Family, addPath bool, compatibility [3]byte) {
	t.Helper()
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: addPath}))
	if err != nil {
		t.Fatal(err)
	}
	peA := netip.MustParseAddr("192.0.2.1")
	peB := netip.MustParseAddr("192.0.2.2")
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	nlri := func(label uint32, rd [8]byte, pathID uint32) []byte {
		if fam == labeledFamily {
			return labeledNLRI(pathID, addPath, prefix, []uint32{label})
		}
		wire := vpnv4NLRI(label, rd, 10)
		if addPath {
			wire = append(binary.BigEndian.AppendUint32(nil, pathID), wire...)
		}
		return wire
	}
	announce := func(peer netip.Addr, hop byte, med uint32, wire []byte) {
		body := labeledUpdateBody([4]byte{10, 0, 0, hop}, med, wire)
		if fam == vpnv4Family {
			body = vpnv4AnnounceBody([4]byte{10, 0, 0, hop}, med, wire)
		} else {
			// This JSON fixture is eBGP. Replace the empty AS_PATH with
			// AS_SEQUENCE 65001 so both peers have a known MED neighbor AS.
			// UPDATE header(4), ORIGIN(4), AS_PATH header(3), then value.
			path := []byte{2, 1, 0, 0, 0xfd, 0xe9}
			body = append(body[:11], append(path, body[11:]...)...)
			body[10] = byte(len(path))
			binary.BigEndian.PutUint16(body[2:4], uint16(len(body)-4))
		}
		// RFC 8277 Section 2.4: announcements establish the session's binding.
		feedReceivedJSONWithIdentifier(t, r, peer, 1, ctxID, body)
	}
	withdraw := func(peer netip.Addr, wire []byte) {
		wire = bytes.Clone(wire)
		off := 1
		if addPath {
			off += 4
		}
		copy(wire[off:off+3], compatibility[:])
		body := labeledWithdrawBody(wire)
		if fam == vpnv4Family {
			body = vpnv4WithdrawBody(wire)
		}
		// RFC 8277 Section 2.4: withdraw using different Compatibility bytes.
		feedReceivedJSONWithIdentifier(t, r, peer, 1, ctxID, body)
	}
	first := nlri(100, vpnRouteKeyRD, 0)
	second := nlri(200, vpnRouteKeyRD, 0)
	announce(peA, 1, 10, first)
	announce(peB, 2, 20, second)
	beforeRelabel := len(vpnBestChanges(bus, fam))
	first = nlri(101, vpnRouteKeyRD, 0)
	announce(peA, 1, 10, first)
	relabelChanges := vpnBestChanges(bus, fam)
	if len(relabelChanges) != beforeRelabel+1 {
		t.Fatalf("same-winner relabel published %d changes, want one", len(relabelChanges)-beforeRelabel)
	}
	relabel := relabelChanges[len(relabelChanges)-1]
	if relabel.Action != routeaction.Update || relabel.NextHop != netip.MustParseAddr("10.0.0.1") {
		t.Fatalf("same-winner relabel changed its action or next hop: %+v", relabel)
	}
	if fam == labeledFamily {
		if !slices.Equal(relabel.Labels, []uint32{101}) {
			t.Fatalf("relabel published labels %v, want [101]", relabel.Labels)
		}
		best, ok := loc.Best(fam, prefix)
		if !ok || !slices.Equal(best.Labels, []uint32{101}) {
			t.Fatalf("relabel did not update Loc-RIB labels: %+v, present=%v", best, ok)
		}
	} else if !bytes.Equal(relabel.NLRI, first) {
		t.Fatalf("relabel published NLRI %x, want %x", relabel.NLRI, first)
	}
	remainingA := 0
	remainingBest := 1
	var sibling []byte
	if addPath {
		sibling = nlri(300, vpnRouteKeyRD, 7)
		announce(peA, 3, 30, sibling)
		remainingA++
	}
	if fam == vpnv4Family {
		otherRD := vpnRouteKeyRD
		otherRD[7] = 2
		announce(peA, 4, 1, nlri(400, otherRD, 0))
		remainingA++
		remainingBest++
	}
	before := len(vpnBestChanges(bus, fam))
	withdraw(peA, first)
	if got := r.bgpPeers[peA].FamilyLen(fam); got != remainingA {
		t.Fatalf("PE A Adj-RIB-In = %d, want %d: only the target RD/path may disappear", got, remainingA)
	}
	if got := r.bgpPeers[peB].FamilyLen(fam); got != 1 {
		t.Fatalf("PE B Adj-RIB-In = %d, want 1", got)
	}
	changes := vpnBestChanges(bus, fam)
	if len(changes) != before+1 {
		t.Fatalf("withdrawal published %d changes, want one surviving-PE election", len(changes)-before)
	}
	last := changes[len(changes)-1]
	if last.Action != routeaction.Update || last.NextHop != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("surviving decision = %+v, want update through PE B", last)
	}
	if fam == vpnv4Family && !bytes.Equal(last.NLRI, second) {
		t.Fatalf("surviving NLRI = %x, want PE B's label and path %x", last.NLRI, second)
	}
	if got := bestRecordCount(r, fam); got != remainingBest {
		t.Fatalf("best records = %d, want %d", got, remainingBest)
	}
	if fam == labeledFamily {
		best, ok := loc.Best(fam, prefix)
		if !ok || best.NextHop != netip.MustParseAddr("10.0.0.2") {
			t.Fatalf("Loc-RIB retains withdrawn PE: %+v, present=%v", best, ok)
		}
	}
	withdraw(peA, first)
	if got := len(vpnBestChanges(bus, fam)); got != len(changes) {
		t.Fatalf("duplicate withdrawal published another decision: %d -> %d", len(changes), got)
	}
	if addPath {
		withdraw(peA, sibling)
		if got := len(vpnBestChanges(bus, fam)); got != len(changes) {
			t.Fatalf("nonbest sibling withdrawal changed the best: %d -> %d", len(changes), got)
		}
	}
	withdraw(peB, second)
	changes = vpnBestChanges(bus, fam)
	if len(changes) != before+2 || changes[len(changes)-1].Action != routeaction.Withdraw {
		t.Fatalf("last path did not publish exactly one withdrawal: %+v", changes)
	}
	if got := bestRecordCount(r, fam); got != remainingBest-1 {
		t.Fatalf("stale best records = %d, want %d", got, remainingBest-1)
	}
	if fam == labeledFamily {
		if _, ok := loc.Best(fam, prefix); ok {
			t.Fatal("last withdrawn path remains in Loc-RIB")
		}
	}
	withdraw(peB, second)
	if got := len(vpnBestChanges(bus, fam)); got != len(changes) {
		t.Fatalf("later duplicate withdrawal published again: %d -> %d", len(changes), got)
	}
}
