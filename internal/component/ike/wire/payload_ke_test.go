package wire

import (
	"bytes"
	"testing"
)

func TestPayloadKERoundtrip(t *testing.T) {
	keyData := make([]byte, 256)
	for i := range keyData {
		keyData[i] = byte(i)
	}
	ke := PayloadKE{
		DHGroup:         14,
		KeyExchangeData: keyData,
	}

	buf := make([]byte, 512)
	n := ke.WriteTo(buf, 0)
	if n != 4+256 {
		t.Fatalf("WriteTo = %d, want %d", n, 4+256)
	}

	var got PayloadKE
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.DHGroup != 14 {
		t.Errorf("DHGroup = %d, want 14", got.DHGroup)
	}
	if !bytes.Equal(got.KeyExchangeData, keyData) {
		t.Error("KeyExchangeData mismatch")
	}
}
