package wire

import (
	"bytes"
	"testing"
)

func TestPayloadVendorIDRoundtrip(t *testing.T) {
	vid := []byte("strongSwan-vendor-id-data")
	p := PayloadVendorID{VendorIDData: vid}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadVendorID
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if !bytes.Equal(got.VendorIDData, vid) {
		t.Errorf("VendorIDData = %q, want %q", got.VendorIDData, vid)
	}
}
