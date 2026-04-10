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
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// rpkiMetrics holds Prometheus metrics for the RPKI plugin.
type rpkiMetrics struct {
	vrpsCached         metrics.Gauge      // VRPs currently in ROA cache
	sessionsActive     metrics.Gauge      // active RTR sessions
	validationOutcomes metrics.CounterVec // validation results (labels: result)
}

// rpkiMetricsPtr stores RPKI metrics, set by SetMetricsRegistry.
var rpkiMetricsPtr atomic.Pointer[rpkiMetrics]

// SetMetricsRegistry creates RPKI metrics from the given registry.
// Called via ConfigureMetrics callback before RunEngine.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &rpkiMetrics{
		vrpsCached:         reg.Gauge("ze_rpki_vrps_cached", "VRPs currently in ROA cache."),
		sessionsActive:     reg.Gauge("ze_rpki_sessions_active", "Active RTR cache sessions."),
		validationOutcomes: reg.CounterVec("ze_rpki_validation_outcomes_total", "RPKI validation outcomes.", []string{"result"}),
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
	family   string
	prefix   string
	state    uint8
}

// RPKIPlugin implements the bgp-rpki plugin.
// It manages RTR sessions to RPKI cache servers, maintains the ROA cache,
// and validates received routes against VRPs.
type RPKIPlugin struct {
	plugin *sdk.Plugin
	cache  *ROACache
	mu     sync.RWMutex

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

	// active is true when at least one cache server is configured.
	// When false, handleEvent/handleStructuredUpdate skip all per-prefix work.
	active atomic.Bool
}

// RunRPKIPlugin runs the bgp-rpki plugin using the SDK RPC protocol.
func RunRPKIPlugin(conn net.Conn) int {
	logger().Debug("bgp-rpki plugin starting")

	p := sdk.NewWithConn("bgp-rpki", conn)
	defer func() { _ = p.Close() }()

	rp := &RPKIPlugin{
		plugin:     p,
		cache:      NewROACache(),
		validateCh: make(chan validationRequest, 4096),
		stopCh:     make(chan struct{}),
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
			if !ok || se.EventType != "update" || se.PeerAddress == "" {
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

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return rp.handleCommand(command, strings.Join(args, " "))
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

	// Enable validation gate in adj-rib-in after plugin startup completes.
	p.OnStarted(func(startCtx context.Context) error {
		enableCtx, cancel := context.WithTimeout(startCtx, 10*time.Second)
		defer cancel()
		status, _, err := p.DispatchCommand(enableCtx, "adj-rib-in enable-validation")
		if err != nil {
			logger().Error("rpki: failed to enable validation gate", "error", err)
			return err
		}
		logger().Info("rpki: validation gate enabled", "status", status)
		return nil
	})

	p.SetStartupSubscriptions([]string{"update direction received"}, nil, "full")

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "rpki status"},
			{Name: "rpki cache"},
			{Name: "rpki roa"},
			{Name: "rpki summary"},
		},
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
// skip per-prefix work when unconfigured.
func (rp *RPKIPlugin) startSessions(cfg *rpkiConfig) {
	rp.active.Store(false)
	if cfg == nil || len(cfg.CacheServers) == 0 {
		logger().Info("rpki: no cache servers configured")
		return
	}

	rp.active.Store(true)

	rp.mu.Lock()
	defer rp.mu.Unlock()

	for _, cs := range cfg.CacheServers {
		session := NewRTRSession(cs.Address, cs.Port, cs.Preference, rp.cache, rp.stopCh)
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
func (rp *RPKIPlugin) handleStructuredUpdate(se *rpc.StructuredEvent) {
	if !rp.active.Load() {
		return
	}

	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.WireUpdate == nil {
		return
	}

	// Extract origin AS from AttrsWire (lazy parse of AS_PATH only).
	originAS := rpkiOriginASFromWire(msg.AttrsWire)
	if originAS == OriginNone {
		return
	}

	peerAddr := se.PeerAddress
	peerName := se.PeerName
	peerASN := se.PeerAS
	msgID := se.MessageID
	wu := msg.WireUpdate
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())

	v4, v6 := rp.cache.Count()
	cacheEmpty := v4+v6 == 0

	// Validate IPv4 unicast NLRIs.
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(family.Family{AFI: 1, SAFI: 1})
		rp.validateNLRIs(peerAddr, peerName, peerASN, msgID, "ipv4/unicast",
			nlriData, addPath, false, originAS, cacheEmpty)
	}

	// Validate MP_REACH_NLRI announces.
	mpReach, err := wu.MPReach()
	if err == nil && mpReach != nil {
		fam := mpReach.Family()
		nlriBytes := mpReach.NLRIBytes()
		if len(nlriBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			rp.validateNLRIs(peerAddr, peerName, peerASN, msgID, fam.String(),
				nlriBytes, addPath, fam.AFI == 2, originAS, cacheEmpty)
		}
	}
}

// rpkiOriginASFromWire extracts the origin AS from AttrsWire's AS_PATH attribute.
func rpkiOriginASFromWire(attrs *attribute.AttributesWire) uint32 {
	if attrs == nil {
		return OriginNone
	}
	attr, err := attrs.Get(attribute.AttrASPath)
	if err != nil || attr == nil {
		return OriginNone
	}
	asp, ok := attr.(*attribute.ASPath)
	if !ok || len(asp.Segments) == 0 {
		return OriginNone
	}
	// Flatten segments and take last ASN.
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

// validateNLRIs walks wire NLRI bytes and validates each prefix against the ROA cache.
func (rp *RPKIPlugin) validateNLRIs(peerAddr, peerName string, peerASN uint32, msgID uint64,
	family string, nlriData []byte, addPath, isIPv6 bool, originAS uint32, cacheEmpty bool) {

	addrLen := 4
	if isIPv6 {
		addrLen = 16
	}

	familyResults := make(map[string]uint8)
	offset := 0
	for offset < len(nlriData) {
		if addPath {
			if offset+4 >= len(nlriData) {
				break
			}
			offset += 4 // skip path-ID
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
			peerAddr: peerAddr,
			family:   family,
			prefix:   prefix,
			state:    state,
		}:
		case <-rp.stopCh:
			return
		}
	}

	if len(familyResults) > 0 || cacheEmpty {
		rp.emitRPKIEvent(peerAddr, peerName, peerASN, msgID, family, familyResults, cacheEmpty)
	}
}

// handleEvent processes BGP events (UPDATE received).
// Validates each prefix against the ROA cache, enqueues accept/reject decisions
// to the async worker, and emits an rpki event with per-prefix validation states.
func (rp *RPKIPlugin) handleEvent(event *bgp.Event) {
	if !rp.active.Load() {
		return
	}

	eventType := event.GetEventType()
	if eventType != "update" {
		return
	}

	peerAddr := event.GetPeerAddress()
	if peerAddr == "" {
		return
	}
	peerName := event.GetPeerName()

	// Use parsed AS_PATH (already ASN4-normalized) when available.
	// Fall back to raw attribute parsing only if ASPath is empty.
	originAS := originASFromParsed(event.ASPath)
	if originAS == OriginNone && event.RawAttributes != "" {
		originAS = extractOriginAS(event.RawAttributes)
	}

	// Check if ROA cache is empty (unavailable).
	v4, v6 := rp.cache.Count()
	cacheEmpty := v4+v6 == 0

	// Validate each NLRI prefix against the ROA cache.
	// Collect per-family results for rpki event emission.
	for famName, ops := range event.FamilyOps {
		familyResults := make(map[string]uint8)

		for _, op := range ops {
			if op.Action != "add" {
				continue
			}

			for _, nlriVal := range op.NLRIs {
				prefix, _ := bgp.ParseNLRIValue(nlriVal)
				if prefix == "" {
					continue
				}

				state := rp.cache.Validate(prefix, originAS)
				familyResults[prefix] = state

				// Blocking enqueue to async worker (backpressure if worker falls behind).
				select {
				case rp.validateCh <- validationRequest{
					peerAddr: peerAddr,
					family:   famName,
					prefix:   prefix,
					state:    state,
				}:
				case <-rp.stopCh:
					return
				}
			}
		}

		// Emit rpki event only if there were "add" operations (skip pure withdrawals).
		if len(familyResults) > 0 || cacheEmpty {
			rp.emitRPKIEvent(peerAddr, peerName, event.GetPeerASN(), event.GetMsgID(), famName, familyResults, cacheEmpty)
		}
	}
}

// emitRPKIEvent emits an rpki validation event via the SDK EmitEvent RPC.
// Called after validating all prefixes in a family for a single UPDATE.
func (rp *RPKIPlugin) emitRPKIEvent(peerAddr, peerName string, peerASN uint32, msgID uint64, famName string, results map[string]uint8, cacheEmpty bool) {
	var event string
	if cacheEmpty {
		event = buildRPKIEventUnavailable(peerAddr, peerName, peerASN, msgID)
	} else {
		event = buildRPKIEvent(peerAddr, peerName, peerASN, msgID, famName, results)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := rp.plugin.EmitEvent(ctx, "bgp", "rpki", "received", peerAddr, event)
	if err != nil {
		logger().Warn("rpki: emit event failed", "error", err)
	}
}

// validationRetries is the number of retries for validation commands.
const validationRetries = 3

// validationWorker is a long-lived goroutine that processes validation decisions.
// It drains validateCh and issues DispatchCommand calls with retry logic,
// keeping the SDK event callback goroutine free from blocking I/O.
func (rp *RPKIPlugin) validationWorker() {
	for {
		select {
		case <-rp.stopCh:
			return
		case req := <-rp.validateCh:
			rp.dispatchValidation(req)
		}
	}
}

// dispatchValidation sends a single accept/reject command with retry.
func (rp *RPKIPlugin) dispatchValidation(req validationRequest) {
	// Guard: NotValidated (0) should not reach here; skip silently.
	if req.state == ValidationNotValidated {
		return
	}

	if m := rpkiMetricsPtr.Load(); m != nil {
		m.validationOutcomes.With(validationStateString(req.state)).Inc()
	}

	// Validate fields contain no whitespace (prevents command injection).
	if strings.ContainsAny(req.peerAddr, " \t\n\r") ||
		strings.ContainsAny(req.family, " \t\n\r") ||
		strings.ContainsAny(req.prefix, " \t\n\r") {
		logger().Warn("rpki: invalid characters in validation request fields",
			"peer", req.peerAddr, "family", req.family, "prefix", req.prefix)
		return
	}

	var cmd string
	if req.state == ValidationInvalid {
		cmd = "adj-rib-in reject-routes " + req.peerAddr + " " + req.family + " " + req.prefix
	} else {
		stateStr := "2" // NotFound
		if req.state == ValidationValid {
			stateStr = "1"
		}
		cmd = "adj-rib-in accept-routes " + req.peerAddr + " " + req.family + " " + req.prefix + " " + stateStr
	}

	for attempt := range validationRetries {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _, err := rp.plugin.DispatchCommand(ctx, cmd)
		cancel()
		if err == nil {
			return
		}
		// Retry with exponential backoff for event ordering race.
		if attempt < validationRetries-1 {
			backoff := time.Duration(10*(1<<attempt)) * time.Millisecond // 10ms, 20ms
			time.Sleep(backoff)
			continue
		}
		logger().Warn("rpki: validation command failed after retries",
			"command", cmd, "error", err, "attempts", validationRetries)
	}
}

// originASFromParsed extracts origin AS from a pre-parsed AS_PATH ([]uint32).
// Returns the last ASN in the slice, or OriginNone if empty.
func originASFromParsed(asPath []uint32) uint32 {
	if len(asPath) == 0 {
		return OriginNone
	}
	return asPath[len(asPath)-1]
}

// handleCommand processes RPKI CLI commands.
func (rp *RPKIPlugin) handleCommand(command, _ string) (string, string, error) {
	switch command {
	case "rpki status":
		return rp.statusCommand()
	case "rpki cache":
		return rp.cacheCommand()
	case "rpki roa":
		return rp.roaCommand()
	case "rpki summary":
		return rp.summaryCommand()
	}
	return statusError, "", fmt.Errorf("unknown command: %s", command)
}

func (rp *RPKIPlugin) statusCommand() (string, string, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	v4, v6 := rp.cache.Count()
	data := fmt.Sprintf(`{"running":true,"vrp-count-ipv4":%d,"vrp-count-ipv6":%d,"sessions":%d}`,
		v4, v6, len(rp.sessions))
	return statusDone, data, nil
}

func (rp *RPKIPlugin) cacheCommand() (string, string, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	data := `{"cache-servers":[]}`
	return statusDone, data, nil
}

func (rp *RPKIPlugin) roaCommand() (string, string, error) {
	v4, v6 := rp.cache.Count()
	data := fmt.Sprintf(`{"total-vrps":%d,"ipv4-vrps":%d,"ipv6-vrps":%d}`, v4+v6, v4, v6)
	return statusDone, data, nil
}

func (rp *RPKIPlugin) summaryCommand() (string, string, error) {
	v4, v6 := rp.cache.Count()
	data := fmt.Sprintf(`{"vrp-count":%d,"validation-enabled":true}`, v4+v6)
	return statusDone, data, nil
}
