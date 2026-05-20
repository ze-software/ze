package wire

import (
	"bytes"
	"testing"
)

func TestPayloadEAPRoundtrip(t *testing.T) {
	eapData := []byte{26, 0x01, 0x02, 0x03, 0x04, 0x05}
	p := PayloadEAP{
		Code:       EAPCodeResponse,
		Identifier: 42,
		EAPData:    eapData,
	}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadEAP
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.Code != EAPCodeResponse {
		t.Errorf("Code = %d, want %d", got.Code, EAPCodeResponse)
	}
	if got.Identifier != 42 {
		t.Errorf("Identifier = %d, want 42", got.Identifier)
	}
	if !bytes.Equal(got.EAPData, eapData) {
		t.Errorf("EAPData = %x, want %x", got.EAPData, eapData)
	}
}

func TestPayloadEAPSuccessFailure(t *testing.T) {
	// Success/Failure have no type-data
	for _, code := range []uint8{EAPCodeSuccess, EAPCodeFailure} {
		p := PayloadEAP{Code: code, Identifier: 1}
		buf := make([]byte, 64)
		n := p.WriteTo(buf, 0)

		var got PayloadEAP
		if err := got.ReadFrom(buf[:n]); err != nil {
			t.Fatalf("ReadFrom code %d: %v", code, err)
		}
		if got.Code != code {
			t.Errorf("Code = %d, want %d", got.Code, code)
		}
		if len(got.EAPData) != 0 {
			t.Errorf("EAPData len = %d, want 0", len(got.EAPData))
		}
	}
}
