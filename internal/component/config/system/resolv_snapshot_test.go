package system

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// TestConfigureWithoutChangePublishesNothing: a Configure or SetLease that
// changes no record and no router selection publishes no snapshot. Method: a
// registry holding a host resolv.conf record and a DHCPv4 lease is
// configured again with the same input a minute later on its clock, and
// takes the same lease again; the Sequence, the watcher and the host
// record's received time must not move. A real change then publishes once.
//
// VALIDATES: refresh publishes only on a change (spec-home-cpe-23b).
// PREVENTS: every reload re-reading the host resolv.conf, stamping its record
// with a new received time and publishing a snapshot that changes nothing,
// which made each consumer plugin relearn, and made a replay test pass
// without the replay it exists to prove.
func TestConfigureWithoutChangePublishesNothing(t *testing.T) {
	clk := &manualClock{now: time.Unix(1_000_000, 0)}
	host := filepath.Join(t.TempDir(), "resolv.conf")
	if err := os.WriteFile(host, []byte("nameserver 192.0.2.99\n"), 0o600); err != nil {
		t.Fatalf("write host resolv.conf: %v", err)
	}
	r := &Resolvers{clock: clk, hostPath: host}
	if err := r.Configure("", []string{"198.51.100.53"}, routerDefault); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, UntilWithdrawn); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	before := r.Snapshot()
	if len(before.Records) != 2 {
		t.Fatalf("records = %+v, want the host record and the DHCPv4 lease", before.Records)
	}
	var seen []uint64
	stop := r.Watch(func(snapshot *dnssource.Snapshot) { seen = append(seen, snapshot.Sequence) })
	defer stop()
	seen = seen[:0] // Watch calls fn at once with the current snapshot.

	clk.advance(time.Minute)
	if err := r.Configure("", []string{"198.51.100.53"}, routerDefault); err != nil {
		t.Fatalf("Configure again: %v", err)
	}
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, UntilWithdrawn); err != nil {
		t.Fatalf("SetLease again: %v", err)
	}
	after := r.Snapshot()
	if after.Sequence != before.Sequence {
		t.Fatalf("Sequence = %d after a Configure and a SetLease that changed nothing, want %d", after.Sequence, before.Sequence)
	}
	if len(seen) != 0 {
		t.Fatalf("watcher saw sequences %v, want none", seen)
	}
	if !dnssource.SameRecords(after.Records, before.Records) {
		t.Fatalf("records = %+v, want %+v (received times kept)", after.Records, before.Records)
	}

	if err := r.Configure("", []string{"198.51.100.54"}, routerDefault); err != nil {
		t.Fatalf("Configure with a new name-server: %v", err)
	}
	if got, want := r.Snapshot().Sequence, before.Sequence+1; got != want {
		t.Fatalf("Sequence = %d after a changed name-server, want %d", got, want)
	}
	if len(seen) != 1 {
		t.Fatalf("watcher saw sequences %v, want one for the change", seen)
	}
}
