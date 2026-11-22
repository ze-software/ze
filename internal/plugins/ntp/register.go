package ntp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/statestore"
	ntpevents "github.com/ze-software/ze/internal/plugins/ntp/events"
	ntpyang "github.com/ze-software/ze/internal/plugins/ntp/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
	"github.com/ze-software/ze/pkg/zefs"
)

// eventBusMu guards eventBusRef.
var (
	eventBusMu  sync.Mutex
	eventBusRef ze.EventBus
)

const configRootEnvironment = "environment"

// configPathNTP is the one environment container ntp owns. Auto-load matches
// ConfigRoots against every container path of the config, so the bare
// environment root would start ntp for any environment block, such as
// api-server, including during reload compensation.
const configPathNTP = configRootEnvironment + "/ntp"

func setEventBus(eb ze.EventBus) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	eventBusRef = eb
}

func getEventBus() ze.EventBus {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	return eventBusRef
}

func init() {
	_ = events.RegisterNamespace(ntpevents.Namespace, ntpevents.EventClockSynced)

	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)

	statestore.RegisterPluginKeys("ntp", zefs.KeyNTPLastTime)

	reg := registry.Registration{
		Name:                    "ntp",
		Description:             "NTP client: system clock synchronization",
		Features:                "yang",
		YANG:                    ntpyang.ZeNTPConfYANG,
		ConfigRoots:             []string{configPathNTP},
		InProcessConfigVerifier: verifyNTPConfig,
		RunEngine:               runNTPPlugin,
		Commands:                commandDecls(),
	}
	registry.SetNTPSyncProvider(ntpSyncInfo)

	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return reg.YANG }
		cfg.ConfigLogger = func(level string) {
			loggerPtr.Store(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:    "ze-ntp:show-system-ntp",
			Handler:       forwardShowSystemNTP,
			PluginCommand: commandNTPStatus,
		},
		pluginserver.RPCRegistration{
			WireMethod:    "ze-ntp:show-system-ntp-peers",
			Handler:       forwardShowSystemNTPPeers,
			PluginCommand: commandNTPPeers,
		},
	)

	reg.ConfigureEngineLogger = func(loggerName string) {
		l := slogutil.Logger(loggerName)
		if l != nil {
			loggerPtr.Store(l)
		}
	}
	reg.ConfigureEventBus = func(eb ze.EventBus) {
		setEventBus(eb)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ntp: registration failed: %v\n", err)
		os.Exit(1)
	}

	// The client reads environment/ntp and adjusts the clock, so this plugin
	// owns the readiness checks that probe its servers and its CAP_SYS_TIME
	// (doctor.go).
	registerNTPDoctorChecks()
}

func verifyNTPConfig(sections []sdk.ConfigSection) error {
	for _, s := range sections {
		if s.Root != configRootEnvironment {
			continue
		}
		if _, err := parseNTPConfig(s.Data); err != nil {
			return fmt.Errorf("ntp: %w", err)
		}
	}
	return nil
}

// runNTPPlugin is the engine-mode entry point for the NTP plugin.
// errApplyWithoutVerify rejects a config-apply that arrives without the
// config-verify that stages its candidate. Returning nil accepted the
// transaction while silently keeping the RUNNING worker, so the daemon would
// report a successful reload over config it never installed.
var errApplyWithoutVerify = errors.New("ntp config apply: no verified config staged (config-apply arrived without config-verify); refusing to report success over the previous config")

// pendingNTPConfig is what OnConfigVerify stages for OnConfigApply.
//
// staged and cfg answer two different questions, and one pointer cannot answer
// both. staged says a verify ran for this transaction. cfg says what that
// verify found, and the ntp block being absent is a real answer: parseNTPConfig
// returns defaultConfig, whose Enabled is false, and startWorker stops the sync
// worker for it. Folding the two made nil mean both "nothing staged" and
// "nothing to do", so an apply arriving out of protocol left the worker running
// and reported success (ai/rules/principles.md -- a value that is silently
// wrong must not be reachable).
//
// NOT safe for concurrent use, and it does not need to be: the plugin SDK
// drives verify, apply and rollback for one transaction on one goroutine.
type pendingNTPConfig struct {
	cfg    ntpConfig
	staged bool
}

// stage records the candidate a config-verify accepted.
func (p *pendingNTPConfig) stage(cfg ntpConfig) {
	p.cfg = cfg
	p.staged = true
}

// take returns the staged candidate and unstages it, so a second apply behind
// one verify gets staged false and the caller's fail-closed branch.
func (p *pendingNTPConfig) take() (cfg ntpConfig, staged bool) {
	cfg, staged = p.cfg, p.staged
	p.clear()
	return cfg, staged
}

// clear unstages the candidate without applying it. The rollback path calls it:
// a transaction that verified here and then failed elsewhere never reaches this
// plugin's apply, and a candidate left staged is applied by the NEXT
// transaction that reaches an apply without a verify, restarting the sync
// worker on config the operator's commit never installed.
func (p *pendingNTPConfig) clear() {
	p.cfg = ntpConfig{}
	p.staged = false
}

// applyStagedConfig is the body of the plugin's OnConfigApply handler: it takes
// the candidate a config-verify staged and hands it to start.
//
// start is startWorker in production. It is a parameter so the verify-to-apply
// seam can be proven without a network, a clock write or an event bus.
func applyStagedConfig(pending *pendingNTPConfig, start func(ntpConfig)) error {
	cfg, staged := pending.take()
	if !staged {
		// Fail closed. The reload transaction drives verify and apply over the
		// SAME participant set -- runTxCoordinator builds both from `affected`
		// (internal/component/plugin/server/reload_tx.go) -- so reaching apply
		// with nothing staged is a protocol violation, not a normal state.
		return errApplyWithoutVerify
	}
	start(cfg)
	return nil
}

func runNTPPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("ntp plugin starting")

	p := sdk.NewWithConn("ntp", conn)
	defer func() { _ = p.Close() }()
	p.OnExecuteCommand(executeNTPCommand)

	var (
		workerMu     sync.Mutex
		worker       *syncWorker
		unsubscribe  func()
		drainRefusal *opererr.Error
		// leaving is the worker a bye told to stop and did not wait for. The
		// shutdown below joins it after Run returns.
		leaving     *syncWorker
		startedOnce sync.Once
		started     atomic.Bool
	)
	startedCh := make(chan struct{})

	takeWorker := func() (*syncWorker, func()) {
		workerMu.Lock()
		defer workerMu.Unlock()
		w, unsub := worker, unsubscribe
		worker, unsubscribe = nil, nil
		return w, unsub
	}

	// startWorker stops any existing worker, then starts a new one
	// with the given config. Safe to call multiple times (reload).
	startWorker := func(cfg ntpConfig) {
		workerMu.Lock()
		defer workerMu.Unlock()

		if worker != nil {
			worker.stopAndWait()
			worker = nil
		}
		if unsubscribe != nil {
			unsubscribe()
			unsubscribe = nil
		}
		if !cfg.Enabled {
			log.Debug("ntp: disabled in config")
			storeState(&syncState{Enabled: cfg.Enabled})
			return
		}
		// A replacement worker MUST publish before a reader can use its state.
		// Clearing the old snapshot makes the asynchronous first publication
		// an explicit unknown, rather than a stale enabled or disabled answer.
		storeState(nil)
		worker = newSyncWorker(cfg, getEventBus())
		worker.state = p
		worker.ready = startedCh
		worker.start()
		log.Info("ntp: sync worker started",
			"servers", cfg.Servers, "interval", cfg.IntervalSec)

		eb := getEventBus()
		if eb != nil {
			unsubscribe = subscribeDHCP(eb, worker)
		}
	}

	// pending holds the candidate between the verify and the apply phase.
	var pending pendingNTPConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRootEnvironment {
				continue
			}
			cfg, err := parseNTPConfig(s.Data)
			if err != nil {
				return fmt.Errorf("ntp: %w", err)
			}
			startWorker(cfg)
			return nil
		}
		return nil
	})

	// Release the worker's SDK state RPC calls only after the 5-stage startup
	// handshake completes (sdk.Plugin state calls MUST begin no earlier than
	// OnStarted).
	p.OnStarted(func(_ context.Context) error {
		started.Store(true)
		startedOnce.Do(func() { close(startedCh) })
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRootEnvironment {
				continue
			}
			cfg, err := parseNTPConfig(s.Data)
			if err != nil {
				return fmt.Errorf("ntp: %w", err)
			}
			pending.stage(cfg)
			return nil
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		return applyStagedConfig(&pending, startWorker)
	})

	// A rollback ends the transaction, so the candidate this plugin verified is
	// unstaged (pendingNTPConfig.clear). The SDK runs this handler only after an
	// apply this plugin accepted (pkg/plugin/sdk/config_tx_gate.go). A candidate
	// verified in a transaction that failed before this plugin's apply stays
	// staged, as after an abort, and the next verify replaces it: verify and
	// apply run over the same participant set.
	p.OnConfigRollback(func(_ string) error {
		pending.clear()
		return nil
	})

	// Persist the final clock snapshot while the daemon RPC transport is still
	// open (works identically for internal and external plugin processes).
	p.OnDrain(func(drainCtx context.Context, _ rpc.DrainInput) error {
		w, unsub := takeWorker()
		if unsub != nil {
			unsub()
		}
		if refusal := finishNTPWorkerContext(drainCtx, w); refusal != nil {
			workerMu.Lock()
			drainRefusal = refusal
			workerMu.Unlock()
			return refusal
		}
		return nil
	})

	// The engine waits pluginRemovalGrace (500 ms) for this answer and refuses
	// the reload when it is late, so the bye does only what fits: it saves the
	// last-known time under byeSaveBudget, while the connection is still open,
	// and tells the worker to stop. It does not wait for the worker, which can
	// be inside an ntp.Query for about 5 seconds: Run returns once the bye is
	// answered, and the join below runs then. A save that did not land is the
	// plugin's refusal at exit, never the removal's.
	p.OnBye(func(_ string) error {
		if !started.Load() {
			return nil
		}
		w, unsub := takeWorker()
		if unsub != nil {
			unsub()
		}
		refusal := leaveNTPWorker(context.Background(), w)
		workerMu.Lock()
		leaving = w
		if refusal != nil {
			drainRefusal = refusal
		}
		workerMu.Unlock()
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	runErr := p.Run(ctx, &sdk.Registration{
		Commands:     commandDecls(),
		WantsConfig:  []string{configRootEnvironment},
		VerifyBudget: 2,
		// ApplyBudget in seconds (orchestrator sums per-tier maxima). The
		// worst-case OnConfigApply cost is the synchronous worker handoff:
		// stopAndWait now returns within one in-flight ntp.Query (~5s, library
		// default) plus up to 250ms jitter and goroutine handoff. 10s leaves
		// comfortable headroom over that residual one-query wait while dead
		// servers keep the phase-2 retry loop busy (startup-resilience FIX 1).
		ApplyBudget: 10,
		DrainBudget: 5,
	})

	// Shutdown: save time (when not already saved during OnDrain/OnBye), stop
	// the worker, and join the worker a bye signaled.
	w, unsub := takeWorker()
	if unsub != nil {
		unsub()
	}
	if w != nil {
		if !started.Load() {
			// Startup aborted before OnStarted: the SDK state RPC transport
			// never became active, so stop the worker without sending StatePut.
			w.state = nil
		}
	}
	shutdownErr := finishNTPWorkerContext(context.Background(), w)
	workerMu.Lock()
	left := leaving
	workerMu.Unlock()
	if left != nil {
		// The bye saved the time and signaled this worker; its in-flight query
		// is waited out here, where no answer is owed to the engine.
		left.stopAndWait()
	}
	workerMu.Lock()
	if shutdownErr == nil {
		shutdownErr = drainRefusal
	}
	workerMu.Unlock()
	refusal := shutdownErr
	if runErr != nil {
		if shutdownErr != nil {
			runErr = errors.Join(runErr, shutdownErr)
		}
		refusal = opererr.Wrap(runErr, "NTP plugin failed",
			"The plugin could not complete startup or runtime. Its configured worker and subscriptions have been stopped.",
			"ze show doctor")
	}
	if refusal != nil {
		_, _ = fmt.Fprintln(os.Stderr, refusal.Error())
		return 1
	}

	log.Info("ntp plugin stopped")
	return 0
}

// finishNTPWorker stops the worker even when its final managed-store write
// fails. A failed save is a failed shutdown, with its original cause retained;
// the caller releases its remaining resources before rendering the refusal.
func finishNTPWorker(worker *syncWorker) *opererr.Error {
	return finishNTPWorkerContext(context.Background(), worker)
}

func finishNTPWorkerContext(ctx context.Context, worker *syncWorker) *opererr.Error {
	if worker == nil {
		return nil
	}
	defer worker.stopAndWait()
	return saveFinalTime(ctx, worker)
}

// byeSaveBudget bounds the last-known-time save a bye performs. It sits below
// pluginRemovalGrace, the 500 ms the engine waits for the bye answer before it
// refuses the reload (internal/component/plugin/server/startup_removal.go), so
// a slow store costs the saved time, reported at exit, and never the removal.
const byeSaveBudget = 400 * time.Millisecond

// leaveNTPWorker is the body of the plugin's OnBye handler for a running
// worker: it saves the last-known time within byeSaveBudget and signals the
// worker to stop, without waiting for it. The caller MUST join the worker
// with stopAndWait once the bye is answered. A nil worker has nothing to
// leave.
//
// The state put is issued from the SDK's callback loop, and it does not wait
// on that loop: its answer arrives on the connection's own read loop
// (rpc.MuxConn) or, for a plugin in the daemon, is the return of a direct
// call (DirectBridge).
func leaveNTPWorker(ctx context.Context, worker *syncWorker) *opererr.Error {
	if worker == nil {
		return nil
	}
	defer worker.requestStop()
	saveCtx, cancel := context.WithTimeout(ctx, byeSaveBudget)
	defer cancel()
	return saveFinalTime(saveCtx, worker)
}

// saveFinalTime writes the last-known time for a worker that is ending.
// persist-path enables the final save; the managed store owns its location.
func saveFinalTime(ctx context.Context, worker *syncWorker) *opererr.Error {
	if worker.cfg.PersistPath == "" {
		return nil
	}
	if err := saveTime(ctx, worker.state, currentTime()); err != nil {
		return opererr.Wrap(err, "NTP shutdown could not save the last-known time",
			"The managed state store refused the write, so the next boot cannot rely on a fresh saved clock. Inspect the store and restore write access before restarting NTP.",
			"ze show doctor")
	}
	return nil
}

// currentTime returns the current system time.
func currentTime() time.Time {
	return time.Now()
}

// ntpSyncInfo reads the NTP owner's runtime answer, including when that owner
// is forked. Only an explicit inactive answer permits a date without metadata.
func ntpSyncInfo(query registry.CommandQuery) (map[string]any, *opererr.Error) {
	if query == nil {
		return nil, ntpReadRefusal(errors.New("the daemon command reader is unavailable"))
	}
	data, err := query(commandNTPStatus)
	if err != nil {
		return nil, ntpReadRefusal(err)
	}
	enabled, err := ntpReplyEnabled(data)
	if err != nil {
		return nil, ntpReadRefusal(err)
	}
	if !enabled {
		// An explicit inactive answer needs no measurements. Nil metadata with
		// no refusal is that answer under the NTPSyncProvider contract.
		return nil, nil
	}
	metadata, err := ntpActiveMetadata(data)
	if err != nil {
		return nil, ntpReadRefusal(err)
	}
	return metadata, nil
}

// ntpReplyEnabled reads whether the NTP owner answered as an active client.
// A missing or non-boolean enabled field is unknown, never an inactive client.
// A caller MUST read enabled with this function, or an equivalent check, and
// MUST NOT call ntpActiveMetadata unless it answered true.
func ntpReplyEnabled(data map[string]any) (bool, error) {
	enabled, ok := data[fieldEnabled].(bool)
	if !ok {
		return false, errors.New("the NTP response has no boolean enabled field")
	}
	return enabled, nil
}

// ntpActiveMetadata validates an active answer's synchronization fields before
// projecting them. The caller MUST have read enabled=true with ntpReplyEnabled
// or an equivalent check first: an inactive answer carries no measurements.
func ntpActiveMetadata(data map[string]any) (map[string]any, error) {
	synced, ok := data["synced"].(bool)
	if !ok {
		return nil, errors.New("the active NTP response has no boolean synced field")
	}
	source, ok := data["source"].(string)
	if !ok {
		return nil, errors.New("the active NTP response has no string source field")
	}
	offset, ok := data["offset"].(string)
	if !ok {
		return nil, errors.New("the active NTP response has no string offset field")
	}
	if _, err := time.ParseDuration(offset); err != nil {
		return nil, fmt.Errorf("the active NTP offset is unreadable: %w", err)
	}
	return map[string]any{
		"ntp-synced": synced,
		"ntp-source": source,
		"ntp-offset": offset,
	}, nil
}

func ntpReadRefusal(cause error) *opererr.Error {
	return ntpCommandReadRefusal("show system date", cause)
}

func ntpCommandReadRefusal(name string, cause error) *opererr.Error {
	return opererr.New(name+" cannot read NTP state",
		cause.Error()+" The NTP state must be read from its running owner before this command can succeed.",
		"ze show system ntp")
}

// fieldEnabled is the YANG leaf that switches NTP on, and the JSON key the
// status view answers with.
const fieldEnabled = "enabled"

// fieldPeers and fieldPeerCount are the JSON keys the peers view answers
// with: one row for each server, and the number of rows in that array.
const (
	fieldPeers     = "peers"
	fieldPeerCount = "count"
)

// handleShowSystemNTP returns the NTP sync status summary.
func handleShowSystemNTP(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	st := loadState()
	if st == nil {
		return plugin.OperatorErrorResponse(ntpStateUnavailable()), nil
	}
	if !st.Enabled {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{fieldEnabled: false},
		}, nil
	}
	data := map[string]any{
		fieldEnabled:    true,
		"synced":        st.Synced,
		"source":        st.Source,
		"offset":        st.Offset.String(),
		"stratum":       st.Stratum,
		"poll-interval": st.PollInterval,
	}
	if !st.LastSync.IsZero() {
		data["last-sync"] = st.LastSync.Format(time.RFC3339)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

// handleShowSystemNTPPeers returns per-server NTP state.
func handleShowSystemNTPPeers(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	st := loadState()
	if st == nil {
		return plugin.OperatorErrorResponse(ntpStateUnavailable()), nil
	}
	if !st.Enabled {
		return plugin.OperatorErrorResponse(ntpCommandReadRefusal(commandNTPPeers,
			errors.New("the NTP worker is disabled while the daemon is reading an active client"))), nil
	}
	if len(st.Servers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{fieldPeers: []map[string]any{}, fieldPeerCount: 0},
		}, nil
	}
	peers := make([]map[string]any, 0, len(st.Servers))
	for i := range st.Servers {
		s := &st.Servers[i]
		row := map[string]any{
			"address": s.Address,
			"offset":  s.Offset.String(),
			"rtt":     s.RTT.String(),
			"stratum": s.Stratum,
			"reach":   s.Reach,
		}
		if !s.LastQuery.IsZero() {
			row["last-query"] = s.LastQuery.Format(time.RFC3339)
		}
		if s.LastError != "" {
			row["last-error"] = s.LastError
		}
		peers = append(peers, row)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldPeers: peers, fieldPeerCount: len(peers)},
	}, nil
}
