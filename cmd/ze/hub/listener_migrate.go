// Design: docs/architecture/web-interface.md -- Graceful listener migration on config reload
// Related: main_reload.go -- doReload calls ListenerMigrator.ReloadListeners

package hub

import (
	"context"
	"fmt"
	"log/slog"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
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
	logger *slog.Logger
}

// NewListenerMigrator creates a migrator. Pass nil for services that are not running.
func NewListenerMigrator(web *zeweb.WebServer) *ListenerMigrator {
	return &ListenerMigrator{
		web:    web,
		logger: slogutil.Logger("hub.listener"),
	}
}

// SetWeb updates the web server reference (e.g., after a fresh start on reload).
func (m *ListenerMigrator) SetWeb(web *zeweb.WebServer) {
	m.web = web
}

// ReloadListeners extracts new listen configs from the config tree and
// reconfigures running services. Returns nil if no listener changes are needed.
func (m *ListenerMigrator) ReloadListeners(ctx context.Context, tree *zeconfig.Tree) error {
	var changes []serviceChange

	if m.web != nil {
		if webCfg, ok := zeconfig.ExtractWebConfig(tree); ok {
			newAddrs := endpointsToAddrs(webCfg.Servers)
			oldAddrs := m.web.Addresses()
			_, add, remove := zeweb.ListenerDiff(oldAddrs, newAddrs)
			if len(add) > 0 || len(remove) > 0 {
				changes = append(changes, serviceChange{
					name:    "web",
					server:  m.web,
					oldAddr: oldAddrs,
					newAddr: newAddrs,
					add:     add,
					remove:  remove,
				})
			}
		}
	}

	if len(changes) == 0 {
		return nil
	}

	conflicts := detectConflicts(changes)

	// Phase 1: non-conflicting services migrate (bind new, then close old).
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

	// Phase 2: conflicting services need sequenced release.
	// For each conflicting service, we must close old listeners first (to free
	// the address for the acquiring service), then bind new ones.
	// This means a brief gap on the conflicting address.
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
