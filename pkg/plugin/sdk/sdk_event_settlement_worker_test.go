// Design: docs/architecture/api/process-protocol.md — applied event settlement.
// Related: sdk_event_settlement.go — bounded worker execution and shutdown.

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func eventSettlementTestParams(t *testing.T) json.RawMessage {
	t.Helper()
	params, err := json.Marshal(rpc.EventSettlementInput{DeadlineMS: time.Now().Add(5 * time.Second).UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	return params
}

// TestEventSettlementQueueIsBounded fills the waiting FIFO without a serving
// goroutine and proves the next admission refuses rather than blocks or drops.
func TestEventSettlementQueueIsBounded(t *testing.T) {
	worker := newEventSettlementWorker(t.Name(), func(context.Context) error {
		t.Error("handler ran during admission")
		return nil
	}, func(error) { t.Error("unexpected abort") })
	params := eventSettlementTestParams(t)
	answer := func(time.Time, json.RawMessage, error) { t.Error("unserved request answered") }
	for range eventSettlementQueueDepth {
		if err := worker.admit(params, answer); err != nil {
			t.Fatal(err)
		}
	}
	if err := worker.admit(params, answer); err == nil {
		t.Fatal("full settlement queue accepted an extra request")
	}
	if len(worker.requests) != eventSettlementQueueDepth {
		t.Fatalf("waiting requests=%d", len(worker.requests))
	}
}

// TestEventSettlementRejectsMalformedAdmission pairs malformed inputs with one
// useful admitted request, proving refusal happens before handler execution.
func TestEventSettlementRejectsMalformedAdmission(t *testing.T) {
	worker := newEventSettlementWorker(t.Name(), func(context.Context) error {
		t.Error("unserved handler invoked")
		return nil
	}, func(error) { t.Error("unexpected abort") })
	for _, params := range []string{`{}`, `null`, `{"deadline-ms":null}`, `{"deadline-ms":1}`, `{"deadline-ms":"bad"}`} {
		if err := worker.admit(json.RawMessage(params), nil); err == nil {
			t.Fatalf("invalid request admitted: %s", params)
		}
	}
	if len(worker.requests) != 0 {
		t.Fatal("invalid request changed the queue")
	}
	if err := worker.admit(eventSettlementTestParams(t), func(time.Time, json.RawMessage, error) {}); err != nil {
		t.Fatalf("valid control refused: %v", err)
	}
}

// TestEventSettlementCancellationJoinsHandler proves closing the loop cancels
// the handler and joins its bounded worker before the stop adapter returns.
func TestEventSettlementCancellationJoinsHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p, _ := newTestPair(t)
	entered, finished := make(chan struct{}), make(chan struct{})
	p.OnEventSettlement(func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		close(finished)
		return ctx.Err()
	})
	workers := p.startAsyncCallbacks(ctx, func(error) { t.Error("unexpected abort") })
	result := make(chan error, 1)
	if err := workers[rpc.MethodEventSettlement].admit(eventSettlementTestParams(t), func(_ time.Time, body json.RawMessage, err error) {
		if err != nil {
			result <- err
			return
		}
		var output rpc.EventSettlementOutput
		if decodeErr := json.Unmarshal(body, &output); decodeErr != nil {
			result <- decodeErr
			return
		}
		if output.Status == rpc.StatusOK {
			result <- errors.New("canceled handler acknowledged success")
			return
		}
		result <- nil
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("settlement handler never started")
	}
	stopped := make(chan struct{})
	go func() {
		workers.stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("settlement worker did not join")
	}
	select {
	case <-finished:
	default:
		t.Fatal("stop returned before the handler joined")
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("canceled handler receipt missing")
	}
}

// TestEventSettlementExpiredQueuedRequestSkipsHandler models a deadline that
// expired after successful admission and proves it produces a failed receipt.
func TestEventSettlementExpiredQueuedRequestSkipsHandler(t *testing.T) {
	worker := newEventSettlementWorker(t.Name(), func(context.Context) error {
		t.Error("expired queued handler ran")
		return nil
	}, func(error) { t.Error("unexpected abort") })
	result := make(chan rpc.EventSettlementOutput, 1)
	if err := worker.admit(eventSettlementTestParams(t), func(_ time.Time, body json.RawMessage, err error) {
		if err != nil {
			t.Errorf("encode receipt: %v", err)
		}
		var output rpc.EventSettlementOutput
		if decodeErr := json.Unmarshal(body, &output); decodeErr != nil {
			t.Errorf("decode receipt: %v", decodeErr)
		}
		result <- output
	}); err != nil {
		t.Fatal(err)
	}
	request := <-worker.requests
	// Advance this already admitted query's clock boundary without a sleep.
	request.query.deadline = time.Now().Add(-time.Second)
	if err := worker.serve(t.Context(), request); err != nil {
		t.Fatalf("expiry treated as a panic: %v", err)
	}
	output := <-result
	if output.Status != rpc.StatusError {
		t.Fatalf("expired request acknowledged: %+v", output)
	}
	if !strings.Contains(output.Error, "deadline") {
		t.Fatalf("expiry cause missing: %+v", output)
	}
}

// TestEventSettlementPanicFailsConnection runs the real SDK loops on both
// transports and requires the panic to fail the caller and loop, never succeed.
func TestEventSettlementPanicFailsConnection(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		h := newSourcePolicyHarness(t, bridged)
		h.p.OnEventSettlement(func(context.Context) error { panic("settlement probe") })
		h.start(t, ctx)
		err := eventSettlementProbeCall(ctx, h)
		if err == nil {
			t.Fatal("handler panic acknowledged settlement")
		}
		if !strings.Contains(err.Error(), "panic") {
			t.Fatalf("panic cause missing: %v", err)
		}
	})
}

// TestEventSettlementDeclarationFollowsHandler checks the actual Stage-1
// request with and without registration; caller-provided support is not trusted.
func TestEventSettlementDeclarationFollowsHandler(t *testing.T) {
	for _, registered := range []bool{false, true} {
		p, engine := newTestPair(t)
		if registered {
			p.OnEventSettlement(func(context.Context) error { return nil })
		}
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		declaration := &Registration{EventSettlement: !registered}
		done := make(chan error, 1)
		go func() { done <- p.Run(ctx, declaration) }()
		request := readEngineRequest(t, ctx, engine.mux)
		var sent Registration
		if err := json.Unmarshal(request.Params, &sent); err != nil {
			t.Fatal(err)
		}
		if sent.EventSettlement != registered {
			t.Errorf("registered=%t declared=%t", registered, sent.EventSettlement)
		}
		if declaration.EventSettlement == registered {
			t.Error("Run changed the caller's declaration")
		}
		cancel()
		<-done
	}
}
