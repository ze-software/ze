package wire

import (
	"bytes"
	"testing"
)

func TestPayloadIDRoundtrip(t *testing.T) {
	tests := []struct {
		name   string
		ptype  uint8
		idType uint8
		data   []byte
	}{
		{"IDi FQDN", PayloadTypeIDi, IDTypeFQDN, []byte("vpn.example.com")},
		{"IDr IPv4", PayloadTypeIDr, IDTypeIPv4Addr, []byte{10, 0, 0, 1}},
		{"IDi RFC822", PayloadTypeIDi, IDTypeRFC822Addr, []byte("user@example.com")},
		{"IDi KeyID", PayloadTypeIDi, IDTypeKeyID, []byte{0x01, 0x02, 0x03}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PayloadID{
				IDPayloadType: tt.ptype,
				IDType:        tt.idType,
				IDData:        tt.data,
			}
			buf := make([]byte, 256)
			n := p.WriteTo(buf, 0)

			got := PayloadID{IDPayloadType: tt.ptype}
			if err := got.ReadFrom(buf[:n]); err != nil {
				t.Fatalf("ReadFrom: %v", err)
			}
			if got.IDType != tt.idType {
				t.Errorf("IDType = %d, want %d", got.IDType, tt.idType)
			}
			if !bytes.Equal(got.IDData, tt.data) {
				t.Errorf("IDData = %q, want %q", got.IDData, tt.data)
			}
		})
	}
}
