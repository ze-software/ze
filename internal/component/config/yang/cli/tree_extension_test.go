package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// misspelledExtensionProbe is a module whose `ze:hepl` names no extension
// ze-extensions declares, so yang.DefaultLoader refuses the module set.
const misspelledExtensionProbe = `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Probe leaf.";
    }
}`

// TestAddCommandNodesSurfacesLoaderError: when yang.DefaultLoader refuses the
// module set, addCommandNodes returns that error and does not hand a nil
// loader to BuildCommandTree. Method: register a probe with a misspelled
// extension for this test only, then call addCommandNodes.
//
// VALIDATES: the command half of the unified tree reports the loader error.
// PREVENTS: a nil-pointer panic in `ze yang` on an extension typo.
func TestAddCommandNodesSurfacesLoaderError(t *testing.T) {
	t.Cleanup(yang.RegisterModuleForTest("ze-probe.yang", misspelledExtensionProbe))

	root := &AnalysisNode{Name: "(root)", Children: make(map[string]*AnalysisNode)}
	err := addCommandNodes(root)
	if !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Fatalf("addCommandNodes answered %v, want ErrUndeclaredExtension", err)
	}
	if !strings.Contains(err.Error(), "ze:hepl") {
		t.Errorf("error %q does not name ze:hepl", err)
	}
}
