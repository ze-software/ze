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

// TestParseExtendedCommunityRateLimitPackets verifies RFC 8955 packet-rate action parsing.
//
// VALIDATES: ExaBGP current unit syntax and 5.0 rate-limit-packets alias map to subtype 0x0c.
//
// PREVENTS: Migrated ExaBGP FlowSpec configs losing packet-rate actions.
func TestParseExtendedCommunityRateLimitPackets(t *testing.T) {
	ec, err := ParseExtendedCommunity("[ rate-limit:1000:packets ]")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x80, 0x0c, 0x00, 0x00, 0x44, 0x7a, 0x00, 0x00}, ec.Bytes)

	ec, err = ParseExtendedCommunity("[ rate-limit-packets:1000 ]")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x80, 0x0c, 0x00, 0x00, 0x44, 0x7a, 0x00, 0x00}, ec.Bytes)

	_, err = ParseExtendedCommunity("[ rate-limit:1:bits ]")
	require.Error(t, err)

	_, err = ParseExtendedCommunity("[ rate-limit:1:packets ]")
	require.NoError(t, err)

	_, err = ParseExtendedCommunity("[ rate-limit:-1:packets ]")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be negative")

	// rate-limit:N:bytes is explicit bytes unit, equivalent to bare rate-limit:N (subtype 0x06).
	ec, err = ParseExtendedCommunity("[ rate-limit:9600:bytes ]")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x80, 0x06, 0x00, 0x00}, ec.Bytes[:4])

	// Bare rate-limit:N and rate-limit:N:bytes produce identical wire bytes.
	ecBare, err := ParseExtendedCommunity("[ rate-limit:9600 ]")
	require.NoError(t, err)
	assert.Equal(t, ecBare.Bytes, ec.Bytes)
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

// TestParsePrefixSIDSRv6_ReservedFieldsZero verifies that the SRv6 Prefix-SID
// encoder emits every RFC 9252 reserved/flags octet as zero. The wire layout of
// ParsePrefixSIDSRv6("l3-service <sid>") is:
//
//	[0]=Type(5) [1..2]=Len [3]=Service-TLV Reserved
//	[4]=SubTLV-Type(1) [5..6]=SubTLV-Len [7]=RESERVED1
//	[8..23]=SID(16) [24]=Service-SID-Flags [25..26]=Behavior [27]=RESERVED2
//
// RFC requirement: RFC9252-3.1-1 positive -- Service TLV Reserved octet is emitted as 0 by the sender.
// RFC requirement: RFC9252-3.2-1 positive -- SID Information Sub-TLV RESERVED1 is emitted as 0 by the sender.
// RFC requirement: RFC9252-3.2-2 positive -- SID Information Sub-TLV Service SID Flags octet is emitted as 0 by the sender.
// RFC requirement: RFC9252-3.2-3 positive -- SID Information Sub-TLV RESERVED2 is emitted as 0 by the sender.
func TestParsePrefixSIDSRv6_ReservedFieldsZero(t *testing.T) {
	// Behavior 0x003e (End.DT6) forces a non-zero Behavior field so the assertions
	// on the surrounding reserved/flags octets are not trivially satisfied by an
	// all-zero value region.
	ps, err := ParsePrefixSIDSRv6("l3-service 2001:db8::1 0x3e")
	require.NoError(t, err)
	require.Len(t, ps.Bytes, 28, "unexpected SRv6 Prefix-SID wire length")

	assert.Equal(t, byte(5), ps.Bytes[0], "outer TLV type must be 5 (L3 Service)")
	assert.EqualValues(t, 0, ps.Bytes[3], "RFC9252-3.1-1: Service TLV Reserved must be 0")
	assert.Equal(t, byte(1), ps.Bytes[4], "sub-TLV type must be 1 (SID Information)")
	assert.EqualValues(t, 0, ps.Bytes[7], "RFC9252-3.2-1: RESERVED1 must be 0")
	assert.EqualValues(t, 0, ps.Bytes[24], "RFC9252-3.2-2: Service SID Flags must be 0")
	// Behavior field carries the requested 0x003e, proving the octets around it
	// are independently zeroed rather than part of a zero-filled buffer.
	assert.Equal(t, []byte{0x00, 0x3e}, ps.Bytes[25:27], "Behavior must round-trip")
	assert.EqualValues(t, 0, ps.Bytes[27], "RFC9252-3.2-3: RESERVED2 must be 0")
}

// TestRFC8955TrafficActionUnusedBitsZero verifies the FlowSpec traffic-action extended
// community is encoded with every unused bit of its 6-octet action field clear.
//
// VALIDATES: parseFlowSpecAction (routeattr_community.go) emits the community as
// 0x80,0x07 followed by five literal zero octets and a final octet carrying only the
// Terminal (0x01) and Sample (0x02) bits, so bits 0..45 of the Traffic Action Field are
// always zero whatever the operator wrote.
//
// PREVENTS: stray bits in the reserved Traffic Action Field, which a receiver that later
// assigns those codepoints would read as an action the operator never configured.
func TestRFC8955TrafficActionUnusedBitsZero(t *testing.T) {
	// rfc-test-change-approved: 2026-08-19 Thomas ruled that "bogus" should not be
	// supported. It sat in this list only because the parser accepted any word, and
	// it asserted that a typo yields a real traffic-action community with S=T=0
	// rather than an error, which is the defect rather than the requirement. The
	// three valid specs prove the reserved bits are zero, and "none" replaces it
	// with a fourth distinct bit pattern, 0x00, where the final octet must be
	// entirely clear. The tag, the assertions and the polarity are untouched.
	for _, spec := range []string{"sample", "terminal", "sample-terminal", "none"} {
		ec, err := ParseExtendedCommunity("[ action " + spec + " ]")
		require.NoError(t, err, "spec %q", spec)
		require.GreaterOrEqual(t, len(ec.Bytes), 8, "spec %q", spec)
		got := ec.Bytes[:8]

		assert.Equal(t, byte(0x80), got[0], "spec %q: type octet", spec)
		assert.Equal(t, byte(0x07), got[1], "spec %q: traffic-action sub-type", spec)

		// RFC 8955 Section 7.3: the Traffic Action Field is reserved apart from the
		// Terminal (bit 47) and Sample (bit 46) bits; the unused bits "MUST be set to 0
		// on encoding".
		// RFC requirement: RFC8955-7.3-1 positive -- traffic-action unused bits are zero on encoding (§7.3)
		for i := 2; i <= 6; i++ {
			assert.Zero(t, got[i], "spec %q: traffic-action unused octet %d must be 0", spec, i)
		}
		assert.Zero(t, got[7]&^byte(0x03),
			"spec %q: only the Terminal and Sample bits may be set in the final octet", spec)
	}
}
