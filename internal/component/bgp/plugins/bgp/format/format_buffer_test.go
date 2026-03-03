package format

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatPrefixFromBytes verifies prefix formatting from wire bytes.
//
// VALIDATES: FormatPrefixFromBytes correctly formats prefix from raw NLRI bytes.
// PREVENTS: Incorrect prefix string from wire format.
func TestFormatPrefixFromBytes(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte // [prefixLen, prefix bytes...]
		isIPv6 bool
		want   string
	}{
		{
			name:   "ipv4 /8",
			data:   []byte{8, 10},
			isIPv6: false,
			want:   "10.0.0.0/8",
		},
		{
			name:   "ipv4 /24",
			data:   []byte{24, 192, 168, 1},
			isIPv6: false,
			want:   "192.168.1.0/24",
		},
		{
			name:   "ipv4 /32",
			data:   []byte{32, 10, 0, 0, 1},
			isIPv6: false,
			want:   "10.0.0.1/32",
		},
		{
			name:   "ipv4 /0 default",
			data:   []byte{0},
			isIPv6: false,
			want:   "0.0.0.0/0",
		},
		{
			name:   "ipv6 /32",
			data:   []byte{32, 0x20, 0x01, 0x0d, 0xb8},
			isIPv6: true,
			want:   "2001:db8::/32",
		},
		{
			name:   "ipv6 /64",
			data:   []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00},
			isIPv6: true,
			want:   "2001:db8:1::/64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPrefixFromBytes(tt.data, tt.isIPv6)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFormatASPathJSON verifies AS-PATH JSON formatting from wire bytes.
//
// VALIDATES: FormatASPathJSON writes correct JSON array from AS-PATH bytes.
// PREVENTS: Incorrect AS-PATH JSON representation.
func TestFormatASPathJSON(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		asn4 bool
		want string
	}{
		{
			name: "empty",
			data: []byte{},
			asn4: true,
			want: "[]",
		},
		{
			name: "single segment 4-byte",
			data: []byte{
				0x02, 0x03, // AS_SEQUENCE, 3 ASNs
				0x00, 0x00, 0xFD, 0xE9, // 65001
				0x00, 0x00, 0xFD, 0xEA, // 65002
				0x00, 0x00, 0xFD, 0xEB, // 65003
			},
			asn4: true,
			want: "[65001,65002,65003]",
		},
		{
			name: "single segment 2-byte",
			data: []byte{
				0x02, 0x02, // AS_SEQUENCE, 2 ASNs
				0xFD, 0xE9, // 65001
				0xFD, 0xEA, // 65002
			},
			asn4: false,
			want: "[65001,65002]",
		},
		{
			name: "as_set",
			data: []byte{
				0x01, 0x02, // AS_SET, 2 ASNs
				0x00, 0x00, 0x00, 0x01, // 1
				0x00, 0x00, 0x00, 0x02, // 2
			},
			asn4: true,
			want: "[{1,2}]", // AS_SET uses braces
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := FormatASPathJSON(tt.data, tt.asn4, &buf)
			require.NoError(t, err)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

// TestFormatCommunitiesJSON verifies community JSON formatting.
//
// VALIDATES: FormatCommunitiesJSON writes correct JSON array.
// PREVENTS: Incorrect community string representation.
func TestFormatCommunitiesJSON(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "empty",
			data: []byte{},
			want: "[]",
		},
		{
			name: "single community",
			data: []byte{0xFD, 0xE9, 0x00, 0x64}, // 65001:100
			want: `["65001:100"]`,
		},
		{
			name: "multiple communities",
			data: []byte{
				0xFD, 0xE9, 0x00, 0x64, // 65001:100
				0xFD, 0xEA, 0x00, 0xC8, // 65002:200
			},
			want: `["65001:100","65002:200"]`,
		},
		{
			name: "well-known no-export",
			data: []byte{0xFF, 0xFF, 0xFF, 0x01}, // NO_EXPORT
			want: `["no-export"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := FormatCommunitiesJSON(tt.data, &buf)
			require.NoError(t, err)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

// TestFormatOriginJSON verifies origin attribute formatting.
//
// VALIDATES: FormatOriginJSON returns correct origin string.
// PREVENTS: Incorrect origin code mapping.
func TestFormatOriginJSON(t *testing.T) {
	tests := []struct {
		value byte
		want  string
	}{
		{0, `"igp"`},
		{1, `"egp"`},
		{2, `"incomplete"`},
		{3, `"unknown"`}, // invalid, but should not panic
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		FormatOriginJSON(tt.value, &buf)
		assert.Equal(t, tt.want, buf.String())
	}
}

// TestFormatMEDJSON verifies MED attribute formatting.
//
// VALIDATES: FormatMEDJSON correctly formats 4-byte MED.
// PREVENTS: Incorrect byte order or value.
func TestFormatMEDJSON(t *testing.T) {
	tests := []struct {
		data []byte
		want string
	}{
		{[]byte{0, 0, 0, 100}, "100"},
		{[]byte{0, 0, 1, 0}, "256"},
		{[]byte{0, 1, 0, 0}, "65536"},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		FormatMEDJSON(tt.data, &buf)
		assert.Equal(t, tt.want, buf.String())
	}
}

// TestFormatLocalPrefJSON verifies LOCAL_PREF formatting.
//
// VALIDATES: FormatLocalPrefJSON correctly formats 4-byte value.
// PREVENTS: Incorrect byte order.
func TestFormatLocalPrefJSON(t *testing.T) {
	data := []byte{0, 0, 0, 100}
	var buf bytes.Buffer
	FormatLocalPrefJSON(data, &buf)
	assert.Equal(t, "100", buf.String())
}
