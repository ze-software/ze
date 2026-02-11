package plugin

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Hub orchestrates plugin communication and command routing.
type Hub struct {
	registry   *SchemaRegistry
	subsystems *SubsystemManager
}

// NewHub creates a new Hub with the given registry and subsystem manager.
func NewHub(registry *SchemaRegistry, subsystems *SubsystemManager) *Hub {
	return &Hub{
		registry:   registry,
		subsystems: subsystems,
	}
}

// ConfigBlock represents a config command to send to a plugin.
type ConfigBlock struct {
	Handler string // Handler path (e.g., "bgp.peer")
	Action  string // create, modify, delete
	Path    string // Full path (e.g., "bgp.peer[address=192.0.2.1]")
	Data    string // JSON data
}

// RouteCommand routes a command to the appropriate plugin.
// Format: <namespace> <path> <action> {json}.
func (h *Hub) RouteCommand(ctx context.Context, block *ConfigBlock) error {
	// Find schema for handler.
	schema, _ := h.registry.FindHandler(block.Handler)
	if schema == nil {
		return fmt.Errorf("unknown handler: %s", block.Handler)
	}

	// Find subsystem that registered this schema.
	handler := h.subsystems.Get(schema.Plugin)
	if handler == nil {
		return fmt.Errorf("plugin not found for handler %s: %s", block.Handler, schema.Plugin)
	}

	// Extract namespace and path from handler.
	// Handler is like "bgp.peer" → namespace="bgp", path="peer".
	namespace, path := splitHandler(block.Handler)

	// Format command: <namespace> <path> <action> {json}.
	// If path is empty (handler is just namespace), omit it.
	var cmd string
	if path == "" {
		cmd = fmt.Sprintf("%s %s %s", namespace, block.Action, block.Data)
	} else {
		cmd = fmt.Sprintf("%s %s %s %s", namespace, path, block.Action, block.Data)
	}

	// Send command to plugin.
	resp, err := handler.Handle(ctx, cmd)
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	if resp.Status == statusError {
		return fmt.Errorf("command rejected: %v", resp.Data)
	}

	return nil
}

// RouteCommit sends a commit command to a plugin.
// Format: <namespace> commit.
func (h *Hub) RouteCommit(ctx context.Context, namespace string) error {
	return h.routeTransaction(ctx, namespace, "commit")
}

// ProcessConfig processes a configuration transaction.
// Sends all commands to plugins, then commits each affected namespace.
func (h *Hub) ProcessConfig(ctx context.Context, blocks []ConfigBlock) error {
	if len(blocks) == 0 {
		return nil
	}

	// Track which namespaces need commit.
	namespaceSet := make(map[string]bool)

	// Send all config commands.
	for _, block := range blocks {
		if err := h.RouteCommand(ctx, &block); err != nil {
			return fmt.Errorf("config failed for %s: %w", block.Path, err)
		}
		// Track namespace for commit.
		namespace, _ := splitHandler(block.Handler)
		namespaceSet[namespace] = true
	}

	// Sort namespaces for deterministic commit order.
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	// Commit all affected namespaces.
	// NOTE: If commit fails mid-way, already-committed namespaces remain committed.
	// Rollback only affects candidate state, not running state, so we cannot
	// undo a successful commit. The system may be left in a partially-committed
	// state, which the operator must resolve manually.
	for _, namespace := range namespaces {
		if err := h.RouteCommit(ctx, namespace); err != nil {
			return fmt.Errorf("commit failed for %s: %w", namespace, err)
		}
	}

	return nil
}

// RouteRollback sends a rollback command to a plugin.
// Format: <namespace> rollback.
func (h *Hub) RouteRollback(ctx context.Context, namespace string) error {
	return h.routeTransaction(ctx, namespace, "rollback")
}

// routeTransaction sends a transaction command (commit/rollback) to a plugin.
func (h *Hub) routeTransaction(ctx context.Context, namespace, action string) error {
	schema, _ := h.registry.FindHandler(namespace)
	if schema == nil {
		return fmt.Errorf("unknown namespace: %s", namespace)
	}

	handler := h.subsystems.Get(schema.Plugin)
	if handler == nil {
		return fmt.Errorf("plugin not found for namespace %s: %s", namespace, schema.Plugin)
	}

	cmd := fmt.Sprintf("%s %s", namespace, action)

	resp, err := handler.Handle(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s failed: %w", action, err)
	}

	if resp.Status == statusError {
		return fmt.Errorf("%s rejected: %v", action, resp.Data)
	}

	return nil
}

// splitHandler splits a handler path into namespace and path.
// "bgp.peer" → "bgp", "peer".
// "bgp" → "bgp", "".
func splitHandler(handler string) (namespace, path string) {
	idx := strings.Index(handler, ".")
	if idx < 0 {
		return handler, ""
	}
	return handler[:idx], handler[idx+1:]
}

// ParseCommand parses a namespace command.
// Format: <namespace> <path> <action> {json}.
// Or: <namespace> <action> {json} (for namespace-level config).
// Or: <namespace> commit|rollback|diff.
func ParseCommand(line string) (*ConfigBlock, error) {
	// Find JSON start.
	jsonIdx := strings.Index(line, "{")
	if jsonIdx < 0 {
		// Check for commit/rollback/diff.
		parts := strings.Fields(line)
		if len(parts) == 2 {
			switch parts[1] {
			case "commit", "rollback", "diff":
				return &ConfigBlock{
					Handler: parts[0],
					Action:  parts[1],
				}, nil
			}
		}
		return nil, fmt.Errorf("expected JSON data or commit/rollback/diff")
	}

	// Parse namespace, path, action from before JSON.
	prefix := strings.TrimSpace(line[:jsonIdx])
	parts := strings.Fields(prefix)

	data := line[jsonIdx:]

	switch len(parts) {
	case 2:
		// Format: <namespace> <action> {json} (namespace-level config).
		namespace := parts[0]
		action := parts[1]
		return &ConfigBlock{
			Handler: namespace,
			Action:  action,
			Path:    namespace,
			Data:    data,
		}, nil

	case 3:
		// Format: <namespace> <path> <action> {json}.
		namespace := parts[0]
		path := parts[1]
		action := parts[2]
		return &ConfigBlock{
			Handler: namespace + "." + path,
			Action:  action,
			Path:    namespace + "." + path,
			Data:    data,
		}, nil

	default:
		return nil, fmt.Errorf("expected '<namespace> <action>' or '<namespace> <path> <action>'")
	}
}
