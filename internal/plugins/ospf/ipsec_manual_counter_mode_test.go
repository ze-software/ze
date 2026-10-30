// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- RFC 4552 manually keyed SAs
// Related: config_ipsec.go -- validateESPConfidentiality
//
// VALIDATES: the OSPFv3 RFC 4552 IPsec block is Ze's only manually keyed SA, and its
// ESP confidentiality accepts only the AES-CBC ciphers and the null cipher. A counter-mode
// cipher (AES-GCM, AES-CCM, AES-CTR, ChaCha20-Poly1305) is refused at config validation,
// so no counter-mode SA is ever keyed by hand.
// PREVENTS: a manually keyed counter-mode SA, whose counter cannot be handled securely
// across reboots, reaching the kernel installer.

package ospf

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// manualKeyedInterface answers an OSPFv3 interface whose RFC 4552 ESP block names the
// cipher word with an encryption key of keyOctets octets.
func manualKeyedInterface(word string, keyOctets int) interfaceConfig {
	return interfaceConfig{
		Name: "eth0",
		IPsec: &ipsecInterfaceConfig{
			Protocol: "esp",
			EncAlgo:  word,
			EncKey:   hex.EncodeToString([]byte(strings.Repeat("k", keyOctets))),
		},
	}
}

// TestManualKeyingRefusesCounterModeCiphers checks the two counter-mode prohibitions that
// bind a manually keyed SA.
//
// RFC 8221 Section 3: "If manual keying is used regardless, Counter Mode algorithms such as
// ENCR_AES_CTR, ENCR_AES_CCM, ENCR_AES_GCM, and ENCR_CHACHA20_POLY1305 MUST NOT be used, as
// it is incompatible with a secure and persistent handling of the counter (as explained in
// the Security Considerations section of [RFC3686])."
//
// RFC 4106 Section 2: "For safety's sake, implementations MUST use an automated key
// management system, such as the Internet Key Exchange (IKE) [RFC2409], to ensure that
// this requirement is met."
func TestManualKeyingRefusesCounterModeCiphers(t *testing.T) {
	// RFC requirement: RFC8221-3-1 negative -- the manually keyed OSPFv3 ESP block refuses
	// the counter-mode cipher words aes128gcm, aes256gcm, aes128ccm16, aes128ctr and
	// chacha20poly1305 with ErrIPsecEncAlgo, each given a key of the length that cipher
	// takes.
	// RFC requirement: RFC4106-2-2 negative -- the manually keyed OSPFv3 ESP block, Ze's only
	// path that installs an SA without IKE, refuses aes128gcm and aes256gcm with
	// ErrIPsecEncAlgo, so an AES-GCM-ESP key never comes from manual configuration.
	for word, octets := range map[string]int{
		"aes128gcm":        20,
		"aes256gcm":        36,
		"aes128ccm16":      19,
		"aes128ctr":        20,
		"chacha20poly1305": 36,
	} {
		err := validateESPConfidentiality(manualKeyedInterface(word, octets))
		if !errors.Is(err, ErrIPsecEncAlgo) {
			t.Errorf("manual keying with %s: validateESPConfidentiality = %v, want ErrIPsecEncAlgo", word, err)
		}
	}
	// RFC requirement: RFC8221-3-1 positive -- the same block accepts aes128 and aes256,
	// the AES-CBC ciphers, and null, so the refusal belongs to the counter-mode cipher.
	// RFC requirement: RFC4106-2-2 positive -- the same block accepts aes128 and aes256,
	// so the refusal above belongs to AES-GCM rather than to manual keying as a whole.
	for word, octets := range map[string]int{"aes128": 16, "aes256": 32} {
		if err := validateESPConfidentiality(manualKeyedInterface(word, octets)); err != nil {
			t.Errorf("manual keying with %s: validateESPConfidentiality = %v, want it accepted", word, err)
		}
	}
	null := manualKeyedInterface("null", 0)
	null.IPsec.EncKey = ""
	if err := validateESPConfidentiality(null); err != nil {
		t.Errorf("manual keying with null: validateESPConfidentiality = %v, want it accepted", err)
	}
}
