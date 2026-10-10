// Tests for the daemon's confirmed-commit window at boot.
//
// VALIDATES: spec-session-editor-file-mode-parity AC-19: a window the last
// daemon left open is reverted before the config is read, in both config
// source modes, and its record is cleared; a store with no record boots as is.
// METHOD: a store holding the unconfirmed config, a stale candidate and the
// window's record; recoverConfirmWindow then ReadConfigSource, as boot runs.
package hub

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
)

func TestRecoverConfirmWindow(t *testing.T) {
	modes := map[string]storage.ConfigSource{"stored": storage.ConfigSourceStored, "file": storage.ConfigSourceFile}
	for name, mode := range modes {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "router.conf")
			store := storage.BindConfigSource(newTestStore(t, filepath.Dir(path)), path, mode)
			if err := os.WriteFile(path, []byte("confirmed"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := initializeConfigSource(store, path, []byte("confirmed")); err != nil {
				t.Fatal(err)
			}
			if err := publishRecoveredConfig(store, path, []byte("confirmed"), []byte("unconfirmed")); err != nil {
				t.Fatal(err)
			}
			if _, err := storage.WriteCandidateVersion(store, path, []byte("interrupted"), time.Now()); err != nil {
				t.Fatal(err)
			}

			if err := recoverConfirmWindow(store, path); err != nil {
				t.Fatalf("no record: %v", err)
			}
			if got, err := storage.ReadConfigSource(store, path); err != nil || string(got) != "unconfirmed" {
				t.Fatalf("no record must boot as is: %q, %v", got, err)
			}

			recorder := confirm.NewStoreRecorder(store, path)
			if err := recorder.Save(confirm.Pending{User: "alice", Deadline: time.Now(), Rollback: []byte("confirmed")}); err != nil {
				t.Fatal(err)
			}
			if err := recoverConfirmWindow(store, path); err != nil {
				t.Fatal(err)
			}
			if got, err := storage.ReadConfigSource(store, path); err != nil || string(got) != "confirmed" {
				t.Fatalf("boot source = %q, %v; want the rollback", got, err)
			}
			if record, err := recorder.Load(); err != nil || record != nil {
				t.Fatalf("record after revert = %+v, %v; want none", record, err)
			}
		})
	}
}
