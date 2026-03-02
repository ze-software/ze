// Design: docs/architecture/api/process-protocol.md — plugin process management

package server

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/ipc"
	"codeberg.org/thomas-mangin/ze/internal/yang"
)

// Schema represents a YANG schema registered by a plugin.
type Schema struct {
	Module      string   // YANG module name
	Namespace   string   // YANG namespace URI
	Yang        string   // Full YANG module text
	Imports     []string // Imported module names (from YANG import statements)
	Handlers    []string // Handler paths (e.g., "bgp", "bgp.peer")
	Plugin      string   // Plugin that registered this schema
	Priority    int      // Config ordering (lower = processed first, default 1000)
	WantsConfig []string // Config roots plugin wants (from "declare wants config <root>")
}

// RegisteredRPC represents an RPC indexed in the schema registry.
type RegisteredRPC struct {
	Module      string          // YANG module name (e.g., "ze-bgp-api")
	Name        string          // RPC name in kebab-case (e.g., "peer-list")
	WireMethod  string          // Wire format "module:rpc-name" (e.g., "ze-bgp:peer-list")
	CLICommand  string          // CLI text command (e.g., "bgp peer list")
	Description string          // From YANG description
	Input       []yang.LeafMeta // Input parameter leaves
	Output      []yang.LeafMeta // Output parameter leaves
	Handler     Handler         // Handler function (set during registration)
}

// RegisteredNotification represents a notification indexed in the schema registry.
type RegisteredNotification struct {
	Module      string          // YANG module name
	Name        string          // Notification name in kebab-case
	WireMethod  string          // Wire format "module:notification-name"
	Description string          // From YANG description
	Leaves      []yang.LeafMeta // Notification data leaves
}

// SchemaRegistry stores and manages schemas from all plugins.
type SchemaRegistry struct {
	// Schemas indexed by module name
	modules map[string]*Schema

	// Handler path → module name mapping
	handlers map[string]string

	// RPCs indexed by wire method (e.g., "ze-bgp:peer-list")
	rpcs map[string]*RegisteredRPC

	// CLI command → wire method mapping
	commands map[string]string

	// Notifications indexed by wire method
	notifications map[string]*RegisteredNotification

	mu sync.RWMutex
}

// Errors for schema registration.
var (
	ErrSchemaModuleEmpty      = errors.New("schema module name is empty")
	ErrSchemaModuleDuplicate  = errors.New("schema module already registered")
	ErrSchemaHandlerDuplicate = errors.New("schema handler already registered")
	ErrSchemaNotFound         = errors.New("schema not found")
	ErrRPCNotFound            = errors.New("RPC not found")
	ErrRPCDuplicate           = errors.New("RPC wire method already registered")
	ErrNotificationDuplicate  = errors.New("notification wire method already registered")
)

// NewSchemaRegistry creates a new schema registry.
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		modules:       make(map[string]*Schema),
		handlers:      make(map[string]string),
		rpcs:          make(map[string]*RegisteredRPC),
		commands:      make(map[string]string),
		notifications: make(map[string]*RegisteredNotification),
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

// RegisterRPCs indexes RPCs extracted from a YANG module.
// Wire methods use the stripped module prefix (e.g., "ze-bgp-api" → "ze-bgp:peer-list").
func (r *SchemaRegistry) RegisterRPCs(module string, rpcs []yang.RPCMeta) error {
	wireModule := yang.WireModule(module)

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, meta := range rpcs {
		wireMethod := ipc.FormatMethod(wireModule, meta.Name)
		if _, exists := r.rpcs[wireMethod]; exists {
			return fmt.Errorf("%w: %s", ErrRPCDuplicate, wireMethod)
		}
		r.rpcs[wireMethod] = &RegisteredRPC{
			Module:      module,
			Name:        meta.Name,
			WireMethod:  wireMethod,
			Description: meta.Description,
			Input:       meta.Input,
			Output:      meta.Output,
		}
	}
	return nil
}

// RegisterNotifications indexes notifications extracted from a YANG module.
func (r *SchemaRegistry) RegisterNotifications(module string, notifs []yang.NotificationMeta) error {
	wireModule := yang.WireModule(module)

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, meta := range notifs {
		wireMethod := ipc.FormatMethod(wireModule, meta.Name)
		if _, exists := r.notifications[wireMethod]; exists {
			return fmt.Errorf("%w: %s", ErrNotificationDuplicate, wireMethod)
		}
		r.notifications[wireMethod] = &RegisteredNotification{
			Module:      module,
			Name:        meta.Name,
			WireMethod:  wireMethod,
			Description: meta.Description,
			Leaves:      meta.Leaves,
		}
	}
	return nil
}

// RegisterCLICommand associates a CLI text command with a wire method.
// The wire method must already be registered via RegisterRPCs.
func (r *SchemaRegistry) RegisterCLICommand(cliCommand, wireMethod string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rpc, exists := r.rpcs[wireMethod]
	if !exists {
		return fmt.Errorf("%w: %s", ErrRPCNotFound, wireMethod)
	}

	r.commands[cliCommand] = wireMethod
	rpc.CLICommand = cliCommand
	return nil
}

// FindRPC returns the registered RPC for an exact wire method match.
func (r *SchemaRegistry) FindRPC(wireMethod string) (*RegisteredRPC, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rpc, exists := r.rpcs[wireMethod]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrRPCNotFound, wireMethod)
	}
	return rpc, nil
}

// FindRPCByCommand returns the registered RPC for a CLI text command.
func (r *SchemaRegistry) FindRPCByCommand(cliCommand string) (*RegisteredRPC, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	wireMethod, exists := r.commands[cliCommand]
	if !exists {
		return nil, fmt.Errorf("%w for command: %s", ErrRPCNotFound, cliCommand)
	}
	return r.rpcs[wireMethod], nil
}

// ListRPCs returns all registered RPCs, optionally filtered by YANG module name.
// Pass empty string to list all RPCs.
func (r *SchemaRegistry) ListRPCs(module string) []*RegisteredRPC {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredRPC, 0, len(r.rpcs))
	for _, rpc := range r.rpcs {
		if module == "" || rpc.Module == module {
			result = append(result, rpc)
		}
	}
	return result
}

// ListNotifications returns all registered notifications, optionally filtered by YANG module name.
// Pass empty string to list all notifications.
func (r *SchemaRegistry) ListNotifications(module string) []*RegisteredNotification {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredNotification, 0, len(r.notifications))
	for _, notif := range r.notifications {
		if module == "" || notif.Module == module {
			result = append(result, notif)
		}
	}
	return result
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
// Predicates like [address=192.0.2.1] are stripped before matching.
func (r *SchemaRegistry) FindHandler(path string) (*Schema, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Strip predicates from path for matching (e.g., "bgp.peer[addr=x]" → "bgp.peer")
	cleanPath := stripPredicates(path)

	// Try exact match first
	if moduleName, exists := r.handlers[cleanPath]; exists {
		return r.modules[moduleName], cleanPath
	}

	// Try progressively shorter prefixes
	parts := strings.Split(cleanPath, ".")
	for i := len(parts) - 1; i > 0; i-- {
		prefix := strings.Join(parts[:i], ".")
		if moduleName, exists := r.handlers[prefix]; exists {
			return r.modules[moduleName], prefix
		}
	}

	return nil, ""
}

// stripPredicates removes YANG predicates like [key=value] from a path.
// Example: "bgp.peer[address=192.0.2.1].timers" → "bgp.peer.timers".
func stripPredicates(path string) string {
	var result strings.Builder
	depth := 0
	for _, c := range path {
		if c == '[' {
			depth++
			continue
		}
		if c == ']' {
			depth--
			continue
		}
		if depth == 0 {
			result.WriteRune(c)
		}
	}
	return result.String()
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
	maps.Copy(result, r.handlers)
	return result
}

// Count returns the number of registered schemas.
func (r *SchemaRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.modules)
}
