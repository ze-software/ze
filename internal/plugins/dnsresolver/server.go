// Design: docs/architecture/dns/server-harness.md -- the recursive wrapper the resolver answers through
// Related: config_tx.go -- the installer that reconciles these listeners
// Related: state.go -- the resolver whose answer function this serves

package dnsresolver

import (
	"log/slog"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// answer is the resolver's answer function, called by dnsserver.Recursive
// with the recursive shape already applied. It returns false to drop the
// query with no reply.
//
// The checks run cheapest first: the source, the rate, the header, then the
// cache, and only then an upstream.
func (r *resolver) answer(msg, q *dns.Msg, p dnsserver.Peer) bool {
	transport := p.RemoteAddr().Network()
	pol := r.policy.Load()
	if pol == nil {
		// RFC 1035 Section 4.1.1: no policy yet, so no answer can be built.
		msg.Rcode = dns.RcodeServerFailure
		r.countQuery(transport, msg.Rcode)
		return true
	}

	source := dnsserver.RemoteAddr(p)
	if _, ok := pol.allow.Lookup(source); !ok {
		// RFC 1035 Section 4.1.1: "Refused - The name server refuses to
		// perform the specified operation for policy reasons."
		msg.Rcode = dns.RcodeRefused
		r.countRefused(refuseACL)
		r.countQuery(transport, msg.Rcode)
		return true
	}
	if !r.limits.admit(source, pol.clientRate, r.now()) {
		r.countRateLimited()
		return false
	}
	if !q.RecursionDesired {
		// RFC 1034 Section 4.3.1: "Note that the name server should never
		// perform recursive service unless asked via RD, since this
		// interferes with trouble shooting of name servers and their
		// databases."
		//
		// Refused whether or not the name is cached, so the cache cannot be
		// snooped (Unbound's default "allow": "Non-recursive queries are
		// refused.").
		msg.Rcode = dns.RcodeRefused
		r.countRefused(refuseRDClear)
		r.countQuery(transport, msg.Rcode)
		return true
	}
	if len(q.Question) != 1 {
		// RFC 1035 Section 4.1.1: "Format error - The name server was unable
		// to interpret the query."
		msg.Rcode = dns.RcodeFormatError
		r.countQuery(transport, msg.Rcode)
		return true
	}

	question := q.Question[0]
	res, ok := r.lookup(question, r.now())
	if ok {
		r.countHit()
	} else {
		r.stats.misses.Add(1)
		res = r.resolveShared(pol, question)
	}
	switch res.outcome {
	case outcomeAnswered:
	case outcomeDropped:
		r.countDropped()
		return false
	case outcomeUnspecified:
		panic("BUG: dnsresolver: resolution returned no outcome")
	}
	msg.Rcode = res.rcode
	msg.Answer = res.answer
	msg.Ns = res.authority
	r.countQuery(transport, msg.Rcode)
	return true
}

// resolverTCPConnMax is how many TCP connections one resolver listener holds
// at once. LAN clients ask over UDP and fall back to TCP only for a truncated
// answer, so the cap sits well above that load while bounding a flood.
const resolverTCPConnMax = 64

// resolverServer owns the resolver's listeners over the shared harness.
// Not safe for concurrent use: the config transaction calls it under its
// lock.
type resolverServer struct {
	mgr *dnsserver.Manager
}

// newResolverServer builds the listener manager over res's answer function:
// Recursive owns the reply shape and the single write, and the UDP read
// buffer matches the size the reply OPT advertises.
//
// Freebind sets IP_FREEBIND on every listener socket, so a listener binds
// an address the host does not hold yet. A LAN resolver's address often comes
// up after the resolver starts: a PPP pool gateway exists only once the first
// subscriber session is up, and an interface address arrives when iface
// applies it. Without it the bind fails with EADDRNOTAVAIL, which refuses a
// reload (BindStrict) and leaves the listener down at boot (BindLenient).
//
// TCPConnMax bounds the TCP connections each listener holds, whoever opens
// them: the ACL is checked per query, after the connection is accepted, so
// without the cap a source outside the ACL could still hold descriptors.
func newResolverServer(log *slog.Logger, res *resolver) *resolverServer {
	return &resolverServer{mgr: dnsserver.New(log, recursiveHandler(log, res), dnsserver.Options{
		UDPSize:    dnsserver.RecursiveUDPSize,
		Freebind:   true,
		TCPConnMax: resolverTCPConnMax,
	})}
}

// recursiveHandler is the handler every listener serves: res's answer
// function inside the harness's recursive wrapper.
func recursiveHandler(log *slog.Logger, res *resolver) dns.HandlerFunc {
	return dnsserver.Recursive(log, res.answer)
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
