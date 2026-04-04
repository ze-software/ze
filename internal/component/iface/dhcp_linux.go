// Design: docs/features/interfaces.md — DHCP client for interface plugin
// Overview: iface.go — shared types and topic constants
// Detail: dhcp_v4_linux.go — DHCPv4 worker, renewal, lease handling
// Detail: dhcp_v6_linux.go — DHCPv6 worker, renewal, lease handling

package iface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// DHCPClient manages DHCP on a single interface unit.
//
// Start MUST be called to begin DHCP negotiation. Stop MUST be called
// after a successful Start to release resources and remove leased addresses.
// Stop is safe to call multiple times (protected by sync.Once).
type DHCPClient struct {
	ifaceName string
	unit      int
	bus       ze.Bus
	stop      chan struct{}
	done      chan struct{}
	v4        bool
	v6        bool
	started   atomic.Bool
	stopOnce  sync.Once
}

// NewDHCPClient creates a DHCP client for the named interface.
// Bus must not be nil. At least one of v4 or v6 must be true.
func NewDHCPClient(ifaceName string, unit int, bus ze.Bus, v4, v6 bool) (*DHCPClient, error) {
	if bus == nil {
		return nil, errors.New("iface dhcp: bus is nil")
	}
	if !v4 && !v6 {
		return nil, errors.New("iface dhcp: at least one of v4 or v6 must be enabled")
	}
	if err := validateIfaceName(ifaceName); err != nil {
		return nil, fmt.Errorf("iface dhcp: %w", err)
	}
	return &DHCPClient{
		ifaceName: ifaceName,
		unit:      unit,
		bus:       bus,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		v4:        v4,
		v6:        v6,
	}, nil
}

// Start begins DHCP negotiation in background goroutines (one per enabled
// protocol version). Returns immediately. MUST call Stop to release resources.
// Must not be called more than once.
func (c *DHCPClient) Start() error {
	if !c.started.CompareAndSwap(false, true) {
		return errors.New("iface dhcp: already started")
	}

	workers := 0
	if c.v4 {
		workers++
	}
	if c.v6 {
		workers++
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	if c.v4 {
		go func() {
			defer wg.Done()
			c.safeRunV4()
		}()
	}
	if c.v6 {
		go func() {
			defer wg.Done()
			c.safeRunV6()
		}()
	}

	// Close done when all workers exit.
	go func() {
		wg.Wait()
		close(c.done)
	}()

	return nil
}

// Stop signals DHCP goroutines to exit and waits for completion.
// Safe to call multiple times. Safe to call if Start was never called.
func (c *DHCPClient) Stop() {
	c.stopOnce.Do(func() { close(c.stop) })
	if c.started.Load() {
		<-c.done
	}
}

// stopped returns true if stop has been signaled.
func (c *DHCPClient) stopped() bool {
	select {
	case <-c.stop:
		return true
	default: // non-blocking: not stopped yet
		return false
	}
}

// safeRunV4 wraps runV4 with panic recovery.
func (c *DHCPClient) safeRunV4() {
	defer func() {
		if r := recover(); r != nil {
			loggerPtr.Load().Error("iface dhcp: panic in v4 worker",
				"iface", c.ifaceName, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	c.runV4()
}

// safeRunV6 wraps runV6 with panic recovery.
func (c *DHCPClient) safeRunV6() {
	defer func() {
		if r := recover(); r != nil {
			loggerPtr.Load().Error("iface dhcp: panic in v6 worker",
				"iface", c.ifaceName, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	c.runV6()
}

// stoppableContext returns a context that is canceled when the DHCP client's
// stop channel is closed. Callers MUST call the returned cancel function when
// the operation completes to release the monitoring goroutine.
func (c *DHCPClient) stoppableContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-c.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// sleepOrStop blocks for the given duration or until stop is signaled.
// Returns true if the sleep completed, false if stop was signaled.
func (c *DHCPClient) sleepOrStop(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-c.stop:
		return false
	}
}

// publishDHCP marshals a DHCPPayload and publishes it to the bus.
func (c *DHCPClient) publishDHCP(topic string, payload DHCPPayload) {
	data, err := json.Marshal(payload)
	if err != nil {
		loggerPtr.Load().Debug("iface dhcp: marshal failed",
			"topic", topic, "err", err)
		return
	}
	c.bus.Publish(topic, data, map[string]string{
		"name": c.ifaceName,
		"unit": fmt.Sprintf("%d", c.unit),
	})
}
