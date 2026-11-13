// Design: docs/architecture/rib/unified-locrib.md -- private test topology wiring.

package sysrib

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// locRIBPtr stores the shared cross-protocol Loc-RIB.
var locRIBPtr atomic.Pointer[locrib.RIB]

// nhResolverPtr stores the NH resolver, created when a Loc-RIB is wired.
var nhResolverPtr atomic.Pointer[nhResolver]

// SetLocRIB wires the shared Loc-RIB and creates the NH resolver over it.
//
// A nil RIB removes both. Leaving the resolver in place would keep answering
// reachability from a Loc-RIB nothing writes to any more: every next-hop would
// resolve to unreachable, sysrib would withdraw the routes that depend on them,
// and getNHResolver would report a resolver to code that asks whether one is
// wired at all.
func SetLocRIB(r *locrib.RIB) {
	locRIBPtr.Store(r)
	if r == nil {
		nhResolverPtr.Store(nil)
		igpcost.Set(nil)
		return
	}

	resolver := newNHResolver(r)
	nhResolverPtr.Store(resolver)
	igpcost.Set(resolver.IGPMetric)
}

func getLocRIB() *locrib.RIB { return locRIBPtr.Load() }

func getNHResolver() *nhResolver { return nhResolverPtr.Load() }
