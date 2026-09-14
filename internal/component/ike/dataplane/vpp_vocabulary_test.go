// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- VPP dataplane backend
// Related: vpp.go -- vppCryptoAlg and vppIntegAlg, the two mappers this test drives
// Related: xfrm_vocabulary_linux_test.go -- the same proof for the XFRM backend
//
// vppCryptoAlg and vppIntegAlg bind each algorithm word a negotiated Child SA carries
// to a VPP algorithm id, and ze-ipsec-conf.yang decides which words an operator can
// put in a proposal. Neither side derives from the other, so the two are gated here.
// A mapper is a switch, so only one direction is enumerable: every word the model
// admits is fed to the mapper, and each must map or be refused for a reason this test
// names. A word the mapper knows and the model does not offer cannot be listed from
// the switch, and is the direction the xfrm tables prove (ai/rules/principles.md).

//go:build ze_vpp

package dataplane

import (
	"errors"
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank import registers ze-ipsec-conf with the loader. It imports only
	// the modules the loader embeds, so the leaves below resolve in a binary that
	// links nothing else.
	_ "github.com/ze-software/ze/internal/component/ike/ipsec/yang"
)

// vppUnnameableCiphers are the model words VPP's API has no id for: ipsec_types
// (vendor/go.fd.io/govpp/binapi/ipsec_types) declares CBC, CTR, GCM, NULL-GMAC,
// DES, 3DES and ChaCha20-Poly1305 and no AES-CCM. The backend MUST refuse them
// rather than install a different cipher, and this list is what lets the test
// tell that refusal from a word the switch merely forgot.
var vppUnnameableCiphers = map[string]bool{
	"aes128ccm8":  true,
	"aes256ccm8":  true,
	"aes128ccm12": true,
	"aes256ccm12": true,
	"aes128ccm16": true,
	"aes256ccm16": true,
}

// TestVPPCipherVocabularyMatchesModel feeds every ESP encryption word the model admits
// to vppCryptoAlg, as an AEAD and as a plain cipher, and requires exactly one of the
// two to map unless VPP cannot name the cipher at all.
func TestVPPCipherVocabularyMatchesModel(t *testing.T) {
	model, err := configyang.EnumValues("vpn/ipsec/esp-group/proposal/encryption")
	if err != nil {
		t.Fatalf("read the ESP encryption enumeration: %v", err)
	}

	for _, word := range model {
		_, aeadErr := vppCryptoAlg(word, true)
		_, plainErr := vppCryptoAlg(word, false)
		mapped := 0
		for _, err := range []error{aeadErr, plainErr} {
			switch {
			case err == nil:
				mapped++
			case !errors.Is(err, ErrNotSupported):
				t.Errorf("vppCryptoAlg(%q) failed for a reason other than ErrNotSupported: %v", word, err)
			}
		}
		switch {
		case vppUnnameableCiphers[word] && mapped != 0:
			t.Errorf("vppCryptoAlg maps %q, which VPP's API cannot name; the id it answers is a different cipher", word)
		case vppUnnameableCiphers[word]:
			continue
		case mapped == 0:
			t.Errorf("the model admits cipher %q and vppCryptoAlg refuses it as both AEAD and plain, so a Child SA that negotiates it installs nothing", word)
		case mapped == 2:
			t.Errorf("vppCryptoAlg maps %q as both AEAD and plain, so the SA's integrity handling depends on a flag the word already decides", word)
		}
	}

	for word := range vppUnnameableCiphers {
		if !slices.Contains(model, word) {
			t.Errorf("vppUnnameableCiphers names %q and the model no longer offers it, so the entry excuses nothing", word)
		}
	}
}

// TestVPPIntegrityVocabularyMatchesModel feeds every ESP hash word the model admits to
// vppIntegAlg and requires each to map.
func TestVPPIntegrityVocabularyMatchesModel(t *testing.T) {
	model, err := configyang.EnumValues("vpn/ipsec/esp-group/proposal/hash")
	if err != nil {
		t.Fatalf("read the ESP hash enumeration: %v", err)
	}
	for _, word := range model {
		if _, err := vppIntegAlg(word, false); err != nil {
			t.Errorf("the model admits integrity algorithm %q and vppIntegAlg refuses it, so a Child SA that negotiates it installs nothing: %v", word, err)
		}
	}
}
