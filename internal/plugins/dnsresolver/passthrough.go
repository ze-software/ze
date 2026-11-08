// Design: docs/architecture/dns/lan-resolver.md -- a query carrying TSIG or TKEY is forwarded unchanged
// RFC: rfc/short/rfc5625.md -- DNS proxy guidelines, Section 4.5
// Related: server.go -- admit, the checks every query meets first
// Related: forward.go -- exchangeWire, the exchange it runs per upstream

package dnsresolver

import (
	"encoding/binary"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// passThrough is the resolver's dnsserver.PassFunc: it forwards wire, a query
// carrying a TSIG or a TKEY RR, to the upstreams in configured order and
// returns the first reply as the upstream sent it.
//
// RFC 5625 Section 4.5: "DNS proxies MUST implement Section 4.7 of [RFC2845]
// and either forward packets unchanged (as recommended above) or fully
// implement TSIG."
//
// RFC 5625 Section 4.5: "As per Section 4.3, DNS proxies MUST be capable of
// proxying packets containing TKEY [RFC2930] Resource Records."
//
// The resolver shares no TSIG key with a client, so it forwards the query
// unchanged and never verifies it; the upstream that holds the key does,
// and its signed reply reaches the client unchanged. Only the Query ID is
// replaced, by a fresh random one per upstream, and put back in the reply.
//
// RFC 5625 Section 6.1: "NB: changing the Query ID is acceptable and
// compatible with proxying TSIG-signed packets since the TSIG signature
// calculation is based on the original message ID, which is carried in the
// TSIG RR."
//
// Neither the query nor the reply touches the cache: an answer from the
// cache would carry no signature, and a signed reply is the upstream's
// answer to this one client. RD is not judged, because no cache can be
// snooped through this path, and RFC 2930 Section 4 says a TKEY query
// "SHOULD NOT be flagged as recursive". Every reply that matches is
// returned, whatever its RCODE, so a TSIG error (BADKEY, BADSIG, BADTIME)
// reaches the client that can read it; the next upstream is tried only when
// one is silent or the exchange fails. The query is sent over the transport
// it arrived on, so a datagram reply with TC set reaches the client with TC
// set and the client asks again over TCP.
func (r *resolver) passThrough(msg *dns.Msg, wire []byte, q *dns.Msg, p dnsserver.Peer) ([]byte, bool) {
	transport := p.RemoteAddr().Network()
	pol, decided, send := r.admit(msg, p)
	if decided {
		return nil, send
	}
	if len(q.Question) != passQuestion {
		// RFC 1035 Section 4.1.1: "Format error - The name server was unable
		// to interpret the query."
		msg.Rcode = dns.RcodeFormatError
		r.countQuery(transport, msg.Rcode)
		return nil, true
	}
	if !r.limits.acquire(pol.outstanding) {
		r.countDropped()
		return nil, false
	}
	reply, ok := r.forwardUnchangedCounted(pol, transport, wire, q)
	if !ok {
		// RFC 1035 Section 4.1.1: "Server failure - The name server was
		// unable to process this query due to a problem with the name
		// server."
		msg.Rcode = dns.RcodeServerFailure
		r.countQuery(transport, msg.Rcode)
		return nil, true
	}
	// The reply parsed, so it holds the 12-octet header. RCODE is the low
	// four bits of octet 3 (RFC 1035 Section 4.1.1: "|QR|   Opcode  |AA|TC|
	// RD|RA|   Z    |   RCODE   |").
	r.countQuery(transport, int(reply[3]&rcodeMask))
	return reply, true
}

// forwardUnchangedCounted runs forwardUnchanged under the outstanding slot
// acquire took, and releases it on every exit. MUST follow a true acquire.
func (r *resolver) forwardUnchangedCounted(pol *policy, network string, wire []byte, q *dns.Msg) ([]byte, bool) {
	r.outstandingGauge()
	defer r.outstandingGauge()
	defer r.limits.release()
	return r.forwardUnchanged(pol, network, wire, q)
}

// passQuestion is the question count a pass-through query must hold: the
// reply is matched by its one question (replyMatches).
const passQuestion = 1

// rcodeMask selects RCODE in octet 3 of the header.
const rcodeMask = 0x0F

// forwardUnchanged sends wire to each upstream in turn, each attempt given
// an equal share of the time left, and returns the first matching reply
// with the client's Query ID put back.
func (r *resolver) forwardUnchanged(pol *policy, network string, wire []byte, q *dns.Msg) ([]byte, bool) {
	b := budget{deadline: time.Now().Add(pol.timeout)}
	for i, up := range pol.forward {
		left := time.Until(b.deadline)
		if left <= 0 {
			return nil, false
		}
		if !b.spendQuery() {
			return nil, false
		}
		attemptEnd := time.Now().Add(left / time.Duration(len(pol.forward)-i))
		id := dns.Id()
		out := make([]byte, len(wire))
		copy(out, wire)
		binary.BigEndian.PutUint16(out, id)
		match := &dns.Msg{Question: q.Question}
		match.Id = id
		_, reply, err := exchangeWire(network, up.addrPort(), match, out, dns.MaxMsgSize, attemptEnd)
		if err != nil {
			if err == errUpstreamTimeout { //nolint:errorlint // exchangeWire returns the sentinel itself
				r.upstreamOutcome(upstreamTimeout)
				continue
			}
			r.upstreamOutcome(upstreamError)
			continue
		}
		r.upstreamOutcome(upstreamAnswered)
		binary.BigEndian.PutUint16(reply, q.Id)
		return reply, true
	}
	return nil, false
}
