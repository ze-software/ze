package nlrisplit

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mupKeyRoute(routeType uint16, fields ...byte) []byte {
	body := append([]byte{0, 0, 0, 1, 0, 0, 0, 2}, fields...)
	return mupNLRI(routeType, body...)
}

func mupRouteKey(t *testing.T, raw []byte) []byte {
	t.Helper()
	var scratch [PrefixKeyScratchSize]byte
	key, err := keyMUP(raw, scratch[:], false)
	require.NoError(t, err)
	return bytes.Clone(key)
}

func TestMUPKeyT1STIgnoresSessionData(t *testing.T) {
	// The existing encoder omits the source-length octet if no source is set.
	older := mupKeyRoute(3, 24, 10, 1, 2, 0, 0, 0, 1, 9, 32, 192, 0, 2, 1)
	// Newer framing: different TEID/QFI, IPv6 endpoint, source and optional TLV.
	newer := mupKeyRoute(3, 24, 10, 1, 2, 0, 0, 0, 2, 10, 128)
	newer = append(newer, bytes.Repeat([]byte{0x20}, 16)...)
	newer = append(newer, 32, 198, 51, 100, 1, 99, 2, 0xab, 0xcd)
	newer[3] = byte(len(newer) - 4)
	before := bytes.Clone(newer)
	assert.Equal(t, mupRouteKey(t, older), mupRouteKey(t, newer))
	assert.Equal(t, before, newer, "key extraction must not mutate wire bytes")

	var scratch [PrefixKeyScratchSize]byte
	withdrawn, err := keyMUP(newer, scratch[:], true)
	require.NoError(t, err)
	assert.Equal(t, mupRouteKey(t, older), withdrawn)
}

func TestMUPKeyPrefixIdentity(t *testing.T) {
	for _, routeType := range []uint16{1, 3} {
		base := mupKeyRoute(routeType, 25, 10, 1, 2, 0x80)
		padding := bytes.Clone(base)
		padding[len(padding)-1] = 0xff
		assert.Equal(t, mupRouteKey(t, base), mupRouteKey(t, padding))
		assert.Equal(t, byte(0xff), padding[len(padding)-1], "padding normalization must not mutate input")

		for _, offset := range []int{11, 12, 13, 16} { // RD, length, prefix, final significant bit.
			changed := bytes.Clone(base)
			if offset == 16 {
				changed[offset] ^= 0x80
			} else {
				changed[offset]++
			}
			assert.NotEqual(t, mupRouteKey(t, base), mupRouteKey(t, changed))
		}
	}
	assert.NotEqual(t, mupRouteKey(t, mupKeyRoute(1, 0)), mupRouteKey(t, mupKeyRoute(3, 0)))
	assert.NotEqual(t, mupRouteKey(t, mupKeyRoute(1, 0)), mupRouteKey(t, mupKeyRoute(1, append([]byte{128}, make([]byte, 16)...)...)))
}

func TestMUPKeyDSDAddressAndRD(t *testing.T) {
	for _, addrLen := range []int{4, 16} {
		raw := mupKeyRoute(2, bytes.Repeat([]byte{1}, addrLen)...)
		for _, offset := range []int{11, len(raw) - 1} {
			changed := bytes.Clone(raw)
			changed[offset]++
			assert.NotEqual(t, mupRouteKey(t, raw), mupRouteKey(t, changed))
		}
	}
}

func TestMUPKeyT2STVariableEndpointIdentifier(t *testing.T) {
	// Endpoint length includes address and identifier. Cover IPv4/IPv6,
	// absent/full TEIDs and partial-byte identifiers emitted by writeTEIDWithBits.
	for _, bits := range []byte{32, 40, 45, 56, 64, 128, 136, 141, 152, 160} {
		t.Run(strconv.Itoa(int(bits)), func(t *testing.T) {
			fields := append([]byte{bits}, bytes.Repeat([]byte{1}, (int(bits)+7)/8)...)
			base := mupKeyRoute(4, fields...)
			withTLV := append(bytes.Clone(base), 99, 2, 0xab, 0xcd)
			withTLV[3] = byte(len(withTLV) - 4)
			assert.Equal(t, mupRouteKey(t, base), mupRouteKey(t, withTLV))
			for _, offset := range []int{11, 13, len(base) - 1} {
				changed := bytes.Clone(base)
				changed[offset]++
				assert.NotEqual(t, mupRouteKey(t, base), mupRouteKey(t, changed))
			}
		})
	}
	// Aggregated identifiers with identical octets but different bit widths differ.
	a := mupKeyRoute(4, 39, 192, 0, 2, 1, 1)
	b := bytes.Clone(a)
	b[12] = 40
	assert.NotEqual(t, mupRouteKey(t, a), mupRouteKey(t, b))
}

func TestMUPKeyUnknownDiscriminatorsRemainDistinct(t *testing.T) {
	base := mupKeyRoute(3, 24, 10, 1, 2)
	for _, offset := range []int{0, 1, 2} {
		changed := bytes.Clone(base)
		changed[offset] = 99
		assert.NotEqual(t, mupRouteKey(t, base), mupRouteKey(t, changed))
		other := bytes.Clone(changed)
		other[len(other)-1]++
		assert.NotEqual(t, mupRouteKey(t, changed), mupRouteKey(t, other))
	}
}

func TestMUPKeyRejectsTruncation(t *testing.T) {
	var scratch [PrefixKeyScratchSize]byte
	for _, raw := range [][]byte{
		mupKeyRoute(1, 24, 10, 1, 2),
		mupKeyRoute(2, 192, 0, 2, 1),
		mupKeyRoute(3, 24, 10, 1, 2, 0, 0, 0, 1, 9, 32, 192, 0, 2, 1),
		mupKeyRoute(4, 64, 192, 0, 2, 1, 0, 0, 0, 1),
		mupNLRI(99, 1, 2, 3),
	} {
		for n := range raw {
			_, err := keyMUP(raw[:n], scratch[:], false)
			require.ErrorIs(t, err, errPrefixKey)
		}
	}
	for _, raw := range [][]byte{
		mupKeyRoute(1),
		mupKeyRoute(1, 128, 0),
		mupKeyRoute(1, 129),
		mupKeyRoute(2, 192, 0, 2),
		mupKeyRoute(3, 24, 10, 1),
		mupKeyRoute(4, 160, 0),
		mupKeyRoute(4, 161),
	} {
		_, err := keyMUP(raw, scratch[:], false)
		require.ErrorIs(t, err, errPrefixKey)
	}
	_, err := keyMUP(mupKeyRoute(1, 0), nil, false)
	require.ErrorIs(t, err, errPrefixKey)
}
