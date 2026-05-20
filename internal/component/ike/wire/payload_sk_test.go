package wire

import (
	"bytes"
	"testing"
)

func TestPayloadSKRoundtrip(t *testing.T) {
	ciphertext := make([]byte, 128)
	for i := range ciphertext {
		ciphertext[i] = byte(i ^ 0xAA)
	}
	p := PayloadSK{CipherText: ciphertext}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadSK
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if !bytes.Equal(got.CipherText, ciphertext) {
		t.Error("CipherText mismatch")
	}
}
