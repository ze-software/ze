// Design: docs/architecture/core-design.md -- published command surfaces come from the live catalog
// Overview: command_surfaces.go -- loadLiveCommandCatalog, BuildDaemon, runCommandCatalog

package docyangcontract

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// VALIDATES: the live catalog the drift check reads is bounded per phase: the
// daemon compile has its own bound, and the catalog bound covers only the run of
// the compiled daemon.
// PREVENTS: a compile on a loaded host outlasting the catalog bound, which
// `./le verify worktree` reported as "could not generate or parse the live
// per-command catalog ... signal: killed".
// METHOD: the stated daemon build takes longer than the catalog bound, then
// answers a script that prints the fixture catalog at once. The catalog must
// parse, and the temporary daemon directory must be gone afterwards.
func TestLiveCommandCatalogBoundExcludesCompile(t *testing.T) {
	root := t.TempDir()
	fixture := writeRenderedCommandCatalogFixture(t, root)

	previousTimeout := commandCatalogGenerationTimeout
	previousDaemon := catalogDaemon
	t.Cleanup(func() {
		commandCatalogGenerationTimeout = previousTimeout
		catalogDaemon = previousDaemon
	})
	commandCatalogGenerationTimeout = 2 * time.Second
	var daemonDir string
	catalogDaemon = func(repository, dir string) (string, error) {
		if repository != root {
			t.Errorf("daemon built from %q, want %q", repository, root)
		}
		daemonDir = dir
		// The compile outlasts the catalog bound, as it did on the loaded host.
		time.Sleep(2*commandCatalogGenerationTimeout + time.Second)
		binary := filepath.Join(dir, "ze")
		script := "#!/bin/sh\nexec cat '" + fixture + "'\n"
		if err := os.WriteFile(binary, []byte(script), 0o700); err != nil { //nolint:gosec // test-owned script in a temp dir
			return "", err
		}
		return binary, nil
	}

	_, commands, err := loadLiveCommandCatalog(root, "")
	if err != nil {
		t.Fatalf("live catalog after a slow compile: %v", err)
	}
	if len(commands) == 0 {
		t.Fatal("live catalog after a slow compile parsed no command")
	}
	if daemonDir == "" {
		t.Fatal("the live catalog compiled no daemon")
	}
	if _, err := os.Stat(daemonDir); !os.IsNotExist(err) {
		t.Fatalf("daemon directory %s survived the catalog read: %v", daemonDir, err)
	}
}
