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
		if items[i].authority == adjOutUnspecified && items[i].provenance == nil && items[i].recovery == nil && items[i].sourceMessageID == 0 && len(items[i].rawBodies) == 1 && len(items[i].updates) == 0 && items[i].peerBufIdx == 0 {
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
			items[j].sourcePeerStr == items[i].sourcePeerStr &&
			items[j].sourceMessageID == items[i].sourceMessageID &&
			items[j].receivedPeer == items[i].receivedPeer &&
			items[j].receivedGeneration == items[i].receivedGeneration &&
			items[j].sourcePolicyEpoch == items[i].sourcePolicyEpoch &&
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
				peer:               items[r.start].peer,
				rawBodies:          [][]byte{merged[b]},
				meta:               items[r.start].meta,
				sourcePeerStr:      items[r.start].sourcePeerStr,
				sourceMessageID:    items[r.start].sourceMessageID,
				receivedPeer:       items[r.start].receivedPeer,
				receivedGeneration: items[r.start].receivedGeneration,
				sourcePolicyEpoch:  items[r.start].sourcePolicyEpoch,
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

// recoveryBucketMerge returns the sections of a recovery admission that admit
// decided to write, in staging order, with ADJACENT sections packed into
// shared UPDATEs. Recovery stages one UPDATE per owed key, so without the
// pack a burst of owed keys reaches the destination as one UPDATE per
// prefix, where the ordinary rail forwards the source's single multi-prefix
// UPDATE.
//
// Announcements of one candidate source's same received message (session
// generation and message ID equal) with byte-identical attributes and the
// same writer authority, session and AIGP send state pack. fwdBucketMerge
// never sees these items: it takes only unowned forwards (authority
// adjOutUnspecified, sourceMessageID 0, no recovery), because the item it
// emits carries only the run head's peer, metadata and source fields. Here
// a section is a full copy of its first member, every field its write
// consults must be equal to join, and each key's ownership is settled per
// staged key by the admission after the flush, never read from the section.
// A withdrawal stays alone. Every owed key of an admission is a distinct
// key, so a pack never moves one operation across another on the same
// prefix. A section with provenance (an ADD-PATH source), more than one
// UPDATE, an AIGP replay or MP framing stays alone too. A packed section is
// written as one raw body.
//
// staged[k] holds the admission item indexes section k writes, so a source
// that ends between admit and the write re-owes every key the section
// carries. A packed body is a fresh copy of the staged pieces, so a returned
// section releases nothing: the admission's own items keep their references
// until releaseItem. Called on the destination's forward worker under
// writeMu, after admit; not safe for concurrent use.
//
// RFC 4271 Section 4.3: "An UPDATE message is used to advertise feasible
// routes that share common path attributes to a peer, or to withdraw
// multiple unfeasible routes from service (see 3.1)."
//
// maxBodySize bounds each packed body. It is the negotiated message size
// less the header (fwdBucketMaxBodySize):
// RFC 4271 Section 4: "The maximum message size is 4096 octets."
// RFC 8654 Section 2: "BGP Extended Messages have a maximum message size of
// 65,535 octets."
func recoveryBucketMerge(admission *recoveryAdmission, maxBodySize int) (sections []fwdItem, staged [][]int) {
	var lead bucketBodyParts // Parts of the last section's first member.
	var packed []byte        // The pieces the last section packs so far.
	leadPacks := false
	for j := range admission.items {
		if !admission.writes(j) {
			continue
		}
		item := &admission.items[j]
		parts, piece, packs := recoveryPackParts(item)
		last := len(sections) - 1
		// RFC 4271 Section 4.3: one UPDATE carries the routes that share
		// these path attributes. RFC 4271 Section 4 and RFC 8654 Section 2
		// bound the packed body: 4 is the two length fields around the
		// attributes.
		if packs && leadPacks && recoveryPackJoins(&sections[last], item, &lead, &parts) &&
			4+lead.attrLen+len(packed)+len(piece) <= maxBodySize {
			packed = append(packed, piece...)
			staged[last] = append(staged[last], j)
			continue
		}
		if last >= 0 && len(staged[last]) > 1 {
			sections[last].rawBodies = [][]byte{buildBucketBody(&lead, packed)}
			sections[last].updates = nil
		}
		sections = append(sections, *item)
		staged = append(staged, []int{j})
		lead, leadPacks = parts, packs
		packed = append(packed[:0], piece...)
	}
	if last := len(sections) - 1; last >= 0 && len(staged[last]) > 1 {
		sections[last].rawBodies = [][]byte{buildBucketBody(&lead, packed)}
		sections[last].updates = nil
	}
	return sections, staged
}

// recoveryPackParts parses a recovery section's body and reports whether it
// can pack, with the piece it contributes: the NLRI field of an announcement
// that carries no withdrawn routes.
func recoveryPackParts(item *fwdItem) (bucketBodyParts, []byte, bool) {
	var none bucketBodyParts
	if !recoveryPackShape(item) {
		return none, nil, false
	}
	parts, ok := recoveryPackBody(item)
	if !ok {
		return none, nil, false
	}
	if parts.wdLen != 0 {
		return none, nil, false
	}
	if len(parts.nlri) == 0 {
		return none, nil, false // MP framing: the prefixes live in the attributes.
	}
	return parts, parts.nlri, true
}

// recoveryPackJoins reports whether item says the same thing as the section
// it would join: the same writer authority and session, the same candidate
// source revision, the same AIGP send state, and the same attributes.
func recoveryPackJoins(section, item *fwdItem, lead, parts *bucketBodyParts) bool {
	same := section.authority == item.authority &&
		section.session == item.session &&
		section.receivedPeer == item.receivedPeer &&
		section.receivedGeneration == item.receivedGeneration &&
		section.sourcePolicyEpoch == item.sourcePolicyEpoch &&
		section.sourceMessageID == item.sourceMessageID &&
		section.sourcePeerStr == item.sourcePeerStr &&
		section.aigpOrigin == item.aigpOrigin &&
		section.aigpRevision == item.aigpRevision &&
		section.aigpCostWithheld == item.aigpCostWithheld
	return same && lead.attrHash == parts.attrHash && bytes.Equal(lead.attrs, parts.attrs)
}

// recoveryPackShape reports whether a section is one plain UPDATE the pack
// can rewrite: one raw body or one re-encoded update, not originated (whose
// write runs the export gate), no provenance, no AIGP replay ticket and no
// End-of-RIB marker.
func recoveryPackShape(item *fwdItem) bool {
	one := len(item.rawBodies)+len(item.updates) == 1
	return one && !item.originated && item.provenance == nil && item.aigpReplay == nil && item.endOfRIB == nil
}

// recoveryPackBody reads the section's one UPDATE body. A survivor whose
// source's encoding context differs from the destination's is staged as a
// re-encoded update; its sections are the wire fields, and an ungated write
// of them is the write of the same body (writeUpdateGated).
func recoveryPackBody(item *fwdItem) (bucketBodyParts, bool) {
	if len(item.rawBodies) == 1 {
		return parseBucketBody(item.rawBodies[0])
	}
	update := item.updates[0]
	parts := bucketBodyParts{wdLen: len(update.WithdrawnRoutes), wd: update.WithdrawnRoutes,
		attrLen: len(update.PathAttributes), attrs: update.PathAttributes, nlri: update.NLRI}
	parts.attrHash = fnvHash(parts.attrs)
	return parts, true
}
