// RFC: rfc/short/rfc2865.md -- RFC2865-5-9, the string data type of Section 5
// Related: packet.go -- EncodeTo, the wire-boundary guard that omits an empty value
// Related: rfc2865_walk_test.go -- encodedPacket, and the text-type twin of this rule

// VALIDATES: an attribute of the string data type (binary octets) whose value is
// zero octets long never reaches the wire, and a one-octet string still does.
// PREVENTS: an empty State, Class, Proxy-State or CHAP-Challenge going out as a
// two-octet attribute, which RFC 2865 Section 5 forbids for the string type.
package radius

import (
	"testing"
)

// stringTypedAttrs are attributes RFC 2865 declares with the string data type.
var stringTypedAttrs = []uint8{AttrState, AttrClass, AttrProxyState, AttrCHAPChallenge}

// TestRFC2865ZeroLengthStringIsOmitted encodes an Access-Request carrying each
// string-typed attribute with an empty value beside a User-Name, and reads the
// decoded wire back: only the User-Name is present.
func TestRFC2865ZeroLengthStringIsOmitted(t *testing.T) {
	for _, attrType := range stringTypedAttrs {
		pkt := encodedPacket(t, CodeAccessRequest, 9, []Attr{
			{Type: AttrUserName, Value: []byte("alice")},
			{Type: attrType, Value: []byte{}},
		})
		decoded, err := Decode(pkt)
		if err != nil {
			t.Fatalf("attribute %d: decode: %v", attrType, err)
		}
		// RFC requirement: RFC2865-5-9 positive -- a string-typed attribute of
		// length zero is not sent: the entire attribute is omitted from the wire.
		if decoded.FindAttr(attrType) != nil {
			t.Fatalf("attribute %d with a zero-length string MUST be omitted", attrType)
		}
		if len(decoded.Attrs) != 1 {
			t.Fatalf("attribute %d: got %d attributes on the wire, want only User-Name", attrType, len(decoded.Attrs))
		}
	}
}

// TestRFC2865OneOctetStringIsSent encodes the same attributes with a one-octet
// value 0x00, the shortest legal string, and reads each back byte-equal, so the
// omission above is driven by the zero length alone.
func TestRFC2865OneOctetStringIsSent(t *testing.T) {
	for _, attrType := range stringTypedAttrs {
		pkt := encodedPacket(t, CodeAccessRequest, 9, []Attr{
			{Type: AttrUserName, Value: []byte("alice")},
			{Type: attrType, Value: []byte{0x00}},
		})
		decoded, err := Decode(pkt)
		if err != nil {
			t.Fatalf("attribute %d: decode: %v", attrType, err)
		}
		// RFC requirement: RFC2865-5-9 negative -- the rule names length zero, so
		// a one-octet string is a legal value and is sent unchanged.
		value := decoded.FindAttr(attrType)
		if len(value) != 1 || value[0] != 0x00 {
			t.Fatalf("attribute %d: got %x on the wire, want the one-octet string 00", attrType, value)
		}
	}
}
