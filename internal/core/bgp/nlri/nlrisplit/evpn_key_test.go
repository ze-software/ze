package nlrisplit

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func evpnKeyFixture(routeType byte, addressLen int) []byte {
	bodyLen := 0
	switch routeType {
	case 1:
		bodyLen = 25
	case 2:
		bodyLen = 33 + addressLen
	case 3:
		bodyLen = 13 + addressLen
	case 4:
		bodyLen = 19 + addressLen
	case 5:
		bodyLen = 26 + 2*addressLen
	}
	raw := make([]byte, 2+bodyLen)
	raw[0], raw[1] = routeType, byte(bodyLen)
	body := raw[2:]
	body[7] = 1 // RD
	switch routeType {
	case 1:
		body[17], body[21], body[24] = 2, 3, 1
	case 2:
		body[17], body[21], body[22], body[28] = 2, 3, 48, 4
		body[29] = byte(addressLen * 8)
		if addressLen != 0 {
			body[30] = 192
		}
		body[len(body)-1] = 1
	case 3:
		body[11], body[12], body[13] = 3, byte(addressLen*8), 192
	case 4:
		body[17], body[18], body[19] = 2, byte(addressLen*8), 192
	case 5:
		body[17], body[21], body[22], body[23] = 2, 3, 13, 192
		body[24] = 168
		body[23+addressLen], body[len(body)-1] = 10, 1
	}
	return raw
}

func evpnIdentity(t *testing.T, raw []byte, withdraw bool) []byte {
	t.Helper()
	before := bytes.Clone(raw)
	var scratch [PrefixKeyScratchSize]byte
	key, err := keyEVPN(raw, scratch[:], withdraw)
	require.NoError(t, err)
	require.Equal(t, before, raw, "key extraction must not mutate the NLRI")
	return bytes.Clone(key)
}

func TestKeyEVPNNonKeyChanges(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		edit func([]byte) []byte
	}{
		{"rt1-label", evpnKeyFixture(1, 0), func(raw []byte) []byte {
			copy(raw[24:27], []byte{9, 8, 7})
			return raw
		}},
		{"rt2-esi-label", evpnKeyFixture(2, 4), func(raw []byte) []byte {
			for i := 10; i < 20; i++ {
				raw[i] ^= 0xff
			}
			copy(raw[len(raw)-3:], []byte{9, 8, 7})
			return raw
		}},
		{"rt2-optional-label", evpnKeyFixture(2, 16), func(raw []byte) []byte {
			raw = append(raw, 9, 8, 7)
			raw[1] += 3
			return raw
		}},
		{"rt5-ipv4-attributes-host-bits", evpnKeyFixture(5, 4), nil},
		{"rt5-ipv6-attributes-host-bits", evpnKeyFixture(5, 16), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := bytes.Clone(tc.raw)
			if tc.edit != nil {
				changed = tc.edit(changed)
			} else {
				for i := 10; i < 20; i++ {
					changed[i] ^= 0xff // ESI
				}
				changed[26] |= 7 // Insignificant bits in the /13 prefix.
				for i := 27; i < len(changed); i++ {
					changed[i] ^= 0xff // Remaining host bits, gateway, label.
				}
			}
			want := evpnIdentity(t, tc.raw, false)
			require.Equal(t, want, evpnIdentity(t, changed, false))
			require.Equal(t, want, evpnIdentity(t, changed, true))
		})
	}
}

func TestKeyEVPNDistinctIdentityFields(t *testing.T) {
	cases := []struct {
		name       string
		routeType  byte
		addressLen int
		offsets    []int // Payload offsets, excluding route-type/length framing.
	}{
		{"rt1", 1, 0, []int{7, 17, 21}},
		{"rt2-mac-only", 2, 0, []int{7, 21, 28}},
		{"rt2-ipv4", 2, 4, []int{7, 21, 28, 30}},
		{"rt2-ipv6", 2, 16, []int{7, 21, 28, 45}},
		{"rt3", 3, 4, []int{7, 11, 13}},
		{"rt4", 4, 16, []int{7, 17, 34}},
		{"rt5-ipv4", 5, 4, []int{7, 21, 22, 23}},
		{"rt5-ipv6", 5, 16, []int{7, 21, 22, 23}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := evpnKeyFixture(tc.routeType, tc.addressLen)
			want := evpnIdentity(t, raw, false)
			require.Equal(t, tc.routeType, want[0], "route type must remain in the key")
			for _, offset := range tc.offsets {
				changed := bytes.Clone(raw)
				changed[2+offset] ^= 1
				require.NotEqual(t, want, evpnIdentity(t, changed, false), "payload offset %d", offset)
			}
		})
	}
	// Equal significant prefix bits must not conflate IPv4 and IPv6.
	require.NotEqual(t, evpnIdentity(t, evpnKeyFixture(5, 4), false), evpnIdentity(t, evpnKeyFixture(5, 16), false))
	// MAC-only, IPv4, and IPv6 advertisements are different routes.
	require.NotEqual(t, evpnIdentity(t, evpnKeyFixture(2, 0), false), evpnIdentity(t, evpnKeyFixture(2, 4), false))
	require.NotEqual(t, evpnIdentity(t, evpnKeyFixture(2, 4), false), evpnIdentity(t, evpnKeyFixture(2, 16), false))
}

func TestKeyEVPNPrefixBoundaries(t *testing.T) {
	for _, addressLen := range []int{4, 16} {
		raw := evpnKeyFixture(5, addressLen)
		raw[24] = 0 // /0 ignores every address bit, including dirty scratch.
		changed := bytes.Clone(raw)
		for i := 25; i < 25+addressLen; i++ {
			changed[i] ^= 0xff
		}
		var scratch [PrefixKeyScratchSize]byte
		for i := range scratch {
			scratch[i] = 0xff
		}
		key, err := keyEVPN(raw, scratch[:], false)
		require.NoError(t, err)
		require.Equal(t, make([]byte, addressLen), key[14:])
		require.Equal(t, key, evpnIdentity(t, changed, false))

		raw[24] = byte(addressLen * 8)
		changed = bytes.Clone(raw)
		changed[24+addressLen] ^= 1
		require.NotEqual(t, evpnIdentity(t, raw, false), evpnIdentity(t, changed, false), "full-length prefix must retain its last bit")
	}
}

func TestKeyEVPNMalformed(t *testing.T) {
	var scratch [PrefixKeyScratchSize]byte
	fixtures := [][]byte{
		evpnKeyFixture(1, 0), evpnKeyFixture(2, 0), evpnKeyFixture(2, 4),
		evpnKeyFixture(2, 16), evpnKeyFixture(3, 4), evpnKeyFixture(3, 16),
		evpnKeyFixture(4, 4), evpnKeyFixture(4, 16),
		evpnKeyFixture(5, 4), evpnKeyFixture(5, 16),
	}
	for _, raw := range fixtures {
		for end := range raw {
			_, err := keyEVPN(raw[:end], scratch[:], false)
			require.ErrorIs(t, err, errPrefixKey, "type %d truncated at %d", raw[0], end)
		}
		truncated := bytes.Clone(raw[:len(raw)-1])
		truncated[1]--
		_, err := keyEVPN(truncated, scratch[:], true)
		require.ErrorIs(t, err, errPrefixKey, "type %d short payload", raw[0])
		_, err = keyEVPN(append(bytes.Clone(raw), 0), scratch[:], false)
		require.ErrorIs(t, err, errPrefixKey, "trailing bytes")
		_, err = keyEVPN(raw, nil, false)
		require.ErrorIs(t, err, errPrefixKey, "missing scratch")
	}
	for _, tc := range []struct {
		raw    []byte
		offset int
		value  byte
	}{
		{evpnKeyFixture(2, 4), 24, 47},
		{evpnKeyFixture(2, 4), 31, 31},
		{evpnKeyFixture(3, 4), 14, 0},
		{evpnKeyFixture(4, 16), 20, 127},
		{evpnKeyFixture(5, 4), 24, 33},
		{evpnKeyFixture(5, 16), 24, 129},
	} {
		tc.raw[tc.offset] = tc.value
		_, err := keyEVPN(tc.raw, scratch[:], false)
		require.ErrorIs(t, err, errPrefixKey)
	}
}

func TestKeyEVPNUnknownType(t *testing.T) {
	raw := []byte{99, 3, 1, 2, 3}
	key, err := keyEVPN(raw, nil, false)
	require.NoError(t, err)
	require.Equal(t, raw, key)
	changed := bytes.Clone(raw)
	changed[0]++
	require.NotEqual(t, key, evpnIdentity(t, changed, false))
	changed = bytes.Clone(raw)
	changed[4]++
	require.NotEqual(t, key, evpnIdentity(t, changed, false))
}
