// VALIDATES: the deterministic PPP session builders — buildCHAPResponse frames a
// RFC 1994 CHAP Response (code, echoed id, length, digest, trailing name) and
// rejects malformed challenges, extractServerOptions parses the peer's
// auth-protocol/algorithm and MRU LCP options, and generateMagic returns a
// non-zero magic number.
// PREVENTS: a CHAP authentication failure from a mis-framed Response, the peer's
// negotiated auth method or MRU being misread, or a zero magic number being used.

package pppoeclient

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// RFC requirement: RFC1994-4.1-4 positive -- on receiving a Challenge the peer
// builds a CHAP Response packet with Code=2 carrying the MD5 digest and its Name.
// RFC requirement: RFC1994-4.1-6 positive -- the Response Identifier is copied
// from the Challenge Identifier (resp[1] == challenge.Identifier).
func TestBuildCHAPResponse(t *testing.T) {
	challengeValue := []byte{0x01, 0x02, 0x03, 0x04}
	challenge := ppp.LCPPacket{
		Code:       0, // code is irrelevant to the builder
		Identifier: 0x42,
		Data:       append([]byte{byte(len(challengeValue))}, challengeValue...),
	}
	cfg := sessionConfig{username: "user", password: "pass"}

	resp := buildCHAPResponse(challenge, cfg)
	if resp == nil {
		t.Fatal("buildCHAPResponse returned nil for a valid challenge")
	}
	if resp[0] != 2 {
		t.Errorf("code = %d, want 2 (Response)", resp[0])
	}
	if resp[1] != 0x42 {
		t.Errorf("identifier = %#x, want 0x42 (echoed)", resp[1])
	}
	if l := binary.BigEndian.Uint16(resp[2:4]); int(l) != len(resp) {
		t.Errorf("length field = %d, want %d", l, len(resp))
	}
	if int(resp[4]) != 16 {
		t.Errorf("value-size = %d, want 16 (MD5 digest length)", resp[4])
	}
	want := chapMD5Response(challenge.Identifier, cfg.password, challengeValue)
	if !bytes.Equal(resp[5:5+16], want[:]) {
		t.Errorf("digest = %x, want %x", resp[5:5+16], want[:])
	}
	if name := string(resp[5+16:]); name != "user" {
		t.Errorf("trailing name = %q, want %q", name, "user")
	}
}

// RFC requirement: RFC1994-4.1-4 negative -- a malformed Challenge (missing or
// truncated Value) yields no Response: buildCHAPResponse returns nil, so the peer
// does not transmit a Response packet for an invalid Challenge.
func TestBuildCHAPResponseMalformed(t *testing.T) {
	cfg := sessionConfig{username: "u", password: "p"}
	// Empty data: no value-size byte.
	if buildCHAPResponse(ppp.LCPPacket{Identifier: 1}, cfg) != nil {
		t.Error("empty challenge data should return nil")
	}
	// value-size claims 5 bytes but only 2 follow.
	trunc := ppp.LCPPacket{Identifier: 1, Data: []byte{0x05, 0xAA, 0xBB}}
	if buildCHAPResponse(trunc, cfg) != nil {
		t.Error("truncated challenge value should return nil")
	}
}

func TestExtractServerOptions(t *testing.T) {
	opts := []ppp.LCPOption{
		{Type: ppp.LCPOptAuthProto, Data: []byte{0xc2, 0x23, 0x05}}, // CHAP + algorithm byte
		{Type: ppp.LCPOptMRU, Data: []byte{0x05, 0xDC}},             // MRU 1500
	}
	authProto, authData, mru := extractServerOptions(opts)
	if authProto != 0xc223 {
		t.Errorf("authProto = %#x, want 0xc223 (CHAP)", authProto)
	}
	if !bytes.Equal(authData, []byte{0x05}) {
		t.Errorf("authData = %x, want 05 (algorithm byte)", authData)
	}
	if mru != 1500 {
		t.Errorf("mru = %d, want 1500", mru)
	}
}

func TestExtractServerOptionsMalformed(t *testing.T) {
	// Auth-proto with a single byte and an MRU with the wrong length are ignored.
	opts := []ppp.LCPOption{
		{Type: ppp.LCPOptAuthProto, Data: []byte{0xc2}},
		{Type: ppp.LCPOptMRU, Data: []byte{0x05, 0xDC, 0x00}},
	}
	authProto, authData, mru := extractServerOptions(opts)
	if authProto != 0 || authData != nil || mru != 0 {
		t.Errorf("malformed opts parsed: proto %#x data %x mru %d, want all zero", authProto, authData, mru)
	}
}

func TestGenerateMagic(t *testing.T) {
	for range 10 {
		m, err := generateMagic()
		if err != nil {
			t.Fatalf("generateMagic: %v", err)
		}
		if m == 0 {
			t.Fatal("generateMagic returned 0")
		}
	}
}

// RFC requirement: RFC2516-x-9 positive -- the client's LCP Configure-Request proposes an MRU of 1492 (the PPPoE default ceiling), so LCP negotiates MRU at 1492 or lower.
func TestLCPConfigRequestMRU(t *testing.T) {
	var scratch [ppp.MaxFrameLen]byte
	var w bytes.Buffer
	sendLCPConfigRequest(&w, scratch[:], 1, 1492, 0x12345678)

	proto, payload, _, err := ppp.ParseFrame(w.Bytes())
	if err != nil {
		t.Fatalf("ParseFrame: %v", err)
	}
	if proto != ppp.ProtoLCP {
		t.Fatalf("proto = %#x, want LCP", proto)
	}
	pkt, err := ppp.ParseLCPPacket(payload)
	if err != nil {
		t.Fatalf("ParseLCPPacket: %v", err)
	}
	if pkt.Code != ppp.LCPConfigureRequest {
		t.Fatalf("code = %d, want Configure-Request", pkt.Code)
	}
	opts, err := ppp.ParseLCPOptions(pkt.Data)
	if err != nil {
		t.Fatalf("ParseLCPOptions: %v", err)
	}
	var (
		mru   uint16
		found bool
	)
	for _, opt := range opts {
		if opt.Type == ppp.LCPOptMRU && len(opt.Data) == 2 {
			mru = binary.BigEndian.Uint16(opt.Data)
			found = true
		}
	}
	if !found {
		t.Fatal("LCP Configure-Request carries no MRU option")
	}
	if mru > 1492 {
		t.Errorf("proposed MRU = %d, want <= 1492", mru)
	}
	if mru != 1492 {
		t.Errorf("proposed MRU = %d, want 1492 (PPPoE default)", mru)
	}
}
