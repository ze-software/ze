// VALIDATES: the RFC 7296 MUST-level obligations the IKEv2 codec discharges on its own.
// The list covers message size (§2), and reserved fields and forward compatibility (§2.5).
// It covers the payload chain and critical-bit handling (§2.5, §3.2), and the Encrypted
// payload's terminal position (§3.1). It covers proposal ordering and internal length
// consistency (§3.3), and nonce bounds (§2.10, §3.9). Last, it covers notify and Delete SPI
// shape (§3.10, §3.11), and Vendor ID handling (§3.12). Every test carries an
// `RFC requirement:` tag binding it to its checklist id.
// PREVENTS: a codec change that silently drops one of these invariants -- a reserved bit
// that starts carrying data, an unknown critical payload that stops being refused, or a
// Delete payload that starts mixing protocol identifiers -- from reaching a peer.
package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// buildChain encodes a payload chain (header + payloads) and returns the wire bytes.
func buildChain(t *testing.T, hdr Header, payloads []PayloadEntry) []byte {
	t.Helper()
	msg := Message{Header: hdr, Payloads: payloads}
	buf := make([]byte, msg.Len()+64)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		t.Fatalf("CheckedWriteTo: %v", err)
	}
	return buf[:n]
}

// testHeader is a minimal well-formed IKE_SA_INIT request header.
func testHeader() Header {
	return Header{
		InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		MajorVersion: 2,
		MinorVersion: 0,
		ExchangeType: ExchangeIKESAInit,
		Flags:        FlagInitiator,
	}
}

// nonceOf returns a Nonce payload of the given length.
func nonceOf(n int) *PayloadNonce {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(i + 1)
	}
	return &PayloadNonce{NonceData: data}
}

// RFC requirement: RFC7296-2-1 positive -- a message of at least 1280 octets is sent and
// received whole. Message.WriteTo encodes it (message.go:24), and the header Length field
// records the full size. Message.ReadFrom recovers every payload (message.go:84).
// MaxMsgSize is 3000 (transport/udp.go:14), so the transport read buffer holds it.
// RFC requirement: RFC7296-2-1 negative -- the 1280-octet capability is not a blanket
// accept-by-size. A message whose header Length claims more than the datagram carries is
// refused with ErrLengthMismatch (message.go:88-90), not short-parsed.
func TestMessageHandles1280Octets(t *testing.T) {
	// A Nonce is capped at 256 octets, so reach 1280 with several payloads.
	payloads := []PayloadEntry{
		{Payload: &PayloadSA{Proposals: []Proposal{{
			Number: 1, ProtocolID: ProtocolIKE,
			Transforms: []Transform{{Type: TransformTypeENCR, ID: 12}},
		}}}},
		{Payload: &PayloadKE{DHGroup: 14, KeyExchangeData: make([]byte, 256)}},
		{Payload: nonceOf(256)},
		{Payload: &PayloadVendorID{VendorIDData: make([]byte, 256)}},
		{Payload: &PayloadAUTH{AuthMethod: 14, AuthData: make([]byte, 256)}},
		{Payload: &PayloadRaw{PayloadType: PayloadTypeCERT, Data: make([]byte, 256)}},
	}
	raw := buildChain(t, testHeader(), payloads)
	if len(raw) < 1280 {
		t.Fatalf("built message is %d octets, want at least 1280", len(raw))
	}

	var got Message
	if err := got.ReadFrom(raw); err != nil {
		t.Fatalf("ReadFrom on a %d-octet message: %v", len(raw), err)
	}
	if int(got.Header.Length) != len(raw) {
		t.Errorf("header Length = %d, want %d (the full message)", got.Header.Length, len(raw))
	}
	if len(got.Payloads) != len(payloads) {
		t.Fatalf("recovered %d payloads, want %d", len(got.Payloads), len(payloads))
	}
	if n, ok := got.Payloads[2].Payload.(*PayloadNonce); !ok || len(n.NonceData) != 256 {
		t.Error("the 256-octet Nonce did not survive a >=1280-octet message")
	}

	// Negative: a Length larger than the data is refused, not short-parsed.
	over := append([]byte(nil), raw...)
	binary.BigEndian.PutUint32(over[24:], uint32(len(raw)+1))
	var bad Message
	if err := bad.ReadFrom(over); !errors.Is(err, ErrLengthMismatch) {
		t.Errorf("ReadFrom on an over-declared Length = %v, want ErrLengthMismatch", err)
	}
}

// RFC requirement: RFC7296-2.5-1 positive -- the minor version is ignored. Header.ReadFrom splits
// octet 17 (header.go:60-61), so a peer that announces minor version 5 still parses as
// major 2. No parse decision reads MinorVersion, so the payload chain is recovered
// identically to a minor-0 message.
// RFC requirement: RFC7296-2.5-1 negative -- the ignore is confined to the minor nibble. A change
// to the MAJOR nibble of the same octet does change MajorVersion, so a version change that
// matters is still observable.
func TestMinorVersionIgnoredMajorIsNot(t *testing.T) {
	raw := buildChain(t, testHeader(), []PayloadEntry{{Payload: nonceOf(32)}})

	var base Message
	if err := base.ReadFrom(raw); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	// Positive: minor version 5 changes nothing but MinorVersion itself.
	minor5 := append([]byte(nil), raw...)
	minor5[17] = (2 << 4) | 5
	var got Message
	if err := got.ReadFrom(minor5); err != nil {
		t.Fatalf("ReadFrom with minor version 5: %v", err)
	}
	if got.Header.MajorVersion != 2 {
		t.Errorf("major version = %d, want 2 (the minor nibble must not bleed into it)", got.Header.MajorVersion)
	}
	if got.Header.MinorVersion != 5 {
		t.Errorf("minor version = %d, want 5 (parsed, then ignored)", got.Header.MinorVersion)
	}
	if len(got.Payloads) != len(base.Payloads) {
		t.Errorf("minor version 5 changed the payload count: %d, want %d", len(got.Payloads), len(base.Payloads))
	}

	// Negative: the major nibble is not ignored.
	major3 := append([]byte(nil), raw...)
	major3[17] = 3 << 4
	var other Message
	if err := other.ReadFrom(major3); err != nil {
		t.Fatalf("ReadFrom with major version 3: %v", err)
	}
	if other.Header.MajorVersion != 3 {
		t.Errorf("major version = %d, want 3; the major nibble must stay observable so a "+
			"version mismatch can be acted on", other.Header.MajorVersion)
	}
}

// RFC requirement: RFC7296-2.5-6 positive -- every RESERVED field is sent as zero.
// GenericHeader.WriteTo writes octet 1 as 0 for a non-critical payload (payload.go:72-80).
// Transform.WriteTo zeroes its two reserved octets (payload_sa.go:42-53), and Proposal.WriteTo
// zeroes its reserved octet (payload_sa.go:109-121). PayloadKE.WriteTo zeroes its 2-octet
// reserved field (payload_ke.go:15-19), and PayloadTS.WriteTo zeroes its three reserved
// octets (payload_ts.go:87-91).
// RFC requirement: RFC7296-2.5-6 negative -- the critical bit sets ONLY bit 7 (0x80). It leaves the
// seven reserved bits of that octet zero, so the reserved region is never used as a spare
// flag field.
func TestReservedFieldsSentAsZero(t *testing.T) {
	ts := &PayloadTS{TSPayloadType: PayloadTypeTSi, TrafficSelectors: []TrafficSelector{{
		TSType: TSTypeIPv4AddrRange, StartPort: 0, EndPort: 65535,
		StartAddress: []byte{10, 0, 0, 0}, EndAddress: []byte{10, 0, 0, 255},
	}}}
	// PayloadTS reserved octets are body[1..3].
	tsBuf := make([]byte, ts.Len())
	ts.WriteTo(tsBuf, 0)
	if tsBuf[1] != 0 || tsBuf[2] != 0 || tsBuf[3] != 0 {
		t.Errorf("TS payload reserved octets = %02x %02x %02x, want all zero", tsBuf[1], tsBuf[2], tsBuf[3])
	}

	ke := &PayloadKE{DHGroup: 14, KeyExchangeData: []byte{1, 2, 3, 4}}
	keBuf := make([]byte, ke.Len())
	ke.WriteTo(keBuf, 0)
	if keBuf[2] != 0 || keBuf[3] != 0 {
		t.Errorf("KE payload reserved octets = %02x %02x, want zero", keBuf[2], keBuf[3])
	}

	prop := Proposal{Number: 1, ProtocolID: ProtocolIKE, Transforms: []Transform{
		{Type: TransformTypeENCR, ID: 12, Attrs: []TransformAttr{{Type: AttrTypeKeyLength, Value: 256}}},
	}}
	pBuf := make([]byte, prop.length())
	prop.WriteTo(pBuf, 0)
	if pBuf[1] != 0 {
		t.Errorf("proposal reserved octet = %02x, want zero", pBuf[1])
	}
	// Transform body starts at proposalHeaderLen (no SPI): reserved at +1 and +5.
	trOff := proposalHeaderLen
	if pBuf[trOff+1] != 0 {
		t.Errorf("transform reserved octet 1 = %02x, want zero", pBuf[trOff+1])
	}
	if pBuf[trOff+5] != 0 {
		t.Errorf("transform reserved octet 5 = %02x, want zero", pBuf[trOff+5])
	}

	// Generic header reserved bits.
	var gh GenericHeader
	ghBuf := make([]byte, GenericHeaderLen)
	gh.NextPayload = PayloadTypeNonce
	gh.Length = 8
	gh.WriteTo(ghBuf, 0)
	if ghBuf[1] != 0 {
		t.Errorf("generic header flags octet = %02x, want 0x00 for a non-critical payload", ghBuf[1])
	}

	// Negative: the critical bit occupies bit 7 alone. The reserved bits stay zero.
	gh.Critical = true
	gh.WriteTo(ghBuf, 0)
	if ghBuf[1] != 0x80 {
		t.Errorf("critical generic header flags octet = %02x, want 0x80 (bit 7 only, seven "+
			"reserved bits still zero)", ghBuf[1])
	}
}

// RFC requirement: RFC7296-2.5-7 positive -- the content of a RESERVED field is ignored on
// receipt. GenericHeader.ReadFrom reads only bit 7 of the flags octet (payload.go:83-91).
// A peer that fills the seven reserved bits parses to the same Critical flag, the same
// Length, and the same payload contents.
// RFC requirement: RFC7296-2.5-7 negative -- the ignore is scoped to the reserved bits. A flip of
// bit 7, the defined critical bit, DOES change the parse. The parser does not discard the
// whole octet.
func TestReservedFieldsIgnoredOnReceipt(t *testing.T) {
	raw := buildChain(t, testHeader(), []PayloadEntry{{Payload: nonceOf(32)}})

	var clean Message
	if err := clean.ReadFrom(raw); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	cleanNonce, ok := clean.Payloads[0].Payload.(*PayloadNonce)
	if !ok {
		t.Fatal("first payload is not a Nonce")
	}

	// The first payload's generic header flags octet sits at HeaderLen+1.
	flagsOff := HeaderLen + 1

	// Positive: all seven reserved bits set, critical bit clear.
	noisy := append([]byte(nil), raw...)
	noisy[flagsOff] = 0x7f
	var got Message
	if err := got.ReadFrom(noisy); err != nil {
		t.Fatalf("ReadFrom with reserved bits set: %v", err)
	}
	if got.Payloads[0].Critical {
		t.Error("reserved bits set Critical; only bit 7 may")
	}
	gotNonce, ok := got.Payloads[0].Payload.(*PayloadNonce)
	if !ok {
		t.Fatal("first payload is not a Nonce after setting the reserved bits")
	}
	if !bytes.Equal(gotNonce.NonceData, cleanNonce.NonceData) {
		t.Error("reserved bits changed the recovered Nonce data")
	}

	// Negative: bit 7 is not reserved and is not ignored.
	crit := append([]byte(nil), raw...)
	crit[flagsOff] = 0x80
	var critMsg Message
	if err := critMsg.ReadFrom(crit); err != nil {
		t.Fatalf("ReadFrom with the critical bit set on a known payload: %v", err)
	}
	if !critMsg.Payloads[0].Critical {
		t.Error("the critical bit was ignored; it is a defined bit, not a reserved one")
	}
}

// RFC requirement: RFC7296-2.5-8 positive -- an undefined payload type is skipped and its
// contents are ignored. decodePayload reports ErrUnknownPayload (payload.go:150-151).
// ReadFrom then stores it as PayloadRaw and leaves the body unread (message.go:126). The
// chain continues to the payload that follows.
// RFC requirement: RFC7296-2.5-8 negative -- the skip is confined to undefined types. A
// DEFINED payload type decodes to its concrete type, and is never demoted to PayloadRaw.
func TestUndefinedPayloadTypeSkipped(t *testing.T) {
	const undefinedType uint8 = 200
	raw := buildChain(t, testHeader(), []PayloadEntry{
		{Payload: &PayloadRaw{PayloadType: undefinedType, Data: []byte{0xde, 0xad, 0xbe, 0xef}}},
		{Payload: nonceOf(32)},
	})

	var got Message
	if err := got.ReadFrom(raw); err != nil {
		t.Fatalf("ReadFrom with an undefined payload type: %v", err)
	}
	if len(got.Payloads) != 2 {
		t.Fatalf("recovered %d payloads, want 2 (the undefined one is skipped, not fatal)", len(got.Payloads))
	}
	skipped, ok := got.Payloads[0].Payload.(*PayloadRaw)
	if !ok {
		t.Fatalf("undefined payload decoded to %T, want *PayloadRaw", got.Payloads[0].Payload)
	}
	if skipped.PayloadType != undefinedType {
		t.Errorf("skipped payload type = %d, want %d", skipped.PayloadType, undefinedType)
	}
	if _, ok := got.Payloads[1].Payload.(*PayloadNonce); !ok {
		t.Errorf("the payload AFTER the undefined one decoded to %T, want *PayloadNonce; "+
			"the chain must continue past a skipped payload", got.Payloads[1].Payload)
	}

	// Negative: a defined type is never demoted to PayloadRaw.
	defined := buildChain(t, testHeader(), []PayloadEntry{{Payload: nonceOf(32)}})
	var known Message
	if err := known.ReadFrom(defined); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if _, isRaw := known.Payloads[0].Payload.(*PayloadRaw); isRaw {
		t.Error("a defined payload type was stored as PayloadRaw; the skip path must not " +
			"swallow types this version understands")
	}
}

// RFC requirement: RFC7296-2.5-9 positive -- an unrecognized payload type with the critical
// flag set rejects the whole message. ReadFrom returns ErrUnsupportedCrit
// (message.go:122-125).
// RFC requirement: RFC7296-2.5-9 negative -- the rejection needs BOTH conditions. The same
// unrecognized type with the critical flag clear is accepted. A critical bit on a type
// this version DOES understand never rejects a conforming message.
func TestCriticalUnrecognizedPayloadRejected(t *testing.T) {
	const undefinedType uint8 = 201

	// Positive: critical + unrecognized rejects.
	raw := buildChain(t, testHeader(), []PayloadEntry{
		{Payload: &PayloadRaw{PayloadType: undefinedType, Data: []byte{1, 2, 3, 4}}, Critical: true},
	})
	var got Message
	if err := got.ReadFrom(raw); !errors.Is(err, ErrUnsupportedCrit) {
		t.Errorf("ReadFrom on a critical unrecognized payload = %v, want ErrUnsupportedCrit", err)
	}

	// Negative: critical set on a RECOGNIZED type must not reject.
	known := buildChain(t, testHeader(), []PayloadEntry{{Payload: nonceOf(32), Critical: true}})
	var ok Message
	if err := ok.ReadFrom(known); err != nil {
		t.Errorf("ReadFrom on a critical but recognized payload = %v, want nil; the reject "+
			"needs the type to be unrecognized too", err)
	}
}

// RFC requirement: RFC7296-2.5-11 positive -- an unsupported payload type whose critical
// flag is clear is ignored. ReadFrom demotes it to PayloadRaw (message.go:126) and
// processes the message, so a future extension cannot break an existing implementation.
// RFC requirement: RFC7296-2.5-11 negative -- the ignore is conditional on a clear critical flag.
// With the flag set the same payload is refused (ErrUnsupportedCrit), so the ignore path
// cannot swallow a payload the sender marked as mandatory.
func TestNonCriticalUnsupportedPayloadIgnored(t *testing.T) {
	const undefinedType uint8 = 202
	body := []byte{9, 8, 7, 6}

	raw := buildChain(t, testHeader(), []PayloadEntry{
		{Payload: &PayloadRaw{PayloadType: undefinedType, Data: body}},
		{Payload: nonceOf(16)},
	})
	var got Message
	if err := got.ReadFrom(raw); err != nil {
		t.Fatalf("ReadFrom on a non-critical unsupported payload: %v", err)
	}
	if len(got.Payloads) != 2 {
		t.Fatalf("recovered %d payloads, want 2", len(got.Payloads))
	}
	if got.Payloads[0].Critical {
		t.Error("the ignored payload is reported critical")
	}

	// Negative: the same payload with the critical flag set is refused.
	crit := buildChain(t, testHeader(), []PayloadEntry{
		{Payload: &PayloadRaw{PayloadType: undefinedType, Data: body}, Critical: true},
		{Payload: nonceOf(16)},
	})
	var refused Message
	if err := refused.ReadFrom(crit); !errors.Is(err, ErrUnsupportedCrit) {
		t.Errorf("ReadFrom = %v, want ErrUnsupportedCrit; the ignore must be conditional "+
			"on a clear critical flag", err)
	}
}

// RFC requirement: RFC7296-2.5-13 positive -- a message is not rejected for payload order.
// ReadFrom walks the chain by each generic header's Next Payload field (message.go:95-136),
// never by expected position. A Nonce/KE/SA order therefore parses with all three recovered.
// RFC requirement: RFC7296-2.5-13 negative -- acceptance of any order is not acceptance of anything.
// A chain whose generic header declares a length below the 4-octet header is still refused
// with ErrPayloadTooShort (message.go:109-111).
func TestPayloadOrderNotRejected(t *testing.T) {
	sa := &PayloadSA{Proposals: []Proposal{{
		Number: 1, ProtocolID: ProtocolIKE,
		Transforms: []Transform{{Type: TransformTypeENCR, ID: 12}},
	}}}
	ke := &PayloadKE{DHGroup: 14, KeyExchangeData: []byte{1, 2, 3, 4}}

	// Unconventional order: Nonce, KE, SA (the RFC's example order is SA, KE, Nonce).
	raw := buildChain(t, testHeader(), []PayloadEntry{
		{Payload: nonceOf(32)},
		{Payload: ke},
		{Payload: sa},
	})
	var got Message
	if err := got.ReadFrom(raw); err != nil {
		t.Fatalf("ReadFrom on an out-of-order payload chain: %v", err)
	}
	if len(got.Payloads) != 3 {
		t.Fatalf("recovered %d payloads, want 3", len(got.Payloads))
	}
	if _, ok := got.Payloads[0].Payload.(*PayloadNonce); !ok {
		t.Errorf("payload 0 = %T, want *PayloadNonce", got.Payloads[0].Payload)
	}
	if _, ok := got.Payloads[1].Payload.(*PayloadKE); !ok {
		t.Errorf("payload 1 = %T, want *PayloadKE", got.Payloads[1].Payload)
	}
	if _, ok := got.Payloads[2].Payload.(*PayloadSA); !ok {
		t.Errorf("payload 2 = %T, want *PayloadSA", got.Payloads[2].Payload)
	}

	// Negative: a malformed generic header is still refused.
	bad := append([]byte(nil), raw...)
	binary.BigEndian.PutUint16(bad[HeaderLen+2:], 3) // length below GenericHeaderLen
	var refused Message
	if err := refused.ReadFrom(bad); !errors.Is(err, ErrPayloadTooShort) {
		t.Errorf("ReadFrom on a payload length of 3 = %v, want ErrPayloadTooShort", err)
	}
}

// RFC requirement: RFC7296-3.2-2 positive -- the critical bit is ignored when the recipient
// understands the payload type. A Nonce marked critical parses to the same *PayloadNonce
// with the same data as an identical uncritical one. The message is accepted.
// RFC requirement: RFC7296-3.2-2 negative -- the bit is honored when the type is NOT understood.
// The same critical flag on an unrecognized type rejects the message, so the bit is not
// discarded unconditionally.
func TestCriticalBitIgnoredForKnownType(t *testing.T) {
	plain := buildChain(t, testHeader(), []PayloadEntry{{Payload: nonceOf(24)}})
	crit := buildChain(t, testHeader(), []PayloadEntry{{Payload: nonceOf(24), Critical: true}})

	var a, b Message
	if err := a.ReadFrom(plain); err != nil {
		t.Fatalf("ReadFrom plain: %v", err)
	}
	if err := b.ReadFrom(crit); err != nil {
		t.Fatalf("ReadFrom critical: %v", err)
	}
	an, ok := a.Payloads[0].Payload.(*PayloadNonce)
	if !ok {
		t.Fatal("plain payload is not a Nonce")
	}
	bn, ok := b.Payloads[0].Payload.(*PayloadNonce)
	if !ok {
		t.Fatal("critical payload is not a Nonce")
	}
	if !bytes.Equal(an.NonceData, bn.NonceData) {
		t.Error("the critical bit changed the recovered Nonce of a type the parser understands")
	}

	// Negative: critical is honored for an unknown type.
	unknown := buildChain(t, testHeader(), []PayloadEntry{
		{Payload: &PayloadRaw{PayloadType: 203, Data: []byte{1, 2, 3, 4}}, Critical: true},
	})
	var refused Message
	if err := refused.ReadFrom(unknown); !errors.Is(err, ErrUnsupportedCrit) {
		t.Errorf("ReadFrom = %v, want ErrUnsupportedCrit; the critical bit must still be "+
			"honored for a type the parser does not understand", err)
	}
}

// RFC requirement: RFC7296-3.2-3 positive -- every payload type this document defines is
// understood. decodePayload has a case for each of the sixteen types declared at
// payload.go:36-51 (payload.go:115-152). Each decodes to its own concrete type rather than
// to PayloadRaw.
// RFC requirement: RFC7296-3.2-3 negative -- a payload type this document does NOT define is
// reported as unknown (ErrUnknownPayload). The "all types understood" claim is therefore
// not produced by a decoder that accepts every number.
func TestAllDefinedPayloadTypesUnderstood(t *testing.T) {
	defined := []uint8{
		PayloadTypeSA, PayloadTypeKE, PayloadTypeIDi, PayloadTypeIDr,
		PayloadTypeCERT, PayloadTypeCERTREQ, PayloadTypeAUTH, PayloadTypeNonce,
		PayloadTypeNotify, PayloadTypeDelete, PayloadTypeVendorID, PayloadTypeTSi,
		PayloadTypeTSr, PayloadTypeSK, PayloadTypeCP, PayloadTypeEAP,
	}
	// Bodies that satisfy each codec's minimum length and validation rules.
	bodies := map[uint8][]byte{
		PayloadTypeSA:       {0, 0, 0, 8, 1, ProtocolIKE, 0, 0},
		PayloadTypeKE:       {0, 14, 0, 0, 1, 2, 3, 4},
		PayloadTypeIDi:      {IDTypeFQDN, 0, 0, 0, 'a'},
		PayloadTypeIDr:      {IDTypeFQDN, 0, 0, 0, 'b'},
		PayloadTypeCERT:     {4, 1, 2, 3},
		PayloadTypeCERTREQ:  {4, 1, 2, 3},
		PayloadTypeAUTH:     {14, 0, 0, 0, 1, 2, 3, 4},
		PayloadTypeNonce:    bytes.Repeat([]byte{7}, NonceMinLen),
		PayloadTypeNotify:   {0, 0, 0, 14},
		PayloadTypeDelete:   {ProtocolESP, 4, 0, 1, 9, 9, 9, 9},
		PayloadTypeVendorID: {1, 2, 3, 4},
		PayloadTypeTSi:      {1, 0, 0, 0, TSTypeIPv4AddrRange, 0, 0, 16, 0, 0, 255, 255, 10, 0, 0, 0, 10, 0, 0, 255},
		PayloadTypeTSr:      {1, 0, 0, 0, TSTypeIPv4AddrRange, 0, 0, 16, 0, 0, 255, 255, 10, 0, 0, 0, 10, 0, 0, 255},
		PayloadTypeSK:       {1, 2, 3, 4},
		PayloadTypeCP:       {1, 0, 0, 0},
		PayloadTypeEAP:      {1, 1, 0, 4},
	}
	for _, ty := range defined {
		body, ok := bodies[ty]
		if !ok {
			t.Fatalf("no test body for defined payload type %d", ty)
		}
		p, err := decodePayload(ty, body)
		if err != nil {
			t.Errorf("decodePayload(%d) = %v, want a decoded payload; every type this "+
				"document defines must be understood", ty, err)
			continue
		}
		if _, isRaw := p.(*PayloadRaw); isRaw {
			t.Errorf("decodePayload(%d) returned PayloadRaw; a defined type must decode to "+
				"its own concrete type", ty)
		}
	}

	// Negative: an undefined type is reported unknown, not silently accepted.
	if _, err := decodePayload(204, []byte{1, 2, 3, 4}); !errors.Is(err, ErrUnknownPayload) {
		t.Errorf("decodePayload(204) = %v, want ErrUnknownPayload", err)
	}
}

// RFC requirement: RFC7296-3.3-3 positive -- multiple proposals keep their sender order.
// PayloadSA.WriteTo emits them in slice order and marks only the last one IsLast
// (payload_sa.go:201-212). PayloadSA.ReadFrom appends them in wire order
// (payload_sa.go:222-247), so the most-preferred proposal stays first end to end.
// RFC requirement: RFC7296-3.3-3 negative -- position carries the order, and the Proposal Num field
// does not. A re-encode of the same proposals with descending numbers still recovers them in
// the order they were written, so a renumbering cannot reorder preference.
func TestProposalOrderPreserved(t *testing.T) {
	mk := func(num uint8, encID uint16) Proposal {
		return Proposal{Number: num, ProtocolID: ProtocolIKE, Transforms: []Transform{
			{Type: TransformTypeENCR, ID: encID},
		}}
	}
	sa := &PayloadSA{Proposals: []Proposal{mk(1, 20), mk(2, 12)}}
	raw := buildChain(t, testHeader(), []PayloadEntry{{Payload: sa}})

	var msg Message
	if err := msg.ReadFrom(raw); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	got, ok := msg.Payloads[0].Payload.(*PayloadSA)
	if !ok {
		t.Fatal("payload is not an SA")
	}
	if len(got.Proposals) != 2 {
		t.Fatalf("recovered %d proposals, want 2", len(got.Proposals))
	}
	if got.Proposals[0].Transforms[0].ID != 20 || got.Proposals[1].Transforms[0].ID != 12 {
		t.Errorf("proposal order = [%d %d], want [20 12] (most preferred first)",
			got.Proposals[0].Transforms[0].ID, got.Proposals[1].Transforms[0].ID)
	}

	// Negative: position, not Proposal Num, carries preference.
	desc := &PayloadSA{Proposals: []Proposal{mk(9, 20), mk(3, 12)}}
	rawDesc := buildChain(t, testHeader(), []PayloadEntry{{Payload: desc}})
	var descMsg Message
	if err := descMsg.ReadFrom(rawDesc); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	gotDesc, ok := descMsg.Payloads[0].Payload.(*PayloadSA)
	if !ok {
		t.Fatal("payload is not an SA")
	}
	if gotDesc.Proposals[0].Number != 9 || gotDesc.Proposals[1].Number != 3 {
		t.Errorf("proposal numbers = [%d %d], want [9 3]; the parser must not sort by "+
			"Proposal Num", gotDesc.Proposals[0].Number, gotDesc.Proposals[1].Number)
	}
}

// RFC requirement: RFC7296-3.3-4 positive -- the SA payload's total length is checked
// against its internal lengths and counts. Proposal.ReadFrom refuses a proposal length
// outside its slice, and an SPI Size that does not fit (payload_sa.go:162-172).
// Transform.ReadFrom refuses a transform length outside its slice (payload_sa.go:78-79).
// RFC requirement: RFC7296-3.3-4 negative -- the check is a bound, not a rejection of every SA.
// A consistent payload whose lengths and counts agree parses successfully.
func TestSAInternalLengthConsistency(t *testing.T) {
	// A proposal declaring length 0x00ff inside an 8-octet body is inconsistent.
	body := []byte{0, 0, 0x00, 0xff, 1, ProtocolIKE, 0, 0}
	var sa PayloadSA
	if err := sa.ReadFrom(body); !errors.Is(err, ErrTruncated) {
		t.Errorf("PayloadSA.ReadFrom with a proposal length past the payload = %v, want ErrTruncated", err)
	}

	// A transform declaring a length past its proposal is inconsistent.
	tooLong := []byte{
		0, 0, 0, 16, 1, ProtocolIKE, 0, 1, // proposal: len 16, one transform
		0, 0, 0x00, 0xff, TransformTypeENCR, 0, 0, 12, // transform: len 255
	}
	var sa2 PayloadSA
	if err := sa2.ReadFrom(tooLong); !errors.Is(err, ErrTruncated) {
		t.Errorf("PayloadSA.ReadFrom with a transform length past the proposal = %v, want ErrTruncated", err)
	}

	// An SPI Size that does not fit in the declared proposal length is inconsistent.
	badSPI := []byte{0, 0, 0, 8, 1, ProtocolESP, 32, 0}
	var sa3 PayloadSA
	if err := sa3.ReadFrom(badSPI); !errors.Is(err, ErrTruncated) {
		t.Errorf("PayloadSA.ReadFrom with an SPI Size past the proposal = %v, want ErrTruncated", err)
	}

	// Negative: a consistent SA payload is accepted.
	good := []byte{
		0, 0, 0, 16, 1, ProtocolIKE, 0, 1,
		0, 0, 0, 8, TransformTypeENCR, 0, 0, 12,
	}
	var ok4 PayloadSA
	if err := ok4.ReadFrom(good); err != nil {
		t.Errorf("PayloadSA.ReadFrom on a consistent payload = %v, want nil", err)
	}
	if len(ok4.Proposals) != 1 || len(ok4.Proposals[0].Transforms) != 1 {
		t.Errorf("consistent payload recovered %d proposals; want 1 with 1 transform", len(ok4.Proposals))
	}
}

// RFC requirement: RFC7296-3.9-1 positive -- Nonce Data of 16 and of 256 octets, the two
// inclusive bounds, are both accepted: PayloadNonce.ReadFrom (payload_nonce.go:25-34) tests
// NonceMinLen and NonceMaxLen (payload_nonce.go:6-9) inclusively.
// RFC requirement: RFC7296-3.9-1 negative -- 15 octets is refused with ErrNonceTooShort and
// 257 with ErrNonceTooLong, so the bound is enforced on both sides rather than assumed.
// RFC requirement: RFC7296-2.10-2 positive -- a nonce is at least 128 bits: the accepted
// minimum, NonceMinLen, is 16 octets, which is exactly 128 bits.
// RFC requirement: RFC7296-2.10-2 negative -- anything under 128 bits is refused: a 15-octet
// nonce (120 bits) returns ErrNonceTooShort.
func TestNonceLengthBounds(t *testing.T) {
	if NonceMinLen*8 != 128 {
		t.Fatalf("NonceMinLen is %d octets (%d bits), want 16 (128 bits)", NonceMinLen, NonceMinLen*8)
	}
	for _, n := range []int{NonceMinLen, 32, NonceMaxLen} {
		var p PayloadNonce
		if err := p.ReadFrom(bytes.Repeat([]byte{1}, n)); err != nil {
			t.Errorf("PayloadNonce.ReadFrom(%d octets) = %v, want nil", n, err)
		}
	}
	var short PayloadNonce
	if err := short.ReadFrom(bytes.Repeat([]byte{1}, NonceMinLen-1)); !errors.Is(err, ErrNonceTooShort) {
		t.Errorf("PayloadNonce.ReadFrom(%d octets) = %v, want ErrNonceTooShort", NonceMinLen-1, err)
	}
	var long PayloadNonce
	if err := long.ReadFrom(bytes.Repeat([]byte{1}, NonceMaxLen+1)); !errors.Is(err, ErrNonceTooLong) {
		t.Errorf("PayloadNonce.ReadFrom(%d octets) = %v, want ErrNonceTooLong", NonceMaxLen+1, err)
	}
}

// RFC requirement: RFC7296-3.10-3 positive -- a notification concerning the IKE SA carries
// SPI Size zero and an empty SPI field. PayloadNotify.WriteTo writes no SPI octets when
// SPISize is 0 (payload_notify.go:53-67). The encoded body is therefore the 4-octet fixed
// part plus the notification data only. Erratum 6940 corrects the RFC's wording here from
// "the field" to "the SPI field". The obligation is unchanged.
//
// RFC requirement: RFC7296-3.10-3 negative -- the zero SPI Size is specific to an IKE-SA
// notification. A Child-SA notification that carries a 4-octet ESP SPI does encode those
// four octets. The empty-SPI rule is not a codec that has dropped SPI support.
func TestNotifyIKESAHasEmptySPI(t *testing.T) {
	ike := &PayloadNotify{NotifyMsgType: NotifyInitialContact}
	buf := make([]byte, ike.Len())
	n := ike.WriteTo(buf, 0)
	if n != 4 {
		t.Errorf("IKE-SA notify body = %d octets, want 4 (fixed part only, no SPI)", n)
	}
	if buf[1] != 0 {
		t.Errorf("IKE-SA notify SPI Size = %d, want 0", buf[1])
	}

	var got PayloadNotify
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("PayloadNotify.ReadFrom: %v", err)
	}
	if got.SPISize != 0 || len(got.SPI) != 0 {
		t.Errorf("recovered SPI Size %d with %d SPI octets, want 0 and 0", got.SPISize, len(got.SPI))
	}

	// Negative: a Child-SA notification does carry its SPI.
	child := &PayloadNotify{
		ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{1, 2, 3, 4},
		NotifyMsgType: NotifyRekeySA,
	}
	cbuf := make([]byte, child.Len())
	cn := child.WriteTo(cbuf, 0)
	if cn != 8 {
		t.Errorf("Child-SA notify body = %d octets, want 8 (fixed part plus a 4-octet SPI)", cn)
	}
	var gotChild PayloadNotify
	if err := gotChild.ReadFrom(cbuf[:cn]); err != nil {
		t.Fatalf("PayloadNotify.ReadFrom: %v", err)
	}
	if !bytes.Equal(gotChild.SPI, []byte{1, 2, 3, 4}) {
		t.Errorf("recovered Child-SA SPI = %x, want 01020304", gotChild.SPI)
	}
}

// RFC requirement: RFC7296-3.11-1 positive -- a Delete payload cannot mix protocol identifiers.
// PayloadDelete carries ONE ProtocolID for the whole payload (payload_delete.go:8-13), and
// its SPIs field is a flat run of equal-width SPIs. Every SPI in a payload is therefore for
// that protocol by construction, and a second protocol needs a second payload.
// RFC requirement: RFC7296-3.11-1 negative -- two protocols in one message are carried as two
// Delete payloads, each with its own ProtocolID. The single-protocol rule therefore does not
// prevent the deletion of an ESP and an AH SA in the same INFORMATIONAL exchange.
func TestDeletePayloadSingleProtocol(t *testing.T) {
	esp := &PayloadDelete{ProtocolID: ProtocolESP, SPISize: 4, NumSPIs: 2, SPIs: []byte{1, 1, 1, 1, 2, 2, 2, 2}}
	buf := make([]byte, esp.Len())
	n := esp.WriteTo(buf, 0)
	if buf[0] != ProtocolESP {
		t.Errorf("Delete Protocol ID = %d, want %d", buf[0], ProtocolESP)
	}
	var got PayloadDelete
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("PayloadDelete.ReadFrom: %v", err)
	}
	if got.ProtocolID != ProtocolESP {
		t.Errorf("recovered Protocol ID = %d, want %d", got.ProtocolID, ProtocolESP)
	}
	if int(got.NumSPIs)*int(got.SPISize) != len(got.SPIs) {
		t.Errorf("recovered %d SPI octets for %d SPIs of %d octets; a Delete payload holds "+
			"one protocol's SPIs at one width", len(got.SPIs), got.NumSPIs, got.SPISize)
	}

	// Negative: two protocols need two payloads, and both survive one message.
	ah := &PayloadDelete{ProtocolID: ProtocolAH, SPISize: 4, NumSPIs: 1, SPIs: []byte{3, 3, 3, 3}}
	raw := buildChain(t, testHeader(), []PayloadEntry{{Payload: esp}, {Payload: ah}})
	var msg Message
	if err := msg.ReadFrom(raw); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if len(msg.Payloads) != 2 {
		t.Fatalf("recovered %d payloads, want 2", len(msg.Payloads))
	}
	protos := make([]uint8, 0, 2)
	for i := range msg.Payloads {
		d, ok := msg.Payloads[i].Payload.(*PayloadDelete)
		if !ok {
			t.Fatalf("payload %d = %T, want *PayloadDelete", i, msg.Payloads[i].Payload)
		}
		protos = append(protos, d.ProtocolID)
	}
	if protos[0] != ProtocolESP || protos[1] != ProtocolAH {
		t.Errorf("Delete protocol IDs = %v, want [%d %d]", protos, ProtocolESP, ProtocolAH)
	}
}

// RFC requirement: RFC7296-3.11-2 positive -- the Delete SPI Size is zero for IKE and four
// for AH and ESP. An IKE Delete encodes SPI Size 0 with no SPI octets (the SPI is in the
// message header). An AH or ESP Delete encodes SPI Size 4 with four octets per SPI.
// PayloadDelete.ReadFrom checks that width as NumSPIs*SPISize (payload_delete.go:35-42).
// RFC requirement: RFC7296-3.11-2 negative -- a declared count and width that the body does not
// carry is refused with ErrTruncated. The width is validated rather than trusted.
func TestDeleteSPISizeByProtocol(t *testing.T) {
	ike := &PayloadDelete{ProtocolID: ProtocolIKE}
	ibuf := make([]byte, ike.Len())
	in := ike.WriteTo(ibuf, 0)
	if in != 4 || ibuf[1] != 0 {
		t.Errorf("IKE Delete = %d octets with SPI Size %d, want 4 octets and SPI Size 0", in, ibuf[1])
	}

	for _, proto := range []uint8{ProtocolAH, ProtocolESP} {
		d := &PayloadDelete{ProtocolID: proto, SPISize: 4, NumSPIs: 1, SPIs: []byte{7, 7, 7, 7}}
		buf := make([]byte, d.Len())
		n := d.WriteTo(buf, 0)
		if buf[1] != 4 {
			t.Errorf("protocol %d Delete SPI Size = %d, want 4", proto, buf[1])
		}
		var got PayloadDelete
		if err := got.ReadFrom(buf[:n]); err != nil {
			t.Fatalf("protocol %d PayloadDelete.ReadFrom: %v", proto, err)
		}
		if len(got.SPIs) != 4 {
			t.Errorf("protocol %d recovered %d SPI octets, want 4", proto, len(got.SPIs))
		}
	}

	// Negative: a declared 4-octet SPI that is not present is refused.
	var short PayloadDelete
	if err := short.ReadFrom([]byte{ProtocolESP, 4, 0, 1}); !errors.Is(err, ErrTruncated) {
		t.Errorf("PayloadDelete.ReadFrom with a missing SPI = %v, want ErrTruncated", err)
	}
}

// RFC requirement: RFC7296-3.12-2 positive -- an unfamiliar Vendor ID is ignored. The payload
// decodes to *PayloadVendorID holding the opaque octets (payload_vendor.go:19-23). No code
// path branches on VendorIDData, and the payloads after it still parse. An unrecognized
// vendor announcement therefore changes no behavior.
// RFC requirement: RFC7296-3.12-3 positive -- a document extending this protocol can announce
// itself. The Vendor ID payload type is defined (payload.go:46) and registered in
// decodePayload (payload.go:138-139), so an extension has the payload the RFC requires.
//
// RFC requirement: RFC7296-3.12-2 negative -- an ignore is not a drop. The payload's octets are
// recovered intact, so an implementation that DID recognize the vendor CAN act on them.
// RFC requirement: RFC7296-3.12-3 negative -- ze itself defines no protocol extension, so it
// announces none. No Vendor ID payload is constructed anywhere outside this codec and its
// tests. The type stays available for a peer's use only.
func TestVendorIDIgnoredButPreserved(t *testing.T) {
	vendor := []byte{0xca, 0xfe, 0xba, 0xbe, 0x01}
	raw := buildChain(t, testHeader(), []PayloadEntry{
		{Payload: &PayloadVendorID{VendorIDData: vendor}},
		{Payload: nonceOf(16)},
	})

	var msg Message
	if err := msg.ReadFrom(raw); err != nil {
		t.Fatalf("ReadFrom with an unfamiliar Vendor ID: %v", err)
	}
	if len(msg.Payloads) != 2 {
		t.Fatalf("recovered %d payloads, want 2; an unfamiliar Vendor ID must not stop the chain", len(msg.Payloads))
	}
	vid, ok := msg.Payloads[0].Payload.(*PayloadVendorID)
	if !ok {
		t.Fatalf("payload 0 = %T, want *PayloadVendorID", msg.Payloads[0].Payload)
	}
	if !bytes.Equal(vid.VendorIDData, vendor) {
		t.Errorf("recovered Vendor ID = %x, want %x; ignoring a vendor must not corrupt its "+
			"octets", vid.VendorIDData, vendor)
	}
	if _, ok := msg.Payloads[1].Payload.(*PayloadNonce); !ok {
		t.Errorf("payload after the Vendor ID = %T, want *PayloadNonce", msg.Payloads[1].Payload)
	}

	// The type is defined and decodable, which is what an extension document needs.
	p, err := decodePayload(PayloadTypeVendorID, vendor)
	if err != nil {
		t.Fatalf("decodePayload(VendorID) = %v, want a decoded payload", err)
	}
	if p.Type() != PayloadTypeVendorID {
		t.Errorf("decoded type = %d, want %d", p.Type(), PayloadTypeVendorID)
	}
}
