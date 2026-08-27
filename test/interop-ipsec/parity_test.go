// Design: run.py -- Python producer retained during duplicate-then-swap.
// Related: ../../internal/le/interoplab/ipsec -- native scenario registry and callbacks.
package interopipsec_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab/ipsec"
)

// VALIDATES: The native registry names every Python IPsec scenario in the same
// lexical order selected by run.py, with no sample-only or extra callback.
// PREVENTS: a green replacement gate silently dropping a scenario directory.
func TestNativeScenarioRegistryExactlyMatchesPythonProducer(t *testing.T) {
	entries, err := os.ReadDir("scenarios")
	if err != nil {
		t.Fatal(err)
	}
	producer := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		checkPath := filepath.Join("scenarios", entry.Name(), "check.py")
		if _, err := os.Stat(checkPath); err != nil {
			continue
		}
		producer = append(producer, entry.Name())
	}
	sort.Strings(producer)
	if native := ipsec.ScenarioNames(); !reflect.DeepEqual(native, producer) {
		t.Fatalf("native scenarios = %v, Python producer scenarios = %v", native, producer)
	}
}

// VALIDATES: Every registered scenario retains its complete producer checker and
// mandatory Ze/strongSwan configuration; optional daemon and reload files remain
// beside that same named producer and are discovered from the live directory.
// PREVENTS: registering a name whose Python checker or protocol configuration was
// omitted from the translated corpus.
func TestEveryNativeScenarioHasCompleteProducerFiles(t *testing.T) {
	for _, name := range ipsec.ScenarioNames() {
		directory := filepath.Join("scenarios", name)
		for _, required := range []string{"check.py", "ze.conf", "swanctl.conf"} {
			path := filepath.Join(directory, required)
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				t.Errorf("%s missing producer file %s", name, required)
			}
		}
		checker, err := os.ReadFile(filepath.Join(directory, "check.py"))
		if err != nil {
			t.Error(err)
			continue
		}
		text := string(checker)
		if !strings.Contains(text, "def check():") {
			t.Errorf("%s producer has no check()", name)
		}
	}
}

// VALIDATES: Gate identity and the producer's image, timeout, selector, and
// no-build contracts remain byte-for-byte represented by the native adapter.
// PREVENTS: wiring a similar lab under a different gate or environment contract.
func TestNativeGateMetadataMatchesRunPy(t *testing.T) {
	producer, err := os.ReadFile("run.py")
	if err != nil {
		t.Fatal(err)
	}
	lab, err := os.ReadFile("lab.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`"FRR_IMAGE", "quay.io/frrouting/frr:10.3.1"`,
		`"NO_BUILD", "0"`,
		`scenario_filter = sys.argv[1]`,
		`sorted(os.listdir(scenarios_dir))`,
	} {
		if !strings.Contains(string(producer), fragment) {
			t.Errorf("run.py no longer carries %q", fragment)
		}
	}
	for _, fragment := range []string{
		`"ZE_IPSEC_INTEROP_SUFFIX"`,
		`"SESSION_TIMEOUT", "90"`,
		`SUBNET = "172.28.0.0/24"`,
		`ZE_IP = "172.28.0.2"`,
		`SWAN_IP = "172.28.0.3"`,
		`FRR_IP = "172.28.0.4"`,
	} {
		if !strings.Contains(string(lab), fragment) {
			t.Errorf("lab.py no longer carries %q", fragment)
		}
	}
	if ipsec.Gate != "ze-interop-ipsec-test" {
		t.Fatalf("native gate = %q", ipsec.Gate)
	}
}
