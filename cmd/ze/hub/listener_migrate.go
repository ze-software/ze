// Design: docs/architecture/web-interface.md -- Graceful listener migration on config reload
// Related: main_reload.go -- doReload calls ListenerMigrator.ReloadListeners

package hub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
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
	web    Reconfigurable
	lg     Reconfigurable
	mcp    Reconfigurable
	rest   Reconfigurable
	grpc   Reconfigurable
	logger *slog.Logger

	// unauth holds the names of services built without authentication. A SIGHUP
	// reload must not migrate any of them to a non-loopback address: the auth
	// mode is fixed at build time, so a boot-time exposure guard (mgmt_guard.go)
	// would otherwise fail open on reload. Populated by MarkUnauthenticated
	// after the boot guard passes.
	unauth map[string]bool
}

// MarkUnauthenticated records that the named service was built without
// authentication, so ReloadListeners refuses any migration that would move it
// to a non-loopback address (AC-7 of the management-listener guard).
func (m *ListenerMigrator) MarkUnauthenticated(name string) {
	if m.unauth == nil {
		m.unauth = make(map[string]bool)
	}
	m.unauth[name] = true
}

// NewListenerMigrator creates a migrator. Pass nil for services that are not running.
func NewListenerMigrator(web Reconfigurable) *ListenerMigrator {
	return &ListenerMigrator{
		web:    web,
		logger: slogutil.Logger("hub.listener"),
	}
}

// SetWeb updates the web server reference.
func (m *ListenerMigrator) SetWeb(web Reconfigurable) { m.web = web }

// SetLG updates the looking glass server reference. Takes Reconfigurable (not
// *lg.LGServer) so always-on code never imports the lg package: lg is built
// through the construction registry and may be compiled out (//go:build ze_lg).
func (m *ListenerMigrator) SetLG(s Reconfigurable) { m.lg = s }

// SetMCP updates the MCP server reference. Takes Reconfigurable (not
// *MCPServerHandle) so always-on code never imports the mcp package: mcp is
// built through the construction registry and may be compiled out
// (//go:build ze_mcp).
func (m *ListenerMigrator) SetMCP(s Reconfigurable) { m.mcp = s }

// SetREST updates the REST API server reference. Takes Reconfigurable (not
// *rest.RESTServer) so always-on code never imports the api/rest package: the
// API servers are built through the ze_api seam and may be compiled out.
func (m *ListenerMigrator) SetREST(s Reconfigurable) { m.rest = s }

// SetGRPC updates the gRPC API server reference. Takes Reconfigurable (see
// SetREST) so always-on code never imports the api/grpc package.
func (m *ListenerMigrator) SetGRPC(s Reconfigurable) { m.grpc = s }

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

	// Fail-closed reload gate (AC-7): refuse before applying ANY change if a
	// service built without authentication would move to a non-loopback
	// address. Returning here leaves every service on its current listeners.
	for i := range changes {
		if !m.unauth[changes[i].name] {
			continue
		}
		for _, addr := range changes[i].add {
			if listenAddrIsNonLoopback(addr) {
				return fmt.Errorf("refusing to migrate %s to non-loopback listener %q without authentication", changes[i].name, addr)
			}
		}
	}

	conflicts := detectConflicts(changes)

	ordered := make([]serviceChange, 0, len(changes))
	for i := range changes {
		if !conflicts[changes[i].name] {
			ordered = append(ordered, changes[i])
		}
	}
	for i := range changes {
		if conflicts[changes[i].name] {
			ordered = append(ordered, changes[i])
		}
	}

	var applied []serviceChange
	for i := range ordered {
		change := ordered[i]
		label := change.name
		if conflicts[change.name] {
			label += " (conflicting)"
			m.logger.Warn("sequenced listener migration (brief gap expected)",
				"service", change.name,
				"add", change.add, "remove", change.remove)
		} else {
			m.logger.Info("reconfiguring listeners", "service", change.name,
				"add", change.add, "remove", change.remove)
		}
		if err := change.server.Reconfigure(ctx, change.newAddr); err != nil {
			if rollbackErr := m.rollbackAppliedListeners(ctx, applied); rollbackErr != nil {
				return fmt.Errorf("reconfigure %s: %w (listener rollback failed: %w)", label, err, rollbackErr)
			}
			return fmt.Errorf("reconfigure %s: %w", label, err)
		}
		applied = append(applied, change)
	}

	return nil
}

func (m *ListenerMigrator) rollbackAppliedListeners(ctx context.Context, applied []serviceChange) error {
	var rollbackErrs []error
	for i := len(applied) - 1; i >= 0; i-- {
		change := applied[i]
		m.logger.Warn("rolling back listener migration", "service", change.name, "addr", change.oldAddr)
		if err := change.server.Reconfigure(ctx, change.oldAddr); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback %s: %w", change.name, err))
		}
	}
	return errors.Join(rollbackErrs...)
}

func (m *ListenerMigrator) buildChange(name string, srv Reconfigurable, newAddrs []string) (serviceChange, bool) {
	oldAddrs := srv.Addresses()
	_, add, remove := listenerDiff(oldAddrs, newAddrs)
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

func listenerDiff(oldAddrs, newAddrs []string) (keep, add, remove []string) {
	oldSet := make(map[string]struct{}, len(oldAddrs))
	for _, a := range oldAddrs {
		oldSet[a] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newAddrs))
	for _, a := range newAddrs {
		newSet[a] = struct{}{}
	}
	for _, a := range newAddrs {
		if _, exists := oldSet[a]; exists {
			keep = append(keep, a)
		} else {
			add = append(add, a)
		}
	}
	for _, a := range oldAddrs {
		if _, exists := newSet[a]; !exists {
			remove = append(remove, a)
		}
	}
	return keep, add, remove
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
