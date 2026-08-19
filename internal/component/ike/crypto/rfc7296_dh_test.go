// VALIDATES: RFC 7296 Section 3.4. The MODP Diffie-Hellman public value Ze sends has the
// length of the prime modulus. A value whose natural encoding is short gains zero octets in
// front and keeps its number.
// PREVENTS: a KE payload whose MODP public value is shorter than the modulus. A peer that
// reads a fixed-length field takes such a value as a different number.
package crypto

import (
	"bytes"
	"errors"
	"math/big"
	"testing"
)

// dhxModulusLen gives the octet length of the MODP-2048 prime modulus. NewDHExchange
// exponentiates over that modulus.
func dhxModulusLen() int { return len(modp2048Prime.Bytes()) }

// dhxShortValues returns group values whose natural big-endian encoding is shorter than the
// modulus. Each one needs zero octets in front to reach the modulus length. The last value is
// one octet short, which is the case a real exchange meets once in 256 times.
func dhxShortValues() []*big.Int {
	return []*big.Int{
		big.NewInt(2),
		new(big.Int).Lsh(big.NewInt(1), 1024),
		new(big.Int).Rsh(modp2048Prime, 8),
	}
}

// RFC requirement: RFC7296-3.4-1 positive -- NewDHExchange pads the MODP-2048 public value at
// dh.go:56. padBigInt at dh.go:105-113 adds the zero octets.
// The test asserts that the public value from the exported constructor has the length of
// modp2048Prime. It then asserts that a short value grows to that same length with zero
// octets in front. The padded form must still hold the original number.
func TestRFC7296MODPPublicValueMatchesModulusLength(t *testing.T) {
	modulusLen := dhxModulusLen()
	if modulusLen != 256 {
		t.Fatalf("MODP-2048 modulus is %d octets, want 256", modulusLen)
	}

	local, err := NewDHExchange(DH_MODP_2048)
	if err != nil {
		t.Fatalf("NewDHExchange: %v", err)
	}
	if len(local.PublicKey) != modulusLen {
		t.Fatalf("public value is %d octets, want the modulus length %d", len(local.PublicKey), modulusLen)
	}

	// The padded form must still name the number the exponentiation produced. A peer derives
	// the same secret from it only when the pad kept the value.
	remote, err := NewDHExchange(DH_MODP_2048)
	if err != nil {
		t.Fatalf("NewDHExchange peer: %v", err)
	}
	ours, err := local.SharedSecret(remote.PublicKey)
	if err != nil {
		t.Fatalf("local SharedSecret: %v", err)
	}
	theirs, err := remote.SharedSecret(local.PublicKey)
	if err != nil {
		t.Fatalf("remote SharedSecret: %v", err)
	}
	if !bytes.Equal(ours, theirs) {
		t.Fatal("the two sides derived different secrets, so the padded public value lost its number")
	}

	for _, value := range dhxShortValues() {
		natural := value.Bytes()
		if len(natural) >= modulusLen {
			t.Fatalf("test value needs a short natural encoding, got %d octets", len(natural))
		}
		padded := padBigInt(value, modulusLen)
		if len(padded) != modulusLen {
			t.Fatalf("a %d-octet value padded to %d octets, want %d", len(natural), len(padded), modulusLen)
		}
		zeros := modulusLen - len(natural)
		for i := range padded[:zeros] {
			if padded[i] != 0 {
				t.Fatalf("octet %d of the pad is 0x%02x, want a zero octet", i, padded[i])
			}
		}
		if !bytes.Equal(padded[zeros:], natural) {
			t.Fatal("the padded value does not end with the natural encoding")
		}
		if new(big.Int).SetBytes(padded).Cmp(value) != 0 {
			t.Fatal("the pad changed the number")
		}
	}
}

// RFC requirement: RFC7296-3.4-1 negative -- the pad on the send side is load-bearing, and not
// cosmetic. A conforming receiver refuses a value of the wrong length. Ze applies that rule
// itself at dh.go:79-87, so a peer that applies the same rule would refuse an unpadded value
// from Ze. This test drives Ze's own receiver as the stand-in for that peer.
//
// A short value is refused with ErrPublicKeyLength. The padded form of the SAME number is
// accepted, so the refusal is about the length of the field. The pad at dh.go:105-113 is
// therefore what a peer permits. The test also asserts that the pad puts the number at the
// END of the field. A left-aligned pad returns the right octet count and still names
// another number.
func TestRFC7296MODPShortPublicValueIsRefusedOnReceipt(t *testing.T) {
	modulusLen := dhxModulusLen()
	local, err := NewDHExchange(DH_MODP_2048)
	if err != nil {
		t.Fatalf("NewDHExchange: %v", err)
	}

	// 2 is the group generator, so it is a valid group element. Its natural encoding is
	// one octet, which is the exact shape Section 3.4 forbids on the wire.
	value := big.NewInt(2)
	short := value.Bytes()
	if len(short) >= modulusLen {
		t.Fatalf("test value must be shorter than the modulus, got %d octets", len(short))
	}
	if _, err := local.SharedSecret(short); !errors.Is(err, ErrPublicKeyLength) {
		t.Fatalf("SharedSecret(%d octets) = %v, want ErrPublicKeyLength", len(short), err)
	}

	// Every short encoding is refused, and not only the one-octet case.
	for _, v := range dhxShortValues() {
		natural := v.Bytes()
		if _, err := local.SharedSecret(natural); !errors.Is(err, ErrPublicKeyLength) {
			t.Errorf("SharedSecret(%d octets) = %v, want ErrPublicKeyLength", len(natural), err)
		}
	}

	// The padded form of the same number IS accepted, so the pad is what carries the value
	// across. A secret of the modulus length comes back.
	fromPadded, err := local.SharedSecret(padBigInt(value, modulusLen))
	if err != nil {
		t.Fatalf("SharedSecret refused the padded remote public value: %v", err)
	}
	if len(fromPadded) != modulusLen {
		t.Fatalf("shared secret is %d octets, want %d", len(fromPadded), modulusLen)
	}

	// The pad puts the number at the end of the field. A left-aligned or masked pad fails here
	// even though it returns the right number of octets.
	padded := padBigInt(value, modulusLen)
	if len(padded) != modulusLen {
		t.Fatalf("padded value is %d octets, want %d", len(padded), modulusLen)
	}
	if padded[modulusLen-1] != 2 {
		t.Fatalf("last octet is 0x%02x, want 0x02", padded[modulusLen-1])
	}
	for i := range padded[:modulusLen-1] {
		if padded[i] != 0 {
			t.Fatalf("octet %d is 0x%02x, want a zero octet", i, padded[i])
		}
	}
}
