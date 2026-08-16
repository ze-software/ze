// Design: docs/architecture/wire/messages.md — wire UPDATE lazy parsing
// RFC: rfc/short/rfc4271.md — UPDATE message wire format (Section 4.3)
// RFC: rfc/short/rfc4760.md — multiprotocol NLRI in UPDATE
// RFC: rfc/short/rfc7606.md — Section 5.1 one NLRI-bearing field per UPDATE

package wireu

import (
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"
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
// Thread-safe for concurrent read access via sync.Once guards on lazy fields.
type WireUpdate struct {
	payload     []byte
	sourceCtxID bgpctx.ContextID
	messageID   uint64          // Unique ID set by reactor after creation
	sourceID    source.SourceID // Source that sent/created this message

	sectionOnce sync.Once
	sections    wire.UpdateSections
	parseErr    error

	attrsOnce      sync.Once
	cachedAttrs    *attribute.AttributesWire
	cachedAttrsErr error

	shapeOnce  sync.Once
	shapeMixed bool
}

// NewWireUpdate creates a WireUpdate from raw UPDATE payload bytes.
// Takes ownership conceptually - caller should not modify payload after this call.
func NewWireUpdate(payload []byte, ctxID bgpctx.ContextID) *WireUpdate {
	return &WireUpdate{
		payload:     payload,
		sourceCtxID: ctxID,
	}
}

// InitWireUpdate initializes a caller-owned WireUpdate in place, avoiding
// the heap allocation from NewWireUpdate. The WireUpdate must be zero-valued
// (not previously used) since sync.Once fields are not safe to reinitialize.
func InitWireUpdate(wu *WireUpdate, payload []byte, ctxID bgpctx.ContextID) {
	wu.payload = payload
	wu.sourceCtxID = ctxID
}

func (u *WireUpdate) ensureParsed() {
	u.sectionOnce.Do(func() {
		sections, err := wire.ParseUpdateSections(u.payload)
		if err != nil {
			u.parseErr = ErrUpdateTruncated
			return
		}
		u.sections = sections
	})
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

// Attrs returns the Path Attributes as an AttributesWire for lazy value parsing.
// RFC 4271 Section 4.3 - Path attribute sequence.
// Returns (nil, nil) if empty, (nil, error) if malformed.
//
// The AttributesWire built here indexes the attribute section eagerly and is
// immutable afterwards, so this call FREEZES which attributes this UPDATE has.
// Any code that rewrites the attribute bytes must therefore run BEFORE the first
// Attrs call, or wrap its result in a new WireUpdate. On the receive path that
// ordering is the reason reactor.Session.publishBase (reactor/session_validation.go)
// is the LAST step of enforceRFC7606: every rewrite runs ahead of it. There are
// three, and the list is the ordering contract rather than an example -- the RFC
// 7606 Section 3.g duplicate strip, the in-place attribute-discard branch, and
// the RFC 4271 Section 9 Partial-bit stamp publishBase applies itself before it
// calls this method.
func (u *WireUpdate) Attrs() (*attribute.AttributesWire, error) {
	u.attrsOnce.Do(func() {
		u.ensureParsed()
		if u.parseErr != nil {
			u.cachedAttrsErr = fmt.Errorf("attrs: %w", u.parseErr)
			return
		}
		attrBytes := u.sections.Attrs(u.payload)
		if attrBytes == nil {
			return
		}
		u.cachedAttrs = attribute.NewAttributesWire(attrBytes, u.sourceCtxID)
	})
	return u.cachedAttrs, u.cachedAttrsErr
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

// MixesNLRIFields reports whether this UPDATE carries more than one of the four
// NLRI-bearing fields, which RFC 7606 Section 5.1 forbids a sender from
// emitting: "An UPDATE message MUST NOT contain more than one of the following:
// non-empty Withdrawn Routes field, non-empty Network Layer Reachability
// Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute."
//
// The verdict is computed once and cached. A received UPDATE is walked once per
// destination peer in the forward loop (reactor/forward_body.go), so without the
// cache the check would cost per peer rather than per message. The common
// single-field UPDATE therefore keeps the zero-copy forward: one bool read.
//
// A payload whose sections do not parse reports false. Classifying shape is not
// validating -- everything the forward path sees has already been through
// enforceRFC7606 (reactor/session_read.go:162), and reporting a violation for
// bytes that could not be read would invent one.
func (u *WireUpdate) MixesNLRIFields() bool {
	u.shapeOnce.Do(func() {
		u.ensureParsed()
		if u.parseErr != nil {
			return
		}
		u.shapeMixed = message.NLRIBearingFieldCount(
			u.sections.Withdrawn(u.payload),
			u.sections.Attrs(u.payload),
			u.sections.NLRI(u.payload),
		) > 1
	})
	return u.shapeMixed
}

// Snapshot returns a new WireUpdate with an owned copy of the payload.
// Use when the original payload references a pool buffer that may be
// reused before all consumers finish reading (e.g., fire-and-forget delivery).
//
// This is buffer-lifetime contract A: an eager Own-by-copy taken before the
// fire-and-forget delivery boundary, so the returned WireUpdate no longer
// borrows the recyclable receive buffer. See
// docs/architecture/memory/lifetime-contracts.md.
//
// The attribute span index is carried across the copy rather than rebuilt. Span
// offsets are relative to the attribute section, and the copy is byte-identical,
// so every span is valid against the new array without arithmetic. The index
// holds offsets and never bytes, so it inherits the copy's lifetime exactly.
func (u *WireUpdate) Snapshot() *WireUpdate {
	owned := make([]byte, len(u.payload))
	copy(owned, u.payload)
	s := NewWireUpdate(owned, u.sourceCtxID)
	s.messageID = u.messageID
	s.sourceID = u.sourceID

	srcAttrs, srcErr := u.Attrs()
	s.attrsOnce.Do(func() {
		s.ensureParsed()
		if s.parseErr != nil {
			// The copy is byte-identical, so a section parse that failed on the
			// original fails here with the same verdict.
			s.cachedAttrsErr = fmt.Errorf("attrs: %w", s.parseErr)
			return
		}
		if srcErr != nil {
			s.cachedAttrsErr = srcErr
			return
		}
		if srcAttrs == nil {
			return // no attribute section
		}
		s.cachedAttrs = srcAttrs.CarryOver(s.sections.Attrs(s.payload))
	})
	return s
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
