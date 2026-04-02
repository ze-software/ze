// Design: docs/architecture/testing/ci-format.md -- deterministic IRR whois mock server
//
// ze-test irr is a fake RPSL whois server for functional tests.
// It returns deterministic responses for AS-SET expansion (!i) and prefix lookup (!a4/!a6).
//
// Supported queries:
//
//	!iAS-TEST   -> "A3\nAS65001 AS65002 AS65003\nC\n"
//	!a4AS-TEST  -> "A3\n10.0.0.0/24 10.0.1.0/24 172.16.0.0/16\nC\n"
//	!a6AS-TEST  -> "A1\n2001:db8::/32\nC\n"
//	anything else -> "D\n" (not found)
//
// Usage:
//
//	ze-test irr --port 4343
//	ze-test irr --port 0        # auto-select port
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

func irrCmd() int {
	fs := flag.NewFlagSet("ze-test irr", flag.ExitOnError)

	var port int

	fs.IntVar(&port, "port", 0, "TCP listen port (0 = auto)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test irr [flags]

Deterministic fake IRR whois server for functional tests.
Responds to RPSL !i (AS-SET expansion) and !a4/!a6 (prefix lookup) queries.

Known AS-SETs:
  AS-TEST  -> members: AS65001, AS65002, AS65003
             ipv4: 10.0.0.0/24, 10.0.1.0/24, 172.16.0.0/16
             ipv6: 2001:db8::/32

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Fprintf(os.Stderr, "ze-test irr: listening on port %s\n", portStr)

	for {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return 0 // listener closed
		}
		go handleIRRConn(conn)
	}
}

// irrResponses maps RPSL queries to deterministic responses.
var irrResponses = map[string]string{
	"!iAS-TEST":  "A3\nAS65001 AS65002 AS65003\nC\n",
	"!a4AS-TEST": "A3\n10.0.0.0/24 10.0.1.0/24 172.16.0.0/16\nC\n",
	"!a6AS-TEST": "A1\n2001:db8::/32\nC\n",
}

func handleIRRConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	buf := make([]byte, 4096)
	n, readErr := conn.Read(buf)
	if readErr != nil {
		return
	}

	query := strings.TrimSpace(string(buf[:n]))

	response, ok := irrResponses[query]
	if !ok {
		response = "D\n"
	}

	if _, writeErr := fmt.Fprint(conn, response); writeErr != nil {
		return
	}
}
