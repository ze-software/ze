// Design: docs/architecture/wire/attributes.md — path attribute encoding
// RFC: rfc/short/rfc7606.md — Section 3.c, the Optional and Transitive flags conflict rule
// Related: attribute.go — AttributeCode and AttributeFlags, the two halves of this question
// Related: partial.go — the Partial walks, which read the same flags octet

package attribute

// FlagsConflict is the handling an attribute's own specification mandates when the
// Optional or Transitive bit of a received Attribute Flags octet disagrees with the
// value that specification fixes.
//
// RFC 7606 Section 3(c): "If the value of either the Optional or Transitive bits in the
// Attribute Flags is in conflict with their specified values, then the attribute MUST be
// treated as malformed and the 'treat-as-withdraw' approach used, unless the
// specification for the attribute mandates different handling for incorrect Attribute
// Flags." That binds every attribute whose specification fixes a value, not a
// chosen few.
//
// The clause at the end of that sentence is why this is a value rather than one fixed
// verdict. Two of the attributes ze implements carry their own answer: RFC 7311
// Section 3.2 makes a transitive AIGP an attribute discard, and RFC 7606 Section 5.3 with
// Section 3(j) makes MP flags that contradict RFC 4760 a session reset.
//
// The values ascend by strength, in the order RFC 7606 Section 2 lists the approaches, so
// a numeric comparison gives the stronger of two verdicts. Section 3(h): "When multiple
// attribute errors exist in an UPDATE message, if the same approach (as described in
// Section 2) is specified for the handling of these malformed attributes, then the
// specified approach MUST be used. Otherwise, the approach with the strongest action MUST
// be used." The two bits of one attribute are two such errors.
type FlagsConflict uint8

const (
	// FlagsConflictUnspecified is the zero value, and no declaration in flagsSpecs
	// produces it. A bit that conflicts with no handling declared for it is an authoring
	// hole, so a caller that meets this value fails closed rather than accepting the
	// attribute. TestFlagsSpecDeclaresHandlingForEveryRule holds the hole shut.
	FlagsConflictUnspecified FlagsConflict = iota
	// FlagsConflictNone means the received bits carry the values the attribute's
	// specification fixes, or that specification fixes no value for them.
	FlagsConflictNone
	// FlagsConflictAttributeDiscard discards the attribute and continues to process the
	// UPDATE (RFC 7606 Section 2).
	FlagsConflictAttributeDiscard
	// FlagsConflictTreatAsWithdraw treats the UPDATE as though its routes were withdrawn
	// (RFC 7606 Section 2). It is the Section 3(c) answer for every attribute whose own
	// specification mandates nothing else.
	FlagsConflictTreatAsWithdraw
	// FlagsConflictSessionReset sends a NOTIFICATION and terminates the session
	// (RFC 7606 Section 2).
	FlagsConflictSessionReset
)

// flagRule is the value an attribute's specification fixes for one flag bit.
type flagRule uint8

const (
	// flagRuleUnspecified: the attribute's specification fixes no value for this bit, so
	// no received value can conflict with it.
	flagRuleUnspecified flagRule = iota
	// flagRuleSet: the specification fixes the bit at 1.
	flagRuleSet
	// flagRuleClear: the specification fixes the bit at 0.
	flagRuleClear
)

// violatedBy reports whether a received bit disagrees with the value this rule fixes.
func (r flagRule) violatedBy(set bool) bool {
	if r == flagRuleSet {
		return !set
	}
	if r == flagRuleClear {
		return set
	}
	return false
}

// flagRequirement is one flag bit's declaration: the value the attribute's specification
// fixes, the handling that specification mandates for a received bit that disagrees, and
// the section that mandates it.
//
// The mandate is prose for an operator, and it names the section a reader opens to check
// the verdict. It stays beside the rule it explains, so the two cannot come apart.
type flagRequirement struct {
	rule     flagRule
	conflict FlagsConflict
	mandate  string
}

// FlagsSpec is one attribute's Optional and Transitive declaration.
//
// Its fields are unexported and the functions below are the only constructors, so a
// registration outside this package declares one of the three shapes RFC 4271
// Section 4.3 names and cannot invent a fourth. The zero value declares nothing, and
// RegisterName refuses it.
type FlagsSpec struct {
	optional   flagRequirement
	transitive flagRequirement
}

// declared reports whether this specification fixes a value for either bit.
func (s FlagsSpec) declared() bool {
	return s.optional.rule != flagRuleUnspecified || s.transitive.rule != flagRuleUnspecified
}

// The three shapes RFC 4271 Section 4.3 names, and the mandate that covers each: "The
// first high-order bit (bit 0) of the Attribute Flags octet is the Optional bit. It
// defines whether the attribute is optional (if set to 1) or well-known (if set to 0).
// The second high-order bit (bit 1) of the Attribute Flags octet is the Transitive bit.
// It defines whether an optional attribute is transitive (if set to 1) or non-transitive
// (if set to 0). For well-known attributes, the Transitive bit MUST be set to 1." A
// conflict with any of the three carries the RFC 7606 Section 3.c handling.
//
// They are functions rather than variables because a package-level variable of an
// exported type is writable by every importer, and a shape another package could
// reassign is a specification ze would then judge received flags against.

// WellKnownFlags is a well-known attribute: Optional 0, Transitive 1.
func WellKnownFlags() FlagsSpec {
	return FlagsSpec{
		optional:   flagRequirement{rule: flagRuleClear, conflict: FlagsConflictTreatAsWithdraw, mandate: rfc7606FlagsMandate},
		transitive: flagRequirement{rule: flagRuleSet, conflict: FlagsConflictTreatAsWithdraw, mandate: rfc7606FlagsMandate},
	}
}

// OptionalTransitiveFlags is an optional transitive attribute: Optional 1, Transitive 1.
func OptionalTransitiveFlags() FlagsSpec {
	return FlagsSpec{
		optional:   flagRequirement{rule: flagRuleSet, conflict: FlagsConflictTreatAsWithdraw, mandate: rfc7606FlagsMandate},
		transitive: flagRequirement{rule: flagRuleSet, conflict: FlagsConflictTreatAsWithdraw, mandate: rfc7606FlagsMandate},
	}
}

// OptionalNonTransitiveFlags is an optional non-transitive attribute: Optional 1,
// Transitive 0.
func OptionalNonTransitiveFlags() FlagsSpec {
	return FlagsSpec{
		optional:   flagRequirement{rule: flagRuleSet, conflict: FlagsConflictTreatAsWithdraw, mandate: rfc7606FlagsMandate},
		transitive: flagRequirement{rule: flagRuleClear, conflict: FlagsConflictTreatAsWithdraw, mandate: rfc7606FlagsMandate},
	}
}

// The attributes whose own specification mandates handling other than the Section 3.c
// default. Each one is core, so each is declared here rather than through a constructor:
// an attribute that needs its own mandate needs its own RFC sentence with it.
var (
	// mpNLRIFlags covers MP_REACH_NLRI and MP_UNREACH_NLRI, whose conflict is a session
	// reset rather than a withdrawal.
	mpNLRIFlags = FlagsSpec{
		optional:   flagRequirement{rule: flagRuleSet, conflict: FlagsConflictSessionReset, mandate: rfc4760FlagsMandate},
		transitive: flagRequirement{rule: flagRuleClear, conflict: FlagsConflictSessionReset, mandate: rfc4760FlagsMandate},
	}
	// aigpFlags covers AIGP. RFC 7311 Section 3: "The AIGP attribute is an optional,
	// non-transitive BGP path attribute." Only the transitive bit gets the Section 3.2
	// discard: that section says nothing about an AIGP arriving with the Optional bit
	// clear, so RFC 7606 Section 3(c) governs that one and the answer there is
	// treat-as-withdraw.
	aigpFlags = FlagsSpec{
		optional:   flagRequirement{rule: flagRuleSet, conflict: FlagsConflictTreatAsWithdraw, mandate: rfc7606FlagsMandate},
		transitive: flagRequirement{rule: flagRuleClear, conflict: FlagsConflictAttributeDiscard, mandate: rfc7311FlagsMandate},
	}
	// tombstoneFlags covers ATTR_TOMBSTONE. The Transitive bit is derived from the
	// attributes the marker replaces, so the draft fixes no value for it and no received
	// value can conflict.
	tombstoneFlags = FlagsSpec{
		optional:   flagRequirement{rule: flagRuleSet, conflict: FlagsConflictTreatAsWithdraw, mandate: tombstoneFlagsMandate},
		transitive: flagRequirement{rule: flagRuleUnspecified},
	}
)

// The mandates, one string for each specification that decides a handling.
const (
	// rfc7606FlagsMandate is the Section 3(c) default: treat-as-withdraw for a conflict
	// the attribute's own specification says nothing else about.
	rfc7606FlagsMandate = "RFC 7606 Section 3.c"
	// rfc4760FlagsMandate covers MP_REACH_NLRI and MP_UNREACH_NLRI. RFC 7606 Section 5.3
	// makes an MP attribute whose flags contradict RFC 4760 "incorrect", and Section 3(j)
	// escalates that to a session reset: an attribute whose framing is in doubt cannot
	// have its NLRI boundaries trusted, which is what treat-as-withdraw would require.
	rfc4760FlagsMandate = "RFC 7606 Section 5.3 with RFC 4760"
	// rfc7311FlagsMandate covers the AIGP transitive bit. RFC 7311 Section 3.2: "If a BGP
	// path attribute is received that has the AIGP attribute codepoint but also has the
	// transitive bit set, the attribute MUST be considered to be a malformed AIGP
	// attribute and MUST be discarded as specified in this section." That section is
	// attribute discard: the UPDATE keeps its routes and loses only the AIGP.
	rfc7311FlagsMandate = "RFC 7311 Section 3.2"
	// tombstoneFlagsMandate covers ATTR_TOMBSTONE, whose Optional bit is fixed and whose
	// Transitive bit is not. draft-mangin-idr-attr-tombstone-00 Section 5.1:
	// "ATTR_TOMBSTONE is always Optional, regardless of whether the original attribute was
	// well-known or optional. The Optional bit MUST be set." The same section derives the
	// Transitive bit from the attributes that were discarded, so no value is fixed for it.
	tombstoneFlagsMandate = "draft-mangin-idr-attr-tombstone-00 Section 5.1"
)

// flagsSpecs is the one declaration of the Optional and Transitive values each attribute's
// specification fixes. Every other surface reads it: the RFC 7606 Section 3(c) check on the
// receive path asks AttributeCode.FlagsConflict, which is this table.
//
// An array rather than a map because the receive path asks the question once for each
// attribute of each UPDATE (ai/rules/performance.md): the answer is one index, with no hash
// and no pointer chase. recognizedCodes carries the same idiom for the same reason.
//
// RegisterName is the only writer, and it takes the specification beside the name, so an
// attribute cannot become RECOGNIZED without declaring what its flags must be. The two
// arrive in one call from the package that owns the attribute, core or plugin, and no
// central list stands here for the next attribute to be missing from.
//
// A code with no entry has every rule unspecified, and that now says one thing rather than
// two: ze holds no meaning for the code. RFC 4271 Section 5 requires an unrecognized
// optional transitive attribute to be passed on unchanged, so ze must not judge flags it
// has no specified values for. RegisterNameOnly leaves a code in that state deliberately,
// for an attribute ze can name and nothing more.
var flagsSpecs [256]FlagsSpec

// coreAttributes is every attribute the core implements: its code, its display name, and
// the Optional and Transitive values its own specification fixes. init registers each one
// through RegisterName, the same door a plugin's attribute comes through.
var coreAttributes = []struct {
	code  AttributeCode
	name  string
	flags FlagsSpec
}{
	{AttrOrigin, "ORIGIN", WellKnownFlags()},                                       // RFC 4271 Section 4.3a: "ORIGIN is a well-known mandatory attribute".
	{AttrASPath, "AS_PATH", WellKnownFlags()},                                      // RFC 4271 Section 4.3b: "AS_PATH is a well-known mandatory attribute".
	{AttrNextHop, "NEXT_HOP", WellKnownFlags()},                                    // RFC 4271 Section 4.3c: "This is a well-known mandatory attribute".
	{AttrMED, "MULTI_EXIT_DISC", OptionalNonTransitiveFlags()},                     // RFC 4271 Section 4.3d: "This is an optional non-transitive attribute".
	{AttrLocalPref, "LOCAL_PREF", WellKnownFlags()},                                // RFC 4271 Section 4.3e: "LOCAL_PREF is a well-known attribute".
	{AttrAtomicAggregate, "ATOMIC_AGGREGATE", WellKnownFlags()},                    // RFC 4271 Section 4.3f: "ATOMIC_AGGREGATE is a well-known discretionary attribute".
	{AttrAggregator, "AGGREGATOR", OptionalTransitiveFlags()},                      // RFC 4271 Section 4.3g: "AGGREGATOR is an optional transitive attribute".
	{AttrCommunity, "COMMUNITIES", OptionalTransitiveFlags()},                      // RFC 1997: "the COMMUNITIES path attribute is an optional transitive attribute".
	{AttrOriginatorID, "ORIGINATOR_ID", OptionalNonTransitiveFlags()},              // RFC 4456 Section 8: "ORIGINATOR_ID is a new optional, non-transitive BGP attribute".
	{AttrClusterList, "CLUSTER_LIST", OptionalNonTransitiveFlags()},                // RFC 4456 Section 8: "CLUSTER_LIST is a new, optional, non-transitive BGP attribute".
	{AttrMPReachNLRI, "MP_REACH_NLRI", mpNLRIFlags},                                // RFC 4760 Section 3: "This is an optional non-transitive attribute".
	{AttrMPUnreachNLRI, "MP_UNREACH_NLRI", mpNLRIFlags},                            // RFC 4760 Section 4: "This is an optional non-transitive attribute".
	{AttrExtCommunity, "EXTENDED_COMMUNITIES", OptionalTransitiveFlags()},          // RFC 4360 Section 2: "The Extended Communities Attribute is a transitive optional BGP attribute".
	{AttrAS4Path, "AS4_PATH", OptionalTransitiveFlags()},                           // RFC 6793 Section 3: "This is an optional transitive attribute".
	{AttrAS4Aggregator, "AS4_AGGREGATOR", OptionalTransitiveFlags()},               // RFC 6793 Section 3: "AS4_AGGREGATOR, which is optional transitive".
	{AttrTunnelEncap, "TUNNEL_ENCAPSULATION", OptionalTransitiveFlags()},           // RFC 9012 Section 2: "The Tunnel Encapsulation attribute is an optional transitive BGP path attribute".
	{AttrIPv6ExtCommunity, "IPV6_EXTENDED_COMMUNITIES", OptionalTransitiveFlags()}, // RFC 5701 Section 2: "transitive, optional BGP attribute".
	{AttrAIGP, "AIGP", aigpFlags},                                                  // RFC 7311 Section 3: "The AIGP attribute is an optional, non-transitive BGP path attribute".
	{AttrLargeCommunity, "LARGE_COMMUNITIES", OptionalTransitiveFlags()},           // RFC 8092 Section 2: "optional transitive path attribute".
	{AttrPrefixSID, "PREFIX_SID", OptionalTransitiveFlags()},                       // RFC 8669 Section 3: "The BGP Prefix-SID attribute is an optional, transitive BGP path attribute".
	{AttrTombstone, "ATTR_TOMBSTONE", tombstoneFlags},                              // draft-mangin-idr-attr-tombstone-00 Section 5.1: "ATTR_TOMBSTONE is always Optional".
}

// FlagsConflict reports the handling a received Attribute Flags octet needs for this
// attribute code, and names the section that mandates it.
//
// The verdict is FlagsConflictNone when both bits carry the values this attribute's
// specification fixes, and when that specification fixes no value for them. The mandate is
// empty for that verdict, and names a section for every other one.
//
// When both bits conflict and their specifications mandate different handling, the
// stronger verdict wins, which is what RFC 7606 Section 3(h) requires of an UPDATE
// carrying several attribute errors.
func (c AttributeCode) FlagsConflict(flags AttributeFlags) (FlagsConflict, string) {
	spec := flagsSpecs[c]

	verdict, mandate := FlagsConflictNone, ""
	if spec.optional.rule.violatedBy(flags.IsOptional()) {
		verdict, mandate = spec.optional.conflict, spec.optional.mandate
	}
	if spec.transitive.rule.violatedBy(flags.IsTransitive()) {
		if spec.transitive.conflict > verdict {
			verdict, mandate = spec.transitive.conflict, spec.transitive.mandate
		}
	}
	return verdict, mandate
}
