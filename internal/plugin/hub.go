package plugin

import (
	"context"
	"fmt"
	"strings"
)

// Hub orchestrates plugin communication and verify/apply routing.
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

// VerifyRequest represents a config verify request.
type VerifyRequest struct {
	Handler string // Handler path (e.g., "bgp.peer")
	Action  string // create, modify, delete
	Path    string // Full path (e.g., "bgp.peer[address=192.0.2.1]")
	Data    string // JSON data
}

// ApplyRequest represents a config apply request.
type ApplyRequest struct {
	Handler string // Handler path
	Path    string // Full path
}

// RouteVerify routes a verify request to the appropriate plugin.
func (h *Hub) RouteVerify(ctx context.Context, req *VerifyRequest) error {
	// Find schema for handler
	schema, handlerPath := h.registry.FindHandler(req.Handler)
	if schema == nil {
		return fmt.Errorf("unknown handler: %s", req.Handler)
	}

	// Find subsystem that registered this schema
	handler := h.subsystems.Get(schema.Plugin)
	if handler == nil {
		return fmt.Errorf("plugin not found for handler %s: %s", req.Handler, schema.Plugin)
	}

	// Format command for plugin
	cmd := fmt.Sprintf("config verify action %s path %q data '%s'", req.Action, req.Path, req.Data)

	// Send request to plugin
	resp, err := handler.Handle(ctx, cmd)
	if err != nil {
		return fmt.Errorf("verify failed: %w", err)
	}

	// Check response
	if resp.Status == statusError {
		return fmt.Errorf("verify rejected: %v", resp.Data)
	}

	_ = handlerPath // Used for logging/debugging
	return nil
}

// RouteApply routes an apply request to the appropriate plugin.
func (h *Hub) RouteApply(ctx context.Context, req *ApplyRequest) error {
	// Find schema for handler
	schema, _ := h.registry.FindHandler(req.Handler)
	if schema == nil {
		return fmt.Errorf("unknown handler: %s", req.Handler)
	}

	// Find subsystem that registered this schema
	handler := h.subsystems.Get(schema.Plugin)
	if handler == nil {
		return fmt.Errorf("plugin not found for handler %s: %s", req.Handler, schema.Plugin)
	}

	// Format command for plugin
	cmd := fmt.Sprintf("config apply path %q", req.Path)

	// Send request to plugin
	resp, err := handler.Handle(ctx, cmd)
	if err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	// Check response
	if resp.Status == statusError {
		return fmt.Errorf("apply rejected: %v", resp.Data)
	}

	return nil
}

// ProcessConfig processes a configuration transaction (all verify, then all apply).
func (h *Hub) ProcessConfig(ctx context.Context, blocks []ConfigBlock) error {
	// Phase 1: Verify all blocks
	for _, block := range blocks {
		req := &VerifyRequest{
			Handler: block.Handler,
			Action:  block.Action,
			Path:    block.Path,
			Data:    block.Data,
		}
		if err := h.RouteVerify(ctx, req); err != nil {
			return fmt.Errorf("verify failed for %s: %w", block.Path, err)
		}
	}

	// Phase 2: Apply all blocks
	for _, block := range blocks {
		req := &ApplyRequest{
			Handler: block.Handler,
			Path:    block.Path,
		}
		if err := h.RouteApply(ctx, req); err != nil {
			return fmt.Errorf("apply failed for %s: %w", block.Path, err)
		}
	}

	return nil
}

// ConfigBlock represents a config block to be verified and applied.
type ConfigBlock struct {
	Handler string // Handler path
	Action  string // create, modify, delete
	Path    string // Full path
	Data    string // JSON data
}

// ParseVerifyCommand parses "config verify handler <h> action <a> path <p> data '<d>'".
func ParseVerifyCommand(line string) (*VerifyRequest, error) {
	// Remove "config verify " prefix
	if !strings.HasPrefix(line, "config verify ") {
		return nil, fmt.Errorf("expected 'config verify' prefix")
	}
	rest := strings.TrimPrefix(line, "config verify ")

	req := &VerifyRequest{}

	// Parse handler
	if !strings.HasPrefix(rest, "handler ") {
		return nil, fmt.Errorf("expected 'handler'")
	}
	rest = strings.TrimPrefix(rest, "handler ")
	handler, rest, err := parseQuotedOrWord(rest)
	if err != nil {
		return nil, fmt.Errorf("parse handler: %w", err)
	}
	req.Handler = handler

	// Parse action
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "action ") {
		return nil, fmt.Errorf("expected 'action'")
	}
	rest = strings.TrimPrefix(rest, "action ")
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("expected action value")
	}
	req.Action = parts[0]
	rest = parts[1]

	// Parse path
	if !strings.HasPrefix(rest, "path ") {
		return nil, fmt.Errorf("expected 'path'")
	}
	rest = strings.TrimPrefix(rest, "path ")
	path, rest, err := parseQuotedOrWord(rest)
	if err != nil {
		return nil, fmt.Errorf("parse path: %w", err)
	}
	req.Path = path

	// Parse data
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "data ") {
		return nil, fmt.Errorf("expected 'data'")
	}
	rest = strings.TrimPrefix(rest, "data ")
	data, _, err := parseQuotedData(rest)
	if err != nil {
		return nil, fmt.Errorf("parse data: %w", err)
	}
	req.Data = data

	return req, nil
}

// parseQuotedOrWord parses a quoted string or unquoted word.
func parseQuotedOrWord(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return "", "", fmt.Errorf("empty input")
	}

	if s[0] == '"' {
		// Find closing quote and unescape
		var result strings.Builder
		i := 1
		for i < len(s) {
			if s[i] == '\\' && i+1 < len(s) {
				// Write the escaped character directly
				i++
				result.WriteByte(s[i])
				i++
				continue
			}
			if s[i] == '"' {
				return result.String(), s[i+1:], nil
			}
			result.WriteByte(s[i])
			i++
		}
		return "", "", fmt.Errorf("unclosed quote")
	}

	// Unquoted word
	end := 0
	for end < len(s) && s[end] != ' ' {
		end++
	}
	return s[:end], s[end:], nil
}

// parseQuotedData parses single-quoted data (JSON).
//
//nolint:unparam // rest returned for API consistency with parseQuotedOrWord
func parseQuotedData(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '\'' {
		return "", "", fmt.Errorf("expected single quote for data")
	}

	// Find closing quote and unescape
	var result strings.Builder
	i := 1
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			// Write the escaped character directly
			i++
			result.WriteByte(s[i])
			i++
			continue
		}
		if s[i] == '\'' {
			return result.String(), s[i+1:], nil
		}
		result.WriteByte(s[i])
		i++
	}
	return "", "", fmt.Errorf("unclosed quote in data")
}
