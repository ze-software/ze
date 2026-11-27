package inprocess

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpreactor "github.com/ze-software/ze/internal/component/bgp/reactor"
)

// TestIsLegalFSMTransition verifies the RFC 4271 Section 8.2.2 state transition
// oracle accepts every valid BGP edge and rejects illegal skips and unknown states.
//
// VALIDATES: Online FSM invariant checker catches illegal state jumps.
// PREVENTS: Vacuous oracle that accepts IDLE->ESTABLISHED or typos in state names.
func TestIsLegalFSMTransition(t *testing.T) {
	legal := [][2]string{
		{"IDLE", "ACTIVE"},
		{"IDLE", "CONNECT"},
		{"ACTIVE", "OPENSENT"},
		{"CONNECT", "OPENSENT"},
		{"OPENSENT", "OPENCONFIRM"},
		{"OPENSENT", "ACTIVE"},
		{"OPENCONFIRM", "ESTABLISHED"},
		{"ESTABLISHED", "IDLE"},
		{"OPENCONFIRM", "IDLE"},
		{"OPENSENT", "IDLE"},
	}
	for _, pair := range legal {
		assert.True(t, IsLegalFSMTransition(pair[0], pair[1]),
			"expected %s -> %s to be legal", pair[0], pair[1])
	}

	illegal := [][2]string{
		{"IDLE", "ESTABLISHED"},
		{"IDLE", "OPENSENT"},
		{"IDLE", "OPENCONFIRM"},
		{"ACTIVE", "ESTABLISHED"},
		{"ACTIVE", "OPENCONFIRM"},
		{"CONNECT", "ESTABLISHED"},
		{"CONNECT", "OPENCONFIRM"},
		{"OPENSENT", "ESTABLISHED"},
		{"ESTABLISHED", "OPENSENT"},
		{"ESTABLISHED", "ACTIVE"},
		{"UNKNOWN", "IDLE"},
		{"IDLE", "BOGUS"},
	}
	for _, pair := range illegal {
		assert.False(t, IsLegalFSMTransition(pair[0], pair[1]),
			"expected %s -> %s to be rejected", pair[0], pair[1])
	}
}

// TestValidateFSMTransitionsFlagsIllegalJump verifies ValidateFSMTransitions
// passes on a valid handshake sequence and reports exact diagnostics on an
// illegal jump.
//
// VALIDATES: StateChecker reports peer identity, states, and reason on failure.
// PREVENTS: Silent passes when an illegal transition appears in Peer.FSMHistory.
func TestValidateFSMTransitionsFlagsIllegalJump(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	validSeq := []bgpreactor.FSMTransition{
		{Timestamp: now, From: "IDLE", To: "ACTIVE", Reason: "passive start"},
		{Timestamp: now.Add(time.Second), From: "ACTIVE", To: "OPENSENT", Reason: "tcp accepted"},
		{Timestamp: now.Add(2 * time.Second), From: "OPENSENT", To: "OPENCONFIRM", Reason: "open ok"},
		{Timestamp: now.Add(3 * time.Second), From: "OPENCONFIRM", To: "ESTABLISHED", Reason: "keepalive"},
		{Timestamp: now.Add(4 * time.Second), From: "ESTABLISHED", To: "IDLE", Reason: "disconnect"},
	}
	assert.Empty(t, ValidateFSMTransitions("127.0.0.2", validSeq))

	badSeq := make([]bgpreactor.FSMTransition, 0, len(validSeq)+1)
	badSeq = append(badSeq, validSeq...)
	badSeq = append(badSeq, bgpreactor.FSMTransition{
		Timestamp: now.Add(5 * time.Second), From: "IDLE", To: "ESTABLISHED", Reason: "bug",
	})
	violations := ValidateFSMTransitions("127.0.0.2", badSeq)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "127.0.0.2")
	assert.Contains(t, violations[0], "IDLE -> ESTABLISHED")
	assert.Contains(t, violations[0], "bug")
}
