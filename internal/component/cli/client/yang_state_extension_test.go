package client

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// probeHeplModule names `ze:hepl`, an extension ze-extensions does not
// declare, so yang.DefaultLoader refuses the module set while it is
// registered.
const probeHeplModule = `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Probe leaf.";
    }
}`

// resetYANGStateForTest drops the cached YANG state, so the next accessor
// builds it again over the modules registered at that moment.
func resetYANGStateForTest() {
	loadYANGState = sync.OnceValues(buildYANGState)
}

// TestYANGStateSurfacesLoaderError: when yang.DefaultLoader refuses the module
// set, every accessor of the package's YANG state returns that error, twice
// over (the cache keeps it), and IsDeclaredCommand never answers false in its
// place. Method: register a probe module whose `ze:hepl` names no declared
// extension, reset the once-cache so the state is built over it, call each
// accessor, then restore the shipped schema and reset the cache again.
//
// VALIDATES: the cli/client YANG state reports the loader error with its cause.
// PREVENTS: a nil-loader panic at package init that took down every binary
// importing this package, and a declared-command guard failing open.
func TestYANGStateSurfacesLoaderError(t *testing.T) {
	restore := yang.RegisterModuleForTest("ze-probe.yang", probeHeplModule)
	t.Cleanup(func() {
		restore()
		resetYANGStateForTest()
	})
	resetYANGStateForTest()

	accessors := []struct {
		name string
		call func() error
	}{
		{"WireToPath", func() error { _, err := WireToPath(); return err }},
		{"WireToPaths", func() error { _, err := WireToPaths(); return err }},
		{"YANGCommandTree", func() error { _, err := YANGCommandTree(); return err }},
		{"BuildCommandTree", func() error { _, err := BuildCommandTree(false); return err }},
		{"BuildVerbCommandTree", func() error { _, err := BuildVerbCommandTree("show"); return err }},
		{"AbsoluteVerbPath", func() error { _, _, err := AbsoluteVerbPath("show", []string{"version"}); return err }},
		{"IsDeclaredCommand", func() error {
			declared, err := IsDeclaredCommand("show version")
			if declared {
				t.Error("IsDeclaredCommand answered true over a refused schema")
			}
			return err
		}},
		{"buildRuntimeTreeFromDispatch", func() error {
			_, err := buildRuntimeTreeFromDispatch(commandListDispatch(""))
			return err
		}},
	}
	for attempt := range 2 {
		for _, accessor := range accessors {
			err := accessor.call()
			if !errors.Is(err, yang.ErrUndeclaredExtension) {
				t.Fatalf("attempt %d: %s answered %v, want ErrUndeclaredExtension", attempt, accessor.name, err)
			}
			if !strings.Contains(err.Error(), "ze:hepl") {
				t.Errorf("attempt %d: %s error %q does not name ze:hepl", attempt, accessor.name, err)
			}
		}
	}
}
