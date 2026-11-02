// Design: docs/architecture/api/process-protocol.md -- a reload re-sends the capability declaration
// Overview: config_tx_bridge.go -- the transaction bridge that stages and installs it
// Related: startup.go -- capabilitiesFromRPC, the Stage 3 conversion this reuses

package server

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// capabilityStage holds the capability declarations a transaction's verify
// answers carried, and what installing them replaced.
//
// Verify stages a participant's candidate declaration after checking it
// against the store, so nothing the engine reads changes before apply. The
// transaction's first apply installs every staged declaration at once, before
// that participant's apply RPC, so a peer the reload adds builds its OPEN from
// the new declaration. A transaction that does not commit restores the
// previous declarations.
//
// Safe for concurrent use.
type capabilityStage struct {
	mu        sync.Mutex
	staged    map[string]*plugin.PluginCapabilities
	previous  map[string]*plugin.PluginCapabilities
	installed bool
}

// stageVerifiedCapabilities converts the declaration a verify answer carried
// and checks it against the store. A nil declaration keeps the plugin's
// current one and stages nothing. The returned text is the refusal the
// participant's verify answers with.
func (b *configTxBridge) stageVerifiedCapabilities(name string, declared *rpc.DeclareCapabilitiesInput) string {
	if declared == nil {
		return ""
	}
	injector := b.server.capInjector
	if injector == nil {
		return "plugin " + name + " declared capabilities, and this engine holds no capability store to install them in"
	}
	caps := capabilitiesFromRPC(declared)
	caps.PluginName = name
	if err := injector.CheckReplacePluginCapabilities(caps); err != nil {
		return "plugin " + name + ": " + err.Error()
	}
	b.capabilities.mu.Lock()
	defer b.capabilities.mu.Unlock()
	if b.capabilities.staged == nil {
		b.capabilities.staged = make(map[string]*plugin.PluginCapabilities)
	}
	b.capabilities.staged[name] = caps
	return ""
}

// installStagedCapabilities installs every staged declaration in place of the
// plugin's current one, once per transaction: the first apply calls it and
// every later apply finds it done. It replaces the store and each plugin's
// process record. When two staged declarations conflict with each other, it
// puts the previous declarations back and returns the conflict, so the apply
// fails and the transaction rolls back.
func (b *configTxBridge) installStagedCapabilities() error {
	stage := &b.capabilities
	stage.mu.Lock()
	defer stage.mu.Unlock()
	if stage.installed {
		return nil
	}
	if len(stage.staged) == 0 {
		return nil
	}
	injector := b.server.capInjector
	if injector == nil {
		return errors.New("capability declarations are staged, and this engine holds no capability store to install them in")
	}

	names := sortedCapabilityPlugins(stage.staged)
	stage.previous = make(map[string]*plugin.PluginCapabilities, len(names))
	for _, name := range names {
		if proc := b.lookupProcess(name); proc != nil {
			stage.previous[name] = proc.Capabilities()
		}
	}
	if err := swapCapabilities(injector, names, stage.staged); err != nil {
		if restoreErr := swapCapabilities(injector, names, stage.previous); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore previous capabilities: %w", restoreErr))
		}
		return err
	}
	stage.installed = true
	b.recordCapabilities(names, stage.staged)
	return nil
}

// restoreCapabilities puts back the declarations installStagedCapabilities
// replaced. runTxCoordinator calls it when the transaction does not commit; it
// does nothing when nothing was installed.
func (b *configTxBridge) restoreCapabilities() error {
	stage := &b.capabilities
	stage.mu.Lock()
	defer stage.mu.Unlock()
	if !stage.installed {
		return nil
	}
	names := sortedCapabilityPlugins(stage.staged)
	if err := swapCapabilities(b.server.capInjector, names, stage.previous); err != nil {
		return fmt.Errorf("restore previous capabilities: %w", err)
	}
	stage.installed = false
	b.recordCapabilities(names, stage.previous)
	return nil
}

// recordCapabilities sets each plugin's process record to the declaration the
// store now holds for it, the record the next reload restores from.
func (b *configTxBridge) recordCapabilities(names []string, declarations map[string]*plugin.PluginCapabilities) {
	for _, name := range names {
		if proc := b.lookupProcess(name); proc != nil {
			proc.SetCapabilities(declarations[name])
		}
	}
}

// swapCapabilities removes every named plugin's entries, then adds each one's
// declaration from declarations, a missing or nil one adding nothing. Removing
// all of them first lets a code move from one of the plugins to another
// without a conflict against the entry that is leaving.
func swapCapabilities(injector *plugin.CapabilityInjector, names []string, declarations map[string]*plugin.PluginCapabilities) error {
	for _, name := range names {
		injector.RemovePluginCapabilities(name)
	}
	var errs []error
	for _, name := range names {
		caps := declarations[name]
		if caps == nil {
			continue
		}
		if err := injector.AddPluginCapabilities(caps); err != nil {
			errs = append(errs, fmt.Errorf("plugin %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

// sortedCapabilityPlugins names the plugins of a declaration map in order, so
// a conflict between two of them always names the same pair.
func sortedCapabilityPlugins(declarations map[string]*plugin.PluginCapabilities) []string {
	names := make([]string, 0, len(declarations))
	for name := range declarations {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
