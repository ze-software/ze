// Design: docs/architecture/wire/qualifiers.md — the label field, Rsrv on relay
// Overview: session_validation.go — publishBase, the ingest step that calls this
// RFC: rfc/short/rfc8277.md — Section 2.2, the Rsrv field

package reactor

import (
	"bytes"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// labelFieldOctets is the size of one label field: Label (20 bits), Rsrv (3),
// S (1).
const labelFieldOctets = 3

// labelRsrvMask selects the three Rsrv bits in the last octet of a label field,
// and labelBottomMask the S bit beside them.
const (
	labelRsrvMask   = 0x0E
	labelBottomMask = 0x01
)

// labelPathIDOctets is the ADD-PATH Path Identifier that precedes each NLRI
// (RFC 7911 Section 3).
const labelPathIDOctets = 4

// clearLabelRsrv returns wu unchanged when no label field of its MP_REACH_NLRI
// carries a non-zero Rsrv bit, and otherwise a new WireUpdate whose label fields
// carry Rsrv 000, every other octet as received.
//
// It is an ingest step of publishBase, beside removeRedundantLargeCommunities and
// for its reason: the bytes it returns are the bytes the RIB retains, both
// forward rails relay zero-copy and every rebuild copies from, so Ze relays a
// zero Rsrv whichever rail sends the route. The common case, a peer that sends
// zero, only reads the NLRI and allocates nothing.
//
// Only SAFI 4 and SAFI 128 carry the field (RFC 8277 Section 2). An NLRI whose
// stack has no S bit inside its length is left as it came: nothing in it can be
// told apart as a label field.
//
// The labeled NLRI (RFC 8277 Section 2.2, Section 2.3 for a stack; under
// ADD-PATH a 4-octet Path Identifier precedes each one):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|    Length     |                 Label                 |Rsrv |S|
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  (more label fields, until S = 1), then the Prefix            ~
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Octet 0 is the length in bits of the rest; the Rsrv bits of each label field
// are bits 1-3 of its third octet.
func clearLabelRsrv(wu *wireu.WireUpdate, addPathFor func(family.Family) bool) *wireu.WireUpdate {
	payload := wu.Payload()
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		// publishBase reports an unparseable payload through wu.Attrs().
		return wu
	}
	attrs := sections.Attrs(payload)
	hdrStart, flags, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if !found {
		return wu
	}
	fam, nlriStart, ok := mpReachLabeledNLRI(value)
	if !ok {
		return wu
	}
	addPath := addPathFor(fam)
	if !labelRsrvWalk(value[nlriStart:], addPath, false) {
		return wu
	}

	headerOctets := 3
	if flags.IsExtLength() {
		headerOctets = 4
	}
	rewritten := bytes.Clone(attrs)
	start := hdrStart + headerOctets + nlriStart
	labelRsrvWalk(rewritten[start:hdrStart+headerOctets+len(value)], addPath, true)

	result := wireu.NewWireUpdate(message.RebuildUpdateBody(payload, rewritten), wu.SourceCtxID())
	result.SetSourceID(wu.SourceID())
	result.SetMessageID(wu.MessageID())
	sessionLogger().Debug("RFC 8277 Section 2.2: cleared non-zero Rsrv bits before relay",
		"family", fam.String())
	return result
}

// mpReachLabeledNLRI answers the family of an MP_REACH_NLRI value and the offset
// of its NLRI, when that family carries label fields.
//
// MP_REACH_NLRI value (RFC 4760 Section 3): AFI (2), SAFI (1), Length of Next
// Hop (1), Next Hop (variable), Reserved (1), NLRI (variable).
func mpReachLabeledNLRI(value []byte) (family.Family, int, bool) {
	if len(value) < 5 {
		return family.Family{}, 0, false
	}
	fam := family.Family{AFI: family.AFI(uint16(value[0])<<8 | uint16(value[1])), SAFI: family.SAFI(value[2])}
	// Only the two SAFIs RFC 8277 Section 2 encodes carry a label field.
	switch fam.SAFI {
	case family.SAFIMPLSLabel, family.SAFIVPN:
	default:
		return family.Family{}, 0, false
	}
	nlriStart := 4 + int(value[3]) + 1
	if nlriStart > len(value) {
		return family.Family{}, 0, false
	}
	return fam, nlriStart, true
}

// labelRsrvWalk visits every label field of the labeled NLRIs in nlri. With
// clear false it reports whether any field carries a non-zero Rsrv bit and
// writes nothing; with clear true it zeroes those bits in place and reports
// whether it zeroed any.
//
// The loop is bounded by len(nlri): each NLRI advances pos by at least its
// length octet.
func labelRsrvWalk(nlri []byte, addPath, clear bool) bool {
	hit := false
	pos := 0
	for pos < len(nlri) {
		if addPath {
			pos += labelPathIDOctets
		}
		if pos >= len(nlri) {
			return hit
		}
		end := pos + 1 + (int(nlri[pos])+7)/8
		if end > len(nlri) {
			return hit
		}
		bottom := labelStackBottom(nlri[pos+1 : end])
		for field := pos + 1; bottom >= 0 && field <= pos+1+bottom; field += labelFieldOctets {
			last := field + labelFieldOctets - 1
			if nlri[last]&labelRsrvMask == 0 {
				continue
			}
			if !clear {
				return true
			}
			hit = true
			// RFC 8277 Section 2.2: "Rsrv: This 3-bit field SHOULD be set to zero
			// on transmission and MUST be ignored on reception."
			nlri[last] &^= labelRsrvMask
		}
		pos = end
	}
	return hit
}

// labelStackBottom returns the offset in body of the label field whose S bit is
// set, or -1 when no whole field inside body has it.
func labelStackBottom(body []byte) int {
	for field := 0; field+labelFieldOctets <= len(body); field += labelFieldOctets {
		if body[field+labelFieldOctets-1]&labelBottomMask != 0 {
			return field
		}
	}
	return -1
}
