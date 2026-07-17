// VALIDATES: cliio resolves "-" to stdin (read) / stdout (write) and reads/writes
// real paths unchanged; the stdin-once guard fails closed on a second claim.
// PREVENTS: a silent empty read when stdin is claimed twice (R-1, AC-9), and a
// "-" that leaks into path handling as a literal filename.
package cliio

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withStdin swaps the package stdin source and resets the once-guard for the
// duration of the test. White-box access keeps the 34 call sites free of any
// injection plumbing.
func withStdin(t *testing.T, content string) {
	t.Helper()
	prev := stdin
	stdin = strings.NewReader(content)
	stdinClaimed.Store(false)
	t.Cleanup(func() {
		stdin = prev
		stdinClaimed.Store(false)
	})
}

func withStdout(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := stdout
	buf := &bytes.Buffer{}
	stdout = buf
	t.Cleanup(func() { stdout = prev })
	return buf
}

func TestReadPathDash(t *testing.T) {
	// "-" reads stdin.
	withStdin(t, "hello from stdin")
	got, err := ReadFile("-")
	if err != nil {
		t.Fatalf("ReadFile(-): %v", err)
	}
	if string(got) != "hello from stdin" {
		t.Fatalf("stdin read = %q, want %q", got, "hello from stdin")
	}

	// A real path reads the file.
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.conf")
	if err := os.WriteFile(p, []byte("on disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", p, err)
	}
	if string(got) != "on disk" {
		t.Fatalf("file read = %q, want %q", got, "on disk")
	}

	// A missing file errors.
	if _, err := ReadFile(filepath.Join(dir, "nope.conf")); err == nil {
		t.Fatal("ReadFile(missing) = nil error, want error")
	}
}

func TestWritePathDash(t *testing.T) {
	// "-" writes stdout.
	buf := withStdout(t)
	if err := WriteFile("-", []byte("to stdout"), 0o600); err != nil {
		t.Fatalf("WriteFile(-): %v", err)
	}
	if buf.String() != "to stdout" {
		t.Fatalf("stdout = %q, want %q", buf.String(), "to stdout")
	}

	// A real path writes the file.
	dir := t.TempDir()
	p := filepath.Join(dir, "out.conf")
	if err := WriteFile(p, []byte("to file"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", p, err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "to file" {
		t.Fatalf("file = %q, want %q", got, "to file")
	}
}

func TestCreateDash(t *testing.T) {
	// "-" yields a writer over stdout whose Close does NOT close os.Stdout.
	buf := withStdout(t)
	w, err := Create("-")
	if err != nil {
		t.Fatalf("Create(-): %v", err)
	}
	if _, err := io.WriteString(w, "streamed"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close stdout writer: %v", err)
	}
	if buf.String() != "streamed" {
		t.Fatalf("stdout = %q, want %q", buf.String(), "streamed")
	}

	// A real path yields a file writer.
	dir := t.TempDir()
	p := filepath.Join(dir, "created.bin")
	w, err = Create(p)
	if err != nil {
		t.Fatalf("Create(%s): %v", p, err)
	}
	if _, err := io.WriteString(w, "filebytes"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "filebytes" {
		t.Fatalf("file = %q, want %q", got, "filebytes")
	}
}

func TestOpenReaderDash(t *testing.T) {
	// "-" yields a reader over stdin; Close does not close os.Stdin.
	withStdin(t, "streamed stdin")
	rc, err := OpenReader("-")
	if err != nil {
		t.Fatalf("OpenReader(-): %v", err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close stdin reader: %v", err)
	}
	if string(got) != "streamed stdin" {
		t.Fatalf("stdin stream = %q, want %q", got, "streamed stdin")
	}

	// A real path yields a file reader.
	dir := t.TempDir()
	p := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(p, []byte("filestream"), 0o600); err != nil {
		t.Fatal(err)
	}
	rc, err = OpenReader(p)
	if err != nil {
		t.Fatalf("OpenReader(%s): %v", p, err)
	}
	got, err = io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
	if string(got) != "filestream" {
		t.Fatalf("file stream = %q, want %q", got, "filestream")
	}
}

// TestStdinClaimedOnce asserts the fail-closed guard: the second claim of the
// single stdin returns ErrStdinClaimed, never a silent empty read (R-1, AC-9).
func TestStdinClaimedOnce(t *testing.T) {
	withStdin(t, "only once")

	if _, err := ReadFile("-"); err != nil {
		t.Fatalf("first ReadFile(-): %v", err)
	}
	_, err := ReadFile("-")
	if err == nil {
		t.Fatal("second ReadFile(-) = nil error, want ErrStdinClaimed")
	}
	if !errors.Is(err, ErrStdinClaimed) {
		t.Fatalf("second ReadFile(-) error = %v, want ErrStdinClaimed", err)
	}

	// A second claim via OpenReader after ReadFile also fails (mixed forms).
	if _, err := OpenReader("-"); !errors.Is(err, ErrStdinClaimed) {
		t.Fatalf("OpenReader(-) after claim error = %v, want ErrStdinClaimed", err)
	}

	// A real path is never blocked by the stdin guard.
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(p); err != nil {
		t.Fatalf("ReadFile(real path) blocked by guard: %v", err)
	}
}

func TestIsStdin(t *testing.T) {
	if !IsStdin("-") {
		t.Fatal(`IsStdin("-") = false, want true`)
	}
	for _, s := range []string{"", "-x", "./-", "file", "/dev/stdin"} {
		if IsStdin(s) {
			t.Fatalf("IsStdin(%q) = true, want false", s)
		}
	}
}
