// Design: docs/architecture/cli/plugin-modes.md — ze_setup build tag validation
//
//go:build ze_setup && !ze_core && !ze_distro && !ze_appliance

package main

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func TestZeSetupStdinDispatchWired(t *testing.T) {
	if binaryDispatch == nil {
		t.Fatal("binaryDispatch not set in ze_setup build; forked 'ze-setup -' will fail")
	}
}

func TestZeSetupBinaryCommands(t *testing.T) {
	roots := registry.ListRoot()
	seen := make(map[string]bool, len(roots))
	for _, root := range roots {
		seen[root.Name] = true
	}

	if !seen["install"] {
		t.Fatal("root \"install\" missing in ze_setup build")
	}
	if !seen["appliance"] {
		t.Fatal("root \"appliance\" missing in ze_setup build")
	}
	if seen["uninstall"] {
		t.Fatal("root \"uninstall\" unexpectedly registered in ze_setup-only build")
	}
}
