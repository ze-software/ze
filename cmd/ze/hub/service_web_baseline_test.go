//go:build ze_web

package hub

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
)

// TestWebBaselineReparseLeavesProcessEnvironment proves the web service's
// start-time re-parse of the committed config applies nothing to the process.
//
// Goal: the daemon already applied the file's `environment { }` block when it
// loaded the config; the web service reading the same file again for its
// compare baseline must not publish it a second time, so the process's base
// log level (`ze.log`) stays as it was.
// Method: a store holding a config whose environment block sets the log
// level; committedBaselineTree parses it, the tree must carry the block, and
// the env key must still be unset (the block writes it only while unset).
func TestWebBaselineReparseLeavesProcessEnvironment(t *testing.T) {
	// The environment block writes ze.log only while it is unset, so the test
	// establishes that itself: an earlier test may have left a level behind.
	isolateBaseLogLevel(t)
	const levelKey = baseLogLevelKey
	if before := env.Get(levelKey); before != "" {
		t.Fatalf("%s is %q before the test: the environment block would not be written over it, so the test proves nothing", levelKey, before)
	}

	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("blob storage: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Errorf("close store: %v", closeErr)
		}
	})
	const configPath = "ze.conf"
	if writeErr := store.WriteFile(configPath, []byte("environment {\n\tlog {\n\t\tlevel debug;\n\t}\n}\n"), 0o600); writeErr != nil {
		t.Fatalf("write config: %v", writeErr)
	}

	tree := committedBaselineTree(store, configPath)
	if tree.GetContainer("environment") == nil {
		t.Fatal("baseline tree has no environment block: the parse did not read the committed config, so the test proves nothing")
	}
	if after := env.Get(levelKey); after != "" {
		t.Fatalf("the web baseline re-parse set the process's %s to %q from the committed config", levelKey, after)
	}
}
