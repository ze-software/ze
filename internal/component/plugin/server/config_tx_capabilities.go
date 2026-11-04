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
// Verify stages every participant's candidate without changing the live store.
// The last section verifier validates the complete candidate, retaining owners
// that did not replace their declaration. The coordinator's own diff filter
// defines verifiers; operation-only participants owe no section answer.
// The transaction's first apply publishes every staged declaration atomically
// before that participant's apply RPC. A transaction that does not commit
// restores the previous complete set through the same injector boundary.
//
// Safe for concurrent use.
type capabilityStage struct {
	mu        sync.Mutex
	verifiers map[string]bool
	staged    map[string]*plugin.PluginCapabilities
	previous  map[string]*plugin.PluginCapabilities
	installed bool
}

// stageVerifiedCapabilities records one successful section verify, including
// a nil declaration that keeps the plugin's current one. The last verifier
// checks the complete candidate before its verify-ok lets apply begin.
func (b *configTxBridge) stageVerifiedCapabilities(name string, declared *rpc.DeclareCapabilitiesInput) string {
	stage := &b.capabilities
	stage.mu.Lock()
	defer stage.mu.Unlock()
	if _, expected := stage.verifiers[name]; !expected {
		return "plugin " + name + " answered section verify without belonging to the transaction's verifier set"
	}
	stage.verifiers[name] = true
	if declared != nil {
		if b.server.capInjector == nil {
			return "plugin " + name + " declared capabilities, and this engine holds no capability store to install them in"
		}
		caps := capabilitiesFromRPC(declared)
		caps.PluginName = name
		if stage.staged == nil {
			stage.staged = make(map[string]*plugin.PluginCapabilities)
		}
		stage.staged[name] = caps
	}
	for _, answered := range stage.verifiers {
		if !answered {
			return ""
		}
	}
	if len(stage.staged) == 0 {
		return ""
	}
	if err := b.server.capInjector.CheckReplacePluginCapabilities(stage.staged); err != nil {
		return err.Error()
	}
	return ""
}

// installStagedCapabilities installs every staged declaration in place of the
// plugin's current one, once per transaction: the first apply calls it and
// every later apply finds it done. It replaces the store and each plugin's
// process record. A validation failure leaves the live store untouched, so no
// reader can observe a partially installed candidate even when apply fails.
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
		stage.previous[name] = nil
		if proc := b.lookupProcess(name); proc != nil {
			stage.previous[name] = proc.Capabilities()
		}
	}
	if err := injector.ReplacePluginCapabilities(stage.staged); err != nil {
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
	if err := b.server.capInjector.ReplacePluginCapabilities(stage.previous); err != nil {
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

// sortedCapabilityPlugins names the plugins of a declaration map in order for
// stable process-record updates.
func sortedCapabilityPlugins(declarations map[string]*plugin.PluginCapabilities) []string {
	names := make([]string, 0, len(declarations))
	for name := range declarations {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
