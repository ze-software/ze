// Design: docs/architecture/api/process-protocol.md -- source-policy validation.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Both real callback paths reject absent/null identities before mutation, while
// explicit zero frontiers reach the handler and a receipt follows its mutation.
func TestSourcePolicyRequiredFields(t *testing.T) {
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			var p *Plugin
			var engine *engineSide
			var bridge *rpc.DirectBridge
			if bridged {
				p, _, bridge = newBridgedTestPair(t)
			} else {
				p, engine = newTestPair(t)
			}
			var mutations atomic.Int64
			p.OnSourcePolicy(func(context.Context, rpc.SourcePolicyInput) error {
				mutations.Add(1)
				return nil
			})
			done := make(chan error, 1)
			if bridged {
				go func() { done <- p.bridgeEventLoop(ctx) }()
			} else {
				go func() { done <- p.Run(ctx, &Registration{}) }()
				completeStartup(t, ctx, engine)
			}
			defer func() { cancel(); <-done }()
			call := func(params any) (json.RawMessage, error) {
				if bridged {
					raw, err := json.Marshal(params)
					if err != nil {
						return nil, err
					}
					return bridge.SendCallback(ctx, callbackSourcePolicy, raw)
				}
				return engine.mux.CallRPC(ctx, callbackSourcePolicy, params)
			}
			request := func() map[string]any {
				return map[string]any{
					"transaction-id": "original", "operation": "prepare", "next-policy-epoch": uint64(0),
					"deadline-ms": time.Now().Add(time.Second).UnixMilli(),
					"source":      map[string]any{"peer": "source", "peer-owner": 1, "session-generation": 2, "policy-epoch": 3, "receive-frontier": 0, "template-generation": 0},
				}
			}
			for _, nested := range []bool{false, true} {
				fields := []string{"transaction-id", "operation", "source", "next-policy-epoch", "deadline-ms"}
				if nested {
					fields = []string{"peer", "peer-owner", "session-generation", "policy-epoch", "receive-frontier", "template-generation"}
				}
				for _, field := range fields {
					for _, null := range []bool{false, true} {
						params := request()
						object := params
						if nested {
							object = params["source"].(map[string]any)
						}
						if null {
							object[field] = nil
						} else {
							delete(object, field)
						}
						if _, err := call(params); err == nil {
							t.Fatalf("accepted missing/null %s, null=%t", field, null)
						}
					}
				}
			}
			for _, operation := range []any{"unknown", "", 1} {
				params := request()
				params["operation"] = operation
				if _, err := call(params); err == nil {
					t.Fatalf("accepted operation %v", operation)
				}
			}
			for _, field := range []string{"transaction-id", "peer", "peer-owner", "session-generation", "policy-epoch", "deadline-ms", "expired", "target"} {
				params := request()
				switch field {
				case "transaction-id":
					params[field] = ""
				case "peer":
					params["source"].(map[string]any)[field] = ""
				case "peer-owner", "session-generation", "policy-epoch":
					params["source"].(map[string]any)[field] = 0
				case "deadline-ms":
					params[field] = 0
				case "expired":
					params["deadline-ms"] = time.Now().Add(-time.Second).UnixMilli()
				case "target":
					params["operation"] = "apply"
					params["next-policy-epoch"] = 3
				}
				if _, err := call(params); err == nil {
					t.Fatalf("accepted invalid %s", field)
				}
			}
			if mutations.Load() != 0 {
				t.Fatal("malformed callback mutated owner")
			}
			raw, err := call(request())
			if err != nil {
				t.Fatal(err)
			}
			var out rpc.SourcePolicyOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatal(err)
			}
			if out.Status != rpc.StatusOK || mutations.Load() != 1 {
				t.Fatalf("valid explicit zeros did not acknowledge mutation: %+v, mutations=%d", out, mutations.Load())
			}
		})
	}
}
