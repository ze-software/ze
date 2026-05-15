// Design: docs/architecture/core-design.md -- netlink route subscription (Linux)

//go:build linux

package routewatch

import (
	"net/netip"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func (w *Watcher) subscribe() {
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
