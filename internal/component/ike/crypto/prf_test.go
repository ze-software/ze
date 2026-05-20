package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"testing"
)

func TestPRFHMACSHA256(t *testing.T) {
	key := []byte("test-key-for-prf")
	data := []byte("test-data-for-prf")

	result, err := PRF(PRF_HMAC_SHA2_256, key, data)
	if err != nil {
		t.Fatalf("PRF(SHA256): %v", err)
	}
	if len(result) != 32 {
		t.Errorf("PRF output length = %d, want 32", len(result))
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	expected := mac.Sum(nil)
	if !hmac.Equal(result, expected) {
		t.Error("PRF output does not match direct HMAC-SHA256")
	}
}

func TestPRFHMACSHA384(t *testing.T) {
	key := []byte("test-key-for-prf")
	data := []byte("test-data-for-prf")

	result, err := PRF(PRF_HMAC_SHA2_384, key, data)
	if err != nil {
		t.Fatalf("PRF(SHA384): %v", err)
	}
	if len(result) != 48 {
		t.Errorf("PRF output length = %d, want 48", len(result))
	}

	mac := hmac.New(sha512.New384, key)
	mac.Write(data)
	expected := mac.Sum(nil)
	if !hmac.Equal(result, expected) {
		t.Error("PRF output does not match direct HMAC-SHA384")
	}
}

func TestPRFHMACSHA512(t *testing.T) {
	key := []byte("test-key-for-prf")
	data := []byte("test-data-for-prf")

	result, err := PRF(PRF_HMAC_SHA2_512, key, data)
	if err != nil {
		t.Fatalf("PRF(SHA512): %v", err)
	}
	if len(result) != 64 {
		t.Errorf("PRF output length = %d, want 64", len(result))
	}

	mac := hmac.New(sha512.New, key)
	mac.Write(data)
	expected := mac.Sum(nil)
	if !hmac.Equal(result, expected) {
		t.Error("PRF output does not match direct HMAC-SHA512")
	}
}

func TestPRFUnsupported(t *testing.T) {
	_, err := PRF(PRFID(99), []byte("k"), []byte("d"))
	if err == nil {
		t.Error("PRF with unsupported ID should fail")
	}
}

func TestPRFPlus(t *testing.T) {
	key := []byte("skeyseed-value-for-testing-prfplus")
	seed := []byte("Ni|Nr|SPIi|SPIr")

	for _, length := range []int{32, 64, 100, 128, 256} {
		result, err := PRFPlus(PRF_HMAC_SHA2_256, key, seed, length)
		if err != nil {
			t.Fatalf("PRFPlus(length=%d): %v", length, err)
		}
		if len(result) != length {
			t.Errorf("PRFPlus(length=%d) output = %d bytes", length, len(result))
		}
	}
}

func TestPRFPlusDeterministic(t *testing.T) {
	key := []byte("deterministic-test-key")
	seed := []byte("deterministic-test-seed")

	a, err := PRFPlus(PRF_HMAC_SHA2_256, key, seed, 100)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PRFPlus(PRF_HMAC_SHA2_256, key, seed, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !hmac.Equal(a, b) {
		t.Error("PRFPlus is not deterministic")
	}
}

func TestPRFPlusPrefix(t *testing.T) {
	key := []byte("prefix-test-key")
	seed := []byte("prefix-test-seed")

	short, err := PRFPlus(PRF_HMAC_SHA2_256, key, seed, 32)
	if err != nil {
		t.Fatal(err)
	}
	long, err := PRFPlus(PRF_HMAC_SHA2_256, key, seed, 64)
	if err != nil {
		t.Fatal(err)
	}
	if !hmac.Equal(short, long[:32]) {
		t.Error("PRFPlus shorter output is not a prefix of longer output")
	}
}
