package api

import (
	"encoding/binary"
	"sync"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
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
	messageID   uint64 // Unique ID set by reactor after creation

	// Cached AttributesWire - lazily initialized, thread-safe
	attrsOnce sync.Once
	attrs     *attribute.AttributesWire
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
// Returns nil if empty or malformed.
func (u *WireUpdate) Withdrawn() []byte {
	if len(u.payload) < 2 {
		return nil
	}
	wdLen := uint32(binary.BigEndian.Uint16(u.payload[0:2]))
	if wdLen == 0 {
		return nil
	}
	if uint32(len(u.payload)) < 2+wdLen { //nolint:gosec // G115: len bounded by BGP max message size
		return nil
	}
	return u.payload[2 : 2+wdLen]
}

// Attrs returns the Path Attributes as an AttributesWire for lazy parsing.
// RFC 4271 Section 4.3 - Path attribute sequence.
// Returns nil if empty or malformed.
// Result is cached - subsequent calls return the same instance.
func (u *WireUpdate) Attrs() *attribute.AttributesWire {
	u.attrsOnce.Do(func() {
		u.attrs = u.deriveAttrs()
	})
	return u.attrs
}

// deriveAttrs extracts AttributesWire from payload.
func (u *WireUpdate) deriveAttrs() *attribute.AttributesWire {
	if len(u.payload) < 2 {
		return nil
	}
	wdLen := uint32(binary.BigEndian.Uint16(u.payload[0:2]))
	attrLenOffset := 2 + wdLen
	if uint32(len(u.payload)) < attrLenOffset+2 { //nolint:gosec // G115: len bounded by BGP max message size
		return nil
	}
	attrLen := uint32(binary.BigEndian.Uint16(u.payload[attrLenOffset:]))
	if attrLen == 0 {
		return nil
	}
	attrStart := attrLenOffset + 2
	if uint32(len(u.payload)) < attrStart+attrLen { //nolint:gosec // G115: len bounded by BGP max message size
		return nil
	}
	return attribute.NewAttributesWire(u.payload[attrStart:attrStart+attrLen], u.sourceCtxID)
}

// NLRI returns the Network Layer Reachability Information section.
// RFC 4271 Section 4.3 - IPv4 prefixes being announced.
// Returns nil if empty or malformed.
func (u *WireUpdate) NLRI() []byte {
	if len(u.payload) < 2 {
		return nil
	}
	wdLen := uint32(binary.BigEndian.Uint16(u.payload[0:2]))
	attrLenOffset := 2 + wdLen
	if uint32(len(u.payload)) < attrLenOffset+2 { //nolint:gosec // G115: len bounded by BGP max message size
		return nil
	}
	attrLen := uint32(binary.BigEndian.Uint16(u.payload[attrLenOffset:]))
	nlriStart := attrLenOffset + 2 + attrLen
	if nlriStart >= uint32(len(u.payload)) { //nolint:gosec // G115: len bounded by BGP max message size
		return nil
	}
	return u.payload[nlriStart:]
}

// MPReach extracts MP_REACH_NLRI (attribute code 14) as MPReachWire.
// RFC 4760 Section 3 - Multiprotocol reachability.
// Returns nil if attribute not present or malformed.
func (u *WireUpdate) MPReach() MPReachWire {
	attrs := u.Attrs()
	if attrs == nil {
		return nil
	}
	raw, err := attrs.GetRaw(attribute.AttrMPReachNLRI)
	if err != nil || len(raw) < 5 {
		return nil
	}
	return MPReachWire(raw)
}

// MPUnreach extracts MP_UNREACH_NLRI (attribute code 15) as MPUnreachWire.
// RFC 4760 Section 4 - Multiprotocol unreachability.
// Returns nil if attribute not present or malformed.
func (u *WireUpdate) MPUnreach() MPUnreachWire {
	attrs := u.Attrs()
	if attrs == nil {
		return nil
	}
	raw, err := attrs.GetRaw(attribute.AttrMPUnreachNLRI)
	if err != nil || len(raw) < 3 {
		return nil
	}
	return MPUnreachWire(raw)
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
