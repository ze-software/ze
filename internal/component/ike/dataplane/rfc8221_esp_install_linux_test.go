// Design: docs/architecture/ike/ipsec-3-data-model.md -- the SA parameters the dataplane installs
// Related: xfrm_linux.go -- xfrmStateFromParams and the kernel algorithm names
// Related: ../crypto/rfc8221_esp_algorithms_test.go -- the MUST NOT algorithms refused at negotiation
//
// VALIDATES: the ESP algorithms RFC 8221 grades MUST reach the kernel as the transform
// they name. The Linux XFRM crypto layer runs each algorithm on every packet, so the
// boundary Ze owns is the state it builds: xfrmStateFromParams turns each configured
// algorithm word into the kernel transform, key and ICV length the XFRM layer is given.
// PREVENTS: a mandatory ESP algorithm installed under another transform name or with a
// wrong key or truncation length, and an unknown word installed as a default.
//
//go:build linux

package dataplane

import (
	"bytes"
	"errors"
	"net"
	"testing"
)

// rfc8221ESPState builds the XFRM state of one ESP SA carrying the given cipher and
// integrity words and keys.
func rfc8221ESPState(t *testing.T, enc string, encKey []byte, auth string, authKey []byte, aead bool) (name, authName string, key []byte, icvBits, truncBits int) {
	t.Helper()
	state, err := xfrmStateFromParams(SAParams{
		SPI:      0x2000,
		Src:      net.ParseIP("192.0.2.1"),
		Dst:      net.ParseIP("198.51.100.1"),
		Proto:    ProtoESP,
		Mode:     ModeTunnel,
		EncAlgo:  enc,
		EncKey:   encKey,
		AuthAlgo: auth,
		AuthKey:  authKey,
		IsAEAD:   aead,
	})
	if err != nil {
		t.Fatalf("xfrmStateFromParams(%s, %s): %v", enc, auth, err)
	}
	if aead {
		if state.Aead == nil || state.Crypt != nil || state.Auth != nil {
			t.Fatalf("%s: want one AEAD transform and no separate cipher or integrity, got %+v", enc, state)
		}
		return state.Aead.Name, "", state.Aead.Key, state.Aead.ICVLen, 0
	}
	if state.Crypt == nil || state.Auth == nil || state.Aead != nil {
		t.Fatalf("%s with %s: want a cipher and an integrity transform, got %+v", enc, auth, state)
	}
	if !bytes.Equal(state.Auth.Key, authKey) {
		t.Errorf("%s: the integrity key installed is %x, want %x", auth, state.Auth.Key, authKey)
	}
	return state.Crypt.Name, state.Auth.Name, state.Crypt.Key, 0, state.Auth.TruncateLen
}

// TestRFC8221ESPMandatoryCiphersInstall checks the ESP ciphers RFC 8221 Section 5 grades
// MUST, with the 128-bit and 256-bit AES keys the same section requires.
//
// RFC 8221 Section 5: "For that purpose, the requirement level for 128-bit keys and 256-bit
// keys is MUST (when applicable)." "ENCR_NULL status was set to MUST in [RFC7321] and
// remains a MUST to enable the use of ESP with only authentication, which is preferred
// over AH due to NAT traversal." "ENCR_AES_CBC MUST be implemented in order to enable
// interoperability between implementations that followed [RFC7321]." "ENCR_AES_GCM_16
// status has been updated from SHOULD+ to MUST in order to favor the use of authenticated
// encryption and AEAD algorithms."
func TestRFC8221ESPMandatoryCiphersInstall(t *testing.T) {
	authKey := bytes.Repeat([]byte{0x5a}, 32)
	// RFC requirement: RFC8221-5-1 positive -- aes128 and aes256 install cbc(aes) with a
	// 16-octet and a 32-octet key, and aes128gcm and aes256gcm install rfc4106(gcm(aes))
	// with the 16-octet or 32-octet key followed by the four-octet salt, each key as given.
	// RFC requirement: RFC8221-5-7 positive -- aes128 and aes256 install the cbc(aes)
	// cipher, with its key as given, beside the hmac(sha256) integrity transform.
	for word, octets := range map[string]int{"aes128": 16, "aes256": 32} {
		key := bytes.Repeat([]byte{0x11}, octets)
		name, authName, got, _, _ := rfc8221ESPState(t, word, key, "sha256", authKey, false)
		if name != "cbc(aes)" || authName != "hmac(sha256)" || !bytes.Equal(got, key) {
			t.Errorf("%s: installed %q with %q and key %x, want cbc(aes) with hmac(sha256) and key %x",
				word, name, authName, got, key)
		}
	}
	// RFC requirement: RFC8221-5-8 positive -- aes128gcm and aes256gcm install the single
	// AEAD transform rfc4106(gcm(aes)) with a 128-bit ICV and the key and salt as given.
	for word, octets := range map[string]int{"aes128gcm": 20, "aes256gcm": 36} {
		key := bytes.Repeat([]byte{0x22}, octets)
		name, _, got, icv, _ := rfc8221ESPState(t, word, key, "", nil, true)
		if name != "rfc4106(gcm(aes))" || icv != 128 || !bytes.Equal(got, key) {
			t.Errorf("%s: installed %q ICV %d key %x, want rfc4106(gcm(aes)) ICV 128 key %x",
				word, name, icv, got, key)
		}
	}
	// RFC requirement: RFC8221-5-6 positive -- null installs the kernel's null cipher
	// ecb(cipher_null) beside the hmac(sha256) integrity transform, which is ESP with
	// authentication only.
	name, authName, _, _, _ := rfc8221ESPState(t, "null", nil, "sha256", authKey, false)
	if name != "ecb(cipher_null)" || authName != "hmac(sha256)" {
		t.Errorf("null: installed %q with %q, want ecb(cipher_null) with hmac(sha256)", name, authName)
	}

	// RFC requirement: RFC8221-5-1 negative -- a cipher word the backend does not name,
	// aes192, is refused with ErrNotSupported rather than installed as AES-CBC.
	// RFC requirement: RFC8221-5-6 negative -- a cipher word the backend does not name is
	// refused with ErrNotSupported rather than installed as the null cipher.
	// RFC requirement: RFC8221-5-7 negative -- a cipher word the backend does not name is
	// refused with ErrNotSupported rather than installed as AES-CBC.
	// RFC requirement: RFC8221-5-8 negative -- an AEAD word the backend does not name is
	// refused with ErrNotSupported rather than installed as AES-GCM.
	for _, tc := range []struct {
		word string
		aead bool
	}{{"aes192", false}, {"des", false}, {"aes192gcm", true}, {"aes128ccm8", true}} {
		_, err := xfrmStateFromParams(SAParams{
			SPI: 0x2001, Src: net.ParseIP("192.0.2.1"), Dst: net.ParseIP("198.51.100.1"),
			Proto: ProtoESP, Mode: ModeTunnel, EncAlgo: tc.word, AuthAlgo: "sha256", IsAEAD: tc.aead,
		})
		if !errors.Is(err, ErrNotSupported) {
			t.Errorf("%s: xfrmStateFromParams err = %v, want ErrNotSupported", tc.word, err)
		}
	}
}

// TestRFC8221ESPMandatoryIntegrityInstall checks the ESP integrity algorithms RFC 8221
// Section 6 grades MUST and MUST-.
//
// RFC 8221 Section 6: "AUTH_HMAC_SHA1_96 has been downgraded from MUST in [RFC7321] to
// MUST- as there is an industry-wide trend to deprecate its usage." and
// "AUTH_HMAC_SHA2_256_128 MUST be implemented in order to replace AUTH_HMAC_SHA1_96."
func TestRFC8221ESPMandatoryIntegrityInstall(t *testing.T) {
	encKey := bytes.Repeat([]byte{0x33}, 16)
	// RFC requirement: RFC8221-6-4 positive -- sha1 installs hmac(sha1) truncated to 96
	// bits with its 20-octet key as given.
	_, authName, _, _, trunc := rfc8221ESPState(t, "aes128", encKey, "sha1", bytes.Repeat([]byte{0x44}, 20), false)
	if authName != "hmac(sha1)" || trunc != 96 {
		t.Errorf("sha1: installed %q truncated to %d bits, want hmac(sha1) truncated to 96", authName, trunc)
	}
	// RFC requirement: RFC8221-6-6 positive -- sha256 installs hmac(sha256) truncated to
	// 128 bits with its 32-octet key as given.
	_, authName, _, _, trunc = rfc8221ESPState(t, "aes128", encKey, "sha256", bytes.Repeat([]byte{0x55}, 32), false)
	if authName != "hmac(sha256)" || trunc != 128 {
		t.Errorf("sha256: installed %q truncated to %d bits, want hmac(sha256) truncated to 128", authName, trunc)
	}
	// RFC requirement: RFC8221-6-4 negative -- an integrity word the backend does not name,
	// md5, is refused with ErrNotSupported rather than installed as another HMAC.
	// RFC requirement: RFC8221-6-6 negative -- an integrity word the backend does not name,
	// md5, is refused with ErrNotSupported rather than installed as another HMAC.
	_, err := xfrmStateFromParams(SAParams{
		SPI: 0x2002, Src: net.ParseIP("192.0.2.1"), Dst: net.ParseIP("198.51.100.1"),
		Proto: ProtoESP, Mode: ModeTunnel, EncAlgo: "aes128", AuthAlgo: "md5",
	})
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("md5: xfrmStateFromParams err = %v, want ErrNotSupported", err)
	}
}
