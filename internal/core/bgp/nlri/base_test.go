package nlri

import (
	"bytes"
	"testing"
)

// TestRDNLRIBaseBuildDataNoAlias verifies buildData returns independent slice.
//
// VALIDATES: Modifying returned slice doesn't affect original data.
// PREVENTS: Slice aliasing bugs when caller mutates buildData result.
func TestRDNLRIBaseBuildDataNoAlias(t *testing.T) {
	t.Parallel()
	original := []byte{0x01, 0x02, 0x03}
	base := RDNLRIBase{data: original}

	result := base.buildData()
	result[0] = 0xFF // Mutate returned slice

	if original[0] == 0xFF {
		t.Error("buildData returned aliased slice - mutation affected original")
	}
}

// TestRDNLRIBaseBuildData verifies buildData returns rd+data or data only.
//
// VALIDATES: buildData() prepends RD bytes only when RD is non-zero.
// PREVENTS: Incorrect wire format for MVPN/MUP types.
func TestRDNLRIBaseBuildData(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rd   RouteDistinguisher
		data []byte
		want []byte
	}{
		{
			name: "zero RD returns data only",
			rd:   RouteDistinguisher{},
			data: []byte{0x01, 0x02, 0x03},
			want: []byte{0x01, 0x02, 0x03},
		},
		{
			name: "non-zero RD type prepends RD",
			rd:   RouteDistinguisher{Type: RDType0, Value: [6]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x64}},
			data: []byte{0xAA, 0xBB},
			want: []byte{
				0x00, 0x00, // Type 0
				0x00, 0x01, 0x00, 0x00, 0x00, 0x64, // Value: ASN 1, assigned 100
				0xAA, 0xBB, // data
			},
		},
		{
			name: "non-zero RD value prepends RD",
			rd:   RouteDistinguisher{Type: 0, Value: [6]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00}},
			data: []byte{0xCC},
			want: []byte{
				0x00, 0x00, // Type 0
				0x01, 0x00, 0x00, 0x00, 0x00, 0x00, // Value
				0xCC, // data
			},
		},
		{
			name: "empty data with RD",
			rd:   RouteDistinguisher{Type: RDType1, Value: [6]byte{0xC0, 0x00, 0x02, 0x01, 0x00, 0x64}},
			data: nil,
			want: []byte{
				0x00, 0x01, // Type 1
				0xC0, 0x00, 0x02, 0x01, 0x00, 0x64, // 192.0.2.1:100
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base := RDNLRIBase{
				rd:   tt.rd,
				data: tt.data,
			}
			got := base.buildData()
			if !bytes.Equal(got, tt.want) {
				t.Errorf("buildData() = %x, want %x", got, tt.want)
			}
		})
	}
}
