// Design: docs/architecture/api/process-protocol.md -- a daemon stop closes a forked plugin's connection without bye.
// Related: route_view_worker.go -- the worker reads its engine session's end as a normal stop.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Goal: an ordinary daemon stop of the forked system RIB is not a failure.
// The daemon closes the forked plugin's connection without `bye`, and the
// owner context ends only after SDK Run returns, so the worker's long-poll
// read sees the close first. Method: a forked (mux, no bridge) consumer
// acknowledges one useful publication, then its next read is held unanswered
// by the engine, which proves the worker is inside RouteViewRead. The engine
// closes the connection while the owner context stays live, exactly the
// production order. The worker must join with no retained failure and report
// the explicit stopped state.
func TestRouteViewForkedEngineCloseDuringReadIsNormalStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	feed := newRouteViewForkedFeed()
	feed.holdRead = true
	h := newRouteViewForkedEngine(t, ctx, feed.answer)
	client := h.startClient(t)
	h.awaitUseful(t, feed, client)
	h.awaitHeld(t, feed, client, rpc.MethodRouteViewRead)
	h.closeEngine(t)
	h.awaitStopped(t, client)
	assertRouteViewForkedNormalStop(t, ctx, client)
}

// Goal: an engine close that meets the forked publish path is a normal stop
// too. The publish fails with the mux reader's EOF, and the publication-reset
// probe read on the same dead connection fails the same way. Method: one
// useful publication is acknowledged, a delta page brings a second route, and
// the engine holds that publication unanswered, which proves the worker is
// inside RouteViewPublish, then closes the connection with the owner context
// live.
func TestRouteViewForkedEngineCloseDuringPublishIsNormalStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	feed := newRouteViewForkedFeed()
	feed.holdPublish = true
	h := newRouteViewForkedEngine(t, ctx, feed.answer)
	client := h.startClient(t)
	h.awaitUseful(t, feed, client)
	h.awaitHeld(t, feed, client, rpc.MethodRouteViewPublish)
	h.closeEngine(t)
	h.awaitStopped(t, client)
	assertRouteViewForkedNormalStop(t, ctx, client)
}

// Goal: a live owner's refusal of the long-poll read stays a retained failure
// however its text reads. Text that names EOF or a closed connection is not a
// transport end: the refusal arrives as an RPCCallError with no wrapped
// cause. Method: after one useful acknowledged publication the engine answers
// the next read with an error line and keeps its connection open.
func TestRouteViewForkedLiveReadRefusalNamingTransportRemainsFatal(t *testing.T) {
	for _, text := range routeViewForkedTransportTexts() {
		t.Run(text, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			feed := newRouteViewForkedFeed()
			feed.readFault = errors.New(text)
			h := newRouteViewForkedEngine(t, ctx, feed.answer)
			client := h.startClient(t)
			h.awaitUseful(t, feed, client)
			h.awaitStopped(t, client)
			assertRouteViewForkedLiveRefusal(t, client, text)
		})
	}
}

// Goal: a live owner's refusal of a forked publication stays a retained
// failure however its text reads. Method: after one useful acknowledged
// publication, the engine refuses the second one with an error line, then
// answers the publication-reset probe with an ordinary delta page, which
// grants no recovery. The connection stays open throughout.
func TestRouteViewForkedLivePublishRefusalNamingTransportRemainsFatal(t *testing.T) {
	for _, text := range routeViewForkedTransportTexts() {
		t.Run(text, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			feed := newRouteViewForkedFeed()
			feed.publishFault = errors.New(text)
			h := newRouteViewForkedEngine(t, ctx, feed.answer)
			client := h.startClient(t)
			h.awaitUseful(t, feed, client)
			h.awaitStopped(t, client)
			if feed.probes != 1 {
				t.Fatalf("refused publication probed its lease %d times, want once", feed.probes)
			}
			assertRouteViewForkedLiveRefusal(t, client, text)
		})
	}
}

// The texts a transport end prints. A live refusal carrying any of them is
// still a refusal.
func routeViewForkedTransportTexts() []string {
	return []string{"EOF", "mux conn read error: EOF", "io: read/write on closed pipe", "use of closed network connection", "mux conn closed", "bridge closed"}
}

func assertRouteViewForkedNormalStop(t *testing.T, ctx context.Context, client *routeViewClient) {
	t.Helper()
	if err := ctx.Err(); err != nil {
		t.Fatalf("worker joined only after its owner context ended: %v", err)
	}
	if err := client.worker.ctx.Err(); err != nil {
		t.Fatalf("fixture canceled the worker before the engine closed: %v", err)
	}
	if err := client.Failure(); err != nil {
		t.Fatalf("ordinary forked engine close became a retained failure while the owner context was live: %v", err)
	}
	findings, err := client.health.findings(routeViewDoctorName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("stopped worker findings=%+v, want its explicit stopped state", findings)
	}
	if findings[0].Severity != "warning" {
		t.Fatalf("normal forked stop retained failed doctor health: %+v", findings)
	}
	if findings[0].Message != "The system routing view is stopped" {
		t.Fatalf("normal forked stop did not report its actual lifecycle state: %+v", findings)
	}
}

func assertRouteViewForkedLiveRefusal(t *testing.T, client *routeViewClient, text string) {
	t.Helper()
	var refusal *rpc.RPCCallError
	if !errors.As(client.Failure(), &refusal) {
		t.Fatalf("live refusal %q was not retained as an RPC refusal: %T %v", text, client.Failure(), client.Failure())
	}
	if refusal.Message != text {
		t.Fatalf("live refusal lost its published cause: %q, want %q", refusal.Message, text)
	}
	findings, err := client.health.findings(routeViewDoctorName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("live refusal findings=%+v, want one actual failed state", findings)
	}
	if findings[0].Severity != "error" {
		t.Fatalf("live refusal %q became normal-stop health: %+v", text, findings[0])
	}
	if findings[0].Code != routeViewDoctorCode {
		t.Fatalf("live refusal doctor code=%q, want %q", findings[0].Code, routeViewDoctorCode)
	}
	if !strings.Contains(findings[0].Message, text) {
		t.Fatalf("failed doctor state omitted the live cause %q: %+v", text, findings[0])
	}
}

// routeViewForkedFeed scripts the engine side of one forked consumer. Its
// answer runs only on the engine's request pump goroutine, so its counters
// need no lock; the test reads probes only after the worker joined.
type routeViewForkedFeed struct {
	first        netip.Prefix
	second       netip.Prefix
	reads        int
	publishes    int
	probes       int
	holdRead     bool
	holdPublish  bool
	readFault    error
	publishFault error
	acknowledged chan netip.Prefix
	held         chan string
}

func newRouteViewForkedFeed() *routeViewForkedFeed {
	// The scripted pages name static, which no import registers when these
	// tests run alone.
	redistevents.RegisterProtocol("static")
	return &routeViewForkedFeed{
		first: netip.MustParsePrefix("198.18.250.0/24"), second: netip.MustParsePrefix("198.18.251.0/24"),
		acknowledged: make(chan netip.Prefix, 8), held: make(chan string, 1),
	}
}

// routeViewForkedAnswer is one scripted engine reply: a result, an error line
// (refusal), or a request the engine holds unanswered.
type routeViewForkedAnswer struct {
	result  any
	refusal error
	held    bool
}

func (feed *routeViewForkedFeed) answer(request *rpc.Request) routeViewForkedAnswer {
	switch request.Method {
	case rpc.MethodRouteViewOpen:
		return routeViewForkedAnswer{result: rpc.RouteViewOpenOutput{Session: "runtime-owner", Epoch: "daemon", Snapshot: "initial"}}
	case rpc.MethodRouteViewDistances:
		var input rpc.RouteViewDistancesInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return routeViewForkedAnswer{refusal: err}
		}
		return routeViewForkedAnswer{result: rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation}}
	case rpc.MethodRouteViewRead:
		return feed.read()
	case rpc.MethodRouteViewPublish:
		return feed.publish(request)
	case rpc.MethodRouteViewClose:
		return routeViewForkedAnswer{result: rpc.RouteViewCloseOutput{Session: "runtime-owner"}}
	default:
		return routeViewForkedAnswer{refusal: fmt.Errorf("unexpected forked engine method %q", request.Method)}
	}
}

func (feed *routeViewForkedFeed) read() routeViewForkedAnswer {
	feed.reads++
	path := rpc.RouteViewPath{Protocol: "static", Distance: 10, RouteType: uint8(routetype.Blackhole)}
	switch feed.reads {
	case 1:
		// A complete snapshot advances the cursor by its records plus one.
		return routeViewForkedAnswer{result: runtimeSnapshot(2, true, runtimeSelected(feed.first, path))}
	case 2:
		if feed.holdRead {
			feed.held <- rpc.MethodRouteViewRead
			return routeViewForkedAnswer{held: true}
		}
		if feed.readFault != nil {
			return routeViewForkedAnswer{refusal: feed.readFault}
		}
		return routeViewForkedAnswer{result: rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 3, Phase: rpc.RouteViewPhaseDelta, Complete: true,
			Entries: []rpc.RouteViewEntry{runtimeSelected(feed.second, path)}}}
	default:
		// Only the refused publication's reset probe reaches a third read.
		feed.probes++
		return routeViewForkedAnswer{result: rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 3, Phase: rpc.RouteViewPhaseDelta, Complete: true}}
	}
}

func (feed *routeViewForkedFeed) publish(request *rpc.Request) routeViewForkedAnswer {
	feed.publishes++
	var input rpc.RouteViewPublishInput
	if err := json.Unmarshal(request.Params, &input); err != nil {
		return routeViewForkedAnswer{refusal: err}
	}
	if feed.publishes > 1 {
		if feed.holdPublish {
			feed.held <- rpc.MethodRouteViewPublish
			return routeViewForkedAnswer{held: true}
		}
		if feed.publishFault != nil {
			return routeViewForkedAnswer{refusal: feed.publishFault}
		}
	}
	var batch outgoingBatch
	if err := json.Unmarshal(input.Batch, &batch); err != nil {
		return routeViewForkedAnswer{refusal: err}
	}
	for i := range batch.Changes {
		feed.acknowledged <- batch.Changes[i].Prefix
	}
	return routeViewForkedAnswer{result: rpc.RouteViewPublishOutput{Session: input.Session, Operation: input.Operation, Cursor: input.Cursor}}
}

// routeViewForkedEngine is the engine end of a forked placement: a net.Pipe
// MuxConn with no DirectBridge, so the consumer reads the selected feed and
// publishes over RPC. One joined pump answers requests in order.
type routeViewForkedEngine struct {
	ctx      context.Context
	plugin   *sdk.Plugin
	engine   *rpc.MuxConn
	pumpDone chan struct{}
}

func newRouteViewForkedEngine(t *testing.T, ctx context.Context, answer func(*rpc.Request) routeViewForkedAnswer) *routeViewForkedEngine {
	t.Helper()
	pluginEnd, engineEnd := net.Pipe()
	h := &routeViewForkedEngine{ctx: ctx, plugin: sdk.NewWithConn("rib", pluginEnd),
		engine: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), pumpDone: make(chan struct{})}
	if h.plugin.IsInternal() {
		t.Fatal("forked stop fixture discovered a DirectBridge")
	}
	go h.pump(answer)
	t.Cleanup(func() {
		_ = h.plugin.Close() //nolint:errcheck // Owned fixture cleanup; the worker's outcome is asserted by the test.
		_ = h.engine.Close() //nolint:errcheck // Owned fixture cleanup; closing twice is safe.
		<-h.pumpDone
	})
	return h
}

func (h *routeViewForkedEngine) pump(answer func(*rpc.Request) routeViewForkedAnswer) {
	defer close(h.pumpDone)
	for {
		select {
		case <-h.ctx.Done():
			return
		case request, open := <-h.engine.Requests():
			if !open {
				return
			}
			reply := answer(request)
			if reply.held {
				continue
			}
			if reply.refusal != nil {
				if err := h.engine.SendError(h.ctx, request.ID, reply.refusal.Error()); err != nil {
					return
				}
				continue
			}
			if err := h.engine.SendResult(h.ctx, request.ID, reply.result); err != nil {
				return
			}
		}
	}
}

func (h *routeViewForkedEngine) startClient(t *testing.T) *routeViewClient {
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
	t.Cleanup(func() { closeRuntimeFeedClient(t, h.ctx, client) })
	if _, selected := client.reader.(routeViewClientSelected); !selected {
		t.Fatalf("forked consumer reader=%T, want the selected feed", client.reader)
	}
	if err := client.Start(h.ctx); err != nil {
		t.Fatal(err)
	}
	return client
}

// awaitUseful waits for the first route's acknowledged publication and checks
// that the consumer reached healthy forwarding before any stop or refusal.
func (h *routeViewForkedEngine) awaitUseful(t *testing.T, feed *routeViewForkedFeed, client *routeViewClient) {
	t.Helper()
	select {
	case prefix := <-feed.acknowledged:
		if prefix != feed.first {
			t.Fatalf("first acknowledged publication=%s, want the useful route %s", prefix, feed.first)
		}
	case <-client.worker.done:
		t.Fatalf("worker stopped before a useful publication: %v", client.Failure())
	case <-h.ctx.Done():
		t.Fatal("useful forked publication was never acknowledged")
	}
}

// awaitHeld waits until the engine holds the named call, so the worker is
// provably inside it, and checks that the acknowledged route made the
// consumer healthy.
func (h *routeViewForkedEngine) awaitHeld(t *testing.T, feed *routeViewForkedFeed, client *routeViewClient, method string) {
	t.Helper()
	select {
	case got := <-feed.held:
		if got != method {
			t.Fatalf("engine held %s, want %s", got, method)
		}
	case <-client.worker.done:
		t.Fatalf("worker stopped before entering %s: %v", method, client.Failure())
	case <-h.ctx.Done():
		t.Fatalf("worker never entered %s", method)
	}
	findings, err := client.health.findings(routeViewDoctorName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("useful acknowledged publication never established healthy forwarding: %+v", findings)
	}
}

func (h *routeViewForkedEngine) closeEngine(t *testing.T) {
	t.Helper()
	if err := h.engine.Close(); err != nil {
		t.Fatal(err)
	}
}

func (h *routeViewForkedEngine) awaitStopped(t *testing.T, client *routeViewClient) {
	t.Helper()
	select {
	case <-client.worker.done:
	case <-h.ctx.Done():
		t.Fatalf("worker did not join: %v", client.Failure())
	}
}
