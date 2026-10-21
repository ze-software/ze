package nlrisplit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestSplitCIDR_Basic exercises the CIDR splitter across common shapes.
//
// VALIDATES: concatenated [prefix-len][addr] NLRIs split into per-NLRI
// byte slices that alias the input; ADD-PATH includes the 4-byte path-id.
// PREVENTS: split regressions on the RIB hot path.
func TestSplitCIDR_Basic(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got, err := Split(family.IPv4Unicast, nil, false)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("single-/24-v4", func(t *testing.T) {
		nlri := []byte{24, 10, 0, 0}
		got, err := Split(family.IPv4Unicast, nlri, false)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, nlri, got[0])
	})

	t.Run("three-/24-v4", func(t *testing.T) {
		data := []byte{
			24, 10, 0, 0,
			24, 10, 0, 1,
			24, 10, 0, 2,
		}
		got, err := Split(family.IPv4Unicast, data, false)
		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, []byte{24, 10, 0, 0}, got[0])
		assert.Equal(t, []byte{24, 10, 0, 1}, got[1])
		assert.Equal(t, []byte{24, 10, 0, 2}, got[2])
	})

	t.Run("add-path-single", func(t *testing.T) {
		// path-id = 42, /24 for 10.0.0.0.
		data := []byte{0, 0, 0, 42, 24, 10, 0, 0}
		got, err := Split(family.IPv4Unicast, data, true)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, data, got[0], "ADD-PATH NLRI includes path-id in returned slice")
	})

	t.Run("v6-/64", func(t *testing.T) {
		// /64: 1 + 8 addr bytes.
		data := []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 1}
		got, err := Split(family.IPv6Unicast, data, false)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, data, got[0])
	})
}

// TestSplitCIDR_Malformed validates error reporting with any successfully-
// parsed NLRIs returned alongside.
func TestSplitCIDR_Malformed(t *testing.T) {
	t.Run("truncated", func(t *testing.T) {
		// prefix-len says /24 but only one byte of address follows.
		data := []byte{24, 10}
		got, err := Split(family.IPv4Unicast, data, false)
		assert.Error(t, err)
		assert.Empty(t, got, "no complete NLRI before truncation")
	})

	t.Run("prefix-length-too-large", func(t *testing.T) {
		// /200 exceeds IPv6 /128.
		data := []byte{200, 0, 0, 0, 0}
		_, err := Split(family.IPv4Unicast, data, false)
		assert.Error(t, err)
	})

	t.Run("partial-success", func(t *testing.T) {
		data := []byte{
			24, 10, 0, 0, // one good NLRI
			24, 10, // then truncation
		}
		got, err := Split(family.IPv4Unicast, data, false)
		assert.Error(t, err)
		require.Len(t, got, 1, "returns complete NLRI parsed before truncation")
		assert.Equal(t, []byte{24, 10, 0, 0}, got[0])
	})
}

// TestRegistryDispatch validates the package-level Split entry point.
func TestRegistryDispatch(t *testing.T) {
	// CIDR families are registered at init(); no setup needed.
	data := []byte{24, 10, 0, 0}
	got, err := Split(family.IPv4Unicast, data, false)
	require.NoError(t, err)
	require.Len(t, got, 1)

	// Unregistered family returns ErrUnsupported.
	unregistered := family.Family{AFI: 999, SAFI: 99}
	_, err = Split(unregistered, data, false)
	assert.ErrorIs(t, err, ErrUnsupported)
}

// BenchmarkSplitCIDR measures splitCIDR, the unicast NLRI hot path, over a
// full IPv4 and IPv6 field. Its goal is to show the per-entry framing step that
// splitByBitLength shares with the labeled withdrawal writer costs the unicast
// path no call: run it before and after that step moved, and compare.
func BenchmarkSplitCIDR(b *testing.B) {
	v4 := make([]byte, 0, 4*1000)
	for i := range 1000 {
		v4 = append(v4, 24, 10, byte(i>>8), byte(i))
	}
	v6 := make([]byte, 0, 9*500)
	for i := range 500 {
		v6 = append(v6, 64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, byte(i>>8), byte(i))
	}
	visit := func([]byte) bool { return true }
	for _, field := range []struct {
		name string
		data []byte
	}{{"ipv4", v4}, {"ipv6", v6}} {
		b.Run(field.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := splitCIDR(field.data, false, visit); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
