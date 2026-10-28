// Design: docs/architecture/dns/lan-resolver.md -- one upstream exchange and reply matching
// Related: resolve.go -- the resolution that drives these exchanges

package dnsresolver

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// upstreamUDPSize is the EDNS0 buffer the resolver advertises upstream, and
// the read buffer it receives a datagram reply into.
const upstreamUDPSize = 1232

var (
	errUpstreamTimeout = errors.New("upstream did not answer in time")
	errReplyMismatch   = errors.New("upstream reply does not match the query")
)

// upstreamQuery builds the query sent upstream for q with identifier id.
//
// RFC 1034 Section 4.3.1: "Note that the name server should never perform
// recursive service unless asked via RD, since this interferes with trouble
// shooting of name servers and their databases."
//
// The resolver is not security-aware: its OPT advertises upstreamUDPSize
// with DO clear, so the upstream returns no DNSSEC records it was not asked
// for, and CD is never forwarded.
func upstreamQuery(q dns.Question, id uint16) *dns.Msg {
	m := new(dns.Msg)
	m.Id = id
	m.Opcode = dns.OpcodeQuery
	m.RecursionDesired = true
	m.Question = []dns.Question{q}
	m.SetEdns0(upstreamUDPSize, false)
	return m
}

// replyMatches reports whether r answers query.
//
// RFC 5452 Section 9.1: "A resolver implementation MUST match responses to
// all of the following attributes of the query:"
//
// The connected socket the exchange reads from matches the addresses and
// ports; this matches the identifier, the name, the class and the type.
// RFC 5452 Section 9.1: "A mismatch and the response MUST be considered
// invalid."
func replyMatches(query, r *dns.Msg) bool {
	if !r.Response {
		return false
	}
	if r.Id != query.Id {
		return false
	}
	if len(r.Question) != 1 {
		return false
	}
	want, got := query.Question[0], r.Question[0]
	if got.Qtype != want.Qtype {
		return false
	}
	if got.Qclass != want.Qclass {
		return false
	}
	return strings.EqualFold(got.Name, want.Name)
}

// The two DNS transports, as net names them: the network of an exchange and
// the protocol of a listener edge.
const (
	networkUDP = "udp"
	networkTCP = "tcp"
)

// exchange sends query to up over network ("udp" or "tcp") and returns the
// first reply that matches it, before deadline.
//
// RFC 5452 Section 9.2: "Use an unpredictable source port for outgoing
// queries from the range of available ports (53, or 1024 and above) that is
// as large as possible and practicable;"
//
// Each exchange dials its own socket, so the kernel picks a fresh
// ephemeral source port for it, and the socket is connected, so a datagram
// from any other address or port never reaches the read. On UDP a reply
// that does not match is ignored and the read goes on until the deadline;
// on TCP the stream carries one reply, so a mismatch fails the exchange.
func exchange(network string, up netip.AddrPort, query *dns.Msg, deadline time.Time) (*dns.Msg, error) {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	var d net.Dialer
	c, err := d.DialContext(ctx, network, up.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()

	if err := c.SetDeadline(deadline); err != nil {
		return nil, err
	}
	co := &dns.Conn{Conn: c, UDPSize: upstreamUDPSize}
	if err := co.WriteMsg(query); err != nil {
		return nil, err
	}
	// Bounded by the socket deadline: every read returns by then.
	for {
		r, err := co.ReadMsg()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return nil, errUpstreamTimeout
			}
			if network != networkUDP {
				return nil, err
			}
			if errors.As(err, &ne) {
				return nil, err
			}
			// A datagram that does not parse is not the reply; keep reading.
			continue
		}
		if replyMatches(query, r) {
			return r, nil
		}
		if network != networkUDP {
			return nil, errReplyMismatch
		}
	}
}
