package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOriginValues(t *testing.T) {
	assert.Equal(t, uint8(0), uint8(OriginIGP))
	assert.Equal(t, uint8(1), uint8(OriginEGP))
	assert.Equal(t, uint8(2), uint8(OriginIncomplete))
}

func TestOriginString(t *testing.T) {
	assert.Equal(t, "IGP", OriginIGP.String())
	assert.Equal(t, "EGP", OriginEGP.String())
	assert.Equal(t, "INCOMPLETE", OriginIncomplete.String())
	assert.Equal(t, "UNKNOWN(99)", Origin(99).String())
}

func TestOriginParse(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    Origin
		wantErr bool
	}{
		{"IGP", []byte{0x00}, OriginIGP, false},
		{"EGP", []byte{0x01}, OriginEGP, false},
		{"INCOMPLETE", []byte{0x02}, OriginIncomplete, false},
		{"empty", []byte{}, 0, true},
		{"invalid value", []byte{0x03}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOrigin(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOriginWriteTo(t *testing.T) {
	buf := make([]byte, 64)

	n := OriginIGP.WriteTo(buf, 0)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte{0x00}, buf[:n])

	n = OriginEGP.WriteTo(buf, 0)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte{0x01}, buf[:n])

	n = OriginIncomplete.WriteTo(buf, 0)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte{0x02}, buf[:n])
}

func TestOriginInterface(t *testing.T) {
	var attr Attribute = OriginIGP

	assert.Equal(t, AttrOrigin, attr.Code())
	assert.Equal(t, FlagTransitive, attr.Flags())
	assert.Equal(t, 1, attr.Len())

	buf := make([]byte, 64)
	n := attr.WriteTo(buf, 0)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte{0x00}, buf[:n])
}

func TestOriginWriteAttrTo(t *testing.T) {
	// Full attribute: flags(1) + code(1) + len(1) + value(1) = 4 bytes
	want := []byte{0x40, 0x01, 0x01, 0x00} // Transitive, ORIGIN, len=1, value=IGP
	buf := make([]byte, 64)
	n := WriteAttrTo(OriginIGP, buf, 0)
	assert.Equal(t, 4, n)
	assert.Equal(t, want, buf[:n])
}
