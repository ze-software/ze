package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// captureCatStdio runs fn with stdout and stderr redirected to pipes and
// answers its exit code and what it wrote to each. Not safe for parallel tests.
func captureCatStdio(t *testing.T, fn func() int) (code int, stdout, stderr string) {
	t.Helper()
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	savedOut, savedErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outWrite, errWrite
	code = fn()
	os.Stdout, os.Stderr = savedOut, savedErr
	if closeErr := outWrite.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if closeErr := errWrite.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	out, _ := io.ReadAll(outRead)    //nolint:errcheck // a short read shows as a mismatch
	errOut, _ := io.ReadAll(errRead) //nolint:errcheck // a short read shows as a mismatch
	outRead.Close()                  //nolint:errcheck // test pipe
	errRead.Close()                  //nolint:errcheck // test pipe
	return code, string(out), string(errOut)
}

// TestShowDataCatAnswersTheStoredBytes proves `show data cat` answers the
// bytes of a stored key as a value, binary included, writing nothing itself,
// and that `ze data cat` prints exactly those bytes, as os.Stdout.Write(data)
// did before the conversion.
//
// VALIDATES: the payload is a command.Blob equal to the stored bytes; the
// handler writes nothing; the printing route writes the bytes unchanged with
// no newline added; a missing key and a missing argument come back as the old
// error line and usage line with their old exit codes.
// PREVENTS: the bytes landing on the daemon's stdout when it serves the
// command, and a binary file gaining or losing a byte on the local route.
func TestShowDataCatAnswersTheStoredBytes(t *testing.T) {
	dir := t.TempDir()
	owner, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	stored := []byte("certificate\n\xff\x00\x01")
	if err := owner.WriteKey("meta/blob", stored); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "database")

	var answer command.LocalAnswer
	_, stdout, stderr := captureCatStdio(t, func() int {
		answer = dataCat(commandtest.Args("--path", path, "meta/blob"))
		return 0
	})
	if stdout != "" || stderr != "" {
		t.Errorf("dataCat wrote stdout %q stderr %q; a handler writes nothing", stdout, stderr)
	}
	blob, isBlob := answer.Payload.(command.Blob)
	if !isBlob || string(blob) != string(stored) || answer.Code != 0 || answer.Diagnostic != "" {
		t.Errorf("dataCat = %+v; want Blob of the stored bytes", answer)
	}

	code, stdout, stderr := captureCatStdio(t, func() int { return Run([]string{"--path", path, "cat", "meta/blob"}) })
	if code != 0 || stdout != string(stored) || stderr != "" {
		t.Errorf("ze data cat: code %d stdout %q stderr %q; want the stored bytes", code, stdout, stderr)
	}

	usage := dataCat(commandtest.Args("--path", path))
	if usage.Payload != nil || usage.Code != 1 || usage.Diagnostic != "usage: ze data cat <key>\n" {
		t.Errorf("cat with no key = %+v; want the usage line and code 1", usage)
	}
	missing := dataCat(commandtest.Args("--path", path, "meta/absent"))
	if missing.Payload != nil || missing.Code != 2 || len(missing.Diagnostic) < len("error: ") ||
		missing.Diagnostic[:len("error: ")] != "error: " {
		t.Errorf("cat of a missing key = %+v; want an error line and code 2", missing)
	}
}
