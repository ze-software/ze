// VALIDATES: the static interface-only next-hop doctor check (D-2 (b)) fires only
// when a static route forwards over an interface-only next-hop AND no
// `interface { backend ... }` stanza is configured, and stays silent otherwise.
// PREVENTS: the runtime "no backend loaded" failure surfacing only at daemon
// start, and false warnings on configs that either declare a backend or use no
// interface-only next-hop.

package static

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
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

// TestCheckRouteSkippedFires
// VALIDATES: AC-3 -- the route-skipped doctor check fires when the running plugin
// has skipped a route, emits doctor-static-route-skipped, and names the skipped
// prefix and reason. PREVENTS: a skip being a silent no-op
// (ai/rules/fail-closed-guards.md).
func TestCheckRouteSkippedFires(t *testing.T) {
	rm := newRouteManager(&mockStaticBackend{})
	bad := netip.MustParsePrefix("203.0.113.0/24")
	rm.skipped[routeKey{prefix: bad}] = skippedRoute{
		route:  staticRoute{Prefix: bad, Action: actionForward},
		reason: "network unreachable",
	}
	activeRouteManager.Store(rm)
	t.Cleanup(func() { activeRouteManager.Store(nil) })

	diags := checkRouteSkipped(registry.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("diags: got %d, want 1", len(diags))
	}
	if diags[0].Code != doctorCodeRouteSkipped {
		t.Errorf("code: got %q, want %q", diags[0].Code, doctorCodeRouteSkipped)
	}
	if diags[0].Severity != "warning" {
		t.Errorf("severity: got %q, want warning", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, "203.0.113.0/24") {
		t.Errorf("message must name the skipped prefix, got %q", diags[0].Message)
	}
	if !strings.Contains(diags[0].Message, "network unreachable") {
		t.Errorf("message must name the skip reason, got %q", diags[0].Message)
	}
}

// TestCheckRouteSkippedSilentWhenNone
// VALIDATES: AC-3 -- the check stays silent when no route is skipped, so it does
// not raise a false doctor warning during normal operation.
func TestCheckRouteSkippedSilentWhenNone(t *testing.T) {
	rm := newRouteManager(&mockStaticBackend{})
	activeRouteManager.Store(rm)
	t.Cleanup(func() { activeRouteManager.Store(nil) })

	if diags := checkRouteSkipped(registry.DoctorCheckContext{}); len(diags) != 0 {
		t.Errorf("no skipped routes: got %d diags, want 0", len(diags))
	}
}

// TestCheckRouteSkippedNoRunningManager
// VALIDATES: the offline `ze doctor <config>` path (no running daemon, no route
// manager published) is silent -- there is no runtime skip state to report.
func TestCheckRouteSkippedNoRunningManager(t *testing.T) {
	activeRouteManager.Store(nil)
	if diags := checkRouteSkipped(registry.DoctorCheckContext{}); len(diags) != 0 {
		t.Errorf("no running manager: got %d diags, want 0", len(diags))
	}
}
