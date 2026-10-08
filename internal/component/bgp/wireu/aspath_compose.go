// Design: docs/architecture/bgp/fanout-dedup.md -- one effective AS-path family per recipient.
// Related: aspath_slot.go -- allocation-free generators and destination-width projection.
package wireu

import (
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func pendingASPathFamily(mods *filterapi.ModAccumulator) bool {
	for _, op := range mods.Ops() {
		if op.Code == byte(attribute.AttrASPath) || op.Code == byte(attribute.AttrAS4Path) {
			return true
		}
	}
	return false
}

// effectiveASPathValue reads the last Set/Suppress, not the received value it
// superseded. A generator has no readable value; only that case needs reusable
// staging for semantic resolution. Ordinary Buf edits and source values borrow.
func effectiveASPathValue(mods *filterapi.ModAccumulator, section []byte, spans *attribute.SpanIndex, code attribute.AttributeCode, scratch *[]byte) ([]byte, bool, error) {
	value, present := []byte(nil), false
	if span, ok := spans.Find(code); ok {
		value, present = spanValue(section, span), true
	}
	var last *filterapi.AttrOp
	for i := range mods.Ops() {
		op := &mods.Ops()[i]
		if op.Code == byte(code) && (op.Action == filterapi.AttrModSet || op.Action == filterapi.AttrModSuppress) {
			last = op
		}
	}
	if last == nil {
		return value, present, nil
	}
	if last.Action == filterapi.AttrModSuppress {
		return nil, false, nil
	}
	if last.GenIdx == 0 {
		return last.Buf, true, nil
	}
	index := int(last.GenIdx) - 1
	if index >= len(mods.Gens()) || mods.Gens()[index] == nil {
		return nil, false, fmt.Errorf("AS_PATH intent: missing generator for attribute %d", code)
	}
	gen := mods.Gens()[index]
	n := gen.GenLen()
	if n < 0 || n > 65535 {
		return nil, false, fmt.Errorf("AS_PATH intent: invalid generated attribute length %d", n)
	}
	*scratch = slices.Grow((*scratch)[:0], n)[:n]
	if gen.GenWrite(*scratch, 0) != n {
		return nil, false, fmt.Errorf("AS_PATH intent: generated attribute length changed")
	}
	return *scratch, true, nil
}

// recordComposed consumes policy once, then replaces peer ASNs, prepends the
// protocol ASNs, and projects the resulting path into AS_PATH/AS4_PATH together.
// This ordering keeps AS_TRANS a projection, never the input to AS override.
func (e *ASPathEdit) recordComposed(mods *filterapi.ModAccumulator, payload, section []byte, spans *attribute.SpanIndex, in ASPathIntent) (bool, error) {
	value, present, err := effectiveASPathValue(mods, section, spans, attribute.AttrASPath, &e.inputASPath)
	if err != nil {
		return false, err
	}
	as4Value, hasAS4, err := effectiveASPathValue(mods, section, spans, attribute.AttrAS4Path, &e.inputAS4Path)
	if err != nil {
		return false, err
	}
	advertises := PayloadAdvertisesNLRI(payload)
	if !advertises {
		in.Prepend = nil
		in.OverridePeerAS = 0
	}
	policyPrepend := false
	suppressed := false
	for _, op := range mods.Ops() {
		if op.Code == byte(attribute.AttrASPath) {
			switch op.Action {
			case filterapi.AttrModSet:
				suppressed = false
			case filterapi.AttrModSuppress:
				suppressed = true
			}
		}
		if op.Code == byte(attribute.AttrASPath) && op.Action == filterapi.AttrModPrepend && len(op.Buf) != 0 {
			policyPrepend = true
		}
	}

	// Matching-width values without a companion keep the one-copy fast path.
	// An effective AS4_PATH needs reconstruction or discard before projection.
	if present && !policyPrepend && !hasAS4 && in.SrcASN4 == in.DstASN4 &&
		(in.DstASN4 || in.OverrideLocalAS <= maxMappableASN) {
		if len(in.Prepend) == 0 {
			if in.OverridePeerAS == 0 || !asPathValueContains(value, in.OverridePeerAS, in.SrcASN4) {
				return false, nil
			}
			mods.RemoveOps(byte(attribute.AttrASPath))
			e.shift = asPathShiftGen{tail: value, overridePeer: in.OverridePeerAS, overrideLocal: in.OverrideLocalAS, asn4: in.DstASN4}
			mods.OpGen(byte(attribute.AttrASPath), &e.shift)
			return true, nil
		}
		if e.tryShift(mods, value, in) {
			// No policy Prepend remains to be emitted twice. Earlier Sets are
			// superseded by the generated Set in the normal last-Set fold.
			return true, nil
		}
	}

	var path *attribute.ASPath
	if present {
		path, err = attribute.ParseASPath(value, in.SrcASN4)
		if err != nil {
			return false, fmt.Errorf("AS_PATH intent: effective path: %w", err)
		}
	} else if !suppressed && (policyPrepend || (advertises && len(in.Prepend) != 0)) {
		path = &attribute.ASPath{}
	}
	// aspathHandler emits all Prepend fragments in recorded order, ahead of
	// the last Set (or source). Preserve that ordering before protocol edits.
	if policyPrepend && path != nil {
		var segments []attribute.ASPathSegment
		for _, op := range mods.Ops() {
			if op.Code != byte(attribute.AttrASPath) || op.Action != filterapi.AttrModPrepend || len(op.Buf) == 0 {
				continue
			}
			prefix, parseErr := attribute.ParseASPath(op.Buf, in.SrcASN4)
			if parseErr != nil {
				return false, fmt.Errorf("AS_PATH intent: policy prepend: %w", parseErr)
			}
			segments = append(segments, prefix.Segments...)
		}
		path.Segments = append(segments, path.Segments...)
	}
	if path != nil && !in.SrcASN4 && hasAS4 {
		// RFC 6793 Section 6: malformed AS4_PATH is discarded, not fatal.
		if legacy, parseErr := attribute.ParseAS4Path(as4Value); parseErr == nil {
			path = attribute.MergeAS4Path(path, legacy)
		}
	}
	if path != nil {
		if in.OverridePeerAS != 0 {
			for _, segment := range path.Segments {
				for i, asn := range segment.ASNs {
					if asn == in.OverridePeerAS {
						segment.ASNs[i] = in.OverrideLocalAS
					}
				}
			}
		}
		for _, asn := range in.Prepend {
			path.Prepend(asn)
		}
	}
	mods.RemoveOps(byte(attribute.AttrASPath))
	mods.RemoveOps(byte(attribute.AttrAS4Path))
	if path != nil {
		e.encode = asPathEncodeGen{path: path, asn4: in.DstASN4}
		mods.OpGen(byte(attribute.AttrASPath), &e.encode)
	} else if spans.Has(attribute.AttrASPath) {
		mods.Op(byte(attribute.AttrASPath), filterapi.AttrModSuppress, nil)
	}
	e.recordAS4Path(mods, spans, as4PathForPath(path, in.DstASN4))
	e.recordAggregator(mods, section, spans, in)
	return true, nil
}

func asPathValueContains(value []byte, asn uint32, asn4 bool) bool {
	segments := attribute.NewASPathIterator(value, asn4)
	for {
		_, raw, ok := segments.Next()
		if !ok {
			return false
		}
		numbers := attribute.NewASNIterator(raw, asn4)
		for {
			number, ok := numbers.Next()
			if !ok {
				break
			}
			if number == asn {
				return true
			}
		}
	}
}

// overrideASPathValue touches only the final writer-owned value. skip excludes
// the protocol prepend: a dual-AS local prepend can itself equal the peer ASN.
func overrideASPathValue(value []byte, peerAS, localAS uint32, asn4 bool, skip int) {
	if peerAS == 0 {
		return
	}
	width := 2
	if asn4 {
		width = 4
	}
	for off := 0; off+2 <= len(value); {
		count := int(value[off+1])
		off += 2
		if count > (len(value)-off)/width {
			return
		}
		for range count {
			if skip != 0 {
				skip--
			} else if asn4 {
				if binary.BigEndian.Uint32(value[off:]) == peerAS {
					binary.BigEndian.PutUint32(value[off:], localAS)
				}
			} else if uint32(binary.BigEndian.Uint16(value[off:])) == peerAS {
				binary.BigEndian.PutUint16(value[off:], uint16(localAS)) //nolint:gosec // G115: caller restricts narrow override to mappable local AS
			}
			off += width
		}
	}
}
