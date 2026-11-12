package flowspec

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/core/textbuf"
	flowspecyang "github.com/ze-software/ze/internal/plugins/ddos/flowspec/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
	"github.com/ze-software/ze/pkg/zefs"
)

var eventBusPtr atomic.Pointer[ze.EventBus]

// bgpPlugin is the registration name of the BGP subsystem, the engine an
// enforcing responder announces its FlowSpec rules through.
const bgpPlugin = "bgp"

// activeResponder publishes the live responder to the in-process show handler
// (show.go). Nil when the plugin is not configured/running.
var activeResponder atomic.Pointer[responder]

// announcementKey is the runtime state key that records the FlowSpec rule the
// plugin has announced and not yet withdrawn (encodeAnnouncementRecord). A
// daemon stop or a kill ends the plugin with the rule still at the peers and no
// connection left to withdraw it through, so the record is what lets the next
// start withdraw it and `ze doctor` report it until then. The name is a
// contract: an operator reads it in `ze data registered`.
var announcementKey = zefs.MustRegister(zefs.KeyEntry{
	Pattern:     "meta/ddos-flowspec/announcement",
	Description: "DDoS FlowSpec rule announced to the BGP peers and not yet withdrawn (JSON)",
})

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-flowspec: event bus not configured")
	}
	return *p, nil
}

func init() {
	// The plugin reads and writes its record through the SDK state calls, which
	// the daemon answers only for a key the owning plugin was granted here.
	statestore.RegisterPluginKeys(Name, announcementKey)
	if err := diagnostic.RegisterDoctorCheck(orphanRecordCheck); err != nil {
		var tb textbuf.Buffer
		_ = tb.Str("ddos-flowspec: doctor check registration failed: ").Err(err).Byte('\n').StdErr()
		os.Exit(1)
	}

	reg := registry.Registration{
		Name:        Name,
		Description: "DDoS FlowSpec/RTBH responder: upstream mitigation with leak-probe clear",
		Features:    "yang",
		YANG:        flowspecyang.ZeDdosFlowspecConfYANG,
		ConfigRoots: []string{configRoot},
		// `response-level enforce` announces FlowSpec rules through the BGP
		// engine (sdkDispatcher), so a build without the BGP subsystem refuses
		// it. `alert` announces nothing (responder.onCharacterized), so it
		// needs no BGP and is accepted in every build.
		OptionalDependencies: []string{bgpPlugin},
		ConfigNeeds: []registry.ConfigNeed{{
			Path:      []string{"ddos", "flowspec", "response-level"},
			Value:     responseEnforce,
			Providers: []string{bgpPlugin},
		}},
		RunEngine: runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			eventBusPtr.Store(&eb)
		},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		var tb textbuf.Buffer
		_ = tb.Str("ddos-flowspec: registration failed: ").Err(err).Byte('\n').StdErr()
		os.Exit(1)
	}
}

// errApplyWithoutVerify rejects a config-apply that arrives without the
// config-verify that stages its candidate. Returning nil accepted the
// transaction while silently keeping the PREVIOUS responder, so the daemon
// would print "sighup reload complete" over config it never installed.
var errApplyWithoutVerify = errors.New("ddos-flowspec config apply: no verified config staged (config-apply arrived without config-verify); refusing to report success over the previous config")

// pendingConfig is what OnConfigVerify stages for OnConfigApply: the parsed
// config, and whether the delivered section carried a `ddos flowspec` body at
// all.
//
// The two are one type because the apply cannot recover the second from the
// first. A removal delivers an empty body, which parses to the same values as a
// block naming no leaf, so the answer has to travel; and a path that cleared one
// while leaving the other would arm the next apply with a mismatched pair.
//
// NOT safe for concurrent use, and it does not need to be: the plugin SDK drives
// verify, apply and rollback for one transaction on one goroutine.
type pendingConfig struct {
	cfg        *Config
	configured bool
}

// stage records the candidate a config-verify accepted.
func (p *pendingConfig) stage(cfg *Config, configured bool) {
	p.cfg = cfg
	p.configured = configured
}

// take returns the staged candidate and unstages it, so a second apply behind
// one verify gets a nil config and the caller's fail-closed branch.
func (p *pendingConfig) take() (cfg *Config, configured bool) {
	cfg, configured = p.cfg, p.configured
	p.clear()
	return cfg, configured
}

// clear unstages the candidate without applying it. The rollback path calls it:
// a transaction that verified here and then failed elsewhere never reaches this
// plugin's apply, and a candidate left staged is applied by the NEXT transaction
// that reaches an apply without a verify. That is the state the apply's nil
// check exists to refuse, and a stale candidate defeats it -- the pair a
// rolled-back removal leaves behind is (DefaultConfig, false), so the next apply
// would withdraw a live announcement nobody asked it to withdraw.
func (p *pendingConfig) clear() {
	p.cfg = nil
	p.configured = false
}

// rollback is the plugin's config-rollback handler, and unstaging the candidate
// is the whole of its job.
func (p *pendingConfig) rollback(_ string) error {
	p.clear()
	return nil
}

// replaceResponder builds the responder for cfg, decides what becomes of an
// announcement the responder it replaces made, and publishes it to the cap
// worker and to the show handler.
//
// configured says the delivered config still carries a `ddos flowspec` section.
//
// The default is to CARRY the announcement, and carrying is what stops an
// ordinary commit orphaning it (adoptAnnouncement states what crosses and why).
// Two commits say the opposite, that the box must stop announcing, and each
// withdraws instead. Both go through the OUTGOING responder, because the
// incoming one has announced nothing and its own removal paths return on
// !active:
//
//   - The section is gone. The plugin is STOPPED as soon as this reload
//     transaction commits (Server.collectProcessesForRemovedConfigPaths,
//     internal/component/plugin/server/startup_autoload.go), while the FlowSpec
//     NLRI sits in the BGP engine's RIB and in every peer's. A carried
//     announcement would have no responder left to withdraw it and no cap worker
//     left to time it out, so the peer would discard the victim's traffic for the
//     life of the daemon. The withdrawal is on `configured` and not on the parsed
//     response-level, which a removed section leaves at the DefaultConfig value
//     of alert: reading the removal off that default would make the guard vanish
//     the day the default changes, with no line deleted and no test red
//     (docs/contributing/ze-go-style.md -- a zero value is never an answer).
//     Deleting the PARENT `ddos` block reaches here the same way: its one
//     removed key "ddos" matches as an ancestor of "ddos/flowspec" in
//     rootHasChanges (plugin/server/reload.go) since 2026-10-10.
//   - response-level left enforce. `alert` is "detect and report, do not act"
//     (onCharacterized logs "would announce" and returns), and lifting an
//     upstream discard is what an operator commits it for. Neither
//     enforceMaxDuration nor probeTick reads response-level, so a carried
//     announcement would outlive the instruction by up to
//     max-mitigation-duration, 3600 seconds by default, with show ddos flowspec
//     naming a rule the operator has just asked to end.
//
// prev MAY be nil, which is the first configure.
//
// Whatever became of the announcement, prev is RETIRED here, inside the same
// critical section: it is unreachable from this point (not in activeResponder,
// unsubscribed), so anything still holding a pointer to it must not be able to
// announce through it. An event dispatched before the unsubscribe is exactly
// such a holder (responder.retire, responder.go).
func replaceResponder(cfg *Config, dispatcher routeDispatcher, prev *responder, configured bool) *responder {
	r := newResponder(cfg, dispatcher)

	if prev != nil {
		// One critical section covers the whole handover: what becomes of the
		// outgoing responder's rule, the carry into the new one, and the publish.
		// A cap tick or a probe tick that raced the swap then finds prev either
		// before the handover or after it, never halfway through.
		prev.mu.Lock()
		defer prev.mu.Unlock()

		switch {
		case !configured:
			prev.withdrawForConfig(withdrawSectionRemoved)
		case cfg.ResponseLevel != responseEnforce:
			prev.withdrawForConfig(withdrawNotEnforcing)
		}
		prev.retire()
	}

	r.adoptAnnouncement(prev)
	activeResponder.Store(r)
	return r
}

// exitWithdrawBudget bounds the withdrawal stopResponder sends, and the record
// removal that follows it. It sits below pluginRemovalGrace, the 500 ms the
// engine waits for this plugin's `bye` answer before it stops the process
// anyway (internal/component/plugin/server/startup_removal.go), so a slow
// engine costs the withdrawal, which leaves the record for the next start and
// for `ze doctor`, and never the removal itself.
const exitWithdrawBudget = 400 * time.Millisecond

// stopResponder takes r out of service on the plugin's way out and withdraws
// the FlowSpec rule it still announces. A nil r is the engine that never
// configured, and an r with nothing live sends nothing.
//
// It is named for the whole act rather than for the flag it ends with, because
// it does two things: (*responder).retire only sets that flag
// (docs/contributing/ze-go-style.md -- do not overload a name).
//
// The rule has to go out with the plugin because nothing else knows it is
// there: the FlowSpec NLRI lives in the BGP engine's RIB and in every peer's,
// and the engine keeps no record of what a plugin announced through
// update-route (withdrawPluginRoutes covers route-install only).
//
// It has two callers, and the second finds nothing live when the first ran:
//
//   - the `bye` handler, the one moment the engine waits for this plugin with
//     the connection open (notifyPluginRemoval sends `bye` and waits for the
//     answer before Process.Stop refuses the plugin's calls);
//   - the deferred call in runEngine, for every other end where the engine
//     still answers: Run returning, a panic unwinding the engine function.
//
// The withdrawal runs under a context of its own, bounded by
// exitWithdrawBudget and NOT derived from the signal context: on the way out
// that context is already canceled, and a withdrawal sent under it would be
// refused before it left.
//
// At a daemon stop the engine sends no `bye` and closes the connection, so the
// withdrawal cannot leave. That is one INFO line, not an error: the BGP
// sessions end with the daemon, the peers drop the rule with the session (or
// hold it as stale under graceful restart), and the record stays for the next
// start to withdraw.
func stopResponder(r *responder) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.retire()

	if !r.active {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), exitWithdrawBudget)
	defer cancel()

	err := r.withdrawThrough(r.dispatcher.WithContext(ctx))
	if err == nil {
		return
	}
	if transportClosed(err) {
		logger().Info("ddos-flowspec: the connection to the engine is closed, so the FlowSpec rule is not withdrawn on the way out; the BGP sessions end with the daemon, and the next start withdraws the recorded rule",
			"target", r.target.DstPrefix)
		return
	}
	logger().Warn("ddos-flowspec: the FlowSpec rule could not be withdrawn on the way out; it stays recorded, the next start withdraws it and ze doctor reports it until then",
		"target", r.target.DstPrefix, "error", err, "code", diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
}

// transportClosed reports whether err says the connection to the engine is
// gone, on either transport: the RPC connection (rpc.ErrMuxConnClosed, or the
// socket's own close error) or the in-process bridge (rpc.ErrBridgeClosed).
func transportClosed(err error) bool {
	if errors.Is(err, rpc.ErrMuxConnClosed) {
		return true
	}
	if errors.Is(err, rpc.ErrBridgeClosed) {
		return true
	}
	return errors.Is(err, net.ErrClosed)
}

// maxDurationCheckInterval is how often the plugin re-checks whether a live
// announce has outlived max-mitigation-duration. One second is finer than any
// cap the YANG admits (the minimum meaningful value is 1) and costs one wakeup
// per second holding no lock unless an announce is live.
const maxDurationCheckInterval = time.Second

// startMaxDurationWorker starts the plugin's one cap worker and returns a
// channel closed when it has exited. The worker stops when ctx is canceled;
// the caller MUST cancel ctx and then wait on the channel, so a tick in flight
// (enforceMaxDuration, a dispatcher round trip) finishes before the caller
// goes on.
//
// One long-lived worker for the whole plugin, not one per announce: it reads
// whichever responder is live through activeResponder, so a config apply that
// replaces the responder needs no restart here, and there is no goroutine per
// event (ai/rules/goroutine-lifecycle.md). enforceMaxDuration is a no-op unless
// an announce is live and the operator set a cap, so it ticks cheaply through
// the common idle case.
func startMaxDurationWorker(ctx context.Context, every time.Duration) (exited <-chan struct{}) {
	done := make(chan struct{})
	go runMaxDurationWorker(ctx, every, done)
	return done
}

// runMaxDurationWorker is the cap worker's loop. It closes done when it returns.
func runMaxDurationWorker(ctx context.Context, every time.Duration, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	// Bounded by ctx: the loop ends when the plugin's signal context ends.
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if r := activeResponder.Load(); r != nil {
				r.enforceMaxDuration()
			}
		}
	}
}

// stateCallTimeout bounds one runtime state call, as the SDK's StateKeys
// adapter bounds each of its calls.
const stateCallTimeout = 5 * time.Second

// sdkDispatcher is the production routeDispatcher. It sends rendered
// update-text commands to the BGP engine through the plugin SDK's UpdateRoute,
// and keeps the announcement record through the SDK state calls, which reach
// the daemon's store on both transports.
type sdkDispatcher struct {
	p   *sdk.Plugin
	ctx context.Context
}

func (d sdkDispatcher) Dispatch(command string) error {
	_, _, err := d.p.UpdateRoute(d.ctx, flowspecSelector, command)
	return err
}

// WithContext returns the same dispatcher under ctx.
func (d sdkDispatcher) WithContext(ctx context.Context) routeDispatcher {
	return sdkDispatcher{p: d.p, ctx: ctx}
}

// Record writes m as the live announcement. Only an acknowledged write returns nil.
func (d sdkDispatcher) Record(m flowspecMatch) error {
	data, err := encodeAnnouncementRecord(m)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(d.ctx, stateCallTimeout)
	defer cancel()
	return d.p.StatePut(ctx, announcementKey.Pattern, data)
}

// Forget removes the announcement record.
func (d sdkDispatcher) Forget() error {
	ctx, cancel := context.WithTimeout(d.ctx, stateCallTimeout)
	defer cancel()
	return d.p.StateRemove(ctx, announcementKey.Pattern)
}

// Recorded reads the announcement record. found is false when none is stored.
func (d sdkDispatcher) Recorded() (data []byte, found bool, err error) {
	ctx, cancel := context.WithTimeout(d.ctx, stateCallTimeout)
	defer cancel()
	return d.p.StateGet(ctx, announcementKey.Pattern)
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ddos-flowspec plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	dispatcher := sdkDispatcher{p: p, ctx: ctx}

	var resp *responder

	// Registered BEFORE the cap worker's own defer, so LIFO runs it AFTER the
	// worker has been canceled and waited for, and before p.Close: the
	// withdrawal is then the last thing this plugin sends the engine.
	// stopResponder says why the rule cannot be left behind, and why a `bye`
	// that already withdrew leaves this call nothing to do.
	defer func() {
		activeResponder.Store(nil)
		stopResponder(resp)
	}()

	parseSections := func(sections []sdk.ConfigSection) (*Config, bool, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, configured, err := ParseConfig(s.Data)
			if err != nil {
				return nil, false, fmt.Errorf("ddos-flowspec config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, false, fmt.Errorf("ddos-flowspec config: %w", err)
			}
			return cfg, configured, nil
		}
		return DefaultConfig(), false, nil
	}

	var (
		pending            pendingConfig
		unsubDetect        func()
		unsubCharacterized func()
		unsubCleared       func()
		unsubOngoing       func()
	)

	subscribe := func(bus ze.EventBus, r *responder) {
		unsubDetect = ddosevent.Detected.Subscribe(bus, r.onDetected)
		unsubCharacterized = ddosevent.Characterized.Subscribe(bus, r.onCharacterized)
		unsubCleared = ddosevent.Cleared.Subscribe(bus, r.onCleared)
		// The leak probe's only driver. Without it probe.Tick never runs outside
		// tests, and onCleared ignores the detector's clear while mitigating, so
		// nothing withdraws an announce at all.
		unsubOngoing = ddosevent.Ongoing.Subscribe(bus, r.onOngoing)
	}

	unsubscribe := func() {
		if unsubDetect != nil {
			unsubDetect()
		}
		if unsubCharacterized != nil {
			unsubCharacterized()
		}
		if unsubCleared != nil {
			unsubCleared()
		}
		if unsubOngoing != nil {
			unsubOngoing()
		}
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, configured, err := parseSections(sections)
		if err != nil {
			return err
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = replaceResponder(cfg, dispatcher, resp, configured)
		subscribe(bus, resp)
		log.Info("ddos-flowspec: configured", "response-level", cfg.ResponseLevel, "action", cfg.Action)
		return nil
	})

	// The first engine call this plugin makes is here: the state calls are
	// refused until the startup handshake ends. A rule a previous run left
	// announced is withdrawn before any detector event can announce a new one,
	// and nothing is adopted: protection after a restart comes from fresh
	// detection, as it does for ddos local.
	p.OnStarted(func(startCtx context.Context) error {
		withdrawRecordedAnnouncement(dispatcher.WithContext(startCtx))
		return nil
	})

	// The engine sends `bye` when it removes this plugin from a running daemon,
	// and waits for the answer with the connection open. The answer is always
	// success: a withdrawal that did not reach the engine leaves the record,
	// which the next start withdraws and `ze doctor` reports, while an error
	// answer would only keep a plugin the engine is removing alive.
	p.OnBye(func(_ string) error {
		stopResponder(resp)
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, configured, err := parseSections(sections)
		if err != nil {
			return err
		}
		pending.stage(cfg, configured)
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg, configured := pending.take()
		if cfg == nil {
			// Fail closed. The reload transaction drives verify and apply over the
			// SAME participant set -- runTxCoordinator builds both from `affected`
			// (plugin/server/reload_tx.go) -- so reaching apply without the config
			// OnConfigVerify stages here is a protocol violation, not a normal
			// state. Returning nil accepted the transaction while silently keeping
			// the PREVIOUS responder, with nothing logged at any level to say so.
			return errApplyWithoutVerify
		}
		unsubscribe()
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = replaceResponder(cfg, dispatcher, resp, configured)
		subscribe(bus, resp)
		// Mirrors the "configured" line OnConfigure emits, so a reload that reached
		// the responder and one that never did do not look identical in the log
		// (ai/rules/cli.md -- one stable phrase per outcome). `announced` is read
		// through status(), which takes no lock, because the responder's mu is held
		// across a dispatcher round trip to the BGP engine.
		announced, _, _ := resp.status()
		log.Info("ddos-flowspec: reconfigured", "response-level", cfg.ResponseLevel,
			"action", cfg.Action, "announced", announced)
		return nil
	})

	// A rollback is the end of the transaction, so the candidate this plugin
	// verified is unstaged rather than left for an apply that will never ask for
	// it (pendingConfig.clear).
	p.OnConfigRollback(pending.rollback)

	p.OnDoctorCheck(func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
		live, isLive := liveMatch()
		return answerOrphanRuntimeCheck(name, dispatcher, live, isLive)
	})

	// The cap worker stops when the signal context is canceled, which is the
	// plugin's own shutdown, and the wait after the cancel lets a tick in flight
	// finish before stopResponder runs (startMaxDurationWorker).
	workerExited := startMaxDurationWorker(ctx, maxDurationCheckInterval)
	defer func() {
		cancel()
		<-workerExited
	}()

	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
		DoctorChecks: []rpc.DoctorCheckDecl{orphanRuntimeCheckDecl},
	}); err != nil {
		log.Error("ddos-flowspec plugin failed", "error", err)
		unsubscribe()
		return 1
	}

	unsubscribe()
	log.Info("ddos-flowspec plugin stopped")
	return 0
}
