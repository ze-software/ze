// Design: docs/architecture/config/syntax.md -- schema evolution at startup
// Related: main.go -- runYANGConfig, Phase 1b, the start these tests drive
// Related: main_evolve.go -- applyEvolutions, the step the start must reach

// VALIDATES: `ze start` reaches schema evolution. runYANGConfig, the start
// itself, hands applyEvolutions the stamp of the content LoadConfig parsed (the
// rollback's own stamp after a recovery), then starts on the evolved tree and
// on the bytes the evolution wrote back, and the behavior notice is logged
// with the file saved.
// Method: a test-only evolution registered for the test and removed after it,
// a stamped config file in a file-bound store, and runYANGConfig driven until
// the BNG registration seam. The seam captures the tree and the boot bytes the
// daemon starts on, then ends the start.
// PREVENTS: a start that stops calling applyEvolutions, passes it the stamp of
// the re-stamped bytes, or drops its tree or its bytes, while applyEvolutions'
// own unit tests stay green because they call it directly.

package hub

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/core/env"
)

const (
	// evolveStartRelease is the release evolveTestSetup stamps this binary
	// with, and the release of the test evolution: a file stamped by it is
	// current, and one stamped earlier is owed the evolution.
	evolveStartRelease = "26.05.26"
	evolveStartOld     = "26.05.01"
	evolveStartName    = "test-start-log-level"
	evolveStartNotice  = "wrote `environment log level error`; set `environment log level warn` to undo it"
	evolveStartWarned  = "config evolution changed behavior"
	evolveStartSaved   = "saved: the file now states the change"
	evolveStartBody    = "set environment log level warn\n"
)

// errEvolveStartReached ends the start at the BNG registration seam, once the
// tree and the boot bytes are captured.
var errEvolveStartReached = errors.New("test: the start reached the BNG registration")

// evolveStartLevel answers the `environment log level` leaf of tree, "" when
// the tree sets none.
func evolveStartLevel(tree *zeconfig.Tree) string {
	environment := tree.GetContainer("environment")
	if environment == nil {
		return ""
	}
	logging := environment.GetContainer("log")
	if logging == nil {
		return ""
	}
	level, ok := logging.Get("level")
	if !ok {
		return ""
	}
	return level
}

// registerStartEvolution registers, for the rest of the test, a test-only
// evolution that changes how the router behaves: it rewrites the log level
// `warn` into `error`. It is limited to stamped files, as an evolution that
// restores an old default is.
func registerStartEvolution(t *testing.T) {
	t.Helper()
	err := migration.RegisterEvolution(migration.Evolution{
		Release:     evolveStartRelease,
		Name:        evolveStartName,
		Notice:      evolveStartNotice,
		StampedOnly: true,
		Detect: func(tree *zeconfig.Tree) bool {
			return evolveStartLevel(tree) == "warn"
		},
		Apply: func(tree *zeconfig.Tree) (*zeconfig.Tree, error) {
			result := tree.Clone()
			result.GetOrCreateContainer("environment").GetOrCreateContainer("log").Set("level", "error")
			return result, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resetEvolutions(t) })
}

// evolveStart is what one start answered up to the BNG registration seam.
type evolveStart struct {
	exit   int
	tree   *zeconfig.Tree // the tree the start hands its subsystems
	booted []byte         // the bytes the start boots from, as its store answers them
	stderr string
}

// startUntilBNG runs runYANGConfig on data, read from configPath through store,
// until the BNG registration seam, and answers what the start ran on.
func startUntilBNG(t *testing.T, store storage.Storage, configPath string, data []byte) evolveStart {
	t.Helper()
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	withCleanRegistry(t)
	oldDir := env.Get("ze.config.dir")
	require.NoError(t, env.Set("ze.config.dir", t.TempDir()))
	t.Cleanup(func() { require.NoError(t, env.Set("ze.config.dir", oldDir)) })
	oldBNG := bngRegister
	t.Cleanup(func() { bngRegister = oldBNG })

	var start evolveStart
	bngRegister = func(tree *zeconfig.Tree, _ map[string]any, _ *engine.Engine) ([]webPortalService, error) {
		start.tree = tree
		booted, err := store.ReadFile(configPath)
		if err != nil {
			t.Errorf("read the boot bytes at the BNG registration: %v", err)
		}
		start.booted = booted
		return nil, errEvolveStartReached
	}

	output, err := os.CreateTemp(t.TempDir(), "evolve-start-stderr-")
	require.NoError(t, err)
	func() {
		original := os.Stderr
		os.Stderr = output
		defer func() { os.Stderr = original }()
		start.exit = runYANGConfig(store, configPath, data, nil, 0, -1,
			false, false, "", false, "", "", false, nil)
	}()
	require.NoError(t, output.Close())
	logged, err := os.ReadFile(output.Name())
	require.NoError(t, err)
	start.stderr = string(logged)

	if start.tree == nil {
		t.Fatalf("the start ended (exit %d) before the BNG registration:\n%s", start.exit, start.stderr)
	}
	require.Equal(t, 1, start.exit, "the seam's refusal ends the start")
	require.Contains(t, start.stderr, errEvolveStartReached.Error())
	return start
}

// requireEvolvedStart checks a start that owed the evolution: it ran on the
// evolved tree and booted from the bytes written back to the file, which carry
// this release's stamp and the evolved leaf, and it warned with the file saved.
func requireEvolvedStart(t *testing.T, start evolveStart, configPath string) {
	t.Helper()
	require.Equal(t, "error", evolveStartLevel(start.tree), "the start runs on the evolved tree")

	disk, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Equal(t, evolveStartRelease, zeconfig.ScanStampRelease(disk), "the file is written back re-stamped")
	saved, err := parseTestConfig(t, string(disk))
	require.NoError(t, err)
	require.Equal(t, "error", evolveStartLevel(saved), "the file written back states the evolved leaf")
	require.True(t, bytes.Equal(start.booted, disk),
		"the start boots from the bytes the evolution wrote back:\nbooted:\n%s\ndisk:\n%s", start.booted, disk)

	for _, want := range []string{evolveStartWarned, "evolution=" + evolveStartName, "environment log level error", evolveStartSaved} {
		require.Contains(t, start.stderr, want)
	}
}

// TestStartEvolvesAnOldFile checks a file stamped by an earlier release than
// the evolution: the start applies it, saves it and runs on it.
func TestStartEvolvesAnOldFile(t *testing.T) {
	defer evolveTestSetup(t)()
	registerStartEvolution(t)

	configPath := filepath.Join(t.TempDir(), "config.conf")
	original := []byte("# ze-schema: " + evolveStartOld + "\n" + evolveStartBody)
	require.NoError(t, os.WriteFile(configPath, original, 0o600))
	store := newTestFileStore(t, configPath)

	requireEvolvedStart(t, startUntilBNG(t, store, configPath, original), configPath)
}

// TestStartEvolvesARecoveredRollback checks the downgrade path: the file was
// written by a newer release, so the start recovers a rollback stamped by an
// earlier one and publishes it re-stamped with this release. Only the
// rollback's own stamp, LoadConfigResult.StampRelease, says it is owed the
// evolution; a start that read the stamp of the published bytes would skip it.
func TestStartEvolvesARecoveredRollback(t *testing.T) {
	defer evolveTestSetup(t)()
	registerStartEvolution(t)

	configPath := filepath.Join(t.TempDir(), "config.conf")
	future := []byte("# ze-schema: 99.01.01\nset unknown-future-leaf value\n")
	require.NoError(t, os.WriteFile(configPath, future, 0o600))
	store := newTestFileStore(t, configPath)
	rollback := []byte("# ze-schema: " + evolveStartOld + "\n" + evolveStartBody)
	require.NoError(t, store.WriteVersion(configPath, rollback, time.Date(2026, 5, 20, 12, 0, 0, 0, time.Local)))

	requireEvolvedStart(t, startUntilBNG(t, store, configPath, future), configPath)
}

// TestStartLeavesACurrentFileAlone checks a file stamped by the evolution's own
// release: the start applies nothing, writes nothing and warns of nothing.
func TestStartLeavesACurrentFileAlone(t *testing.T) {
	defer evolveTestSetup(t)()
	registerStartEvolution(t)

	configPath := filepath.Join(t.TempDir(), "config.conf")
	original := []byte("# ze-schema: " + evolveStartRelease + "\n" + evolveStartBody)
	require.NoError(t, os.WriteFile(configPath, original, 0o600))
	store := newTestFileStore(t, configPath)

	start := startUntilBNG(t, store, configPath, original)
	require.Equal(t, "warn", evolveStartLevel(start.tree), "a current file runs as written")
	disk, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.True(t, bytes.Equal(original, disk), "a current file is not written back:\n%s", disk)
	require.True(t, bytes.Equal(original, start.booted), "a current file boots as written:\n%s", start.booted)
	require.False(t, strings.Contains(start.stderr, evolveStartWarned), "a current file is warned of no change:\n%s", start.stderr)
}
