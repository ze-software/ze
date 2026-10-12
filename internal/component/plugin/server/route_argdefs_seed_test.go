// Design: docs/architecture/api/commands.md -- the argument definitions every dispatch route judges

package server

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// undeclaredExtensionProbe is a module the loader refuses: it uses a ze:
// extension the extensions module does not declare.
const undeclaredExtensionProbe = `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Probe leaf.";
    }
}`

// TestNewServerSeedsRouteArgumentDefinitions proves NewServer hands the
// argument definitions it built to every dispatch route, so the first plugin
// command does not load and resolve the model a second time. That load cost
// about 70 ms and delayed `request bgp rib fastpath enable` past the first
// UPDATE it was sent to precede (plugin test bgp-rs-fastpath).
//
// Method: empty the definitions, build a server, then register a module the
// loader refuses. A route that still had to load the model answers that
// refusal; a route reading the seeded definitions judges the call.
func TestNewServerSeedsRouteArgumentDefinitions(t *testing.T) {
	t.Cleanup(yang.ResetCommandArgDefsForTest())

	if _, err := NewServer(&ServerConfig{}, nil); err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(yang.RegisterModuleForTest("ze-probe.yang", undeclaredExtensionProbe))

	if _, err := command.ValidateModelArgs("request bgp rib fastpath", []string{"enable"}, nil); err != nil {
		t.Fatalf("route R9 judged the first plugin command by loading the model again: %v", err)
	}
}
