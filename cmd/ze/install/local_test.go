// Design: plan/spec-install-0-umbrella.md — tests for ze install local

package install

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

func TestPromptPrefix_NonNumeric(t *testing.T) {
	r := strings.NewReader("abc\n")
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

func TestBuildSystemdUnit(t *testing.T) {
	unit := buildSystemdUnit("/usr/local/bin/ze")
	assert.Contains(t, unit, "ExecStart=/usr/local/bin/ze start")
	assert.Contains(t, unit, "Description=Ze Network OS")
	assert.Contains(t, unit, "After=network-online.target")
	assert.Contains(t, unit, "Restart=on-failure")
	assert.Contains(t, unit, "WantedBy=multi-user.target")
}

func TestBuildSystemdUnit_OptPrefix(t *testing.T) {
	unit := buildSystemdUnit("/opt/ze/bin/ze")
	assert.Contains(t, unit, "ExecStart=/opt/ze/bin/ze start")
}

func TestRunLocal_Help(t *testing.T) {
	code := runLocal([]string{"-h"})
	assert.Equal(t, 0, code)
}

func TestRunLocal_MutuallyExclusiveFlags(t *testing.T) {
	code := runLocal([]string{"--systemd", "--no-systemd", "--prefix", "/usr/local", "--dry-run"})
	assert.Equal(t, 1, code, "should reject --systemd and --no-systemd together")
}
