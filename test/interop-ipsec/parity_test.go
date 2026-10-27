package interopipsec_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab/ipsec"
)

// TestIPsecScenarioSelectorRefusesMissingOrUnimplemented exercises the runner
// with private scenario directories. A selector is exact, and a directory
// without a checker is an error rather than an empty successful run.
func TestIPsecScenarioSelectorRefusesMissingOrUnimplemented(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"psk-site-to-site", "unregistered-peer"} {
		if err := os.MkdirAll(filepath.Join(root, "test", "interop-ipsec", "scenarios", name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, selector := range []string{"missing-peer", "psk", "psk-site-to-site*", "unregistered-peer"} {
		t.Run(selector, func(t *testing.T) {
			report, code := ipsec.RunAt(t.Context(), root, selector)
			if code == 0 {
				t.Fatal("invalid scenario selector returned success")
			}
			if !strings.Contains(report.SetupError, selector) {
				t.Fatalf("refusal %q does not name selector %q", report.SetupError, selector)
			}
			want := "no scenario matching '" + selector + "' found"
			if selector == "unregistered-peer" {
				want = "scenario unregistered-peer has no Go checker"
			}
			if report.SetupError != want {
				t.Fatalf("selector refusal = %q, want %q", report.SetupError, want)
			}
			if len(report.Scenarios) != 0 {
				t.Fatalf("invalid selection ran scenarios: %v", report.Scenarios)
			}
		})
	}
}

// VALIDATES: Every typed checker retains both protocol endpoints' configuration,
// plus the extra inputs consumed by reload, PKI, and FRR scenarios.
// PREVENTS: Registering a checker whose independent peer or transition fixture was removed.
func TestEveryNativeScenarioHasCompleteInputs(t *testing.T) {
	extra := map[string][]string{
		"eap-tls":                         {"ze-env"},
		"eap-tls13":                       {"strongswan.conf", "pki/ca.pem", "pki/server.pem", "pki/server-key.pem"},
		"eap-nak-method-negotiation":      {"strongswan.conf"},
		"initiator-rekey-answer-narrows":  {"ze-env"},
		"ipsec-bgp-redistribute-frr":      {"frr.conf"},
		"mobike-initiator":                {"strongswan.conf"},
		"mobike-responder":                {"strongswan.conf"},
		"peer-reload-narrowing":           {"ze-narrowed.conf"},
		"real-nat-transport-ze-initiator": {"nat.conf"},
		"real-nat-transport-ze-responder": {"nat.conf"},
		"real-nat-tunnel-control":         {"nat.conf"},
		"responder-eap-mschapv2":          {"strongswan.conf"},
		"responder-eap-tls13":             {"strongswan.conf", "pki/ca.pem", "pki/server.pem", "pki/server-key.pem"},
	}
	for _, name := range ipsec.ScenarioNames() {
		required := append([]string{"ze.conf", "swanctl.conf"}, extra[name]...)
		for _, relative := range required {
			path := filepath.Join("scenarios", name, relative)
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				t.Errorf("%s missing input %s", name, relative)
			}
		}
		config, err := os.ReadFile(filepath.Join("scenarios", name, "ze.conf"))
		if err != nil {
			t.Errorf("%s unreadable ze.conf: %v", name, err)
			continue
		}
		if strings.Contains(string(config), "user interop") ||
			strings.Contains(string(config), "$2a$04$UlwuiuH82Unfsq") {
			t.Errorf("%s source config contains native renderer boilerplate", name)
		}
		// renderZeConfig writes the resolved config into the run's scratch work
		// directory, so a copy beside the source is a leftover dump. It carries
		// the expanded PKI material and the renderer boilerplate the assertion
		// above forbids, and a reader cannot tell it from an input.
		if _, err := os.Stat(filepath.Join("scenarios", name, "ze.conf.resolved")); err == nil {
			t.Errorf("%s holds a rendered ze.conf.resolved beside its source", name)
		}
	}
}

// VALIDATES: The native gate is the sole executable implementation in this tree.
// PREVENTS: Reintroducing source, bytecode, or an interpreter-backed fallback.
func TestIPsecTreeIsGoOnly(t *testing.T) {
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
	if ipsec.Action != "test integration/interop-ipsec" {
		t.Fatalf("native action = %q", ipsec.Action)
	}
}

// VALIDATES: charon's bypass-lan plugin is disabled in exactly one place, the
// lab-wide drop-in every strongSwan peer mounts.
// PREVENTS: a scenario reintroducing a private copy, which is two files setting
// one value with nothing to arbitrate a disagreement (`ai/rules/no-layering.md`).
func TestNoScenarioCarriesItsOwnBypassLanOverride(t *testing.T) {
	shared, err := os.ReadFile("strongswan-lab.conf")
	if err != nil {
		t.Fatalf("shared lab drop-in unreadable: %v", err)
	}
	if !strings.Contains(string(shared), "bypass-lan") {
		t.Fatal("strongswan-lab.conf no longer settles bypass-lan for the lab")
	}
	err = filepath.WalkDir("scenarios", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), "bypass-lan") {
			t.Errorf("%s carries its own bypass-lan setting; the lab settles it in strongswan-lab.conf", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
