// Design: docs/architecture/api/commands.md -- stored configuration rollback.

package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestConfigRollbackRootUpdatesTheAuthoritativeVersion proves the stored
// adapter changes the active pointer, retains its former content in history
// and leaves another configuration unchanged.
func TestConfigRollbackRootUpdatesTheAuthoritativeVersion(t *testing.T) {
	store := newImportBlobStore(t)
	const name = "router.conf"
	const current = validConfig
	previous := strings.Replace(current, "203.0.113.253", "203.0.113.254", 1)
	if err := store.WriteFile(name, []byte(current), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.EnsureActiveVersion(store, name, []byte(current), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteVersion(name, []byte(previous), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile("other.conf", []byte("unrelated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	versions, err := store.ListVersions(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("positive control has %d versions, want two", len(versions))
	}
	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdRollbackWithStorage(store, []string{"2", name})
	})
	if code != exitOK {
		t.Fatalf("stored rollback refused: %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Rolled back to revision 2") {
		t.Fatalf("successful rollback lost its result: %s", stdout)
	}
	active, err := storage.ReadActiveConfig(store, name)
	if err != nil {
		t.Fatal(err)
	}
	if string(active) != previous {
		t.Fatalf("active pointer still resolves %q, want %q", active, previous)
	}
	mirror, err := store.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if string(mirror) != previous {
		t.Fatalf("mirror = %q, want %q", mirror, previous)
	}
	rollback, err := store.ReadKey(zefs.KeyConfigRollback.Key(name))
	if err != nil {
		t.Fatal(err)
	}
	if string(rollback) != versions[0].Stamp+"\n" {
		t.Fatalf("previous active rollback = %q, want %q", rollback, versions[0].Stamp+"\n")
	}
	unrelated, err := store.ReadFile("other.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(unrelated) != "unrelated\n" {
		t.Fatalf("unrelated configuration changed: %q", unrelated)
	}
}

// TestConfigRollbackRootRoutesBusyStoreAndChecksIdentity proves the selected
// store's owner receives the exact indexed operation and only its complete,
// matching acknowledgment can report success.
func TestConfigRollbackRootRoutesBusyStoreAndChecksIdentity(t *testing.T) {
	_, folder := busyStore(t)
	for _, test := range []struct {
		name string
		body string
		code int
	}{
		{name: "matching", body: `{"name":"router.conf","revision":2,"source-version":"old","version":"new","source-time":"2026-11-01T12:34:56Z"}`, code: exitOK},
		{name: "wrong name", body: `{"name":"other.conf","revision":2,"source-version":"old","version":"new","source-time":"2026-11-01T12:34:56Z"}`, code: exitError},
		{name: "wrong revision", body: `{"name":"router.conf","revision":1,"source-version":"old","version":"new","source-time":"2026-11-01T12:34:56Z"}`, code: exitError},
		{name: "missing publication", body: `{"name":"router.conf","revision":2,"source-version":"old","source-time":"2026-11-01T12:34:56Z"}`, code: exitError},
		{name: "missing source", body: `{"name":"router.conf","revision":2,"version":"new","source-time":"2026-11-01T12:34:56Z"}`, code: exitError},
		{name: "missing time", body: `{"name":"router.conf","revision":2,"source-version":"old","version":"new"}`, code: exitError},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			ask := func(gotFolder, line string) ([]byte, error) {
				calls++
				if gotFolder != folder {
					t.Fatalf("wrong daemon folder: %q, want %q", gotFolder, folder)
				}
				if line != "request data config rollback name router.conf revision 2" {
					t.Fatalf("wrong indexed operation: %s", line)
				}
				return []byte(test.body), nil
			}
			code, stdout, stderr := captureBlobStdio(t, func() int {
				return cmdRollbackThrough(nil, []string{"2", filepath.Join(folder, "router.conf")}, ask)
			})
			if calls != 1 {
				t.Fatalf("daemon calls = %d, want one: %s", calls, stderr)
			}
			if code != test.code {
				t.Fatalf("code = %d, want %d: %s", code, test.code, stderr)
			}
			if test.code == exitOK {
				if !strings.Contains(stdout, "Rolled back to revision 2 (2026-11-01 12:34:56)") {
					t.Fatalf("matching owner result lost the selected source time: %s", stdout)
				}
			} else if stdout != "" {
				t.Fatalf("unconfirmed owner answer reported success: %s", stdout)
			}
		})
	}
}

// TestConfigRollbackRootPreservesStoppedWindowRemedy proves a stored rollback
// keeps the recorded window's operator and exact restart remedy. Method: leave
// an unconfirmed record, invoke the stored root adapter and inspect its refusal.
func TestConfigRollbackRootPreservesStoppedWindowRemedy(t *testing.T) {
	store := newImportBlobStore(t)
	const name = "router.conf"
	if err := store.WriteFile(name, []byte(validConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.EnsureActiveVersion(store, name, []byte(validConfig), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteVersion(name, []byte(validConfig), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	recorder := confirm.NewStoreRecorder(store, name)
	if err := recorder.Save(confirm.Pending{User: "alice", Deadline: time.Now().Add(time.Minute), Rollback: []byte(validConfig)}); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := captureBlobStdio(t, func() int {
		return cmdRollbackWithStorage(store, []string{"2", name})
	})
	if code != exitError {
		t.Fatalf("rollback code = %d, want %d: %s", code, exitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("refused rollback reported success: %s", stdout)
	}
	if !strings.Contains(stderr, "alice") {
		t.Fatalf("refusal lost the window owner: %s", stderr)
	}
	if !strings.Contains(stderr, "when the daemon starts") {
		t.Fatalf("refusal lost the pending recovery explanation: %s", stderr)
	}
	if !strings.HasSuffix(stderr, "\nfix: ze start "+name+"\n") {
		t.Fatalf("refusal lost the exact command that clears the window: %s", stderr)
	}
	active, err := storage.ReadActiveConfig(store, name)
	if err != nil {
		t.Fatal(err)
	}
	if string(active) != validConfig {
		t.Fatalf("refused rollback changed the active configuration: %s", active)
	}
}
