package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	_ "github.com/ze-software/ze/internal/component/config/storage/yang" // the update data config model ConfigLine judges by
	_ "github.com/ze-software/ze/internal/component/config/yang"         // the loader that builds the command model
)

// importBytesMax is the largest content `update data config` carries, the
// bound ze-data-cmd.yang declares on its content leaf (261,120 base64
// characters).
const importBytesMax = 195840

// importEdgeName is the config the daemon-route tests replace.
const importEdgeName = "edge.conf"

// importInput writes data to a file named name in a temporary folder and
// answers its path.
func importInput(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// stdinFromNull points os.Stdin at the null device for the test, so the
// confirmation never finds a terminal, however go test was started.
func stdinFromNull(t *testing.T) {
	t.Helper()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdin
	os.Stdin = devNull
	t.Cleanup(func() {
		os.Stdin = saved
		_ = devNull.Close()
	})
}

// busyStore creates a store in a temporary folder and holds it as its owner
// for the test, as a running daemon does, so `ze config import --dir <folder>`
// meets storage.ErrBusy. It answers the owner and the folder.
func busyStore(t *testing.T) (storage.Storage, string) {
	t.Helper()
	folder := t.TempDir()
	owner, err := storage.Create(folder)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	return owner, folder
}

// configDaemonStub answers `update data config` lines as the daemon's handler
// does for a config that is not its own, committing the decoded content as a
// new active version through the owner's handle, and records every line it
// was sent. Its answer carries the name and the byte count, the two fields the
// local route reads. A name in refuse is refused, as a daemon refuses a
// config.
func configDaemonStub(t *testing.T, owner storage.Storage, refuse string, sent *[]string) daemonroute.Ask {
	t.Helper()
	return func(_, line string) ([]byte, error) {
		*sent = append(*sent, line)
		rest, found := strings.CutPrefix(line, "update data config name ")
		if !found {
			return nil, &daemonroute.Refusal{Message: "unexpected command"}
		}
		name, encoded, found := strings.Cut(rest, " content ")
		if !found {
			return nil, &daemonroute.Refusal{Message: "no content"}
		}
		if name == refuse {
			return nil, &daemonroute.Refusal{Message: "update data config: " + name + " was not imported\nfix: ze config validate " + name}
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, &daemonroute.Refusal{Message: "content is not base64"}
		}
		if _, err := storage.RestoreConfig(owner, name, data); err != nil {
			return nil, &daemonroute.Refusal{Message: err.Error()}
		}
		return json.Marshal(map[string]any{"name": name, "bytes": len(data)})
	}
}

// doneLine is the line both routes print for one landed config.
func doneLine(verb, name string, octets int) string {
	return verb + " " + name + " (" + strconv.Itoa(octets) + " bytes)\n"
}

// activeEdge answers the active config of importEdgeName in store.
func activeEdge(t *testing.T, store storage.Storage) string {
	t.Helper()
	data, err := storage.ReadActiveConfig(store, importEdgeName)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// VALIDATES: a config the store holds only through its active pointer (a
// version, no file key) counts as existing, so the import replaces it as a
// new active version and prints `replaced`.
// PREVENTS: a fresh write beside the pointer, which the pointer shadows, so
// the daemon goes on reading the old config while the import said imported.
func TestImportExistingByActivePointerIsReplaced(t *testing.T) {
	store := newImportBlobStore(t)
	if _, _, err := storage.EnsureActiveVersion(store, importEdgeName, []byte("old config\n"), time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if store.Exists(importEdgeName) {
		t.Fatal("the seed holds a file key; this test needs an active pointer alone")
	}
	replaced := []byte("system { host replaced }\n")
	path := importInput(t, importEdgeName, replaced)

	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdImportWithStorage(store, []string{"--yes", path})
	})
	if code != exitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, exitOK, stderr)
	}
	if want := doneLine("replaced", importEdgeName, len(replaced)) + "1 file(s) imported\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if got := activeEdge(t, store); got != string(replaced) {
		t.Fatalf("active %s = %q, want the imported %q", importEdgeName, got, replaced)
	}
}

// VALIDATES: AC-16. While a daemon owns the destination store, ze config
// import --yes sends one `update data config name <name> content <base64>`
// per input, in input order, and prints the offline route's lines: replaced
// for a config the store holds, imported for a new one, then the file count.
// PREVENTS: a busy store answered with the bare ErrBusy refusal while its
// daemon could import, or a daemon import printed differently.
func TestImportThroughDaemonWhenBusy(t *testing.T) {
	owner, folder := busyStore(t)
	if _, err := storage.RestoreConfig(owner, importEdgeName, []byte("old config\n")); err != nil {
		t.Fatal(err)
	}
	replaced := []byte("system { host replaced }\n")
	fresh := []byte("system { host fresh }\n")
	files := []string{importInput(t, importEdgeName, replaced), importInput(t, "fresh.conf", fresh)}
	var sent []string
	ask := configDaemonStub(t, owner, "", &sent)

	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdImportThrough(nil, append([]string{"--dir", folder, "--yes"}, files...), ask)
	})
	if code != exitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, exitOK, stderr)
	}
	if len(sent) != 2 {
		t.Fatalf("sent %d commands, want one per input: %q", len(sent), sent)
	}
	if want := "update data config name " + importEdgeName + " content " + base64.StdEncoding.EncodeToString(replaced); sent[0] != want {
		t.Fatalf("sent %q, want %q", sent[0], want)
	}
	want := doneLine("replaced", importEdgeName, len(replaced)) + doneLine("imported", "fresh.conf", len(fresh)) + "2 file(s) imported\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if got := activeEdge(t, owner); got != string(replaced) {
		t.Fatalf("active %s = %q, want %q", importEdgeName, got, replaced)
	}
}

// VALIDATES: AC-16. Replacing an existing config through the daemon still
// asks first: with no --yes and no terminal the import is refused naming
// --yes, and nothing is sent.
// PREVENTS: a daemon route that replaces a config nobody confirmed.
func TestImportThroughDaemonAsksBeforeSending(t *testing.T) {
	stdinFromNull(t)
	owner, folder := busyStore(t)
	if _, err := storage.RestoreConfig(owner, importEdgeName, []byte("old config\n")); err != nil {
		t.Fatal(err)
	}
	var sent []string
	ask := configDaemonStub(t, owner, "", &sent)
	path := importInput(t, importEdgeName, []byte("system { host replaced }\n"))

	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdImportThrough(nil, []string{"--dir", folder, path}, ask)
	})
	if code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	if len(sent) != 0 {
		t.Fatalf("sent %q before the replace was confirmed", sent)
	}
	if !strings.Contains(stderr, "rerun with --yes to replace it") {
		t.Fatalf("stderr does not name --yes:\n%s", stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing", stdout)
	}
	if got := activeEdge(t, owner); got != "old config\n" {
		t.Fatalf("active %s changed without confirmation: %q", importEdgeName, got)
	}
}

// VALIDATES: AC-16. A daemon refusal of one input is printed with its reason
// and fix and ends the import, as the first failed write ends the offline
// one: the inputs before it stay imported, no count line is printed.
// PREVENTS: a refusal reported as an import, or one that hides the daemon's
// reason.
func TestImportThroughDaemonRefusalStopsTheImport(t *testing.T) {
	owner, folder := busyStore(t)
	var sent []string
	ask := configDaemonStub(t, owner, "b.conf", &sent)
	first := []byte("system { host a }\n")
	files := []string{importInput(t, "a.conf", first), importInput(t, "b.conf", []byte("x\n")), importInput(t, "c.conf", []byte("y\n"))}

	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdImportThrough(nil, append([]string{"--dir", folder}, files...), ask)
	})
	if code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	if len(sent) != 2 {
		t.Fatalf("sent %d commands, want 2 (the refusal ends the import): %q", len(sent), sent)
	}
	if want := doneLine("imported", "a.conf", len(first)); stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	for _, want := range []string{"error: import b.conf: update data config: b.conf was not imported", "fix: ze config validate b.conf"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	if owner.Exists("c.conf") {
		t.Fatal("an input after the refusal was imported")
	}
}

// VALIDATES: AC-12 on the AC-16 route. An input over the bound one command
// carries stops the import before anything is sent, even when an earlier
// input fits, with the daemon's own bound refusal; an input at the bound is
// sent.
// PREVENTS: an oversize import that connects and fails as "packet too large",
// or one that imports the inputs before it and then fails.
func TestImportThroughDaemonOversizeSendsNothing(t *testing.T) {
	owner, folder := busyStore(t)
	var sent []string
	ask := configDaemonStub(t, owner, "", &sent)
	small := importInput(t, "a.conf", []byte("system { host a }\n"))
	over := importInput(t, "b.conf", bytes.Repeat([]byte{'y'}, importBytesMax+1))

	code, _, stderr := captureBlobStdio(t, func() int {
		return cmdImportThrough(nil, []string{"--dir", folder, small, over}, ask)
	})
	if code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	if len(sent) != 0 {
		t.Fatalf("an import holding an input over the bound sent %d commands, want none", len(sent))
	}
	for _, want := range []string{"error: import b.conf: ", "195,840 bytes", "request data restore path <file> config"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("refusal lacks %q:\n%s", want, stderr)
		}
	}
	if owner.Exists("a.conf") {
		t.Fatal("the input before the oversize one was imported")
	}

	atBound := importInput(t, "c.conf", bytes.Repeat([]byte{'z'}, importBytesMax))
	code, _, stderr = captureBlobStdio(t, func() int {
		return cmdImportThrough(nil, []string{"--dir", folder, atBound}, ask)
	})
	if code != exitOK {
		t.Fatalf("an input at the bound: exit = %d, want %d; stderr:\n%s", code, exitOK, stderr)
	}
	if len(sent) != 1 {
		t.Fatalf("an input at the bound sent %d commands, want 1", len(sent))
	}
}

// VALIDATES: AC-16 survivor. A busy store whose daemon does not answer keeps
// the store's refusal and says why the daemon did not answer, with its fix.
// PREVENTS: a busy store with no daemon read as an import.
func TestImportThroughDaemonUnansweredNamesTheOwner(t *testing.T) {
	_, folder := busyStore(t)
	ask := func(_, _ string) ([]byte, error) {
		return nil, &daemonroute.Unanswered{
			Why:   "no daemon listens at 127.0.0.1:2222",
			Fix:   daemonroute.FixDaemonStatus,
			Cause: errors.New("connection refused"),
		}
	}
	path := importInput(t, "a.conf", []byte("system { host a }\n"))

	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdImportThrough(nil, []string{"--dir", folder, path}, ask)
	})
	if code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing", stdout)
	}
	for _, want := range []string{"error: ze config import: ", storage.ErrBusy.Error(), "no daemon listens at 127.0.0.1:2222", "fix: " + daemonroute.FixDaemonStatus} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("refusal lacks %q:\n%s", want, stderr)
		}
	}
}

// VALIDATES: AC-16 survivor. --dir naming a store no daemon owns stays on the
// offline route: the daemon is never asked and the store holds the config.
// PREVENTS: an import sent to a daemon of another store.
func TestImportOnAFreeStoreStaysOffline(t *testing.T) {
	folder := t.TempDir()
	seed, err := storage.Create(folder)
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	ask := func(_, line string) ([]byte, error) { //nolint:unparam // daemonroute.Ask fixes the results; this stub only ever refuses.
		t.Errorf("asked a daemon %q for a store no daemon owns", line)
		return nil, &daemonroute.Refusal{Message: "unexpected"}
	}
	data := []byte("system { host a }\n")
	path := importInput(t, "a.conf", data)

	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdImportThrough(nil, []string{"--dir", folder, path}, ask)
	})
	if code != exitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, exitOK, stderr)
	}
	if want := doneLine("imported", "a.conf", len(data)) + "1 file(s) imported\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	store, err := storage.OpenReadOnly(folder, folder)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	stored, err := store.ReadFile("a.conf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, data) {
		t.Fatalf("a.conf holds %q, want %q", stored, data)
	}
}

// VALIDATES: with the store free and the record of a confirmed-commit window
// a stopped daemon left for the config, ze config import --yes of that config
// is refused with the window's own refusal and the active config is
// unchanged.
// PREVENTS: an offline import the daemon's next start reverts with the
// window, so the operator's import is silently undone.
func TestImportRefusedWhileStoppedWindowRecorded(t *testing.T) {
	store := newImportBlobStore(t)
	if _, err := storage.RestoreConfig(store, importEdgeName, []byte("old config\n")); err != nil {
		t.Fatal(err)
	}
	pending := confirm.Pending{User: "alice", Deadline: time.Now().Add(time.Minute), Rollback: []byte("older config\n")}
	if err := confirm.NewStoreRecorder(store, importEdgeName).Save(pending); err != nil {
		t.Fatal(err)
	}
	path := importInput(t, importEdgeName, []byte("system { host replaced }\n"))

	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdImportWithStorage(store, []string{"--yes", path})
	})
	if code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	want := (&confirm.StoppedWindowError{Owner: "alice", ConfigPath: importEdgeName}).Error()
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr lacks the window's refusal %q:\n%s", want, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing", stdout)
	}
	if got := activeEdge(t, store); got != "old config\n" {
		t.Fatalf("active %s = %q after the refused import, want the old config", importEdgeName, got)
	}
}
