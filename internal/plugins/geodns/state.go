// Design: docs/architecture/dns/geodns.md -- geodns resolver state (atomic snapshot)

package geodns

import (
	"sync/atomic"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// resolverState is the immutable snapshot the engine publishes on each config
// generation. The server (spec 2) and the show handler (spec 3) read it without
// locking; reload swaps the pointer atomically so a query never sees a torn
// state. Mirrors the ntp plugin's published-state pattern.
type resolverState struct {
	cfg     geodnsConfig
	matcher *dnsserver.Matcher
	// names is every domain name the configuration gives existence to, in the
	// lower-cased trailing-dot form fqdn produces. It is the set nameExists
	// reads to tell a name error from a no-data answer, computed once per
	// generation because the answer path consults it on every miss.
	names map[string]struct{}
	// serial is the SOA serial computed for this config generation (spec 2),
	// monotonic across reloads per the configured serial-mode.
	serial uint32
}

// stateP holds the current published snapshot (nil until the first configure).
var stateP atomic.Pointer[resolverState]

// buildState builds a resolver snapshot: the validated config, the
// longest-prefix matcher over its sources, and the set of names the config
// gives existence to.
func buildState(cfg geodnsConfig) *resolverState {
	return &resolverState{cfg: cfg, matcher: buildMatcher(cfg.Sources), names: buildNames(cfg)}
}

// buildNames collects every name the configuration gives existence to: each
// configured host, in every host set, plus every interior node between that
// host and its zone apex.
//
// The interior nodes matter because RFC 1035 Section 3.1 makes a domain name a
// path through a tree rather than a flat string: "Each node has a label". A
// configuration holding only "a.b.example.com." therefore gives "b.example.com."
// existence as well, as the node the leaf hangs from. A query for it owns no
// record of any type, so it is no data rather than a name error, and answering
// RCODE 3 there would deny the existence of a name that has a descendant.
func buildNames(cfg geodnsConfig) map[string]struct{} {
	names := make(map[string]struct{})
	for _, hs := range cfg.HostSets {
		for host := range hs.Hosts {
			zone := matchZone(host, cfg.Zones)
			if zone == "" {
				continue
			}
			for n := fqdn(host); n != zone && n != ""; n = parentName(n) {
				names[n] = struct{}{}
			}
		}
	}
	return names
}

// parentName returns the name one label up, or "" at the root. It reads the
// label offsets dns.Split computes rather than cutting at the first '.', so an
// escaped dot inside a label does not split the name at the wrong place.
func parentName(name string) string {
	labels := dns.Split(name)
	if len(labels) < 2 {
		return ""
	}
	return name[labels[1]:]
}

// loadState returns the current snapshot (nil if geodns has not configured yet).
func loadState() *resolverState { return stateP.Load() }

// storeState publishes a new snapshot.
func storeState(s *resolverState) { stateP.Store(s) }
