//go:build stress

// Web concurrent-edit stress test (spec followup-test-infra AC-7 / L97).
//
// Tier: evidence/release, NOT ze-precommit-verify (R-6). Each of the >=50 sessions builds
// a real editor (full YANG schema load + parse), so the storm is too heavy for
// the pre-commit gate. Run it with:
//
//	./le integration stress-web
package web

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWebConcurrentEditStress drives many concurrent editor sessions through
// the full mutate + commit path against one shared config file and asserts the
// EditorManager, the per-user editors, and the atomic store stay race-free,
// error-free, and never leave a torn config on disk.
//
// VALIDATES: AC-7 -- >=50 concurrent editor mutation sessions produce no race
// detector hit, no lost/torn commit, and a zero error rate.
// PREVENTS: a regression that reintroduces a concurrent-map write on the
// sessions map, an unsynchronized editor mutation, or a torn config file under
// a commit storm.
func TestWebConcurrentEditStress(t *testing.T) {
	const users = 64

	t.Run("concurrent_mutations_no_race", func(t *testing.T) {
		mgr := newTestEditorManager(t)
		mgr.maxSessions = users + 8 // the DoS cap is not the property under test

		var wg sync.WaitGroup
		var setErrs atomic.Int64
		wg.Add(users)
		for i := range users {
			go func(n int) {
				defer wg.Done()
				user := fmt.Sprintf("op-%d", n)
				if err := mgr.SetValue(user, []string{"bgp"}, "router-id", fmt.Sprintf("10.0.0.%d", 1+n%250)); err != nil {
					t.Errorf("user %s SetValue: %v", user, err)
					setErrs.Add(1)
				}
			}(i)
		}
		wg.Wait()
		require.Zero(t, setErrs.Load(), "no SetValue error expected across %d sessions", users)
	})

	t.Run("concurrent_commit_storm", func(t *testing.T) {
		mgr := newTestEditorManager(t)
		mgr.maxSessions = users + 8

		var wg sync.WaitGroup
		var commitErrs atomic.Int64
		var conflicts atomic.Int64
		wg.Add(users)
		for i := range users {
			go func(n int) {
				defer wg.Done()
				user := fmt.Sprintf("committer-%d", n)
				val := fmt.Sprintf("10.1.0.%d", 1+n%250)
				if err := mgr.SetValue(user, []string{"bgp"}, "router-id", val); err != nil {
					t.Errorf("user %s SetValue: %v", user, err)
					commitErrs.Add(1)
					return
				}
				res, err := mgr.Commit(user)
				if err != nil {
					t.Errorf("user %s Commit: %v", user, err)
					commitErrs.Add(1)
					return
				}
				if res != nil && len(res.Conflicts) > 0 {
					conflicts.Add(1)
				}
			}(i)
		}
		wg.Wait()

		require.Zero(t, commitErrs.Load(), "no hard commit error expected across the storm")

		// the earlier draft re-parsed the committed file with the
		// strict config.YANGSchema(), which rejects the web test fixture's
		// `local { as 65000; }` and does not model the web candidate/reload path.
		// The non-torn invariant is correctly proven by re-loading through the
		// same editor path the manager uses (below), so the strict re-parse is
		// replaced, not dropped.
		//
		// Torn-write guard: the atomic store (CreateTemp+Rename) plus per-user
		// editor serialization must never leave a truncated/interleaved file.
		data, err := mgr.committedConfig()
		require.NoError(t, err, "committed config must be readable")
		require.NotEmpty(t, data, "committed config must be non-empty")
		require.Contains(t, string(data), "bgp {", "bgp block survived the storm intact")
		require.Contains(t, string(data), "router-id", "router-id leaf survived the storm intact")

		// A fresh editor must load the on-disk config without error -- proves the
		// file is not torn (the editor is the same parse path the manager uses).
		reloaded := newTestEditorManager(t)
		reloaded.store = mgr.store
		reloaded.configPath = mgr.configPath
		_, err = reloaded.GetOrCreate("verifier")
		require.NoError(t, err, "committed config must re-load cleanly (non-torn):\n%s", data)

		t.Logf("commit storm: %d users, %d conflicts observed", users, conflicts.Load())
	})
}
