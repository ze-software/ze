// VALIDATES: the RFC 7296 Section 2.5 RESERVED-field obligations where the Configuration
// payload carries them. Section 3.15.1 splits each Configuration Attribute header into a
// 1-bit Reserved field and a 15-bit Attribute Type. The CP codec is therefore a producer of
// the send-as-zero rule and of the ignore-on-receipt rule.
// PREVENTS: a return to a full sixteen-bit read or write of the attribute type. Sixteen bits
// fold the Reserved bit into the type. A peer that sets that bit turns INTERNAL_IP4_ADDRESS
// (1) into 0x8001, and an ordinary address request reads as an unrecognized attribute.

package wire

import (
	"encoding/binary"
	"testing"
)

// cpAttrHeader lays out one zero-length Configuration Attribute as it arrives on the wire.
// It writes the raw sixteen bits the peer sent, then a zero value length. Zero length is the
// form a CFG_REQUEST uses to ask for a value. The type stays unmasked on purpose. These
// tests exist to check what ze does with the top bit.
func cpAttrHeader(rawType uint16) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint16(out[0:], rawType)
	binary.BigEndian.PutUint16(out[2:], 0)
	return out
}

// cpRequestBody builds a CFG_REQUEST Configuration payload body: the CFG type, three
// reserved octets, then the attribute headers as given. Every fixture here is a request,
// because a CFG_REQUEST is what an IRAC sends and what these tests parse.
func cpRequestBody(attrs ...[]byte) []byte {
	body := []byte{CFGTypeRequest, 0, 0, 0}
	for _, a := range attrs {
		body = append(body, a...)
	}
	return body
}

// RFC requirement: RFC7296-2.5-7 positive -- the content of a RESERVED field is ignored on
// receipt. The Configuration Attribute header is one of the places RFC 7296 marks such a
// field. RFC 7296 Section 3.15.1 gives the header as |R| Attribute Type (15 bits) |. It says
// of R: "This bit MUST be set to zero and MUST be ignored on receipt".
//
// PayloadCP.ReadFrom
// (payload_cp.go) masks the octet pair with cpAttrTypeMask. An INTERNAL_IP4_ADDRESS request
// that carries a set Reserved bit therefore parses to the same attribute as the clean
// encoding. Without the mask the attribute reads as 0x8001, and no recognizer matches it.
//
// RFC requirement: RFC7296-2.5-7 negative -- the ignore is scoped to the Reserved bit alone. The
// remaining fifteen bits still decide which attribute this is. A set Reserved bit over
// INTERNAL_IP4_NETMASK parses to the netmask and not to the address. An unrecognized type
// survives the mask unchanged. The parser discards one bit, not the octet pair.
func TestConfigAttributeReservedBitIgnoredOnReceipt(t *testing.T) {
	// Positive: the same attribute, with and without the Reserved bit set.
	clean := cpRequestBody(cpAttrHeader(CPAttrInternalIP4Address))
	noisy := cpRequestBody(cpAttrHeader(0x8000 | CPAttrInternalIP4Address))

	var cleanCP, noisyCP PayloadCP
	if err := cleanCP.ReadFrom(clean); err != nil {
		t.Fatalf("ReadFrom(clean): %v", err)
	}
	if err := noisyCP.ReadFrom(noisy); err != nil {
		t.Fatalf("ReadFrom(reserved bit set): %v", err)
	}
	if len(noisyCP.Attrs) != 1 {
		t.Fatalf("attribute count = %d, want 1", len(noisyCP.Attrs))
	}
	if noisyCP.Attrs[0].Type != CPAttrInternalIP4Address {
		t.Errorf("INTERNAL_IP4_ADDRESS sent with the Reserved bit set parsed as type %#04x, "+
			"want %#04x; the Reserved bit was folded into the attribute type",
			noisyCP.Attrs[0].Type, CPAttrInternalIP4Address)
	}
	if noisyCP.Attrs[0].Type != cleanCP.Attrs[0].Type {
		t.Errorf("the Reserved bit changed the parse: got %#04x, clean encoding gives %#04x",
			noisyCP.Attrs[0].Type, cleanCP.Attrs[0].Type)
	}

	// The same masking applies on the real inbound path, not only to a direct ReadFrom.
	// decodePayload routes payload type 47 to PayloadCP.ReadFrom (payload.go).
	raw := buildChain(t, testHeader(), []PayloadEntry{
		{Payload: &PayloadCP{CFGType: CFGTypeRequest, Attrs: []ConfigAttr{
			{Type: 0x8000 | CPAttrInternalIP4Address},
		}}},
	})
	var msg Message
	if err := msg.ReadFrom(raw); err != nil {
		t.Fatalf("Message.ReadFrom: %v", err)
	}
	decoded, ok := msg.Payloads[0].Payload.(*PayloadCP)
	if !ok {
		t.Fatalf("first payload is %T, want *PayloadCP", msg.Payloads[0].Payload)
	}
	if len(decoded.Attrs) != 1 || decoded.Attrs[0].Type != CPAttrInternalIP4Address {
		t.Errorf("through the generic decoder: attrs = %+v, want one attribute of type %#04x",
			decoded.Attrs, CPAttrInternalIP4Address)
	}

	// Negative: the fifteen type bits still select the attribute.
	netmask := cpRequestBody(cpAttrHeader(0x8000 | CPAttrInternalIP4Netmask))
	var netmaskCP PayloadCP
	if err := netmaskCP.ReadFrom(netmask); err != nil {
		t.Fatalf("ReadFrom(netmask): %v", err)
	}
	if netmaskCP.Attrs[0].Type != CPAttrInternalIP4Netmask {
		t.Errorf("INTERNAL_IP4_NETMASK with the Reserved bit set parsed as %#04x, want %#04x",
			netmaskCP.Attrs[0].Type, CPAttrInternalIP4Netmask)
	}
	if netmaskCP.Attrs[0].Type == CPAttrInternalIP4Address {
		t.Error("masking collapsed two different attribute types onto one")
	}

	// Negative: an unrecognized type is preserved, not rewritten. Ze must be able to tell
	// "an attribute I do not know" apart from "an attribute I do know", because RFC 7296
	// Section 3.15.1 requires unrecognized attributes to be ignored rather than acted on.
	unknown := cpRequestBody(cpAttrHeader(0x7ffe))
	var unknownCP PayloadCP
	if err := unknownCP.ReadFrom(unknown); err != nil {
		t.Fatalf("ReadFrom(unknown type): %v", err)
	}
	if unknownCP.Attrs[0].Type != 0x7ffe {
		t.Errorf("unknown attribute type parsed as %#04x, want 0x7ffe; the mask must clear "+
			"the Reserved bit only", unknownCP.Attrs[0].Type)
	}
}

// RFC requirement: RFC7296-2.5-6 positive -- every RESERVED field is sent as zero. The
// Configuration Attribute header carries one. PayloadCP.WriteTo (payload_cp.go) masks the
// attribute type with cpAttrTypeMask. The Reserved bit therefore reaches the wire as zero
// even when a caller supplies a type with the top bit set. The same function writes the
// three reserved octets after the CFG type as zero.
// RFC requirement: RFC7296-2.5-6 negative -- the zeroing is scoped to the Reserved bit. The
// fifteen type bits reach the wire unchanged, so 0x7fff round-trips intact. WriteTo clears
// one bit. It does not clamp the field to a known attribute set, which would silently
// rewrite an attribute ze had been asked to send.
func TestConfigAttributeReservedBitSentAsZero(t *testing.T) {
	// Positive: a caller-supplied type with the Reserved bit set is emitted with it clear.
	cp := &PayloadCP{CFGType: CFGTypeReply, Attrs: []ConfigAttr{
		{Type: 0x8000 | CPAttrInternalIP4Address, Value: []byte{10, 0, 0, 1}},
	}}
	buf := make([]byte, cp.Len())
	cp.WriteTo(buf, 0)

	if buf[1] != 0 || buf[2] != 0 || buf[3] != 0 {
		t.Errorf("CP reserved octets = %02x %02x %02x, want all zero", buf[1], buf[2], buf[3])
	}
	onWire := binary.BigEndian.Uint16(buf[4:])
	if onWire&0x8000 != 0 {
		t.Errorf("attribute type octets = %#04x, want the Reserved bit clear", onWire)
	}
	if onWire != CPAttrInternalIP4Address {
		t.Errorf("attribute type octets = %#04x, want %#04x", onWire, CPAttrInternalIP4Address)
	}

	// Negative: the fifteen type bits are not clamped. The largest type expressible in the
	// field survives the write and comes back unchanged.
	wide := &PayloadCP{CFGType: CFGTypeRequest, Attrs: []ConfigAttr{{Type: 0x7fff}}}
	wideBuf := make([]byte, wide.Len())
	wide.WriteTo(wideBuf, 0)
	if got := binary.BigEndian.Uint16(wideBuf[4:]); got != 0x7fff {
		t.Errorf("attribute type 0x7fff was written as %#04x; the mask must clear the "+
			"Reserved bit only, never narrow the type", got)
	}

	var back PayloadCP
	if err := back.ReadFrom(wideBuf); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if len(back.Attrs) != 1 || back.Attrs[0].Type != 0x7fff {
		t.Errorf("round trip of type 0x7fff gave %+v, want one attribute of type 0x7fff",
			back.Attrs)
	}
}
