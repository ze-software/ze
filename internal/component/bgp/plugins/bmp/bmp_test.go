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

// TestBMPCollectorSourceAddress verifies the collector's source-address leaf is
// parsed from the config JSON into collectorConfig, and that a collector without
// it leaves the field empty (OS-selected source).
//
// VALIDATES: AC-3/AC-4 -- source-address flows from YANG-derived JSON to the
// collector config that the sender dialer binds as LocalAddr.
// PREVENTS: the leaf being accepted by YANG but dropped before the dialer.
func TestBMPCollectorSourceAddress(t *testing.T) {
	data := `{"bgp":{"bmp":{"sender":{"collector":{
		"withsrc":{"address":"10.0.0.100","port":"11019","source-address":"192.168.1.1"},
		"nosrc":{"address":"10.0.0.101","port":"11019"}
	}}}}}`

	cfg, err := parseSenderConfig(data)
	if err != nil {
		t.Fatalf("parseSenderConfig: %v", err)
	}

	withSrc, ok := cfg.Collectors["withsrc"]
	if !ok {
		t.Fatal("collector \"withsrc\" missing")
	}
	if withSrc.SourceAddress != "192.168.1.1" {
		t.Errorf("SourceAddress = %q, want %q", withSrc.SourceAddress, "192.168.1.1")
	}

	noSrc, ok := cfg.Collectors["nosrc"]
	if !ok {
		t.Fatal("collector \"nosrc\" missing")
	}
	if noSrc.SourceAddress != "" {
		t.Errorf("SourceAddress = %q, want empty (unconfigured)", noSrc.SourceAddress)
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
