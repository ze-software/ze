// Design: docs/architecture/hub-architecture.md — hub coordination

package hub

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

var reloadLogger = slogutil.LazyLogger("hub.reload")

// diffPluginDefs computes the difference between old and new plugin definitions.
// Returns: added (new PluginDefs), removed (names of old-only plugins), kept (names in both).
func diffPluginDefs(old, new []PluginDef) (added []PluginDef, removed, kept []string) {
	oldByName := make(map[string]struct{}, len(old))
	for _, p := range old {
		oldByName[p.Name] = struct{}{}
	}

	newByName := make(map[string]PluginDef, len(new))
	for _, p := range new {
		newByName[p.Name] = p
	}

	// Find removed and kept
	for _, p := range old {
		if _, exists := newByName[p.Name]; exists {
			kept = append(kept, p.Name)
		} else {
			removed = append(removed, p.Name)
		}
	}

	// Find added
	for _, p := range new {
		if _, exists := oldByName[p.Name]; !exists {
			added = append(added, p)
		}
	}

	return added, removed, kept
}

// envChanged reports whether the env maps differ.
func envChanged(old, new map[string]string) bool {
	if len(old) != len(new) {
		return true
	}
	for k, v := range old {
		if new[k] != v {
			return true
		}
	}
	return false
}

// Reload re-reads the config file and applies plugin changes.
// On error (file unreadable, parse error), the running config is preserved.
func (o *Orchestrator) Reload(configPath string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Read and parse new config from disk.
	data, err := os.ReadFile(configPath) //nolint:gosec // Config path from trusted source
	if err != nil {
		return fmt.Errorf("reload: read config: %w", err)
	}

	newCfg, err := ParseHubConfig(string(data))
	if err != nil {
		return fmt.Errorf("reload: parse config: %w", err)
	}

	// Check for env changes (require restart, cannot apply at runtime).
	if envChanged(o.config.Env, newCfg.Env) {
		reloadLogger().Warn("env changes detected; env changes require restart")
	}

	// Compute plugin diff.
	added, removed, kept := diffPluginDefs(o.config.Plugins, newCfg.Plugins)

	if len(added) == 0 && len(removed) == 0 {
		reloadLogger().Info("reload: no plugin changes")
	}

	// Start added plugins BEFORE stopping removed ones.
	// If any fail, we abort the reload without having disrupted running plugins.
	var started []string
	for _, p := range added {
		reloadLogger().Info("reload: registering new plugin", slog.String("plugin", p.Name))
		o.subsystems.Register(plugin.SubsystemConfig{
			Name:       p.Name,
			Binary:     p.Run,
			ConfigPath: configPath,
		})
		if o.ctx != nil {
			handler := o.subsystems.Get(p.Name)
			if handler != nil {
				if err := handler.Start(o.ctx); err != nil {
					reloadLogger().Error("reload: failed to start plugin",
						slog.String("plugin", p.Name), slog.String("error", err.Error()))
					// Roll back: unregister all added plugins.
					for _, name := range started {
						o.subsystems.Unregister(name)
					}
					o.subsystems.Unregister(p.Name)
					return fmt.Errorf("reload: start plugin %s: %w", p.Name, err)
				}
			}
		}
		started = append(started, p.Name)
	}

	// All new plugins started — now safe to stop removed ones.
	for _, name := range removed {
		reloadLogger().Info("reload: stopping removed plugin", slog.String("plugin", name))
		o.subsystems.Unregister(name)
	}

	// Forward SIGHUP to kept (unchanged) children so they reload their own config.
	for _, name := range kept {
		handler := o.subsystems.Get(name)
		if handler == nil {
			continue
		}
		if err := handler.Signal(syscall.SIGHUP); err != nil {
			reloadLogger().Debug("reload: signal forwarding",
				slog.String("plugin", name), slog.String("error", err.Error()))
		}
	}

	// Update stored config — preserve env (env changes require restart).
	newCfg.Env = o.config.Env
	newCfg.ConfigPath = configPath
	o.config = newCfg

	return nil
}
