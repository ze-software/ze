// VALIDATES: the decrypted inner payload chain applies the same RFC 7296 payload rules as
// the outer message chain. ParsePayloadChain (chain.go:10) is a second parser. It rejects an
// unrecognized critical payload and demotes a non-critical one, exactly as Message.ReadFrom
// does. Each test here carries an `RFC requirement:` tag for the row it proves on this path.
// PREVENTS: a row that reads as proven while half its production surface is untested. After
// IKE_SA_INIT almost every IKEv2 payload arrives inside an Encrypted payload. The inner
// parser therefore carries most of the traffic. A change that drops the reject from
// chain.go alone would leave every outer-path test green.
package wire

import (
	"bytes"
	"errors"
	"testing"
)

// innerChainEntry is one payload in a raw inner chain.
type innerChainEntry struct {
	ptype    uint8
	critical bool
	body     []byte
}

// buildInnerChain encodes entries as a bare payload chain, with no IKE header. It returns
// the chain bytes and the payload type of the first entry, which is what ParsePayloadChain
// takes as firstType. The Next Payload field of each generic header names the type that
// follows it, and the last one is zero.
func buildInnerChain(t *testing.T, entries []innerChainEntry) ([]byte, uint8) {
	t.Helper()
	if len(entries) == 0 {
		t.Fatal("an inner chain needs at least one payload")
	}
	total := 0
	for i := range entries {
		total += GenericHeaderLen + len(entries[i].body)
	}
	buf := make([]byte, total)
	off := 0
	for i := range entries {
		next := uint8(0)
		if i+1 < len(entries) {
			next = entries[i+1].ptype
		}
		gh := GenericHeader{
			NextPayload: next,
			Critical:    entries[i].critical,
			Length:      uint16(GenericHeaderLen + len(entries[i].body)),
		}
		off += gh.WriteTo(buf, off)
		copy(buf[off:], entries[i].body)
		off += len(entries[i].body)
	}
	return buf, entries[0].ptype
}

// nonceBody returns a Nonce payload body of n octets.
func nonceBody(n int) []byte {
	body := make([]byte, n)
	for i := range body {
		body[i] = byte(i + 1)
	}
	return body
}

// RFC requirement: RFC7296-2.5-9 positive -- the inner chain rejects an unrecognized critical
// payload. ParsePayloadChain returns ErrUnsupportedCrit (chain.go:38-40). Message.ReadFrom
// makes the same refusal on the outer chain (message.go:123-125). A critical payload
// arrives on this path, because it sits inside the Encrypted payload.
// RFC requirement: RFC7296-2.5-9 negative -- the inner refusal needs both conditions. The same
// unrecognized type parses when the critical flag is clear, so the parser does not reject
// on the type alone.
func TestInnerChainRejectsCriticalUnrecognized(t *testing.T) {
	const undefinedType uint8 = 203

	raw, first := buildInnerChain(t, []innerChainEntry{
		{ptype: undefinedType, critical: true, body: []byte{1, 2, 3, 4}},
	})
	if _, err := ParsePayloadChain(raw, first); !errors.Is(err, ErrUnsupportedCrit) {
		t.Errorf("ParsePayloadChain on a critical unrecognized payload = %v, want ErrUnsupportedCrit", err)
	}

	// Negative: the same type without the critical flag is accepted.
	ok, first := buildInnerChain(t, []innerChainEntry{
		{ptype: undefinedType, body: []byte{1, 2, 3, 4}},
	})
	got, err := ParsePayloadChain(ok, first)
	if err != nil {
		t.Fatalf("ParsePayloadChain on a non-critical unrecognized payload = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("recovered %d payloads, want 1", len(got))
	}
}

// RFC requirement: RFC7296-2.5-11 positive -- the inner chain ignores an unsupported payload whose
// critical flag is clear. ParsePayloadChain stores it as PayloadRaw (chain.go:41). The
// parser then continues, so it still recovers the payload that follows.
// RFC requirement: RFC7296-2.5-11 negative -- the ignore does not swallow the rest of the chain. A
// known Nonce after the ignored payload decodes to its concrete type. The parser resumed
// rather than stopped.
func TestInnerChainIgnoresNonCriticalUnsupported(t *testing.T) {
	const undefinedType uint8 = 204
	body := []byte{9, 8, 7, 6}
	nonce := nonceBody(32)

	raw, first := buildInnerChain(t, []innerChainEntry{
		{ptype: undefinedType, body: body},
		{ptype: PayloadTypeNonce, body: nonce},
	})
	got, err := ParsePayloadChain(raw, first)
	if err != nil {
		t.Fatalf("ParsePayloadChain: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("recovered %d payloads, want 2; the parser must resume after an ignored payload", len(got))
	}
	rawPayload, ok := got[0].Payload.(*payloadRaw)
	if !ok {
		t.Fatalf("payload 0 = %T, want *PayloadRaw", got[0].Payload)
	}
	if rawPayload.PayloadType != undefinedType {
		t.Errorf("PayloadRaw type = %d, want %d", rawPayload.PayloadType, undefinedType)
	}
	if !bytes.Equal(rawPayload.Data, body) {
		t.Errorf("PayloadRaw data = %x, want %x", rawPayload.Data, body)
	}
	if got[0].Critical {
		t.Error("the ignored payload is reported critical")
	}

	// Negative: the payload after the ignored one is decoded, not demoted.
	recovered, ok := got[1].Payload.(*PayloadNonce)
	if !ok {
		t.Fatalf("payload 1 = %T, want *PayloadNonce; the ignore must not swallow the rest", got[1].Payload)
	}
	if !bytes.Equal(recovered.NonceData, nonce) {
		t.Errorf("recovered Nonce = %x, want %x", recovered.NonceData, nonce)
	}
}

// RFC requirement: RFC7296-2.5-8 positive -- the inner chain skips an undefined payload type. It
// does not interpret the contents. ParsePayloadChain records PayloadRaw with the body
// verbatim (chain.go:41), then continues to the next payload.
// RFC requirement: RFC7296-2.5-8 negative -- the skip is confined to undefined types. A defined
// type in the same chain decodes to its concrete Go type. It is never demoted to
// PayloadRaw.
func TestInnerChainSkipsUndefinedType(t *testing.T) {
	const undefinedType uint8 = 205
	opaque := []byte{0xde, 0xad, 0xbe, 0xef}

	raw, first := buildInnerChain(t, []innerChainEntry{
		{ptype: PayloadTypeNonce, body: nonceBody(16)},
		{ptype: undefinedType, body: opaque},
	})
	got, err := ParsePayloadChain(raw, first)
	if err != nil {
		t.Fatalf("ParsePayloadChain: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("recovered %d payloads, want 2", len(got))
	}
	if _, ok := got[0].Payload.(*PayloadNonce); !ok {
		t.Errorf("payload 0 = %T, want *PayloadNonce; a defined type must not be demoted", got[0].Payload)
	}
	skipped, ok := got[1].Payload.(*payloadRaw)
	if !ok {
		t.Fatalf("payload 1 = %T, want *PayloadRaw", got[1].Payload)
	}
	if !bytes.Equal(skipped.Data, opaque) {
		t.Errorf("skipped body = %x, want %x verbatim", skipped.Data, opaque)
	}
}

// RFC requirement: RFC7296-3.2-2 positive -- the inner parser ignores the critical bit for a type
// it understands. A Nonce marked critical parses to *PayloadNonce and returns no error.
// chain.go consults gh.Critical only inside the ErrUnknownPayload branch (chain.go:35-40).
// RFC requirement: RFC7296-3.2-2 negative -- the parser honors the bit for a type it does NOT
// understand. The rule is scoped to recognized types. It is not universal.
func TestInnerChainIgnoresCriticalOnKnownType(t *testing.T) {
	nonce := nonceBody(24)
	raw, first := buildInnerChain(t, []innerChainEntry{
		{ptype: PayloadTypeNonce, critical: true, body: nonce},
	})
	got, err := ParsePayloadChain(raw, first)
	if err != nil {
		t.Fatalf("ParsePayloadChain on a critical but recognized payload = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("recovered %d payloads, want 1", len(got))
	}
	recovered, ok := got[0].Payload.(*PayloadNonce)
	if !ok {
		t.Fatalf("payload 0 = %T, want *PayloadNonce", got[0].Payload)
	}
	if !bytes.Equal(recovered.NonceData, nonce) {
		t.Errorf("recovered Nonce = %x, want %x", recovered.NonceData, nonce)
	}
	if !got[0].Critical {
		t.Error("the critical bit was dropped from the parsed entry; it is reported, then ignored")
	}

	// Negative: an unrecognized type with the same bit set is refused.
	const undefinedType uint8 = 206
	bad, first := buildInnerChain(t, []innerChainEntry{
		{ptype: undefinedType, critical: true, body: []byte{1, 2, 3, 4}},
	})
	if _, err := ParsePayloadChain(bad, first); !errors.Is(err, ErrUnsupportedCrit) {
		t.Errorf("ParsePayloadChain on a critical unrecognized payload = %v, want ErrUnsupportedCrit; "+
			"the bit is ignored only for types the parser understands", err)
	}
}
