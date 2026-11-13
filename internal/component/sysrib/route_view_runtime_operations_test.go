// Design: docs/architecture/rib/unified-locrib.md -- serialized runtime operations.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// A reset must reconstruct the current authority and forget output withdrawn by
// the old lease. A newer removal cannot be revived by a stale cached value.
func TestRouteViewRuntimeResetReopensCurrentSelection(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	prefix := netip.MustParsePrefix("198.18.230.0/24")
	var opens atomic.Int32
	reopened := make(chan struct{})
	var feed *routeViewRuntimeFeed
	feed = runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		switch request.Method {
		case rpc.MethodRouteViewOpen:
			if opens.Add(1) == 1 {
				return nil, false, nil
			}
			feed.pages <- rpc.RouteViewReadOutput{Session: "replacement", Epoch: "daemon", Snapshot: "replacement-snapshot", Cursor: 1, Phase: rpc.RouteViewPhaseSnapshot, Complete: true}
			close(reopened)
			return rpc.RouteViewOpenOutput{Session: "replacement", Epoch: "daemon", Snapshot: "replacement-snapshot"}, true, nil
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation}, true, nil
		case rpc.MethodRouteViewClose:
			var input rpc.RouteViewCloseInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			return rpc.RouteViewCloseOutput(input), true, nil
		case rpc.MethodRouteViewCandidates:
			var input rpc.RouteViewCandidatesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			if input.Session != "replacement" {
				return nil, true, fmt.Errorf("command retained the reset session")
			}
			return rpc.RouteViewCandidatesOutput{Session: input.Session, Epoch: "daemon", Snapshot: "current-candidates", Complete: true}, true, nil
		}
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(2, true, runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, RouteType: uint8(routetype.Blackhole)}))
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if control := feed.awaitChange(t, ctx, prefix, false); control.Action != routeaction.Add {
		t.Fatal("initial selected control was not installed")
	}
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 3, Phase: rpc.RouteViewPhaseReset}
	select {
	case <-reopened:
	case <-ctx.Done():
		t.Fatalf("reset never opened a replacement: %v", client.Failure())
	}
	if _, err := client.Candidates(ctx, nil); err != nil {
		t.Fatalf("reset did not reconstruct a current session: %v", err)
	}
	client.rib.mu.RLock()
	_, retained := client.rib.routes[prefixKey{family: family.IPv4Unicast, prefix: prefix}]
	client.rib.mu.RUnlock()
	if retained {
		t.Fatal("reset retained a selected route absent from current authority")
	}
	if opens.Load() != 2 {
		t.Fatalf("reset opened %d sessions, want 2", opens.Load())
	}
}

// Failed application must restore the previous complete declaration under a
// new lease, and acknowledge its output before answering the failed caller.
func TestRouteViewRuntimeConfigurationRestoresFreshSession(t *testing.T) {
	testRouteViewRuntimeCompensation(t, false)
}

// Failed restoration cannot report the old configuration as running. Closing
// the SDK must retain the fatal cause for the plugin owner's exit status.
func TestRouteViewRuntimeConfigurationLostClosesSDK(t *testing.T) {
	testRouteViewRuntimeCompensation(t, true)
}

func testRouteViewRuntimeCompensation(t *testing.T, failRestore bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	prefix := netip.MustParsePrefix("198.18.231.0/24")
	var opens atomic.Int32
	var feed *routeViewRuntimeFeed
	feed = runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		switch request.Method {
		case rpc.MethodRouteViewOpen:
			if opens.Add(1) == 1 {
				return nil, false, nil
			}
			feed.pages <- rpc.RouteViewReadOutput{Session: "restore", Epoch: "daemon", Snapshot: "restore-snapshot", Cursor: 2, Phase: rpc.RouteViewPhaseSnapshot, Complete: true, Entries: []rpc.RouteViewEntry{runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, RouteType: uint8(routetype.Blackhole)})}}
			return rpc.RouteViewOpenOutput{Session: "restore", Epoch: "daemon", Snapshot: "restore-snapshot"}, true, nil
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			if input.Session == "runtime-owner" && input.Operation > 1 {
				return nil, true, errors.New("fixture-forward-apply-refused")
			}
			if input.Session == "restore" {
				if failRestore {
					return nil, true, errors.New("fixture-restoration-refused")
				}
				if input.Operation != 1 {
					return nil, true, fmt.Errorf("restoration reused old session operation")
				}
				for _, row := range input.Distances {
					if row.Protocol == "static" && row.Distance == 99 {
						return nil, true, fmt.Errorf("restoration published desired rather than old distances")
					}
				}
			}
			return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation}, true, nil
		case rpc.MethodRouteViewClose:
			var input rpc.RouteViewCloseInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			return rpc.RouteViewCloseOutput(input), true, nil
		}
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(2, true, runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, RouteType: uint8(routetype.Blackhole)}))
	client := openRuntimeFeedClient(t, ctx, feed)
	if !failRestore {
		defer closeRuntimeFeedClient(t, ctx, client)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	feed.awaitChange(t, ctx, prefix, false)
	distances := maps.Clone(client.configuration.distances)
	distances["static"] = 99
	err := client.ApplyConfiguration(ctx, distances, client.configuration.permitted)
	if err == nil {
		t.Fatal("failed authoritative apply was accepted")
	}
	if opens.Load() != 2 {
		t.Fatalf("failed apply did not reconstruct an independent old session: opens=%d, error=%v", opens.Load(), err)
	}
	if failRestore {
		if !errors.Is(err, sdk.ErrRunningConfigLost) {
			t.Fatalf("restoration failure lost running-config verdict: %v", err)
		}
		if client.Failure() == nil {
			t.Fatal("SDK shutdown lost its fatal cause")
		}
		return
	}
	if errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Fatalf("successful restoration reported running config lost: %v", err)
	}
	if control := feed.awaitChange(t, ctx, prefix, false); control.Action != routeaction.Add {
		t.Fatalf("restored session did not republish its initial control: %+v", control)
	}
	if client.configuration.distances["static"] == 99 {
		t.Fatal("failed configuration became the committed declaration")
	}
}

// A candidate scan pins all four identities independently of selected state.
// Each malformed continuation follows an admitted, nonempty initial page.
func TestRouteViewRuntimeCandidatesRefuseChangedPin(t *testing.T) {
	for _, change := range []string{"session", "epoch", "snapshot", "cursor", "empty-progress"} {
		t.Run(change, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			pages := 0
			feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
				if request.Method != rpc.MethodRouteViewCandidates {
					return nil, false, nil
				}
				pages++
				page := rpc.RouteViewCandidatesOutput{Session: "runtime-owner", Epoch: "daemon", Snapshot: "pin", Cursor: uint64(pages), Entries: []json.RawMessage{json.RawMessage(`{"prefix":"198.18.232.0/24"}`)}}
				if pages == 2 {
					page.Complete = true
					switch change {
					case "session":
						page.Session = "foreign"
					case "epoch":
						page.Epoch = "foreign"
					case "snapshot":
						page.Snapshot = "foreign"
					case "cursor":
						page.Cursor++
					case "empty-progress":
						page.Complete, page.Cursor, page.Entries = false, 1, nil
					}
				}
				return page, true, nil
			})
			feed.pages <- runtimeSnapshot(1, true)
			client := openRuntimeFeedClient(t, ctx, feed)
			defer closeRuntimeFeedClient(t, ctx, client)
			if err := client.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Candidates(ctx, nil); err == nil {
				t.Fatalf("candidate scan accepted changed %s", change)
			}
			if pages != 2 {
				t.Fatalf("malformed continuation was not reached after admitted control: pages=%d", pages)
			}
		})
	}
}

// A reset while consuming an apply barrier must carry the desired declaration
// into the new lease. Only the final permission may reach publication.
func TestRouteViewRuntimeConfigurationResetUsesDesiredDeclaration(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	prefix := netip.MustParsePrefix("198.18.235.0/24")
	var opens atomic.Int32
	var replacementDistances atomic.Int32
	var feed *routeViewRuntimeFeed
	feed = runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		switch request.Method {
		case rpc.MethodRouteViewOpen:
			if opens.Add(1) == 1 {
				return nil, false, nil
			}
			feed.pages <- rpc.RouteViewReadOutput{Session: "desired", Epoch: "daemon", Snapshot: "desired-snapshot", Cursor: 2, Phase: rpc.RouteViewPhaseSnapshot, Complete: true, Entries: []rpc.RouteViewEntry{runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 99, RouteType: uint8(routetype.Blackhole)})}}
			return rpc.RouteViewOpenOutput{Session: "desired", Epoch: "daemon", Snapshot: "desired-snapshot"}, true, nil
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			if input.Session == "runtime-owner" && input.Operation == 1 {
				return nil, false, nil
			}
			found := false
			for _, row := range input.Distances {
				if row.Protocol == "static" && row.Distance == 99 {
					found = true
				}
			}
			if !found {
				return nil, true, errors.New("reset reopened with old committed declaration")
			}
			if input.Session == "desired" {
				replacementDistances.Add(1)
				return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation}, true, nil
			}
			feed.pages <- rpc.RouteViewReadOutput{Session: input.Session, Epoch: "daemon", Cursor: 3, Phase: rpc.RouteViewPhaseReset}
			return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation, Cursor: 3}, true, nil
		case rpc.MethodRouteViewPublish:
			var input rpc.RouteViewPublishInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			if input.Session == "desired" {
				var batch outgoingBatch
				if err := json.Unmarshal(input.Batch, &batch); err != nil {
					return nil, true, err
				}
				for _, entry := range batch.Changes {
					if entry.Action != routeaction.Withdraw {
						return nil, true, errors.New("reset exposed transient desired-selection output under old permission")
					}
				}
			}
		case rpc.MethodRouteViewClose:
			var input rpc.RouteViewCloseInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			return rpc.RouteViewCloseOutput(input), true, nil
		}
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(2, true, runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, RouteType: uint8(routetype.Blackhole)}))
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	feed.awaitChange(t, ctx, prefix, false)
	desired, permitted := maps.Clone(client.configuration.distances), maps.Clone(client.configuration.permitted)
	desired["static"], permitted["static"] = 99, false
	if err := client.ApplyConfiguration(ctx, desired, permitted); err != nil {
		t.Fatalf("apply through reset: %v", err)
	}
	if replacementDistances.Load() != 1 {
		t.Fatal("replacement did not acknowledge the desired table exactly once")
	}
	if got := feed.awaitChange(t, ctx, prefix, false); got.Action != routeaction.Withdraw {
		t.Fatalf("final withheld output was lost across reset: %+v", got)
	}
}

// Caller cancellation after apply admission must still await an independently
// bounded restoration and its fresh-session output before returning.
func TestRouteViewRuntimeCanceledApplyStillRestores(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	caller, cancelCaller := context.WithCancel(ctx)
	defer cancelCaller()
	entered := make(chan struct{})
	var opens atomic.Int32
	var restored atomic.Bool
	var feed *routeViewRuntimeFeed
	feed = runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		switch request.Method {
		case rpc.MethodRouteViewOpen:
			if opens.Add(1) == 1 {
				return nil, false, nil
			}
			feed.pages <- rpc.RouteViewReadOutput{Session: "canceled-restore", Epoch: "daemon", Snapshot: "restored", Cursor: 1, Phase: rpc.RouteViewPhaseSnapshot, Complete: true}
			return rpc.RouteViewOpenOutput{Session: "canceled-restore", Epoch: "daemon", Snapshot: "restored"}, true, nil
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			if input.Operation == 2 {
				close(entered)
				select {
				case <-caller.Done():
					return nil, true, caller.Err()
				case <-ctx.Done():
					return nil, true, ctx.Err()
				}
			}
			if input.Session == "canceled-restore" {
				restored.Store(true)
			}
			return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation}, true, nil
		case rpc.MethodRouteViewClose:
			var input rpc.RouteViewCloseInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			return rpc.RouteViewCloseOutput(input), true, nil
		}
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(1, true)
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		result <- client.ApplyConfiguration(caller, client.configuration.distances, client.configuration.permitted)
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("apply was never admitted")
	}
	cancelCaller()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled forward apply returned %v", err)
		}
		if !restored.Load() {
			t.Fatal("ApplyConfiguration returned before its independent restoration")
		}
	case <-ctx.Done():
		t.Fatal("canceled apply never completed restoration")
	}
}

// A page can change both a dependent and its covering route. A cached earlier
// resolution must not escape before the same page's cascade finds the final
// target, and each prefix must occur once in its acknowledged publication.
func TestRouteViewRuntimeCoalescesSelectedAndCascade(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	prefix, cover := netip.MustParsePrefix("198.18.238.0/24"), netip.MustParsePrefix("192.0.2.0/24")
	feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		if request.Method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		var input rpc.RouteViewPublishInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, true, err
		}
		var batch outgoingBatch
		if err := json.Unmarshal(input.Batch, &batch); err != nil {
			return nil, true, err
		}
		seen := make(map[netip.Prefix]bool)
		for _, entry := range batch.Changes {
			if seen[entry.Prefix] {
				return nil, true, fmt.Errorf("one selected page published intermediate values for %s", entry.Prefix)
			}
			seen[entry.Prefix] = true
			if input.Cursor == 5 && entry.Prefix == prefix && entry.Interface != "final-port" {
				return nil, true, fmt.Errorf("selected update escaped before final cascade resolution: %+v", entry)
			}
		}
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(3, true,
		runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, NextHop: netip.MustParseAddr("192.0.2.9"), Metric: 1}),
		runtimeSelected(cover, rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "initial-port"}))
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if got := feed.awaitChange(t, ctx, prefix, false); got.Interface != "initial-port" {
		t.Fatalf("initial recursive control was not installed: %+v", got)
	}
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 5, Phase: rpc.RouteViewPhaseDelta, Complete: true, Entries: []rpc.RouteViewEntry{
		runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, NextHop: netip.MustParseAddr("192.0.2.9"), Metric: 2}),
		runtimeSelected(cover, rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "final-port"}),
	}}
	if got := feed.awaitChange(t, ctx, prefix, false); got.Interface != "final-port" || got.Metric != 2 {
		t.Fatalf("page did not publish the final dependent value: %+v", got)
	}
}

// An Open result must establish its complete identity before configuration can
// mutate the daemon. This applies to the local reader as well as selected feed.
func TestRouteViewRuntimeOpenRefusesIncompleteIdentity(t *testing.T) {
	for _, missing := range []string{"session", "epoch", "snapshot"} {
		t.Run(missing, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var distances atomic.Int32
			feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
				switch request.Method {
				case rpc.MethodRouteViewOpen:
					opened := rpc.RouteViewOpenOutput{Session: "runtime-owner", Epoch: "daemon", Snapshot: "initial"}
					switch missing {
					case "session":
						opened.Session = ""
					case "epoch":
						opened.Epoch = ""
					case "snapshot":
						opened.Snapshot = ""
					}
					return opened, true, nil
				case rpc.MethodRouteViewDistances:
					distances.Add(1)
				}
				return nil, false, nil
			})
			declared, err := parseAdminDistanceConfig("{}")
			if err != nil {
				t.Fatal(err)
			}
			_, err = openRouteViewClient(ctx, feed.plugin, newSysRIB(), declared, map[string]bool{"static": true})
			if err == nil {
				t.Errorf("Open accepted missing %s before worker initialization", missing)
			}
			if distances.Load() != 0 {
				t.Errorf("invalid %s identity reached authoritative mutation", missing)
			}
		})
	}
}
