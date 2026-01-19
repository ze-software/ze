package attribute

import (
	"fmt"
	"sync"

	bgpctx "codeberg.org/thomas-mangin/zebgp/internal/bgp/context"
)

// attrIndex caches attribute location and parsed value within packed bytes.
// Built lazily on first scan, reused for subsequent lookups.
// hdrLen is retained to locate original flags for unknown attributes.
type attrIndex struct {
	code   AttributeCode
	offset uint16 // Points to value (after header)
	length uint16
	hdrLen uint8     // 3 or 4; flags at packed[offset-hdrLen]
	parsed Attribute // nil until parsed on demand
}

// AttributesWire stores path attributes in wire format with lazy parsing.
//
// Wire bytes are the canonical representation. Parsed attributes are
// cached in the index on demand. Thread-safe for concurrent read access.
//
// Memory contract: packed is NOT owned by AttributesWire. Caller must
// ensure the underlying buffer outlives this struct and is not modified.
type AttributesWire struct {
	mu          sync.RWMutex
	packed      []byte
	sourceCtxID bgpctx.ContextID
	index       []attrIndex // nil until first scan; parsed cached in each entry
}

// NewAttributesWire creates from raw packed bytes.
// WARNING: packed is NOT copied. Caller retains ownership and must not modify.
func NewAttributesWire(packed []byte, ctxID bgpctx.ContextID) *AttributesWire {
	return &AttributesWire{
		packed:      packed,
		sourceCtxID: ctxID,
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

// Get returns a specific attribute by code (lazy parse).
// Returns (nil, nil) if attribute is not present.
func (a *AttributesWire) Get(code AttributeCode) (Attribute, error) {
	a.mu.RLock()
	if a.index != nil {
		for i := range a.index {
			if a.index[i].code == code {
				if attr := a.index[i].parsed; attr != nil {
					a.mu.RUnlock()
					return attr, nil
				}
				// Found but not parsed - need write lock
				a.mu.RUnlock()
				return a.getAndParse(i, code)
			}
		}
		// Index exists but code not found
		a.mu.RUnlock()
		return nil, nil //nolint:nilnil // nil means not found
	}
	a.mu.RUnlock()

	// Index not built yet
	return a.getAndParse(-1, code)
}

// getAndParse acquires write lock and parses attribute.
// If hint >= 0, it's the index position from RLock scan (still needs double-check).
// If hint < 0, index needs to be built first.
func (a *AttributesWire) getAndParse(hint int, code AttributeCode) (Attribute, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ensureIndexLocked(); err != nil {
		return nil, err
	}

	// If we have a hint, check that position first (double-check after lock upgrade)
	if hint >= 0 && hint < len(a.index) && a.index[hint].code == code {
		if attr := a.index[hint].parsed; attr != nil {
			return attr, nil
		}
		attr, err := a.parseAtLocked(a.index[hint])
		if err != nil {
			return nil, err
		}
		a.index[hint].parsed = attr
		return attr, nil
	}

	// Full search (hint invalid or index was rebuilt)
	for i := range a.index {
		if a.index[i].code == code {
			if attr := a.index[i].parsed; attr != nil {
				return attr, nil
			}
			attr, err := a.parseAtLocked(a.index[i])
			if err != nil {
				return nil, err
			}
			a.index[i].parsed = attr
			return attr, nil
		}
	}

	return nil, nil //nolint:nilnil // nil means not found
}

// Has checks if attribute exists without parsing value.
// Returns error if wire bytes are malformed.
func (a *AttributesWire) Has(code AttributeCode) (bool, error) {
	a.mu.RLock()
	if a.index != nil {
		for i := range a.index {
			if a.index[i].code == code {
				a.mu.RUnlock()
				return true, nil
			}
		}
		a.mu.RUnlock()
		return false, nil
	}
	a.mu.RUnlock()

	// Build index (upgrades to write lock)
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ensureIndexLocked(); err != nil {
		return false, err
	}

	for i := range a.index {
		if a.index[i].code == code {
			return true, nil
		}
	}
	return false, nil
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
func (a *AttributesWire) GetRaw(code AttributeCode) ([]byte, error) {
	a.mu.RLock()
	if a.index != nil {
		for i := range a.index {
			if a.index[i].code == code {
				result := a.packed[a.index[i].offset : a.index[i].offset+a.index[i].length]
				a.mu.RUnlock()
				return result, nil
			}
		}
		a.mu.RUnlock()
		return nil, nil //nolint:nilnil // nil means not found
	}
	a.mu.RUnlock()

	// Index not built yet
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ensureIndexLocked(); err != nil {
		return nil, err
	}

	for i := range a.index {
		if a.index[i].code == code {
			return a.packed[a.index[i].offset : a.index[i].offset+a.index[i].length], nil
		}
	}

	return nil, nil //nolint:nilnil // nil means not found
}

// All returns all attributes (full parse).
// Attributes are returned in wire order.
func (a *AttributesWire) All() ([]Attribute, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ensureIndexLocked(); err != nil {
		return nil, err
	}

	result := make([]Attribute, 0, len(a.index))
	for i := range a.index {
		if a.index[i].parsed != nil {
			result = append(result, a.index[i].parsed)
			continue
		}

		attr, err := a.parseAtLocked(a.index[i])
		if err != nil {
			return nil, err
		}
		a.index[i].parsed = attr
		result = append(result, attr)
	}

	return result, nil
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

// ensureIndexLocked builds the attribute index if not already built.
// Caller must hold write lock.
// RFC 4271: Duplicate attributes are a Malformed Attribute List error.
//
// Index is built atomically: on error, a.index remains nil so subsequent
// calls will retry and return the same error.
func (a *AttributesWire) ensureIndexLocked() error {
	if a.index != nil {
		return nil
	}

	// Build index locally first - only assign to a.index on success
	// This ensures parse errors leave a.index nil for retry
	index := make([]attrIndex, 0, 8)
	seen := make(map[AttributeCode]bool, 8)

	offset := 0
	for offset < len(a.packed) {
		_, code, length, hdrLen, err := ParseHeader(a.packed[offset:])
		if err != nil {
			return fmt.Errorf("parsing header at offset %d: %w", offset, err)
		}

		// RFC 4271: duplicate attributes are malformed
		if seen[code] {
			return fmt.Errorf("duplicate attribute %s at offset %d", code, offset)
		}
		seen[code] = true

		// Validate we have enough data
		if offset+hdrLen+int(length) > len(a.packed) {
			return fmt.Errorf("attribute %s truncated at offset %d", code, offset)
		}

		index = append(index, attrIndex{
			code:   code,
			offset: uint16(offset + hdrLen), //nolint:gosec // G115: bounded by packed length (max 65535)
			length: length,
			hdrLen: uint8(hdrLen), //nolint:gosec // G115: hdrLen is 3 or 4
		})

		offset += hdrLen + int(length)
	}

	// Success - atomically publish the index
	a.index = index
	return nil
}

// parseAtLocked parses the attribute at the given index.
// Caller must hold lock.
func (a *AttributesWire) parseAtLocked(idx attrIndex) (Attribute, error) {
	valueBytes := a.packed[idx.offset : idx.offset+idx.length]

	// Get source context for context-dependent parsing (e.g., ASN4)
	srcCtx := bgpctx.Registry.Get(a.sourceCtxID)
	if srcCtx == nil {
		return nil, fmt.Errorf("unknown source context ID: %d", a.sourceCtxID)
	}

	// Try known attribute parsers first
	attr, err := parseKnownAttribute(idx.code, valueBytes, srcCtx)
	if err != nil {
		return nil, err
	}
	if attr != nil {
		return attr, nil
	}

	// Unknown attribute: read original flags from header for preservation
	// Flags are at the start of the header: packed[offset - hdrLen]
	flags := AttributeFlags(a.packed[idx.offset-uint16(idx.hdrLen)])
	return NewOpaqueAttribute(flags, idx.code, valueBytes), nil
}

// packWithContext re-encodes all attributes with destination context.
func (a *AttributesWire) packWithContext(destCtx *bgpctx.EncodingContext) ([]byte, error) {
	attrs, err := a.All()
	if err != nil {
		return nil, err
	}

	srcCtx := bgpctx.Registry.Get(a.sourceCtxID)
	if srcCtx == nil {
		return nil, fmt.Errorf("unknown source context ID: %d", a.sourceCtxID)
	}

	// Estimate size
	buf := make([]byte, 0, len(a.packed))

	for _, attr := range attrs {
		packed := attr.PackWithContext(srcCtx, destCtx)
		hdr := PackHeader(attr.Flags(), attr.Code(), uint16(len(packed))) //nolint:gosec // G115: attr value max 65535
		buf = append(buf, hdr...)
		buf = append(buf, packed...)
	}

	return buf, nil
}

// parseKnownAttribute parses a known attribute value by code.
// Returns (nil, nil) for unknown attribute codes - caller handles as OpaqueAttribute.
// Known attributes derive their flags from type; only OpaqueAttribute needs stored flags.
// REQUIRES: ctx != nil (caller must validate context exists).
func parseKnownAttribute(code AttributeCode, data []byte, ctx *bgpctx.EncodingContext) (Attribute, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil encoding context")
	}
	fourByteAS := ctx.ASN4()

	switch code {
	case AttrOrigin:
		return ParseOrigin(data)
	case AttrASPath:
		return ParseASPath(data, fourByteAS)
	case AttrNextHop:
		return ParseNextHop(data)
	case AttrMED:
		return ParseMED(data)
	case AttrLocalPref:
		return ParseLocalPref(data)
	case AttrAtomicAggregate:
		// RFC 4271: ATOMIC_AGGREGATE has length 0
		if len(data) != 0 {
			return nil, fmt.Errorf("ATOMIC_AGGREGATE must be empty, got %d bytes", len(data))
		}
		return &AtomicAggregate{}, nil
	case AttrAggregator:
		return ParseAggregator(data, fourByteAS)
	case AttrOriginatorID:
		return ParseOriginatorID(data)
	case AttrClusterList:
		return ParseClusterList(data)
	case AttrCommunity:
		return ParseCommunities(data)
	case AttrMPReachNLRI:
		return ParseMPReachNLRI(data)
	case AttrMPUnreachNLRI:
		return ParseMPUnreachNLRI(data)
	case AttrExtCommunity:
		return ParseExtendedCommunities(data)
	case AttrAS4Path:
		return ParseAS4Path(data)
	case AttrAS4Aggregator:
		return ParseAS4Aggregator(data)
	case AttrLargeCommunity:
		return ParseLargeCommunities(data)
	case AttrIPv6ExtCommunity:
		return ParseIPv6ExtendedCommunities(data)
	case AttrPMSI, AttrTunnelEncap, AttrAIGP, AttrBGPLS, AttrPrefixSID:
		// Known codes without parsers yet - treat as opaque
		return nil, nil //nolint:nilnil // nil signals unknown, caller creates OpaqueAttribute
	default:
		// Unknown - caller will create OpaqueAttribute with preserved flags
		return nil, nil //nolint:nilnil // nil signals unknown, caller creates OpaqueAttribute
	}
}
