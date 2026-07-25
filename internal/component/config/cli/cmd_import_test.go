// VALIDATES: `ze config import` reads a file arg from stdin via "-", and rejects a
// second "-" in one invocation with a non-zero exit (AC-9), never a silent skip.
// PREVENTS: the multi-arg stdin-once guard regressing to a silent empty second read.
package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
)

func newImportBlobStore(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestImportSingleStdin verifies `ze config import --name k -` reads the config
// from stdin (AC-3): a single "-" is accepted and stored with the piped bytes.
func TestImportSingleStdin(t *testing.T) {
	store := newImportBlobStore(t)
	restore := cliio.SwapStreams(strings.NewReader(showTestConfig), &bytes.Buffer{})
	defer restore()

	rc := cmdImportWithStorage(store, []string{"--name", "piped.conf", "-"})
	if rc != exitOK {
		t.Fatalf("import --name piped.conf - exit = %d, want %d", rc, exitOK)
	}
	if !store.Exists("piped.conf") {
		t.Fatal("stdin import did not store the config under the given name")
	}
	stored, err := store.ReadFile("piped.conf")
	if err != nil {
		t.Fatalf("read back stored config: %v", err)
	}
	if string(stored) != showTestConfig {
		t.Fatalf("stored config != piped stdin\nstored:\n%s", stored)
	}
}

// TestImportDoubleStdin verifies `ze config import <file> - -` exits non-zero:
// the second "-" hits the stdin-once guard and fails closed (AC-9).
func TestImportDoubleStdin(t *testing.T) {
	store := newImportBlobStore(t)
	aConf := writeTestConfig(t, showTestConfig)

	restore := cliio.SwapStreams(strings.NewReader(showTestConfig), &bytes.Buffer{})
	defer restore()

	rc := cmdImportWithStorage(store, []string{aConf, "-", "-"})
	if rc == exitOK {
		t.Fatal("import a.conf - - exited 0; want non-zero for the double-stdin conflict (AC-9)")
	}
}
