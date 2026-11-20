// Design: docs/guide/l2tp.md -- L2TP CLI handlers
//
// Package l2tp registers engine-side RPC handlers that expose the L2TP
// subsystem's observability and teardown surface to the CLI. The L2TP
// subsystem runs in the same process as the engine, so handlers reach
// it through the l2tp.LookupService() service locator rather than
// crossing a plugin pipe.
//
// Two package-level schemas register via init():
//
//   - internal/component/l2tp/yang (ze-l2tp-api.yang) -- RPC definitions
//   - internal/component/l2tp/cmd/yang (ze-l2tp-cmd.yang) -- CLI tree
//
// Both are imported here so a blank import of this package wires the
// full CLI surface without touching the core dispatcher.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/component/l2tp/yang" // register ze-l2tp-api.yang
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errL2tpObserverNotEnabledCqmDisabled = errors.New("l2tp: observer not enabled (CQM disabled)")
	errL2tpMissingLoginArgument          = errors.New("l2tp: missing login argument")
)

// The CLI argument keywords these handlers read.
const (
	argAll       = "all"
	argSessionID = "session-id"
	argTunnelID  = "tunnel-id"
)

// The disconnect audit keywords are shared by parsing and completion.
const (
	kwActor  = "actor"
	kwReason = "reason"
	kwCause  = "cause"
)

// The response payload keys. Two of them hold the same text as an argument
// keyword above and name something else: one is what an operator types, the
// other is what the JSON carries.
const (
	keyAction       = "action"
	keyCount        = "count"
	keyLogin        = "login"
	keySubscriberID = "subscriber-id"
	keyPPPInterface = "ppp-interface"
	keyError        = "error"
	keySessionID    = "session-id"
	keyState        = "state"
	keyTunnelID     = "tunnel-id"
)

// errSubsystemUnavailable is returned when any show/teardown command
// runs while the L2TP subsystem has not been started (or has been
// stopped). The handler converts it into a plugin.StatusError response
// so the CLI prints a clear message.
var errSubsystemUnavailable = errors.New("l2tp: subsystem not running")

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:summary", Handler: handleSummary},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:tunnels", Handler: handleTunnels},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:tunnel", Handler: handleTunnel},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:sessions", Handler: handleSessions},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:session", Handler: handleSession, Complete: completeSessionRead},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:statistics", Handler: handleStatistics},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:listeners", Handler: handleListeners},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:config", Handler: handleConfig},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:observer", Handler: handleObserver, Complete: completeSessionRead},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:cqm", Handler: handleCQM},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:echo", Handler: handleEcho},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:reliable", Handler: handleReliable},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:tunnel-history", Handler: handleTunnelHistory},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:session-history", Handler: handleSessionHistory, Complete: completeSessionRead},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:tunnel-teardown", Handler: handleTunnelTeardown},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:tunnel-teardown-all", Handler: handleTunnelTeardownAll},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:session-teardown", Handler: handleSessionTeardown, Complete: completeSessionTeardown},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:session-teardown-user", Handler: handleSessionTeardownUser, Complete: completeSessionTeardownUser},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:session-teardown-address", Handler: handleSessionTeardownAddress, Complete: completeSessionTeardownAddress},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:session-teardown-all", Handler: handleSessionTeardownAll},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:session-traffic", Handler: handleSessionTraffic, Complete: completeSessionRead},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp:outgoing-call", Handler: handleOutgoingCall},
	)
}

// -----------------------------------------------------------------
// Read handlers
// -----------------------------------------------------------------

// handleSummary returns aggregate counters.
func handleSummary(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	snap := svc.Snapshot()
	listeners := svc.Listeners()
	payload := map[string]any{
		"tunnel-count":   snap.TunnelCount,
		"session-count":  snap.SessionCount,
		"listener-count": len(listeners),
		"captured-at":    snap.CapturedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	return jsonResponse("l2tp summary", payload)
}

// handleTunnels returns the tunnel table.
func handleTunnels(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	snap := svc.Snapshot()
	out := make([]map[string]any, 0, len(snap.Tunnels))
	for i := range snap.Tunnels {
		out = append(out, tunnelJSON(&snap.Tunnels[i], false))
	}
	return jsonResponse("l2tp tunnels", out)
}

// idSelectorArgs returns the typed `id` selector value (from
// `show l2tp tunnel id <n>` / `show l2tp session id <n>`) followed by any
// positional args, so parseIDArg accepts both the typed and bare forms.
func idSelectorArgs(ctx *pluginserver.CommandContext, args []string) []string {
	if ctx != nil {
		if id := ctx.Selector("id"); id != "" {
			return append([]string{id}, args...)
		}
	}
	return args
}

// handleTunnel returns one tunnel by ID.
func handleTunnel(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	tid, err := parseIDArg(idSelectorArgs(ctx, args), argTunnelID)
	if err != nil {
		return errResponse(err), nil
	}
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	ts, ok := svc.LookupTunnel(tid)
	if !ok {
		return errResponse(fmt.Errorf("l2tp: no tunnel with local-tid=%d", tid)), nil
	}
	return jsonResponse("l2tp tunnel", tunnelJSON(&ts, true))
}

// handleSessions returns the session table (flattened across tunnels).
func handleSessions(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	snap := svc.Snapshot()
	out := make([]map[string]any, 0, snap.SessionCount)
	for i := range snap.Tunnels {
		for j := range snap.Tunnels[i].Sessions {
			out = append(out, sessionJSON(&snap.Tunnels[i].Sessions[j], false))
		}
	}
	return jsonResponse("l2tp sessions", out)
}

// handleSession returns one session by ID.
func handleSession(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	captured, _, refusal := captureSessionRead(svc, idSelectorArgs(ctx, validated.Tokens()))
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	ss, err := captured.Snapshot()
	if err != nil {
		return plugin.OperatorErrorResponse(sessionReadError(err, "The captured L2TP session is no longer available")), nil
	}
	return jsonResponse("l2tp session", sessionJSON(&ss, true))
}

// handleStatistics returns protocol counters. spec-l2tp-10 will add
// per-message counters; spec-l2tp-7 returns the basic aggregates
// already available.
func handleStatistics(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	snap := svc.Snapshot()
	payload := map[string]any{
		"tunnels-active":  snap.TunnelCount,
		"sessions-active": snap.SessionCount,
	}
	return jsonResponse("l2tp statistics", payload)
}

// handleListeners returns original names and bound UDP endpoints.
func handleListeners(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	lns := svc.Listeners()
	out := make([]map[string]any, 0, len(lns))
	for _, ln := range lns {
		row := map[string]any{
			"address": ln.Addr.Addr().String(),
			"port":    int(ln.Addr.Port()),
		}
		if ln.Name != "" {
			row["name"] = ln.Name
		}
		out = append(out, row)
	}
	return jsonResponse("l2tp listeners", out)
}

// handleConfig returns the effective runtime configuration.
func handleConfig(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	cs := svc.EffectiveConfig()
	listenAddrs := make([]string, 0, len(cs.ListenAddrs))
	for _, a := range cs.ListenAddrs {
		listenAddrs = append(listenAddrs, a.String())
	}
	payload := map[string]any{
		"enabled":              cs.Enabled,
		"max-tunnels":          int(cs.MaxTunnels),
		"max-sessions":         int(cs.MaxSessions),
		"auth-method":          cs.AuthMethod,
		"mru":                  int(cs.MRU),
		"authentication-allow": cs.AuthAllow,
		"allow-no-auth":        cs.AllowNoAuth,
		"hello-interval":       int(cs.HelloInterval.Seconds()),
		"hello-retries":        int(cs.HelloRetries),
		"shared-secret":        cs.SharedSecret,
		"listeners":            listenAddrs,
	}
	return jsonResponse("l2tp config", payload)
}

// -----------------------------------------------------------------
// Diagnostic handlers (spec-diag-1-runtime-state)
// -----------------------------------------------------------------

func handleObserver(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := idSelectorArgs(ctx, validated.Tokens())
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	arg := firstPositionalArg(args)
	if arg == argAll || arg == "" {
		if len(args) > 1 {
			return plugin.OperatorErrorResponse(sessionReadError(nil, "L2TP observer summary accepts no session qualifiers")), nil
		}
		summaries := svc.SessionSummaries()
		if summaries == nil {
			return errResponse(errL2tpObserverNotEnabledCqmDisabled), nil
		}
		out := make([]map[string]any, 0, len(summaries))
		for i := range summaries {
			s := &summaries[i]
			m := map[string]any{
				keySessionID:    int(s.SessionID),
				"event-count":   s.EventCount,
				keyTunnelID:     int(s.TunnelID),
				keySubscriberID: s.Birth,
			}
			if s.EventCount > 0 {
				m["last-event-type"] = s.LastEventType
				m["last-event-time"] = s.LastEventTime.UTC().Format("2006-01-02T15:04:05Z07:00")
			}
			out = append(out, m)
		}
		return jsonResponse("l2tp observer", map[string]any{
			"sessions": out,
			keyCount:   len(out),
		})
	}
	captured, sid, refusal := captureSessionRead(svc, args)
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	events, err := captured.SessionEvents()
	if err != nil {
		return plugin.OperatorErrorResponse(sessionReadError(err, "The captured L2TP session history is no longer available")), nil
	}
	if events == nil {
		return plugin.OperatorErrorResponse(sessionReadError(nil, "The captured L2TP session has no observer data")), nil
	}
	out := make([]map[string]any, 0, len(events))
	for i := range events {
		ev := &events[i]
		m := map[string]any{
			"timestamp":     ev.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			keySubscriberID: ev.Birth,
			"type":          ev.Type.String(),
			keyTunnelID:     int(ev.TunnelID),
			keySessionID:    int(ev.SessionID),
		}
		if ev.RTT > 0 {
			m["rtt-ms"] = float64(ev.RTT.Microseconds()) / 1000.0
		}
		if ev.Actor != "" {
			m["actor"] = ev.Actor
		}
		if ev.Reason != "" {
			m["reason"] = ev.Reason
		}
		if ev.Cause != 0 {
			m["cause"] = int(ev.Cause)
		}
		out = append(out, m)
	}
	return jsonResponse("l2tp observer", map[string]any{
		keySessionID:    int(sid),
		keySubscriberID: captured.Birth(),
		argListener:     originalReadListener(captured.Listener()),
		keyTunnelID:     observerReadTunnel(events),
		"events":        out,
		keyCount:        len(out),
	})
}

func handleCQM(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	arg := firstPositionalArg(args)
	if arg == argAll || arg == "summary" || arg == "" {
		summaries := svc.LoginSummaries()
		if summaries == nil {
			return errResponse(errL2tpObserverNotEnabledCqmDisabled), nil
		}
		out := make([]map[string]any, 0, len(summaries))
		for i := range summaries {
			s := &summaries[i]
			out = append(out, map[string]any{
				keyLogin:       s.Login,
				"bucket-count": s.BucketCount,
				"last-state":   s.LastState,
				"echo-count":   s.EchoCount,
				"avg-rtt-ms":   float64(s.AvgRTT.Microseconds()) / 1000.0,
			})
		}
		return jsonResponse("l2tp cqm", map[string]any{
			"logins": out,
			keyCount: len(out),
		})
	}
	buckets := svc.LoginSamples(arg)
	if buckets == nil {
		return errResponse(fmt.Errorf("l2tp: no CQM data for login %q", arg)), nil
	}
	out := make([]map[string]any, 0, len(buckets))
	for i := range buckets {
		b := &buckets[i]
		out = append(out, map[string]any{
			"start":      b.Start.UTC().Format("2006-01-02T15:04:05Z07:00"),
			keyState:     b.State.String(),
			"echo-count": b.EchoCount,
			"min-rtt-ms": float64(b.MinRTT.Microseconds()) / 1000.0,
			"max-rtt-ms": float64(b.MaxRTT.Microseconds()) / 1000.0,
			"avg-rtt-ms": float64(b.AvgRTT().Microseconds()) / 1000.0,
		})
	}
	return jsonResponse("l2tp cqm", map[string]any{
		keyLogin:  arg,
		"buckets": out,
		keyCount:  len(out),
	})
}

func handleEcho(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	login := firstPositionalArg(args)
	if login == "" {
		return errResponse(errL2tpMissingLoginArgument), nil
	}
	state := svc.EchoState(login)
	if state == nil {
		return errResponse(fmt.Errorf("l2tp: no echo data for login %q", login)), nil
	}
	m := map[string]any{
		keyLogin:       state.Login,
		"last-rtt-ms":  float64(state.LastRTT.Microseconds()) / 1000.0,
		"bucket-state": state.BucketState,
	}
	if state.EchoInterval > 0 {
		m["echo-interval"] = state.EchoInterval.String()
	}
	if state.LossRatio != nil {
		m["loss-ratio"] = *state.LossRatio
	}
	return jsonResponse("l2tp echo", m)
}

func handleReliable(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	tid, err := parseIDArg(args, argTunnelID)
	if err != nil {
		return errResponse(err), nil
	}
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	stats := svc.ReliableStats(tid)
	if stats == nil {
		return errResponse(fmt.Errorf("l2tp: no tunnel with local-tid=%d", tid)), nil
	}
	return jsonResponse("l2tp reliable", map[string]any{
		keyTunnelID:        int(tid),
		"ns":               int(stats.NextSendSeq),
		"nr":               int(stats.NextRecvSeq),
		"peer-nr":          int(stats.PeerNr),
		"outstanding":      stats.Outstanding,
		"retransmit-count": stats.RetransmitCount,
		"cwnd":             int(stats.CWND),
		"ssthresh":         int(stats.SSThresh),
		"peer-rws":         int(stats.PeerRWS),
	})
}

// firstPositionalArg returns the first non-empty, non-flag argument.
func firstPositionalArg(args []string) string {
	for _, a := range args {
		if a != "" && !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// -----------------------------------------------------------------
// FSM history handlers (spec-diag-2)
// -----------------------------------------------------------------

func handleTunnelHistory(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	tid, err := parseIDArg(args, argTunnelID)
	if err != nil {
		return errResponse(err), nil
	}
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	history := svc.TunnelFSMHistory(tid)
	if history == nil {
		return errResponse(fmt.Errorf("l2tp: no tunnel with local-tid=%d", tid)), nil
	}
	return jsonResponse("l2tp tunnel history", fsmTransitionsJSON(history))
}

func handleSessionHistory(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	captured, _, refusal := captureSessionRead(svc, idSelectorArgs(ctx, validated.Tokens()))
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	ss, err := captured.Snapshot()
	if err != nil {
		return plugin.OperatorErrorResponse(sessionReadError(err, "The captured L2TP session is no longer available")), nil
	}
	history, err := captured.FSMHistory()
	if err != nil {
		return plugin.OperatorErrorResponse(sessionReadError(err, "The captured L2TP session transitions are no longer available")), nil
	}
	row := fsmTransitionsJSON(history)
	row[keySubscriberID] = ss.SubscriberID
	row[keySessionID] = int(ss.LocalSID)
	row[keyTunnelID] = int(ss.TunnelLocalTID)
	if ss.Listener.IsValid() {
		row[argListener] = ss.Listener.String()
	}
	return jsonResponse("l2tp session history", row)
}

func fsmTransitionsJSON(transitions []l2tp.FSMTransition) map[string]any {
	out := make([]map[string]any, 0, len(transitions))
	for i := range transitions {
		t := &transitions[i]
		m := map[string]any{
			"from": t.From,
			"to":   t.To,
		}
		if !t.Timestamp.IsZero() {
			m["timestamp"] = t.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		if t.Trigger != "" {
			m["trigger"] = t.Trigger
		}
		out = append(out, m)
	}
	return map[string]any{"transitions": out, keyCount: len(out)}
}

// -----------------------------------------------------------------
// Destructive handlers
// -----------------------------------------------------------------

func handleTunnelTeardown(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	tid, err := parseIDArg(args, argTunnelID)
	if err != nil {
		return errResponse(err), nil
	}
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	if err := svc.TeardownTunnel(tid); err != nil {
		return errResponse(err), nil
	}
	return jsonResponse("l2tp tunnel teardown", map[string]any{
		keyAction:   "tunnel-teardown",
		keyTunnelID: int(tid),
		"status":    "sent",
	})
}

func handleTunnelTeardownAll(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	n := svc.TeardownAllTunnels()
	return jsonResponse("l2tp tunnel teardown-all", map[string]any{
		keyAction:         "tunnel-teardown-all",
		"tunnels-cleared": n,
	})
}

func handleSessionTeardown(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	request, refusal := parseSessionTeardownRequest(validated.Tokens())
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	svc := l2tp.LookupService()
	if svc == nil {
		return sessionTeardownFailure(request.selector.SID, errSubsystemUnavailable), nil
	}
	selector, refusal := request.resolveSelector(svc)
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	captured, err := svc.CaptureSession(selector)
	if err != nil {
		if errors.Is(err, l2tp.ErrSessionAmbiguous) {
			return sessionTeardownAmbiguity(svc, selector, err), nil
		}
		if errors.Is(err, l2tp.ErrSessionNotFound) {
			return sessionTeardownNotFound(selector.SID, err), nil
		}
		return sessionTeardownFailure(selector.SID, err), nil
	}
	if captured == nil {
		return sessionTeardownNotFound(selector.SID, l2tp.ErrSessionNotFound), nil
	}
	if refusal := finishSessionTeardown(ctx, captured, selector.SID, request.disconnectAudit); refusal != nil {
		return refusal, nil
	}
	return jsonResponse("l2tp session teardown", sessionTeardownResult(selector.SID, request.disconnectAudit))
}

func handleSessionTeardownAll(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	n := svc.TeardownAllSessions()
	return jsonResponse("l2tp session teardown-all", map[string]any{
		keyAction:          "session-teardown-all",
		"sessions-cleared": n,
	})
}

// -----------------------------------------------------------------
// JSON shape helpers
// -----------------------------------------------------------------

// tunnelJSON renders a TunnelSnapshot. detail=true adds per-session
// entries; detail=false returns the table summary only. Takes a
// pointer to avoid copying the TunnelSnapshot value (the linter
// flags rangeValCopy on the 176-byte struct otherwise).
func tunnelJSON(t *l2tp.TunnelSnapshot, detail bool) map[string]any {
	m := map[string]any{
		"local-tid":     int(t.LocalTID),
		"remote-tid":    int(t.RemoteTID),
		"peer":          t.PeerAddr.String(),
		"peer-hostname": t.PeerHostName,
		keyState:        t.State,
		"session-count": t.SessionCount,
		"created-at":    t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"last-activity": formatTime(t.LastActivity),
		"max-sessions":  int(t.MaxSessions),
	}
	if detail {
		m["peer-framing"] = l2tp.FormatFraming(t.PeerFraming)
		m["peer-bearer"] = t.PeerBearer
		m["peer-recv-window"] = int(t.PeerRecvWindow)
		ss := make([]map[string]any, 0, len(t.Sessions))
		for i := range t.Sessions {
			ss = append(ss, sessionJSON(&t.Sessions[i], false))
		}
		m["sessions"] = ss
	}
	return m
}

// sessionJSON renders a SessionSnapshot. detail=true adds the less
// commonly-shown fields (speeds, framing, lns-mode). Takes a pointer
// to match tunnelJSON.
func sessionJSON(s *l2tp.SessionSnapshot, detail bool) map[string]any {
	assigned := ""
	if s.AssignedAddr.IsValid() {
		assigned = s.AssignedAddr.String()
	}
	m := map[string]any{
		"local-sid":        int(s.LocalSID),
		"remote-sid":       int(s.RemoteSID),
		"tunnel-local-tid": int(s.TunnelLocalTID),
		keyState:           s.State,
		"username":         s.Username,
		"assigned-addr":    assigned,
		"family":           s.Family,
		"created-at":       s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if s.Birth.IsValid() {
		m["birth"] = s.Birth.String()
	}
	if s.Listener.IsValid() {
		m[argListener] = s.Listener.String()
	}
	if s.SubscriberID != "" {
		m[keySubscriberID] = s.SubscriberID
	}
	// A session may exist before LCP opens or while it renegotiates. Keep
	// unknown distinct from an agreed no-auth method and from local config.
	m["mru"] = nil
	m["auth-method"] = nil
	if s.MRU != 0 {
		m["mru"] = int(s.MRU)
		m["auth-method"] = s.AuthMethod
	}
	// The IP MTU PPP configured on pppN: the LCP transmit limit, lowered to
	// an accepted RADIUS Framed-MTU (RFC 2865 Section 5.12). Null until the
	// network lifetime is up and after LCP goes down.
	m["mtu"] = nil
	if s.MTU != 0 {
		m["mtu"] = int(s.MTU)
	}
	// What the LNS did with the LAC's proxy LCP (RFC 2661 Section 4.4.5):
	// accepted, renegotiated or none; null until LCP first opens.
	m["proxy-lcp"] = nil
	if s.ProxyLCP != "" {
		m["proxy-lcp"] = s.ProxyLCP
	}
	if detail {
		m["tx-connect-speed"] = int64(s.TxConnectSpeed)
		m["rx-connect-speed"] = int64(s.RxConnectSpeed)
		m["framing-type"] = l2tp.FormatFraming(s.FramingType)
		m["sequencing-required"] = s.SequencingRequired
		m["lns-mode"] = s.LNSMode
		m["kernel-setup-needed"] = s.KernelSetupNeeded
	}
	// A realm-relayed LAC session names its binding and what its ICCN
	// carried; a Service-Name relay or a local session carries none of these
	// keys (spec-home-cpe-14 AC-12).
	if rr := s.RealmRelay; rr != nil {
		m["relay-realm"] = rr.Realm
		m["relay-remote"] = rr.Remote
		m["proxy-lcp-sent"] = rr.ProxyLCPSent
		m["proxy-auth-sent"] = rr.ProxyAuthSent
		if rr.ProxyAuthReason != "" {
			m["proxy-auth-reason"] = rr.ProxyAuthReason
		}
	}
	return m
}

// formatTime renders zero-valued times as "" so CLI consumers can
// distinguish "never" from "1970-01-01T00:00:00Z".
func formatTime(t interface{ IsZero() bool }) string {
	if t.IsZero() {
		return ""
	}
	if tt, ok := t.(interface {
		UTC() interface {
			Format(string) string
		}
	}); ok {
		return tt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return fmt.Sprintf("%v", t)
}

// -----------------------------------------------------------------
// Misc helpers
// -----------------------------------------------------------------

// parseIDArg extracts the first positional (non-flag, non-empty)
// argument and parses it as a uint16 1..65535. Returns an error with
// a human-readable message naming `fieldName` when parsing fails.
func parseIDArg(args []string, fieldName string) (uint16, error) {
	var raw string
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		raw = a
		break
	}
	if raw == "" {
		return 0, fmt.Errorf("l2tp: missing %s argument", fieldName)
	}
	n, err := strconv.ParseUint(raw, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("l2tp: invalid %s %q: %w", fieldName, raw, err)
	}
	if n == 0 {
		return 0, fmt.Errorf("l2tp: invalid %s 0 (reserved by RFC 2661)", fieldName)
	}
	return uint16(n), nil
}

// parseKeywordArgs scans args after the first positional (the ID) for
// keyword-prefixed optional arguments: `actor <name>`, `reason <text...>`,
// and `cause <code>`. Text after "reason" is collected until the next
// keyword or end of args. "actor" and "cause" expect a single value each.
func parseKeywordArgs(args []string) (actor, reason string, cause uint32, err error) {
	seen := make(map[string]bool, 3)
	// The first token is the separately validated SID. The token count bounds
	// both this keyword walk and the nested reason-word walk.
	for index := 1; index < len(args); {
		keyword := args[index]
		if !isDisconnectAuditKeyword(keyword) {
			return "", "", 0, fmt.Errorf("unknown disconnect keyword %q", keyword)
		}
		if seen[keyword] {
			return "", "", 0, fmt.Errorf("duplicate disconnect keyword %q", keyword)
		}
		seen[keyword] = true
		index++
		if index == len(args) {
			return "", "", 0, fmt.Errorf("disconnect keyword %q requires a value", keyword)
		}
		if isDisconnectAuditKeyword(args[index]) {
			return "", "", 0, fmt.Errorf("disconnect keyword %q requires a value", keyword)
		}
		switch keyword {
		case kwActor:
			actor = args[index]
			if err := validateDisconnectText(keyword, actor); err != nil {
				return "", "", 0, err
			}
			index++
		case kwReason:
			start := index
			for index < len(args) {
				if isDisconnectAuditKeyword(args[index]) {
					break
				}
				index++
			}
			reason = textbuf.Join(args[start:index], " ")
			if err := validateDisconnectText(keyword, reason); err != nil {
				return "", "", 0, err
			}
		case kwCause:
			number, parseErr := strconv.ParseUint(args[index], 10, 32)
			if parseErr != nil {
				return "", "", 0, fmt.Errorf("invalid disconnect cause %q: %w", args[index], parseErr)
			}
			cause = uint32(number)
			index++
		}
	}
	return actor, reason, cause, nil
}

// -----------------------------------------------------------------
// Per-session traffic handler (diag-0 remaining gap)
// -----------------------------------------------------------------

func handleSessionTraffic(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := idSelectorArgs(ctx, validated.Tokens())
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}

	arg := firstPositionalArg(args)
	if arg == "" || arg == argAll {
		if len(args) > 1 {
			return plugin.OperatorErrorResponse(sessionReadError(nil, "L2TP traffic summary accepts no session qualifiers")), nil
		}
		return sessionTrafficAll(svc)
	}

	captured, _, refusal := captureSessionRead(svc, args)
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	row, err := sessionTrafficRow(captured)
	if err != nil {
		return plugin.OperatorErrorResponse(sessionReadError(err, "The captured L2TP session traffic is no longer available")), nil
	}
	return jsonResponse("l2tp session-traffic", row)
}

func sessionTrafficAll(svc l2tp.Service) (*plugin.Response, error) {
	snap := svc.Snapshot()
	rows := make([]map[string]any, 0)
	for i := range snap.Tunnels {
		for j := range snap.Tunnels[i].Sessions {
			ss := snap.Tunnels[i].Sessions[j]
			if ss.PppInterface == "" {
				continue
			}
			if !ss.Birth.IsValid() {
				rows = append(rows, map[string]any{
					keySessionID: int(ss.LocalSID), keyTunnelID: int(ss.TunnelLocalTID),
					keyPPPInterface: ss.PppInterface,
					keyError:        sessionReadError(nil, "The inventory row has no original session reference").Error(),
				})
				continue
			}
			captured, err := svc.CaptureSession(l2tp.SessionSelector{SID: ss.LocalSID, TID: ss.TunnelLocalTID, Listener: ss.Listener, Birth: ss.Birth})
			var row map[string]any
			if err == nil {
				row, err = sessionTrafficRow(captured)
			}
			if err != nil {
				row = map[string]any{
					keySessionID:    int(ss.LocalSID),
					keyTunnelID:     int(ss.TunnelLocalTID),
					keySubscriberID: ss.SubscriberID,
					argListener:     originalReadListener(ss.Listener),
					keyPPPInterface: ss.PppInterface,
					keyError:        sessionReadError(err, "The original L2TP session traffic could not be read").Error(),
				}
			}
			rows = append(rows, row)
		}
	}
	return jsonResponse("l2tp session-traffic", map[string]any{
		"sessions": rows,
		keyCount:   len(rows),
	})
}

func sessionTrafficRow(captured l2tp.CapturedSession) (map[string]any, error) {
	ss, err := captured.Snapshot()
	if err != nil {
		return nil, err
	}
	if ss.PppInterface == "" {
		return nil, fmt.Errorf("l2tp: session %d has no PPP interface", ss.LocalSID)
	}
	stats, err := captured.Counters()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		keySessionID:    int(ss.LocalSID),
		keyTunnelID:     int(ss.TunnelLocalTID),
		keySubscriberID: ss.SubscriberID,
		argListener:     originalReadListener(ss.Listener),
		keyPPPInterface: ss.PppInterface,
		"rx-bytes":      stats.RxBytes,
		"tx-bytes":      stats.TxBytes,
		"rx-packets":    stats.RxPackets,
		"tx-packets":    stats.TxPackets,
		"rx-errors":     sessionCounterJSON(stats.RxErrors),
		"tx-errors":     sessionCounterJSON(stats.TxErrors),
		"rx-dropped":    sessionCounterJSON(stats.RxDropped),
		"tx-dropped":    sessionCounterJSON(stats.TxDropped),
	}, nil
}

// jsonResponse marshals payload into a plugin.StatusDone response.
// Returns the marshal error as a transport-level error so the caller
// surfaces it to the CLI.
func jsonResponse(_ string, payload any) (*plugin.Response, error) {
	if m, ok := payload.(map[string]any); ok {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(m)}, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(data)}, nil
}

// errResponse wraps err into a plugin.StatusError response.
func errResponse(err error) *plugin.Response {
	return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}
}
