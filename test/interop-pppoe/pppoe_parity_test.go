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

// pppoeScenarios is the whole reviewed population, in the lexical order
// ScenarioNames answers and os.ReadDir walks. The four named scenarios arrived
// after the two numbered ones and each carries its own RFC claim:
// pppoe-empty-service-name (0e1543cc9a, the mandatory Service-Name tag on every
// PADO and PADS), pppoe-padr-replay (4efaa4fd8d, the per-MAC allocation bound),
// and the two ipv6cp cases (fd7cd7b44e, whether the peer's identifier was ever
// negotiated).
var pppoeScenarios = []string{
	"01-pppoe-chap-ipv4",
	"02-ze-ac-pppd-client",
	"ipv6cp-missing-option",
	"ipv6cp-zero-identifier",
	"pppoe-empty-service-name",
	"pppoe-padr-replay",
}

// VALIDATES: The native selector exposes every reviewed PPPoE role under its
// exact name and lexical order.
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
		// Repinned for 5837fd3247, which removed the Go toolchain from this
		// image: it is now an alpine:3.21 base plus one apk add plus a COPY of
		// the binary StageBinaries cross-compiles
		// (internal/le/interoplab/zebuild.go).
		// Repinned for 4a923c72de: the header comment names `./le test integration`.
		"Dockerfile.ze":    "54a8c905d8b39034273744f3c7fbefa3eae5397213917a8402d59fd1893b4a90",
		"Dockerfile.accel": "9d64c266c9481adc00df37b70a83aa8c7bddbab8dfc75f4c7c05ddabe1bbcc6a",
		// Repinned for 0e1543cc9a, which added tcpdump to the client image: the
		// pppoe-empty-service-name checker captures the discovery exchange to
		// read the Service-Name tags Ze puts on the wire.
		"Dockerfile.client":                           "eeac1673a6a64276d24806a863f3718c7c58c98644ba4c2688603014343a706d",
		"scenarios/01-pppoe-chap-ipv4/ze.conf":        "1b1427eb24d3cc599f02d7e285606a99d91ca64a7d9df1f222bb757e26d8f54b",
		"scenarios/01-pppoe-chap-ipv4/accel-ppp.conf": "c7b096b09feb5492123e4f32fff09a167abc30b798f5a6b22ae6e007f3d5fa05",
		"scenarios/01-pppoe-chap-ipv4/chap-secrets":   "04525c6958851189a53ea92d539f1dd5970eceff8e95d626eb7033438b912067",
		"scenarios/01-pppoe-chap-ipv4/role":           "a15ed3e38a6f9a28ca3acbe40026602dfff0c833b992f2937e4722520480f6fc",
		"scenarios/02-ze-ac-pppd-client/ze.conf":      "86018076f6fa758abb91a5108f39fda38ca78ddd6d0c15325f77ca630daa89e5",
		"scenarios/02-ze-ac-pppd-client/role":         "9390bb877bffd73137ca2201fb106d5b6096755e34a44c52946c8b658fd6100e",
		// The four scenarios added after the numbered pair. Their bytes were
		// unpinned, so this test's claim covered two scenarios while the suite
		// mounted six.
		"scenarios/ipv6cp-missing-option/ze.conf":    "47eaeb37cef6bae4b41710514e18c577e94bf684a827c0346b7569f5cfcd6fe4",
		"scenarios/ipv6cp-missing-option/role":       "9390bb877bffd73137ca2201fb106d5b6096755e34a44c52946c8b658fd6100e",
		"scenarios/ipv6cp-zero-identifier/ze.conf":   "cdd26c75e0a8a41bfe041e4cffea01134b0a16b105c939300c3b43f2123736d8",
		"scenarios/ipv6cp-zero-identifier/role":      "9390bb877bffd73137ca2201fb106d5b6096755e34a44c52946c8b658fd6100e",
		"scenarios/pppoe-empty-service-name/ze.conf": "45df553e263889bd953585a55638c05b8844b9ef7ac14456f8f873db26e5eddf",
		"scenarios/pppoe-empty-service-name/role":    "9390bb877bffd73137ca2201fb106d5b6096755e34a44c52946c8b658fd6100e",
		"scenarios/pppoe-padr-replay/ze.conf":        "a8cf774fccc14bac46919f33ab03db40cb05950d0bd0762ec169ea8c877a5887",
		"scenarios/pppoe-padr-replay/role":           "9390bb877bffd73137ca2201fb106d5b6096755e34a44c52946c8b658fd6100e",
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
