// Design: docs/architecture/dns/server-harness.md -- the pass-through of a query carrying transaction security
// RFC: rfc/short/rfc5625.md -- DNS proxy guidelines, Section 4.5
// Related: recursive.go -- the wrapper that hands such a query to the pass-through
// Related: manager.go -- the listeners that keep the received copy

package dnsserver

import (
	"encoding/binary"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// PassFunc forwards a query that carries transaction security (a TSIG RR or
// a TKEY RR) and returns the reply exactly as the upstream sent it, to be
// written to the client unchanged.
//
// wire is the query as the client sent it: for a signed query the copy the
// listener kept, byte for byte, and for an unsigned TKEY query the parsed
// query packed again. msg is the reply skeleton in the recursive shape, as
// for an AnswerFunc. The func returns reply=nil with send=true to have msg
// written instead (a refusal, a SERVFAIL), and send=false to drop the query.
type PassFunc func(msg *dns.Msg, wire []byte, r *dns.Msg, p Peer) (reply []byte, send bool)

// PassThrough is the recursive service's route for a query carrying
// transaction security: Signed holds the bytes each listener received, and
// Forward sends them on. A service sets both, and passes the same Signed as
// Options.Signed, so its listeners keep the copies Forward needs.
type PassThrough struct {
	Signed  *SignedQueries
	Forward PassFunc
}

// carriesTransactionSecurity reports whether r is a TSIG-signed query, a
// TKEY query, or a query carrying a TKEY RR: the queries the pass-through
// forwards unchanged. dnsmasq draws the same line (find_pseudoheader in
// src/edns0.c sets is_sign for a TKEY question in class IN or a TSIG RR as
// the last additional record, and then "we cannot change a single bit
// before forwarding"). The class is not judged here: a TKEY question in
// class ANY is still a key exchange.
func carriesTransactionSecurity(r *dns.Msg) bool {
	if r.IsTsig() != nil {
		return true
	}
	for _, q := range r.Question {
		if q.Qtype == dns.TypeTKEY {
			return true
		}
	}
	for _, rr := range r.Extra {
		if rr.Header().Rrtype == dns.TypeTKEY {
			return true
		}
	}
	return false
}

// passThrough answers r, a query carrying transaction security, through
// pass. msg is the reply skeleton in the recursive shape.
//
// RFC 5625 Section 4.5: "DNS proxies MUST implement Section 4.7 of [RFC2845]
// and either forward packets unchanged (as recommended above) or fully
// implement TSIG."
//
// RFC 8945 Section 5.5: "If the name on the TSIG is not of a secret that the
// server shares with the originator, the server MUST forward the message
// unchanged including the TSIG."
//
// Ze shares no TSIG secret with a LAN client, so every signed query is
// forwarded unchanged, and the upstream's reply is written back unchanged:
// a reply the shape touched (AA, RA, AD, the OPT) would fail the client's
// check of the upstream's TSIG. A signed query whose received copy is gone
// (more signed queries in flight than SignedQueries holds, or a transport
// whose listener keeps none) is answered SERVFAIL rather than forwarded from
// the parsed form, which could differ from what the client signed.
func passThrough(w dns.ResponseWriter, r, msg *dns.Msg, pass *PassThrough, rejected RejectFunc, log *slog.Logger) {
	wire, ok := passWire(w, r, pass.Signed)
	if !ok {
		// RFC 1035 Section 4.1.1: "Server failure - The name server was
		// unable to process this query due to a problem with the name
		// server."
		msg.Rcode = dns.RcodeServerFailure
		send(w, msg, recursiveReplyLimit(r), log)
		if rejected != nil {
			rejected(w, msg.Rcode)
		}
		return
	}
	reply, sendReply := pass.Forward(msg, wire, r, w)
	if !sendReply {
		return
	}
	if reply == nil {
		shapeRecursive(msg, r)
		send(w, msg, recursiveReplyLimit(r), log)
		return
	}
	sendUnchanged(w, r, reply, log)
}

// passWire returns the bytes to forward for r: the copy the listener kept
// for a signed query, and r packed again for an unsigned TKEY query, whose
// bytes no signature covers.
func passWire(w dns.ResponseWriter, r *dns.Msg, signed *SignedQueries) ([]byte, bool) {
	t := r.IsTsig()
	if t == nil {
		wire, err := r.Pack()
		if err != nil {
			// The query was unpacked a moment ago, so it packs; a failure
			// is answered SERVFAIL by the caller, never forwarded.
			return nil, false
		}
		return wire, true
	}
	if signed == nil {
		return nil, false
	}
	return signed.take(w.RemoteAddr(), t.MAC)
}

// sendUnchanged writes reply, the upstream's bytes, to the client.
//
// RFC 5625 Section 4.4: "If a proxy must unilaterally truncate a response,
// then the proxy MUST set the TC bit."
//
// RFC 5625 Section 4.4: "Similarly, proxies MUST NOT remove the TC bit from
// responses."
//
// The reply is written whole, its TC bit as the upstream set it. Only a
// datagram reply longer than the client offered (udpReplyLimit: the client's
// OPT reached the upstream unchanged, so this is rare) is cut, by
// Msg.Truncate, which sets TC; the client then asks again over TCP.
func sendUnchanged(w dns.ResponseWriter, r *dns.Msg, reply []byte, log *slog.Logger) {
	if isDatagram(w) {
		if limit := udpReplyLimit(r); len(reply) > limit {
			m := new(dns.Msg)
			if err := m.Unpack(reply); err != nil {
				log.Warn("dnsserver: an oversized pass-through reply does not parse; dropped", logKeyError, err)
				return
			}
			send(w, m, limit, log)
			return
		}
	}
	if _, err := w.Write(reply); err != nil {
		recordWriteFailure(transportOf(w))
		log.Debug("dnsserver: pass-through reply write failed", logKeyError, err)
	}
}

// transportOf names w's transport for the write-failure counter.
func transportOf(w dns.ResponseWriter) string {
	if isDatagram(w) {
		return transportDatagram
	}
	return transportStream
}

// signedQueriesMax is how many received signed queries SignedQueries holds
// at once. A query's copy is taken by its own handler a moment after the
// read, so the ring only has to cover the signed queries in flight; the
// oldest copy is overwritten past it, and its query is answered SERVFAIL.
const signedQueriesMax = 64

// signedQueryAge is how long a kept copy may wait for its handler. A copy
// the handler never takes (a query the header filter rejected) is not
// handed to a later query that reuses its address and MAC.
const signedQueryAge = 5 * time.Second

// SignedQueries keeps the bytes of each TSIG-signed query a listener reads,
// so the pass-through forwards what the client signed rather than the parsed
// message packed again, whose name compression can differ. Safe for
// concurrent use: every listener's reader and every handler share it.
type SignedQueries struct {
	mu    sync.Mutex
	slots [signedQueriesMax]signedQuery
	next  int
}

// signedQuery is one kept copy, found by the client's address and the
// query's TSIG MAC.
type signedQuery struct {
	remote string
	mac    string
	wire   []byte
	read   time.Time
}

// NewSignedQueries returns an empty store.
func NewSignedQueries() *SignedQueries { return &SignedQueries{} }

// keep stores a copy of m, a signed query read from remote.
func (s *SignedQueries) keep(remote net.Addr, m []byte) {
	wire := slices.Clone(m)
	parsed := new(dns.Msg)
	if parsed.Unpack(wire) != nil {
		// The server's own unpack refuses it too, so no handler asks.
		return
	}
	t := parsed.IsTsig()
	if t == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slots[s.next] = signedQuery{remote: remote.String(), mac: t.MAC, wire: wire, read: time.Now()}
	s.next = (s.next + 1) % signedQueriesMax
}

// take returns and forgets the copy of the signed query from remote whose
// TSIG MAC is mac (hex, as dns.TSIG carries it). A copy older than
// signedQueryAge is forgotten on the way.
func (s *SignedQueries) take(remote net.Addr, mac string) ([]byte, bool) {
	key := remote.String()
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.slots {
		q := &s.slots[i]
		if q.wire == nil {
			continue
		}
		if now.Sub(q.read) > signedQueryAge {
			*q = signedQuery{}
			continue
		}
		if q.remote != key {
			continue
		}
		if q.mac != mac {
			continue
		}
		wire := q.wire
		*q = signedQuery{}
		return wire, true
	}
	return nil, false
}

// decorate wraps a listener's reader so it keeps every signed query it reads.
func (s *SignedQueries) decorate(inner dns.Reader) dns.Reader {
	return &signedReader{inner: inner, signed: s}
}

// signedReader is the dns.Reader that keeps signed queries. It implements
// dns.PacketConnReader too, which the library needs for a generic
// net.PacketConn.
type signedReader struct {
	inner  dns.Reader
	signed *SignedQueries
}

var _ dns.PacketConnReader = (*signedReader)(nil)

// ReadTCP reads one message from a stream and keeps it when it is signed.
func (sr *signedReader) ReadTCP(conn net.Conn, timeout time.Duration) ([]byte, error) {
	m, err := sr.inner.ReadTCP(conn, timeout)
	if err == nil && lastIsTSIG(m) {
		sr.signed.keep(conn.RemoteAddr(), m)
	}
	return m, err
}

// ReadUDP reads one datagram and keeps it when it is signed.
func (sr *signedReader) ReadUDP(conn *net.UDPConn, timeout time.Duration) ([]byte, *dns.SessionUDP, error) {
	m, session, err := sr.inner.ReadUDP(conn, timeout)
	if err == nil && lastIsTSIG(m) {
		sr.signed.keep(session.RemoteAddr(), m)
	}
	return m, session, err
}

// ReadPacketConn reads one datagram from a generic packet conn and keeps it
// when it is signed.
func (sr *signedReader) ReadPacketConn(conn net.PacketConn, timeout time.Duration) ([]byte, net.Addr, error) {
	pr, ok := sr.inner.(dns.PacketConnReader)
	if !ok {
		panic("BUG: dnsserver: the library's reader is not a PacketConnReader")
	}
	m, from, err := pr.ReadPacketConn(conn, timeout)
	if err == nil && lastIsTSIG(m) {
		sr.signed.keep(from, m)
	}
	return m, from, err
}

// dnsHeaderOctets is the length of the DNS header.
const dnsHeaderOctets = 12

// lastIsTSIG reports whether the last record of m is a TSIG RR, without
// unpacking m: the reader runs it on every query, and only a signed one is
// copied and parsed.
//
// RFC 8945 Section 4.2: "If a TSIG record is present in any other position,
// the DNS message is dropped and a response with RCODE 1 (FORMERR) MUST be
// returned."
//
// The TSIG record is the last one of the additional section, which is the
// last record of the message, so only that position is looked at.
//
//	header (12) | questions: name, type (2), class (2) |
//	records: name, type (2), class (2), TTL (4), RDLENGTH (2), RDATA
//
// Every step advances the offset by at least one octet and fails past the
// end of m, so the walk is bounded by len(m).
func lastIsTSIG(m []byte) bool {
	if len(m) < dnsHeaderOctets {
		return false
	}
	questions := int(binary.BigEndian.Uint16(m[4:]))
	records := int(binary.BigEndian.Uint16(m[6:])) + int(binary.BigEndian.Uint16(m[8:])) + int(binary.BigEndian.Uint16(m[10:]))
	if binary.BigEndian.Uint16(m[10:]) == 0 {
		return false
	}
	off := dnsHeaderOctets
	for range questions {
		off = skipName(m, off)
		if off < 0 {
			return false
		}
		off += 4
	}
	for i := range records {
		off = skipName(m, off)
		if off < 0 {
			return false
		}
		if off+10 > len(m) {
			return false
		}
		rrtype := binary.BigEndian.Uint16(m[off:])
		next := off + 10 + int(binary.BigEndian.Uint16(m[off+8:]))
		if next > len(m) {
			return false
		}
		if i == records-1 {
			return rrtype == dns.TypeTSIG
		}
		off = next
	}
	return false
}

// skipName returns the offset just past the domain name at off in m, or -1
// when the name runs past the end of m or holds a label type RFC 1035 does
// not define. A compression pointer ends the name (RFC 1035 Section 4.1.4).
func skipName(m []byte, off int) int {
	for off < len(m) {
		label := int(m[off])
		switch label & 0xC0 {
		case 0x00:
			if label == 0 {
				return off + 1
			}
			off += 1 + label
		case 0xC0:
			if off+2 > len(m) {
				return -1
			}
			return off + 2
		default:
			return -1
		}
	}
	return -1
}
