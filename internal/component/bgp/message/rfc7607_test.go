// Design: docs/architecture/wire/messages.md — AS 0 is proscribed on the wire
// RFC: rfc/short/rfc7607.md — Codification of AS 0 Processing
// Related: rfc7607.go — the AS4_PATH and AS4_AGGREGATOR validators these drive
// Related: rfc7606.go — validateASPath and validateAggregatorAttr, extended for AS 0

package message

import "testing"

// asPathAttr builds a path-attribute TLV for AS_PATH (code 2) holding one
// AS_SEQUENCE segment over the supplied ASNs, encoded two-octet or four-octet.
func asPathAttr(asns []uint32, asn4 bool) []byte {
	value := []byte{asPathTypeASSequence, byte(len(asns))}
	for _, asn := range asns {
		if asn4 {
			value = append(value, byte(asn>>24), byte(asn>>16), byte(asn>>8), byte(asn))
			continue
		}
		value = append(value, byte(asn>>8), byte(asn))
	}
	return append([]byte{0x40, attrCodeASPath, byte(len(value))}, value...)
}

// TestRFC7607ASPathZeroTreatAsWithdraw drives validateASPath, the producer the RFC
// 7606 attribute walk calls for code 2, over an AS_SEQUENCE that carries AS 0. RFC
// 7607 Section 2 makes that UPDATE malformed and RFC 7606 Section 7.2 makes a
// malformed AS_PATH a treat-as-withdraw.
//
// RFC requirement: RFC7607-2-2 positive -- AS 0 in AS_PATH is malformed and is
// handled by the RFC 7606 procedure for that attribute, which is treat-as-withdraw.
func TestRFC7607ASPathZeroTreatAsWithdraw(t *testing.T) {
	for _, asn4 := range []bool{false, true} {
		result := validateASPath(asPathAttr([]uint32{65001, 0, 65002}, asn4)[3:], asn4)
		if result == nil {
			t.Fatalf("asn4=%v: AS 0 in AS_PATH was accepted", asn4)
		}
		if result.Action != RFC7606ActionTreatAsWithdraw {
			t.Errorf("asn4=%v: action is %v, want treat-as-withdraw", asn4, result.Action)
		}
		if result.AttrCode != attrCodeASPath {
			t.Errorf("asn4=%v: attribute code is %d, want %d", asn4, result.AttrCode, attrCodeASPath)
		}
	}
}

// TestRFC7607ASPathNonZeroAccepted holds the discrimination: without it the positive
// case above would also pass against a validator that rejected every AS_PATH.
//
// RFC requirement: RFC7607-2-2 negative -- a path with no AS 0 stays valid, so the
// check is bound to the zero and not to the presence of an AS_PATH.
func TestRFC7607ASPathNonZeroAccepted(t *testing.T) {
	for _, asn4 := range []bool{false, true} {
		if result := validateASPath(asPathAttr([]uint32{65001, 65002}, asn4)[3:], asn4); result != nil {
			t.Errorf("asn4=%v: a path with no AS 0 was rejected: %s", asn4, result.Description)
		}
	}
}

// TestRFC7607AggregatorZeroDiscard drives the AGGREGATOR validator over both wire
// widths, because the AS field is two octets without the four-octet AS capability
// and four octets with it.
//
// RFC requirement: RFC7607-2-2 positive -- AS 0 in AGGREGATOR is malformed, and RFC
// 7606 Section 7.7 makes a malformed AGGREGATOR an attribute discard.
func TestRFC7607AggregatorZeroDiscard(t *testing.T) {
	cases := []struct {
		name string
		asn4 bool
		data []byte
	}{
		{"two-octet", false, []byte{0x00, 0x00, 192, 0, 2, 1}},
		{"four-octet", true, []byte{0x00, 0x00, 0x00, 0x00, 192, 0, 2, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := validateAggregatorAttr(attrCodeAggregator, len(tc.data), tc.data, false, tc.asn4)
			if result == nil {
				t.Fatal("AS 0 in AGGREGATOR was accepted")
			}
			if result.Action != RFC7606ActionAttributeDiscard {
				t.Errorf("action is %v, want attribute-discard", result.Action)
			}
			if result.Reason != DiscardReasonMalformedValue {
				t.Errorf("reason is %d, want DiscardReasonMalformedValue", result.Reason)
			}
		})
	}
}

// RFC requirement: RFC7607-2-2 negative -- an AGGREGATOR naming a real AS is accepted,
// so the discard is bound to the zero AS and not to the attribute's presence.
func TestRFC7607AggregatorNonZeroAccepted(t *testing.T) {
	cases := []struct {
		name string
		asn4 bool
		data []byte
	}{
		{"two-octet", false, []byte{0xFD, 0xE9, 192, 0, 2, 1}},
		{"four-octet", true, []byte{0x00, 0x01, 0x00, 0x01, 192, 0, 2, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := validateAggregatorAttr(attrCodeAggregator, len(tc.data), tc.data, false, tc.asn4)
			if result != nil {
				t.Errorf("a non-zero AGGREGATOR was rejected: %s", result.Description)
			}
		})
	}
}

// RFC requirement: RFC7607-2-3 positive -- AS 0 in AS4_PATH is malformed, and the RFC
// 6793 Section 6 procedure for a malformed AS4_PATH is attribute discard.
func TestRFC7607AS4PathZeroDiscard(t *testing.T) {
	// One AS_SEQUENCE of two four-octet ASNs, the second of which is zero.
	data := []byte{asPathTypeASSequence, 2, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x00}
	result := validateAS4PathAttr(attrCodeAS4Path, len(data), data, false, true)
	if result == nil {
		t.Fatal("AS 0 in AS4_PATH was accepted")
	}
	if result.Action != RFC7606ActionAttributeDiscard {
		t.Errorf("action is %v, want attribute-discard", result.Action)
	}
	if result.AttrCode != attrCodeAS4Path {
		t.Errorf("attribute code is %d, want %d", result.AttrCode, attrCodeAS4Path)
	}
}

// RFC requirement: RFC7607-2-3 negative -- an AS4_PATH of real ASNs survives, so the
// discard is bound to the zero and not to the attribute being present at all.
func TestRFC7607AS4PathNonZeroAccepted(t *testing.T) {
	data := []byte{asPathTypeASSequence, 2, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x02}
	result := validateAS4PathAttr(attrCodeAS4Path, len(data), data, false, true)
	if result != nil {
		t.Errorf("a non-zero AS4_PATH was rejected: %s", result.Description)
	}
}

// RFC requirement: RFC7607-2-3 positive -- AS 0 in AS4_AGGREGATOR is malformed, and the
// RFC 6793 Section 6 procedure for a malformed AS4_AGGREGATOR is attribute discard.
func TestRFC7607AS4AggregatorZeroDiscard(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 192, 0, 2, 1}
	result := validateAS4AggregatorAttr(attrCodeAS4Aggregator, len(data), data, false, true)
	if result == nil {
		t.Fatal("AS 0 in AS4_AGGREGATOR was accepted")
	}
	if result.Action != RFC7606ActionAttributeDiscard {
		t.Errorf("action is %v, want attribute-discard", result.Action)
	}
	if result.Reason != DiscardReasonMalformedValue {
		t.Errorf("reason is %d, want DiscardReasonMalformedValue", result.Reason)
	}
}

// RFC requirement: RFC7607-2-3 negative -- an AS4_AGGREGATOR naming a real four-octet
// AS is accepted, so the discard is bound to the zero AS.
func TestRFC7607AS4AggregatorNonZeroAccepted(t *testing.T) {
	data := []byte{0x00, 0x01, 0x00, 0x01, 192, 0, 2, 1}
	result := validateAS4AggregatorAttr(attrCodeAS4Aggregator, len(data), data, false, true)
	if result != nil {
		t.Errorf("a non-zero AS4_AGGREGATOR was rejected: %s", result.Description)
	}
}

// mandatoryAttrs returns ORIGIN and NEXT_HOP, the two well-known mandatory attributes
// an UPDATE with NLRI owes besides AS_PATH (RFC 7606 Section 3.d). A walk test that
// omits them collects a missing-attribute finding of the same strength as the AS 0
// finding, and would then reach its verdict without the AS 0 check existing at all.
func mandatoryAttrs() []byte {
	origin := []byte{0x40, attrCodeOrigin, 1, 0}
	nextHop := []byte{0x40, attrCodeNextHop, 4, 192, 0, 2, 254}
	return append(origin, nextHop...)
}

// TestRFC7607CompleteUpdateASPathZero carries the discrimination for the whole-UPDATE
// walk: the UPDATE is complete, so the AS 0 in AS_PATH is the only finding available
// and TestRFC7607CompleteUpdateAccepted below shows the same bytes without the zero
// are accepted outright.
//
// RFC requirement: RFC7607-2-2 positive -- an UPDATE whose AS_PATH carries AS 0 is
// treat-as-withdrawn by the whole-UPDATE attribute walk.
func TestRFC7607CompleteUpdateASPathZero(t *testing.T) {
	attrs := append(mandatoryAttrs(), asPathAttr([]uint32{65001, 0}, false)...)
	result := ValidateUpdateRFC7606(attrs, true, false, false)
	if result == nil {
		t.Fatal("the attribute walk accepted an UPDATE whose AS_PATH holds AS 0")
	}
	if result.Action != RFC7606ActionTreatAsWithdraw {
		t.Fatalf("action is %v (%s), want treat-as-withdraw", result.Action, result.Description)
	}
}

// RFC requirement: RFC7607-2-2 negative -- the same complete UPDATE with a real AS in
// place of the zero is accepted whole, so the walk's verdict is bound to the AS 0.
func TestRFC7607CompleteUpdateAccepted(t *testing.T) {
	attrs := append(mandatoryAttrs(), asPathAttr([]uint32{65001, 65002}, false)...)
	result := ValidateUpdateRFC7606(attrs, true, false, false)
	if result == nil {
		t.Fatal("the attribute walk returned no result")
	}
	if result.Action != RFC7606ActionNone {
		t.Errorf("action is %v (%s), want none", result.Action, result.Description)
	}
}

// TestRFC7607CompleteUpdateAS4PathZero carries the discrimination for AS4_PATH on the
// whole-UPDATE walk, against TestRFC7607CompleteUpdateAS4PathAccepted below.
//
// RFC requirement: RFC7607-2-3 positive -- an UPDATE whose AS4_PATH carries AS 0 has
// that attribute discarded by the whole-UPDATE attribute walk.
func TestRFC7607CompleteUpdateAS4PathZero(t *testing.T) {
	value := []byte{asPathTypeASSequence, 1, 0x00, 0x00, 0x00, 0x00}
	attrs := append(mandatoryAttrs(), asPathAttr([]uint32{65001}, false)...)
	attrs = append(attrs, 0xC0, attrCodeAS4Path, byte(len(value)))
	attrs = append(attrs, value...)
	result := ValidateUpdateRFC7606(attrs, true, false, false)
	if result == nil {
		t.Fatal("the attribute walk accepted an UPDATE whose AS4_PATH holds AS 0")
	}
	if result.Action != RFC7606ActionAttributeDiscard {
		t.Fatalf("action is %v (%s), want attribute-discard", result.Action, result.Description)
	}
}

// RFC requirement: RFC7607-2-3 negative -- the same complete UPDATE with a real
// four-octet AS in the AS4_PATH is accepted whole, so the discard is bound to the AS 0.
func TestRFC7607CompleteUpdateAS4PathAccepted(t *testing.T) {
	value := []byte{asPathTypeASSequence, 1, 0x00, 0x01, 0x00, 0x01}
	attrs := append(mandatoryAttrs(), asPathAttr([]uint32{65001}, false)...)
	attrs = append(attrs, 0xC0, attrCodeAS4Path, byte(len(value)))
	attrs = append(attrs, value...)
	result := ValidateUpdateRFC7606(attrs, true, false, false)
	if result == nil {
		t.Fatal("the attribute walk returned no result")
	}
	if result.Action != RFC7606ActionNone {
		t.Errorf("action is %v (%s), want none", result.Action, result.Description)
	}
}

// TestRFC7607ReachesTheAttributeWalk proves the checks are reachable through the entry
// point the session uses, not only through the leaf validators above.
// ValidateUpdateRFC7606 is what Session.enforceRFC7606 calls.
//
// RFC requirement: RFC7607-2-2 positive -- an UPDATE whose AS_PATH carries AS 0 is
// treat-as-withdrawn by the whole-UPDATE attribute walk.
func TestRFC7607ReachesTheAttributeWalk(t *testing.T) {
	attrs := asPathAttr([]uint32{65001, 0}, false)
	result := ValidateUpdateRFC7606(attrs, true, false, false)
	if result == nil {
		t.Fatal("the attribute walk accepted an UPDATE whose AS_PATH holds AS 0")
	}
	if result.Action != RFC7606ActionTreatAsWithdraw {
		t.Errorf("action is %v, want treat-as-withdraw", result.Action)
	}
}

// TestRFC7607AS4PathReachesTheAttributeWalk proves the walk dispatches to the code-17
// validator this package registers, so that validator is not dead code.
//
// The UPDATE declares no NLRI and carries the AS4_PATH alone. RFC 7606 Section 3.d then
// asks for no well-known mandatory attribute, and Section 5.2 escalates only an action
// STRONGER than attribute discard, so the AS 0 is the only verdict the walk can reach.
// TestRFC7607CompleteUpdateAS4PathZero above covers the same rule inside a complete
// UPDATE that does carry NLRI.
//
// RFC requirement: RFC7607-2-3 positive -- an UPDATE whose only path attribute is an
// AS4_PATH carrying AS 0 has that attribute discarded by the whole-UPDATE attribute walk.
func TestRFC7607AS4PathReachesTheAttributeWalk(t *testing.T) {
	value := []byte{asPathTypeASSequence, 1, 0x00, 0x00, 0x00, 0x00}
	attrs := append([]byte{0xC0, attrCodeAS4Path, byte(len(value))}, value...)
	result := ValidateUpdateRFC7606(attrs, false, false, false)
	if result == nil {
		t.Fatal("the attribute walk accepted an UPDATE whose AS4_PATH holds AS 0")
	}
	if result.Action != RFC7606ActionAttributeDiscard {
		t.Fatalf("action is %v (%s), want attribute-discard", result.Action, result.Description)
	}
	if result.AttrCode != attrCodeAS4Path {
		t.Errorf("attribute code is %d, want %d", result.AttrCode, attrCodeAS4Path)
	}
}
