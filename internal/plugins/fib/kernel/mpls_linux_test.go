// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS label validation tests

//go:build linux

package fibkernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMPLSLabelValidation(t *testing.T) {
	t.Run("valid single label", func(t *testing.T) {
		require.NoError(t, validateMPLSLabels([]uint32{100}))
	})

	t.Run("valid label stack", func(t *testing.T) {
		require.NoError(t, validateMPLSLabels([]uint32{100, 200, 300}))
	})

	t.Run("empty stack rejected", func(t *testing.T) {
		err := validateMPLSLabels([]uint32{})
		require.Error(t, err)
		assert.ErrorIs(t, err, errMPLSEmptyLabelStack)
	})

	t.Run("nil stack rejected", func(t *testing.T) {
		err := validateMPLSLabels(nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, errMPLSEmptyLabelStack)
	})

	t.Run("label exceeds 20-bit max", func(t *testing.T) {
		err := validateMPLSLabels([]uint32{1048576})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds 20-bit maximum")
	})

	t.Run("stack depth exceeds limit", func(t *testing.T) {
		labels := make([]uint32, 17)
		for i := range labels {
			labels[i] = uint32(i + 100)
		}
		err := validateMPLSLabels(labels)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds limit")
	})
}

func TestMPLSLabelValidationBoundary(t *testing.T) {
	t.Run("label 0 valid", func(t *testing.T) {
		require.NoError(t, validateMPLSLabels([]uint32{0}))
	})

	t.Run("label max valid (1048575)", func(t *testing.T) {
		require.NoError(t, validateMPLSLabels([]uint32{maxMPLSLabel}))
	})

	t.Run("label max+1 invalid (1048576)", func(t *testing.T) {
		require.Error(t, validateMPLSLabels([]uint32{maxMPLSLabel + 1}))
	})

	t.Run("stack depth 16 valid", func(t *testing.T) {
		labels := make([]uint32, 16)
		for i := range labels {
			labels[i] = uint32(i + 100)
		}
		require.NoError(t, validateMPLSLabels(labels))
	})

	t.Run("stack depth 17 invalid", func(t *testing.T) {
		labels := make([]uint32, 17)
		for i := range labels {
			labels[i] = uint32(i + 100)
		}
		require.Error(t, validateMPLSLabels(labels))
	})

	t.Run("large label in middle of stack", func(t *testing.T) {
		err := validateMPLSLabels([]uint32{100, maxMPLSLabel + 1, 200})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds 20-bit maximum")
	})
}
