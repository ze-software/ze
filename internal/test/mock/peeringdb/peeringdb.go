// Design: docs/architecture/testing/ci-format.md -- deterministic PeeringDB mock server

package peeringdb

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func Run(args []string) int {
	fs := flag.NewFlagSet("le mock peeringdb", flag.ExitOnError)

	var port int
	var servedCount bool

	fs.IntVar(&port, "port", 0, "HTTP listen port (0 = auto)")
	fs.BoolVar(&servedCount, "served-count", false,
		"answer irr_as_set as AS-SERVED-<n>, n counting the requests served for that ASN")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: le mock peeringdb [flags]

Deterministic fake PeeringDB HTTP server for functional tests.

Prefix counts are derived from the ASN:
  info_prefixes4 = ASN
  info_prefixes6 = ASN / 5
  ASN 0          = not found (empty data array)

With --served-count, irr_as_set is AS-SERVED-<n>, where n counts the requests
served for that ASN: 1 on the first, 2 on the next. A client that answers
AS-SERVED-1 twice served the second answer from its cache.

The server listens on 127.0.0.1 and prints the actual port to stderr.
Point ze config peeringdb-url at http://127.0.0.1:<port> to use.

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	mux := http.NewServeMux()
	served := &servedCounts{byASN: map[uint64]uint64{}, enabled: servedCount}
	mux.HandleFunc("/api/net", func(w http.ResponseWriter, r *http.Request) { handleNet(w, r, served) })

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Fprintf(os.Stderr, "le mock peeringdb: listening on port %s\n", portStr)

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

// servedCounts counts the requests served for each ASN when --served-count is
// set. Safe for concurrent use.
type servedCounts struct {
	mu      sync.Mutex
	enabled bool
	byASN   map[uint64]uint64
}

// next counts one more request for asn and returns the new count.
func (s *servedCounts) next(asn uint64) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byASN[asn]++
	return s.byASN[asn]
}

func handleNet(w http.ResponseWriter, r *http.Request, served *servedCounts) {
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

	if asn == 0 {
		if _, wErr := io.WriteString(w, `{"data":[]}`); wErr != nil {
			return
		}
		return
	}

	ipv4 := asn
	ipv6 := asn / 5

	var irrASSet string
	switch {
	case asn%3 == 0:
		irrASSet = ""
	case asn%2 == 1:
		irrASSet = "AS-TEST"
	default:
		irrASSet = "AS-FOO AS-BAR"
	}

	if served.enabled {
		var name textbuf.Buffer
		name.Str("AS-SERVED-").Uint(served.next(asn))
		irrASSet = name.String()
	}

	var b textbuf.Buffer
	b.Str(`{"data":[{"asn":`)
	b.Uint(asn)
	b.Str(`,"info_prefixes4":`)
	b.Uint(ipv4)
	b.Str(`,"info_prefixes6":`)
	b.Uint(ipv6)
	b.Str(`,"irr_as_set":"`)
	b.Str(irrASSet)
	b.Str(`"}]}`)

	if _, wErr := io.WriteString(w, b.Slice()); wErr != nil {
		return
	}
}
