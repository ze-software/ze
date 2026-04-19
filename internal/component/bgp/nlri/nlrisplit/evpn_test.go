package nlrisplit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestSplitEVPN_Basic exercises the [route-type][length][body] framing
// used by every EVPN NLRI (RFC 7432 Section 7.1).
//
// VALIDATES: multi-NLRI EVPN inputs split into per-NLRI byte slices;
// ADD-PATH carries the 4-byte path-id in the returned slice.
// PREVENTS: silent corruption when back-to-back EVPN NLRIs share wire
// bytes.
func TestSplitEVPN_Basic(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got, err := SplitEVPN(nil, false)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("single-route-type-2", func(t *testing.T) {
		// route-type 2 (MAC/IP Advertisement), length 5, body 5 bytes.
		nlri := []byte{2, 5, 0xaa, 0xbb, 0xcc, 0xdd, 0xee}
		got, err := SplitEVPN(nlri, false)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, nlri, got[0])
	})

	t.Run("three-mixed-route-types", func(t *testing.T) {
		data := []byte{
			1, 3, 0x01, 0x02, 0x03, // route-type 1, length 3
			2, 2, 0x04, 0x05, // route-type 2, length 2
			3, 4, 0x06, 0x07, 0x08, 0x09, // route-type 3, length 4
		}
		got, err := SplitEVPN(data, false)
		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, uint8(1), got[0][0])
		assert.Equal(t, uint8(2), got[1][0])
		assert.Equal(t, uint8(3), got[2][0])
	})

	t.Run("add-path", func(t *testing.T) {
		// path-id 7, route-type 5, length 3, body 3 bytes.
		nlri := []byte{0, 0, 0, 7, 5, 3, 0x11, 0x22, 0x33}
		got, err := SplitEVPN(nlri, true)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, nlri, got[0], "ADD-PATH slice includes path-id")
	})
}

// TestSplitEVPN_Malformed validates error reporting with partial success.
func TestSplitEVPN_Malformed(t *testing.T) {
	t.Run("truncated-header", func(t *testing.T) {
		// Missing length byte after route-type.
		got, err := SplitEVPN([]byte{2}, false)
		assert.Error(t, err)
		assert.Empty(t, got)
	})

	t.Run("length-exceeds-data", func(t *testing.T) {
		// Declares length=10 but only 3 body bytes follow.
		got, err := SplitEVPN([]byte{2, 10, 1, 2, 3}, false)
		assert.Error(t, err)
		assert.Empty(t, got)
	})

	t.Run("partial-success", func(t *testing.T) {
		data := []byte{
			2, 3, 0x11, 0x22, 0x33, // good NLRI
			4, 10, 0x44, // truncated -- claims len=10 but only 1 body byte
		}
		got, err := SplitEVPN(data, false)
		assert.Error(t, err)
		require.Len(t, got, 1, "good NLRIs returned before the malformed one")
		assert.Equal(t, []byte{2, 3, 0x11, 0x22, 0x33}, got[0])
	})
}

// TestRegistryDispatchEVPN verifies EVPN is bound in the registry.
func TestRegistryDispatchEVPN(t *testing.T) {
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	data := []byte{2, 3, 0x11, 0x22, 0x33}
	got, err := Split(fam, data, false)
	require.NoError(t, err)
	require.Len(t, got, 1)
}
