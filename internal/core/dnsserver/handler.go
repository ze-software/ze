// Design: docs/architecture/dns/server-harness.md -- authoritative-answer
// shaping, the single-source-of-truth recursion guard (child-2 R-3)
// RFC: rfc/short/rfc1035.md -- DNS message structure
// Related: metrics.go -- the write-failure counter send() increments

package dnsserver

import (
	"log/slog"
	"net"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// Peer is the read-only view of a query's transport that an AnswerFunc may
// inspect. It exposes only RemoteAddr (resolve the packet source with
// RemoteAddr(Peer)) and deliberately omits WriteMsg: the answer func cannot
// write to the wire, so it cannot bypass the authoritative shaping
// Authoritative enforces. dns.ResponseWriter satisfies it, so Authoritative
// passes the ResponseWriter straight through with no per-query allocation.
type Peer interface {
	RemoteAddr() net.Addr
}

// AnswerFunc builds the reply for one query by mutating msg, which
// Authoritative has already shaped (SetReply plus shapeAuthoritative:
// Compress=false, RecursionAvailable=false, and AA set unless the func chooses
// RCODE 5). p is a
// read-only view of the transport peer; resolve the packet source lazily with
// RemoteAddr(p) only on the paths whose answer depends on it (combine it with
// ClientIP for source-based selection) so a path that refuses or drops pays
// nothing for it.
//
// The answer func never writes to the wire itself: it returns send=true to
// have Authoritative send msg (after re-asserting the authoritative shape), or
// send=false to drop the query with no reply. Withholding the ResponseWriter
// is deliberate -- it makes the authoritative-only / recursion-refused shape an
// invariant Authoritative enforces, not a convention every consumer must
// remember to honor (child-2 R-3).
type AnswerFunc func(msg, r *dns.Msg, p Peer) (send bool)

// Authoritative wraps fn as a dns.HandlerFunc that owns the RFC 1035
// authoritative-answer shape, the opcode check, the transport size bound, the
// single wire write, and the panic-recovery guard. It shapes msg before fn (so
// fn builds on a correct base) and re-asserts the same shape after fn (so no
// answer func can advertise recursion, and none can set or clear the
// authoritative bit against the RCODE it chose), then
// bounds and writes msg exactly once when fn returns send=true. If fn panics,
// onPanic (if non-nil) receives the recovered value and no reply is written --
// one bad query can never crash the listener, nor receive a malformed or
// accidentally-recursive answer.
//
// A query whose opcode is not a standard query never reaches fn: it draws Not
// Implemented here, before any zone lookup or client resolution, so an opcode
// Ze does not serve costs one header.
//
// RFC 1035 Section 6.4: "If a name server receives an inverse query that it
// does not support, it returns an error response with the "Not Implemented"
// error set in the header.  While inverse query support is optional, all name
// servers must be at least able to return the error response."
//
// Ze serves standard queries only, so every other opcode -- inverse query,
// server status request, notify, update -- draws that same reply. Without the
// check they fall through to fn, where a name inside a served zone draws a
// normal answer and a name outside one draws NXDOMAIN. Both claim Ze acted on
// a request it never implemented.
//
// log receives the one line the harness writes on the query path, the failure
// of the wire write (see send). A nil log is taken as "this consumer wants no
// line" and discards it, which is what the answer-func tests pass: a nil
// dereference on the query path would be a defect this package can inflict on
// every reply, and no logging decision is worth that.
func Authoritative(log *slog.Logger, fn AnswerFunc, onPanic func(any)) dns.HandlerFunc {
	if log == nil {
		log = slogutil.DiscardLogger()
	}
	return func(w dns.ResponseWriter, r *dns.Msg) {
		defer func() {
			if rec := recover(); rec != nil && onPanic != nil {
				onPanic(rec)
			}
		}()
		msg := new(dns.Msg)
		if r.Opcode != dns.OpcodeQuery {
			msg.SetRcode(r, dns.RcodeNotImplemented)
			shapeAuthoritative(msg)
			send(w, msg, r, log)
			return
		}
		msg.SetReply(r)
		shapeAuthoritative(msg)
		if !fn(msg, r, w) {
			return
		}
		shapeAuthoritative(msg)
		send(w, msg, r, log)
	}
}

// send bounds msg for the transport it goes out on, then writes it exactly
// once.
//
// RFC 1035 Section 4.2.1: "Messages carried by UDP are restricted to 512 bytes
// (not counting the IP or UDP headers).  Longer messages are truncated and the
// TC bit is set in the header."
//
// The bound is a datagram one. Section 4.2.1 is headed "UDP usage" and Section
// 4.2.2 puts no length ceiling on a stream, so a reply on TCP, DoT or DoH is
// written whole and never carries TC.
//
// Msg.Truncate enables compression on a reply it has to shorten, and measures
// its octet budget against that compressed form, so the Compress=false half of
// shapeAuthoritative is deliberately not re-asserted after it. The invariant
// that matters survives: an answer func still cannot put compression on the
// wire, because only an oversized reply on a datagram transport gets it and the
// harness decides that, never fn. Forcing Compress=false back on would pack the
// reply longer than the budget Truncate measured, which is the bound this
// function exists to hold.
//
// A write that fails is counted and logged, never discarded. It is the
// transport refusing this one reply -- a full send buffer, a peer whose route
// went away, a stream the client already closed -- so there is nothing to
// retry: the querier retransmits or gives up, and the next query is
// unaffected. That is why the line is Debug rather than Error. A host that
// stops reading would otherwise put one line in an operator's log for every
// query it draws, which is the flood the counter exists to measure instead.
// The counter is also what survives a log level that hides the line, so it is
// what an operator alerts on.
func send(w dns.ResponseWriter, msg, r *dns.Msg, log *slog.Logger) {
	transport := transportStream
	if isDatagram(w) {
		transport = transportDatagram
		msg.Truncate(udpReplyLimit(r))
	}
	if err := w.WriteMsg(msg); err != nil {
		recordWriteFailure(transport)
		log.Debug("dnsserver: reply write failed", "transport", transport, logKeyError, err)
	}
}

// isDatagram reports whether the query arrived over a datagram transport, the
// only transport RFC 1035 Section 4.2.1 bounds.
//
// miekg/dns reports a *net.UDPAddr from both of its UDP read paths (the
// SessionUDP remote address, and the generic PacketConn source address). TCP,
// DoT and DoH all report a *net.TCPAddr -- DoH through dohResponseWriter, which
// synthesizes one from the HTTP peer so client-IP policy reads it identically.
func isDatagram(w dns.ResponseWriter) bool {
	if w == nil {
		return false
	}
	_, ok := w.RemoteAddr().(*net.UDPAddr)
	return ok
}

// udpReplyLimit returns the largest reply, in octets, that can go out over a
// datagram transport in answer to r.
//
// RFC 1035 Section 2.3.4 states the size limit as "UDP messages    512 octets
// or less", and that is a floor rather than a constant: RFC 6891 Section 6.2.3
// lets a requestor raise it by advertising its own reassembly buffer in an OPT
// record, and a responder answers up to the size the requestor offered. A query
// advertising less than 512 does not lower the bound, so a requestor cannot
// shrink a reply below what Section 2.3.4 already allows.
func udpReplyLimit(r *dns.Msg) int {
	limit := dns.MinMsgSize
	if opt := r.IsEdns0(); opt != nil {
		if advertised := int(opt.UDPSize()); advertised > limit {
			limit = advertised
		}
	}
	return limit
}

// shapeAuthoritative applies the authoritative-only answer shape -- the
// authoritative bit set on every reply that answers for a zone Ze serves,
// recursion never available, the reserved Z field zero, and no name
// compression. Authoritative calls it both before fn (so fn builds on a correct
// base) and after fn (so no answer func can leave the message
// non-authoritative, recursion-advertising, reserved-bit-dirty, or compressed),
// keeping the whole shape a single invariant defined in exactly one place.
//
// AA is the one bit of the shape that is not a constant, and the RCODE decides
// it. RFC 1035 Section 4.1.1 gives AA one meaning: "AA              Authoritative
// Answer - this bit is valid in responses, and specifies that the responding
// name server is an authority for the domain name in question section." The
// same section gives RCODE 5 the opposite one: "Refused - The name server
// refuses to perform the specified operation for policy reasons."
//
// An answer func returns Refused for a name under no zone it serves, and for a
// service the operator turned off. Neither reply is authoritative data about
// the name in the question, so AA is cleared for it here rather than at each
// call site. A responder that keeps AA set on a Refused reply asserts authority
// over a namespace it holds no zone for.
//
// RFC 1035 Section 4.1.1 states that Z must be zero in every query and every
// response. SetReply does not copy Z from the query, so the assignment here is
// what holds the field down against an answer func that sets it. A responder
// that leaks a reserved bit corrupts the one signal a later extension has.
//
// AD is the second of Z's three bits, reassigned by RFC 4035 Section 3.1.6,
// which permits it only on data the server has verified. Ze verifies nothing,
// so the whole field goes out zero apart from CD, which SetReply copies from
// the query for the resolver that asked. Both bits are held here rather than
// trusted to each answer func, which is the same reason AA and RA are.
//
// One step runs after the last call: send truncates an oversized datagram
// reply, and Msg.Truncate turns compression back on for exactly those replies
// because its octet budget is measured against the compressed form. That is the
// harness's own decision on a reply it must shorten, so what this function
// guarantees is unchanged -- no answer func can put compression on the wire.
func shapeAuthoritative(msg *dns.Msg) {
	msg.Authoritative = msg.Rcode != dns.RcodeRefused
	msg.RecursionAvailable = false
	msg.Zero = false
	msg.AuthenticatedData = false
	msg.Compress = false
}
