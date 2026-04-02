// Design: docs/architecture/testing/ci-format.md -- deterministic PeeringDB mock server
//
// ze-test peeringdb is a fake PeeringDB-compatible HTTP server for functional tests.
// It returns deterministic prefix counts derived from the ASN:
//
//	info_prefixes4 = ASN
//	info_prefixes6 = ASN / 5
//
// ASN 0 returns an empty data array (not found).
//
// Usage:
//
//	ze-test peeringdb --port 8080
//	ze-test peeringdb --port 0        # auto-select port
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

func peeringdbCmd() int {
	fs := flag.NewFlagSet("ze-test peeringdb", flag.ExitOnError)

	var port int

	fs.IntVar(&port, "port", 0, "HTTP listen port (0 = auto)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test peeringdb [flags]

Deterministic fake PeeringDB HTTP server for functional tests.

Prefix counts are derived from the ASN:
  info_prefixes4 = ASN
  info_prefixes6 = ASN / 5
  ASN 0          = not found (empty data array)

The server listens on 127.0.0.1 and prints the actual port to stderr.
Point ze config peeringdb-url at http://127.0.0.1:<port> to use.

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/net", handlePeeringDBNet)

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Fprintf(os.Stderr, "ze-test peeringdb: listening on port %s\n", portStr)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "error: serve: %v\n", err)
		return 1
	}

	return 0
}

// handlePeeringDBNet serves /api/net?asn=N with deterministic prefix counts.
func handlePeeringDBNet(w http.ResponseWriter, r *http.Request) {
	asnStr := r.URL.Query().Get("asn")
	if asnStr == "" {
		http.Error(w, `{"meta":{"error":"asn parameter required"}}`, http.StatusBadRequest)
		return
	}

	asn, err := strconv.ParseUint(asnStr, 10, 32)
	if err != nil {
		http.Error(w, `{"meta":{"error":"invalid asn"}}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// ASN 0 = not found.
	if asn == 0 {
		if _, wErr := io.WriteString(w, `{"data":[]}`); wErr != nil {
			return
		}
		return
	}

	// Deterministic formula: ipv4 = ASN, ipv6 = ASN / 5.
	ipv4 := asn
	ipv6 := asn / 5

	// Deterministic AS-SET: odd ASN -> "AS-TEST", even ASN -> "AS-FOO AS-BAR", ASN%3==0 -> empty.
	var irrASSet string
	switch {
	case asn%3 == 0:
		irrASSet = "" // no AS-SET registered
	case asn%2 == 1:
		irrASSet = "AS-TEST"
	default:
		irrASSet = "AS-FOO AS-BAR"
	}

	if _, wErr := fmt.Fprintf(w,
		`{"data":[{"asn":%d,"info_prefixes4":%d,"info_prefixes6":%d,"irr_as_set":"%s"}]}`,
		asn, ipv4, ipv6, irrASSet); wErr != nil {
		return
	}
}
