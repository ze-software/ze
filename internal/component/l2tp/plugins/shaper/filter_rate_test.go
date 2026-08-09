// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- Filter-Id rate parsing tests

package l2tpshaper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFilterRateSymmetric(t *testing.T) {
	down, up, ok := parseFilterRate("10mbit")
	require.True(t, ok)
	require.Equal(t, uint64(10_000_000), down)
	require.Equal(t, uint64(10_000_000), up)
}

func TestParseFilterRateAsymmetric(t *testing.T) {
	down, up, ok := parseFilterRate("20mbit/5mbit")
	require.True(t, ok)
	require.Equal(t, uint64(20_000_000), down)
	require.Equal(t, uint64(5_000_000), up)
}

func TestParseFilterRateWithPrefix(t *testing.T) {
	down, up, ok := parseFilterRate("rate:100mbit/50mbit")
	require.True(t, ok)
	require.Equal(t, uint64(100_000_000), down)
	require.Equal(t, uint64(50_000_000), up)
}

func TestParseFilterRateSymmetricWithPrefix(t *testing.T) {
	down, up, ok := parseFilterRate("rate:10mbit")
	require.True(t, ok)
	require.Equal(t, uint64(10_000_000), down)
	require.Equal(t, uint64(10_000_000), up)
}

func TestParseFilterRateInvalid(t *testing.T) {
	tests := []string{
		"",
		"rate:",
		"not-a-rate",
		"10",
		"abc/def",
	}
	for _, s := range tests {
		_, _, ok := parseFilterRate(s)
		require.False(t, ok, "expected false for %q", s)
	}
}

func TestParseFilterRateGbit(t *testing.T) {
	down, up, ok := parseFilterRate("1gbit")
	require.True(t, ok)
	require.Equal(t, uint64(1_000_000_000), down)
	require.Equal(t, uint64(1_000_000_000), up)
}
