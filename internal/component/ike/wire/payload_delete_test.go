package wire

import (
	"bytes"
	"testing"
)

func TestPayloadDeleteRoundtrip(t *testing.T) {
	spis := []byte{
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x02,
	}
	p := PayloadDelete{
		ProtocolID: ProtocolESP,
		SPISize:    4,
		NumSPIs:    2,
		SPIs:       spis,
	}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadDelete
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.ProtocolID != ProtocolESP {
		t.Errorf("ProtocolID = %d, want %d", got.ProtocolID, ProtocolESP)
	}
	if got.SPISize != 4 {
		t.Errorf("SPISize = %d, want 4", got.SPISize)
	}
	if got.NumSPIs != 2 {
		t.Errorf("NumSPIs = %d, want 2", got.NumSPIs)
	}
	if !bytes.Equal(got.SPIs, spis) {
		t.Errorf("SPIs = %x, want %x", got.SPIs, spis)
	}
}
