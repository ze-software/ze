// Design: docs/architecture/ospf/ospf-4-component-config.md -- the problems OSPF raises on the daemon's report bus
// Related: issues.go -- the report queue these tests replace
package ospf

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/opererr"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// recordedIssues is a publisher that keeps every raise, standing in for the SDK plugin
// whose report-issue reaches the daemon. Safe for concurrent use: the report queue's
// worker calls it.
type recordedIssues struct {
	mu     sync.Mutex
	issues []sdk.Issue
	queue  *issuequeue.Queue
}

func (r *recordedIssues) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.issues = append(r.issues, issue)
	return nil
}

// ClearIssue takes a clear and keeps nothing: OSPF raises only error events, which
// are never cleared.
func (r *recordedIssues) ClearIssue(context.Context, string, string) error { return nil }

// withCode answers the raises made under code, in the order they were made, once the
// queue has sent every report queued before the call.
func (r *recordedIssues) withCode(code string) []sdk.Issue {
	r.queue.Flush()
	r.mu.Lock()
	defer r.mu.Unlock()
	var found []sdk.Issue
	for _, issue := range r.issues {
		if issue.Code == code {
			found = append(found, issue)
		}
	}
	return slices.Clip(found)
}

// useIssuePublisher starts a report queue over publisher for the test's duration, in
// place of the one runOSPFEngine starts, and puts the previous one back after it. The
// queue is package state, so a test that calls it MUST NOT run in parallel.
func useIssuePublisher(t *testing.T, publisher issuequeue.Publisher) *issuequeue.Queue {
	t.Helper()
	previous := issueQueue.Load()
	queue := issuequeue.Start(publisher)
	issueQueue.Store(queue)
	t.Cleanup(func() {
		issueQueue.Store(previous)
		queue.Stop()
	})
	return queue
}

// recordIssues installs a recordedIssues publisher behind a report queue for the
// test's duration. Its tests MUST NOT run in parallel.
func recordIssues(t *testing.T) *recordedIssues {
	t.Helper()
	recorded := &recordedIssues{}
	recorded.queue = useIssuePublisher(t, recorded)
	return recorded
}

// VALIDATES: spec-dropped-error-6 AC-3 -- a raise made while no report queue is started
// is refused with issuequeue.ErrNotStarted, and the WARN line says so under
// report-error, so a problem whose raise did not reach the daemon says so.
// Method: clear the queue, report a failed IPsec removal, read the WARN line.
// PREVENTS: a raise that reached nobody reading as queued.
func TestRaiseWithoutReportQueueSaysSo(t *testing.T) {
	previous := issueQueue.Load()
	issueQueue.Store(nil)
	t.Cleanup(func() { issueQueue.Store(previous) })
	inst := &ipsecInstaller{}
	sink := warnSinkFor(inst)

	inst.warnRemoval("eth1", "dst fe80::2 proto esp spi 256", opererr.Wrap(errors.New("xfrm: state del: operation not permitted"),
		"OSPF IPsec could not remove the kernel security association", ipsecRemovalWhy,
		"ip -6 xfrm state delete dst fe80::2 proto esp spi 256"))

	lines := warnLines(sink.String())
	if len(lines) != 1 {
		t.Fatalf("WARN lines = %d, want 1:\n%s", len(lines), sink.String())
	}
	for _, want := range []string{"report-error=", issuequeue.ErrNotStarted.Error()} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("WARN line lacks %q:\n%s", want, lines[0])
		}
	}
	if strings.Contains(lines[0], "report=queued") {
		t.Errorf("a raise refused for want of a queue reads as queued:\n%s", lines[0])
	}
}

// heldPublisher holds every raise until release is closed or the raise's context
// ends, as a daemon that is slow to answer does. entered is closed when the first
// raise arrives, and returned counts the raises that came back. Safe for concurrent
// use.
type heldPublisher struct {
	entered     chan struct{}
	release     chan struct{}
	enteredOnce sync.Once
	returned    atomic.Int32
}

func (h *heldPublisher) RaiseIssue(ctx context.Context, _ sdk.Issue) error {
	h.enteredOnce.Do(func() { close(h.entered) })
	defer h.returned.Add(1)
	select {
	case <-h.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *heldPublisher) ClearIssue(context.Context, string, string) error { return nil }

// VALIDATES: spec-dropped-error-6 ISSUE 1 -- a removal the kernel refuses raises its
// error event without holding the IPsec installer's lock while the daemon answers:
// onNeighborSeen, which every received Hello reaches through that lock, returns while
// the removal's raise is still held by the publisher.
// Method: protect a neighbor, make its delete fail, report it lost on one goroutine,
// wait until the publisher holds the raise, then see a second neighbor and require
// that call to return before the publisher does.
// PREVENTS: a slow daemon plus a failing XFRM delete stalling the Hello processing of
// every interface, one report timeout per failed delete.
func TestOSPFIPsecRemovalRaiseHoldsNoInstallerLock(t *testing.T) {
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	warnSinkFor(inst)
	held := &heldPublisher{entered: make(chan struct{}), release: make(chan struct{})}
	useIssuePublisher(t, held)
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")
	lostNeighbor, nextNeighbor := ridMust(t, "10.0.0.2"), ridMust(t, "10.0.0.3")
	inst.onNeighborSeen("eth1", lostNeighbor, netip.MustParseAddr("fe80::2"))
	fake.failRemovals(fmt.Errorf("xfrm: state del spi=256: %w", syscall.EPERM), nil)

	lost := make(chan struct{})
	go func() {
		defer close(lost)
		inst.onNeighborLost("eth1", lostNeighbor)
	}()
	select {
	case <-held.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the failed removal's raise never reached the publisher within 5s")
	}

	seen := make(chan struct{})
	go func() {
		defer close(seen)
		inst.onNeighborSeen("eth1", nextNeighbor, netip.MustParseAddr("fe80::3"))
	}()
	select {
	case <-seen:
	case <-time.After(5 * time.Second):
		t.Fatal("onNeighborSeen did not return within 5s while a removal's raise was held: the raise holds the installer lock")
	}
	if got := held.returned.Load(); got != 0 {
		t.Fatalf("the publisher returned %d raises before onNeighborSeen returned, want 0: onNeighborSeen waited for the daemon", got)
	}
	<-lost
	close(held.release)
}
