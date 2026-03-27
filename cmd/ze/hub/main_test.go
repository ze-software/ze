package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// TestRunMissingConfig verifies error handling for missing config.
//
// VALIDATES: Hub returns error for non-existent config.
// PREVENTS: Silent failure when config file not found.
func TestRunMissingConfig(t *testing.T) {
	exit := Run(storage.NewFilesystem(), "/nonexistent/config.conf", nil, 0, -1, false)
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

	exit := Run(storage.NewFilesystem(), configPath, nil, 0, -1, false)
	assert.Equal(t, 1, exit)
}

// TestReadStdinConfigWithNUL verifies that a NUL sentinel stops reading
// and reports stdin as still open for liveness monitoring.
//
// VALIDATES: Config data before NUL is returned, stdinOpen=true.
// PREVENTS: Ze blocking forever on stdin when upstream keeps pipe open.
func TestReadStdinConfigWithNUL(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdin = r
	config := "bgp {\n  local {\n    as 65000;\n  }\n}\n"

	// Keep the write end open until test completes (simulates long-lived upstream).
	done := make(chan struct{})
	go func() {
		if _, wErr := w.WriteString(config); wErr != nil {
			return
		}
		if _, wErr := w.Write([]byte{0}); wErr != nil {
			return
		}
		<-done
	}()

	data, stdinOpen, readErr := readStdinConfig()
	close(done) // Release goroutine.

	assert.NoError(t, readErr)
	assert.True(t, stdinOpen, "stdin should remain open after NUL sentinel")
	assert.Equal(t, config, string(data))

	if closeErr := w.Close(); closeErr != nil {
		t.Log("close pipe writer:", closeErr)
	}
	if closeErr := r.Close(); closeErr != nil {
		t.Log("close pipe reader:", closeErr)
	}
}

// TestReadStdinConfigEOF verifies that plain EOF (no NUL) returns the
// full data with stdinOpen=false — the normal "cat file | ze -" case.
//
// VALIDATES: Full config returned, stdinOpen=false on plain EOF.
// PREVENTS: False liveness monitoring when stdin is a regular file/pipe.
func TestReadStdinConfigEOF(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	// Write and close before reading — pipe buffer holds the data.
	config := "bgp {\n  local {\n    as 65000;\n  }\n}\n"
	if _, wErr := w.WriteString(config); wErr != nil {
		t.Fatal(wErr)
	}
	if closeErr := w.Close(); closeErr != nil {
		t.Log("close pipe writer:", closeErr)
	}

	os.Stdin = r

	data, stdinOpen, readErr := readStdinConfig()
	assert.NoError(t, readErr)
	assert.False(t, stdinOpen, "stdin should be closed after EOF")
	assert.Equal(t, config, string(data))

	if closeErr := r.Close(); closeErr != nil {
		t.Log("close pipe reader:", closeErr)
	}
}

// TestReadStdinConfigEmpty verifies empty stdin returns empty data.
//
// VALIDATES: Empty stdin returns empty slice, no error.
// PREVENTS: Panic or error on empty pipe input.
func TestReadStdinConfigEmpty(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	if closeErr := w.Close(); closeErr != nil {
		t.Log("close pipe writer:", closeErr)
	}
	os.Stdin = r

	data, stdinOpen, readErr := readStdinConfig()
	assert.NoError(t, readErr)
	assert.False(t, stdinOpen)
	assert.Empty(t, data)

	if closeErr := r.Close(); closeErr != nil {
		t.Log("close pipe reader:", closeErr)
	}
}
