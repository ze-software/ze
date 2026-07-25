package env

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestMain(m *testing.M) {
	env.MustRegister(env.EnvEntry{Key: "ze.test.environ.public", Type: "string", Default: "pub", Description: "public test var"})
	env.MustRegister(env.EnvEntry{Key: "ze.test.environ.secret", Type: "string", Default: "sec", Description: "private test var", Private: true})
	os.Exit(m.Run())
}

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

func TestShowAllExcludesPrivate(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"list"})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "ze.test.environ.public", "public var should appear in list")
	assert.NotContains(t, out, "ze.test.environ.secret", "private var should not appear in list")
}

func TestShowOneFindsPrivate(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"get", "ze.test.environ.secret"})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "ze.test.environ.secret")
	assert.Contains(t, out, "Private:     yes")
}

func TestShowOnePublicNoPrivateLabel(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"get", "ze.test.environ.public"})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "ze.test.environ.public")
	assert.NotContains(t, out, "Private:")
}

func TestShowOneUnknownKey(t *testing.T) {
	var code int
	stderr := captureStderr(t, func() {
		code = Run([]string{"get", "ze.does.not.exist"})
	})

	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "unknown env var")
}

// VALIDATES: AC-6 — ze env get resolves concrete log-subsystem keys.
// PREVENTS: completed log key returning "unknown env var" from showOne.
func TestShowOneFindsConcreteLogSubsystem(t *testing.T) {
	slogutil.LazyLogger("test.env.logsub")

	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"get", "ze.log.test.env.logsub"})
	})

	assert.Equal(t, 0, code, "ze env get should succeed for concrete log subsystem key")
	assert.Contains(t, out, "ze.log.test.env.logsub")
	assert.Contains(t, out, "Key:")
}

func TestListRejectsUnknownFlags(t *testing.T) {
	var code int
	_ = captureStderr(t, func() {
		code = Run([]string{"list", "--garbage"})
	})

	assert.Equal(t, 1, code)
}
