// Design: docs/architecture/testing/ci-format.md -- deterministic IRR whois mock server

package irr

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

func Run(args []string) int {
	fs := flag.NewFlagSet("le test irr", flag.ExitOnError)

	var port int
	var emptyAfterFirst bool
	var failFirst int

	fs.IntVar(&port, "port", 0, "TCP listen port (0 = auto)")
	fs.BoolVar(&emptyAfterFirst, "empty-after-first", false,
		"answer each query with its data once, then with \"D\" (key not found) forever after")

	fs.IntVar(&failFirst, "fail-first", 0,
		"answer the first N queries with \"F\" (query failed), then normally")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: le test irr [flags]

Deterministic fake IRR whois server for functional tests.
Responds to RPSL !i (AS-SET expansion) and !a4/!a6 (prefix lookup) queries.

Known AS-SETs:
  AS-TEST    -> members: AS65001, AS65002, AS65003
               ipv4: 10.0.0.0/24, 10.0.1.0/24, 172.16.0.0/16
               ipv6: 2001:db8::/32
  AS-V4ONLY  -> members: AS65004
               ipv4: 192.0.2.0/24
               ipv6: none, so !a6 answers "D" (key not found)

An AS-SET announcing one family is ordinary, and it is the case a fixture
answering both families cannot reach. The IPv6 query returns not-found rather
than an error, so the entry caches with an empty IPv6 family.

With -empty-after-first, every query is answered once with its data and then
with "D" (key not found) forever after. It models an IRR server that has a bad
minute after a good refresh, which is the case ze must not let empty a live
filter.

With -fail-first N, the first N queries are answered "F" (query failed) and
every later one normally. It models an IRR server that is down when ze starts
and recovers, which is the case where a route rejected for a missing list must
be evaluated again once the list arrives.

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Fprintf(os.Stderr, "le test irr: listening on port %s\n", portStr)

	var answered *servedQueries
	if emptyAfterFirst {
		answered = &servedQueries{seen: make(map[string]bool)}
	}

	failing := &failBudget{}
	failing.left.Store(int64(failFirst))

	for {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return 0
		}
		go handleIRRConn(conn, answered, failing)
	}
}

// irrResponses maps an RPSL query to its answer. A query absent from the map
// answers "D" (key not found). That is how AS-V4ONLY has no IPv6: an AS-SET
// announcing one family answers exactly that way. lookupFamilyPrefixes
// (internal/component/resolve/irr/client.go) reads it as an empty family
// rather than a failed lookup.
var irrResponses = map[string]string{
	"!iAS-TEST":  "A3\nAS65001 AS65002 AS65003\nC\n",
	"!a4AS-TEST": "A3\n10.0.0.0/24 10.0.1.0/24 172.16.0.0/16\nC\n",
	"!a6AS-TEST": "A1\n2001:db8::/32\nC\n",

	"!iAS-V4ONLY":  "A1\nAS65004\nC\n",
	"!a4AS-V4ONLY": "A1\n192.0.2.0/24\nC\n",
}

// servedQueries records which queries have already been answered with data, so
// -empty-after-first can answer each one exactly once.
type servedQueries struct {
	mu   sync.Mutex
	seen map[string]bool
}

// firstTime reports whether query has not been answered with data before, and
// records that it now has been.
func (s *servedQueries) firstTime(query string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[query] {
		return false
	}
	s.seen[query] = true
	return true
}

// failBudget counts the queries -fail-first still answers "F".
type failBudget struct {
	left atomic.Int64
}

// take reports whether this query is one of the failing ones.
func (b *failBudget) take() bool {
	return b.left.Add(-1) >= 0
}

func handleIRRConn(conn net.Conn, answered *servedQueries, failing *failBudget) {
	defer func() { _ = conn.Close() }()

	buf := make([]byte, 4096)
	n, readErr := conn.Read(buf)
	if readErr != nil {
		return
	}

	query := strings.TrimSpace(string(buf[:n]))

	response, ok := irrResponses[query]
	if !ok || (answered != nil && !answered.firstTime(query)) {
		response = "D\n"
	}
	if failing.take() {
		response = "F mock IRR server failing on purpose (-fail-first)\n"
	}

	if _, writeErr := fmt.Fprint(conn, response); writeErr != nil {
		return
	}
}
