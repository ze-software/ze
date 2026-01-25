package plugin

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/source"
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
// Thread-safe for concurrent read access.
type WireUpdate struct {
	payload     []byte
	sourceCtxID bgpctx.ContextID
	messageID   uint64          // Unique ID set by reactor after creation
	sourceID    source.SourceID // Source that sent/created this message
}

// NewWireUpdate creates a WireUpdate from raw UPDATE payload bytes.
// Takes ownership conceptually - caller should not modify payload after this call.
func NewWireUpdate(payload []byte, ctxID bgpctx.ContextID) *WireUpdate {
	return &WireUpdate{
		payload:     payload,
		sourceCtxID: ctxID,
	}
}

// Withdrawn returns the Withdrawn Routes section.
// RFC 4271 Section 4.3 - IPv4 prefixes being withdrawn.
// Returns (nil, nil) if empty, (nil, error) if malformed.
func (u *WireUpdate) Withdrawn() ([]byte, error) {
	if len(u.payload) < 2 {
		return nil, fmt.Errorf("withdrawn: %w", ErrUpdateTruncated)
	}
	wdLen := uint32(binary.BigEndian.Uint16(u.payload[0:2]))
	if wdLen == 0 {
		return nil, nil
	}
	if uint32(len(u.payload)) < 2+wdLen { //nolint:gosec // G115: len bounded by BGP max message size
		return nil, fmt.Errorf("withdrawn: %w", ErrUpdateTruncated)
	}
	return u.payload[2 : 2+wdLen], nil
}

// Attrs returns the Path Attributes as an AttributesWire for lazy parsing.
// RFC 4271 Section 4.3 - Path attribute sequence.
// Returns (nil, nil) if empty, (nil, error) if malformed.
// AttributesWire is cheap to create (slice wrapper), so no caching needed.
func (u *WireUpdate) Attrs() (*attribute.AttributesWire, error) {
	return u.deriveAttrs()
}

// deriveAttrs extracts AttributesWire from payload.
func (u *WireUpdate) deriveAttrs() (*attribute.AttributesWire, error) {
	if len(u.payload) < 2 {
		return nil, fmt.Errorf("attrs: %w", ErrUpdateTruncated)
	}
	wdLen := uint32(binary.BigEndian.Uint16(u.payload[0:2]))
	attrLenOffset := 2 + wdLen
	if uint32(len(u.payload)) < attrLenOffset+2 { //nolint:gosec // G115: len bounded by BGP max message size
		return nil, fmt.Errorf("attrs: %w", ErrUpdateTruncated)
	}
	attrLen := uint32(binary.BigEndian.Uint16(u.payload[attrLenOffset:]))
	if attrLen == 0 {
		return nil, nil //nolint:nilnil // nil,nil = valid empty (no attributes)
	}
	attrStart := attrLenOffset + 2
	if uint32(len(u.payload)) < attrStart+attrLen { //nolint:gosec // G115: len bounded by BGP max message size
		return nil, fmt.Errorf("attrs: %w", ErrUpdateTruncated)
	}
	return attribute.NewAttributesWire(u.payload[attrStart:attrStart+attrLen], u.sourceCtxID), nil
}

// NLRI returns the Network Layer Reachability Information section.
// RFC 4271 Section 4.3 - IPv4 prefixes being announced.
// Returns (nil, nil) if empty, (nil, error) if malformed.
func (u *WireUpdate) NLRI() ([]byte, error) {
	if len(u.payload) < 2 {
		return nil, fmt.Errorf("nlri: %w", ErrUpdateTruncated)
	}
	wdLen := uint32(binary.BigEndian.Uint16(u.payload[0:2]))
	attrLenOffset := 2 + wdLen
	if uint32(len(u.payload)) < attrLenOffset+2 { //nolint:gosec // G115: len bounded by BGP max message size
		return nil, fmt.Errorf("nlri: %w", ErrUpdateTruncated)
	}
	attrLen := uint32(binary.BigEndian.Uint16(u.payload[attrLenOffset:]))
	nlriStart := attrLenOffset + 2 + attrLen
	if nlriStart > uint32(len(u.payload)) { //nolint:gosec // G115: len bounded by BGP max message size
		return nil, fmt.Errorf("nlri: %w", ErrUpdateTruncated)
	}
	if nlriStart == uint32(len(u.payload)) { //nolint:gosec // G115: len bounded by BGP max message size
		return nil, nil // No NLRI section (valid)
	}
	return u.payload[nlriStart:], nil
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
// Returns (nil, nil) if attributes section is empty.
// Returns (nil, error) if payload is malformed.
func (u *WireUpdate) AttrIterator() (*attribute.AttrIterator, error) {
	attrs, err := u.Attrs()
	if err != nil {
		return nil, err
	}
	if attrs == nil {
		return nil, nil //nolint:nilnil // nil,nil = valid empty
	}
	return attribute.NewAttrIterator(attrs.Packed()), nil
}
