//go:build debug

package attrpool

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The debug build (validate_debug.go) and the release build (validate_release.go)
// both reject invalid handles with the same sentinel errors. The debug build is
// distinguished by attaching diagnostic detail to the error message (e.g. the
// offending handle, slot, and bounds). These tests assert both: the sentinel
// (shared contract) and the debug-only detail suffix (proof the debug validator
// is wired into the public API). Asserting the detail is what gives the
// shard-aware debug validators real coverage; a plain errors.Is would pass under
// either build tag and prove nothing about the debug path.

// TestDebugValidationCatchesInvalidHandle verifies the debug validator rejects
// an invalid handle on Get with a detailed error.
//
// VALIDATES: Debug builds catch programming errors early.
//
// PREVENTS: Silent corruption in production from invalid handle usage.
func TestDebugValidationCatchesInvalidHandle(t *testing.T) {
	p := New(1024)

	_, err := p.Get(InvalidHandle)
	require.ErrorIs(t, err, ErrInvalidHandle, "Get(InvalidHandle) must report ErrInvalidHandle")
	require.True(t, strings.Contains(err.Error(), "handle="),
		"debug build must attach handle detail, got %q", err.Error())
}

// TestDebugValidationCatchesOutOfBounds verifies the debug validator rejects an
// out-of-bounds slot on Get with a detailed error.
//
// VALIDATES: Bounds checking in debug mode.
//
// PREVENTS: Buffer overflow exploits from malformed handles.
func TestDebugValidationCatchesOutOfBounds(t *testing.T) {
	p := New(1024)

	// Create one entry so at least one shard has a non-empty slot table.
	mustIntern(t, p, []byte("data"))

	// Handle(999999) has a valid PoolIdx (0) but a slot far beyond any shard's
	// slot table, so it must trip the bounds check rather than IsValid/PoolIdx.
	_, err := p.Get(Handle(999999))
	require.ErrorIs(t, err, ErrSlotOutOfBounds, "Get(OOB handle) must report ErrSlotOutOfBounds")
	require.True(t, strings.Contains(err.Error(), "slot="),
		"debug build must attach slot detail, got %q", err.Error())
}

// TestDebugValidationCatchesDeadSlot verifies the debug validator rejects a
// released handle on Get with a detailed error.
//
// VALIDATES: Use-after-free detection in debug mode.
//
// PREVENTS: Accessing released entries that may have been reused.
func TestDebugValidationCatchesDeadSlot(t *testing.T) {
	p := New(1024)
	h := mustIntern(t, p, []byte("data"))
	require.NoError(t, p.Release(h))

	_, err := p.Get(h)
	require.ErrorIs(t, err, ErrSlotDead, "Get(released handle) must report ErrSlotDead")
	require.True(t, strings.Contains(err.Error(), "slot="),
		"debug build must attach slot detail, got %q", err.Error())
}

// TestDebugReleaseInvalidHandle verifies the debug validator rejects an invalid
// handle on Release with a detailed error.
//
// VALIDATES: Invalid handle detection on Release.
//
// PREVENTS: Corrupting reference counts with invalid handles.
func TestDebugReleaseInvalidHandle(t *testing.T) {
	p := New(1024)

	err := p.Release(InvalidHandle)
	require.ErrorIs(t, err, ErrInvalidHandle, "Release(InvalidHandle) must report ErrInvalidHandle")
	require.True(t, strings.Contains(err.Error(), "handle="),
		"debug build must attach handle detail, got %q", err.Error())
}

// TestDebugLengthInvalidHandle verifies the debug validator rejects an invalid
// handle on Length with a detailed error.
//
// VALIDATES: Invalid handle detection on Length.
//
// PREVENTS: Reading garbage length values.
func TestDebugLengthInvalidHandle(t *testing.T) {
	p := New(1024)

	_, err := p.Length(InvalidHandle)
	require.ErrorIs(t, err, ErrInvalidHandle, "Length(InvalidHandle) must report ErrInvalidHandle")
	require.True(t, strings.Contains(err.Error(), "handle="),
		"debug build must attach handle detail, got %q", err.Error())
}
