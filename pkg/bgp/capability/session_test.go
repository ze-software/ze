package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSessionCapsBasic verifies session capability storage.
//
// VALIDATES: SessionCaps stores all session-level capabilities.
// Note: ExtendedMessage moved to EncodingCaps (affects wire encoding).
//
// PREVENTS: Missing capability data after negotiation.
func TestSessionCapsBasic(t *testing.T) {
	sess := &SessionCaps{
		RouteRefresh:         true,
		EnhancedRouteRefresh: false,
		HoldTime:             90,
		GracefulRestart:      nil,
	}

	assert.True(t, sess.RouteRefresh)
	assert.False(t, sess.EnhancedRouteRefresh)
	assert.Equal(t, uint16(90), sess.HoldTime)
	assert.Nil(t, sess.GracefulRestart)
}

// TestSessionCapsWithGracefulRestart verifies GR storage.
//
// VALIDATES: GracefulRestart pointer is properly stored.
//
// PREVENTS: Lost GR state during capability negotiation.
func TestSessionCapsWithGracefulRestart(t *testing.T) {
	gr := &GracefulRestart{
		RestartTime: 120,
	}
	sess := &SessionCaps{
		GracefulRestart: gr,
	}

	assert.NotNil(t, sess.GracefulRestart)
	assert.Equal(t, uint16(120), sess.GracefulRestart.RestartTime)
}

// TestSessionCapsMismatches verifies mismatch tracking.
//
// VALIDATES: Mismatches slice is stored for logging/reporting.
//
// PREVENTS: Lost mismatch information per RFC 5492 Section 3.
func TestSessionCapsMismatches(t *testing.T) {
	sess := &SessionCaps{
		Mismatches: []Mismatch{
			{Code: CodeExtendedMessage, LocalSupported: true, PeerSupported: false},
			{Code: CodeRouteRefresh, LocalSupported: false, PeerSupported: true},
		},
	}

	assert.Len(t, sess.Mismatches, 2)
	assert.Equal(t, CodeExtendedMessage, sess.Mismatches[0].Code)
}
