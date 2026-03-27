package environ

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

func TestMain(m *testing.M) {
	env.MustRegister(env.EnvEntry{Key: "ze.test.environ.public", Type: "string", Default: "pub", Description: "public test var"})
	env.MustRegister(env.EnvEntry{Key: "ze.test.environ.secret", Type: "string", Default: "sec", Description: "private test var", Private: true})
	os.Exit(m.Run())
}

// captureStdout runs fn and returns what it wrote to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	require.NoError(t, w.Close())
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	return buf.String()
}

// captureStderr runs fn and returns what it wrote to stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	fn()

	require.NoError(t, w.Close())
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured stderr: %v", err)
	}
	return buf.String()
}

// TestShowAllExcludesPrivate verifies "ze env list" hides private entries.
//
// VALIDATES: Private env vars are hidden from listing output.
// PREVENTS: Private vars leaking into ze env list.
func TestShowAllExcludesPrivate(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"list"})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "ze.test.environ.public", "public var should appear in list")
	assert.NotContains(t, out, "ze.test.environ.secret", "private var should not appear in list")
}

// TestShowOneFindsPrivate verifies "ze env get" can look up private entries.
//
// VALIDATES: Direct lookup by key finds private entries and shows Private flag.
// PREVENTS: Regression if showOne() reverts to using Entries() instead of AllEntries().
func TestShowOneFindsPrivate(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"get", "ze.test.environ.secret"})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "ze.test.environ.secret")
	assert.Contains(t, out, "Private:     yes")
}

// TestShowOnePublicNoPrivateLabel verifies public entries don't show "Private:".
//
// VALIDATES: Public entries omit the Private label.
// PREVENTS: Spurious "Private:" line on non-private entries.
func TestShowOnePublicNoPrivateLabel(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"get", "ze.test.environ.public"})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "ze.test.environ.public")
	assert.NotContains(t, out, "Private:")
}

// TestShowOneUnknownKey verifies unknown key returns error.
//
// VALIDATES: Unknown key produces exit code 1 and stderr message.
// PREVENTS: Silent failure on unknown keys.
func TestShowOneUnknownKey(t *testing.T) {
	var code int
	stderr := captureStderr(t, func() {
		code = Run([]string{"get", "ze.does.not.exist"})
	})

	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "unknown env var")
}

// TestListRejectsUnknownFlags verifies "ze env list --garbage" fails.
//
// VALIDATES: Unknown flags are rejected via flag.NewFlagSet.
// PREVENTS: Silent ignore of typos in flags.
func TestListRejectsUnknownFlags(t *testing.T) {
	var code int
	_ = captureStderr(t, func() {
		code = Run([]string{"list", "--garbage"})
	})

	assert.Equal(t, 1, code)
}
