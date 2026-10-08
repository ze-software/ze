package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestBuildEditorCommandTreeSurfacesLoaderError: when yang.DefaultLoader
// refuses the module set, buildEditorCommandTree returns that error, so
// `ze config edit` reports the extension typo instead of handing a nil loader
// to BuildCommandTree. Method: register a probe module whose `ze:hepl` names
// no declared extension, for this test only.
//
// VALIDATES: the editor's completion tree reports the loader error.
// PREVENTS: a nil-pointer panic when the editor starts.
func TestBuildEditorCommandTreeSurfacesLoaderError(t *testing.T) {
	t.Cleanup(yang.RegisterModuleForTest("ze-probe.yang", `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Probe leaf.";
    }
}`))

	tree, err := buildEditorCommandTree()
	if !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Fatalf("buildEditorCommandTree answered (%v, %v), want ErrUndeclaredExtension", tree, err)
	}
	if !strings.Contains(err.Error(), "ze:hepl") {
		t.Errorf("error %q does not name ze:hepl", err)
	}
}
