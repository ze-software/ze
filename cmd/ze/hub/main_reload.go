// Design: docs/architecture/hub-architecture.md -- SIGHUP config reload orchestration
// Related: main.go -- runYANGConfig starts reload goroutine

package hub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/engine"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// handleSIGHUPReload is the SIGHUP reload worker. Reads signals from reloadCh,
// triggers plugin-level reload via ReloadFromDisk, refreshes the shared
// ConfigProvider with the freshly loaded tree, then fans Reload out to every
// registered subsystem (engine.Reload) so diff-able knobs hot-apply.
// If a transaction is in progress (lock held), the SIGHUP is queued and replayed
// after the current reload completes.
// Lifecycle goroutine (one-time, runs for daemon lifetime).
func handleSIGHUPReload(reloadCh <-chan os.Signal, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, load func() (map[string]any, error)) {
	for range reloadCh {
		fmt.Fprintf(os.Stderr, "received SIGHUP, reloading config...\n")
		if err := doReload(s, eng, cp, load); err != nil {
			if errors.Is(err, pluginserver.ErrReloadInProgress) {
				fmt.Fprintf(os.Stderr, "transaction in progress, queuing SIGHUP...\n")
				s.QueueSIGHUP()
				continue
			}
			fmt.Fprintf(os.Stderr, "reload error: %v\n", err)
		}
		// After reload completes, drain any queued SIGHUP.
		if s.DrainSIGHUP() {
			fmt.Fprintf(os.Stderr, "replaying queued SIGHUP...\n")
			if err := doReload(s, eng, cp, load); err != nil {
				fmt.Fprintf(os.Stderr, "queued reload error: %v\n", err)
			}
		}
	}
}

// doReload performs a single config reload with a 30-second timeout.
//
// The load/plugin-apply/provider-refresh/subsystem-reload sequence runs in
// lock-step from a SINGLE tree snapshot:
//
//  1. load() reads and parses the config file once.
//  2. ReloadConfig(ctx, newTree) drives the plugin-server diff + plugin
//     verify/apply path with that tree (public API that accepts a
//     pre-parsed tree, so we don't re-read the file).
//  3. The shared ConfigProvider is refreshed root-by-root from the same
//     tree: new/changed roots are SetRoot'd, orphan roots (present in
//     cp but absent from newTree) get an empty map so watchers see the
//     removal.
//  4. engine.Reload(ctx) fans the refreshed provider out to every
//     registered subsystem so they hot-apply diff-able knobs (e.g.,
//     l2tp shared-secret / hello-interval).
//
// Keeping the tree single-sourced eliminates the race where the file
// changes between the plugin-server read and the subsystem read, and
// avoids redundant I/O + YANG parse on every SIGHUP.
func doReload(s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, load func() (map[string]any, error)) error {
	reloadCtx, reloadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer reloadCancel()

	if load == nil {
		// stdin-config daemons have no reload source. Fall back to the
		// plugin server's own ReloadFromDisk (which also errors if no
		// loader is configured) so the error message stays familiar.
		return s.ReloadFromDisk(reloadCtx)
	}

	newTree, loadErr := load()
	if loadErr != nil {
		return fmt.Errorf("reload: parse config: %w", loadErr)
	}
	var priorProvider map[string]map[string]any
	if cp != nil {
		var snapErr error
		priorProvider, snapErr = snapshotProvider(cp)
		if snapErr != nil {
			return fmt.Errorf("reload: snapshot config provider: %w", snapErr)
		}
	}

	if err := s.ReloadConfig(reloadCtx, newTree); err != nil {
		return err
	}

	if cp != nil {
		applyLoadedTreeToProvider(cp, newTree)
	}

	if eng != nil {
		if err := eng.Reload(reloadCtx); err != nil {
			if rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider); rollbackErr != nil {
				return fmt.Errorf("subsystem reload: %w (rollback failed: %w)", err, rollbackErr)
			}
			return fmt.Errorf("subsystem reload: %w", err)
		}
	}

	applyHostTuningFromMap(newTree)
	applyConsoleFromMap(newTree)
	applyConntrackFromMap(newTree, s)
	applyUpdateCheckerFromMap(newTree)

	return nil
}

func snapshotProvider(cp *zeconfig.Provider) (map[string]map[string]any, error) {
	snapshot := make(map[string]map[string]any)
	for _, root := range cp.Roots() {
		tree, err := cp.Get(root)
		if err != nil {
			return nil, fmt.Errorf("root %s: %w", root, err)
		}
		snapshot[root] = cloneStringAnyMap(tree)
	}
	return snapshot, nil
}

func applyLoadedTreeToProvider(cp *zeconfig.Provider, tree map[string]any) {
	existing := rootsSet(cp.Roots())
	for root, subtree := range tree {
		sub, ok := subtree.(map[string]any)
		if !ok {
			continue
		}
		cp.SetRoot(root, cloneStringAnyMap(sub))
		delete(existing, root)
	}
	// Any root left in `existing` disappeared from the new tree. DeleteRoot
	// removes the entry entirely so the next reload does not re-run the orphan
	// path for the same root and re-fire watcher notifications.
	for orphan := range existing {
		cp.DeleteRoot(orphan)
	}
}

func restoreProviderSnapshot(cp *zeconfig.Provider, snapshot map[string]map[string]any) {
	existing := rootsSet(cp.Roots())
	for root, subtree := range snapshot {
		cp.SetRoot(root, cloneStringAnyMap(subtree))
		delete(existing, root)
	}
	for orphan := range existing {
		cp.DeleteRoot(orphan)
	}
}

func rollbackReload(ctx context.Context, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, prior map[string]map[string]any) error {
	if prior == nil {
		return nil
	}
	var rollbackErrs []error
	if s != nil {
		if err := s.ReloadConfig(ctx, snapshotToLoadedTree(prior)); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("plugin rollback: %w", err))
		}
	}
	if cp != nil {
		restoreProviderSnapshot(cp, prior)
	}
	if eng != nil && cp != nil {
		if err := eng.Reload(ctx); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("subsystem rollback: %w", err))
		}
	}
	return errors.Join(rollbackErrs...)
}

func rootsSet(roots []string) map[string]struct{} {
	set := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		set[root] = struct{}{}
	}
	return set
}

func snapshotToLoadedTree(snapshot map[string]map[string]any) map[string]any {
	out := make(map[string]any, len(snapshot))
	for root, subtree := range snapshot {
		out[root] = cloneStringAnyMap(subtree)
	}
	return out
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneStringAnyMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneAny(item)
		}
		return out
	case []string:
		out := make([]string, len(typed))
		copy(out, typed)
		return out
	default:
		return typed
	}
}
