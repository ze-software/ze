// Design: docs/architecture/iface/netlink-monitor.md -- kernel netlink event streaming
// Related: interface_rate.go -- existing streaming monitor handler in iface/cmd
//
//go:build linux

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	actionNew     = "new"
	actionDel     = "del"
	prefixDefault = "default"
)

// Members every netlink monitor event carries, whatever its kind. A consumer
// reads the stream one JSON object per line and switches on eventKeyType.
const (
	eventKeyType      = "type"
	eventKeyAction    = "action"
	eventKeyTimestamp = "timestamp"
)

func streamNetlinkMonitor(ctx context.Context, _ *pluginserver.Server, w io.Writer, _ string, args []string) error {
	group, err := netlinkGroupFromArgs(args)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	defer close(done)

	eventCh := make(chan map[string]any, 64)

	if group == netlinkGroupRoute || group == netlinkGroupAll {
		routeCh := make(chan netlink.RouteUpdate, 64)
		if err := netlink.RouteSubscribeWithOptions(routeCh, done, netlink.RouteSubscribeOptions{}); err != nil {
			return fmt.Errorf("route subscribe: %w", err)
		}
		go forwardRouteUpdates(ctx, routeCh, eventCh)
	}

	if group == netlinkGroupLink || group == netlinkGroupAll {
		linkCh := make(chan netlink.LinkUpdate, 64)
		if err := netlink.LinkSubscribeWithOptions(linkCh, done, netlink.LinkSubscribeOptions{}); err != nil {
			return fmt.Errorf("link subscribe: %w", err)
		}
		go forwardLinkUpdates(ctx, linkCh, eventCh)
	}

	if group == netlinkGroupAddress || group == netlinkGroupAll {
		addrCh := make(chan netlink.AddrUpdate, 64)
		if err := netlink.AddrSubscribeWithOptions(addrCh, done, netlink.AddrSubscribeOptions{}); err != nil {
			return fmt.Errorf("addr subscribe: %w", err)
		}
		go forwardAddrUpdates(ctx, addrCh, eventCh)
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-eventCh:
			if !ok {
				return nil
			}
			if err := enc.Encode(ev); err != nil {
				return err
			}
		}
	}
}

func forwardRouteUpdates(ctx context.Context, ch <-chan netlink.RouteUpdate, out chan<- map[string]any) {
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-ch:
			if !ok {
				return
			}
			ev := routeUpdateToEvent(&update)
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}
}

func forwardLinkUpdates(ctx context.Context, ch <-chan netlink.LinkUpdate, out chan<- map[string]any) {
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-ch:
			if !ok {
				return
			}
			ev := linkUpdateToEvent(update)
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}
}

func forwardAddrUpdates(ctx context.Context, ch <-chan netlink.AddrUpdate, out chan<- map[string]any) {
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-ch:
			if !ok {
				return
			}
			ev := addrUpdateToEvent(update)
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}
}

var ifNameCache sync.Map

// ifName resolves an interface index to a name for the route and address
// events, which carry an index and no name.
//
// Only linkUpdateToEvent writes ifNameCache, because a link message names the
// link it is about. This lookup deliberately does not write it: a kernel
// interface index is reusable, so a route or address update still in flight
// for a deleted link would otherwise cache the NEW link's name against that
// index and mislabel every later event until the next link message corrected
// it. An uncached miss costs one netlink call on a monitor stream.
func ifName(index int) string {
	if name, ok := ifNameCache.Load(index); ok {
		if s, isStr := name.(string); isStr {
			return s
		}
	}
	link, err := netlink.LinkByIndex(index)
	if err != nil {
		return ""
	}
	return link.Attrs().Name
}

func routeUpdateToEvent(u *netlink.RouteUpdate) map[string]any {
	action := actionNew
	if u.Type == unix.RTM_DELROUTE {
		action = actionDel
	}

	ev := map[string]any{
		eventKeyType:      "route",
		eventKeyAction:    action,
		eventKeyTimestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if u.Dst != nil {
		ev["prefix"] = u.Dst.String()
	} else {
		ev["prefix"] = prefixDefault
	}

	if u.Gw != nil {
		ev["gateway"] = u.Gw.String()
	}

	if u.LinkIndex > 0 {
		if name := ifName(u.LinkIndex); name != "" {
			ev["interface"] = name
		}
		ev["interface-index"] = u.LinkIndex
	}

	ev["table"] = u.Table
	ev["protocol"] = u.Protocol
	ev["scope"] = int(u.Scope)
	ev["priority"] = u.Priority

	return ev
}

func linkUpdateToEvent(u netlink.LinkUpdate) map[string]any {
	action := "change"
	if u.Header.Type == unix.RTM_DELLINK {
		action = actionDel
	}

	attrs := u.Attrs()
	if u.Header.Type == unix.RTM_DELLINK {
		ifNameCache.Delete(attrs.Index)
	} else {
		ifNameCache.Store(attrs.Index, attrs.Name)
	}
	flags := attrs.Flags

	state := "down"
	if flags&net.FlagUp != 0 {
		state = "up"
	}

	ev := map[string]any{
		eventKeyType:      "link",
		eventKeyAction:    action,
		eventKeyTimestamp: time.Now().UTC().Format(time.RFC3339),
		"interface":       attrs.Name,
		"interface-index": attrs.Index,
		"state":           state,
		"mtu":             attrs.MTU,
	}

	if attrs.HardwareAddr != nil {
		ev["mac"] = attrs.HardwareAddr.String()
	}

	if flags&net.FlagLoopback != 0 {
		ev["loopback"] = true
	}

	return ev
}

func addrUpdateToEvent(u netlink.AddrUpdate) map[string]any {
	action := actionDel
	if u.NewAddr {
		action = actionNew
	}

	ev := map[string]any{
		eventKeyType:      "address",
		eventKeyAction:    action,
		eventKeyTimestamp: time.Now().UTC().Format(time.RFC3339),
		"address":         u.LinkAddress.String(),
		"interface-index": u.LinkIndex,
	}

	if name := ifName(u.LinkIndex); name != "" {
		ev["interface"] = name
	}

	return ev
}
