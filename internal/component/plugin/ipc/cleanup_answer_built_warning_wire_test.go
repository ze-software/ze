// Design: docs/architecture/api/process-protocol.md -- built warnings on the wire.
package ipc_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Goal: the actual SDK built-value producer reaches the IPC consumer with its
// warning outcome and actual zero-or-one count. Method: complete the real
// five-stage startup, verify the SDK declared no bridge transport, then send
// empty and nonempty built values through its actual wire answer writer and
// PluginConn.SendExecuteCommand's ExecuteCommandValue projection.
func TestCleanupAnswerSDKBuiltWarningWireReachesIPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	engineEnd, pluginEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	engine := ipc.NewMuxPluginConn(mux)
	producer := sdk.NewWithConn("built-warning-wire", pluginEnd)
	producer.OnExecuteCommand(func(_ string, command string, _ []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
		if command == "empty" {
			return rpc.StatusWarning, nil, nil
		}
		return rpc.StatusWarning, map[string]string{"owner": "original"}, nil
	})
	finished := make(chan error, 1)
	go func() { finished <- producer.Run(ctx, &sdk.Registration{}) }()
	t.Cleanup(func() {
		cancel()
		if err := producer.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		select {
		case err := <-finished:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("SDK warning producer did not stop")
		}
	})
	for _, stage := range []struct{ request, callback string }{
		{rpc.MethodDeclareRegistration, "ze-plugin-callback:configure"},
		{"ze-plugin-engine:declare-capabilities", "ze-plugin-callback:share-registry"},
	} {
		request, err := engine.ReadRequest(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if request.Method != stage.request {
			t.Fatalf("startup request=%s want=%s", request.Method, stage.request)
		}
		if err := engine.SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.CallRPC(ctx, stage.callback, struct{}{}); err != nil {
			t.Fatal(err)
		}
	}
	ready, err := engine.ReadRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Method != "ze-plugin-engine:ready" {
		t.Fatalf("startup ready=%s", ready.Method)
	}
	var negotiated rpc.ReadyInput
	if err := json.Unmarshal(ready.Params, &negotiated); err != nil {
		t.Fatal(err)
	}
	// sdk.Run starts ReadyInput with no transport and sets "bridge" only when
	// construction discovered a bridge. Plain net.Pipe must retain the wire.
	if negotiated.Transport != "" {
		t.Fatalf("SDK unexpectedly negotiated transport=%s", negotiated.Transport)
	}
	if err := engine.SendOK(ctx, ready.ID); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		command, data string
		count         uint64
	}{
		{command: "empty"},
		{command: "value", data: `{"owner":"original"}`, count: 1},
	} {
		t.Run(test.command, func(t *testing.T) {
			out, err := engine.SendExecuteCommand(ctx, &rpc.ExecuteCommandInput{Command: test.command})
			if err != nil {
				t.Fatal(err)
			}
			if out.Status != rpc.StatusWarning {
				t.Fatalf("built warning became %s", out.Status)
			}
			if string(out.Data) != test.data {
				t.Fatalf("built warning data=%s want=%s", out.Data, test.data)
			}
			encoded, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			var value struct {
				Metadata *struct {
					Count   uint64 `json:"count"`
					Faults  uint64 `json:"faults"`
					Warning bool   `json:"warning"`
				} `json:"record-answer"`
			}
			if err := json.Unmarshal(encoded, &value); err != nil {
				t.Fatal(err)
			}
			if value.Metadata == nil {
				t.Fatal("actual built warning lost its completed terminator")
			}
			if value.Metadata.Count != test.count {
				t.Fatalf("built warning count=%d want=%d", value.Metadata.Count, test.count)
			}
			if value.Metadata.Faults != 0 {
				t.Fatal("built warning acquired refused rows")
			}
			if !value.Metadata.Warning {
				t.Fatal("actual warning terminator changed")
			}
		})
	}
}
