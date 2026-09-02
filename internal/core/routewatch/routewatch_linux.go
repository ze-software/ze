// Design: docs/architecture/core-design.md -- netlink route subscription (Linux)

//go:build linux

package routewatch

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// resubscribeDelay paces re-subscription after the netlink receive loop
// dies (the library closes the update channel on ANY receive error,
// including transient ones like ENOBUFS under route churn).
const resubscribeDelay = time.Second

// subscribeSetupTries bounds consecutive failures to CREATE the subscription
// socket. That is the one failure waiting cannot clear. A daemon without
// CAP_NET_ADMIN is refused on every attempt. An unbounded retry then writes
// one warning a second for the whole life of the process. It never recovers,
// and it buries whatever else the log carries.
//
// A functional test met that flood and spent 45 s in it before its own timeout
// fired (test/plugin/fib-srv6-kernel.ci). Three attempts leave room for a
// transient cause, a temporary file-descriptor shortage for example.
//
// A subscription that was created and then died is the other case. It keeps
// its unbounded retry, because the receive loop dies on ordinary route churn.
const subscribeSetupTries = 3

// platformState pins the network namespace the subscription socket must be
// opened in.
type platformState struct {
	ns netns.NsHandle
}

func newPlatformState() platformState {
	return platformState{ns: netns.None()}
}

// captureNamespace records the network namespace of the goroutine calling
// Start. The subscription goroutine runs on an arbitrary OS thread whose
// namespace depends on when the runtime cloned it (threads created after a
// setns inherit that namespace); without pinning, a namespace-scoped caller
// subscribes in a random namespace. In the normal daemon every thread shares
// the init namespace and this is a no-op in effect.
func (w *Watcher) captureNamespace() {
	ns, err := netns.Get()
	if err != nil {
		if w.errCb != nil {
			w.errCb(err)
		}
		return
	}
	w.platform.ns = ns
}

// subscribe keeps one netlink route subscription alive until Stop. The
// netlink library terminates its receive loop on any error; without the
// retry the watcher would silently stop delivering route events for the
// rest of the process lifetime. Each resubscribe re-lists existing routes
// (ListExisting), so handlers see repeated adds after a gap -- consumers
// reconcile idempotently.
//
// A host that refuses the subscription outright is the one case retrying
// cannot fix. It gets subscribeSetupTries attempts, and then a report naming
// what the watcher needs and what the caller has lost. Ending quietly is not
// an option. Nothing downstream can tell an absence of external route changes
// from a monitor that never started (ai/rules/principles.md).
func (w *Watcher) subscribe() {
	defer func() {
		if w.platform.ns.IsOpen() {
			_ = w.platform.ns.Close()
		}
	}()
	setupFailures := 0
	for {
		err := w.subscribeOnce()
		if err == nil {
			setupFailures = 0
		} else {
			setupFailures++
			if w.errCb != nil {
				w.errCb(err)
			}
			if setupFailures >= subscribeSetupTries {
				w.reportSetupGaveUp(setupFailures, err)
				return
			}
		}
		select {
		case <-w.stopCh:
			return
		case <-time.After(resubscribeDelay):
		}
	}
}

// reportSetupGaveUp says that route monitoring has stopped, why waiting will
// not restart it, and which capability an operator has to grant. The message
// names CAP_NET_ADMIN because that is what both halves of the setup need: the
// netlink route socket, and the setns into the pinned namespace.
func (w *Watcher) reportSetupGaveUp(attempts int, last error) {
	if w.errCb == nil {
		return
	}
	w.errCb(fmt.Errorf("route monitor stopped after %d failed subscriptions."+
		" External route changes are no longer detected."+
		" The netlink route subscription needs CAP_NET_ADMIN: %w", attempts, last))
}

// subscribeOnce runs a single subscription until the kernel socket dies or
// Stop is called. It answers the error that stopped the subscription from
// being CREATED. It answers nil once the socket existed, whatever later ended
// the receive loop. The two are different failures. The caller retries a dead
// receive loop for as long as the process runs, and a refused socket only a
// few times.
func (w *Watcher) subscribeOnce() error {
	updates := make(chan netlink.RouteUpdate, 64)
	done := make(chan struct{})
	defer close(done)

	opts := netlink.RouteSubscribeOptions{
		ListExisting: true,
		ErrorCallback: func(err error) {
			if w.errCb != nil {
				w.errCb(err)
			}
		},
	}
	if w.platform.ns.IsOpen() {
		opts.Namespace = &w.platform.ns
	}
	if err := netlink.RouteSubscribeWithOptions(updates, done, opts); err != nil {
		return fmt.Errorf("subscribe to netlink route updates: %w", err)
	}

	for {
		select {
		case <-w.stopCh:
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Dst == nil {
				continue
			}
			if update.Type != unix.RTM_NEWROUTE && update.Type != unix.RTM_DELROUTE {
				continue
			}

			ip, ok := netip.AddrFromSlice(update.Dst.IP)
			if !ok {
				continue
			}
			ones, _ := update.Dst.Mask.Size()
			prefix := netip.PrefixFrom(ip.Unmap(), ones)

			var nextHop netip.Addr
			if update.Gw != nil {
				if nh, ok := netip.AddrFromSlice(update.Gw); ok {
					nextHop = nh.Unmap()
				}
			}

			var action Action
			if update.Type == unix.RTM_NEWROUTE {
				action = ActionAdd
			} else {
				action = ActionRemove
			}

			w.deliver(RouteEvent{
				Prefix:   prefix,
				NextHop:  nextHop,
				Protocol: int(update.Protocol),
				Metric:   uint32(update.Priority),
				Action:   action,
			})
		}
	}
}
