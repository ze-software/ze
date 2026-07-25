// Design: ai/rules/feature-gate-registration.md -- ze_vpp present (compile-out) validation
//
//go:build ze_vpp

package hub

// VALIDATES: with the ze_vpp build tag (a default ze / ze-appliance feature)
// the VPP dataplane is wired: the fib-vpp plugin is registered (the generated
// all_ze_vpp.go group file reached the composition root) and the iface "vpp"
// backend factory is registered (its LoadBackend failure, if any, is a factory
// / connector error, never "unknown backend").
// PREVENTS: a regression where ze_vpp is set but the VPP backends are not
// wired -- the group file dropped, or a backend registration import lost.

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_VPP_Present(t *testing.T) {
	if !pluginreg.Has("fib-vpp") {
		t.Error("ze_vpp build: fib-vpp plugin not registered")
	}
	// LoadBackend on failure keeps the previous backend active (iface
	// backend.go LoadBackend), so probing the registry through it is safe: in
	// a ze_vpp build the name resolves and any error is the factory failing
	// on the missing VPP connector, not the registry.
	if err := iface.LoadBackend("vpp"); err != nil {
		if strings.Contains(err.Error(), "unknown backend") {
			t.Errorf("ze_vpp build: iface vpp backend not registered: %v", err)
		}
	} else {
		// A live VPP in the test environment made the load succeed; drop it
		// so the test leaves no active backend behind.
		if err := iface.CloseBackend(); err != nil {
			t.Logf("close vpp backend: %v", err)
		}
	}
}
