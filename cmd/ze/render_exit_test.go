//go:build ze_core

package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// brokenPipeWriter fails every Write, simulating `ze help ai | head` closing the
// pipe (EPIPE) partway through a render.
type brokenPipeWriter struct{}

func (brokenPipeWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// TestRenderAIHelpErrorExit is the AC-10 non-zero-exit contract for the AI
// reference render path.
//
// VALIDATES: renderAIHelp returns 0 when output succeeds and 1 when a write
// fails, across the section, summary, and --json paths.
// PREVENTS: a regression back to the old behavior where fmt.Println swallowed
// the write error and the command exited 0 despite a truncated page.
func TestRenderAIHelpErrorExit(t *testing.T) {
	var buf bytes.Buffer
	assert.Equal(t, 0, renderAIHelp(&buf, []string{"cli"}), "healthy writer -> exit 0")
	assert.NotEmpty(t, buf.String(), "healthy render must produce output")

	assert.Equal(t, 1, renderAIHelp(brokenPipeWriter{}, []string{"cli"}), "broken pipe on a section -> exit 1")
	assert.Equal(t, 1, renderAIHelp(brokenPipeWriter{}, nil), "broken pipe on the summary -> exit 1")
	assert.Equal(t, 1, renderAIHelp(brokenPipeWriter{}, []string{"--json"}), "broken pipe on --json -> exit 1")

	var jbuf bytes.Buffer
	assert.Equal(t, 0, renderAIHelp(&jbuf, []string{"--json"}), "healthy --json -> exit 0")
	assert.NotEmpty(t, jbuf.String())
}

// TestRenderHelpCommandErrorExit is the AC-10 non-zero-exit contract for the
// command-catalog render path.
//
// VALIDATES: renderHelpCommand returns 0 on success and 1 on a write error for
// the table, verbose, and --json variants.
// PREVENTS: silent zero-exit on a truncated `ze help command` page.
func TestRenderHelpCommandErrorExit(t *testing.T) {
	var buf bytes.Buffer
	assert.Equal(t, 0, renderHelpCommand(&buf, nil), "healthy table -> exit 0")
	assert.NotEmpty(t, buf.String())

	assert.Equal(t, 1, renderHelpCommand(brokenPipeWriter{}, nil), "broken pipe table -> exit 1")
	assert.Equal(t, 1, renderHelpCommand(brokenPipeWriter{}, []string{"--verbose"}), "broken pipe verbose -> exit 1")
	assert.Equal(t, 1, renderHelpCommand(brokenPipeWriter{}, []string{"--json"}), "broken pipe json -> exit 1")
}
