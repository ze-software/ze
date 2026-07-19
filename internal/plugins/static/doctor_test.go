// VALIDATES: the static interface-only next-hop doctor check (D-2 (b)) fires only
// when a static route forwards over an interface-only next-hop AND no
// `interface { backend ... }` stanza is configured, and stays silent otherwise.
// PREVENTS: the runtime "no backend loaded" failure surfacing only at daemon
// start, and false warnings on configs that either declare a backend or use no
// interface-only next-hop.

package static

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// treeWithInterfaceNexthop builds a config tree with one static route that
// forwards over an interface-only next-hop. When backend != "" it also adds an
// `interface { backend <backend> }` stanza.
func treeWithInterfaceNexthop(backend string) *config.Tree {
	tree := config.NewTree()
	static := tree.GetOrCreateContainer("static")

	route := config.NewTree()
	next := route.GetOrCreateContainer("next")
	ifEntry := config.NewTree()
	ifEntry.Set("name", "tun100")
	next.AddListEntry("interface", "tun100", ifEntry)

	table := config.NewTree()
	table.AddListEntry("route", "0.0.0.0/0", route)
	static.AddListEntry("table", "lns", table)

	if backend != "" {
		iface := tree.GetOrCreateContainer("interface")
		iface.Set("backend", backend)
	}
	return tree
}

func TestCheckInterfaceNexthopBackendMissing(t *testing.T) {
	diags := checkInterfaceNexthopBackend(registry.DoctorCheckContext{Tree: treeWithInterfaceNexthop("")})
	if len(diags) != 1 {
		t.Fatalf("diags: got %d, want 1", len(diags))
	}
	if diags[0].Code != doctorCodeInterfaceNexthopNoBackend {
		t.Errorf("code: got %q, want %q", diags[0].Code, doctorCodeInterfaceNexthopNoBackend)
	}
	if diags[0].Severity != "warning" {
		t.Errorf("severity: got %q, want warning", diags[0].Severity)
	}
}

func TestCheckInterfaceNexthopBackendPresent(t *testing.T) {
	diags := checkInterfaceNexthopBackend(registry.DoctorCheckContext{Tree: treeWithInterfaceNexthop("netlink")})
	if len(diags) != 0 {
		t.Errorf("diags: got %d, want 0 (a backend is configured)", len(diags))
	}
}

func TestCheckInterfaceNexthopBackendNoInterfaceNexthop(t *testing.T) {
	// A static route with only an address next-hop must not trip the check.
	tree := config.NewTree()
	static := tree.GetOrCreateContainer("static")
	route := config.NewTree()
	next := route.GetOrCreateContainer("next")
	hop := config.NewTree()
	hop.Set("address", "192.168.1.1")
	next.AddListEntry("hop", "192.168.1.1", hop)
	table := config.NewTree()
	table.AddListEntry("route", "10.0.0.0/24", route)
	static.AddListEntry("table", "default", table)

	if diags := checkInterfaceNexthopBackend(registry.DoctorCheckContext{Tree: tree}); len(diags) != 0 {
		t.Errorf("diags: got %d, want 0 (no interface-only next-hop)", len(diags))
	}
}

func TestCheckInterfaceNexthopBackendNilOrEmpty(t *testing.T) {
	if diags := checkInterfaceNexthopBackend(registry.DoctorCheckContext{Tree: nil}); len(diags) != 0 {
		t.Errorf("nil tree: got %d diags, want 0", len(diags))
	}
	if diags := checkInterfaceNexthopBackend(registry.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Errorf("empty tree: got %d diags, want 0", len(diags))
	}
}
