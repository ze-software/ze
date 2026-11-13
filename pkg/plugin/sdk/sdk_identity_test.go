// Design: docs/architecture/api/process-protocol.md -- authenticated instance identity.
package sdk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestInstanceNameAcknowledgedBeforeConfigure(t *testing.T) {
	p, engine := newTestPair(t)
	if got := p.InstanceName(); got != "" {
		t.Fatalf("unacknowledged identity = %q", got)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	got := make(chan string, 1)
	p.OnConfigure(func(_ []ConfigSection) error {
		got <- p.InstanceName()
		return errors.New("identity fixture complete")
	})
	ended := make(chan error, 1)
	go func() { ended <- p.Run(ctx, &Registration{}) }()
	t.Cleanup(func() {
		cancel()
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-ended:
		case <-time.After(5 * time.Second):
			t.Error("identity runner did not join")
		}
	})
	select {
	case request := <-engine.mux.Requests():
		if request.Method != rpc.MethodDeclareRegistration {
			t.Fatalf("first method = %q", request.Method)
		}
		if err := engine.mux.SendResult(ctx, request.ID, map[string]any{"protocol-version": rpc.ProtocolVersion, "plugin-name": "forwarding-west"}); err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// The intentional configure refusal ends startup after observing identity.
	if _, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{}); err == nil {
		t.Fatal("fixture configure refusal was lost")
	}
	select {
	case name := <-got:
		if name != "forwarding-west" {
			t.Fatalf("configured identity = %q, want engine alias forwarding-west", name)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
