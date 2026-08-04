// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc7606.md — revised error handling for UPDATE messages
// Related: attr_discard.go — ATTR_TOMBSTONE in-place marker implementation

package message

import (
	"encoding/binary"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// RFC7606Action represents the error handling action per RFC 7606.
type RFC7606Action int

// RFC 7606 Section 2: action strength ordering (strongest to weakest):
// session-reset > treat-as-withdraw > attribute-discard > none
// Iota values match this ordering so numeric comparison gives strongest action.
const (
	// RFC7606ActionNone - No error detected.
	RFC7606ActionNone RFC7606Action = iota
	// RFC7606ActionAttributeDiscard - Discard malformed attribute, continue (RFC 7606 Section 2).
	RFC7606ActionAttributeDiscard
	// RFC7606ActionTreatAsWithdraw - Treat UPDATE as withdrawal (RFC 7606 Section 2).
	RFC7606ActionTreatAsWithdraw
	// RFC7606ActionSessionReset - Reset session (RFC 4271 behavior).
	RFC7606ActionSessionReset
)

func (a RFC7606Action) String() string {
	switch a {
	case RFC7606ActionNone:
		return "none"
	case RFC7606ActionTreatAsWithdraw:
		return "treat-as-withdraw"
	case RFC7606ActionAttributeDiscard:
		return "attribute-discard"
	case RFC7606ActionSessionReset:
		return "session-reset"
	default:
		return "unknown"
	}
}

// AttrRange is a byte range [Start, End) within a path-attributes section,
// identifying one whole attribute (flags + type + length + value).
type AttrRange struct {
	Start int
	End   int
}

// RFC7606ValidationResult contains the result of UPDATE validation.
type RFC7606ValidationResult struct {
	Action         RFC7606Action
	AttrCode       uint8          // Attribute code that caused the strongest error (0 if N/A)
	Reason         uint8          // Discard reason code (draft-mangin-idr-attr-tombstone-00 Section 4.4)
	Description    string         // Human-readable error description for the strongest error
	DiscardEntries []DiscardEntry // Attributes to discard with reason codes when Action is AttributeDiscard
	// DuplicateRanges holds the byte range of every later (keep-first) occurrence
	// of a non-MP attribute code that appeared more than once. RFC 7606 Section 3.g
	// says duplicates other than MP_REACH/MP_UNREACH must be handled by discarding
	// all but the first occurrence; recording the ranges here lets the enforcement
	// layer actually strip those bytes (StripAttrRanges) so every downstream
	// consumer sees a single copy, rather than merely skipping their validation and
	// leaving the duplicate on the wire. Ranges are in ascending order and
	// non-overlapping (one forward parse). Empty when there are no duplicates.
	DuplicateRanges []AttrRange
	// PrefixSIDPresent reports a well-framed Prefix-SID attribute (code 40, RFC 8669) in
	// the attribute section. The EBGP-boundary check of RFC 8669 Section 4
	// (reactor.enforceRFC7606) reads it. That check no longer walks these bytes again.
	//
	// Only the two returns below a completed forward parse set it. Every earlier return
	// abandons the parse and leaves it false. It therefore never reports absence for
	// bytes the walk did not read.
	//
	// That false is safe, not merely cautious. Each of those returns carries
	// treat-as-withdraw or session-reset. Section 4's discard applies only under a weaker
	// action. The one early return that carries RFC7606ActionNone is the empty section,
	// which holds no attribute of any code.
	PrefixSIDPresent bool
	// MPReachNLRI and MPUnreachNLRI locate the NLRI portion of the MP_REACH_NLRI
	// and MP_UNREACH_NLRI attributes, observed on this walk so the RFC 7606
	// Section 5.4 typed-NLRI check (reactor.enforceRFC7606) does not repeat it.
	// Zero-valued (Present false) when the attribute is absent, or when its
	// header is too short to locate the NLRI at all.
	//
	// At most one of each survives validation: Section 3.g session-resets a
	// duplicate MP attribute, so a later occurrence never overwrites an earlier
	// one in a result the caller acts on.
	MPReachNLRI   MPNLRILocation
	MPUnreachNLRI MPNLRILocation
}

// Attribute type codes per RFC 4271.
const (
	attrCodeOrigin         uint8 = 1
	attrCodeASPath         uint8 = 2
	attrCodeNextHop        uint8 = 3
	attrCodeMED            uint8 = 4
	attrCodeLocalPref      uint8 = 5
	attrCodeAtomicAgg      uint8 = 6
	attrCodeAggregator     uint8 = 7
	attrCodeCommunity      uint8 = 8
	attrCodeOriginatorID   uint8 = 9
	attrCodeClusterList    uint8 = 10
	attrCodeMPReachNLRI    uint8 = 14
	attrCodeMPUnreachNLRI  uint8 = 15
	attrCodeExtCommunity   uint8 = 16
	attrCodeLargeCommunity uint8 = 32
	attrCodePrefixSID      uint8 = 40
)

// Attribute flags bits (RFC 4271 Section 4.3).
const (
	attrFlagOptional   uint8 = 0x80 // Bit 0: Optional (1) vs Well-known (0)
	attrFlagTransitive uint8 = 0x40 // Bit 1: Transitive (1) vs Non-transitive (0)
)

// wellKnownAttrs lists attributes that are well-known (not optional).
// Well-known attributes must NOT have the Optional bit set.
// Well-known attributes MUST have the Transitive bit set.
var wellKnownAttrs = map[uint8]bool{
	attrCodeOrigin:    true,
	attrCodeASPath:    true,
	attrCodeNextHop:   true,
	attrCodeLocalPref: true, // Well-known for IBGP
	attrCodeAtomicAgg: true,
}

// validateAttributeFlags validates attribute flags per RFC 7606 Section 3.c.
//
// Well-known attributes must:
// - NOT have the Optional bit set (they are mandatory)
// - MUST have the Transitive bit set
//
// Returns nil if valid, or RFC7606ValidationResult with treat-as-withdraw action.
func validateAttributeFlags(code, flags uint8) *RFC7606ValidationResult {
	// RFC 7606 Section 5.3: the MP_REACH_NLRI/MP_UNREACH_NLRI attribute is "incorrect" if
	// its flags are inconsistent with RFC 4760, which defines both as optional (Optional bit
	// set) and non-transitive (Transitive bit clear). Section 3(j) escalates that to session
	// reset -- STRONGER than the generic Section 3.c treat-as-withdraw for a well-known flag
	// conflict -- because an MP attribute whose framing is in doubt cannot have its NLRI
	// boundaries trusted. Only the Optional and Transitive bits are constrained: the
	// Extended-Length bit (0x10) is a legal encoding choice and the Partial bit (0x20) is
	// not restricted by RFC 4760, so neither is rejected here.
	if code == attrCodeMPReachNLRI || code == attrCodeMPUnreachNLRI {
		if flags&attrFlagOptional == 0 || flags&attrFlagTransitive != 0 {
			var b textbuf.Buffer
			return &RFC7606ValidationResult{
				Action:   RFC7606ActionSessionReset,
				AttrCode: code,
				Description: b.Reset().Str("RFC 7606 Section 5.3: MP attribute ").Int(int64(code)).
					Str(" flags inconsistent with RFC 4760 (must be optional, non-transitive)").String(),
			}
		}
		return nil
	}

	if !wellKnownAttrs[code] {
		// Optional attribute - flags not restricted
		return nil
	}

	// Well-known attribute: must NOT be optional
	if flags&attrFlagOptional != 0 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 3.c: well-known attribute ").Int(int64(code)).Str(" marked as optional").String(),
		}
	}

	// Well-known attribute: must be transitive
	if flags&attrFlagTransitive == 0 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 3.c: well-known attribute ").Int(int64(code)).Str(" not transitive").String(),
		}
	}

	return nil
}

// ValidateUpdateRFC7606 validates an UPDATE message per RFC 7606.
//
// RFC 7606 revises error handling for UPDATE messages to minimize session resets.
// This function checks ALL path attributes for malformations and returns the
// strongest error handling action per RFC 7606 Section 3.h.
//
// Parameters:
//   - pathAttrs: Raw path attributes bytes from UPDATE message
//   - hasNLRI: Whether the UPDATE has traditional IPv4 NLRI field
//   - isIBGP: Whether this is an IBGP session (affects LOCAL_PREF, ORIGINATOR_ID, CLUSTER_LIST)
//   - asn4: Whether 4-octet AS capability is negotiated (affects AS_PATH, AGGREGATOR length)
//
// Returns:
//   - ValidationResult with strongest action, error details, and attribute codes to discard
func ValidateUpdateRFC7606(pathAttrs []byte, hasNLRI, isIBGP, asn4 bool) *RFC7606ValidationResult {
	return ValidateUpdateRFC7606AddPath(pathAttrs, hasNLRI, isIBGP, asn4, nil)
}

// ValidateUpdateRFC7606AddPath is ValidateUpdateRFC7606 with per-family ADD-PATH awareness.
// addPathFor reports whether RFC 7911 ADD-PATH is negotiated for an (AFI, SAFI); it is
// consulted only for the Section 5.3 NLRI-syntax check inside MP attributes, so that a valid
// ADD-PATH UPDATE (whose NLRI carry a 4-byte path identifier) is not misread as malformed.
// A nil addPathFor means "no ADD-PATH", which is the correct default for callers that do not
// have negotiated state (the plain ValidateUpdateRFC7606 wrapper, and unit tests).
func ValidateUpdateRFC7606AddPath(
	pathAttrs []byte, hasNLRI, isIBGP, asn4 bool, addPathFor func(afi uint16, safi uint8) bool,
) *RFC7606ValidationResult {
	if len(pathAttrs) == 0 {
		// Empty path attributes with NLRI = missing mandatory attributes
		if hasNLRI {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				Description: "RFC 7606 Section 3.d: missing well-known mandatory attributes",
			}
		}
		return &RFC7606ValidationResult{Action: RFC7606ActionNone}
	}

	// Track which mandatory attributes are present
	var hasOrigin, hasASPath, hasNextHop bool
	var mpReachCount, mpUnreachCount int

	// RFC 7606 Section 3.g: Track seen attribute codes to detect duplicates
	var seenCodes [256]bool

	// RFC 7606 Section 3.h: "If multiple errors are found, use the strongest action."
	// Collect all errors to determine the strongest action.
	strongest := RFC7606ActionNone
	var strongestCode uint8
	var strongestDesc string
	var discardEntries []DiscardEntry

	// RFC 7606 Section 3.g: byte ranges of later (keep-first) occurrences of a
	// duplicated non-MP attribute, recorded so the enforcement layer can strip them.
	var duplicateRanges []AttrRange

	// RFC 8669 Section 4: presence of the Prefix-SID attribute, observed on this walk so
	// the EBGP-boundary check does not repeat it. Reported only if the loop below runs to
	// completion (see PrefixSIDPresent).
	var sawPrefixSID bool

	// RFC 7606 Section 5.4: where the MP attributes' NLRI bytes live, observed on this
	// walk so the typed-NLRI check does not repeat it (see MPReachNLRI).
	//
	// EVERY exit below carries these, including the four Section 4 structural returns that
	// abandon the walk. Dropping them there was a way to bypass the Section 5.4 MUST with
	// one octet: put MP_REACH first so this walk records it, then append an attribute whose
	// declared length overruns the section. The walk returns treat-as-withdraw with a zero
	// location, Session.typedNLRIEdit finds !loc.Present and filters nothing, and
	// mpUnreachAttrList (rfc7606_withdraw.go) rescans the attributes with its own iterator
	// and converts the untouched MP_REACH into an MP_UNREACH carrying the same unrecognized
	// NLRI, which is then dispatched to every peer that negotiated the family. The location
	// is a fact about bytes already read; a later attribute's framing error does not unmake it.
	var mpReachNLRI, mpUnreachNLRI MPNLRILocation

	// structuralError builds the verdict for an RFC 7606 Section 4 framing error, the one
	// class that ABANDONS the walk below instead of collecting and continuing.
	//
	// It exists because an abandoned walk still owes everything the completed walk owes, and
	// four separate `return &RFC7606ValidationResult{...}` literals owed it separately and
	// paid it nowhere. Two obligations were being skipped, both wire-visible:
	//
	//   - Section 5.4. The MP NLRI locations this walk had ALREADY recorded were dropped, so
	//     the typed-NLRI filter downstream saw no MP attribute and discarded nothing. See the
	//     comment on mpReachNLRI above for the full route back out to a peer.
	//   - Section 5.2. "An UPDATE message with only path attributes and no associated NLRI"
	//     whose error action is stronger than attribute-discard MUST be a session reset. The
	//     completed walk escalates that below; these returns handed back treat-as-withdraw,
	//     which synthesizes no withdrawal at all for such a body, so the peer got silence
	//     where the RFC requires a NOTIFICATION.
	//
	// One helper rather than four literals is the point: the next exit added to this loop
	// inherits both obligations instead of re-owing them.
	structuralError := func(attrCode uint8, description string) *RFC7606ValidationResult {
		if !hasNLRI && mpReachCount == 0 && len(pathAttrs) > 0 {
			var sb textbuf.Buffer
			return &RFC7606ValidationResult{
				Action:   RFC7606ActionSessionReset,
				AttrCode: attrCode,
				Description: sb.Str("RFC 7606 Section 5.2: ").Str(description).
					Str(" (escalated -- attrs with no NLRI)").String(),
			}
		}
		return &RFC7606ValidationResult{
			Action:        RFC7606ActionTreatAsWithdraw,
			AttrCode:      attrCode,
			Description:   description,
			MPReachNLRI:   mpReachNLRI,
			MPUnreachNLRI: mpUnreachNLRI,
		}
	}

	// recordError updates the strongest action and tracks discard entries.
	recordError := func(r *RFC7606ValidationResult) {
		if r.Action == RFC7606ActionAttributeDiscard {
			discardEntries = append(discardEntries, DiscardEntry{Code: r.AttrCode, Reason: r.Reason})
		}
		if r.Action > strongest {
			strongest = r.Action
			strongestCode = r.AttrCode
			strongestDesc = r.Description
		}
	}

	// Parse attributes
	pos := 0
	for pos < len(pathAttrs) {
		attrStart := pos // start of this attribute (flags byte); used for duplicate stripping
		// Need at least flags + type code
		if pos+2 > len(pathAttrs) {
			// RFC 7606 Section 4: "If the remaining number of octets ... is less than three
			// (or less than four if the Attribute Flags field has the Extended Length bit set)"
			// MUST use treat-as-withdraw — structural error, can't continue parsing.
			return structuralError(strongestCode, "RFC 7606 Section 4: insufficient data for attribute header")
		}

		flags := pathAttrs[pos]
		attrCode := pathAttrs[pos+1]
		pos += 2

		// Determine attribute length
		var attrLen int
		if flags&0x10 != 0 { // Extended length
			if pos+2 > len(pathAttrs) {
				return structuralError(attrCode, "RFC 7606 Section 4: insufficient data for extended length")
			}
			attrLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos+1 > len(pathAttrs) {
				return structuralError(attrCode, "RFC 7606 Section 4: insufficient data for length")
			}
			attrLen = int(pathAttrs[pos])
			pos++
		}

		// Check attribute data bounds
		if pos+attrLen > len(pathAttrs) {
			// RFC 7606 Section 4: "attribute length ... exceeds the amount of data"
			// Structural error — can't continue parsing remaining attributes.
			var b textbuf.Buffer
			return structuralError(attrCode, b.Reset().Str("RFC 7606 Section 4: attribute ").
				Int(int64(attrCode)).Str(" length ").Int(int64(attrLen)).Str(" exceeds remaining data").String())
		}

		attrData := pathAttrs[pos : pos+attrLen]
		pos += attrLen

		// RFC 8669 Section 4: note the Prefix-SID attribute here. This point is after the
		// bounds check and before every branch that can skip the iteration. Earlier would
		// count an attribute whose declared length overruns the section. Later would miss
		// the first occurrence, because the flags-error and duplicate paths below both
		// continue.
		if attrCode == attrCodePrefixSID {
			sawPrefixSID = true
		}

		// RFC 7606 Section 3.c: Validate attribute flags
		if flagsResult := validateAttributeFlags(attrCode, flags); flagsResult != nil {
			// RFC 7606 Section 3.h: session-reset is immediate -- no point collecting
			// more (a Section 5.3 MP-attribute flag conflict lands here).
			if flagsResult.Action == RFC7606ActionSessionReset {
				return flagsResult
			}
			recordError(flagsResult)
			continue // Collect more errors
		}

		// RFC 7606 Section 3.g: Handle duplicate attributes
		// MP_REACH/MP_UNREACH duplicates are handled below with session-reset.
		// Other duplicates: "Discard all but first occurrence." Record the byte
		// range of this later occurrence so the enforcement layer strips it
		// keep-first; skipping validation alone would leave the duplicate bytes on
		// the wire, which the attribute index later rejects as malformed.
		if seenCodes[attrCode] && attrCode != attrCodeMPReachNLRI && attrCode != attrCodeMPUnreachNLRI {
			duplicateRanges = append(duplicateRanges, AttrRange{Start: attrStart, End: pos})
			continue
		}
		seenCodes[attrCode] = true

		// Validate specific attributes per RFC 7606 Section 7
		result := validateAttribute(attrCode, attrLen, attrData, isIBGP, asn4)
		if result != nil && result.Action != RFC7606ActionNone {
			// RFC 7606 Section 3.h: session-reset is immediate — no point collecting more
			if result.Action == RFC7606ActionSessionReset {
				return result
			}
			recordError(result)
			// Don't return — continue to collect all errors
		}

		// RFC 7606 Section 5.3: NLRI syntax inside MP attributes, ADD-PATH aware. Done here
		// rather than in validateMPReachAttr/validateMPUnreachAttr because it needs the
		// per-family ADD-PATH state to skip the RFC 7911 path identifier. The MP attribute
		// has already passed its minimum-length (and, for MP_REACH, next-hop) checks above.
		if attrCode == attrCodeMPReachNLRI || attrCode == attrCodeMPUnreachNLRI {
			if r := validateMPNLRIField(attrCode, attrData, addPathFor); r != nil {
				if r.Action == RFC7606ActionSessionReset {
					return r
				}
				recordError(r)
			}
		}

		// Track mandatory attributes and MP attribute counts
		switch attrCode {
		case attrCodeOrigin:
			hasOrigin = true
		case attrCodeASPath:
			hasASPath = true
		case attrCodeNextHop:
			hasNextHop = true
		case attrCodeMPReachNLRI:
			mpReachCount++
			hasNextHop = true // MP_REACH provides next-hop
			mpReachNLRI = locateMPNLRI(attrCode, attrData)
		case attrCodeMPUnreachNLRI:
			mpUnreachCount++
			mpUnreachNLRI = locateMPNLRI(attrCode, attrData)
		}

		// RFC 7606 Section 3.g: "If the MP_REACH_NLRI attribute or the MP_UNREACH_NLRI
		// attribute appears more than once in the UPDATE message, then a NOTIFICATION
		// message MUST be sent with the Error Subcode 'Malformed Attribute List'."
		//
		// Judged HERE, the moment the second one is seen, not after the loop. Section 3.h
		// already makes session reset immediate, and every other session-reset verdict in
		// this loop returns on the spot. Deciding it after the loop made the MUST skippable
		// by anything that abandons the walk: append one attribute whose declared length
		// overruns the section and the duplicate was never reported.
		//
		// That was not only a missed NOTIFICATION. mpReachNLRI holds the LAST MP_REACH seen
		// while attribute.AttrFind returns the FIRST, so a skipped duplicate handed
		// Session.typedNLRIEdit a location describing one attribute and bytes belonging to
		// another -- the Section 5.4 recognizer for one family applied to another family's
		// NLRI. Reporting the duplicate when it is seen removes the whole class: no exit
		// added to this loop later can outrun it.
		if mpReachCount > 1 || mpUnreachCount > 1 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionSessionReset,
				Description: "RFC 7606 Section 3.g: multiple MP_REACH_NLRI or MP_UNREACH_NLRI attributes",
			}
		}
	}

	// RFC 7606 Section 3.d: Missing well-known mandatory attributes
	// For UPDATE with NLRI: ORIGIN, AS_PATH, NEXT_HOP are mandatory
	// (NEXT_HOP can be in MP_REACH_NLRI instead of explicit attribute)
	if hasNLRI && mpReachCount == 0 {
		if !hasOrigin {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeOrigin,
				Description: "RFC 7606 Section 3.d: missing ORIGIN attribute",
			})
		}
		if !hasASPath {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 3.d: missing AS_PATH attribute",
			})
		}
		if !hasNextHop {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeNextHop,
				Description: "RFC 7606 Section 3.d: missing NEXT_HOP attribute",
			})
		}
	}

	// For MP_REACH_NLRI UPDATE: ORIGIN and AS_PATH are mandatory
	if mpReachCount > 0 {
		if !hasOrigin {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeOrigin,
				Description: "RFC 7606 Section 3.d: missing ORIGIN attribute",
			})
		}
		if !hasASPath {
			recordError(&RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 3.d: missing AS_PATH attribute",
			})
		}
	}

	// RFC 7606 Section 5.2: "An UPDATE message with only path attributes and no associated
	// NLRI ... if any path attribute fails the checks ... and the error action is not
	// 'attribute discard' ... the session-reset action MUST be used."
	// No reachable NLRI means: no traditional NLRI AND no MP_REACH_NLRI.
	if !hasNLRI && mpReachCount == 0 && len(pathAttrs) > 0 && strongest > RFC7606ActionAttributeDiscard {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    strongestCode,
			Description: "RFC 7606 Section 5.2: " + strongestDesc + " (escalated — attrs with no NLRI)",
		}
	}

	if strongest == RFC7606ActionNone {
		return &RFC7606ValidationResult{
			Action:           RFC7606ActionNone,
			DuplicateRanges:  duplicateRanges,
			PrefixSIDPresent: sawPrefixSID,
			MPReachNLRI:      mpReachNLRI,
			MPUnreachNLRI:    mpUnreachNLRI,
		}
	}

	return &RFC7606ValidationResult{
		Action:           strongest,
		AttrCode:         strongestCode,
		Description:      strongestDesc,
		DiscardEntries:   discardEntries,
		DuplicateRanges:  duplicateRanges,
		PrefixSIDPresent: sawPrefixSID,
		MPReachNLRI:      mpReachNLRI,
		MPUnreachNLRI:    mpUnreachNLRI,
	}
}

// attrValidatorFn checks a single attribute and returns a validation result, or nil if valid.
type attrValidatorFn func(code uint8, length int, data []byte, isIBGP, asn4 bool) *RFC7606ValidationResult

// attrValidators maps attribute type codes to per-attribute RFC 7606 validators.
// nil entries mean no specific validation for that code.
var attrValidators [256]attrValidatorFn

func init() {
	attrValidators[attrCodeOrigin] = validateOriginAttr
	attrValidators[attrCodeASPath] = validateASPathAttr
	attrValidators[attrCodeNextHop] = validateNextHopAttr
	attrValidators[attrCodeMED] = validateMEDAttr
	attrValidators[attrCodeLocalPref] = validateLocalPrefAttr
	attrValidators[attrCodeAtomicAgg] = validateAtomicAggAttr
	attrValidators[attrCodeAggregator] = validateAggregatorAttr
	attrValidators[attrCodeCommunity] = validateCommunityAttr
	attrValidators[attrCodeOriginatorID] = validateOriginatorIDAttr
	attrValidators[attrCodeClusterList] = validateClusterListAttr
	attrValidators[attrCodeExtCommunity] = validateExtCommunityAttr
	attrValidators[attrCodeLargeCommunity] = validateLargeCommunityAttr
	attrValidators[attrCodeMPReachNLRI] = validateMPReachAttr
	attrValidators[attrCodeMPUnreachNLRI] = validateMPUnreachAttr
	attrValidators[attrCodePrefixSID] = validatePrefixSIDAttr
}

// validateAttribute checks a single attribute per RFC 7606 Section 7.
func validateAttribute(code uint8, length int, attrData []byte, isIBGP, asn4 bool) *RFC7606ValidationResult {
	if fn := attrValidators[code]; fn != nil {
		return fn(code, length, attrData, isIBGP, asn4)
	}
	return nil
}

// RFC 7606 Section 7.1: ORIGIN must be length 1, value 0-2.
func validateOriginAttr(code uint8, length int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	if length != 1 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 7.1: ORIGIN length ").Int(int64(length)).Str(" != 1").String(),
		}
	}
	if len(attrData) > 0 && attrData[0] > 2 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 7.1: ORIGIN undefined value ").Int(int64(attrData[0])).String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.2: Validate AS_PATH segment structure.
func validateASPathAttr(_ uint8, _ int, attrData []byte, _, asn4 bool) *RFC7606ValidationResult {
	return validateASPath(attrData, asn4)
}

// RFC 7606 Section 7.3: NEXT_HOP must be length 4.
func validateNextHopAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length != 4 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 7.3: NEXT_HOP length ").Int(int64(length)).Str(" != 4").String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.4: MED must be length 4.
func validateMEDAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length != 4 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 7.4: MED length ").Int(int64(length)).Str(" != 4").String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.5: LOCAL_PREF from EBGP discarded; must be length 4.
func validateLocalPrefAttr(code uint8, length int, _ []byte, isIBGP, _ bool) *RFC7606ValidationResult {
	if !isIBGP {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonEBGPInvalid,
			Description: "RFC 7606 Section 7.5: LOCAL_PREF from external neighbor must be discarded",
		}
	}
	if length != 4 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 7.5: LOCAL_PREF length ").Int(int64(length)).Str(" != 4").String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.6: ATOMIC_AGGREGATE must be length 0 (attribute-discard).
func validateAtomicAggAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length != 0 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonInvalidLength,
			Description: b.Reset().Str("RFC 7606 Section 7.6: ATOMIC_AGGREGATE length ").Int(int64(length)).Str(" != 0").String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.7: AGGREGATOR length depends on 4-octet AS capability (attribute-discard).
func validateAggregatorAttr(code uint8, length int, _ []byte, _, asn4 bool) *RFC7606ValidationResult {
	expectedLen := 6
	if asn4 {
		expectedLen = 8
	}
	if length != expectedLen {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonInvalidLength,
			Description: b.Reset().Str("RFC 7606 Section 7.7: AGGREGATOR length ").Int(int64(length)).Str(", expected ").Int(int64(expectedLen)).Str(" (asn4=").Bool(asn4).Byte(')').String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.8: Community must be non-zero multiple of 4.
func validateCommunityAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	// RFC 7606 Section 7.8: the COMMUNITIES attribute is malformed if its length is zero or
	// not a non-zero multiple of 4. Name which clause fired so the two are distinguishable.
	if length == 0 || length%4 != 0 {
		reason := " not a multiple of 4"
		if length == 0 {
			reason = " is zero"
		}
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 7.8: Community length ").Int(int64(length)).Str(reason).String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.9: ORIGINATOR_ID from EBGP discarded; must be length 4.
func validateOriginatorIDAttr(code uint8, length int, _ []byte, isIBGP, _ bool) *RFC7606ValidationResult {
	if !isIBGP {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonEBGPInvalid,
			Description: "RFC 7606 Section 7.9: ORIGINATOR_ID from external neighbor must be discarded",
		}
	}
	if length != 4 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 7.9: ORIGINATOR_ID length ").Int(int64(length)).Str(" != 4").String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.10: CLUSTER_LIST from EBGP discarded; must be non-zero multiple of 4.
func validateClusterListAttr(code uint8, length int, _ []byte, isIBGP, _ bool) *RFC7606ValidationResult {
	if !isIBGP {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonEBGPInvalid,
			Description: "RFC 7606 Section 7.10: CLUSTER_LIST from external neighbor must be discarded",
		}
	}
	if length == 0 || length%4 != 0 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 7.10: CLUSTER_LIST length ").Int(int64(length)).Str(" not multiple of 4").String(),
		}
	}
	return nil
}

// RFC 7606 Section 7.14: Extended Community must be non-zero multiple of 8.
func validateExtCommunityAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length == 0 || length%8 != 0 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 7.14: Extended Community length ").Int(int64(length)).Str(" not multiple of 8").String(),
		}
	}
	return nil
}

// RFC 8092 Section 5: Large Community must be non-zero multiple of 12.
func validateLargeCommunityAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length == 0 || length%12 != 0 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 8092 Section 5: Large Community length ").Int(int64(length)).Str(" not multiple of 12").String(),
		}
	}
	return nil
}

// RFC 7606 Section 5.3/7.11: MP_REACH_NLRI minimum length 5, next-hop validation.
func validateMPReachAttr(code uint8, length int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	if length < 5 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 5.3: MP_REACH_NLRI length ").Int(int64(length)).Str(" < 5").String(),
		}
	}
	return validateMPReachNextHop(attrData)
}

// RFC 7606 Section 5.3: MP_UNREACH_NLRI minimum length 3.
func validateMPUnreachAttr(code uint8, length int, _ []byte, _, _ bool) *RFC7606ValidationResult {
	if length < 3 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    code,
			Description: b.Reset().Str("RFC 7606 Section 5.3: MP_UNREACH_NLRI length ").Int(int64(length)).Str(" < 3").String(),
		}
	}
	return nil
}

// validateMPNLRIField runs the RFC 7606 Section 5.3 NLRI checks on the NLRI carried inside
// an MP attribute. It lives outside the attrValidators table (it is called from the main
// loop) because it needs the per-family ADD-PATH state, which validateAttribute's fixed
// signature cannot carry. The attribute has already passed its minimum-length check
// (validateMPReachAttr/validateMPUnreachAttr) and, for MP_REACH, its next-hop check, so the
// AFI/SAFI/NH_LEN bytes are present.
func validateMPNLRIField(code uint8, attrData []byte, addPathFor func(afi uint16, safi uint8) bool) *RFC7606ValidationResult {
	afi := attribute.AFI(binary.BigEndian.Uint16(attrData[0:2]))
	safi := attribute.SAFI(attrData[2])
	// MPNLRIStart owns the header arithmetic for both codes, so this walk and the
	// Section 5.4 typed-NLRI check locate the NLRI identically.
	start, ok := MPNLRIStart(code, attrData)
	if !ok {
		// Only MP_REACH reaches here: validateMPReachAttr already refused length < 5 and
		// validateMPUnreachAttr length < 3, and both refusals return session-reset before
		// this call. So the failure is always a next-hop length running past the value.
		// The length byte is read under its own bound rather than on that reasoning, so a
		// future reordering of the walk degrades the message instead of panicking.
		var b textbuf.Buffer
		b.Reset().Str("RFC 7606 Section 5.3: MP_REACH_NLRI next-hop length ")
		if len(attrData) > 3 {
			b.Int(int64(attrData[3]))
		} else {
			b.Str("field")
		}
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    code,
			Description: b.Str(" overruns the attribute").String(),
		}
	}
	nlri := attrData[start:]
	addPath := addPathFor != nil && addPathFor(uint16(afi), uint8(safi))
	return validateMPNLRISyntax(code, afi, safi, nlri, addPath)
}

// validateMPNLRISyntax runs the RFC 7606 Section 5.3 NLRI checks -- length inconsistent
// with the AFI/SAFI, and a last NLRI that overruns the attribute -- on the NLRI portion of
// an MP attribute. Both criteria route via Section 3(j) to session reset, because an NLRI
// field that cannot be parsed cannot be treated-as-withdraw.
//
// The checks apply only to the address families whose NLRI is a plain list of
// length-prefixed prefixes: IPv4/IPv6 unicast and multicast. Labeled, VPN, EVPN and other
// typed NLRI encode a label stack, route distinguisher, or type before the prefix, so a
// plain prefix walk would misread them; for those this returns nil (permissive), matching
// validateMPReachNextHop, which is permissive for AFI/SAFI whose next-hop length it does
// not know. `addPath` reflects RFC 7911 negotiation for the family so the walk skips the
// 4-byte path identifier.
//
// Permissive here does NOT mean unchecked everywhere. A typed family that registers an
// RFC 7606 Section 5.4 recognizer is walked by its own splitter at the Section 5.4 pass
// (reactor.typedNLRIEdit), which reaches the same Section 3(j) session reset on the same
// "last NLRI overruns the attribute" condition. Nothing here may assume it ran first: the
// Section 5.4 pass runs after this one.
func validateMPNLRISyntax(code uint8, afi attribute.AFI, safi attribute.SAFI, nlri []byte, addPath bool) *RFC7606ValidationResult {
	if (afi != attribute.AFIIPv4 && afi != attribute.AFIIPv6) ||
		(safi != attribute.SAFIUnicast && safi != attribute.SAFIMulticast) {
		return nil
	}
	if r := ValidateNLRISyntaxAddPath(nlri, afi == attribute.AFIIPv6, addPath); r != nil {
		r.AttrCode = code
		return r
	}
	return nil
}

// AS_PATH segment type constants (RFC 4271 Section 4.3).
const (
	asPathTypeASSet      = 1 // AS_SET: unordered set of ASes
	asPathTypeASSequence = 2 // AS_SEQUENCE: ordered set of ASes
	asPathTypeConfedSeq  = 3 // AS_CONFED_SEQUENCE (RFC 5065)
	asPathTypeConfedSet  = 4 // AS_CONFED_SET (RFC 5065)
)

// validateASPath validates AS_PATH segment structure per RFC 7606 Section 7.2.
//
// An AS_PATH is considered malformed if:
// - Unrecognized segment type is encountered
// - Segment overrun: segment length exceeds remaining data
// - Segment underrun: only 1 byte remaining after last segment
// - Zero segment length
//
// Parameters:
//   - data: Raw AS_PATH attribute value bytes
//   - asn4: True if ASNs are 4 bytes, false for 2 bytes
//
// Returns nil if valid, or RFC7606ValidationResult with treat-as-withdraw action.
func validateASPath(data []byte, asn4 bool) *RFC7606ValidationResult {
	// Empty AS_PATH is valid per RFC 7606 Section 5 (AS_PATH may have length zero)
	if len(data) == 0 {
		return nil
	}

	asSize := 2
	if asn4 {
		asSize = 4
	}

	pos := 0
	for pos < len(data) {
		// Need at least 2 bytes for segment header (type + length)
		if pos+2 > len(data) {
			// RFC 7606 Section 7.2: underrun - not enough for segment header
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 7.2: AS_PATH segment underrun (incomplete header)",
			}
		}

		segType := data[pos]
		segLen := int(data[pos+1])
		pos += 2

		// RFC 7606 Section 7.2: Validate segment type (1-4 are valid)
		if segType < asPathTypeASSet || segType > asPathTypeConfedSet {
			var b textbuf.Buffer
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: b.Reset().Str("RFC 7606 Section 7.2: unrecognized AS_PATH segment type ").Int(int64(segType)).String(),
			}
		}

		// RFC 7606 Section 7.2: Zero segment length is malformed
		if segLen == 0 {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: "RFC 7606 Section 7.2: AS_PATH segment with zero length",
			}
		}

		// RFC 7606 Section 7.2: Check for overrun
		segDataLen := segLen * asSize
		if pos+segDataLen > len(data) {
			var b textbuf.Buffer
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodeASPath,
				Description: b.Reset().Str("RFC 7606 Section 7.2: AS_PATH segment overrun (need ").Int(int64(segDataLen)).Str(" bytes, have ").Int(int64(len(data) - pos)).Str(")").String(),
			}
		}
		pos += segDataLen
	}

	// RFC 7606 Section 7.2: Check for underrun (trailing partial data)
	// This is already handled above - if we exit the loop with pos < len(data),
	// the next iteration would catch it. But if pos == len(data) exactly, we're good.

	return nil
}

// validateMPReachNextHop validates MP_REACH_NLRI next-hop length per RFC 7606 Section 7.11.
//
// The next-hop length must be consistent with the AFI/SAFI. Invalid lengths make it
// impossible to reliably locate the NLRI, so session-reset is required.
//
// Valid lengths are defined by attribute.ValidNextHopLens -- the single source of truth
// shared between encode (MPReachNLRI.WriteTo) and decode (this validator).
func validateMPReachNextHop(data []byte) *RFC7606ValidationResult {
	// Need at least AFI (2) + SAFI (1) + NH_LEN (1) = 4 bytes
	if len(data) < 4 {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    attrCodeMPReachNLRI,
			Description: "RFC 7606 Section 7.11: MP_REACH_NLRI too short to parse next-hop",
		}
	}

	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]
	nhLen := int(data[3])

	// Use the shared valid-length table. nil means unknown AFI/SAFI -- be permissive.
	validLens := attribute.ValidNextHopLens(attribute.AFI(afi), attribute.SAFI(safi))
	if validLens != nil && !slices.Contains(validLens, nhLen) {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionSessionReset,
			AttrCode:    attrCodeMPReachNLRI,
			Description: b.Reset().Str("RFC 7606 Section 7.11: invalid next-hop length ").Int(int64(nhLen)).Str(" for AFI=").Int(int64(afi)).Str(" SAFI=").Int(int64(safi)).String(),
		}
	}

	return nil
}

// RFC 9252 Section 3.4: malformed SRv6 Service TLV triggers treat-as-withdraw.
// RFC 8669 Section 6: overall malformed Prefix-SID triggers attribute-discard.
func validatePrefixSIDAttr(code uint8, length int, attrData []byte, _, _ bool) *RFC7606ValidationResult {
	off := 0
	for off+3 <= length {
		tlvLen := int(attrData[off+1])<<8 | int(attrData[off+2])
		off += 3
		if off+tlvLen > length {
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionAttributeDiscard,
				AttrCode:    code,
				Reason:      DiscardReasonInvalidLength,
				Description: "RFC 8669 Section 6: Prefix-SID TLV length exceeds attribute bounds",
			}
		}
		tlvType := attrData[off-3]
		if tlvType == 5 || tlvType == 6 {
			if err := validateSRv6ServiceTLV(attrData[off:off+tlvLen], tlvType); err != nil {
				return err
			}
		}
		off += tlvLen
	}
	// RFC 8669 Section 6: trailing bytes after last TLV are malformed.
	if off != length {
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionAttributeDiscard,
			AttrCode:    code,
			Reason:      DiscardReasonInvalidLength,
			Description: "RFC 8669 Section 6: trailing bytes in Prefix-SID attribute",
		}
	}
	return nil
}

// validateSRv6ServiceTLV checks internal structure of an SRv6 L3/L2 Service TLV.
func validateSRv6ServiceTLV(value []byte, tlvType uint8) *RFC7606ValidationResult {
	// RFC 9252 Section 3.1: Service TLV value = Reserved(1) + Sub-TLVs.
	if len(value) < 1 {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    attrCodePrefixSID,
			Description: b.Reset().Str("RFC 9252 Section 3.4: SRv6 Service TLV type ").Int(int64(tlvType)).Str(" empty value").String(),
		}
	}
	off := 1 // skip Reserved
	for off+3 <= len(value) {
		subLen := int(value[off+1])<<8 | int(value[off+2])
		off += 3
		if off+subLen > len(value) {
			var b textbuf.Buffer
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodePrefixSID,
				Description: b.Reset().Str("RFC 9252 Section 3.4: SRv6 Sub-TLV length exceeds Service TLV type ").Int(int64(tlvType)).String(),
			}
		}
		// RFC 9252 Section 3.2: SID Info Sub-TLV (type 1) minimum length 21.
		if value[off-3] == 1 && subLen < 21 {
			var b textbuf.Buffer
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionTreatAsWithdraw,
				AttrCode:    attrCodePrefixSID,
				Description: b.Reset().Str("RFC 9252 Section 3.2: SID Info Sub-TLV length ").Int(int64(subLen)).Str(" < 21").String(),
			}
		}
		// RFC 9252 Section 3.2: validate sub-sub-TLV bounds within SID Info.
		if value[off-3] == 1 && subLen > 21 {
			ssOff := 21
			for ssOff+3 <= subLen {
				ssLen := int(value[off+ssOff+1])<<8 | int(value[off+ssOff+2])
				ssOff += 3
				if ssOff+ssLen > subLen {
					var b textbuf.Buffer
					return &RFC7606ValidationResult{
						Action:      RFC7606ActionTreatAsWithdraw,
						AttrCode:    attrCodePrefixSID,
						Description: b.Reset().Str("RFC 9252 Section 3.2: sub-sub-TLV exceeds SID Info bounds in Service TLV type ").Int(int64(tlvType)).String(),
					}
				}
				ssOff += ssLen
			}
		}
		off += subLen
	}
	// RFC 9252 Section 3.4: trailing bytes after last sub-TLV are malformed.
	if off != len(value) {
		var b textbuf.Buffer
		return &RFC7606ValidationResult{
			Action:      RFC7606ActionTreatAsWithdraw,
			AttrCode:    attrCodePrefixSID,
			Description: b.Reset().Str("RFC 9252 Section 3.4: trailing bytes in SRv6 Service TLV type ").Int(int64(tlvType)).String(),
		}
	}
	return nil
}

// ValidateNLRISyntax validates NLRI field structure per RFC 7606 Section 5.3, for a field
// whose NLRI are NOT ADD-PATH encoded (no 4-byte Path Identifier).
//
// An NLRI field is considered syntactically incorrect if:
// - Any prefix length exceeds the maximum for the address family (32 for IPv4, 128 for IPv6)
// - Any prefix's byte count exceeds the remaining data in the field (overrun)
//
// Returns nil if valid, or RFC7606ValidationResult with session-reset action.
func ValidateNLRISyntax(nlri []byte, isIPv6 bool) *RFC7606ValidationResult {
	return ValidateNLRISyntaxAddPath(nlri, isIPv6, false)
}

// ValidateNLRISyntaxAddPath is ValidateNLRISyntax for a field whose NLRI may carry the RFC
// 7911 4-byte Path Identifier. When addPath is true (ADD-PATH negotiated for this family),
// each NLRI is skipped past its path identifier before the prefix length is read -- without
// this a path-id byte would be misread as a prefix length and session-reset a VALID UPDATE.
func ValidateNLRISyntaxAddPath(nlri []byte, isIPv6, addPath bool) *RFC7606ValidationResult {
	if len(nlri) == 0 {
		return nil
	}

	maxLen := 32
	if isIPv6 {
		maxLen = 128
	}

	pos := 0
	for pos < len(nlri) {
		// RFC 7911 Section 3: an ADD-PATH NLRI is prefixed with a 4-byte Path Identifier.
		// Skip it before reading the prefix length. A path id that runs off the end, or one
		// with no prefix following, means the field cannot be parsed -> session reset (3(j)).
		if addPath {
			if pos+4 > len(nlri) {
				var b textbuf.Buffer
				return &RFC7606ValidationResult{
					Action:      RFC7606ActionSessionReset,
					Description: b.Reset().Str("RFC 7606 Section 5.3/3(j): ADD-PATH identifier overruns the NLRI field").String(),
				}
			}
			pos += 4
			if pos >= len(nlri) {
				var b textbuf.Buffer
				return &RFC7606ValidationResult{
					Action:      RFC7606ActionSessionReset,
					Description: b.Reset().Str("RFC 7606 Section 5.3/3(j): ADD-PATH NLRI has a path identifier but no prefix").String(),
				}
			}
		}

		// Each NLRI starts with 1-byte prefix length
		prefixLen := int(nlri[pos]) //nolint:gosec // pos < len(nlri) guaranteed above
		pos++

		// RFC 7606 Section 5.3: the field is "syntactically incorrect" if "the length of
		// any of the included NLRI is greater than 32" (128 for IPv6).
		//
		// Section 3 (j) then mandates session reset, not treat-as-withdraw: treat-as-
		// withdraw requires that "the entire NLRI field ... need[s] to be successfully
		// parsed -- ... If this is not possible ... the 'session reset' approach (or the
		// 'AFI/SAFI disable' approach) MUST be followed." A prefix length past the family
		// maximum means the remaining NLRI boundaries cannot be trusted, so the field has
		// not been successfully parsed.
		if prefixLen > maxLen {
			var b textbuf.Buffer
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionSessionReset,
				Description: b.Reset().Str("RFC 7606 Section 5.3/3(j): prefix length ").Int(int64(prefixLen)).Str(" > ").Int(int64(maxLen)).String(),
			}
		}

		// Bytes needed for this prefix: ceiling(prefixLen / 8).
		prefixBytes := (prefixLen + 7) / 8

		// RFC 7606 Section 3(j): treat-as-withdraw needs the entire NLRI field parsed; if not
		// possible the session-reset approach MUST be followed. Overrun = unparseable.
		if pos+prefixBytes > len(nlri) {
			var b textbuf.Buffer
			return &RFC7606ValidationResult{
				Action:      RFC7606ActionSessionReset,
				Description: b.Reset().Str("RFC 7606 Section 5.3/3(j): NLRI overrun (need ").Int(int64(prefixBytes)).Str(" bytes, have ").Int(int64(len(nlri) - pos)).Str(")").String(),
			}
		}

		pos += prefixBytes
	}

	return nil
}
