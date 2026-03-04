package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPromptCreateConfigYes verifies that answering "y" creates the file.
//
// VALIDATES: "y" answer creates an empty config file with correct permissions.
//
// PREVENTS: Create prompt accepting "y" but failing to create the file.
func TestPromptCreateConfigYes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	in := strings.NewReader("y\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if !ok {
		t.Fatal("expected true for 'y' answer")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	if info.Size() != 0 {
		t.Errorf("expected empty file, got %d bytes", info.Size())
	}

	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected permissions 0600, got %04o", info.Mode().Perm())
	}
}

// TestPromptCreateConfigYesFull verifies that "yes" (full word) also works.
//
// VALIDATES: "yes" is accepted as affirmative.
//
// PREVENTS: Only single-letter "y" being accepted.
func TestPromptCreateConfigYesFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	in := strings.NewReader("yes\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if !ok {
		t.Fatal("expected true for 'yes' answer")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

// TestPromptCreateConfigNo verifies that "n" does not create the file.
//
// VALIDATES: "n" answer returns false without creating file.
//
// PREVENTS: Accidental file creation on negative answer.
func TestPromptCreateConfigNo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	in := strings.NewReader("n\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if ok {
		t.Fatal("expected false for 'n' answer")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should not exist after 'n' answer")
	}
}

// TestPromptCreateConfigEmpty verifies that empty input (just Enter) is treated as no.
//
// VALIDATES: Default [N] behavior — empty input declines creation.
//
// PREVENTS: Empty input being treated as affirmative.
func TestPromptCreateConfigEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	in := strings.NewReader("\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if ok {
		t.Fatal("expected false for empty answer")
	}
}

// TestPromptCreateConfigTimeout verifies that no input triggers timeout.
//
// VALIDATES: Timeout returns false and prints error message.
//
// PREVENTS: Hanging forever when stdin has no input.
func TestPromptCreateConfigTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.conf")

	// Reader that blocks forever (never returns data)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close() //nolint:errcheck // test cleanup
	defer w.Close() //nolint:errcheck // test cleanup

	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, r, &errBuf, 50*time.Millisecond)

	if ok {
		t.Fatal("expected false on timeout")
	}

	if !strings.Contains(errBuf.String(), "no response") {
		t.Errorf("expected timeout message, got: %s", errBuf.String())
	}
}

// TestPromptCreateConfigCreatesParentDirs verifies that parent directories are created.
//
// VALIDATES: Missing parent directories are created with 0750 permissions.
//
// PREVENTS: Failure when parent directory doesn't exist.
func TestPromptCreateConfigCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "new.conf")

	in := strings.NewReader("y\n")
	var errBuf bytes.Buffer

	ok := doPromptCreateConfig(path, in, &errBuf, time.Second)

	if !ok {
		t.Fatalf("expected true, stderr: %s", errBuf.String())
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// Check parent dir permissions
	info, err := os.Stat(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatalf("parent dir not created: %v", err)
	}

	if info.Mode().Perm() != 0o750 {
		t.Errorf("expected dir permissions 0750, got %04o", info.Mode().Perm())
	}
}

// TestPromptCreateConfigCaseInsensitive verifies that "Y" and "YES" are accepted.
//
// VALIDATES: Answer comparison is case-insensitive.
//
// PREVENTS: Uppercase input being rejected.
func TestPromptCreateConfigCaseInsensitive(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"uppercase Y", "Y\n", true},
		{"uppercase YES", "YES\n", true},
		{"mixed Yes", "Yes\n", true},
		{"no", "no\n", false},
		{"random", "maybe\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "new.conf")

			in := strings.NewReader(tt.input)
			var errBuf bytes.Buffer

			got := doPromptCreateConfig(path, in, &errBuf, time.Second)

			if got != tt.want {
				t.Errorf("input %q: got %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestPromptCreateConfigOutputFormat verifies the prompt message format.
//
// VALIDATES: Prompt includes file path and [y/N] indicator.
//
// PREVENTS: Missing context in the prompt shown to users.
func TestPromptCreateConfigOutputFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myconfig.conf")

	in := strings.NewReader("n\n")
	var errBuf bytes.Buffer

	doPromptCreateConfig(path, in, &errBuf, time.Second)

	output := errBuf.String()

	if !strings.Contains(output, path) {
		t.Errorf("expected path in output, got: %s", output)
	}

	if !strings.Contains(output, "[y/N]") {
		t.Errorf("expected [y/N] prompt, got: %s", output)
	}
}
