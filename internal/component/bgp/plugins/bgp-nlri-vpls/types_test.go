package bgp_nlri_vpls

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVPLSBasic verifies basic VPLS NLRI creation.
func TestVPLSBasic(t *testing.T) {
	vpls := NewVPLS(RouteDistinguisher{Type: 1}, 100, 200, []byte{1, 2, 3})

	assert.Equal(t, uint16(100), vpls.VEBlockOffset())
	assert.Equal(t, uint16(200), vpls.VEBlockSize())
}

// TestVPLSFamily verifies VPLS address family.
func TestVPLSFamily(t *testing.T) {
	vpls := NewVPLS(RouteDistinguisher{}, 0, 0, nil)

	assert.Equal(t, AFIL2VPN, vpls.Family().AFI)
	assert.Equal(t, SAFIVPLS, vpls.Family().SAFI)
}

// TestVPLSBytes verifies VPLS wire format.
func TestVPLSBytes(t *testing.T) {
	vpls := NewVPLS(RouteDistinguisher{Type: 1}, 100, 200, []byte{1, 2, 3})

	data := vpls.Bytes()
	require.NotEmpty(t, data)
	// VPLS NLRI is 19 bytes: 2 len + 8 RD + 2 VE ID + 2 offset + 2 size + 3 label
	assert.Equal(t, 19, len(data))
}

// TestVPLSFull verifies full VPLS NLRI creation.
func TestVPLSFull(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	vpls := NewVPLSFull(rd, 1, 10, 20, 16000)

	assert.Equal(t, rd, vpls.RD())
	assert.Equal(t, uint16(1), vpls.VEID())
	assert.Equal(t, uint16(10), vpls.VEBlockOffset())
	assert.Equal(t, uint16(20), vpls.VEBlockSize())
	assert.Equal(t, uint32(16000), vpls.LabelBase())
}

// TestVPLSRoundTrip verifies encode/decode cycle.
func TestVPLSRoundTrip(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0}
	binary.BigEndian.PutUint16(rd.Value[:2], 65001)
	binary.BigEndian.PutUint32(rd.Value[2:6], 100)

	original := NewVPLSFull(rd, 5, 100, 200, 16000)
	data := original.Bytes()

	parsed, remaining, err := ParseVPLS(data)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Equal(t, original.RD(), parsed.RD())
	assert.Equal(t, original.VEID(), parsed.VEID())
	assert.Equal(t, original.VEBlockOffset(), parsed.VEBlockOffset())
	assert.Equal(t, original.VEBlockSize(), parsed.VEBlockSize())
	assert.Equal(t, original.LabelBase(), parsed.LabelBase())
}

// TestVPLSParseErrors verifies error handling.
func TestVPLSParseErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated length", []byte{0x00}},
		{"short length", []byte{0x00, 0x02}},
		{"too short", []byte{0x00, 0x11, 0, 0, 0, 0, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseVPLS(tt.data)
			assert.Error(t, err)
		})
	}
}

// TestVPLSStringCommandStyle verifies command-style string representation.
//
// VALIDATES: VPLS String() outputs command-style format for API round-trip.
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestVPLSStringCommandStyle(t *testing.T) {
	tests := []struct {
		name     string
		vpls     *VPLS
		expected string
	}{
		{
			name: "basic vpls",
			vpls: func() *VPLS {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewVPLSFull(rd, 5, 0, 0, 16000)
			}(),
			expected: "rd 0:65001:100 ve-id 5 label 16000",
		},
		{
			name: "vpls with type1 rd",
			vpls: func() *VPLS {
				rd := RouteDistinguisher{Type: RDType1}
				copy(rd.Value[:4], []byte{10, 0, 0, 1})
				binary.BigEndian.PutUint16(rd.Value[4:6], 200)
				return NewVPLSFull(rd, 10, 0, 0, 500)
			}(),
			expected: "rd 1:10.0.0.1:200 ve-id 10 label 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.vpls.String())
		})
	}
}

// TestVPLSWriteToMatchesBytes verifies VPLS.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for VPLS NLRI.
// PREVENTS: Label encoding errors, VE block field ordering issues.
func TestVPLSWriteToMatchesBytes(t *testing.T) {
	tests := []struct {
		name string
		vpls *VPLS
	}{
		{
			name: "basic vpls",
			vpls: NewVPLS(RouteDistinguisher{Type: 1}, 100, 200, []byte{1, 2, 3}),
		},
		{
			name: "full vpls",
			vpls: func() *VPLS {
				rd := RouteDistinguisher{Type: RDType0}
				binary.BigEndian.PutUint16(rd.Value[:2], 65001)
				binary.BigEndian.PutUint32(rd.Value[2:6], 100)
				return NewVPLSFull(rd, 5, 100, 200, 16000)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.vpls.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.vpls.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}
