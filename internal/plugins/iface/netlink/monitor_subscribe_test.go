package ifacenetlink

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// VALIDATES: every netlink multicast subscription this monitor opens asks for a
// receive buffer, rather than taking the kernel default.
//
// PREVENTS: silent loss of link-state notifications. The kernel drops a
// multicast message it cannot queue, and the drop happens before ze sees it, so
// nothing on the reader side recovers it. With the default buffer, commonly
// net.core.rmem_default at 208 KiB, the arm64 QEMU VM lost 1054 and 207
// notifications in two of three runs of
// test/plugin/iface-link-flap-during-commit at 101 transitions per burst. A
// reviewer reverting any of these three to the plain Subscribe form restores
// that, and nothing else in the tree would notice: the loss is load-dependent
// and invisible in a quiet checkout.
//
// This reads the source because the alternative needs a live netlink socket,
// which is Linux-only, root-only and cannot assert what SO_RCVBUF was asked for
// after the fact.
func TestEverySubscriptionAsksForAReceiveBuffer(t *testing.T) {
	data, err := os.ReadFile("monitor_linux.go")
	if err != nil {
		t.Fatalf("read monitor_linux.go: %v", err)
	}
	source := string(data)

	for _, family := range []string{"Link", "Addr", "Neigh"} {
		plain := regexp.MustCompile(`netlink\.` + family + `Subscribe\(`)
		if plain.MatchString(source) {
			t.Errorf("netlink.%sSubscribe( is called without options, so that socket keeps the kernel default receive buffer and the kernel drops what it cannot queue; use %sSubscribeWithOptions with ReceiveBufferSize", family, family)
		}
		withOptions := regexp.MustCompile(`netlink\.` + family + `SubscribeWithOptions\(`)
		if !withOptions.MatchString(source) {
			t.Errorf("no netlink.%sSubscribeWithOptions( call found; this test reads the source and the shape it reads has changed", family)
		}
	}

	// Each options literal must actually carry the size. A WithOptions call
	// with an empty literal is the same default buffer with more ceremony.
	if got := strings.Count(source, "ReceiveBufferSize: monitorReceiveBufferBytes"); got != 3 {
		t.Errorf("found %d subscriptions setting ReceiveBufferSize, want 3 (link, addr, neigh)", got)
	}
}

// VALIDATES: the buffer is a real size and is asked for rather than forced.
//
// Forcing takes SO_RCVBUFFORCE, which needs CAP_NET_ADMIN and overrides
// net.core.rmem_max. That is the operator's ceiling and not a monitor's
// decision, so the unforced call is deliberate: the kernel clamps and a host
// with a smaller maximum gets what it allows instead of an error.
func TestTheReceiveBufferIsAskedForNotForced(t *testing.T) {
	data, err := os.ReadFile("monitor_linux.go")
	if err != nil {
		t.Fatalf("read monitor_linux.go: %v", err)
	}
	// The constant is declared in a _linux.go file, so it is not compiled on
	// every host this test runs on. Read the declaration rather than the
	// symbol: a source-reading test that could only run on one platform would
	// stop guarding the other.
	declaration := regexp.MustCompile(`monitorReceiveBufferBytes\s*=\s*([0-9]+)\s*<<\s*([0-9]+)`)
	match := declaration.FindStringSubmatch(string(data))
	if len(match) != 3 {
		t.Fatalf("could not read the monitorReceiveBufferBytes declaration; the shape this test reads has changed")
	}
	mantissa, shift := atoiOrFail(t, match[1]), atoiOrFail(t, match[2])
	size := mantissa << shift
	if size <= 0 {
		t.Fatalf("monitorReceiveBufferBytes = %d, want a positive size", size)
	}
	// Smaller than the default it replaces would be a regression that reads
	// like a fix. 208 KiB is the common net.core.rmem_default.
	if size < 208*1024 {
		t.Errorf("monitorReceiveBufferBytes = %d, which is below the common kernel default of 208 KiB", size)
	}

	if strings.Contains(string(data), "ReceiveBufferForceSize") {
		t.Error("the monitor forces its receive buffer; SO_RCVBUFFORCE needs CAP_NET_ADMIN and overrides net.core.rmem_max, which is the operator's ceiling to set")
	}
}

func atoiOrFail(t *testing.T, text string) int {
	t.Helper()
	value, err := strconv.Atoi(text)
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	return value
}
