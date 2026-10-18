// VALIDATES: DecodeNLRIHex reads the RFC 9830 Section 2.1 wire form of an MP NLRI
// section: the NLRI Length octet first, checked against the AFI, then the
// distinguisher, color and endpoint, for each NLRI packed in the section.
// PREVENTS: the Length octet read as the first distinguisher octet (distinguisher
// 0x60000000), a wrong length accepted, a truncated section read past its end,
// and every NLRI after the first in a packed section dropped.

package srpolicy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wire forms used across the tests, one NLRI each.
const (
	// 60 = 96 bits; distinguisher 0; color 100; endpoint 10.0.0.1.
	wireIPv4Policy = "60" + "00000000" + "00000064" + "0A000001"
	// C0 = 192 bits; distinguisher 42; color 200; endpoint 2001:db8::1.
	wireIPv6Policy = "C0" + "0000002A" + "000000C8" + "20010DB8000000000000000000000001"
	// 60 = 96 bits; distinguisher 7; color 300; endpoint 10.0.0.3.
	wireIPv4PolicyB = "60" + "00000007" + "0000012C" + "0A000003"
)

// TestDecodeNLRIHexWireForm decodes one NLRI of each AFI in its wire form.
//
// Goal: the fields land where RFC 9830 Section 2.1 puts them, past the Length
// octet. Method: explicit hex, one case per AFI, each field asserted.
func TestDecodeNLRIHexWireForm(t *testing.T) {
	tests := []struct {
		name          string
		family        string
		hex           string
		distinguisher uint32
		color         uint32
		endpoint      string
	}{
		{"ipv4", "ipv4/sr-policy", wireIPv4Policy, 0, 100, "10.0.0.1"},
		{"ipv6", "ipv6/sr-policy", wireIPv6Policy, 42, 200, "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeNLRIHex(tt.family, tt.hex, false, false)
			require.NoError(t, err)
			m, ok := result.(map[string]any)
			require.True(t, ok, "one NLRI decodes to one object, got %T", result)
			assert.Equal(t, tt.distinguisher, m[fieldDistinguisher])
			assert.Equal(t, tt.color, m[fieldColor])
			assert.Equal(t, tt.endpoint, m[fieldEndpoint])
			_, hasPathID := m["path-id"]
			assert.False(t, hasPathID, "no ADD-PATH, so no path-id")
		})
	}
}

// TestDecodeNLRIHexPacked decodes a section of two NLRIs.
//
// Goal: every NLRI of an MP_REACH or MP_UNREACH section is shown, in wire
// order. Method: two IPv4 NLRIs concatenated, then the same pair under ADD-PATH.
func TestDecodeNLRIHexPacked(t *testing.T) {
	result, err := DecodeNLRIHex("ipv4/sr-policy", wireIPv4Policy+wireIPv4PolicyB, false, false)
	require.NoError(t, err)
	list, ok := result.([]map[string]any)
	require.True(t, ok, "two NLRIs decode to a list, got %T", result)
	require.Len(t, list, 2)
	assert.Equal(t, uint32(0), list[0][fieldDistinguisher])
	assert.Equal(t, uint32(100), list[0][fieldColor])
	assert.Equal(t, "10.0.0.1", list[0][fieldEndpoint])
	assert.Equal(t, uint32(7), list[1][fieldDistinguisher])
	assert.Equal(t, uint32(300), list[1][fieldColor])
	assert.Equal(t, "10.0.0.3", list[1][fieldEndpoint])

	// RFC 7911 Section 3: a 4-octet Path Identifier precedes each NLRI.
	result, err = DecodeNLRIHex("ipv4/sr-policy", "00000001"+wireIPv4Policy+"00000002"+wireIPv4PolicyB, true, false)
	require.NoError(t, err)
	list, ok = result.([]map[string]any)
	require.True(t, ok, "two NLRIs decode to a list, got %T", result)
	require.Len(t, list, 2)
	assert.Equal(t, uint32(1), list[0]["path-id"])
	assert.Equal(t, uint32(0), list[0][fieldDistinguisher])
	assert.Equal(t, uint32(2), list[1]["path-id"])
	assert.Equal(t, uint32(7), list[1][fieldDistinguisher])
}

// TestDecodeNLRIHexRefused feeds malformed sections.
//
// Goal: a section a peer can send but RFC 9830 Section 2.1 does not allow is
// refused with an error naming the problem, never decoded and never a panic.
// Method: one case per defect, each asserting the error text.
func TestDecodeNLRIHexRefused(t *testing.T) {
	tests := []struct {
		name   string
		family string
		hex    string
		addP   bool
		want   string
	}{
		{"body without length octet", "ipv4/sr-policy", "00000000000000640A000001", false, "length 0 bits"},
		{"ipv6 length on ipv4", "ipv4/sr-policy", wireIPv6Policy, false, "length 192 bits"},
		{"ipv4 length on ipv6", "ipv6/sr-policy", wireIPv4Policy + "00000000000000000000000000000000", false, "length 96 bits"},
		{"wrong second length", "ipv4/sr-policy", wireIPv4Policy + "58" + "0000000000000064" + "0A0000", false, "length 88 bits"},
		{"truncated body", "ipv4/sr-policy", wireIPv4Policy[:len(wireIPv4Policy)-2], false, "extends past"},
		{"truncated second NLRI", "ipv4/sr-policy", wireIPv4Policy + "60000000", false, "extends past"},
		{"length octet only", "ipv6/sr-policy", "C0", false, "extends past"},
		{"path id cut short", "ipv4/sr-policy", "000000", true, "path identifier"},
		{"empty section", "ipv4/sr-policy", "", false, "no NLRI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeNLRIHex(tt.family, tt.hex, tt.addP, false)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.True(t, strings.Contains(err.Error(), tt.want), "error %q lacks %q", err, tt.want)
		})
	}
}

// FuzzDecodeSRPolicySection feeds arbitrary bytes to the section decoder.
//
// VALIDATES: decodeSRPolicySection never panics, and every NLRI it returns
// came from a Length octet that matches the AFI.
// PREVENTS: a peer-controlled section crashing the daemon or the decoder.
// SECURITY: SR Policy NLRI bytes come from an untrusted BGP UPDATE.
func FuzzDecodeSRPolicySection(f *testing.F) {
	f.Add([]byte{0x60, 0, 0, 0, 0, 0, 0, 0, 0x64, 10, 0, 0, 1}, false, false)
	f.Add([]byte{0, 0, 0, 1, 0x60, 0, 0, 0, 0, 0, 0, 0, 0x64, 10, 0, 0, 1}, true, false)
	f.Add([]byte{0xC0, 0, 0, 0, 0x2A, 0, 0, 0, 0xC8, 0x20, 1, 0x0D, 0xB8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, false, true)
	f.Add([]byte{}, false, false)
	f.Add([]byte{0x60}, false, false)
	f.Add([]byte{0xFF, 0xFF}, false, true)

	f.Fuzz(func(t *testing.T, data []byte, addPath, ipv6 bool) {
		fam := IPv4SRPolicy
		if ipv6 {
			fam = IPv6SRPolicy
		}
		entries, err := decodeSRPolicySection(fam.AFI, data, addPath)
		if err != nil {
			return
		}
		require.NotEmpty(t, entries)
		for _, e := range entries {
			assert.Equal(t, fam.AFI, e.policy.afi)
		}
	})
}
