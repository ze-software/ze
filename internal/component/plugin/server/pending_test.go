package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// TestPendingRequests_AddComplete verifies basic request lifecycle.
//
// VALIDATES: Requests can be added and completed with responses.
// PREVENTS: Lost responses, misrouted requests.
func TestPendingRequests_AddComplete(t *testing.T) {
	pending := newPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	// Mock client - we'll check response delivery
	respCh := make(chan *plugin.Response, 1)

	req := &PendingRequest{
		Command:  "myapp status",
		Process:  proc,
		Timeout:  DefaultCommandTimeout,
		RespChan: respCh,
	}

	serial := pending.Add(req)

	// Serial should be alpha encoded
	if serial == "" {
		t.Fatal("Add should return non-empty serial")
	}
	if !isAlphaSerial(serial) {
		t.Errorf("expected alpha serial, got %q", serial)
	}

	// Complete the request
	if err := pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"result": "test"}}); err != nil {
		t.Errorf("Complete should deliver for a valid serial: %v", err)
	}

	// Check response was delivered
	select {
	case resp := <-respCh:
		if resp.Status != plugin.StatusDone {
			t.Errorf("expected status 'done', got %q", resp.Status)
		}
	default:
		t.Error("response should have been delivered")
	}

	// Completing again should fail (already completed)
	if err := pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone}); !errors.Is(err, ErrPendingUnknownSerial) {
		t.Errorf("Complete on an already-completed serial should report it unknown, got %v", err)
	}
}

// TestPendingRequests_Timeout verifies timeout handling.
//
// VALIDATES: Timed-out requests are cleaned up and error delivered.
// PREVENTS: Memory leaks from stuck requests, clients waiting forever.
func TestPendingRequests_Timeout(t *testing.T) {
	pending := newPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	respCh := make(chan *plugin.Response, 1)

	req := &PendingRequest{
		Command:  "myapp status",
		Process:  proc,
		Timeout:  50 * time.Millisecond, // Short timeout for test
		RespChan: respCh,
	}

	serial := pending.Add(req)

	// Wait for timeout to deliver error response
	require.Eventually(t, func() bool {
		select {
		case resp := <-respCh:
			if resp.Status != plugin.StatusError {
				t.Errorf("expected status 'error' for timeout, got %q", resp.Status)
			}
			if resp.Error == "" {
				t.Error("expected error message in Error")
			}
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "timeout response should have been delivered")

	// Complete after timeout should fail
	if err := pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone}); !errors.Is(err, ErrPendingUnknownSerial) {
		t.Errorf("Complete after a timeout should report the serial unknown, got %v", err)
	}
}

// TestPendingRequests_CancelAll verifies cleanup on process death.
//
// VALIDATES: All pending requests for a process are canceled on death.
// PREVENTS: Clients waiting forever when process dies.
func TestPendingRequests_CancelAll(t *testing.T) {
	pending := newPendingRequests()
	proc1 := process.NewProcess(plugin.PluginConfig{Name: "proc1"})
	proc2 := process.NewProcess(plugin.PluginConfig{Name: "proc2"})

	respCh1 := make(chan *plugin.Response, 1)
	respCh2 := make(chan *plugin.Response, 1)
	respCh3 := make(chan *plugin.Response, 1)

	// Add requests from two processes
	pending.Add(&PendingRequest{
		Command:  "myapp status",
		Process:  proc1,
		Timeout:  DefaultCommandTimeout,
		RespChan: respCh1,
	})
	serial2 := pending.Add(&PendingRequest{
		Command:  "otherapp status",
		Process:  proc2,
		Timeout:  DefaultCommandTimeout,
		RespChan: respCh2,
	})
	pending.Add(&PendingRequest{
		Command:  "myapp reload",
		Process:  proc1,
		Timeout:  DefaultCommandTimeout,
		RespChan: respCh3,
	})

	// Cancel all for proc1
	pending.CancelAll(proc1)

	// Check proc1 requests got error responses
	select {
	case resp := <-respCh1:
		if resp.Status != plugin.StatusError {
			t.Error("expected error for canceled request")
		}
	default:
		t.Error("canceled request should have received error")
	}

	select {
	case resp := <-respCh3:
		if resp.Status != plugin.StatusError {
			t.Error("expected error for canceled request")
		}
	default:
		t.Error("canceled request should have received error")
	}

	// proc2 request should still be pending
	select {
	case <-respCh2:
		t.Error("proc2 request should not have received response")
	default:
		// Good - still pending
	}

	// Complete proc2 request should work
	if err := pending.Complete(serial2, &plugin.Response{Status: plugin.StatusDone}); err != nil {
		t.Errorf("proc2 request should still be completable: %v", err)
	}
}

// TestPendingRequests_Limit verifies per-process limit enforcement.
//
// VALIDATES: Processes cannot exceed MaxPendingPerProcess.
// PREVENTS: Memory exhaustion from stuck process.
func TestPendingRequests_Limit(t *testing.T) {
	pending := newPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	// Fill up to limit
	for i := range MaxPendingPerProcess {
		respCh := make(chan *plugin.Response, 1)
		serial := pending.Add(&PendingRequest{
			Command:  "myapp status",
			Process:  proc,
			Timeout:  DefaultCommandTimeout,
			RespChan: respCh,
		})
		if serial == "" {
			t.Fatalf("Add should succeed for request %d", i)
		}
	}

	// Next add should fail
	respCh := make(chan *plugin.Response, 1)
	serial := pending.Add(&PendingRequest{
		Command:  "myapp overflow",
		Process:  proc,
		Timeout:  DefaultCommandTimeout,
		RespChan: respCh,
	})
	if serial != "" {
		t.Error("Add should return empty serial when limit exceeded")
	}

	// Error should be sent to channel
	select {
	case resp := <-respCh:
		if resp.Status != plugin.StatusError {
			t.Error("expected error response for limit exceeded")
		}
	default:
		t.Error("limit exceeded should send error to channel")
	}
}

// TestPendingRequests_SerialUniqueness verifies serial generation.
//
// VALIDATES: Each request gets a unique alpha serial.
// PREVENTS: Collisions between concurrent requests.
func TestPendingRequests_SerialUniqueness(t *testing.T) {
	pending := newPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	serials := make(map[string]bool)
	for range 100 {
		respCh := make(chan *plugin.Response, 1)
		serial := pending.Add(&PendingRequest{
			Command:  "myapp status",
			Process:  proc,
			Timeout:  DefaultCommandTimeout,
			RespChan: respCh,
		})
		if serials[serial] {
			t.Errorf("duplicate serial: %s", serial)
		}
		serials[serial] = true

		// Complete to free up limit
		if err := pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone}); err != nil {
			t.Errorf("Complete should free the limit slot: %v", err)
		}
	}
}

// TestPendingRequests_StreamingResponse verifies partial response handling.
//
// VALIDATES: Streaming responses reset timeout between chunks.
// PREVENTS: Timeout during large data transfers.
func TestPendingRequests_StreamingResponse(t *testing.T) {
	pending := newPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	respCh := make(chan *plugin.Response, 10)

	req := &PendingRequest{
		Command:  "myapp dump",
		Process:  proc,
		Timeout:  100 * time.Millisecond,
		RespChan: respCh,
	}

	serial := pending.Add(req)

	// Send partial responses, waiting for each to be received before sending next.
	for i := range 3 {
		if err := pending.Partial(serial, &plugin.Response{
			Status: "partial",
			Data:   plugin.Map{"chunk": i},
		}); err != nil {
			t.Errorf("Partial should succeed for chunk %d: %v", i, err)
		}
		// Wait for this partial to be received before sending next.
		require.Eventually(t, func() bool {
			return len(respCh) > i
		}, 2*time.Second, time.Millisecond, "partial %d should be received", i)
	}

	// Complete
	if err := pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"result": "final"}}); err != nil {
		t.Errorf("Complete should succeed after partials: %v", err)
	}

	// Wait for all responses to arrive, then count.
	require.Eventually(t, func() bool {
		return len(respCh) == 4 // 3 partials + 1 final
	}, 2*time.Second, time.Millisecond, "expected 4 responses")
	close(respCh)
	count := 0
	for range respCh {
		count++
	}
	if count != 4 { // 3 partials + 1 final
		t.Errorf("expected 4 responses, got %d", count)
	}
}

// TestPendingRequests_ConcurrentAccess verifies thread safety.
//
// VALIDATES: Concurrent adds, completes, and lookups work correctly.
// PREVENTS: Race conditions in production use.
func TestPendingRequests_ConcurrentAccess(t *testing.T) {
	pending := newPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 100)

	// Spawn concurrent adders and completers
	for range 50 {
		go func() {
			respCh := make(chan *plugin.Response, 1)
			serial := pending.Add(&PendingRequest{
				Command:  "myapp status",
				Process:  proc,
				Timeout:  DefaultCommandTimeout,
				RespChan: respCh,
			})
			if serial == "" {
				done <- nil
				return
			}
			done <- pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone})
		}()
	}

	// Wait for all
	for range 50 {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("concurrent Complete should deliver: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for concurrent operations")
		}
	}
}

// TestPartialWaitsForASlowConsumerRatherThanDropping checks that a stream of
// responses reaches a consumer that drains slower than the producer sends. The
// method: a channel that holds one response and a consumer that takes one every
// few milliseconds, against a producer that sends five as fast as it can.
//
// VALIDATES: R-9 -- a full channel applies backpressure to the producer instead
// of discarding the response.
// PREVENTS: a record sequence losing rows while its terminator still counts
// them, which makes the answer wrong rather than slow.
func TestPartialWaitsForASlowConsumerRatherThanDropping(t *testing.T) {
	pending := newPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	const rows = 5

	// Capacity one: every row after the first meets a full channel.
	respCh := make(chan *plugin.Response, 1)
	serial := pending.Add(&PendingRequest{
		Command:  "myapp dump",
		Process:  proc,
		Timeout:  5 * time.Second,
		RespChan: respCh,
	})
	require.NotEmpty(t, serial)

	received := make(chan int, rows+1)
	go func() {
		for range rows {
			resp := <-respCh
			data, ok := resp.Data.(plugin.Map)
			if !ok {
				return
			}
			row, ok := data["row"].(int)
			if !ok {
				return
			}
			received <- row
			time.Sleep(2 * time.Millisecond)
		}
	}()

	for row := range rows {
		if err := pending.Partial(serial, &plugin.Response{
			Status: "partial",
			Data:   plugin.Map{"row": row},
		}); err != nil {
			t.Fatalf("row %d was not delivered: %v", row, err)
		}
	}

	for row := range rows {
		select {
		case got := <-received:
			require.Equal(t, row, got, "rows arrive in order and none is skipped")
		case <-time.After(5 * time.Second):
			t.Fatalf("row %d never arrived", row)
		}
	}
}

// TestPartialReportsAResponseItCannotDeliver checks that a consumer which stops
// reading altogether is told, rather than losing rows quietly. The method: a
// channel that holds one response, a caller that reads none, and a request
// whose timeout is short.
//
// VALIDATES: R-9 -- an undeliverable response is reported and is NOT counted as
// delivered.
// PREVENTS: the silent drop that a non-blocking send with an empty default arm
// performs, which no counter and no log records.
func TestPartialReportsAResponseItCannotDeliver(t *testing.T) {
	pending := newPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	respCh := make(chan *plugin.Response, 1)
	serial := pending.Add(&PendingRequest{
		Command:  "myapp dump",
		Process:  proc,
		Timeout:  50 * time.Millisecond,
		RespChan: respCh,
	})
	require.NotEmpty(t, serial)

	require.NoError(t, pending.Partial(serial, &plugin.Response{
		Status: "partial",
		Data:   plugin.Map{"row": 0},
	}), "the first row fits in the channel")

	err := pending.Partial(serial, &plugin.Response{
		Status: "partial",
		Data:   plugin.Map{"row": 1},
	})
	require.ErrorIs(t, err, ErrPendingUndeliverable,
		"a row nobody takes is reported, never dropped in silence")

	require.Len(t, respCh, 1, "the channel still holds the row that did fit, and no other")
}
