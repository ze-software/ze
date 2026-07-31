// VALIDATES: the RFC 7296 Section 3.3 obligations the SA payload codec discharges.
// The list covers proposal numbering (§3.3) and the SPI Size field (§3.3.1). It
// covers duplicate transform attributes (§3.3) and the attribute encoding formats
// (§3.3.5). It also covers the network byte order of the Key Length attribute, and
// the transforms that must not carry it (§3.3.5). Each test carries an `RFC requirement:`
// tag with its checklist id.
// PREVENTS: a parser change that admits a malformed attribute list. It also prevents
// a mis-read of the Key Length byte order. Last, it prevents an unbounded parse of a
// peer-supplied attribute or transform count. The SA payload is parsed before any
// peer is authenticated.
package wire

import (
	"bytes"
	"errors"
	"testing"
)

// transformBytes builds one transform substructure. last selects the Last Substruc
// value, and body holds the attribute area verbatim.
func transformBytes(last bool, ttype uint8, id uint16, body []byte) []byte {
	out := make([]byte, transformHeaderLen, transformHeaderLen+len(body))
	if last {
		out[0] = 0
	} else {
		out[0] = 3
	}
	total := transformHeaderLen + len(body)
	out[2] = byte(total >> 8)
	out[3] = byte(total)
	out[4] = ttype
	out[6] = byte(id >> 8)
	out[7] = byte(id)
	return append(out, body...)
}

// proposalBytes builds one proposal substructure around already-encoded transforms.
// last selects the Last Substruc value: 0 ends the SA payload, 2 says more follow.
func proposalBytes(last bool, num, proto, spiSize uint8, spi []byte, count uint8, transforms []byte) []byte {
	out := make([]byte, proposalHeaderLen)
	if last {
		out[0] = 0
	} else {
		out[0] = 2
	}
	out[4] = num
	out[5] = proto
	out[6] = spiSize
	out[7] = count
	out = append(out, spi...)
	out = append(out, transforms...)
	out[2] = byte(len(out) >> 8)
	out[3] = byte(len(out))
	return out
}

// tvAttr encodes one attribute in the TV (fixed-length) form: the AF bit is one.
func tvAttr(atype, value uint16) []byte {
	head := atype | 0x8000
	return []byte{byte(head >> 8), byte(head), byte(value >> 8), byte(value)}
}

// tlvAttr encodes one attribute in the TLV (variable-length) form: the AF bit is zero
// and the second half-word carries the value length.
func tlvAttr(atype uint16, value []byte) []byte {
	out := []byte{byte(atype >> 8), byte(atype), byte(len(value) >> 8), byte(len(value))}
	return append(out, value...)
}

// ikeProposalWith wraps one transform area into a single-proposal SA payload body.
func ikeProposalWith(ttype uint8, id uint16, attrs []byte) []byte {
	tr := transformBytes(true, ttype, id, attrs)
	return proposalBytes(true, 1, ProtocolIKE, 0, nil, 1, tr)
}

// RFC requirement: RFC7296-3.3-6 negative -- RFC 7296 Section 3.3 states that a transform MUST
// NOT have multiple attributes of one type. Transform.ReadFrom refuses a transform whose
// attribute area repeats an attribute type. The malformed encoding therefore never
// reaches negotiation.
// RFC requirement: RFC7296-3.3-6 positive -- one attribute of a given type is the conforming
// encoding, and it parses. The rejection names the repeat, not attributes as such.
func TestPropDuplicateAttributeRejected(t *testing.T) {
	twice := append(tvAttr(AttrTypeKeyLength, 128), tvAttr(AttrTypeKeyLength, 256)...)
	var sa PayloadSA
	if err := sa.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, twice)); !errors.Is(err, ErrDuplicateAttr) {
		t.Errorf("ReadFrom(two Key Length attributes) = %v, want ErrDuplicateAttr", err)
	}

	var ok PayloadSA
	if err := ok.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 128))); err != nil {
		t.Fatalf("ReadFrom(one Key Length attribute) = %v, want a parsed payload", err)
	}
	if got := ok.Proposals[0].Transforms[0].Attrs[0].Value; got != 128 {
		t.Errorf("key length = %d, want 128", got)
	}
}

// RFC requirement: RFC7296-3.3-7 positive -- RFC 7296 Section 3.3 states that alternate values
// for an attribute MUST use multiple transforms of one Transform Type, each with a
// single attribute. Two ENCR transforms, each with one Key Length attribute, parse into
// two separate transforms. Both survive, so the codec supports that encoding.
// RFC requirement: RFC7296-3.3-7 negative -- one transform that holds both key lengths is the
// other encoding of the same intent, and it is refused. Alternate values have one
// legal shape.
func TestPropAlternateKeyLengthsUseSeparateTransforms(t *testing.T) {
	first := transformBytes(false, TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 128))
	second := transformBytes(true, TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 256))
	body := proposalBytes(true, 1, ProtocolIKE, 0, nil, 2, append(first, second...))

	var sa PayloadSA
	if err := sa.ReadFrom(body); err != nil {
		t.Fatalf("ReadFrom(two ENCR transforms) = %v, want a parsed payload", err)
	}
	tr := sa.Proposals[0].Transforms
	if len(tr) != 2 {
		t.Fatalf("recovered %d transforms, want 2", len(tr))
	}
	if len(tr[0].Attrs) != 1 || len(tr[1].Attrs) != 1 {
		t.Fatalf("attribute counts = %d and %d, want 1 each", len(tr[0].Attrs), len(tr[1].Attrs))
	}
	if tr[0].Attrs[0].Value != 128 || tr[1].Attrs[0].Value != 256 {
		t.Errorf("key lengths = %d and %d, want 128 and 256", tr[0].Attrs[0].Value, tr[1].Attrs[0].Value)
	}

	both := append(tvAttr(AttrTypeKeyLength, 128), tvAttr(AttrTypeKeyLength, 256)...)
	var packed PayloadSA
	if err := packed.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, both)); !errors.Is(err, ErrDuplicateAttr) {
		t.Errorf("ReadFrom(one transform, two key lengths) = %v, want ErrDuplicateAttr", err)
	}
}

// RFC requirement: RFC7296-3.3.5-1 negative -- RFC 7296 Section 3.3.5 states that a fixed-length
// attribute MUST NOT use the variable-length encoding, unless its length is more than
// two bytes. Key Length is fixed at two octets. Its TLV form is therefore refused,
// whatever value length it declares.
// RFC requirement: RFC7296-3.3.5-1 positive -- the TV form of the same attribute is the
// conforming encoding and parses to the value it carries.
func TestPropFixedLengthAttributeRejectsTLVEncoding(t *testing.T) {
	for _, value := range [][]byte{{0x01, 0x00}, {0x00, 0x00, 0x01, 0x00}, {}} {
		body := ikeProposalWith(TransformTypeENCR, 12, tlvAttr(AttrTypeKeyLength, value))
		var sa PayloadSA
		if err := sa.ReadFrom(body); !errors.Is(err, ErrAttrEncoding) {
			t.Errorf("ReadFrom(TLV Key Length, %d-octet value) = %v, want ErrAttrEncoding", len(value), err)
		}
	}

	var sa PayloadSA
	if err := sa.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 256))); err != nil {
		t.Fatalf("ReadFrom(TV Key Length) = %v, want a parsed payload", err)
	}
	if got := sa.Proposals[0].Transforms[0].Attrs[0]; got.Variable || got.Value != 256 {
		t.Errorf("attribute = {variable:%v value:%d}, want {false 256}", got.Variable, got.Value)
	}
}

// RFC requirement: RFC7296-3.3.5-2 negative -- "Variable-length attributes MUST NOT be encoded as
// fixed-length even if their value can fit into two octets" (RFC 7296 Section 3.3.5). The
// codec keeps one registry of attribute formats and refuses a TV encoding of any
// attribute the registry declares variable length. RFC 7296 defines no variable-length
// attribute, so the test registers one for the length of the test to reach the branch.
// RFC requirement: RFC7296-3.3.5-2 positive -- the TLV form of the same attribute parses, and the
// value bytes survive, so the refusal is specific to the encoding format.
func TestPropVariableLengthAttributeRejectsTVEncoding(t *testing.T) {
	const testAttrType uint16 = 4096
	attrFormats[testAttrType] = attrSpec{variable: true}
	t.Cleanup(func() { delete(attrFormats, testAttrType) })

	var sa PayloadSA
	if err := sa.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, tvAttr(testAttrType, 7))); !errors.Is(err, ErrAttrEncoding) {
		t.Errorf("ReadFrom(TV encoding of a variable-length attribute) = %v, want ErrAttrEncoding", err)
	}

	var ok PayloadSA
	body := ikeProposalWith(TransformTypeENCR, 12, tlvAttr(testAttrType, []byte{9, 8, 7}))
	if err := ok.ReadFrom(body); err != nil {
		t.Fatalf("ReadFrom(TLV encoding of a variable-length attribute) = %v, want a parsed payload", err)
	}
	got := ok.Proposals[0].Transforms[0].Attrs[0]
	if !got.Variable || !bytes.Equal(got.Data, []byte{9, 8, 7}) {
		t.Errorf("attribute = {variable:%v data:%x}, want {true 090807}", got.Variable, got.Data)
	}
}

// RFC requirement: RFC7296-3.3.5-3 positive -- "The Key Length attribute specifies the key length
// in bits and MUST use network byte order" (RFC 7296 Section 3.3.5). A key length of 256
// bits encodes as 0x01 0x00, most significant octet first, and reads back as 256.
// RFC requirement: RFC7296-3.3.5-3 negative -- the byte order is load-bearing rather than
// incidental. The same two octets in the opposite order read back as 1 rather than 256,
// so a little-endian writer cannot pass unnoticed.
func TestPropKeyLengthUsesNetworkByteOrder(t *testing.T) {
	tr := Transform{Type: TransformTypeENCR, ID: 12, IsLast: true,
		Attrs: []TransformAttr{{Type: AttrTypeKeyLength, Value: 256}}}
	buf := make([]byte, tr.length())
	tr.WriteTo(buf, 0)
	value := buf[transformHeaderLen+2 : transformHeaderLen+4]
	if value[0] != 0x01 || value[1] != 0x00 {
		t.Errorf("key length 256 encoded as %02x %02x, want 01 00", value[0], value[1])
	}

	var sa PayloadSA
	if err := sa.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 256))); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got := sa.Proposals[0].Transforms[0].Attrs[0].Value; got != 256 {
		t.Errorf("key length = %d, want 256", got)
	}

	swapped := tvAttr(AttrTypeKeyLength, 256)
	swapped[2], swapped[3] = swapped[3], swapped[2]
	var rev PayloadSA
	if err := rev.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, swapped)); err != nil {
		t.Fatalf("ReadFrom(swapped octets): %v", err)
	}
	if got := rev.Proposals[0].Transforms[0].Attrs[0].Value; got != 1 {
		t.Errorf("swapped octets read as %d, want 1; the parser must read network byte order", got)
	}
}

// RFC requirement: RFC7296-3.3.5-4 negative -- "The Key Length attribute MUST NOT be used with
// transforms that use a fixed-length key ... this includes ENCR_DES, ENCR_IDEA, and all
// the Type 2 (Pseudorandom Function) and Type 3 (Integrity Algorithm) transforms" (RFC
// 7296 Section 3.3.5). Each of those carriers is refused.
// RFC requirement: RFC7296-3.3.5-4 positive -- a variable-length-key encryption transform is the
// carrier the attribute exists for, and ENCR_AES_CBC with a Key Length attribute parses.
func TestPropKeyLengthRejectedOnFixedKeyTransform(t *testing.T) {
	cases := []struct {
		name  string
		ttype uint8
		id    uint16
	}{
		{"PRF", TransformTypePRF, 5},
		{"INTEG", TransformTypeINTG, 12},
		// rfc-test-change-approved: 2026-07-30 review found the tag overstated Section 3.3.5; ESN and DH are not named by the RFC
		{"ENCR_DES", TransformTypeENCR, encrDES},
		{"ENCR_IDEA", TransformTypeENCR, encrIDEA},
	}
	for _, c := range cases {
		body := ikeProposalWith(c.ttype, c.id, tvAttr(AttrTypeKeyLength, 128))
		var sa PayloadSA
		if err := sa.ReadFrom(body); !errors.Is(err, ErrKeyLengthNotAllowed) {
			t.Errorf("ReadFrom(Key Length on %s) = %v, want ErrKeyLengthNotAllowed", c.name, err)
		}
	}

	var sa PayloadSA
	if err := sa.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 128))); err != nil {
		t.Errorf("ReadFrom(Key Length on ENCR_AES_CBC) = %v, want a parsed payload", err)
	}
}

// RFC requirement: RFC7296-3.3.1-2 positive -- RFC 7296 Section 3.3.1 makes the field zero for an
// initial IKE SA negotiation, because the SPI comes from the outer header. An IKE
// proposal with SPI Size zero parses and holds no SPI octets. The sizes that section
// gives for a later negotiation also parse: 8 for IKE, and 4 for ESP and AH.
// RFC requirement: RFC7296-3.3.1-2 negative -- any other size is refused with ErrInvalidSPISize.
// A peer cannot shift the transform area with a size the protocol never uses.
func TestPropSPISizeMatchesProtocol(t *testing.T) {
	tr := transformBytes(true, TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 128))

	var initial PayloadSA
	if err := initial.ReadFrom(proposalBytes(true, 1, ProtocolIKE, 0, nil, 1, tr)); err != nil {
		t.Fatalf("ReadFrom(initial IKE, SPI Size 0) = %v, want a parsed payload", err)
	}
	if len(initial.Proposals[0].SPI) != 0 {
		t.Errorf("initial IKE proposal carries %d SPI octets, want 0", len(initial.Proposals[0].SPI))
	}

	valid := []struct {
		name  string
		proto uint8
		size  uint8
	}{
		{"IKE rekey", ProtocolIKE, 8},
		{"ESP", ProtocolESP, 4},
		{"AH", ProtocolAH, 4},
	}
	for _, c := range valid {
		body := proposalBytes(true, 1, c.proto, c.size, make([]byte, c.size), 1, tr)
		var sa PayloadSA
		if err := sa.ReadFrom(body); err != nil {
			t.Errorf("ReadFrom(%s, SPI Size %d) = %v, want a parsed payload", c.name, c.size, err)
		}
	}

	bad := []struct {
		name  string
		proto uint8
		size  uint8
	}{
		{"IKE with an ESP-sized SPI", ProtocolIKE, 4},
		{"ESP with an IKE-sized SPI", ProtocolESP, 8},
		{"ESP with no SPI", ProtocolESP, 0},
		{"AH with a 1-octet SPI", ProtocolAH, 1},
	}
	for _, c := range bad {
		body := proposalBytes(true, 1, c.proto, c.size, make([]byte, c.size), 1, tr)
		var sa PayloadSA
		if err := sa.ReadFrom(body); !errors.Is(err, ErrInvalidSPISize) {
			t.Errorf("ReadFrom(%s) = %v, want ErrInvalidSPISize", c.name, err)
		}
	}
}

// RFC requirement: RFC7296-3.3-5 positive -- "Each structure MUST have a proposal number one (1)
// greater than the previous structure. The first Proposal in the initiator's SA payload
// MUST have a Proposal Num of one (1)" (RFC 7296 Section 3.3). An offer numbered 1, 2, 3
// satisfies ValidateOfferNumbering.
// RFC requirement: RFC7296-3.3-5 negative -- four shapes are refused: a first number that is
// not one, a repeated number, a skipped number, and a descending sequence. A
// malformed offer cannot be negotiated.
func TestPropProposalNumbering(t *testing.T) {
	mk := func(nums ...uint8) *PayloadSA {
		sa := &PayloadSA{}
		for _, n := range nums {
			sa.Proposals = append(sa.Proposals, Proposal{Number: n, ProtocolID: ProtocolIKE})
		}
		return sa
	}

	for _, good := range [][]uint8{{1}, {1, 2}, {1, 2, 3}} {
		if err := mk(good...).ValidateOfferNumbering(); err != nil {
			t.Errorf("ValidateOfferNumbering(%v) = %v, want nil", good, err)
		}
	}

	for _, bad := range [][]uint8{{2}, {0}, {1, 1}, {1, 3}, {2, 3}, {3, 2, 1}} {
		if err := mk(bad...).ValidateOfferNumbering(); !errors.Is(err, ErrProposalNumbering) {
			t.Errorf("ValidateOfferNumbering(%v) = %v, want ErrProposalNumbering", bad, err)
		}
	}

	// The same check applied to numbers recovered from the wire rather than set in
	// Go, so a peer's own encoding is what is judged.
	onWire := func(second uint8) *PayloadSA {
		tr := transformBytes(true, TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 128))
		body := proposalBytes(false, 1, ProtocolIKE, 0, nil, 1, tr)
		body = append(body, proposalBytes(true, second, ProtocolIKE, 0, nil, 1, tr)...)
		sa := &PayloadSA{}
		if err := sa.ReadFrom(body); err != nil {
			t.Fatalf("ReadFrom(two proposals, second numbered %d) = %v", second, err)
		}
		return sa
	}
	if err := onWire(2).ValidateOfferNumbering(); err != nil {
		t.Errorf("ValidateOfferNumbering(wire offer 1,2) = %v, want nil", err)
	}
	if err := onWire(3).ValidateOfferNumbering(); !errors.Is(err, ErrProposalNumbering) {
		t.Errorf("ValidateOfferNumbering(wire offer 1,3) = %v, want ErrProposalNumbering", err)
	}
}

// TestPropHostileAttributeCount feeds the parser attacker-shaped attribute and transform
// counts. The SA payload is parsed before any peer is authenticated. Every loop over a
// peer-supplied count must therefore be bounded, and no input CAN be allowed to panic.
func TestPropHostileAttributeCount(t *testing.T) {
	// attrs builds n attributes of distinct unknown types, so the count is what the
	// parser judges. A repeated type would be refused for the repeat instead.
	attrs := func(n int) []byte {
		var out []byte
		for i := range n {
			out = append(out, tvAttr(uint16(4096+i), 1)...)
		}
		return out
	}

	var sa PayloadSA
	if err := sa.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, attrs(MaxTransformAttrs+8))); !errors.Is(err, ErrTooManyAttrs) {
		t.Errorf("ReadFrom(%d attributes) = %v, want ErrTooManyAttrs", MaxTransformAttrs+8, err)
	}

	// The bound is pinned from both sides. MaxTransformAttrs attributes parse, and one
	// more is refused. A fixture well above the bound leaves the comparison free to
	// drift, because an off-by-one guard still refuses it.
	var atBound PayloadSA
	if err := atBound.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, attrs(MaxTransformAttrs))); err != nil {
		t.Fatalf("ReadFrom(%d attributes) = %v, want a parsed payload", MaxTransformAttrs, err)
	}
	if n := len(atBound.Proposals[0].Transforms[0].Attrs); n != MaxTransformAttrs {
		t.Errorf("recovered %d attributes, want %d", n, MaxTransformAttrs)
	}
	var overBound PayloadSA
	if err := overBound.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, attrs(MaxTransformAttrs+1))); !errors.Is(err, ErrTooManyAttrs) {
		t.Errorf("ReadFrom(%d attributes) = %v, want ErrTooManyAttrs", MaxTransformAttrs+1, err)
	}

	// A TLV attribute whose declared length runs past the transform is refused rather
	// than read out of bounds.
	over := tlvAttr(4096, []byte{1, 2})
	over[2], over[3] = 0xff, 0xff
	var overrun PayloadSA
	if err := overrun.ReadFrom(ikeProposalWith(TransformTypeENCR, 12, over)); !errors.Is(err, ErrTruncated) {
		t.Errorf("ReadFrom(TLV length 65535) = %v, want ErrTruncated", err)
	}

	// The bound is the transform's own declared length, not the octets that follow
	// it. Take a first transform whose TLV attribute declares 8 value octets while
	// its own length leaves none. It must be refused, even though the second
	// transform supplies those octets inside the proposal.
	first := transformBytes(false, TransformTypeENCR, 12, []byte{0x10, 0x00, 0x00, 0x08})
	second := transformBytes(true, TransformTypeINTG, 12, nil)
	var reach PayloadSA
	body := proposalBytes(true, 1, ProtocolIKE, 0, nil, 2, append(first, second...))
	if err := reach.ReadFrom(body); !errors.Is(err, ErrTruncated) {
		t.Errorf("ReadFrom(TLV length reaching into the next transform) = %v, want ErrTruncated", err)
	}

	// A proposal that declares 255 transforms and holds one stops at its own length.
	// The count field is a claim, and the length field is the bound.
	tr := transformBytes(false, TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 128))
	var lying PayloadSA
	if err := lying.ReadFrom(proposalBytes(true, 1, ProtocolIKE, 0, nil, 255, tr)); err != nil {
		t.Fatalf("ReadFrom(255-transform claim holding one) = %v, want a parsed payload", err)
	}
	if n := len(lying.Proposals[0].Transforms); n != 1 {
		t.Errorf("parsed %d transforms from a 255-transform claim holding one, want 1", n)
	}

	// MaxNestingDepth bounds the transforms read from one proposal. This fixture holds
	// one more than the bound, so the loop reaches the guard instead of the end of the
	// proposal. The whole proposal is refused, and none of it is kept.
	depth := func(n int) []byte {
		var out []byte
		for i := range n {
			out = append(out, transformBytes(i == n-1, TransformTypeENCR, uint16(i), nil)...)
		}
		return out
	}
	var deep PayloadSA
	err := deep.ReadFrom(proposalBytes(true, 1, ProtocolIKE, 0, nil, uint8(MaxNestingDepth+1), depth(MaxNestingDepth+1)))
	if !errors.Is(err, ErrTooManyPayloads) {
		t.Errorf("ReadFrom(%d transforms) = %v, want ErrTooManyPayloads", MaxNestingDepth+1, err)
	}
	if n := len(deep.Proposals); n != 0 {
		t.Errorf("kept %d proposals from an over-deep offer, want 0", n)
	}

	// Exactly MaxNestingDepth transforms is the last accepted count.
	var atDepth PayloadSA
	if err := atDepth.ReadFrom(proposalBytes(true, 1, ProtocolIKE, 0, nil, uint8(MaxNestingDepth), depth(MaxNestingDepth))); err != nil {
		t.Fatalf("ReadFrom(%d transforms) = %v, want a parsed payload", MaxNestingDepth, err)
	}
	if n := len(atDepth.Proposals[0].Transforms); n != MaxNestingDepth {
		t.Errorf("recovered %d transforms, want %d", n, MaxNestingDepth)
	}

	// A transform header whose length field undercuts the header is refused.
	short := transformBytes(true, TransformTypeENCR, 12, nil)
	short[2], short[3] = 0, 2
	var undersized PayloadSA
	if err := undersized.ReadFrom(proposalBytes(true, 1, ProtocolIKE, 0, nil, 1, short)); !errors.Is(err, ErrTruncated) {
		t.Errorf("ReadFrom(transform length 2) = %v, want ErrTruncated", err)
	}
}

// messageWithSABody wraps an SA payload body, verbatim, in a message that also carries a
// Nonce payload. The raw body lets a test place octets that the SA encoder never writes.
func messageWithSABody(t *testing.T, saBody []byte) []byte {
	t.Helper()
	msg := Message{
		Header: Header{MajorVersion: 2, ExchangeType: ExchangeIKESAInit, Flags: FlagInitiator},
		Payloads: []PayloadEntry{
			{Payload: &PayloadRaw{PayloadType: PayloadTypeSA, Data: saBody}},
			{Payload: &PayloadNonce{NonceData: make([]byte, 32)}},
		},
	}
	buf := make([]byte, msg.Len())
	return buf[:msg.WriteTo(buf, 0)]
}

// VALIDATES: RFC 7296 Section 3.3.6, "however, other proposals in the same SA payload
// are processed as usual". A proposal this parser refuses is dropped, and the proposals
// beside it survive the parse.
// PREVENTS: a per-proposal refusal that discards the whole SA payload. One malformed
// proposal would then hide every proposal the responder can use.
func TestPropRejectedProposalKeepsSiblings(t *testing.T) {
	tr := transformBytes(true, TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 128))
	body := proposalBytes(false, 1, ProtocolIKE, 0, nil, 1, tr)
	// RFC 7296 Section 3.3.1 gives ESP an SPI of 4 octets, so 8 refuses this proposal.
	body = append(body, proposalBytes(false, 2, ProtocolESP, 8, make([]byte, 8), 1, tr)...)
	body = append(body, proposalBytes(true, 3, ProtocolIKE, 0, nil, 1, tr)...)

	var sa PayloadSA
	if err := sa.ReadFrom(body); !errors.Is(err, ErrInvalidSPISize) {
		t.Fatalf("ReadFrom(three proposals, the second malformed) = %v, want ErrInvalidSPISize", err)
	}
	if len(sa.Proposals) != 2 {
		t.Fatalf("kept %d proposals, want 2", len(sa.Proposals))
	}
	if sa.Proposals[0].Number != 1 || sa.Proposals[1].Number != 3 {
		t.Errorf("kept proposals %d and %d, want 1 and 3", sa.Proposals[0].Number, sa.Proposals[1].Number)
	}
}

// VALIDATES: RFC 7296 Section 3.3.6, "other transforms with the same Transform Type are
// processed as usual". A transform this parser refuses is dropped, and its proposal
// keeps the transforms beside it.
// PREVENTS: one malformed transform that discards its proposal, or the whole SA payload.
// The responder would then refuse an offer the RFC requires it to consider.
func TestPropRejectedTransformKeepsSiblings(t *testing.T) {
	first := transformBytes(false, TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 128))
	twice := append(tvAttr(AttrTypeKeyLength, 128), tvAttr(AttrTypeKeyLength, 256)...)
	middle := transformBytes(false, TransformTypeENCR, 12, twice)
	last := transformBytes(true, TransformTypeINTG, 12, nil)
	area := append(append(first, middle...), last...)

	var sa PayloadSA
	if err := sa.ReadFrom(proposalBytes(true, 1, ProtocolIKE, 0, nil, 3, area)); !errors.Is(err, ErrDuplicateAttr) {
		t.Fatalf("ReadFrom(three transforms, the middle one malformed) = %v, want ErrDuplicateAttr", err)
	}
	if len(sa.Proposals) != 1 {
		t.Fatalf("kept %d proposals, want 1", len(sa.Proposals))
	}
	tr := sa.Proposals[0].Transforms
	if len(tr) != 2 {
		t.Fatalf("kept %d transforms, want 2", len(tr))
	}
	if tr[0].Type != TransformTypeENCR || tr[1].Type != TransformTypeINTG {
		t.Errorf("kept transform types %d and %d, want %d and %d",
			tr[0].Type, tr[1].Type, TransformTypeENCR, TransformTypeINTG)
	}
}

// VALIDATES: RFC 7296 Section 3.3.6 at the message boundary. A refused transform costs
// its own octets and nothing else. The message parses, the other payloads survive, and
// the surviving proposal reaches negotiation.
// PREVENTS: the blast radius the hardening added. One transform inside proposal three
// must not discard proposals one and two, and it must not discard the message.
func TestPropMessageSurvivesRejectedTransform(t *testing.T) {
	good := transformBytes(false, TransformTypeENCR, 12, tvAttr(AttrTypeKeyLength, 128))
	twice := append(tvAttr(AttrTypeKeyLength, 128), tvAttr(AttrTypeKeyLength, 256)...)
	bad := transformBytes(true, TransformTypeENCR, 12, twice)
	saBody := proposalBytes(true, 1, ProtocolIKE, 0, nil, 2, append(good, bad...))

	var got Message
	if err := got.ReadFrom(messageWithSABody(t, saBody)); err != nil {
		t.Fatalf("ReadFrom(message whose SA payload holds one malformed transform) = %v, want a parsed message", err)
	}
	if len(got.Payloads) != 2 {
		t.Fatalf("recovered %d payloads, want 2", len(got.Payloads))
	}
	sa, ok := got.Payloads[0].Payload.(*PayloadSA)
	if !ok {
		t.Fatalf("first payload is %T, want *PayloadSA", got.Payloads[0].Payload)
	}
	if len(sa.Proposals) != 1 || len(sa.Proposals[0].Transforms) != 1 {
		t.Fatalf("recovered %d proposals holding %v transforms, want 1 proposal holding 1 transform",
			len(sa.Proposals), transformCounts(sa.Proposals))
	}
	if _, ok := got.Payloads[1].Payload.(*PayloadNonce); !ok {
		t.Errorf("second payload is %T, want *PayloadNonce", got.Payloads[1].Payload)
	}
}

// transformCounts reports the transform count of each proposal, for a failure message.
func transformCounts(props []Proposal) []int {
	out := make([]int, len(props))
	for i := range props {
		out[i] = len(props[i].Transforms)
	}
	return out
}

// VALIDATES: the line between a refused item and a malformed message. A length field
// that disagrees with its container leaves every later offset unknown. The parse stops
// there, and Message.ReadFrom refuses the message.
// PREVENTS: a per-item tolerance widened until it swallows a framing fault. RFC 7296
// Section 3.3.6 excuses an unacceptable proposal or transform. It excuses nothing about
// lengths, and a parser that reads past one is reading attacker-chosen offsets.
func TestPropFramingFaultStillRefusesTheMessage(t *testing.T) {
	over := tlvAttr(4096, []byte{1, 2})
	over[2], over[3] = 0xff, 0xff
	var got Message
	if err := got.ReadFrom(messageWithSABody(t, ikeProposalWith(TransformTypeENCR, 12, over))); !errors.Is(err, ErrTruncated) {
		t.Errorf("ReadFrom(message whose TLV attribute runs past its transform) = %v, want ErrTruncated", err)
	}
}

// VALIDATES: the scope of RFC 7296 Section 3.3.5. The Key Length rule names ENCR_DES,
// ENCR_IDEA, and the Type 2 and Type 3 transforms. It never names Type 4
// (Diffie-Hellman Group) or Type 5 (Extended Sequence Numbers), so the attribute on
// either of those is not a MUST NOT violation and the payload parses.
// PREVENTS: a predicate that refuses every carrier except an encryption transform. Ze
// dropped a legal message, and the refusal claimed an obligation the RFC does not state.
func TestPropKeyLengthAllowedOnTypesTheRFCDoesNotName(t *testing.T) {
	cases := []struct {
		name  string
		ttype uint8
		id    uint16
	}{
		{"Diffie-Hellman group 14", TransformTypeDH, 14},
		{"ESN", TransformTypeESN, 1},
	}
	for _, c := range cases {
		var sa PayloadSA
		if err := sa.ReadFrom(ikeProposalWith(c.ttype, c.id, tvAttr(AttrTypeKeyLength, 128))); err != nil {
			t.Errorf("ReadFrom(Key Length on %s) = %v, want a parsed payload", c.name, err)
			continue
		}
		if n := len(sa.Proposals[0].Transforms); n != 1 {
			t.Errorf("recovered %d transforms for %s, want 1", n, c.name)
		}
	}
}
