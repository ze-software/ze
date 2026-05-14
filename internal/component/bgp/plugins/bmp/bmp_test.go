package bmp

import (
	"testing"
)

func TestBMPCompositeKeyFormat(t *testing.T) {
	tests := []struct {
		name   string
		router string
		peer   PeerHeader
		want   string
	}{
		{
			name:   "ipv4 peer",
			router: "10.0.0.1:12345",
			peer: PeerHeader{
				Address: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 192, 168, 1, 1},
				Flags:   0,
			},
			want: "10.0.0.1:12345:192.168.1.1",
		},
		{
			name:   "ipv6 peer",
			router: "10.0.0.1:12345",
			peer: PeerHeader{
				Address: [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				Flags:   PeerFlagV,
			},
			want: "10.0.0.1:12345:2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bmpCompositeKey(tt.router, tt.peer)
			if got != tt.want {
				t.Errorf("bmpCompositeKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPeerAddressString(t *testing.T) {
	tests := []struct {
		name string
		peer PeerHeader
		want string
	}{
		{
			name: "ipv4",
			peer: PeerHeader{
				Address: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 10, 0, 0, 1},
			},
			want: "10.0.0.1",
		},
		{
			name: "ipv6",
			peer: PeerHeader{
				Address: [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				Flags:   PeerFlagV,
			},
			want: "2001:db8::1",
		},
		{
			name: "ipv4 zero",
			peer: PeerHeader{
				Address: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			},
			want: "0.0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := peerAddressString(tt.peer)
			if got != tt.want {
				t.Errorf("peerAddressString() = %q, want %q", got, tt.want)
			}
		})
	}
}
