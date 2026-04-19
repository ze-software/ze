// Design: docs/architecture/testing/ci-format.md -- deterministic Cymru DNS mock server
//
// ze-test cymru is a fake DNS server that returns Team Cymru-formatted TXT responses
// for ASN queries. Deterministic: ASN -> "ASN | CC | RIR | Date | TESTNET-<ASN> - Test AS<ASN>, XX"
//
// ASN 0 returns NXDOMAIN. Non-Cymru queries return REFUSED.
//
// Usage:
//
//	ze-test cymru --port 5553
//	ze-test cymru --port 0        # auto-select port
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	mdns "github.com/miekg/dns"
)

var _ = register("cymru", "Deterministic Cymru DNS mock server (ASN to TXT responses)", cymruCmd)

func cymruCmd() int {
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

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}

	lc := &net.ListenConfig{}
	pc, err := lc.ListenPacket(context.Background(), "udp", fmt.Sprintf("127.0.0.1:%d", port))
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

	// Only handle TXT queries for *.asn.cymru.com.
	if q.Qtype != mdns.TypeTXT || !strings.HasSuffix(strings.ToLower(q.Name), ".asn.cymru.com.") {
		m.Rcode = mdns.RcodeRefused
		_ = w.WriteMsg(m)
		return
	}

	// Extract ASN from "AS<N>.asn.cymru.com."
	name := strings.ToUpper(q.Name)
	asnPart := strings.TrimSuffix(name, ".ASN.CYMRU.COM.")
	asnPart = strings.TrimPrefix(asnPart, "AS")

	asn, err := strconv.ParseUint(asnPart, 10, 32)
	if err != nil || asn == 0 {
		m.Rcode = mdns.RcodeNameError // NXDOMAIN
		_ = w.WriteMsg(m)
		return
	}

	txt := fmt.Sprintf("%d | XX | test | 2000-01-01 | TESTNET-%d - Test AS%d, XX", asn, asn, asn)

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
