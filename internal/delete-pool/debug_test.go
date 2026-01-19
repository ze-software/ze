//go:build debug

package pool

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDebugValidationCatchesInvalidHandle verifies debug build catches bad handles.
//
// VALIDATES: Debug builds catch programming errors early.
//
// PREVENTS: Silent corruption in production from invalid handle usage.
// Debug builds should panic to catch errors during development.
func TestDebugValidationCatchesInvalidHandle(t *testing.T) {
	p := New(1024)

	require.Panics(t, func() {
		p.Get(InvalidHandle)
	}, "Get(InvalidHandle) must panic in debug build")
}

// TestDebugValidationCatchesOutOfBounds verifies debug build catches OOB.
//
// VALIDATES: Bounds checking in debug mode.
//
// PREVENTS: Buffer overflow exploits from malformed handles.
func TestDebugValidationCatchesOutOfBounds(t *testing.T) {
	p := New(1024)

	// Create one entry so slots has length 1
	p.Intern([]byte("data"))

	require.Panics(t, func() {
		p.Get(Handle(999999)) // Way out of bounds
	}, "Get(OOB handle) must panic in debug build")
}

// TestDebugValidationCatchesDeadSlot verifies debug build catches dead access.
//
// VALIDATES: Use-after-free detection in debug mode.
//
// PREVENTS: Accessing released entries that may have been reused.
func TestDebugValidationCatchesDeadSlot(t *testing.T) {
	p := New(1024)
	h := p.Intern([]byte("data"))
	p.Release(h)

	require.Panics(t, func() {
		p.Get(h)
	}, "Get(released handle) must panic in debug build")
}

// TestDebugReleaseInvalidHandle verifies Release catches invalid handles.
//
// VALIDATES: Invalid handle detection on Release.
//
// PREVENTS: Corrupting reference counts with invalid handles.
func TestDebugReleaseInvalidHandle(t *testing.T) {
	p := New(1024)

	require.Panics(t, func() {
		p.Release(InvalidHandle)
	}, "Release(InvalidHandle) must panic in debug build")
}

// TestDebugLengthInvalidHandle verifies Length catches invalid handles.
//
// VALIDATES: Invalid handle detection on Length.
//
// PREVENTS: Reading garbage length values.
func TestDebugLengthInvalidHandle(t *testing.T) {
	p := New(1024)

	require.Panics(t, func() {
		p.Length(InvalidHandle)
	}, "Length(InvalidHandle) must panic in debug build")
}
