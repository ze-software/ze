// Design: docs/architecture/dns/server-harness.md -- authoritative-answer
// shaping, the single-source-of-truth recursion guard (child-2 R-3)
// RFC: rfc/short/rfc1035.md -- DNS message structure

package dnsserver

import (
	"net"

	"github.com/miekg/dns"
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
// Authoritative=true, Compress=false, RecursionAvailable=false). p is a
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
// authoritative-answer shape, the single wire write, and the panic-recovery
// guard. It shapes msg before fn (so fn builds on a correct base) and
// re-asserts the same shape after fn (so no answer func can advertise
// recursion, clear the authoritative bit, or enable compression), then writes
// msg exactly once when fn returns send=true. If fn panics, onPanic (if
// non-nil) receives the recovered value and no reply is written -- one bad
// query can never crash the listener, nor receive a malformed or
// accidentally-recursive answer.
func Authoritative(fn AnswerFunc, onPanic func(any)) dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		defer func() {
			if rec := recover(); rec != nil && onPanic != nil {
				onPanic(rec)
			}
		}()
		msg := new(dns.Msg)
		msg.SetReply(r)
		shapeAuthoritative(msg)
		if !fn(msg, r, w) {
			return
		}
		shapeAuthoritative(msg)
		_ = w.WriteMsg(msg)
	}
}

// shapeAuthoritative applies the authoritative-only answer shape -- the
// authoritative bit set, recursion never available, and no name compression.
// Authoritative calls it both before fn (so fn builds on a correct base) and
// after fn (so no answer func can leave the message non-authoritative,
// recursion-advertising, or compressed), keeping the whole shape a single
// invariant defined in exactly one place.
func shapeAuthoritative(msg *dns.Msg) {
	msg.Authoritative = true
	msg.RecursionAvailable = false
	msg.Compress = false
}
