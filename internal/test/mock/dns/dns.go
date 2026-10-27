// Design: docs/functional-tests.md -- `le mock dns`, the deterministic DNS mock server
// Related: server.go -- the in-process form a fixture drives; zone.go -- the answers

package dns

import (
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// maxPort is the largest UDP port number. --port is refused above it rather
// than handed to the kernel, so an author reads the range instead of an errno.
const maxPort = 65535

// delayMsMax bounds --delay: a minute is past every resolver timeout a test
// configures, so a larger value is a typo, not a scenario.
const delayMsMax = 60000

// parseDelay reads one --delay value, <name>=<ms>.
func parseDelay(value string) (string, time.Duration, error) {
	name, ms, found := strings.Cut(value, "=")
	if !found {
		return "", 0, errors.New("no '=': write --delay <name>=<ms>, as --delay slow.test=800")
	}
	if name == "" {
		return "", 0, errors.New("an empty name: write --delay <name>=<ms>, as --delay slow.test=800")
	}
	n, err := strconv.ParseUint(ms, 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("milliseconds %q are not a whole number: write --delay %s=800", ms, name)
	}
	if n > delayMsMax {
		return "", 0, fmt.Errorf("%d ms is above the %d ms bound: give a delay up to one minute", n, delayMsMax)
	}
	return name, time.Duration(n) * time.Millisecond, nil
}

// dropFirstMax bounds --drop-first: a test asks a handful of questions, so a
// larger count is a typo, not a scenario.
const dropFirstMax = 1000

// parseDropFirst reads one --drop-first value, <name>=<count>.
func parseDropFirst(value string) (string, uint32, error) {
	name, count, found := strings.Cut(value, "=")
	if !found {
		return "", 0, errors.New("no '=': write --drop-first <name>=<count>, as --drop-first slow.test=1")
	}
	if name == "" {
		return "", 0, errors.New("an empty name: write --drop-first <name>=<count>, as --drop-first slow.test=1")
	}
	n, err := strconv.ParseUint(count, 10, 32)
	if err != nil {
		return "", 0, errors.New(textbuf.New().Str("count \"").Str(count).
			Str("\" is not a whole number: write --drop-first ").Str(name).Str("=1").String())
	}
	if n == 0 {
		return "", 0, errors.New("a count of 0 drops nothing: give 1 or more, or leave the flag out")
	}
	if n > dropFirstMax {
		return "", 0, errors.New(textbuf.New().Uint(n).Str(" is above the bound of ").Uint(dropFirstMax).
			Str(" queries: give a count up to that").String())
	}
	return name, uint32(n), nil
}

// Run serves the default zone until the process is killed. It is the form a
// `.ci` file starts with `cmd=background:exec=le mock dns --port 53`; a fixture
// that must change an answer while the test runs uses Start instead.
func Run(args []string) int {
	// ContinueOnError, not the ExitOnError the sibling mocks use: flag's exit
	// path calls os.Exit, which skips the crashlog.Flush that cmd/ze/main.go
	// runs after dispatch. crashlog.Init (internal/core/crashlog/crashlog.go)
	// has replaced os.Stderr with a pipe by then, and only that flush drains it,
	// so an exit inside flag discards the usage text this command owes its
	// reader. Returning a code instead of exiting keeps the text.
	fs := flag.NewFlagSet("le mock dns", flag.ContinueOnError)

	var port int

	// No backquotes in the usage string: the flag package reads a backquoted
	// word as the argument's NAME and prints it after -port.
	fs.IntVar(&port, "port", 0, "UDP listen port (0 = auto; 53 is the port a daemon's system name-server leaf reaches)")

	var logQueries bool
	fs.BoolVar(&logQueries, "log-queries", false, "write \"le mock dns: query <name> <type>\" to stderr for every question received")
	delays := make(map[string]time.Duration)
	fs.Func("delay", "<name>=<ms>: hold every answer for name back by ms milliseconds (repeatable)", func(value string) error {
		name, delay, err := parseDelay(value)
		if err != nil {
			return err
		}
		delays[name] = delay
		return nil
	})
	var drops []string
	fs.Func("drop", "<name>: read every query for name and never answer it (repeatable)", func(value string) error {
		if value == "" {
			return errors.New("an empty name: give the name whose queries go unanswered, as --drop slow.test")
		}
		drops = append(drops, value)
		return nil
	})
	dropsFirst := make(map[string]uint32)
	fs.Func("drop-first", "<name>=<count>: read the first count queries for name and never answer them, then answer (repeatable)", func(value string) error {
		name, count, err := parseDropFirst(value)
		if err != nil {
			return err
		}
		dropsFirst[name] = count
		return nil
	})
	// The default is the one loopback address every Linux host answers. Two
	// stubs on one port need two addresses: Linux answers the whole of
	// 127.0.0.0/8 on lo, so a test can run one on 127.0.0.2 and one on
	// 127.0.0.3.
	address := netip.AddrFrom4([4]byte{127, 0, 0, 1})
	fs.Func("address", "IP address to listen on (default 127.0.0.1; any 127.0.0.0/8 address works on Linux)", func(value string) error {
		parsed, err := netip.ParseAddr(value)
		if err != nil {
			return errors.New(textbuf.New().Str("\"").Str(value).
				Str("\" is not an IP address: write --address 127.0.0.2").String())
		}
		address = parsed
		return nil
	})

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: le mock dns [flags]

Deterministic fake DNS server for functional tests. It answers A and AAAA
queries for the names below over UDP, on 127.0.0.1 or the --address given.

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

--log-queries, --delay, --drop and --drop-first shape what a resolver under
test meets: a log line per question proves how many queries reached the stub,
a delay makes one name slow, a dropped name is never answered, so the
resolver times out, and --drop-first drops only a name's first queries, so
the resolver times out and is then answered. --address puts a second stub on
the same port (127.0.0.2, 127.0.0.3), for a resolver with two upstreams.
le mock dns help prints this text.

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

	// The flags are the whole interface, so the one word accepted after them
	// is help, which `le help mock dns` names as the way to reach this text.
	if fs.NArg() > 0 {
		if fs.NArg() == 1 && fs.Arg(0) == "help" {
			fs.Usage()
			return 0
		}
		_ = textbuf.New().Str("error: le mock dns takes flags only, and \"").Str(fs.Arg(0)).
			Str("\" is not one: the flags are --port, --address, --log-queries, --delay <name>=<ms>, --drop <name> and --drop-first <name>=<count>; run le mock dns help\n").
			StdErr() //nolint:errcheck // exiting with the code; stderr is the last channel
		return 1
	}

	if port < 0 || port > maxPort {
		fmt.Fprintf(os.Stderr, "error: port %d is outside the range 0-%d\n", port, maxPort)
		return 1
	}

	server, err := Start(netip.AddrPortFrom(address, uint16(port)).String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if logQueries {
		server.LogQueries(os.Stderr)
	}
	for name, delay := range delays {
		server.Delay(name, delay)
	}
	for _, name := range drops {
		server.Drop(name)
	}
	for name, count := range dropsFirst {
		server.DropFirst(name, count)
	}

	_ = textbuf.New().Str("le mock dns: listening on port ").Uint(uint64(server.Addr().Port())).Byte('\n').
		StdErr() //nolint:errcheck // a test reads the line; nothing else can report its loss

	if err := server.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "error: serve: %v\n", err)
		return 1
	}
	return 0
}
