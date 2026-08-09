// Design: docs/architecture/dns/geodns.md -- geodns resolver state (atomic snapshot)

package geodns

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// resolverState is the immutable snapshot the engine publishes on each config
// generation. The server (spec 2) and the show handler (spec 3) read it without
// locking; reload swaps the pointer atomically so a query never sees a torn
// state. Mirrors the ntp plugin's published-state pattern.
type resolverState struct {
	cfg     geodnsConfig
	matcher *dnsserver.Matcher
	// serial is the SOA serial computed for this config generation (spec 2),
	// monotonic across reloads per the configured serial-mode.
	serial uint32
}

// stateP holds the current published snapshot (nil until the first configure).
var stateP atomic.Pointer[resolverState]

// buildState builds a resolver snapshot: the validated config plus the
// longest-prefix matcher over its sources.
func buildState(cfg geodnsConfig) *resolverState {
	return &resolverState{cfg: cfg, matcher: buildMatcher(cfg.Sources)}
}

// loadState returns the current snapshot (nil if geodns has not configured yet).
func loadState() *resolverState { return stateP.Load() }

// storeState publishes a new snapshot.
func storeState(s *resolverState) { stateP.Store(s) }
