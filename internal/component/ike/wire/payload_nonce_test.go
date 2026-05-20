package wire

import (
	"bytes"
	"errors"
	"testing"
)

func TestPayloadNonceRoundtrip(t *testing.T) {
	nonce := make([]byte, 32)
	for i := range nonce {
		nonce[i] = byte(i + 100)
	}
	pn := PayloadNonce{NonceData: nonce}

	buf := make([]byte, 512)
	n := pn.WriteTo(buf, 0)

	var got PayloadNonce
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if !bytes.Equal(got.NonceData, nonce) {
		t.Error("NonceData mismatch")
	}
}

func TestPayloadNonceBoundary(t *testing.T) {
	tests := []struct {
		name string
		size int
		err  error
	}{
		{"min valid", 16, nil},
		{"max valid", 256, nil},
		{"too short", 15, ErrNonceTooShort},
		{"too long", 257, ErrNonceTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pn PayloadNonce
			err := pn.ReadFrom(make([]byte, tt.size))
			if !errors.Is(err, tt.err) {
				t.Errorf("ReadFrom(%d bytes) = %v, want %v", tt.size, err, tt.err)
			}
		})
	}
}
