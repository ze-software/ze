//go:build linux

package ifacenetlink

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: spec-fixit-route-removal-protocol-blind AC-4 -- a route this
// backend installs always names the producer that owns it.
// PREVENTS: an install with rtproto.Any, which puts an unowned route in the
// kernel that only a protocol-blind delete could then remove. The check runs
// before any netlink call, so no kernel state is needed to prove it.
func TestAddRouteRejectsTheBlindProtocol(t *testing.T) {
	b := &netlinkBackend{}
	err := b.AddRoute("lo", "0.0.0.0/0", "127.0.0.1", 0, rtproto.Any)
	if err == nil {
		t.Fatal("AddRoute with rtproto.Any returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "rtproto.Any") {
		t.Fatalf("error does not name the offending value: %v", err)
	}
}

// VALIDATES: spec-fixit-route-removal-protocol-blind AC-5 -- the orphan WARN
// reaches a real operator, not a logger a test installed. It asserts the
// record arrives in slogutil's global log ring, which only a logger built by
// slogutil.Logger feeds (slogutil.createHandler wraps every handler in
// newRingHandler). A handler a test hands the package cannot put a record
// there, so this assertion cannot be satisfied by test-only wiring.
// PREVENTS: the package logging into slogutil.DiscardLogger in production,
// which is what it did while every WARN in this package went nowhere and the
// AC-5 integration test still passed on a buffer it installed itself. The
// component name is asserted too: it is what an operator types for
// `log set iface.netlink debug` and what `show log recent` filters on.
// The holder protocol is asserted as a NAME: an operator reading the record has
// to know which producer to go and look at, and 3 is not that.
func TestRemoveRouteMissWarnsThroughTheProductionLogger(t *testing.T) {
	if !logger().Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("the package logger discards WARN: no operator can see the orphan report")
	}

	const (
		wantIface = "zert-wiring0"
		wantDest  = "0.0.0.0/0"
		wantGW    = "192.0.2.2"
	)
	logRemoveRouteMiss(wantIface, wantDest, wantGW, 5, rtproto.Iface, unix.RTPROT_BOOT)

	entries := slogutil.GlobalLogRing().Snapshot(0, "WARN", "iface.netlink")
	if len(entries) == 0 {
		t.Fatal("no WARN from iface.netlink in the log ring: the package logger is not the production one")
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e.Message, "the kernel still holds this route under another one") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("the orphan WARN is not in the log ring; iface.netlink records seen: %d", len(entries))
	}
}

// VALIDATES: the WARN states only what the code established. It fires for a
// route the kernel still holds under another protocol, and it names that
// protocol; the routine miss -- nothing there at all -- is not a WARN.
// PREVENTS: the message this replaced, which reported "a route installed by an
// earlier version can survive it" on EVERY stamped ESRCH. That cause is one of
// three, and the other two carry no orphan: a route another path already took
// away, and a route table this backend could not read. A WARN that fires on
// healthy teardown is a WARN an operator stops reading.
func TestRemoveRouteMissNamesTheProtocolThatHoldsTheRoute(t *testing.T) {
	const (
		wantIface = "zert-wiring0"
		wantDest  = "0.0.0.0/0"
		wantGW    = "192.0.2.2"
	)
	buf := &bytes.Buffer{}
	previous := logger
	captured := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger = func() *slog.Logger { return captured }
	t.Cleanup(func() { logger = previous })

	logRemoveRouteMiss(wantIface, wantDest, wantGW, 5, rtproto.Iface, unix.RTPROT_BOOT)

	out := buf.String()
	if !strings.Contains(out, "level=WARN") {
		t.Fatalf("the surviving route was not reported at WARN: %q", out)
	}
	if !strings.Contains(out, "held-by=boot") {
		t.Fatalf("the WARN does not name the protocol holding the route: %q", out)
	}
	if strings.Contains(out, "earlier version") {
		t.Fatalf("the WARN still blames a cause the code did not establish: %q", out)
	}
}

// captureMissReport runs reportRemoveRouteMiss against a route table the test
// supplies and returns what the package logger recorded.
func captureMissReport(t *testing.T, table []netlink.Route, listErr error) string {
	t.Helper()

	buf := &bytes.Buffer{}
	previousLogger := logger
	captured := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger = func() *slog.Logger { return captured }
	t.Cleanup(func() { logger = previousLogger })

	previousList := listRoutesForMiss
	listRoutesForMiss = func(netlink.Link, int) ([]netlink.Route, error) { return table, listErr }
	t.Cleanup(func() { listRoutesForMiss = previousList })

	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "zert-miss0", Index: 7}}
	_, dst, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}
	route := &netlink.Route{
		LinkIndex: 7,
		Dst:       dst,
		Gw:        net.ParseIP("192.0.2.2"),
		Priority:  5,
		Protocol:  netlink.RouteProtocol(rtproto.Iface),
	}
	reportRemoveRouteMiss(link, route, "zert-miss0", "0.0.0.0/0", "192.0.2.2", 5, rtproto.Iface)
	return buf.String()
}

// VALIDATES: each of the three outcomes of a stamped ESRCH is reported as what
// the kernel established, the FAILED READ included.
// PREVENTS: routeUnderAnotherProtocol answering (0, false) for both "the read
// found nothing" and "the read failed", which made AC-5's orphan WARN disappear
// exactly when the route table could not be read: ENOBUFS on a large FIB, or an
// interrupted dump (netlink.ErrDumpInterrupted, which makes RouteListFiltered
// drop the routes it did read and return none). A report that says "no route
// carries this key" after reading nothing states a fact it does not hold.
func TestRemoveRouteMissReportsWhatTheTableReadEstablished(t *testing.T) {
	_, defaultDst, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}
	survivor := netlink.Route{
		LinkIndex: 7,
		Dst:       defaultDst,
		Gw:        net.ParseIP("192.0.2.2"),
		Priority:  5,
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  netlink.RouteProtocol(unix.RTPROT_BOOT),
	}

	t.Run("the read failed", func(t *testing.T) {
		out := captureMissReport(t, nil, errors.New("dump interrupted"))
		if !strings.Contains(out, "level=WARN") {
			t.Fatalf("a failed table read was not reported at WARN: %q", out)
		}
		if !strings.Contains(out, "route table read failed") {
			t.Fatalf("the report does not say the read failed: %q", out)
		}
		if strings.Contains(out, "no route carries") {
			t.Fatalf("a failed read was reported as an absence: %q", out)
		}
	})

	t.Run("another protocol holds the route", func(t *testing.T) {
		out := captureMissReport(t, []netlink.Route{survivor}, nil)
		if !strings.Contains(out, "level=WARN") {
			t.Fatalf("the surviving route was not reported at WARN: %q", out)
		}
		if !strings.Contains(out, "held-by=boot") {
			t.Fatalf("the WARN does not name the protocol holding the route: %q", out)
		}
	})

	t.Run("the read found nothing", func(t *testing.T) {
		out := captureMissReport(t, nil, nil)
		if !strings.Contains(out, "level=DEBUG") {
			t.Fatalf("a routine miss was not reported at DEBUG: %q", out)
		}
		if !strings.Contains(out, "no route carries") {
			t.Fatalf("the report does not say what the read established: %q", out)
		}
	})
}
