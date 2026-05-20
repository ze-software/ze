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

	ct, err := EncryptAESGCM(key, plaintext, aad)
	if err != nil {
		t.Fatalf("EncryptAESGCM: %v", err)
	}

	pt, err := DecryptAESGCM(key, ct, aad)
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

	ct, err := EncryptAESGCM(key, plaintext, nil)
	if err != nil {
		t.Fatalf("EncryptAESGCM: %v", err)
	}

	pt, err := DecryptAESGCM(key, ct, nil)
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
	ct, err := EncryptAESGCM(key, []byte("test"), nil)
	if err != nil {
		t.Fatal(err)
	}

	ct[len(ct)-1] ^= 0xff
	_, err = DecryptAESGCM(key, ct, nil)
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("tampered ciphertext: got %v, want ErrDecryptionFailed", err)
	}
}

func TestAESGCMWrongAAD(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	ct, err := EncryptAESGCM(key, []byte("test"), []byte("aad1"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecryptAESGCM(key, ct, []byte("aad2"))
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("wrong AAD: got %v, want ErrDecryptionFailed", err)
	}
}

func TestAESCBCEncryptDecrypt128(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("hello AES-CBC-128 test data here")

	ct, err := EncryptAESCBC(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptAESCBC: %v", err)
	}

	pt, err := DecryptAESCBC(key, ct)
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

	ct, err := EncryptAESCBC(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptAESCBC: %v", err)
	}

	pt, err := DecryptAESCBC(key, ct)
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

		ct, err := EncryptAESCBC(key, plaintext)
		if err != nil {
			t.Fatalf("EncryptAESCBC(size=%d): %v", size, err)
		}

		pt, err := DecryptAESCBC(key, ct)
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
