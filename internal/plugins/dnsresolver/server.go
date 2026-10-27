// Design: docs/architecture/dns/server-harness.md -- the recursive wrapper the resolver answers through
// Related: config_tx.go -- the installer that reconciles these listeners

package dnsresolver

import (
	"log/slog"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// answerQuery is the resolver's answer function, called by
// dnsserver.Recursive with the recursive shape already applied.
//
// RFC 1035 Section 4.1.1: "Server failure - The name server was unable to
// process this query due to a problem with the name server."
//
// No resolution path exists yet, so every query is answered SERVFAIL: the
// RCODE for a server that cannot answer, and never NOERROR with no data,
// which a stub would read as a name that holds no record.
func answerQuery(msg, _ *dns.Msg, _ dnsserver.Peer) bool {
	// RFC 1035 Section 4.1.1
	msg.Rcode = dns.RcodeServerFailure
	return true
}

// onPanic logs a query whose answer panicked; the harness has recovered it
// and written no reply.
func onPanic(rec any) {
	loggerPtr.Load().Error("dnsresolver: recovered panic handling query", "panic", rec)
}

// resolverServer owns the resolver's listeners over the shared harness.
// Not safe for concurrent use: the config transaction calls it under its
// lock.
type resolverServer struct {
	mgr *dnsserver.Manager
}

// newResolverServer builds the listener manager: Recursive owns the reply
// shape and the single write, and the UDP read buffer matches the size the
// reply OPT advertises.
func newResolverServer(log *slog.Logger) *resolverServer {
	handler := dnsserver.Recursive(log, answerQuery, onPanic)
	return &resolverServer{mgr: dnsserver.New(log, handler, dnsserver.Options{
		UDPSize: dnsserver.RecursiveUDPSize,
	})}
}

// reconcile binds the listeners cfg names, or none when it is disabled. An
// unchanged endpoint set is a no-op in the harness, so a reload that changes
// only policy never rebinds. The resolver serves no DoT or DoH, so the
// secure config is the harness default, both transports off.
func (s *resolverServer) reconcile(cfg resolverConfig, policy dnsserver.BindPolicy) error {
	plain := make([]dnsserver.Endpoint, len(cfg.Listeners))
	for i, l := range cfg.Listeners {
		plain[i] = dnsserver.Endpoint{IP: l.IP, Port: l.Port}
	}
	return s.mgr.ApplyWithSecure(cfg.Enabled, plain, dnsserver.DefaultSecureConfig(), policy, loggerPtr.Load())
}

// stop unbinds every listener. MUST be called once the plugin stops.
func (s *resolverServer) stop() { s.mgr.Stop() }
