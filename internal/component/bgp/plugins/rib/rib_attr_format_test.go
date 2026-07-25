package rib

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestFormatOrigin verifies ORIGIN byte to string conversion.
//
// VALIDATES: Raw pool bytes correctly mapped to RFC 4271 origin names.
// PREVENTS: Wrong origin name for IGP/EGP/INCOMPLETE values.
func TestFormatNextHop(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"ipv4", []byte{10, 0, 0, 1}, "10.0.0.1"},
		{"ipv6", []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}, "2001:db8::1"},
		{"ipv6_loopback", []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, "::1"},
		{"odd_length", []byte{0xaa, 0xbb, 0xcc}, "aabbcc"},
		{"nil", nil, ""},
		{"empty", []byte{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, formatNextHop(tt.data))
		})
	}
}

func TestFormatOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"igp", []byte{0x00}, "igp"},
		{"egp", []byte{0x01}, "egp"},
		{"incomplete", []byte{0x02}, "incomplete"},
		{"unknown_3", []byte{0x03}, "unknown(3)"},
		{"empty", []byte{}, ""},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, formatOrigin(tt.data))
		})
	}
}

// TestFormatASPath verifies AS_PATH wire bytes to ASN slice conversion.
//
// VALIDATES: AS_SEQUENCE segments parsed into flat ASN list per RFC 4271.
// PREVENTS: AS_PATH corruption from segment header misparse.
func TestFormatASPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		want []uint32
	}{
		{
			"single_asn",
			// AS_SEQUENCE: type=2, count=1, ASN=65001
			[]byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9},
			[]uint32{65001},
		},
		{
			"two_asns",
			// AS_SEQUENCE: type=2, count=2, ASN=65001, ASN=65002
			[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA},
			[]uint32{65001, 65002},
		},
		{
			"two_segments",
			// AS_SEQUENCE: type=2, count=1, ASN=65001
			// AS_SEQUENCE: type=2, count=1, ASN=65002
			[]byte{
				0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9,
				0x02, 0x01, 0x00, 0x00, 0xFD, 0xEA,
			},
			[]uint32{65001, 65002},
		},
		{"empty", []byte{}, nil},
		{"nil", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, formatASPath(tt.data))
		})
	}
}

// TestFormatUint32Attr verifies 4-byte big-endian to uint32 conversion.
//
// VALIDATES: MED and LOCAL_PREF raw bytes correctly converted to uint32.
// PREVENTS: Byte order confusion in numeric attribute parsing.
func TestFormatUint32Attr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		want uint32
		ok   bool
	}{
		{"value_100", []byte{0x00, 0x00, 0x00, 0x64}, 100, true},
		{"value_0", []byte{0x00, 0x00, 0x00, 0x00}, 0, true},
		{"max_u32", []byte{0xFF, 0xFF, 0xFF, 0xFF}, 4294967295, true},
		{"too_short", []byte{0x00, 0x00}, 0, false},
		{"empty", []byte{}, 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := formatUint32Attr(tt.data)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestEnrichCommunityJSONByteIdentical verifies that the lazy MarshalJSON
// wrappers produce byte-identical JSON to the old []string approach.
//
// VALIDATES: AC-1 — JSON output byte-identical across community kinds,
// well-known names, zero/one/many elements.
// PREVENTS: Wrapper output diverging from the established JSON format.
func TestEnrichCommunityJSONByteIdentical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		communities      []attribute.Community
		largeCommunities []attribute.LargeCommunity
		extCommunities   []attribute.ExtendedCommunity
	}{
		{
			"single_standard",
			[]attribute.Community{attribute.Community(65000<<16 | 100)},
			nil, nil,
		},
		{
			"well_known_no_export",
			[]attribute.Community{attribute.CommunityNoExport},
			nil, nil,
		},
		{
			"mixed_wellknown_and_numeric",
			[]attribute.Community{
				attribute.Community(65000<<16 | 100),
				attribute.CommunityNoExport,
				attribute.CommunityNoAdvertise,
				attribute.Community(1<<16 | 2),
			},
			nil, nil,
		},
		{
			"large_communities",
			nil,
			[]attribute.LargeCommunity{
				{GlobalAdmin: 65000, LocalData1: 100, LocalData2: 200},
				{GlobalAdmin: 1, LocalData1: 0, LocalData2: 0},
			},
			nil,
		},
		{
			"extended_communities",
			nil, nil,
			[]attribute.ExtendedCommunity{
				{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64},
				{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			},
		},
		{
			"all_three_kinds",
			[]attribute.Community{attribute.Community(65000<<16 | 100)},
			[]attribute.LargeCommunity{{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2}},
			[]attribute.ExtendedCommunity{{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Build "old" style: []string values
			oldMap := make(map[string]any)
			if len(tt.communities) > 0 {
				strs := make([]string, len(tt.communities))
				for i, c := range tt.communities {
					strs[i] = c.String()
				}
				oldMap["community"] = attrWithFlags(strs, attribute.FlagOptional|attribute.FlagTransitive)
			}
			if len(tt.largeCommunities) > 0 {
				strs := make([]string, len(tt.largeCommunities))
				for i, lc := range tt.largeCommunities {
					strs[i] = lc.String()
				}
				oldMap["large-community"] = attrWithFlags(strs, attribute.FlagOptional|attribute.FlagTransitive)
			}
			if len(tt.extCommunities) > 0 {
				strs := make([]string, len(tt.extCommunities))
				for i, ec := range tt.extCommunities {
					strs[i] = textbuf.StringHex(ec[:])
				}
				oldMap["extended-community"] = attrWithFlags(strs, attribute.FlagOptional|attribute.FlagTransitive)
			}

			// Build "new" style: wrapper values via enrichRouteMapFromRoute
			rt := &Route{
				Communities:         tt.communities,
				LargeCommunities:    tt.largeCommunities,
				ExtendedCommunities: tt.extCommunities,
			}
			newMap := make(map[string]any)
			enrichRouteMapFromRoute(newMap, rt)

			oldJSON, err := json.Marshal(oldMap)
			if err != nil {
				t.Fatalf("old marshal: %v", err)
			}
			newJSON, err := json.Marshal(newMap)
			if err != nil {
				t.Fatalf("new marshal: %v", err)
			}

			if !bytes.Equal(oldJSON, newJSON) {
				t.Errorf("JSON mismatch:\n old: %s\n new: %s", oldJSON, newJSON)
			}
		})
	}
}

// TestFormatCommunities verifies community 4-byte pairs to display strings.
//
// VALIDATES: RFC 1997 community wire format correctly converted to display format.
// Well-known communities (NO_EXPORT, NO_ADVERTISE, etc.) resolve to names.
// PREVENTS: Byte offset errors in community pair parsing.
func TestFormatCommunities(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		want []string
	}{
		{
			"single_community",
			// 65000:100 = 0xFDE8:0x0064
			[]byte{0xFD, 0xE8, 0x00, 0x64},
			[]string{"65000:100"},
		},
		{
			"two_communities",
			[]byte{0xFD, 0xE8, 0x00, 0x64, 0x00, 0x01, 0x00, 0x02},
			[]string{"65000:100", "1:2"},
		},
		{
			"no_export",
			[]byte{0xFF, 0xFF, 0xFF, 0x01},
			[]string{"no-export"},
		},
		{
			"no_advertise",
			[]byte{0xFF, 0xFF, 0xFF, 0x02},
			[]string{"no-advertise"},
		},
		{
			"mixed_wellknown_and_normal",
			[]byte{0xFD, 0xE8, 0x00, 0x64, 0xFF, 0xFF, 0xFF, 0x01},
			[]string{"65000:100", "no-export"},
		},
		{"empty", []byte{}, nil},
		{"nil", nil, nil},
		{"odd_bytes", []byte{0x01, 0x02, 0x03}, nil}, // not multiple of 4
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, formatCommunities(tt.data))
		})
	}
}
