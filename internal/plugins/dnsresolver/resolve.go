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
func (r *resolver) resolveShared(pol *policy, q dns.Question) result {
	key := keyOf(q.Name, q.Qtype, q.Qclass)
	f, leader := r.flights.join(key)
	if !leader {
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
	if !r.limits.acquire(pol.outstanding) {
		res := result{outcome: outcomeDropped}
		r.flights.finish(key, f, res)
		return res
	}
	r.outstandingGauge()
	res := r.resolve(pol, q)
	r.limits.release()
	r.outstandingGauge()
	r.flights.finish(key, f, res)
	return res
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
		reply, err := exchange("udp", addr, query, attemptEnd)
		if err == nil && reply.Truncated {
			// RFC 1035 Section 4.2.1: "Messages carried by UDP are restricted
			// to 512 bytes (not counting the IP or UDP headers).  Longer
			// messages are truncated and the TC bit is set in the header."
			r.upstreamOutcome(upstreamTruncated)
			if !b.spendQuery() {
				return nil, false
			}
			query = upstreamQuery(q, dns.Id())
			reply, err = exchange("tcp", addr, query, attemptEnd)
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
func (r *resolver) absorb(pol *policy, reply *dns.Msg, name string, q dns.Question, b *budget, seen *[]string) step {
	now := r.now()
	sets := groupAnswer(reply.Answer, q.Qclass)
	var s step
	current := name
	for {
		if rrs, ok := sets[keyOf(current, q.Qtype, q.Qclass)]; ok {
			// RFC 2181 Section 5.4.1
			s.answer = append(s.answer, r.storeRRset(pol, rrs, rankOf(reply, current, name), now)...)
			s.kind, s.rcode = stepDone, dns.RcodeSuccess
			return s
		}
		if q.Qtype == dns.TypeCNAME {
			break
		}
		rrs, ok := sets[keyOf(current, dns.TypeCNAME, q.Qclass)]
		if !ok {
			break
		}
		target := cnameTarget(rrs)
		if target == "" {
			break
		}
		// RFC 2181 Section 5.4.1
		s.answer = append(s.answer, r.storeRRset(pol, rrs, rankOf(reply, current, name), now)...)
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
// the copies the reply carries.
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
	ttl := receivedTTL(rrs[0].Header().Ttl, pol.maxTTL)
	for _, rr := range rrs[1:] {
		ttl = min(ttl, receivedTTL(rr.Header().Ttl, pol.maxTTL))
	}
	stored := withTTL(rrs, ttl)
	h := rrs[0].Header()
	r.answers.put(&cacheEntry{
		key: keyOf(h.Name, h.Rrtype, h.Class), rrs: stored, rank: rk, stored: now, ttl: ttl,
	}, now)
	return stored
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
