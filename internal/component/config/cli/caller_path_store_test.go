// Design: docs/architecture/api/commands.md — path arguments and stdin on the daemon route

package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
)

// storeSecret is a value only the store holds: no answer may carry it.
const storeSecret = "ZE-STORE-SECRET-4f1c9a"

// TestDaemonRouteRefusesTheStoresOwnFiles proves a caller-named path on the
// daemon route never opens one of the configuration store's own files.
//
// Goal: the store's directory is a configuration directory, so CallerPath
// admits a file in it, but the live tree under `database/`, the lock beside
// it and any other entry the store owns hold every stored value (the HTTPS
// private key, the password hashes). A daemon-route caller reads a stored
// value only through `show data cat`, which is edit-authorized; naming the
// store's file to a config reader must not be a second route.
// Method: a live store in the configuration directory holds a secret key;
// every regular file the store owns is named to each daemon-served config
// reader on the daemon route, and each answer must be a refusal that names
// the store and carries no byte of the secret.
func TestDaemonRouteRefusesTheStoresOwnFiles(t *testing.T) {
	file := confinedConfigDir(t, testConfigPlain)
	dir := filepath.Dir(file)
	// env.Get caches, so the store directory is set through env.Set and
	// restored after, as the package's other tests read it too.
	previous := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", previous) })
	if err := env.Set("ze.config.dir", dir); err != nil {
		t.Fatalf("set ze.config.dir: %v", err)
	}

	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := store.WriteKey("meta/web/key", []byte(storeSecret+"\n")); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	var owned []string
	walkErr := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if path == file {
			return nil
		}
		owned = append(owned, path)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", dir, walkErr)
	}
	if len(owned) == 0 {
		t.Fatalf("the store wrote no file under %s", dir)
	}

	for _, path := range []string{
		"show config dump", "show config validation", "show config graph", "show config fmt",
	} {
		t.Run(path, func(t *testing.T) {
			for _, target := range owned {
				answer := answerOn(t, path, []string{target}, true)
				encoded := encodedAnswer(t, answer)
				if strings.Contains(encoded, storeSecret) {
					t.Errorf("%s answered the store's secret from %s:\n%s", path, target, encoded)
				}
				if answer.Code == 0 || answer.Payload != nil {
					t.Errorf("daemon route read the store's file %s: code %d, payload present %t", target, answer.Code, answer.Payload != nil)
					continue
				}
				if !strings.Contains(answer.Diagnostic, "configuration store's own") {
					t.Errorf("refusal of %s does not say it is the store's own file: %q", target, answer.Diagnostic)
				}
			}
		})
	}

	// The operator's own process still reads any file it names.
	local := answerOn(t, "show config validation", []string{owned[0]}, false)
	if strings.Contains(local.Diagnostic, "configuration store's own") {
		t.Errorf("the local route refused the store's file: %q", local.Diagnostic)
	}
}

// TestHistoryOpensTheStoreBesideTheResolvedFile proves `show config history`
// on the daemon route reads the revisions stored beside the file CallerPath
// resolved, not beside the spelling the caller typed.
//
// Goal: CallerPath confines the file a symbolic link resolves to, so a link
// placed anywhere (a world-writable directory included) that points into the
// configuration directory is admitted. The store opened must be the one
// beside that resolved file: opening the store beside the link opens a store
// in a directory the confinement never judged.
// Method: outside/x.conf links to a/x.conf inside the configuration directory.
// Only a/ holds a store, with one revision of x.conf; the daemon-route answer
// for the link must list that revision.
func TestHistoryOpensTheStoreBesideTheResolvedFile(t *testing.T) {
	file := confinedConfigDir(t, testConfigPlain)
	nested := filepath.Join(filepath.Dir(file), "a")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(nested, "x.conf")
	if err := os.WriteFile(target, []byte(testConfigPlain), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "x.conf")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Create(nested)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := store.WriteVersion(target, []byte("old config"), time.Now()); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	answer := answerOn(t, "show config history", []string{link}, true)
	if answer.Code != 0 {
		t.Fatalf("history of %s on the daemon route refused: code %d, %q", link, answer.Code, answer.Diagnostic)
	}
	encoded := encodedAnswer(t, answer)
	if !strings.Contains(encoded, `"revision":1`) {
		t.Errorf("history of %s did not list the revision stored beside %s:\n%s", link, target, encoded)
	}
}
