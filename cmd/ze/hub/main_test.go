package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRunMissingConfig verifies error handling for missing config.
//
// VALIDATES: Hub returns error for non-existent config.
// PREVENTS: Silent failure when config file not found.
func TestRunMissingConfig(t *testing.T) {
	exit := Run("/nonexistent/config.conf", nil, 0, -1)
	assert.Equal(t, 1, exit)
}

// TestRunInvalidConfig verifies error handling for invalid config.
//
// VALIDATES: Hub returns error for malformed config.
// PREVENTS: Crash on invalid config syntax.
func TestRunInvalidConfig(t *testing.T) {
	// Create temp config with invalid syntax
	dir := t.TempDir()
	configPath := filepath.Join(dir, "invalid.conf")
	err := os.WriteFile(configPath, []byte("invalid { syntax"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	exit := Run(configPath, nil, 0, -1)
	assert.Equal(t, 1, exit)
}
