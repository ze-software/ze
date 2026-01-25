package reactor

import (
	"net/netip"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
)

// TestBuildAPIMUPNLRI_FamilyValidation verifies family mismatch is rejected.
//
// VALIDATES: IPv6 prefix/address with IPv4 AFI returns error
// VALIDATES: IPv4 prefix/address with IPv6 AFI returns error
//
// PREVENTS: Malformed wire format where NLRI bytes don't match declared AFI.
func TestBuildAPIMUPNLRI_FamilyValidation(t *testing.T) {
	tests := []struct {
		name    string
		spec    plugin.MUPRouteSpec
		wantErr string
	}{
		{
			name: "ISD IPv6 prefix with IPv4 AFI",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-isd",
				IsIPv6:    false, // IPv4 AFI
				Prefix:    "2001:db8::/32",
				RD:        "100:100",
			},
			wantErr: "not IPv4",
		},
		{
			name: "ISD IPv4 prefix with IPv6 AFI",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-isd",
				IsIPv6:    true, // IPv6 AFI
				Prefix:    "10.0.1.0/24",
				RD:        "100:100",
			},
			wantErr: "not IPv6",
		},
		{
			name: "DSD IPv6 address with IPv4 AFI",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-dsd",
				IsIPv6:    false, // IPv4 AFI
				Address:   "2001:db8::1",
				RD:        "100:100",
			},
			wantErr: "not IPv4",
		},
		{
			name: "DSD IPv4 address with IPv6 AFI",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-dsd",
				IsIPv6:    true, // IPv6 AFI
				Address:   "10.0.0.1",
				RD:        "100:100",
			},
			wantErr: "not IPv6",
		},
		{
			name: "T1ST IPv6 prefix with IPv4 AFI",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-t1st",
				IsIPv6:    false, // IPv4 AFI
				Prefix:    "2001:db8::/32",
				RD:        "100:100",
			},
			wantErr: "not IPv4",
		},
		{
			name: "T2ST IPv6 address with IPv4 AFI",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-t2st",
				IsIPv6:    false, // IPv4 AFI
				Address:   "2001:db8::1",
				RD:        "100:100",
			},
			wantErr: "not IPv4",
		},
		// Valid cases - should NOT error
		{
			name: "ISD IPv4 prefix with IPv4 AFI (valid)",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-isd",
				IsIPv6:    false,
				Prefix:    "10.0.1.0/24",
				RD:        "100:100",
			},
			wantErr: "", // no error
		},
		{
			name: "ISD IPv6 prefix with IPv6 AFI (valid)",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-isd",
				IsIPv6:    true,
				Prefix:    "2001:db8::/32",
				RD:        "100:100",
			},
			wantErr: "", // no error
		},
		{
			name: "DSD IPv4 address with IPv4 AFI (valid)",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-dsd",
				IsIPv6:    false,
				Address:   "10.0.0.1",
				RD:        "100:100",
			},
			wantErr: "", // no error
		},
		{
			name: "DSD IPv6 address with IPv6 AFI (valid)",
			spec: plugin.MUPRouteSpec{
				RouteType: "mup-dsd",
				IsIPv6:    true,
				Address:   "2001:db8::1",
				RD:        "100:100",
			},
			wantErr: "", // no error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildAPIMUPNLRI(tt.spec)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

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
			rd, err := parseRD(tt.input)
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
			got := buildMUPPrefix(prefix)
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

// TestParseAPIExtCommunity verifies extended community parsing.
//
// VALIDATES: target:ASN:value format
// VALIDATES: Brackets are stripped
//
// PREVENTS: Malformed extended community in wire format.
func TestParseAPIExtCommunity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{
			name:    "target with brackets",
			input:   "[target:10:10]",
			wantLen: 8,
			wantErr: false,
		},
		{
			name:    "target without brackets",
			input:   "target:10:10",
			wantLen: 8,
			wantErr: false,
		},
		{
			name:    "origin community",
			input:   "[origin:100:200]",
			wantLen: 8,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAPIExtCommunity(tt.input)
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
			if len(got) != tt.wantLen {
				t.Errorf("length = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}
