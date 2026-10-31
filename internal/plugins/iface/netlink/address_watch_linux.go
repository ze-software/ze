// Design: docs/features/interfaces.md -- Bus Topics: the address table watch BGP runs with or without an interface block
// Overview: ifacenetlink.go -- package hub
// Related: register_address_watch_linux.go -- installs watchKernelAddresses as the network address watcher
// RFC: rfc/short/rfc2545.md

//go:build linux

package ifacenetlink

import (
	"context"
	"fmt"
	"sync"

	"github.com/vishvananda/netlink"
)

// addressWatchQueueDepth is the depth of the channel the netlink subscription
// delivers address updates on. The watch only leaves a signal for each one, so
// it drains the queue as fast as updates arrive; the kernel side is bounded by
// monitorReceiveBufferBytes, and an overflow there ends the subscription,
// which the consumer answers by watching again and reading the table whole.
const addressWatchQueueDepth = 64

// watchKernelAddresses is the network.AddressWatcher this package registers.
// It opens a netlink subscription of its own to RTM_NEWADDR and RTM_DELADDR
// for both families, independent of the interface monitor and of which
// interface backend is active: the monitor runs only when an interface block
// configures a backend, while BGP needs address changes in every config (RFC
// 2545 Section 3's connected subnets, and a listener for an address added
// after start). The kernel table is what network.InterfacePrefixes reads
// whichever backend programs the interfaces, so the watch follows the kernel
// rather than a backend.
//
// Each update signals, flag changes included: an IPv6 address that leaves
// duplicate address detection is reported again with its tentative flag
// clear, so a consumer that could not bind it while tentative binds it then.
//
// Each watch is one long-lived goroutine (addressWatch.run) reading the
// subscription channel; it ends with ctx or when the subscription fails, and
// closes the returned channel last.
func watchKernelAddresses(ctx context.Context) (<-chan struct{}, error) {
	w := newAddressWatch()
	if err := netlink.AddrSubscribeWithOptions(w.updates, w.stop, netlink.AddrSubscribeOptions{
		ReceiveBufferSize: monitorReceiveBufferBytes,
		ErrorCallback:     w.recordFailure,
	}); err != nil {
		close(w.stop)
		return nil, fmt.Errorf("subscribe to kernel address changes: %w", err)
	}
	go w.run(ctx)
	return w.changes, nil
}

// addressWatch is one running watchKernelAddresses. run is its only goroutine.
type addressWatch struct {
	updates chan netlink.AddrUpdate
	// stop ends the netlink subscription; closing it closes its socket, after
	// which the subscription closes updates.
	stop    chan struct{}
	changes chan struct{}

	failureMu sync.Mutex
	failure   error
}

// newAddressWatch builds the channels of one watch, before its subscription.
func newAddressWatch() *addressWatch {
	return &addressWatch{
		updates: make(chan netlink.AddrUpdate, addressWatchQueueDepth),
		stop:    make(chan struct{}),
		changes: make(chan struct{}, 1),
	}
}

// recordFailure keeps the last error the subscription reported, so the line
// that says the watch ended names why. The subscription calls it from its own
// goroutine.
func (w *addressWatch) recordFailure(err error) {
	w.failureMu.Lock()
	w.failure = err
	w.failureMu.Unlock()
}

// run reads the subscription until ctx ends or the subscription fails. It
// releases the subscription before it closes changes, so a caller that sees
// changes closed knows no goroutine of this watch runs.
func (w *addressWatch) run(ctx context.Context) {
	defer close(w.changes)
	defer w.release()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-w.updates:
			if !ok {
				w.failureMu.Lock()
				failure := w.failure
				w.failureMu.Unlock()
				logger().Warn("kernel address table watch ended: the netlink subscription closed",
					"error", failure,
					"effect", "the consumer starts a new watch and reads the address table again",
					"fix", "none if the watch starts again; if this repeats, raise net.core.rmem_max for an ENOBUFS error, and look for kernel messages with: dmesg | grep -i netlink")
				return
			}
			w.signal()
		}
	}
}

// release closes the subscription and drains it until it closes updates, so
// its reader never blocks on a send nobody receives.
func (w *addressWatch) release() {
	close(w.stop)
	for {
		if _, open := <-w.updates; !open {
			return
		}
	}
}

// signal leaves ONE pending signal and never blocks: the consumer re-reads the
// whole table, so a second change while one is pending is already covered.
func (w *addressWatch) signal() {
	select {
	case w.changes <- struct{}{}:
	default:
	}
}
