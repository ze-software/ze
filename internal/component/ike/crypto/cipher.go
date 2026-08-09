// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- AEAD and non-AEAD ciphers, integrity

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"errors"
	"hash"
)

var (
	ErrDecryptionFailed = errors.New("decryption failed: authentication tag mismatch")
	ErrInvalidKeyLength = errors.New("invalid key length")
	ErrIntegrityFailed  = errors.New("integrity verification failed")
)

// EncryptAESGCM encrypts plaintext using AES-GCM with a 16-byte authentication tag.
// Returns nonce || ciphertext || tag.
func EncryptAESGCM(key, plaintext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, aad)
	return ciphertext, nil
}

// DecryptAESGCM decrypts data produced by EncryptAESGCM (nonce || ciphertext || tag).
func DecryptAESGCM(key, data, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, ErrDecryptionFailed
	}
	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}

// EncryptIKEAEAD encrypts using the IKEv2 AEAD construction (RFC 5282).
// keyWithSalt is the AEAD key material: AES key || 4-byte salt.
// Returns IV(8) || ciphertext || tag(16) for the wire.
func EncryptIKEAEAD(keyWithSalt, plaintext, aad []byte) ([]byte, error) {
	if len(keyWithSalt) < 4 {
		return nil, ErrInvalidKeyLength
	}
	aesKey := keyWithSalt[:len(keyWithSalt)-4]
	salt := keyWithSalt[len(keyWithSalt)-4:]

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	iv := make([]byte, 8)
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}
	nonce := make([]byte, 0, 12)
	nonce = append(nonce, salt...)
	nonce = append(nonce, iv...)

	sealed := gcm.Seal(nil, nonce, plaintext, aad)
	result := make([]byte, 8+len(sealed))
	copy(result, iv)
	copy(result[8:], sealed)
	return result, nil
}

// DecryptIKEAEAD decrypts using the IKEv2 AEAD construction (RFC 5282).
// keyWithSalt is the AEAD key material: AES key || 4-byte salt.
// data is IV(8) || ciphertext || tag(16) from the wire.
func DecryptIKEAEAD(keyWithSalt, data, aad []byte) ([]byte, error) {
	if len(keyWithSalt) < 4 {
		return nil, ErrInvalidKeyLength
	}
	if len(data) < 8 {
		return nil, ErrDecryptionFailed
	}
	aesKey := keyWithSalt[:len(keyWithSalt)-4]
	salt := keyWithSalt[len(keyWithSalt)-4:]
	iv := data[:8]
	ciphertext := data[8:]

	nonce := make([]byte, 0, 12)
	nonce = append(nonce, salt...)
	nonce = append(nonce, iv...)

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}

// EncryptAESCBC encrypts plaintext using AES-CBC with PKCS#7 padding.
// Returns iv || ciphertext.
func EncryptAESCBC(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	ct := make([]byte, len(padded))
	mode.CryptBlocks(ct, padded)
	return append(iv, ct...), nil
}

// DecryptAESCBC decrypts data produced by EncryptAESCBC (iv || ciphertext).
func DecryptAESCBC(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(data) < aes.BlockSize*2 || len(data)%aes.BlockSize != 0 {
		return nil, ErrDecryptionFailed
	}
	iv := data[:aes.BlockSize]
	ct := data[aes.BlockSize:]
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ct))
	mode.CryptBlocks(plaintext, ct)
	unpadded, err := pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return unpadded, nil
}

// DecryptAESCBCRaw decrypts without unpadding (caller handles IKEv2 padding).
// Input is iv(blockSize) || ciphertext.
func DecryptAESCBCRaw(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(data) < aes.BlockSize*2 || len(data)%aes.BlockSize != 0 {
		return nil, ErrDecryptionFailed
	}
	iv := data[:aes.BlockSize]
	ct := data[aes.BlockSize:]
	plaintext := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ct)
	return plaintext, nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}

var errBadPadding = errors.New("bad padding")

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errBadPadding
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize {
		return nil, errBadPadding
	}
	good := byte(1)
	for i := len(data) - pad; i < len(data); i++ {
		good &= byte(subtle.ConstantTimeByteEq(data[i], byte(pad)))
	}
	if good != 1 {
		return nil, errBadPadding
	}
	return data[:len(data)-pad], nil
}

func integrityHashFunc(id IntegrityID) func() hash.Hash {
	switch id {
	case AUTH_HMAC_SHA2_256_128:
		return sha256.New
	case AUTH_HMAC_SHA2_384_192:
		return sha512.New384
	case AUTH_HMAC_SHA2_512_256:
		return sha512.New
	default:
		return nil
	}
}

// ComputeIntegrity computes a truncated HMAC for the given data.
func ComputeIntegrity(id IntegrityID, key, data []byte) ([]byte, error) {
	t, err := LookupIntegrity(id.String())
	if err != nil {
		return nil, err
	}
	hf := integrityHashFunc(id)
	if hf == nil {
		return nil, ErrUnsupportedAlgorithm
	}
	mac := hmac.New(hf, key)
	mac.Write(data)
	full := mac.Sum(nil)
	return full[:t.TruncatedLength], nil
}

// VerifyIntegrity checks a truncated HMAC using constant-time comparison.
func VerifyIntegrity(id IntegrityID, key, data, expected []byte) error {
	computed, err := ComputeIntegrity(id, key, data)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(computed, expected) != 1 {
		return ErrIntegrityFailed
	}
	return nil
}
