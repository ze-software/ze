package peer

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestDeclaredShapesReachTheRegistry proves the declaration is wired: the
// command declares its shape in its own register path and the core registry
// answers for it, with no core package naming the command.
func TestDeclaredShapesReachTheRegistry(t *testing.T) {
	// `show bgp` carries its peer rows beside the aggregate keys, read against
	// a declared column order.
	if shape, declared := command.ShapeForCommand(cmdBgp); !declared || shape != command.ShapeTab {
		t.Errorf("%s = %v/%v, want tab/declared", cmdBgp, shape, declared)
	}
	if shape, declared := command.ShapeForCommand(cmdBgpPeerList); !declared || shape != command.ShapeTab {
		t.Errorf("%s = %v/%v, want tab/declared", cmdBgpPeerList, shape, declared)
	}

	// A branch under `show bgp` declares none, so it does not inherit `tab`
	// and get published as supporting row operators over an answer with no
	// rows in it.
	for _, child := range cmdBgpChildren {
		if _, declared := command.ShapeForCommand(child); declared {
			t.Errorf("%s inherits a shape; every child of show bgp must declare none", child)
		}
	}
}
