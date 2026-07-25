// VALIDATES: SR-Policy NLRI splitter correctly splits concatenated NLRIs by length-bit prefix.
// PREVENTS: Wrong NLRI boundaries, silent truncation, unregistered splitter.

package srpolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

func TestSRPolicySplitIPv4Single(t *testing.T) {
	// 1-byte length (96 bits = 0x60) + 12-byte body
	data := make([]byte, 13)
	data[0] = 96 // 12*8 bits
	result, err := SplitSRPolicy(data, false)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Len(t, result[0], 13)
}

func TestSRPolicySplitIPv4Multiple(t *testing.T) {
	// Two IPv4 SR-Policy NLRIs concatenated
	data := make([]byte, 26) // 13 + 13
	data[0] = 96
	data[13] = 96
	result, err := SplitSRPolicy(data, false)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result[0], 13)
	assert.Len(t, result[1], 13)
}

func TestSRPolicySplitIPv6Single(t *testing.T) {
	// 1-byte length (192 bits = 0xC0) + 24-byte body
	data := make([]byte, 25)
	data[0] = 192 // 24*8 bits
	result, err := SplitSRPolicy(data, false)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Len(t, result[0], 25)
}

func TestSRPolicySplitEmpty(t *testing.T) {
	result, err := SplitSRPolicy(nil, false)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestSRPolicySplitTruncated(t *testing.T) {
	// Length says 96 bits (12 bytes) but only 5 bytes of body
	data := make([]byte, 6)
	data[0] = 96
	_, err := SplitSRPolicy(data, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extends past data")
}

func TestSRPolicySplitZeroLength(t *testing.T) {
	data := []byte{0}
	_, err := SplitSRPolicy(data, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zero-length")
}

func TestSRPolicySplitNotByteAligned(t *testing.T) {
	data := []byte{97} // 97 bits, not byte-aligned
	_, err := SplitSRPolicy(data, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not byte-aligned")
}

func TestSRPolicySplitRegistered(t *testing.T) {
	fam4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	assert.True(t, nlrisplit.Supported(fam4), "ipv4/sr-policy splitter must be registered")

	fam6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFISRPolicy}
	assert.True(t, nlrisplit.Supported(fam6), "ipv6/sr-policy splitter must be registered")
}

func TestSRPolicySplitViaRegistry(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	data := make([]byte, 13)
	data[0] = 96
	result, err := nlrisplit.Split(fam, data, false)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}
