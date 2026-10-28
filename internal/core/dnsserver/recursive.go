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
// A query carrying more than one OPT RR draws Format error here, first of
// all (RFC 6891 Section 6.1.1). Otherwise an EDNS query whose one OPT names a
// version other than 0 draws BADVERS, before the opcode check (RFC 6891
// Section 6.1.3). Neither reaches fn.
//
// rejected, when not nil, is called once for each reply Recursive writes
// without consulting fn (the Format error, Not Implemented and BADVERS
// replies above), so
// the consumer counts every query it answers and not only those fn saw.
//
// If fn panics, queryGuard contains it as it does for Authoritative: the panic
// is logged, counted and recorded, and no reply is written.
//
// A nil log discards the write-failure line (see send), never the panic line.
func Recursive(log *slog.Logger, fn AnswerFunc, rejected RejectFunc) dns.HandlerFunc {
	if log == nil {
		log = slogutil.DiscardLogger()
	}
	task := queryGuard.Task(taskRecursiveQuery, queryRestart)
	return func(w dns.ResponseWriter, r *dns.Msg) {
		defer task.Contain(describeQuery(w, r), nil)
		var opt *dns.OPT
		for _, rr := range r.Extra {
			found, isOPT := rr.(*dns.OPT)
			if !isOPT {
				continue
			}
			if opt != nil {
				// RFC 6891 Section 6.1.1: "If a query message with more than one
				// OPT RR is received, a FORMERR (RCODE=1) MUST be returned."
				rejectRecursive(w, r, dns.RcodeFormatError, rejected, log)
				return
			}
			opt = found
		}
		if opt != nil {
			// RFC 6891 Section 6.1.3: "If a responder does not implement the
			// VERSION level of the request, then it MUST respond with
			// RCODE=BADVERS."
			if opt.Version() != recursiveEDNSVersion {
				rejectRecursive(w, r, dns.RcodeBadVers, rejected, log)
				return
			}
		}
		if r.Opcode != dns.OpcodeQuery {
			// RFC 1035 Section 6.4
			rejectRecursive(w, r, dns.RcodeNotImplemented, rejected, log)
			return
		}
		msg := new(dns.Msg)
		msg.SetReply(r)
		shapeRecursive(msg, r)
		if !fn(msg, r, w) {
			return
		}
		shapeRecursive(msg, r)
		send(w, msg, recursiveReplyLimit(r), log)
	}
}

// RejectFunc observes a reply Recursive wrote without calling the answer
// func: p is the client and rcode the reply's response code.
type RejectFunc func(p Peer, rcode int)

// recursiveEDNSVersion is the EDNS version this service implements, the
// only one RFC 6891 defines. A reply OPT carries it, because setReplyOPT
// never sets another, so every reply is limited in format to version 0.
const recursiveEDNSVersion = 0

// rejectRecursive writes the reply Recursive gives r without consulting the
// answer func, rcode in the recursive shape, and tells rejected of it. An
// extended rcode such as BADVERS needs the reply OPT to carry its upper bits,
// and shapeRecursive adds one exactly when r carried one, which is the only
// case that draws it.
func rejectRecursive(w dns.ResponseWriter, r *dns.Msg, rcode int, rejected RejectFunc, log *slog.Logger) {
	msg := new(dns.Msg)
	msg.SetRcode(r, rcode)
	shapeRecursive(msg, r)
	send(w, msg, recursiveReplyLimit(r), log)
	if rejected != nil {
		rejected(w, msg.Rcode)
	}
}

// headerQR is the QR bit of the header flags word (dns.Header.Bits).
//
// RFC 1035 Section 4.1.1: "QR              A one bit field that specifies
// whether this message is a query (0), or a response (1)."
//
//	                                1  1  1  1  1  1
//	  0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//	|QR|   Opcode  |AA|TC|RD|RA|   Z    |   RCODE   |   octets 2-3
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
const headerQR = 1 << 15

// recursiveQuestionsMax is the most questions RecursiveAccept hands to the
// handler. Every client asks one, and the handler answers any other count
// with Format error in the recursive shape; the bound only stops a header
// that claims thousands of questions from being unpacked before that answer.
const recursiveQuestionsMax = 16

// RecursiveAccept is the header filter a recursive service sets as
// Options.MsgAcceptFunc. The DNS library's default filter answers a query
// whose opcode is not QUERY or NOTIFY, or whose question count is not one,
// with its own NOTIMP or FORMERR before the handler runs, so the reply misses
// the recursive shape (RA=0) and the consumer never counts it. This filter
// lets those queries through to Recursive and the answer func, which answer
// them shaped and counted.
//
// It keeps the default filter's other protections, so a message whose
// sections the library would unpack at length never reaches the handler:
// a response is ignored (answering it could start a reply loop), and a
// header claiming more than recursiveQuestionsMax questions, more than one
// answer or authority record, or more than two additional records is
// rejected with the library's FORMERR, unshaped. An UPDATE carrying more than
// one prerequisite or update record is refused there for the same reason.
func RecursiveAccept(dh dns.Header) dns.MsgAcceptAction {
	if dh.Bits&headerQR != 0 {
		return dns.MsgIgnore
	}
	if dh.Qdcount > recursiveQuestionsMax {
		return dns.MsgReject
	}
	if dh.Ancount > 1 {
		return dns.MsgReject
	}
	if dh.Nscount > 1 {
		return dns.MsgReject
	}
	if dh.Arcount > 2 {
		return dns.MsgReject
	}
	return dns.MsgAccept
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
