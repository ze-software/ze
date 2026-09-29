// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-MSCHAPv2 (RFC 2759)
// RFC: rfc/short/rfc2759.md -- Sections 3, 4, 9.1.3 and 9.2: the clauses the older units left unasserted
// Related: eap_mschapv2_test.go -- the first Response field units
// Related: rfc2759_peer_challenge_test.go -- the Peer-Challenge source unit
//
// The older units for these rows read the Response on the authenticator side
// only, or called a helper directly. Each test here reads what Ze's peer puts
// on the wire, or drives the authenticator through its real Process path, so a
// defect in the path the helper sits on goes red.
//
// VALIDATES: the 49-octet Response Value layout the peer emits, with its
// Reserved and Flags octets zero; the authenticator refusing each non-zero
// Reserved octet and a non-zero Flags octet; the bare user name in the
// NT-Response on both roles; the Section 9.2 password vector; the Peer-Challenge
// inside the NT-Response; the authenticator challenge drawn from crypto/rand;
// the R=0 field of the Section 9.1.3 Failure.
// PREVENTS: a peer that leaks state into Reserved or Flags; a check of the first
// Reserved octet only; a domain prefix hashed into the NT-Response on the real
// path; a UTF-8 password hash; an NT-Response computed without the Peer-Challenge;
// a predictable authenticator challenge; a Failure that invites a retry.

package eap

import (
	"bytes"
	"crypto/rand"
	"testing"
)

// rfc2759PeerResponse drives Ze's peer through Identity and the authenticator's
// real Challenge and returns the MS-CHAPv2 Response Type-Data it sends, beside
// the authenticator method that issued the Challenge.
func rfc2759PeerResponse(t *testing.T, identity, password string) (*mschapv2Method, *PeerSession, []byte) {
	t.Helper()

	server := &mschapv2Method{password: password}
	peer := NewPeerSession(TypeMSCHAPv2, identity, password)
	if res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); res.Err != nil {
		t.Fatalf("identity: %v", res.Err)
	}
	res := peer.Process(server.Start(2))
	if res.Err != nil {
		t.Fatalf("challenge: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("the peer sent no MS-CHAPv2 Response")
	}
	if len(res.Response.TypeData) < 54 {
		t.Fatalf("the MS-CHAPv2 Response is %d octets, want at least 54", len(res.Response.TypeData))
	}
	return server, peer, res.Response.TypeData
}

// rfc2759Refused feeds a copy of td with one change to a copy of the
// authenticator, taken at the Challenge, and reports whether it refused the
// Response. The copy matters: a refusal moves the method out of its Challenge
// state, and every later Response would then be refused whatever it carried.
func rfc2759Refused(server *mschapv2Method, td []byte, change func([]byte)) bool {
	bad := bytes.Clone(td)
	change(bad)
	probe := *server
	res := probe.Process(&Packet{Code: CodeResponse, Identifier: 2, Type: TypeMSCHAPv2, TypeData: bad})
	return res.Err != nil
}

// TestRFC2759PeerSendsTheResponseValueLayout reads the 49-octet Value Ze's peer
// emits field by field, then shows the authenticator refuses every octet of
// Reserved and the Flags octet when they are not zero.
func TestRFC2759PeerSendsTheResponseValueLayout(t *testing.T) {
	server, peer, td := rfc2759PeerResponse(t, "user", "pw")

	// RFC requirement: RFC2759-x-3 positive -- RFC 2759 Section 4: "However, the
	// Value field is sub-formatted differently as follows: 16 octets:
	// Peer-Challenge 8 octets: Reserved, must be zero 24 octets: NT-Response 1
	// octet : Flags". Value-Size is 49, the first 16 octets are the peer's own
	// Peer-Challenge, the next 24 after the Reserved octets are the NT-Response
	// computed outside the peer, and one Flags octet closes the Value.
	if td[4] != 49 {
		t.Fatalf("Value-Size is %d, want 49", td[4])
	}
	if !bytes.Equal(td[5:21], peer.peerChallenge[:]) {
		t.Fatalf("Value octets 0-15 are % x, want the Peer-Challenge % x", td[5:21], peer.peerChallenge)
	}
	want := GenerateNTResponse(server.authChallenge, peer.peerChallenge, "user", "pw")
	if !bytes.Equal(td[29:53], want[:]) {
		t.Fatalf("Value octets 24-47 are % x, want the NT-Response % x", td[29:53], want)
	}
	if string(td[54:]) != "user" {
		t.Fatalf("the Name after the 49-octet Value is %q, want %q", td[54:], "user")
	}

	// RFC requirement: RFC2759-x-1 positive -- RFC 2759 Section 4: "8 octets:
	// Reserved, must be zero". The eight octets after the Peer-Challenge that Ze's
	// peer sends are all zero.
	if !bytes.Equal(td[21:29], make([]byte, 8)) {
		t.Fatalf("the Reserved octets are % x, want eight zeros", td[21:29])
	}

	// RFC requirement: RFC2759-x-2 positive -- RFC 2759 Section 4: "The Flag field
	// is reserved for future use and MUST be zero." The octet after the
	// NT-Response that Ze's peer sends is zero.
	if td[53] != 0 {
		t.Fatalf("the Flags octet is %#02x, want 0", td[53])
	}

	// RFC requirement: RFC2759-x-3 negative -- a Value-Size other than 49 is not
	// that sub-format, and the authenticator refuses it.
	for _, size := range []byte{48, 50} {
		if !rfc2759Refused(server, td, func(b []byte) { b[4] = size }) {
			t.Fatalf("the authenticator accepted Value-Size %d", size)
		}
	}

	// RFC requirement: RFC2759-x-1 negative -- each Reserved octet, not only the
	// first, is read: one non-zero octet anywhere in the eight draws a refusal.
	for i := 21; i < 29; i++ {
		if !rfc2759Refused(server, td, func(b []byte) { b[i] = 0x01 }) {
			t.Fatalf("the authenticator accepted Reserved octet %d set to 1", i-21)
		}
	}

	// RFC requirement: RFC2759-x-2 negative -- a non-zero Flags octet draws a
	// refusal.
	if !rfc2759Refused(server, td, func(b []byte) { b[53] = 0x01 }) {
		t.Fatal("the authenticator accepted a Flags octet of 1")
	}
}

// TestRFC2759NTResponseUsesTheBareUserName drives both roles with a Name that
// carries a Windows NT domain.
func TestRFC2759NTResponseUsesTheBareUserName(t *testing.T) {
	// RFC requirement: RFC2759-x-4 positive -- RFC 2759 Section 4: "When computing
	// the NT-Response field contents, only the user name is used, without any
	// associated Windows NT domain name. This is true regardless of whether a
	// Windows NT domain name is present in the Name field (see below)." Ze's peer
	// sends the full Name but an NT-Response over the bare user name, and the
	// authenticator accepts it.
	server, peer, td := rfc2759PeerResponse(t, `DOMAIN\user`, "pw")
	if string(td[54:]) != `DOMAIN\user` {
		t.Fatalf("the Name is %q, want the full %q", td[54:], `DOMAIN\user`)
	}
	bare := GenerateNTResponse(server.authChallenge, peer.peerChallenge, "user", "pw")
	if !bytes.Equal(td[29:53], bare[:]) {
		t.Fatalf("the NT-Response is % x, want the one over the bare user name % x", td[29:53], bare)
	}
	if rfc2759Refused(server, td, func([]byte) {}) {
		t.Fatal("the authenticator refused the bare-name NT-Response")
	}

	// RFC requirement: RFC2759-x-4 negative -- an NT-Response computed over the
	// Name with its domain is refused by the authenticator, so the positive
	// reads the stripped name on the real Process path.
	withDomain := GenerateNTResponse(server.authChallenge, peer.peerChallenge, `DOMAIN\user`, "pw")
	if !rfc2759Refused(server, td, func(b []byte) { copy(b[29:53], withDomain[:]) }) {
		t.Fatal("the authenticator accepted an NT-Response over the domain-qualified Name")
	}
}

// TestRFC2759PasswordHashVector checks the Section 9.2 intermediate values.
func TestRFC2759PasswordHashVector(t *testing.T) {
	// RFC requirement: RFC2759-x-10 positive -- RFC 2759 Section 9.2:
	// "0-to-256-unicode-char Password: 63 00 6C 00 69 00 65 00 6E 00 74 00 50 00
	// 61 00 73 00 73 00". ntPasswordHash hashes "clientPass" as those UTF-16LE
	// octets: its result is the MD4 of them and the Section 9.2 PasswordHash
	// 44 EB BA 8D 53 12 B8 D6 11 47 44 11 F5 69 89 AE.
	unicode := []byte{0x63, 0, 0x6C, 0, 0x69, 0, 0x65, 0, 0x6E, 0, 0x74, 0, 0x50, 0, 0x61, 0, 0x73, 0, 0x73, 0}
	vector := [16]byte{0x44, 0xEB, 0xBA, 0x8D, 0x53, 0x12, 0xB8, 0xD6, 0x11, 0x47, 0x44, 0x11, 0xF5, 0x69, 0x89, 0xAE}
	got := ntPasswordHash("clientPass")
	if got != vector {
		t.Fatalf("PasswordHash is % x, want the Section 9.2 % x", got, vector)
	}
	if got != md4Sum(unicode) {
		t.Fatal("PasswordHash is not the MD4 of the Section 9.2 unicode octets")
	}

	// RFC requirement: RFC2759-x-10 negative -- a character outside ASCII is
	// hashed as its 2-octet unicode unit, not as its UTF-8 octets: "é" hashes as
	// E9 00, and the UTF-8 C3 A9 would give another hash.
	accent := ntPasswordHash("é")
	if accent != md4Sum([]byte{0xE9, 0x00}) {
		t.Fatalf("the hash of %q is % x, want the MD4 of E9 00", "é", accent)
	}
	if accent == md4Sum([]byte{0xC3, 0xA9}) {
		t.Fatal("the password was hashed as UTF-8 octets")
	}
}

// TestRFC2759PeerChallengeEntersTheNTResponse shows the Peer-Challenge is an
// input of the NT-Response the peer computes.
func TestRFC2759PeerChallengeEntersTheNTResponse(t *testing.T) {
	// RFC requirement: RFC2759-x-9 positive -- RFC 2759 Section 4: "The
	// Peer-Challenge field is a 16-octet random number. As the name implies, it is
	// generated by the peer and is used in the calculation of the NT-Response
	// field, below." The NT-Response on the wire is the one computed over the
	// Peer-Challenge the peer sent, and the authenticator refuses it once that
	// Peer-Challenge is swapped, so an NT-Response computed without it goes red.
	server, peer, td := rfc2759PeerResponse(t, "user", "pw")
	want := GenerateNTResponse(server.authChallenge, peer.peerChallenge, "user", "pw")
	if !bytes.Equal(td[29:53], want[:]) {
		t.Fatalf("the NT-Response is % x, want the one over the Peer-Challenge % x", td[29:53], want)
	}
	if !rfc2759Refused(server, td, func(b []byte) { b[5] ^= 0xFF }) {
		t.Fatal("the authenticator accepted the NT-Response under another Peer-Challenge")
	}
}

// TestRFC2759AuthenticatorChallengeComesFromCryptoRand substitutes
// crypto/rand.Reader and reads the challenge the authenticator sends.
func TestRFC2759AuthenticatorChallengeComesFromCryptoRand(t *testing.T) {
	saved := rand.Reader
	t.Cleanup(func() { rand.Reader = saved })

	var marker [16]byte
	for i := range marker {
		marker[i] = byte(0xA0 + i)
	}
	rand.Reader = bytes.NewReader(append(bytes.Clone(marker[:]), bytes.Repeat([]byte{0x22}, 256)...))

	// RFC requirement: RFC2759-x-8 positive -- RFC 2759 Section 3: "MS-CHAP-V2
	// authenticators send an 16-octet challenge Value field. Peers need not
	// duplicate Microsoft's algorithm for selecting the 16- octet value, but the
	// standard guidelines on randomness [1,2,7] SHOULD be observed." The
	// Challenge carries Value-Size 16, and its 16 octets are the ones
	// crypto/rand.Reader supplied, so a time-seeded or math/rand source goes red.
	server := &mschapv2Method{password: "pw"}
	td := server.Start(2).TypeData
	if td[4] != 16 {
		t.Fatalf("Value-Size is %d, want 16", td[4])
	}
	if !bytes.Equal(td[5:21], marker[:]) {
		t.Fatalf("the challenge is % x, want the crypto/rand octets % x", td[5:21], marker)
	}
}

// TestRFC2759FailureInvitesNoRetry reads the R= field of the Failure the
// authenticator sends for a wrong password.
func TestRFC2759FailureInvitesNoRetry(t *testing.T) {
	// RFC requirement: RFC2759-x-12 positive -- RFC 2759 Section 9.1.3: "Peer
	// Response/Challenge -> <- Failure (E=691 R=0) (Authenticator disconnects)".
	// The Failure carries E=691 and R=0, and the exchange ends in EAP-Failure
	// after the peer's answer.
	sess, verdict := mschapv2Refusal(t, "WrongPassword", "TestPassword")
	message := string(verdict.TypeData[4:])
	if code, ok := failureField(message, "E="); !ok || code != "691" {
		t.Fatalf("the Failure E= field is %q in %q, want 691", code, message)
	}
	if retry, ok := failureField(message, "R="); !ok || retry != "0" {
		t.Fatalf("the Failure R= field is %q in %q, want 0", retry, message)
	}
	final := sess.Process(&Packet{Code: CodeResponse, Identifier: verdict.Identifier, Type: TypeMSCHAPv2, TypeData: []byte{mschapv2OpFailure}})
	if final == nil || final.Code != CodeFailure {
		t.Fatalf("the answer to the Failure drew %v, want EAP-Failure", final)
	}

	// RFC requirement: RFC2759-x-12 negative -- a verified NT-Response draws no
	// Failure: the same exchange with one password ends in EAP-Success.
	flight := driveClauseFlight(t, "TestPassword", "TestPassword")
	if flight.last.Code != CodeSuccess {
		t.Fatalf("a verified NT-Response ended with code %d, want EAP-Success", flight.last.Code)
	}
}
