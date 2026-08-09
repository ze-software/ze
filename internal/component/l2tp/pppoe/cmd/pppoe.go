// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PPPoE CLI handlers
//
// Package pppoe registers engine-side RPC handlers that expose the PPPoE
// subsystem's observability surface to the CLI. Handlers reach the
// subsystem through pppoe.LookupService() rather than crossing a plugin pipe.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	_ "github.com/ze-software/ze/internal/component/l2tp/pppoe/yang" // register ze-pppoe-api.yang
)

var (
	errPppoeMissingSessionIdArgument  = errors.New("pppoe: missing session-id argument")
	errPppoeInvalidSessionId0Reserved = errors.New("pppoe: invalid session-id 0 (reserved by RFC 2516)")
)

var errSubsystemUnavailable = errors.New("pppoe: subsystem not running")

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-pppoe-api:summary", Handler: handleSummary},
		pluginserver.RPCRegistration{WireMethod: "ze-pppoe-api:sessions", Handler: handleSessions},
		pluginserver.RPCRegistration{WireMethod: "ze-pppoe-api:session", Handler: handleSession},
		pluginserver.RPCRegistration{WireMethod: "ze-pppoe-api:statistics", Handler: handleStatistics},
		pluginserver.RPCRegistration{WireMethod: "ze-pppoe-api:interfaces", Handler: handleInterfaces},
	)
}

func handleSummary(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := pppoe.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	snap := svc.Snapshot()
	payload := map[string]any{
		"session-count":   snap.SessionCount,
		"interface-count": snap.InterfaceCount,
		"captured-at":     snap.CapturedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	return jsonResponse("pppoe summary", payload)
}

func handleSessions(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := pppoe.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	snap := svc.Snapshot()
	out := make([]map[string]any, 0, len(snap.Sessions))
	for i := range snap.Sessions {
		out = append(out, sessionJSON(&snap.Sessions[i]))
	}
	return jsonResponse("pppoe sessions", out)
}

func handleSession(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	sid, err := parseIDArg(args)
	if err != nil {
		return errResponse(err), nil
	}
	svc := pppoe.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	ss, ok := svc.LookupSession(sid)
	if !ok {
		return errResponse(fmt.Errorf("pppoe: no session with sid=%d", sid)), nil
	}
	return jsonResponse("pppoe session", sessionJSON(&ss))
}

func handleStatistics(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := pppoe.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	snap := svc.Snapshot()
	ifaces := make([]map[string]any, 0, len(snap.Interfaces))
	for i := range snap.Interfaces {
		ifaces = append(ifaces, map[string]any{
			"interface":    snap.Interfaces[i].Name,
			"sessions":     snap.Interfaces[i].SessionCount,
			"max-sessions": snap.Interfaces[i].MaxSessions,
		})
	}
	payload := map[string]any{
		"sessions-active": snap.SessionCount,
		"interfaces":      ifaces,
	}
	return jsonResponse("pppoe statistics", payload)
}

func handleInterfaces(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := pppoe.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}
	snap := svc.Snapshot()
	out := make([]map[string]any, 0, len(snap.Interfaces))
	for i := range snap.Interfaces {
		entry := map[string]any{
			"name":          snap.Interfaces[i].Name,
			"ifindex":       snap.Interfaces[i].IfIndex,
			"sessions":      snap.Interfaces[i].SessionCount,
			"max-sessions":  snap.Interfaces[i].MaxSessions,
			"service-names": snap.Interfaces[i].ServiceNames,
		}
		out = append(out, entry)
	}
	return jsonResponse("pppoe interfaces", out)
}

func sessionJSON(s *pppoe.SessionSnapshot) map[string]any {
	var state string
	switch s.State {
	case pppoe.StateDiscovery:
		state = "discovery"
	case pppoe.StateSession:
		state = "session"
	case pppoe.StateTeardown:
		state = "teardown"
	}
	return map[string]any{
		"sid":          s.SID,
		"mac":          s.MAC.String(),
		"interface":    s.IfName,
		"service-name": s.ServiceName,
		"state":        state,
		"unit":         s.UnitNum,
		"created-at":   s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func parseIDArg(args []string) (uint16, error) {
	var raw string
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		raw = a
		break
	}
	if raw == "" {
		return 0, errPppoeMissingSessionIdArgument
	}
	n, err := strconv.ParseUint(raw, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("pppoe: invalid session-id %q: %w", raw, err)
	}
	if n == 0 {
		return 0, errPppoeInvalidSessionId0Reserved
	}
	return uint16(n), nil
}

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

func errResponse(err error) *plugin.Response {
	return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}
}
