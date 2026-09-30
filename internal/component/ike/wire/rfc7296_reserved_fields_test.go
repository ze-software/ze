// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- payload codecs
// Related: rfc7296_test.go -- TestReservedFieldsIgnoredOnReceipt, the generic header flags
// Related: rfc7296_cp_test.go -- TestConfigAttributeReservedBitIgnoredOnReceipt, the CP attribute R bit

package wire

import (
	"reflect"
	"testing"
)

// rsvCodec is a payload body codec: it encodes a body and decodes one.
type rsvCodec interface {
	Len() int
	WriteTo(buf []byte, off int) int
	ReadFrom(data []byte) error
}

// rsvCase is one payload whose body carries RESERVED octets. reserved lists every
// RESERVED octet offset in the body; defined is the offset of a defined field next to
// them; fresh returns an empty value of the same type to parse into.
type rsvCase struct {
	name     string
	payload  rsvCodec
	reserved []int
	defined  int
	fresh    func() rsvCodec
}

// rsvCases covers every payload body Ze decodes that has a RESERVED field: AUTH, KE, ID,
// TS and CP (the octets after the first field), and the SA payload's Proposal (octet 1)
// and Transform (octets 1 and 5) substructures. The generic header and the CP attribute
// R bit have their own tests.
func rsvCases() []rsvCase {
	transform := Transform{
		IsLast: true, Type: TransformTypeENCR, ID: 12,
		Attrs: []TransformAttr{{Type: AttrTypeKeyLength, Value: 128}},
	}
	// The transform follows the 8-octet proposal header; an IKE_SA_INIT proposal has no SPI.
	const transformOff = 8
	return []rsvCase{
		{"AUTH", &PayloadAUTH{AuthMethod: AuthMethodPSK, AuthData: []byte{1, 2, 3, 4}},
			[]int{1, 2, 3}, 0, func() rsvCodec { return &PayloadAUTH{} }},
		{"KE", &PayloadKE{DHGroup: 14, KeyExchangeData: []byte{5, 6, 7, 8}},
			[]int{2, 3}, 1, func() rsvCodec { return &PayloadKE{} }},
		{"ID", &PayloadID{IDType: IDTypeFQDN, IDData: []byte("ze.example")},
			[]int{1, 2, 3}, 0, func() rsvCodec { return &PayloadID{} }},
		{"TS", &PayloadTS{TrafficSelectors: []TrafficSelector{{
			TSType: TSTypeIPv4AddrRange, StartPort: 0, EndPort: 65535,
			StartAddress: []byte{10, 0, 0, 0}, EndAddress: []byte{10, 0, 0, 255},
		}}}, []int{1, 2, 3}, 0, func() rsvCodec { return &PayloadTS{} }},
		{"CP", &PayloadCP{CFGType: CFGTypeRequest, Attrs: []ConfigAttr{{Type: CPAttrInternalIP4Address}}},
			[]int{1, 2, 3}, 0, func() rsvCodec { return &PayloadCP{} }},
		{"SA", &PayloadSA{Proposals: []Proposal{{
			IsLast: true, Number: 1, ProtocolID: ProtocolIKE, Transforms: []Transform{transform},
		}}}, []int{1, transformOff + 1, transformOff + 5}, transformOff + 7,
			func() rsvCodec { return &PayloadSA{} }},
	}
}

// rsvBody encodes the case's payload body.
func rsvBody(c *rsvCase) []byte {
	body := make([]byte, c.payload.Len())
	c.payload.WriteTo(body, 0)
	return body
}

// VALIDATES: RFC 7296 Section 2.5, "their content MUST be ignored by an implementation
// running version 2.0": every RESERVED octet of every payload body is ignored on receipt.
// PREVENTS: a decoder that folds a RESERVED octet into a field, or refuses a peer that
// sets one.
//
// METHOD: each payload body is encoded, parsed once as sent (RESERVED zero), then parsed
// again with every RESERVED octet set to 0xff. Both parses must succeed and decode to
// the same value. The encoder is also checked to send each RESERVED octet as zero, so the
// comparison starts from a clean body.
//
// RFC requirement: RFC7296-2.5-7 positive -- the AUTH, KE, ID, TS and CP payload bodies and
// the SA payload's Proposal and Transform substructures, parsed with every RESERVED octet
// set to 0xff, decode without error to the same value as with those octets zero.
func TestRFC7296EveryReservedOctetIsIgnoredOnReceipt(t *testing.T) {
	for _, c := range rsvCases() {
		body := rsvBody(&c)
		for _, off := range c.reserved {
			if body[off] != 0 {
				t.Fatalf("%s: the encoder sent RESERVED octet %d as %#x, want 0", c.name, off, body[off])
			}
		}
		clean := c.fresh()
		if err := clean.ReadFrom(body); err != nil {
			t.Fatalf("%s: ReadFrom(clean body): %v", c.name, err)
		}
		noisy := append([]byte(nil), body...)
		for _, off := range c.reserved {
			noisy[off] = 0xff
		}
		got := c.fresh()
		if err := got.ReadFrom(noisy); err != nil {
			t.Errorf("%s: ReadFrom refused a body whose RESERVED octets are 0xff: %v", c.name, err)
			continue
		}
		if !reflect.DeepEqual(got, clean) {
			t.Errorf("%s: RESERVED octets changed the decode:\n got %+v\nwant %+v", c.name, got, clean)
		}
	}
}

// RFC requirement: RFC7296-2.5-7 negative -- the ignore is confined to the RESERVED octets:
// for each of the same bodies, setting the DEFINED octet next to them (AUTH Auth Method, KE
// DH Group, ID Type, TS Number of TSs, CP CFG Type, the Transform ID) to 0xff changes the
// decode or is refused, so the decoder reads the defined fields it is handed.
func TestRFC7296DefinedOctetBesideReservedIsNotIgnored(t *testing.T) {
	for _, c := range rsvCases() {
		body := rsvBody(&c)
		clean := c.fresh()
		if err := clean.ReadFrom(body); err != nil {
			t.Fatalf("%s: ReadFrom(clean body): %v", c.name, err)
		}
		if body[c.defined] == 0xff {
			t.Fatalf("%s: defined octet %d is already 0xff, so the flip tests nothing", c.name, c.defined)
		}
		flipped := append([]byte(nil), body...)
		flipped[c.defined] = 0xff
		got := c.fresh()
		if err := got.ReadFrom(flipped); err != nil {
			continue
		}
		if reflect.DeepEqual(got, clean) {
			t.Errorf("%s: defined octet %d set to 0xff decoded to the same value; it was ignored",
				c.name, c.defined)
		}
	}
}
