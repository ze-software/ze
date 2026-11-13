// Design: docs/architecture/rib/unified-locrib.md -- reset fences prior output.
// Related: route_view_worker_output.go -- publication after an admitted read.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// A valid delta can wait in transport while the daemon resets that same lease.
// Refusing its later publication must lead to explicit Reset and current-state
// reconstruction in the same worker, never delivery of that obsolete delta.
func TestRouteViewRuntimeFencedPublicationReadsReset(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	h := newPublicationResetHarness(t, ctx, "reset")
	client := h.start(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			closeRuntimeFeedClient(t, ctx, client)
		}
	})
	h.await(t, client, h.oldPrefix, 1)
	worker := client.worker
	h.arm.Store(true)
	h.rib.Insert(family.IPv4Unicast, h.oldPrefix, h.path(2))
	h.awaitInterleaving(t, client)
	h.await(t, client, h.freshPrefix, 3)
	if client.worker != worker {
		t.Fatal("a new worker replaced the current worker instead of reconstructing its reset lease")
	}
	if client.Failure() != nil {
		t.Fatalf("recoverable fenced lease retained a fatal failure: %v", client.Failure())
	}
	if h.refused.Load() != 1 {
		t.Fatalf("obsolete admitted delta was not refused exactly once: %d", h.refused.Load())
	}
	if h.resetReads.Load() != 1 {
		t.Fatalf("recovery did not consume the existing lease's explicit Reset: %d", h.resetReads.Load())
	}
	if h.opens.Load() != 2 {
		t.Fatalf("same authenticated owner opened %d leases, want initial and reconstructed", h.opens.Load())
	}
	h.mu.Lock()
	_, stale := h.current[h.oldPrefix]
	fresh, present := h.current[h.freshPrefix]
	oldWithdrawals := h.withdrawals[h.oldPrefix]
	h.mu.Unlock()
	if stale {
		t.Fatal("reset reinstalled the obsolete route absent from current authority")
	}
	if !present {
		t.Fatal("current authority's useful replacement route was not installed")
	}
	if fresh.Metric != 3 {
		t.Fatalf("reconstructed current route has metric %d, want 3", fresh.Metric)
	}
	if oldWithdrawals != 1 {
		t.Fatalf("fence/close withdrew old output %d times, want once", oldWithdrawals)
	}
	if err := client.Close(ctx); err != nil {
		t.Fatal(err)
	}
	closed = true
	h.mu.Lock()
	remaining := len(h.current)
	freshWithdrawals := h.withdrawals[h.freshPrefix]
	h.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("closing reconstructed lease abandoned %d output entries", remaining)
	}
	if freshWithdrawals != 1 {
		t.Fatalf("closing reconstructed lease withdrew its current output %d times", freshWithdrawals)
	}
}

// Output delivery failure can also fence a lease, while an authorization or
// replacement refusal grants no right to reopen. None may become automatic
// recovery merely because an explicit reset probe is available.
func TestRouteViewRuntimePublicationFailureCannotStealReplacement(t *testing.T) {
	for _, mode := range []string{"delivery", "foreign-owner", "replacement"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			h := newPublicationResetHarness(t, ctx, mode)
			client := h.start(t)
			t.Cleanup(func() { closeRuntimeFeedClient(t, ctx, client) })
			h.await(t, client, h.oldPrefix, 1)
			h.arm.Store(true)
			h.rib.Insert(family.IPv4Unicast, h.oldPrefix, h.path(2))
			h.awaitInterleaving(t, client)
			select {
			case <-client.worker.done:
			case <-ctx.Done():
				t.Fatalf("%s publication failure did not end the old worker", mode)
			}
			if client.Failure() == nil {
				t.Fatalf("%s publication failure became a successful stop", mode)
			}
			if h.opens.Load() != 1 {
				t.Fatalf("%s refusal attempted %d consumer opens, want only its original lease", mode, h.opens.Load())
			}
			if h.refused.Load() != 1 {
				t.Fatalf("%s fixture never reached its actual publication refusal: %d", mode, h.refused.Load())
			}
			// Restore the controlled fault and admit useful output through a new
			// authenticated generation, independently of the failed consumer.
			h.bus.fail.Store(false)
			if h.replacement == nil {
				if err := h.admitReplacement(); err != nil {
					t.Fatal(err)
				}
			}
			if err := h.authority.cleanup(h.owner); err != nil {
				t.Fatal(err)
			}
			h.mu.Lock()
			fresh, present := h.current[h.freshPrefix]
			withdrawals := h.withdrawals[h.freshPrefix]
			h.mu.Unlock()
			if !present {
				t.Fatal("failed predecessor or its cleanup removed healthy replacement output")
			}
			if fresh.Metric != 3 {
				t.Fatalf("healthy replacement metric %d, want 3", fresh.Metric)
			}
			if withdrawals != 0 {
				t.Fatalf("old cleanup withdrew replacement output %d times", withdrawals)
			}
		})
	}
}

type publicationResetBus struct {
	*testEventBus
	fail atomic.Bool
}

func (bus *publicationResetBus) Emit(namespace, event string, payload any) (int, error) {
	count, err := bus.testEventBus.Emit(namespace, event, payload)
	if err != nil {
		return count, err
	}
	if !bus.fail.Load() {
		return count, nil
	}
	batch, valid := payload.(*sysribevents.BestChangeBatch)
	if !valid {
		return count, errors.New("controlled publication bus received a noncanonical batch")
	}
	for i := range batch.Changes {
		if batch.Changes[i].Action != routeaction.Withdraw {
			return count, errors.New("controlled output delivery failure after publication")
		}
	}
	return count, nil
}

// The socket mock maps every request to one real authority and exact Process.
// Only scheduling and the explicit output fault are controlled here. It uses
// no alternate selector and drives the unchanged runtime worker and publisher.
type publicationResetHarness struct {
	ctx          context.Context
	mode         string
	authority    routeViewAuthority
	owner        *process.Process
	foreign      *process.Process
	replacement  *process.Process
	rib          *locrib.RIB
	bus          *publicationResetBus
	plugin       *sdk.Plugin
	oldPrefix    netip.Prefix
	freshPrefix  netip.Prefix
	source       redistevents.ProtocolID
	arm          atomic.Bool
	opens        atomic.Int32
	refused      atomic.Int32
	resetReads   atomic.Int32
	interleaved  bool
	resetSession string
	interleaving chan struct{}
	changes      chan sysribevents.BestChangeEntry
	mu           sync.Mutex
	current      map[netip.Prefix]sysribevents.BestChangeEntry
	withdrawals  map[netip.Prefix]int
}

func newPublicationResetHarness(t *testing.T, ctx context.Context, mode string) *publicationResetHarness {
	t.Helper()
	h := &publicationResetHarness{
		ctx: ctx, mode: mode, owner: routeViewOwner(),
		foreign: process.NewProcess(plugin.PluginConfig{Name: "foreign", Run: "static", Internal: true}),
		rib:     locrib.NewRIB(), bus: &publicationResetBus{testEventBus: newTestEventBus()},
		oldPrefix: netip.MustParsePrefix("198.18.249.0/24"), freshPrefix: netip.MustParsePrefix("198.18.250.0/24"),
		source: redistevents.RegisterProtocol("static"), interleaving: make(chan struct{}),
		changes: make(chan sysribevents.BestChangeEntry, 64),
		current: make(map[netip.Prefix]sysribevents.BestChangeEntry), withdrawals: make(map[netip.Prefix]int),
	}
	sysribevents.BestChange.Subscribe(h.bus, func(batch *sysribevents.BestChangeBatch) {
		for i := range batch.Changes {
			entry := &batch.Changes[i]
			h.mu.Lock()
			if entry.Action == routeaction.Withdraw {
				delete(h.current, entry.Prefix)
				h.withdrawals[entry.Prefix]++
			} else {
				h.current[entry.Prefix] = *entry
			}
			h.mu.Unlock()
			select {
			case h.changes <- *entry:
			case <-ctx.Done():
			}
		}
	})
	h.rib.Insert(family.IPv4Unicast, h.oldPrefix, h.path(1))
	h.plugin = routeViewRuntimePeer(t, ctx, h.answer)
	t.Cleanup(func() {
		if err := h.authority.cleanup(h.owner); err != nil {
			t.Error(err)
		}
		if h.replacement != nil {
			if err := h.authority.cleanup(h.replacement); err != nil {
				t.Error(err)
			}
		}
	})
	defaults, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { publishDistances(defaults) })
	return h
}

func (h *publicationResetHarness) path(metric uint32) locrib.Path {
	return locrib.Path{Source: h.source, Metric: metric, RouteType: routetype.Blackhole}
}

func (h *publicationResetHarness) start(t *testing.T) *routeViewClient {
	t.Helper()
	distances, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	permit, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	client, err := openRouteViewClient(h.ctx, h.plugin, newSysRIB(), distances, permit)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(h.ctx); err != nil {
		t.Fatal(err)
	}
	return client
}

func (h *publicationResetHarness) answer(request *rpc.Request) (any, error) {
	caller := h.owner
	if h.interleaved {
		if h.mode == "foreign-owner" {
			caller = h.foreign
		}
	}
	switch request.Method {
	case rpc.MethodRouteViewOpen:
		h.opens.Add(1)
		var input rpc.RouteViewOpenInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		return h.authority.open(h.ctx, caller, h.rib, input)
	case rpc.MethodRouteViewDistances:
		var input rpc.RouteViewDistancesInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		return h.authority.applyDistances(h.ctx, caller, input)
	case rpc.MethodRouteViewRead:
		var input rpc.RouteViewReadInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		page, err := h.authority.read(h.ctx, caller, input)
		if err != nil {
			return nil, err
		}
		if page.Phase == rpc.RouteViewPhaseReset {
			if input.Session != h.resetSession {
				return nil, errors.New("reset probe did not retain the fenced lease's session")
			}
			h.resetReads.Add(1)
		}
		if h.arm.Load() {
			for i := range page.Entries {
				entry := &page.Entries[i]
				if entry.Prefix != h.oldPrefix {
					continue
				}
				if entry.Path.Metric != 2 {
					continue
				}
				h.arm.Store(false)
				if err := h.interleave(input.Session); err != nil {
					return nil, err
				}
				close(h.interleaving)
				break
			}
		}
		return page, nil
	case rpc.MethodRouteViewPublish:
		var input rpc.RouteViewPublishInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		batch, err := routeViewDecodeOutputBatch(input.Batch)
		if err != nil {
			return nil, err
		}
		ack, err := h.authority.publish(h.ctx, caller, routeViewPublication{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation, Batch: batch}, h.bus)
		if err != nil {
			h.refused.Add(1)
		}
		return ack, err
	case rpc.MethodRouteViewClose:
		var input rpc.RouteViewCloseInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		return h.authority.close(caller, input.Session)
	default:
		return nil, fmt.Errorf("unexpected publication-reset fixture method %s", request.Method)
	}
}

func (h *publicationResetHarness) interleave(session string) error {
	h.interleaved = true
	h.resetSession = session
	switch h.mode {
	case "reset":
		h.rib.Remove(family.IPv4Unicast, h.oldPrefix, h.source, 0)
		h.rib.Insert(family.IPv4Unicast, h.freshPrefix, h.path(3))
		return h.authority.withSession(h.ctx, h.owner, session, func(active *routeViewSession) error {
			return active.invalidate(active.restartRefusal("Selected routing journal exceeded 4096 records", "The admitted reply predates this controlled reset"))
		})
	case "delivery":
		h.bus.fail.Store(true)
		return nil
	case "foreign-owner":
		return nil
	case "replacement":
		return h.admitReplacement()
	default:
		return fmt.Errorf("unknown publication-reset fixture mode %q", h.mode)
	}
}

func (h *publicationResetHarness) admitReplacement() error {
	if err := h.authority.cleanup(h.owner); err != nil {
		return err
	}
	h.rib.Remove(family.IPv4Unicast, h.oldPrefix, h.source, 0)
	h.rib.Insert(family.IPv4Unicast, h.freshPrefix, h.path(3))
	h.replacement = routeViewOwner()
	opened, err := h.authority.open(h.ctx, h.replacement, h.rib, rpc.RouteViewOpenInput{})
	if err != nil {
		return err
	}
	page, err := h.authority.read(h.ctx, h.replacement, rpc.RouteViewReadInput{Session: opened.Session, Limit: rpc.RouteViewRecordsMax})
	if err != nil {
		return err
	}
	if !page.Complete {
		return errors.New("replacement positive control did not complete its selected snapshot")
	}
	_, err = h.authority.publish(h.ctx, h.replacement, routeViewPublication{
		Session: opened.Session, Cursor: page.Cursor, Operation: 1,
		Batch: sysribevents.BestChangeBatch{Family: family.IPv4Unicast, Changes: []sysribevents.BestChangeEntry{{
			Action: routeaction.Add, Prefix: h.freshPrefix, Protocol: "static", RouteType: routetype.Blackhole, Metric: 3,
		}}},
	}, h.bus)
	return err
}

func (h *publicationResetHarness) awaitInterleaving(t *testing.T, client *routeViewClient) {
	t.Helper()
	select {
	case <-h.interleaving:
	case <-client.worker.done:
		t.Fatalf("worker ended before the actual admitted-read interleaving: %v", client.Failure())
	case <-h.ctx.Done():
		t.Fatal("actual authority never returned the changed valid delta before the controlled fence")
	}
}

// await waits for an install of prefix with metric; every caller awaits an add.
func (h *publicationResetHarness) await(t *testing.T, client *routeViewClient, prefix netip.Prefix, metric uint32) {
	t.Helper()
	const action = routeaction.Add
	for {
		select {
		case entry := <-h.changes:
			if entry.Prefix != prefix {
				continue
			}
			if entry.Action != action {
				continue
			}
			if entry.Metric == metric {
				return
			}
		case <-client.worker.done:
			t.Fatalf("worker ended before useful %s %s metric %d: %v", action, prefix, metric, client.Failure())
		case <-h.ctx.Done():
			t.Fatalf("useful %s %s metric %d was not published", action, prefix, metric)
		}
	}
}
