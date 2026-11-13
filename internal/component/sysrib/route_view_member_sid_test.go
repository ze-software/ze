// Design: docs/architecture/rib/unified-locrib.md -- selected member SID recovery.

package sysrib

import (
	"context"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRouteViewRuntimeSelectedMemberSIDLossRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	feed := newRouteViewRuntimeFeed(t, ctx)
	prefix := netip.MustParsePrefix("198.18.233.0/24")
	primaryCover := netip.MustParsePrefix("192.0.2.0/24")
	memberCover := netip.MustParsePrefix("2001:db8:233::/64")
	primarySID := netip.MustParseAddr("2001:db8:234::1")
	memberSID := netip.MustParseAddr("2001:db8:233::1")
	sidCover := func(prefix netip.Prefix) rpc.RouteViewEntry {
		entry := runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "sid-port"})
		entry.AFI = uint16(family.AFIIPv6)
		return entry
	}
	feed.pages <- runtimeSnapshot(5, true,
		runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, NextHop: netip.MustParseAddr("192.0.2.9"), Labels: []uint32{101}, SRv6SID: primarySID,
			ECMP: []rpc.RouteViewNextHop{{Interface: "member", Weight: 3, Labels: []uint32{202}, SRv6SID: memberSID}}}),
		runtimeSelected(primaryCover, rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "primary"}),
		sidCover(memberCover), sidCover(netip.MustParsePrefix("2001:db8:234::/64")))
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	primary := feed.awaitChange(t, ctx, prefix, false)
	if primary.Interface != "primary" || primary.SRv6SID != primarySID || !slices.Equal(primary.Labels, []uint32{101}) {
		t.Fatalf("healthy primary SID control was not installed: %+v", primary)
	}
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 6, Phase: rpc.RouteViewPhaseDelta, Complete: true,
		Entries: []rpc.RouteViewEntry{{AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast), Prefix: primaryCover, Action: rpc.RouteViewActionWithdraw}}}
	control := feed.awaitChange(t, ctx, prefix, false)
	if control.Interface != "member" || control.SRv6SID != memberSID || !slices.Equal(control.Labels, []uint32{202}) {
		t.Fatalf("reachable member SID control was not promoted: %+v", control)
	}
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 7, Phase: rpc.RouteViewPhaseDelta, Complete: true,
		Entries: []rpc.RouteViewEntry{{AFI: uint16(family.AFIIPv6), SAFI: uint8(family.SAFIUnicast), Prefix: memberCover, Action: rpc.RouteViewActionWithdraw}}}
	lost := feed.awaitChange(t, ctx, prefix, false)
	if lost.Action != routeaction.Withdraw {
		t.Fatalf("member SID lost coverage but retained forwarding: %+v", lost)
	}
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 8, Phase: rpc.RouteViewPhaseDelta, Complete: true,
		Entries: []rpc.RouteViewEntry{sidCover(memberCover)}}
	recovered := feed.awaitChange(t, ctx, prefix, false)
	if recovered.Action != routeaction.Add || recovered.Interface != "member" || recovered.Weight != 3 || recovered.SRv6SID != memberSID || !slices.Equal(recovered.Labels, []uint32{202}) {
		t.Fatalf("member SID recovery needed a producer reannouncement: %+v", recovered)
	}
}
