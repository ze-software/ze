package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseExtendedCommunityHex verifies hex format parsing for extended communities.
//
// VALIDATES: ExaBGP-compatible 0x... format for extended communities (RFC 4360).
// ExaBGP outputs communities like "0x0002fde800000001" and accepts them in config.
//
// PREVENTS: Config rejection for valid ExaBGP configs using hex format,
// which breaks test Z (vpn) and real-world ExaBGP migration scenarios.
func TestParseExtendedCommunityHex(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantBytes []byte
		wantErr   bool
		errMsg    string
	}{
		{
			name:  "valid hex - route target type 2",
			input: "0x0002fde800000001",
			// Type=0x00 (2-byte AS), Subtype=0x02 (RT), ASN=0xfde8 (65000), Value=0x00000001
			wantBytes: []byte{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01},
		},
		{
			name:      "valid hex - uppercase",
			input:     "0X0002FDE800000001",
			wantBytes: []byte{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01},
		},
		{
			name:  "valid hex - mixed case",
			input: "0x0002271000000001",
			// Type=0x00, Subtype=0x02, ASN=0x2710 (10000), Value=0x00000001
			wantBytes: []byte{0x00, 0x02, 0x27, 0x10, 0x00, 0x00, 0x00, 0x01},
		},
		{
			name:    "invalid - odd length",
			input:   "0x0002fde8000001",
			wantErr: true,
			errMsg:  "hex format must be 16 chars",
		},
		{
			name:    "invalid - too short",
			input:   "0x0002",
			wantErr: true,
			errMsg:  "hex format must be 16 chars",
		},
		{
			name:    "invalid - too long",
			input:   "0x0002fde80000000100",
			wantErr: true,
			errMsg:  "hex format must be 16 chars",
		},
		{
			name:    "invalid - not hex",
			input:   "0xGGGGHHHHIIIIJJJJ",
			wantErr: true,
			errMsg:  "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec, err := ParseExtendedCommunity(tt.input)
			if tt.wantErr {
				require.Error(t, err, "expected error for input %q", tt.input)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err, "unexpected error for input %q", tt.input)
			assert.Equal(t, tt.wantBytes, ec.Bytes, "bytes mismatch for input %q", tt.input)
		})
	}
}

// TestParseExtendedCommunityHexList verifies parsing multiple hex communities in brackets.
//
// VALIDATES: ExaBGP config format with bracketed list of hex communities.
//
// PREVENTS: Failure parsing configs like: extended-community [ 0x0002fde800000001 0x0002271000000001 ].
func TestParseExtendedCommunityHexList(t *testing.T) {
	input := "[ 0x0002fde800000001 0x0002271000000001 ]"
	ec, err := ParseExtendedCommunity(input)
	require.NoError(t, err)

	// Should have 16 bytes (2 communities * 8 bytes each)
	require.Len(t, ec.Bytes, 16, "expected 16 bytes for 2 communities")

	// First community
	assert.Equal(t, []byte{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01}, ec.Bytes[0:8])
	// Second community
	assert.Equal(t, []byte{0x00, 0x02, 0x27, 0x10, 0x00, 0x00, 0x00, 0x01}, ec.Bytes[8:16])
}

// TestParsePrefixSIDSRv6Integration verifies SRv6 Prefix-SID parsing flow.
//
// VALIDATES: bgp-prefix-sid-srv6 config field is correctly parsed to bytes.
// PREVENTS: Silent drop of SRv6 Prefix-SID when loading VPN routes from config.
func TestParsePrefixSIDSRv6Integration(t *testing.T) {
	src := StaticRouteConfig{
		PrefixSID: "l3-service 2001:1:0:0::",
	}
	attrs, err := ParseRouteAttributes(&src)
	require.NoError(t, err)
	require.NotNil(t, attrs)
	assert.NotEmpty(t, attrs.PrefixSID.Bytes, "PrefixSID bytes should not be empty for SRv6 format")
	t.Logf("PrefixSID bytes: %x", attrs.PrefixSID.Bytes)
}
