// Design: docs/guide/plugins.md -- installed-route repair replay.
// Related: fibkernel.go -- processEvent and the last successful forwarding state.

package fibkernel

import (
	"slices"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
)

// replayRouteBackend optionally reasserts an unchanged, previously installed
// route after checking current kernel ownership. An absent interface selects the
// ordinary replacement path; implementations require an initialized, nonnil
// backend, including when held by an interface.
//
// handled=false with nil error requires the caller to use ordinary replacement.
// handled=true reports an actual reassertion attempt, whose error MUST reach the
// ordinary install-failure path. A successful probe alone is not a reassertion.
type replayRouteBackend interface {
	reassertRichRoute(r RichRoute) (handled bool, err error)
}

// tryReplay reports whether the installed-route replay path handled this entry.
// The caller holds f.mu and MUST preserve ordinary replacement on false, nil.
func (f *fibKernel) tryReplay(c *incomingChange, pfx string) (bool, error) {
	backend, capable := f.backend.(replayRouteBackend)
	if !capable {
		return false, nil
	}
	if _, installed := f.installed[pfx]; !installed {
		return false, nil
	}
	if f.retained[pfx] {
		return false, nil
	}
	if _, pending := f.pending[pfx]; pending {
		return false, nil
	}
	previous, known := f.forwarding[pfx]
	if !known {
		return false, nil
	}
	route := changeToRichRoute(c)
	if !sameReplayRoute(route, changeToRichRoute(&previous)) {
		return false, nil
	}
	handled, err := backend.reassertRichRoute(route)
	if err != nil {
		// A failed backend attempt MUST never become an ordinary replacement.
		return true, err
	}
	return handled, nil
}

func sameReplayRoute(a, b RichRoute) bool {
	return a.Prefix == b.Prefix &&
		a.NextHop == b.NextHop &&
		a.Interface == b.Interface &&
		a.OnLink == b.OnLink &&
		a.Weight == b.Weight &&
		a.RouteType == b.RouteType &&
		a.Metric == b.Metric &&
		a.TableID == b.TableID &&
		a.PathMTU == b.PathMTU &&
		a.SRv6SID == b.SRv6SID &&
		slices.Equal(a.Labels, b.Labels) &&
		slices.EqualFunc(a.ECMPPaths, b.ECMPPaths, sameReplayPath) &&
		slices.EqualFunc(a.Backup, b.Backup, sameReplayPath)
}

func sameReplayPath(a, b sysribevents.ECMPPath) bool {
	return a.NextHop == b.NextHop &&
		a.Interface == b.Interface &&
		a.OnLink == b.OnLink &&
		a.Weight == b.Weight &&
		slices.Equal(a.Labels, b.Labels)
}
