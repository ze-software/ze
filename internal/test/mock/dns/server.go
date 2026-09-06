// Design: docs/functional-tests.md -- the in-process form of the ze-test dns stub
// Related: zone.go -- the answers it serves; dns.go -- the ze-test dns subcommand
// RFC: rfc/short/rfc1035.md -- the RCODE values; rfc/full/rfc2308.txt -- NODATA

package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"

	mdns "github.com/miekg/dns"
)

// Server answers DNS queries from a zone a test controls.
//
// Safe for concurrent use: Set may be called while queries are being answered.
// The caller MUST call Close when done, which stops the server and releases the
// port. A caller that owns the whole process rather than one test step calls
// Wait instead, and Close is what makes Wait return.
type Server struct {
	mu   sync.RWMutex
	zone map[string]Name

	server *mdns.Server
	addr   netip.AddrPort
	// served carries the result of the one serving goroutine. It is buffered,
	// so the goroutine ends whether or not anybody calls Wait.
	served chan error
}

// Start binds addr ("127.0.0.1:53"; port 0 asks the OS to choose a port) and
// serves the default zone. It returns once the server is listening, so a caller
// may query it or Close it as soon as Start returns.
//
// A bind that fails returns the error, which names the address. It is never
// downgraded to a server that accepts nothing: every test using this stub reads
// its answers as fact, so a silent failure would make them lie
// (ai/rules/principles.md).
func Start(addr string) (*Server, error) {
	listen := &net.ListenConfig{}
	conn, err := listen.ListenPacket(context.Background(), "udp", addr)
	if err != nil {
		return nil, fmt.Errorf("dns stub: listen on %s: %w", addr, err)
	}

	bound, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		_ = conn.Close() //nolint:errcheck // the listen already failed; this is cleanup
		return nil, fmt.Errorf("dns stub: bound address %q: %w", conn.LocalAddr(), err)
	}

	server := &Server{zone: defaultZone(), addr: bound, served: make(chan error, 1)}
	listening := make(chan struct{})
	server.server = &mdns.Server{
		PacketConn:        conn,
		Handler:           mdns.HandlerFunc(server.handle),
		NotifyStartedFunc: func() { close(listening) },
	}

	// One goroutine for one lifecycle: this server. It ends when Close shuts
	// the library server down, and its result is what Wait reports. serveUDP
	// closes the PacketConn on the way out, so the port is released there.
	go func() { server.served <- server.server.ActivateAndServe() }()

	select {
	case <-listening:
		return server, nil
	case serveErr := <-server.served:
		if serveErr == nil {
			serveErr = errors.New("stopped before it started listening")
		}
		return nil, fmt.Errorf("dns stub: serve on %s: %w", addr, serveErr)
	}
}

// Addr is the address the server bound, so a caller can print it or point a
// daemon at it. It is meaningful as soon as Start returns.
func (s *Server) Addr() netip.AddrPort {
	return s.addr
}

// Set replaces what the stub answers for one name and one record type, in
// effect for the next query. A name the zone does not carry is added and
// answers NOERROR, so a fixture can serve a name this package never declared.
//
// It refuses an answer the record type cannot carry (checkAnswer), because the
// caller is a test author and a silently dropped record would make the test
// reading it assert the wrong thing.
func (s *Server) Set(name string, qtype uint16, answer Answer) {
	checkAnswer(qtype, answer)

	key := canonical(name)

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, held := s.zone[key]
	if !held {
		entry = Name{Rcode: mdns.RcodeSuccess, Answers: make(map[uint16]Answer)}
	}
	entry.Answers[qtype] = answer
	s.zone[key] = entry
}

// Close stops the server and releases the port. Wait returns once it has. The
// caller MUST NOT query the server after Close returns.
func (s *Server) Close() error {
	if err := s.server.Shutdown(); err != nil {
		return fmt.Errorf("dns stub: shutdown: %w", err)
	}
	return nil
}

// Wait blocks until the server stops serving and reports why it stopped. Close
// is what makes it return. One caller at a time: the process form (Run) waits,
// and a fixture calls Close instead.
func (s *Server) Wait() error {
	return <-s.served
}

// answerFor reports what the zone says about one question: the response code for
// the NAME, the answer for the TYPE, and whether the name carries a record of
// that type at all. All three are read under one lock, so a concurrent Set is
// never observed half applied. The returned Addresses slice is never mutated in
// place -- Set replaces the whole Answer -- so a reader may hold it after the
// lock is dropped.
func (s *Server) answerFor(name string, qtype uint16) (int, Answer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, held := s.zone[canonical(name)]
	if !held {
		// RFC 1035 Section 4.1.1: "Name Error - Meaningful only for responses
		// from an authoritative name server, this code signifies that the
		// domain name referenced in the query does not exist."
		return mdns.RcodeNameError, Answer{}, false
	}

	answer, carries := entry.Answers[qtype]
	return entry.Rcode, answer, carries
}

// handle answers one query. A message it cannot answer is REFUSED rather than
// dropped, the way handleCymruDNS (internal/test/mock/cymru/cymru.go) answers
// one: a silent drop reaches the caller as a timeout, which reads like a stub
// that never started.
func (s *Server) handle(w mdns.ResponseWriter, query *mdns.Msg) {
	reply := new(mdns.Msg)
	reply.SetReply(query)
	reply.Authoritative = true

	if len(query.Question) == 0 {
		reply.Rcode = mdns.RcodeRefused
		writeReply(w, query, reply)
		return
	}

	question := query.Question[0]
	rcode, answer, carries := s.answerFor(question.Name, question.Qtype)
	reply.Rcode = rcode

	// A name the zone carries without a record of this type leaves the answer
	// section empty and the code NOERROR. RFC 2308 Section 2.2: "NODATA is
	// indicated by an answer with the RCODE set to NOERROR and no relevant
	// answers in the answer section." Answering NXDOMAIN instead would say the
	// name is gone, and resolveAndRecord
	// (internal/component/firewall/plugins/domain/domain.go) would empty what
	// that name contributes to a live firewall set.
	if rcode == mdns.RcodeSuccess && carries {
		for _, address := range answer.Addresses {
			reply.Answer = append(reply.Answer, record(question, address, answer.TTL))
		}
	}

	writeReply(w, query, reply)
}

// record builds the resource record one address answers with. checkAnswer has
// already refused an address whose family does not match the record type, and
// the only types it accepts are A and AAAA, so the two cases here are total.
func record(question mdns.Question, address netip.Addr, ttl uint32) mdns.RR {
	header := mdns.RR_Header{
		Name:   question.Name,
		Rrtype: question.Qtype,
		Class:  mdns.ClassINET,
		Ttl:    ttl,
	}
	if question.Qtype == mdns.TypeAAAA {
		return &mdns.AAAA{Hdr: header, AAAA: net.IP(address.AsSlice())}
	}
	return &mdns.A{Hdr: header, A: net.IP(address.AsSlice())}
}

// writeReply sends the reply, bounded by what the requester said it can
// receive: 512 octets by default, or the EDNS0 buffer size it advertised. A
// reply past that bound is not deliverable, and the TC bit is how DNS says so.
// Ze's own resolver advertises 4096 and logs a truncated answer rather than
// retrying it over TCP (Resolver.query,
// internal/component/resolve/dns/resolver.go), which is why this stub serves no
// TCP listener.
func writeReply(w mdns.ResponseWriter, query, reply *mdns.Msg) {
	size := mdns.MinMsgSize
	if opt := query.IsEdns0(); opt != nil {
		if advertised := int(opt.UDPSize()); advertised > size {
			size = advertised
		}
	}
	reply.Truncate(size)

	_ = w.WriteMsg(reply) //nolint:errcheck // a client that vanished mid-test is not a stub failure
}
