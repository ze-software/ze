// Design: docs/architecture/api/process-protocol.md -- routing-view SDK transport.

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The same public calls must cross socket RPC and DirectBridge without losing
// their method or input. Each call receives a real nonzero acknowledgment.
func TestRouteViewSDKUsesBothTransports(t *testing.T) {
	cases := []struct {
		method string
		input  string
		output string
		want   any
		call   func(context.Context, *Plugin) (any, error)
	}{
		{rpc.MethodRouteViewOpen, `{}`, `{"session":"owner","epoch":"engine","snapshot":"initial","cursor":0}`,
			&rpc.RouteViewOpenOutput{Session: "owner", Epoch: "engine", Snapshot: "initial"},
			func(ctx context.Context, p *Plugin) (any, error) {
				return p.RouteViewOpen(ctx, &rpc.RouteViewOpenInput{})
			}},
		{rpc.MethodRouteViewRead, `{"session":"owner","cursor":0,"limit":32,"wait-ms":0}`, `{"session":"owner","epoch":"engine","snapshot":"initial","cursor":1,"phase":"snapshot","complete":true,"entries":[]}`,
			&rpc.RouteViewReadOutput{Session: "owner", Epoch: "engine", Snapshot: "initial", Cursor: 1, Phase: rpc.RouteViewPhaseSnapshot, Complete: true, Entries: []rpc.RouteViewEntry{}},
			func(ctx context.Context, p *Plugin) (any, error) {
				return p.RouteViewRead(ctx, &rpc.RouteViewReadInput{Session: "owner", Limit: 32})
			}},
		{rpc.MethodRouteViewCandidates, `{"session":"owner","cursor":0,"limit":32}`, `{"session":"owner","epoch":"engine","snapshot":"candidates","cursor":1,"complete":true,"entries":[{"prefix":"192.0.2.0/24","paths":[{"tag":0,"hidden":true}]}]}`,
			&rpc.RouteViewCandidatesOutput{Session: "owner", Epoch: "engine", Snapshot: "candidates", Cursor: 1, Complete: true, Entries: []json.RawMessage{json.RawMessage(`{"prefix":"192.0.2.0/24","paths":[{"tag":0,"hidden":true}]}`)}},
			func(ctx context.Context, p *Plugin) (any, error) {
				return p.RouteViewCandidates(ctx, &rpc.RouteViewCandidatesInput{Session: "owner", Limit: 32})
			}},
		{rpc.MethodRouteViewDistances, `{"session":"owner","operation":7,"distances":[{"protocol":"connected","distance":0}]}`, `{"session":"owner","operation":7,"cursor":8}`,
			&rpc.RouteViewDistancesOutput{Session: "owner", Operation: 7, Cursor: 8},
			func(ctx context.Context, p *Plugin) (any, error) {
				return p.RouteViewDistances(ctx, &rpc.RouteViewDistancesInput{Session: "owner", Operation: 7, Distances: []rpc.RouteViewDistance{{Protocol: "connected", Distance: 0}}})
			}},
		{rpc.MethodRouteViewPublish, `{"session":"owner","cursor":8,"operation":9,"batch":{"family":"ipv4/unicast","changes":[]}}`, `{"session":"owner","operation":9,"cursor":8}`,
			&rpc.RouteViewPublishOutput{Session: "owner", Operation: 9, Cursor: 8},
			func(ctx context.Context, p *Plugin) (any, error) {
				return p.RouteViewPublish(ctx, &rpc.RouteViewPublishInput{Session: "owner", Cursor: 8, Operation: 9, Batch: json.RawMessage(`{"family":"ipv4/unicast","changes":[]}`)})
			}},
		{rpc.MethodRouteViewClose, `{"session":"owner"}`, `{"session":"owner"}`,
			&rpc.RouteViewCloseOutput{Session: "owner"},
			func(ctx context.Context, p *Plugin) (any, error) {
				return p.RouteViewClose(ctx, &rpc.RouteViewCloseInput{Session: "owner"})
			}},
	}
	for _, transport := range []string{"socket", "direct"} {
		for _, tc := range cases {
			t.Run(transport+"/"+tc.method, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				var p *Plugin
				var engine *engineSide
				seen := make(chan string, 1)
				if transport == "socket" {
					p, engine = newTestPair(t)
					worker := &routeViewTestPeer{ctx: ctx, engine: engine, method: tc.method, input: tc.input, output: tc.output, seen: seen, done: make(chan struct{})}
					go worker.run()
					t.Cleanup(worker.stop)
				} else {
					var bridge *rpc.DirectBridge
					p, engine, bridge = newBridgedTestPair(t)
					bridge.SetDispatchRPC(func(_ context.Context, method string, input json.RawMessage) (json.RawMessage, error) {
						if method != tc.method || string(input) != tc.input {
							return nil, errors.New("Direct call changed method or input")
						}
						seen <- method
						return json.RawMessage(tc.output), nil
					})
					bridge.SetReady()
				}
				got, err := tc.call(ctx, p)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("decoded reply %+v, want %+v", got, tc.want)
				}
				select {
				case method := <-seen:
					if method != tc.method {
						t.Fatalf("got method %q, want %q", method, tc.method)
					}
				case <-ctx.Done():
					t.Fatal("SDK returned without crossing its selected transport")
				}
			})
		}
	}
}

// routeViewTestPeer owns one request/reply lifecycle. Its context bounds the
// read when the missing SDK operation never sends a request.
type routeViewTestPeer struct {
	ctx    context.Context
	engine *engineSide
	method string
	input  string
	output string
	seen   chan<- string
	done   chan struct{}
}

func (peer *routeViewTestPeer) run() {
	defer close(peer.done)
	select {
	case req := <-peer.engine.mux.Requests():
		if req == nil {
			return
		}
		if req.Method != peer.method || string(req.Params) != peer.input {
			_ = peer.engine.mux.SendError(peer.ctx, req.ID, "socket call changed method or input")
			return
		}
		peer.seen <- req.Method
		_ = peer.engine.mux.SendResult(peer.ctx, req.ID, json.RawMessage(peer.output))
	case <-peer.ctx.Done():
		return
	}
}

func (peer *routeViewTestPeer) stop() {
	// Closing the mux releases its request channel and this responder even
	// when a failing call never reached the socket. Join before cleanup returns.
	_ = peer.engine.mux.Close()
	<-peer.done
}

func TestRouteViewSDKRejectsInvalidResults(t *testing.T) {
	cases := []struct {
		name   string
		output json.RawMessage
	}{
		{"absent", nil},
		{"null", json.RawMessage(`null`)},
		{"malformed", json.RawMessage(`{"session":`)},
		{"empty object", json.RawMessage(`{}`)},
		{"missing cursor", json.RawMessage(`{"session":"owner","epoch":"engine","snapshot":"initial"}`)},
		{"null cursor", json.RawMessage(`{"session":"owner","epoch":"engine","snapshot":"initial","cursor":null}`)},
		{"wrong cursor type", json.RawMessage(`{"session":"owner","epoch":"engine","snapshot":"initial","cursor":"0"}`)},
		{"empty session", json.RawMessage(`{"session":"","epoch":"engine","snapshot":"initial","cursor":0}`)},
		{"empty epoch", json.RawMessage(`{"session":"owner","epoch":"","snapshot":"initial","cursor":0}`)},
		{"empty snapshot", json.RawMessage(`{"session":"owner","epoch":"engine","snapshot":"","cursor":0}`)},
		{"past frame budget", json.RawMessage(strings.Repeat(" ", rpc.RouteViewResultBudget()+1))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, _, bridge := newBridgedTestPair(t)
			bridge.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) { return tc.output, nil })
			bridge.SetReady()
			if result, err := p.RouteViewOpen(context.Background(), &rpc.RouteViewOpenInput{}); err == nil || result != nil {
				t.Fatalf("invalid reply returned result %+v, error %v", result, err)
			}
		})
	}
}

func TestRouteViewSDKRequiresExplicitPageOutcome(t *testing.T) {
	for _, output := range []string{
		`{"session":"owner","epoch":"engine","cursor":0,"complete":false}`,
		`{"session":"owner","epoch":"engine","cursor":0,"phase":null,"complete":false}`,
		`{"session":"owner","epoch":"engine","cursor":0,"phase":"delta"}`,
		`{"session":"owner","epoch":"engine","cursor":0,"phase":"delta","complete":null}`,
		`{"session":"owner","epoch":"engine","cursor":0,"phase":"unknown","complete":false}`,
		`{"session":"owner","epoch":"engine","cursor":0,"phase":"delta","complete":false,"entries":[` + strings.Repeat(`{},`, rpc.RouteViewRecordsMax) + `{}]}`,
	} {
		p, _, bridge := newBridgedTestPair(t)
		bridge.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(output), nil
		})
		bridge.SetReady()
		if result, err := p.RouteViewRead(context.Background(), &rpc.RouteViewReadInput{Session: "owner", Limit: 1}); err == nil || result != nil {
			t.Fatalf("invalid page returned result %+v, error %v", result, err)
		}
	}
}
