// Goal: prove the ze-test dns subcommand reports a bind it could not make,
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

// The usage reaches an author through `ze-test dns --help`, not only through
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
			t.Errorf("`ze-test dns --help` does not name %s", bare)
		}
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
