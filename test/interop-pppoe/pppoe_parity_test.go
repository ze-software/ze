package interop_pppoe_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	pppoeleaf "github.com/ze-software/ze/internal/le/interoplab/pppoe"
)

var _ func(context.Context, pppoeleaf.Options) interoplab.SuiteReport = pppoeleaf.Run
var _ func(context.Context, string, pppoeleaf.Options) interoplab.SuiteReport = pppoeleaf.RunAt

// TestUnknownSelectorFailsBeforePeerSetup retains the explicit and environment
// failure contracts independently of any enumerated scenario population.
func TestUnknownSelectorFailsBeforePeerSetup(t *testing.T) {
	root := repositoryRoot(t)
	report := pppoeleaf.RunAt(context.Background(), root, pppoeleaf.Options{
		Scenario: "not-a-pppoe-scenario",
		NoBuild:  true,
		Suffix:   "selector",
	})
	if report.Code != 1 || report.SetupError != "no scenario matching 'not-a-pppoe-scenario' found" {
		t.Fatalf("native missing-selector result = %#v", report)
	}
	t.Setenv("ZE_PPPOE_INTEROP_SCENARIO", "missing-from-environment")
	report = pppoeleaf.RunAt(context.Background(), root, pppoeleaf.Options{
		NoBuild: true,
		Suffix:  "selector",
	})
	if report.Code != 1 || report.SetupError != "no scenario matching 'missing-from-environment' found" {
		t.Fatalf("environment-selected missing scenario result = %#v", report)
	}
}

// VALIDATES: The PPPoE fixture tree has no interpreter-backed source or cache.
// PREVENTS: Reintroducing an executable fallback beside the typed plans.
func TestPPPoETreeIsGoOnly(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			t.Errorf("Python cache directory remains: %s", path)
			return filepath.SkipDir
		}
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".py") ||
			strings.HasSuffix(entry.Name(), ".pyc") || strings.HasSuffix(entry.Name(), ".sh")) {
			t.Errorf("interpreter-backed artifact remains: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(working, "..", ".."))
}
