//go:build ze_core && !ze_cos

// Design: docs/architecture/traffic/cos-plugin.md -- a build without CoS refuses profile configuration.

package cosabsent

import (
	"debug/buildinfo"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/env"
)

// Keep the runner's existing ZE_BIN contract without importing its build tools.
var _ = env.MustRegister(env.EnvEntry{Key: "ze.bin", Type: "string", Description: "Pre-built ze binary path for the test runner (absolute or repo-relative)"})

// TestConfigValidateWithoutCoSRefusesProfile drives the operator's command
// through the real composition with ze_cos omitted, not a cleared test registry.
// The schema can refuse the reference before resolution: either way this build
// must never report that it can apply the requested profile.
func TestConfigValidateWithoutCoSRefusesProfile(t *testing.T) {
	if registry.Has("cos") {
		t.Fatal("absent-build test linked the CoS provider")
	}
	path := filepath.Join(t.TempDir(), "profile.conf")
	const input = `interface {
 backend netlink;
 ethernet eth0 {
  class-of-service residential;
  unit v100 { vlan-id 100; }
 }
}`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	code, message := validateWithoutCoS(t, path)
	if code == 0 {
		t.Fatal("operator command accepted a profile without its provider")
	}
	if !strings.Contains(message, "class-of-service") {
		t.Fatalf("refusal did not identify the unavailable setting: %s", message)
	}
}

// TestConfigValidateWithoutCoSAcceptsInlineMaps proves the absent build still
// accepts interface-owned VLAN maps, through the same operator command.
func TestConfigValidateWithoutCoSAcceptsInlineMaps(t *testing.T) {
	if registry.Has("cos") {
		t.Fatal("absent-build test linked the CoS provider")
	}
	path := filepath.Join(t.TempDir(), "inline.conf")
	const input = `interface {
 backend netlink;
 ethernet eth0 {
  unit v100 {
   vlan-id 100;
   ingress-qos-map 3 { priority 3; }
   egress-qos-map 3 { pcp 3; }
  }
 }
}`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	code, message := validateWithoutCoS(t, path)
	if code != 0 {
		t.Fatalf("operator command refused inline maps without CoS: %s", message)
	}
}

// validateWithoutCoS executes the actual prebuilt command, after verifying that
// its build omits CoS. Missing or unusable artifacts fail rather than skip proof.
func validateWithoutCoS(t *testing.T, path string) (int, string) {
	t.Helper()
	binary := env.Get("ze.bin")
	if binary == "" {
		t.Fatal("set ZE_BIN to the ze_core binary built without ze_cos")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	var core, cos bool
	for _, setting := range info.Settings {
		if setting.Key != "-tags" {
			continue
		}
		for _, tag := range strings.FieldsFunc(setting.Value, func(r rune) bool { return r == ',' || r == ' ' }) {
			core = core || tag == "ze_core"
			cos = cos || tag == "ze_cos"
		}
	}
	if !core || cos {
		t.Fatalf("ZE_BIN must be built with ze_core and without ze_cos: core=%v cos=%v", core, cos)
	}
	data, err := exec.CommandContext(t.Context(), binary, "config", "validate", path).CombinedOutput()
	if err == nil {
		return 0, string(data)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("run ZE_BIN: %v", err)
	}
	return exit.ExitCode(), string(data)
}

// TestInlineQoSSchemaKeepsItsFourInterfaceKindsWithoutCoS checks the resolved
// schema rather than source text: moving the owner must neither lose a former
// interface kind nor add maps to every user of the shared unit grouping.
func TestInlineQoSSchemaKeepsItsFourInterfaceKindsWithoutCoS(t *testing.T) {
	schema, err := config.YANGSchemaWithPlugins(plugin.CollectPluginYANG(nil))
	if err != nil {
		t.Fatal(err)
	}
	node, err := schema.Lookup("interface")
	if err != nil {
		t.Fatal(err)
	}
	interfaces, ok := node.(*config.ContainerNode)
	if !ok {
		t.Fatal("interface root is not a container")
	}
	want := map[string]bool{"ethernet": true, "dummy": true, "veth": true, "bridge": true}
	seen := 0
	for _, kind := range interfaces.Children() {
		unit, err := schema.LookupDeclared("interface/" + kind + "/unit")
		if err != nil {
			continue
		}
		units, ok := unit.(*config.ListNode)
		if !ok {
			t.Fatalf("%s unit is not a list", kind)
		}
		for _, name := range []string{"ingress-qos-map", "egress-qos-map"} {
			if units.Has(name) != want[kind] {
				t.Errorf("%s/%s present=%v, want %v", kind, name, units.Has(name), want[kind])
			}
		}
		if want[kind] {
			seen++
		}
	}
	if seen != len(want) {
		t.Fatalf("schema exposes %d original inline-map interface kinds, want %d", seen, len(want))
	}
}
