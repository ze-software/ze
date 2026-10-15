// Design: docs/architecture/testing/ci-format.md -- fixtures the le binary links
// Related: register_drain.go -- the drain fixtures, which share the fakedrain responder

package fixture

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// testPluginPackages is the import path prefix of the test-only plugins. Only
// a ze built with the zetest tag links them, through
// cmd/ze/plugins_zetest.go and internal/test/plugins/all.
const testPluginPackages = "github.com/ze-software/ze/internal/test/plugins/"

// VALIDATES: importing the fixture package registers no test-only plugin.
// PREVENTS: a test plugin landing in the le binary's registry, which links this
// package through internal/le/test/fixture, so `./le doc check verify` reports
// it as a shipped plugin missing from docs/DESIGN.md.
// TestFixtureRegistersNoTestPlugin walks every registration this package's
// imports produced and names the package of each handler, so a fixture that
// imports a test plugin's helpers from the package that registers it fails
// here, whatever the plugin is called.
func TestFixtureRegistersNoTestPlugin(t *testing.T) {
	for _, reg := range registry.All() {
		for _, handler := range []any{reg.RunEngine, reg.CLIHandler} {
			value := reflect.ValueOf(handler)
			if value.IsNil() {
				continue
			}
			function := runtime.FuncForPC(value.Pointer())
			if function == nil {
				t.Fatalf("plugin %q: handler has no symbol to name its package", reg.Name)
			}
			if strings.HasPrefix(function.Name(), testPluginPackages) {
				t.Errorf("plugin %q registered by %s: the fixture package imports a test plugin "+
					"that registers itself; import its registration-free helpers instead",
					reg.Name, function.Name())
			}
		}
	}
}
