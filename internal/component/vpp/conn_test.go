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
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()
	err := c.WaitConnected(ctx, 5*time.Second)
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
	go func() {
		time.Sleep(80 * time.Millisecond)
		c.mu.Lock()
		c.connected = true
		c.mu.Unlock()
	}()
	if err := c.WaitConnected(context.Background(), 500*time.Millisecond); err != nil {
		t.Fatalf("WaitConnected should have succeeded: %v", err)
	}
}
