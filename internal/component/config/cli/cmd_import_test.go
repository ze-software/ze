// VALIDATES: `ze config import` reads a file arg from stdin via "-", and rejects a
// second "-" in one invocation with a non-zero exit (AC-9), never a silent skip.
// PREVENTS: the multi-arg stdin-once guard regressing to a silent empty second read.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
)

func newImportBlobStore(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
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

// writeImportInput writes one loose input file and returns its path.
func writeImportInput(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.conf")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	return path
}

// VALIDATES: an import onto an existing config, with no --yes and no terminal
// to ask on, is refused and leaves the stored config as it was.
// PREVENTS: a script replacing a config nobody confirmed, or hanging on a prompt.
func TestImportRefusesExistingWithoutConfirmation(t *testing.T) {
	store := newImportBlobStore(t)
	if _, err := storage.RestoreConfig(store, "edge.conf", []byte(showTestConfig)); err != nil {
		t.Fatalf("seed edge.conf: %v", err)
	}
	path := writeImportInput(t, "system { host replaced }\n")

	if rc := cmdImportWithStorage(store, []string{"--name", "edge.conf", path}); rc != exitError {
		t.Fatalf("import over edge.conf without --yes exit = %d, want %d", rc, exitError)
	}
	got, err := storage.ReadActiveConfig(store, "edge.conf")
	if err != nil {
		t.Fatalf("read edge.conf: %v", err)
	}
	if string(got) != showTestConfig {
		t.Fatalf("edge.conf changed without confirmation: %q", got)
	}
}

// VALIDATES: --yes replaces an existing config as a new active version, so
// the config the daemon reads is the imported one, even when the old config
// already had committed versions.
// PREVENTS: a replace that writes only the file/active mirror, which an
// existing active pointer shadows, so the import reports success and changes
// nothing the daemon reads.
func TestImportReplacesExistingWithYes(t *testing.T) {
	store := newImportBlobStore(t)
	if _, err := storage.RestoreConfig(store, "edge.conf", []byte(showTestConfig)); err != nil {
		t.Fatalf("seed edge.conf: %v", err)
	}
	replaced := "system { host replaced }\n"
	path := writeImportInput(t, replaced)

	if rc := cmdImportWithStorage(store, []string{"--yes", "--name", "edge.conf", path}); rc != exitOK {
		t.Fatalf("import --yes over edge.conf exit = %d, want %d", rc, exitOK)
	}
	got, err := storage.ReadActiveConfig(store, "edge.conf")
	if err != nil {
		t.Fatalf("read edge.conf: %v", err)
	}
	if string(got) != replaced {
		t.Fatalf("active edge.conf = %q, want the imported %q", got, replaced)
	}
}

// VALIDATES: the replace question names every config it would replace, takes
// y or yes as consent and anything else, including no answer, as a refusal;
// without a terminal it asks nothing and names --yes.
// PREVENTS: a prompt that hides what it replaces, or a failed read read as consent.
func TestConfirmImportReplace(t *testing.T) {
	tests := []struct {
		name        string
		interactive bool
		answer      string
		want        bool
		mention     string
	}{
		{name: "non-interactive refuses and names --yes", interactive: false, answer: "yes\n", want: false, mention: "--yes"},
		{name: "yes replaces", interactive: true, answer: "yes\n", want: true, mention: "replace"},
		{name: "y replaces", interactive: true, answer: "y\n", want: true, mention: "replace"},
		{name: "no refuses", interactive: true, answer: "n\n", want: false, mention: "not replaced"},
		{name: "no answer refuses", interactive: true, answer: "", want: false, mention: "not replaced"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errBuf bytes.Buffer
			got := confirmImportReplace([]string{"edge.conf", "core.conf"}, tt.interactive, strings.NewReader(tt.answer), &errBuf)
			if got != tt.want {
				t.Fatalf("confirm = %v, want %v; stderr %q", got, tt.want, errBuf.String())
			}
			out := errBuf.String()
			for _, want := range []string{"edge.conf", "core.conf", tt.mention} {
				if !strings.Contains(out, want) {
					t.Fatalf("stderr %q does not mention %q", out, want)
				}
			}
		})
	}
}
