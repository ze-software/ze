package crypto

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"
)

func TestAESGCMEncryptDecrypt128(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("hello AES-GCM-128 encryption test")
	aad := []byte("additional authenticated data")

	ct, err := encryptAESGCM(key, plaintext, aad)
	if err != nil {
		t.Fatalf("EncryptAESGCM: %v", err)
	}

	pt, err := decryptAESGCM(key, ct, aad)
	if err != nil {
		t.Fatalf("DecryptAESGCM: %v", err)
	}

	if !bytes.Equal(pt, plaintext) {
		t.Error("AES-GCM-128 roundtrip failed")
	}
}

func TestAESGCMEncryptDecrypt256(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("hello AES-GCM-256 encryption test")

	ct, err := encryptAESGCM(key, plaintext, nil)
	if err != nil {
		t.Fatalf("EncryptAESGCM: %v", err)
	}

	pt, err := decryptAESGCM(key, ct, nil)
	if err != nil {
		t.Fatalf("DecryptAESGCM: %v", err)
	}

	if !bytes.Equal(pt, plaintext) {
		t.Error("AES-GCM-256 roundtrip failed")
	}
}

func TestAESGCMTagVerification(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	ct, err := encryptAESGCM(key, []byte("test"), nil)
	if err != nil {
		t.Fatal(err)
	}

	ct[len(ct)-1] ^= 0xff
	_, err = decryptAESGCM(key, ct, nil)
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("tampered ciphertext: got %v, want ErrDecryptionFailed", err)
	}
}

func TestAESGCMWrongAAD(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	ct, err := encryptAESGCM(key, []byte("test"), []byte("aad1"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = decryptAESGCM(key, ct, []byte("aad2"))
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("wrong AAD: got %v, want ErrDecryptionFailed", err)
	}
}

func TestIKEAEADRoundTrip128(t *testing.T) {
	keyWithSalt := make([]byte, 16+4) // AES-128 key + 4-byte salt
	if _, err := rand.Read(keyWithSalt); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("IKE AEAD roundtrip test payload")
	aad := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08} // IKE header stub

	ct, err := encryptIKEAEAD(keyWithSalt, plaintext, aad)
	if err != nil {
		t.Fatalf("EncryptIKEAEAD: %v", err)
	}
	if len(ct) < 8+len(plaintext)+16 {
		t.Fatalf("ciphertext too short: %d", len(ct))
	}

	pt, err := DecryptIKEAEAD(keyWithSalt, ct, aad)
	if err != nil {
		t.Fatalf("DecryptIKEAEAD: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Error("IKE AEAD-128 roundtrip mismatch")
	}
}

func TestIKEAEADRoundTrip256(t *testing.T) {
	keyWithSalt := make([]byte, 32+4) // AES-256 key + 4-byte salt
	if _, err := rand.Read(keyWithSalt); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("IKE AEAD 256-bit roundtrip")
	aad := make([]byte, 32) // IKE header + SK generic header
	if _, err := rand.Read(aad); err != nil {
		t.Fatal(err)
	}

	ct, err := encryptIKEAEAD(keyWithSalt, plaintext, aad)
	if err != nil {
		t.Fatalf("EncryptIKEAEAD: %v", err)
	}

	pt, err := DecryptIKEAEAD(keyWithSalt, ct, aad)
	if err != nil {
		t.Fatalf("DecryptIKEAEAD: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Error("IKE AEAD-256 roundtrip mismatch")
	}
}

func TestIKEAEADWrongAAD(t *testing.T) {
	keyWithSalt := make([]byte, 32+4)
	if _, err := rand.Read(keyWithSalt); err != nil {
		t.Fatal(err)
	}
	ct, err := encryptIKEAEAD(keyWithSalt, []byte("test"), []byte("correct-aad"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecryptIKEAEAD(keyWithSalt, ct, []byte("wrong-aad"))
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("wrong AAD: got %v, want ErrDecryptionFailed", err)
	}
}

func TestIKEAEADShortKey(t *testing.T) {
	_, err := encryptIKEAEAD([]byte{1, 2, 3}, []byte("test"), nil)
	if !errors.Is(err, ErrInvalidKeyLength) {
		t.Errorf("short key: got %v, want ErrInvalidKeyLength", err)
	}
}

func TestIKEAEADShortData(t *testing.T) {
	keyWithSalt := make([]byte, 32+4)
	if _, err := rand.Read(keyWithSalt); err != nil {
		t.Fatal(err)
	}
	_, err := DecryptIKEAEAD(keyWithSalt, []byte{1, 2, 3}, nil)
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("short data: got %v, want ErrDecryptionFailed", err)
	}
}

func TestIKEAEADWireFormat(t *testing.T) {
	keyWithSalt := make([]byte, 16+4)
	if _, err := rand.Read(keyWithSalt); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("wire format check")

	ct, err := encryptIKEAEAD(keyWithSalt, plaintext, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Wire format: IV(8) || ciphertext || GCM tag(16)
	expectedLen := 8 + len(plaintext) + 16
	if len(ct) != expectedLen {
		t.Errorf("wire length = %d, want %d (8 IV + %d plaintext + 16 tag)", len(ct), expectedLen, len(plaintext))
	}
}

func TestAESCBCEncryptDecrypt128(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("hello AES-CBC-128 test data here")

	ct, err := encryptAESCBC(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptAESCBC: %v", err)
	}

	pt, err := decryptAESCBC(key, ct)
	if err != nil {
		t.Fatalf("DecryptAESCBC: %v", err)
	}

	if !bytes.Equal(pt, plaintext) {
		t.Error("AES-CBC-128 roundtrip failed")
	}
}

func TestAESCBCEncryptDecrypt256(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("hello AES-CBC-256")

	ct, err := encryptAESCBC(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptAESCBC: %v", err)
	}

	pt, err := decryptAESCBC(key, ct)
	if err != nil {
		t.Fatalf("DecryptAESCBC: %v", err)
	}

	if !bytes.Equal(pt, plaintext) {
		t.Error("AES-CBC-256 roundtrip failed")
	}
}

func TestAESCBCPadding(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	for _, size := range []int{1, 15, 16, 17, 31, 32, 33, 100} {
		plaintext := make([]byte, size)
		for i := range plaintext {
			plaintext[i] = byte(i)
		}

		ct, err := encryptAESCBC(key, plaintext)
		if err != nil {
			t.Fatalf("EncryptAESCBC(size=%d): %v", size, err)
		}

		pt, err := decryptAESCBC(key, ct)
		if err != nil {
			t.Fatalf("DecryptAESCBC(size=%d): %v", size, err)
		}

		if !bytes.Equal(pt, plaintext) {
			t.Errorf("AES-CBC roundtrip failed for size=%d", size)
		}
	}
}

func TestHMACSHA256Integrity(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	data := []byte("data to authenticate")

	mac, err := ComputeIntegrity(AUTH_HMAC_SHA2_256_128, key, data)
	if err != nil {
		t.Fatalf("ComputeIntegrity: %v", err)
	}
	if len(mac) != 16 {
		t.Errorf("HMAC-SHA256-128 output length = %d, want 16", len(mac))
	}

	if err := VerifyIntegrity(AUTH_HMAC_SHA2_256_128, key, data, mac); err != nil {
		t.Errorf("VerifyIntegrity on valid data: %v", err)
	}

	tampered := append([]byte(nil), mac...)
	tampered[0] ^= 0xff
	if err := VerifyIntegrity(AUTH_HMAC_SHA2_256_128, key, data, tampered); !errors.Is(err, ErrIntegrityFailed) {
		t.Errorf("VerifyIntegrity on tampered: got %v, want ErrIntegrityFailed", err)
	}
}

func TestHMACSHA384Integrity(t *testing.T) {
	key := make([]byte, 48)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	data := []byte("data to authenticate with sha384")

	mac, err := ComputeIntegrity(AUTH_HMAC_SHA2_384_192, key, data)
	if err != nil {
		t.Fatalf("ComputeIntegrity: %v", err)
	}
	if len(mac) != 24 {
		t.Errorf("HMAC-SHA384-192 output length = %d, want 24", len(mac))
	}

	if err := VerifyIntegrity(AUTH_HMAC_SHA2_384_192, key, data, mac); err != nil {
		t.Errorf("VerifyIntegrity on valid data: %v", err)
	}
}

func TestHMACSHA512Integrity(t *testing.T) {
	key := make([]byte, 64)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	data := []byte("data to authenticate with sha512")

	mac, err := ComputeIntegrity(AUTH_HMAC_SHA2_512_256, key, data)
	if err != nil {
		t.Fatalf("ComputeIntegrity: %v", err)
	}
	if len(mac) != 32 {
		t.Errorf("HMAC-SHA512-256 output length = %d, want 32", len(mac))
	}

	if err := VerifyIntegrity(AUTH_HMAC_SHA2_512_256, key, data, mac); err != nil {
		t.Errorf("VerifyIntegrity on valid data: %v", err)
	}
}
