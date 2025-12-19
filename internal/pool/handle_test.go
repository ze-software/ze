package pool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHandleValid verifies that Valid() correctly identifies valid handles.
//
// VALIDATES: Handle validity check works correctly.
//
// PREVENTS: Invalid handles being used in pool operations, causing
// out-of-bounds access or data corruption.
func TestHandleValid(t *testing.T) {
	tests := []struct {
		name  string
		h     Handle
		valid bool
	}{
		{"zero is valid", Handle(0), true},
		{"positive is valid", Handle(100), true},
		{"max-1 is valid", Handle(0xFFFFFFFE), true},
		{"InvalidHandle is not valid", InvalidHandle, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.h.Valid())
		})
	}
}

// TestInvalidHandleConstant verifies InvalidHandle has expected value.
//
// VALIDATES: Sentinel value is correct.
//
// PREVENTS: Accidental collision with valid handle values.
func TestInvalidHandleConstant(t *testing.T) {
	assert.Equal(t, Handle(0xFFFFFFFF), InvalidHandle)
}

// TestHandleString verifies string representation for debugging.
//
// VALIDATES: Handle is printable for debugging.
//
// PREVENTS: Opaque values in logs making debugging difficult.
func TestHandleString(t *testing.T) {
	tests := []struct {
		h        Handle
		expected string
	}{
		{Handle(0), "Handle(0)"},
		{Handle(42), "Handle(42)"},
		{InvalidHandle, "InvalidHandle"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.h.String())
		})
	}
}
