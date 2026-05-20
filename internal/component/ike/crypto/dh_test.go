package crypto

import (
	"bytes"
	"errors"
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

func TestDHInvalidPublicKey(t *testing.T) {
	a, err := NewDHExchange(DH_MODP_2048)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Clear()

	_, err = a.SharedSecret([]byte{0})
	if !errors.Is(err, ErrInvalidPublicKey) {
		t.Errorf("SharedSecret(zero) = %v, want ErrInvalidPublicKey", err)
	}
}
