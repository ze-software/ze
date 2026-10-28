// Design: docs/architecture/dns/lan-resolver.md -- forwarding, failover, bailiwick and caching one reply
// Related: forward.go -- one upstream exchange
// Related: cache.go -- where the accepted data is stored

package dnsresolver

import (
	"slices"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// outcome says what the resolver does with a query. Zero is unspecified, so
// an unset result is never read as an answer.
type outcome uint8

const (
	outcomeUnspecified outcome = iota
	// outcomeAnswered: the result's rcode and sections are the reply.
	outcomeAnswered
	// outcomeDropped: the query gets no reply, so the client retries.
	outcomeDropped
)

// result is the answer to one question. Its records are read-only: they
// may be shared by every query a flight answers.
type result struct {
	outcome   outcome
	rcode     int
	answer    []dns.RR
	authority []dns.RR
}

// servfail is the answer when the resolution cannot finish.
//
// RFC 1035 Section 4.1.1: "Server failure - The name server was unable to
// process this query due to a problem with the name server."
//
// Never NOERROR with no data, which a client would read as a name that holds
// no record, and never cached.
func servfail() result {
	return result{outcome: outcomeAnswered, rcode: dns.RcodeServerFailure}
}

// resolveShared answers q from upstream, coalescing identical questions in
// flight and bounding concurrent resolutions by the policy's outstanding
// limit. A query over that limit is dropped.
//
// A query that joins a resolution already in flight takes no resolution
// slot, but it parks its request goroutine and a timer, so the waiters are
// bounded too: at most the outstanding limit of them at once, and one beyond
// it is dropped, as a new resolution beyond the limit is. Unbound bounds the
// same way: num-queries-per-thread counts every client waiting for an
// answer, not only the resolutions.
func (r *resolver) resolveShared(pol *policy, q dns.Question) result {
	key := keyOf(q.Name, q.Qtype, q.Qclass)
	f, leader := r.flights.join(key)
	if !leader {
		return r.awaitFlight(pol, f)
	}
	// The flight is finished on every exit, a panic in resolve included, so
	// its waiters are released and a later identical question can lead.
	res := result{outcome: outcomeDropped}
	defer func() { r.flights.finish(key, f, res) }()
	if !r.limits.acquire(pol.outstanding) {
		return res
	}
	// What the waiters are given when resolve panics.
	res = servfail()
	res = r.resolveCounted(pol, q)
	return res
}

// resolveCounted runs resolve under the outstanding slot acquire took, and
// releases the slot on every exit, a panic included. MUST follow a true
// acquire.
func (r *resolver) resolveCounted(pol *policy, q dns.Question) result {
	r.outstandingGauge()
	defer r.outstandingGauge()
	defer r.limits.release()
	return r.resolve(pol, q)
}

// awaitFlight waits for the leader of f within the policy's timeout. The
// query is dropped when the outstanding limit of waiters is already parked.
func (r *resolver) awaitFlight(pol *policy, f *flight) result {
	if !r.limits.acquireWaiter(pol.outstanding) {
		return result{outcome: outcomeDropped}
	}
	defer r.limits.releaseWaiter()
	r.stats.coalesced.Add(1)
	t := time.NewTimer(pol.timeout)
	defer t.Stop()
	select {
	case <-f.done:
		return f.res
	case <-t.C:
		return servfail()
	}
}

// resolve forwards q, following a CNAME chain the upstream left unfinished,
// within one budget.
func (r *resolver) resolve(pol *policy, q dns.Question) result {
	// The deadline is wall time: it bounds real sockets, while r.now only
	// ages cache entries.
	b := budget{deadline: time.Now().Add(pol.timeout)}
	var answer []dns.RR
	seen := []string{strings.ToLower(dns.Fqdn(q.Name))}
	name := q.Name
	for {
		reply, ok := r.forward(pol, &b, dns.Question{Name: name, Qtype: q.Qtype, Qclass: q.Qclass})
		if !ok {
			return servfail()
		}
		step := r.absorb(pol, reply, name, q, &b, &seen)
		answer = append(answer, step.answer...)
		switch step.kind {
		case stepFailed:
			return servfail()
		case stepDone:
			return result{outcome: outcomeAnswered, rcode: step.rcode, answer: answer, authority: step.authority}
		case stepRestart:
			name = step.target
		case stepUnspecified:
			panic("BUG: dnsresolver: absorb returned no step")
		}
	}
}

// forward asks the upstreams in configured order and returns the first
// usable reply. An upstream that is silent, fails, or answers with an RCODE
// other than NOERROR or NXDOMAIN is passed over for the next.
//
// RFC 1035 Section 4.2.1: "The client should try other servers and server
// addresses before repeating a query to a specific address of a server."
//
// One pass over the list, each attempt given an equal share of the time
// left, so every upstream is tried before the deadline.
func (r *resolver) forward(pol *policy, b *budget, q dns.Question) (*dns.Msg, bool) {
	for i, up := range pol.forward {
		left := time.Until(b.deadline)
		if left <= 0 {
			return nil, false
		}
		if !b.spendQuery() {
			return nil, false
		}
		addr := up.addrPort()
		attemptEnd := time.Now().Add(left / time.Duration(len(pol.forward)-i))
		query := upstreamQuery(q, dns.Id())
		reply, err := exchange(networkUDP, addr, query, attemptEnd)
		if err == nil && reply.Truncated {
			// RFC 1035 Section 4.2.1: "Messages carried by UDP are restricted
			// to 512 bytes (not counting the IP or UDP headers).  Longer
			// messages are truncated and the TC bit is set in the header."
			r.upstreamOutcome(upstreamTruncated)
			if !b.spendQuery() {
				return nil, false
			}
			query = upstreamQuery(q, dns.Id())
			reply, err = exchange(networkTCP, addr, query, attemptEnd)
		}
		if err != nil {
			if err == errUpstreamTimeout { //nolint:errorlint // exchange returns the sentinel itself
				r.upstreamOutcome(upstreamTimeout)
				continue
			}
			r.upstreamOutcome(upstreamError)
			continue
		}
		if reply.Rcode != dns.RcodeSuccess && reply.Rcode != dns.RcodeNameError {
			r.upstreamOutcome(upstreamFailure)
			continue
		}
		r.upstreamOutcome(upstreamAnswered)
		return reply, true
	}
	return nil, false
}

// stepKind is what one absorbed reply left to do.
type stepKind uint8

const (
	stepUnspecified stepKind = iota
	stepDone
	stepRestart
	stepFailed
)

// step is the part of the answer one reply gave.
type step struct {
	kind      stepKind
	rcode     int
	answer    []dns.RR
	authority []dns.RR
	target    string
}

// absorb takes from reply the records on the chain from name to the
// answer, caches them, and says whether the answer is complete.
//
// Bailiwick: only answer-section records owned by name or a CNAME target on
// its chain, of the question's class, and an authority-section SOA that owns
// the chain's last name or one of its ancestors are used. Everything else
// in the reply is neither cached nor returned, so an upstream cannot plant
// data about a name nobody asked for.
//
// An ANY query is answered with every answer-section RRset name owns, and
// follows no alias (storeOwned, followsAlias).
func (r *resolver) absorb(pol *policy, reply *dns.Msg, name string, q dns.Question, b *budget, seen *[]string) step {
	now := r.now()
	sets := groupAnswer(reply.Answer, q.Qclass)
	var s step
	current := name
	// diverged: a held alias moved the chain off the names the reply covers.
	diverged := false
	if q.Qtype == dns.TypeANY {
		if s.answer = r.storeOwned(pol, reply, sets, name, now); len(s.answer) > 0 {
			s.kind, s.rcode = stepDone, dns.RcodeSuccess
			return s
		}
	}
	for {
		if rrs, ok := sets[keyOf(current, q.Qtype, q.Qclass)]; ok {
			// RFC 2181 Section 5.4.1
			s.answer = append(s.answer, r.storeRRset(pol, rrs, rankOf(reply, current, name), now)...)
			s.kind, s.rcode = stepDone, dns.RcodeSuccess
			return s
		}
		if !followsAlias(q.Qtype) {
			break
		}
		rrs, ok := sets[keyOf(current, dns.TypeCNAME, q.Qclass)]
		if !ok {
			break
		}
		if cnameTarget(rrs) == "" {
			break
		}
		// RFC 2181 Section 5.4.1
		alias := r.storeRRset(pol, rrs, rankOf(reply, current, name), now)
		s.answer = append(s.answer, alias...)
		// The chain follows the alias the client is given: a held
		// higher-ranked CNAME can point elsewhere than the reply's.
		target := cnameTarget(alias)
		if target == "" {
			break
		}
		if !strings.EqualFold(dns.Fqdn(target), dns.Fqdn(cnameTarget(rrs))) {
			diverged = true
		}
		if !b.spendRestart() {
			s.kind = stepFailed
			return s
		}
		lower := strings.ToLower(dns.Fqdn(target))
		if slices.Contains(*seen, lower) {
			// A CNAME loop never ends in an answer.
			s.kind = stepFailed
			return s
		}
		*seen = append(*seen, lower)
		current = target
	}
	if diverged {
		// The reply's RCODE and authority section speak for its own chain,
		// which left this one at the held alias: ask for the target.
		s.kind, s.target = stepRestart, current
		return s
	}
	soa := findSOA(reply.Ns, current, q.Qclass)
	if reply.Rcode == dns.RcodeNameError {
		s.kind, s.rcode = stepDone, dns.RcodeNameError
		s.authority = r.storeNegative(pol, soa, keyOf(current, typeNameError, q.Qclass), dns.RcodeNameError, now)
		return s
	}
	if soa == nil && !strings.EqualFold(current, name) {
		// The upstream followed the alias but sent nothing about its target:
		// ask for the target.
		s.kind, s.target = stepRestart, current
		return s
	}
	s.kind, s.rcode = stepDone, dns.RcodeSuccess
	s.authority = r.storeNegative(pol, soa, keyOf(current, q.Qtype, q.Qclass), dns.RcodeSuccess, now)
	return s
}

// followsAlias reports whether a query of qtype restarts at a CNAME's
// target. A CNAME query asks for the alias record itself, and an ANY query
// is matched by the CNAME as by any other type at the node.
//
// RFC 1034 Section 4.3.2: "If the data at the node is a CNAME, and QTYPE
// doesn't match CNAME, copy the CNAME RR into the answer section of the
// response, change QNAME to the canonical name in the CNAME RR, and go back
// to step 1."
func followsAlias(qtype uint16) bool {
	if qtype == dns.TypeCNAME {
		return false
	}
	return qtype != dns.TypeANY
}

// storeOwned caches every answer-section RRset owned by name, of the
// question's class, and returns them in the order the reply carries them:
// the answer to an ANY query.
//
// RFC 2181 Section 5.3.1: "However, where SIG records are being returned in
// the answer section, in response to a query for SIG records, or a query for
// all records associated with a name (type=ANY) the entire SIG RRSet must be
// included, as for any other RR type."
//
// Each RRset but the RRSIG one is cached under its own type, so a later
// query for that type is answered from the cache. The ANY answer itself is
// never cached: the
// per-type entries cannot prove they are every RRset the node holds, so an
// ANY query answered from them could leave sets out. Every ANY query is
// forwarded, where Unbound caches the whole ANY reply as one message.
func (r *resolver) storeOwned(pol *policy, reply *dns.Msg, sets map[cacheKey][]dns.RR, name string, now time.Time) []dns.RR {
	var answer []dns.RR
	var taken []cacheKey
	for _, rr := range reply.Answer {
		h := rr.Header()
		k := keyOf(h.Name, h.Rrtype, h.Class)
		rrs, ok := sets[k]
		if !ok {
			continue
		}
		if !strings.EqualFold(dns.Fqdn(h.Name), dns.Fqdn(name)) {
			continue
		}
		if slices.Contains(taken, k) {
			continue
		}
		taken = append(taken, k)
		if h.Rrtype == dns.TypeRRSIG {
			// An ANY answer need not carry every signature the node holds,
			// so its RRSIG records are returned, never cached where an RRSIG
			// query would read them as the node's whole RRSIG set.
			answer = append(answer, withTTL(rrs, trustedTTL(pol, rrs))...)
			continue
		}
		// RFC 2181 Section 5.4.1
		answer = append(answer, r.storeRRset(pol, rrs, rankOf(reply, name, name), now)...)
	}
	return answer
}

// groupAnswer groups the answer section's records of class qclass into
// RRsets by owner and type.
func groupAnswer(rrs []dns.RR, qclass uint16) map[cacheKey][]dns.RR {
	sets := make(map[cacheKey][]dns.RR, len(rrs))
	for _, rr := range rrs {
		h := rr.Header()
		if h.Class != qclass {
			continue
		}
		k := keyOf(h.Name, h.Rrtype, h.Class)
		sets[k] = append(sets[k], rr)
	}
	return sets
}

// findSOA returns the authority-section SOA of class qclass that owns name
// or one of its ancestors, or nil.
func findSOA(ns []dns.RR, name string, qclass uint16) *dns.SOA {
	for _, rr := range ns {
		soa, ok := rr.(*dns.SOA)
		if !ok {
			continue
		}
		if soa.Hdr.Class != qclass {
			continue
		}
		if dns.IsSubDomain(soa.Hdr.Name, name) {
			return soa
		}
	}
	return nil
}

// storeRRset caches one RRset under the TTL it is trusted for and returns
// the copies the reply carries. When the cache keeps a live higher-ranked
// RRset under the same key, that RRset is returned instead, at its
// remaining TTL: the client is given the data the resolver trusts, never the
// lower-ranked copy the cache refused.
//
// RFC 2181 Section 5.2: "Should an authoritative source send such a
// malformed RRSet, the client should treat the RRs for all purposes as if
// all TTLs in the RRSet had been set to the value of the lowest TTL in the
// RRSet."
//
// RFC 2181 Section 5.2: "Clients that are configured to send all queries to
// one, or more, particular servers should treat those servers as
// authoritative for this purpose."
func (r *resolver) storeRRset(pol *policy, rrs []dns.RR, rk rank, now time.Time) []dns.RR {
	ttl := trustedTTL(pol, rrs)
	stored := withTTL(rrs, ttl)
	h := rrs[0].Header()
	e := &cacheEntry{key: keyOf(h.Name, h.Rrtype, h.Class), rrs: stored, rank: rk, stored: now, ttl: ttl}
	held := r.answers.store(e, now)
	if held == nil {
		return stored
	}
	if held == e {
		return stored
	}
	left, live := held.remaining(now)
	if !live {
		panic("BUG: dnsresolver: the cache kept an expired entry over a new one")
	}
	return withTTL(held.rrs, left)
}

// trustedTTL is the one TTL every record of rrs is given: the lowest
// received TTL, bounded by the configured maximum (RFC 2181 Section 5.2,
// quoted at storeRRset).
func trustedTTL(pol *policy, rrs []dns.RR) uint32 {
	ttl := receivedTTL(rrs[0].Header().Ttl, pol.maxTTL)
	for _, rr := range rrs[1:] {
		ttl = min(ttl, receivedTTL(rr.Header().Ttl, pol.maxTTL))
	}
	return ttl
}

// storeNegative caches a negative answer under key and returns the
// authority section the reply carries. Without an SOA nothing is cached.
//
// RFC 2308 Section 5: "When the authoritative server creates this record
// its TTL is taken from the minimum of the SOA.MINIMUM field and SOA's
// TTL."
//
// RFC 2308 Section 5: "Negative responses without SOA records SHOULD NOT be
// cached as there is no way to prevent the negative responses looping
// forever between a pair of servers even with a short TTL."
func (r *resolver) storeNegative(pol *policy, soa *dns.SOA, key cacheKey, rcode int, now time.Time) []dns.RR {
	if soa == nil {
		return nil
	}
	ttl := min(receivedTTL(soa.Hdr.Ttl, pol.maxNegTTL), receivedTTL(soa.Minttl, pol.maxNegTTL))
	stored := withTTL([]dns.RR{soa}, ttl)
	r.negatives.put(&cacheEntry{key: key, rrs: stored, rcode: rcode, rank: rankAnswer, stored: now, ttl: ttl}, now)
	return stored
}
