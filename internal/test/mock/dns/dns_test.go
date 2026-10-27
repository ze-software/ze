// Goal: prove the le mock dns subcommand reports a bind it could not make,
// refuses a port outside the range, and prints a usage text derived from the
// zone. Method: call Run with os.Stderr redirected to a pipe.
//
// VALIDATES: AC-1, AC-10, and the --port boundary rows of the spec.
// PREVENTS: R-1, a stub that fails to bind and is mistaken for one that answers
// nothing, and a usage text written beside the zone that goes stale the first
// time a name is added.

package dns

import (
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	mdns "github.com/miekg/dns"
)

// Compile-time signature check: Run must be func([]string) int.
var _ func([]string) int = Run //nolint:staticcheck // intentional type annotation for signature verification

// runCapturingStderr calls Run and returns its exit code with everything it
// wrote to stderr.
func runCapturingStderr(t *testing.T, args ...string) (int, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	original := os.Stderr
	os.Stderr = writer

	code := Run(args)

	os.Stderr = original
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return code, string(out)
}

func TestRunReportsListenFailure(t *testing.T) {
	listen := &net.ListenConfig{}
	held, err := listen.ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer held.Close() //nolint:errcheck // test cleanup
	_, port, err := net.SplitHostPort(held.LocalAddr().String())
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	code, stderr := runCapturingStderr(t, "--port", port)
	if code != 1 {
		t.Fatalf("Run on a busy port returned %d, want 1", code)
	}
	if !strings.Contains(stderr, port) {
		t.Fatalf("Run wrote %q, which does not name the port it could not bind (%s)", stderr, port)
	}
}

func TestRunRefusesAPortOutsideTheRange(t *testing.T) {
	for _, port := range []string{"-1", "65536"} {
		code, stderr := runCapturingStderr(t, "--port", port)
		if code != 1 {
			t.Errorf("Run --port %s returned %d, want 1", port, code)
		}
		if !strings.Contains(stderr, port) {
			t.Errorf("Run --port %s wrote %q, which does not name the value it refused", port, stderr)
		}
	}
}

// The usage reaches an author through `le mock dns --help`, not only through
// zoneUsage. Run must therefore RETURN rather than exit: os.Exit skips the
// crashlog.Flush cmd/ze/main.go runs after dispatch, and crashlog.Init has
// replaced os.Stderr with a pipe only that flush drains, so an exit inside the
// flag package discards every line this command wrote.
func TestHelpPrintsTheZone(t *testing.T) {
	code, stderr := runCapturingStderr(t, "--help")
	if code != 0 {
		t.Fatalf("Run --help returned %d, want 0", code)
	}
	for name := range defaultZone() {
		bare := strings.TrimSuffix(name, ".")
		if !strings.Contains(stderr, bare) {
			t.Errorf("`le mock dns --help` does not name %s", bare)
		}
	}
}

// TestHelpWordAndStrayArgument proves `le mock dns help` prints the usage and
// returns, as `le help mock dns` promises, rather than starting the stub and
// serving until killed; and that any other word is refused naming the flags.
// Method: Run in process, so a stub that started would block the test.
func TestHelpWordAndStrayArgument(t *testing.T) {
	code, stderr := runCapturingStderr(t, "help")
	if code != 0 {
		t.Fatalf("Run help returned %d, want 0", code)
	}
	if !strings.Contains(stderr, "Usage: le mock dns") {
		t.Fatalf("Run help printed %q, want the usage", stderr)
	}
	if strings.Contains(stderr, "listening on port") {
		t.Fatal("Run help started the stub")
	}

	code, stderr = runCapturingStderr(t, "--port", "0", "serve")
	if code != 1 {
		t.Fatalf("Run serve returned %d, want 1", code)
	}
	for _, flag := range []string{"--port", "--log-queries", "--delay", "--drop", "le mock dns help"} {
		if !strings.Contains(stderr, flag) {
			t.Errorf("the refusal of a stray word %q does not name %s", stderr, flag)
		}
	}
}

// TestDelayFlagRefusals proves a malformed --delay is refused before the stub
// starts, each with the form to write instead.
func TestDelayFlagRefusals(t *testing.T) {
	for _, value := range []string{"slow.test", "=800", "slow.test=fast", "slow.test=60001"} {
		code, stderr := runCapturingStderr(t, "--delay", value)
		if code != 1 {
			t.Errorf("--delay %s returned %d, want 1", value, code)
		}
		if strings.Contains(stderr, "listening on port") {
			t.Errorf("--delay %s started the stub", value)
		}
	}
	if code, _ := runCapturingStderr(t, "--drop", ""); code != 1 {
		t.Errorf("--drop with an empty name returned %d, want 1", code)
	}
}

// TestDropFirstAndAddressFlagRefusals proves --drop-first and --address
// refuse a value they cannot act on before the stub binds. Method: Run with
// each bad value; none may print the listening line.
func TestDropFirstAndAddressFlagRefusals(t *testing.T) {
	cases := [][]string{
		{"--drop-first", "slow.test"},
		{"--drop-first", "=1"},
		{"--drop-first", "slow.test=many"},
		{"--drop-first", "slow.test=0"},
		{"--drop-first", "slow.test=1001"},
		{"--address", "localhost"},
		{"--address", "127.0.0.300"},
	}
	for _, args := range cases {
		code, stderr := runCapturingStderr(t, args...)
		if code != 1 {
			t.Errorf("%v returned %d, want 1", args, code)
		}
		if strings.Contains(stderr, "listening on port") {
			t.Errorf("%v started the stub", args)
		}
	}
}

// TestAddressReachesTheBind proves --address is the address the stub binds,
// not a flag that parses and is dropped. Method: 192.0.2.1 (RFC 5737
// TEST-NET-1) is on no interface of a test host, so the bind fails and the
// error names it. A stub that ignored the flag would bind 127.0.0.1 and
// serve, and this test would hang to its deadline instead.
func TestAddressReachesTheBind(t *testing.T) {
	code, stderr := runCapturingStderr(t, "--address", "192.0.2.1")
	if code != 1 {
		t.Fatalf("Run on an address no interface holds returned %d, want 1", code)
	}
	if !strings.Contains(stderr, "192.0.2.1") {
		t.Fatalf("Run wrote %q, which does not name the address it could not bind", stderr)
	}
}

func TestUsageListsEveryZoneName(t *testing.T) {
	usage := zoneUsage(defaultZone())

	for name, entry := range defaultZone() {
		bare := strings.TrimSuffix(name, ".")
		if !strings.Contains(usage, bare) {
			t.Errorf("the usage text does not name %s, so an author cannot pick it without reading the source", bare)
		}
		if !strings.Contains(usage, mdns.RcodeToString[entry.Rcode]) {
			t.Errorf("the usage text does not name the %s response code %s reaches", mdns.RcodeToString[entry.Rcode], bare)
		}
		for qtype, answer := range entry.Answers {
			if !strings.Contains(usage, mdns.TypeToString[qtype]) {
				t.Errorf("%s: the usage text does not name the %s record it holds", bare, mdns.TypeToString[qtype])
			}
			if !strings.Contains(usage, "ttl="+strconv.FormatUint(uint64(answer.TTL), 10)) {
				t.Errorf("%s: the usage text does not name the TTL %d it answers with", bare, answer.TTL)
			}
			for _, address := range answer.Addresses {
				if !strings.Contains(usage, address.String()) {
					t.Errorf("%s: the usage text does not name the address %s it answers", bare, address)
				}
			}
		}
	}
}
