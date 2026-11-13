// Design: docs/architecture/api/process-protocol.md -- normal connection stop and live output failures.
// Related: route_view_worker.go -- terminal worker errors retain their actual provenance.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Process.Stop fences dispatch before its callback closure lets SDK Run return.
// The owner's signal context can still be live while the worker observes that
// fence. One acknowledged useful publication precedes the controlled stop.
func TestRouteViewRuntimeNativeDispatchStopJoinsBeforeContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	h := newRouteViewNativeStopHarness(t, ctx, nil)
	defer closeRuntimeFeedClient(t, ctx, h.client)
	barrier := h.afterUsefulAcknowledgment(t)
	defer barrier.release()
	h.local.Insert(family.IPv4Unicast, h.prefix, h.path(2))
	h.bridge.StopDispatch()
	if err := h.client.worker.ctx.Err(); err != nil {
		t.Fatalf("stop fixture canceled the worker before dispatch stopped: %v", err)
	}
	barrier.release()
	h.awaitStopped(t)
	if err := h.client.Failure(); err != nil {
		t.Fatalf("ordinary engine dispatch stop became a retained output failure while the owner context was live: %v", err)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("worker joined only after its owner context ended: %v", err)
	}
	if h.refused.Load() != 0 {
		t.Fatal("closed dispatch entered the live publication evaluator")
	}
	findings, err := h.client.health.findings(routeViewDoctorName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("stopped worker findings=%+v, want its explicit stopped state", findings)
	}
	if findings[0].Severity != "warning" {
		t.Fatalf("normal stop retained failed doctor health: %+v", findings)
	}
	if findings[0].Message != "The system routing view is stopped" {
		t.Fatalf("normal stop did not report its actual lifecycle state: %+v", findings)
	}
}

// The server converts live owner errors to RPCCallError before returning over
// native dispatch, exactly as the wire does. The companion server test drives
// that real adapter; this worker mock consumes its published error contract.
// Every refusal follows a useful acknowledged route with the bridge still live.
func TestRouteViewRuntimeNativeLiveOutputFailuresRemainFatal(t *testing.T) {
	for _, test := range []struct {
		name  string
		cause error
	}{
		{"ordinary", errors.New("controlled live output refusal")},
		{"net-closed", net.ErrClosed},
		{"closed-pipe", io.ErrClosedPipe},
		{"eof", io.EOF},
		{"bridge-closed", rpc.ErrBridgeClosed},
		{"mux-closed", rpc.ErrMuxConnClosed},
		{"bridge-failed", rpc.ErrBridgeFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			ownerCause := fmt.Errorf("live forwarding evaluator: %w", test.cause)
			fault := &rpc.RPCCallError{Message: ownerCause.Error()}
			h := newRouteViewNativeStopHarness(t, ctx, fault)
			defer closeRuntimeFeedClient(t, ctx, h.client)
			barrier := h.afterUsefulAcknowledgment(t)
			defer barrier.release()
			h.local.Insert(family.IPv4Unicast, h.prefix, h.path(2))
			barrier.release()
			h.awaitStopped(t)
			if h.refused.Load() != 1 {
				t.Fatalf("live second publication was refused %d times, want once", h.refused.Load())
			}
			if !h.liveAtRefusal.Load() {
				t.Fatal("output failure control stopped its SDK transport before the evaluator refused")
			}
			var refusal *rpc.RPCCallError
			if !errors.As(h.client.Failure(), &refusal) {
				t.Fatalf("live evaluator failure lost its RPC refusal: %T %v", h.client.Failure(), h.client.Failure())
			}
			if refusal.Message != ownerCause.Error() {
				t.Fatalf("live evaluator failure lost its published cause: %q, want %q", refusal.Message, ownerCause.Error())
			}
			findings, err := h.client.health.findings(routeViewDoctorName)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 {
				t.Fatalf("live failure findings=%+v, want one actual failed state", findings)
			}
			finding := findings[0]
			if finding.Severity != "error" {
				t.Fatalf("live evaluator failure became normal-stop health: %+v", finding)
			}
			if finding.Code != routeViewDoctorCode {
				t.Fatalf("live failure doctor code=%q, want %q", finding.Code, routeViewDoctorCode)
			}
			if !strings.Contains(finding.Message, fault.Error()) {
				t.Fatalf("failed doctor state omitted the actual live cause: %+v", finding)
			}
			if finding.Fix == "" {
				t.Fatal("live output failure omitted its recovery command")
			}
		})
	}
}

type routeViewNativeStopHarness struct {
	ctx           context.Context
	client        *routeViewClient
	bridge        *rpc.DirectBridge
	local         *locrib.RIB
	prefix        netip.Prefix
	source        redistevents.ProtocolID
	acknowledged  chan outgoingChange
	refused       atomic.Int32
	liveAtRefusal atomic.Bool
}

func newRouteViewNativeStopHarness(t *testing.T, ctx context.Context, fault error) *routeViewNativeStopHarness {
	t.Helper()
	local := locrib.Default()
	if local == nil {
		t.Fatal("native stop regression requires the daemon authority")
	}
	h := &routeViewNativeStopHarness{ctx: ctx, local: local, bridge: rpc.NewDirectBridge(),
		prefix: netip.MustParsePrefix("203.0.113.241/32"), source: redistevents.RegisterProtocol("static"),
		acknowledged: make(chan outgoingChange, 4)}
	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn("rib", rpc.NewBridgedConn(pluginEnd, h.bridge))
	t.Cleanup(func() {
		_ = p.Close()         //nolint:errcheck // The worker's retained failure and Close are asserted by the test.
		_ = engineEnd.Close() //nolint:errcheck // Owned test pipe cleanup.
		local.Remove(family.IPv4Unicast, h.prefix, h.source, 0)
	})
	h.bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		switch method {
		case rpc.MethodRouteViewOpen:
			return json.Marshal(rpc.RouteViewOpenOutput{Session: "stop-owner", Epoch: "daemon", Snapshot: "local"})
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			return json.Marshal(rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation})
		case rpc.MethodRouteViewClose:
			return json.Marshal(rpc.RouteViewCloseOutput{Session: "stop-owner"})
		default:
			return nil, fmt.Errorf("native stop regression reached unexpected raw operation %q", method)
		}
	})
	h.bridge.SetDispatchNative(func(_ context.Context, method string, payload any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, valid := payload.(routeViewPublication)
		if !valid {
			return nil, true, fmt.Errorf("native stop publication has invalid type %T", payload)
		}
		for i := range input.Batch.Changes {
			change := &input.Batch.Changes[i]
			if change.Prefix != h.prefix {
				continue
			}
			if change.Metric == 2 {
				if fault != nil {
					h.liveAtRefusal.Store(h.bridge.Ready())
					h.refused.Add(1)
					return nil, true, fault
				}
			}
			select {
			case h.acknowledged <- *change:
			case <-ctx.Done():
				return nil, true, ctx.Err()
			}
		}
		return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
	})
	h.bridge.SetReady()
	local.Insert(family.IPv4Unicast, h.prefix, h.path(1))
	distances, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	permitted, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	h.client, err = openRouteViewClient(ctx, p, newSysRIB(), distances, permitted)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h *routeViewNativeStopHarness) path(metric uint32) locrib.Path {
	return locrib.Path{Source: h.source, RouteType: routetype.Blackhole, Metric: metric}
}

// Enqueuing this ordinary worker task after seeing the first route guarantees
// its publication ACK has returned before the barrier can begin. It holds the
// worker BEFORE the next native dispatch, rather than inside an admitted call.
func (h *routeViewNativeStopHarness) afterUsefulAcknowledgment(t *testing.T) *routeViewStopBarrier {
	t.Helper()
	select {
	case change := <-h.acknowledged:
		if change.Action != routeaction.Add {
			t.Fatalf("useful initial publication action=%v, want Add", change.Action)
		}
		if change.RouteType != routetype.Blackhole {
			t.Fatalf("useful initial publication lost blackhole action: %+v", change)
		}
		if change.Metric != 1 {
			t.Fatalf("first acknowledged publication=%+v, want the useful metric1 route", change)
		}
	case <-h.ctx.Done():
		t.Fatal("useful native forwarding publication was never acknowledged")
	}
	release := make(chan struct{})
	barrier := &routeViewStopBarrier{entered: make(chan struct{}), released: release, release: sync.OnceFunc(func() { close(release) })}
	if _, err := h.client.enqueue(h.ctx, barrier); err != nil {
		t.Fatal(err)
	}
	select {
	case <-barrier.entered:
	case <-h.client.worker.done:
		t.Fatalf("worker stopped before the post-ACK barrier: %v", h.client.Failure())
	case <-h.ctx.Done():
		t.Fatal("worker never reached its post-ACK barrier")
	}
	findings, err := h.client.health.findings(routeViewDoctorName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("useful acknowledged publication never established healthy forwarding: %+v", findings)
	}
	return barrier
}

func (h *routeViewNativeStopHarness) awaitStopped(t *testing.T) {
	t.Helper()
	select {
	case <-h.client.worker.done:
	case <-h.ctx.Done():
		t.Fatalf("worker did not join after its controlled second publication: %v", h.client.Failure())
	}
}

type routeViewStopBarrier struct {
	entered  chan struct{}
	released <-chan struct{}
	release  func()
}

func (barrier *routeViewStopBarrier) perform(worker *routeViewWorker) error {
	close(barrier.entered)
	select {
	case <-barrier.released:
		return nil
	case <-worker.ctx.Done():
		return worker.ctx.Err()
	}
}
