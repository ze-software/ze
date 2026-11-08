// Design: docs/architecture/core-design.md -- netlink route subscription (Linux)

//go:build linux

package routewatch

import (
	"fmt"
	"net/netip"
	"runtime"
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

// stopping reports whether Stop has been called.
func (w *Watcher) stopping() bool {
	select {
	case <-w.stopCh:
		return true
	default:
		return false
	}
}

// reportSetupGaveUp says that route monitoring has stopped, why waiting will
// not restart it, and which capability an operator has to grant. The message
// names CAP_NET_ADMIN. The setns into a pinned namespace the opening thread is
// not in needs CAP_SYS_ADMIN as well, and namespaceToEnter skips it in the
// usual case, where the daemon runs in the namespace it pinned.
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
//
// The netlink library runs its own receive goroutine, which calls the error
// callback and closes updates when it exits. On Stop, subscribeOnce closes
// done (the library then closes the socket) and drains updates until that
// goroutine has closed it. It returns only once the receive goroutine is
// gone, so no callback runs after Wait returns. The receive error the close
// itself causes is Ze's own shutdown, not a failure, and is not reported.
func (w *Watcher) subscribeOnce() error {
	updates := make(chan netlink.RouteUpdate, 64)
	done := make(chan struct{})

	opts := netlink.RouteSubscribeOptions{
		ListExisting: true,
		ErrorCallback: func(err error) {
			if w.stopping() {
				return
			}
			if w.errCb != nil {
				w.errCb(err)
			}
		},
	}
	if err := w.openSubscription(updates, done, opts); err != nil {
		// The library may already watch done to close the socket it opened.
		close(done)
		return fmt.Errorf("subscribe to netlink route updates: %w", err)
	}

	for {
		select {
		case <-w.stopCh:
			close(done)
			// Drain until the library's receive goroutine closes updates.
			for range updates {
			}
			return nil
		case update, ok := <-updates:
			if !ok {
				// The receive goroutine has exited; done closes the socket.
				close(done)
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
				TableID:  uint32(update.Table),
				// The output device lets a consumer ask "is there a default
				// route through this interface" (RFC 7084 G-4 on the RA side).
				OutIfindex: update.LinkIndex,
				Action:     action,
			})
		}
	}
}

// openSubscription opens the netlink route subscription in the pinned
// namespace. The OS thread stays locked until the socket is open, so the
// namespace namespaceToEnter compared is the one the socket opens in; once
// open, a socket keeps its namespace whichever thread reads it.
func (w *Watcher) openSubscription(updates chan<- netlink.RouteUpdate, done <-chan struct{}, opts netlink.RouteSubscribeOptions) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	target, err := w.namespaceToEnter()
	if err != nil {
		return err
	}
	if target.IsOpen() {
		opts.Namespace = &target
	}
	return netlink.RouteSubscribeWithOptions(updates, done, opts)
}

// namespaceToEnter returns the namespace the netlink library must move the
// calling thread into before it opens a socket: the pinned namespace, or
// netns.None() when the thread already runs in it, or when nothing was
// pinned. The caller MUST hold its OS thread locked from this call until the
// socket is open, or the answer may describe another thread.
//
// The library enters a namespace it is given with setns(2) even when the
// thread is already in it, and the kernel allows that only with
// CAP_SYS_ADMIN (netns_install). A daemon granted CAP_NET_ADMIN alone, as a
// container usually is, then had both its subscription and ListRoutes
// refused: no route event and no listed route reached any consumer, and the
// iface-ra sender of a CE unit read its WAN as holding no default route, so
// every LAN advertisement carried Router Lifetime 0. Entering only a namespace
// the thread is not in leaves CAP_SYS_ADMIN to the one case that needs it.
func (w *Watcher) namespaceToEnter() (netns.NsHandle, error) {
	if !w.platform.ns.IsOpen() {
		return netns.None(), nil
	}
	current, err := netns.Get()
	if err != nil {
		return netns.None(), fmt.Errorf("read the network namespace of the thread opening the route socket: %w", err)
	}
	same := current.Equal(w.platform.ns)
	_ = current.Close()
	if same {
		return netns.None(), nil
	}
	return w.platform.ns, nil
}

// ListRoutes returns the main-table routes for prefix as add events, read
// once from the kernel in the watcher's namespace. The subscription lists the
// existing routes only when it opens, so a handler registered after Start
// reads them here. It MUST be called after Start, which pins the namespace.
func (w *Watcher) ListRoutes(prefix netip.Prefix) ([]RouteEvent, error) {
	family := netlink.FAMILY_V4
	if prefix.Addr().Is6() {
		family = netlink.FAMILY_V6
	}
	filter := &netlink.Route{Table: unix.RT_TABLE_MAIN}
	var routes []netlink.Route

	// The thread stays locked until the listing socket is open, so the
	// namespace namespaceToEnter compared is the one the socket opens in.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	target, err := w.namespaceToEnter()
	if err != nil {
		return nil, fmt.Errorf("list routes for %s: %w", prefix, err)
	}
	if target.IsOpen() {
		h, herr := netlink.NewHandleAt(target)
		if herr != nil {
			return nil, fmt.Errorf("list routes: open a handle in the watched namespace: %w", herr)
		}
		defer h.Close()
		routes, err = h.RouteListFiltered(family, filter, netlink.RT_FILTER_TABLE)
	} else {
		routes, err = netlink.RouteListFiltered(family, filter, netlink.RT_FILTER_TABLE)
	}
	if err != nil {
		return nil, fmt.Errorf("list routes for %s: %w", prefix, err)
	}
	want := prefix.Masked()
	var out []RouteEvent
	for i := range routes {
		r := &routes[i]
		dst, ok := routeDst(r, prefix.Addr().Is6())
		if !ok {
			continue
		}
		if dst != want {
			continue
		}
		var nextHop netip.Addr
		if nh, ok := netip.AddrFromSlice(r.Gw); ok {
			nextHop = nh.Unmap()
		}
		out = append(out, RouteEvent{
			Prefix:     dst,
			NextHop:    nextHop,
			Protocol:   int(r.Protocol),
			Metric:     uint32(r.Priority), //nolint:gosec // the kernel metric is a u32
			TableID:    uint32(r.Table),    //nolint:gosec // the kernel table id is a u32
			OutIfindex: r.LinkIndex,
			Action:     ActionAdd,
		})
	}
	return out, nil
}

// routeDst returns a listed route's destination. The library reports a
// default route with no destination at all, which is the zero-length prefix
// of the listed family.
func routeDst(r *netlink.Route, v6 bool) (netip.Prefix, bool) {
	if r.Dst == nil {
		if v6 {
			return netip.PrefixFrom(netip.IPv6Unspecified(), 0), true
		}
		return netip.PrefixFrom(netip.IPv4Unspecified(), 0), true
	}
	ip, ok := netip.AddrFromSlice(r.Dst.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	ones, _ := r.Dst.Mask.Size()
	return netip.PrefixFrom(ip.Unmap(), ones), true
}
