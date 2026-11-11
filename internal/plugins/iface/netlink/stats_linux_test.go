// Design: docs/features/interfaces.md -- Interface statistics from the netlink backend.
// Related: manage_linux.go -- GetStats refuses unavailable counters.

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestNetlinkGetStatsMissingLink preserves typed absence through GetStats by
// supplying the kernel lookup's not-found error, wrapped by a read context.
func TestNetlinkGetStatsMissingLink(t *testing.T) {
	lookupErr := fmt.Errorf("read link: %w", statsLinkNotFound{})
	stubStatsLookup(t, nil, lookupErr)
	b := &netlinkBackend{}
	stats, err := b.GetStats("stats0")
	if stats != nil {
		t.Fatalf("missing link returned counters: %+v", stats)
	}
	if !errors.Is(err, iface.ErrInterfaceNotFound) {
		t.Fatalf("GetStats error = %v, want ErrInterfaceNotFound", err)
	}
	if !errors.Is(err, lookupErr) {
		t.Fatalf("GetStats lost the lookup error: %v", err)
	}
	if _, ok := errors.AsType[netlink.LinkNotFoundError](err); !ok {
		t.Fatalf("GetStats lost the netlink not-found type: %v", err)
	}
	if !strings.Contains(err.Error(), `"stats0"`) {
		t.Fatalf("GetStats error does not name the interface: %v", err)
	}
}

// TestNetlinkGetStatsReadErrorIsNotAbsence drives failed reads through GetStats,
// including a partial link, and requires the original error without counters.
func TestNetlinkGetStatsReadErrorIsNotAbsence(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"permission", unix.EPERM},
		{"wrapped-io", fmt.Errorf("read link: %w", unix.EIO)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			link := &netlink.Dummy{
				Name: "stats0", Statistics: &netlink.LinkStatistics{RxBytes: 42},
			}
			stubStatsLookup(t, link, tc.err)
			b := &netlinkBackend{}
			stats, err := b.GetStats("stats0")
			if stats != nil {
				t.Fatalf("failed read returned counters: %+v", stats)
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("GetStats error = %v, want lookup error %v", err, tc.err)
			}
			if errors.Is(err, iface.ErrInterfaceNotFound) {
				t.Fatalf("failed read became absence: %v", err)
			}
			if !strings.Contains(err.Error(), `"stats0"`) {
				t.Fatalf("GetStats error does not name the interface: %v", err)
			}
		})
	}
}

// TestNetlinkGetStatsMissingCounters refuses an otherwise valid link without
// statistics instead of reporting an idle interface.
func TestNetlinkGetStatsMissingCounters(t *testing.T) {
	stubStatsLookup(t, &netlink.Dummy{Name: "stats0"}, nil)
	b := &netlinkBackend{}
	stats, err := b.GetStats("stats0")
	if stats != nil {
		t.Fatalf("missing statistics returned counters: %+v", stats)
	}
	if err == nil {
		t.Fatal("GetStats accepted a link without statistics")
	}
	if errors.Is(err, iface.ErrInterfaceNotFound) {
		t.Fatalf("a link without statistics became absent: %v", err)
	}
	if !strings.Contains(err.Error(), `"stats0"`) {
		t.Fatalf("GetStats error does not name the interface: %v", err)
	}
	if !strings.Contains(err.Error(), "no counters") {
		t.Fatalf("GetStats error does not explain the missing statistics: %v", err)
	}
}

// TestNetlinkGetStatsZeroCounters accepts an allocated, all-zero statistics
// record through GetStats, distinguishing an idle link from missing counters.
func TestNetlinkGetStatsZeroCounters(t *testing.T) {
	stubStatsLookup(t, &netlink.Dummy{
		Name: "stats0", Statistics: &netlink.LinkStatistics{},
	}, nil)
	b := &netlinkBackend{}
	stats, err := b.GetStats("stats0")
	if err != nil {
		t.Fatalf("GetStats refused zero counters: %v", err)
	}
	if stats == nil {
		t.Fatal("GetStats returned no counters for an idle link")
	}
	if *stats != (iface.InterfaceStats{}) {
		t.Fatalf("GetStats invented counters: %+v", stats)
	}
}

// TestNetlinkGetStatsMeasuredCounters checks every counter through GetStats
// with distinct values so a constant answer or a field swap cannot pass.
func TestNetlinkGetStatsMeasuredCounters(t *testing.T) {
	stubStatsLookup(t, &netlink.Dummy{
		Name: "stats0",
		Statistics: &netlink.LinkStatistics{
			RxBytes: 101, RxPackets: 102, RxErrors: 103, RxDropped: 104, Multicast: 105,
			TxBytes: 201, TxPackets: 202, TxErrors: 203, TxDropped: 204,
		},
	}, nil)
	b := &netlinkBackend{}
	stats, err := b.GetStats("stats0")
	if err != nil {
		t.Fatalf("GetStats refused measured counters: %v", err)
	}
	if stats == nil {
		t.Fatal("GetStats returned no measured counters")
	}
	want := iface.InterfaceStats{
		RxBytes: 101, RxPackets: 102, RxErrors: 103, RxDropped: 104, RxMulticast: 105,
		TxBytes: 201, TxPackets: 202, TxErrors: 203, TxDropped: 204,
	}
	if *stats != want {
		t.Fatalf("GetStats counters = %+v, want %+v", *stats, want)
	}
}

// stubStatsLookup supplies one kernel answer while checking that GetStats
// queried the requested interface exactly once. Callers MUST NOT run in parallel.
func stubStatsLookup(t *testing.T, link netlink.Link, lookupErr error) {
	t.Helper()
	previous := linkByNameForStats
	calls := 0
	linkByNameForStats = func(name string) (netlink.Link, error) {
		calls++
		if name != "stats0" {
			t.Fatalf("lookup name = %q, want stats0", name)
		}
		return link, lookupErr
	}
	t.Cleanup(func() {
		linkByNameForStats = previous
		if calls != 1 {
			t.Errorf("GetStats queried the link %d times, want 1", calls)
		}
	})
}

// statsLinkNotFound gives netlink's not-found type a printable outer error;
// its embedded error field cannot be initialized outside the netlink package.
type statsLinkNotFound struct{}

func (statsLinkNotFound) Error() string { return "link not found" }

func (statsLinkNotFound) Unwrap() error { return netlink.LinkNotFoundError{} }
