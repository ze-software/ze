// Design: docs/architecture/hub-architecture.md -- SIGHUP config reload orchestration
// Related: main.go -- runYANGConfig starts reload goroutine

package hub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/engine"
	zepki "github.com/ze-software/ze/internal/component/pki"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// handleSIGHUPReload is the SIGHUP reload worker. Reads signals from reloadCh,
// triggers plugin-level reload via ReloadFromDisk, refreshes the shared
// ConfigProvider with the freshly loaded tree, then fans Reload out to every
// registered subsystem (engine.Reload) so diff-able knobs hot-apply.
// If a transaction is in progress (lock held), the SIGHUP is queued and replayed
// after the current reload completes.
// Lifecycle goroutine (one-time, runs for daemon lifetime).
func handleSIGHUPReload(reloadCh <-chan os.Signal, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, error), lm *ListenerMigrator, recorder audit.Recorder) {
	for range reloadCh {
		fmt.Fprintf(os.Stderr, "received SIGHUP, reloading config...\n")
		if err := stageSIGHUPCandidate(store, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "reload error: %v\n", err)
			continue
		}
		if err := doReload(s, eng, cp, store, configPath, load, lm); err != nil {
			if errors.Is(err, pluginserver.ErrReloadInProgress) {
				fmt.Fprintf(os.Stderr, "transaction in progress, queuing SIGHUP...\n")
				s.QueueSIGHUP()
				continue
			}
			fmt.Fprintf(os.Stderr, "reload error: %v\n", err)
		} else {
			recordDaemonReloadAudit(recorder, "system", "signal", audit.System, "SIGHUP")
			reloadComplete()
		}
		// After reload completes, drain any queued SIGHUP.
		if s.DrainSIGHUP() {
			fmt.Fprintf(os.Stderr, "replaying queued SIGHUP...\n")
			if err := stageSIGHUPCandidate(store, configPath); err != nil {
				fmt.Fprintf(os.Stderr, "queued reload error: %v\n", err)
				continue
			}
			if err := doReload(s, eng, cp, store, configPath, load, lm); err != nil {
				fmt.Fprintf(os.Stderr, "queued reload error: %v\n", err)
			} else {
				recordDaemonReloadAudit(recorder, "system", "signal", audit.System, "queued SIGHUP")
				reloadComplete()
			}
		}
	}
}

// reloadCompleteLine is the stable line a finished SIGHUP reload prints on
// stderr. Until it existed a successful reload printed NOTHING, so "received
// SIGHUP, reloading config..." was the daemon's last word whether the reload
// took a millisecond or never finished.
//
// Two consequences, both real:
//   - an operator could not tell a completed reload from a wedged one;
//   - no functional test could fence on completion, so every reload .ci raced
//     its own teardown. Under load the daemon was killed mid-reload, leaving a
//     partial atomic write (a stray `.ze-storage-*` in rollback/) and no
//     meta/config/rollback pointer, which is what made `commit-transactional`
//     and its neighbors look like flaky "load-sensitive" tests rather than a
//     missing signal.
//
// The phrase is "sighup reload complete" and NOT the obvious "reload complete"
// because the latter is a SUBSTRING of an existing log line,
// `logger().Info("config reload completed")` (plugin/server/reload.go). That
// line is emitted INSIDE ReloadConfig -- before applyLoadedTreeToProvider,
// eng.Reload, and critically before storage.PromoteCandidate writes
// meta/config/active and meta/config/rollback. It is suppressed at the default
// WARN level, so a `contains=reload complete` fence would work today purely by
// accident; the first .ci raising the level to info would fence on the EARLIER
// line, tear the daemon down mid-promotion, and reintroduce exactly the race
// this marker exists to kill -- while reporting green.
//
// Keep it stable and keep it non-colliding: .ci tests fence with
// `await=stderr:contains=sighup reload complete` (ai/rules/cli.md --
// one stable phrase per outcome, and no phrase a prefix of another).
const reloadCompleteLine = "sighup reload complete\n"

// reloadComplete announces that a SIGHUP reload finished successfully. BOTH
// SIGHUP reload loops in this binary call it: handleSIGHUPReload (YANG/BGP
// config) and the orchestrator loop in main.go (hub config), which Run selects
// between on zeconfig.ProbeConfigType. A reload path that does not call it is
// invisible to operators and un-fenceable by tests.
func reloadComplete() {
	if _, err := os.Stderr.WriteString(reloadCompleteLine); err != nil {
		return // stderr is gone; there is nowhere left to report it
	}
}

func recordDaemonReloadAudit(recorder audit.Recorder, actor, remoteAddr, surface, detail string) { //nolint:unparam // surface is always System today but callers should state intent
	if recorder == nil {
		return
	}
	if err := recorder.Record(audit.Entry{
		Actor:      actor,
		RemoteAddr: remoteAddr,
		Surface:    surface,
		Action:     audit.ActionDaemonReload,
		Detail:     detail,
		Outcome:    audit.OutcomeSuccess,
	}); err != nil {
		slogutil.Logger("hub.audit").Warn("audit record failed", "action", audit.ActionDaemonReload, "error", err)
	}
}

// doReload performs a single config reload and records the "reload processed"
// fence that `show reload-status` surfaces.
//
// The fence is marked HERE, around the whole of runReload, rather than inside
// pluginserver.Server.reloadConfig, because a reload's rejections are decided
// by the subsystems engine.Reload fans out to (e.g. l2tp refusing a listener
// rebind), which runs after ReloadConfig. Marking at the plugin-server layer
// would advance the fence before those rejections had happened, and an observer
// polling it could read state the reload had not finished touching.
//
// ErrReloadInProgress is deliberately NOT marked: that reload never ran: it is
// queued and replayed by handleSIGHUPReload, and the replay marks it. Marking
// it here would fence an observer on a reload that had not been processed.
func doReload(s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, error), lm *ListenerMigrator) error {
	err := runReload(s, eng, cp, store, configPath, load, lm)
	if s != nil && !errors.Is(err, pluginserver.ErrReloadInProgress) {
		s.MarkReloadProcessed(err == nil)
	}
	return err
}

// runReload performs a single config reload with a 30-second timeout.
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
//
// Callers go through doReload, which wraps this to record the reload fence.
func runReload(s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, error), lm *ListenerMigrator) error {
	reloadCtx, reloadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer reloadCancel()
	candidateSet := false
	if store != nil && configPath != "" && configPath != "-" {
		_, _, ok, err := storage.ReadCandidateConfig(store, configPath)
		if err != nil {
			return fmt.Errorf("reload: read candidate: %w", err)
		}
		candidateSet = ok
	}
	clearCandidate := func() error {
		if !candidateSet {
			return nil
		}
		return storage.ClearCandidate(store, configPath)
	}

	if load == nil {
		// stdin-config daemons have no reload source. Fall back to the
		// plugin server's own ReloadFromDisk (which also errors if no
		// loader is configured) so the error message stays familiar.
		return s.ReloadFromDisk(reloadCtx)
	}

	newTree, parsedTree, loadErr := load()
	if loadErr != nil {
		if clearErr := clearCandidate(); clearErr != nil {
			return fmt.Errorf("reload: parse config: %w (candidate cleanup failed: %w)", loadErr, clearErr)
		}
		return fmt.Errorf("reload: parse config: %w", loadErr)
	}

	pkiConfig, pkiErr := preparePKIConfig(newTree)
	if pkiErr != nil {
		if clearErr := clearCandidate(); clearErr != nil {
			return fmt.Errorf("reload: pki config: %w (candidate cleanup failed: %w)", pkiErr, clearErr)
		}
		return fmt.Errorf("reload: pki config: %w", pkiErr)
	}

	var priorProvider map[string]map[string]any
	if cp != nil {
		var snapErr error
		priorProvider, snapErr = snapshotProvider(cp)
		if snapErr != nil {
			if clearErr := clearCandidate(); clearErr != nil {
				return fmt.Errorf("reload: snapshot config provider: %w (candidate cleanup failed: %w)", snapErr, clearErr)
			}
			return fmt.Errorf("reload: snapshot config provider: %w", snapErr)
		}
	}

	if err := s.ReloadConfig(reloadCtx, newTree); err != nil {
		if clearErr := clearCandidate(); clearErr != nil {
			return fmt.Errorf("%w (candidate cleanup failed: %w)", err, clearErr)
		}
		return err
	}

	if cp != nil {
		applyLoadedTreeToProvider(cp, newTree)
	}

	if eng != nil {
		if err := eng.Reload(reloadCtx); err != nil {
			if rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider); rollbackErr != nil {
				if clearErr := clearCandidate(); clearErr != nil {
					return fmt.Errorf("subsystem reload: %w (rollback failed: %w; candidate cleanup failed: %w)", err, rollbackErr, clearErr)
				}
				return fmt.Errorf("subsystem reload: %w (rollback failed: %w)", err, rollbackErr)
			}
			if clearErr := clearCandidate(); clearErr != nil {
				return fmt.Errorf("subsystem reload: %w (candidate cleanup failed: %w)", err, clearErr)
			}
			return fmt.Errorf("subsystem reload: %w", err)
		}
	}

	if lm != nil && parsedTree != nil {
		if err := lm.ReloadListeners(reloadCtx, parsedTree); err != nil {
			if rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider); rollbackErr != nil {
				if clearErr := clearCandidate(); clearErr != nil {
					return fmt.Errorf("reload: listener migration: %w (rollback failed: %w; candidate cleanup failed: %w)", err, rollbackErr, clearErr)
				}
				return fmt.Errorf("reload: listener migration: %w (rollback failed: %w)", err, rollbackErr)
			}
			if clearErr := clearCandidate(); clearErr != nil {
				return fmt.Errorf("reload: listener migration: %w (candidate cleanup failed: %w)", err, clearErr)
			}
			return fmt.Errorf("reload: listener migration: %w", err)
		}
	}

	if err := zepki.Load(pkiConfig); err != nil {
		if rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider); rollbackErr != nil {
			if clearErr := clearCandidate(); clearErr != nil {
				return fmt.Errorf("reload: pki config: %w (rollback failed: %w; candidate cleanup failed: %w)", err, rollbackErr, clearErr)
			}
			return fmt.Errorf("reload: pki config: %w (rollback failed: %w)", err, rollbackErr)
		}
		if clearErr := clearCandidate(); clearErr != nil {
			return fmt.Errorf("reload: pki config: %w (candidate cleanup failed: %w)", err, clearErr)
		}
		return fmt.Errorf("reload: pki config: %w", err)
	}

	applyHostTuningFromMap(newTree)
	applyConsoleFromMap(newTree)
	applyConntrackFromMap(newTree, s)
	applyUpdateCheckerFromMap(newTree)
	startArchiveScheduler(parsedTree, configPath, store, s)
	applyResolvConf(parsedTree)
	reloadSmartManager(parsedTree)

	if candidateSet {
		if err := storage.PromoteCandidate(store, configPath); err != nil {
			return fmt.Errorf("reload: promote candidate: %w", err)
		}
	}

	return nil
}

func stageSIGHUPCandidate(store storage.Storage, configPath string) error {
	if store == nil || configPath == "" || configPath == "-" {
		return nil
	}
	if _, _, ok, err := storage.ReadCandidateConfig(store, configPath); err != nil || ok {
		if err != nil {
			return err
		}
		return storage.ErrCandidateExists
	}
	data, err := os.ReadFile(configPath) //nolint:gosec // daemon operator supplied path
	if err != nil && storage.IsBlobStorage(store) {
		data, err = store.ReadFile(configPath)
	}
	if err != nil {
		return fmt.Errorf("stage SIGHUP candidate: read config: %w", err)
	}
	if _, err := storage.WriteCandidateVersion(store, configPath, data, time.Now()); err != nil {
		return fmt.Errorf("stage SIGHUP candidate: %w", err)
	}
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
