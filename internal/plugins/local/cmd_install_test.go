// Design: docs/architecture/cli/plugin-modes.md — tests for local install

package local

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptPrefix_Default(t *testing.T) {
	r := strings.NewReader("\n")
	var w bytes.Buffer
	p, err := promptPrefix(r, &w)
	require.NoError(t, err)
	assert.Equal(t, "/usr/local", p)
	assert.Contains(t, w.String(), "Select installation prefix:")
}

func TestPromptPrefix_Choice2(t *testing.T) {
	r := strings.NewReader("2\n")
	var w bytes.Buffer
	p, err := promptPrefix(r, &w)
	require.NoError(t, err)
	assert.Equal(t, "/usr", p)
}

func TestPromptPrefix_Choice3(t *testing.T) {
	r := strings.NewReader("3\n")
	var w bytes.Buffer
	p, err := promptPrefix(r, &w)
	require.NoError(t, err)
	assert.Equal(t, "/opt/ze", p)
}

func TestPromptPrefix_Invalid(t *testing.T) {
	r := strings.NewReader("5\n")
	var w bytes.Buffer
	_, err := promptPrefix(r, &w)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid choice")
}

func TestPromptPrefix_NoInput(t *testing.T) {
	r := strings.NewReader("")
	var w bytes.Buffer
	_, err := promptPrefix(r, &w)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no input")
}

func TestInstallHelp(t *testing.T) {
	code := RunInstall([]string{"-h"})
	assert.Equal(t, 0, code)
}

func TestUninstallHelp(t *testing.T) {
	code := RunUninstall([]string{"-h"})
	assert.Equal(t, 0, code)
}

func TestInstallRejectsWhitespacePrefix(t *testing.T) {
	code := RunInstall([]string{"--prefix", "/usr local", "--dry-run"})
	assert.Equal(t, 1, code)
}
