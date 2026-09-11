package nlrisplit

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func vplsKeyRoute() []byte {
	return []byte{
		0, 17, // Native two-octet length.
		0, 0, 0, 1, 0, 0, 0, 2, // RD.
		0, 3, 0, 4, // VE ID and VE Block Offset.
		0, 8, 0, 16, 1, // VE Block Size and Label Base.
	}
}

func TestVPLSKeyIgnoresBlockSizeAndLabel(t *testing.T) {
	base := vplsKeyRoute()
	key, err := keyVPLS(base, nil, false)
	require.NoError(t, err)
	for _, offset := range []int{14, 15, 16, 17, 18} {
		changed := bytes.Clone(base)
		changed[offset]++
		before := bytes.Clone(changed)
		other, err := keyVPLS(changed, nil, true)
		require.NoError(t, err)
		assert.Equal(t, key, other)
		assert.Equal(t, before, changed)
	}
	// The existing typed parser accepts extensions beyond the fixed fields.
	extended := append(bytes.Clone(base), 0xab, 0xcd)
	binary.BigEndian.PutUint16(extended[:2], uint16(len(extended)-2))
	other, err := keyVPLS(extended, nil, false)
	require.NoError(t, err)
	assert.Equal(t, key, other, "native length is not part of the route key")
}

func TestVPLSKeyDistinguishesRDVEAndOffset(t *testing.T) {
	base := vplsKeyRoute()
	key, err := keyVPLS(base, nil, false)
	require.NoError(t, err)
	for _, offset := range []int{9, 10, 11, 12, 13} {
		changed := bytes.Clone(base)
		changed[offset]++
		other, err := keyVPLS(changed, nil, false)
		require.NoError(t, err)
		assert.NotEqual(t, key, other)
	}
}

func TestVPLSKeyRejectsTruncation(t *testing.T) {
	raw := vplsKeyRoute()
	for n := range raw {
		_, err := keyVPLS(raw[:n], nil, false)
		require.ErrorIs(t, err, errPrefixKey)
		if n >= 2 {
			short := bytes.Clone(raw[:n])
			binary.BigEndian.PutUint16(short[:2], uint16(n-2))
			_, err = keyVPLS(short, nil, false)
			require.ErrorIs(t, err, errPrefixKey)
		}
	}
	_, err := keyVPLS(append(raw, 0), nil, false)
	require.ErrorIs(t, err, errPrefixKey)
}
