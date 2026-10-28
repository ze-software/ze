// Design: docs/architecture/api/process-protocol.md -- source-policy receipts.
package ipc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// An acknowledged callback authorizes the caller only for the exact captured
// identity, including fields whose legitimate value is zero.
func TestSourcePolicyExactReceipt(t *testing.T) {
	for _, field := range []string{"transaction-id", "operation", "source", "next-policy-epoch", "status", "peer", "peer-owner", "session-generation", "policy-epoch", "receive-frontier", "template-generation", "success"} {
		for _, fault := range []string{"missing", "null", "foreign"} {
			t.Run(field+"/"+fault, func(t *testing.T) {
				engine, plugin := newMuxEnginePluginConn(t)
				ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer cancel()
				input := rpc.SourcePolicyInput{
					TransactionID: "original", Operation: rpc.SourcePolicyPrepare,
					Source:     rpc.SourcePolicyIdentity{Peer: "source", PeerOwner: 1, SessionGeneration: 2, PolicyEpoch: 3},
					DeadlineMS: time.Now().Add(time.Second).UnixMilli(),
				}
				done := make(chan error, 1)
				go func() { done <- engine.SendSourcePolicy(ctx, input) }()
				req, err := plugin.ReadRequest(ctx)
				if err != nil {
					t.Fatal(err)
				}
				out := map[string]any{
					"transaction-id": "original", "operation": "prepare", "next-policy-epoch": 0, "status": "ok",
					"source": map[string]any{"peer": "source", "peer-owner": 1, "session-generation": 2, "policy-epoch": 3, "receive-frontier": 0, "template-generation": 0},
				}
				object := out
				switch field {
				case "peer", "peer-owner", "session-generation", "policy-epoch", "receive-frontier", "template-generation":
					object = out["source"].(map[string]any)
				}
				if field != "success" {
					switch fault {
					case "missing":
						delete(object, field)
					case "null":
						object[field] = nil
					case "foreign":
						switch field {
						case "transaction-id", "peer":
							object[field] = "successor"
						case "operation":
							object[field] = "finalize"
						case "status":
							object[field] = "error"
						case "source":
							object[field] = map[string]any{}
						default:
							object[field] = 99
						}
					}
				}
				encoded, err := json.Marshal(out)
				if err != nil {
					t.Fatal(err)
				}
				if err := plugin.SendResult(ctx, req.ID, json.RawMessage(encoded)); err != nil {
					t.Fatal(err)
				}
				err = <-done
				if field == "success" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil {
					t.Fatal("foreign or incomplete receipt authorized source-policy completion")
				}
			})
		}
	}
}
