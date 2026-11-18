// Design: docs/guide/l2tp.md -- independent native subscriber route contributions
// Related: route_observer.go -- full birth records retain their contribution token

package l2tp

import (
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

const (
	framedRoutesMax          = 64
	codeL2TPFramedRouteLimit = "doctor-l2tp-framed-route-limit"
	framedRouteLimitFix      = "ze show l2tp sessions"
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: codeL2TPFramedRouteLimit, Title: "L2TP framed-route update exceeds its limit",
		Description: fmt.Sprintf("Ze observed more than %d valid framed routes in a RADIUS authorization. The whole route update was refused. Correct the subscriber's RADIUS profile, inspect the current session coordinates, then reconnect that subscriber.", framedRoutesMax),
		Examples:    []string{framedRouteLimitFix, "ze show diagnostic " + codeL2TPFramedRouteLimit},
	}); err != nil {
		panic("BUG: L2TP framed-route diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeL2TPFramedRouteLimit}); err != nil {
		panic("BUG: L2TP framed-route event registration failed: " + err.Error())
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
	"L2TP redistribution source identities are exhausted",
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
func (o *subscriberRouteObserver) reportSourceRefusal(birth subscriberid.L2TPBirth, tunnelID, sessionID uint16, cause error) {
	id := birth.String() // human/report boundary, never an internal lookup key
	o.logger.Warn("l2tp: subscriber route contribution refused",
		"code", report.CodeInternalDefect,
		"subscriber-id", id, "tunnel-id", tunnelID, "session-id", sessionID,
		"error", cause, "fix", errRouteSourceExhausted.Fix())
	report.RaiseWarningFix(SubsystemName, report.CodeInternalDefect, "subscriber route source",
		errRouteSourceExhausted.Problem()+": "+errRouteSourceExhausted.Why(), errRouteSourceExhausted.Fix(),
		map[string]any{"subscriber-id": id, "tunnel-id": tunnelID, "session-id": sessionID, "cause": cause.Error()})
}

// reportFramedRouteRefusal MUST run without mu or emitMu held. The caller
// refuses before any route mutation; reporting does not own cleanup or retry.
// Safe for concurrent use; the bounded error ring retains immutable context.
func (o *subscriberRouteObserver) reportFramedRouteRefusal(birth subscriberid.L2TPBirth, tunnelID, sessionID uint16, count int) {
	id := birth.String() // human/report boundary, never an internal lookup key
	cause := fmt.Errorf("Ze observed at least %d valid RADIUS framed routes. Maximum is %d", count, framedRoutesMax)
	failure := opererr.Wrap(cause,
		fmt.Sprintf("L2TP routes refused for subscriber %s (tunnel %d, session %d)", id, tunnelID, sessionID),
		fmt.Sprintf("The oversized update was not applied. Reduce this subscriber's RADIUS framed-route list to at most %d entries. Inspect the current session and listener before reconnecting the affected subscriber", framedRoutesMax),
		framedRouteLimitFix)
	o.logger.Warn(failure.Problem(),
		"code", codeL2TPFramedRouteLimit,
		"subscriber-id", id, "tunnel-id", tunnelID, "session-id", sessionID,
		"why", failure.Why(), "error", cause, "fix", failure.Fix())
	report.RaiseErrorFix(SubsystemName, codeL2TPFramedRouteLimit, id,
		report.FitMessage(failure.Problem()+". "+failure.Why()+": "+cause.Error()), failure.Fix(),
		map[string]any{
			"subscriber-id": id, "tunnel-id": tunnelID, "session-id": sessionID,
			"cause": cause.Error(), "framed-routes-at-least": count, "limit": framedRoutesMax,
		})
}
