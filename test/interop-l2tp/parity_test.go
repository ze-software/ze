package interop_l2tp_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/l2tp"
)

var _ func(context.Context, string, string) interoplab.SuiteReport = l2tp.RunAt

// VALIDATES: the native adapter is compiled beside the complete Python scenario corpus and the selector names remain exact.
// PREVENTS: translating only check.py scenarios while dropping either self-contained run.py branch.
func TestNativeScenarioNameParity(t *testing.T) {
	entries, err := os.ReadDir("scenarios")
	if err != nil {
		t.Fatalf("read scenarios: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	want := []string{"01-ppp-ipv4", "02-ppp-bgp-redistribute-frr", "03-ze-lac-xl2tpd-lns", "04-radius-acct-attrs"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("producer selectors = %v, want %v", names, want)
	}
}

// VALIDATES: every byte mounted by the native peer plans remains the reviewed producer input.
// PREVENTS: a config, PPP option, secret, FRR fixture, or RADIUS helper drifting outside the Go parity review.
func TestProducerConfigByteParity(t *testing.T) {
	files := map[string]string{
		"run.py":                             "d8829ef596658958179503dedc3c213d132460fa324d8b40ae92f7fb29373e00",
		"lab.py":                             "58c0db83a9a0255459d310152f2b24ecd1757075d715d58445b6646efada03ca",
		"Dockerfile.ze":                      "b213af5cdf02b01be9cf8e995dd1def63cc7574ac44dbf1323f6ac801fbcfff0",
		"Dockerfile.lac":                     "de2f22de8c1815e8d0d6a37b96d7518d8157b26cd5598fb95c8e4f624909a180",
		"daemons":                            "6c0f1be1b722ff89041b5bea87ed1212dd0595019b44730616de9e338cb08cd0",
		"vtysh.conf":                         "dc8aa539965a4cebabbe1a75a53b48ae8471bafe77ce59af22d5352d28da4df6",
		"scenarios/01-ppp-ipv4/ze.conf":      "1025ac71136323782a35a27cf6c2278d668ad9e597e6fa610e943bed39d97b33",
		"scenarios/01-ppp-ipv4/xl2tpd.conf":  "37c2eb77bfdb632f4c03ef5ba06a71b3ddb8200369e30395d6bf8b5d738a3cae",
		"scenarios/01-ppp-ipv4/ppp-options":  "f09bbbc2e509408723541c5009d4819b9d01cc9d81b4f0961117c31f53314004",
		"scenarios/01-ppp-ipv4/l2tp-secrets": "6c08c01fa683780b644f441f8d7f4191d64feaf05a035fe4fe79f63e7365b7e5",
		"scenarios/01-ppp-ipv4/check.py":     "2d739a3190b179e60dd08e6253568c76536d6d79e7c81348c54c1e3b3bd5ef1d",
		"scenarios/02-ppp-bgp-redistribute-frr/ze.conf":      "439e7a53836e4ef5a0e4994897fb9fcbebb7471da1f5785b9563e679628d60b9",
		"scenarios/02-ppp-bgp-redistribute-frr/xl2tpd.conf":  "37c2eb77bfdb632f4c03ef5ba06a71b3ddb8200369e30395d6bf8b5d738a3cae",
		"scenarios/02-ppp-bgp-redistribute-frr/ppp-options":  "f09bbbc2e509408723541c5009d4819b9d01cc9d81b4f0961117c31f53314004",
		"scenarios/02-ppp-bgp-redistribute-frr/l2tp-secrets": "6c08c01fa683780b644f441f8d7f4191d64feaf05a035fe4fe79f63e7365b7e5",
		"scenarios/02-ppp-bgp-redistribute-frr/check.py":     "1bc061a5c362b5dc9dc241f1439e939b92ab3f991ae82245b20904fa6769c01e",
		"scenarios/02-ppp-bgp-redistribute-frr/frr.conf":     "638a8bbc479cf1ddf922bd3d47b5d2550b28b56fda749afe5d5dcdb88b276d99",
		"scenarios/03-ze-lac-xl2tpd-lns/ze.conf":             "45d808a81f7c7d65092b50d7b03a5f68515a60fd320cd6dcc196ea499bdbb107",
		"scenarios/03-ze-lac-xl2tpd-lns/xl2tpd.conf":         "bdd8acec47050dd362774130bccc6f17a97d9956b514143cd93e827b716bf013",
		"scenarios/03-ze-lac-xl2tpd-lns/options.xl2tpd":      "24bd437d8d4bd6d2916236a7036d01d4f990ead0951763b18347c068c809536f",
		"scenarios/03-ze-lac-xl2tpd-lns/run.py":              "cbc3021bd6450ebcb312f858dabf5df6ca2a333437233dfd7b0cc3528ff7f038",
		"scenarios/04-radius-acct-attrs/ze.conf":             "5773b223fd57351ed46ea079e8a39e0157634238914dd90527ff0d2d32d1db39",
		"scenarios/04-radius-acct-attrs/xl2tpd.conf":         "37c2eb77bfdb632f4c03ef5ba06a71b3ddb8200369e30395d6bf8b5d738a3cae",
		"scenarios/04-radius-acct-attrs/ppp-options":         "f09bbbc2e509408723541c5009d4819b9d01cc9d81b4f0961117c31f53314004",
		"scenarios/04-radius-acct-attrs/l2tp-secrets":        "6c08c01fa683780b644f441f8d7f4191d64feaf05a035fe4fe79f63e7365b7e5",
		"scenarios/04-radius-acct-attrs/run.py":              "e9a49c8ac24573ed70cf7a7e0e210d0f8b6e9be2585edbfbc1e66b14deeac1fb",
		"scenarios/04-radius-acct-attrs/radius-mock.py":      "4efb2d5078dc9abb270fcd73b6afe858ab3da353607ba62ddf0725d9da8a9ce5",
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
