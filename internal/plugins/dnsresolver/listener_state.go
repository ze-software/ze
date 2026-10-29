// Design: docs/architecture/dns/lan-resolver.md -- listener state in show dns resolver and ze_dns_resolver_listener_up
// Related: server.go -- newResolverServer wires the harness's OnListenerChange here
// Related: show.go -- show reports the state per listener

package dnsresolver

import (
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin"
)

// listenerKey names one bound socket: the transport ("udp" or "tcp") and the
// bound address with its port, as the harness reports them, so two listeners
// on one IP with different ports are two keys.
type listenerKey struct {
	protocol string
	listen   netip.AddrPort
}

// listenerStates records whether each listener socket is bound, from the
// harness's OnListenerChange edges, so show can say which listeners serve and
// the gauge follows the same edges. A socket the harness never reported is
// absent, never down: show then says so by its missing entry. Safe for
// concurrent use: the edges arrive from the config transaction and from a
// crashed listener's serve goroutine.
type listenerStates struct {
	mu sync.Mutex
	up map[listenerKey]bool
}

// change records one edge and moves the gauge with it.
func (s *listenerStates) change(protocol string, listen netip.AddrPort, up bool) {
	s.mu.Lock()
	if s.up == nil {
		s.up = map[listenerKey]bool{}
	}
	s.up[listenerKey{protocol: protocol, listen: listen}] = up
	s.mu.Unlock()

	value := 0.0
	if up {
		value = 1
	}
	rmetrics().listenerUp.With(protocol, listen.String()).Set(value)
}

// prune forgets every socket whose address is not one of keep's, and
// deletes its gauge series: a listener the config no longer names was
// unbound for the last time, and a series left behind would report a socket
// that no longer exists, at 0, forever. A kept listener's state stays, down
// included, so show still says a configured listener is not serving.
func (s *listenerStates) prune(keep []listenerEndpoint) {
	s.mu.Lock()
	var gone []listenerKey
	for k := range s.up {
		if !slices.ContainsFunc(keep, func(l listenerEndpoint) bool { return netip.AddrPortFrom(l.IP, l.Port) == k.listen }) {
			gone = append(gone, k)
			delete(s.up, k)
		}
	}
	s.mu.Unlock()

	for _, k := range gone {
		rmetrics().listenerUp.Delete(k.protocol, k.listen.String())
	}
}

// show returns the state of the sockets bound on listen (the listen IP and
// port, the form the harness reports), one key per transport:
// {"udp": "up", "tcp": "down"}. A transport never reported is "unknown",
// because no edge says whether it is bound.
func (s *listenerStates) show(listen netip.AddrPort) plugin.Map {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := plugin.Map{}
	for _, protocol := range []string{networkUDP, networkTCP} {
		up, known := s.up[listenerKey{protocol: protocol, listen: listen}]
		switch {
		case !known:
			out[protocol] = "unknown"
		case up:
			out[protocol] = "up"
		default:
			out[protocol] = "down"
		}
	}
	return out
}
