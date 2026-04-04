// Design: docs/features/interfaces.md -- DHCP client lifecycle
// Overview: ifacedhcp.go -- package hub

//go:build linux

package ifacedhcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
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
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
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

// Start begins DHCP negotiation in background goroutines.
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

	go func() {
		wg.Wait()
		close(c.done)
	}()

	return nil
}

// Stop signals DHCP goroutines to exit and waits for completion.
func (c *DHCPClient) Stop() {
	c.stopOnce.Do(func() { close(c.stop) })
	if c.started.Load() {
		<-c.done
	}
}

func (c *DHCPClient) stopped() bool {
	select {
	case <-c.stop:
		return true
	default: // non-blocking check, not a silent ignore
		return false
	}
}

func (c *DHCPClient) safeRunV4() {
	defer func() {
		if r := recover(); r != nil {
			loggerPtr.Load().Error("iface dhcp: panic in v4 worker",
				"iface", c.ifaceName, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	c.runV4()
}

func (c *DHCPClient) safeRunV6() {
	defer func() {
		if r := recover(); r != nil {
			loggerPtr.Load().Error("iface dhcp: panic in v6 worker",
				"iface", c.ifaceName, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	c.runV6()
}

func (c *DHCPClient) stoppableContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-c.stop:
			cancel()
		case <-ctx.Done(): // context canceled normally
		}
	}()
	return ctx, cancel
}

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

func (c *DHCPClient) publishDHCP(topic string, payload iface.DHCPPayload) {
	data, err := json.Marshal(payload)
	if err != nil {
		loggerPtr.Load().Debug("iface dhcp: marshal failed", "topic", topic, "err", err)
		return
	}
	c.bus.Publish(topic, data, map[string]string{
		"name": c.ifaceName,
		"unit": fmt.Sprintf("%d", c.unit),
	})
}
