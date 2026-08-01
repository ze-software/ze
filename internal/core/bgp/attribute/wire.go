// Design: docs/architecture/wire/attributes.md — path attribute encoding
// RFC: rfc/short/rfc4271.md — path attribute wire format (Section 4.3)
// Detail: span.go — Span, SpanIndex, and the single eager index builder

package attribute

import (
	"errors"
	"fmt"
	"sync"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

var errNilEncodingContext = errors.New("nil encoding context")

// AttributesWire stores path attributes in wire format with lazy value parsing.
//
// Wire bytes are the canonical representation. The attribute index is built once,
// eagerly, by the constructor, and is never written afterwards; parsed values are
// cached on demand in a side table.
//
// The split is what makes the read surface safe without a lock. packed,
// sourceCtxID, index and indexErr are written only by NewAttributesWire, so
// Packed, SourceContext, Has, GetRaw, Count and Spilled take no lock at all and
// the forward path never contends. Get, All and ForEach fill the parsed side
// table, and parseMu guards that table alone.
//
// Memory contract: packed is NOT owned by AttributesWire. Caller must ensure the
// underlying buffer outlives this struct and is not modified. The index holds
// offsets into those bytes, so it shares that boundary exactly: a caller that
// rewrites the bytes must build a new AttributesWire over them
// (docs/architecture/memory/lifetime-contracts.md).
type AttributesWire struct {
	packed      []byte
	sourceCtxID bgpctx.ContextID
	index       SpanIndex // built once by the constructor; immutable afterwards
	indexErr    error     // the build verdict, immutable afterwards

	// parsed is the parsed-value side table. It is the only mutable state left on
	// this type and is reached only by Get, All and ForEach, which is why the
	// parsed values no longer sit inside the index entries: a span stays 6 bytes,
	// and no heap pointer is retained for the life of a recent-update cache entry.
	parseMu sync.Mutex
	parsed  []Attribute
}

// NewAttributesWire creates from raw packed bytes and indexes them once.
//
// The index is built here rather than on first access so that the value it
// publishes is immutable and needs no lock to read. A build failure is recorded
// and returned identically by every accessor: the builder is deterministic, so
// the error a caller sees is the one the lazy build produced on first use.
//
// WARNING: packed is NOT copied. Caller retains ownership and must not modify.
func NewAttributesWire(packed []byte, ctxID bgpctx.ContextID) *AttributesWire {
	a := &AttributesWire{
		packed:      packed,
		sourceCtxID: ctxID,
	}
	a.indexErr = a.index.build(packed)
	return a
}

// CarryOver returns a new AttributesWire over newPacked that reuses this one's
// index instead of rebuilding it.
//
// Span offsets are relative to the attribute section, so they are valid against
// any array holding the same section contents. The caller MUST pass a
// byte-identical copy; the length equality below is the cheap half of that
// contract, and a length mismatch falls back to a full rebuild rather than
// publishing an index that does not describe the bytes.
func (a *AttributesWire) CarryOver(newPacked []byte) *AttributesWire {
	if len(newPacked) != len(a.packed) {
		return NewAttributesWire(newPacked, a.sourceCtxID)
	}
	return &AttributesWire{
		packed:      newPacked,
		sourceCtxID: a.sourceCtxID,
		index:       a.index,
		indexErr:    a.indexErr,
	}
}

// Packed returns raw wire bytes for transmission.
// WARNING: Do not modify the returned slice.
func (a *AttributesWire) Packed() []byte {
	return a.packed
}

// SourceContext returns the encoding context ID.
func (a *AttributesWire) SourceContext() bgpctx.ContextID {
	return a.sourceCtxID
}

// Count returns the number of path attributes present. Takes no lock.
func (a *AttributesWire) Count() int {
	return a.index.Len()
}

// Spilled reports whether this UPDATE carried more attributes than the inline
// span capacity, so the receive path can count the event for an operator.
func (a *AttributesWire) Spilled() bool {
	return a.index.Spilled()
}

// Get returns a specific attribute by code (lazy parse).
// Returns (nil, nil) if attribute is not present.
func (a *AttributesWire) Get(code AttributeCode) (Attribute, error) {
	if a.indexErr != nil {
		return nil, a.indexErr
	}
	i := a.index.findIndex(code)
	if i < 0 {
		return nil, nil //nolint:nilnil // nil means not found
	}

	a.parseMu.Lock()
	defer a.parseMu.Unlock()
	return a.parseAtLocked(i)
}

// Has checks if attribute exists without parsing value.
// Returns error if wire bytes are malformed.
//
// Answered from the presence bitset: no scan, no lock, no parse.
func (a *AttributesWire) Has(code AttributeCode) (bool, error) {
	if a.indexErr != nil {
		return false, a.indexErr
	}
	return a.index.Has(code), nil
}

// GetMultiple returns multiple attributes (for API output).
func (a *AttributesWire) GetMultiple(codes []AttributeCode) (map[AttributeCode]Attribute, error) {
	result := make(map[AttributeCode]Attribute, len(codes))
	for _, code := range codes {
		attr, err := a.Get(code)
		if err != nil {
			return nil, fmt.Errorf("getting %s: %w", code, err)
		}
		if attr != nil {
			result[code] = attr
		}
	}
	return result, nil
}

// GetRaw returns raw attribute value bytes without parsing.
// Zero-copy: returns a slice into the packed buffer.
// Returns (nil, nil) if attribute is not present.
// Use this for attributes that need custom handling (e.g., MP_REACH_NLRI for MPReachWire).
//
// This is the accessor the forward path uses, and it takes no lock.
func (a *AttributesWire) GetRaw(code AttributeCode) ([]byte, error) {
	if a.indexErr != nil {
		return nil, a.indexErr
	}
	span, ok := a.index.Find(code)
	if !ok {
		return nil, nil //nolint:nilnil // nil means not found
	}
	return a.packed[span.Offset : span.Offset+span.Length], nil
}

// All returns all attributes (full parse).
// Attributes are returned in wire order.
func (a *AttributesWire) All() ([]Attribute, error) {
	if a.indexErr != nil {
		return nil, a.indexErr
	}

	a.parseMu.Lock()
	defer a.parseMu.Unlock()

	result := make([]Attribute, 0, a.index.Len())
	for i := range a.index.Len() {
		attr, err := a.parseAtLocked(i)
		if err != nil {
			return nil, err
		}
		result = append(result, attr)
	}

	return result, nil
}

// ForEach iterates all attributes in wire order, calling fn for each one.
// Unlike All(), no result slice is allocated. Attributes are parsed on demand
// and cached for subsequent calls. If fn returns false, iteration stops early.
func (a *AttributesWire) ForEach(fn func(AttributeCode, Attribute) bool) error {
	if a.indexErr != nil {
		return a.indexErr
	}

	a.parseMu.Lock()
	defer a.parseMu.Unlock()

	for i := range a.index.Len() {
		attr, err := a.parseAtLocked(i)
		if err != nil {
			return err
		}
		if !fn(a.index.At(i).Code, attr) {
			break
		}
	}
	return nil
}

// PackFor returns packed bytes for destination context.
// Zero-copy if contexts match, otherwise re-encode.
func (a *AttributesWire) PackFor(destCtxID bgpctx.ContextID) ([]byte, error) {
	if a.sourceCtxID == destCtxID {
		return a.packed, nil
	}

	// Slow path: re-encode with destination context
	destCtx := bgpctx.Registry.Get(destCtxID)
	if destCtx == nil {
		return nil, fmt.Errorf("unknown context ID: %d", destCtxID)
	}

	return a.packWithContext(destCtx)
}

// parseAtLocked returns the parsed value of the i-th attribute, filling the side
// table on a miss. Caller must hold parseMu.
func (a *AttributesWire) parseAtLocked(i int) (Attribute, error) {
	if i < len(a.parsed) && a.parsed[i] != nil {
		return a.parsed[i], nil
	}

	attr, err := a.parseSpan(a.index.At(i))
	if err != nil {
		return nil, err
	}

	if a.parsed == nil {
		a.parsed = make([]Attribute, a.index.Len())
	}
	a.parsed[i] = attr
	return attr, nil
}

// parseSpan parses the attribute one span describes. It reads only immutable
// state, so it needs no lock of its own.
func (a *AttributesWire) parseSpan(span Span) (Attribute, error) {
	valueBytes := a.packed[span.Offset : span.Offset+span.Length]

	// Get source context for context-dependent parsing (e.g., ASN4)
	srcCtx := bgpctx.Registry.Get(a.sourceCtxID)
	if srcCtx == nil {
		return nil, fmt.Errorf("unknown source context ID: %d", a.sourceCtxID)
	}

	// Try known attribute parsers first
	attr, err := parseKnownAttribute(span.Code, valueBytes, srcCtx)
	if err != nil {
		return nil, err
	}
	if attr != nil {
		return attr, nil
	}

	// Unknown attribute: read original flags from header for preservation
	// Flags are at the start of the header: packed[offset - hdrLen]
	flags := AttributeFlags(a.packed[span.Offset-uint16(span.HdrLen)])
	return NewOpaqueAttribute(flags, span.Code, valueBytes), nil
}

// packWithContext re-encodes all attributes with destination context.
// Single allocation: calculates total size, then writes all attributes via WriteAttrToWithContext.
func (a *AttributesWire) packWithContext(destCtx *bgpctx.EncodingContext) ([]byte, error) {
	attrs, err := a.All()
	if err != nil {
		return nil, err
	}

	srcCtx := bgpctx.Registry.Get(a.sourceCtxID)
	if srcCtx == nil {
		return nil, fmt.Errorf("unknown source context ID: %d", a.sourceCtxID)
	}

	// Pass 1: calculate total size
	total := 0
	for _, attr := range attrs {
		valueLen := attrLenWithContext(attr, destCtx)
		if valueLen > 255 {
			total += 4 + valueLen // extended length header
		} else {
			total += 3 + valueLen // normal header
		}
	}

	// Pass 2: write all attributes
	buf := make([]byte, total)
	off := 0
	for _, attr := range attrs {
		off += WriteAttrToWithContext(attr, buf, off, srcCtx, destCtx)
	}

	return buf[:off], nil
}

// knownAttrParsers maps attribute type codes to parser functions.
// nil entries mean unknown or known-without-parser — caller creates OpaqueAttribute.
// Two parsers (AS_PATH, AGGREGATOR) use the fourByteAS parameter; the rest ignore it.
var knownAttrParsers [256]func(data []byte, fourByteAS bool) (Attribute, error)

func init() {
	knownAttrParsers[AttrOrigin] = func(d []byte, _ bool) (Attribute, error) { return ParseOrigin(d) }
	knownAttrParsers[AttrASPath] = func(d []byte, asn4 bool) (Attribute, error) { return ParseASPath(d, asn4) }
	knownAttrParsers[AttrNextHop] = func(d []byte, _ bool) (Attribute, error) { return ParseNextHop(d) }
	knownAttrParsers[AttrMED] = func(d []byte, _ bool) (Attribute, error) { return ParseMED(d) }
	knownAttrParsers[AttrLocalPref] = func(d []byte, _ bool) (Attribute, error) { return ParseLocalPref(d) }
	knownAttrParsers[AttrAtomicAggregate] = func(d []byte, _ bool) (Attribute, error) { return ParseAtomicAggregate(d) }
	knownAttrParsers[AttrAggregator] = func(d []byte, asn4 bool) (Attribute, error) { return ParseAggregator(d, asn4) }
	knownAttrParsers[AttrCommunity] = func(d []byte, _ bool) (Attribute, error) { return ParseCommunities(d) }
	knownAttrParsers[AttrOriginatorID] = func(d []byte, _ bool) (Attribute, error) { return ParseOriginatorID(d) }
	knownAttrParsers[AttrClusterList] = func(d []byte, _ bool) (Attribute, error) { return ParseClusterList(d) }
	knownAttrParsers[AttrMPReachNLRI] = func(d []byte, _ bool) (Attribute, error) { return ParseMPReachNLRI(d) }
	knownAttrParsers[AttrMPUnreachNLRI] = func(d []byte, _ bool) (Attribute, error) { return ParseMPUnreachNLRI(d) }
	knownAttrParsers[AttrExtCommunity] = func(d []byte, _ bool) (Attribute, error) { return ParseExtendedCommunities(d) }
	knownAttrParsers[AttrAS4Path] = func(d []byte, _ bool) (Attribute, error) { return ParseAS4Path(d) }
	knownAttrParsers[AttrAS4Aggregator] = func(d []byte, _ bool) (Attribute, error) { return ParseAS4Aggregator(d) }
	knownAttrParsers[AttrLargeCommunity] = func(d []byte, _ bool) (Attribute, error) { return ParseLargeCommunities(d) }
	knownAttrParsers[AttrIPv6ExtCommunity] = func(d []byte, _ bool) (Attribute, error) { return ParseIPv6ExtendedCommunities(d) }
	knownAttrParsers[AttrAIGP] = func(d []byte, _ bool) (Attribute, error) { return ParseAIGP(d) }
	knownAttrParsers[AttrTunnelEncap] = func(d []byte, _ bool) (Attribute, error) { return ParseTunnelEncap(d) }
	// Known codes without parsers yet (PMSI, BGPLS):
	// left nil — treated as opaque, same as truly unknown codes.
	// PrefixSID (40): stored in OtherAttrs; SRv6 SID extracted at best-path time.
}

// parseKnownAttribute parses a known attribute value by code.
// Returns (nil, nil) for unknown attribute codes - caller handles as OpaqueAttribute.
// Known attributes derive their flags from type; only OpaqueAttribute needs stored flags.
// REQUIRES: ctx != nil (caller must validate context exists).
func parseKnownAttribute(code AttributeCode, data []byte, ctx *bgpctx.EncodingContext) (Attribute, error) {
	if ctx == nil {
		return nil, errNilEncodingContext
	}

	fn := knownAttrParsers[code]
	if fn == nil {
		return nil, nil //nolint:nilnil // nil signals unknown, caller creates OpaqueAttribute
	}

	return fn(data, ctx.ASN4())
}
