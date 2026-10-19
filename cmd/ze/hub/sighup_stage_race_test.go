// Design: docs/guide/config-reload.md -- a commit and a SIGHUP racing
// Related: main_reload.go -- stageSIGHUPCandidate
// Related: config_source.go -- promoteConfigCandidate

package hub

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// commitBeforeLockStore runs a commit the first time a config lock is asked
// for, before handing the lock out. It places a whole commit at the instant
// the SIGHUP staging first takes the config lock. Not safe for concurrent use.
type commitBeforeLockStore struct {
	storage.Storage
	commit func()
}

func (s *commitBeforeLockStore) AcquireLock(path string) (storage.WriteGuard, error) {
	if commit := s.commit; commit != nil {
		s.commit = nil
		commit()
	}
	return s.Storage.AcquireLock(path)
}

// TestSIGHUPStageDoesNotUndoCommitLandingBeforeTheLock proves a SIGHUP reload
// never stages a source a commit has already replaced.
//
// Goal: the staging reads the config source and writes the candidate under
// one hold of the config lock, so no commit can land between the two. When
// the read happened before the lock, a commit that staged, applied and
// promoted in that gap was undone: the SIGHUP staged, applied and promoted
// the configuration the commit had replaced.
//
// Method: the daemon runs A. A wrapping store runs a whole commit of B (stage
// the candidate, promote it) the moment stageSIGHUPCandidate first asks for
// the config lock, which is the instant a staging that read A before the lock
// would go on to write A. The staged candidate MUST be B, in both source
// modes. Deterministic: the commit runs synchronously inside AcquireLock.
func TestSIGHUPStageDoesNotUndoCommitLandingBeforeTheLock(t *testing.T) {
	modes := map[string]storage.ConfigSource{"stored": storage.ConfigSourceStored, "file": storage.ConfigSourceFile}
	for name, mode := range modes {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "router.conf")
			raw := newTestStore(t, filepath.Dir(path))
			committer := storage.BindConfigSource(raw, path, mode)
			if err := os.WriteFile(path, []byte("A"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := initializeConfigSource(committer, path, []byte("A")); err != nil {
				t.Fatal(err)
			}

			var commitErr error
			racing := &commitBeforeLockStore{Storage: raw}
			racing.commit = func() {
				if _, err := storage.WriteCandidateVersion(committer, path, []byte("B"), time.Now()); err != nil {
					commitErr = err
					return
				}
				commitErr = promoteConfigCandidate(committer, path)
			}
			signaller := storage.BindConfigSource(racing, path, mode)

			if err := stageSIGHUPCandidate(signaller, path); err != nil {
				t.Fatalf("stage SIGHUP candidate: %v", err)
			}
			if commitErr != nil {
				t.Fatalf("the racing commit failed, so the race was not reproduced: %v", commitErr)
			}
			if racing.commit != nil {
				t.Fatal("the staging never took the config lock, so the race was not reproduced")
			}
			staged, _, ok, err := storage.ReadCandidateConfig(committer, path)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatal("no candidate staged")
			}
			if string(staged) != "B" {
				t.Fatalf("SIGHUP staged %q after the commit of %q landed: the reload would undo the commit", staged, "B")
			}
		})
	}
}
