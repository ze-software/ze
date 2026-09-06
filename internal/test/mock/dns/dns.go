// Design: docs/functional-tests.md -- `ze-test dns`, the deterministic DNS mock server
// Related: server.go -- the in-process form a fixture drives; zone.go -- the answers

package dns

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
)

// maxPort is the largest UDP port number. --port is refused above it rather
// than handed to the kernel, so an author reads the range instead of an errno.
const maxPort = 65535

// Run serves the default zone until the process is killed. It is the form a
// `.ci` file starts with `cmd=background:exec=ze-test dns --port 53`; a fixture
// that must change an answer while the test runs uses Start instead.
func Run(args []string) int {
	// ContinueOnError, not the ExitOnError the sibling mocks use: flag's exit
	// path calls os.Exit, which skips the crashlog.Flush that cmd/ze/main.go
	// runs after dispatch. crashlog.Init (internal/core/crashlog/crashlog.go)
	// has replaced os.Stderr with a pipe by then, and only that flush drains it,
	// so an exit inside flag discards the usage text this command owes its
	// reader. Returning a code instead of exiting keeps the text.
	fs := flag.NewFlagSet("ze-test dns", flag.ContinueOnError)

	var port int

	// No backquotes in the usage string: the flag package reads a backquoted
	// word as the argument's NAME and prints it after -port.
	fs.IntVar(&port, "port", 0, "UDP listen port (0 = auto; 53 is the port a daemon's system name-server leaf reaches)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test dns [flags]

Deterministic fake DNS server for functional tests. It answers A and AAAA
queries for the names below, on 127.0.0.1 over UDP.

A name absent from the zone answers NXDOMAIN (RFC 1035 Section 4.1.1). A name
the zone carries, queried for a record type it does not hold, answers NOERROR
with an empty answer section (RFC 2308 Section 2.2), which says the name is
alive and holds no address in that family.

Zone:
%s
A TTL of 0 keeps the answer out of the daemon's resolver cache, so a second
lookup reaches this server rather than the cache.

`+"`system name-server`"+` is an IP address and carries no port, so a test that points
a daemon at this stub runs it on port 53 and declares
option=needs-linux:caps=net-bind.

Flags:
`, zoneUsage(defaultZone()))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		// flag has already written the reason and the usage to fs.Output().
		// --help is a request that was answered, not a failure.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if port < 0 || port > maxPort {
		fmt.Fprintf(os.Stderr, "error: port %d is outside the range 0-%d\n", port, maxPort)
		return 1
	}

	server, err := Start(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "ze-test dns: listening on port %d\n", server.Addr().Port())

	if err := server.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "error: serve: %v\n", err)
		return 1
	}
	return 0
}
