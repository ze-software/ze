// Design: docs/architecture/wire/nlri-bgpls.md -- Link-State NLRI framing
// RFC: rfc/short/rfc9552.md -- Section 8.2.2 fault management for BGP-LS
// Related: rfc7606_bgpls.go -- validateBGPLSAttr, the attribute half of the same section
// Related: rfc7606.go -- validateMPNLRISyntax, which calls the framing walk below
// Related: ../reactor/session_validation_nlritype.go -- the pass that applies the NLRI discard

package message

import (
	"bytes"
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// RFC 9552 syntactic validation of the Link-State NLRI on the session path.
//
// Section 8.2.2: "A BGP-LS Speaker MUST perform the following syntactic validation of the
// Link-State NLRI to determine if it is malformed." Seven bullets follow, and this file
// implements six of them. The seventh, "The syntactic correctness of the NLRI fields has
// been verified as per [RFC7606]", is the generic Section 4 and Section 5.3 framing that
// ValidateUpdateRFC7606AddPath already ran before either entry point here is reached.
//
// The six split by the ACTION Section 8.2.2 prescribes, and the split is what gives this
// file two entry points rather than one.
//
//	Bullet                                        Detected by             Action
//	------------------------------------------    --------------------   -----------------
//	TLV lengths sum to the MP_REACH length        validateBGPLSNLRI...   session reset
//	TLV lengths sum to the MP_UNREACH length      validateBGPLSNLRI...   session reset
//	TLV lengths sum to the Total NLRI Length      bgplsNLRIWellFormed    NLRI discard
//	TLV and recognized sub-TLV lengths are valid  bgplsNLRIWellFormed    NLRI discard
//	Section 5.1 TLV ordering is followed          bgplsNLRIWellFormed    NLRI discard
//	One instance of each Node Descriptor sub-TLV  bgplsNLRIWellFormed    NLRI discard
//
// Section 8.2.2 states the rule that assigns those actions: "When the error that is
// determined allows for the router to skip the malformed NLRI(s) and continue the
// processing of the rest of the BGP UPDATE message (e.g., when the TLV ordering rule is
// violated), then it MUST handle such malformed NLRIs as 'NLRI discard' (i.e., processing
// similar to what is described in Section 5.4 of [RFC7606]). In other cases, where the
// error in the NLRI encoding results in the inability to process the BGP UPDATE message
// (e.g., length-related encoding errors), then the router SHOULD handle such malformed
// NLRIs as 'AFI/SAFI disable' when other AFI/SAFI besides BGP-LS are being advertised over
// the same session. Alternately, the router MUST perform a 'session reset' when the session
// is only being used for BGP-LS or if 'AFI/SAFI disable' action is not possible."
//
// The parenthetical "length-related encoding errors" reads two ways, and the reading
// matters because it decides whether a peer can drop a session with one bad octet.
//
//   - Every length error is non-skipable. A TLV inside one NLRI that overruns that NLRI is
//     then a session reset.
//   - The normative test is the SENTENCE, and the parenthetical is one example of it. An
//     error is non-skipable when it leaves ze unable to process the UPDATE, which for an
//     NLRI section means unable to say where one NLRI ends and the next begins.
//
// The second reading governs here, because it is the one the sentence states and the first
// one contradicts the bullet list: a Node Descriptor carrying a sub-TLV twice is a
// length-free error, and under the first reading it would have no prescribed action at all.
// So the boundary is drawn at the Total NLRI Length field. An NLRI whose header or Total
// NLRI Length runs past the attribute makes every following boundary a guess, and that is
// the session reset. Everything inside a well-framed NLRI is skipable, and that is the
// discard.
//
// ze has no 'AFI/SAFI disable', so the SHOULD above does not apply and its "Alternately"
// MUST does: a non-skipable Link-State NLRI error is a session reset whatever else the
// session carries.

// Link-State NLRI framing, RFC 9552 Section 5.2 (Figures 5 and 6). The header is the same
// for both SAFIs; SAFI 72 inserts an 8-octet Route Distinguisher before the NLRI body, and
// the Total NLRI Length covers it.
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|            NLRI Type          |     Total NLRI Length         |   offsets 0..3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                Route Distinguisher (8 octets)                 |   offsets 4..11, SAFI 72 only
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Protocol-ID  |                                               |   offset 4 (12 for SAFI 72)
//	+-+-+-+-+-+-+-+-+                                               +
//	|                    Identifier (8 octets)                      |   offsets 5..12
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	//                    Descriptor TLVs (variable)               //   offset 13
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Section 5.2 defines the length field this way: "The Total NLRI Length field contains the
// cumulative length, in octets, of the rest of the NLRI, not including the NLRI Type field
// or itself. For VPN applications, it also includes the length of the Route Distinguisher".
const (
	// bgplsNLRIHeaderLen is the NLRI Type and Total NLRI Length fields.
	bgplsNLRIHeaderLen = 4
	// bgplsRDLen is the Route Distinguisher SAFI 72 carries (RFC 4364).
	bgplsRDLen = 8
	// bgplsNLRIFixedLen is the Protocol-ID and Identifier fields every NLRI Type this
	// document defines starts with (Figures 7, 8 and 9).
	bgplsNLRIFixedLen = 9
)

// BGP-LS families, RFC 9552 Section 5.2: "All non-VPN link, node, and prefix information
// SHALL be encoded using AFI 16388 / SAFI 71. VPN link, node, and prefix information SHALL
// be encoded using AFI 16388 / SAFI 72". Table 1 gives the NLRI Type range.
const (
	afiBGPLS      attribute.AFI  = 16388
	safiBGPLS     attribute.SAFI = 71
	safiBGPLSVPN  attribute.SAFI = 72
	bgplsNLRITop  uint16         = 4 // the highest NLRI Type this document defines (Table 1)
	bgplsNLRIBase uint16         = 1 // the lowest
)

// Node Descriptor TLVs, RFC 9552 Sections 5.2.1.2 and 5.2.1.3. These two are the TLVs whose
// value is a list of sub-TLVs, so they are the two Section 8.2.2 bullet 4 asks ze to walk
// into ("when the TLV is recognized then, the length of its sub-TLVs") and the two bullet 7
// names ("For NLRIs carrying either a Local or Remote Node Descriptor TLV, there is not more
// than one instance of a sub-TLV present").
const (
	bgplsTLVLocalNodeDescriptors  uint16 = 256
	bgplsTLVRemoteNodeDescriptors uint16 = 257
)

// NLRISyntaxRuled reports whether the family's own specification prescribes discarding an
// individual malformed NLRI, so the receive path knows to call RetainWellFormedNLRI.
//
// It exists so the RFC 7606 Section 5.4 pass can skip locating the MP attribute at all for
// a family nothing rules on, which is every family but BGP-LS today. The alternative is an
// attribute-section walk on every UPDATE ze receives.
func NLRISyntaxRuled(afi attribute.AFI, safi attribute.SAFI) bool {
	return afi == afiBGPLS && (safi == safiBGPLS || safi == safiBGPLSVPN)
}

// RetainWellFormedNLRI returns section with every malformed NLRI removed, and the number
// removed. It answers the skipable half of RFC 9552 Section 8.2.2 for BGP-LS, and returns
// section unchanged for every family whose specification prescribes no per-NLRI discard.
//
// ok is false when the NLRI framing cannot be walked at all. The boundaries between NLRIs
// are then unknowable, so no discard decision is possible and the caller must reach for the
// session reset the non-skipable half prescribes. Section 8.2.2's own list starts there:
// "The sum of all TLV lengths found in the BGP MP_REACH_NLRI attribute corresponds to the
// BGP MP_REACH_NLRI length."
//
// It returns section itself, sharing the caller's backing array, whenever nothing was
// removed. That is every conforming UPDATE, so a well-formed BGP-LS UPDATE stays zero-copy
// and this walk allocates only when a peer sends something malformed.
//
// addPath reflects RFC 7911 negotiation for the family. BGP-LS defines no use for a path
// identifier, but a peer that negotiated ADD-PATH for the family sends one, and reading its
// first two octets as an NLRI Type would condemn well-formed NLRIs.
func RetainWellFormedNLRI(
	afi attribute.AFI,
	safi attribute.SAFI,
	section []byte,
	addPath bool,
) (kept []byte, dropped int, ok bool) {
	if len(section) == 0 || !NLRISyntaxRuled(afi, safi) {
		return section, 0, true
	}
	vpn := safi == safiBGPLSVPN

	// One pass to count, so the common "nothing dropped" answer costs no allocation and the
	// rewrite below is sized exactly. Both passes are bounded by len(section), which the
	// caller read from the wire, and each iteration advances off past a header it checked.
	keepBytes := 0
	for off := 0; off < len(section); {
		start, end, framed := bgplsNLRIBounds(section, off, addPath)
		if !framed {
			return section, 0, false
		}
		if bgplsNLRIWellFormed(section[start:end], vpn) {
			keepBytes += end - off
		} else {
			dropped++
		}
		off = end
	}
	if dropped == 0 {
		return section, 0, true
	}
	if keepBytes == 0 {
		return nil, dropped, true
	}

	out := make([]byte, 0, keepBytes)
	for off := 0; off < len(section); {
		start, end, framed := bgplsNLRIBounds(section, off, addPath)
		if !framed {
			return section, 0, false
		}
		if bgplsNLRIWellFormed(section[start:end], vpn) {
			out = append(out, section[off:end]...)
		}
		off = end
	}
	return out, dropped, true
}

// bgplsNLRIBounds locates the NLRI that starts at off. start is the offset of its NLRI Type
// field, past the RFC 7911 path identifier when one is present, and end is the offset one
// past its last octet. framed is false when the header or the Total NLRI Length runs past
// the section.
//
// The path identifier stays outside [start, end) because it is not part of the NLRI the
// syntactic walk judges, and inside [off, end) because the caller that keeps this NLRI must
// keep the identifier with it.
func bgplsNLRIBounds(section []byte, off int, addPath bool) (start, end int, framed bool) {
	start = off
	if addPath {
		start += 4
	}
	if start+bgplsNLRIHeaderLen > len(section) {
		return 0, 0, false
	}
	total := int(binary.BigEndian.Uint16(section[start+2 : start+4]))
	end = start + bgplsNLRIHeaderLen + total
	if end > len(section) {
		return 0, 0, false
	}
	return start, end, true
}

// bgplsNLRIWellFormed reports whether one Link-State NLRI passes the four Section 8.2.2
// bullets that are skipable. one is the NLRI from its NLRI Type field to its last octet, as
// bgplsNLRIBounds carved it, so its Total NLRI Length is already known to fit.
//
// An NLRI Type this document does not define is well-formed by construction, and reading
// one octet of its body would be the defect. RFC 9552 Section 5.2: "An implementation MUST
// handle unknown Link-State NLRI types as opaque objects and MUST preserve and propagate
// them." Nothing is known about the layout of a type introduced after this code was
// written, so nothing about it can be called malformed.
//
// No TLV type beyond the two Node Descriptors is recognized, and no octet of any TLV value
// is read. Section 8.2.2 opens by forbidding it: "A Link-State NLRI MUST NOT be considered
// malformed or invalid based on the inclusion/exclusion of TLVs or contents of the TLV
// fields (i.e., semantic errors)", and it names "the length of a fixed-length TLV is
// correct" among the semantic validations a BGP-LS Propagator does not perform. So a Node
// Descriptor sub-TLV 512 whose length is 7 rather than the 4 Table 3 gives is kept.
func bgplsNLRIWellFormed(one []byte, vpn bool) bool {
	nlriType := binary.BigEndian.Uint16(one[0:2])
	if nlriType < bgplsNLRIBase || nlriType > bgplsNLRITop {
		return true
	}

	body := one[bgplsNLRIHeaderLen:]
	off := 0
	if vpn {
		if len(body) < bgplsRDLen {
			return false
		}
		off = bgplsRDLen
	}
	// RFC 9552 Section 8.2.2: "The sum of all TLV lengths found in a Link-State NLRI
	// corresponds to the Total NLRI Length field of all its descriptors." A body that
	// cannot even hold the Protocol-ID and the Identifier the NLRI Type declares fails that
	// sum before the first TLV is read.
	if off+bgplsNLRIFixedLen > len(body) {
		return false
	}
	off += bgplsNLRIFixedLen

	// The walk is bounded by the body length, which bgplsNLRIBounds proved is inside the
	// attribute. Every iteration advances off by at least bgplsTLVHeaderLen, so a peer
	// cannot stall it.
	previousType := uint16(0)
	previousValue := []byte(nil)
	first := true
	for off < len(body) {
		if off+bgplsTLVHeaderLen > len(body) {
			return false
		}
		tlvType := binary.BigEndian.Uint16(body[off : off+2])
		tlvLen := int(binary.BigEndian.Uint16(body[off+2 : off+4]))
		off += bgplsTLVHeaderLen
		// RFC 9552 Section 8.2.2: "The length of the TLVs and, when the TLV is recognized
		// then, the length of its sub-TLVs in the NLRI are valid."
		if off+tlvLen > len(body) {
			return false
		}
		value := body[off : off+tlvLen]
		off += tlvLen

		// RFC 9552 Section 8.2.2: "The rule regarding the ordering of TLVs has been
		// followed as described in Section 5.1."
		if !first && !bgplsTLVOrdered(previousType, previousValue, tlvType, value) {
			return false
		}
		if tlvType == bgplsTLVLocalNodeDescriptors || tlvType == bgplsTLVRemoteNodeDescriptors {
			if !bgplsNodeDescriptorWellFormed(value) {
				return false
			}
		}
		previousType = tlvType
		previousValue = value
		first = false
	}
	return true
}

// bgplsTLVOrdered reports whether two consecutive TLVs of one Link-State NLRI are in the
// order RFC 9552 Section 5.1 requires.
//
// Section 5.1: "To compare NLRIs with unknown TLVs, all TLVs within the NLRI MUST be
// ordered in ascending order by TLV Type. If there are multiple TLVs of the same type
// within a single NLRI, then the TLVs sharing the same type MUST be first in ascending
// order based on the Length field followed by ascending order based on the Value field.
// Comparison of the Value fields is performed by treating the entire field as opaque binary
// data and ordered lexicographically." The same paragraph makes the violation malformed:
// "NLRIs having TLVs that do not follow the above ordering rules MUST be considered as
// malformed by a BGP-LS Propagator."
//
// Length is compared before Value, and bytes.Compare alone would not do that: it orders a
// shorter value first only when it is a prefix of the longer one, so {0xff} would sort
// after {0x00, 0x00} while Section 5.1 puts it first.
//
// Two identical TLVs are ordered. Section 5.1 anticipates "multiple TLVs of the same type"
// and states no uniqueness rule for them, and Section 8.2.2 forbids judging an NLRI
// malformed on which TLVs it includes.
func bgplsTLVOrdered(previousType uint16, previousValue []byte, tlvType uint16, value []byte) bool {
	if previousType != tlvType {
		return previousType < tlvType
	}
	if len(previousValue) != len(value) {
		return len(previousValue) < len(value)
	}
	return bytes.Compare(previousValue, value) <= 0
}

// bgplsNodeDescriptorWellFormed reports whether the sub-TLVs of one Local or Remote Node
// Descriptor TLV frame correctly and appear at most once each.
//
// RFC 9552 Section 8.2.2 bullet 7: "For NLRIs carrying either a Local or Remote Node
// Descriptor TLV, there is not more than one instance of a sub-TLV present." Section 5.2.1.4
// states the same rule beside the ordering rule that makes it cheap to enforce: "At most,
// there MUST be one instance of each sub-TLV type present in any Node Descriptor. The
// sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type."
//
// So the test is one strict comparison per sub-TLV: a type that does not exceed its
// predecessor is either a duplicate, which bullet 7 forbids, or out of order, which
// Section 5.2.1.4 forbids. Detecting a duplicate without relying on the order would need a
// set keyed by a 16-bit type, and a peer chooses how many sub-TLVs it sends, so that set
// would be either an allocation or a quadratic scan on the receive path.
//
// The uniqueness rule is about sub-TLV TYPES, so no value is read and no length is judged
// against Table 3. Section 8.2.2 puts a fixed-length TLV's length among the semantic
// validations a Propagator does not perform.
func bgplsNodeDescriptorWellFormed(value []byte) bool {
	previousType := uint16(0)
	first := true
	// Bounded by len(value), which the caller proved is inside the NLRI body. Every
	// iteration advances off by at least bgplsTLVHeaderLen.
	for off := 0; off < len(value); {
		if off+bgplsTLVHeaderLen > len(value) {
			return false
		}
		subType := binary.BigEndian.Uint16(value[off : off+2])
		subLen := int(binary.BigEndian.Uint16(value[off+2 : off+4]))
		off += bgplsTLVHeaderLen
		if off+subLen > len(value) {
			return false
		}
		off += subLen
		if !first && subType <= previousType {
			return false
		}
		previousType = subType
		first = false
	}
	return true
}

// validateBGPLSNLRISyntax answers the non-skipable half of RFC 9552 Section 8.2.2 for a
// Link-State NLRI section: the first two bullets, "The sum of all TLV lengths found in the
// BGP MP_REACH_NLRI attribute corresponds to the BGP MP_REACH_NLRI length" and the same
// sentence for MP_UNREACH_NLRI. At this level a "TLV" is one Link-State NLRI, whose Type
// and Total NLRI Length are the TLV header of Section 5.1 (Figure 4).
//
// The two ways that sum can fail are one fact stated twice: a Total NLRI Length that runs
// past the section, and a tail too short to hold another NLRI header.
//
// Only the headers are read. Everything inside a well-framed NLRI is skipable and belongs
// to RetainWellFormedNLRI, which the receive path calls after this.
//
// The verdict is session reset. Section 8.2.2 reaches it through 'AFI/SAFI disable', which
// ze does not implement, and its "Alternately, the router MUST perform a 'session reset'".
// RFC 7606 Section 3(j) reaches the same verdict independently: treat-as-withdraw needs the
// NLRI field parsed, and here it cannot be.
func validateBGPLSNLRISyntax(code uint8, nlri []byte, addPath bool) *RFC7606ValidationResult {
	// Bounded by len(nlri), which the caller bounds-checked against the attribute value.
	for off := 0; off < len(nlri); {
		_, end, framed := bgplsNLRIBounds(nlri, off, addPath)
		if !framed {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionSessionReset,
				AttrCode:    code,
				Description: "RFC 9552 Section 8.2.2: Link-State NLRI lengths do not sum to the MP attribute length",
			}
		}
		off = end
	}
	return nil
}
