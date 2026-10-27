// Design: docs/architecture/testing/ci-format.md -- the harness command `le dns query`
// Related: ../mock/dns/dns.go -- `le mock dns`, the upstream a resolver test points at

// Package dnsquery is the DNS query client functional tests drive: it sends
// one query with the header bits and OPT a test names, and prints every
// field of the reply a test asserts on, header bits included, which the
// daemon's own stub resolver never shows.
package dnsquery

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// defaultPort is the DNS port (RFC 1035 Section 4.2.1), used when
	// --server names an address only.
	defaultPort = 53
	// defaultEDNSSize is the OPT size a query offers unless --edns or
	// --no-edns says otherwise.
	defaultEDNSSize = 1232
	// attemptTimeout bounds one exchange. --wait retries within its own
	// budget, so a slow first attempt does not end the run.
	attemptTimeout = 2 * time.Second
	// retryPause spaces the attempts --wait makes: a listener that is not
	// bound yet refuses at once, and the pause keeps the loop from spinning.
	retryPause = 100 * time.Millisecond
	// opcodeMax is the largest value of the four-bit OPCODE field (RFC 1035
	// Section 4.1.1).
	opcodeMax = 15
)

const usageText = `Usage: le dns query --server <ip>[:port] --name <name> [flags]

Sends one DNS query with the header bits and OPT named below and prints the
reply, one field per line, so a functional test can assert on header bits:

  rcode <RCODE>
  flags <the ones set, of qr aa tc rd ra ad cd>
  size <octets on the wire>
  edns size <n> do <0 or 1>     (only when the reply carries an OPT)
  <section> <record>            (question, answer, authority or additional)

Exits 0 on any reply, whatever its RCODE, and 1 with the reason when none
arrives within --wait.

Flags:
`

// query is one parsed command line.
type query struct {
	server   netip.AddrPort
	source   netip.Addr
	tcp      bool
	name     string
	qtype    uint16
	rd       bool
	cd       bool
	ad       bool
	do       bool
	ednsSize uint16 // 0: no OPT
	opcode   int
	wait     time.Duration
}

// Run sends one query and prints the reply to stdout. It exits 0 on any
// reply, whatever its RCODE, and 1 with the reason on stderr when no reply
// arrives or the command line is wrong.
func Run(args []string) int {
	return run(args, os.Stdout, os.Stderr)
}

// run is Run over the given streams, so a test reads what a .ci would.
func run(args []string, out, errOut io.Writer) int {
	q, err := parseArgs(args, errOut)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		writeLine(errOut, textbuf.New().Str("error: ").Err(err))
		return 1
	}
	got, err := exchange(q)
	if err != nil {
		writeLine(errOut, textbuf.New().Str("error: no reply from ").Str(q.server.String()).Str(": ").Err(err))
		return 1
	}
	if werr := writeReply(out, got); werr != nil {
		writeLine(errOut, textbuf.New().Str("error: write: ").Err(werr))
		return 1
	}
	return 0
}

// parseArgs reads the command line. flag has already written its own reason
// to usage when it refuses one.
func parseArgs(args []string, usage io.Writer) (query, error) {
	fs := flag.NewFlagSet("le dns query", flag.ContinueOnError)
	fs.SetOutput(usage)

	var (
		server, source, name, qtype string
		edns, opcode                int
		noEDNS                      bool
		q                           query
	)
	fs.StringVar(&server, "server", "", "destination, ip or ip:port (port 53 when absent); required")
	fs.StringVar(&source, "source", "", "source address the query is sent from")
	fs.BoolVar(&q.tcp, "tcp", false, "send over TCP instead of UDP")
	fs.StringVar(&name, "name", "", "question name; required")
	fs.StringVar(&qtype, "type", "A", "question type, by mnemonic (A, AAAA, MX, ...)")
	fs.BoolVar(&q.rd, "rd", false, "set RD (recursion desired)")
	fs.BoolVar(&q.cd, "cd", false, "set CD (checking disabled)")
	fs.BoolVar(&q.ad, "ad", false, "set AD (authentic data)")
	fs.BoolVar(&q.do, "do", false, "set DO (DNSSEC OK) in the OPT")
	fs.IntVar(&edns, "edns", defaultEDNSSize, "UDP payload size the OPT offers")
	fs.BoolVar(&noEDNS, "no-edns", false, "send no OPT record")
	fs.IntVar(&opcode, "opcode", dns.OpcodeQuery, "OPCODE, 0 to 15")
	fs.DurationVar(&q.wait, "wait", 0, "keep retrying an unanswered query for this long, for a server still starting")
	fs.Usage = func() {
		_, _ = io.WriteString(usage, usageText) //nolint:errcheck // usage text on a stream with no reader to tell
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return query{}, err
	}
	// A trailing help word is how le forwards a help request to a harness
	// tool (internal/le/test/harnesstool), so it is answered, not refused.
	if fs.NArg() > 0 && fs.Arg(fs.NArg()-1) == "help" {
		fs.Usage()
		return query{}, flag.ErrHelp
	}
	if fs.NArg() > 0 {
		return query{}, fmt.Errorf("unexpected argument %q: every input is a flag (le dns query --help)", fs.Arg(0))
	}

	addr, err := parseServer(server)
	if err != nil {
		return query{}, err
	}
	q.server = addr
	if source != "" {
		src, serr := netip.ParseAddr(source)
		if serr != nil {
			return query{}, fmt.Errorf("--source %q is not an IP address", source)
		}
		q.source = src
	}
	if name == "" {
		return query{}, errors.New("--name is required, for example --name www.example.test")
	}
	q.name = dns.Fqdn(name)
	t, ok := dns.StringToType[strings.ToUpper(qtype)]
	if !ok {
		return query{}, fmt.Errorf("--type %q is not a record type mnemonic, for example A or AAAA", qtype)
	}
	q.qtype = t
	if opcode < 0 || opcode > opcodeMax {
		return query{}, fmt.Errorf("--opcode %d is outside 0-%d", opcode, opcodeMax)
	}
	q.opcode = opcode
	if noEDNS {
		if q.do {
			return query{}, errors.New("--do needs an OPT record, so it cannot be combined with --no-edns")
		}
		return q, nil
	}
	if edns < dns.MinMsgSize || edns > dns.MaxMsgSize {
		return query{}, fmt.Errorf("--edns %d is outside %d-%d", edns, dns.MinMsgSize, dns.MaxMsgSize)
	}
	q.ednsSize = uint16(edns)
	return q, nil
}

// parseServer reads --server as ip or ip:port; an IPv6 address with a port
// is written [ip]:port.
func parseServer(server string) (netip.AddrPort, error) {
	if server == "" {
		return netip.AddrPort{}, errors.New("--server is required, for example --server 127.0.0.1:5353")
	}
	if addr, err := netip.ParseAddr(server); err == nil {
		return netip.AddrPortFrom(addr, defaultPort), nil
	}
	ap, err := netip.ParseAddrPort(server)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("--server %q is neither ip nor ip:port", server)
	}
	return ap, nil
}

// message builds the query q describes.
func message(q query) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(q.name, q.qtype)
	m.Opcode = q.opcode
	m.RecursionDesired = q.rd
	m.CheckingDisabled = q.cd
	m.AuthenticatedData = q.ad
	if q.ednsSize != 0 {
		m.SetEdns0(q.ednsSize, q.do)
	}
	return m
}

// reply is one received message and its size on the wire.
type reply struct {
	msg  *dns.Msg
	size int
}

// exchange sends q until a reply arrives or the --wait budget ends. Every
// attempt is a new message with a new ID on a new socket, so a late reply to
// an earlier attempt is never taken for this one's. The loop ends at the
// deadline: each attempt is bounded by attemptTimeout.
func exchange(q query) (reply, error) {
	c := client(q)
	deadline := time.Now().Add(q.wait)
	for {
		got, err := exchangeOnce(c, q)
		if err == nil {
			return got, nil
		}
		if !time.Now().Before(deadline) {
			return reply{}, err
		}
		time.Sleep(retryPause)
	}
}

func exchangeOnce(c *dns.Client, q query) (reply, error) {
	conn, err := c.Dial(q.server.String())
	if err != nil {
		return reply{}, err
	}
	defer func() { _ = conn.Close() }()
	m, _, err := c.ExchangeWithConn(message(q), conn)
	if err != nil {
		return reply{}, err
	}
	wire, err := m.Pack()
	if err != nil {
		return reply{}, fmt.Errorf("repack reply: %w", err)
	}
	return reply{msg: m, size: len(wire)}, nil
}

// client returns the dns.Client for q: its transport, its per-attempt
// timeout, its source address, and a UDP buffer large enough for the OPT
// size the query offers.
func client(q query) *dns.Client {
	c := &dns.Client{Net: "udp", Timeout: attemptTimeout, UDPSize: max(q.ednsSize, dns.MinMsgSize)}
	if q.tcp {
		c.Net = "tcp"
	}
	d := &net.Dialer{Timeout: attemptTimeout}
	if q.source.IsValid() {
		if q.tcp {
			d.LocalAddr = &net.TCPAddr{IP: q.source.AsSlice()}
		} else {
			d.LocalAddr = &net.UDPAddr{IP: q.source.AsSlice()}
		}
	}
	c.Dialer = d
	return c
}

// flagNames lists the header bits the flags line can print, in wire order.
var flagNames = [...]string{"qr", "aa", "tc", "rd", "ra", "ad", "cd"}

// writeReply prints the reply one field per line. No line holds a colon
// outside a record or an IPv6 address, because a .ci needle stops at the
// first one.
func writeReply(w io.Writer, got reply) error {
	m := got.msg
	b := textbuf.New()
	b.Str("rcode ").Str(dns.RcodeToString[m.Rcode]).Str("\nflags")
	set := [...]bool{m.Response, m.Authoritative, m.Truncated, m.RecursionDesired,
		m.RecursionAvailable, m.AuthenticatedData, m.CheckingDisabled}
	for i, on := range set {
		if on {
			b.Byte(' ').Str(flagNames[i])
		}
	}
	b.Str("\nsize ").Int(int64(got.size)).Byte('\n')
	if opt := m.IsEdns0(); opt != nil {
		b.Str("edns size ").Uint16(opt.UDPSize()).Str(" do ").Str(bit(opt.Do())).Byte('\n')
	}
	for _, question := range m.Question {
		b.Str("question ").Str(question.Name).Byte(' ').
			Str(dns.ClassToString[question.Qclass]).Byte(' ').
			Str(dns.TypeToString[question.Qtype]).Byte('\n')
	}
	writeSection(b, "answer", m.Answer)
	writeSection(b, "authority", m.Ns)
	writeSection(b, "additional", m.Extra)
	_, err := w.Write(b.Bytes())
	return err
}

// writeSection prints each record of a section in presentation form, tabs
// turned to spaces, leaving out the OPT pseudo-record the edns line shows.
func writeSection(b *textbuf.Buffer, section string, rrs []dns.RR) {
	for _, rr := range rrs {
		if _, isOPT := rr.(*dns.OPT); isOPT {
			continue
		}
		b.Str(section).Byte(' ').Str(strings.ReplaceAll(rr.String(), "\t", " ")).Byte('\n')
	}
}

func bit(set bool) string {
	if set {
		return "1"
	}
	return "0"
}

// writeLine writes one diagnostic line. A stream that refuses it has no
// reader left to tell.
func writeLine(w io.Writer, b *textbuf.Buffer) {
	b.Byte('\n')
	_, _ = w.Write(b.Bytes()) //nolint:errcheck // a refused diagnostic has no reader left to tell
}
