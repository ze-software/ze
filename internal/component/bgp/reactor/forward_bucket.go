// Design: docs/architecture/forward-congestion-pool.md -- outbound attribute bucket grouping
// Related: forward_pool.go -- per-destination forward worker pool
// Related: reactor_api_forward.go -- UPDATE forwarding dispatches

package reactor

import (
	"bytes"
	"hash/fnv"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// bucketBodyParts holds the parsed components of an UPDATE body for bucket grouping.
type bucketBodyParts struct {
	wdLen    int
	wd       []byte
	attrLen  int
	attrs    []byte
	nlri     []byte
	attrHash uint64
}

// bucketEligible pairs a batch index with its parsed body parts.
type bucketEligible struct {
	idx   int
	parts bucketBodyParts
}

// bucketScratch holds reusable working buffers for fwdBucketMerge.
type bucketScratch struct {
	eligible []bucketEligible
	groups   map[uint64][]int
	merged   [][]byte
}

var bucketScratchPool = sync.Pool{
	New: func() any {
		return &bucketScratch{
			groups: make(map[uint64][]int),
		}
	},
}

// fwdBucketMerge attempts to merge fwdItems with identical path attributes
// into fewer outbound UPDATEs by packing NLRIs. Items using the rawBodies
// path (zero-copy) are eligible if they share byte-identical path attributes.
// Items using the parsed updates path or with per-peer modifications pass
// through unchanged.
//
// The merge reduces the number of BGP UPDATE messages written to TCP,
// saving per-message header overhead and syscall count.
//
// maxBodySize is the max UPDATE body size (message size - header length).
func fwdBucketMerge(items []fwdItem, maxBodySize int) []fwdItem {
	if len(items) <= 1 {
		return items
	}

	scratch, ok := bucketScratchPool.Get().(*bucketScratch)
	if !ok {
		return items
	}
	defer func() {
		scratch.eligible = scratch.eligible[:0]
		for k := range scratch.groups {
			delete(scratch.groups, k)
		}
		scratch.merged = scratch.merged[:0]
		bucketScratchPool.Put(scratch)
	}()

	eligible := scratch.eligible[:0]
	for i := range items {
		if len(items[i].rawBodies) != 1 || len(items[i].updates) > 0 || items[i].peerBufIdx > 0 {
			continue
		}
		body := items[i].rawBodies[0]
		parts, parsed := parseBucketBody(body)
		if !parsed || parts.wdLen > 0 {
			continue
		}
		eligible = append(eligible, bucketEligible{idx: i, parts: parts})
	}
	scratch.eligible = eligible

	if len(eligible) < 2 {
		return items
	}

	// Group eligible items by attr hash.
	for j := range eligible {
		h := eligible[j].parts.attrHash
		scratch.groups[h] = append(scratch.groups[h], j)
	}

	anyMerged := false
	merged := scratch.merged[:0]
	used := make([]bool, len(items))

	// Track metadata for each merged body (from reference item of each group).
	var mergedMeta []map[string]any

	for _, grp := range scratch.groups {
		if len(grp) < 2 {
			continue
		}
		refAttrs := eligible[grp[0]].parts.attrs
		sameCount := 0
		for _, g := range grp {
			if bytes.Equal(refAttrs, eligible[g].parts.attrs) {
				sameCount++
			}
		}
		if sameCount < 2 {
			continue
		}

		refParts := &eligible[grp[0]].parts
		refMeta := items[eligible[grp[0]].idx].meta
		attrOverhead := 2 + refParts.wdLen + 2 + refParts.attrLen

		var nlriBuf []byte
		for _, g := range grp {
			ep := &eligible[g]
			if !bytes.Equal(refAttrs, ep.parts.attrs) {
				continue
			}
			nlri := ep.parts.nlri
			if len(nlri) == 0 {
				used[ep.idx] = true
				continue
			}
			if len(nlriBuf)+len(nlri)+attrOverhead > maxBodySize && len(nlriBuf) > 0 {
				merged = append(merged, buildBucketBody(refParts, nlriBuf))
				mergedMeta = append(mergedMeta, refMeta)
				nlriBuf = nil
			}
			nlriBuf = append(nlriBuf, nlri...)
			used[ep.idx] = true
			anyMerged = true
		}
		if len(nlriBuf) > 0 {
			merged = append(merged, buildBucketBody(refParts, nlriBuf))
			mergedMeta = append(mergedMeta, refMeta)
		}
	}
	scratch.merged = merged

	if !anyMerged {
		return items
	}

	// Build result: keep consumed items (for done() callbacks) with cleared rawBodies,
	// keep non-consumed items unchanged, then append merged synthetic items.
	result := make([]fwdItem, 0, len(items)+len(merged))
	for i := range items {
		if used[i] {
			stripped := items[i]
			stripped.rawBodies = nil
			result = append(result, stripped)
			continue
		}
		result = append(result, items[i])
	}
	for i, body := range merged {
		result = append(result, fwdItem{
			peer:      items[0].peer,
			rawBodies: [][]byte{body},
			meta:      mergedMeta[i],
		})
	}
	return result
}

// parseBucketBody extracts the components of an UPDATE body for bucket grouping.
func parseBucketBody(body []byte) (bucketBodyParts, bool) {
	var bp bucketBodyParts
	if len(body) < 4 {
		return bp, false
	}
	bp.wdLen = int(body[0])<<8 | int(body[1])
	if 2+bp.wdLen+2 > len(body) {
		return bp, false
	}
	bp.wd = body[2 : 2+bp.wdLen]
	off := 2 + bp.wdLen
	bp.attrLen = int(body[off])<<8 | int(body[off+1])
	attrStart := off + 2
	attrEnd := attrStart + bp.attrLen
	if attrEnd > len(body) {
		return bp, false
	}
	bp.attrs = body[attrStart:attrEnd]
	bp.nlri = body[attrEnd:]

	h := fnv.New64a()
	h.Write(bp.attrs) //nolint:errcheck // fnv never errors
	bp.attrHash = h.Sum64()
	return bp, true
}

// buildBucketBody constructs an UPDATE body from reference attrs and merged NLRIs.
func buildBucketBody(ref *bucketBodyParts, nlri []byte) []byte {
	size := 2 + ref.wdLen + 2 + ref.attrLen + len(nlri)
	buf := make([]byte, size)
	off := 0
	buf[off] = byte(ref.wdLen >> 8)
	buf[off+1] = byte(ref.wdLen)
	off += 2
	copy(buf[off:], ref.wd)
	off += ref.wdLen
	buf[off] = byte(ref.attrLen >> 8)
	buf[off+1] = byte(ref.attrLen)
	off += 2
	copy(buf[off:], ref.attrs)
	off += ref.attrLen
	copy(buf[off:], nlri)
	return buf
}

// fwdBucketMaxBodySize returns the max UPDATE body size for a peer.
func fwdBucketMaxBodySize(extendedMessage bool) int {
	return int(message.MaxMessageLength(message.TypeUPDATE, extendedMessage)) - message.HeaderLen
}
