package local

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/slogutil"
	localyang "github.com/ze-software/ze/internal/plugins/ddos/local/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// errApplyWithoutVerify rejects a config-apply that arrives without the
// config-verify that stages the candidate. The message names the stale-config
// consequence, because that is what the operator actually experiences: a reload
// that reports success and changes nothing.
var errApplyWithoutVerify = errors.New("ddos-local config apply: no verified config staged (config-apply arrived without config-verify); refusing to report success over the previous config")

var eventBusPtr atomic.Pointer[ze.EventBus]

// activeResponder publishes the live responder to the in-process show handler
// (show.go). Nil when the plugin is not configured/running.
var activeResponder atomic.Pointer[responder]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-local: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:         Name,
		Description:  "DDoS local responder: on-host nft drop on attack detection",
		Features:     "yang",
		YANG:         localyang.ZeDdosLocalConfYANG,
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{"firewall"},
		RunEngine:    runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			eventBusPtr.Store(&eb)
		},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ddos-local: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// maxDurationCheckInterval is how often the plugin re-checks whether a live drop
// rule has outlived max-mitigation-duration. One second is finer than any cap
// the YANG admits (the shortest meaningful value is 1) and costs one wakeup per
// second holding no lock unless a rule is installed. It keeps the name its
// flowspec sibling uses, so a reader who greps one finds both.
const maxDurationCheckInterval = time.Second

// startMaxDurationWorker starts the one worker goroutine that removes a drop
// rule which has outlived max-mitigation-duration.
//
// One worker serves the whole plugin, never one per mitigation: it reads
// whichever responder is live through activeResponder, so a config apply that
// replaces the responder needs no restart here
// (ai/rules/goroutine-lifecycle.md). enforceMaxDuration is a no-op unless a
// rule is installed and the operator set a cap, so an unattacked box pays one
// wakeup a second and nothing else.
//
// The caller owns ctx and MUST cancel it to stop the worker. The returned
// channel is closed after the worker has exited, so a caller that must know the
// worker is gone waits on it after the cancel.
func startMaxDurationWorker(ctx context.Context, every time.Duration) (exited <-chan struct{}) {
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
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
	}()

	return done
}

// replaceResponder builds the responder for cfg, decides what becomes of a drop
// rule the responder it replaces installed, and publishes it to the cap worker
// and to the show handler.
//
// configured says the delivered config still carries a `ddos local` section.
//
// The default is to CARRY the rule, and carrying is what stops an ordinary
// commit orphaning it. The firewall registry is keyed by TABLE NAME rather than
// by responder, so the nftables table applyMitigation registered outlives the
// responder that registered it. A fresh idle responder believes no rule exists,
// and both removal paths return on !active -- enforceMaxDuration and onCleared
// alike -- so the drop would be bounded by neither the operator's cap nor a
// clear, for the life of the daemon, while show ddos local reported no
// mitigation.
//
// Three commits say the opposite, that the box must stop dropping, and each
// withdraws instead. All go through the OUTGOING responder, because the
// incoming one has installed nothing and its own removal paths return on
// !active:
//
//   - The section is gone. The plugin is STOPPED as soon as this reload
//     transaction commits (Server.collectProcessesForRemovedConfigPaths,
//     internal/component/plugin/server/startup_autoload.go), and it runs
//     in-process, so the firewall registry it wrote to is the daemon's own and
//     outlives this engine. A carried rule would sit in the kernel with no
//     responder to see it and no cap worker to remove it, for the life of the
//     daemon. The withdrawal is on `configured` and not on the parsed
//     response-level, which a removed section leaves at the DefaultConfig value
//     of alert: reading the removal off that default would make the guard vanish
//     the day the default changes, with no line deleted and no test red
//     (docs/contributing/ze-go-style.md -- a zero value is never an answer).
//   - response-level left enforce. `alert` is "detect and report, do not block"
//     (applyMitigation), and un-blackholing a victim is what an operator commits
//     it for. Neither enforceMaxDuration nor onCleared reads response-level, so a
//     carried rule would outlive the instruction by up to
//     max-mitigation-duration, 3600 seconds by default, with show ddos local
//     naming a drop the operator has just asked to end.
//   - forward-mitigation left true while the live rule sits on the FORWARD hook.
//     The leaf's whole subject is whether this box drops a transit victim's
//     traffic on-host, and turning it off is the commit that says stop
//     (hookForDirection, responder.go, refuses a remote victim without it). The
//     same argument as response-level applies: no later event removes the rule,
//     and the one path that could is applyMitigation, which now returns before
//     reaching a removal. The hook is read rather than the direction so an INPUT
//     drop for a LOCAL victim, which this leaf says nothing about, is left alone.
//
// prev MAY be nil, which is the first configure.
//
// Whatever became of the rule, prev is RETIRED here, inside the same critical
// section: it is unreachable from this point (not in activeResponder,
// unsubscribed), so anything still holding a pointer to it must not be able to
// install through it. An event dispatched before the unsubscribe is exactly such
// a holder (responder.retire, responder.go).
func replaceResponder(cfg *Config, bus eventBus, prev *responder, configured bool) *responder {
	r := newResponder(cfg, bus)

	if prev != nil {
		// One critical section covers the whole handover: what becomes of the
		// outgoing responder's rule, the carry into the new one, and the publish.
		// A cap tick or a clear that raced the swap then finds prev either before
		// the handover or after it, never halfway through, and never re-installs
		// between a withdrawal and the carry that would not have seen it.
		// adoptMitigation states the obligation.
		prev.mu.Lock()
		defer prev.mu.Unlock()

		switch {
		case !configured:
			prev.withdrawMitigation(withdrawSectionRemoved)
		case cfg.ResponseLevel != responseEnforce:
			prev.withdrawMitigation(withdrawNotEnforcing)
		case prev.hook == firewall.HookForward && !cfg.ForwardMitigation:
			prev.withdrawMitigation(withdrawForwardDisabled)
		}
		prev.retire()
	}

	r.adoptMitigation(prev)
	activeResponder.Store(r)
	return r
}

// pendingConfig is what OnConfigVerify stages for OnConfigApply: the parsed
// config, and whether the delivered section carried a `ddos local` body at all.
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
// plugin's apply, and a candidate left staged is applied by the NEXT
// transaction that reaches an apply without a verify. That is the state the
// apply's nil check exists to refuse, and a stale candidate defeats it -- the
// pair a rolled-back removal leaves behind is (DefaultConfig, false), so the
// next apply would withdraw a live drop rule nobody asked it to remove.
func (p *pendingConfig) clear() {
	p.cfg = nil
	p.configured = false
}

// rollback is the plugin's config-rollback handler, and unstaging the candidate
// is the whole of its job, so it IS this type's method rather than a closure in
// runEngine that calls one. The handler the SDK is given is then the function a
// test can drive.
func (p *pendingConfig) rollback(string) error {
	p.clear()
	return nil
}

// clearStaleDropRule removes a previous process's ze_ddos-local response table.
// Cleanup failures are returned for best-effort startup logging.
//
// A ddos drop is an attack response and not provisioned state. It is never saved
// across a restart, because a reboot can be exactly how an operator clears state,
// and protection afterwards comes from the attack being DETECTED again
// (docs/guide/ddos-mitigation.md carries the operator's exposure window).
//
// The exit-path withdrawal is best-effort at a daemon stop (stopResponder).
// A crash runs no exit path, and flush-on-shutdown can leave tables installed.
//
// The nft backend refuses to delete a ze_ table it did not apply itself.
// shouldDeleteTable (internal/plugins/firewall/nft/backend_linux.go) sweeps a ze_
// name only when it is in the desired set or in THIS backend instance's applied
// map, and a fresh process's map is empty.
//
// The desired set is the one lever this package holds, so the sweep uses it. Two
// reconciles, in this order:
//
//  1. Register a table carrying the name and no chains, then reconcile. The
//     backend deletes every kernel table of that name and creates the empty one
//     in its place. desiredNames is keyed by NAME alone, so an ip table and an ip6
//     table both go. That is what a responder which picks its family from the
//     victim prefix can leave behind (familyFromPrefix, responder.go).
//  2. Withdraw the name and reconcile again. The empty table is in the backend's
//     applied map now, so this reconcile deletes it and the kernel holds no
//     ze_ddos-local table at all.
//
// One reconcile cannot do it. A name in neither the desired set nor the applied
// map is a name the backend leaves alone, which is what makes the stale table
// survive in the first place. Both reconciles preserve other owners' desired
// rules through the shared firewall registry.
//
// It also carries the one-time removal of the tables an older ze build wrote
// under a name with no ownership prefix. That removal runs inside Backend.Apply,
// and ApplyAll returns early when the merged desired set is empty and no backend
// is loaded. Step 1 always presents a non-empty set, so ddos-local needs no
// separate trigger for it (internal/component/firewall/legacy_tables.go).
//
// The caller MUST run this once at initial OnConfigure, after the firewall
// dependency configured its backend and before any responder subscribes.
// Engines start concurrently before dependency-tier handshakes, so calling this
// before the SDK handshake can reconcile against the wrong, autoloaded backend.
func clearStaleDropRule() error {
	// No chains: the table exists for one reconcile, only so the backend counts
	// the name as one it owns.
	claim := firewall.Table{Name: tableName, Family: firewall.FamilyIP}
	if err := registerTables(tableName, []firewall.Table{claim}); err != nil {
		return fmt.Errorf("claiming the ddos-local table name: %w", err)
	}
	if err := applyAll(); err != nil {
		// Leave no claim behind: a registration kept here would put an empty
		// ddos-local table into the kernel on the next owner's reconcile.
		_ = registerTables(tableName, nil) // a withdraw registers no name, so it cannot be refused
		return fmt.Errorf("sweeping a drop rule left by a previous process: %w", err)
	}

	_ = registerTables(tableName, nil) // a withdraw registers no name, so it cannot be refused
	if err := applyAll(); err != nil {
		return fmt.Errorf("removing the empty ddos-local table: %w", err)
	}
	return nil
}

// stopResponder takes r out of service on the engine's exit path and removes any
// drop rule it still owns. A nil r is the engine that never configured.
//
// It is named for the whole act rather than for the flag it ends with, because
// it does two things: (*responder).retire only sets that flag
// (docs/contributing/ze-go-style.md -- do not overload a name).
//
// The rule goes out with the engine because nothing survives the engine that
// could remove it: the responder becomes unreachable, the cap worker exits, and
// the firewall registry the rule lives in belongs to the daemon rather than to
// this plugin, so it outlives both (firewall.RegisterTables, keyed by table
// name). FlushAllTables runs only from the firewall engine's own clean shutdown,
// which any other firewall dependent keeps from happening.
//
// This is the exit path, beside the config boundary in replaceResponder, and the
// two cover different stops. replaceResponder covers a stop that DELIVERS a
// config: `delete ddos local`, a response-level that leaves enforce,
// forward-mitigation turned off. A stop that delivers NOTHING reaches only this
// one, and deleting the PARENT `ddos` block is such a stop: diffMapsRecursive
// (internal/component/config/diff.go) records the one key "ddos" for the whole
// subtree, which rootHasChanges (plugin/server/reload.go) does not match against
// the root "ddos/local", so no verify and no apply is delivered -- while
// parentRemoved (plugin/server/startup_autoload.go) does match it and stops the
// plugin anyway. That delivery defect is recorded in
// plan/journal/component-rebuilt-during-reload.md; this path is what stops it
// blackholing a victim in the meantime.
//
// The two cannot double-withdraw. withdrawMitigation returns on !active, and a
// config-boundary withdrawal leaves the responder it acted on inactive while the
// engine goes on holding the fresh idle one this call then finds.
//
// At a DAEMON stop this call is BEST-EFFORT and promises nothing. No caller and
// no page may state that a daemon stop removes the drop.
//
// ProcessManager.Stop cancels every plugin at once, calls Process.Stop over its
// process map in map order, then waits on every engine concurrently
// (internal/component/plugin/process/manager.go). Nothing orders this withdrawal
// against the firewall engine's own exit. That engine goes to CloseBackend as
// soon as its p.Run returns, and at `flush-on-shutdown false` it does not flush
// first (internal/component/firewall/engine.go). Once activeBackend is nil,
// ApplyAll writes nothing to the kernel
// (internal/component/firewall/registry.go). The same loss happens whenever this
// path costs more than the manager's stop grace.
//
// The next ddos-local engine start attempts clearStaleDropRule after the firewall
// dependency configures its backend. A failure is logged and startup continues.
// Protection then comes from fresh detection, with the exposure window described
// in docs/guide/ddos-mitigation.md.
func stopResponder(r *responder) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.withdrawMitigation(withdrawEngineStopped)
	r.retire()
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ddos-local plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	var resp *responder

	// Registered BEFORE the cap worker's own defer, so LIFO runs it AFTER the
	// worker has been canceled and waited for: the withdrawal is then the last
	// thing this plugin does to the kernel. stopResponder says why the rule
	// cannot be left behind.
	defer func() {
		activeResponder.Store(nil)
		stopResponder(resp)
	}()

	// parseSections returns the ddos local config the delivered sections carry,
	// and whether they carry one at all. The second answer is what a removal
	// looks like from in here: the server sends an empty body for a root the new
	// tree no longer holds, so an absent section is a value the plugin is TOLD
	// rather than a message it misses.
	parseSections := func(sections []sdk.ConfigSection) (*Config, bool, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, configured, err := ParseConfig(s.Data)
			if err != nil {
				return nil, false, fmt.Errorf("ddos-local config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, false, fmt.Errorf("ddos-local config: %w", err)
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
	)

	subscribe := func(bus ze.EventBus, r *responder) {
		unsubDetect = ddosevent.Detected.Subscribe(bus, r.onDetected)
		unsubCharacterized = ddosevent.Characterized.Subscribe(bus, r.onCharacterized)
		unsubCleared = ddosevent.Cleared.Subscribe(bus, r.onCleared)
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
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		// The firewall dependency has completed its configure before this tier.
		// Sweep before any responder can receive events, once per engine start.
		// Reload uses verify/apply and MUST NOT sweep a live response table.
		if err := clearStaleDropRule(); err != nil {
			log.Warn("ddos-local: could not clear a drop rule left by a previous process",
				"error", err, "action", "inspect the configured firewall backend for stale ddos-local state")
		}

		cfg, configured, err := parseSections(sections)
		if err != nil {
			return err
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = replaceResponder(cfg, bus, resp, configured)
		subscribe(bus, resp)

		log.Info("ddos-local: configured", "response-level", cfg.ResponseLevel)
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
			// (plugin/server/reload_tx.go:40-45) -- so reaching apply without the
			// config OnConfigVerify stages here is a protocol violation, not a
			// normal state. Returning nil accepted the transaction while silently
			// keeping the PREVIOUS responder, so `ddos { local { forward-mitigation
			// false } }` could be reloaded, the daemon would print "sighup reload
			// complete", and the old value kept mitigating -- with nothing logged at
			// any level to say so. Reject instead, so the reload fails loudly and
			// rolls back rather than reporting success over stale config.
			// See ai/rules/evidence.md.
			return errApplyWithoutVerify
		}
		unsubscribe()
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = replaceResponder(cfg, bus, resp, configured)
		subscribe(bus, resp)
		// Mirrors the "configured" line OnConfigure emits. Without it a reload that
		// reached the responder and one that never did looked identical in the log,
		// which is what made the forward-mitigation reload failure undiagnosable
		// from a QEMU run (ai/rules/cli.md -- one stable phrase per
		// outcome).
		log.Info("ddos-local: reconfigured", "response-level", cfg.ResponseLevel,
			"forward-mitigation", cfg.ForwardMitigation)
		return nil
	})

	// A rollback is the end of the transaction, so the candidate this plugin
	// verified is unstaged rather than left for an apply that will never ask for
	// it (pendingConfig.clear).
	p.OnConfigRollback(pending.rollback)

	ctx, cancel := sdk.SignalContext()

	// The cap worker outlives every config apply and stops when the signal
	// context is canceled, which is the plugin's own shutdown. The wait after the
	// cancel is owed: a tick in flight sits inside enforceMaxDuration ->
	// removeMitigation -> applyAll, a netlink round trip, so returning on the
	// cancel alone would report the engine done while it was still writing the
	// kernel (ai/rules/goroutine-lifecycle.md).
	//
	// The wait is bounded, and the bound is the firewall's rather than this
	// plugin's: applyAll takes the process-wide reconcileMu
	// (internal/component/firewall/registry.go) and every backend operation
	// behind it runs under firewall.MaxBackendDeadline, 60 seconds
	// (internal/component/firewall/backend.go). One tick is in flight at a time,
	// so the worst case is one queued reconcile for each firewall owner.
	workerExited := startMaxDurationWorker(ctx, maxDurationCheckInterval)
	defer func() {
		cancel()
		<-workerExited
	}()

	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("ddos-local plugin failed", "error", err)
		unsubscribe()
		return 1
	}

	unsubscribe()
	log.Info("ddos-local plugin stopped")
	return 0
}
