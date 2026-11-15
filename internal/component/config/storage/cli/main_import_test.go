package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/pkg/zefs"
)

// importTestName is the config the import tests replace; it is the base name
// of the file each test imports.
const importTestName = "router.conf"

// importTestFile writes data to a file named name in a temporary folder and
// answers its path, the file ze data import is given.
func importTestFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// importedLine is the line both routes print for one imported file.
func importedLine(name string, octets int) string {
	return "imported " + zefs.KeyFileActive.Key(name) + " (" + strconv.Itoa(octets) + " bytes)\n"
}

// readActive answers the active config of name, failing the test when it does
// not read.
func readActive(t *testing.T, store storage.Storage, name string) string {
	t.Helper()
	active, err := storage.ReadActiveConfig(store, name)
	if err != nil {
		t.Fatal(err)
	}
	return string(active)
}

// VALIDATES: AC-9. With the store free, ze data import of a config that
// already has an active version commits a new active version, so the active
// config reads the imported bytes and the previous version is kept as
// rollback, and prints the local route's lines. A config with no active
// version is written as its file/active key, as before.
// PREVENTS: the raw write of file/active/<name> that the active pointer
// shadows, so a daemon started after the import ran the old config while the
// import reported success.
func TestDataImportReplacesThroughAVersion(t *testing.T) {
	dir := t.TempDir()
	seed, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.EnsureActiveVersion(seed, filepath.Join(dir, importTestName), []byte("old config"), time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, storage.TreeName)
	replaced := []byte("new config")
	fresh := []byte("fresh config")
	files := []string{importTestFile(t, importTestName, replaced), importTestFile(t, "fresh.conf", fresh)}

	var stdout, stderr bytes.Buffer
	if code := importConfigs(path, files, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("import returned %d, want 0; stderr:\n%s", code, stderr.String())
	}
	want := importedLine(importTestName, len(replaced)) + importedLine("fresh.conf", len(fresh)) +
		"2 files imported into " + path + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}

	store, err := storage.OpenTree(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	if got := readActive(t, store, importTestName); got != string(replaced) {
		t.Fatalf("the active config reads %q after the import, want %q", got, replaced)
	}
	if _, err := store.ReadKey(zefs.KeyConfigRollback.Key(importTestName)); err != nil {
		t.Fatalf("the previous version is not kept as rollback: %v", err)
	}
	stored, err := store.ReadKey(zefs.KeyFileActive.Key("fresh.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, fresh) {
		t.Fatalf("the new config holds %q, want %q", stored, fresh)
	}
	if _, err := store.ReadKey(zefs.KeyConfigActive.Key("fresh.conf")); err == nil {
		t.Fatal("a config with no active version gained one: it is written as today")
	}
}

// VALIDATES: with the store free and the record of a confirmed-commit window a
// stopped daemon left for the config, ze data import of that config is
// refused with the window's own refusal (who opened it, and `ze start
// <config>`), exits 2, and leaves the active config as it was.
// PREVENTS: an offline import the daemon's next start reverts with the
// window, so the operator's import is silently undone.
func TestDataImportRefusedWhileStoppedWindowRecorded(t *testing.T) {
	dir := t.TempDir()
	seed, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.EnsureActiveVersion(seed, filepath.Join(dir, importTestName), []byte("old config"), time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	pending := confirm.Pending{User: "alice", Deadline: time.Now().Add(time.Minute), Rollback: []byte("older config")}
	if err := confirm.NewStoreRecorder(seed, importTestName).Save(pending); err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, storage.TreeName)

	var stdout, stderr bytes.Buffer
	if code := importConfigs(path, []string{importTestFile(t, importTestName, []byte("new config"))}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("import over a recorded window returned %d, want 2; stderr:\n%s", code, stderr.String())
	}
	want := (&confirm.StoppedWindowError{Owner: "alice", ConfigPath: importTestName}).Error()
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr lacks the window's refusal %q:\n%s", want, stderr.String())
	}
	if want := "0 files imported into " + path + "\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}

	store, err := storage.OpenTree(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	if got := readActive(t, store, importTestName); got != "old config" {
		t.Fatalf("the active config reads %q after the refused import, want the old config", got)
	}
}

// daemonImportStub answers update data config lines as the daemon's handler
// does for a name that is not its own, committing the decoded content as a new
// version through the owner's handle, and records every line it was sent. A
// name in refuse is refused, as a daemon refuses a config.
func daemonImportStub(t *testing.T, owner storage.Storage, refuse string, sent *[]string) daemonroute.Ask {
	t.Helper()
	return func(_, line string) ([]byte, error) {
		*sent = append(*sent, line)
		rest, found := strings.CutPrefix(line, pathUpdateDataConfig+" "+keywordName+" ")
		if !found {
			return nil, &daemonroute.Refusal{Message: "unexpected command"}
		}
		name, encoded, found := strings.Cut(rest, " "+keywordContent+" ")
		if !found {
			return nil, &daemonroute.Refusal{Message: "no content"}
		}
		if name == refuse {
			return nil, &daemonroute.Refusal{Message: "update data config: " + name + " was not imported"}
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, &daemonroute.Refusal{Message: "update data config: content is not base64"}
		}
		stamp, err := storage.RestoreConfig(owner, name, data)
		if err != nil {
			return nil, &daemonroute.Refusal{Message: err.Error()}
		}
		return json.Marshal(map[string]any{keywordName: name, answerVersion: stamp, keywordBytes: len(data)})
	}
}

// VALIDATES: AC-10, local half, and the Known Limitation of one command per
// file. While another process owns the store, ze data import sends each file
// to the daemon as `update data config name <name> content <base64>`, one
// command per file, and prints the local route's lines. A refusal of the
// third file is an error line carrying the daemon's reason and leaves the
// first two imported; the exit code is 0 because two were imported.
// PREVENTS: a busy store answered with the bare refusal while the daemon could
// import, a daemon import printed differently from a local one, or a refusal
// that undoes or stops the files before it.
func TestDataImportForwardsToDaemonWhenBusy(t *testing.T) {
	owner, path := ownedStore(t)
	var sent []string
	ask := daemonImportStub(t, owner, "third.conf", &sent)
	texts := map[string][]byte{"first.conf": []byte("first config"), "second.conf": {0x00, 0xff, '\n'}, "third.conf": []byte("third")}
	files := []string{
		importTestFile(t, "first.conf", texts["first.conf"]),
		importTestFile(t, "second.conf", texts["second.conf"]),
		importTestFile(t, "third.conf", texts["third.conf"]),
	}

	var stdout, stderr bytes.Buffer
	if code := importConfigs(path, files, ask, &stdout, &stderr); code != 0 {
		t.Fatalf("busy import returned %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if len(sent) != 3 {
		t.Fatalf("sent %d commands, want one per file: %q", len(sent), sent)
	}
	wantFirst := pathUpdateDataConfig + " " + keywordName + " first.conf " + keywordContent + " " +
		base64.StdEncoding.EncodeToString(texts["first.conf"])
	if sent[0] != wantFirst {
		t.Fatalf("sent %q, want %q", sent[0], wantFirst)
	}
	want := importedLine("first.conf", len(texts["first.conf"])) + importedLine("second.conf", len(texts["second.conf"])) +
		"2 files imported into " + path + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if !strings.Contains(stderr.String(), "error: update data config: third.conf was not imported") {
		t.Fatalf("the refusal of the third file is not reported:\n%s", stderr.String())
	}
	for _, name := range []string{"first.conf", "second.conf"} {
		if got := readActive(t, owner, name); got != string(texts[name]) {
			t.Fatalf("%s reads %q, want %q", name, got, texts[name])
		}
	}
	if owner.Exists("third.conf") {
		t.Fatal("the refused file was imported")
	}
}

// VALIDATES: AC-12, local half of the import. A busy ze data import of a file
// of 195,840 bytes is sent; one byte more is refused before the daemon is
// asked, with the daemon's own bound refusal (the bound in bytes and the
// restore route for a larger config), and the content is not in the refusal.
// PREVENTS: an oversize import that connects and fails as "packet too large",
// read as a daemon that did not answer.
func TestDataImportRefusesOversizeBeforeConnecting(t *testing.T) {
	owner, path := ownedStore(t)
	var sent []string
	ask := daemonImportStub(t, owner, "", &sent)

	var stdout, stderr bytes.Buffer
	atBound := bytes.Repeat([]byte{'z'}, updateContentBytesMax)
	if code := importConfigs(path, []string{importTestFile(t, importTestName, atBound)}, ask, &stdout, &stderr); code != 0 {
		t.Fatalf("an import at the bound returned %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if len(sent) != 1 {
		t.Fatalf("an import at the bound asked the daemon %d times, want once", len(sent))
	}

	sent = nil
	stdout.Reset()
	stderr.Reset()
	over := bytes.Repeat([]byte{'y'}, updateContentBytesMax+1)
	if code := importConfigs(path, []string{importTestFile(t, importTestName, over)}, ask, &stdout, &stderr); code != 2 {
		t.Fatalf("an import over the bound returned %d, want 2", code)
	}
	if len(sent) != 0 {
		t.Fatalf("an import over the bound asked the daemon %d times, want none", len(sent))
	}
	text := stderr.String()
	for _, want := range []string{"error: ze data import: ", "195,840 bytes", "request data restore path <file> config", redact.Placeholder} {
		if !strings.Contains(text, want) {
			t.Fatalf("refusal lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, base64.StdEncoding.EncodeToString(over[:48])) {
		t.Fatalf("refusal holds the content:\n%s", text)
	}
	if got := readActive(t, owner, importTestName); got != string(atBound) {
		t.Fatal("the refused import changed the config")
	}
}

// VALIDATES: ze data import refuses stdin ("-") on both routes, before it
// reads anything, and names `ze config import --name <name> -`; the files
// beside it are still imported.
// PREVENTS: the offline route committing stdin as a config named "-", which
// no daemon loads and which `update data config` refuses, so the two routes
// disagreed about the same input.
func TestDataImportRefusesStdinOnBothRoutes(t *testing.T) {
	const wantFix = "ze config import --name <name> -"

	// With the store free: nothing named "-" is written, the file is imported.
	dir := t.TempDir()
	seed, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(dir, storage.TreeName)
	var stdout, stderr bytes.Buffer
	file := importTestFile(t, importTestName, []byte("new config"))
	unasked := func(_, _ string) ([]byte, error) {
		t.Fatal("the daemon was asked while the store was free")
		return nil, nil
	}
	if code := importConfigs(tree, []string{"-", file}, unasked, &stdout, &stderr); code != 0 {
		t.Fatalf("an import of stdin and one file returned %d, want 0; stderr:\n%s", code, stderr.String())
	}
	for _, want := range []string{"error: ze data import: \"-\" was not imported", wantFix} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("offline refusal lacks %q:\n%s", want, stderr.String())
		}
	}
	if stdout.String() != importedLine(importTestName, len("new config"))+"1 files imported into "+tree+"\n" {
		t.Fatalf("offline stdout:\n%s", stdout.String())
	}
	reader, err := storage.OpenTree(tree, false)
	if err != nil {
		t.Fatal(err)
	}
	if reader.Exists(zefs.KeyFileActive.Key("-")) {
		t.Fatal("stdin was committed as a config named \"-\"")
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}

	// While a daemon owns the store: the daemon is not asked for stdin.
	owner, path := ownedStore(t)
	var sent []string
	ask := daemonImportStub(t, owner, "", &sent)
	stdout.Reset()
	stderr.Reset()
	if code := importConfigs(path, []string{"-"}, ask, &stdout, &stderr); code != 2 {
		t.Fatalf("an import of stdin alone beside a daemon returned %d, want 2", code)
	}
	if len(sent) != 0 {
		t.Fatalf("the daemon was asked %d times for stdin, want none", len(sent))
	}
	if !strings.Contains(stderr.String(), wantFix) {
		t.Fatalf("daemon-route refusal lacks %q:\n%s", wantFix, stderr.String())
	}
}
