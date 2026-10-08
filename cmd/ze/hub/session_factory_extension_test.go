//go:build ze_ssh

package hub

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestBuildCommandTreeSurfacesLoaderError: when yang.DefaultLoader refuses
// the module set, the SSH session's buildCommandTree returns that error, so
// the session factory refuses the session with the cause instead of handing a
// nil loader to BuildCommandTree. Method: register a probe module whose
// `ze:hepl` names no declared extension, for this test only.
//
// VALIDATES: the SSH completion tree reports the loader error.
// PREVENTS: a nil-pointer panic in the daemon when an SSH session opens.
func TestBuildCommandTreeSurfacesLoaderError(t *testing.T) {
	t.Cleanup(yang.RegisterModuleForTest("ze-probe.yang", `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Probe leaf.";
    }
}`))

	tree, err := buildCommandTree()
	if !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Fatalf("buildCommandTree answered (%v, %v), want ErrUndeclaredExtension", tree, err)
	}
	if !strings.Contains(err.Error(), "ze:hepl") {
		t.Errorf("error %q does not name ze:hepl", err)
	}
}
