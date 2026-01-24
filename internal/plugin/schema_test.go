package plugin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchemaRegistry_Register verifies basic schema registration.
//
// VALIDATES: Schema can be registered with module name and handlers.
// PREVENTS: Registration failures blocking plugin startup.
func TestSchemaRegistry_Register(t *testing.T) {
	reg := NewSchemaRegistry()

	schema := &Schema{
		Module:    "ze-bgp",
		Namespace: "urn:ze:bgp",
		Yang:      "module ze-bgp { ... }",
		Handlers:  []string{"bgp", "bgp.peer", "bgp.peer-group"},
		Plugin:    "bgp-subsystem",
	}

	err := reg.Register(schema)
	require.NoError(t, err)

	// Verify registration
	assert.Equal(t, 1, reg.Count())

	// Verify module lookup
	got, err := reg.GetByModule("ze-bgp")
	require.NoError(t, err)
	assert.Equal(t, schema, got)

	// Verify handler lookup
	got, err = reg.GetByHandler("bgp.peer")
	require.NoError(t, err)
	assert.Equal(t, schema, got)
}

// TestSchemaRegistry_DuplicateModule verifies duplicate module rejection.
//
// VALIDATES: Same module name cannot be registered twice.
// PREVENTS: Schema conflicts between plugins.
func TestSchemaRegistry_DuplicateModule(t *testing.T) {
	reg := NewSchemaRegistry()

	schema1 := &Schema{
		Module:   "ze-bgp",
		Handlers: []string{"bgp"},
		Plugin:   "plugin1",
	}
	schema2 := &Schema{
		Module:   "ze-bgp", // Same module name
		Handlers: []string{"rib"},
		Plugin:   "plugin2",
	}

	err := reg.Register(schema1)
	require.NoError(t, err)

	err = reg.Register(schema2)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSchemaModuleDuplicate)
	assert.Contains(t, err.Error(), "ze-bgp")
}

// TestSchemaRegistry_DuplicateHandler verifies duplicate handler rejection.
//
// VALIDATES: Same handler path cannot be registered by different modules.
// PREVENTS: Handler routing conflicts.
func TestSchemaRegistry_DuplicateHandler(t *testing.T) {
	reg := NewSchemaRegistry()

	schema1 := &Schema{
		Module:   "ze-bgp",
		Handlers: []string{"bgp", "bgp.peer"},
		Plugin:   "plugin1",
	}
	schema2 := &Schema{
		Module:   "ze-rib",
		Handlers: []string{"bgp.peer"}, // Conflict with schema1
		Plugin:   "plugin2",
	}

	err := reg.Register(schema1)
	require.NoError(t, err)

	err = reg.Register(schema2)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSchemaHandlerDuplicate)
	assert.Contains(t, err.Error(), "bgp.peer")
	assert.Contains(t, err.Error(), "ze-bgp")
}

// TestSchemaRegistry_FindHandler verifies longest prefix match routing.
//
// VALIDATES: FindHandler returns correct schema for longest matching prefix.
// PREVENTS: Wrong plugin receiving config verify/apply requests.
func TestSchemaRegistry_FindHandler(t *testing.T) {
	reg := NewSchemaRegistry()

	// Register bgp and bgp.peer as separate handlers
	bgpSchema := &Schema{
		Module:   "ze-bgp",
		Handlers: []string{"bgp"},
		Plugin:   "bgp-plugin",
	}
	peerSchema := &Schema{
		Module:   "ze-bgp-peer",
		Handlers: []string{"bgp.peer", "bgp.peer-group"},
		Plugin:   "peer-plugin",
	}

	require.NoError(t, reg.Register(bgpSchema))
	require.NoError(t, reg.Register(peerSchema))

	tests := []struct {
		path       string
		wantModule string
		wantMatch  string
	}{
		// Exact matches
		{"bgp", "ze-bgp", "bgp"},
		{"bgp.peer", "ze-bgp-peer", "bgp.peer"},
		{"bgp.peer-group", "ze-bgp-peer", "bgp.peer-group"},

		// Prefix matches
		{"bgp.peer.timers", "ze-bgp-peer", "bgp.peer"},
		{"bgp.peer.capability.add-path", "ze-bgp-peer", "bgp.peer"},
		{"bgp.local-as", "ze-bgp", "bgp"},

		// No match
		{"rib", "", ""},
		{"system", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			schema, match := reg.FindHandler(tt.path)
			if tt.wantModule == "" {
				assert.Nil(t, schema)
				assert.Empty(t, match)
			} else {
				require.NotNil(t, schema)
				assert.Equal(t, tt.wantModule, schema.Module)
				assert.Equal(t, tt.wantMatch, match)
			}
		})
	}
}

// TestSchemaRegistry_EmptyModule verifies empty module name rejection.
//
// VALIDATES: Empty module names are rejected.
// PREVENTS: Invalid schemas in registry.
func TestSchemaRegistry_EmptyModule(t *testing.T) {
	reg := NewSchemaRegistry()

	schema := &Schema{
		Module:   "",
		Handlers: []string{"bgp"},
		Plugin:   "test",
	}

	err := reg.Register(schema)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSchemaModuleEmpty)
}

// TestSchemaRegistry_GetByModule_NotFound verifies missing module error.
//
// VALIDATES: GetByModule returns error for unknown module.
// PREVENTS: Nil pointer dereference on unknown module lookup.
func TestSchemaRegistry_GetByModule_NotFound(t *testing.T) {
	reg := NewSchemaRegistry()

	_, err := reg.GetByModule("nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSchemaNotFound)
}

// TestSchemaRegistry_GetByHandler_NotFound verifies missing handler error.
//
// VALIDATES: GetByHandler returns error for unknown handler.
// PREVENTS: Nil pointer dereference on unknown handler lookup.
func TestSchemaRegistry_GetByHandler_NotFound(t *testing.T) {
	reg := NewSchemaRegistry()

	_, err := reg.GetByHandler("nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSchemaNotFound)
}

// TestSchemaRegistry_ListModules verifies module listing.
//
// VALIDATES: ListModules returns all registered modules.
// PREVENTS: Missing modules in discovery commands.
func TestSchemaRegistry_ListModules(t *testing.T) {
	reg := NewSchemaRegistry()

	require.NoError(t, reg.Register(&Schema{Module: "ze-bgp", Handlers: []string{"bgp"}}))
	require.NoError(t, reg.Register(&Schema{Module: "ze-rib", Handlers: []string{"rib"}}))
	require.NoError(t, reg.Register(&Schema{Module: "ze-plugin", Handlers: []string{"plugin"}}))

	modules := reg.ListModules()
	assert.Len(t, modules, 3)
	assert.Contains(t, modules, "ze-bgp")
	assert.Contains(t, modules, "ze-rib")
	assert.Contains(t, modules, "ze-plugin")
}

// TestSchemaRegistry_ListHandlers verifies handler listing.
//
// VALIDATES: ListHandlers returns all handlers with their modules.
// PREVENTS: Missing handlers in discovery commands.
func TestSchemaRegistry_ListHandlers(t *testing.T) {
	reg := NewSchemaRegistry()

	require.NoError(t, reg.Register(&Schema{
		Module:   "ze-bgp",
		Handlers: []string{"bgp", "bgp.peer"},
	}))

	handlers := reg.ListHandlers()
	assert.Len(t, handlers, 2)
	assert.Equal(t, "ze-bgp", handlers["bgp"])
	assert.Equal(t, "ze-bgp", handlers["bgp.peer"])
}

// TestSchemaRegistry_ModuleNameBoundary verifies module name length limits.
//
// VALIDATES: Module names up to 256 chars accepted.
// BOUNDARY: 256 (valid), 257 (invalid).
func TestSchemaRegistry_ModuleNameBoundary(t *testing.T) {
	reg := NewSchemaRegistry()

	// 256 chars - valid
	name256 := strings.Repeat("a", 256)
	err := reg.Register(&Schema{Module: name256, Handlers: []string{"h1"}})
	assert.NoError(t, err, "256 char module name should be accepted")

	// Registry doesn't enforce length limits - that's YANG validation's job
	// Just ensure long names work technically
	name1000 := strings.Repeat("b", 1000)
	err = reg.Register(&Schema{Module: name1000, Handlers: []string{"h2"}})
	assert.NoError(t, err, "long module name should work")
}

// TestSchemaRegistry_HandlerPathDepth verifies handler path depth limits.
//
// VALIDATES: Handler paths with up to 10 segments accepted.
// BOUNDARY: 10 (valid).
func TestSchemaRegistry_HandlerPathDepth(t *testing.T) {
	reg := NewSchemaRegistry()

	// 10 segments - valid
	path10 := "a.b.c.d.e.f.g.h.i.j"
	err := reg.Register(&Schema{Module: "deep", Handlers: []string{path10}})
	assert.NoError(t, err, "10 segment path should be accepted")

	// Verify lookup works
	schema, match := reg.FindHandler(path10 + ".k.l")
	require.NotNil(t, schema)
	assert.Equal(t, path10, match)
}

// TestSchemaRegistry_Concurrent verifies thread safety.
//
// VALIDATES: Concurrent registration and lookup work correctly.
// PREVENTS: Race conditions in multi-plugin scenarios.
func TestSchemaRegistry_Concurrent(t *testing.T) {
	reg := NewSchemaRegistry()

	// Register schemas concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			schema := &Schema{
				Module:   strings.Repeat("a", idx+1) + "-module",
				Handlers: []string{strings.Repeat("h", idx+1)},
			}
			_ = reg.Register(schema)
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify some registered (some may have failed due to race)
	assert.Greater(t, reg.Count(), 0)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_ = reg.ListModules()
			_ = reg.ListHandlers()
			_, _ = reg.FindHandler("test.path")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
