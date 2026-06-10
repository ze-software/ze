// Design: docs/architecture/core-design.md -- netlink route subscription (Linux)

//go:build linux

package routewatch

import (
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
func (w *Watcher) subscribe() {
	defer func() {
		if w.platform.ns.IsOpen() {
			_ = w.platform.ns.Close()
		}
	}()
	for {
		w.subscribeOnce()
		select {
		case <-w.stopCh:
			return
		case <-time.After(resubscribeDelay):
		}
	}
}

// subscribeOnce runs a single subscription until the kernel socket dies or
// Stop is called.
func (w *Watcher) subscribeOnce() {
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
		if w.errCb != nil {
			w.errCb(err)
		}
		return
	}

	for {
		select {
		case <-w.stopCh:
			return
		case update, ok := <-updates:
			if !ok {
				return
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
