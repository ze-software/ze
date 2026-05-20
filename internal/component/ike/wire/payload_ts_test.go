package wire

import (
	"bytes"
	"testing"
)

func TestPayloadTSRoundtrip(t *testing.T) {
	p := PayloadTS{
		TSPayloadType: PayloadTypeTSi,
		TrafficSelectors: []TrafficSelector{
			{
				TSType:       TSTypeIPv4AddrRange,
				IPProtocol:   0,
				StartPort:    0,
				EndPort:      65535,
				StartAddress: []byte{10, 0, 0, 0},
				EndAddress:   []byte{10, 0, 0, 255},
			},
			{
				TSType:       TSTypeIPv6AddrRange,
				IPProtocol:   6,
				StartPort:    80,
				EndPort:      443,
				StartAddress: make([]byte, 16),
				EndAddress:   []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			},
		},
	}

	buf := make([]byte, 512)
	n := p.WriteTo(buf, 0)

	got := PayloadTS{TSPayloadType: PayloadTypeTSi}
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if len(got.TrafficSelectors) != 2 {
		t.Fatalf("got %d selectors, want 2", len(got.TrafficSelectors))
	}

	ts0 := got.TrafficSelectors[0]
	if ts0.TSType != TSTypeIPv4AddrRange {
		t.Errorf("ts[0].TSType = %d, want %d", ts0.TSType, TSTypeIPv4AddrRange)
	}
	if ts0.StartPort != 0 || ts0.EndPort != 65535 {
		t.Errorf("ts[0] ports = %d-%d, want 0-65535", ts0.StartPort, ts0.EndPort)
	}
	if !bytes.Equal(ts0.StartAddress, []byte{10, 0, 0, 0}) {
		t.Errorf("ts[0].StartAddress = %v, want 10.0.0.0", ts0.StartAddress)
	}
	if !bytes.Equal(ts0.EndAddress, []byte{10, 0, 0, 255}) {
		t.Errorf("ts[0].EndAddress = %v, want 10.0.0.255", ts0.EndAddress)
	}

	ts1 := got.TrafficSelectors[1]
	if ts1.TSType != TSTypeIPv6AddrRange {
		t.Errorf("ts[1].TSType = %d, want %d", ts1.TSType, TSTypeIPv6AddrRange)
	}
	if ts1.IPProtocol != 6 {
		t.Errorf("ts[1].IPProtocol = %d, want 6", ts1.IPProtocol)
	}
	if ts1.StartPort != 80 || ts1.EndPort != 443 {
		t.Errorf("ts[1] ports = %d-%d, want 80-443", ts1.StartPort, ts1.EndPort)
	}
}
