// VALIDATES: an evolution that changes how the router behaves is reported at the
// default log level, naming the file, the evolution, what it wrote and why, and
// whether the file was saved; a shape-only evolution stays at Info.
// PREVENTS: a config rewrite that changes security behavior (an IPsec catch-all
// kept on bypass) happening in silence, because Info is below slogutil.DefaultLevel.

package hub

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const testNotice = "wrote `old-leaf on`; set `old-leaf off` to take the new default"

// registerNoticeEvolution registers an evolution like registerTestEvolution,
// with notice as its Notice, StampedOnly as given.
func registerNoticeEvolution(t *testing.T, release, name, notice string, stampedOnly bool) func() {
	t.Helper()
	err := migration.RegisterEvolution(migration.Evolution{
		Release:     release,
		Name:        name,
		Notice:      notice,
		StampedOnly: stampedOnly,
		Detect: func(tree *zeconfig.Tree) bool {
			_, ok := tree.Get("evolved-" + name)
			return !ok
		},
		Apply: func(tree *zeconfig.Tree) (*zeconfig.Tree, error) {
			tree.Set("evolved-"+name, "yes")
			return tree, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return func() { resetEvolutions(t) }
}

// defaultLevelLogger logs to buf at the level a subsystem gets with no ze.log setting.
func defaultLevelLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slogutil.DefaultLevel}))
}

// TestApplyEvolutionsWarnsANoticeForAFile checks the saved-file case: the Warn
// names the file, the evolution and the notice, and says the file was saved.
func TestApplyEvolutionsWarnsANoticeForAFile(t *testing.T) {
	cleanup := evolveTestSetup(t)
	defer cleanup()
	defer registerNoticeEvolution(t, "26.06.01", "old-default", testNotice, true)()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	originalData := []byte("# ze-schema: 26.05.01\nset environment log level warn\n")
	if err := os.WriteFile(configPath, originalData, 0o600); err != nil {
		t.Fatal(err)
	}
	store := newTestFileStore(t, configPath)
	tree, err := parseTestConfig(t, string(originalData))
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := applyEvolutions(defaultLevelLogger(&buf), store, configPath, originalData, tree, "26.05.01"); err != nil {
		t.Fatal(err)
	}
	logged := buf.String()
	for _, want := range []string{"level=WARN", configPath, "old-default", "set `old-leaf off`", "saved"} {
		if !strings.Contains(logged, want) {
			t.Errorf("default-level log lacks %q:\n%s", want, logged)
		}
	}
	if strings.Contains(logged, "not saved") {
		t.Errorf("a file written back must not be reported as not saved:\n%s", logged)
	}
}

// TestApplyEvolutionsWarnsANoticeForStdin checks the stdin case: nothing is
// written back, so the Warn says so and that the change repeats at every start.
func TestApplyEvolutionsWarnsANoticeForStdin(t *testing.T) {
	cleanup := evolveTestSetup(t)
	defer cleanup()
	defer registerNoticeEvolution(t, "26.06.01", "old-default", testNotice, true)()

	originalData := []byte("# ze-schema: 26.05.01\nset environment log level warn\n")
	tree, err := parseTestConfig(t, string(originalData))
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := applyEvolutions(defaultLevelLogger(&buf), newTestStore(t), "-", originalData, tree, "26.05.01"); err != nil {
		t.Fatal(err)
	}
	logged := buf.String()
	for _, want := range []string{"level=WARN", "stdin", "old-default", "set `old-leaf off`", "not saved"} {
		if !strings.Contains(logged, want) {
			t.Errorf("default-level log lacks %q:\n%s", want, logged)
		}
	}
}

// TestApplyEvolutionsKeepsAShapeChangeQuiet checks that an evolution with no
// Notice logs nothing at the default level.
func TestApplyEvolutionsKeepsAShapeChangeQuiet(t *testing.T) {
	cleanup := evolveTestSetup(t)
	defer cleanup()
	defer registerNoticeEvolution(t, "26.06.01", "shape", "", false)()

	originalData := []byte("# ze-schema: 26.05.01\nset environment log level warn\n")
	tree, err := parseTestConfig(t, string(originalData))
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	outcome, err := applyEvolutions(defaultLevelLogger(&buf), newTestStore(t), "-", originalData, tree, "26.05.01")
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.applied) != 1 {
		t.Fatalf("applied %v, want the shape evolution", outcome.applied)
	}
	if buf.Len() != 0 {
		t.Errorf("a shape-only evolution logged at the default level:\n%s", buf.String())
	}
}
