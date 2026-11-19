// Design: docs/guide/l2tp.md -- independent native subscriber route contributions
// Related: route_observer.go -- original subscriber records retain their contribution token

package l2tp

import (
	"fmt"
	"sync"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	codeL2TPFramedRouteLimit  = "doctor-l2tp-framed-route-limit"
	framedRouteLimitFix       = "ze show l2tp sessions"
	codePPPoEFramedRouteLimit = "doctor-pppoe-framed-route-limit"
	pppoeFramedRouteLimitFix  = "ze show pppoe sessions"
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: codeL2TPFramedRouteLimit, Title: "L2TP framed-route update exceeds its limit",
		Description: fmt.Sprintf("Ze observed more than %d valid framed routes in a RADIUS authorization. The whole route update was refused. Correct the subscriber's RADIUS profile, inspect the current session coordinates, then reconnect that subscriber.", l2tpevents.FramedRoutesMax),
		Examples:    []string{framedRouteLimitFix, "ze show diagnostic " + codeL2TPFramedRouteLimit},
	}); err != nil {
		panic("BUG: L2TP framed-route diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeL2TPFramedRouteLimit}); err != nil {
		panic("BUG: L2TP framed-route event registration failed: " + err.Error())
	}
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: codePPPoEFramedRouteLimit, Title: "PPPoE framed-route update exceeds its limit",
		Description: fmt.Sprintf("Ze observed more than %d valid framed routes in a RADIUS authorization. The whole route update was refused. Correct the subscriber's RADIUS profile, inspect the current session, then reconnect that subscriber.", l2tpevents.FramedRoutesMax),
		Examples:    []string{pppoeFramedRouteLimitFix, "ze show diagnostic " + codePPPoEFramedRouteLimit},
	}); err != nil {
		panic("BUG: PPPoE framed-route diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codePPPoEFramedRouteLimit}); err != nil {
		panic("BUG: PPPoE framed-route event registration failed: " + err.Error())
	}
}

// routeSourceSequence allocates a separate native contribution token once per
// route lifetime. It neither narrows a Birth nor retains identity history.
// Tokens survive add/replay/remove and are never recycled within this process.
// Safe for concurrent use; the cold-path mutex protects exhaustion and increment.
type routeSourceSequence struct {
	mu   sync.Mutex
	last uint32
}

var subscriberRouteSources routeSourceSequence

var errRouteSourceExhausted = opererr.New(
	"Subscriber redistribution source identities are exhausted",
	"Ze refused another route contribution instead of reusing an existing source identity",
	"sudo systemctl restart ze")

func (source *routeSourceSequence) next() (uint32, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.last == ^uint32(0) {
		return 0, errRouteSourceExhausted
	}
	source.last++
	return source.last, nil
}

// reportSourceRefusal MUST run after releasing the observer's map lock.
// The allocator only returns a cause; it MUST NOT call reporters under that lock.
func (o *subscriberRouteObserver) reportSourceRefusal(session subscriber.Session, cause error) {
	label, value := routeDiagnosticAttachment(session)
	o.logger.Warn(string(session.AccessType)+": subscriber route contribution refused",
		"code", report.CodeInternalDefect,
		"subscriber-id", session.ID, label, value, "session-id", routeSessionLabel(session),
		"error", cause, "fix", errRouteSourceExhausted.Fix())
	report.RaiseWarningFix(string(session.AccessType), report.CodeInternalDefect, "subscriber route source",
		errRouteSourceExhausted.Problem()+": "+errRouteSourceExhausted.Why(), errRouteSourceExhausted.Fix(),
		map[string]any{"subscriber-id": session.ID, label: value, "session-id": routeSessionLabel(session), "cause": cause.Error()})
}

// reportFramedRouteRefusal MUST run without mu or emitMu held. The caller
// refuses before any route mutation; reporting does not own cleanup or retry.
// Safe for concurrent use; the bounded error ring retains immutable context.
func (o *subscriberRouteObserver) reportFramedRouteRefusal(session subscriber.Session, count int) {
	label, value := routeDiagnosticAttachment(session)
	location := fmt.Sprintf("L2TP routes refused for subscriber %s (tunnel %d, session %d)", session.ID, session.TunnelID, session.SessionID)
	fix := framedRouteLimitFix
	if session.AccessType == subscriber.AccessPPPoE {
		location = fmt.Sprintf("PPPoE routes refused for subscriber %s (interface index %d, session %d)", session.ID, session.AccessIfIndex, session.PPPoESID)
		fix = pppoeFramedRouteLimitFix
	}
	cause := fmt.Errorf("Ze observed at least %d valid RADIUS framed routes. Maximum is %d", count, l2tpevents.FramedRoutesMax)
	failure := opererr.Wrap(cause,
		location,
		fmt.Sprintf("The oversized update was not applied. Reduce this subscriber's RADIUS framed-route list to at most %d entries. Inspect the current session and listener before reconnecting the affected subscriber", l2tpevents.FramedRoutesMax),
		fix)
	sid := routeSessionLabel(session)
	detail := map[string]any{
		"subscriber-id": session.ID, label: value, "session-id": sid,
		"cause": cause.Error(), "framed-routes-at-least": count, "limit": l2tpevents.FramedRoutesMax,
	}
	message := report.FitMessage(failure.Problem() + ". " + failure.Why() + ": " + cause.Error())
	if session.AccessType == subscriber.AccessPPPoE {
		o.logger.Warn(failure.Problem(), "code", codePPPoEFramedRouteLimit,
			"subscriber-id", session.ID, label, value, "session-id", sid,
			"why", failure.Why(), "error", cause, "fix", failure.Fix())
		report.RaiseErrorFix(string(subscriber.AccessPPPoE), codePPPoEFramedRouteLimit, session.ID, message, failure.Fix(), detail)
		return
	}
	o.logger.Warn(failure.Problem(), "code", codeL2TPFramedRouteLimit,
		"subscriber-id", session.ID, label, value, "session-id", sid,
		"why", failure.Why(), "error", cause, "fix", failure.Fix())
	report.RaiseErrorFix(SubsystemName, codeL2TPFramedRouteLimit, session.ID, message, failure.Fix(), detail)
}

func routeDiagnosticAttachment(session subscriber.Session) (string, any) {
	if session.AccessType == subscriber.AccessPPPoE {
		return "access-ifindex", session.AccessIfIndex
	}
	return "tunnel-id", session.TunnelID
}
