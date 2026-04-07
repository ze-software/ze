// Design: docs/architecture/config/transaction-protocol.md -- reload to TxCoordinator wiring
// Related: reload.go -- hub flow (lock, diff, auto-load/stop, commit)
// Related: config_tx_bridge.go -- engine-side RPC bridge for per-plugin verify/apply events
// Related: engine_event_gateway.go -- gateway the orchestrator publishes on

package server

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"codeberg.org/thomas-mangin/ze/internal/component/config/transaction"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
)

// runTxCoordinator runs the transaction orchestrator for a reload once the
// caller has computed the affected plugins and the raw diff. It builds
// participants from the affected plugin registrations, converts the diff
// into per-root DiffSection slices the orchestrator's filterDiffs walks,
// starts an RPC bridge so the stream-based orchestrator can reach the
// plugin SDK callbacks that still speak RPC, and blocks on
// TxCoordinator.Execute.
//
// Returns nil on commit. Returns a non-nil error on abort (verify failed),
// rollback (apply failed), or gateway misconfiguration. Callers use the
// returned error verbatim as the reload error so operators see the same
// message they did on the legacy RPC loop.
func (s *Server) runTxCoordinator(ctx context.Context, affected []affectedPlugin, diff *configDiff) error {
	if len(affected) == 0 {
		return nil
	}

	participants, diffs, err := buildTxInputs(affected, diff)
	if err != nil {
		return fmt.Errorf("build transaction inputs: %w", err)
	}

	gateway := NewConfigEventGateway(s)
	bridge := newConfigTxBridge(s, gateway, participantNames(participants))
	if err := bridge.Subscribe(ctx); err != nil {
		return fmt.Errorf("config tx bridge subscribe: %w", err)
	}
	defer bridge.Close()

	coordinator, err := transaction.NewTxCoordinator(gateway, participants, s.restartPluginFn())
	if err != nil {
		return fmt.Errorf("create transaction coordinator: %w", err)
	}

	result := coordinator.Execute(ctx, diffs)
	return txResultToError(result)
}

// buildTxInputs turns the affected plugin list into the typed participant
// slice and the diff map the orchestrator expects. Participants come from
// the affected plugin registrations; the diff map is indexed by top-level
// root so filterDiffs in the orchestrator can dispatch only the relevant
// sections to each participant. Diff sections are taken straight from the
// same buildDiffSections helper the legacy reload path used, so the shape
// of Added/Removed/Changed is identical between the two paths.
//
// Wildcard config roots (["*"]) are expanded to the concrete list of roots
// present in the diff, because the orchestrator's filterDiffs does exact
// match lookups and has no wildcard awareness. Expanding here keeps the
// orchestrator simple and matches the legacy reload.go semantics where a
// wildcard plugin received every changed root.
func buildTxInputs(affected []affectedPlugin, diff *configDiff) ([]transaction.Participant, map[string][]transaction.DiffSection, error) {
	diffMap := make(map[string][]transaction.DiffSection)
	for _, section := range buildDiffSections(diff) {
		diffMap[section.Root] = append(diffMap[section.Root], transaction.DiffSection{
			Root:    section.Root,
			Added:   section.Added,
			Removed: section.Removed,
			Changed: section.Changed,
		})
	}
	allRoots := make([]string, 0, len(diffMap))
	for root := range diffMap {
		allRoots = append(allRoots, root)
	}

	participants := make([]transaction.Participant, 0, len(affected))
	for _, ap := range affected {
		reg := ap.proc.Registration()
		if reg == nil {
			return nil, nil, fmt.Errorf("plugin %q has no registration", ap.proc.Name())
		}
		if err := transaction.ValidatePluginName(ap.proc.Name()); err != nil {
			return nil, nil, fmt.Errorf("plugin %q: %w", ap.proc.Name(), err)
		}
		roots := expandWildcardRoots(reg.WantsConfigRoots, allRoots)
		participants = append(participants, transaction.Participant{
			Name:         ap.proc.Name(),
			ConfigRoots:  roots,
			VerifyBudget: reg.VerifyBudget,
			ApplyBudget:  reg.ApplyBudget,
		})
	}

	return participants, diffMap, nil
}

// expandWildcardRoots replaces a "*" entry in the plugin's declared roots
// with the concrete list of roots that actually changed this transaction.
// Plugins with explicit roots are copied verbatim so the orchestrator sees
// exactly the roots the plugin registered interest in.
func expandWildcardRoots(declared, allRoots []string) []string {
	if slices.Contains(declared, "*") {
		out := make([]string, len(allRoots))
		copy(out, allRoots)
		return out
	}
	out := make([]string, len(declared))
	copy(out, declared)
	return out
}

// participantNames projects participant names for the RPC bridge. Kept as
// a helper so the caller does not hand the bridge the full participant slice
// (it only needs names; decoupling the two keeps the bridge simple).
func participantNames(participants []transaction.Participant) []string {
	names := make([]string, len(participants))
	for i, p := range participants {
		names[i] = p.Name
	}
	return names
}

// txResultToError converts a TxResult into the error shape reload.go's
// callers expect. StateCommitted maps to nil; StateAborted and
// StateRolledBack wrap the coordinator's error with the legacy prefixes
// so test assertions on error substrings ("config verify failed",
// "config apply") keep working.
func txResultToError(result *transaction.TxResult) error {
	if result == nil {
		return errors.New("transaction coordinator returned nil result")
	}
	if result.State == transaction.StateCommitted {
		return nil
	}
	if result.State == transaction.StateAborted {
		return fmt.Errorf("config verify failed: %w", result.Err)
	}
	if result.State == transaction.StateRolledBack {
		return fmt.Errorf("config apply partial failure: %w", result.Err)
	}
	if result.Err != nil {
		return fmt.Errorf("config transaction %s: %w", result.State, result.Err)
	}
	return fmt.Errorf("config transaction ended in unexpected state %q", result.State)
}

// restartPluginFn returns a RestartFunc that delegates plugin restart to the
// Server's spawner, or nil if no spawner is wired (tests). Nil is acceptable
// to NewTxCoordinator; the orchestrator skips the restart step when the
// function is nil.
func (s *Server) restartPluginFn() transaction.RestartFunc {
	if s.spawner == nil {
		return nil
	}
	return func(pluginName string) error {
		return s.restartPlugin(pluginName)
	}
}

// restartPlugin performs a best-effort restart of a broken plugin. Called by
// the orchestrator when a rollback ack reports CodeBroken. The spawner owns
// the respawn logic; the bridge only decides WHEN to restart.
func (s *Server) restartPlugin(pluginName string) error {
	if s.spawner == nil {
		return fmt.Errorf("no plugin spawner available to restart %s", pluginName)
	}
	pm := s.procManager.Load()
	if pm == nil {
		return fmt.Errorf("no process manager available to restart %s", pluginName)
	}
	if err := pm.Respawn(pluginName); err != nil {
		return fmt.Errorf("respawn %s: %w", pluginName, err)
	}
	return nil
}

// Silence the unused import for process when the package-level var block is
// the only usage site. The bridge and buildTxInputs both inspect
// *process.Process via methods, so the import is genuinely needed at build
// time even though nothing here constructs one.
var _ = (*process.Process)(nil)
