package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCmdArchive_NoArgs verifies error when no arguments are provided.
//
// VALIDATES: Missing arguments produces usage error.
// PREVENTS: Panic or confusing error on missing argument.
func TestCmdArchive_NoArgs(t *testing.T) {
	code := cmdArchiveImpl([]string{})
	assert.Equal(t, exitError, code)
}

// TestCmdArchive_NoDaemon verifies error when daemon is not running.
//
// VALIDATES: Archive command fails gracefully when daemon is unreachable.
// PREVENTS: Panic or hang when daemon is not running.
func TestCmdArchive_NoDaemon(t *testing.T) {
	t.Setenv("ze_ssh_port", "19999")
	code := cmdArchiveImpl([]string{"backup"})
	assert.Equal(t, exitError, code)
}
