// Design: docs/architecture/l2tp.md -- L2TP CLI handlers
//
// Package l2tp registers engine-side RPC handlers that expose the L2TP
// subsystem's observability and teardown surface to the CLI. The L2TP
// subsystem runs in the same process as the engine, so handlers reach
// it through the l2tp.LookupService() service locator rather than
// crossing a plugin pipe.
//
// Two package-level schemas register via init():
//
//   - internal/component/l2tp/schema (ze-l2tp-api.yang) -- RPC definitions
//   - internal/component/cmd/l2tp/schema (ze-l2tp-cmd.yang) -- CLI tree
//
// Both are imported here so a blank import of this package wires the
// full CLI surface without touching the core dispatcher.
package l2tp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"

	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/l2tp/schema" // register ze-l2tp-cmd.yang
	_ "codeberg.org/thomas-mangin/ze/internal/component/l2tp/schema"     // register ze-l2tp-api.yang
)

// errSubsystemUnavailable is returned when any show/teardown command
// runs while the L2TP subsystem has not been started (or has been
// stopped). The handler converts it into a plugin.StatusError response
// so the CLI prints a clear message.
var errSubsystemUnavailable = errors.New("l2tp: subsystem not running")

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:summary", Handler: handleSummary},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:tunnels", Handler: handleTunnels},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:tunnel", Handler: handleTunnel},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:sessions", Handler: handleSessions},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:session", Handler: handleSession},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:statistics", Handler: handleStatistics},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:listeners", Handler: handleListeners},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:config", Handler: handleConfig},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:tunnel-teardown", Handler: handleTunnelTeardown},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:tunnel-teardown-all", Handler: handleTunnelTeardownAll},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:session-teardown", Handler: handleSessionTeardown},
		pluginserver.RPCRegistration{WireMethod: "ze-l2tp-api:session-teardown-all", Handler: handleSessionTeardownAll},
	)
}

// -----------------------------------------------------------------
// Read handlers
// -----------------------------------------------------------------

// handleSummary returns aggregate counters.
func handleSummary(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
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
func handleTunnels(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
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

// handleTunnel returns one tunnel by ID.
func handleTunnel(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	tid, err := parseIDArg(args, "tunnel-id")
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
func handleSessions(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
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
func handleSession(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	sid, err := parseIDArg(args, "session-id")
	if err != nil {
		return errResponse(err), nil
	}
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	ss, ok := svc.LookupSession(sid)
	if !ok {
		return errResponse(fmt.Errorf("l2tp: no session with local-sid=%d", sid)), nil
	}
	return jsonResponse("l2tp session", sessionJSON(&ss, true))
}

// handleStatistics returns protocol counters. spec-l2tp-10 will add
// per-message counters; spec-l2tp-7 returns the basic aggregates
// already available.
func handleStatistics(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
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

// handleListeners returns the bound UDP endpoints.
func handleListeners(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	lns := svc.Listeners()
	out := make([]map[string]any, 0, len(lns))
	for _, ln := range lns {
		out = append(out, map[string]any{
			"address": ln.Addr.Addr().String(),
			"port":    int(ln.Addr.Port()),
		})
	}
	return jsonResponse("l2tp listeners", out)
}

// handleConfig returns the effective runtime configuration.
func handleConfig(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
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
		"enabled":        cs.Enabled,
		"max-tunnels":    int(cs.MaxTunnels),
		"max-sessions":   int(cs.MaxSessions),
		"hello-interval": int(cs.HelloInterval.Seconds()),
		"shared-secret":  cs.SharedSecret,
		"listeners":      listenAddrs,
	}
	return jsonResponse("l2tp config", payload)
}

// -----------------------------------------------------------------
// Destructive handlers
// -----------------------------------------------------------------

func handleTunnelTeardown(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	tid, err := parseIDArg(args, "tunnel-id")
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
		"action":    "tunnel-teardown",
		"tunnel-id": int(tid),
		"status":    "sent",
	})
}

func handleTunnelTeardownAll(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	n := svc.TeardownAllTunnels()
	return jsonResponse("l2tp tunnel teardown-all", map[string]any{
		"action":          "tunnel-teardown-all",
		"tunnels-cleared": n,
	})
}

func handleSessionTeardown(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	sid, err := parseIDArg(args, "session-id")
	if err != nil {
		return errResponse(err), nil
	}
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	actor, reason, cause := parseKeywordArgs(args)
	if reason != "" || cause != 0 || actor != "" {
		if _, exists := svc.LookupSession(sid); exists {
			svc.RecordDisconnect(sid, actor, reason, cause)
		}
	}
	if err := svc.TeardownSession(sid); err != nil {
		return errResponse(err), nil
	}
	result := map[string]any{
		"action":     "session-teardown",
		"session-id": int(sid),
		"status":     "sent",
	}
	if reason != "" {
		result["reason"] = reason
	}
	if cause != 0 {
		result["cause"] = int(cause)
	}
	return jsonResponse("l2tp session teardown", result)
}

func handleSessionTeardownAll(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	n := svc.TeardownAllSessions()
	return jsonResponse("l2tp session teardown-all", map[string]any{
		"action":           "session-teardown-all",
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
		"state":         t.State,
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
		"state":            s.State,
		"username":         s.Username,
		"assigned-addr":    assigned,
		"family":           s.Family,
		"created-at":       s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if detail {
		m["tx-connect-speed"] = int64(s.TxConnectSpeed)
		m["rx-connect-speed"] = int64(s.RxConnectSpeed)
		m["framing-type"] = l2tp.FormatFraming(s.FramingType)
		m["sequencing-required"] = s.SequencingRequired
		m["lns-mode"] = s.LNSMode
		m["kernel-setup-needed"] = s.KernelSetupNeeded
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
func parseKeywordArgs(args []string) (actor, reason string, cause uint32) {
	const (
		kwActor  = "actor"
		kwReason = "reason"
		kwCause  = "cause"
	)
	// Skip the first positional arg (the ID).
	started := false
	var reasonParts []string
	collecting := ""
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		if !started {
			started = true
			continue
		}
		switch {
		case a == kwActor:
			collecting = kwActor
		case a == kwReason:
			collecting = kwReason
		case a == kwCause:
			collecting = kwCause
		case collecting == kwActor:
			actor = a
			collecting = ""
		case collecting == kwReason:
			reasonParts = append(reasonParts, a)
		case collecting == kwCause:
			if n, err := strconv.ParseUint(a, 10, 32); err == nil {
				cause = uint32(n)
			}
			collecting = ""
		}
	}
	reason = strings.Join(reasonParts, " ")
	return actor, reason, cause
}

// jsonResponse marshals payload into a plugin.StatusDone response.
// Returns the marshal error as a transport-level error so the caller
// surfaces it to the CLI.
func jsonResponse(op string, payload any) (*plugin.Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal: %w", op, err)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
}

// errResponse wraps err into a plugin.StatusError response.
func errResponse(err error) *plugin.Response {
	return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}
}
