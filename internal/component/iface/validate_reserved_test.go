package iface

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestValidateIfaceNameSurfacesLoaderError: when yang.DefaultLoader refuses
// the module set, ValidateIfaceName returns that error instead of checking the
// name against an empty reserved set. Method: reset the once-loaded reserved
// set, register a probe module whose `ze:hepl` names no declared extension
// for this test only, and validate an ordinary name.
//
// VALIDATES: the reserved-keyword check reports the loader error.
// PREVENTS: every reserved CLI keyword accepted as an interface name, silently.
func TestValidateIfaceNameSurfacesLoaderError(t *testing.T) {
	resetReserved := func() {
		reservedIfaceNames = map[string]string{}
		reservedIfaceNamesOnce = sync.Once{}
		reservedIfaceNamesErr = nil
	}
	resetReserved()
	t.Cleanup(resetReserved)
	t.Cleanup(yang.RegisterModuleForTest("ze-probe.yang", `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Probe leaf.";
    }
}`))

	err := ValidateIfaceName("eth0")
	if !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Fatalf("ValidateIfaceName answered %v, want ErrUndeclaredExtension", err)
	}
	if !strings.Contains(err.Error(), "ze:hepl") {
		t.Errorf("error %q does not name ze:hepl", err)
	}
}
