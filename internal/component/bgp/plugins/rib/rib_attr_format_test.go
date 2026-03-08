package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatOrigin verifies ORIGIN byte to string conversion.
//
// VALIDATES: Raw pool bytes correctly mapped to RFC 4271 origin names.
// PREVENTS: Wrong origin name for IGP/EGP/INCOMPLETE values.
func TestFormatOrigin(t *testing.T) {
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
			assert.Equal(t, tt.want, formatOrigin(tt.data))
		})
	}
}

// TestFormatASPath verifies AS_PATH wire bytes to ASN slice conversion.
//
// VALIDATES: AS_SEQUENCE segments parsed into flat ASN list per RFC 4271.
// PREVENTS: AS_PATH corruption from segment header misparse.
func TestFormatASPath(t *testing.T) {
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
			assert.Equal(t, tt.want, formatASPath(tt.data))
		})
	}
}

// TestFormatUint32Attr verifies 4-byte big-endian to uint32 conversion.
//
// VALIDATES: MED and LOCAL_PREF raw bytes correctly converted to uint32.
// PREVENTS: Byte order confusion in numeric attribute parsing.
func TestFormatUint32Attr(t *testing.T) {
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
			got, ok := formatUint32Attr(tt.data)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestFormatCommunities verifies community 4-byte pairs to "high:low" strings.
//
// VALIDATES: RFC 1997 community wire format correctly converted to display format.
// PREVENTS: Byte offset errors in community pair parsing.
func TestFormatCommunities(t *testing.T) {
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
		{"empty", []byte{}, nil},
		{"nil", nil, nil},
		{"odd_bytes", []byte{0x01, 0x02, 0x03}, nil}, // not multiple of 4
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatCommunities(tt.data))
		})
	}
}
