package interopipsec_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab/ipsec"
)

var ipsecScenarios = []string{
	"child-rekey",
	"child-rekey-narrowing",
	"clear-reestablish",
	"cookie-challenge",
	"delete-while-window-held",
	"eap-mschapv2",
	"eap-tls",
	"eap-tls13",
	"esn-both-offered",
	"esn-extended-only-refused",
	"esp-form-change",
	"initiator-rekey-answer-narrows",
	"invalid-ke-retry",
	"ipsec-bgp-redistribute-frr",
	"peer-reload-narrowing",
	"psk-site-to-site",
	"responder-accepts-reinit",
	"responder-eap-mschapv2",
	"responder-eap-tls13",
	"responder-ike-rekey",
	"responder-psk",
	"responder-raises-child-rekey",
}

// VALIDATES: The native registry exposes the complete reviewed IPsec scenario
// population in lexical selection order.
// PREVENTS: A Go-only gate silently dropping a scenario after its old runner is gone.
func TestNativeScenarioRegistryIsExact(t *testing.T) {
	if got := ipsec.ScenarioNames(); !reflect.DeepEqual(got, ipsecScenarios) {
		t.Fatalf("native scenarios = %v, want %v", got, ipsecScenarios)
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
	if !reflect.DeepEqual(directories, ipsecScenarios) {
		t.Fatalf("fixture directories = %v, want %v", directories, ipsecScenarios)
	}
}

// VALIDATES: Every typed checker retains both protocol endpoints' configuration,
// plus the extra inputs consumed by reload, PKI, and FRR scenarios.
// PREVENTS: Registering a checker whose independent peer or transition fixture was removed.
func TestEveryNativeScenarioHasCompleteInputs(t *testing.T) {
	extra := map[string][]string{
		"eap-tls":                        {"ze-env"},
		"eap-tls13":                      {"strongswan.conf", "pki/ca.pem", "pki/server.pem", "pki/server-key.pem"},
		"initiator-rekey-answer-narrows": {"ze-env"},
		"ipsec-bgp-redistribute-frr":     {"frr.conf"},
		"peer-reload-narrowing":          {"ze-narrowed.conf"},
		"responder-eap-mschapv2":         {"strongswan.conf"},
		"responder-eap-tls13":            {"strongswan.conf", "pki/ca.pem", "pki/server.pem", "pki/server-key.pem"},
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
	if ipsec.Action != "integration/interop-ipsec" {
		t.Fatalf("native action = %q", ipsec.Action)
	}
}
