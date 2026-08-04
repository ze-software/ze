package crypto

import (
	"bytes"
	"errors"
	"math/big"
	"testing"
)

func TestDHGroup14SharedSecret(t *testing.T) {
	a, err := NewDHExchange(DH_MODP_2048)
	if err != nil {
		t.Fatalf("NewDHExchange(MODP_2048) initiator: %v", err)
	}
	defer a.Clear()

	b, err := NewDHExchange(DH_MODP_2048)
	if err != nil {
		t.Fatalf("NewDHExchange(MODP_2048) responder: %v", err)
	}
	defer b.Clear()

	if len(a.PublicKey) != 256 {
		t.Errorf("initiator public key length = %d, want 256", len(a.PublicKey))
	}
	if len(b.PublicKey) != 256 {
		t.Errorf("responder public key length = %d, want 256", len(b.PublicKey))
	}

	secretA, err := a.SharedSecret(b.PublicKey)
	if err != nil {
		t.Fatalf("initiator SharedSecret: %v", err)
	}
	secretB, err := b.SharedSecret(a.PublicKey)
	if err != nil {
		t.Fatalf("responder SharedSecret: %v", err)
	}

	if !bytes.Equal(secretA, secretB) {
		t.Error("MODP 2048 shared secrets do not match")
	}
	if len(secretA) != 256 {
		t.Errorf("shared secret length = %d, want 256", len(secretA))
	}
}

func TestDHGroup19SharedSecret(t *testing.T) {
	a, err := NewDHExchange(DH_ECP_256)
	if err != nil {
		t.Fatalf("NewDHExchange(ECP_256) initiator: %v", err)
	}
	defer a.Clear()

	b, err := NewDHExchange(DH_ECP_256)
	if err != nil {
		t.Fatalf("NewDHExchange(ECP_256) responder: %v", err)
	}
	defer b.Clear()

	secretA, err := a.SharedSecret(b.PublicKey)
	if err != nil {
		t.Fatalf("initiator SharedSecret: %v", err)
	}
	secretB, err := b.SharedSecret(a.PublicKey)
	if err != nil {
		t.Fatalf("responder SharedSecret: %v", err)
	}

	if !bytes.Equal(secretA, secretB) {
		t.Error("ECP-256 shared secrets do not match")
	}
	if len(secretA) != 32 {
		t.Errorf("shared secret length = %d, want 32", len(secretA))
	}
}

func TestDHGroup20SharedSecret(t *testing.T) {
	a, err := NewDHExchange(DH_ECP_384)
	if err != nil {
		t.Fatalf("NewDHExchange(ECP_384) initiator: %v", err)
	}
	defer a.Clear()

	b, err := NewDHExchange(DH_ECP_384)
	if err != nil {
		t.Fatalf("NewDHExchange(ECP_384) responder: %v", err)
	}
	defer b.Clear()

	secretA, err := a.SharedSecret(b.PublicKey)
	if err != nil {
		t.Fatalf("initiator SharedSecret: %v", err)
	}
	secretB, err := b.SharedSecret(a.PublicKey)
	if err != nil {
		t.Fatalf("responder SharedSecret: %v", err)
	}

	if !bytes.Equal(secretA, secretB) {
		t.Error("ECP-384 shared secrets do not match")
	}
	if len(secretA) != 48 {
		t.Errorf("shared secret length = %d, want 48", len(secretA))
	}
}

func TestDHUnsupportedGroup(t *testing.T) {
	_, err := NewDHExchange(DHGroupID(99))
	if !errors.Is(err, ErrUnsupportedGroup) {
		t.Errorf("NewDHExchange(99) = %v, want ErrUnsupportedGroup", err)
	}
}

// TestDHInvalidPublicKey drives the MODP range guard of SharedSecret (dh.go), which
// refuses a peer value outside the open interval (1, p-1).
//
// Every value below carries the modulus length, so it reaches that guard. A shorter value
// is refused one line earlier, by the length check, with ErrPublicKeyLength. That error
// WRAPS ErrInvalidPublicKey. A one-octet fixture therefore keeps an
// errors.Is(ErrInvalidPublicKey) assertion green while the range guard never runs. The
// second assertion of each case states that. The next change to the length check then
// cannot silence this test the same way.
func TestDHInvalidPublicKey(t *testing.T) {
	a, err := NewDHExchange(DH_MODP_2048)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Clear()

	pMinusOne := new(big.Int).Sub(modp2048Prime, big.NewInt(1))
	cases := []struct {
		name  string
		value *big.Int
	}{
		{"zero", big.NewInt(0)},
		{"one, the lower bound", big.NewInt(1)},
		{"p-1, the upper bound", pMinusOne},
		{"p, the modulus itself", modp2048Prime},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			padded := padBigInt(c.value, modp2048Len)
			if len(padded) != modp2048Len {
				t.Fatalf("the fixture is %d octets, want %d: it would not reach the range guard",
					len(padded), modp2048Len)
			}
			_, err := a.SharedSecret(padded)
			if !errors.Is(err, ErrInvalidPublicKey) {
				t.Errorf("SharedSecret(%s) = %v, want ErrInvalidPublicKey", c.name, err)
			}
			if errors.Is(err, ErrPublicKeyLength) {
				t.Errorf("SharedSecret(%s) was refused for its LENGTH, so the range guard never ran", c.name)
			}
		})
	}
}
