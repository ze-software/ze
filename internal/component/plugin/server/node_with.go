// Design: docs/architecture/api/commands.md — generic "set X with" handler
// Related: command.go — RPC handler infrastructure

package server

import (
	"fmt"
	"log/slog"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// NodePrepare is called after YANG parsing to apply node-specific validation
// and defaults. selector is the dispatcher-extracted selector (IP, name, etc.);
// nodeTree is the parsed config map.
// MUST return (nil, nil) on success or (response, error) on failure.
type NodePrepare func(selector string, nodeTree map[string]any) (*plugin.Response, error)

// NodeApply applies the validated config tree to the reactor.
// Called after prepare succeeds. Each node type provides its own apply
// (e.g., peers call AddDynamicPeer, groups would call AddDynamicGroup).
type NodeApply func(selector string, nodeTree map[string]any) error

// HandleNodeWith is the generic handler for "set <domain> <node> <selector> with <args>".
// It parses inline args via the YANG schema at schemaPath, runs the prepare callback
// for node-specific validation/defaults, then calls apply to commit the change.
func HandleNodeWith(
	ctx *CommandContext,
	args []string,
	schemaPath string,
	treeKey string,
	prepare NodePrepare,
	apply NodeApply,
) (*plugin.Response, error) {
	_, errResp, err := RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	selector := ctx.PeerSelector()
	if selector == "*" || selector == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("set requires specific %s selector", treeKey),
		}, fmt.Errorf("no %s specified", treeKey)
	}

	if len(args) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("set %s requires configuration arguments", treeKey),
		}, fmt.Errorf("no config args for %s", treeKey)
	}

	// Parse inline args via YANG schema.
	nodeTree, resp, parseErr := ParseInlineArgsForSchema(schemaPath, args)
	if resp != nil {
		return resp, parseErr
	}

	// Node-specific validation and defaults.
	if resp, err := prepare(selector, nodeTree); resp != nil || err != nil {
		return resp, err
	}

	// Apply via node-specific callback (e.g., AddDynamicPeer for peers).
	if err := apply(selector, nodeTree); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("failed to set %s %s: %v", treeKey, selector, err),
		}, fmt.Errorf("set %s %s: %w", treeKey, selector, err)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			treeKey:   selector,
			"message": treeKey + " set",
		},
	}, nil
}

// schemaCache caches YANG schema nodes by path (loaded lazily).
var (
	schemaCacheMu    sync.Mutex
	schemaCacheNodes = map[string]config.Node{}
)

// GetSchemaNode returns a cached YANG schema node for the given config path.
func GetSchemaNode(path string) config.Node {
	schemaCacheMu.Lock()
	defer schemaCacheMu.Unlock()

	if node, ok := schemaCacheNodes[path]; ok {
		return node
	}

	schema, err := config.YANGSchema()
	if err != nil {
		slog.Debug("YANG schema load failed", "error", err)
		return nil
	}
	node, err := schema.Lookup(path)
	if err != nil {
		slog.Debug("schema lookup failed", "path", path, "error", err)
		return nil
	}
	schemaCacheNodes[path] = node
	return node
}

// ParseInlineArgsForSchema parses config-syntax inline args using the YANG
// schema at the given path. Generic: works for any YANG node (peer, group, etc.).
// Returns (map, nil, nil) on success. Returns (nil, response, error) on failure.
func ParseInlineArgsForSchema(schemaPath string, args []string) (map[string]any, *plugin.Response, error) {
	node := GetSchemaNode(schemaPath)
	if node == nil {
		msg := fmt.Sprintf("internal error: schema %s not available", schemaPath)
		return nil, &plugin.Response{Status: plugin.StatusError, Data: msg}, fmt.Errorf("%s", msg)
	}

	tree, err := config.ParseInlineArgs(node, args)
	if err != nil {
		return nil, &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, err
	}

	return tree.ToMap(), nil, nil
}
