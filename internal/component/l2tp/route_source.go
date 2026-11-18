// Design: docs/guide/l2tp.md -- independent native subscriber route contributions
// Related: route_observer.go -- full birth records retain their contribution token

package l2tp

import (
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

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
