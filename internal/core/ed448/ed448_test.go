package ed448

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// rfc8032Vector is one test vector of RFC 8032 Section 7.4, Ed448.
type rfc8032Vector struct {
	name, secret, public, message, signature string
}

// rfc8032Ed448Vectors are the "Blank" and "1 octet" vectors of RFC 8032 Section
// 7.4, copied from rfc/full/rfc8032.txt. Both have an empty context, the only one
// IKE uses.
var rfc8032Ed448Vectors = []rfc8032Vector{
	{
		name: "Blank",
		secret: "6c82a562cb808d10d632be89c8513ebf6c929f34ddfa8c9f63c9960ef6e348a3" +
			"528c8a3fcc2f044e39a3fc5b94492f8f032e7549a20098f95b",
		public: "5fd7449b59b461fd2ce787ec616ad46a1da1342485a70e1f8a0ea75d80e96778" +
			"edf124769b46c7061bd6783df1e50f6cd1fa1abeafe8256180",
		message: "",
		signature: "533a37f6bbe457251f023c0d88f976ae2dfb504a843e34d2074fd823d41a591f" +
			"2b233f034f628281f2fd7a22ddd47d7828c59bd0a21bfd3980ff0d2028d4b18a" +
			"9df63e006c5d1c2d345b925d8dc00b4104852db99ac5c7cdda8530a113a0f4db" +
			"b61149f05a7363268c71d95808ff2e652600",
	},
	{
		name: "1 octet",
		secret: "c4eab05d357007c632f3dbb48489924d552b08fe0c353a0d4a1f00acda2c463a" +
			"fbea67c5e8d2877c5e3bc397a659949ef8021e954e0a12274e",
		public: "43ba28f430cdff456ae531545f7ecd0ac834a55d9358c0372bfa0c6c6798c086" +
			"6aea01eb00742802b8438ea4cb82169c235160627b4c3a9480",
		message: "03",
		signature: "26b8f91727bd62897af15e41eb43c377efb9c610d48f2335cb0bd0087810f435" +
			"2541b143c4b981b7e18f62de8ccdf633fc1bf037ab7cd779805e0dbcc0aae1cb" +
			"cee1afb2e027df36bc04dcecbf154336c19f0af7e0a6472905e799f1953d2a0f" +
			"f3348ab21aa4adafd1d234441cf807c03a00",
	},
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex %q: %v", s, err)
	}
	return b
}

// pkcs8 wraps seed in the OneAsymmetricKey of RFC 8410 Section 7, as
// openssl genpkey -algorithm ed448 writes it.
func pkcs8(seed []byte) []byte {
	der := []byte{0x30, 0x47, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x71, 0x04, 0x3b, 0x04, 0x39}
	return append(der, seed...)
}

// spki wraps a public key in the SubjectPublicKeyInfo of RFC 8410 Section 4.
func spki(public []byte) []byte {
	der := []byte{0x30, 0x43, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x71, 0x03, 0x3a, 0x00}
	return append(der, public...)
}

// TestSignMatchesRFC8032Ed448Vectors proves Sign is pure Ed448 with an empty
// context: each RFC 8032 Section 7.4 vector's secret, read through the PKCS #8
// parser, derives the vector's public key and signs the vector's message to the
// vector's signature, byte for byte, and Verify accepts it under the public key
// read through the SubjectPublicKeyInfo parser. Ed448ph or a context would
// produce another signature.
func TestSignMatchesRFC8032Ed448Vectors(t *testing.T) {
	for _, v := range rfc8032Ed448Vectors {
		t.Run(v.name, func(t *testing.T) {
			priv, err := ParsePKCS8PrivateKey(pkcs8(mustHex(t, v.secret)))
			if err != nil {
				t.Fatalf("ParsePKCS8PrivateKey: %v", err)
			}
			wantPublic := mustHex(t, v.public)
			if got := priv.Public().Bytes(); !bytes.Equal(got, wantPublic) {
				t.Fatalf("public key = %x, want %x", got, wantPublic)
			}
			message := mustHex(t, v.message)
			signature, err := Sign(priv, message)
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if want := mustHex(t, v.signature); !bytes.Equal(signature, want) {
				t.Fatalf("signature = %x, want %x", signature, want)
			}
			pub, err := ParsePKIXPublicKey(spki(wantPublic))
			if err != nil {
				t.Fatalf("ParsePKIXPublicKey: %v", err)
			}
			if !Verify(pub, message, signature) {
				t.Fatal("Verify refused the RFC 8032 signature")
			}
		})
	}
}

// TestVerifyRefusesAlteredInput proves Verify answers false for an altered
// signature, an altered message, a signature one octet short or long, and the
// zero key, rather than panicking or accepting.
func TestVerifyRefusesAlteredInput(t *testing.T) {
	v := rfc8032Ed448Vectors[1]
	pub, err := NewPublicKey(mustHex(t, v.public))
	if err != nil {
		t.Fatal(err)
	}
	message := mustHex(t, v.message)
	signature := mustHex(t, v.signature)

	altered := bytes.Clone(signature)
	altered[10] ^= 0x01
	if Verify(pub, message, altered) {
		t.Error("an altered signature verified")
	}
	if Verify(pub, []byte{0x04}, signature) {
		t.Error("an altered message verified")
	}
	if Verify(pub, message, signature[:SignatureSize-1]) {
		t.Error("a 113-octet signature verified")
	}
	if Verify(pub, message, append(bytes.Clone(signature), 0)) {
		t.Error("a 115-octet signature verified")
	}
	if Verify(PublicKey{}, message, signature) {
		t.Error("the zero public key verified")
	}
	if _, err := Sign(PrivateKey{}, message); !errors.Is(err, errNoKey) {
		t.Errorf("Sign with the zero private key: err = %v, want errNoKey", err)
	}
}

// TestParseRefusesOtherAlgorithmsAndMalformedKeys proves the parsers answer
// ErrNotEd448 for id-Ed25519 and for id-Ed448 with parameters (RFC 8410 Section
// 3: the parameters MUST be absent), and a malformed-key error for a key of the
// wrong length or a publicKey field that is not the seed's.
func TestParseRefusesOtherAlgorithmsAndMalformedKeys(t *testing.T) {
	v := rfc8032Ed448Vectors[0]
	seed := mustHex(t, v.secret)
	public := mustHex(t, v.public)

	ed25519SPKI := bytes.Clone(spki(public))
	ed25519SPKI[8] = 0x70
	if _, err := ParsePKIXPublicKey(ed25519SPKI); !errors.Is(err, ErrNotEd448) {
		t.Errorf("id-Ed25519 SubjectPublicKeyInfo: err = %v, want ErrNotEd448", err)
	}
	// SEQUENCE { SEQUENCE { OID 1.3.101.113, NULL }, BIT STRING }.
	withParams := append([]byte{0x30, 0x45, 0x30, 0x07, 0x06, 0x03, 0x2b, 0x65, 0x71, 0x05, 0x00, 0x03, 0x3a, 0x00}, public...)
	if _, err := ParsePKIXPublicKey(withParams); !errors.Is(err, ErrNotEd448) {
		t.Errorf("id-Ed448 with NULL parameters: err = %v, want ErrNotEd448", err)
	}
	short := []byte{0x30, 0x42, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x71, 0x03, 0x39, 0x00}
	short = append(short, public[:56]...)
	if _, err := ParsePKIXPublicKey(short); !errors.Is(err, errMalformed) {
		t.Errorf("56-octet public key: err = %v, want errMalformed", err)
	}

	ed25519Key := bytes.Clone(pkcs8(seed))
	ed25519Key[11] = 0x70
	if _, err := ParsePKCS8PrivateKey(ed25519Key); !errors.Is(err, ErrNotEd448) {
		t.Errorf("id-Ed25519 PKCS #8: err = %v, want ErrNotEd448", err)
	}
	shortKey := []byte{0x30, 0x46, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x71, 0x04, 0x3a, 0x04, 0x38}
	shortKey = append(shortKey, seed[:56]...)
	if _, err := ParsePKCS8PrivateKey(shortKey); !errors.Is(err, errMalformed) {
		t.Errorf("56-octet seed: err = %v, want errMalformed", err)
	}

	// Version 1 with a publicKey [1] that belongs to another seed. The content is
	// 3 + 7 + 61 + 60 = 131 octets, so the SEQUENCE length takes the long form.
	other := mustHex(t, rfc8032Ed448Vectors[1].public)
	mismatched := []byte{0x30, 0x81, 0x83, 0x02, 0x01, 0x01, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x71, 0x04, 0x3b, 0x04, 0x39}
	mismatched = append(mismatched, seed...)
	mismatched = append(mismatched, 0x81, 0x3a, 0x00)
	mismatched = append(mismatched, other...)
	if _, err := ParsePKCS8PrivateKey(mismatched); err == nil || !strings.Contains(err.Error(), "publicKey field") {
		t.Errorf("mismatched publicKey field: err = %v, want the publicKey refusal", err)
	}
	matched := bytes.Clone(mismatched)
	copy(matched[len(matched)-PublicKeySize:], public)
	if _, err := ParsePKCS8PrivateKey(matched); err != nil {
		t.Errorf("matching publicKey field: %v", err)
	}
}
