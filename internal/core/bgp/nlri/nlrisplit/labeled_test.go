package nlrisplit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSplitLabeledSingle validates a single labeled unicast NLRI with one label.
// Wire: [totalBits=48][label(3)][addr(3)] for 10.0.0.0/24 label 100.
// RFC 8277: totalBits includes label bits (24 per label) + prefix bits.
func TestSplitLabeledSingle(t *testing.T) {
	// label 100: 100 << 4 = 0x0640, S=1 -> byte2 |= 0x01
	// bytes: 0x06, 0x40, 0x01
	// prefix 10.0.0.0/24: 3 address bytes
	// totalBits = 24 (label) + 24 (prefix) = 48
	data := []byte{
		48,
		0x06, 0x40, 0x01,
		10, 0, 0,
	}
	got, err := splitAll(t, SplitLabeled, data, false)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, data, got[0])
}

// TestSplitLabeledStack validates multi-label stack parsing (S-bit detection).
func TestSplitLabeledStack(t *testing.T) {
	// Two labels: 100 (S=0) then 200 (S=1)
	// label 100: 0x06, 0x40, 0x00 (S=0)
	// label 200: 0x0C, 0x80, 0x01 (S=1)
	// prefix 10.0.0.0/24, totalBits = 24+24+24 = 72
	data := []byte{
		72,
		0x06, 0x40, 0x00,
		0x0C, 0x80, 0x01,
		10, 0, 0,
	}
	got, err := splitAll(t, SplitLabeled, data, false)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, data, got[0])
}

// TestSplitLabeledMultipleNLRI validates splitting concatenated labeled NLRIs.
func TestSplitLabeledMultipleNLRI(t *testing.T) {
	nlri1 := []byte{48, 0x06, 0x40, 0x01, 10, 0, 0}
	nlri2 := []byte{48, 0x0C, 0x80, 0x01, 192, 168, 1}
	data := append(append([]byte{}, nlri1...), nlri2...)

	got, err := splitAll(t, SplitLabeled, data, false)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, nlri1, got[0])
	assert.Equal(t, nlri2, got[1])
}

// TestSplitLabeledIPv6 validates IPv6 labeled unicast.
func TestSplitLabeledIPv6(t *testing.T) {
	// 2001:db8::/32 with label 300
	// label 300: 300<<4 = 0x12C0, |0x01 -> 0x12, 0xC0, 0x01
	// totalBits = 24 + 32 = 56
	data := []byte{
		56,
		0x12, 0xC0, 0x01,
		0x20, 0x01, 0x0D, 0xB8,
	}
	got, err := splitAll(t, SplitLabeled, data, false)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, data, got[0])
}

// TestSplitLabeledMalformed validates error handling on truncated/malformed input.
func TestSplitLabeledMalformed(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got, err := splitAll(t, SplitLabeled, nil, false)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("truncated-label", func(t *testing.T) {
		data := []byte{48, 0x06, 0x40}
		_, err := splitAll(t, SplitLabeled, data, false)
		assert.Error(t, err)
	})

	t.Run("truncated-prefix", func(t *testing.T) {
		data := []byte{48, 0x06, 0x40, 0x01, 10, 0}
		_, err := splitAll(t, SplitLabeled, data, false)
		assert.Error(t, err)
	})

	t.Run("no-s-bit-then-truncated", func(t *testing.T) {
		data := []byte{48, 0x06, 0x40, 0x00}
		_, err := splitAll(t, SplitLabeled, data, false)
		assert.Error(t, err)
	})

	t.Run("partial-success", func(t *testing.T) {
		good := []byte{48, 0x06, 0x40, 0x01, 10, 0, 0}
		bad := []byte{48, 0x06}
		data := append(append([]byte{}, good...), bad...)

		got, err := splitAll(t, SplitLabeled, data, false)
		assert.Error(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, good, got[0])
	})
}

// TestSplitLabeledAddPath validates ADD-PATH 4-byte path-id on labeled NLRI.
func TestSplitLabeledAddPath(t *testing.T) {
	data := []byte{
		0, 0, 0, 42,
		48,
		0x06, 0x40, 0x01,
		10, 0, 0,
	}
	got, err := splitAll(t, SplitLabeled, data, true)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, data, got[0])
}

// TestSplitLabeledAddPathMultiple validates ADD-PATH with multiple NLRIs.
func TestSplitLabeledAddPathMultiple(t *testing.T) {
	nlri1 := []byte{0, 0, 0, 1, 48, 0x06, 0x40, 0x01, 10, 0, 0}
	nlri2 := []byte{0, 0, 0, 2, 48, 0x0C, 0x80, 0x01, 192, 168, 1}
	data := append(append([]byte{}, nlri1...), nlri2...)

	got, err := splitAll(t, SplitLabeled, data, true)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, nlri1, got[0])
	assert.Equal(t, nlri2, got[1])
}

// TestExtractLabels validates label+CIDR extraction from a single labeled NLRI.
func TestExtractLabels(t *testing.T) {
	t.Run("single-label", func(t *testing.T) {
		// label 100 (0x64): byte0=0x00, byte1=0x06, byte2=(0x4<<4)|S=0x41
		data := []byte{48, 0x00, 0x06, 0x41, 10, 0, 0}
		labels, cidr, err := ExtractLabels(data, false)
		require.NoError(t, err)
		assert.Equal(t, []uint32{100}, labels)
		assert.Equal(t, []byte{24, 10, 0, 0}, cidr)
	})

	t.Run("two-labels", func(t *testing.T) {
		// label 100 S=0: 0x00, 0x06, 0x40
		// label 200 (0xC8) S=1: byte0=0x00, byte1=0x0C, byte2=(0x8<<4)|1=0x81
		data := []byte{72, 0x00, 0x06, 0x40, 0x00, 0x0C, 0x81, 10, 0, 0}
		labels, cidr, err := ExtractLabels(data, false)
		require.NoError(t, err)
		assert.Equal(t, []uint32{100, 200}, labels)
		assert.Equal(t, []byte{24, 10, 0, 0}, cidr)
	})

	t.Run("add-path", func(t *testing.T) {
		// path-id=42, label 100 S=1
		data := []byte{0, 0, 0, 42, 48, 0x00, 0x06, 0x41, 10, 0, 0}
		labels, cidr, err := ExtractLabels(data, true)
		require.NoError(t, err)
		assert.Equal(t, []uint32{100}, labels)
		assert.Equal(t, []byte{0, 0, 0, 42, 24, 10, 0, 0}, cidr)
	})

	t.Run("ipv6", func(t *testing.T) {
		// label 300 (0x12C) S=1: byte0=0x00, byte1=0x12, byte2=(0xC<<4)|1=0xC1
		data := []byte{56, 0x00, 0x12, 0xC1, 0x20, 0x01, 0x0D, 0xB8}
		labels, cidr, err := ExtractLabels(data, false)
		require.NoError(t, err)
		assert.Equal(t, []uint32{300}, labels)
		assert.Equal(t, []byte{32, 0x20, 0x01, 0x0D, 0xB8}, cidr)
	})

	t.Run("truncated", func(t *testing.T) {
		data := []byte{48, 0x06}
		_, _, err := ExtractLabels(data, false)
		assert.Error(t, err)
	})
}
