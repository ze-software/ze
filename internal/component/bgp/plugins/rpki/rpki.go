// Design: docs/architecture/plugin/rib-storage-design.md — RPKI origin validation plugin
// Detail: rpki_config.go — config parsing from OnConfigure JSON
// Detail: rtr_pdu.go — RTR PDU wire format types and parsing
// Detail: rtr_session.go — RTR session lifecycle management
// Detail: roa_cache.go — ROA cache VRP storage and covering-prefix lookup
// Detail: validate.go — RFC 6811 origin validation algorithm
// Detail: emit.go — RPKI validation event JSON building
package rpki

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/asn"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/core/textbuf"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	configRootBGP   = "bgp"
	configRootPKI   = "pki"
	commandShowRPKI = "show bgp rpki"
	shapeTab        = "tab"
	columnAddress   = "address"
)

// rpkiMetrics holds Prometheus metrics for the RPKI plugin.
type rpkiMetrics struct {
	vrpsCached         metrics.Gauge      // VRPs currently in ROA cache
	sessionsActive     metrics.Gauge      // active RTR sessions
	validationOutcomes metrics.CounterVec // origin validation results (labels: result)
	aspaOutcomes       metrics.CounterVec // ASPA path verification results (labels: result)
}

// rpkiMetricsPtr stores RPKI metrics, set by SetMetricsRegistry.
var rpkiMetricsPtr atomic.Pointer[rpkiMetrics]

// SetMetricsRegistry creates RPKI metrics from the given registry.
// Called via ConfigureMetrics callback before RunEngine.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &rpkiMetrics{
		vrpsCached:         reg.Gauge("ze_rpki_vrps_cached", "VRPs currently in ROA cache."),
		sessionsActive:     reg.Gauge("ze_rpki_sessions_active", "Active RTR cache sessions."),
		validationOutcomes: reg.CounterVec("ze_rpki_validation_outcomes_total", "RPKI origin validation outcomes.", []string{"result"}),
		aspaOutcomes:       reg.CounterVec("ze_rpki_aspa_outcomes_total", "ASPA path verification outcomes.", []string{"result"}),
	}
	rpkiMetricsPtr.Store(m)
}

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

const (
	statusDone  = "done"
	statusError = "error"
)

// validationRequest is a pending validation decision to be processed by the worker.
type validationRequest struct {
	peerAddr string
	// peerGroup is the name of the group the source session belongs to, empty for a
	// standalone peer. It is carried because a session created from a listen-range
	// group has an address the config document never mentions, so the group's name
	// is the only key its stated actions can be found under.
	peerGroup  string
	family     string
	prefix     string
	pathID     uint32
	msgID      uint64
	state      uint8 // origin validation state
	aspaState  uint8 // ASPA path verification state
	originAS   uint32
	blackhole  bool   // the announcement carried the RFC 7999 BLACKHOLE community
	generation uint64 // configuration generation that produced this verdict
}

// aspaOverridesAccept returns true if ASPA policy demands rejecting a route
// that origin validation would otherwise accept.
func aspaOverridesAccept(aspaState, invalidAction, unknownAction uint8) bool {
	switch aspaState {
	case ASPAInvalid:
		return invalidAction == ASPAPolicyReject
	case ASPAUnknown:
		return unknownAction == ASPAPolicyReject
	}
	return false
}

// rPKIPlugin implements the bgp-rpki plugin.
// It manages RTR sessions to RPKI cache servers, maintains the ROA cache,
// and validates received routes against VRPs.
type rPKIPlugin struct {
	plugin      *sdk.Plugin
	cache       *ROACache
	aspaCache   *aSPACache
	dataLease   *rtrDataLease
	aspaTracker *aSPATracker
	// originTracker records active routes for RFC 6811 Section 4 re-validation when the ROA
	// cache (VRP set) changes. Populated whenever origin validation runs (independent of ASPA).
	originTracker     *originTracker
	aspaEnabled       atomic.Bool
	aspaInvalidAction atomic.Uint32
	aspaUnknownAction atomic.Uint32
	// originInvalidAction is the RFC 6811 operator-configured action for the Invalid state
	// (ASPAPolicyReject/LogOnly/Accept); only Reject excludes the route (RFC 6811 Section 2/3).
	originInvalidAction atomic.Uint32
	// originNotFoundAction is the operator-configured action for the NotFound state.
	// Reject excludes the route; LogOnly keeps it with a warning; Accept keeps it silently.
	originNotFoundAction atomic.Uint32
	// perPeerActions holds per-peer resolved action overrides keyed by configjson.KeyFor
	// (a remote IP, or a dynamic group's name for the template its members inherit),
	// swapped atomically on each config reload. nil (or a miss) means the route uses the
	// global actions.
	// Read lock-free from the validation worker; published during config apply.
	perPeerActions atomic.Pointer[map[configjson.PeerConfigKey]peerActionSet]
	mu             sync.RWMutex

	// Serialize route registration with cache-change callbacks: a notification
	// must not run between the initial verdict and registration of its route.
	validationMu sync.Mutex
	// dispatchMu fences in-flight decisions while configuration changes policy.
	dispatchMu           sync.Mutex
	validationGeneration atomic.Uint64
	// sessions holds the RTR sessions of the current config generation, most preferred
	// first. startSessions is the only writer; a status snapshot reads it under mu.
	sessions []*RTRSession

	// groupStopCh stops the cache group that polls those sessions, and is nil while no
	// generation is running. A config reload calls startSessions again, and two groups
	// over one ROA cache would each replace the other's set at every poll, so
	// stopSessions closes this channel and waits before the next generation starts.
	groupStopCh chan struct{}

	// sessionWg tracks the cache group goroutine for clean shutdown.
	sessionWg sync.WaitGroup

	// validateCh receives validation decisions for async dispatch.
	// The worker goroutine drains this channel and issues DispatchCommand calls,
	// preventing blocking the SDK event callback goroutine.
	validateCh chan validationRequest

	// stopCh signals all background goroutines to stop.
	stopCh chan struct{}

	// active is true when at least one cache server is CONFIGURED. It says that RPKI is in
	// play, which is what the per-prefix work in handleEvent/handleStructuredUpdate and the
	// adj-rib-in validation gate need to know before any data can have arrived.
	//
	// It does NOT say that validation has usable data: no VRP need have arrived, and a cache
	// whose PDUs ze mis-decodes keeps it true forever. Whether a cache server has ever
	// completed a sync is per-session state (RTRSession.synced), reported by syncedSessions.
	active atomic.Bool
}

// commandDecls names the commands this plugin serves and states what each
// answer holds, so the engine publishes the operators a command supports and
// refuses the ones it cannot before the command is dispatched
// (pkg/plugin/rpc/types.go, CommandDecl).
//
// A column order names the keys of one ROW, in the order a person reads them,
// and every name below is a key the producing function writes. A name no
// producer writes orders nothing and publishes a field that does not exist to
// `| display` completion, and NOTHING fails when it is wrong:
// TestDeclaredColumnsExistInPayload drives each producer and holds the two
// together.
//
// `request bgp rpki validate` declares no shape. It answers one verdict for one
// prefix, and it is outside the population this spec measured, so it keeps the
// derived-at-apply-time behavior every undeclared command has.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{
			Name:      commandShowRPKI,
			ShortHelp: "Show RPKI validation counters with one row for each cache server",
			// appendSummaryFields writes the aggregate keys and
			// appendCacheServers the rows, as siblings at one level. The order
			// names the row keys: the aggregate half carries none of them, so
			// it keeps the alphabetical rendering it has today.
			Shape:         shapeTab,
			Columns:       []string{columnAddress, "port", "state", "synced", "version"},
			AddressFields: []string{columnAddress},
		},
		{
			Name:      "show bgp rpki status",
			ShortHelp: "Show RPKI validation status and cache server overview",
			// statusCommand writes two candidate row sets, "cache-servers" and
			// the per-peer actions, so rowsInKeyed
			// (internal/component/command/answer_shape.go) can choose neither
			// and the answer is one document.
			Shape: "doc",
		},
		{
			Name:      "show bgp rpki cache",
			ShortHelp: "Show RTR cache server sessions with protocol details",
			// cacheCommand writes the rows, adding the protocol detail of a
			// session to what the overview carries.
			Shape: shapeTab,
			Columns: []string{
				columnAddress, "port", "preference", "state", "synced", "version",
				"session-id", "serial", "refresh-interval", "retry-interval",
				"expire-interval",
			},
			AddressFields: []string{columnAddress},
		},
		{
			Name:      "show bgp rpki roa",
			ShortHelp: "Show ROA table entries or lookup covering VRPs for a prefix",
			// roaCommand and roaLookupCommand both write their rows under
			// "entries" in these three keys, so one declaration describes the
			// command whichever branch its argument takes.
			Shape:         shapeTab,
			Columns:       []string{"prefix", "max-length", "asn"},
			AddressFields: []string{"prefix"},
		},
		{
			Name:      "show bgp rpki summary",
			ShortHelp: "Show RPKI validation summary with session and ASPA counts",
			// summaryCommand writes the aggregate keys alone. No row set, and
			// no field holding an address, so the row operators and the address
			// operators are each refused by name.
			Shape: "doc",
		},
		{
			Name:      "request bgp rpki validate",
			ShortHelp: "Validate a prefix against the ROA cache",
			Args:      []string{"<prefix>", "<origin-asn>"},
		},
		{
			Name:      "show bgp rpki aspa",
			ShortHelp: "Show ASPA cache or lookup providers for a customer AS",
			// aspaCommand writes its rows under "entries" in both branches. A
			// row holds an AS number and the AS numbers that AS authorizes, and
			// neither is an address.
			Shape:   shapeTab,
			Columns: []string{"customer-asn", "providers"},
		},
	}
}

// pipeDecls names the pipe aliases this plugin puts on its own commands
// (pkg/plugin/rpc/types.go, PipeDecl).
//
// It has two readers and MUST stay one function, the same rule commandDecls
// carries. init() puts it on the registry.Registration, which anything linking
// the composition root reads without starting an engine, and the runner sends
// it in the Stage 1 registration message, which a running daemon reads.
//
// The alias is a selection over the sibling keys the bare command writes.
// `show bgp rpki summary` keeps answering, and the engine derives from the
// command list the empty declaration that stops this name reaching a command
// below the one it sits on.
func pipeDecls() []sdk.PipeDecl {
	return []sdk.PipeDecl{{
		Command:     commandShowRPKI,
		Name:        "summary",
		Description: "The validation counters, without the cache server rows",
		Expansion:   summaryAliasExpansion,
	}}
}

// runRPKIPlugin runs the bgp-rpki plugin using the SDK RPC protocol.
func runRPKIPlugin(conn net.Conn) int {
	logger().Debug("bgp-rpki plugin starting")

	p := sdk.NewWithConn("bgp-rpki", conn)
	defer func() { _ = p.Close() }()

	rp := &rPKIPlugin{
		plugin:        p,
		cache:         newROACache(),
		aspaCache:     newASPACache(),
		aspaTracker:   newASPATracker(),
		originTracker: newOriginTracker(),
		validateCh:    make(chan validationRequest, 4096),
		stopCh:        make(chan struct{}),
	}

	// Start async validation worker (long-lived goroutine per Ze rules).
	var workerWg sync.WaitGroup
	workerWg.Go(rp.validationWorker)
	defer func() {
		close(rp.stopCh)
		// The cache group runs on its own generation channel, which rp.stopCh does not
		// reach, so the group is stopped and waited for by name.
		rp.stopSessions()
		if rp.dataLease != nil {
			rp.dataLease.stop()
		}
		workerWg.Wait()
	}()

	// Structured event handler for DirectBridge delivery.
	// Receives UPDATE events as StructuredEvent with RawMessage — no JSON parsing.
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.PeerAddress == "" {
				continue
			}
			if se.EventType == rpc.EventKindState && se.State == rpc.SessionStateDown {
				rp.removePeer(se.PeerAddress)
			} else if se.EventType == rpc.EventKindUpdate {
				rp.handleStructuredUpdate(se)
			}
		}
		return nil
	})

	// Fallback: JSON event handler for non-DirectBridge delivery.
	p.OnEvent(func(jsonStr string) error {
		event, err := bgp.ParseEvent([]byte(jsonStr))
		if err != nil {
			logger().Warn("rpki: parse error", "error", err, "line", jsonStr[:min(100, len(jsonStr))])
			return nil
		}
		rp.handleEvent(event)
		return nil
	})

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return rp.handleCommand(command, args)
	})

	// Runtime verify deliveries contain changed roots only. Compose them over
	// the committed candidate; serialized SDK callbacks own these pointers.
	var current, pending, previous *rpkiConfig
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		pending = nil
		// Rollback belongs to this transaction, not the last successful apply.
		previous = nil
		cfg, err := parseRPKISections(sections, current)
		if err != nil {
			return err
		}
		pending = cfg
		return nil
	})
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseRPKISections(sections, nil)
		if err != nil {
			return err
		}
		current = cfg
		return nil
	})
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		if pending == nil {
			return errors.New("rpki: config apply requires a verified candidate")
		}
		next := pending
		pending = nil
		if err := rp.replaceConfig(current, next); err != nil {
			restoreErr := rp.replaceConfig(next, current)
			previous = nil
			return errors.Join(err, restoreErr)
		}
		previous, current = current, next
		return nil
	})
	p.OnConfigRollback(func(_ string) error {
		pending = nil
		if previous == nil {
			return nil
		}
		if err := rp.replaceConfig(current, previous); err != nil {
			return err
		}
		current, previous = previous, nil
		return nil
	})

	// Enable validation gate in adj-rib-in after the engine has finished
	// loading every plugin across every startup phase. Using OnAllPluginsReady
	// instead of OnStarted is critical here: bgp-rpki auto-loads in Phase 1
	// via ConfigRoots: ["bgp"], but bgp-adj-rib-in commonly lands in Phase 2
	// (explicit --plugin ze.bgp-adj-rib-in). Dispatching from OnStarted would
	// hit a dispatcher that has not yet registered adj-rib-in's commands.
	// OnAllPluginsReady fires via the event loop after the engine's
	// signalStartupComplete has frozen the dispatcher command registry, so
	// the cross-plugin dispatch is guaranteed to find the target command.
	p.OnAllPluginsReady(func() error {
		return rp.replaceConfig(nil, current)
	})

	p.SetStartupSubscriptions([]string{"update direction received", "state"}, nil, "full")

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Commands:    commandDecls(),
		Pipes:       pipeDecls(),
		WantsConfig: []string{configRootBGP, configRootPKI},
	})
	if err != nil {
		logger().Error("bgp-rpki plugin failed", "error", err)
		return 1
	}

	return 0
}

// setSessionsActive records how many cache servers are feeding this router. The group loads
// from one cache at a time, so the gauge is 1 or 0, and it answers the question an alert
// asks: is a cache feeding me? How many are CONFIGURED is `sessions-total` on
// `show bgp rpki summary`, which is a different question.
func setSessionsActive(count int) {
	m := rpkiMetricsPtr.Load()
	if m == nil {
		return
	}
	m.sessionsActive.Set(float64(count))
}

// stopSessions stops the cache group of the current config generation and waits for its
// goroutine to return. Safe to call when no generation is running, and the caller MUST call
// it before starting the next one and before the plugin exits.
func (rp *rPKIPlugin) stopSessions() {
	rp.mu.Lock()
	stopCh := rp.groupStopCh
	sessions := rp.sessions
	rp.groupStopCh = nil
	rp.sessions = nil
	rp.mu.Unlock()

	if stopCh == nil {
		return
	}

	close(stopCh)
	for _, session := range sessions {
		session.close()
	}
	rp.sessionWg.Wait()
}

// startSessions creates the RTR sessions of one config generation and starts the single
// goroutine that polls them in preference order.
//
// Sets active=true only when servers exist, so handleEvent/handleStructuredUpdate
// skip per-prefix work when unconfigured. active stays a statement about the CONFIG:
// whether any of those servers ever delivers data is RTRSession.synced (syncedSessions).
func (rp *rPKIPlugin) startSessions(cfg *rpkiConfig) {
	rp.stopSessions()
	if cfg == nil {
		cfg = &rpkiConfig{
			OriginNotFoundAction: ASPAPolicyAccept,
			ASPAUnknownAction:    ASPAPolicyAccept,
		}
	}
	active := len(cfg.CacheServers) != 0
	rp.active.Store(active)
	setSessionsActive(0)

	rp.aspaEnabled.Store(cfg.ASPAValidation)
	rp.aspaInvalidAction.Store(uint32(cfg.ASPAInvalidAction))
	rp.aspaUnknownAction.Store(uint32(cfg.ASPAUnknownAction))
	rp.originInvalidAction.Store(uint32(cfg.OriginInvalidAction))
	rp.originNotFoundAction.Store(uint32(cfg.OriginNotFoundAction))
	// Swap in the per-peer action map (may be nil when no peer/group overrides exist).
	// atomic.Pointer publishes an immutable map; buildDecisions reads it lock-free.
	peerActions := cfg.PeerActions
	rp.perPeerActions.Store(&peerActions)
	if !active {
		logger().Info("rpki: no cache servers configured")
		return
	}

	// RFC 8210 Section 10: "The client router attempts to establish a session with each
	// potential serving cache in preference order", where "Preference: An unsigned integer
	// denoting the router's preference to connect to that cache; the lower the value, the
	// more preferred." The sort is STABLE because the same section calls it "a non-unique
	// preference value", so servers sharing one keep the order the operator wrote.
	slices.SortStableFunc(cfg.CacheServers, func(a, b cacheServerConfig) int {
		return cmp.Compare(a.Preference, b.Preference)
	})

	rp.mu.Lock()
	if rp.dataLease == nil {
		rp.dataLease = newRTRDataLease(rp.cache, rp.aspaCache, rp.handleROAChange, rp.handleASPAChange)
	}
	rp.mu.Unlock()
	stopCh := make(chan struct{})
	sessions := make([]*RTRSession, 0, len(cfg.CacheServers))
	for _, cs := range cfg.CacheServers {
		session := newRTRSession(cs.Address, cs.Port, cs.Preference, cs.SourceAddress, rp.cache, rp.aspaCache, stopCh)
		session.tlsSettings = cs.TLS
		session.pkiConfig = cfg.pkiConfig
		session.dataLease = rp.dataLease
		session.onASPAChange = rp.handleASPAChange
		session.onROAChange = rp.handleROAChange
		sessions = append(sessions, session)
		logger().Info("rpki: configured RTR cache server",
			"address", cs.Address, "port", cs.Port, "preference", cs.Preference)
	}

	rp.mu.Lock()
	rp.sessions = sessions
	rp.groupStopCh = stopCh
	rp.mu.Unlock()

	// One goroutine for the whole list. Ze loads from one cache at a time, so a poller for
	// each server would put every server's records in the one VRP set, which is the union
	// the preference leaf exists to replace.
	rp.sessionWg.Go(newCacheGroup(sessions, stopCh).Run)
}

// aspaModeForPeer uses the role/import leaf resolved with the peer's actions.
func (rp *rPKIPlugin) aspaModeForPeer(address, name, group string) aspaMode {
	if configured := rp.perPeerActions.Load(); configured != nil {
		if peer, ok := configjson.LookupPeerConfig(*configured, address, name, group); ok {
			return peer.ASPAMode
		}
	}
	return aspaModeUnspecified
}

// handleStructuredUpdate processes a structured UPDATE event from DirectBridge.
// Extracts AS_PATH from AttrsWire and NLRIs from WireUpdate, then validates
// each prefix against the ROA cache. No JSON parsing needed.
func (rp *rPKIPlugin) handleStructuredUpdate(se *rpc.StructuredEvent) {
	if !rp.active.Load() {
		return
	}

	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.WireUpdate == nil {
		return
	}
	rp.validationMu.Lock()
	defer rp.validationMu.Unlock()
	if !rp.active.Load() {
		return
	}

	// RFC requirement: RFC6811-2-1 -- every route reaches the lookup and
	// retains its result, including a route whose origin is NONE.
	asp := rpkiASPathFromWire(msg.AttrsWire)
	originAS := rpkiOriginASFromASPath(asp, se.LocalAS)

	peerAddr := se.PeerAddress
	peerName := se.PeerName
	peerGroup := se.PeerGroup
	peerASN := se.PeerAS
	msgID := se.MessageID
	wu := msg.WireUpdate
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())

	v4, v6 := rp.cache.Count()
	cacheEmpty := v4+v6 == 0

	// RFC 7999 Section 3.3, the operator obligation. Read once per UPDATE, from
	// the same attribute bytes the AS_PATH came out of, and carried per prefix
	// so buildDecisions can apply the exemption without re-reading the wire. The
	// communities that count are the ones THIS session agreed to, which is why
	// the peer's address is an input.
	carriesBlackhole := rp.carriesAgreedBlackhole(peerAddr, peerGroup, msg.AttrsWire)

	// ASPA verification (once per UPDATE, not per-prefix).
	aspaState := aspaStateNone
	var normalizedPath []uint32
	if rp.aspaEnabled.Load() && se.PeerAS != se.LocalAS {
		aspaState = ASPAInvalid
		if asp != nil {
			aspaState, normalizedPath = aspaStateForPath(rp.aspaCache, asp.Segments,
				rp.aspaModeForPeer(peerAddr, peerName, peerGroup))
		}
	}

	// Remove withdrawn routes from the ASPA and origin trackers FIRST. Unconditional: the
	// origin tracker is populated whenever RPKI is active, so it must be pruned on
	// withdrawal even when ASPA is disabled (the ASPA tracker is empty then, so its
	// removal is a no-op).
	//
	// The pruning runs BEFORE the announce tracking below, and the order is load-bearing.
	// RFC 4271 Section 4.3 says an UPDATE naming one prefix in both WITHDRAWN ROUTES and
	// NLRI is treated as though WITHDRAWN did not name it, so that prefix stays installed
	// and must stay tracked (RFC4271-4.3-5, RFC4271-4.3-7). Pruning last dropped it from
	// the tracker, and a route missing from the tracker is a route ASPA re-validation
	// never revisits.
	rp.removeWithdrawnFromTracker(peerAddr, msgID, wu, ctx)

	results := make(map[string]map[string]uint8)
	eventASPA := aspaStateNone
	// Validate IPv4 unicast NLRIs.
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(family.Family{AFI: 1, SAFI: 1})
		results["ipv4/unicast"] = rp.validateNLRIs(peerAddr, peerName, peerGroup, peerASN, msgID, "ipv4/unicast",
			nlriData, addPath, false, originAS, cacheEmpty, aspaState, carriesBlackhole)
		eventASPA = aspaState
		// Track announced routes for ASPA re-validation (AC-5).
		if aspaState != aspaStateNone {
			rp.trackNLRIs(peerAddr, peerName, peerGroup, peerASN, msgID, "ipv4/unicast",
				nlriData, addPath, false, normalizedPath, aspaState)
		}
	}

	// Validate MP_REACH_NLRI announces.
	mpReach, err := wu.MPReach()
	if err == nil && mpReach != nil {
		fam := mpReach.Family()
		nlriBytes := mpReach.NLRIBytes()
		if len(nlriBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			// draft-ietf-sidrops-aspa-verification Section 6.2: only IPv4 and IPv6
			// unicast are verified; every other family carries no ASPA state
			// (aspaAppliesTo). The plain-NLRI branch above is IPv4 unicast by
			// construction, so only this MP_REACH branch needs the gate.
			mpASPAState, mpNormalizedPath := aspaState, normalizedPath
			if !aspaAppliesTo(fam) {
				mpASPAState, mpNormalizedPath = aspaStateNone, nil
			}
			prefixes := rp.validateNLRIs(peerAddr, peerName, peerGroup, peerASN, msgID, fam.String(),
				nlriBytes, addPath, fam.AFI == 2, originAS, cacheEmpty, mpASPAState, carriesBlackhole)
			if existing := results[fam.String()]; existing != nil {
				for prefix, state := range prefixes {
					existing[prefix] = state
				}
			} else {
				results[fam.String()] = prefixes
			}
			if mpASPAState != aspaStateNone {
				eventASPA = mpASPAState
			}
			if mpASPAState != aspaStateNone {
				rp.trackNLRIs(peerAddr, peerName, peerGroup, peerASN, msgID, fam.String(),
					nlriBytes, addPath, fam.AFI == 2, mpNormalizedPath, mpASPAState)
			}
		}
	}
	if len(results) > 0 {
		rp.emitRPKIEvent(peerAddr, peerName, peerASN, msgID, results, cacheEmpty, eventASPA)
	}
}

// validateNLRIs walks wire NLRI bytes and validates each prefix against the ROA cache.
func (rp *rPKIPlugin) validateNLRIs(peerAddr, peerName, peerGroup string, peerASN uint32, msgID uint64,
	family string, nlriData []byte, addPath, isIPv6 bool, originAS uint32, cacheEmpty bool, aspaState uint8,
	blackhole bool) map[string]uint8 {

	addrLen := 4
	if isIPv6 {
		addrLen = 16
	}

	familyResults := make(map[string]uint8)
	offset := 0
	for offset < len(nlriData) {
		var pathID uint32
		if addPath {
			if offset+4 >= len(nlriData) {
				break
			}
			pathID = uint32(nlriData[offset])<<24 | uint32(nlriData[offset+1])<<16 |
				uint32(nlriData[offset+2])<<8 | uint32(nlriData[offset+3])
			offset += 4
		}
		if offset >= len(nlriData) {
			break
		}
		prefixLen := int(nlriData[offset])
		byteCount := (prefixLen + 7) / 8
		offset++
		if offset+byteCount > len(nlriData) {
			break
		}
		var buf [16]byte // stack-allocated
		clear(buf[:])
		copy(buf[:], nlriData[offset:offset+byteCount])
		offset += byteCount

		addr, ok := netip.AddrFromSlice(buf[:addrLen])
		if !ok {
			continue
		}
		prefix := netip.PrefixFrom(addr, prefixLen).String()

		state := rp.cache.Validate(prefix, originAS)
		familyResults[prefix] = state

		select {
		case rp.validateCh <- validationRequest{
			peerAddr:   peerAddr,
			peerGroup:  peerGroup,
			family:     family,
			prefix:     prefix,
			pathID:     pathID,
			msgID:      msgID,
			state:      state,
			aspaState:  aspaState,
			originAS:   originAS,
			blackhole:  blackhole,
			generation: rp.validationGeneration.Load(),
		}:
		case <-rp.stopCh:
			return familyResults
		}

		// Track the route for RFC 6811 Section 4 origin re-validation when the ROA cache (VRP set)
		// changes. Independent of ASPA: origin validation runs whenever RPKI is active.
		rp.originTracker.Track(routeKey{peerAddr: peerAddr, family: family, prefix: prefix, pathID: pathID},
			originRoute{peerGroup: peerGroup, peerName: peerName, peerASN: peerASN,
				originAS: originAS, state: state, aspaState: aspaState,
				blackhole: blackhole, msgID: msgID, unavailable: cacheEmpty})
	}

	return familyResults
}

// handleEvent processes BGP events (UPDATE received).
// Validates each prefix against the ROA cache, enqueues accept/reject decisions
// to the async worker, and emits an rpki event with per-prefix validation states.
func (rp *rPKIPlugin) handleEvent(event *bgp.Event) {
	if event.GetEventType() == rpc.EventKindState && event.GetPeerState() == "down" {
		rp.removePeer(event.GetPeerAddress())
		return
	}
	if !rp.active.Load() {
		return
	}

	eventType := event.GetEventType()
	if eventType != rpc.EventKindUpdate {
		return
	}

	var peer bgp.PeerInfoJSON
	if err := json.Unmarshal(event.Peer, &peer); err != nil || peer.Remote.Address == "" {
		return
	}
	rp.validationMu.Lock()
	defer rp.validationMu.Unlock()
	if !rp.active.Load() {
		return
	}
	peerAddr, peerName, peerGroup := peer.Remote.Address, peer.Name, peer.Group

	// Use the same segment-aware origin rule on both delivery paths, including
	// the local speaker AS for an empty or confederation-ending AS_PATH.
	originAS := originASFromParsed(event.ASPath)
	var localAS uint32
	if peer.Local != nil {
		localAS = peer.Local.AS
		if len(event.ASPath) == 0 {
			originAS = localAS
		}
	}
	var attrs *attribute.AttributesWire
	var asp *attribute.ASPath
	raw := event.GetRawAttributesBytes()
	if len(raw) > 0 {
		attrs = attribute.NewAttributesWire(raw, bgpctx.APIContextID)
		asp = rpkiASPathFromWire(attrs)
		originAS = OriginNone
		if asp != nil {
			originAS = rpkiOriginASFromASPath(asp, localAS)
		}
	} else if event.RawAttributes != "" {
		// A malformed hex field must not fall back to the flattened path.
		originAS = OriginNone
	}

	// Check if ROA cache is empty (unavailable).
	v4, v6 := rp.cache.Count()
	cacheEmpty := v4+v6 == 0

	// RFC 7999 Section 3.3, the operator obligation. Read once per UPDATE.
	//
	// This is the JSON fallback path, which carries the attributes as raw bytes
	// rather than an indexed AttributesWire, so one is built here. When the event
	// carries no raw attributes at all the answer is false, and the exemption
	// stays closed: a route whose communities were never delivered must not be
	// treated as if it asked for a blackhole.
	blackhole := rp.carriesAgreedBlackhole(peerAddr, peerGroup, attrs)

	// Full events preserve the segment types, including AS_SET. Flattened JSON
	// AS paths are used only when the event carries no raw attributes.
	aspaState := aspaStateNone
	var normalizedPath []uint32
	if rp.aspaEnabled.Load() && (peer.Local == nil || peer.Remote.AS != peer.Local.AS) {
		mode := rp.aspaModeForPeer(peerAddr, peerName, peerGroup)
		if len(raw) > 0 || event.RawAttributes != "" {
			if asp != nil {
				aspaState, normalizedPath = aspaStateForPath(rp.aspaCache, asp.Segments, mode)
			} else {
				aspaState = ASPAInvalid
			}
		} else {
			normalizedPath = deduplicateASPath(event.ASPath)
			aspaState = verifyASPAPath(rp.aspaCache, normalizedPath, mode)
		}
	}

	// Validate each NLRI prefix against the ROA cache.
	// A single secondary must contain all families for the decorator's UPDATE.
	results := make(map[string]map[string]uint8)
	eventASPA := aspaStateNone
	for fam, ops := range event.FamilyOps {
		famName := fam.String()
		familyResults := make(map[string]uint8)
		familyASPA := aspaState
		if !aspaAppliesTo(fam) {
			familyASPA = aspaStateNone
		}

		// RFC 4271 Section 4.3: a simultaneous announcement wins over
		// withdrawal, regardless of the order of operations in JSON.
		for _, pass := range [...]routeaction.Action{routeaction.Del, routeaction.Add} {
			for _, op := range ops {
				if op.Action != pass {
					continue
				}

				for _, nlriVal := range op.NLRIs {
					prefix, pathID := bgp.ParseNLRIValue(nlriVal)
					if prefix == "" {
						continue
					}
					key := routeKey{peerAddr: peerAddr, family: famName, prefix: prefix, pathID: pathID}
					if op.Action == routeaction.Del {
						rp.aspaTracker.Remove(key, event.GetMsgID())
						rp.originTracker.Remove(key, event.GetMsgID())
						continue
					}

					state := rp.cache.Validate(prefix, originAS)
					familyResults[prefix] = state

					// Blocking enqueue to async worker (backpressure if worker falls behind).
					select {
					case rp.validateCh <- validationRequest{
						peerAddr:   peerAddr,
						peerGroup:  peerGroup,
						family:     famName,
						prefix:     prefix,
						pathID:     pathID,
						msgID:      event.GetMsgID(),
						state:      state,
						aspaState:  familyASPA,
						originAS:   originAS,
						blackhole:  blackhole,
						generation: rp.validationGeneration.Load(),
					}:
					case <-rp.stopCh:
						return
					}
					rp.originTracker.Track(key, originRoute{
						peerGroup: peerGroup, peerName: peerName, peerASN: peer.Remote.AS,
						originAS: originAS, state: state, aspaState: familyASPA,
						blackhole: blackhole, msgID: event.GetMsgID(), unavailable: cacheEmpty,
					})
					if familyASPA != aspaStateNone {
						rp.aspaTracker.Track(trackedRoute{
							key: key, peerName: peerName, peerGroup: peerGroup, peerASN: peer.Remote.AS,
							msgID: event.GetMsgID(), path: normalizedPath, aspaState: familyASPA,
							mode: rp.aspaModeForPeer(peerAddr, peerName, peerGroup),
						})
					}
				}
			}
		}

		if len(familyResults) > 0 {
			results[famName] = familyResults
			if familyASPA != aspaStateNone {
				eventASPA = familyASPA
			}
		}
	}
	if len(results) > 0 {
		rp.emitRPKIEvent(peerAddr, peerName, peer.Remote.AS, event.GetMsgID(), results, cacheEmpty, eventASPA)
	}
}

// emitRPKIEvent emits an rpki validation event via the SDK EmitEvent RPC.
// Called after validating all prefixes and families for a single UPDATE.
// aspaState is included when != aspaStateNone.
func (rp *rPKIPlugin) emitRPKIEvent(peerAddr, peerName string, peerASN uint32, msgID uint64, results map[string]map[string]uint8, cacheEmpty bool, aspaState uint8) {
	var event string
	if cacheEmpty {
		event = buildRPKIEventUnavailable(peerAddr, peerName, peerASN, msgID)
	} else {
		event = buildRPKIEvent(peerAddr, peerName, peerASN, msgID, results, aspaState)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := rp.plugin.EmitEvent(ctx, configRootBGP, "rpki", "received", peerAddr, event)
	if err != nil {
		logger().Warn("rpki: emit event failed", "error", err)
	}
}

const (
	maxBatchSize = 128
	batchWait    = 1 * time.Millisecond
)

// validationWorker is a long-lived goroutine that coalesces validation
// decisions from validateCh into batches and dispatches them to adj-rib-in
// in a single command. Bounded batch size (maxBatchSize) and bounded wait
// (batchWait) control latency. Shutdown drains remaining decisions.
func (rp *rPKIPlugin) validationWorker() {
	batch := make([]validationRequest, 0, maxBatchSize)
	timer := time.NewTimer(batchWait)
	timer.Stop()

	for {
		select {
		case <-rp.stopCh:
			rp.drainAndDispatch(batch[:0])
			return
		case req := <-rp.validateCh:
			if req.state == ValidationNotValidated {
				continue
			}
			batch = append(batch[:0], req)
		}

		timer.Reset(batchWait)
		timerFired := false
	fill:
		for len(batch) < maxBatchSize {
			select {
			case req := <-rp.validateCh:
				if req.state == ValidationNotValidated {
					continue
				}
				batch = append(batch, req)
			case <-timer.C:
				timerFired = true
				break fill
			case <-rp.stopCh:
				if !timer.Stop() {
					<-timer.C
				}
				rp.drainAndDispatch(batch)
				return
			}
		}
		if !timerFired && !timer.Stop() {
			<-timer.C
		}

		rp.dispatchBatch(batch)
		batch = batch[:0]
	}
}

// drainAndDispatch drains remaining validateCh items and dispatches them
// in maxBatchSize chunks so the lock hold time stays bounded.
func (rp *rPKIPlugin) drainAndDispatch(batch []validationRequest) {
	for {
		select {
		case req := <-rp.validateCh:
			if req.state != ValidationNotValidated {
				batch = append(batch, req)
			}
			if len(batch) >= maxBatchSize {
				rp.dispatchBatch(batch)
				batch = batch[:0]
			}
		default:
			rp.dispatchBatch(batch)
			return
		}
	}
}

// dispatchBatch sends a batch of validation decisions to adj-rib-in
// via the typed BatchValidate path (no string serialization for internal plugins).
func (rp *rPKIPlugin) dispatchBatch(batch []validationRequest) {
	rp.dispatchMu.Lock()
	defer rp.dispatchMu.Unlock()
	// Enqueue is serialized by validationMu, so generations are monotonic.
	// Discard only the obsolete prefix; current requests need no copying.
	generation := rp.validationGeneration.Load()
	for len(batch) != 0 && batch[0].generation != generation {
		batch = batch[1:]
	}
	if len(batch) == 0 {
		return
	}

	decisions := rp.buildDecisions(batch)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := rp.plugin.BatchValidate(ctx, decisions)
	if err != nil {
		logger().Warn("rpki: batch validation command failed",
			"count", len(batch), "error", err)
	}
}

// buildDecisions converts validation requests into typed decisions.
// Updates metrics counters and applies ASPA override logic.
func (rp *rPKIPlugin) buildDecisions(batch []validationRequest) []rpc.ValidationDecision {
	m := rpkiMetricsPtr.Load()

	decisions := make([]rpc.ValidationDecision, len(batch))
	// Global actions -- the fallback for any peer without an override.
	// RFC 6811 Section 2/3: excluding an Invalid (or NotFound) origin-validation route is an
	// operator policy choice, not an automatic side effect. Only action == ASPAPolicyReject
	// excludes it; LogOnly and Accept keep the route (still marked with its state) in the
	// Adj-RIB-In and the decision process.
	gInvalidAction := uint8(rp.aspaInvalidAction.Load())     //nolint:gosec // stored as uint8, fits
	gUnknownAction := uint8(rp.aspaUnknownAction.Load())     //nolint:gosec // stored as uint8, fits
	gOriginInvalid := uint8(rp.originInvalidAction.Load())   //nolint:gosec // stored as uint8, fits
	gOriginNotFound := uint8(rp.originNotFoundAction.Load()) //nolint:gosec // stored as uint8, fits
	// Per-peer overrides (keyed by remote IP, or by a dynamic group's name for the
	// template its members inherit). nil map => every route uses the global actions.
	var perPeer map[configjson.PeerConfigKey]peerActionSet
	if p := rp.perPeerActions.Load(); p != nil {
		perPeer = *p
	}

	for i := range batch {
		req := &batch[i]
		if m != nil {
			m.validationOutcomes.With(validationStateString(req.state)).Inc()
			// ASPA outcome, only when ASPA verification was active for the route.
			if req.aspaState != aspaStateNone {
				m.aspaOutcomes.With(aspaStateString(req.aspaState)).Inc()
			}
		}

		// Resolve the effective actions for this route's source peer: a per-peer override
		// (already merged peer > group > global at config time) wins, else the group the
		// session was created from, else the global values.
		originInvalidAction, originNotFoundAction := gOriginInvalid, gOriginNotFound
		invalidAction, unknownAction := gInvalidAction, gUnknownAction
		var blackholeExempt bool
		if perPeer != nil {
			if set, ok := configjson.LookupPeerConfig(perPeer, req.peerAddr, "", req.peerGroup); ok {
				originInvalidAction = set.OriginInvalid.Action
				originNotFoundAction = set.OriginNotFound.Action
				invalidAction = set.ASPAInvalid.Action
				unknownAction = set.ASPAUnknown.Action
				blackholeExempt = set.BlackholeExempt
			}
		}

		var reject bool
		switch req.state {
		case ValidationInvalid:
			reject = originInvalidAction == ASPAPolicyReject
			// RFC 7999 Section 3.3 (RFC7999-3.3-4): "An operator MUST ensure
			// that origin validation techniques (such as the one described in
			// [RFC6811]) do not inadvertently block legitimate announcements
			// carrying the BLACKHOLE community."
			//
			// Three conditions, and all three hold or the route is rejected as
			// any Invalid route is. The operator asked for the exemption on
			// this session, the announcement carries BLACKHOLE, and the only
			// origin-validation fault is that the prefix is longer than a
			// covering VRP whose ASN it matches. A wrong origin AS is not a
			// length problem and never reaches this branch's exemption.
			if reject && blackholeExempt && req.blackhole &&
				rp.cache.invalidByLengthOnly(req.prefix, req.originAS) {
				reject = false
				logger().Info("rpki: BLACKHOLE announcement kept despite an Invalid origin state",
					"prefix", req.prefix, "peer", req.peerAddr, "origin-as", req.originAS,
					"reason", "invalid by prefix length only, RFC 7999 Section 3.3")
			}
			if originInvalidAction == ASPAPolicyLogOnly {
				logger().Warn("rpki: Invalid origin retained under log-only policy",
					"prefix", req.prefix, "peer", req.peerAddr)
			}
		case ValidationNotFound:
			reject = originNotFoundAction == ASPAPolicyReject
			if originNotFoundAction == ASPAPolicyLogOnly {
				logger().Warn("rpki: NotFound origin retained under log-only policy",
					"prefix", req.prefix, "peer", req.peerAddr)
			}
		}
		if !reject && req.aspaState != aspaStateNone {
			reject = aspaOverridesAccept(req.aspaState, invalidAction, unknownAction)
		}
		if (req.aspaState == ASPAInvalid && invalidAction == ASPAPolicyLogOnly) ||
			(req.aspaState == ASPAUnknown && unknownAction == ASPAPolicyLogOnly) {
			logger().Warn("rpki: ASPA log-only policy", "prefix", req.prefix,
				"peer", req.peerAddr, "aspa-state", aspaStateString(req.aspaState))
		}

		// Retain received bytes for cache/config revalidation and rollback.
		// ASPA Section 5.7 requires this for Invalid paths; origin policy
		// rejection likewise changes eligibility, not ownership of the UPDATE.
		decisions[i] = rpc.ValidationDecision{
			Accept:     !reject,
			Ineligible: reject,
			MsgID:      req.msgID,
			PeerAddr:   req.peerAddr,
			Family:     req.family,
			Prefix:     req.prefix,
			PathID:     req.pathID,
			ValState:   req.state,
		}
	}
	return decisions
}

// originASFromParsed extracts origin AS from a pre-parsed AS_PATH ([]uint32).
// Returns the last ASN in the slice, or OriginNone if empty.
func originASFromParsed(asPath []uint32) uint32 {
	if len(asPath) == 0 {
		return OriginNone
	}
	return asPath[len(asPath)-1]
}

// trackNLRIs walks wire NLRI bytes and tracks each route in the ASPA tracker.
// Called after ASPA verification to enable re-validation when cache data changes (AC-5).
func (rp *rPKIPlugin) trackNLRIs(peerAddr, peerName, peerGroup string, peerASN uint32, msgID uint64,
	fam string, nlriData []byte, addPath, isIPv6 bool, normalizedPath []uint32, aspaState uint8) {

	addrLen := 4
	if isIPv6 {
		addrLen = 16
	}

	offset := 0
	for offset < len(nlriData) {
		var pathID uint32
		if addPath {
			if offset+4 >= len(nlriData) {
				break
			}
			pathID = uint32(nlriData[offset])<<24 | uint32(nlriData[offset+1])<<16 |
				uint32(nlriData[offset+2])<<8 | uint32(nlriData[offset+3])
			offset += 4
		}
		if offset >= len(nlriData) {
			break
		}
		prefixLen := int(nlriData[offset])
		byteCount := (prefixLen + 7) / 8
		offset++
		if offset+byteCount > len(nlriData) {
			break
		}
		var buf [16]byte
		clear(buf[:])
		copy(buf[:], nlriData[offset:offset+byteCount])
		offset += byteCount

		addr, ok := netip.AddrFromSlice(buf[:addrLen])
		if !ok {
			continue
		}
		prefix := netip.PrefixFrom(addr, prefixLen).String()

		rp.aspaTracker.Track(trackedRoute{
			key:       routeKey{peerAddr: peerAddr, family: fam, prefix: prefix, pathID: pathID},
			peerName:  peerName,
			peerGroup: peerGroup,
			peerASN:   peerASN,
			msgID:     msgID,
			path:      normalizedPath,
			aspaState: aspaState,
			mode:      rp.aspaModeForPeer(peerAddr, peerName, peerGroup),
		})
	}
}

// removeWithdrawnFromTracker removes withdrawn routes from the ASPA tracker.
func (rp *rPKIPlugin) removeWithdrawnFromTracker(peerAddr string, msgID uint64, wu *wireu.WireUpdate, ctx *bgpctx.EncodingContext) {
	// IPv4 withdrawn routes.
	wdData, err := wu.Withdrawn()
	if err == nil && len(wdData) > 0 {
		addPath := ctx != nil && ctx.AddPath(family.Family{AFI: 1, SAFI: 1})
		rp.removeTrackedNLRIs(peerAddr, msgID, "ipv4/unicast", wdData, addPath, false)
	}

	// MP_UNREACH_NLRI withdrawn routes.
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		wdBytes := mpUnreach.WithdrawnBytes()
		if len(wdBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			rp.removeTrackedNLRIs(peerAddr, msgID, fam.String(), wdBytes, addPath, fam.AFI == 2)
		}
	}
}

// removeTrackedNLRIs walks wire NLRI bytes and removes each from the ASPA tracker.
func (rp *rPKIPlugin) removeTrackedNLRIs(peerAddr string, msgID uint64, fam string, nlriData []byte, addPath, isIPv6 bool) {
	addrLen := 4
	if isIPv6 {
		addrLen = 16
	}

	offset := 0
	for offset < len(nlriData) {
		var pathID uint32
		if addPath {
			if offset+4 >= len(nlriData) {
				break
			}
			pathID = uint32(nlriData[offset])<<24 | uint32(nlriData[offset+1])<<16 |
				uint32(nlriData[offset+2])<<8 | uint32(nlriData[offset+3])
			offset += 4
		}
		if offset >= len(nlriData) {
			break
		}
		prefixLen := int(nlriData[offset])
		byteCount := (prefixLen + 7) / 8
		offset++
		if offset+byteCount > len(nlriData) {
			break
		}
		var buf [16]byte
		clear(buf[:])
		copy(buf[:], nlriData[offset:offset+byteCount])
		offset += byteCount

		addr, ok := netip.AddrFromSlice(buf[:addrLen])
		if !ok {
			continue
		}
		prefix := netip.PrefixFrom(addr, prefixLen).String()

		key := routeKey{peerAddr: peerAddr, family: fam, prefix: prefix, pathID: pathID}
		rp.aspaTracker.Remove(key, msgID)
		rp.originTracker.Remove(key, msgID)
	}
}

// handleROAChange is called by RTR sessions when the ROA cache (VRP set) changes at End of Data.
// RFC 6811 Section 4: it re-validates tracked origins against the updated cache.
// Changed origin states and Invalid BLACKHOLE routes reach buildDecisions again:
// the latter can gain or lose their length-only exemption without changing state.
func (rp *rPKIPlugin) handleROAChange() {
	rp.validationMu.Lock()
	defer rp.validationMu.Unlock()
	changed := rp.originTracker.revalidate(rp.cache)
	if len(changed) == 0 {
		return
	}
	affected := make(map[rpkiUpdateKey]struct{})
	for _, c := range changed {
		affected[rpkiUpdateKey{peerAddr: c.key.peerAddr, msgID: c.msgID}] = struct{}{}
		if !c.decisionRequired {
			continue
		}
		select {
		case rp.validateCh <- validationRequest{
			peerAddr:   c.key.peerAddr,
			peerGroup:  c.peerGroup,
			family:     c.key.family,
			prefix:     c.key.prefix,
			pathID:     c.key.pathID,
			msgID:      c.msgID,
			state:      c.state,
			aspaState:  c.aspaState,
			originAS:   c.originAS,
			blackhole:  c.blackhole,
			generation: rp.validationGeneration.Load(),
		}:
		case <-rp.stopCh:
			return
		}
	}
	rp.emitRPKIUpdates(rp.originTracker.updateResults(affected))
	logger().Info("rpki: re-validated routes after VRP change", "changed", len(changed))
}

// handleASPAChange re-evaluates affected paths and applies both current verdicts.
// Recovery is a decision too: a repaired ASPA can restore a retained route without
// a fresh UPDATE, but cannot override an origin-validation rejection.
func (rp *rPKIPlugin) handleASPAChange(changedCustomers []uint32) {
	if !rp.aspaEnabled.Load() {
		return
	}
	rp.validationMu.Lock()
	defer rp.validationMu.Unlock()
	changed := rp.aspaTracker.revalidate(rp.aspaCache, changedCustomers)
	affected := make(map[rpkiUpdateKey]struct{})
	for _, rt := range changed {
		current, ok := rp.originTracker.updateASPA(rp.cache, rt.key, rt.msgID, rt.aspaState)
		if !ok {
			continue
		}
		affected[rpkiUpdateKey{peerAddr: current.key.peerAddr, msgID: current.msgID}] = struct{}{}
		select {
		case rp.validateCh <- validationRequest{
			peerAddr: current.key.peerAddr, peerGroup: current.peerGroup,
			family: current.key.family, prefix: current.key.prefix, pathID: current.key.pathID,
			state: current.state, aspaState: current.aspaState, originAS: current.originAS,
			blackhole: current.blackhole, msgID: current.msgID,
			generation: rp.validationGeneration.Load(),
		}:
		case <-rp.stopCh:
			return
		}
	}
	if len(affected) > 0 {
		rp.emitRPKIUpdates(rp.originTracker.updateResults(affected))
	}
}

// removePeer prunes received-route metadata when the corresponding Adj-RIB-In
// is cleared, so later cache changes cannot issue decisions for an old session.
func (rp *rPKIPlugin) removePeer(peerAddr string) {
	rp.validationMu.Lock()
	defer rp.validationMu.Unlock()
	rp.aspaTracker.removePeer(peerAddr)
	rp.originTracker.removePeer(peerAddr)
}

// rpkiASPathFromWire extracts the full *attribute.ASPath from AttrsWire.
func rpkiASPathFromWire(attrs *attribute.AttributesWire) *attribute.ASPath {
	if attrs == nil {
		return nil
	}
	attr, err := attrs.Get(attribute.AttrASPath)
	if err != nil || attr == nil {
		return nil
	}
	asp, ok := attr.(*attribute.ASPath)
	if !ok {
		return nil
	}
	return asp
}

// rpkiOriginASFromASPath derives the Route Origin ASN as RFC 6811 Section 2
// defines it. The final segment type is significant. An AS_SET is NONE, and an
// empty or confederation path uses the local speaker AS.
func rpkiOriginASFromASPath(asp *attribute.ASPath, localAS uint32) uint32 {
	if asp == nil || len(asp.Segments) == 0 {
		return localAS
	}
	last := &asp.Segments[len(asp.Segments)-1]
	if len(last.ASNs) == 0 {
		return OriginNone
	}
	switch last.Type {
	case attribute.ASSequence:
		return last.ASNs[len(last.ASNs)-1]
	case attribute.ASConfedSequence, attribute.ASConfedSet:
		return localAS
	default:
		return OriginNone
	}
}

// handleCommand processes RPKI CLI commands.
func (rp *rPKIPlugin) handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case commandShowRPKI:
		return rp.overviewCommand()
	case "show bgp rpki status":
		return rp.statusCommand()
	case "show bgp rpki cache":
		return rp.cacheCommand()
	case "show bgp rpki roa":
		return rp.roaCommand(args)
	case "show bgp rpki summary":
		return rp.summaryCommand()
	case "request bgp rpki validate":
		return rp.validateCommand(args)
	case "show bgp rpki aspa":
		return rp.aspaCommand(args)
	}
	return statusError, "", fmt.Errorf("unknown command: %s", command)
}

// snapshots returns a point-in-time copy of every configured cache server's diagnostic fields.
func (rp *rPKIPlugin) snapshots() []SessionSnapshot {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	snaps := make([]SessionSnapshot, len(rp.sessions))
	for i, sess := range rp.sessions {
		snaps[i] = sess.Snapshot()
	}
	return snaps
}

// validationEnabled reports usable data, including a retained set while a new
// transport/config generation has not synchronized yet. An authenticated empty
// set is data too; record counts cannot distinguish it from an expired cache.
func (rp *rPKIPlugin) validationEnabled() bool {
	if !rp.active.Load() {
		return false
	}
	rp.mu.RLock()
	lease := rp.dataLease
	rp.mu.RUnlock()
	if lease == nil {
		return false
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	return !lease.stopped && !lease.deadline.IsZero() && time.Now().Before(lease.deadline)
}

func syncedSessions(snaps []SessionSnapshot) int {
	synced := 0
	for _, snap := range snaps {
		if snap.Synced {
			synced++
		}
	}
	return synced
}

func (rp *rPKIPlugin) statusCommand() (string, any, error) {
	snaps := rp.snapshots()

	v4, v6 := rp.cache.Count()
	aspaCount := rp.aspaCache.count()
	aspaEnabled := rp.aspaEnabled.Load()
	synced := syncedSessions(snaps)

	b := textbuf.Get()
	defer b.Release()
	// "running" is read from active, the same flag the per-prefix work and the adj-rib-in
	// validation gate are held behind, so it answers what it says: the RPKI machinery is
	// in play because the config names a cache server. It was written as the literal true
	// until 2026-09-20, which answered "the plugin is loaded", a thing the operator knows
	// because they typed the command.
	b.Str(`{"running":`).Bool(rp.active.Load())
	b.Str(`,"vrp-count-ipv4":`).Int(int64(v4))
	b.Str(`,"vrp-count-ipv6":`).Int(int64(v6))
	b.Str(`,"sessions":`).Int(int64(len(snaps)))
	b.Str(`,"sessions-synced":`).Int(int64(synced))
	// Configured sessions can all be unsynced while a previous generation's
	// data remains usable until its original expiration deadline.
	b.Str(`,"synced":`).Bool(synced > 0)
	b.Str(`,"aspa-enabled":`).Bool(aspaEnabled)
	b.Str(`,"aspa-records":`).Int(int64(aspaCount))

	if len(snaps) > 0 {
		b.Byte(',')
		appendCacheServers(b, snaps)
	}

	// Effective global actions and the per-peer resolved overrides, read from the same atomic
	// sources buildDecisions enforces, so the display reflects the enforced policy. These are
	// independent atomic loads (not one snapshot), so a config reload landing mid-serialization
	// could briefly mix generations in the OUTPUT; each value stays individually valid and
	// enforcement is unaffected (display-only).
	rp.appendGlobalActions(b)
	rp.appendPeerActions(b)

	b.Byte('}')
	return statusDone, json.RawMessage(b.String()), nil
}

func (rp *rPKIPlugin) cacheCommand() (string, any, error) {
	snaps := rp.snapshots()

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"cache-servers":[`)
	for i, snap := range snaps {
		if i > 0 {
			b.Byte(',')
		}
		b.Str(`{"address":"`).Str(snap.Address).Byte('"')
		b.Str(`,"port":`).Uint16(snap.Port)
		b.Str(`,"preference":`).Uint8(snap.Preference)
		b.Str(`,"state":"`).Str(snap.State).Byte('"')
		b.Str(`,"synced":`).Bool(snap.Synced)
		b.Str(`,"version":`).Uint8(snap.Version)
		b.Str(`,"session-id":`).Uint(uint64(snap.SessionID))
		b.Str(`,"serial":`).Uint32(snap.Serial)
		b.Str(`,"refresh-interval":`).Int(int64(snap.RefreshInterval.Seconds()))
		b.Str(`,"retry-interval":`).Int(int64(snap.RetryInterval.Seconds()))
		b.Str(`,"expire-interval":`).Int(int64(snap.ExpireInterval.Seconds()))
		b.Byte('}')
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

const roaDiagLimit = 1000

func (rp *rPKIPlugin) roaCommand(args []string) (string, any, error) {
	// "show bgp rpki roa <prefix>" looks up covering VRPs for prefix.
	if len(args) > 0 && args[0] != "" {
		_, _, err := net.ParseCIDR(args[0])
		if err != nil {
			return statusError, "", fmt.Errorf("invalid prefix: %s", args[0])
		}
		return rp.roaLookupCommand(args[0])
	}

	v4, v6 := rp.cache.Count()
	total := v4 + v6
	entries := rp.cache.Entries(roaDiagLimit)

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"total-vrps":`).Int(int64(total))
	b.Str(`,"ipv4-vrps":`).Int(int64(v4))
	b.Str(`,"ipv6-vrps":`).Int(int64(v6))

	if total > roaDiagLimit {
		b.Str(`,"truncated":true,"limit":`).Int(int64(roaDiagLimit))
	}

	b.Str(`,"entries":[`)
	for i, e := range entries {
		if i > 0 {
			b.Byte(',')
		}
		b.Str(`{"prefix":"`).Str(e.Prefix).Byte('"')
		b.Str(`,"max-length":`).Uint8(e.MaxLength)
		// The ROA's AS number is a row an operator reads, so it carries the
		// notation bgp/as-notation selected (asn.AppendJSON states the rule).
		b.Str(`,"asn":`).Str(asn.JSONValue(e.ASN))
		b.Byte('}')
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

func (rp *rPKIPlugin) roaLookupCommand(prefix string) (string, any, error) {
	_, ipnet, _ := net.ParseCIDR(prefix) // already validated by caller
	canonical := ipnet.String()
	entries := rp.cache.Lookup(canonical)

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"prefix":"`).Str(canonical).Byte('"')
	// A zero "covering-vrps" from a cache that never synced says nothing about the ROA.
	b.Str(`,"synced":`).Bool(syncedSessions(rp.snapshots()) > 0)
	b.Str(`,"covering-vrps":`).Int(int64(len(entries)))
	b.Str(`,"entries":[`)
	for i, e := range entries {
		if i > 0 {
			b.Byte(',')
		}
		b.Str(`{"prefix":"`).Str(e.Prefix).Byte('"')
		b.Str(`,"max-length":`).Uint8(e.MaxLength)
		b.Str(`,"asn":`).Str(asn.JSONValue(e.ASN))
		b.Byte('}')
	}
	// "covered" states whether the VRP set holds anything for this prefix, which is what
	// Lookup just answered. Deriving it from Validate would read a state that also carries
	// "ze could not parse this prefix" (validate.go), a different question.
	b.Str(`],"covered":`).Bool(len(entries) > 0)
	b.Byte('}')
	return statusDone, json.RawMessage(b.String()), nil
}

// summaryFieldNames are the keys of the RPKI aggregate half, in the order both
// answers write them. It is the one authored list: appendSummaryFields writes
// these keys, and summaryAliasExpansion selects them.
//
// Four of the seven are computed. vrp-count is the sum of the two family
// counts, sessions-established counts the sessions in one state,
// sessions-total spells what `show bgp rpki status` calls sessions, and
// validation-enabled reads whether any cache server has delivered a set to
// validate against. A pipe operator can do none of those, which is why the
// command computes them and the alias only selects them.
var summaryFieldNames = []string{
	"vrp-count",
	"validation-enabled",
	"sessions-total",
	"sessions-established",
	"sessions-synced",
	"aspa-enabled",
	"aspa-records",
}

// summaryAliasExpansion is the operator chain `show bgp rpki | summary` stands
// for. It is built from summaryFieldNames rather than repeating them, so a
// counter added to the aggregate half reaches the alias with it.
var summaryAliasExpansion = buildSummaryAliasExpansion()

// buildSummaryAliasExpansion writes the expansion once, at package
// initialization. Selection is the whole of what a pipe alias may do, so the
// chain is the display operator and the aggregate field names.
func buildSummaryAliasExpansion() string {
	var tb textbuf.Buffer
	return tb.Str("display ").Join(summaryFieldNames, " ").String()
}

// appendSummaryFields writes the aggregate half of the RPKI answer: how much
// data the cache holds, how many sessions carry it, and whether ASPA is on.
//
// It writes no leading or trailing brace and no leading comma, so a caller
// places it. Both `show bgp rpki summary` and the bare `show bgp rpki` write it,
// which is what lets `show bgp rpki | summary` reproduce the subcommand instead
// of approximating it.
func (rp *rPKIPlugin) appendSummaryFields(b *textbuf.Buffer, snaps []SessionSnapshot) {
	v4, v6 := rp.cache.Count()

	established := 0
	for _, snap := range snaps {
		if snap.State == sessionEstablish {
			established++
		}
	}

	b.Str(`"vrp-count":`).Int(int64(v4 + v6))
	b.Str(`,"validation-enabled":`).Bool(rp.validationEnabled())
	b.Str(`,"sessions-total":`).Int(int64(len(snaps)))
	b.Str(`,"sessions-established":`).Int(int64(established))
	b.Str(`,"sessions-synced":`).Int(int64(syncedSessions(snaps)))
	b.Str(`,"aspa-enabled":`).Bool(rp.aspaEnabled.Load())
	b.Str(`,"aspa-records":`).Int(int64(rp.aspaCache.count()))
}

// appendCacheServers writes one row for each configured cache server, carrying
// what an operator reads at a glance: where the session points, what state it
// is in, and whether it has delivered data. The protocol detail of a session,
// the serial and the three intervals, is what `show bgp rpki cache` adds.
//
// It writes the key and its array and nothing around them, so a caller places
// the comma that separates it from the fields before it.
func appendCacheServers(b *textbuf.Buffer, snaps []SessionSnapshot) {
	b.Str(`"cache-servers":[`)
	for i, snap := range snaps {
		if i > 0 {
			b.Byte(',')
		}
		b.Str(`{"address":"`).Str(snap.Address).Byte('"')
		b.Str(`,"port":`).Uint16(snap.Port)
		b.Str(`,"state":"`).Str(snap.State).Byte('"')
		b.Str(`,"synced":`).Bool(snap.Synced)
		b.Str(`,"version":`).Uint8(snap.Version)
		b.Byte('}')
	}
	b.Byte(']')
}

// overviewCommand answers the bare `show bgp rpki`: the aggregate counters and
// one row for each cache server, as siblings at one level.
//
// The shape is what `show bgp` uses, and it is what a pipe alias needs. An
// alias selects among sibling keys and computes nothing, so `| summary` answers
// the aggregate half only because the aggregate half is written here.
func (rp *rPKIPlugin) overviewCommand() (string, any, error) {
	snaps := rp.snapshots()

	b := textbuf.Get()
	defer b.Release()
	b.Byte('{')
	rp.appendSummaryFields(b, snaps)
	b.Byte(',')
	appendCacheServers(b, snaps)
	b.Byte('}')
	return statusDone, json.RawMessage(b.String()), nil
}

func (rp *rPKIPlugin) summaryCommand() (string, any, error) {
	b := textbuf.Get()
	defer b.Release()
	b.Byte('{')
	rp.appendSummaryFields(b, rp.snapshots())
	b.Byte('}')
	return statusDone, json.RawMessage(b.String()), nil
}

func (rp *rPKIPlugin) validateCommand(args []string) (string, any, error) {
	if len(args) < 2 {
		return statusError, "", fmt.Errorf("usage: rpki validate <prefix> <origin-asn>")
	}

	_, ipnet, err := net.ParseCIDR(args[0])
	if err != nil {
		return statusError, "", fmt.Errorf("invalid prefix: %s", args[0])
	}
	prefix := ipnet.String()

	// asn.Parse reads all three RFC 5396 spellings, so an operator validates
	// the origin they read on a show output.
	originAS, err := asn.Parse(args[1])
	if err != nil {
		return statusError, "", fmt.Errorf("invalid ASN: %s", args[1])
	}

	state := rp.cache.Validate(prefix, originAS)
	covering := rp.cache.Lookup(prefix)

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"prefix":"`).Str(prefix).Byte('"')
	b.Str(`,"origin-asn":`).Str(asn.JSONValue(originAS))
	b.Str(`,"state":"`).Str(validationStateString(state)).Byte('"')
	// "not-found" from a cache that never synced is not a verdict about the prefix.
	b.Str(`,"synced":`).Bool(syncedSessions(rp.snapshots()) > 0)
	b.Str(`,"covering-vrps":[`)
	for i, e := range covering {
		if i > 0 {
			b.Byte(',')
		}
		b.Str(`{"prefix":"`).Str(e.Prefix).Byte('"')
		b.Str(`,"max-length":`).Uint8(e.MaxLength)
		b.Str(`,"asn":`).Str(asn.JSONValue(e.ASN))
		b.Byte('}')
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

const aspaDiagLimit = 1000

func (rp *rPKIPlugin) aspaCommand(args []string) (string, any, error) {
	// "show bgp rpki aspa <customer-asn>" looks up a specific customer.
	if len(args) > 0 && args[0] != "" {
		customer, err := asn.Parse(args[0])
		if err != nil {
			return statusError, "", fmt.Errorf("invalid ASN: %s", args[0])
		}
		providers := rp.aspaCache.lookupCustomer(customer)
		b := textbuf.Get()
		defer b.Release()

		// The one result goes under "entries", in the row shape the no-argument
		// branch writes below, so the command answers one shape whatever its
		// argument and a single answer-shape declaration can describe it. "found"
		// stays beside it: it separates "this customer has no ASPA record" from
		// "the cache is empty", which the row count alone cannot say.
		b.Str(`{"customer-asn":`).Str(asn.JSONValue(customer))
		if providers == nil {
			b.Str(`,"found":false,"entries":[]}`)
			return statusDone, json.RawMessage(b.String()), nil
		}

		b.Str(`,"found":true,"entries":[{"customer-asn":`).Str(asn.JSONValue(customer))
		b.Str(`,"providers":[`)
		for i, p := range providers {
			if i > 0 {
				b.Byte(',')
			}
			b.Str(asn.JSONValue(p))
		}
		b.Str(`]}]}`)
		return statusDone, json.RawMessage(b.String()), nil
	}

	// No args: dump ASPA cache summary
	total := rp.aspaCache.count()
	entries := rp.aspaCache.Entries(aspaDiagLimit)

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"total-records":`).Int(int64(total))
	b.Str(`,"enabled":`).Bool(rp.aspaEnabled.Load())

	if total > aspaDiagLimit {
		b.Str(`,"truncated":true,"limit":`).Int(int64(aspaDiagLimit))
	}

	b.Str(`,"entries":[`)
	for i, e := range entries {
		if i > 0 {
			b.Byte(',')
		}
		b.Str(`{"customer-asn":`).Str(asn.JSONValue(e.CustomerAS))
		b.Str(`,"providers":[`)
		for j, p := range e.Providers {
			if j > 0 {
				b.Byte(',')
			}
			b.Str(asn.JSONValue(p))
		}
		b.Str(`]}`)
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}
