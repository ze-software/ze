// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- PRF and prf+ key expansion

package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"hash"
)

func prfHashFunc(id PRFID) func() hash.Hash {
	switch id {
	case PRF_HMAC_SHA2_256:
		return sha256.New
	case PRF_HMAC_SHA2_384:
		return sha512.New384
	case PRF_HMAC_SHA2_512:
		return sha512.New
	default:
		return nil
	}
}

// PRF computes prf(key, data) using the selected HMAC algorithm.
func PRF(id PRFID, key, data []byte) ([]byte, error) {
	hf := prfHashFunc(id)
	if hf == nil {
		return nil, ErrUnsupportedAlgorithm
	}
	mac := hmac.New(hf, key)
	mac.Write(data)
	return mac.Sum(nil), nil
}

// PRFPlus implements RFC 7296 Section 2.13 prf+ key expansion.
// T1 = prf(K, S | 0x01)
// T2 = prf(K, T1 | S | 0x02)
// T3 = prf(K, T2 | S | 0x03)
// ...
// prf+(K, S) = T1 | T2 | T3 | ...
func PRFPlus(id PRFID, key, seed []byte, length int) ([]byte, error) {
	hf := prfHashFunc(id)
	if hf == nil {
		return nil, ErrUnsupportedAlgorithm
	}

	hashLen := hf().Size()
	if length > 255*hashLen {
		return nil, errors.New("prf+ requested length exceeds 255 iterations")
	}

	result := make([]byte, 0, length)
	var prev []byte
	for i := byte(1); len(result) < length; i++ {
		mac := hmac.New(hf, key)
		mac.Write(prev)
		mac.Write(seed)
		mac.Write([]byte{i})
		prev = mac.Sum(nil)
		result = append(result, prev...)
	}
	return result[:length], nil
}
