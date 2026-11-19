// Design: docs/guide/l2tp.md -- qualified original-session reads.
// Related: session_teardown.go -- the shared selector parser and listener inventory.
package cmd

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

// captureSessionRead parses cold input through the existing guarded selector
// parser, then captures exactly one original. It grants no additional authority.
// The token count bounds the read-only keyword check; audit keywords are never
// accepted on a read. The caller MUST read through this capture, never an SID.
func captureSessionRead(service l2tp.Service, arguments []string) (l2tp.CapturedSession, uint16, *opererr.Error) {
	for index := 1; index < len(arguments); index += 2 {
		keyword := arguments[index]
		if keyword == argExpectedID {
			continue
		}
		if keyword == argTunnelID {
			continue
		}
		if keyword == argListener {
			continue
		}
		return nil, 0, sessionReadError(nil, fmt.Sprintf("L2TP session read keyword %q is unknown", keyword))
	}
	request, refusal := parseSessionTeardownRequest(arguments)
	if refusal != nil {
		return nil, 0, sessionReadError(refusal, "L2TP session read selector is invalid")
	}
	selector, refusal := request.resolveSelector(service)
	if refusal != nil {
		return nil, 0, refusal
	}
	captured, err := service.CaptureSession(selector)
	if err != nil {
		if errors.Is(err, l2tp.ErrSessionAmbiguous) {
			return nil, 0, opererr.Wrap(err, "L2TP session selector is ambiguous",
				sessionSelectorCandidates(service, selector, "show"), "show l2tp sessions")
		}
		return nil, 0, sessionReadError(err, "The L2TP session could not be selected")
	}
	return captured, selector.SID, nil
}

func sessionReadError(cause error, problem string) *opererr.Error {
	return opererr.Wrap(cause, problem,
		"Select one current original session using its session ID, tunnel ID and listener",
		"show l2tp sessions")
}

// sessionSelectorCandidates describes a fresh inventory only after capture refused.
// This snapshot is diagnostic input, never a second selection or authorization.
// Every loop is bounded by the existing live session inventory.
func sessionSelectorCandidates(service l2tp.Service, selector l2tp.SessionSelector, verb string) string {
	var candidates []string
	snapshot := service.Snapshot()
	for tunnelIndex := range snapshot.Tunnels {
		tunnel := &snapshot.Tunnels[tunnelIndex]
		for sessionIndex := range tunnel.Sessions {
			session := &tunnel.Sessions[sessionIndex]
			if session.LocalSID != selector.SID {
				continue
			}
			if selector.TID != 0 {
				if session.TunnelLocalTID != selector.TID {
					continue
				}
			}
			if selector.Listener.IsValid() {
				if session.Listener != selector.Listener {
					continue
				}
			}
			if !session.Listener.IsValid() {
				continue
			}
			candidate := fmt.Sprintf("%s l2tp session id %d tunnel-id %d listener %s", verb, session.LocalSID, session.TunnelLocalTID, session.Listener)
			if session.Birth.IsValid() {
				candidate += " expected-id " + session.Birth.String()
			}
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return "The readable coordinates match multiple originals; inspect current sessions and listeners before retrying"
	}
	return "Qualify the current original with one of: " + strings.Join(candidates, "; ")
}

// completeSessionRead offers only cold read qualifiers, using the same original
// listener and ID inventories as guarded clear. No completion selects a target.
// Walks are bounded by completed tokens and the existing inventory.
func completeSessionRead(server *pluginserver.Server, completed []string) []pluginserver.Completion {
	if len(completed) == 0 {
		return completeSessionTeardown(server, nil)
	}
	// SID occupies token zero; each complete selector consumes one pair.
	// An even count ends at a keyword waiting for its value, regardless of
	// whether an earlier listener value happens to spell another keyword.
	if len(completed)%2 == 0 {
		if completed[len(completed)-1] == argListener {
			return completeSessionTeardown(server, completed)
		}
		return nil
	}
	candidates := completeSessionTeardown(server, completed[:1])
	seen := make(map[string]bool, len(completed)/2)
	for index := 1; index < len(completed); index += 2 {
		seen[completed[index]] = true
	}
	selected := candidates[:0]
	for _, candidate := range candidates {
		if isDisconnectAuditKeyword(candidate.Value) {
			continue
		}
		if seen[candidate.Value] {
			continue
		}
		selected = append(selected, candidate)
	}
	return selected
}

// A diagnostic counter's zero value is Unknown. JSON preserves that absence
// as null; a measured zero remains a number. Value has only this known refusal.
func sessionCounterJSON(reading ppp.CounterReading) any {
	value, err := reading.Value()
	if err != nil {
		return nil
	}
	return value
}

// An absent original endpoint remains Unknown at the JSON boundary; it must
// never become the text of netip's invalid zero value.
func originalReadListener(listener netip.AddrPort) any {
	if !listener.IsValid() {
		return nil
	}
	return listener.String()
}

// Observer events retain the captured Birth's original wire labels. An empty
// history has no measured label to publish; do not invent a tunnel from an SID.
func observerReadTunnel(events []l2tp.ObserverEvent) any {
	if len(events) == 0 {
		return nil
	}
	return int(events[0].TunnelID)
}
