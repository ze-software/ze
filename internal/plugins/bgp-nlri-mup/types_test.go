package bgp_nlri_mup

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMUPTypes verifies MUP route types.
func TestMUPTypes(t *testing.T) {
	assert.Equal(t, MUPRouteType(1), MUPISD)
	assert.Equal(t, MUPRouteType(2), MUPDSD)
	assert.Equal(t, MUPRouteType(3), MUPT1ST)
	assert.Equal(t, MUPRouteType(4), MUPT2ST)
}

// TestMUPBasic verifies basic MUP NLRI creation.
func TestMUPBasic(t *testing.T) {
	mup := NewMUP(MUPISD, []byte{1, 2, 3, 4})

	assert.Equal(t, MUPISD, mup.RouteType())
	assert.Equal(t, MUPArch3GPP5G, mup.ArchType())
}

// TestMUPFamily verifies MUP address family.
func TestMUPFamily(t *testing.T) {
	mup := NewMUP(MUPISD, nil)

	assert.Equal(t, AFIIPv4, mup.Family().AFI)
	assert.Equal(t, SAFIMUP, mup.Family().SAFI)
}

// TestMUPFull verifies full MUP NLRI creation.
func TestMUPFull(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	mup := NewMUPFull(AFIIPv6, MUPArch3GPP5G, MUPT1ST, rd, []byte{1, 2, 3, 4})

	assert.Equal(t, AFIIPv6, mup.Family().AFI)
	assert.Equal(t, MUPArch3GPP5G, mup.ArchType())
	assert.Equal(t, MUPT1ST, mup.RouteType())
	assert.Equal(t, rd, mup.RD())
}

// TestMUPRoundTrip verifies encode/decode cycle.
func TestMUPRoundTrip(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	original := NewMUPFull(AFIIPv4, MUPArch3GPP5G, MUPISD, rd, []byte{10, 0, 0, 1})
	data := original.Bytes()

	parsed, remaining, err := ParseMUP(AFIIPv4, data)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Equal(t, original.ArchType(), parsed.ArchType())
	assert.Equal(t, original.RouteType(), parsed.RouteType())
	assert.Equal(t, original.RD(), parsed.RD())
}

// TestMUPParseErrors verifies error handling.
func TestMUPParseErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated header", []byte{0x01}},
		{"truncated body", []byte{0x01, 0x00, 0x01, 0x10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseMUP(AFIIPv4, tt.data)
			assert.Error(t, err)
		})
	}
}

// TestMUPStringCommandStyle verifies command-style string representation.
//
// VALIDATES: MUP String() outputs command-style format for API round-trip.
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestMUPStringCommandStyle(t *testing.T) {
	tests := []struct {
		name     string
		mup      *MUP
		expected string
	}{
		{
			name:     "mup without rd",
			mup:      NewMUP(MUPISD, []byte{1, 2, 3, 4}),
			expected: "isd",
		},
		{
			name: "mup with rd",
			mup: func() *MUP {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewMUPFull(AFIIPv4, MUPArch3GPP5G, MUPT1ST, rd, []byte{10, 0, 0, 1})
			}(),
			expected: "t1st rd set 0:65001:100",
		},
		{
			name: "mup dsd with rd",
			mup: func() *MUP {
				rd := RouteDistinguisher{Type: RDType1}
				copy(rd.Value[:4], []byte{10, 0, 0, 1})
				binary.BigEndian.PutUint16(rd.Value[4:6], 200)
				return NewMUPFull(AFIIPv6, MUPArch3GPP5G, MUPDSD, rd, nil)
			}(),
			expected: "dsd rd set 1:10.0.0.1:200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mup.String())
		})
	}
}

// TestMUPWriteToMatchesBytes verifies MUP.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for MUP NLRI.
// PREVENTS: Architecture type encoding errors, route type confusion.
func TestMUPWriteToMatchesBytes(t *testing.T) {
	tests := []struct {
		name string
		mup  *MUP
	}{
		{
			name: "basic mup",
			mup:  NewMUP(MUPISD, []byte{1, 2, 3, 4}),
		},
		{
			name: "mup with rd",
			mup: func() *MUP {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewMUPFull(AFIIPv4, MUPArch3GPP5G, MUPT1ST, rd, []byte{10, 0, 0, 1})
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.mup.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.mup.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}
