// Design: ai/rules/plugins.md -- ze_container absent (compile-out) validation
//
//go:build !ze_container

package hub

// VALIDATES: without the ze_container build tag the container plugin is not
// registered and its config root is unknown, with a refusal that names the
// build tag, while the rest of the plugin registry is still populated. The
// binary symbol-drop proof (the plugin, the vendored runtime library and the
// control group module) is in build_tag_gate12_absent_test.go, which builds
// one bare binary for every gate.
// PREVENTS: a regression where the container runtime leaks into a hardened
// build through an always-on import, such as the early-role leaf importing
// the plugin in place of the plugin registering its role.

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_Container_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate container absence (all.go not linked)")
	}
	if pluginreg.Has("container") {
		t.Fatal("non-ze_container build: container plugin unexpectedly registered (not compiled out)")
	}
}

// TestBuildTag_Container_AbsentRejectsContainerConfig proves the container
// schema is gone with the plugin, and that the refusal tells the operator
// which build carries it: the keyword is unknown, and the hint names the tag.
func TestBuildTag_Container_AbsentRejectsContainerConfig(t *testing.T) {
	const cfg = `container {
	instance web {
		enabled false;
	}
}
`
	_, err := zeconfig.ParseTreeWithYANG(cfg, nil)
	if err == nil {
		t.Fatal("non-ze_container build unexpectedly accepted a container config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("container config rejection = %v, want a clean unknown-keyword rejection", err)
	}
	if !strings.Contains(err.Error(), "ze_container") {
		t.Fatalf("container config rejection = %v, want it to name the build tag ze_container", err)
	}
}
