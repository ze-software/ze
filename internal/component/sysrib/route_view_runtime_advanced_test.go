// Design: docs/architecture/rib/unified-locrib.md -- consumer continuation proofs.

package sysrib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Candidates must keep one separately pinned scan and return the owner's exact
// rows, including numbers that cannot survive a float64 intermediate.
func TestRouteViewRuntimeCandidatesKeepPinnedRawRows(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	rows := []json.RawMessage{
		json.RawMessage(`{"prefix":"198.18.220.0/24","metric":18446744073709551614}`),
		json.RawMessage(`{"prefix":"198.18.221.0/24","metric":18446744073709551615}`),
	}
	page := 0
	feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		if request.Method != rpc.MethodRouteViewCandidates {
			return nil, false, nil
		}
		var input rpc.RouteViewCandidatesInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, true, err
		}
		if page >= len(rows) {
			return nil, true, fmt.Errorf("candidate scan continued after completion")
		}
		if input.Session != "runtime-owner" || input.Cursor != uint64(page) || input.Limit != rpc.RouteViewRecordsMax {
			return nil, true, fmt.Errorf("candidate request lost session/frontier/bound: %+v", input)
		}
		if page == 0 && input.Snapshot != "" {
			return nil, true, fmt.Errorf("new scan reused a pin")
		}
		if page == 1 && input.Snapshot != "candidate-pin" {
			return nil, true, fmt.Errorf("continued scan lost its pin")
		}
		result := rpc.RouteViewCandidatesOutput{Session: input.Session, Epoch: "daemon", Snapshot: "candidate-pin", Cursor: uint64(page + 1), Complete: page == 1, Entries: []json.RawMessage{rows[page]}}
		page++
		return result, true, nil
	})
	feed.pages <- runtimeSnapshot(1, true)
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	answer, err := client.Candidates(ctx, nil)
	if err != nil {
		t.Fatalf("read authoritative candidate pages: %v", err)
	}
	got, ok := answer.([]json.RawMessage)
	if !ok || len(got) != len(rows) {
		t.Fatalf("candidate rows lost their raw value shape: %T %+v", answer, answer)
	}
	for index := range rows {
		if !bytes.Equal(got[index], rows[index]) {
			t.Fatalf("candidate row changed numeric values: %s", got[index])
		}
	}
}

// A distance change must consume its acknowledged frontier, then apply FIB
// permissions before exposing any transient forwarding result to the daemon.
func TestRouteViewRuntimeConfigurationAcknowledgesFinalPermissions(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	prefix := netip.MustParsePrefix("198.18.222.0/24")
	changing := false
	var feed *routeViewRuntimeFeed
	feed = runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		switch request.Method {
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			if input.Operation == 1 {
				return nil, false, nil
			}
			found := false
			for _, row := range input.Distances {
				if row.Protocol == "static" && row.Distance == 99 {
					found = true
				}
			}
			if !found {
				return nil, true, fmt.Errorf("desired static distance was not declared")
			}
			changing = true
			feed.pages <- rpc.RouteViewReadOutput{Session: input.Session, Epoch: "daemon", Cursor: 3, Phase: rpc.RouteViewPhaseDelta, Complete: true,
				Entries: []rpc.RouteViewEntry{runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 99, RouteType: uint8(routetype.Blackhole)})}}
			return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation, Cursor: 3}, true, nil
		case rpc.MethodRouteViewPublish:
			if changing {
				var input rpc.RouteViewPublishInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					return nil, true, err
				}
				var batch outgoingBatch
				if err := json.Unmarshal(input.Batch, &batch); err != nil {
					return nil, true, err
				}
				for _, change := range batch.Changes {
					if change.Prefix == prefix && change.Action != routeaction.Withdraw {
						return nil, true, fmt.Errorf("configuration exposed a transient install under old permissions")
					}
				}
			}
		}
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(2, true, runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 1, RouteType: uint8(routetype.Blackhole)}))
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if control := feed.awaitChange(t, ctx, prefix, false); control.Action != routeaction.Add {
		t.Fatalf("initial permitted control was not installed: %+v", control)
	}
	distances := maps.Clone(client.configuration.distances)
	permit := maps.Clone(client.configuration.permitted)
	distances["static"], permit["static"] = 99, false
	if err := client.ApplyConfiguration(ctx, distances, permit); err != nil {
		t.Fatalf("apply authoritative distances and final permission: %v", err)
	}
	if change := feed.awaitChange(t, ctx, prefix, false); change.Action != routeaction.Withdraw {
		t.Fatalf("configuration did not acknowledge final withheld state: %+v", change)
	}
}

// The selected exporter must retain each selected sibling's own metadata and
// own its label storage, rather than reconstructing every hop from the winner.
func TestRouteViewSelectedExportOwnsMemberMetadata(t *testing.T) {
	source := redistevents.RegisterProtocol("static")
	firstSID, secondSID := netip.MustParseAddr("2001:db8:220::1"), netip.MustParseAddr("2001:db8:221::1")
	group := locrib.PathGroup{Best: 0, Paths: []locrib.Path{
		{Source: source, Instance: 1, Interface: "primary", AdminDistance: 10, Labels: []uint32{101}, SRv6SID: firstSID},
		{Source: source, Instance: 2, Interface: "member", AdminDistance: 10, Labels: []uint32{202}, SRv6SID: secondSID, Weight: 3},
	}}
	var entry rpc.RouteViewEntry
	if err := routeViewCopyEntry(family.IPv4Unicast, netip.MustParsePrefix("198.18.223.0/24"), &group, &entry); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(entry.Path.Labels, []uint32{101}) || entry.Path.SRv6SID != firstSID {
		t.Fatalf("selected primary control lost metadata: %+v", entry.Path)
	}
	if len(entry.Path.ECMP) != 1 || entry.Path.ECMP[0].Interface != "member" || entry.Path.ECMP[0].Weight != 3 {
		t.Fatalf("selected sibling target control was not exported: %+v", entry.Path.ECMP)
	}
	member := &entry.Path.ECMP[0]
	if !slices.Equal(member.Labels, []uint32{202}) || member.SRv6SID != secondSID {
		t.Fatalf("selected member metadata was replaced or omitted: %+v", member)
	}
	group.Paths[1].Labels[0] = 303
	if !slices.Equal(member.Labels, []uint32{202}) {
		t.Fatal("selected member export retained authority label storage")
	}
}

// Primary reachability loss must promote the surviving selected member's
// complete labels and SID while retaining the existing prefix-level FIB shape.
func TestRouteViewRuntimePromotesSelectedMemberMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	feed := newRouteViewRuntimeFeed(t, ctx)
	prefix, cover := netip.MustParsePrefix("198.18.224.0/24"), netip.MustParsePrefix("192.0.2.0/24")
	firstSID, secondSID := netip.MustParseAddr("2001:db8:220::1"), netip.MustParseAddr("2001:db8:221::1")
	ipv6Cover := func(prefix string) rpc.RouteViewEntry {
		entry := runtimeSelected(netip.MustParsePrefix(prefix), rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "sid-port"})
		entry.AFI = uint16(family.AFIIPv6)
		return entry
	}
	feed.pages <- runtimeSnapshot(5, true,
		runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, NextHop: netip.MustParseAddr("192.0.2.9"), Labels: []uint32{101}, SRv6SID: firstSID,
			ECMP: []rpc.RouteViewNextHop{{Interface: "member", Weight: 3, Labels: []uint32{202}, SRv6SID: secondSID}}}),
		runtimeSelected(cover, rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "primary"}),
		ipv6Cover("2001:db8:220::/64"), ipv6Cover("2001:db8:221::/64"))
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	control := feed.awaitChange(t, ctx, prefix, false)
	if control.Interface != "primary" || !slices.Equal(control.Labels, []uint32{101}) || control.SRv6SID != firstSID {
		t.Fatalf("primary metadata control was not installed: %+v", control)
	}
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 6, Phase: rpc.RouteViewPhaseDelta, Complete: true,
		Entries: []rpc.RouteViewEntry{{AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast), Prefix: cover, Action: rpc.RouteViewActionWithdraw}}}
	promoted := feed.awaitChange(t, ctx, prefix, false)
	if promoted.Interface != "member" || promoted.Weight != 3 || !slices.Equal(promoted.Labels, []uint32{202}) || promoted.SRv6SID != secondSID {
		t.Fatalf("promotion lost the selected member's own metadata: %+v", promoted)
	}
}

func runtimeFeedOverride(t *testing.T, ctx context.Context, override func(*rpc.Request) (any, bool, error)) *routeViewRuntimeFeed {
	t.Helper()
	feed := &routeViewRuntimeFeed{ctx: ctx, pages: make(chan rpc.RouteViewReadOutput, 8), published: make(chan outgoingBatch, 64), firstRead: make(chan struct{})}
	feed.plugin = routeViewRuntimePeer(t, ctx, func(request *rpc.Request) (any, error) {
		answer, handled, err := override(request)
		if handled {
			return answer, err
		}
		return feed.answer(request)
	})
	return feed
}
