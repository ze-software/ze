// Design: docs/architecture/edge-cases/as4.md -- the RFC 6793 receive procedure, run once
// RFC: rfc/short/rfc6793.md -- AS4_PATH reconstruction and the NEW-speaker discard
// Related: aspath_as4.go -- the egress question, "is an AS4_PATH owed toward THIS peer"
// Related: internal/core/bgp/attribute/as4.go -- ReconcileASPathFamily, the rule this applies
//
// The collapse runs at INGEST, once per received UPDATE, and everything
// downstream of it reads one four-octet truth. FRR (aspath_reconcile_as4, from
// bgp_attr_parse) and BIRD (bgp_process_as4_attrs, which unsets BA_AS4_PATH)
// both place it there, and neither carries the attribute pair past it. That is
// why no forward rail needs a Section 4.2.3 step of its own.

package wireu

import (
	"encoding/binary"
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// CollapseAS4FamilySize answers the largest output CollapseAS4Family can write
// for payload, so a caller can size one buffer before it knows whether the
// collapse has work to do.
//
// The reconciled AS_PATH can hold at most as many AS numbers as the received
// AS_PATH and AS4_PATH together, each at four octets, and the AGGREGATOR grows
// by two when a two-octet one is widened. Nothing else grows.
func CollapseAS4FamilySize(payload []byte) int {
	return len(payload) + len(payload) + 8
}

// AS4FamilyPlan retains one reconciliation without retaining received buffers.
// Owned canonical values stay in canonical; values borrowed from the input are
// represented by spans and rebound to the eventual payload by WriteTo. This lets
// a receiver check the first AS before coalescing, then encode the same result.
type AS4FamilyPlan struct {
	canonical      attribute.CanonicalASPathFamily
	pathSpan       attrSpan
	aggregatorSpan attrSpan
	attrsStart     int
	rewrite        bool
}

// ASPath returns the reconciled path. payload MUST carry the same attributes and
// section offsets that PrepareAS4Family read. The returned path may borrow payload.
func (p *AS4FamilyPlan) ASPath(payload []byte) []byte {
	if p.pathSpan.hdrLen != 0 {
		return p.pathSpan.value(payload)
	}
	return p.canonical.ASPath
}

// Discards reports the existing RFC 6793 reconciliation's attribute discards.
func (p *AS4FamilyPlan) Discards() []attribute.ASPathDiscard {
	return p.canonical.Discards
}

// CollapseAS4Family rewrites payload into dst so that its AS-path family is
// four-octet truth: AS_PATH carries the reconciled path, AGGREGATOR carries the
// four-octet aggregating node, and neither AS4_PATH nor AS4_AGGREGATOR
// survives.
//
// It answers 0 when the payload is already canonical and the caller MUST then
// keep the payload it has. That is the ordinary case on a four-octet fleet: a
// NEW speaker sends neither AS4 attribute, so there is nothing to reconcile,
// nothing to copy and no AS_PATH to parse.
//
// srcASN4 is what the SENDING session negotiated. RFC 6793 Section 4.1: "The
// new attributes, AS4_PATH and AS4_AGGREGATOR, MUST NOT be carried in an UPDATE
// message between NEW BGP speakers", so an AS4 attribute from a NEW speaker is
// discarded rather than merged, and Section 6 obliges exactly that: "A NEW BGP
// speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute
// in an UPDATE message from another NEW BGP speaker MUST discard the path
// attribute and continue processing the UPDATE message."
//
// The discards are a report for the caller's log, never a failure. A malformed
// AS4_PATH is one of them: Section 6 chooses "attribute discard" for it, so the
// UPDATE continues with the AS_PATH it carried.
func CollapseAS4Family(dst, payload []byte, srcASN4 bool) (int, []attribute.ASPathDiscard, error) {
	plan, err := PrepareAS4Family(payload, srcASN4)
	if err != nil {
		return 0, nil, err
	}
	return plan.WriteTo(dst, payload)
}

// PrepareAS4Family performs the RFC 6793 receive reconciliation once, separately
// from encoding. Its plan owns reconstructed values and stores offsets, never
// pointers, for values that already have canonical encoding.
// Callers MUST preserve those attributes until ASPath or WriteTo consumes the plan.
// ASPath MUST receive the original section offsets; WriteTo permits rebasing.
func PrepareAS4Family(payload []byte, srcASN4 bool) (AS4FamilyPlan, error) {
	var plan AS4FamilyPlan
	if len(payload) < 4 {
		return plan, fmt.Errorf("collapse AS4 family: %w", ErrUpdateTruncated)
	}

	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	if len(payload) < 2+wdLen+2 {
		return plan, fmt.Errorf("collapse AS4 family: %w", ErrUpdateTruncated)
	}

	attrLenOff := 2 + wdLen
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2
	if len(payload) < attrsStart+attrLen {
		return plan, fmt.Errorf("collapse AS4 family: %w", ErrUpdateTruncated)
	}

	var found as4FamilySpans
	if err := found.scan(payload, attrsStart, attrLen); err != nil {
		return plan, err
	}
	plan.attrsStart = attrsStart

	// The fast path, and the reason a four-octet fleet pays nothing: a NEW
	// speaker that sent neither AS4 attribute has already sent the truth.
	if srcASN4 && found.as4Path.off == -1 && found.as4Agg.off == -1 {
		plan.pathSpan = found.asPath
		return plan, nil
	}

	canonical, err := attribute.ReconcileASPathFamily(attribute.ReceivedASPathFamily{
		ASPath:        found.asPath.value(payload),
		AS4Path:       found.as4Path.value(payload),
		Aggregator:    found.agg.value(payload),
		AS4Aggregator: found.as4Agg.value(payload),
		SourceASN4:    srcASN4,
	})
	if err != nil {
		return plan, fmt.Errorf("collapse AS4 family: %w", err)
	}
	plan.canonical = canonical
	plan.rewrite = true
	if srcASN4 {
		plan.pathSpan = found.asPath
		plan.canonical.ASPath = nil
	}
	// The selected AS4_AGGREGATOR already has four-octet encoding. It can
	// borrow the received buffer even on an OLD-speaker session. A malformed
	// AGGREGATOR can also be carried unchanged by the low-level API.
	if canonical.Aggregator != nil && len(canonical.Aggregator) == 0 {
		plan.aggregatorSpan = found.agg
		plan.canonical.Aggregator = nil
	} else if len(canonical.Aggregator) > 0 {
		for _, span := range [...]attrSpan{found.agg, found.as4Agg} {
			if span.off >= 0 && span.length > 0 &&
				&canonical.Aggregator[0] == &payload[span.off+span.hdrLen] {
				plan.aggregatorSpan = span
				plan.canonical.Aggregator = nil
				break
			}
		}
	}
	return plan, nil
}

// WriteTo encodes a prepared reconciliation. payload MUST retain the prepared
// attributes; its withdrawn and announced NLRI may differ after coalescing or
// route-ignore processing. Borrowed spans are rebased to its current attribute
// section, so the plan never depends on the lifetime of the original buffer.
func (p *AS4FamilyPlan) WriteTo(dst, payload []byte) (int, []attribute.ASPathDiscard, error) {
	if !p.rewrite {
		return 0, nil, nil
	}
	if len(payload) < 4 {
		return 0, nil, fmt.Errorf("collapse AS4 family: %w", ErrUpdateTruncated)
	}
	attrLenOff := 2 + int(binary.BigEndian.Uint16(payload[:2]))
	if attrLenOff+2 > len(payload) {
		return 0, nil, fmt.Errorf("collapse AS4 family: %w", ErrUpdateTruncated)
	}
	attrsStart := attrLenOff + 2
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff:attrsStart]))
	if attrsStart+attrLen > len(payload) {
		return 0, nil, fmt.Errorf("collapse AS4 family: %w", ErrUpdateTruncated)
	}
	canonical := p.canonical
	if span := p.pathSpan; span.hdrLen != 0 {
		span.off += attrsStart - p.attrsStart
		canonical.ASPath = span.value(payload)
	}
	if span := p.aggregatorSpan; span.hdrLen != 0 {
		span.off += attrsStart - p.attrsStart
		canonical.Aggregator = span.value(payload)
	}

	// The writes below are sized from the reconciled values rather than from the
	// received ones, so a caller that sized dst by any other arithmetic would
	// have its buffer overrun. A refusal here is a dropped UPDATE; the overrun
	// is a panic on the session read goroutine, which ends the daemon for every
	// peer at once.
	if len(dst) < CollapseAS4FamilySize(payload) {
		return 0, nil, fmt.Errorf("collapse AS4 family: buffer holds %d octets and the collapse needs up to %d",
			len(dst), CollapseAS4FamilySize(payload))
	}

	n := copy(dst, payload[:attrsStart])
	written := 0

	off := attrsStart
	for off < attrsStart+attrLen {
		span := attrSpanAt(payload, off)
		//exhaustive:ignore // Rewrite only the AS-path family; copy all unrelated attributes unchanged.
		switch span.code {
		case attribute.AttrASPath:
			if canonical.ASPath != nil {
				written += writeAttrValue(dst, n+written, attribute.FlagTransitive,
					attribute.AttrASPath, canonical.ASPath)
			}
		case attribute.AttrAggregator:
			// CanonicalASPathFamily.Aggregator is nil exactly when the received
			// UPDATE carried no AGGREGATOR, so the reconciled value always has
			// this slot to be written into and never needs one appended.
			if canonical.Aggregator != nil {
				written += writeAttrValue(dst, n+written, attribute.FlagOptional|attribute.FlagTransitive,
					attribute.AttrAggregator, canonical.Aggregator)
			}
		case attribute.AttrAS4Path, attribute.AttrAS4Aggregator:
			// Dropped: nothing downstream of the collapse is an OLD speaker, so
			// RFC 6793 Section 4.1 leaves neither attribute a destination.
		default:
			written += copy(dst[n+written:], payload[off:off+span.hdrLen+span.length])
		}
		off += span.hdrLen + span.length
	}

	binary.BigEndian.PutUint16(dst[attrLenOff:attrLenOff+2], uint16(written)) //nolint:gosec // bounded by BGP max
	n += written
	n += copy(dst[n:], payload[attrsStart+attrLen:])
	return n, canonical.Discards, nil
}

// writeAttrValue writes one attribute header and value at off, and answers the
// bytes written.
func writeAttrValue(dst []byte, off int, flags attribute.AttributeFlags, code attribute.AttributeCode, value []byte) int {
	n := attribute.WriteHeaderTo(dst, off, flags, code, uint16(len(value))) //nolint:gosec // bounded by BGP max
	n += copy(dst[off+n:], value)
	return n
}

// attrSpan locates one attribute inside a payload. An off of -1 means the
// attribute is absent, which value() reports as a nil slice so the reconciler
// reads "not carried" rather than "carried empty".
type attrSpan struct {
	off    int
	hdrLen int
	length int
	code   attribute.AttributeCode
}

func (s attrSpan) value(payload []byte) []byte {
	if s.off == -1 {
		return nil
	}
	return payload[s.off+s.hdrLen : s.off+s.hdrLen+s.length]
}

// as4FamilySpans holds the four attributes the collapse reads.
type as4FamilySpans struct {
	asPath  attrSpan
	as4Path attrSpan
	agg     attrSpan
	as4Agg  attrSpan
}

// The bound every check below reads is the end of the ATTRIBUTE SECTION, never
// the end of the payload. An attribute whose declared length runs past the
// section but stops inside the NLRI that follows it is inside the payload, so a
// payload-end bound accepts it and hands the reconciliation a value built from
// NLRI octets. The walk then jumps past the section end and the loop exits with
// nothing said.
func (f *as4FamilySpans) scan(payload []byte, attrsStart, attrLen int) error {
	f.asPath.off, f.as4Path.off, f.agg.off, f.as4Agg.off = -1, -1, -1, -1

	attrsEnd := attrsStart + attrLen
	off := attrsStart
	for off < attrsEnd {
		if off+3 > attrsEnd {
			return fmt.Errorf("collapse AS4 family: truncated attribute at offset %d: %w", off, ErrUpdateMalformed)
		}
		// The ext-length bound is checked BEFORE the header is read, because
		// reading it is what consumes the fourth octet: attrSpanAt takes
		// payload[off+2:off+4] for an extended length, so bounding afterwards
		// slices past the end when the attribute section ends at off+3 and the
		// payload has no spare capacity. TranscodeASPath orders the same pair
		// this way.
		if attribute.AttributeFlags(payload[off]).IsExtLength() && off+4 > attrsEnd {
			return fmt.Errorf("collapse AS4 family: truncated ext-length attribute: %w", ErrUpdateMalformed)
		}
		span := attrSpanAt(payload, off)
		if off+span.hdrLen+span.length > attrsEnd {
			return fmt.Errorf("collapse AS4 family: attribute value overflows the attribute section: %w", ErrUpdateMalformed)
		}

		//exhaustive:ignore // Project only the four AS-path-family spans from the attribute stream.
		switch span.code {
		case attribute.AttrASPath:
			f.asPath = span
		case attribute.AttrAS4Path:
			f.as4Path = span
		case attribute.AttrAggregator:
			f.agg = span
		case attribute.AttrAS4Aggregator:
			f.as4Agg = span
		default:
			// Every other attribute is copied through untouched: the collapse
			// reads the AS-path family and nothing else.
		}
		off += span.hdrLen + span.length
	}
	return nil
}

// attrSpanAt reads the attribute header at off. The caller has already bounded
// off against the payload.
func attrSpanAt(payload []byte, off int) attrSpan {
	flags := attribute.AttributeFlags(payload[off])
	span := attrSpan{off: off, code: attribute.AttributeCode(payload[off+1])}
	if flags.IsExtLength() {
		span.length = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
		span.hdrLen = 4
		return span
	}
	span.length = int(payload[off+2])
	span.hdrLen = 3
	return span
}
