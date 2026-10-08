package aihelp

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

// TestCLISubcommandsSurfacesLoaderError: when yang.DefaultLoader refuses the
// module set, CLISubcommands returns that error instead of a list that lacks
// every YANG verb. Method: register the probe for this test only.
//
// VALIDATES: `ze help ai` reports the loader error.
// PREVENTS: an AI reference that silently drops the YANG verb tree.
func TestCLISubcommandsSurfacesLoaderError(t *testing.T) {
	t.Cleanup(yang.RegisterModuleForTest("ze-probe.yang", misspelledExtensionProbe))

	commands, err := CLISubcommands()
	if !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Fatalf("CLISubcommands answered (%d commands, %v), want ErrUndeclaredExtension", len(commands), err)
	}
	if !strings.Contains(err.Error(), "ze:hepl") {
		t.Errorf("error %q does not name ze:hepl", err)
	}
}

// TestBuildSurfacesLoaderError: when yang.DefaultLoader refuses the module
// set, Build returns that error instead of a reference whose dispatch keys
// are an empty map. Method: register the probe for this test only.
//
// VALIDATES: `ze help ai --json` and the MCP ze_reference tool report the
// loader error.
// PREVENTS: a reference published with no dispatch keys and no cause.
func TestBuildSurfacesLoaderError(t *testing.T) {
	t.Cleanup(yang.RegisterModuleForTest("ze-probe.yang", misspelledExtensionProbe))

	ref, err := Build()
	if !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Fatalf("Build answered (%d dispatch keys, %v), want ErrUndeclaredExtension", len(ref.DispatchKeys), err)
	}
	if !strings.Contains(err.Error(), "ze:hepl") {
		t.Errorf("error %q does not name ze:hepl", err)
	}
}
