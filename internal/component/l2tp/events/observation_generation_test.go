// Design: docs/architecture/l2tp/l2tp-9-observer.md -- observation scope boundaries
// Related: observation_generation.go -- the cold allocator and JSON encoding

package events

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestObservationGenerationConstructor rejects absent scope and proves two
// real allocations remain distinct comparable keys through JSON round trip.
func TestObservationGenerationConstructor(t *testing.T) {
	first, err := NewObservationGeneration()
	require.NoError(t, err)
	second, err := NewObservationGeneration()
	require.NoError(t, err)
	require.True(t, first.IsValid())
	require.True(t, second.IsValid())
	require.NotEqual(t, first, second)
	require.Len(t, map[ObservationGeneration]bool{first: true, second: true}, 2)
	var zero ObservationGeneration
	require.False(t, zero.IsValid())
	_, err = zero.MarshalText()
	require.Error(t, err)
	encoded, err := json.Marshal(first)
	require.NoError(t, err)
	var decoded ObservationGeneration
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, first, decoded)
}

// TestObservationGenerationDecodePreservesReceiver pairs valid canonical input
// with zero, leading signs/zeros, malformed bytes and overflow refusals.
func TestObservationGenerationDecodePreservesReceiver(t *testing.T) {
	generation, err := NewObservationGeneration()
	require.NoError(t, err)
	for _, text := range []string{"", "0", "00", "01", "+1", "-1", " 1", "1 ", "1_0", "1x", "18446744073709551616", "999999999999999999999"} {
		t.Run(text, func(t *testing.T) {
			previous := generation
			require.Error(t, generation.UnmarshalText([]byte(text)))
			require.Equal(t, previous, generation)
		})
	}
	require.NoError(t, generation.UnmarshalText([]byte("18446744073709551615")))
	encoded, err := generation.MarshalText()
	require.NoError(t, err)
	require.Equal(t, "18446744073709551615", string(encoded))
	var absent *ObservationGeneration
	require.Error(t, absent.UnmarshalText([]byte("1")))
}

// TestObservationGenerationExhaustion exercises a local cold allocator at its
// last value, preserving the process allocator and refusing wrap to zero.
func TestObservationGenerationExhaustion(t *testing.T) {
	source := observationSequence{last: math.MaxUint64 - 1}
	last, err := source.next()
	require.NoError(t, err)
	require.Equal(t, uint64(math.MaxUint64), last.sequence)
	refused, err := source.next()
	require.Error(t, err)
	require.False(t, refused.IsValid())
	require.Equal(t, uint64(math.MaxUint64), source.last)
}
