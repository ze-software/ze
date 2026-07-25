// Design: plan/learned/1033-as112-2-dns-server.md -- as112 published state (atomic snapshot)

package as112

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// as112State is the immutable snapshot the engine publishes on each config
// generation. The server and the show handler read it without locking;
// reload swaps the pointer atomically so a query never sees a torn state.
type as112State struct {
	cfg     as112Config
	matcher *dnsserver.Matcher // allow-from prefixes; nil/empty means answer all
	serial  uint32
}

var stateP atomic.Pointer[as112State]

// allowFromLabel is the single label every allow-from entry maps to; the
// matcher only needs to answer "is this IP in ANY configured prefix", not
// select between multiple named groups (unlike a host-set selector).
const allowFromLabel = "allow"

// buildState builds a published snapshot: the validated config plus the
// compiled allow-from matcher.
func buildState(cfg as112Config, serial uint32) *as112State {
	entries := make([]dnsserver.Entry, 0, len(cfg.AllowFrom))
	for _, p := range cfg.AllowFrom {
		entries = append(entries, dnsserver.Entry{Prefix: p, Label: allowFromLabel})
	}
	var matcher *dnsserver.Matcher
	if len(entries) > 0 {
		matcher = dnsserver.BuildMatcher(entries)
	}
	return &as112State{cfg: cfg, matcher: matcher, serial: serial}
}

func loadState() *as112State { return stateP.Load() }

func storeState(s *as112State) { stateP.Store(s) }
