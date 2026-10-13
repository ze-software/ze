// Design: docs/architecture/hub-architecture.md -- explicit-file commit authority
// Related: config_source.go -- promoteSignalCandidate, promoteConfigCandidate
// Related: main_reload.go -- handleSIGHUPReload, promoteReloadCandidate

package hub

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestSIGHUPReloadKeepsOperatorEditMadeDuringReload reproduces the overlap
// journaled in plan/journal/reload-rolls-back-instead-of-applying.md
// (bgp-bfd-removed-live, 2 in 9).
//
// Goal: a signal reload MUST NOT write the explicit config file, and an edit
// the operator makes while it runs MUST NOT fail or roll it back. That edit
// belongs to the SIGHUP that announced it, which runs next.
//
// Method: the daemon runs file A. The operator writes B and sends SIGHUP; the
// reload stages B (stageSIGHUPCandidate) and runs through doReloadContext with
// the signal source. While its loader runs, the operator writes the next edit:
// either A back, byte for byte (what bgp-bfd-removed-live's config3.conf is
// beside its boot config, and the case that once rewrote the file to B), or a
// third value C (the case that once failed the reload and rolled it back). The
// reload must succeed, record B as the accepted version, leave the file holding
// the operator's edit, and the queued SIGHUP must stage that edit.
func TestSIGHUPReloadKeepsOperatorEditMadeDuringReload(t *testing.T) {
	bootConfig, reloaded := []byte("bfd enabled"), []byte("bfd disabled")
	for _, tc := range []struct {
		name     string
		nextEdit []byte
	}{
		{name: "written back to the accepted bytes", nextEdit: bootConfig},
		{name: "a third value", nextEdit: []byte("bfd removed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "router.conf")
			store := newTestFileStore(t, path)
			reactor := &reloadTestReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}}
			server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { server.Stop() })

			if err := os.WriteFile(path, bootConfig, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := initializeConfigSource(store, path, bootConfig); err != nil {
				t.Fatal(err)
			}

			// First SIGHUP: the operator's edit is staged.
			if err := os.WriteFile(path, reloaded, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := stageSIGHUPCandidate(store, path); err != nil {
				t.Fatal(err)
			}

			// The loader reads the staged candidate, as diskConfigLoaders does,
			// and the operator's next edit lands while the reload applies it.
			load := func() (map[string]any, *zeconfig.Tree, error) {
				staged, readErr := storage.ReadReloadConfig(store, path)
				if readErr != nil {
					return nil, nil, readErr
				}
				if !bytes.Equal(staged, reloaded) {
					t.Errorf("the reload loaded %q, the SIGHUP staged %q", staged, reloaded)
				}
				if writeErr := os.WriteFile(path, tc.nextEdit, 0o600); writeErr != nil {
					return nil, nil, writeErr
				}
				return map[string]any{"bgp": map[string]any{"router-id": "2.2.2.2"}}, nil, nil
			}

			if err := doReloadContext(context.Background(), reloadSourceSignal, server, nil, nil, store, path, load, nil); err != nil {
				t.Fatalf("the reload failed on an edit made after it staged: %v", err)
			}

			disk, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(disk, tc.nextEdit) {
				t.Fatalf("the reload overwrote the operator's edit: file = %q, operator wrote %q", disk, tc.nextEdit)
			}
			active, err := storage.ReadActiveConfig(store, path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(active, reloaded) {
				t.Fatalf("accepted version = %q, the reload applied %q", active, reloaded)
			}

			// The queued SIGHUP reads what the operator wrote.
			if err := stageSIGHUPCandidate(store, path); err != nil {
				t.Fatal(err)
			}
			next, err := storage.ReadReloadConfig(store, path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(next, tc.nextEdit) {
				t.Fatalf("the queued SIGHUP would apply %q, the operator wrote %q", next, tc.nextEdit)
			}
		})
	}
}

// TestCommitPromotionRefusesFileWrittenBackToAcceptedBytes pins the other half
// of the split: a commit the daemon authored still publishes through the file
// and still refuses a file that no longer holds the accepted bytes, even when
// it holds the candidate's own bytes. That match was once read as a signal
// reload adopting the file, which is the inference reloadSource replaced.
func TestCommitPromotionRefusesFileWrittenBackToAcceptedBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.conf")
	store := newTestFileStore(t, path)
	accepted, candidate := []byte("accepted"), []byte("candidate")
	if err := os.WriteFile(path, accepted, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := initializeConfigSource(store, path, accepted); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.WriteCandidateVersion(store, path, candidate, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, candidate, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := promoteReloadCandidate(reloadSourceCommit, store, path); err == nil {
		t.Fatal("a commit promoted over a file edited outside the daemon")
	}

	// With the file untouched the commit publishes its candidate to it.
	if err := os.WriteFile(path, accepted, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := promoteReloadCandidate(reloadSourceCommit, store, path); err != nil {
		t.Fatal(err)
	}
	disk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(disk, candidate) {
		t.Fatalf("file = %q, the commit published %q", disk, candidate)
	}
}

// TestRequestReloadPromotesTheOperatorFile covers the `request reload` RPC,
// which the hub wires through newRequestReloadFunc (main.go).
//
// Goal: a reload the operator asks for over the API applies the file they
// wrote and records it as the accepted version, without writing the file, the
// same as SIGHUP. Run as a commit, it applied the file without promoting it,
// so the accepted version went stale, and the next commit refused the
// operator's own file as "changed externally".
//
// Method: through newRequestReloadFunc, the handler main.go wires. The daemon
// runs A; the operator writes B and requests a reload. It must succeed, leave B
// on disk and make B the accepted version; a request while the reload mutex is
// held is refused; a commit from B then publishes.
func TestRequestReloadPromotesTheOperatorFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.conf")
	store := newTestFileStore(t, path)
	reactor := &reloadTestReactor{tree: map[string]any{"bgp": map[string]any{"router-id": "1.1.1.1"}}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Stop() })

	bootConfig, edited := []byte("bfd enabled"), []byte("bfd removed")
	if err := os.WriteFile(path, bootConfig, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := initializeConfigSource(store, path, bootConfig); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	load := func() (map[string]any, *zeconfig.Tree, error) { //nolint:unparam // the daemon loader type; this reload parses no tree
		staged, readErr := storage.ReadReloadConfig(store, path)
		if readErr != nil {
			return nil, nil, readErr
		}
		return map[string]any{"bgp": map[string]any{"router-id": string(staged)}}, nil, nil
	}
	// The handler main.go wires as the server's FullReloadFunc, with the
	// startup wait already satisfied.
	ready := func(context.Context) error { return nil }
	reload := func() error {
		return newRequestReloadFunc(&reloadGate{}, &sync.Mutex{}, ready, nil, server, nil, nil, store, path, load, nil)(context.Background())
	}

	if err := reload(); err != nil {
		t.Fatalf("request reload: %v", err)
	}
	active, err := storage.ReadActiveConfig(store, path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(active, edited) {
		t.Fatalf("accepted version = %q after the reload applied %q", active, edited)
	}

	// A request while another signal reload holds the daemon's mutex is
	// refused at once with ErrReloadInProgress, never queued: a plugin that
	// sends it from inside its own apply would otherwise deadlock.
	held := &sync.Mutex{}
	held.Lock()
	err = newRequestReloadFunc(&reloadGate{}, held, ready, nil, server, nil, nil, store, path, load, nil)(context.Background())
	held.Unlock()
	if !errors.Is(err, pluginserver.ErrReloadInProgress) {
		t.Fatalf("request reload under a held reload = %v, want ErrReloadInProgress", err)
	}
	if err := reload(); err != nil {
		t.Fatalf("request reload after the holder released: %v", err)
	}
	disk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(disk, edited) {
		t.Fatalf("file = %q, the operator wrote %q", disk, edited)
	}

	// The next commit starts from the operator's file and publishes.
	committed := []byte("bfd profile slow")
	if _, err := storage.WriteCandidateVersion(store, path, committed, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := promoteReloadCandidate(reloadSourceCommit, store, path); err != nil {
		t.Fatalf("a commit after request reload refused the operator's file: %v", err)
	}
}
