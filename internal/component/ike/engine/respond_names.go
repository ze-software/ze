// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the DNS responder cache
// Related: register.go -- matchResponderPeer, the packet-path reader
// Related: dynamic_wan.go -- the address worker, the only goroutine that resolves

package engine

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// respondNameRefresh is how often the address worker resolves every `respond` peer's
// remote-address name again. Go's resolver exposes no TTL, so the period is fixed, and it
// is the reconnect backoff ceiling (reconnectMax): a name that moves is followed within
// the time an initiator would take to retry.
const respondNameRefresh = 60 * time.Second

// respondNameTimeout bounds one resolution, so a resolver that never answers holds the
// address worker for this long and no longer.
const respondNameTimeout = 5 * time.Second

// lookupNameFn resolves a name to its IPv4 addresses. IKE over IPv6 is not supported, so
// only A records are asked for. Swapped in unit tests, which count the calls to prove
// that nothing resolves on the packet path.
var lookupNameFn = func(ctx context.Context, name string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip4", name)
}

// respondNameEntry is what one `respond` peer's name last resolved to.
type respondNameEntry struct {
	name  string
	addrs []netip.Addr
	at    time.Time
}

// respondNameQuery names one resolution the worker owes.
type respondNameQuery struct {
	peer string
	name string
}

// respondNameCache holds, for each `respond` peer whose remote-address is a DNS name, the
// addresses that name last resolved to. matchResponderPeer reads it to accept an
// unsolicited IKE_SA_INIT, and never resolves: a datagram anyone can forge must not make
// Ze query DNS. The address match only selects which configuration the initiator must
// then authenticate against, so a stale set admits no one IKE_AUTH would not.
//
// Safe for concurrent use: the packet path reads under the read lock, the apply path and
// the address worker write under the write lock, and no resolution runs under either.
type respondNameCache struct {
	mu      sync.RWMutex
	entries map[string]respondNameEntry
}

// respondNames is the cache of the running engine. Package-level for the reason
// activePeersMap is: matchResponderPeer is a package function on the dispatch path.
var respondNames = &respondNameCache{entries: map[string]respondNameEntry{}}

// respondNamesOf lists the `respond` peers of cfg whose remote-address is a name, keyed
// by peer. A literal is matched as itself and needs no cache entry.
func respondNamesOf(cfg *ipsec.IPsecConfig) map[string]string {
	out := make(map[string]string)
	for name := range cfg.Peers {
		peer := cfg.Peers[name]
		if peer.ConnectionType != ipsec.ConnectionRespond || peer.RemoteAddress == "" {
			continue
		}
		if _, err := netip.ParseAddr(peer.RemoteAddress); err == nil {
			continue
		}
		out[name] = peer.RemoteAddress
	}
	return out
}

// sync adopts the peers of a newly applied configuration. A peer that is new, or whose
// name changed, gets an EMPTY set, so it matches nothing until the worker answers; a peer
// that is gone, or whose remote-address became a literal, loses its entry. A peer whose
// name is unchanged keeps the set it has. It returns every resolution owed.
func (c *respondNameCache) sync(cfg *ipsec.IPsecConfig) []respondNameQuery {
	want := respondNamesOf(cfg)
	c.mu.Lock()
	defer c.mu.Unlock()
	for peer := range c.entries {
		if _, ok := want[peer]; !ok {
			delete(c.entries, peer)
		}
	}
	queries := make([]respondNameQuery, 0, len(want))
	for peer, name := range want {
		if entry, ok := c.entries[peer]; !ok || entry.name != name {
			c.entries[peer] = respondNameEntry{name: name}
		}
		queries = append(queries, respondNameQuery{peer: peer, name: name})
	}
	return queries
}

// queries lists every resolution a refresh owes.
func (c *respondNameCache) queries() []respondNameQuery {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]respondNameQuery, 0, len(c.entries))
	for peer, entry := range c.entries {
		out = append(out, respondNameQuery{peer: peer, name: entry.name})
	}
	return out
}

// store records one resolution's answer. An answer for a peer that was removed, or whose
// name changed while the query ran, is dropped: it answers a question nobody asks now.
//
// A failed resolution KEEPS the previous set and says so at WARN, with the peer, the
// name, the resolver error and the fix: emptying it would refuse a working peer for a
// transient resolver fault. A successful answer REPLACES the set (never merges), so an
// address the name no longer returns is refused from then on. An answer with no IPv4
// address empties the set.
func (c *respondNameCache) store(q respondNameQuery, addrs []netip.Addr, err error, log *slog.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[q.peer]
	if !ok || entry.name != q.name {
		return
	}
	if err != nil {
		log.Warn("ike: respond peer remote-address does not resolve, keeping the addresses it resolved to before. "+
			"Check the name and the system resolvers, or give the address with: set vpn ipsec site-to-site peer "+q.peer+" remote-address <ipv4>",
			"peer", q.peer, "name", q.name, "error", err, "kept", len(entry.addrs))
		return
	}
	v4 := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		if a = a.Unmap(); a.Is4() {
			v4 = append(v4, a)
		}
	}
	c.entries[q.peer] = respondNameEntry{name: q.name, addrs: v4, at: time.Now()}
}

// matches reports whether src is one of the addresses name last resolved to for peer.
// It never resolves.
func (c *respondNameCache) matches(peer, name string, src netip.Addr) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[peer]
	if !ok || entry.name != name {
		return false
	}
	return slices.Contains(entry.addrs, src)
}

// resolveRespondNames runs the owed resolutions, one at a time, each bounded by
// respondNameTimeout. It runs on the address worker only, and holds no lock while a
// resolution is in flight.
func resolveRespondNames(queries []respondNameQuery, stop <-chan struct{}, log *slog.Logger) {
	for _, q := range queries {
		select {
		case <-stop:
			return
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), respondNameTimeout)
		addrs, err := lookupNameFn(ctx, q.name)
		cancel()
		respondNames.store(q, addrs, err, log)
	}
}
