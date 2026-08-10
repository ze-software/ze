//go:build ze_vpp

package dataplane

import (
	"testing"

	"go.fd.io/govpp/binapi/ipsec_types"
)

func TestVPPBackendRequiresConnector(t *testing.T) {
	_, err := newVPPBackend()
	if err == nil {
		t.Fatal("newVPPBackend should fail without VPP connector")
	}
}

func TestVPPCryptoAlg(t *testing.T) {
	tests := []struct {
		algo   string
		aead   bool
		wantID ipsec_types.IpsecCryptoAlg
	}{
		{"aes128gcm", true, 7},
		{"aes256gcm", true, 9},
		{"chacha20poly1305", true, 12},
		{"aes128", false, 1},
		{"aes256", false, 3},
		// 11 is IPSEC_API_CRYPTO_ALG_3DES_CBC. This row asserted 4 until 2026-08-10,
		// and 4 is AES_CTR_128. The test agreed with the code, and both were wrong.
		// An operator who configured 3DES got a different cipher.
		{"3des", false, 11},
	}
	for _, tt := range tests {
		id, err := vppCryptoAlg(tt.algo, tt.aead)
		if err != nil {
			t.Fatalf("vppCryptoAlg(%q, %v): %v", tt.algo, tt.aead, err)
		}
		if id != tt.wantID {
			t.Errorf("vppCryptoAlg(%q, %v) = %d, want %d", tt.algo, tt.aead, id, tt.wantID)
		}
	}
}

func TestVPPIntegAlg(t *testing.T) {
	tests := []struct {
		algo   string
		aead   bool
		wantID ipsec_types.IpsecIntegAlg
	}{
		{"sha256", false, 4},
		{"sha384", false, 5},
		{"sha512", false, 6},
		{"sha1", false, 2},
		{"sha256", true, 0},
	}
	for _, tt := range tests {
		id, err := vppIntegAlg(tt.algo, tt.aead)
		if err != nil {
			t.Fatalf("vppIntegAlg(%q, %v): %v", tt.algo, tt.aead, err)
		}
		if id != tt.wantID {
			t.Errorf("vppIntegAlg(%q, %v) = %d, want %d", tt.algo, tt.aead, id, tt.wantID)
		}
	}
}

func TestVPPKeyData(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	k := vppKey(key)
	if k.Length != 32 {
		t.Errorf("vppKey length = %d, want 32", k.Length)
	}
	for i := range 32 {
		if k.Data[i] != byte(i) {
			t.Errorf("vppKey data[%d] = %d, want %d", i, k.Data[i], i)
		}
	}
}

func TestVPPKeyDataTruncation(t *testing.T) {
	key := make([]byte, 200)
	k := vppKey(key)
	if k.Length != 128 {
		t.Errorf("vppKey length = %d, want 128 (truncated)", k.Length)
	}
}
