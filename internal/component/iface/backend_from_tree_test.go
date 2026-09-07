// VALIDATES: BackendNameFromTree answers the same data-plane name
// parseIfaceBackend gives the iface plugin, from the lowered config map and
// before any plugin handshake runs.
// PREVENTS: the engine resolving a route producer's FIB writer against a
// different backend from the one the iface component then loads.

package iface

import "testing"

// TestBackendNameFromTreeReadsTheConfiguredBackend proves an operator's
// `interface { backend vpp }` reaches the reader that decides which FIB plugin
// programs their routes.
func TestBackendNameFromTreeReadsTheConfiguredBackend(t *testing.T) {
	tree := map[string]any{
		"interface": map[string]any{"backend": "vpp"},
	}
	if got := BackendNameFromTree(tree); got != "vpp" {
		t.Fatalf("BackendNameFromTree = %q, want vpp", got)
	}
}

// TestBackendNameFromTreeFallsBackToTheBuildDefault proves a config that names
// no backend answers with the same default parseIfaceBackend uses, so a config
// with no `interface { }` block at all still resolves a data plane.
func TestBackendNameFromTreeFallsBackToTheBuildDefault(t *testing.T) {
	cases := map[string]map[string]any{
		"no interface block":  {},
		"no backend leaf":     {"interface": map[string]any{"dhcp-auto": "true"}},
		"empty backend leaf":  {"interface": map[string]any{"backend": ""}},
		"interface not a map": {"interface": "netlink"},
	}
	for name, tree := range cases {
		t.Run(name, func(t *testing.T) {
			if got := BackendNameFromTree(tree); got != DefaultBackendName() {
				t.Fatalf("BackendNameFromTree = %q, want the build default %q", got, DefaultBackendName())
			}
		})
	}
}
