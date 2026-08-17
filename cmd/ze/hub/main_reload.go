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
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/engine"
	zepki "github.com/ze-software/ze/internal/component/pki"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// diskConfigLoaders builds the two config loaders the SIGHUP path runs on:
// the map-only one the plugin server takes as its ConfigLoader (diff + plugin
// reload) and the map-plus-tree one doReload passes to runReload.
//
// Both go through zeconfig.LoadConfig, which is where the config transforms
// live: the ze:validate custom validators, and the ze:bcrypt password hashing.
// The reload therefore needs no branch of its own for either -- the SAME
// function that hashes a plaintext-password at boot hashes it at SIGHUP.
//
// It reads the candidate first, then the active version, then the file, which
// is what makes a store that is blob-only (gokrazy read-only root, ze-test
// tmpfs) reload from a filesystem path at all.
//
// This is a named function rather than a closure inside runYANGConfig so a test
// can drive the loader the daemon actually uses. A test that builds its own
// closure proves only that the test can call LoadConfig.
func diskConfigLoaders(store storage.Storage, configPath string, plugins []string) (
	loadMap func() (map[string]any, error),
	loadBoth func() (map[string]any, *zeconfig.Tree, error),
) {
	readAndParse := func() (*zeconfig.LoadConfigResult, error) {
		var reloadData []byte
		var readErr error
		var hasCandidate bool
		reloadData, _, hasCandidate, readErr = storage.ReadCandidateConfig(store, configPath)
		if readErr == nil && !hasCandidate {
			reloadData, readErr = storage.ReadActiveConfig(store, configPath)
		}
		if readErr != nil {
			reloadData, readErr = os.ReadFile(configPath) //nolint:gosec // daemon operator supplied path
		}
		if readErr != nil {
			return nil, fmt.Errorf("read config: %w", readErr)
		}
		return zeconfig.LoadConfig(string(reloadData), configPath, plugins)
	}
	loadBoth = func() (map[string]any, *zeconfig.Tree, error) {
		result, err := readAndParse()
		if err != nil {
			return nil, nil, err
		}
		return result.Tree.ToMap(), result.Tree, nil
	}
	loadMap = func() (map[string]any, error) {
		m, _, err := loadBoth()
		return m, err
	}
	return loadMap, loadBoth
}

// handleSIGHUPReload is the SIGHUP reload worker. Reads signals from reloadCh,
// triggers plugin-level reload via ReloadFromDisk, refreshes the shared
// ConfigProvider with the freshly loaded tree, then fans Reload out to every
// registered subsystem (engine.Reload) so diff-able knobs hot-apply.
// If a transaction is in progress (lock held), the SIGHUP is queued and replayed
// after the current reload completes.
// Lifecycle goroutine (one-time, runs for daemon lifetime).
//
// It closes done when reloadCh is closed and the reload it was running has
// reported. Shutdown waits on that (awaitReloadWorker), so a SIGTERM racing a
// SIGHUP does not take the verdict with it.
func handleSIGHUPReload(reloadCh <-chan os.Signal, done chan<- struct{}, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, error), lm *listenerMigrator, recorder audit.Recorder) {
	defer close(done)

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

// reloadShutdownGrace bounds how long shutdown waits for the SIGHUP reload
// worker to finish the reload it is running and report the verdict.
//
// It matches the txShutdownGrace the plugin server already spends waiting for
// an in-flight config transaction to unwind (plugin/server/reload.go). That
// wait covers a reload that reached a transaction. It covers nothing before
// one: a reload refused by the config parser, the value validators, the PKI
// load or the provider snapshot decides its verdict in runReload and never
// reaches ReloadConfig at all.
//
// So an operator who sends SIGHUP and then SIGTERM saw "received SIGHUP,
// reloading config..." as the daemon's last word and never learned whether the
// config was applied or refused. test/reload/config-reload-invalid-validator.ci
// is where that surfaced: ze-peer signals SIGTERM a fixed 500ms after SIGHUP
// (pauseForSignal, internal/test/peer/peer.go), and under load the daemon exited
// with the refusal still unprinted. The test asserts the refusal, so it went red
// for the one reason a test must never go red: the daemon dropped the answer.
const reloadShutdownGrace = 3 * time.Second

// awaitReloadWorker waits for the SIGHUP reload worker to drain its channel and
// return, so a reload already running when SIGTERM arrives still reports what it
// decided. A worker that outlasts the grace is left behind rather than holding
// shutdown open, and it says so: a missing verdict with no explanation is the
// thing this whole path exists to remove.
func awaitReloadWorker(done <-chan struct{}, grace time.Duration) {
	select {
	case <-done:
	case <-time.After(grace):
		fmt.Fprintf(os.Stderr, "shutdown: config reload still running after %s, stopping without its result\n", grace)
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
func doReload(s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, error), lm *listenerMigrator) error {
	return doReloadContext(context.Background(), s, eng, cp, store, configPath, load, lm)
}

func doReloadContext(ctx context.Context, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, error), lm *listenerMigrator) error {
	err := runReloadContext(ctx, s, eng, cp, store, configPath, load, lm)
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
func runReload(s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, error), lm *listenerMigrator) error {
	return runReloadContext(context.Background(), s, eng, cp, store, configPath, load, lm)
}

func runReloadContext(ctx context.Context, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, error), lm *listenerMigrator) error {
	reloadCtx, reloadCancel := context.WithTimeout(ctx, 30*time.Second)
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

	// Capture the PKI config the store currently holds, so any failure after the
	// install below can put it back. Without this, a rejected reload would leave
	// the daemon serving the NEW certificates under the OLD config (R-3).
	var priorPKI *zepki.PKIConfig
	if priorProvider != nil {
		var priorErr error
		priorPKI, priorErr = preparePKIConfig(snapshotToLoadedTree(priorProvider))
		if priorErr != nil {
			if clearErr := clearCandidate(); clearErr != nil {
				return fmt.Errorf("reload: snapshot pki config: %w (candidate cleanup failed: %w)", priorErr, clearErr)
			}
			return fmt.Errorf("reload: snapshot pki config: %w", priorErr)
		}
	}

	// Install the new store BEFORE any consumer applies the new config. A
	// reference-style leaf (`certificate <name>` -> store entry) only resolves
	// within its own commit if the store is already the new one when the plugin
	// or subsystem asks (AC-10). Installing it last, as this used to, meant a
	// single commit that added a certificate AND referenced it resolved against
	// the PREVIOUS store and failed.
	if err := zepki.Load(pkiConfig); err != nil {
		if clearErr := clearCandidate(); clearErr != nil {
			return fmt.Errorf("reload: pki config: %w (candidate cleanup failed: %w)", err, clearErr)
		}
		return fmt.Errorf("reload: pki config: %w", err)
	}

	// Reject the commit when the web listener's certificate reference does not
	// resolve against the just-installed store (R-5). Checked here, before any
	// consumer applies, so the reload fails as a whole rather than leaving the
	// listener on a certificate the config no longer describes.
	webCertName := reloadWebCertificate(parsedTree)
	if webCertName != "" {
		if _, _, certErr := zepki.ServerTLSMaterial(webCertName); certErr != nil {
			return restorePKIAfter(priorPKI, clearCandidate,
				fmt.Errorf("reload: environment.web.certificate: %w", certErr))
		}
	}

	if err := s.ReloadConfig(reloadCtx, newTree); err != nil {
		if restoreErr := restorePKI(priorPKI); restoreErr != nil {
			err = fmt.Errorf("%w (pki restore failed: %w)", err, restoreErr)
		}
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
			if rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI); rollbackErr != nil {
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

	// Undo the credentials reloadListeners installs on the running management
	// servers, unless this reload reaches its end. The operator is told a failed
	// reload was rejected and the config is rolled back, so a listener left
	// authenticating against the rejected config is a divergence nothing else
	// repairs: rollbackReload restores config and PKI, and takes no lm.
	//
	// A deferred flag, not a call on each failure path. There are five returns
	// below and every one of them is a rejected reload; hand-placing the undo on
	// each made it a line a later edit can forget, and the promote-candidate
	// path had already been written without it. This way a new failure path
	// inherits the undo instead of needing to remember it.
	restoreAuth := noAuthRestore
	reloadApplied := false
	defer func() {
		if !reloadApplied {
			restoreAuth()
		}
	}()
	if lm != nil && parsedTree != nil {
		undo, err := lm.reloadListeners(reloadCtx, parsedTree)
		restoreAuth = undo
		if err != nil {
			if rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI); rollbackErr != nil {
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

	// The store is already installed (above). Push the possibly-rotated material
	// onto the running web listener, which serves it from the next handshake
	// with no rebind, so open SSE streams survive the rotation (AC-9).
	if lm != nil {
		if err := lm.updateWebCertificate(webCertName); err != nil {
			if rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI); rollbackErr != nil {
				if clearErr := clearCandidate(); clearErr != nil {
					return fmt.Errorf("reload: %w (rollback failed: %w; candidate cleanup failed: %w)", err, rollbackErr, clearErr)
				}
				return fmt.Errorf("reload: %w (rollback failed: %w)", err, rollbackErr)
			}
			if clearErr := clearCandidate(); clearErr != nil {
				return fmt.Errorf("reload: %w (candidate cleanup failed: %w)", err, clearErr)
			}
			return fmt.Errorf("reload: %w", err)
		}
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

	// Authorization changes become live only after every fallible reload step,
	// including candidate promotion, has succeeded. The stable local AAA
	// authorizer dereferences this store on its next decision.
	if parsedTree == nil {
		swapLocalAuthzStore(nil)
	} else {
		swapLocalAuthzStore(infra.ExtractAuthzStore(parsedTree))
	}
	reloadApplied = true
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

func rollbackReload(ctx context.Context, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, prior map[string]map[string]any, priorPKI *zepki.PKIConfig) error {
	if prior == nil {
		return nil
	}
	var rollbackErrs []error
	// Reinstall the prior store FIRST: the plugin and subsystem rollbacks below
	// re-apply the old config, and a consumer resolving a certificate reference
	// during that apply must see the old material, not the rejected commit's
	// (R-3). This mirrors the install-before-apply ordering of the forward path.
	if err := restorePKI(priorPKI); err != nil {
		rollbackErrs = append(rollbackErrs, err)
	}
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

// restorePKI reinstalls a previously captured PKI config. A nil config means
// nothing was captured (no provider snapshot), so there is nothing to undo.
func restorePKI(priorPKI *zepki.PKIConfig) error {
	if priorPKI == nil {
		return nil
	}
	if err := zepki.Load(priorPKI); err != nil {
		return fmt.Errorf("pki restore: %w", err)
	}
	return nil
}

// restorePKIAfter undoes the store install for a failure that happened before
// any consumer applied, so no plugin or subsystem rollback is owed: only the
// store and the candidate need unwinding.
func restorePKIAfter(priorPKI *zepki.PKIConfig, clearCandidate func() error, cause error) error {
	if restoreErr := restorePKI(priorPKI); restoreErr != nil {
		cause = fmt.Errorf("%w (pki restore failed: %w)", cause, restoreErr)
	}
	if clearErr := clearCandidate(); clearErr != nil {
		return fmt.Errorf("%w (candidate cleanup failed: %w)", cause, clearErr)
	}
	return cause
}

// reloadWebCertificate returns the environment.web.certificate name the reloaded
// config asks for. The env var wins, exactly as it does at startup, so a
// deployment that pins the certificate through ze.web.certificate is not
// silently re-pointed by a config edit.
func reloadWebCertificate(parsedTree *zeconfig.Tree) string {
	if name := env.Get("ze.web.certificate"); name != "" {
		return name
	}
	if parsedTree == nil {
		return ""
	}
	// Settings, not addresses: the certificate applies to a listener started by
	// a flag or env var too (learned 1327).
	cfg, ok := zeconfig.ExtractWebSettings(parsedTree)
	if !ok {
		return ""
	}
	return cfg.Certificate
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
