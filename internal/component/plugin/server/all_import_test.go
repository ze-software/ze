package server_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// Trigger all plugin init() registrations for integration tests.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// buildTestWireToPath creates a WireMethod->path map from the shared YANG loader.
func buildTestWireToPath() map[string]string {
	loader, _ := yang.DefaultLoader()
	return yang.WireMethodToPath(loader)
}

// TestEveryRPCHasYANGPath verifies every non-editor RPC has a YANG-derived CLI path.
//
// VALIDATES: All builtin RPCs (except editor-internal) have YANG path mappings.
// PREVENTS: RPCs registered without YANG schema, invisible to CLI dispatch and authz.
func TestEveryRPCHasYANGPath(t *testing.T) {
	wireToPath := buildTestWireToPath()

	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if strings.HasPrefix(reg.WireMethod, "ze-cli:editor-") {
			continue
		}
		path := wireToPath[reg.WireMethod]
		assert.NotEmpty(t, path, "RPC %s has no YANG path mapping", reg.WireMethod)
	}
}

// TestYANGPathsAreUnique verifies no two RPCs share the same YANG CLI path.
//
// VALIDATES: YANG-derived CLI paths are unique across all builtin RPCs.
// PREVENTS: Two RPCs mapping to the same CLI path, causing dispatch ambiguity.
func TestYANGPathsAreUnique(t *testing.T) {
	wireToPath := buildTestWireToPath()

	pathToWire := make(map[string]string)
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		path := wireToPath[reg.WireMethod]
		if path == "" {
			continue
		}
		if existing, ok := pathToWire[path]; ok {
			t.Errorf("duplicate YANG path %q: used by both %s and %s", path, existing, reg.WireMethod)
		}
		pathToWire[path] = reg.WireMethod
	}
}

// TestNoOwnerHoldsAnotherOwnersName runs the startup collision check over the
// whole linked set, builtins included, and expects it to find nothing.
//
// VALIDATES: AC-12 over the real population. No wire method is carried by two
// linked builtins, and NewServer, which refuses a duplicated wire method or a
// command name two owners reach, builds a server with every component linked.
// PREVENTS: a collision introduced by a new component reaching a daemon that
// would then refuse to start; this test names it first.
func TestNoOwnerHoldsAnotherOwnersName(t *testing.T) {
	seen := make(map[string]bool)
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if seen[reg.WireMethod] {
			t.Errorf("wire method %s is registered by two linked builtins", reg.WireMethod)
		}
		seen[reg.WireMethod] = true
	}
	if len(seen) == 0 {
		t.Fatal("no builtin RPC is linked: the check would pass vacuously")
	}

	if _, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil); err != nil {
		t.Fatalf("NewServer with every component linked: %v", err)
	}
}
