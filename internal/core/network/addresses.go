// Design: docs/features/interfaces.md — Bus Topics: why BGP hears address changes from a watch, not a bus topic
// Overview: connected.go — InterfacePrefixes, the read a watch signal says to repeat
// RFC: rfc/short/rfc2545.md
//
// The address watch is how BGP learns that the host's interface addresses
// moved without importing the interface component and without waiting for its
// monitor, which runs only when an interface block configures a backend. The
// netlink interface backend registers the watcher at init(); InterfacePrefixes
// stays the read. With no watcher linked, WatchAddresses answers
// ErrAddressWatchNotRegistered, never a watch that stays silent
// (ai/rules/principles.md).

package network

import (
	"context"
	"errors"
	"sync"
)

// AddressWatcher starts a watch of the kernel interface address table that
// InterfacePrefixes reads, or answers why it cannot. A started watch sends on
// the returned channel after each address update the kernel reports: an
// address that appears, goes away, or changes its flags (an IPv6 address that
// leaves duplicate address detection). The channel holds ONE pending signal
// and the watch never blocks on it, so a burst coalesces: a signal says "read
// the table again", never which address moved.
//
// The watch closes the channel when ctx ends, and also when it stops on its
// own (the kernel subscription failed). A close while ctx is live therefore
// means the table is no longer watched, and the caller starts a new watch to
// hear changes again. The watcher owns the goroutine behind the channel; that
// goroutine has exited once the channel is closed.
type AddressWatcher func(ctx context.Context) (<-chan struct{}, error)

// ErrAddressWatchNotRegistered is the answer of WatchAddresses in a build where
// no package registered an address watcher, which is distinct from a watch
// that saw no change.
var ErrAddressWatchNotRegistered = errors.New("no interface address watcher is registered: this build or platform has no netlink interface backend")

var addressWatch struct {
	sync.Mutex
	watch AddressWatcher
}

// RegisterAddressWatcher installs the one watcher of the kernel address table.
// The netlink interface backend calls it from its init(), so this package
// imports no plugin. A second registration, or a nil watcher, is a build
// defect.
func RegisterAddressWatcher(watch AddressWatcher) {
	if watch == nil {
		panic("BUG: address watcher registered as nil")
	}
	addressWatch.Lock()
	defer addressWatch.Unlock()
	if addressWatch.watch != nil {
		panic("BUG: address watcher registered twice")
	}
	addressWatch.watch = watch
}

// WatchAddresses starts a watch of the kernel address table through the
// registered watcher; AddressWatcher states the channel contract. Each call is
// a watch of its own. Safe for concurrent use.
func WatchAddresses(ctx context.Context) (<-chan struct{}, error) {
	addressWatch.Lock()
	watch := addressWatch.watch
	addressWatch.Unlock()
	if watch == nil {
		return nil, ErrAddressWatchNotRegistered
	}
	return watch(ctx)
}
