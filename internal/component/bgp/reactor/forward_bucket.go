// Design: docs/architecture/forward-congestion-pool.md -- outbound attribute bucket grouping
// Related: forward_pool.go -- per-destination forward worker pool
// Related: reactor_api_forward.go -- UPDATE forwarding dispatches

package reactor

import (
	"bytes"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
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

// bucketEligible is one batch item's parsed body, and whether it can be merged
// at all. An item that cannot is a BARRIER: see fwdBucketMerge.
type bucketEligible struct {
	parts bucketBodyParts
	ok    bool
}

// bucketRun is one span of adjacent, mergeable, identically-attributed items,
// with the packed bodies it produced. bodyStart and bodyEnd index bucketScratch.merged.
type bucketRun struct {
	start     int
	end       int // exclusive
	bodyStart int
	bodyEnd   int // exclusive
}

// bucketScratch holds reusable working buffers for fwdBucketMerge.
type bucketScratch struct {
	parsed []bucketEligible
	runs   []bucketRun
	merged [][]byte
}

var bucketScratchPool = sync.Pool{
	New: func() any { return &bucketScratch{} },
}

// fwdBucketMerge merges ADJACENT fwdItems that carry byte-identical path
// attributes into fewer outbound UPDATEs by packing their NLRIs. An item on the
// parsed-updates path, one with per-peer modifications, one carrying withdrawn
// routes, and one whose body does not parse are all left alone.
//
// The merge reduces the number of BGP UPDATE messages written to TCP,
// saving per-message header overhead and syscall count.
//
// ADJACENT is the whole safety argument, and it is why this does not group by
// attribute hash across the batch. Packing item k's NLRI into a body emitted at
// item i's position MOVES that announcement, and an announcement moved across an
// operation on the SAME prefix inverts the pair: an announce packed past a
// withdraw of its own prefix leaves the peer holding a route that was withdrawn,
// which is the failure the batch is ordered to prevent. Every item that cannot
// join a run is therefore a barrier rather than something to merge over, and a
// run's members are equivalent to merge among themselves: same attributes, no
// withdrawals, so packing their NLRIs into one message says exactly what the
// separate messages said.
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
		scratch.parsed = scratch.parsed[:0]
		scratch.runs = scratch.runs[:0]
		scratch.merged = scratch.merged[:0]
		bucketScratchPool.Put(scratch)
	}()

	parsed := scratch.parsed[:0]
	for i := range items {
		var e bucketEligible
		if len(items[i].rawBodies) == 1 && len(items[i].updates) == 0 && items[i].peerBufIdx == 0 {
			if parts, okParse := parseBucketBody(items[i].rawBodies[0]); okParse && parts.wdLen == 0 {
				e.parts, e.ok = parts, true
			}
		}
		parsed = append(parsed, e)
	}
	scratch.parsed = parsed

	runs := scratch.runs[:0]
	merged := scratch.merged[:0]

	for i := 0; i < len(items); {
		if !parsed[i].ok {
			i++
			continue
		}
		j := i + 1
		for j < len(items) && parsed[j].ok &&
			parsed[j].parts.attrHash == parsed[i].parts.attrHash &&
			bytes.Equal(parsed[j].parts.attrs, parsed[i].parts.attrs) {
			j++
		}
		if j-i < 2 {
			i++
			continue
		}

		ref := &parsed[i].parts
		attrOverhead := 2 + ref.wdLen + 2 + ref.attrLen
		bodyStart := len(merged)
		var nlriBuf []byte
		for k := i; k < j; k++ {
			nlri := parsed[k].parts.nlri
			if len(nlri) == 0 {
				// The prefixes live in the attributes (MP_REACH), so there is
				// nothing to pack and the body must not be dropped.
				continue
			}
			if len(nlriBuf)+len(nlri)+attrOverhead > maxBodySize && len(nlriBuf) > 0 {
				merged = append(merged, buildBucketBody(ref, nlriBuf))
				nlriBuf = nil
			}
			nlriBuf = append(nlriBuf, nlri...)
		}
		if len(nlriBuf) > 0 {
			merged = append(merged, buildBucketBody(ref, nlriBuf))
		}
		if len(merged) > bodyStart {
			runs = append(runs, bucketRun{start: i, end: j, bodyStart: bodyStart, bodyEnd: len(merged)})
		}
		i = j
	}
	scratch.runs = runs
	scratch.merged = merged

	if len(runs) == 0 {
		return items
	}

	// Build the result in batch order: every consumed item stays in place with
	// its rawBodies cleared (its done() and its pooled buffers are still owed),
	// and the run's packed bodies go where the run was.
	result := make([]fwdItem, 0, len(items)+len(merged))
	run := 0
	for i := 0; i < len(items); {
		if run >= len(runs) || i != runs[run].start {
			result = append(result, items[i])
			i++
			continue
		}
		r := runs[run]
		for k := r.start; k < r.end; k++ {
			stripped := items[k]
			stripped.rawBodies = nil
			result = append(result, stripped)
		}
		for b := r.bodyStart; b < r.bodyEnd; b++ {
			result = append(result, fwdItem{
				// Every run member is a real item -- a nil-peer sentinel carries
				// no body, so it never parses and never joins a run.
				peer:          items[r.start].peer,
				rawBodies:     [][]byte{merged[b]},
				meta:          items[r.start].meta,
				sourcePeerStr: items[r.start].sourcePeerStr,
			})
		}
		i = r.end
		run++
	}
	return result
}

const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

// fnvHash computes FNV-1a hash inline without allocating a hash.Hash interface.
func fnvHash(data []byte) uint64 {
	h := uint64(fnvOffset64)
	for _, b := range data {
		h ^= uint64(b)
		h *= fnvPrime64
	}
	return h
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

	bp.attrHash = fnvHash(bp.attrs)
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
	return int(message.MaxMessageLength(msgtype.TypeUPDATE, extendedMessage)) - message.HeaderLen
}
