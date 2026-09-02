// Design: docs/architecture/core-design.md -- netlink route subscription (Linux)
// Related: routewatch_linux.go -- subscribe, subscribeSetupTries, reportSetupGaveUp

//go:build linux

package routewatch

import (
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// subscribeGiveUpBudget bounds the test's own wait. subscribe pauses
// resubscribeDelay between attempts. A run that keeps its bound therefore
// finishes in about (subscribeSetupTries-1) * resubscribeDelay. The budget is
// several times that. The failure this test exists to catch is a watcher that
// never returns at all.
const subscribeGiveUpBudget = 30 * time.Second

// TestARefusedSubscriptionStopsAfterItsBoundAndNamesWhatItNeeds drives the
// failure an unprivileged daemon meets: the netlink route subscription is
// refused on every attempt, so no amount of waiting produces a socket.
//
// The method is a pinned namespace handle that names a file which is not a
// namespace. netns.Set then fails inside RouteSubscribeWithOptions on every
// call, on any kernel and at any privilege level. So the test needs no
// capability of its own, and it asserts nothing about the host.
//
// Two properties are checked, and both were absent before subscribe counted
// its setup failures. It gives up, rather than writing one warning a second
// for the life of the process. And the last thing it says names the lost
// behavior and the capability, rather than the kernel's own errno.
func TestARefusedSubscriptionStopsAfterItsBoundAndNamesWhatItNeeds(t *testing.T) {
	// A raw fd, not an *os.File. subscribe closes the handle it was given. An
	// os.File finalizer that closed the same number again would reach whatever
	// unrelated fd this process opened next.
	fd, err := unix.Open("/dev/null", unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}

	w := New()
	w.platform.ns = netns.NsHandle(fd)
	if !w.platform.ns.IsOpen() {
		t.Fatalf("the handle must read as open, or subscribe takes the unpinned path")
	}
	var reports []error
	w.errCb = func(err error) { reports = append(reports, err) }

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		w.subscribe()
	}()

	select {
	case <-returned:
	case <-time.After(subscribeGiveUpBudget):
		t.Fatalf("subscribe never returned: a refused subscription is being retried without a bound")
	}

	// One report for each refused attempt, and one for the decision to stop.
	if len(reports) != subscribeSetupTries+1 {
		t.Fatalf("reports = %d, want %d: %v", len(reports), subscribeSetupTries+1, reports)
	}
	last := reports[len(reports)-1].Error()
	for _, want := range []string{"route monitor stopped", "no longer detected", "CAP_NET_ADMIN"} {
		if !strings.Contains(last, want) {
			t.Errorf("the give-up report does not name %q: %s", want, last)
		}
	}
}
