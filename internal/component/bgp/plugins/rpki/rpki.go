// Design: docs/architecture/plugin/rib-storage-design.md — RPKI origin validation plugin
// Detail: rpki_config.go — config parsing from OnConfigure JSON
// Detail: rtr_pdu.go — RTR PDU wire format types and parsing
// Detail: rtr_session.go — RTR session lifecycle management
// Detail: roa_cache.go — ROA cache VRP storage and covering-prefix lookup
// Detail: validate.go — RFC 6811 origin validation algorithm
// Detail: emit.go — RPKI validation event JSON building
package rpki

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

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
	peerGroup string
	family    string
	prefix    string
	pathID    uint32
	state     uint8 // origin validation state
	aspaState uint8 // ASPA path verification state
	originAS  uint32
	blackhole bool // the announcement carried the RFC 7999 BLACKHOLE community
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
	// Read lock-free from the single validationWorker goroutine; written from OnConfigure.
	perPeerActions atomic.Pointer[map[configjson.PeerConfigKey]peerActionSet]
	mu             sync.RWMutex

	// sessions holds active RTR sessions to cache servers.
	sessions []*RTRSession

	// sessionWg tracks RTR session goroutines for clean shutdown.
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
		rp.sessionWg.Wait()
		workerWg.Wait()
	}()

	// Structured event handler for DirectBridge delivery.
	// Receives UPDATE events as StructuredEvent with RawMessage — no JSON parsing.
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.EventType != rpc.EventKindUpdate || se.PeerAddress == "" {
				continue
			}
			rp.handleStructuredUpdate(se)
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

	// OnConfigure: parse RPKI config and start RTR sessions to cache servers.
	// Called during Stage 2 of the 5-stage plugin startup protocol.
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			cfg, err := parseRPKIConfig(section.Data)
			if err != nil {
				logger().Error("rpki: config parse failed", "error", err)
				return err
			}
			rp.startSessions(cfg)
		}
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
		if !rp.active.Load() {
			logger().Info("rpki: no cache servers configured, skipping validation gate")
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		status, _, err := p.DispatchCommandArgs(ctx, "request bgp adj-rib-in enable-validation", nil, "")
		if err != nil {
			logger().Error("rpki: failed to enable validation gate", "error", err)
			return fmt.Errorf("enable validation gate: %w", err)
		}
		logger().Info("rpki: validation gate enabled", "status", status)
		return nil
	})

	p.SetStartupSubscriptions([]string{"update direction received"}, nil, "full")

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "show bgp rpki", Description: "Show RPKI validation counters with one row for each cache server"},
			{Name: "show bgp rpki status", Description: "Show RPKI validation status and cache server overview"},
			{Name: "show bgp rpki cache", Description: "Show RTR cache server sessions with protocol details"},
			{Name: "show bgp rpki roa", Description: "Show ROA table entries or lookup covering VRPs for a prefix"},
			{Name: "show bgp rpki summary", Description: "Show RPKI validation summary with session and ASPA counts"},
			{Name: "request bgp rpki validate", Description: "Validate a prefix against the ROA cache", Args: []string{"<prefix>", "<origin-asn>"}},
			{Name: "show bgp rpki aspa", Description: "Show ASPA cache or lookup providers for a customer AS"},
		},
		// The alias is a selection over the sibling keys the bare command
		// writes. `show bgp rpki summary` keeps answering, and the engine
		// derives from the command list above the empty declaration that stops
		// this name reaching a command below the one it sits on.
		Pipes: []sdk.PipeDecl{{
			Command:     "show bgp rpki",
			Name:        "summary",
			Description: "The validation counters, without the cache server rows",
			Expansion:   summaryAliasExpansion,
		}},
		WantsConfig: []string{"bgp"},
	})
	if err != nil {
		logger().Error("bgp-rpki plugin failed", "error", err)
		return 1
	}

	return 0
}

// startSessions creates and starts RTR sessions from parsed config.
// Each cache server gets a long-lived goroutine running RTRSession.Run().
// Sets active=true only when servers exist, so handleEvent/handleStructuredUpdate
// skip per-prefix work when unconfigured. active stays a statement about the CONFIG:
// whether any of those servers ever delivers data is RTRSession.synced (syncedSessions).
func (rp *rPKIPlugin) startSessions(cfg *rpkiConfig) {
	rp.active.Store(false)
	if cfg == nil || len(cfg.CacheServers) == 0 {
		logger().Info("rpki: no cache servers configured")
		return
	}

	rp.active.Store(true)
	rp.aspaEnabled.Store(cfg.ASPAValidation)
	rp.aspaInvalidAction.Store(uint32(cfg.ASPAInvalidAction))
	rp.aspaUnknownAction.Store(uint32(cfg.ASPAUnknownAction))
	rp.originInvalidAction.Store(uint32(cfg.OriginInvalidAction))
	rp.originNotFoundAction.Store(uint32(cfg.OriginNotFoundAction))
	// Swap in the per-peer action map (may be nil when no peer/group overrides exist).
	// atomic.Pointer publishes an immutable map; buildDecisions reads it lock-free.
	peerActions := cfg.PeerActions
	rp.perPeerActions.Store(&peerActions)

	rp.mu.Lock()
	defer rp.mu.Unlock()

	for _, cs := range cfg.CacheServers {
		session := newRTRSession(cs.Address, cs.Port, cs.Preference, cs.SourceAddress, rp.cache, rp.aspaCache, rp.stopCh)
		session.onASPAChange = rp.handleASPAChange
		session.onROAChange = rp.handleROAChange
		rp.sessions = append(rp.sessions, session)
		rp.sessionWg.Go(session.Run)
		logger().Info("rpki: started RTR session", "address", cs.Address, "port", cs.Port)
	}

	if m := rpkiMetricsPtr.Load(); m != nil {
		m.sessionsActive.Set(float64(len(rp.sessions)))
	}
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

	// Extract AS_PATH once for both origin AS and ASPA verification.
	asp := rpkiASPathFromWire(msg.AttrsWire)
	originAS := rpkiOriginASFromASPath(asp)
	if originAS == OriginNone {
		return
	}

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
	if rp.aspaEnabled.Load() && asp != nil {
		aspaState, normalizedPath = aspaStateForPath(rp.aspaCache, asp.Segments)
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
	rp.removeWithdrawnFromTracker(peerAddr, wu, ctx)

	// Validate IPv4 unicast NLRIs.
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(family.Family{AFI: 1, SAFI: 1})
		rp.validateNLRIs(peerAddr, peerName, peerGroup, peerASN, msgID, "ipv4/unicast",
			nlriData, addPath, false, originAS, cacheEmpty, aspaState, carriesBlackhole)
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
			rp.validateNLRIs(peerAddr, peerName, peerGroup, peerASN, msgID, fam.String(),
				nlriBytes, addPath, fam.AFI == 2, originAS, cacheEmpty, aspaState, carriesBlackhole)
			if aspaState != aspaStateNone {
				rp.trackNLRIs(peerAddr, peerName, peerGroup, peerASN, msgID, fam.String(),
					nlriBytes, addPath, fam.AFI == 2, normalizedPath, aspaState)
			}
		}
	}

}

// validateNLRIs walks wire NLRI bytes and validates each prefix against the ROA cache.
func (rp *rPKIPlugin) validateNLRIs(peerAddr, peerName, peerGroup string, peerASN uint32, msgID uint64,
	family string, nlriData []byte, addPath, isIPv6 bool, originAS uint32, cacheEmpty bool, aspaState uint8,
	blackhole bool) {

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
			peerAddr:  peerAddr,
			peerGroup: peerGroup,
			family:    family,
			prefix:    prefix,
			pathID:    pathID,
			state:     state,
			aspaState: aspaState,
			originAS:  originAS,
			blackhole: blackhole,
		}:
		case <-rp.stopCh:
			return
		}

		// Track the route for RFC 6811 Section 4 origin re-validation when the ROA cache (VRP set)
		// changes. Independent of ASPA: origin validation runs whenever RPKI is active.
		rp.originTracker.Track(routeKey{peerAddr: peerAddr, family: family, prefix: prefix, pathID: pathID},
			peerGroup, originAS, state, aspaState, blackhole)
	}

	if len(familyResults) > 0 || cacheEmpty {
		rp.emitRPKIEvent(peerAddr, peerName, peerASN, msgID, family, familyResults, cacheEmpty, aspaState)
	}
}

// handleEvent processes BGP events (UPDATE received).
// Validates each prefix against the ROA cache, enqueues accept/reject decisions
// to the async worker, and emits an rpki event with per-prefix validation states.
func (rp *rPKIPlugin) handleEvent(event *bgp.Event) {
	if !rp.active.Load() {
		return
	}

	eventType := event.GetEventType()
	if eventType != rpc.EventKindUpdate {
		return
	}

	peerAddr := event.GetPeerAddress()
	if peerAddr == "" {
		return
	}
	peerName := event.GetPeerName()
	peerGroup := event.GetPeerGroup()

	// Use parsed AS_PATH (already ASN4-normalized) when available.
	// Fall back to raw attribute parsing only if ASPath is empty.
	originAS := originASFromParsed(event.ASPath)
	if originAS == OriginNone {
		if len(event.RawAttributeBytes) > 0 {
			originAS = extractOriginASFromBytes(event.RawAttributeBytes)
		} else if event.RawAttributes != "" {
			originAS = extractOriginAS(event.RawAttributes)
		}
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
	blackhole := false
	if len(event.RawAttributeBytes) > 0 {
		blackhole = rp.carriesAgreedBlackhole(peerAddr, peerGroup,
			attribute.NewAttributesWire(event.RawAttributeBytes, bgpctx.APIContextID))
	}

	// ASPA verification on JSON fallback path.
	// event.ASPath is a flat []uint32 without segment types, so AS_SET
	// detection is unavailable. Consecutive-dup removal is applied.
	aspaState := aspaStateNone
	if rp.aspaEnabled.Load() && len(event.ASPath) > 0 {
		path := deduplicateASPath(event.ASPath)
		aspaState = verifyASPA(rp.aspaCache, path)
	}

	// Validate each NLRI prefix against the ROA cache.
	// Collect per-family results for rpki event emission.
	for fam, ops := range event.FamilyOps {
		famName := fam.String()
		familyResults := make(map[string]uint8)

		for _, op := range ops {
			if op.Action != routeaction.Add {
				continue
			}

			for _, nlriVal := range op.NLRIs {
				prefix, pathID := bgp.ParseNLRIValue(nlriVal)
				if prefix == "" {
					continue
				}

				state := rp.cache.Validate(prefix, originAS)
				familyResults[prefix] = state

				// Blocking enqueue to async worker (backpressure if worker falls behind).
				select {
				case rp.validateCh <- validationRequest{
					peerAddr:  peerAddr,
					peerGroup: peerGroup,
					family:    famName,
					prefix:    prefix,
					pathID:    pathID,
					state:     state,
					aspaState: aspaState,
					originAS:  originAS,
					blackhole: blackhole,
				}:
				case <-rp.stopCh:
					return
				}
			}
		}

		// Emit rpki event only if there were "add" operations (skip pure withdrawals).
		if len(familyResults) > 0 || cacheEmpty {
			rp.emitRPKIEvent(peerAddr, peerName, event.GetPeerASN(), event.GetMsgID(), famName, familyResults, cacheEmpty, aspaState)
		}
	}
}

// emitRPKIEvent emits an rpki validation event via the SDK EmitEvent RPC.
// Called after validating all prefixes in a family for a single UPDATE.
// aspaState is included when != aspaStateNone.
func (rp *rPKIPlugin) emitRPKIEvent(peerAddr, peerName string, peerASN uint32, msgID uint64, famName string, results map[string]uint8, cacheEmpty bool, aspaState uint8) {
	var event string
	if cacheEmpty {
		event = buildRPKIEventUnavailable(peerAddr, peerName, peerASN, msgID)
	} else {
		event = buildRPKIEvent(peerAddr, peerName, peerASN, msgID, famName, results, aspaState)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := rp.plugin.EmitEvent(ctx, "bgp", "rpki", "received", peerAddr, event)
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

		var valState uint8
		if !reject {
			valState = req.state
		}
		decisions[i] = rpc.ValidationDecision{
			Accept:   !reject,
			PeerAddr: req.peerAddr,
			Family:   req.family,
			Prefix:   req.prefix,
			PathID:   req.pathID,
			ValState: valState,
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

	// Make an owned copy of the path for the tracker.
	pathCopy := make([]uint32, len(normalizedPath))
	copy(pathCopy, normalizedPath)

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
			path:      pathCopy,
			aspaState: aspaState,
		})
	}
}

// removeWithdrawnFromTracker removes withdrawn routes from the ASPA tracker.
func (rp *rPKIPlugin) removeWithdrawnFromTracker(peerAddr string, wu *wireu.WireUpdate, ctx *bgpctx.EncodingContext) {
	// IPv4 withdrawn routes.
	wdData, err := wu.Withdrawn()
	if err == nil && len(wdData) > 0 {
		addPath := ctx != nil && ctx.AddPath(family.Family{AFI: 1, SAFI: 1})
		rp.removeTrackedNLRIs(peerAddr, "ipv4/unicast", wdData, addPath, false)
	}

	// MP_UNREACH_NLRI withdrawn routes.
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		wdBytes := mpUnreach.WithdrawnBytes()
		if len(wdBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			rp.removeTrackedNLRIs(peerAddr, fam.String(), wdBytes, addPath, fam.AFI == 2)
		}
	}
}

// removeTrackedNLRIs walks wire NLRI bytes and removes each from the ASPA tracker.
func (rp *rPKIPlugin) removeTrackedNLRIs(peerAddr, fam string, nlriData []byte, addPath, isIPv6 bool) {
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
		rp.aspaTracker.Remove(key)
		rp.originTracker.Remove(key)
	}
}

// handleROAChange is called by RTR sessions when the ROA cache (VRP set) changes at End of Data.
// RFC 6811 Section 4: it re-validates every tracked route's origin state against the updated cache
// and re-dispatches an accept/reject decision for each route whose state changed, so the
// Adj-RIB-In and the decision process reflect the new VRPs. buildDecisions applies the configured
// invalid-action to the re-dispatched decisions, exactly as it does for freshly received routes.
func (rp *rPKIPlugin) handleROAChange() {
	changed := rp.originTracker.revalidate(rp.cache)
	if len(changed) == 0 {
		return
	}
	for _, c := range changed {
		select {
		case rp.validateCh <- validationRequest{
			peerAddr:  c.key.peerAddr,
			peerGroup: c.peerGroup,
			family:    c.key.family,
			prefix:    c.key.prefix,
			pathID:    c.key.pathID,
			state:     c.state,
			aspaState: c.aspaState,
			originAS:  c.originAS,
			blackhole: c.blackhole,
		}:
		case <-rp.stopCh:
			return
		}
	}
	logger().Info("rpki: re-validated routes after VRP change", "changed", len(changed))
}

// handleASPAChange is called by RTR sessions when ASPA cache data changes.
// Re-validates tracked routes, emits updated events for state changes,
// and dispatches reject for routes that ASPA policy now demands rejecting.
func (rp *rPKIPlugin) handleASPAChange(changedCustomers []uint32) {
	if !rp.aspaEnabled.Load() {
		return
	}
	invalidAction := uint8(rp.aspaInvalidAction.Load()) //nolint:gosec // stored as uint8
	unknownAction := uint8(rp.aspaUnknownAction.Load()) //nolint:gosec // stored as uint8

	changed := rp.aspaTracker.revalidate(rp.aspaCache, changedCustomers)
	for _, rt := range changed {
		rp.emitRPKIEvent(rt.key.peerAddr, rt.peerName, rt.peerASN, rt.msgID,
			rt.key.family, nil, false, rt.aspaState)

		if aspaOverridesAccept(rt.aspaState, invalidAction, unknownAction) {
			select {
			case rp.validateCh <- validationRequest{
				peerAddr:  rt.key.peerAddr,
				peerGroup: rt.peerGroup,
				family:    rt.key.family,
				prefix:    rt.key.prefix,
				pathID:    rt.key.pathID,
				state:     ValidationValid,
				aspaState: rt.aspaState,
			}:
			case <-rp.stopCh:
				return
			}
		}
	}
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

// rpkiOriginASFromASPath extracts the origin AS from an already-parsed ASPath.
func rpkiOriginASFromASPath(asp *attribute.ASPath) uint32 {
	if asp == nil || len(asp.Segments) == 0 {
		return OriginNone
	}
	var lastASN uint32
	for _, seg := range asp.Segments {
		if len(seg.ASNs) > 0 {
			lastASN = seg.ASNs[len(seg.ASNs)-1]
		}
	}
	if lastASN == 0 {
		return OriginNone
	}
	return lastASN
}

// handleCommand processes RPKI CLI commands.
func (rp *rPKIPlugin) handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case "show bgp rpki":
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

// syncedSessions counts how many configured cache servers have completed at least one sync.
// This is the question rp.active does not answer: active says a cache server is configured,
// and stays true for a server whose data never arrives or arrives unreadable. Without this
// count an operator cannot tell "the VRP set covers nothing" from "ze has no VRP set".
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
	b.Str(`{"running":true,"vrp-count-ipv4":`).Int(int64(v4))
	b.Str(`,"vrp-count-ipv6":`).Int(int64(v6))
	b.Str(`,"sessions":`).Int(int64(len(snaps)))
	b.Str(`,"sessions-synced":`).Int(int64(synced))
	// "synced" false with "running" true is the state an operator cannot otherwise see:
	// RPKI is configured and validating, and no cache server has ever delivered data, so
	// every prefix reads not-found for want of a VRP set rather than for want of a ROA.
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
		b.Str(`,"asn":`).Uint32(e.ASN)
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
		b.Str(`,"asn":`).Uint32(e.ASN)
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
// validation-enabled is a constant. A pipe operator can do none of those, which
// is why the command computes them and the alias only selects them.
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
	b.Str(`,"validation-enabled":true`)
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

	originAS, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil {
		return statusError, "", fmt.Errorf("invalid ASN: %s", args[1])
	}

	state := rp.cache.Validate(prefix, uint32(originAS)) //nolint:gosec // range checked by ParseUint
	covering := rp.cache.Lookup(prefix)

	b := textbuf.Get()
	defer b.Release()
	b.Str(`{"prefix":"`).Str(prefix).Byte('"')
	b.Str(`,"origin-asn":`).Uint(originAS)
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
		b.Str(`,"asn":`).Uint32(e.ASN)
		b.Byte('}')
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}

const aspaDiagLimit = 1000

func (rp *rPKIPlugin) aspaCommand(args []string) (string, any, error) {
	// "show bgp rpki aspa <customer-asn>" looks up a specific customer.
	if len(args) > 0 && args[0] != "" {
		asn, err := strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			return statusError, "", fmt.Errorf("invalid ASN: %s", args[0])
		}
		providers := rp.aspaCache.lookupCustomer(uint32(asn)) //nolint:gosec // range checked by ParseUint
		b := textbuf.Get()
		defer b.Release()
		b.Str(`{"customer-asn":`).Uint(asn)
		if providers == nil {
			b.Str(`,"found":false,"providers":[]}`)
		} else {
			b.Str(`,"found":true,"providers":[`)
			for i, p := range providers {
				if i > 0 {
					b.Byte(',')
				}
				b.Uint32(p)
			}
			b.Str(`]}`)
		}
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
		b.Str(`{"customer-asn":`).Uint32(e.CustomerAS)
		b.Str(`,"providers":[`)
		for j, p := range e.Providers {
			if j > 0 {
				b.Byte(',')
			}
			b.Uint32(p)
		}
		b.Str(`]}`)
	}
	b.Str(`]}`)
	return statusDone, json.RawMessage(b.String()), nil
}
