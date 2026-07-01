// Design: plan/spec-dns-server-harness.md -- authoritative-answer shaping,
// the single-source-of-truth recursion guard (child-2 R-3)
// RFC: rfc/short/rfc1035.md -- DNS message structure

package dnsserver

import "github.com/miekg/dns"

// AnswerFunc builds and writes the reply for one query. msg already has
// SetReply/Authoritative/Compress/RecursionAvailable applied by
// Authoritative; fn is responsible for calling w.WriteMsg at whatever
// point(s) it wants a reply sent (including never, if it intends to drop the
// query -- e.g. an unresolvable client source).
type AnswerFunc func(msg, r *dns.Msg, w dns.ResponseWriter)

// Authoritative wraps fn as a dns.HandlerFunc providing the RFC 1035
// authoritative-answer shape and the single-source-of-truth panic-recovery
// guard: SetReply, Authoritative=true, Compress=false, RecursionAvailable
// explicitly never set. If fn panics, onPanic (if non-nil) receives the
// recovered value and no reply is written for that query -- one bad query can
// never crash the listener, and can never receive a malformed or
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
		msg.Authoritative = true
		msg.RecursionAvailable = false
		msg.Compress = false
		fn(msg, r, w)
	}
}
