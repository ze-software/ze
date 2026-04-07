// Design: docs/architecture/wire/messages.md — wire UPDATE lazy parsing
// RFC: rfc/short/rfc4271.md — UPDATE message wire format (Section 4.3)
// RFC: rfc/short/rfc4760.md — multiprotocol NLRI in UPDATE

package wireu

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wire"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/source"
)

// WireUpdate holds UPDATE message payload bytes for zero-copy lazy parsing.
// RFC 4271 Section 4.3 - UPDATE message format (after header):
//
//	+-----------------------------------------------------+
//	|   Withdrawn Routes Length (2 octets)                |
//	+-----------------------------------------------------+
//	|   Withdrawn Routes (variable)                       |
//	+-----------------------------------------------------+
//	|   Total Path Attribute Length (2 octets)            |
//	+-----------------------------------------------------+
//	|   Path Attributes (variable)                        |
//	+-----------------------------------------------------+
//	|   Network Layer Reachability Information (variable) |
//	+-----------------------------------------------------+
//
// All methods return slices into the original payload - do not modify.
// GC manages lifetime; no pool or reference counting.
//
// Thread-safe for concurrent read access. Lazy parsing has a benign race:
// if two goroutines call accessors simultaneously on a fresh WireUpdate,
// both may parse (same input → same result). The struct field assignment
// is not atomic, but since both goroutines compute identical values from
// identical input, any interleaving produces correct results.
type WireUpdate struct {
	payload     []byte
	sourceCtxID bgpctx.ContextID
	messageID   uint64          // Unique ID set by reactor after creation
	sourceID    source.SourceID // Source that sent/created this message

	// Cached section offsets (parsed lazily on first accessor call)
	sections wire.UpdateSections
	parseErr error // Cached error if parsing failed

	// Cached Attrs() result (parsed lazily on first call).
	// Thread-safety: benign race — concurrent calls may both compute,
	// but both produce identical results from identical input.
	cachedAttrs    *attribute.AttributesWire
	cachedAttrsSet bool // distinguishes nil-result from not-yet-computed
}

// NewWireUpdate creates a WireUpdate from raw UPDATE payload bytes.
// Takes ownership conceptually - caller should not modify payload after this call.
func NewWireUpdate(payload []byte, ctxID bgpctx.ContextID) *WireUpdate {
	return &WireUpdate{
		payload:     payload,
		sourceCtxID: ctxID,
	}
}

// ensureParsed parses section offsets if not already done.
// Thread-safety: benign race - concurrent calls may both parse,
// but result is identical (same input → same output).
func (u *WireUpdate) ensureParsed() {
	if u.sections.Valid() || u.parseErr != nil {
		return // Already parsed (success or failure)
	}

	sections, err := wire.ParseUpdateSections(u.payload)
	if err != nil {
		// Map wire.ErrUpdateTruncated to plugin.ErrUpdateTruncated for consistency
		u.parseErr = ErrUpdateTruncated
		return
	}
	u.sections = sections
}

// Withdrawn returns the Withdrawn Routes section.
// RFC 4271 Section 4.3 - IPv4 prefixes being withdrawn.
// Returns (nil, nil) if empty, (nil, error) if malformed.
func (u *WireUpdate) Withdrawn() ([]byte, error) {
	u.ensureParsed()
	if u.parseErr != nil {
		return nil, fmt.Errorf("withdrawn: %w", u.parseErr)
	}
	return u.sections.Withdrawn(u.payload), nil
}

// Attrs returns the Path Attributes as an AttributesWire for lazy parsing.
// RFC 4271 Section 4.3 - Path attribute sequence.
// Returns (nil, nil) if empty, (nil, error) if malformed.
// Result is cached: the first call parses and stores, subsequent calls return the cached value.
// Thread-safety: benign race — concurrent first calls may both compute,
// but both produce identical results from identical immutable input.
func (u *WireUpdate) Attrs() (*attribute.AttributesWire, error) {
	if u.cachedAttrsSet {
		return u.cachedAttrs, u.attrsErr()
	}
	return u.deriveAttrs()
}

// attrsErr reconstructs the error for cached nil results.
// When cachedAttrsSet is true and cachedAttrs is nil, it could be either
// a successful empty result or a parse error. We check parseErr to distinguish.
func (u *WireUpdate) attrsErr() error {
	if u.parseErr != nil {
		return fmt.Errorf("attrs: %w", u.parseErr)
	}
	return nil
}

// deriveAttrs extracts AttributesWire from payload using cached sections.
func (u *WireUpdate) deriveAttrs() (*attribute.AttributesWire, error) {
	u.ensureParsed()
	if u.parseErr != nil {
		u.cachedAttrsSet = true
		return nil, fmt.Errorf("attrs: %w", u.parseErr)
	}

	attrBytes := u.sections.Attrs(u.payload)
	if attrBytes == nil {
		u.cachedAttrsSet = true
		return nil, nil //nolint:nilnil // nil,nil = valid empty (no attributes)
	}
	result := attribute.NewAttributesWire(attrBytes, u.sourceCtxID)
	u.cachedAttrs = result
	u.cachedAttrsSet = true
	return result, nil
}

// NLRI returns the Network Layer Reachability Information section.
// RFC 4271 Section 4.3 - IPv4 prefixes being announced.
// Returns (nil, nil) if empty, (nil, error) if malformed.
func (u *WireUpdate) NLRI() ([]byte, error) {
	u.ensureParsed()
	if u.parseErr != nil {
		return nil, fmt.Errorf("nlri: %w", u.parseErr)
	}
	return u.sections.NLRI(u.payload), nil
}

// MPReach extracts MP_REACH_NLRI (attribute code 14) as MPReachWire.
// RFC 4760 Section 3 - Multiprotocol reachability.
// Returns (nil, nil) if attribute not present, (nil, error) if malformed.
func (u *WireUpdate) MPReach() (MPReachWire, error) {
	attrs, err := u.Attrs()
	if err != nil {
		return nil, fmt.Errorf("mp_reach: %w", err)
	}
	if attrs == nil {
		return nil, nil // No attributes, so no MP_REACH
	}
	raw, err := attrs.GetRaw(attribute.AttrMPReachNLRI)
	if err != nil {
		return nil, fmt.Errorf("mp_reach: %w", err)
	}
	if raw == nil {
		return nil, nil // Attribute not present
	}
	if len(raw) < 5 {
		return nil, fmt.Errorf("mp_reach: %w", ErrUpdateMalformed)
	}
	return MPReachWire(raw), nil
}

// MPUnreach extracts MP_UNREACH_NLRI (attribute code 15) as MPUnreachWire.
// RFC 4760 Section 4 - Multiprotocol unreachability.
// Returns (nil, nil) if attribute not present, (nil, error) if malformed.
func (u *WireUpdate) MPUnreach() (MPUnreachWire, error) {
	attrs, err := u.Attrs()
	if err != nil {
		return nil, fmt.Errorf("mp_unreach: %w", err)
	}
	if attrs == nil {
		return nil, nil // No attributes, so no MP_UNREACH
	}
	raw, err := attrs.GetRaw(attribute.AttrMPUnreachNLRI)
	if err != nil {
		return nil, fmt.Errorf("mp_unreach: %w", err)
	}
	if raw == nil {
		return nil, nil // Attribute not present
	}
	if len(raw) < 3 {
		return nil, fmt.Errorf("mp_unreach: %w", ErrUpdateMalformed)
	}
	return MPUnreachWire(raw), nil
}

// SourceCtxID returns the encoding context ID for zero-copy decisions.
func (u *WireUpdate) SourceCtxID() bgpctx.ContextID {
	return u.sourceCtxID
}

// Payload returns the raw UPDATE payload bytes.
// Used for passthrough when forwarding unchanged.
func (u *WireUpdate) Payload() []byte {
	return u.payload
}

// MessageID returns the unique identifier for this UPDATE.
// Set by reactor after creation via SetMessageID.
func (u *WireUpdate) MessageID() uint64 {
	return u.messageID
}

// SetMessageID sets the message ID. Called once by reactor after creation.
func (u *WireUpdate) SetMessageID(id uint64) {
	u.messageID = id
}

// SourceID returns the source that sent/created this message.
func (u *WireUpdate) SourceID() source.SourceID {
	return u.sourceID
}

// SetSourceID sets the source ID. Called once by reactor after creation.
func (u *WireUpdate) SetSourceID(id source.SourceID) {
	u.sourceID = id
}

// NLRIIterator returns an iterator over the NLRI section.
// Set addPath=true when ADD-PATH is negotiated (RFC 7911).
// Returns (nil, nil) if NLRI section is empty.
// Returns (nil, error) if payload is malformed.
func (u *WireUpdate) NLRIIterator(addPath bool) (*nlri.NLRIIterator, error) {
	data, err := u.NLRI()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil //nolint:nilnil // nil,nil = valid empty
	}
	return nlri.NewNLRIIterator(data, addPath), nil
}

// WithdrawnIterator returns an iterator over the Withdrawn Routes section.
// Set addPath=true when ADD-PATH is negotiated (RFC 7911).
// Returns (nil, nil) if withdrawn section is empty.
// Returns (nil, error) if payload is malformed.
func (u *WireUpdate) WithdrawnIterator(addPath bool) (*nlri.NLRIIterator, error) {
	data, err := u.Withdrawn()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil //nolint:nilnil // nil,nil = valid empty
	}
	return nlri.NewNLRIIterator(data, addPath), nil
}

// AttrIterator returns an iterator over the Path Attributes section.
// Returns zero-value iterator (yields nothing) if attributes section is empty.
// Returns error if payload is malformed.
func (u *WireUpdate) AttrIterator() (attribute.AttrIterator, error) {
	attrs, err := u.Attrs()
	if err != nil {
		return attribute.AttrIterator{}, err
	}
	if attrs == nil {
		return attribute.AttrIterator{}, nil
	}
	return attribute.NewAttrIterator(attrs.Packed()), nil
}

// IsEOR detects End-of-RIB markers per RFC 4724 Section 2.
// Returns the address family and true if this UPDATE is an EOR marker.
// IPv4 unicast EOR: empty UPDATE (no withdrawn, no attrs, no NLRI).
// Other families: UPDATE with only MP_UNREACH_NLRI containing AFI/SAFI, no withdrawn prefixes.
func (u *WireUpdate) IsEOR() (family.Family, bool) {
	// Check IPv4 sections (cheap, no attribute parsing).
	withdrawn, err := u.Withdrawn()
	if err != nil || len(withdrawn) > 0 {
		return family.Family{}, false
	}
	nlriBytes, err := u.NLRI()
	if err != nil || len(nlriBytes) > 0 {
		return family.Family{}, false
	}

	// Check for MP_REACH_NLRI — if present, not an EOR.
	mpReach, err := u.MPReach()
	if err != nil || mpReach != nil {
		return family.Family{}, false
	}

	// Check for MP_UNREACH_NLRI.
	mpUnreach, err := u.MPUnreach()
	if err != nil {
		return family.Family{}, false
	}

	if mpUnreach != nil {
		// Multiprotocol EOR: MP_UNREACH with AFI/SAFI only, no withdrawn prefixes.
		if len(mpUnreach.WithdrawnBytes()) > 0 {
			return family.Family{}, false
		}
		return mpUnreach.Family(), true
	}

	// No MP attributes and no IPv4 content: IPv4 unicast EOR.
	attrs, err := u.Attrs()
	if err != nil || attrs != nil {
		return family.Family{}, false
	}
	return family.Family{AFI: 1, SAFI: 1}, true
}
