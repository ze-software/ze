// Design: docs/architecture/rib/unified-locrib.md -- selected routes and forwarding output are distinct.
// Related: route_view_worker.go -- the real joined consumer driven by these tests.
// Related: route_view_output.go -- strict publication validation remains authoritative.

package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A valid BGP origination can have no local forwarding target. The worker must
// retain that selected route, publish the useful control, and exclude the
// targetless route from replay without closing its SDK connection.
func TestRouteViewTargetlessInitialSelectionKeepsWorker(t *testing.T) {
	for _, projected := range []bool{false, true} {
		t.Run(targetlessReaderName(projected), func(t *testing.T) {
			initial := targetlessOriginPath()
			h := newTargetlessWorkerProbe(t, projected, &initial)
			h.awaitControl(t, 1, false)
			h.requireTarget(t, false, 0, 0)
			h.requestReplay(t)
			h.awaitControl(t, 1, true)
			h.requireTarget(t, false, 0, 0)
		})
	}
}

// Changing a programmed route into a targetless origination owes one withdrawal,
// not a stale FIB entry or a fatal consumer error. A later usable target owes an
// Add, and a broadcast replay must describe only what remains programmable.
func TestRouteViewTargetlessTransitionLifecycle(t *testing.T) {
	for _, projected := range []bool{false, true} {
		t.Run(targetlessReaderName(projected), func(t *testing.T) {
			usable := targetlessOriginPath()
			usable.Interface = "origin-link"
			h := newTargetlessWorkerProbe(t, projected, &usable)
			h.awaitControl(t, 1, false)
			h.requireTarget(t, true, 1, 0)
			targetless := targetlessOriginPath()
			h.update(t, &targetless, 2)
			h.awaitControl(t, 2, false)
			h.requireTarget(t, false, 1, 1)
			h.requestReplay(t)
			h.awaitControl(t, 2, true)
			h.requireTarget(t, false, 1, 1)
			h.update(t, &targetless, 3)
			h.awaitControl(t, 3, false)
			h.requireTarget(t, false, 1, 1)
			usable.Interface = "replacement-link"
			h.update(t, &usable, 4)
			h.awaitControl(t, 4, false)
			h.requireTarget(t, true, 2, 1)
			h.mu.Lock()
			current := h.installed[h.target]
			h.mu.Unlock()
			if current.Interface != usable.Interface {
				t.Fatalf("recovery lost the new forwarding device: %+v", current)
			}
			if current.Action != routeaction.Add {
				t.Fatalf("recovery replaced an absent route instead of adding it: %+v", current)
			}
			h.requestReplay(t)
			h.awaitControl(t, 4, true)
			h.requireTarget(t, true, 3, 1)
		})
	}
}

// The targetless exception must not hide another malformed output property.
// Both readers carry seventeen individually valid labels through their selected
// representation; the strict output validator rejects that excessive depth.
// A malformed backup is checked even when the primary has no target.
func TestRouteViewTargetlessMalformedOutputStaysFatal(t *testing.T) {
	for _, projected := range []bool{false, true} {
		for _, backup := range []bool{false, true} {
			name := targetlessReaderName(projected) + "/primary-labels"
			if backup {
				name = targetlessReaderName(projected) + "/backup-labels"
			}
			t.Run(name, func(t *testing.T) {
				usable := targetlessOriginPath()
				usable.Interface = "origin-link"
				h := newTargetlessWorkerProbe(t, projected, &usable)
				h.awaitControl(t, 1, false)
				invalid := targetlessOriginPath()
				labels := make([]uint32, 17)
				for i := range labels {
					labels[i] = uint32(100 + i)
				}
				if backup {
					invalid.BackupNextHop = netip.MustParseAddr("192.0.2.254")
					invalid.BackupRepairLabels = labels
				} else {
					invalid.Labels = labels
				}
				h.update(t, &invalid, 2)
				select {
				case <-h.client.worker.done:
					failure := h.client.Failure()
					if failure == nil {
						t.Fatal("malformed output became a successful worker stop")
					}
					if !strings.Contains(failure.Error(), "label stack is invalid") {
						t.Fatalf("target absence masked the malformed labels: %v", failure)
					}
				case <-h.ctx.Done():
					t.Fatal("malformed targetless output did not fail the worker")
				}
				h.mu.Lock()
				_, oldPresent := h.installed[h.target]
				count := h.targetAdds
				h.mu.Unlock()
				if !oldPresent {
					t.Fatal("a refused malformed publication partially mutated the output ledger")
				}
				if count != 1 {
					t.Fatalf("malformed output reached the writer: installs=%d", count)
				}
			})
		}
	}
}

// A targetless selection under an unregistered family is malformed input, not
// an origin-only exception. The local reader can carry numeric family keys, so
// the complete publication validator must still stop this real worker.
func TestRouteViewTargetlessUnknownFamilyStaysFatal(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	unknown := family.Family{AFI: family.AFIIPv4, SAFI: 250}
	if _, registered := family.LookupFamily(unknown.String()); registered {
		t.Fatalf("negative control family is registered: %s", unknown)
	}
	accepted := 0
	client := routeViewNativeRuntimeClient(t, ctx, func(_ context.Context, method string, payload any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, ok := payload.(routeViewPublication)
		if !ok {
			return nil, true, fmt.Errorf("native family probe received %T", payload)
		}
		if err := routeViewValidateOutput(&input.Batch); err != nil {
			return nil, true, err
		}
		accepted++
		return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
	})
	t.Cleanup(func() {
		cancel()
		cleanup, finish := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer finish()
		if err := client.Close(cleanup); err != nil {
			t.Error(err)
		}
	})
	rib := locrib.NewRIB()
	rib.Insert(unknown, netip.MustParsePrefix("198.18.251.0/24"), targetlessOriginPath())
	client.reader = routeViewClientLocal{rib: rib}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.worker.done:
		failure := client.Failure()
		if failure == nil {
			t.Fatal("unregistered family became a successful worker stop")
		}
		if !strings.Contains(failure.Error(), "family is not registered") {
			t.Fatalf("target absence masked the unregistered family: %v", failure)
		}
	case <-ctx.Done():
		t.Fatal("targetless unregistered family did not fail the worker")
	}
	if accepted != 0 {
		t.Fatalf("unregistered family reached the writer: publications=%d", accepted)
	}
}

func targetlessReaderName(projected bool) string {
	if projected {
		return "selected-json"
	}
	return "local-native"
}

func targetlessOriginPath() locrib.Path {
	return locrib.Path{Source: redistevents.RegisterProtocol("bgp"), IsBGP: true}
}

// targetlessWorkerProbe uses the production runtime worker and real selected
// exporter/JSON projection or local capture. Its controlled transport validates
// every publication with the production validator before atomically recording
// it. This proves consumer behavior, not daemon registration or kernel writes.
// Each probe owns one joined worker and the existing helper's joined transport.
type targetlessWorkerProbe struct {
	ctx               context.Context
	client            *routeViewClient
	rib               *locrib.RIB
	feed              *routeViewRuntimeFeed
	target            netip.Prefix
	control           netip.Prefix
	cursor            uint64
	controlSource     redistevents.ProtocolID
	published         chan outgoingBatch
	mu                sync.Mutex
	installed         map[netip.Prefix]outgoingChange
	targetAdds        int
	targetWithdrawals int
}

func newTargetlessWorkerProbe(t *testing.T, projected bool, initial *locrib.Path) *targetlessWorkerProbe {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	h := &targetlessWorkerProbe{ctx: ctx, rib: locrib.NewRIB(),
		target: netip.MustParsePrefix("198.18.249.0/24"), control: netip.MustParsePrefix("198.18.250.0/24"),
		controlSource: redistevents.RegisterProtocol("static"), published: make(chan outgoingBatch, 32),
		installed: make(map[netip.Prefix]outgoingChange)}
	if projected {
		// The peer MUST outlive the later client cleanup's lease close.
		peer, stopPeer := context.WithCancel(context.WithoutCancel(ctx))
		h.feed = runtimeFeedOverride(t, peer, func(request *rpc.Request) (any, bool, error) {
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
			if err := h.accept(batch); err != nil {
				return nil, true, err
			}
			return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
		})
		t.Cleanup(stopPeer)
		h.client = openRuntimeFeedClient(t, ctx, h.feed)
	} else {
		h.client = routeViewNativeRuntimeClient(t, ctx, func(_ context.Context, method string, payload any) (any, bool, error) {
			if method != rpc.MethodRouteViewPublish {
				return nil, false, nil
			}
			input, ok := payload.(routeViewPublication)
			if !ok {
				return nil, true, fmt.Errorf("native targetless probe received %T", payload)
			}
			if err := h.accept(input.Batch); err != nil {
				return nil, true, err
			}
			return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
		})
		h.client.reader = routeViewClientLocal{rib: h.rib}
	}
	t.Cleanup(func() {
		cancel()
		cleanup, finish := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer finish()
		if err := h.client.Close(cleanup); err != nil {
			t.Error(err)
		}
	})
	h.update(t, initial, 1)
	if err := h.client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h *targetlessWorkerProbe) update(t *testing.T, path *locrib.Path, metric uint32) {
	t.Helper()
	h.rib.Insert(family.IPv4Unicast, h.target, *path)
	h.rib.Insert(family.IPv4Unicast, h.control, locrib.Path{Source: h.controlSource, Interface: "control-link", Metric: metric})
	if h.feed == nil {
		return
	}
	entries := make([]rpc.RouteViewEntry, 0, 2)
	for _, prefix := range []netip.Prefix{h.target, h.control} {
		var entry rpc.RouteViewEntry
		var copyErr error
		h.rib.Inspect(family.IPv4Unicast, prefix, func(group locrib.PathGroup) {
			copyErr = routeViewCopyEntry(family.IPv4Unicast, prefix, &group, &entry)
		})
		if copyErr != nil {
			t.Fatal(copyErr)
		}
		entries = append(entries, entry)
	}
	snapshot := h.cursor == 0
	h.cursor += uint64(len(entries))
	if snapshot {
		h.cursor++
	}
	page := runtimeSnapshot(h.cursor, true, entries...)
	if !snapshot {
		page.Phase = rpc.RouteViewPhaseDelta
		page.Snapshot = ""
	}
	select {
	case h.feed.pages <- page:
	case <-h.ctx.Done():
		t.Fatal(h.ctx.Err())
	}
}

func (h *targetlessWorkerProbe) accept(batch outgoingBatch) error {
	for i := range batch.Changes {
		if err := routeViewValidateOutputEntry(batch.Family, &batch.Changes[i]); err != nil {
			return err
		}
	}
	h.mu.Lock()
	for i := range batch.Changes {
		entry := &batch.Changes[i]
		if entry.Action == routeaction.Withdraw {
			delete(h.installed, entry.Prefix)
			if entry.Prefix == h.target {
				h.targetWithdrawals++
			}
			continue
		}
		h.installed[entry.Prefix] = *entry
		if entry.Prefix == h.target {
			h.targetAdds++
		}
	}
	h.mu.Unlock()
	select {
	case h.published <- batch:
		return nil
	case <-h.ctx.Done():
		return h.ctx.Err()
	}
}

func (h *targetlessWorkerProbe) awaitControl(t *testing.T, metric uint32, replayed bool) {
	t.Helper()
	for {
		select {
		case batch := <-h.published:
			if replay.IsReplay(batch.ReplayID) != replayed {
				continue
			}
			for i := range batch.Changes {
				entry := &batch.Changes[i]
				if entry.Prefix != h.control {
					continue
				}
				if entry.Metric == metric {
					return
				}
			}
		case <-h.client.worker.done:
			t.Fatalf("worker stopped before control %d, replay=%t: %v", metric, replayed, h.client.Failure())
		case <-h.ctx.Done():
			t.Fatalf("control %d never reached publication, replay=%t: %v", metric, replayed, h.client.Failure())
		}
	}
}

func (h *targetlessWorkerProbe) requireTarget(t *testing.T, installed bool, adds, withdrawals int) {
	t.Helper()
	key := prefixKey{family: family.IPv4Unicast, prefix: h.target}
	h.client.rib.mu.RLock()
	_, selected := h.client.rib.best[key]
	_, consumerInstalled := h.client.rib.installed[key]
	h.client.rib.mu.RUnlock()
	if !selected {
		t.Fatal("targetless handling dropped the selected origination")
	}
	if consumerInstalled != installed {
		t.Fatalf("consumer installed target=%t, want %t", consumerInstalled, installed)
	}
	h.mu.Lock()
	_, writerInstalled := h.installed[h.target]
	gotAdds, gotWithdrawals := h.targetAdds, h.targetWithdrawals
	h.mu.Unlock()
	if writerInstalled != installed {
		t.Fatalf("writer retained target=%t, want %t", writerInstalled, installed)
	}
	if gotAdds != adds {
		t.Fatalf("target installs=%d, want %d", gotAdds, adds)
	}
	if gotWithdrawals != withdrawals {
		t.Fatalf("target withdrawals=%d, want %d", gotWithdrawals, withdrawals)
	}
	if failure := h.client.Failure(); failure != nil {
		t.Fatalf("targetless handling failed the worker: %v", failure)
	}
}

func (h *targetlessWorkerProbe) requestReplay(t *testing.T) {
	t.Helper()
	payload, err := json.Marshal(replay.Request{ReplayID: replay.Broadcast})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: sysribevents.Namespace, Event: sysribevents.EventReplayRequest, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.client.HandleEvent(string(envelope)); err != nil {
		t.Fatal(err)
	}
}
