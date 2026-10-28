// Design: docs/architecture/dns/server-harness.md -- the recursive-answer
// shape, the second wrapper beside Authoritative
// RFC: rfc/short/rfc1035.md -- DNS message structure
// Related: handler.go -- Authoritative, send and the datagram bound both wrappers share

package dnsserver

import (
	"log/slog"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// RecursiveUDPSize is the datagram size, in octets, a recursive service reads
// and answers up to, and the size its reply OPT advertises. 1232 is the size
// the DNS flag day 2020 operators chose to keep a datagram under the IPv6
// minimum MTU, and it is at least the 1220 octets RFC 4035 Section 3 asks a
// name server to accept. A consumer of Recursive passes the same value as
// Options.UDPSize so the listener reads what the reply side offers.
const RecursiveUDPSize = 1232

// Recursive wraps fn as a dns.HandlerFunc for a recursive service, as
// Authoritative does for an authoritative one: it owns the reply shape, the
// opcode check, the datagram bound, the single wire write, and the panic
// guard. The two wrappers are separate rather than one wrapper with a flag,
// because each shape is an invariant the wrapper enforces after fn, and a
// flag would make the shape a choice every caller can get wrong.
//
// The shape (shapeRecursive) is applied before fn, so fn builds on it, and
// again after fn, so no answer func can claim authority, claim validation,
// deny recursion, or change the CD bit the query carried. fn returns
// send=true to have the reply written, or send=false to drop the query.
//
// RFC 1035 Section 6.4: "If a name server receives an inverse query that it
// does not support, it returns an error response with the "Not Implemented"
// error set in the header.  While inverse query support is optional, all name
// servers must be at least able to return the error response."
//
// A query whose opcode is not a standard query draws that reply here and
// never reaches fn, so no opcode Ze does not implement is forwarded upstream.
//
// If fn panics, queryGuard contains it as it does for Authoritative: the panic
// is logged, counted and recorded, and no reply is written.
//
// A nil log discards the write-failure line (see send), never the panic line.
func Recursive(log *slog.Logger, fn AnswerFunc) dns.HandlerFunc {
	if log == nil {
		log = slogutil.DiscardLogger()
	}
	task := queryGuard.Task(taskRecursiveQuery, queryRestart)
	return func(w dns.ResponseWriter, r *dns.Msg) {
		defer task.Contain(describeQuery(w, r), nil)
		msg := new(dns.Msg)
		if r.Opcode != dns.OpcodeQuery {
			msg.SetRcode(r, dns.RcodeNotImplemented)
			shapeRecursive(msg, r)
			// RFC 1035 Section 6.4
			send(w, msg, recursiveReplyLimit(r), log)
			return
		}
		msg.SetReply(r)
		shapeRecursive(msg, r)
		if !fn(msg, r, w) {
			return
		}
		shapeRecursive(msg, r)
		send(w, msg, recursiveReplyLimit(r), log)
	}
}

// recursiveReplyLimit returns the largest datagram reply to r: what the
// requestor offered (udpReplyLimit, never below 512), held to
// RecursiveUDPSize, the buffer this service advertises in its own OPT.
//
// RFC 6891 Section 6.2.5: "A requestor MAY choose to implement a fallback to
// smaller advertised sizes to work around firewall or other network
// limitations."
//
// RFC 6891 Section 6.2.5: "Values of less than 512 bytes MUST be treated as
// equal to 512 bytes."
//
// A responder that answered up to any size a client offered would send
// datagrams that fragment, so the service's own bound wins when it is the
// smaller of the two.
func recursiveReplyLimit(r *dns.Msg) int {
	return min(udpReplyLimit(r), RecursiveUDPSize)
}

// shapeRecursive applies the recursive-answer shape to msg, the reply to r.
//
// RFC 1035 Section 4.1.1: "RA              Recursion Available - this be is
// set or cleared in a response, and denotes whether recursive query support
// is available in the name server."
//
// Recursion is what this service provides, so RA is set on every reply,
// refusals included: the refusal is a policy answer, not a statement that the
// server cannot recurse.
//
// RFC 1035 Section 4.1.1: "AA              Authoritative Answer - this bit is
// valid in responses, and specifies that the responding name server is an
// authority for the domain name in question section."
//
// A forwarding resolver holds no zone, so AA is always clear, whatever an
// upstream's reply said. Z is reserved and zero ("Z               Reserved
// for future use.  Must be zero in all queries and responses."), and AD is
// clear because this service validates nothing (RFC 4035 Section 3.1.6
// permits AD only on data the server verified).
//
// RFC 4035 Section 3.2.2: "The name server side MUST copy the setting of the
// CD bit from a query to the corresponding response."
//
// SetReply copies CD for a standard query only, and an answer func could
// change it, so CD is copied from r here on every reply.
//
// The reply carries an OPT exactly when the query did (RFC 6891 Section
// 6.1.1), advertising RecursiveUDPSize with DO clear: the service is not
// security-aware, so it never claims to return DNSSEC records it asked for.
// Compression stays off, as in the authoritative shape, and send turns it on
// only for a datagram reply it has to shorten.
func shapeRecursive(msg, r *dns.Msg) {
	msg.Authoritative = false
	msg.RecursionAvailable = true
	msg.Zero = false
	msg.AuthenticatedData = false
	// RFC 4035 Section 3.2.2
	msg.CheckingDisabled = r.CheckingDisabled
	msg.Compress = false
	setReplyOPT(msg, r.IsEdns0() != nil)
}

// setReplyOPT leaves msg with one OPT advertising RecursiveUDPSize when
// wanted is true, and with none when it is false, removing any OPT an answer
// func added. The OPT is the last additional record, where Pack expects it.
func setReplyOPT(msg *dns.Msg, wanted bool) {
	kept := msg.Extra[:0]
	for _, rr := range msg.Extra {
		if _, isOPT := rr.(*dns.OPT); isOPT {
			continue
		}
		kept = append(kept, rr)
	}
	msg.Extra = kept
	if !wanted {
		return
	}
	opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	opt.SetUDPSize(RecursiveUDPSize)
	msg.Extra = append(msg.Extra, opt)
}
