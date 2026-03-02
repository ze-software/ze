package server

import (
	"context"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/process"
)

// TestPendingRequests_AddComplete verifies basic request lifecycle.
//
// VALIDATES: Requests can be added and completed with responses.
// PREVENTS: Lost responses, misrouted requests.
func TestPendingRequests_AddComplete(t *testing.T) {
	pending := NewPendingRequests()
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
	ok := pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone, Data: "test"})
	if !ok {
		t.Error("Complete should return true for valid serial")
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
	ok = pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone})
	if ok {
		t.Error("Complete should return false for already-completed serial")
	}
}

// TestPendingRequests_Timeout verifies timeout handling.
//
// VALIDATES: Timed-out requests are cleaned up and error delivered.
// PREVENTS: Memory leaks from stuck requests, clients waiting forever.
func TestPendingRequests_Timeout(t *testing.T) {
	pending := NewPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	respCh := make(chan *plugin.Response, 1)

	req := &PendingRequest{
		Command:  "myapp status",
		Process:  proc,
		Timeout:  50 * time.Millisecond, // Short timeout for test
		RespChan: respCh,
	}

	serial := pending.Add(req)

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Check timeout response was delivered
	select {
	case resp := <-respCh:
		if resp.Status != plugin.StatusError {
			t.Errorf("expected status 'error' for timeout, got %q", resp.Status)
		}
		if resp.Data == nil {
			t.Error("expected error message in Data")
		}
	default:
		t.Error("timeout response should have been delivered")
	}

	// Complete after timeout should fail
	ok := pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone})
	if ok {
		t.Error("Complete should return false after timeout")
	}
}

// TestPendingRequests_CancelAll verifies cleanup on process death.
//
// VALIDATES: All pending requests for a process are canceled on death.
// PREVENTS: Clients waiting forever when process dies.
func TestPendingRequests_CancelAll(t *testing.T) {
	pending := NewPendingRequests()
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
	ok := pending.Complete(serial2, &plugin.Response{Status: plugin.StatusDone})
	if !ok {
		t.Error("proc2 request should still be completable")
	}
}

// TestPendingRequests_Limit verifies per-process limit enforcement.
//
// VALIDATES: Processes cannot exceed MaxPendingPerProcess.
// PREVENTS: Memory exhaustion from stuck process.
func TestPendingRequests_Limit(t *testing.T) {
	pending := NewPendingRequests()
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
	pending := NewPendingRequests()
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
		pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone})
	}
}

// TestPendingRequests_StreamingResponse verifies partial response handling.
//
// VALIDATES: Streaming responses reset timeout between chunks.
// PREVENTS: Timeout during large data transfers.
func TestPendingRequests_StreamingResponse(t *testing.T) {
	pending := NewPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	respCh := make(chan *plugin.Response, 10)

	req := &PendingRequest{
		Command:  "myapp dump",
		Process:  proc,
		Timeout:  100 * time.Millisecond,
		RespChan: respCh,
	}

	serial := pending.Add(req)

	// Send partial responses
	for i := range 3 {
		ok := pending.Partial(serial, &plugin.Response{
			Status: "partial",
			Data:   map[string]int{"chunk": i},
		})
		if !ok {
			t.Errorf("Partial should succeed for chunk %d", i)
		}
		time.Sleep(30 * time.Millisecond) // Below timeout
	}

	// Complete
	ok := pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone, Data: "final"})
	if !ok {
		t.Error("Complete should succeed after partials")
	}

	// Count responses
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
	pending := NewPendingRequests()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan bool, 100)

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
			if serial != "" {
				pending.Complete(serial, &plugin.Response{Status: plugin.StatusDone})
			}
			done <- true
		}()
	}

	// Wait for all
	for range 50 {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("timeout waiting for concurrent operations")
		}
	}
}
