// Design: plan/learned/805-ipsec-11-interop-eap.md -- EAP peer session tests

package eap

import (
	"encoding/binary"
	"testing"
)

func TestPeerSessionIdentityExchange(t *testing.T) {
	ps := NewPeerSession(TypeMSCHAPv2, "testuser", "testpass")

	identityReq := &Packet{
		Code:       CodeRequest,
		Identifier: 1,
		Type:       TypeIdentity,
	}

	result := ps.Process(identityReq)
	if result.Err != nil {
		t.Fatalf("identity request: %v", result.Err)
	}
	if result.Done {
		t.Fatal("should not be done after identity")
	}
	if result.Response == nil {
		t.Fatal("expected identity response")
	}
	if result.Response.Code != CodeResponse {
		t.Fatalf("expected CodeResponse, got %d", result.Response.Code)
	}
	if result.Response.Identifier != 1 {
		t.Fatalf("expected identifier 1, got %d", result.Response.Identifier)
	}
	if result.Response.Type != TypeIdentity {
		t.Fatalf("expected TypeIdentity, got %d", result.Response.Type)
	}
	if string(result.Response.TypeData) != "testuser" {
		t.Fatalf("expected identity 'testuser', got %q", result.Response.TypeData)
	}
}

func TestPeerSessionMSCHAPv2FullExchange(t *testing.T) {
	password := "TestPassword"
	userName := "testuser"
	ps := NewPeerSession(TypeMSCHAPv2, userName, password)

	// Server creates an authenticator-side session for verification.
	server := &mschapv2Method{password: password}

	// Step 1: Identity request.
	identityReq := &Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}
	result := ps.Process(identityReq)
	if result.Err != nil {
		t.Fatalf("identity: %v", result.Err)
	}

	// Step 2: Server sends MS-CHAPv2 Challenge.
	challengePkt := server.Start(2)
	result = ps.Process(challengePkt)
	if result.Err != nil {
		t.Fatalf("challenge: %v", result.Err)
	}
	if result.Response == nil {
		t.Fatal("expected response to challenge")
	}
	if result.Response.Type != TypeMSCHAPv2 {
		t.Fatalf("expected TypeMSCHAPv2 response, got %d", result.Response.Type)
	}

	// Step 3: Verify the peer's response using the server method.
	serverResult := server.Process(result.Response)
	if serverResult.Err != nil {
		t.Fatalf("server rejected response: %v", serverResult.Err)
	}

	// Step 4: Server sends MS-CHAPv2 Success (opcode 3, carries S= authenticator response).
	if serverResult.Response == nil {
		t.Fatal("expected success from server")
	}
	result = ps.Process(serverResult.Response)
	if result.Err != nil {
		t.Fatalf("success ack: %v", result.Err)
	}
	if result.Response == nil {
		t.Fatal("expected ack response from peer")
	}

	// Step 5: EAP-Success from authenticator (the EAP layer, not the method).
	eapSuccess := &Packet{Code: CodeSuccess, Identifier: 5}
	result = ps.Process(eapSuccess)
	if result.Err != nil {
		t.Fatalf("eap success: %v", result.Err)
	}
	if !result.Done {
		t.Fatal("expected done after EAP-Success")
	}
	if !ps.Succeeded() {
		t.Fatal("peer session should report succeeded")
	}

	// MSK should be non-zero.
	var zeroMSK [64]byte
	if result.MSK == zeroMSK {
		t.Fatal("expected non-zero MSK")
	}
}

func TestPeerSessionEAPFailure(t *testing.T) {
	ps := NewPeerSession(TypeMSCHAPv2, "user", "pass")

	failPkt := &Packet{Code: CodeFailure, Identifier: 1}
	result := ps.Process(failPkt)
	if result.Err == nil {
		t.Fatal("expected error on EAP-Failure")
	}
	if result.Done {
		t.Fatal("should not be done on failure")
	}
}

func TestPeerSessionMaxRounds(t *testing.T) {
	ps := NewPeerSession(TypeMSCHAPv2, "user", "pass")

	req := &Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}
	for i := range maxEAPRounds {
		result := ps.Process(req)
		if result.Err != nil {
			t.Fatalf("round %d unexpected error: %v", i, result.Err)
		}
		ps.state = peerStateIdentity
	}

	result := ps.Process(req)
	if result.Err == nil {
		t.Fatal("expected max rounds error")
	}
}

func TestPeerMSCHAPv2ResponsePacketStructure(t *testing.T) {
	password := "testpassword"
	identity := "testuser"

	server := &mschapv2Method{password: password}
	challengePkt := server.Start(0x48)

	peer := NewPeerSession(TypeMSCHAPv2, identity, password)
	peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity})

	result := peer.Process(&Packet{
		Code:       challengePkt.Code,
		Identifier: challengePkt.Identifier,
		Type:       challengePkt.Type,
		TypeData:   challengePkt.TypeData,
	})
	if result.Err != nil {
		t.Fatalf("challenge: %v", result.Err)
	}

	resp := result.Response
	td := resp.TypeData

	// Verify MSCHAPv2 Response structure per RFC 2759.
	if td[0] != mschapv2OpResponse {
		t.Fatalf("OpCode: got %d, want %d", td[0], mschapv2OpResponse)
	}
	if td[1] != 0x48 {
		t.Fatalf("MS-ID: got %d, want %d", td[1], 0x48)
	}
	msLen := int(td[2])<<8 | int(td[3])
	if msLen != len(td) {
		t.Fatalf("MS-Length: got %d, want %d (actual len)", msLen, len(td))
	}
	if td[4] != 49 {
		t.Fatalf("ValueSize: got %d, want 49", td[4])
	}

	// Reserved bytes [21:29] must be zero.
	for i := 21; i < 29; i++ {
		if td[i] != 0 {
			t.Fatalf("reserved byte at offset %d: got %d, want 0", i, td[i])
		}
	}
	// Flags byte at offset 53 must be zero.
	if td[53] != 0 {
		t.Fatalf("flags: got %d, want 0", td[53])
	}

	// Name field starts at offset 54.
	name := string(td[54:])
	if name != identity {
		t.Fatalf("Name: got %q, want %q", name, identity)
	}

	// Extract peer challenge and NT-Response for verification.
	var peerChallenge [16]byte
	copy(peerChallenge[:], td[5:21])
	var ntResponse [24]byte
	copy(ntResponse[:], td[29:53])

	// Verify using the server's challenge.
	if !VerifyNTResponse(server.authChallenge, peerChallenge, StripDomain(name), password, ntResponse) {
		t.Fatalf("NT-Response verification FAILED\n  authChallenge: %x\n  peerChallenge: %x\n  ntResponse:    %x\n  userName:      %q\n  password:      %q",
			server.authChallenge, peerChallenge, ntResponse, StripDomain(name), password)
	}

	t.Log("NT-Response verified against server challenge")
}

func TestPeerMSCHAPv2ViaEncodeDecodeRoundTrip(t *testing.T) {
	password := "testpassword"
	identity := "testuser"

	server := &mschapv2Method{password: password}
	challengePkt := server.Start(0x48)

	peer := NewPeerSession(TypeMSCHAPv2, identity, password)
	idReq := &Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}
	peer.Process(idReq)

	challengeAsReceived := &Packet{
		Code:       challengePkt.Code,
		Identifier: challengePkt.Identifier,
		Type:       challengePkt.Type,
		TypeData:   challengePkt.TypeData,
	}
	result := peer.Process(challengeAsReceived)
	if result.Err != nil {
		t.Fatalf("challenge: %v", result.Err)
	}

	// Encode the response (as sendEAPResponsePacket does).
	eapData := result.Response.Encode()
	t.Logf("encoded EAP response: %d bytes, code=%d, id=%d", len(eapData), eapData[0], eapData[1])

	// Simulate what buildEAPResponse does: extract fields for wire.PayloadEAP.
	if len(eapData) < 5 {
		t.Fatal("eapData too short")
	}
	wireCode := eapData[0]
	wireID := eapData[1]
	wireEAPData := eapData[4:]

	// Simulate what the receiver does: reconstruct eap.Packet from wire.PayloadEAP.
	// This is what wireEAPToPacket does on the receiving side.
	receivedPkt := &Packet{
		Code:       wireCode,
		Identifier: wireID,
	}
	if len(wireEAPData) > 0 {
		receivedPkt.Type = wireEAPData[0]
		if len(wireEAPData) > 1 {
			receivedPkt.TypeData = wireEAPData[1:]
		}
	}

	// Feed to server.
	serverResult := server.Process(receivedPkt)
	if serverResult.Err != nil {
		t.Fatalf("server REJECTED after encode/decode: %v", serverResult.Err)
	}
	t.Logf("server accepted after encode/decode round-trip")
}

func TestPeerMSCHAPv2ViaWireFormat(t *testing.T) {
	password := "testpassword"
	identity := "testuser"

	// Simulate server side: create a challenge.
	server := &mschapv2Method{password: password}
	challengePkt := server.Start(0x48) // MS-ID = 0x48 like strongSwan

	// Simulate wire encoding: challengePkt.TypeData is the raw MSCHAPv2 data.
	// In IKE, this would be inside wire.PayloadEAP with EAPData = [Type=26, TypeData...].
	// wireEAPToPacket extracts: pkt.Type = 26, pkt.TypeData = MSCHAPv2 data.
	// So the peer sees: req.Type = TypeMSCHAPv2, req.TypeData = challengePkt.TypeData.

	// Create peer and process Identity first.
	peer := NewPeerSession(TypeMSCHAPv2, identity, password)
	idReq := &Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}
	idResult := peer.Process(idReq)
	if idResult.Err != nil {
		t.Fatalf("identity: %v", idResult.Err)
	}

	// Feed the challenge to the peer as if it came through wireEAPToPacket.
	challengeAsReceived := &Packet{
		Code:       challengePkt.Code,
		Identifier: challengePkt.Identifier,
		Type:       challengePkt.Type,
		TypeData:   challengePkt.TypeData,
	}
	result := peer.Process(challengeAsReceived)
	if result.Err != nil {
		t.Fatalf("challenge: %v", result.Err)
	}
	if result.Response == nil {
		t.Fatal("expected response to challenge")
	}

	// Now simulate what happens on the wire: the response is encoded, then
	// the server receives it. On the server side, it sees TypeData directly.
	// Verify the server can verify the peer's response.
	serverResult := server.Process(result.Response)
	if serverResult.Err != nil {
		t.Fatalf("server REJECTED peer response: %v", serverResult.Err)
	}

	t.Logf("server accepted response (MS-CHAPv2 verification passed)")
}

func TestMSCHAPv2PeerChallengeRandom16(t *testing.T) {
	newPeerChallenge := func() [16]byte {
		server := &mschapv2Method{password: "pw"}
		challengePkt := server.Start(0x10)
		peer := NewPeerSession(TypeMSCHAPv2, "user", "pw")
		peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity})
		res := peer.Process(&Packet{
			Code:       challengePkt.Code,
			Identifier: challengePkt.Identifier,
			Type:       challengePkt.Type,
			TypeData:   challengePkt.TypeData,
		})
		if res.Err != nil {
			t.Fatalf("peer challenge: %v", res.Err)
		}
		return peer.peerChallenge
	}

	// RFC requirement: RFC2759-x-9 positive -- the peer fills its 16-octet Peer-Challenge
	// from crypto/rand: the field is exactly 16 octets, not all-zero, and different in two
	// independent sessions.
	pc1 := newPeerChallenge()
	pc2 := newPeerChallenge()
	if len(pc1) != 16 {
		t.Fatalf("peerChallenge length: got %d, want 16", len(pc1))
	}
	var zero [16]byte
	if pc1 == zero {
		t.Fatal("peerChallenge is all-zero; not randomized")
	}
	if pc1 == pc2 {
		t.Fatal("two sessions produced identical Peer-Challenge")
	}
}

func TestTLSFragmenterRoundTrip(t *testing.T) {
	// RFC requirement: RFC5216-2.1.5-1 positive -- a TLS message larger than one
	// EAP-TLS fragment is split into multiple fragments on send and reassembled
	// byte-for-byte on receive (RFC 5216 Section 2.1.5 fragmentation).
	data := make([]byte, 3000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	var sender tlsFragmenter
	sender.startSending(data)

	var receiver tlsFragmenter
	var fragments int

	for {
		td := sender.nextFragment()
		fragments++
		if fragments > 10 {
			t.Fatal("too many fragments")
		}

		if err := receiver.reassemble(td); err != nil {
			t.Fatalf("reassemble fragment %d: %v", fragments, err)
		}

		flags := td[0]
		if flags&eapTLSFlagM == 0 {
			break
		}
	}

	reassembled := receiver.drainReassembled()
	if len(reassembled) != len(data) {
		t.Fatalf("reassembled %d bytes, want %d", len(reassembled), len(data))
	}
	for i := range data {
		if reassembled[i] != data[i] {
			t.Fatalf("mismatch at byte %d: got %d, want %d", i, reassembled[i], data[i])
		}
	}

	if fragments < 3 {
		t.Fatalf("expected at least 3 fragments for 3000 bytes at 1024/fragment, got %d", fragments)
	}
}

func TestTLSFragmenterFirstFragmentHasLength(t *testing.T) {
	// RFC requirement: RFC5216-3-1 positive -- the FIRST fragment of a
	// multi-fragment TLS message carries the L (Length included) bit and the
	// 4-octet TLS Message Length (RFC 5216 Section 3, Section 2.1.5).
	data := make([]byte, 2000)

	var f tlsFragmenter
	f.startSending(data)

	td := f.nextFragment()

	if td[0]&eapTLSFlagL == 0 {
		t.Fatal("first fragment must have L flag set")
	}
	if td[0]&eapTLSFlagM == 0 {
		t.Fatal("first fragment of multi-fragment must have M flag set")
	}

	declaredLen := int(binary.BigEndian.Uint32(td[1:5]))
	if declaredLen != 2000 {
		t.Fatalf("declared length = %d, want 2000", declaredLen)
	}
}

func TestTLSFragmenterSmallDataNoFragment(t *testing.T) {
	data := make([]byte, 100)

	var f tlsFragmenter
	f.startSending(data)

	td := f.nextFragment()

	if td[0]&eapTLSFlagM != 0 {
		t.Fatal("small data should not have M flag")
	}
	if td[0]&eapTLSFlagL == 0 {
		t.Fatal("first (and only) fragment should have L flag")
	}
	if f.waitFragAck {
		t.Fatal("single fragment should not require ACK")
	}
}

func TestTLSReassemblyRejectsOversized(t *testing.T) {
	// RFC requirement: RFC5216-2.1.5-1 negative -- a first fragment whose L-flag
	// TLS Message Length exceeds the reassembly cap is rejected rather than
	// buffered, bounding the memory a peer can force during reassembly.
	var f tlsFragmenter

	td := make([]byte, 5)
	td[0] = eapTLSFlagL | eapTLSFlagM
	binary.BigEndian.PutUint32(td[1:5], uint32(eapTLSMaxReassembly+1))

	err := f.reassemble(td)
	if err == nil {
		t.Fatal("expected error for oversized TLS message")
	}
}

func TestTLSFragmenterMiddleAndLastFragmentsHaveNoLength(t *testing.T) {
	// A 3000-byte message at 1024 bytes/fragment produces three fragments:
	// first (L+M), middle (M only), last (neither).
	data := make([]byte, 3000)

	var f tlsFragmenter
	f.startSending(data)

	first := f.nextFragment()
	if first[0]&eapTLSFlagL == 0 {
		t.Fatal("first fragment must have the L flag set")
	}

	middle := f.nextFragment()
	last := f.nextFragment()

	// RFC requirement: RFC5216-3-1 negative -- only the first fragment carries
	// the L bit; the middle fragment (More set, not first) and the last fragment
	// (neither More nor first) MUST NOT set L, so the 4-octet TLS Message Length
	// appears exactly once per message (RFC 5216 Section 3, Section 2.1.5).
	if middle[0]&eapTLSFlagM == 0 {
		t.Fatalf("middle fragment should have the M flag set, flags=0x%02x", middle[0])
	}
	if middle[0]&eapTLSFlagL != 0 {
		t.Fatalf("middle fragment must NOT set the L flag, flags=0x%02x", middle[0])
	}
	if last[0]&eapTLSFlagM != 0 {
		t.Fatalf("last fragment must NOT set the M flag, flags=0x%02x", last[0])
	}
	if last[0]&eapTLSFlagL != 0 {
		t.Fatalf("last fragment must NOT set the L flag, flags=0x%02x", last[0])
	}
}

func TestTLSFragmentReservedFlagBitsAreZero(t *testing.T) {
	// RFC requirement: RFC5216-3-2 positive -- every EAP-TLS flags octet emitted
	// on send has reserved bits 3..7 (mask 0x1F) clear; only L (0x80), M (0x40)
	// and S (0x20) are ever set (RFC 5216 Section 3).
	const reservedMask = 0x1F

	// The Start request uses the S flag alone.
	if eapTLSFlagS&reservedMask != 0 {
		t.Fatalf("Start flags 0x%02x set a reserved bit", eapTLSFlagS)
	}

	// A multi-fragment send exercises first (L+M), middle (M), last (none); a
	// small send exercises the single-fragment (L only) case.
	for _, size := range []int{3000, 100} {
		var f tlsFragmenter
		f.startSending(make([]byte, size))
		for {
			td := f.nextFragment()
			if td[0]&reservedMask != 0 {
				t.Fatalf("size %d: fragment flags 0x%02x set a reserved bit (mask 0x%02x)", size, td[0], reservedMask)
			}
			if td[0]&eapTLSFlagM == 0 {
				break
			}
		}
	}
}
