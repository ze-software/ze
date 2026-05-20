package wire

import (
	"bytes"
	"testing"
)

func TestPayloadAuthRoundtrip(t *testing.T) {
	tests := []struct {
		name   string
		method uint8
		data   []byte
	}{
		{"RSA sig", AuthMethodRSASig, []byte("rsa-signature-data-here")},
		{"PSK", AuthMethodPSK, []byte("psk-auth-data")},
		{"Digital Sig", AuthMethodDigitalSig, buildDigitalSigData()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PayloadAUTH{AuthMethod: tt.method, AuthData: tt.data}
			buf := make([]byte, 512)
			n := p.WriteTo(buf, 0)

			var got PayloadAUTH
			if err := got.ReadFrom(buf[:n]); err != nil {
				t.Fatalf("ReadFrom: %v", err)
			}
			if got.AuthMethod != tt.method {
				t.Errorf("AuthMethod = %d, want %d", got.AuthMethod, tt.method)
			}
			if !bytes.Equal(got.AuthData, tt.data) {
				t.Error("AuthData mismatch")
			}
		})
	}
}

func buildDigitalSigData() []byte {
	// RFC 7427: ASN.1 length + AlgorithmIdentifier + signature
	algID := []byte{0x30, 0x0d, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86,
		0xf7, 0x0d, 0x01, 0x01, 0x0b, 0x05, 0x00}
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = byte(i)
	}
	// ASN.1 length (1 byte) + AlgorithmIdentifier + signature
	result := make([]byte, 0, 1+len(algID)+len(sig))
	result = append(result, byte(len(algID)))
	result = append(result, algID...)
	result = append(result, sig...)
	return result
}
