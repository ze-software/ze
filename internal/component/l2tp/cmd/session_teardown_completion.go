// Design: docs/guide/l2tp.md -- guarded-session runtime argument completion

package cmd

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/l2tp"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// completeSessionTeardown reads cold inventories only. Names are offered at
// the listener-value position, never as generic session arguments. The caller
// retains its slice; each walk is bounded by completed or the runtime snapshot.
func completeSessionTeardown(_ *pluginserver.Server, completed []string) []pluginserver.Completion {
	service := l2tp.LookupService()
	if service == nil {
		return nil
	}
	if len(completed) == 0 {
		return completeSessionTeardownIDs(service)
	}
	last := completed[len(completed)-1]
	if last == argListener {
		var candidates []pluginserver.Completion
		seen := make(map[string]bool)
		for _, listener := range service.Listeners() {
			if listener.Name == "" {
				continue
			}
			if seen[listener.Name] {
				continue
			}
			seen[listener.Name] = true
			candidates = append(candidates, pluginserver.Completion{Value: listener.Name, ShortHelp: "Original bound L2TP listener."})
		}
		return candidates
	}
	if isSessionTeardownKeyword(last) {
		return nil
	}
	seen := make(map[string]bool, len(completed))
	for _, token := range completed {
		seen[token] = true
	}
	var candidates []pluginserver.Completion
	for _, candidate := range []pluginserver.Completion{
		{Value: argExpectedID, ShortHelp: "Require the displayed immutable session reference."},
		{Value: argTunnelID, ShortHelp: "Select the original local tunnel."},
		{Value: argListener, ShortHelp: "Select a configured listener name or bound IP:port."},
		{Value: kwActor, ShortHelp: "Record the disconnect request's actor label."},
		{Value: kwReason, ShortHelp: "Record the disconnect reason."},
		{Value: kwCause, ShortHelp: "Record the RADIUS disconnect cause."},
	} {
		if seen[candidate.Value] {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func completeSessionTeardownIDs(service l2tp.Service) []pluginserver.Completion {
	seen := make(map[uint16]bool)
	var candidates []pluginserver.Completion
	snapshot := service.Snapshot()
	for tunnelIndex := range snapshot.Tunnels {
		tunnel := &snapshot.Tunnels[tunnelIndex]
		for sessionIndex := range tunnel.Sessions {
			session := &tunnel.Sessions[sessionIndex]
			if session.LocalSID == 0 {
				continue
			}
			if seen[session.LocalSID] {
				continue
			}
			seen[session.LocalSID] = true
			candidates = append(candidates, pluginserver.Completion{Value: strconv.FormatUint(uint64(session.LocalSID), 10), ShortHelp: "Local session ID; qualify an ambiguous original."})
		}
	}
	return candidates
}
