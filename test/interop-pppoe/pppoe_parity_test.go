package interop_pppoe_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	pppoeleaf "github.com/ze-software/ze/internal/le/interoplab/pppoe"
)

var _ func(context.Context, pppoeleaf.Options) interoplab.SuiteReport = pppoeleaf.Run
var _ func(context.Context, string, pppoeleaf.Options) interoplab.SuiteReport = pppoeleaf.RunAt

var pppoeScenarios = []string{
	"01-pppoe-chap-ipv4",
	"02-ze-ac-pppd-client",
}

// VALIDATES: The native selector exposes both reviewed PPPoE roles under their
// exact names and lexical order.
// PREVENTS: A Go-only gate dropping either the Ze client or access-concentrator role.
func TestNativeScenarioPopulationIsExact(t *testing.T) {
	if got := pppoeleaf.ScenarioNames(); !reflect.DeepEqual(got, pppoeScenarios) {
		t.Fatalf("native scenarios = %v, want %v", got, pppoeScenarios)
	}
	entries, err := os.ReadDir("scenarios")
	if err != nil {
		t.Fatal(err)
	}
	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, entry.Name())
		}
	}
	if !reflect.DeepEqual(directories, pppoeScenarios) {
		t.Fatalf("fixture directories = %v, want %v", directories, pppoeScenarios)
	}

	root := repositoryRoot(t)
	report := pppoeleaf.RunAt(context.Background(), root, pppoeleaf.Options{
		Scenario: "not-a-pppoe-scenario",
		NoBuild:  true,
		Suffix:   "population",
	})
	if report.Code != 1 || report.SetupError != "no scenario matching 'not-a-pppoe-scenario' found" {
		t.Fatalf("native missing-selector result = %#v", report)
	}

	t.Setenv("ZE_PPPOE_INTEROP_SCENARIO", "missing-from-environment")
	report = pppoeleaf.RunAt(context.Background(), root, pppoeleaf.Options{
		NoBuild: true,
		Suffix:  "population",
	})
	if report.Code != 1 || report.SetupError != "no scenario matching 'missing-from-environment' found" {
		t.Fatalf("environment-selected missing scenario result = %#v", report)
	}
}

// VALIDATES: Docker peers, credentials, role selection, and protocol configs
// remain the exact reviewed inputs consumed by the typed plans.
// PREVENTS: Peer substitution or copied fixture drift after runner removal.
func TestNativeConfigBytesArePinned(t *testing.T) {
	files := map[string]string{
		"Dockerfile.ze":                               "73cb9f9e42bdefdd491ae76b8673ebec6f5066e10618ce0c6c8d70584aa71963",
		"Dockerfile.accel":                            "9d64c266c9481adc00df37b70a83aa8c7bddbab8dfc75f4c7c05ddabe1bbcc6a",
		"Dockerfile.client":                           "5482e6e0f678503e95c1ff10977ca91ff70211942a3cb449ec3f830582346e28",
		"scenarios/01-pppoe-chap-ipv4/ze.conf":        "1b1427eb24d3cc599f02d7e285606a99d91ca64a7d9df1f222bb757e26d8f54b",
		"scenarios/01-pppoe-chap-ipv4/accel-ppp.conf": "c7b096b09feb5492123e4f32fff09a167abc30b798f5a6b22ae6e007f3d5fa05",
		"scenarios/01-pppoe-chap-ipv4/chap-secrets":   "04525c6958851189a53ea92d539f1dd5970eceff8e95d626eb7033438b912067",
		"scenarios/01-pppoe-chap-ipv4/role":           "a15ed3e38a6f9a28ca3acbe40026602dfff0c833b992f2937e4722520480f6fc",
		"scenarios/02-ze-ac-pppd-client/ze.conf":      "86018076f6fa758abb91a5108f39fda38ca78ddd6d0c15325f77ca630daa89e5",
		"scenarios/02-ze-ac-pppd-client/role":         "9390bb877bffd73137ca2201fb106d5b6096755e34a44c52946c8b658fd6100e",
	}
	for name, want := range files {
		data, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		digest := sha256.Sum256(data)
		if got := hex.EncodeToString(digest[:]); got != want {
			t.Errorf("%s sha256 = %s, want %s", name, got, want)
		}
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
