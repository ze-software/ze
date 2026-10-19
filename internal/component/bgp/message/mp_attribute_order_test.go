package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mpOrderAttr builds one path attribute with a one-octet length.
func mpOrderAttr(flags, code byte, value ...byte) []byte {
	return append([]byte{flags, code, byte(len(value))}, value...)
}

// mpOrderJoin concatenates attributes into one path attribute section.
func mpOrderJoin(attrs ...[]byte) []byte {
	var out []byte
	for _, a := range attrs {
		out = append(out, a...)
	}
	return out
}

var (
	mpOrderOrigin  = mpOrderAttr(0x40, 1, 0x00)
	mpOrderMED     = mpOrderAttr(0x80, 4, 0, 0, 0, 10)
	mpOrderReach   = mpOrderAttr(0x80, 14, 0x00, 0x02, 0x01, 0x00, 0x00, 0x00)
	mpOrderUnreach = mpOrderAttr(0x80, 15, 0x00, 0x02, 0x01)
	// An extended-length attribute proves the header size is read from the flags.
	mpOrderLong = append([]byte{0x90, 32, 0x00, 0x0C}, make([]byte, 12)...)
)

// VALIDATES: MPAttributesTrail answers true exactly when an MP attribute follows
// another attribute or MP_REACH_NLRI precedes MP_UNREACH_NLRI.
// PREVENTS: the relay reordering a compliant UPDATE (losing its zero-copy forward)
// or leaving a trailing MP attribute in place.
func TestMPAttributesTrail(t *testing.T) {
	cases := []struct {
		name  string
		attrs []byte
		want  bool
	}{
		{"empty", nil, false},
		{"no MP attribute", mpOrderJoin(mpOrderOrigin, mpOrderMED), false},
		{"MP_REACH first", mpOrderJoin(mpOrderReach, mpOrderOrigin, mpOrderMED), false},
		{"MP_UNREACH first", mpOrderJoin(mpOrderUnreach, mpOrderOrigin), false},
		{"MP_UNREACH then MP_REACH", mpOrderJoin(mpOrderUnreach, mpOrderReach, mpOrderOrigin), false},
		{"MP_REACH after ORIGIN", mpOrderJoin(mpOrderOrigin, mpOrderReach, mpOrderMED), true},
		{"MP_REACH last after extended length", mpOrderJoin(mpOrderOrigin, mpOrderLong, mpOrderReach), true},
		{"MP_UNREACH after ORIGIN", mpOrderJoin(mpOrderOrigin, mpOrderUnreach), true},
		{"MP_REACH then MP_UNREACH", mpOrderJoin(mpOrderReach, mpOrderUnreach, mpOrderOrigin), true},
		{"truncated tail is not judged", append(mpOrderJoin(mpOrderReach, mpOrderOrigin), 0x80, 14), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, MPAttributesTrail(tc.attrs))
		})
	}
}

// VALIDATES: WriteMPAttributesFirst moves MP_UNREACH_NLRI, then MP_REACH_NLRI, to
// the front, keeps every other attribute in source order, and copies every byte
// (headers, extended lengths, an unparsed tail) exactly once.
// PREVENTS: a reorder that changes a byte other than an attribute's position.
func TestWriteMPAttributesFirst(t *testing.T) {
	tail := []byte{0x80, 14}
	source := mpOrderJoin(mpOrderOrigin, mpOrderReach, mpOrderLong, mpOrderUnreach, mpOrderMED, tail)
	want := mpOrderJoin(mpOrderUnreach, mpOrderReach, mpOrderOrigin, mpOrderLong, mpOrderMED, tail)

	dst := make([]byte, len(source))
	n := WriteMPAttributesFirst(dst, source)
	require.Equal(t, len(source), n)
	assert.Equal(t, want, dst)
	assert.False(t, MPAttributesTrail(dst[:len(dst)-len(tail)]))

	// A compliant section is rewritten to itself.
	same := make([]byte, len(want))
	WriteMPAttributesFirst(same, want)
	assert.Equal(t, want, same)
}
