// Design: docs/architecture/testing/interop.md -- installed transform evidence.
// Related: vpnproof.go -- AES-CBC/SHA256 kernel-state oracle.
package ipsec

import "testing"

// TestCBCKernelTransformsRejectWrongInstalledAlgorithm distinguishes an installed
// AES-CBC/SHA256 pair from a missing transform, an AEAD SA and substring matches.
func TestCBCKernelTransformsRejectWrongInstalledAlgorithm(t *testing.T) {
	for _, test := range []struct {
		name, state string
		valid       bool
	}{
		{"installed", "enc cbc(aes) 0x1234\nauth-trunc hmac(sha256) 0x1234 128", true},
		{"empty", "", false},
		{"wrong authentication", "enc cbc(aes) 0x1234\nauth-trunc hmac(sha1) 0x1234 96", false},
		{"wrong encryption", "aead rfc4106(gcm(aes)) 0x1234 128", false},
		{"substring", "enc not-cbc(aes) 0x1234\nauth-trunc hmac(sha256) 0x1234 128", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireESPKernelTransforms(test.state, []string{"cbc(aes)", "hmac(sha256)"})
			if (err == nil) != test.valid {
				t.Fatalf("installed transform verdict: %v; want valid=%t", err, test.valid)
			}
		})
	}
}
