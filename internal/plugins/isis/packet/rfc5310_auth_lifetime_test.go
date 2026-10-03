// Design: docs/architecture/isis/isis-10-auth.md -- the CRYPTO_AUTH LSP digest
// Related: auth_sign.go -- SignPDU zeroes Checksum and Remaining Lifetime before the digest
// Related: auth_verify_test.go -- testLSP, finalizeLSPChecksum

package packet

import "testing"

// VALIDATES: on the RFC 5310 CRYPTO_AUTH path (HMAC-SHA-256, authentication type
// 3), an LSP whose Remaining Lifetime changes after signing still verifies, for
// several lifetimes up to the maximum (0 is a purge, which RFC 5304 sec 2 governs). A change to an authenticated
// field (the sequence number) is refused, so the acceptance is not a verifier
// that accepts everything.
// PREVENTS: a CRYPTO_AUTH digest that covers the Remaining Lifetime, which breaks
// every LSP as it ages in flight.
//
// RFC requirement: RFC5310-4-1 positive -- an HMAC-SHA-256 (type 3) signed LSP verifies after its Remaining Lifetime is set to 1, 600 and 0xFFFF with the Fletcher checksum recomputed, while a changed sequence number is refused.
func TestRFC5310LifetimeNotAuthenticated(t *testing.T) {
	key := Key{Algorithm: AuthAlgoHMACSHA256, Secret: []byte("crypto-auth"), KeyID: 7}
	signed, err := SignPDU(testLSP(t), key)
	if err != nil {
		t.Fatalf("SignPDU: %v", err)
	}
	if err := VerifyPDU(signed, []Key{key}); err != nil {
		t.Fatalf("VerifyPDU of the signed LSP: %v", err)
	}

	lifeOff := CommonHeaderLen + lspRemLifetimeOff
	for _, lifetime := range []uint16{1, 600, 0xFFFF} {
		aged := append([]byte(nil), signed...)
		aged[lifeOff], aged[lifeOff+1] = byte(lifetime>>8), byte(lifetime)
		finalizeLSPChecksum(aged)
		if err := VerifyPDU(aged, []Key{key}); err != nil {
			t.Fatalf("lifetime %d: VerifyPDU refused an LSP whose only change is the Remaining Lifetime: %v", lifetime, err)
		}
	}

	// The sequence number ends 4 octets after the LSP ID, which follows the lifetime.
	seqLast := lifeOff + 2 + 8 + 3
	tampered := append([]byte(nil), signed...)
	tampered[seqLast]++
	finalizeLSPChecksum(tampered)
	if err := VerifyPDU(tampered, []Key{key}); err == nil {
		t.Fatal("VerifyPDU accepted an LSP whose sequence number changed after signing")
	}
}
