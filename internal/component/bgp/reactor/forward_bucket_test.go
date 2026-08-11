package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// makeTestBody builds a minimal UPDATE body: [wdLen=0][attrLen][attrs][nlri].
func makeTestBody(attrs, nlri []byte) []byte {
	attrLen := len(attrs)
	body := make([]byte, 2+2+attrLen+len(nlri))
	body[0] = 0
	body[1] = 0
	body[2] = byte(attrLen >> 8)
	body[3] = byte(attrLen)
	copy(body[4:], attrs)
	copy(body[4+attrLen:], nlri)
	return body
}

func TestForwardBucketGroupsIdenticalAttrs(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // origin IGP
	nlri1 := []byte{24, 10, 0, 1}           // 10.0.1.0/24
	nlri2 := []byte{24, 10, 0, 2}           // 10.0.2.0/24
	nlri3 := []byte{24, 10, 0, 3}           // 10.0.3.0/24

	items := []fwdItem{
		{rawBodies: [][]byte{makeTestBody(attrs, nlri1)}},
		{rawBodies: [][]byte{makeTestBody(attrs, nlri2)}},
		{rawBodies: [][]byte{makeTestBody(attrs, nlri3)}},
	}

	maxBody := message.MaxMsgLen - message.HeaderLen
	result := fwdBucketMerge(items, maxBody)

	// Original 3 items should be stripped (rawBodies nil) + 1 merged item.
	mergedCount := 0
	strippedCount := 0
	for _, it := range result {
		if len(it.rawBodies) > 0 {
			mergedCount++
		} else {
			strippedCount++
		}
	}
	if mergedCount != 1 {
		t.Errorf("expected 1 merged item, got %d", mergedCount)
	}
	if strippedCount != 3 {
		t.Errorf("expected 3 stripped items, got %d", strippedCount)
	}

	// The merged body should contain all 3 NLRIs.
	for _, it := range result {
		if len(it.rawBodies) == 0 {
			continue
		}
		body := it.rawBodies[0]
		parts, ok := parseBucketBody(body)
		if !ok {
			t.Fatal("failed to parse merged body")
		}
		if len(parts.nlri) != len(nlri1)+len(nlri2)+len(nlri3) {
			t.Errorf("expected merged NLRI length %d, got %d",
				len(nlri1)+len(nlri2)+len(nlri3), len(parts.nlri))
		}
	}
}

// makeTestWithdrawBody builds a withdraw-only UPDATE body: [wdLen][wd][attrLen=0].
func makeTestWithdrawBody(wd []byte) []byte {
	body := make([]byte, 2+len(wd)+2)
	body[0] = byte(len(wd) >> 8)
	body[1] = byte(len(wd))
	copy(body[2:], wd)
	return body
}

// TestForwardBucketKeepsOrderAcrossAWithdraw pins the ordering half of the merge.
//
// VALIDATES: AC-1 -- an announcement may not be moved past an operation on the
// same prefix. Merging two announcements that sit either side of a withdraw of
// one of those prefixes puts the announce AFTER the withdraw, and the peer ends
// holding a route that was withdrawn.
// PREVENTS: the batch reorder this spec removed returning through the merge,
// which moved every merged body to the end of the batch.
func TestForwardBucketKeepsOrderAcrossAWithdraw(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // origin IGP
	nlriP := []byte{24, 10, 0, 1}           // 10.0.1.0/24
	nlriQ := []byte{24, 10, 0, 2}           // 10.0.2.0/24

	// Announce P, withdraw P, announce Q. P's announce and Q's announce share
	// attributes, so they are the pair a merge wants to pack together.
	items := []fwdItem{
		{rawBodies: [][]byte{makeTestBody(attrs, nlriP)}},
		{rawBodies: [][]byte{makeTestWithdrawBody(nlriP)}},
		{rawBodies: [][]byte{makeTestBody(attrs, nlriQ)}},
	}

	result := fwdBucketMerge(items, message.MaxMsgLen-message.HeaderLen)

	announceP, withdrawP := -1, -1
	for i, it := range result {
		for _, body := range it.rawBodies {
			parts, ok := parseBucketBody(body)
			if !ok {
				t.Fatalf("result item %d has an unparseable body", i)
			}
			if containsNLRI(parts.nlri, nlriP) && announceP < 0 {
				announceP = i
			}
			if containsNLRI(parts.wd, nlriP) {
				withdrawP = i
			}
		}
	}
	if announceP < 0 || withdrawP < 0 {
		t.Fatalf("both operations on 10.0.1.0/24 must survive the merge (announce=%d withdraw=%d)",
			announceP, withdrawP)
	}
	if announceP > withdrawP {
		t.Errorf("the announce of 10.0.1.0/24 must stay BEFORE its withdraw (announce=%d withdraw=%d)",
			announceP, withdrawP)
	}
}

// containsNLRI reports whether a prefix encoding appears in an NLRI section.
func containsNLRI(section, prefix []byte) bool {
	return bytes.Contains(section, prefix)
}

// bucketMPReachAttrs is one MP_REACH_NLRI attribute announcing 2001:db8::/64 with
// next hop 2001:db8::1, and bucketMPUnreachAttrs is one MP_UNREACH_NLRI attribute
// withdrawing the same prefix. Both carry their prefixes INSIDE the attribute,
// so the body that holds them has an empty legacy NLRI section.
var (
	bucketMPReachAttrs = []byte{
		0x80, 14, 30, // optional, MP_REACH_NLRI, length 30
		0x00, 0x02, // AFI = IPv6
		0x01,                                                          // SAFI = unicast
		0x10,                                                          // next-hop length 16
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01, // 2001:db8::1
		0x00,                                     // reserved
		0x40, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, // NLRI = 2001:db8::/64
	}
	bucketMPUnreachAttrs = []byte{
		0x80, 15, 12, // optional, MP_UNREACH_NLRI, length 12
		0x00, 0x02, // AFI = IPv6
		0x01,                                     // SAFI = unicast
		0x40, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, // withdrawn = 2001:db8::/64
	}
)

// countBodiesWithAttrs reports how many UPDATE bodies in a merge result carry
// exactly this attribute section.
func countBodiesWithAttrs(t *testing.T, items []fwdItem, attrs []byte) int {
	t.Helper()
	count := 0
	for i, it := range items {
		for _, body := range it.rawBodies {
			parts, ok := parseBucketBody(body)
			if !ok {
				t.Fatalf("result item %d has an unparseable body", i)
			}
			if bytes.Equal(parts.attrs, attrs) {
				count++
			}
		}
	}
	return count
}

// TestForwardBucketKeepsMPOnlyBodies pins the third defect the merge carried: a
// run whose members pack no NLRI was consumed anyway, and the UPDATEs it held
// left the batch with nothing emitted in their place.
//
// An MP_REACH or MP_UNREACH body carries its prefixes in the attributes, so its
// legacy NLRI section is empty and there is nothing for the packer to move. Two
// such bodies with byte-identical attributes still form a run, and a run that
// emits no packed body must consume nothing.
//
// VALIDATES: AC-1 -- no forwarded UPDATE is dropped by the merge. The legacy
// pair still merges, which is what makes the batch take the rebuild path at all.
// PREVENTS: the silent loss of every IPv6 (or any MP-encoded) UPDATE forwarded
// in a batch where some other group merged. The peer never learns the prefix,
// and nothing on the rail reports it as undelivered.
func TestForwardBucketKeepsMPOnlyBodies(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // origin IGP
	nlri1 := []byte{24, 10, 0, 1}           // 10.0.1.0/24
	nlri2 := []byte{24, 10, 0, 2}           // 10.0.2.0/24

	items := []fwdItem{
		// A legacy pair that merges, which is what defect 3 needed to show: a
		// batch where nothing merges returns its input untouched, so the two
		// assertions below would pass on an early return.
		{rawBodies: [][]byte{makeTestBody(attrs, nlri1)}},
		{rawBodies: [][]byte{makeTestBody(attrs, nlri2)}},
		// Two MP_REACH-only UPDATEs, then two MP_UNREACH-only ones. Each pair
		// is adjacent and identically attributed, so each is one run.
		{rawBodies: [][]byte{makeTestBody(bucketMPReachAttrs, nil)}},
		{rawBodies: [][]byte{makeTestBody(bucketMPReachAttrs, nil)}},
		{rawBodies: [][]byte{makeTestBody(bucketMPUnreachAttrs, nil)}},
		{rawBodies: [][]byte{makeTestBody(bucketMPUnreachAttrs, nil)}},
	}

	result := fwdBucketMerge(items, message.MaxMsgLen-message.HeaderLen)

	if got := countBodiesWithAttrs(t, result, bucketMPReachAttrs); got != 2 {
		t.Errorf("both MP_REACH UPDATEs must survive the merge, got %d of 2", got)
	}
	if got := countBodiesWithAttrs(t, result, bucketMPUnreachAttrs); got != 2 {
		t.Errorf("both MP_UNREACH UPDATEs must survive the merge, got %d of 2", got)
	}

	// The legacy pair still merges into one body carrying both prefixes, so the
	// two assertions above are not passing on a merge that never ran.
	if got := countBodiesWithAttrs(t, result, attrs); got != 1 {
		t.Fatalf("the legacy pair must merge into 1 body, got %d", got)
	}
	for _, it := range result {
		for _, body := range it.rawBodies {
			parts, ok := parseBucketBody(body)
			if !ok || !bytes.Equal(parts.attrs, attrs) {
				continue
			}
			if !containsNLRI(parts.nlri, nlri1) || !containsNLRI(parts.nlri, nlri2) {
				t.Errorf("the merged legacy body must carry both prefixes, got %x", parts.nlri)
			}
		}
	}
}

func TestForwardBucketFlushesAtMessageLimit(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	// Each NLRI is 4 bytes. Attrs overhead = 2(wd_len) + 2(attr_len) + 4(attrs) = 8 bytes.
	// Max body = 20 bytes -> 12 bytes available for NLRI -> 3 NLRIs per message.
	nlri1 := []byte{24, 10, 0, 1}
	nlri2 := []byte{24, 10, 0, 2}
	nlri3 := []byte{24, 10, 0, 3}
	nlri4 := []byte{24, 10, 0, 4}

	items := []fwdItem{
		{rawBodies: [][]byte{makeTestBody(attrs, nlri1)}},
		{rawBodies: [][]byte{makeTestBody(attrs, nlri2)}},
		{rawBodies: [][]byte{makeTestBody(attrs, nlri3)}},
		{rawBodies: [][]byte{makeTestBody(attrs, nlri4)}},
	}

	maxBody := 20 // Forces split after 3 NLRIs
	result := fwdBucketMerge(items, maxBody)

	mergedCount := 0
	for _, it := range result {
		if len(it.rawBodies) > 0 {
			mergedCount++
		}
	}
	if mergedCount != 2 {
		t.Errorf("expected 2 merged items (split at limit), got %d", mergedCount)
	}
}

func TestForwardBucketBypassesModifiedPeer(t *testing.T) {
	attrs1 := []byte{0x40, 0x01, 0x01, 0x00}
	attrs2 := []byte{0x40, 0x01, 0x01, 0x02} // different origin
	nlri1 := []byte{24, 10, 0, 1}
	nlri2 := []byte{24, 10, 0, 2}

	items := []fwdItem{
		{rawBodies: [][]byte{makeTestBody(attrs1, nlri1)}},
		{rawBodies: [][]byte{makeTestBody(attrs2, nlri2)}},
	}

	maxBody := message.MaxMsgLen - message.HeaderLen
	result := fwdBucketMerge(items, maxBody)

	// Different attrs -- no merge should happen.
	if len(result) != 2 {
		t.Errorf("expected 2 items (no merge), got %d", len(result))
	}
	for _, it := range result {
		if len(it.rawBodies) != 1 {
			t.Error("expected each item to keep its rawBody")
		}
	}
}

func TestForwardBucketSingleItem(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	nlri := []byte{24, 10, 0, 1}
	items := []fwdItem{
		{rawBodies: [][]byte{makeTestBody(attrs, nlri)}},
	}
	result := fwdBucketMerge(items, 4096)
	if len(result) != 1 {
		t.Errorf("expected 1 item unchanged, got %d", len(result))
	}
}

func TestForwardBucketSkipsUpdatePath(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	nlri := []byte{24, 10, 0, 1}
	items := []fwdItem{
		{rawBodies: [][]byte{makeTestBody(attrs, nlri)}, updates: []*message.Update{{}}},
		{rawBodies: [][]byte{makeTestBody(attrs, nlri)}},
	}
	result := fwdBucketMerge(items, 4096)
	// First item ineligible (has updates), only 1 eligible -- no merge.
	if len(result) != 2 {
		t.Errorf("expected 2 items (no merge), got %d", len(result))
	}
}

func BenchmarkForwardBucketDrain(b *testing.B) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x04, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}
	items := make([]fwdItem, 20)
	for i := range items {
		nlri := []byte{24, 10, byte(i), 0}
		items[i] = fwdItem{rawBodies: [][]byte{makeTestBody(attrs, nlri)}}
	}
	maxBody := message.MaxMsgLen - message.HeaderLen

	b.ResetTimer()
	for range b.N {
		fwdBucketMerge(items, maxBody)
	}
}
