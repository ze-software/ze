// Design: docs/architecture/hub-architecture.md — hub coordination

package hub

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var reloadLogger = slogutil.LazyLogger("hub.reload")

// diffPluginDefs computes the difference between old and new plugin definitions.
// Returns: added (new PluginDefs), removed (names of old-only plugins), kept
// (same definitions in both). A same-name definition with different fields is
// returned as both removed and added so reload restarts it.
func diffPluginDefs(old, new []PluginDef) (added []PluginDef, removed, kept []string) {
	oldByName := make(map[string]PluginDef, len(old))
	for _, p := range old {
		oldByName[p.Name] = p
	}

	newByName := make(map[string]PluginDef, len(new))
	for _, p := range new {
		newByName[p.Name] = p
	}

	// Find removed and kept
	for _, p := range old {
		if newDef, exists := newByName[p.Name]; exists {
			if !pluginDefEqual(p, newDef) {
				removed = append(removed, p.Name)
				continue
			}
			kept = append(kept, p.Name)
		} else {
			removed = append(removed, p.Name)
		}
	}

	// Find added
	for _, p := range new {
		oldDef, exists := oldByName[p.Name]
		if !exists || !pluginDefEqual(oldDef, p) {
			added = append(added, p)
		}
	}

	return added, removed, kept
}

func pluginDefEqual(a, b PluginDef) bool {
	return a.Name == b.Name && a.Run == b.Run
}

func splitChangedPlugins(added []PluginDef, removed []string) (addedOnly []PluginDef, removedOnly []string, changed []PluginDef) {
	removedSet := make(map[string]bool, len(removed))
	for _, name := range removed {
		removedSet[name] = true
	}

	changedSet := make(map[string]bool)
	for _, p := range added {
		if removedSet[p.Name] {
			changed = append(changed, p)
			changedSet[p.Name] = true
			continue
		}
		addedOnly = append(addedOnly, p)
	}

	for _, name := range removed {
		if !changedSet[name] {
			removedOnly = append(removedOnly, name)
		}
	}
	return addedOnly, removedOnly, changed
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

	// Read and parse new config via storage (filesystem or blob). When the
	// store is blob-backed (e.g., gokrazy read-only root, ze-test tmpfs)
	// fall back to a direct filesystem read. Without this fallback, SIGHUP
	// reload fails with "read file/active/<name>: file does not exist" on
	// any daemon started with a filesystem path that is not a blob key.
	// Mirrors the initial-load fallback in cmd/ze/hub/main.go:107-113.
	data, err := o.store.ReadFile(configPath)
	if err != nil && storage.IsBlobStorage(o.store) {
		data, err = os.ReadFile(configPath) //nolint:gosec // operator-supplied path
	}
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
	addedOnly, removedOnly, changed := splitChangedPlugins(added, removed)

	if len(added) == 0 && len(removed) == 0 {
		reloadLogger().Info("reload: no plugin changes")
	}

	startPlugin := func(p PluginDef) error {
		reloadLogger().Info("reload: registering plugin", slog.String("plugin", p.Name))
		o.subsystems.Register(subsystemConfig(p, configPath))
		if o.ctx == nil {
			return nil
		}
		handler := o.subsystems.Get(p.Name)
		if handler == nil {
			return nil
		}
		if err := handler.Start(o.ctx); err != nil {
			reloadLogger().Error("reload: failed to start plugin",
				slog.String("plugin", p.Name), slog.String("error", err.Error()))
			o.subsystems.Unregister(p.Name)
			return fmt.Errorf("reload: start plugin %s: %w", p.Name, err)
		}
		return nil
	}

	startReplacement := func(p PluginDef) (*pluginserver.SubsystemHandler, error) {
		reloadLogger().Info("reload: starting changed plugin replacement", slog.String("plugin", p.Name))
		handler := pluginserver.NewSubsystemHandler(subsystemConfig(p, configPath))
		if o.ctx == nil {
			return handler, nil
		}
		if err := handler.Start(o.ctx); err != nil {
			handler.Stop()
			reloadLogger().Error("reload: failed to start plugin",
				slog.String("plugin", p.Name), slog.String("error", err.Error()))
			return nil, fmt.Errorf("reload: start plugin %s: %w", p.Name, err)
		}
		return handler, nil
	}

	type replacement struct {
		def     PluginDef
		handler *pluginserver.SubsystemHandler
	}
	var replacements []replacement
	var started []string

	rollbackStarted := func() {
		for _, repl := range replacements {
			repl.handler.Stop()
		}
		for _, name := range started {
			o.subsystems.Unregister(name)
		}
	}

	// Start added plugins BEFORE stopping removed ones.
	// If any fail, we abort the reload without having disrupted running plugins.
	for _, p := range addedOnly {
		if err := startPlugin(p); err != nil {
			rollbackStarted()
			return err
		}
		started = append(started, p.Name)
	}
	// Start changed-plugin replacements before unregistering the old handlers.
	// If any replacement fails, the old plugin remains registered and running.
	for _, p := range changed {
		handler, err := startReplacement(p)
		if err != nil {
			rollbackStarted()
			return err
		}
		replacements = append(replacements, replacement{def: p, handler: handler})
	}
	for _, repl := range replacements {
		reloadLogger().Info("reload: replacing changed plugin", slog.String("plugin", repl.def.Name))
		o.subsystems.Replace(repl.def.Name, repl.handler)
	}

	// All new plugins started — now safe to stop removed ones.
	for _, name := range removedOnly {
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

func subsystemConfig(p PluginDef, configPath string) pluginserver.SubsystemConfig {
	return pluginserver.SubsystemConfig{
		Name:       p.Name,
		Binary:     p.Run,
		ConfigPath: configPath,
	}
}
