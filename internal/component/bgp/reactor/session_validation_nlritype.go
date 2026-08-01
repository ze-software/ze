// Design: docs/architecture/core-design.md -- RFC 7606 UPDATE validation
// RFC: rfc/short/rfc7606.md -- Section 5.4 typed NLRI; rfc/short/rfc9552.md -- Section 5.2
// Overview: session_validation.go -- enforceRFC7606, which applies this
//
// RFC 7606 Section 5.4: "A BGP speaker advertising support for such a typed
// address family MUST handle routes with unrecognized NLRI types within that
// address family by discarding them, unless the relevant specification for that
// address family specifies otherwise."
//
// Two things decide where this runs.
//
// The escape clause is per family, so the ruling is per family and lives in the
// nlritype registry, registered by the plugin that owns the family. Nothing here
// names a family. RFC 9552 Section 5.2 uses that clause for BGP-LS and requires
// unknown Link-State NLRI types to be preserved and propagated, so a blanket
// discard would violate a MUST rather than meet one.
//
// The discard belongs at ingress because the RIB is not the propagation gate.
// reactorForwardRS (forward_rs.go) fans the RECEIVED wire straight to every
// eligible peer off this same read goroutine, and buildFwdBody (forward_body.go)
// appends peerWire.Payload() verbatim on the same-context path. A discard at the
// RIB would stop installation and leave the relay untouched, which is the half
// Section 5.4 exists for on a speaker that is only ever a control-plane relay
// for these families.

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/family"
)

// mpNLRIEdit is one MP attribute's replacement NLRI section.
type mpNLRIEdit struct {
	code    uint8
	nlri    []byte // the surviving NLRIs; empty means drop the whole attribute
	dropped int
}

// applyTypedNLRIDiscard removes every route with an unrecognized NLRI type from
// the UPDATE's MP attributes. It returns the UPDATE downstream consumers will
// see, together with that UPDATE's attribute section.
//
// Returns wu and pathAttrs unchanged, sharing the received wire bytes, whenever
// nothing was discarded. That covers every family with no Section 5.4 ruling and
// every conforming UPDATE, so the zero-copy relay survives for every peer that
// sends types ze implements. The rewrite allocates only when a route is
// genuinely discarded.
//
// pathAttrs must be the attribute section of wu.Payload() as it stands after the
// Section 3.g keep-first strip, and attrsOffset its start within that payload.
func (s *Session) applyTypedNLRIDiscard(
	wu *wireu.WireUpdate,
	pathAttrs []byte,
	attrsOffset int,
	mpReach, mpUnreach message.MPNLRILocation,
	addPathFor func(afi uint16, safi uint8) bool,
) (*wireu.WireUpdate, []byte) {
	edits := make([]mpNLRIEdit, 0, 2)
	edits = s.typedNLRIEdit(edits, uint8(attribute.AttrMPReachNLRI), mpReach, pathAttrs, addPathFor)
	edits = s.typedNLRIEdit(edits, uint8(attribute.AttrMPUnreachNLRI), mpUnreach, pathAttrs, addPathFor)
	if len(edits) == 0 {
		return wu, pathAttrs
	}

	newAttrs := rewriteMPNLRISections(pathAttrs, edits)
	oldCtxID := wu.SourceCtxID()
	oldSourceID := wu.SourceID()
	newBody := message.RebuildUpdateBody(wu.Payload(), newAttrs)
	rebuilt := wireu.NewWireUpdate(newBody, oldCtxID)
	rebuilt.SetSourceID(oldSourceID)
	// Keep pathAttrs a subslice of the body the caller now holds, so the
	// attribute-discard branch's in-place rewrite still shows through.
	return rebuilt, newBody[attrsOffset : attrsOffset+len(newAttrs)]
}

// typedNLRIEdit appends the edit for one MP attribute, or nothing when that
// attribute is absent, its family has no ruling, or every route survives.
func (s *Session) typedNLRIEdit(
	edits []mpNLRIEdit,
	code uint8,
	loc message.MPNLRILocation,
	pathAttrs []byte,
	addPathFor func(afi uint16, safi uint8) bool,
) []mpNLRIEdit {
	if !loc.Present {
		return edits
	}
	fam := family.Family{AFI: family.AFI(loc.AFI), SAFI: family.SAFI(loc.SAFI)}
	if !nlritype.Bound(fam) {
		return edits
	}

	_, _, value, found := attribute.AttrFind(pathAttrs, attribute.AttributeCode(code))
	if !found {
		return edits
	}
	start, ok := message.MPNLRIStart(code, value)
	if !ok {
		return edits
	}

	addPath := addPathFor != nil && addPathFor(loc.AFI, loc.SAFI)
	kept, dropped, err := nlritype.Retain(fam, value[start:], addPath)
	if err != nil {
		// The framing could not be trusted, so no NLRI boundary is knowable and no
		// discard decision can be made. Leaving the bytes alone keeps the behavior
		// ze had before Section 5.4 was enforced; inventing boundaries would rewrite
		// the wire from a guess. Malformed framing is Section 5.3's business, and it
		// has already run on this attribute.
		sessionLogger().Debug("RFC 7606 Section 5.4: NLRI framing not parseable, leaving routes alone",
			"peer", s.settings.Address, "family", fam, "error", err)
		return edits
	}
	if dropped == 0 {
		return edits
	}

	// RFC 7606 Section 6: name what was dropped, so an operator can trace a route
	// that stopped arriving back to the type ze does not implement.
	sessionLogger().Info("RFC 7606 Section 5.4: discarded routes with unrecognized NLRI types",
		"peer", s.settings.Address, "family", fam, "attr", code, "discarded", dropped)

	return append(edits, mpNLRIEdit{code: code, nlri: kept, dropped: dropped})
}

// rewriteMPNLRISections returns a new attribute section with each edited MP
// attribute's NLRI replaced, and an attribute whose NLRI is now empty removed.
//
// Attribute flags are preserved byte for byte, including the Extended Length
// bit. A replacement NLRI is never longer than the one it replaces, so the
// original length encoding always still fits and re-deciding it could only
// introduce a difference the peer did not send.
func rewriteMPNLRISections(pathAttrs []byte, edits []mpNLRIEdit) []byte {
	out := make([]byte, 0, len(pathAttrs))

	pos := 0
	for pos+2 <= len(pathAttrs) {
		attrStart := pos
		flags := pathAttrs[pos]
		code := pathAttrs[pos+1]
		pos += 2

		lenWidth := 1
		var attrLen int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				break
			}
			lenWidth = 2
			attrLen = int(pathAttrs[pos])<<8 | int(pathAttrs[pos+1])
		} else {
			if pos+1 > len(pathAttrs) {
				break
			}
			attrLen = int(pathAttrs[pos])
		}
		pos += lenWidth
		if pos+attrLen > len(pathAttrs) {
			break
		}
		value := pathAttrs[pos : pos+attrLen]
		pos += attrLen

		edit, edited := findEdit(edits, code)
		if !edited {
			out = append(out, pathAttrs[attrStart:pos]...)
			continue
		}
		if len(edit.nlri) == 0 {
			// Every route in this attribute was discarded. An MP_REACH announcing
			// nothing, or an MP_UNREACH withdrawing nothing, is not something to relay.
			continue
		}

		start, ok := message.MPNLRIStart(code, value)
		if !ok {
			out = append(out, pathAttrs[attrStart:pos]...)
			continue
		}
		newLen := start + len(edit.nlri)
		out = append(out, flags, code)
		if lenWidth == 2 {
			out = append(out, byte(newLen>>8), byte(newLen))
		} else {
			out = append(out, byte(newLen))
		}
		out = append(out, value[:start]...)
		out = append(out, edit.nlri...)
	}

	// Any trailing bytes the walk could not frame are copied through untouched. The
	// Section 4 bounds checks in the validator reject such a section before this
	// runs, so this is a belt on top of a brace rather than a live path.
	if pos < len(pathAttrs) {
		out = append(out, pathAttrs[pos:]...)
	}
	return out
}

// findEdit returns the edit for an attribute code. At most two edits exist, so a
// linear scan beats a map allocation.
func findEdit(edits []mpNLRIEdit, code uint8) (mpNLRIEdit, bool) {
	for _, e := range edits {
		if e.code == code {
			return e, true
		}
	}
	return mpNLRIEdit{}, false
}
