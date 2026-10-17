package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPathID verifies the ADD-PATH Path Identifier parser config and
// `update text` share.
//
// VALIDATES: the decimal and the dotted a.b.c.d forms both name the four
// octets of RFC 7911 Section 3, the dotted octets in wire order.
// PREVENTS: one surface reading 0.0.0.1, the form ExaBGP and the interop
// helper write, while the other refuses it.
func TestPathID(t *testing.T) {
	tests := []struct {
		input string
		want  uint32
	}{
		{"0", 0},
		{"1", 1},
		{"42", 42},
		{"4294967295", 4294967295},
		{"0.0.0.0", 0},
		{"0.0.0.1", 1},
		{"1.2.3.4", 16909060},
		{"4.3.2.1", 67305985},
		{"255.255.255.255", 4294967295},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := PathID(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPathIDRefuses verifies a value that is neither form is refused by name.
//
// VALIDATES: a value wider than four octets, a negative, an IPv6 address, a
// short or out-of-range dotted form and junk each fail, and the error names
// the token and both accepted forms.
// PREVENTS: a mistyped identifier silently becoming a different path.
func TestPathIDRefuses(t *testing.T) {
	for _, input := range []string{
		"", "4294967296", "-1", "abc", "1.2.3", "1.2.3.256", "::1", "::ffff:1.2.3.4", "0x01",
	} {
		t.Run(input, func(t *testing.T) {
			_, err := PathID(input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"`+input+`"`)
			assert.Contains(t, err.Error(), "0..4294967295")
			assert.Contains(t, err.Error(), "a.b.c.d")
		})
	}
}
