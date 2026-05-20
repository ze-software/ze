// Design: docs/architecture/web-interface.md -- Graceful listener migration on config reload
// Related: main_reload.go -- doReload calls ListenerMigrator.ReloadListeners

package hub

import (
	"context"
	"fmt"
	"log/slog"

	apigrpc "codeberg.org/thomas-mangin/ze/internal/component/api/grpc"
	"codeberg.org/thomas-mangin/ze/internal/component/api/rest"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/lg"
	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// Reconfigurable is implemented by any server that supports live listener migration.
type Reconfigurable interface {
	Addresses() []string
	Reconfigure(ctx context.Context, newAddrs []string) error
}

// serviceChange describes a single service's listener migration.
type serviceChange struct {
	name    string
	server  Reconfigurable
	oldAddr []string
	newAddr []string
	add     []string
	remove  []string
}

// ListenerMigrator coordinates listener reconfiguration across services on
// config reload. It detects cross-service address conflicts and sequences
// migrations to minimize downtime.
type ListenerMigrator struct {
	web    *zeweb.WebServer
	lg     *lg.LGServer
	mcp    *MCPServerHandle
	rest   *rest.RESTServer
	grpc   *apigrpc.GRPCServer
	logger *slog.Logger
}

// NewListenerMigrator creates a migrator. Pass nil for services that are not running.
func NewListenerMigrator(web *zeweb.WebServer) *ListenerMigrator {
	return &ListenerMigrator{
		web:    web,
		logger: slogutil.Logger("hub.listener"),
	}
}

// SetWeb updates the web server reference.
func (m *ListenerMigrator) SetWeb(web *zeweb.WebServer) { m.web = web }

// SetLG updates the looking glass server reference.
func (m *ListenerMigrator) SetLG(s *lg.LGServer) { m.lg = s }

// SetMCP updates the MCP server reference.
func (m *ListenerMigrator) SetMCP(s *MCPServerHandle) { m.mcp = s }

// SetREST updates the REST API server reference.
func (m *ListenerMigrator) SetREST(s *rest.RESTServer) { m.rest = s }

// SetGRPC updates the gRPC API server reference.
func (m *ListenerMigrator) SetGRPC(s *apigrpc.GRPCServer) { m.grpc = s }

// ReloadListeners extracts new listen configs from the config tree and
// reconfigures running services. Returns nil if no listener changes are needed.
func (m *ListenerMigrator) ReloadListeners(ctx context.Context, tree *zeconfig.Tree) error {
	var changes []serviceChange

	if m.web != nil {
		if webCfg, ok := zeconfig.ExtractWebConfig(tree); ok {
			if sc, ok := m.buildChange("web", m.web, endpointsToAddrs(webCfg.Servers)); ok {
				changes = append(changes, sc)
			}
		}
	}

	if m.lg != nil {
		if lgCfg, ok := zeconfig.ExtractLGConfig(tree); ok {
			if sc, ok := m.buildChange("lg", m.lg, endpointsToAddrs(lgCfg.Servers)); ok {
				changes = append(changes, sc)
			}
		}
	}

	if m.mcp != nil {
		if mcpCfg, ok := zeconfig.ExtractMCPConfig(tree); ok {
			if sc, ok := m.buildChange("mcp", m.mcp, endpointsToAddrs(mcpCfg.Servers)); ok {
				changes = append(changes, sc)
			}
		}
	}

	if m.rest != nil || m.grpc != nil {
		if apiCfg, ok := zeconfig.ExtractAPIConfig(tree); ok {
			if m.rest != nil && apiCfg.RESTOn {
				if sc, ok := m.buildChange("rest", m.rest, apiListenToAddrs(apiCfg.REST)); ok {
					changes = append(changes, sc)
				}
			}
			if m.grpc != nil && apiCfg.GRPCOn {
				if sc, ok := m.buildChange("grpc", m.grpc, apiListenToAddrs(apiCfg.GRPC)); ok {
					changes = append(changes, sc)
				}
			}
		}
	}

	if len(changes) == 0 {
		return nil
	}

	conflicts := detectConflicts(changes)

	for i := range changes {
		if conflicts[changes[i].name] {
			continue
		}
		m.logger.Info("reconfiguring listeners", "service", changes[i].name,
			"add", changes[i].add, "remove", changes[i].remove)
		if err := changes[i].server.Reconfigure(ctx, changes[i].newAddr); err != nil {
			return fmt.Errorf("reconfigure %s: %w", changes[i].name, err)
		}
	}

	for i := range changes {
		if !conflicts[changes[i].name] {
			continue
		}
		m.logger.Warn("sequenced listener migration (brief gap expected)",
			"service", changes[i].name,
			"add", changes[i].add, "remove", changes[i].remove)
		if err := changes[i].server.Reconfigure(ctx, changes[i].newAddr); err != nil {
			return fmt.Errorf("reconfigure %s (conflicting): %w", changes[i].name, err)
		}
	}

	return nil
}

func (m *ListenerMigrator) buildChange(name string, srv Reconfigurable, newAddrs []string) (serviceChange, bool) {
	oldAddrs := srv.Addresses()
	_, add, remove := zeweb.ListenerDiff(oldAddrs, newAddrs)
	if len(add) == 0 && len(remove) == 0 {
		return serviceChange{}, false
	}
	return serviceChange{
		name:    name,
		server:  srv,
		oldAddr: oldAddrs,
		newAddr: newAddrs,
		add:     add,
		remove:  remove,
	}, true
}

func apiListenToAddrs(configs []zeconfig.APIListenConfig) []string {
	out := make([]string, 0, len(configs))
	for _, c := range configs {
		out = append(out, c.Listen())
	}
	return out
}

// detectConflicts returns a set of service names that have address conflicts
// with other services. A conflict occurs when an address in one service's
// "remove" set appears in another service's "add" set.
func detectConflicts(changes []serviceChange) map[string]bool {
	addSets := make(map[string]map[string]bool, len(changes))
	for _, c := range changes {
		s := make(map[string]bool, len(c.add))
		for _, a := range c.add {
			s[a] = true
		}
		addSets[c.name] = s
	}

	conflicted := make(map[string]bool)
	for _, c := range changes {
		for _, removed := range c.remove {
			for otherName, otherAdd := range addSets {
				if otherName == c.name {
					continue
				}
				if otherAdd[removed] {
					conflicted[c.name] = true
					conflicted[otherName] = true
				}
			}
		}
	}
	return conflicted
}
