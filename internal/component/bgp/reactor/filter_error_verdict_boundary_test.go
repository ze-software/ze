// Design: docs/plugin-development/protocol.md -- A route the filter cannot judge.
package reactor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// errorVerdictTransport uses the real typed or encoded connection dispatcher
// while the existing declaration fixture supplies the member's on-error.
type errorVerdictTransport struct {
	*rawBoundaryTransport
	conn *plugipc.PluginConn
}

func (e *errorVerdictTransport) CallFilterUpdate(ctx context.Context, _ string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	return e.conn.CallFilter(ctx, route)
}

// serveErrorVerdicts is one test plugin lifecycle. It sends the same verdict
// values the typed evaluator returns, over actual filter-update frames.
func serveErrorVerdicts(ctx context.Context, conn *plugipc.PluginConn, judge func(rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error), done chan<- error) {
	for {
		request, err := conn.ReadRequest(ctx)
		if err != nil {
			if ctx.Err() != nil {
				done <- nil
			} else {
				done <- err
			}
			return
		}
		var batch rpc.FilterUpdateBatchInput
		if err := json.Unmarshal(request.Params, &batch); err != nil {
			done <- err
			return
		}
		out := rpc.FilterUpdateBatchOutput{Verdicts: make([]rpc.FilterUpdateOutput, len(batch.Routes))}
		for i := range batch.Routes {
			verdict, err := judge(batch.Routes[i])
			if err != nil {
				out.Verdicts[i] = rpc.FilterUpdateOutput{Error: err.Error()}
				continue
			}
			out.Verdicts[i] = *verdict
		}
		if err := conn.SendResult(ctx, request.ID, out); err != nil {
			if ctx.Err() != nil {
				done <- nil
			} else {
				done <- err
			}
			return
		}
	}
}

func errorVerdictConnection(t *testing.T, typed bool, judge func(rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error)) *plugipc.PluginConn {
	t.Helper()
	engineEnd, pluginEnd := net.Pipe()
	engine := plugipc.NewPluginConn(engineEnd, engineEnd)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	var bridge *rpc.DirectBridge
	if typed {
		bridge = rpc.NewDirectBridge()
		bridge.SetFilterSlot(func(route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
			input, err := route.Input()
			if err != nil {
				return nil, err
			}
			return judge(input)
		})
		bridge.SetReady()
		engine.SetBridge(bridge)
		done <- nil
	} else {
		peer := plugipc.NewPluginConn(pluginEnd, pluginEnd)
		go serveErrorVerdicts(ctx, peer, judge, done)
	}
	t.Cleanup(func() {
		cancel()
		if bridge != nil {
			bridge.CloseCallbacks()
		}
		engine.Close()    //nolint:errcheck // fixture cleanup closes either transport.
		pluginEnd.Close() //nolint:errcheck // fixture cleanup also closes the unused typed peer.
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("filter reply peer did not stop after its context and connection closed")
		}
	})
	return engine
}

// TestErrorVerdictUsesSameMemberFailureOnBothTransports sends identical
// error-bearing accept, modify, raw and teardown replies through real typed
// and forked dispatch into the policy chain. Error must discard every other
// field, count one failed member and let only on-error decide continuation.
func TestErrorVerdictUsesSameMemberFailureOnBothTransports(t *testing.T) {
	const failure = "filter data is unavailable"
	for _, shape := range []struct {
		name string
		out  rpc.FilterUpdateOutput
	}{
		{"accept", rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Error: failure}},
		{"modify", rpc.FilterUpdateOutput{Action: rpc.FilterModify, Error: failure,
			Sets: []rpc.FilterSet{{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("200")}}}},
		{"raw", rpc.FilterUpdateOutput{Action: rpc.FilterModify, Error: failure, Raw: libraryTestBody(200)}},
		{"teardown", rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Error: failure, Teardown: true, NotifyCode: 6, NotifySubcode: 3}},
	} {
		for _, typed := range []bool{false, true} {
			transportName := "forked"
			if typed {
				transportName = "typed"
			}
			for _, direction := range []string{directionImport, directionExport} {
				for _, onError := range []rpc.OnErrorPolicy{rpc.OnErrorAccept, rpc.OnErrorReject} {
					t.Run(shape.name+"/"+transportName+"/"+direction+"/"+onError.String(), func(t *testing.T) {
						observer := &answerTransport{reads: []string{rpc.FilterFieldRaw, "local-preference"}, sets: []string{"local-preference"}}
						judge := func(input rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
							input.Fields = append(json.RawMessage(nil), input.Fields...)
							input.Raw = nil // Only owned field observations survive the borrowed call.
							observer.mu.Lock()
							observer.sent = append(observer.sent, input)
							observer.mu.Unlock()
							if input.Filter == "failed" {
								out := shape.out
								return &out, nil
							}
							if string(input.Fields) != `{"local-preference":100}` {
								return nil, fmt.Errorf("following member reads %s, want original local-preference", input.Fields)
							}
							return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
						}
						transport := &errorVerdictTransport{rawBoundaryTransport: &rawBoundaryTransport{answerTransport: observer, onError: onError},
							conn: errorVerdictConnection(t, typed, judge)}
						policy := answerPolicy(t, routepolicy.FinalAccept, "failed", "following")
						chain, ok := policy.Library().(*policychain.Chain)
						if !ok {
							t.Fatal("policy has no compiled chain")
						}
						body := libraryTestBody(100)
						r := &Reactor{filterTransportSeam: transport, attrModHandlers: attrModHandlersWithDefaults()}
						got := r.runLibraryChain(chain, testWireUpdate(body), body, policyview.Session{}, libraryIntentContext{},
							direction, "error-verdict", "192.0.2.4", nil)
						wantAction, wantCalls := policychain.OutcomeReject, 1
						if onError == rpc.OnErrorAccept {
							wantAction, wantCalls = policychain.OutcomeAccept, 2
						}
						if got.outcome.Action != wantAction || got.outcome.Failed != (onError == rpc.OnErrorReject) || got.outcome.Teardown || got.failed {
							t.Errorf("outcome=%+v reactor-failed=%v, want action %v and only on-error failure", got.outcome, got.failed, wantAction)
						}
						if got.modifiedPayload != nil || chain.Refused() != 1 || observer.calls() != wantCalls {
							t.Errorf("rewrite=%x refused=%d calls=%d, want no rewrite, one failed member and %d calls", got.modifiedPayload, chain.Refused(), observer.calls(), wantCalls)
						}
					})
				}
			}
		}
	}
}
