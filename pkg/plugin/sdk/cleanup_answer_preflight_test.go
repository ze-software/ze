// Design: docs/architecture/api/process-protocol.md -- preflight before a value walk.
package sdk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: the real typed execute callback refuses a reserved envelope before
// entering its producer. Method: complete SDK startup, call its installed
// bridge slot with the invalid key, then call the same producer with a valid
// key and require exactly one walk and its complete document.
func TestCleanupAnswerTypedBridgePreflightsEnvelope(t *testing.T) {
	p, engine, bridge := newBridgedTestPair(t)
	visits := 0
	p.OnExecuteCommand(func(_ string, key string, _ []string, _ string) (string, any, error) {
		return rpc.StatusDone, Records{Key: key, Rows: func(yield func(Record) bool) {
			visits++
			row := rpc.RawRow(`{"owner":"original"}`)
			yield(Record{Item: &row})
		}}, nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	finished := make(chan error, 1)
	go func() { finished <- p.Run(ctx, &Registration{}) }()
	defer func() {
		cancel()
		select {
		case err := <-finished:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("SDK callback owner did not stop")
		}
	}()
	completeStartup(t, ctx, engine)
	require.Eventually(t, bridge.HasExecuteCommand, 2*time.Second, time.Millisecond)
	_, err := bridge.ExecuteCommand(ctx, "preflight", rpc.AnswerErrorsKey, nil, "*")
	if !errors.Is(err, rpc.ErrReservedEnvelopeKey) {
		t.Fatalf("reserved envelope error=%v", err)
	}
	if visits != 0 {
		t.Fatalf("refused header entered producer %d times", visits)
	}
	out, err := bridge.ExecuteCommand(ctx, "preflight", "owners", nil, "*")
	if err != nil {
		t.Fatal(err)
	}
	if visits != 1 {
		t.Fatalf("healthy header visits=%d want one", visits)
	}
	if out.Status != rpc.StatusDone {
		t.Fatalf("healthy status=%s", out.Status)
	}
	if string(out.Data) != `{"owners":[{"owner":"original"}]}` {
		t.Fatalf("healthy data=%s", out.Data)
	}
}
