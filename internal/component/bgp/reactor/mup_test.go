package reactor

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	mup "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/nlri/mup"
)

// TestParseRD verifies Route Distinguisher parsing.
//
// VALIDATES: Type 0 (ASN:value) format
// VALIDATES: Type 1 (IP:value) format
// VALIDATES: Invalid formats return error
//
// PREVENTS: Malformed RD in wire format.
func TestParseRD(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType nlri.RDType
		wantErr  bool
	}{
		{
			name:     "Type 0 ASN:value",
			input:    "100:100",
			wantType: nlri.RDType(0),
			wantErr:  false,
		},
		{
			name:     "Type 1 IP:value",
			input:    "1.2.3.4:100",
			wantType: nlri.RDType(1),
			wantErr:  false,
		},
		{
			name:    "invalid format no colon",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "invalid format too many colons",
			input:   "1:2:3",
			wantErr: true,
		},
		{
			name:    "IPv6 in RD (invalid)",
			input:   "2001:db8::1:100",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd, err := nlri.ParseRDString(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if rd.Type != tt.wantType {
				t.Errorf("RD type = %d, want %d", rd.Type, tt.wantType)
			}
		})
	}
}

// TestBuildMUPPrefix verifies MUP prefix encoding.
//
// VALIDATES: Prefix length byte is correct
// VALIDATES: Only required prefix bytes are included
//
// PREVENTS: Wrong prefix length in wire format.
func TestBuildMUPPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   []byte
	}{
		{
			name:   "IPv4 /24",
			prefix: "10.0.1.0/24",
			want:   []byte{24, 10, 0, 1}, // bits, 3 bytes
		},
		{
			name:   "IPv4 /32",
			prefix: "10.0.1.1/32",
			want:   []byte{32, 10, 0, 1, 1}, // bits, 4 bytes
		},
		{
			name:   "IPv4 /8",
			prefix: "10.0.0.0/8",
			want:   []byte{8, 10}, // bits, 1 byte
		},
		{
			name:   "IPv6 /32",
			prefix: "2001:db8::/32",
			want:   []byte{32, 0x20, 0x01, 0x0d, 0xb8}, // bits, 4 bytes
		},
		{
			name:   "IPv6 /64",
			prefix: "2001:db8:1:1::/64",
			want:   []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x01}, // bits, 8 bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, err := netip.ParsePrefix(tt.prefix)
			if err != nil {
				t.Fatalf("invalid test prefix: %v", err)
			}
			got := make([]byte, mup.MUPPrefixLen(prefix))
			mup.WriteMUPPrefix(got, 0, prefix)
			if len(got) != len(tt.want) {
				t.Errorf("length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("byte[%d] = %02x, want %02x", i, got[i], tt.want[i])
				}
			}
		})
	}
}
