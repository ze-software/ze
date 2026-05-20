// Design: docs/architecture/web-interface.md -- Cross-service conflict detection tests

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConflictDetection verifies that swapping addresses between two services
// is detected as a conflict.
// VALIDATES: AC-9.
func TestConflictDetection(t *testing.T) {
	changes := []serviceChange{
		{
			name:   "web",
			remove: []string{"0.0.0.0:3443"},
			add:    []string{"0.0.0.0:8080"},
		},
		{
			name:   "api",
			remove: []string{"0.0.0.0:8080"},
			add:    []string{"0.0.0.0:3443"},
		},
	}

	conflicts := detectConflicts(changes)
	assert.True(t, conflicts["web"], "web must be flagged as conflicting")
	assert.True(t, conflicts["api"], "api must be flagged as conflicting")
}

// TestConflictDetectionNoConflict verifies that independent address changes
// are not flagged as conflicts.
func TestConflictDetectionNoConflict(t *testing.T) {
	changes := []serviceChange{
		{
			name:   "web",
			remove: []string{"0.0.0.0:3443"},
			add:    []string{"0.0.0.0:9090"},
		},
		{
			name:   "api",
			remove: []string{"0.0.0.0:8080"},
			add:    []string{"0.0.0.0:7070"},
		},
	}

	conflicts := detectConflicts(changes)
	assert.Empty(t, conflicts, "no conflicts expected for independent changes")
}

// TestConflictDetectionOneDirection verifies that a one-directional conflict
// (one service releases an address another acquires, but not vice versa)
// is still detected.
func TestConflictDetectionOneDirection(t *testing.T) {
	changes := []serviceChange{
		{
			name:   "web",
			remove: []string{"0.0.0.0:3443"},
			add:    []string{"0.0.0.0:9090"},
		},
		{
			name:   "mcp",
			remove: nil,
			add:    []string{"0.0.0.0:3443"},
		},
	}

	conflicts := detectConflicts(changes)
	assert.True(t, conflicts["web"], "web must be flagged (releases addr mcp wants)")
	assert.True(t, conflicts["mcp"], "mcp must be flagged (acquires addr web releases)")
}

// TestConflictDetectionSingleService verifies that a single service changing
// addresses has no cross-service conflict.
func TestConflictDetectionSingleService(t *testing.T) {
	changes := []serviceChange{
		{
			name:   "web",
			remove: []string{"0.0.0.0:3443"},
			add:    []string{"0.0.0.0:8080"},
		},
	}

	conflicts := detectConflicts(changes)
	assert.Empty(t, conflicts, "single service cannot conflict with itself")
}
