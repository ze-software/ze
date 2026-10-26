// Design: docs/architecture/ike/ipsec-3-data-model.md -- the esp-group proposal model
// Related: config.go -- parseESPProposal
//
// VALIDATES: an ESP proposal whose cipher is not AEAD cannot be configured without an
// integrity algorithm, so a Child SA Ze offers or accepts never pairs encryption with
// AUTH_NONE, while an AEAD cipher takes no integrity algorithm at all.
// PREVENTS: an ESP SA that encrypts without authenticating.
package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// rfc8221Proposal parses one esp-group proposal holding the cipher word and, when hash is
// not empty, the hash word.
func rfc8221Proposal(enc, hash string) (ESPProposal, error) {
	prop := config.NewTree()
	prop.Set("encryption", enc)
	if hash != "" {
		prop.Set("hash", hash)
	}
	return parseESPProposal("g", "1", prop)
}

// TestRFC8221ESPEncryptionIsAuthenticated checks that a non-AEAD ESP cipher always comes
// with an integrity algorithm.
//
// RFC 8221 Section 4: "Encryption without authentication is not effective and MUST NOT be
// used." Section 6: "AUTH_NONE has been downgraded from MAY in [RFC7321] to MUST NOT." and
// "In all other cases, AUTH_NONE MUST NOT be selected."
func TestRFC8221ESPEncryptionIsAuthenticated(t *testing.T) {
	// RFC requirement: RFC8221-4-1 negative -- an esp-group proposal naming aes128, aes256,
	// 3des or null with no hash is refused at parse with "hash is required".
	// RFC requirement: RFC8221-6-1 negative -- an esp-group proposal naming aes128, aes256,
	// 3des or null with no hash, which would select AUTH_NONE, is refused at parse.
	// RFC requirement: RFC8221-6-2 negative -- an esp-group proposal naming aes128, aes256,
	// 3des or null with no hash, which would select AUTH_NONE beside a cipher that is not
	// AEAD, is refused at parse.
	for _, enc := range []string{"aes128", "aes256", "3des", "null"} {
		p, err := rfc8221Proposal(enc, "")
		if err == nil || !strings.Contains(err.Error(), "hash is required") {
			t.Errorf("%s with no hash: parseESPProposal = %+v, %v; want the hash-required refusal", enc, p, err)
		}
	}
	// RFC requirement: RFC8221-4-1 positive -- the same ciphers with sha256 parse with that
	// hash, and aes128gcm, aes256gcm and chacha20poly1305 parse with no hash.
	// RFC requirement: RFC8221-6-1 positive -- the same ciphers with sha256 parse with that
	// hash, and the AEAD ciphers aes128gcm, aes256gcm and chacha20poly1305 parse with none.
	// RFC requirement: RFC8221-6-2 positive -- the only proposals that parse with no hash
	// are the AEAD ciphers aes128gcm, aes256gcm and chacha20poly1305.
	for _, enc := range []string{"aes128", "aes256", "3des", "null"} {
		p, err := rfc8221Proposal(enc, "sha256")
		if err != nil {
			t.Errorf("%s with sha256: parseESPProposal = %v, want it parsed", enc, err)
			continue
		}
		if p.Hash != HashSHA256 {
			t.Errorf("%s with sha256: parsed hash %v, want sha256", enc, p.Hash)
		}
	}
	for _, enc := range []string{"aes128gcm", "aes256gcm", "chacha20poly1305"} {
		p, err := rfc8221Proposal(enc, "")
		if err != nil {
			t.Errorf("%s with no hash: parseESPProposal = %v, want it parsed", enc, err)
			continue
		}
		if !p.Encryption.IsAEAD() {
			t.Errorf("%s parsed as a cipher that is not AEAD", enc)
		}
	}
}
