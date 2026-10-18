package interop_l2tp_test

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
	"github.com/ze-software/ze/internal/le/interoplab/l2tp"
)

var _ func(context.Context, string, string) interoplab.SuiteReport = l2tp.RunAt

var l2tpScenarios = []string{
	"01-ppp-ipv4",
	"02-ppp-bgp-redistribute-frr",
	"03-ze-lac-xl2tpd-lns",
	"04-radius-acct-attrs",
}

// VALIDATES: The native selector exposes the complete reviewed L2TP scenario
// population in lexical order.
// PREVENTS: Dropping either inverse-role or three-peer scenario during cutover.
func TestNativeScenarioPopulationIsExact(t *testing.T) {
	if got := l2tp.ScenarioNames(); !reflect.DeepEqual(got, l2tpScenarios) {
		t.Fatalf("native scenarios = %v, want %v", got, l2tpScenarios)
	}
	entries, err := os.ReadDir("scenarios")
	if err != nil {
		t.Fatalf("read scenarios: %v", err)
	}
	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, entry.Name())
		}
	}
	if !reflect.DeepEqual(directories, l2tpScenarios) {
		t.Fatalf("fixture directories = %v, want %v", directories, l2tpScenarios)
	}
}

// VALIDATES: Every byte mounted by the native peer plans remains the reviewed
// protocol fixture.
// PREVENTS: A config, PPP option, secret, or FRR fixture drifting during runner removal.
func TestNativeConfigBytesArePinned(t *testing.T) {
	files := map[string]string{
		// Repinned for 5837fd3247, which removed the Go toolchain from this
		// image: the LNS image is now an alpine:3.21 base plus one apk add plus
		// a COPY of the binary StageBinaries cross-compiles
		// (internal/le/interoplab/zebuild.go). The change was reviewed with that
		// commit and the pin was not carried with it.
		// Repinned for 4a923c72de: the header comment names `./le test integration`.
		// Repinned for 6948695bdc, which dropped kmod from Dockerfile.ze and
		// Dockerfile.lac: no lab container loads a kernel module.
		// Repinned for the 2026-10-17 nightly fix: the 03-ze-lac-xl2tpd-lns
		// ze.conf L2TP listener binds 0.0.0.0 instead of 127.0.0.1, which
		// cannot send to the LNS on the lab bridge
		// (TestZeL2TPListenerReachesTheLabBridge).
		// Repinned for b3a5960dc7: the 02-ppp-bgp-redistribute-frr frr.conf
		// sets no bgp ebgp-requires-policy, because FRR 10.3.1 refused Ze's
		// UPDATE with "inbound policy missing" and the scenario waited forever.
		"Dockerfile.ze":                                      "d18dcba6226a09fb874cee777af6566f92bdb5723f82a91568951b8421cd2213",
		"Dockerfile.lac":                                     "67007f9802268dbc1762c754fb7187abafca0408314c3e2d9d7252abd36e00f8",
		"daemons":                                            "6c0f1be1b722ff89041b5bea87ed1212dd0595019b44730616de9e338cb08cd0",
		"vtysh.conf":                                         "dc8aa539965a4cebabbe1a75a53b48ae8471bafe77ce59af22d5352d28da4df6",
		"scenarios/01-ppp-ipv4/ze.conf":                      "1025ac71136323782a35a27cf6c2278d668ad9e597e6fa610e943bed39d97b33",
		"scenarios/01-ppp-ipv4/xl2tpd.conf":                  "37c2eb77bfdb632f4c03ef5ba06a71b3ddb8200369e30395d6bf8b5d738a3cae",
		"scenarios/01-ppp-ipv4/ppp-options":                  "f09bbbc2e509408723541c5009d4819b9d01cc9d81b4f0961117c31f53314004",
		"scenarios/01-ppp-ipv4/l2tp-secrets":                 "6c08c01fa683780b644f441f8d7f4191d64feaf05a035fe4fe79f63e7365b7e5",
		"scenarios/02-ppp-bgp-redistribute-frr/ze.conf":      "439e7a53836e4ef5a0e4994897fb9fcbebb7471da1f5785b9563e679628d60b9",
		"scenarios/02-ppp-bgp-redistribute-frr/xl2tpd.conf":  "37c2eb77bfdb632f4c03ef5ba06a71b3ddb8200369e30395d6bf8b5d738a3cae",
		"scenarios/02-ppp-bgp-redistribute-frr/ppp-options":  "f09bbbc2e509408723541c5009d4819b9d01cc9d81b4f0961117c31f53314004",
		"scenarios/02-ppp-bgp-redistribute-frr/l2tp-secrets": "6c08c01fa683780b644f441f8d7f4191d64feaf05a035fe4fe79f63e7365b7e5",
		"scenarios/02-ppp-bgp-redistribute-frr/frr.conf":     "f5deb33e635976334915d703188566d39535de16acf38677c5affced7051b251",
		"scenarios/03-ze-lac-xl2tpd-lns/ze.conf":             "f9e55699a9b087c079b518d5fab2897590a4e10cbbfc071d4a77e9fbbeda9411",
		"scenarios/03-ze-lac-xl2tpd-lns/xl2tpd.conf":         "bdd8acec47050dd362774130bccc6f17a97d9956b514143cd93e827b716bf013",
		"scenarios/03-ze-lac-xl2tpd-lns/options.xl2tpd":      "24bd437d8d4bd6d2916236a7036d01d4f990ead0951763b18347c068c809536f",
		"scenarios/04-radius-acct-attrs/ze.conf":             "a834e28c29c8e80e1d1c135bc6f5f3f02f4a39c33b21914bc3e5af60489e15ae",
		"scenarios/04-radius-acct-attrs/xl2tpd.conf":         "37c2eb77bfdb632f4c03ef5ba06a71b3ddb8200369e30395d6bf8b5d738a3cae",
		"scenarios/04-radius-acct-attrs/ppp-options":         "f09bbbc2e509408723541c5009d4819b9d01cc9d81b4f0961117c31f53314004",
		"scenarios/04-radius-acct-attrs/l2tp-secrets":        "6c08c01fa683780b644f441f8d7f4191d64feaf05a035fe4fe79f63e7365b7e5",
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

// VALIDATES: The L2TP fixture tree has no interpreter-backed source or cache.
// PREVENTS: Reintroducing an executable fallback beside the typed plans.
func TestL2TPTreeIsGoOnly(t *testing.T) {
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
