package vpp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitConnectedImmediate(t *testing.T) {
	c := &Connector{connected: true}
	if err := c.WaitConnected(context.Background(), 100*time.Millisecond); err != nil {
		t.Fatalf("WaitConnected on already-connected Connector: %v", err)
	}
}

func TestWaitConnectedTimeout(t *testing.T) {
	c := NewConnector("/does/not/matter")
	start := time.Now()
	err := c.WaitConnected(context.Background(), 120*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("returned too early: %s", elapsed)
	}
	if elapsed > 400*time.Millisecond {
		t.Fatalf("returned too late: %s", elapsed)
	}
}

func TestWaitConnectedContextCancel(t *testing.T) {
	c := NewConnector("/does/not/matter")
	ctx, cancel := context.WithCancel(context.Background())
	// Run WaitConnected (never connects) in a goroutine and cancel the context
	// from the test body. WaitConnected observes ctx.Done() in its select loop
	// and returns context.Canceled; no fixed delay needed.
	errc := make(chan error, 1)
	go func() {
		errc <- c.WaitConnected(ctx, 5*time.Second)
	}()
	cancel()
	err := <-errc
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestWaitConnectedPreCancelledContext(t *testing.T) {
	// VALIDATES: the upfront ctx.Err() check short-circuits before the
	// 50ms polling loop would fire. Without it, a canceled ctx would
	// still wait one tick before returning.
	c := NewConnector("/does/not/matter")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	err := c.WaitConnected(ctx, time.Second)
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on pre-canceled ctx, got %v", err)
	}
	if elapsed > 20*time.Millisecond {
		t.Errorf("pre-canceled ctx should return immediately, took %s", elapsed)
	}
}

func TestWaitConnectedZeroTimeout(t *testing.T) {
	c := NewConnector("/does/not/matter")
	if err := c.WaitConnected(context.Background(), 0); err == nil {
		t.Fatal("expected error on zero timeout, got nil")
	}
}

func TestWaitConnectedBecomesConnected(t *testing.T) {
	c := NewConnector("/does/not/matter")
	// Run WaitConnected first (it is not connected yet, so it enters its 50ms
	// polling loop), then flip connected=true from the test body. The next
	// poll tick detects the transition and WaitConnected returns nil. No fixed
	// delay: the polling loop is the synchronization point being exercised.
	errc := make(chan error, 1)
	go func() {
		errc <- c.WaitConnected(context.Background(), 5*time.Second)
	}()
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	if err := <-errc; err != nil {
		t.Fatalf("WaitConnected should have succeeded: %v", err)
	}
}
