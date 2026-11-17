// Design: docs/plugin-development/protocol.md -- full update-route results.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Missing whole results and real transport failures never become admission.
func TestUpdateRouteResultRejectsMissingOutput(t *testing.T) {
	for _, transport := range []string{"typed", "json-direct", "socket"} {
		t.Run(transport, func(t *testing.T) {
			client, engine := net.Pipe()
			var p *Plugin
			if transport == "socket" {
				p = NewWithConn("result-missing", client)
				conn := rpc.NewConn(engine, engine)
				done := make(chan error, 1)
				go func() {
					request, err := conn.ReadRequest(t.Context())
					if err != nil {
						done <- err
						return
					}
					done <- conn.SendResult(t.Context(), request.ID, json.RawMessage("null"))
				}()
				t.Cleanup(func() {
					if err := <-done; err != nil {
						t.Error(err)
					}
				})
			} else {
				bridge := rpc.NewDirectBridge()
				bridge.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
					return json.RawMessage("null"), nil
				})
				if transport == "typed" {
					bridge.SetUpdateRouteSelResult(func(context.Context, *selector.Selector, string, map[string]any, uint64) (*rpc.UpdateRouteOutput, error) {
						return nil, nil //nolint:nilnil // Deliberate bad-success output: a missing typed result must be refused.
					})
				}
				bridge.SetReady()
				p = NewWithConn("result-missing", rpc.NewBridgedConn(client, bridge))
			}
			t.Cleanup(func() {
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				if err := engine.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := p.UpdateRouteSelResult(t.Context(), selector.All(), "route"); err == nil {
				t.Fatal("missing result accepted")
			}
		})
	}
}

func TestUpdateRouteResultPreservesTransportFailure(t *testing.T) {
	cause := errors.New("origin transport unavailable")
	bridge := rpc.NewDirectBridge()
	bridge.SetUpdateRouteSelResult(func(context.Context, *selector.Selector, string, map[string]any, uint64) (*rpc.UpdateRouteOutput, error) {
		return nil, cause
	})
	bridge.SetReady()
	client, engine := net.Pipe()
	p := NewWithConn("result-failure", rpc.NewBridgedConn(client, bridge))
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := p.UpdateRouteSelResult(t.Context(), selector.All(), "route"); !errors.Is(err, cause) {
		t.Fatalf("transport failure = %v", err)
	}
	if _, _, err := p.UpdateRouteSel(t.Context(), selector.All(), "route"); !errors.Is(err, cause) {
		t.Fatalf("legacy transport failure = %v", err)
	}
}

// Closing a framed connection cannot manufacture a zero-success receipt.
func TestUpdateRouteResultSocketFailureHasNoReceipt(t *testing.T) {
	client, engine := net.Pipe()
	p := NewWithConn("result-disconnect", client)
	conn := rpc.NewConn(engine, engine)
	done := make(chan error, 1)
	go func() {
		if _, err := conn.ReadRequest(t.Context()); err != nil {
			done <- err
			return
		}
		done <- engine.Close()
	}()
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	output, err := p.UpdateRouteResult(t.Context(), "*", "route")
	if err == nil {
		t.Fatal("closed connection became successful admission")
	}
	if output != nil {
		t.Fatalf("closed connection fabricated result: %+v", output)
	}
}
