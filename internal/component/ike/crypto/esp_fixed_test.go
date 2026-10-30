// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- ESP-only fixed-key transforms
package crypto

import (
	"bytes"
	"errors"
	"math/bits"
	"testing"
)

// TestESPFixedCipherAdmission exercises negotiation, not just registry membership,
// and keeps the same transforms out of IKE even when local policy asks for them.
func TestESPFixedCipherAdmission(t *testing.T) {
	for _, name := range []string{"3des", "chacha20poly1305"} {
		t.Run(name, func(t *testing.T) {
			enc, err := LookupESPEncryption(name)
			if err != nil {
				t.Fatal(err)
			}
			integ := IntegrityTransform{ID: AUTH_NONE}
			if name == "3des" {
				integ, err = LookupESPIntegrity("sha1")
				if err != nil {
					t.Fatal(err)
				}
			}
			p := ESPProposal{Number: 1, Encryption: enc, Integrity: integ}
			if _, err := NegotiateESP([]ESPProposal{p}, []ESPProposal{p}); err != nil {
				t.Fatal(err)
			}
			for _, length := range []uint16{1, 128, 192, 256, 65535} {
				bad := p
				bad.Encryption.KeyLength = length
				if _, err := NegotiateESP([]ESPProposal{bad}, []ESPProposal{p}); err == nil {
					t.Fatalf("accepted fixed-key transform with length %d", length)
				}
			}
			if _, err := LookupEncryption(name); !errors.Is(err, ErrUnsupportedAlgorithm) {
				t.Fatalf("IKE lookup: %v", err)
			}
			ike := *aeadIKEProposal(enc.ID)
			ike.Encryption = enc
			ike.Integrity = integ
			if _, err := NegotiateIKE([]IKEProposal{ike}, []IKEProposal{ike}); err == nil {
				t.Fatal("ESP-only cipher negotiated for IKE")
			}
		})
	}
	if _, err := LookupPRF("sha1"); !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf("SHA1 PRF: %v", err)
	}
	if _, err := LookupIntegrity("sha1"); !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf("SHA1 IKE integrity: %v", err)
	}
	ike := *aeadIKEProposal(ENCR_AES_CBC)
	ike.Encryption = NewEncryptionTransform(ENCR_AES_CBC, 256)
	ike.Integrity = IntegrityTransform{ID: AUTH_HMAC_SHA1_96}
	if _, err := NegotiateIKE([]IKEProposal{ike}, []IKEProposal{ike}); !errors.Is(err, ErrTransformUnspecified) {
		t.Fatalf("SHA1 IKE wire integrity: %v", err)
	}
	ike.Integrity = IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128}
	ike.PRF = PRFTransform{ID: PRFID(2)}
	if _, err := NegotiateIKE([]IKEProposal{ike}, []IKEProposal{ike}); !errors.Is(err, ErrTransformUnspecified) {
		t.Fatalf("SHA1 IKE wire PRF: %v", err)
	}
	if _, err := SealIKEAEAD(ENCR_CHACHA20_POLY1305, make([]byte, 36), nil, nil); !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf("ChaCha IKE sealer: %v", err)
	}
}

// TestESPFixedCipherKeyMaterial pins both directional offsets for initial and PFS
// rekeys against the PRF stream; 3DES comparisons ignore only DES parity bits.
func TestESPFixedCipherKeyMaterial(t *testing.T) {
	for _, tc := range []struct {
		name            string
		encLen, authLen int
	}{
		{"chacha20poly1305", 36, 0}, {"3des", 24, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := LookupESPEncryption(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			integ := IntegrityTransform{ID: AUTH_NONE}
			if tc.authLen != 0 {
				integ, err = LookupESPIntegrity("sha1")
				if err != nil {
					t.Fatal(err)
				}
				if integ.ID != 2 || integ.TruncatedLength != 12 {
					t.Fatalf("SHA1 transform: %+v", integ)
				}
			}
			for _, pfs := range []bool{false, true} {
				skD, ni, nr, dh := bytes.Repeat([]byte{1}, 32), []byte("initiator"), []byte("responder"), []byte("shared-secret")
				var keys *ChildSAKeys
				seed := append(append([]byte(nil), ni...), nr...)
				if pfs {
					keys, err = DeriveChildSAKeysPFS(PRF_HMAC_SHA2_256, skD, dh, ni, nr, enc, integ)
					seed = append(append([]byte(nil), dh...), seed...)
				} else {
					keys, err = DeriveChildSAKeys(PRF_HMAC_SHA2_256, skD, ni, nr, enc, integ)
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(keys.Clear)
				stream, err := PRFPlus(PRF_HMAC_SHA2_256, skD, seed, 2*(tc.encLen+tc.authLen))
				if err != nil {
					t.Fatal(err)
				}
				for i, got := range [][]byte{keys.EncryptKeyI, keys.IntegKeyI, keys.EncryptKeyR, keys.IntegKeyR} {
					wantLen := tc.authLen
					if i%2 == 0 {
						wantLen = tc.encLen
					}
					if len(got) != wantLen {
						t.Fatalf("key %d length %d, want %d", i, len(got), wantLen)
					}
					for j, octet := range got {
						want := stream[j]
						if tc.name == "3des" && i%2 == 0 {
							if bits.OnesCount8(octet)%2 != 1 {
								t.Fatal("DES key has even parity")
							}
							octet &= 0xfe
							want &= 0xfe
						}
						if octet != want {
							t.Fatalf("key %d offset %d differs from PRF stream", i, j)
						}
					}
					stream = stream[wantLen:]
				}
			}
		})
	}
}

// Test3DESKeyAdmission rejects adjacent equal DES keys even if their parity differs.
func Test3DESKeyAdmission(t *testing.T) {
	for _, length := range []int{0, 8, 16, 23, 25} {
		if err := prepare3DESKey(make([]byte, length)); !errors.Is(err, ErrInvalidKeyLength) {
			t.Fatalf("length %d: %v", length, err)
		}
	}
	for _, secondPair := range []bool{false, true} {
		key := append(bytes.Repeat([]byte{2}, 8), bytes.Repeat([]byte{3}, 8)...)
		key = append(key, bytes.Repeat([]byte{4}, 8)...)
		if secondPair {
			key = append(bytes.Repeat([]byte{4}, 8), key[:16]...)
		}
		if err := prepare3DESKey(key); err == nil {
			t.Fatal("degenerate 3DES key accepted")
		}
	}
}
