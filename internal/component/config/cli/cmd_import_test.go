// VALIDATES: `ze config import` reads a file arg from stdin via "-", and rejects a
// second "-" in one invocation with a non-zero exit (AC-9), never a silent skip.
// PREVENTS: the multi-arg stdin-once guard regressing to a silent empty second read.
package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

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

// TestImportRefusesExistingWithoutConfirmation proves an import onto an
// existing config, with no --yes and no terminal to ask on, is refused and
// leaves the stored config as it was, so a script neither replaces a config
// nobody confirmed nor hangs on a prompt.
//
// The method seeds edge.conf, imports a different config under that name
// without --yes, and reads the active config back. The import asks only when
// os.Stdin is a terminal, and go test may hand the test binary the terminal it
// was started from, so the test points os.Stdin at the null device: the
// refusal it asserts then does not depend on how go test was run.
func TestImportRefusesExistingWithoutConfirmation(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devNull.Close() //nolint:errcheck // test cleanup
	stdin := os.Stdin
	os.Stdin = devNull
	defer func() { os.Stdin = stdin }()

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

// TestImportConfirm proves the replace question names every config it would
// replace, takes y or yes as consent and anything else as a refusal, reports a
// failed read instead of reading it as no, asks nothing without a terminal,
// and speaks of one config in the singular.
//
// The method answers cmdImportConfirm from a reader and checks its verdict,
// the text it must write, and the text that would mean the wrong branch ran:
// a consent never prints the refusal, and a non-interactive refusal never
// prints the question.
func TestImportConfirm(t *testing.T) {
	both := []string{"edge.conf", "core.conf"}
	tests := []struct {
		name        string
		names       []string
		interactive bool
		answers     io.Reader
		want        bool
		mention     string
		absent      string
	}{
		{
			name: "non-interactive refuses and names --yes", names: both, interactive: false,
			answers: strings.NewReader("yes\n"), want: false,
			mention: "configs edge.conf, core.conf already exist; rerun with --yes to replace them", absent: "[y/N]",
		},
		{
			name: "one config is singular", names: []string{"edge.conf"}, interactive: false,
			answers: strings.NewReader(""), want: false,
			mention: "config edge.conf already exists; rerun with --yes to replace it", absent: "configs",
		},
		{
			name: "yes replaces", names: both, interactive: true,
			answers: strings.NewReader("yes\n"), want: true,
			mention: "replace them? [y/N]", absent: "not replaced",
		},
		{
			name: "y replaces", names: both, interactive: true,
			answers: strings.NewReader("Y\n"), want: true,
			mention: "replace them? [y/N]", absent: "not replaced",
		},
		{
			name: "no refuses", names: both, interactive: true,
			answers: strings.NewReader("n\n"), want: false,
			mention: "not replaced", absent: "read answer",
		},
		{
			name: "no answer refuses", names: both, interactive: true,
			answers: strings.NewReader(""), want: false,
			mention: "not replaced", absent: "read answer",
		},
		{
			name: "failed read refuses and says so", names: both, interactive: true,
			answers: iotest.ErrReader(errors.New("terminal gone")), want: false,
			mention: "error: read answer: terminal gone", absent: "not replaced",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prompts bytes.Buffer
			got := cmdImportConfirm(tt.names, tt.interactive, tt.answers, &prompts)
			if got != tt.want {
				t.Fatalf("confirm = %v, want %v; stderr %q", got, tt.want, prompts.String())
			}
			out := prompts.String()
			for _, want := range append([]string{tt.mention}, tt.names...) {
				if !strings.Contains(out, want) {
					t.Fatalf("stderr %q does not mention %q", out, want)
				}
			}
			if strings.Contains(out, tt.absent) {
				t.Fatalf("stderr %q mentions %q, which the other branch writes", out, tt.absent)
			}
		})
	}
}
