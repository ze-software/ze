package plugin

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Schema represents a YANG schema registered by a plugin.
type Schema struct {
	Module    string   // YANG module name
	Namespace string   // YANG namespace URI
	Yang      string   // Full YANG module text
	Handlers  []string // Handler paths (e.g., "bgp", "bgp.peer")
	Plugin    string   // Plugin that registered this schema
}

// SchemaRegistry stores and manages schemas from all plugins.
type SchemaRegistry struct {
	// Schemas indexed by module name
	modules map[string]*Schema

	// Handler path → module name mapping
	handlers map[string]string

	mu sync.RWMutex
}

// Errors for schema registration.
var (
	ErrSchemaModuleEmpty      = errors.New("schema module name is empty")
	ErrSchemaModuleDuplicate  = errors.New("schema module already registered")
	ErrSchemaHandlerDuplicate = errors.New("schema handler already registered")
	ErrSchemaNotFound         = errors.New("schema not found")
)

// NewSchemaRegistry creates a new schema registry.
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		modules:  make(map[string]*Schema),
		handlers: make(map[string]string),
	}
}

// Register adds a schema to the registry.
// Returns error if module name or handler paths conflict with existing registrations.
func (r *SchemaRegistry) Register(schema *Schema) error {
	if schema.Module == "" {
		return ErrSchemaModuleEmpty
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate module
	if _, exists := r.modules[schema.Module]; exists {
		return fmt.Errorf("%w: %s", ErrSchemaModuleDuplicate, schema.Module)
	}

	// Check for duplicate handlers
	for _, handler := range schema.Handlers {
		if existingModule, exists := r.handlers[handler]; exists {
			return fmt.Errorf("%w: %s (already registered by %s)", ErrSchemaHandlerDuplicate, handler, existingModule)
		}
	}

	// Register module
	r.modules[schema.Module] = schema

	// Register all handlers
	for _, handler := range schema.Handlers {
		r.handlers[handler] = schema.Module
	}

	return nil
}

// GetByModule returns a schema by module name.
func (r *SchemaRegistry) GetByModule(name string) (*Schema, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schema, exists := r.modules[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSchemaNotFound, name)
	}
	return schema, nil
}

// GetByHandler returns a schema by exact handler path.
func (r *SchemaRegistry) GetByHandler(path string) (*Schema, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	moduleName, exists := r.handlers[path]
	if !exists {
		return nil, fmt.Errorf("%w for handler: %s", ErrSchemaNotFound, path)
	}
	return r.modules[moduleName], nil
}

// FindHandler returns the schema for a handler path using longest prefix match.
// For example, if "bgp" and "bgp.peer" are registered, FindHandler("bgp.peer.timers")
// returns the schema for "bgp.peer".
func (r *SchemaRegistry) FindHandler(path string) (*Schema, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact match first
	if moduleName, exists := r.handlers[path]; exists {
		return r.modules[moduleName], path
	}

	// Try progressively shorter prefixes
	parts := strings.Split(path, ".")
	for i := len(parts) - 1; i > 0; i-- {
		prefix := strings.Join(parts[:i], ".")
		if moduleName, exists := r.handlers[prefix]; exists {
			return r.modules[moduleName], prefix
		}
	}

	return nil, ""
}

// ListModules returns all registered module names.
func (r *SchemaRegistry) ListModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.modules))
	for name := range r.modules {
		names = append(names, name)
	}
	return names
}

// ListHandlers returns all registered handler paths with their modules.
func (r *SchemaRegistry) ListHandlers() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string, len(r.handlers))
	for handler, module := range r.handlers {
		result[handler] = module
	}
	return result
}

// Count returns the number of registered schemas.
func (r *SchemaRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.modules)
}
