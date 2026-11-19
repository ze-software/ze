// Design: docs/guide/l2tp.md -- guarded original-session teardown

package cmd

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

const (
	argExpectedID = "expected-id"
	argListener   = "listener"
)

// sessionTeardownRequest is parsed cold input, not mutation authority. Its
// private zero value is invalid; only a successful parser supplies its SID.
// A listener name remains a cold alias until resolveSelector binds an endpoint.
type sessionTeardownRequest struct {
	selector l2tp.SessionSelector
	listener string
	disconnectAudit
}

// disconnectAudit is the optional audit a clear records against the captured
// original before its teardown. Its zero value records nothing.
type disconnectAudit struct {
	actor  string
	reason string
	cause  uint32
}

func parseSessionTeardownRequest(args []string) (sessionTeardownRequest, *opererr.Error) {
	var request sessionTeardownRequest
	if len(args) == 0 {
		return request, sessionTeardownInputError(nil, "L2TP session ID is required")
	}
	sid, err := parseTeardownID(args[0], argSessionID)
	if err != nil {
		return request, sessionTeardownInputError(err, "L2TP session ID is invalid")
	}
	request.selector.SID = sid
	audit := []string{args[0]}
	seen := make(map[string]bool, 3)
	// Each iteration consumes a keyword/value pair or reason words. The input
	// token count bounds both walks; no session is selected during parsing.
	for index := 1; index < len(args); {
		keyword := args[index]
		index++
		if !isSessionTeardownKeyword(keyword) {
			return sessionTeardownRequest{}, sessionTeardownInputError(nil, fmt.Sprintf("Unknown L2TP disconnect keyword %q", keyword))
		}
		if index == len(args) {
			return sessionTeardownRequest{}, sessionTeardownInputError(nil, fmt.Sprintf("L2TP disconnect keyword %q requires a value", keyword))
		}
		if keyword != argListener {
			if isSessionTeardownKeyword(args[index]) {
				return sessionTeardownRequest{}, sessionTeardownInputError(nil, fmt.Sprintf("L2TP disconnect keyword %q requires a value", keyword))
			}
		}
		if isDisconnectAuditKeyword(keyword) {
			audit = append(audit, keyword, args[index])
			index++
			if keyword == kwReason {
				for index < len(args) {
					if isSessionTeardownKeyword(args[index]) {
						break
					}
					audit = append(audit, args[index])
					index++
				}
			}
			continue
		}
		if seen[keyword] {
			return sessionTeardownRequest{}, sessionTeardownInputError(nil, fmt.Sprintf("Duplicate L2TP disconnect keyword %q", keyword))
		}
		seen[keyword] = true
		value := args[index]
		index++
		switch keyword {
		case argExpectedID:
			request.selector.Birth, err = subscriberid.ParseL2TPBirth(value)
		case argTunnelID:
			request.selector.TID, err = parseTeardownID(value, argTunnelID)
		case argListener:
			if value == "" {
				return sessionTeardownRequest{}, sessionTeardownListenerError(nil, "L2TP listener is empty")
			}
			if utf8.RuneCountInString(value) > 253 {
				return sessionTeardownRequest{}, sessionTeardownListenerError(nil, "L2TP listener is longer than 253 characters")
			}
			if strings.HasPrefix(value, "-") {
				return sessionTeardownRequest{}, sessionTeardownListenerError(nil, fmt.Sprintf("L2TP listener %q is an option, not a listener", value))
			}
			request.listener = value
		}
		if err != nil {
			return sessionTeardownRequest{}, sessionTeardownInputError(err, fmt.Sprintf("L2TP disconnect value for %q is invalid", keyword))
		}
	}
	request.actor, request.reason, request.cause, err = parseKeywordArgs(audit)
	if err != nil {
		return sessionTeardownRequest{}, sessionTeardownInputError(err, "L2TP disconnect details are invalid")
	}
	return request, nil
}

func parseTeardownID(value, field string) (uint16, error) {
	number, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", field, value, err)
	}
	if number == 0 {
		return 0, fmt.Errorf("%s zero is reserved by RFC 2661", field)
	}
	return uint16(number), nil
}

func isSessionTeardownKeyword(value string) bool {
	if isDisconnectAuditKeyword(value) {
		return true
	}
	return value == argExpectedID || value == argTunnelID || value == argListener
}

func isDisconnectAuditKeyword(value string) bool {
	return value == kwActor || value == kwReason || value == kwCause
}

// RFC 7950 Section 9.4.4: "A \"length\" statement restricts the number of
// Unicode characters in the string." The HTTP reason limit uses the same unit.
func validateDisconnectText(keyword, value string) error {
	if value == "" {
		return fmt.Errorf("disconnect %s is empty", keyword)
	}
	if utf8.RuneCountInString(value) > 256 {
		return fmt.Errorf("disconnect %s is longer than 256 characters", keyword)
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("disconnect %s %q is an option, not a value", keyword, value)
	}
	return nil
}

func (request sessionTeardownRequest) resolveSelector(service l2tp.Service) (l2tp.SessionSelector, *opererr.Error) {
	selector := request.selector
	if request.listener == "" {
		return selector, nil
	}
	endpoint, err := netip.ParseAddrPort(request.listener)
	if err == nil {
		if endpoint.Port() == 0 {
			return l2tp.SessionSelector{}, sessionTeardownListenerError(nil, "L2TP listener port is zero")
		}
		selector.Listener = endpoint
		return selector, nil
	}
	found := false
	for _, listener := range service.Listeners() {
		if listener.Name != request.listener {
			continue
		}
		if !listener.Addr.IsValid() {
			return l2tp.SessionSelector{}, sessionTeardownListenerError(nil, fmt.Sprintf("L2TP listener %q has no bound endpoint", request.listener))
		}
		if found {
			if selector.Listener != listener.Addr {
				return l2tp.SessionSelector{}, sessionTeardownListenerError(nil, fmt.Sprintf("L2TP listener name %q is ambiguous", request.listener))
			}
		}
		found = true
		selector.Listener = listener.Addr
	}
	if !found {
		return l2tp.SessionSelector{}, sessionTeardownListenerError(err, fmt.Sprintf("L2TP listener %q is unknown", request.listener))
	}
	return selector, nil
}

func (audit disconnectAudit) hasAudit() bool {
	if audit.actor != "" {
		return true
	}
	if audit.reason != "" {
		return true
	}
	return audit.cause != 0
}

// finishSessionTeardown records the audit against one captured original and
// tears it down. It answers the refusal to send, or nil once the original's
// local physical cleanup is confirmed. Every clear form ends here, so the id,
// user and address forms share one audit and one teardown path.
func finishSessionTeardown(ctx *pluginserver.CommandContext, captured l2tp.CapturedSession, sid uint16, audit disconnectAudit) *plugin.Response {
	if err := ctx.Context().Err(); err != nil {
		return sessionTeardownFailure(sid, err)
	}
	if audit.hasAudit() {
		if err := captured.RecordDisconnect(audit.actor, audit.reason, audit.cause); err != nil {
			return sessionTeardownFailure(sid, err)
		}
	}
	if err := captured.Teardown(ctx.Context(), l2tpevents.TerminateCauseAdminReset, audit.reason); err != nil {
		return sessionTeardownCleanupFailure(sid, captured.Birth(), err)
	}
	return nil
}

// sessionTeardownResult is the answer of a confirmed clear, whichever form
// selected the session. reason and cause appear only when the request had them.
func sessionTeardownResult(sid uint16, audit disconnectAudit) map[string]any {
	result := map[string]any{
		keyAction:    "session-teardown",
		keySessionID: int(sid),
		"status":     "sent",
	}
	if audit.reason != "" {
		result["reason"] = audit.reason
	}
	if audit.cause != 0 {
		result["cause"] = audit.cause
	}
	return result
}

func sessionTeardownInputError(cause error, problem string) *opererr.Error {
	return opererr.Wrap(cause, problem,
		"The request must identify one original session before it can be cleared",
		"show l2tp sessions")
}

func sessionTeardownListenerError(cause error, problem string) *opererr.Error {
	return opererr.Wrap(cause, problem,
		"The listener must identify one original bound endpoint before selection",
		"show l2tp listeners")
}

func sessionTeardownFailure(sid uint16, cause error) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.Wrap(cause,
		fmt.Sprintf("L2TP session %d was not cleared", sid),
		"The original session could not be selected or is no longer available",
		"show l2tp sessions"))
}

func sessionTeardownNotFound(sid uint16, cause error) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.Wrap(cause,
		fmt.Sprintf("L2TP session %d was not cleared", sid),
		"No L2TP session matches this selector",
		"show l2tp sessions"))
}

func sessionTeardownAmbiguity(service l2tp.Service, selector l2tp.SessionSelector, cause error) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.Wrap(cause,
		fmt.Sprintf("L2TP session ID %d is ambiguous", selector.SID),
		sessionSelectorCandidates(service, selector, "clear"),
		"show l2tp sessions"))
}

func sessionTeardownCleanupFailure(sid uint16, birth subscriberid.L2TPBirth, cause error) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.Wrap(cause,
		fmt.Sprintf("L2TP session %d cleanup is not confirmed", sid),
		"Retry the same original session reference to finish its retained cleanup",
		fmt.Sprintf("clear l2tp session id %d expected-id %s", sid, birth.String())))
}
