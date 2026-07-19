// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- MS-CHAPv2 crypto tests

package eap

import (
	"encoding/hex"
	"testing"
)

func TestMD4KnownVector(t *testing.T) {
	// RFC 1320 test vector: MD4("abc") = a448017aaf21d8525fc10ae87aa6729d.
	got := md4Sum([]byte("abc"))
	want := mustHex16("a448017aaf21d8525fc10ae87aa6729d")
	if got != want {
		t.Fatalf("MD4(abc): got %x, want %x", got, want)
	}
}

func TestMD4Empty(t *testing.T) {
	got := md4Sum(nil)
	want := mustHex16("31d6cfe0d16ae931b73c59d7e0c089c0")
	if got != want {
		t.Fatalf("MD4(empty): got %x, want %x", got, want)
	}
}

func TestNtPasswordHash(t *testing.T) {
	// Well-known: NtPasswordHash("Password") = a4f49c406510bdcab6824ee7c30fd852.
	// RFC requirement: RFC2759-x-10 positive -- the known-answer vector matches only when
	// the password is MD4-hashed over its UTF-16LE encoding, as NtPasswordHash does.
	got := ntPasswordHash("Password")
	want := mustHex16("a4f49c406510bdcab6824ee7c30fd852")
	if got != want {
		t.Fatalf("NtPasswordHash: got %x, want %x", got, want)
	}

	// RFC requirement: RFC2759-x-10 negative -- MD4 over the UTF-8 (raw ASCII) bytes of the
	// same password produces a different hash, so the vector falsifies a UTF-8 encoding.
	utf8Hash := md4Sum([]byte("Password"))
	if utf8Hash == want {
		t.Fatalf("UTF-8 MD4 must not match the UTF-16LE NtPasswordHash vector: %x", utf8Hash)
	}
}

func TestHashNtPasswordHash(t *testing.T) {
	pwHash := ntPasswordHash("Password")
	got := hashNtPasswordHash(pwHash)
	// Verify double-hash is deterministic and different from single hash.
	if got == pwHash {
		t.Fatal("HashNtPasswordHash should differ from NtPasswordHash")
	}
	got2 := hashNtPasswordHash(pwHash)
	if got != got2 {
		t.Fatal("HashNtPasswordHash not deterministic")
	}
}

func TestEAPMSCHAPv2Challenge(t *testing.T) {
	authChallenge := mustHex16("5b5d7c7d7b3f2f3e3c2c602132262628")
	peerChallenge := mustHex16("21402324255e262a28295f2b3a337c7e")
	userName := "User"
	password := "clientPass"

	ntResp := GenerateNTResponse(authChallenge, peerChallenge, userName, password)
	if len(ntResp) != 24 {
		t.Fatalf("NT-Response length: got %d, want 24", len(ntResp))
	}

	expected := mustHex24("82309ecd8d708b5ea08faa3981cd83544233114a3d85d6df")
	if ntResp != expected {
		t.Fatalf("NT-Response: got %x, want %x", ntResp, expected)
	}
}

func TestEAPMSCHAPv2AuthResponse(t *testing.T) {
	authChallenge := mustHex16("5b5d7c7d7b3f2f3e3c2c602132262628")
	peerChallenge := mustHex16("21402324255e262a28295f2b3a337c7e")
	userName := "User"
	password := "clientPass"
	ntResp := GenerateNTResponse(authChallenge, peerChallenge, userName, password)

	authResp := GenerateAuthenticatorResponse(password, ntResp, peerChallenge, authChallenge, userName)
	expected := mustHex20("407a5589115fd0d6209f510fe9c04566932cda56")
	if authResp != expected {
		t.Fatalf("AuthenticatorResponse: got %x, want %x", authResp, expected)
	}
}

func TestEAPMSCHAPv2MSK(t *testing.T) {
	authChallenge := mustHex16("5b5d7c7d7b3f2f3e3c2c602132262628")
	peerChallenge := mustHex16("21402324255e262a28295f2b3a337c7e")
	userName := "User"
	password := "clientPass"
	ntResp := GenerateNTResponse(authChallenge, peerChallenge, userName, password)

	msk := DeriveMSK(password, ntResp)
	if len(msk) != 64 {
		t.Fatalf("MSK length: got %d, want 64", len(msk))
	}

	masterKey := GetMasterKey(password, ntResp)
	if len(masterKey) != 16 {
		t.Fatalf("MasterKey length: got %d, want 16", len(masterKey))
	}

	recvKey := GetAsymmetricStartKey(masterKey, 16, true, true)
	sendKey := GetAsymmetricStartKey(masterKey, 16, false, true)
	if len(recvKey) != 16 || len(sendKey) != 16 {
		t.Fatalf("key lengths: recv=%d send=%d", len(recvKey), len(sendKey))
	}

	for i := range 16 {
		if msk[i] != recvKey[i] {
			t.Fatalf("MSK[%d] != recvKey[%d]", i, i)
		}
		if msk[16+i] != sendKey[i] {
			t.Fatalf("MSK[%d] != sendKey[%d]", 16+i, i)
		}
	}
}

func TestEAPMSCHAPv2WrongPassword(t *testing.T) {
	authChallenge := mustHex16("5b5d7c7d7b3f2f3e3c2c602132262628")
	peerChallenge := mustHex16("21402324255e262a28295f2b3a337c7e")
	userName := "User"

	correct := GenerateNTResponse(authChallenge, peerChallenge, userName, "clientPass")
	wrong := GenerateNTResponse(authChallenge, peerChallenge, userName, "wrongPass")

	if correct == wrong {
		t.Fatal("different passwords produced same NT-Response")
	}

	if VerifyNTResponse(authChallenge, peerChallenge, userName, "clientPass", wrong) {
		t.Fatal("wrong password should not verify")
	}
	if !VerifyNTResponse(authChallenge, peerChallenge, userName, "clientPass", correct) {
		t.Fatal("correct password should verify")
	}
}

func TestStripDomain(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"User", "User"},
		{"DOMAIN\\User", "User"},
		{"CORP\\admin", "admin"},
		{"nodomain", "nodomain"},
	}
	for _, tt := range tests {
		got := StripDomain(tt.input)
		if got != tt.want {
			t.Errorf("StripDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestChallengeHashExcludesDomainPrefix(t *testing.T) {
	authChallenge := mustHex16("5b5d7c7d7b3f2f3e3c2c602132262628")
	peerChallenge := mustHex16("21402324255e262a28295f2b3a337c7e")
	password := "clientPass"

	// RFC requirement: RFC2759-x-4 positive -- ChallengeHash consumes the bare username:
	// StripDomain removes the DOMAIN\ prefix so the domain-qualified name hashes
	// identically to its bare form.
	if StripDomain("DOMAIN\\User") != "User" {
		t.Fatalf("StripDomain(%q) = %q, want %q", "DOMAIN\\User", StripDomain("DOMAIN\\User"), "User")
	}
	if challengeHash(peerChallenge, authChallenge, StripDomain("DOMAIN\\User")) !=
		challengeHash(peerChallenge, authChallenge, "User") {
		t.Fatal("stripped DOMAIN\\ name must hash identically to the bare name")
	}

	// RFC requirement: RFC2759-x-4 negative -- feeding the unstripped DOMAIN\ name to
	// ChallengeHash yields a different Challenge, so a valid bare-username NT-Response no
	// longer verifies: excluding the prefix is load-bearing, not cosmetic.
	if challengeHash(peerChallenge, authChallenge, "DOMAIN\\User") ==
		challengeHash(peerChallenge, authChallenge, "User") {
		t.Fatal("unstripped DOMAIN\\ name must hash differently from the bare name")
	}
	bare := GenerateNTResponse(authChallenge, peerChallenge, "User", password)
	if VerifyNTResponse(authChallenge, peerChallenge, "DOMAIN\\User", password, bare) {
		t.Fatal("unstripped DOMAIN\\ username must not verify a bare-username NT-Response")
	}
}

func TestDESKeyExpansion(t *testing.T) {
	key7 := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd}
	got := expandDESKey(key7)
	for i, b := range got {
		bits := 0
		for bit := range 8 {
			if b&(1<<uint(bit)) != 0 {
				bits++
			}
		}
		if bits%2 == 0 {
			t.Errorf("key8[%d]=0x%02x: even parity (want odd)", i, b)
		}
	}
}

func mustHex16(s string) [16]byte {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 16 {
		panic("mustHex16: " + s)
	}
	var out [16]byte
	copy(out[:], b)
	return out
}

func mustHex20(s string) [20]byte {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 20 {
		panic("mustHex20: " + s)
	}
	var out [20]byte
	copy(out[:], b)
	return out
}

func mustHex24(s string) [24]byte {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 24 {
		panic("mustHex24: " + s)
	}
	var out [24]byte
	copy(out[:], b)
	return out
}
