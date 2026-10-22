// Design: docs/architecture/api/update-syntax.md -- the generic attribute keyword
// Related: keywords.go -- KWAttribute and the keywords attributeKeywordCodes pairs with type codes
// RFC: rfc/short/rfc4271.md -- Section 4.3, the Attribute Flags octet Ze sends

package textparse

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// RawAttribute is one path attribute given as its wire octets: the type code,
// the Attribute Flags octet and the value. The length is counted from the value
// when the attribute is written.
type RawAttribute struct {
	Code  uint8
	Flags uint8
	Value []byte
	// Typed is the value decoded by the wire decoder of its attribute, set
	// for a code one of Ze's keywords writes (attributeKeywordCodes) and nil
	// for any other code. A destination stores a typed attribute in the field
	// that keyword fills, in place of that field's default, and never as raw
	// octets beside it: an UPDATE carrying an attribute twice is malformed.
	Typed attribute.Attribute
}

// rawAttributeValueMax is the longest value an attribute carries: the Attribute
// Length field is at most two octets (RFC 4271 Section 4.3).
const rawAttributeValueMax = 0xffff

// RawAttributeForm is the spelling every refusal of a generic attribute names.
const RawAttributeForm = "attribute [ 0x<code> 0x<flags> 0x<value> ]"

// ErrRawAttributeShape refuses a generic attribute that is not three
// hexadecimal words.
var ErrRawAttributeShape = errors.New("write " + RawAttributeForm)

// rawAttributeBuilt names each type code Ze builds from another statement of
// the same route, in the update text as in the config file, so a generic copy
// would put the attribute on the wire twice.
var rawAttributeBuilt = map[attribute.AttributeCode]string{
	attribute.AttrNextHop:       "NEXT_HOP, which Ze writes from next-hop",
	attribute.AttrMPReachNLRI:   "MP_REACH_NLRI, which Ze builds from the route's family and next-hop",
	attribute.AttrMPUnreachNLRI: "MP_UNREACH_NLRI, which Ze builds from the withdrawn routes of a family",
	attribute.AttrAS4Path:       "AS4_PATH, which Ze builds from as-path for a peer without four-octet AS",
	attribute.AttrAS4Aggregator: "AS4_AGGREGATOR, which Ze builds from aggregator for a peer without four-octet AS",
}

// attributeKeywordCodes pairs each update text keyword that writes one path
// attribute with that attribute's type code. It is the one declaration of the
// pairing: the update text refuses a generic attribute of a code listed here,
// and the ExaBGP bridge reads which code each keyword writes from it. The first
// row of a code names the keyword a refusal points to, and the first row of a
// keyword the code it writes.
var attributeKeywordCodes = []struct {
	code    attribute.AttributeCode
	keyword string
}{
	{attribute.AttrOrigin, KWOrigin},
	{attribute.AttrASPath, KWASPath},
	{attribute.AttrNextHop, KWNextHop},
	{attribute.AttrMED, KWMED},
	{attribute.AttrLocalPref, KWLocalPreference},
	{attribute.AttrAtomicAggregate, KWAtomicAggregate},
	{attribute.AttrAggregator, KWAggregator},
	{attribute.AttrCommunity, KWCommunity},
	{attribute.AttrOriginatorID, KWOriginatorID},
	{attribute.AttrClusterList, KWClusterList},
	{attribute.AttrExtCommunity, KWExtendedCommunity},
	{attribute.AttrIPv6ExtCommunity, KWExtendedCommunity},
	{attribute.AttrAIGP, KWAIGP},
	{attribute.AttrLargeCommunity, KWLargeCommunity},
	{attribute.AttrPrefixSID, KWPrefixSID},
	{attribute.AttrPrefixSID, KWPrefixSIDSRv6},
}

// AttributeKeyword answers the update text keyword that writes the attribute of
// this type code, false when no keyword writes it.
func AttributeKeyword(code attribute.AttributeCode) (string, bool) {
	for _, row := range attributeKeywordCodes {
		if row.code == code {
			return row.keyword, true
		}
	}
	return "", false
}

// KeywordAttributeCode answers the type code of the attribute an update text
// keyword, or its short form, writes; false for a keyword that writes none.
func KeywordAttributeCode(keyword string) (attribute.AttributeCode, bool) {
	canonical := ResolveAlias(keyword)
	for _, row := range attributeKeywordCodes {
		if row.keyword == canonical {
			return row.code, true
		}
	}
	return 0, false
}

// ParseRawAttribute reads a generic attribute, `0x<code> 0x<flags> 0x<value>`,
// with or without the surrounding brackets. The value may be `0x` alone, an
// empty attribute. It refuses a code Ze builds itself, a flags octet Ze must
// not send (CheckRawAttributeFlags) and a value longer than an attribute length
// holds. For a code one of Ze's keywords writes it decodes the value with that
// attribute's wire decoder into Typed, and refuses a value the decoder refuses
// or does not use every octet of (decodeTypedRawAttribute). The update text
// and the config file both read the generic attribute here, so the two accept
// and refuse the same statements.
//
// RFC 4271 Section 4.3: "Each path attribute is a triple <attribute type,
// attribute length, attribute value> of variable length." and "Attribute Type is
// a two-octet field that consists of the Attribute Flags octet, followed by the
// Attribute Type Code octet." The statement gives the type code first, then the
// flags octet, then the value.
func ParseRawAttribute(text string) (RawAttribute, error) {
	inner := strings.TrimSpace(text)
	if strings.HasPrefix(inner, "[") {
		var closed bool
		inner, closed = strings.CutSuffix(inner[1:], "]")
		if !closed {
			return RawAttribute{}, fmt.Errorf("%w: %q has no closing bracket", ErrRawAttributeShape, text)
		}
	}
	words := strings.Fields(inner)
	if len(words) != 3 {
		return RawAttribute{}, fmt.Errorf("%w: %q holds %d words, it needs three", ErrRawAttributeShape, text, len(words))
	}
	code, err := rawAttributeOctet(words[0], "code")
	if err != nil {
		return RawAttribute{}, err
	}
	if built, generated := rawAttributeBuilt[attribute.AttributeCode(code)]; generated {
		return RawAttribute{}, fmt.Errorf("attribute 0x%02x is %s: write that statement instead of the generic attribute", code, built)
	}
	flags, err := rawAttributeOctet(words[1], "flags")
	if err != nil {
		return RawAttribute{}, err
	}
	if err := CheckRawAttributeFlags(attribute.AttributeCode(code), attribute.AttributeFlags(flags)); err != nil {
		return RawAttribute{}, err
	}
	value, err := rawAttributeValue(words[2])
	if err != nil {
		return RawAttribute{}, err
	}
	// RFC 4271 Section 4.3: "If the Extended Length bit of the Attribute Flags
	// octet is set to 1, the third and fourth octets of the path attribute
	// contain the length of the attribute data in octets."
	if len(value) > rawAttributeValueMax {
		return RawAttribute{}, fmt.Errorf("attribute 0x%02x value is %d octets, above the %d an attribute length field holds: split the data or shorten it",
			code, len(value), rawAttributeValueMax)
	}
	typed, err := decodeTypedRawAttribute(attribute.AttributeCode(code), value)
	if err != nil {
		return RawAttribute{}, err
	}
	return RawAttribute{Code: code, Flags: flags, Value: value, Typed: typed}, nil
}

// rawAttributeNonEmpty names each list attribute whose specification holds a
// zero-length value malformed, which the wire decoders accept as an empty list.
var rawAttributeNonEmpty = map[attribute.AttributeCode]string{
	// RFC 7606 Section 7.8: "The Community attribute SHALL be considered
	// malformed if its length is not a non-zero multiple of 4."
	attribute.AttrCommunity: "RFC 7606 Section 7.8 holds a COMMUNITIES attribute malformed unless its length is a non-zero multiple of 4",
	// RFC 7606 Section 7.10: "it SHALL be considered malformed if its length
	// is not a non-zero multiple of 4."
	attribute.AttrClusterList: "RFC 7606 Section 7.10 holds a CLUSTER_LIST malformed unless its length is a non-zero multiple of 4",
	// RFC 7606 Section 7.14: "The Extended Community attribute SHALL be
	// considered malformed if its length is not a non-zero multiple of 8."
	attribute.AttrExtCommunity: "RFC 7606 Section 7.14 holds an EXTENDED_COMMUNITIES attribute malformed unless its length is a non-zero multiple of 8",
	// RFC 7606 Section 7.15: "The IPv6 Address Specific Extended Community
	// attribute SHALL be considered malformed if its length is not a non-zero
	// multiple of 20."
	attribute.AttrIPv6ExtCommunity: "RFC 7606 Section 7.15 holds an IPV6_EXTENDED_COMMUNITIES attribute malformed unless its length is a non-zero multiple of 20",
	// RFC 8092 Section 6: "A BGP Large Communities attribute SHALL be
	// considered malformed if the length of the BGP Large Communities
	// Attribute value, expressed in octets, is not a non-zero multiple of 12."
	attribute.AttrLargeCommunity: "RFC 8092 Section 6 holds a LARGE_COMMUNITY attribute malformed unless its length is a non-zero multiple of 12",
}

// decodeTypedRawAttribute decodes the value of a code one of Ze's keywords
// writes with that attribute's own wire decoder, the one an UPDATE from a peer
// meets, and answers nil for any other code. AS_PATH and AGGREGATOR are read
// with four-octet AS numbers, the form Ze holds them in whatever a peer
// negotiates. A value the decoder refuses is malformed, and so is one with
// octets the decoder leaves unread: the typed attribute must write the value
// back octet for octet, or the operator's octets are not what Ze would send.
func decodeTypedRawAttribute(code attribute.AttributeCode, value []byte) (attribute.Attribute, error) {
	keyword, typed := AttributeKeyword(code)
	if !typed {
		return nil, nil //nolint:nilnil // nil Typed is the opaque attribute, documented on RawAttribute
	}
	attr, decodable, err := attribute.ParseKnownAttribute(code, value, true)
	if !decodable {
		return nil, fmt.Errorf("attribute 0x%02x is %s, which Ze has no decoder for in the generic form: write the %s keyword instead of the generic attribute",
			uint8(code), code, keyword)
	}
	if err != nil {
		return nil, fmt.Errorf("attribute 0x%02x (%s) value 0x%x is malformed (%w): write a valid %s value, or the %s keyword",
			uint8(code), code, value, err, code, keyword)
	}
	if rule, listed := rawAttributeNonEmpty[code]; listed && len(value) == 0 {
		return nil, fmt.Errorf("attribute 0x%02x (%s) has no value, and %s: give at least one entry, or the %s keyword",
			uint8(code), code, rule, keyword)
	}
	written := make([]byte, attr.Len()) // config and command path, once per statement
	attr.WriteTo(written, 0)
	if repeat, repeated := largeCommunityRepeat(code, value); repeated {
		// RFC 8092 Section 3: "Duplicate BGP Large Community values MUST NOT
		// be transmitted." LargeCommunities writes each value once.
		return nil, fmt.Errorf("attribute 0x%02x (%s) value 0x%x repeats the large community 0x%x, and RFC 8092 Section 3 says \"Duplicate BGP Large Community values MUST NOT be transmitted.\": write 0x%x, which carries each large community once",
			uint8(code), code, value, repeat, written)
	}
	if !bytes.Equal(written, value) {
		return nil, fmt.Errorf("attribute 0x%02x (%s) value 0x%x is malformed: it holds %d octets and %s reads it as the %d octets 0x%x: write 0x%x, or the %s keyword",
			uint8(code), code, value, len(value), code, len(written), written, written, keyword)
	}
	return attr, nil
}

// largeCommunityRepeat answers the first twelve-octet large community that a
// LARGE_COMMUNITY value holds twice, false for any other code or a value with
// no repeat. The value is at most rawAttributeValueMax octets, so the set holds
// at most 5461 entries, once per statement on the config and command path.
func largeCommunityRepeat(code attribute.AttributeCode, value []byte) ([]byte, bool) {
	if code != attribute.AttrLargeCommunity {
		return nil, false
	}
	const entryOctets = 12
	seen := make(map[[entryOctets]byte]struct{}, len(value)/entryOctets)
	for off := 0; off+entryOctets <= len(value); off += entryOctets {
		entry := [entryOctets]byte(value[off : off+entryOctets])
		if _, held := seen[entry]; held {
			return value[off : off+entryOctets], true
		}
		seen[entry] = struct{}{}
	}
	return nil, false
}

// CheckRawAttributeFlags refuses an Attribute Flags octet Ze must not send for
// this type code. Ze is the speaker that sends what the operator wrote, so the
// sender's half of RFC 4271 Section 4.3 binds it, and a receiver would treat a
// conflicting Optional or Transitive bit as malformed (RFC 7606 Section 3(c)).
// The Extended Length bit is free: either length form carries the value. The
// Partial bit is refused on a code one of Ze's keywords writes, which Ze sends
// from its decoded value and so with that attribute's own flags.
func CheckRawAttributeFlags(code attribute.AttributeCode, flags attribute.AttributeFlags) error {
	// RFC 4271 Section 4.3: "The lower-order four bits of the Attribute Flags
	// octet are unused. They MUST be zero when sent and MUST be ignored when
	// received."
	if unused := flags & 0x0f; unused != 0 {
		return fmt.Errorf("attribute 0x%02x flags 0x%02x set unused bits 0x%02x, which RFC 4271 Section 4.3 requires to be zero when sent: write 0x%02x",
			uint8(code), uint8(flags), uint8(unused), uint8(flags&^0x0f))
	}
	// RFC 7606 Section 3(c): "If the value of either the Optional or Transitive
	// bits in the Attribute Flags is in conflict with their specified values,
	// then the attribute MUST be treated as malformed".
	if verdict, _ := code.FlagsConflict(flags); verdict != attribute.FlagsConflictNone {
		return fmt.Errorf("attribute 0x%02x (%s) flags 0x%02x disagree with the Optional and Transitive bits its specification fixes: write 0x%02x",
			uint8(code), code, uint8(flags), uint8(specifiedFlags(code)|flags&attribute.FlagExtLength))
	}
	// RFC 4271 Section 4.3: "It defines whether the attribute is optional (if
	// set to 1) or well-known (if set to 0)." RFC 4271 Section 5: "BGP
	// implementations MUST recognize all well-known attributes." A receiver
	// cannot recognize a well-known attribute of a code Ze knows none for, so
	// that code can only be sent as an optional one. A well-known code with the
	// Transitive bit clear never reaches the Partial check below: FlagsConflict
	// above refuses it (RFC 4271 Section 4.3: "For well-known attributes, the
	// Transitive bit MUST be set to 1.").
	if !flags.IsOptional() && !code.WellKnown() {
		return fmt.Errorf("attribute 0x%02x flags 0x%02x clear the Optional bit, which marks a well-known attribute, and 0x%02x is not one: write 0x%02x for an optional transitive attribute or 0x%02x for an optional non-transitive one",
			uint8(code), uint8(flags), uint8(code),
			uint8(flags|attribute.FlagOptional|attribute.FlagTransitive),
			uint8(flags&^(attribute.FlagTransitive|attribute.FlagPartial)|attribute.FlagOptional))
	}
	// RFC 4271 Section 4.3: "For well-known attributes and for optional
	// non-transitive attributes, the Partial bit MUST be set to 0."
	if flags&attribute.FlagPartial == 0 {
		return nil
	}
	if !flags.IsOptional() {
		return partialRefused(code, flags, "a well-known")
	}
	if !flags.IsTransitive() {
		return partialRefused(code, flags, "an optional non-transitive")
	}
	// A code one of Ze's keywords writes is sent from its decoded form
	// (decodeTypedRawAttribute), whose flags carry no Partial bit, so the bit
	// would not reach the wire. Partial records how the path travelled, not
	// the value. RFC 4271 Section 5: "If they are not attached by the
	// originator, the Partial bit in the Attribute Flags octet is set to 1."
	// and RFC 4271 Section 9: "If an optional transitive attribute is
	// unrecognized, the Partial bit (the third high-order bit) in the
	// attribute flags octet is set to 1". Ze originates this route and
	// recognises this attribute, so neither applies.
	if keyword, typed := AttributeKeyword(code); typed {
		return fmt.Errorf("attribute 0x%02x (%s) flags 0x%02x set the Partial bit, which Ze does not send for an attribute it decodes: Ze sends this code from its decoded value, as it sends the %s keyword, and RFC 4271 Section 5 sets Partial only on an optional transitive attribute an AS passed along unrecognised or attached to a path it did not originate: write 0x%02x",
			uint8(code), code, uint8(flags), keyword, uint8(flags&^attribute.FlagPartial))
	}
	return nil
}

// partialRefused names the Partial bit set on an attribute of a kind that MUST
// send it clear, and the flags octet to write instead.
func partialRefused(code attribute.AttributeCode, flags attribute.AttributeFlags, kind string) error {
	return fmt.Errorf("attribute 0x%02x flags 0x%02x set the Partial bit, which RFC 4271 Section 4.3 requires to be zero for %s attribute: write 0x%02x",
		uint8(code), uint8(flags), kind, uint8(flags&^attribute.FlagPartial))
}

// specifiedFlags answers the Optional and Transitive bits the specification of
// a code fixes, for the refusal to name the octet to write. It is asked only of
// a code FlagsConflict judged, which has a specification.
func specifiedFlags(code attribute.AttributeCode) attribute.AttributeFlags {
	if code.WellKnown() {
		return attribute.FlagTransitive
	}
	if verdict, _ := code.FlagsConflict(attribute.FlagOptional); verdict != attribute.FlagsConflictNone {
		return attribute.FlagOptional | attribute.FlagTransitive
	}
	return attribute.FlagOptional
}

// rawAttributeOctet reads one `0x`-prefixed hexadecimal octet, the attribute
// code or its flags.
func rawAttributeOctet(word, what string) (uint8, error) {
	digits, prefixed := strings.CutPrefix(strings.ToLower(word), "0x")
	if !prefixed {
		return 0, fmt.Errorf("attribute %s %q is not hexadecimal: write it as 0x<two digits>, as in %s", what, word, RawAttributeForm)
	}
	octet, err := strconv.ParseUint(digits, 16, 8)
	if err != nil {
		return 0, fmt.Errorf("attribute %s %q is not one octet: write 0x00 to 0xff, as in %s", what, word, RawAttributeForm)
	}
	return uint8(octet), nil
}

// rawAttributeValue reads the `0x`-prefixed hexadecimal attribute value.
func rawAttributeValue(word string) ([]byte, error) {
	digits, prefixed := strings.CutPrefix(strings.ToLower(word), "0x")
	if !prefixed {
		return nil, fmt.Errorf("attribute value %q is not hexadecimal: write it as 0x<digits>, as in %s", word, RawAttributeForm)
	}
	if len(digits)%2 != 0 {
		return nil, fmt.Errorf("attribute value %q has an odd number of hexadecimal digits: write two per octet", word)
	}
	value, err := hex.DecodeString(digits)
	if err != nil {
		return nil, fmt.Errorf("attribute value %q is not hexadecimal: write it as 0x<digits>, two per octet", word)
	}
	return value, nil
}
