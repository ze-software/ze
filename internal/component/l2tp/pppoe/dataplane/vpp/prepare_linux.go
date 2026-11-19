// Design: docs/architecture/l2tp/bng-5-pppoe.md -- actual Linux LCP readback
// Related: prepare.go -- publication requires exact pair and Linux identities

//go:build linux

package vpp

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

type controlLinkResult struct {
	link controlLink
	err  error
}

// readControlLink reads the CP device in its actual LCP namespace. A one-time
// access-setup worker keeps the caller's thread untouched. NewHandleAt's
// vendored namespace helper ignores restore errors, so the worker deliberately
// exits still locked: Go discards its thread even if that restoration failed.
// The caller joins this lifecycle step before publishing the preparation.
func readControlLink(ctx context.Context, pair iface.LCPPair) (controlLink, error) {
	result := make(chan controlLinkResult, 1)
	go func() {
		runtime.LockOSThread()
		link, err := readControlLinkThread(ctx, pair)
		result <- controlLinkResult{link: link, err: err}
		// No UnlockOSThread: see the namespace restoration contract above.
	}()
	read := <-result
	return read.link, read.err
}

func readControlLinkThread(ctx context.Context, pair iface.LCPPair) (link controlLink, err error) {
	if err := ctx.Err(); err != nil {
		return controlLink{}, err
	}
	namespace := netns.None()
	if pair.Namespace != "" {
		namespace, err = netns.GetFromName(pair.Namespace)
		if err != nil {
			return controlLink{}, fmt.Errorf("open LCP namespace %q: %w", pair.Namespace, err)
		}
		defer func() { err = errors.Join(err, namespace.Close()) }()
	}
	handle, err := netlink.NewHandleAt(namespace, unix.NETLINK_ROUTE)
	if err != nil {
		return controlLink{}, fmt.Errorf("open LCP link reader: %w", err)
	}
	defer handle.Close()
	timeout := apiTimeout
	if deadline, exists := ctx.Deadline(); exists {
		timeout = min(timeout, time.Until(deadline))
	}
	if timeout <= 0 {
		return controlLink{}, context.DeadlineExceeded
	}
	if err := handle.SetSocketTimeout(timeout); err != nil {
		return controlLink{}, err
	}
	device, err := handle.LinkByIndex(pair.LinuxIndex)
	if err != nil {
		return controlLink{}, fmt.Errorf("read Linux LCP device %q: %w", pair.ControlDevice, err)
	}
	if device == nil {
		return controlLink{}, errors.New("Linux LCP readback returned no device")
	}
	attributes := device.Attrs()
	if attributes == nil {
		return controlLink{}, errors.New("Linux LCP readback returned no attributes")
	}
	if attributes.Name != pair.ControlDevice {
		return controlLink{}, errors.New("Linux LCP readback names another device")
	}
	if attributes.Index != pair.LinuxIndex {
		return controlLink{}, errors.New("Linux LCP readback has another interface index")
	}
	if len(attributes.HardwareAddr) != 6 {
		return controlLink{}, errors.New("Linux LCP device has no Ethernet MAC address")
	}
	if err := ctx.Err(); err != nil {
		return controlLink{}, err
	}
	link = controlLink{name: attributes.Name, index: attributes.Index, mtu: attributes.MTU}
	copy(link.mac[:], attributes.HardwareAddr)
	return link, nil
}
