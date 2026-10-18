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

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/clock"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/pkg/ze"
)

// dHCPConfig holds optional DHCP client parameters parsed from config.
type dHCPConfig struct {
	Hostname             string // DHCPv4 option 12
	ClientID             string // DHCPv4 option 61
	PDLength             int    // DHCPv6 requested prefix delegation length (0 = server decides)
	DUID                 string // DHCPv6 DUID override
	ResolvConfPath       string // path for DNS resolv.conf (empty = disabled)
	HasStaticNameServers bool   // when true, DHCP does not write resolv.conf (static servers take priority)
	RouteMetric          int    // metric for DHCP-installed default routes (0 = kernel default)
}

// DHCPClient manages DHCP on a single interface unit.
//
// Start MUST be called to begin DHCP negotiation. Stop MUST be called
// after a successful Start to release resources and remove leased addresses.
// Stop is safe to call multiple times (protected by sync.Once).
type DHCPClient struct {
	ifaceName string
	unit      string
	eventBus  ze.EventBus
	config    dHCPConfig
	stop      chan struct{}
	done      chan struct{}
	v4        bool
	v6        bool
	started   atomic.Bool
	stopOnce  sync.Once

	// clock is the only time source of the DHCP workers: every wait and every
	// lease deadline reads it, so a test drives lease timing with a fake
	// clock. clock.RealClock{} in production.
	clock clock.Clock
	// v4Exchange sends the DHCPv4 messages and reads the answers. The
	// production exchanger runs over nclient4; a test replaces it before Start.
	v4Exchange v4Exchanger

	// v4Owner names this client in the leased-address registry
	// (iface.RegisterLeasedAddresses). One client serves one unit.
	v4Owner string
	// v4Address is the CIDR the v4 worker installed and registered as leased,
	// empty when it holds none. Read and written by the v4 worker only.
	v4Address string
}

// newDHCPClient creates a DHCP client for the named interface.
// eventBus must not be nil. At least one of v4 or v6 must be true.
// cfg carries optional parameters (hostname, client-id) from the config.
// clk must not be nil: it times every wait and every lease.
func newDHCPClient(ifaceName, unit string, eventBus ze.EventBus, v4, v6 bool, cfg dHCPConfig, clk clock.Clock) (*DHCPClient, error) {
	if eventBus == nil {
		return nil, errors.New("iface dhcp: event bus is nil")
	}
	if clk == nil {
		return nil, errors.New("iface dhcp: clock is nil")
	}
	if !v4 && !v6 {
		return nil, errors.New("iface dhcp: at least one of v4 or v6 must be enabled")
	}
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return nil, fmt.Errorf("iface dhcp: %w", err)
	}
	return &DHCPClient{
		ifaceName:  ifaceName,
		unit:       unit,
		eventBus:   eventBus,
		config:     cfg,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		v4:         v4,
		v6:         v6,
		v4Owner:    "iface-dhcp-v4 " + ifaceName + " unit " + unit,
		clock:      clk,
		v4Exchange: nclient4Exchanger{clock: clk},
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
	// Whatever way the worker ends, it no longer holds a lease. runV4 removes
	// the address before it returns; after a panic the address stays without
	// an owner, and the next reconcile treats it as any unnamed address.
	defer iface.UnregisterLeasedAddresses(c.v4Owner)
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

// sleepOrStop waits d on the client's clock. It answers true when the wait
// ran out and false when Stop was called first.
//
// The wait is an AfterFunc closing a channel, never a clock timer channel: a
// fake clock fires only AfterFunc callbacks, so this is what lets a test step
// the worker through its deadlines.
func (c *DHCPClient) sleepOrStop(d time.Duration) bool {
	if d <= 0 {
		return !c.stopped()
	}

	woken := make(chan struct{})
	timer := c.clock.AfterFunc(d, func() { close(woken) })
	defer timer.Stop()

	select {
	case <-woken:
		return true
	case <-c.stop:
		return false
	}
}

// sleepUntilOrStop waits until deadline on the client's clock, with the
// answers of sleepOrStop.
func (c *DHCPClient) sleepUntilOrStop(deadline time.Time) bool {
	return c.sleepOrStop(deadline.Sub(c.clock.Now()))
}

// publishDHCP emits a DHCP lease event on the EventBus under the
// interface namespace. topic is one of the legacy iface.TopicDHCPLease*
// constants, which are mapped to the corresponding stream event type.
func (c *DHCPClient) publishDHCP(topic string, payload iface.DHCPPayload) {
	eventType, ok := dhcpTopicToEventType(topic)
	if !ok {
		loggerPtr.Load().Debug("iface dhcp: unknown topic", "topic", topic)
		return
	}
	// Ensure name and unit are set in the payload before publishing.
	if payload.Name == "" {
		payload.Name = c.ifaceName
	}
	if payload.Unit == "" {
		payload.Unit = c.unit
	}
	data, err := json.Marshal(payload)
	if err != nil {
		loggerPtr.Load().Debug("iface dhcp: marshal failed", "event", eventType, "err", err)
		return
	}
	if _, err := c.eventBus.Emit(ifaceevents.Namespace, eventType, string(data)); err != nil {
		loggerPtr.Load().Debug("iface dhcp: emit failed", "event", eventType, "err", err)
	}
}

// dhcpTopicToEventType maps the legacy bus topic strings for DHCP lease
// events to the corresponding stream event types in the interface namespace.
func dhcpTopicToEventType(topic string) (string, bool) {
	switch topic {
	case iface.TopicDHCPLeaseAcquired:
		return ifaceevents.EventDHCPAcquired, true
	case iface.TopicDHCPLeaseRenewed:
		return ifaceevents.EventDHCPRenewed, true
	case iface.TopicDHCPLeaseExpired:
		return ifaceevents.EventDHCPExpired, true
	}
	return "", false
}
