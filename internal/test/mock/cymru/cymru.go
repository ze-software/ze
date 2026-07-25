// Design: docs/architecture/testing/ci-format.md — deterministic Cymru DNS mock server

package cymru

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"

	mdns "github.com/miekg/dns"
)

func Run(args []string) int {
	fs := flag.NewFlagSet("ze-test cymru", flag.ExitOnError)

	var port int

	fs.IntVar(&port, "port", 0, "UDP listen port (0 = auto)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test cymru [flags]

Deterministic fake DNS server returning Team Cymru-formatted TXT records.

Query format: TXT AS<N>.asn.cymru.com.
Response:     "<N> | XX | test | 2000-01-01 | TESTNET-<N> - Test AS<N>, XX"
ASN 0:        NXDOMAIN

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	lc := &net.ListenConfig{}
	addr := textbuf.StrInt("127.0.0.1:", int64(port))
	pc, err := lc.ListenPacket(context.Background(), "udp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = pc.Close() }()

	_, portStr, _ := net.SplitHostPort(pc.LocalAddr().String())
	fmt.Fprintf(os.Stderr, "ze-test cymru: listening on port %s\n", portStr)

	server := &mdns.Server{
		PacketConn: pc,
		Handler:    mdns.HandlerFunc(handleCymruDNS),
	}

	if err := server.ActivateAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "error: serve: %v\n", err)
		return 1
	}

	return 0
}

func handleCymruDNS(w mdns.ResponseWriter, r *mdns.Msg) {
	m := new(mdns.Msg)
	m.SetReply(r)

	if len(r.Question) == 0 {
		m.Rcode = mdns.RcodeRefused
		_ = w.WriteMsg(m)
		return
	}

	q := r.Question[0]

	if q.Qtype != mdns.TypeTXT || !strings.HasSuffix(strings.ToLower(q.Name), ".asn.cymru.com.") {
		m.Rcode = mdns.RcodeRefused
		_ = w.WriteMsg(m)
		return
	}

	name := strings.ToUpper(q.Name)
	asnPart := strings.TrimSuffix(name, ".ASN.CYMRU.COM.")
	asnPart = strings.TrimPrefix(asnPart, "AS")

	var asn uint64
	for _, c := range asnPart {
		if c < '0' || c > '9' {
			m.Rcode = mdns.RcodeNameError
			_ = w.WriteMsg(m)
			return
		}
		asn = asn*10 + uint64(c-'0')
	}
	if asn == 0 {
		m.Rcode = mdns.RcodeNameError
		_ = w.WriteMsg(m)
		return
	}

	b := textbuf.Get()
	b.Uint(asn).Str(" | XX | test | 2000-01-01 | TESTNET-").Uint(asn).Str(" - Test AS").Uint(asn).Str(", XX")
	txt := b.String()
	b.Release()

	m.Answer = append(m.Answer, &mdns.TXT{
		Hdr: mdns.RR_Header{
			Name:   q.Name,
			Rrtype: mdns.TypeTXT,
			Class:  mdns.ClassINET,
			Ttl:    300,
		},
		Txt: []string{txt},
	})

	_ = w.WriteMsg(m)
}
