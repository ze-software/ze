// Design: docs/architecture/hub-architecture.md -- SIGHUP config reload orchestration
// Related: main.go -- runYANGConfig starts reload goroutine

package hub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/component/kernelcap"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/opid"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// reloadSelection is the candidate plugin selection LoadConfig judged and
// diskConfigLoaders passes beside the parsed tree to Server.ReloadConfigWithSelection.
type reloadSelection = []zePlugin.PluginConfig

// diskConfigLoaders builds the two config loaders the SIGHUP path runs on:
// the map-plus-selection one the plugin server takes as its ConfigLoader (diff
// + plugin reload) and the map-plus-tree-plus-selection one doReloadContext
// passes to runReloadContext.
//
// Both go through zeconfig.LoadConfig, which is where the config transforms
// live: the ze:validate custom validators, data-plane verdict, and ze:bcrypt
// password hashing. LoadConfig keeps the exact candidate plugin selection its
// verdict judged (LoadConfigResult.CandidateSelection), and both loaders carry
// that selection unchanged beside the tree to pluginserver.Server so reload's
// writer judgement reads the verdict's own selection and never boot-time
// s.config.Plugins.
//
// Candidate reads precede the explicitly selected source. Read errors are fatal,
// never a reason to choose another authority.
//
// started holds the external plugins the daemon booted with, as the first pass
// answered them (zeconfig.FirstPass, External). Every reload reads the config
// through this loader, so it is where a reload whose config declares other
// external plugins is refused, whole, before the full parse: a plugin, its
// declaration and its config schema change only when ze restarts (owner
// ruling), while every other part of the config reloads. plugins is the
// command line's list, the same for boot and for every reload, so a
// command-line plugin never differs.
//
// This is a named function rather than a closure inside runYANGConfig so a test
// can drive the loader the daemon actually uses. A test that builds its own
// closure proves only that the test can call LoadConfig.
func diskConfigLoaders(store storage.Storage, configPath string, plugins []string, started []zePlugin.PluginConfig) (
	loadMap pluginserver.ConfigLoader,
	loadBoth func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error),
) {
	readAndParse := func() (*zeconfig.LoadConfigResult, error) {
		reloadData, readErr := storage.ReadReloadConfig(store, configPath)
		if readErr != nil {
			return nil, fmt.Errorf("read config: %w", readErr)
		}
		text := string(reloadData)
		// The declarations are compared before the full parse: a config that
		// adds a plugin and a section of that plugin is refused with the
		// restart, where the full parse would refuse the section as a name no
		// schema defines.
		firstPass, firstPassErr := zeconfig.FirstPass(text, configPath, plugins)
		if firstPassErr == nil {
			if refusal := pluginserver.RefuseChangedDeclarations(started, firstPass.External); refusal != nil {
				return nil, refusal
			}
		}
		result, err := zeconfig.LoadConfig(text, configPath, plugins)
		if err != nil {
			// A first pass that refused the text leaves the verdict to this
			// parse, which refuses the same `plugin` or `environment` block
			// with the message it always gave.
			return nil, err
		}
		if firstPassErr != nil {
			return nil, unreadDeclarationsRefusal(firstPassErr)
		}
		if !result.HasCandidateSelection {
			return nil, errors.New("loaded config carries no judged plugin selection")
		}
		return result, nil
	}
	loadBoth = func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error) {
		result, err := readAndParse()
		if err != nil {
			return nil, nil, nil, err
		}
		// The running authority retains inactive intent as well as list order.
		// Plugin sections project the same map back to active-only data.
		return result.Tree.ToRunningMap(), result.Tree, result.CandidateSelection, nil
	}
	loadMap = func() (map[string]any, []zePlugin.PluginConfig, error) {
		m, _, selection, err := loadBoth()
		return m, selection, err
	}
	return loadMap, loadBoth
}

// unreadDeclarationsRefusal refuses a reload whose config the full parse
// accepted and the first pass refused. The two parse the `plugin` block with
// the same schema nodes, so this is a Ze defect; the reload cannot say whether
// the plugin declarations changed, and a reload that cannot say refuses.
func unreadDeclarationsRefusal(firstPassErr error) error {
	slog.Error("the first pass refused a reload's config the full parse accepted, so the reload cannot compare "+
		"its plugin declarations with the ones ze started with. This is a Ze defect: report it",
		"error", firstPassErr, "code", diagnostic.CodeDoctorInternalDefect)
	return opererr.Wrap(firstPassErr,
		"the reload was refused: ze could not read the plugin declarations of the new config",
		"ze refuses a reload that changes the external plugins it runs, and it could not compare them. "+
			"This is a Ze defect, to report with the config; ze keeps the config it was running. What refused the declarations",
		"restart ze")
}

// handleSIGHUPReload is the SIGHUP reload worker. Reads signals from reloadCh
// and runs signalReload for each: stage the config source, reload the plugins
// and the shared ConfigProvider from it, fan Reload out to every registered
// subsystem (engine.Reload) so diff-able knobs hot-apply, then record it.
// Lifecycle goroutine (one-time, runs for daemon lifetime).
//
// A SIGHUP that finds another config transaction holding the lock (a commit,
// an API reload) is queued: the worker waits for that transaction to end, with
// success or failure, and then runs the reload once (awaitConfigTransaction).
// The holder's end is the only event that can start the queued reload. Waiting
// for the next SIGHUP instead left an operator's edit unapplied, with nothing
// reported, until an unrelated signal came.
//
// reloads is the daemon's signal-reload mutex, shared with request reload
// (newRequestReloadFunc).
//
// It closes done when reloadCh is closed or ctx is canceled and the reload it
// was running has reported. Shutdown waits on that (awaitReloadWorker), so a
// SIGTERM racing a SIGHUP does not take the verdict with it.
func handleSIGHUPReload(ctx context.Context, reloadCh <-chan os.Signal, done chan<- struct{}, reloads *sync.Mutex, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error), lm *listenerMigrator, recorder audit.Recorder) {
	defer close(done)

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-reloadCh:
			if !ok {
				return
			}
		}
		if ctx.Err() != nil {
			return
		}
		fmt.Fprintf(os.Stderr, "received SIGHUP, reloading config...\n")
		trigger := "SIGHUP"
		// Each pass follows the end of another config transaction, so the loop
		// ends when one attempt finds the lock free or shutdown begins.
		for {
			err := sighupReload(ctx, reloads, s, eng, cp, store, configPath, load, lm, recorder, trigger)
			if !errors.Is(err, pluginserver.ErrReloadInProgress) {
				break
			}
			fmt.Fprintf(os.Stderr, "transaction in progress, queuing SIGHUP...\n")
			if !awaitConfigTransaction(ctx, s, reloadCh) {
				return
			}
			fmt.Fprintf(os.Stderr, "replaying queued SIGHUP...\n")
			trigger = "queued SIGHUP"
		}
	}
}

// sighupReload runs signalReload for the SIGHUP worker under reloads, waiting
// for a request reload that holds it, so a SIGHUP queues rather than drops. It
// audits the reload, prints every error except ErrReloadInProgress, which it
// returns unprinted so the caller can queue the reload, and announces success
// with reloadComplete. trigger names the reload in the audit log.
func sighupHistoryTrigger(trigger string) string {
	if trigger == "queued SIGHUP" {
		return "queued-sighup"
	}
	return "sighup"
}

func sighupReload(ctx context.Context, reloads *sync.Mutex, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error), lm *listenerMigrator, recorder audit.Recorder, trigger string) error {
	ctx, opID := opid.Ensure(ctx)
	reloads.Lock()
	err := signalReload(withReloadTrigger(ctx, sighupHistoryTrigger(trigger)), s, eng, cp, store, configPath, load, lm)
	reloads.Unlock()
	if errors.Is(err, pluginserver.ErrReloadInProgress) {
		return err
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "reload error: %v\n", err)
		return err
	}
	recordDaemonReloadAuditWithOp(recorder, "system", "signal", audit.System, trigger, opID)
	reloadComplete()
	return nil
}

// newRequestReloadFunc builds the `request reload` handler the hub wires as the
// plugin server's FullReloadFunc (ze-plugin:system-daemon-reload, sent by
// `ze config edit --reload`). That RPC announces an edit the operator made to
// the file, as SIGHUP does, so it runs signalReload under the same per-daemon
// reloads mutex. It takes the mutex with TryLock and answers
// ErrReloadInProgress when another signal reload holds it, as the RPC always
// has: waiting would deadlock a plugin that sends it from inside its own
// apply. It does not audit: the dispatcher already records the RPC with the
// real caller. gate and ready are the commit reloads' shutdown gate and plugin
// startup wait; notify, when set, runs after a successful reload. The reload
// runs under the gate's shutdown scope (reloadGate.admit), so the daemon's stop
// stands it down and its caller is answered with stoodDownAnswer.
func newRequestReloadFunc(gate *reloadGate, reloads *sync.Mutex, ready func(context.Context) error, notify func(), s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error), lm *listenerMigrator) pluginserver.FullReloadFunc {
	return func(ctx context.Context) error {
		ctx, leave, admitted := gate.admit(ctx)
		if !admitted {
			return errDaemonStopping
		}
		defer leave()
		startupCtx, startupCancel := context.WithTimeout(ctx, 30*time.Second)
		defer startupCancel()
		if err := ready(startupCtx); err != nil {
			return stoodDownAnswer(ctx, fmt.Errorf("wait for plugin startup: %w", err))
		}
		if !reloads.TryLock() {
			return pluginserver.ErrReloadInProgress
		}
		err := signalReload(withReloadTrigger(ctx, "request-reload"), s, eng, cp, store, configPath, load, lm)
		reloads.Unlock()
		if err != nil {
			return stoodDownAnswer(ctx, err)
		}
		if notify != nil {
			notify()
		}
		return nil
	}
}

// signalReload reloads the config source the operator wrote. It is the one
// implementation behind both triggers that announce such an edit: SIGHUP
// (sighupReload) and `request reload` (newRequestReloadFunc). It stages the
// source as the candidate and reloads it as reloadSourceSignal, so promotion
// records it without writing the file. The caller MUST hold the daemon's
// reloads mutex, so the staging of one reload and the reload itself never
// interleave with another's: a request reload could otherwise load the
// candidate a SIGHUP had staged but not yet applied, or refuse on it with
// ErrCandidateExists. ErrReloadInProgress is returned as is.
func signalReload(ctx context.Context, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error), lm *listenerMigrator) error {
	if err := stageSIGHUPCandidate(store, configPath); err != nil {
		return err
	}
	return doReloadContext(ctx, reloadSourceSignal, s, eng, cp, store, configPath, load, lm)
}

// awaitConfigTransaction waits for the config transaction holding the lock to
// end. It returns true when the queued reload can run, and false when shutdown
// began first: ctx is canceled or reloadCh is closed. A SIGHUP read meanwhile
// is coalesced into the queued reload, which reads the config source when it
// runs, so it applies every edit the coalesced signal announced.
func awaitConfigTransaction(ctx context.Context, s *pluginserver.Server, reloadCh <-chan os.Signal) bool {
	held := s.ConfigTransactionDone()
	for held != nil {
		select {
		case <-held:
			held = nil
		case _, open := <-reloadCh:
			if !open {
				return false
			}
		case <-ctx.Done():
			return false
		}
	}
	// The transaction can end at the instant shutdown begins, and select picks
	// either ready case, so shutdown is asked once more before the reload runs.
	select {
	case _, open := <-reloadCh:
		if !open {
			return false
		}
	default:
	}
	return ctx.Err() == nil
}

// reloadGate owns every configuration operation that runs on a request
// goroutine (SSH upload, web source commit, commit reload, API full reload,
// managed commit and the confirmed-commit revert), from its admission to its
// return. The shared mutation owner admits before source reads or writes.
//
// It is also the one owner of their cancel at daemon stop. admit opens a
// shutdown scope for the operation (transaction.WithShutdownScope) and keeps
// its cancel function until the operation leaves. standDown ends every scope
// held with transaction.ErrShutdown, so a reload the stop interrupts emits no
// rollback, and one already rolling back stops waiting for a silent plugin
// and restarts none. An ordinary end of the caller's context ends no scope,
// so a rollback it started still runs to its own bound.
//
// The stop order is fenceCommitReloads: close admission and join, and stand
// down what is still running when the grace ends. Safe for concurrent use.
type reloadGate struct {
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
	// standDowns holds the scope cancel of each operation admit let in, keyed
	// by its admission number. An entry leaves with its operation, so the map
	// is bounded by the operations in flight.
	standDowns map[uint64]context.CancelCauseFunc
	// admissions numbers the operations admit let in.
	admissions uint64
}

// errDaemonStopping is the refusal a commit reload gets once shutdown began.
var errDaemonStopping = errors.New("daemon is shutting down; config reload refused")

// enter admits one operation that runs no reload; its caller MUST call leave
// when it returns. It returns false once close or standDown has run. An
// operation that reloads enters through admit, which also gives the stop its
// cancel.
func (g *reloadGate) enter() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return false
	}
	g.wg.Add(1)
	return true
}

// leave releases one operation enter admitted.
func (g *reloadGate) leave() { g.wg.Done() }

// admit admits one operation and returns the context it MUST run under: ctx
// inside a shutdown scope whose cancel the gate holds until the operation
// leaves. The caller MUST call the returned leave function when the operation
// returns. admitted is false once close or standDown has run, and the other
// two results are then nil.
func (g *reloadGate) admit(ctx context.Context) (scoped context.Context, leave func(), admitted bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil, nil, false
	}
	g.wg.Add(1)
	scoped, endScope := transaction.WithShutdownScope(ctx)
	g.admissions++
	admission := g.admissions
	if g.standDowns == nil {
		g.standDowns = make(map[uint64]context.CancelCauseFunc)
	}
	g.standDowns[admission] = endScope
	return scoped, func() {
		g.mu.Lock()
		delete(g.standDowns, admission)
		g.mu.Unlock()
		// The operation is over: release the scope. After standDown this is
		// a second cancel, which changes nothing.
		endScope(nil)
		g.wg.Done()
	}, true
}

// standDown refuses later operations and cancels every operation admit let in
// that is still running, with transaction.ErrShutdown as the cause. It does
// not wait for them: close does. Only a daemon stop calls it. It is
// idempotent.
func (g *reloadGate) standDown() {
	g.mu.Lock()
	g.closed = true
	running := make([]context.CancelCauseFunc, 0, len(g.standDowns))
	for _, endScope := range g.standDowns {
		running = append(running, endScope)
	}
	g.mu.Unlock()
	// Canceled outside the lock: an operation that unwinds calls leave, which
	// takes it.
	for _, endScope := range running {
		endScope(transaction.ErrShutdown)
	}
}

// close refuses later operations and waits for admitted ones. It is idempotent.
func (g *reloadGate) close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	g.wg.Wait()
}

// stoodDown reports whether the daemon's stop ended the operation that ran
// under ctx, a context admit returned, and failed with err. The error names
// the shutdown when the config transaction was the step it interrupted. The
// context's cause names it when another step was: the wait for plugin
// startup, a subsystem reload. A context keeps its first cause, so a caller
// that went away before the stop reads as an ordinary cancellation there, and
// only the transaction's error can still name the shutdown.
func stoodDown(ctx context.Context, err error) bool {
	if errors.Is(err, transaction.ErrShutdown) {
		return true
	}
	return errors.Is(context.Cause(ctx), transaction.ErrShutdown)
}

// The operator's answer for a configuration change the daemon's stop ended.
// The change may be saved or not: the stop can arrive before the plugins
// applied it, or after, while the hub published it. So the answer claims
// neither and names the check. Every surface shares these parts.
const (
	stoodDownProblem = "the configuration change was interrupted because ze is stopping"
	stoodDownWhy     = "ze ended the change before it was complete, so it may not be saved. " +
		"After ze starts again, check the configuration it runs and commit the change again if it is missing"
	stoodDownFix = "show config running"
)

// stoodDownAnswer turns the failure of an operation the daemon's stop ended
// into the operator's answer, and returns every other result as it is: nil
// when the operation finished before the stop reached it, err when it failed
// for a reason of its own. The answer keeps err as its evidence, so
// errors.Is still finds transaction.ErrShutdown.
func stoodDownAnswer(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if !stoodDown(ctx, err) {
		return err
	}
	return opererr.Wrap(err, stoodDownProblem, stoodDownWhy+". What ended it", stoodDownFix)
}

// reloadShutdownGrace gives the running reload time to finish before shutdown
// cancels it. Cancellation still requires a join before closing its resources.
const reloadShutdownGrace = 3 * time.Second

// awaitReloadWorker cancels an overdue worker and waits for its cleanup to finish.
// Plugin callbacks and candidate publication borrow the daemon's resources until
// done closes. A second termination signal remains available for a forced exit.
func awaitReloadWorker(done <-chan struct{}, grace time.Duration, cancel context.CancelFunc) {
	select {
	case <-done:
	case <-time.After(grace):
		fmt.Fprintf(os.Stderr, "shutdown: canceling config reload after %s; waiting for cleanup\n", grace)
		cancel()
		<-done
	}
}

// fenceCommitReloads is the daemon stop's wait for the configuration changes
// that run on request goroutines. It stops window, the confirmed-commit
// window, and then closes gate, in that order: the window's worker runs the
// commits and the revert, and each of them reloads through gate, so a gate
// closed first would refuse a revert already decided.
//
// The wait is bounded as the SIGHUP worker's is (awaitReloadWorker): a change
// still running after grace is stood down with transaction.ErrShutdown
// (reloadGate.standDown), which also closes admission, and the join that
// follows is short because a transaction stood down that way waits for no
// plugin. Without the stand-down a stop waited the whole apply or rollback
// deadline of a commit on one silent plugin, and a broken-plugin answer
// arriving meanwhile restarted a plugin the stop was about to end.
//
// closeCommitReloads runs on the one-time join goroutine of this stop, not a
// worker: it ends when the window's worker and every admitted operation have
// returned, which awaitReloadWorker waits for before it returns.
func fenceCommitReloads(window *confirm.Window, gate *reloadGate, grace time.Duration) {
	fenced := make(chan struct{})
	go closeCommitReloads(window, gate, fenced)
	awaitReloadWorker(fenced, grace, gate.standDown)
}

// closeCommitReloads stops window, closes gate and waits for both, then closes
// fenced. It is the join of fenceCommitReloads, which starts it once per
// daemon stop and waits on fenced; reloadGate.standDown is what ends an
// operation it would otherwise wait on.
func closeCommitReloads(window *confirm.Window, gate *reloadGate, fenced chan<- struct{}) {
	defer close(fenced)
	stopConfirmWindow(window)
	gate.close()
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
//     partial atomic write (a stray `.ze-storage-*` frame in the folder that
//     holds the store, which the next owner open now removes) and no
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
	recordDaemonReloadAuditWithOp(recorder, actor, remoteAddr, surface, detail, "")
}

func recordDaemonReloadAuditWithOp(recorder audit.Recorder, actor, remoteAddr, surface, detail, opID string) {
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
		OpID:       opID,
	}); err != nil {
		slogutil.Logger("hub.audit").Warn("audit record failed", "code", "doctor-audit-record-failed", "action", audit.ActionDaemonReload, "error", err)
	}
}

type reloadTriggerContextKey struct{}

func withReloadTrigger(ctx context.Context, trigger string) context.Context {
	if trigger == "" {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, reloadTriggerContextKey{}, trigger)
}

func reloadTriggerFrom(ctx context.Context, source reloadSource) string {
	if ctx != nil {
		if trigger, ok := ctx.Value(reloadTriggerContextKey{}).(string); ok && trigger != "" {
			return trigger
		}
	}
	switch source {
	case reloadSourceCommit:
		return "commit"
	case reloadSourceSignal:
		return "sighup"
	case reloadSourceUnspecified:
		return "unknown"
	default:
		return "unknown"
	}
}

// reloadSource names who authored the candidate a reload promotes, which
// decides whether promotion may write the explicit config file. The caller
// states it: the bytes cannot, because a file the operator wrote back to the
// accepted bytes looks exactly like a file nobody touched.
type reloadSource uint8

const (
	reloadSourceUnspecified reloadSource = iota
	// reloadSourceCommit: the daemon wrote the candidate (SSH, web, API or
	// managed commit), so promotion publishes it to the file.
	reloadSourceCommit
	// reloadSourceSignal: SIGHUP staged the candidate from the file the
	// operator wrote, so promotion records it and leaves the file alone.
	reloadSourceSignal
)

// promoteReloadCandidate promotes the candidate a successful reload applied,
// by the rule its source owns.
func promoteReloadCandidate(source reloadSource, store storage.Storage, configPath string) error {
	switch source {
	case reloadSourceCommit:
		return promoteConfigCandidate(store, configPath)
	case reloadSourceSignal:
		return promoteSignalCandidate(store, configPath)
	case reloadSourceUnspecified:
		panic("BUG: reload promotion with an unspecified source")
	}
	panic("BUG: reload promotion with an unknown source")
}

// doReload is a test entry point: doReloadContext under a background context,
// promoting as a commit (reloadSourceCommit). No production path calls it.
func doReload(s *pluginserver.Server, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error), lm *listenerMigrator) error {
	return doReloadContext(context.Background(), reloadSourceCommit, s, nil, cp, store, configPath, load, lm)
}

// doReloadContext performs a single config reload and records the "reload
// processed" fence that `show system reload status` surfaces. source states who
// authored the candidate: reloadSourceSignal from signalReload (SIGHUP and
// `request reload`), reloadSourceCommit from every daemon-authored commit.
//
// The fence is marked HERE, around the whole of runReloadContext, rather than inside
// pluginserver.Server.reloadConfig, because a reload's rejections are decided
// by the subsystems engine.Reload fans out to (e.g. l2tp refusing a listener
// rebind), which runs after ReloadConfig. Marking at the plugin-server layer
// would advance the fence before those rejections had happened, and an observer
// polling it could read state the reload had not finished touching.
//
// ErrReloadInProgress is deliberately NOT marked: that reload never ran: it is
// queued, handleSIGHUPReload runs it when the holder's transaction ends, and
// that run marks it. Marking
// it here would fence an observer on a reload that had not been processed.
func doReloadContext(ctx context.Context, source reloadSource, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error), lm *listenerMigrator) error {
	startedAt := time.Now().UTC()
	ctx, opID := opid.Ensure(ctx)
	trigger := reloadTriggerFrom(ctx, source)
	trace := &pluginserver.ReloadTrace{}
	ctx = pluginserver.WithReloadTrace(ctx, trace)

	err := runReloadContext(ctx, source, s, eng, cp, store, configPath, load, lm)
	if s != nil {
		if !errors.Is(err, pluginserver.ErrReloadInProgress) {
			finishedAt := time.Now().UTC()
			outcome := pluginserver.ReloadOutcomeApplied
			errStr := ""
			if err != nil {
				outcome = pluginserver.ReloadOutcomeFailed
				errStr = err.Error()
			}
			roots, txState, plugins := trace.Snapshot()
			durationMS := max(finishedAt.Sub(startedAt).Milliseconds(), 0)
			s.MarkReloadProcessedWithRecord(pluginserver.ReloadRecord{
				OpID:         opID,
				Trigger:      trigger,
				Outcome:      outcome,
				StartedAt:    startedAt,
				FinishedAt:   finishedAt,
				DurationMS:   durationMS,
				ChangedRoots: roots,
				TxState:      txState,
				Plugins:      plugins,
				Error:        errStr,
			})
			if err != nil {
				slogutil.Logger("hub.reload").WarnContext(ctx, "config reload failed",
					"code", pluginserver.CodeDoctorConfigReloadRefused,
					"trigger", trigger, "operation-id", opID, "error", err)
			} else {
				slogutil.Logger("hub.reload").InfoContext(ctx, "config reload applied",
					"trigger", trigger, "operation-id", opID)
			}
		}
	}
	return err
}

// runReload is a test entry point: runReloadContext with no store, so no
// candidate and no fence. No production path calls it.
func runReload(s *pluginserver.Server, cp *zeconfig.Provider, load func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error), lm *listenerMigrator) error {
	return runReloadContext(context.Background(), reloadSourceCommit, s, nil, cp, nil, "", load, lm)
}

// runReloadContext performs a single config reload with a 30-second timeout.
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
// Production callers go through doReloadContext, which wraps this to record
// the reload fence.
func runReloadContext(ctx context.Context, source reloadSource, s *pluginserver.Server, eng *engine.Engine, cp *zeconfig.Provider, store storage.Storage, configPath string, load func() (map[string]any, *zeconfig.Tree, []zePlugin.PluginConfig, error), lm *listenerMigrator) (result error) {
	reloadCtx, reloadCancel := context.WithTimeout(ctx, 30*time.Second)
	defer reloadCancel()
	// The plugin transaction commits before listener migration and publication.
	// Only this whole-reload boundary may release irreversible plugin cleanup.
	reloadCtx, acceptReload := s.DeferReloadAcceptance(reloadCtx)
	acceptedDecision := false
	defer func() {
		if !acceptedDecision {
			result = errors.Join(result, acceptReload(false))
		}
	}()
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
		return clearUncommittedCandidate(store, configPath)
	}

	if load == nil {
		// stdin-config daemons have no reload source. Fall back to the
		// plugin server's own ReloadFromDisk (which also errors if no
		// loader is configured) so the error message stays familiar.
		err := s.ReloadFromDisk(reloadCtx)
		acceptedDecision = true
		return errors.Join(err, acceptReload(err == nil))
	}

	// The loader names the stage that refused (parse config, a validation
	// walk, a missing plugin), so the reload adds only that it was a reload.
	newTree, parsedTree, selection, loadErr := load()
	if loadErr != nil {
		return withUnwind(fmt.Errorf("reload: %w", loadErr), unwindStep{unwindCandidateCleanup, clearCandidate()})
	}
	// The same kernel capability gate the daemon start applies (main.go,
	// runYANGConfig). It runs BEFORE ReloadConfig, before the provider refresh
	// and before engine.Reload, so a reload into a configuration this kernel
	// cannot carry is refused with the running configuration still serving. The
	// operator sees the refusal through `ze config commit`.
	if capErr := kernelcap.Refuse(parsedTree); capErr != nil {
		return withUnwind(fmt.Errorf("reload: kernel capability: %w", capErr), unwindStep{unwindCandidateCleanup, clearCandidate()})
	}

	// Number this configuration the moment it is read. The acceptance tail uses
	// the number to refuse a chain built from a configuration a later reload has
	// already replaced (aaa_lifecycle.go, nextAAAConfigReadOrder).
	configOrder := nextAAAConfigReadOrder()

	pkiConfig, pkiErr := preparePKIConfig(parsedTree)
	if pkiErr != nil {
		return withUnwind(fmt.Errorf("reload: pki config: %w", pkiErr), unwindStep{unwindCandidateCleanup, clearCandidate()})
	}

	var priorProvider map[string]map[string]any
	if cp != nil {
		var snapErr error
		priorProvider, snapErr = snapshotProvider(cp)
		if snapErr != nil {
			return withUnwind(fmt.Errorf("reload: snapshot config provider: %w", snapErr), unwindStep{unwindCandidateCleanup, clearCandidate()})
		}
	}

	// The certificate each hub TLS listener serves under the running config,
	// read before this reload changes anything. A reload that installs the
	// reloaded config's certificate on a listener and then fails puts these
	// back, once rollbackReload has put the running store back (R-3).
	var priorCertificates *listenerCertificateNames
	if priorProvider != nil {
		names, namesErr := runningListenerCertificates(priorProvider)
		if namesErr != nil {
			return withUnwind(fmt.Errorf("reload: read the running listener certificates: %w", namesErr), unwindStep{unwindCandidateCleanup, clearCandidate()})
		}
		priorCertificates = names
	}

	// Capture the PKI config the store currently holds, so any failure after the
	// install below can put it back. Without this, a rejected reload would leave
	// the daemon serving the NEW certificates under the OLD config (R-3).
	var priorPKI *zepki.PKIConfig
	if priorProvider != nil {
		// Ask the STORE what it is serving rather than re-deriving it from the
		// provider snapshot. The snapshot holds plugin-facing maps, which carry
		// no leaf-list faithfully, so a config rebuilt from one comes back
		// without its intermediates and the restore would install a shorter
		// chain than the one it replaced. Snapshot shares the installed maps, so
		// handing it straight back to Load reinstalls exactly what was running.
		priorPKI = zepki.Snapshot()
	}

	// Install the new store BEFORE any consumer applies the new config. A
	// reference-style leaf (`certificate <name>` -> store entry) only resolves
	// within its own commit if the store is already the new one when the plugin
	// or subsystem asks (AC-10). Installing it last, as this used to, meant a
	// single commit that added a certificate AND referenced it resolved against
	// the PREVIOUS store and failed.
	if err := zepki.Load(pkiConfig); err != nil {
		return withUnwind(fmt.Errorf("reload: pki config: %w", err), unwindStep{unwindCandidateCleanup, clearCandidate()})
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

	// The same refusal for the looking glass, on its own name (AC-5, R-5). The
	// looking glass is the surface a stranger visits, so a name that stopped
	// resolving must stop the commit here rather than leave the listener on a
	// certificate the config no longer describes.
	lgConfig, lgConfigErr := lgListenerConfigFrom(parsedTree)
	if lgConfigErr != nil {
		return restorePKIAfter(priorPKI, clearCandidate, fmt.Errorf("reload: %w", lgConfigErr))
	}
	if lgConfig.certName != "" {
		if _, _, certErr := zepki.ServerTLSMaterial(lgConfig.certName); certErr != nil {
			return restorePKIAfter(priorPKI, clearCandidate,
				fmt.Errorf("reload: environment.looking-glass.certificate: %w", certErr))
		}
	}
	// A server entry that says `tls true` with no certificate to present is
	// refused here, before anything is applied. A start leaves that entry down
	// and serves the others; a reload keeps what runs and refuses the config.
	if lm != nil {
		if err := lm.checkLGTransport(lgConfig); err != nil {
			return restorePKIAfter(priorPKI, clearCandidate, fmt.Errorf("reload: %w", err))
		}
	}

	// Prepare the managed listener's certificate and serving declarations
	// against the just-installed PKI store before any consumer applies (AC-39).
	// Unresolved names, unparseable pairs, or an authority unable to replace a
	// serving named chain refuse the reload whole and restore the prior PKI.
	var managedPlan *pluginserver.ManagedListenerPlan
	if lm != nil {
		if srv := lm.managedServer(); srv != nil {
			plan, planErr := prepareManagedReload(parsedTree, srv)
			if planErr != nil {
				return restorePKIAfter(priorPKI, clearCandidate, fmt.Errorf("reload: %w", planErr))
			}
			managedPlan = plan
		}
	}

	// The named next-hop ranges of the looking glass, read by the reader the
	// startup path uses, whatever the `enabled` leaf says. A list the reader
	// refuses stops the reload here, before anything is applied.
	lgNextHopRanges, lgNextHopRangesErr := zeconfig.ExtractLGNextHopRanges(parsedTree)
	if lgNextHopRangesErr != nil {
		return restorePKIAfter(priorPKI, clearCandidate, fmt.Errorf("reload: %w", lgNextHopRangesErr))
	}

	if err := s.ReloadConfigWithSelection(reloadCtx, newTree, selection); err != nil {
		return withUnwind(err,
			unwindStep{unwindPKIRestore, restorePKI(priorPKI)}, unwindStep{unwindCandidateCleanup, clearCandidate()})
	}

	if cp != nil {
		applyLoadedTreeToProvider(cp, newTree)
	}

	// Stage credentials and policy from the candidate provider roots, but keep
	// the accepted generation published while every remaining reload step can
	// still fail. The resolver belongs to the accepted state and carries the
	// fixed zefs boot snapshot, so config-over-zefs precedence is identical to
	// boot without authentication reading ConfigProvider directly.
	var candidateIdentity *acceptedLocalIdentityState
	if accepted := acceptedLocalIdentity.Load(); accepted != nil {
		if accepted.resolveCandidateUsers == nil {
			err := fmt.Errorf("reload: resolve candidate local identity: %w", errNoLiveConfigProvider)
			rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI)
			return withUnwind(err,
				unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearCandidate()})
		}
		candidateUsers, candidateErr := accepted.resolveCandidateUsers()
		if candidateErr != nil {
			err := fmt.Errorf("reload: resolve candidate local identity: %w", candidateErr)
			rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI)
			return withUnwind(err,
				unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearCandidate()})
		}
		var candidateStore *authz.Store
		if parsedTree != nil {
			var authzErr error
			candidateStore, authzErr = infra.ExtractAuthzStore(parsedTree)
			if authzErr != nil {
				err := fmt.Errorf("reload: candidate authorization config: %w", authzErr)
				rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI)
				return withUnwind(err,
					unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearCandidate()})
			}
		}
		// An absent API block leaves an environment- or flag-started listener
		// running, so retain its accepted token. A present block re-resolves
		// config-over-environment precedence for the candidate generation. The
		// exposure classifier uses candidateAPIToken too, so it guards exactly
		// the credentials this identity will publish.
		candidateAPITokenValue := accepted.apiToken
		if parsedTree != nil {
			candidateAPI, _, apiErr := resolveAPIListeners(parsedTree)
			if apiErr != nil {
				err := fmt.Errorf("reload: resolve candidate API identity: %w", apiErr)
				rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI)
				return withUnwind(err,
					unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearCandidate()})
			}
			_, apiBlockPresent := zeconfig.ExtractAPISettings(parsedTree)
			candidateAPITokenValue = candidateAPIToken(
				candidateAPI.Token, apiBlockPresent, accepted.apiToken, env.Get("ze.api-server.token"))
		}
		candidateIdentity = newAcceptedLocalIdentity(
			candidateUsers,
			candidateStore,
			accepted.resolveCandidateUsers,
			candidateAPITokenValue,
		)
	}

	// Rebuild the AAA chain from the reloaded tree. The local backend already
	// follows the accepted generation on every login, but a RADIUS or TACACS+
	// client holds the address, the shared secret and the timeout it was
	// CONSTRUCTED with and can re-read none of them, so rotating a secret was
	// accepted into the configuration and reached nothing until the daemon
	// restarted.
	//
	// The result is a CANDIDATE. Build has already opened the new client's
	// socket and started its accounting worker, so a reload that fails after
	// this point closes it instead of leaking it, and the swap that retires the
	// running chain waits for the acceptance point beside the identity
	// publication. A daemon with no bundle installed gets none here: boot owns
	// the single construction attempt (aaa_lifecycle.go, claimAAABundleBoot).
	//
	// A reload that changes nothing the chain is built from rebuilds nothing.
	// The rebuild is not free: it closes the live RADIUS socket and drains the
	// TACACS+ accounting worker, so an unrelated `ze config commit` would
	// otherwise bounce a working chain (aaaAuthenticationChanged).
	var candidateBundle *aaa.Bundle
	bundleInstalled := false
	defer func() {
		if bundleInstalled || candidateBundle == nil {
			return
		}
		if closeErr := candidateBundle.Close(); closeErr != nil {
			slogutil.Logger("hub.aaa").Warn("aaa: abandoned reload bundle close error", "error", closeErr)
		}
	}()
	if aaaBundle.Load() != nil && parsedTree != nil && aaaAuthenticationChanged(priorProvider, newTree) {
		// No snapshot users: the local backend prefers LocalUsersFunc, and the
		// generation this reload publishes is accepted before the swap below.
		built, buildErr := buildAAABundle(parsedTree, nil, liveAcceptedLocalUsers, slogutil.Logger("hub.aaa"))
		if buildErr != nil {
			// Fail closed. An AAA chain the reloaded config describes and the
			// daemon cannot build is not a chain to shrug at: refusing the
			// reload keeps the running one, and reports the refusal through
			// `ze config commit` rather than through a log line nobody reads.
			err := fmt.Errorf("reload: rebuild aaa bundle: %w", buildErr)
			rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI)
			return withUnwind(err,
				unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearCandidate()})
		}
		candidateBundle = built
	}

	if eng != nil {
		if err := eng.Reload(reloadCtx); err != nil {
			rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI)
			return withUnwind(fmt.Errorf("subsystem reload: %w", err),
				unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearCandidate()})
		}
	}

	// API requests fail closed while listeners migrate. The staging generation
	// contains no candidate credentials. A failed reload restores the accepted
	// API generation only after its listener addresses are restored. If listener
	// rollback fails, the affected API listeners stay fail closed.
	restoreIdentity := func() {}
	if lm != nil && (lm.rest != nil || lm.grpc != nil) {
		restoreIdentity = stageAPIAuthentication()
	}
	restoreListeners := noListenerRestore
	// restoreLGNextHopRanges puts back the names the looking glass showed
	// before this reload replaced them. It does nothing until the list is
	// replaced, and it runs for every reload that does not end applied: a
	// candidate whose promotion fails is rolled back after the names changed,
	// and the page must not keep the names of a config that is not in force.
	restoreLGNextHopRanges := func() {}
	// restoreCertificates puts back the certificate each listener served before
	// this reload installed the reloaded config's, and the protocol of each
	// looking-glass listener. It runs before restoreListeners, so an address the
	// looking glass binds again has its running protocol recorded. It does
	// nothing until an install succeeds. Every return that leaves the reload
	// unapplied after an install calls rollbackReload first, so the running
	// store is back when it runs and the names resolve to the running config's
	// material.
	restoreCertificates := func() error { return nil }
	reloadApplied := false
	defer func() {
		if reloadApplied {
			return
		}
		restoreLGNextHopRanges()
		if restoreErr := restoreCertificates(); restoreErr != nil {
			result = withUnwind(result, unwindStep{unwindCertificateRestore, restoreErr})
		}
		if restoreErr := restoreListeners(); restoreErr != nil {
			slogutil.Logger("hub.reload").Error("listener rollback failed", "error", restoreErr)
			return
		}
		restoreIdentity()
	}()
	// The store is already installed (above). Push the certificate the reloaded
	// config describes onto each running TLS listener: the named chain, possibly
	// rotated, or the self-signed pair when the config names none, and hand the
	// looking glass the protocol of each of its listeners. A listener serves the
	// certificate from the next handshake with no rebind, so open SSE streams
	// survive the rotation (AC-9).
	//
	// This runs BEFORE the listeners move. The looking glass binds an address a
	// reload adds with the protocol named here, so a `tls false` entry the
	// reload adds answers plain HTTP from its first connection, and a TLS one
	// presents the reloaded config's certificate from its first handshake. Moved
	// first, a new listener had no protocol yet and served TLS until this ran.
	if lm != nil && parsedTree != nil {
		// Each listener takes its OWN name onto its OWN handle, so rotating one
		// can never install the other's identity (R-1). A viewer holding an
		// open looking-glass connection keeps it across the swap (AC-6).
		//
		// Only for a reload that parsed a tree, as for the listener move and the
		// next-hop ranges below: a reload with no tree says nothing about either
		// leaf, and an empty name now installs the self-signed pair, so pushing
		// it would replace a named chain the config still describes.
		//
		// A listener whose install succeeded gets its running certificate and
		// protocol back when a later step fails, the listener move included:
		// the deferred rollback above calls restoreCertificates before it moves
		// the listeners back. A failed install leaves that listener on what it
		// was serving, so only the installs before it are undone.
		err := lm.updateWebCertificate(webCertName)
		if err == nil {
			restoreCertificates = priorCertificates.restoreOn(lm, false)
			err = lm.updateLGTransport(lgConfig)
		}
		if err == nil {
			restoreCertificates = priorCertificates.restoreOn(lm, true)
		}
		if err != nil {
			rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI)
			return withUnwind(fmt.Errorf("reload: %w", err),
				unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearCandidate()})
		}

		undo, err := lm.reloadListeners(reloadCtx, parsedTree)
		restoreListeners = undo
		if err != nil {
			rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI)
			return withUnwind(fmt.Errorf("reload: listener migration: %w", err),
				unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearCandidate()})
		}

		// Last, and only for a reload that parsed a tree: a reload with no tree
		// says nothing about the list, and handing the looking glass an empty
		// one would erase the names it shows. The names it replaced go back
		// when a later step fails: the deferred rollback above calls
		// restoreLGNextHopRanges.
		restore, err := lm.updateLGNextHopRanges(lgNextHopRanges)
		if err != nil {
			rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI)
			return withUnwind(fmt.Errorf("reload: %w", err),
				unwindStep{unwindRollback, rollbackErr}, unwindStep{unwindCandidateCleanup, clearCandidate()})
		}
		restoreLGNextHopRanges = restore
	}

	if candidateSet {
		if err := promoteReloadCandidate(source, store, configPath); err != nil {
			if rollbackErr := rollbackReload(reloadCtx, s, eng, cp, priorProvider, priorPKI); rollbackErr != nil {
				return withUnwind(fmt.Errorf("reload: promote candidate: %w", err), unwindStep{unwindRollback, rollbackErr})
			}
			return withUnwind(fmt.Errorf("reload: promote candidate: %w", err), unwindStep{unwindCandidateCleanup, clearCandidate()})
		}
	}

	// Host-side system mutations and name-server publication go live only once
	// candidate promotion has succeeded, never before: a reload rolled back
	// above must leave host tuning, hostname, resolv.conf, schedulers and the
	// accepted name-server list untouched.
	applyHostTuningFromMap(newTree)
	applyConsoleFromMap(newTree)
	applyKernelHostnameFromMap(newTree)
	applyConntrackFromMap(newTree, s)
	applyUpdateCheckerFromMap(newTree)
	startArchiveScheduler(parsedTree, configPath, store, s)
	applyResolvConf(parsedTree)
	reloadSmartManager(parsedTree)
	publishNameServers(parsedTree)
	if lm != nil {
		if srv := lm.managedServer(); srv != nil {
			srv.ApplyManagedListeners(managedPlan)
		}
	}

	// The complete local and API identity becomes live only after every
	// fallible reload step, including candidate promotion, has succeeded.
	// The accepted identity and the rebuilt chain go live as ONE acceptance. The
	// chain's local backend answers from the identity published here, so no
	// reader may see one without the other, and a reload whose configuration a
	// later reload has already superseded publishes neither.
	//
	// The install returns the chain it retires. Closing that chain drains the
	// retired TACACS+ accounting worker and the retired RADIUS socket, and it
	// runs OUTSIDE the acceptance lock because it joins a goroutine.
	retired, installed := acceptReloadedAAA(candidateIdentity, candidateBundle, configOrder)
	bundleInstalled = installed
	closeRetiredAAABundle(retired, slogutil.Logger("hub.aaa"))
	reloadApplied = true
	acceptedDecision = true
	return acceptReload(true)
}

// aaaAuthenticationChanged reports whether the reloaded configuration changes
// anything the AAA chain is BUILT from. That is the `system authentication`
// subtree and nothing else: it is what both remote backends read
// (radiusBackend.Build and tacacsBackend.Build, each through their package's
// ExtractConfig), while the local backend reads no tree at all and answers from
// the accepted identity generation on every login.
//
// prior is the snapshot of the RUNNING configuration, taken before this reload
// applied anything. A reload with no snapshot cannot prove the chain is
// unchanged, so it rebuilds.
//
// The two sides are lowered by different callers, and both lower with
// ToPluginMap: boot at main.go Phase 2 and a reload at loadTreeForReload. So
// the comparison is exact from the first reload after boot, including for a
// multi-entry list under `system authentication`, whose entry order is part of
// what the two sides compare.
func aaaAuthenticationChanged(prior map[string]map[string]any, next map[string]any) bool {
	if prior == nil {
		return true
	}
	return !reflect.DeepEqual(prior["system"]["authentication"], aaaAuthenticationSubtree(next))
}

// aaaAuthenticationSubtree returns the `system authentication` subtree of a
// lowered config map, and nil when the configuration declares none.
func aaaAuthenticationSubtree(root map[string]any) any {
	system, _ := root["system"].(map[string]any)
	return system["authentication"]
}

func stageSIGHUPCandidate(store storage.Storage, configPath string) error {
	if store == nil || configPath == "" || configPath == "-" {
		return nil
	}
	// One lock hold for the read and the write: a commit landing between them
	// would be undone by this reload applying the source it replaced.
	if _, err := storage.StageSourceCandidate(store, configPath, time.Now()); err != nil {
		if errors.Is(err, storage.ErrCandidateExists) {
			return err
		}
		return fmt.Errorf("stage SIGHUP candidate: %w", err)
	}
	return nil
}

// lowerForPlugins lowers the boot authority with member intent and list order.
// The coordinator keeps that running projection. ConfigProvider MUST receive
// only active data, before roots are enumerated, and a clone of each root:
// it serves public/subsystem consumers and stores references handed to SetRoot.
// applyLoadedTreeToProvider keeps the same projection and ownership on reload.
func lowerForPlugins(tree *zeconfig.Tree, cp *zeconfig.Provider) map[string]any {
	configTree := tree.ToRunningMap()
	for root, subtree := range zeconfig.ActiveConfigMap(configTree) {
		if sub, ok := subtree.(map[string]any); ok {
			cp.SetRoot(root, cloneStringAnyMap(sub))
		}
	}
	return configTree
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
	for root, subtree := range zeconfig.ActiveConfigMap(tree) {
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
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 90*time.Second)
	defer cancel()
	var rollbackErrs []error
	// Reinstall the prior store FIRST: the plugin and subsystem rollbacks below
	// re-apply the old config, and a consumer resolving a certificate reference
	// during that apply must see the old material, not the rejected commit's
	// (R-3). This mirrors the install-before-apply ordering of the forward path.
	if err := restorePKI(priorPKI); err != nil {
		rollbackErrs = append(rollbackErrs, err)
	}
	if s != nil {
		if err := s.AbortReload(ctx); err != nil {
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
	return withUnwind(cause,
		unwindStep{unwindPKIRestore, restorePKI(priorPKI)}, unwindStep{unwindCandidateCleanup, clearCandidate()})
}

// listenerCertificateNames is what one config gives each hub TLS listener: the
// web listener's certificate name, and the looking glass's listeners with the
// name their TLS entries present. An empty name is the listener's self-signed
// pair, as it is for listenerTLSMaterial.
type listenerCertificateNames struct {
	web string
	lg  lgListenerConfig
}

// runningListenerCertificates answers the certificate names of the running
// config, read from the provider snapshot runReloadContext takes before it
// applies anything. It reads them through the readers the reload uses for the
// reloaded config (reloadWebCertificate, lgListenerConfigFrom), so a rollback
// returns each listener to the certificate, and each looking-glass listener to
// the protocol, a restart with the running config would serve.
func runningListenerCertificates(prior map[string]map[string]any) (*listenerCertificateNames, error) {
	roots := make(map[string]any, 1)
	if environment, ok := prior["environment"]; ok {
		roots["environment"] = environment
	}
	tree, err := zeconfig.TreeFromPluginMap(roots)
	if err != nil {
		return nil, err
	}
	lgConfig, err := lgListenerConfigFrom(tree)
	if err != nil {
		return nil, err
	}
	return &listenerCertificateNames{web: reloadWebCertificate(tree), lg: lgConfig}, nil
}

// restoreOn answers the undo of the certificate installs a reload made on lm:
// it reinstalls these names on the web listener, and on the looking glass too
// when lookingGlass is set. The reload installs the web listener's certificate
// first and stops at the first install that fails, so those are the two sets
// of installs it can have made. The caller MUST run the undo after the running
// store is back, or the names resolve to the refused commit's material.
func (names *listenerCertificateNames) restoreOn(lm *listenerMigrator, lookingGlass bool) func() error {
	// refuse-by-default: guards runReloadContext -- no names means no provider snapshot, so rollbackReload restores no store either and there is no running certificate to return to
	if names == nil {
		return func() error { return nil }
	}
	return func() error {
		err := lm.updateWebCertificate(names.web)
		if !lookingGlass {
			return err
		}
		return errors.Join(err, lm.restoreLGTransport(names.lg))
	}
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

// lgCertificateName returns the environment.looking-glass.certificate name the
// looking glass presents on listeners, the ones parsedTree describes
// (resolveLGListeners). The startup gate (cmd/ze/hub/main.go) and the reload
// gate both reach it through lgListenerConfigFrom, so a restart and a reload
// cannot disagree about which certificate one config asks for.
//
// The env var wins, exactly as it does for the web certificate, so a deployment
// that pins the certificate through ze.looking-glass.certificate is not
// silently re-pointed by a config edit.
//
// A looking glass whose every server entry says `tls false` presents no
// certificate, so it reports no name. The leaf is inert there rather than
// refused: no listener resolves it (service_lg.go, buildLGService), so a
// commit rejected here over a name nothing reads would refuse a config the same
// daemon boots happily. A config that describes no listener says nothing
// about their protocol, and the name is reported: a looking glass that keeps
// running on TLS presents it.
func lgCertificateName(parsedTree *zeconfig.Tree, listeners []lgListener) string {
	if len(listeners) > 0 {
		if !lgAsksForTLS(listeners) {
			return ""
		}
	}
	if name := env.Get("ze.looking-glass.certificate"); name != "" {
		return name
	}
	if parsedTree == nil {
		return ""
	}
	// Settings, not addresses: the certificate applies to a listener started by
	// a flag or env var too (learned 1327).
	cfg, ok := zeconfig.ExtractLGSettings(parsedTree)
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
