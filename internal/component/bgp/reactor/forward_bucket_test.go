package reactor

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
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
