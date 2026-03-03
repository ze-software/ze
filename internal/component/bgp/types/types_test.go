package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
)

// TestRawMessageIsAsyncSafe verifies IsAsyncSafe depends on WireUpdate.
//
// VALIDATES: RawMessage with nil WireUpdate is async-safe; with non-nil is not.
// PREVENTS: Regression if IsAsyncSafe logic is accidentally inverted.
func TestRawMessageIsAsyncSafe(t *testing.T) {
	safe := &RawMessage{RawBytes: []byte{1, 2, 3}, Timestamp: time.Now()}
	assert.True(t, safe.IsAsyncSafe(), "nil WireUpdate should be async-safe")

	unsafe := &RawMessage{WireUpdate: &wireu.WireUpdate{}}
	assert.False(t, unsafe.IsAsyncSafe(), "non-nil WireUpdate should not be async-safe")
}

// TestContentConfigWithDefaults verifies default values are applied.
//
// VALIDATES: Empty Encoding/Format get defaults "text"/"parsed".
// PREVENTS: Missing defaults causing empty-string comparisons downstream.
func TestContentConfigWithDefaults(t *testing.T) {
	// Zero value gets defaults
	zero := ContentConfig{}
	filled := zero.WithDefaults()
	assert.Equal(t, "text", filled.Encoding)
	assert.Equal(t, "parsed", filled.Format)

	// Explicit values preserved
	explicit := ContentConfig{Encoding: "json", Format: "raw"}
	kept := explicit.WithDefaults()
	assert.Equal(t, "json", kept.Encoding)
	assert.Equal(t, "raw", kept.Format)
}
