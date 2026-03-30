// Design: plan/spec-iface-0-umbrella.md — DHCP client for interface plugin
// Overview: iface.go — shared types and topic constants

package iface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/nclient6"
	"github.com/vishvananda/netlink"

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

// runV4 is the long-lived DHCPv4 worker. It performs DORA (Discover-Offer-
// Request-Ack), installs the leased address, and renews at T1/T2 intervals.
func (c *DHCPClient) runV4() {
	logger := loggerPtr.Load()

	for !c.stopped() {
		_, err := net.InterfaceByName(c.ifaceName)
		if err != nil {
			logger.Warn("iface dhcp v4: interface lookup failed",
				"iface", c.ifaceName, "err", err)
			if !c.sleepOrStop(5 * time.Second) {
				return
			}
			continue
		}

		client, err := nclient4.New(c.ifaceName)
		if err != nil {
			logger.Warn("iface dhcp v4: client creation failed",
				"iface", c.ifaceName, "err", err)
			if !c.sleepOrStop(5 * time.Second) {
				return
			}
			continue
		}

		ctx, ctxCancel := c.stoppableContext()
		lease, err := client.Request(ctx)
		ctxCancel()
		if closeErr := client.Close(); closeErr != nil {
			logger.Debug("iface dhcp v4: client close failed",
				"iface", c.ifaceName, "err", closeErr)
		}

		if err != nil {
			logger.Warn("iface dhcp v4: request failed",
				"iface", c.ifaceName, "err", err)
			if !c.sleepOrStop(10 * time.Second) {
				return
			}
			continue
		}

		if lease == nil || lease.ACK == nil {
			logger.Warn("iface dhcp v4: nil lease or ACK",
				"iface", c.ifaceName)
			if !c.sleepOrStop(10 * time.Second) {
				return
			}
			continue
		}

		ack := lease.ACK
		c.handleV4Lease(ack, TopicDHCPLeaseAcquired)

		// Renewal loop: renew at T1, rebind at T2, expire at lease time.
		leaseTime := c.v4LeaseTime(ack)
		t1 := leaseTime / 2
		t2 := leaseTime * 7 / 8

		if !c.sleepOrStop(t1) {
			c.removeV4Addr(ack)
			return
		}

		// T1 renewal attempt.
		newLease, renewed := c.renewV4(lease)
		if renewed {
			lease = newLease
			ack = lease.ACK
			leaseTime = c.v4LeaseTime(ack)
			t1 = leaseTime / 2
			t2 = leaseTime * 7 / 8
			if !c.sleepOrStop(t2 - t1) {
				c.removeV4Addr(ack)
				return
			}
		}

		if !renewed {
			// Wait until T2 before rebind attempt (RFC 2131).
			if !c.sleepOrStop(t2 - t1) {
				c.removeV4Addr(ack)
				return
			}
			newLease, renewed = c.renewV4(lease)
			if renewed {
				lease = newLease
				ack = lease.ACK
				leaseTime = c.v4LeaseTime(ack)
				t2 = leaseTime * 7 / 8
			}
		}

		if renewed {
			remainingLease := leaseTime - t2
			if !c.sleepOrStop(remainingLease) {
				c.removeV4Addr(ack)
				return
			}
		}

		// Lease expired: remove address and publish event.
		c.removeV4Addr(ack)
		c.publishDHCP(TopicDHCPLeaseExpired, c.v4Payload(ack))
	}
}

// renewV4 attempts to renew the DHCPv4 lease. Returns the renewed lease and
// true on success, or nil and false on failure. Callers MUST update their
// local ack/leaseTime/t1/t2 from the returned lease on success.
func (c *DHCPClient) renewV4(lease *nclient4.Lease) (*nclient4.Lease, bool) {
	logger := loggerPtr.Load()

	client, err := nclient4.New(c.ifaceName)
	if err != nil {
		logger.Warn("iface dhcp v4: renewal client failed",
			"iface", c.ifaceName, "err", err)
		return nil, false
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			loggerPtr.Load().Debug("iface dhcp v4: renewal client close failed",
				"iface", c.ifaceName, "err", closeErr)
		}
	}()

	ctx, ctxCancel := c.stoppableContext()
	defer ctxCancel()
	renewed, err := client.Renew(ctx, lease)
	if err != nil {
		logger.Warn("iface dhcp v4: renewal failed",
			"iface", c.ifaceName, "err", err)
		return nil, false
	}

	if renewed == nil || renewed.ACK == nil {
		return nil, false
	}

	c.handleV4Lease(renewed.ACK, TopicDHCPLeaseRenewed)
	return renewed, true
}

// handleV4Lease installs the leased address on the interface and publishes an event.
func (c *DHCPClient) handleV4Lease(ack *dhcpv4.DHCPv4, topic string) {
	logger := loggerPtr.Load()

	ip := ack.YourIPAddr
	mask := ack.SubnetMask()
	if ip == nil || ip.IsUnspecified() {
		logger.Warn("iface dhcp v4: no address in ACK", "iface", c.ifaceName)
		return
	}

	ones, _ := mask.Size()
	if ones == 0 {
		ones = 24
		logger.Warn("iface dhcp v4: no subnet mask in ACK, defaulting to /24",
			"iface", c.ifaceName, "address", ip.String())
	}

	cidr := fmt.Sprintf("%s/%d", ip.String(), ones)
	leaseTime := c.v4LeaseTime(ack)

	link, err := netlink.LinkByName(c.ifaceName)
	if err != nil {
		logger.Warn("iface dhcp v4: link lookup failed",
			"iface", c.ifaceName, "err", err)
		return
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		logger.Warn("iface dhcp v4: parse addr failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
		return
	}
	addr.ValidLft = int(leaseTime.Seconds())
	addr.PreferedLft = int(leaseTime.Seconds())

	if err := netlink.AddrReplace(link, addr); err != nil {
		logger.Warn("iface dhcp v4: addr add failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
		return
	}

	payload := c.v4Payload(ack)
	c.publishDHCP(topic, payload)

	logger.Info("iface dhcp v4: lease obtained",
		"iface", c.ifaceName, "addr", cidr, "lease", leaseTime)
}

// removeV4Addr removes the leased DHCPv4 address from the interface.
func (c *DHCPClient) removeV4Addr(ack *dhcpv4.DHCPv4) {
	logger := loggerPtr.Load()

	ip := ack.YourIPAddr
	mask := ack.SubnetMask()
	ones, _ := mask.Size()
	if ones == 0 {
		ones = 24
	}

	cidr := fmt.Sprintf("%s/%d", ip.String(), ones)

	link, err := netlink.LinkByName(c.ifaceName)
	if err != nil {
		logger.Debug("iface dhcp v4: link lookup for removal",
			"iface", c.ifaceName, "err", err)
		return
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		logger.Debug("iface dhcp v4: parse addr for removal",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
		return
	}

	if err := netlink.AddrDel(link, addr); err != nil {
		logger.Debug("iface dhcp v4: addr removal failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
	}
}

// v4LeaseTime extracts the lease duration from a DHCPv4 ACK.
func (c *DHCPClient) v4LeaseTime(ack *dhcpv4.DHCPv4) time.Duration {
	leaseTime := ack.IPAddressLeaseTime(time.Hour)
	if leaseTime <= 0 {
		leaseTime = time.Hour
	}
	return leaseTime
}

// v4Payload builds a DHCPPayload from a DHCPv4 ACK.
func (c *DHCPClient) v4Payload(ack *dhcpv4.DHCPv4) DHCPPayload {
	ip := ack.YourIPAddr
	mask := ack.SubnetMask()
	ones, _ := mask.Size()
	if ones == 0 {
		ones = 24
	}

	var router string
	routerIP := dhcpv4.GetIP(dhcpv4.OptionRouter, ack.Options)
	if routerIP != nil {
		router = routerIP.String()
	}

	var dns string
	dnsServers := dhcpv4.GetIPs(dhcpv4.OptionDomainNameServer, ack.Options)
	if len(dnsServers) > 0 {
		dns = dnsServers[0].String()
	}

	return DHCPPayload{
		Name:         c.ifaceName,
		Unit:         c.unit,
		Address:      ip.String(),
		PrefixLength: ones,
		Router:       router,
		DNS:          dns,
		LeaseTime:    int(c.v4LeaseTime(ack).Seconds()),
	}
}

// runV6 is the long-lived DHCPv6 worker. It performs SARR (Solicit-Advertise-
// Request-Reply), installs leased addresses (IA_NA) and prefix delegations
// (IA_PD), and handles renewals.
func (c *DHCPClient) runV6() {
	logger := loggerPtr.Load()

	for !c.stopped() {
		_, err := net.InterfaceByName(c.ifaceName)
		if err != nil {
			logger.Warn("iface dhcp v6: interface lookup failed",
				"iface", c.ifaceName, "err", err)
			if !c.sleepOrStop(5 * time.Second) {
				return
			}
			continue
		}

		client, err := nclient6.New(c.ifaceName)
		if err != nil {
			logger.Warn("iface dhcp v6: client creation failed",
				"iface", c.ifaceName, "err", err)
			if !c.sleepOrStop(5 * time.Second) {
				return
			}
			continue
		}

		ctx, ctxCancel := c.stoppableContext()
		// RapidSolicit tries rapid commit first; falls back to full SARR.
		msg, err := client.RapidSolicit(ctx)
		ctxCancel()
		if closeErr := client.Close(); closeErr != nil {
			logger.Debug("iface dhcp v6: client close failed",
				"iface", c.ifaceName, "err", closeErr)
		}

		if err != nil {
			logger.Warn("iface dhcp v6: solicit failed",
				"iface", c.ifaceName, "err", err)
			if !c.sleepOrStop(10 * time.Second) {
				return
			}
			continue
		}

		if msg == nil {
			logger.Warn("iface dhcp v6: nil reply", "iface", c.ifaceName)
			if !c.sleepOrStop(10 * time.Second) {
				return
			}
			continue
		}

		c.handleV6Reply(msg, TopicDHCPLeaseAcquired)

		// Renewal based on IA_NA T1/T2.
		t1, t2, validLft := c.v6Timers(msg)

		if !c.sleepOrStop(t1) {
			c.removeV6Addrs(msg)
			return
		}

		// T1 renewal.
		newMsg, renewed := c.renewV6()
		if renewed {
			msg = newMsg
			t1, t2, validLft = c.v6Timers(msg)
			if !c.sleepOrStop(t2 - t1) {
				c.removeV6Addrs(msg)
				return
			}
		}

		if !renewed {
			// Wait until T2 before rebind attempt, matching v4 behavior.
			if !c.sleepOrStop(t2 - t1) {
				c.removeV6Addrs(msg)
				return
			}
			// T2 rebind attempt.
			newMsg, renewed = c.renewV6()
			if renewed {
				msg = newMsg
				_, t2, validLft = c.v6Timers(msg)
			}
		}

		if renewed {
			remaining := validLft - t2
			if remaining < 0 {
				remaining = time.Minute
			}
			if !c.sleepOrStop(remaining) {
				c.removeV6Addrs(msg)
				return
			}
		}

		// Expired: remove addresses and publish events.
		c.removeV6Addrs(msg)
		c.publishV6Expired(msg)
	}
}

// renewV6 attempts to renew the DHCPv6 lease. Returns the new message and
// true on success, or nil and false on failure. Callers MUST update their
// local msg/t1/t2/validLft from the returned message on success.
//
// Note: nclient6 does not expose Renew/Rebind methods. RapidSolicit performs
// a full re-solicitation which may return different addresses. This is a known
// limitation -- proper DHCPv6 Renew requires raw message construction.
func (c *DHCPClient) renewV6() (*dhcpv6.Message, bool) {
	logger := loggerPtr.Load()

	client, err := nclient6.New(c.ifaceName)
	if err != nil {
		logger.Warn("iface dhcp v6: renewal client failed",
			"iface", c.ifaceName, "err", err)
		return nil, false
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			loggerPtr.Load().Debug("iface dhcp v6: renewal client close failed",
				"iface", c.ifaceName, "err", closeErr)
		}
	}()

	ctx, ctxCancel := c.stoppableContext()
	defer ctxCancel()
	reply, err := client.RapidSolicit(ctx)
	if err != nil {
		logger.Warn("iface dhcp v6: renewal failed",
			"iface", c.ifaceName, "err", err)
		return nil, false
	}

	if reply == nil {
		return nil, false
	}

	c.handleV6Reply(reply, TopicDHCPLeaseRenewed)
	return reply, true
}

// handleV6Reply installs leased addresses from IA_NA options and publishes events.
// Also processes IA_PD (prefix delegation) for publishing.
func (c *DHCPClient) handleV6Reply(msg *dhcpv6.Message, topic string) {
	logger := loggerPtr.Load()

	link, err := netlink.LinkByName(c.ifaceName)
	if err != nil {
		logger.Warn("iface dhcp v6: link lookup failed",
			"iface", c.ifaceName, "err", err)
		return
	}

	// Process IA_NA (non-temporary addresses). Cap iterations to prevent
	// unbounded processing from a rogue DHCPv6 server.
	const maxAddrs = 16
	addrCount := 0
	for _, iana := range msg.Options.IANA() {
		for _, iaAddr := range iana.Options.Addresses() {
			if addrCount >= maxAddrs {
				logger.Warn("iface dhcp v6: too many IA_NA addresses, capping",
					"iface", c.ifaceName, "max", maxAddrs)
				break
			}
			addrCount++
			ip := iaAddr.IPv6Addr
			if ip == nil {
				continue
			}

			cidr := fmt.Sprintf("%s/128", ip.String())
			addr, err := netlink.ParseAddr(cidr)
			if err != nil {
				logger.Warn("iface dhcp v6: parse addr failed",
					"iface", c.ifaceName, "addr", cidr, "err", err)
				continue
			}
			addr.ValidLft = int(iaAddr.ValidLifetime.Seconds())
			addr.PreferedLft = int(iaAddr.PreferredLifetime.Seconds())

			if err := netlink.AddrReplace(link, addr); err != nil {
				logger.Warn("iface dhcp v6: addr add failed",
					"iface", c.ifaceName, "addr", cidr, "err", err)
				continue
			}

			var dns string
			dnsServers := msg.Options.DNS()
			if len(dnsServers) > 0 {
				dns = dnsServers[0].String()
			}

			payload := DHCPPayload{
				Name:         c.ifaceName,
				Unit:         c.unit,
				Address:      ip.String(),
				PrefixLength: 128,
				DNS:          dns,
				LeaseTime:    int(iaAddr.ValidLifetime.Seconds()),
			}
			c.publishDHCP(topic, payload)

			logger.Info("iface dhcp v6: address obtained",
				"iface", c.ifaceName, "addr", cidr,
				"valid", iaAddr.ValidLifetime)
		}
	}

	// Process IA_PD (prefix delegation). Cap iterations like IA_NA above.
	pdCount := 0
	for _, iapd := range msg.Options.IAPD() {
		for _, prefix := range iapd.Options.Prefixes() {
			if pdCount >= maxAddrs {
				logger.Warn("iface dhcp v6: too many IA_PD prefixes, capping",
					"iface", c.ifaceName, "max", maxAddrs)
				break
			}
			pdCount++
			pfx := prefix.Prefix
			if pfx == nil {
				continue
			}

			ones, _ := pfx.Mask.Size()
			payload := DHCPPayload{
				Name:         c.ifaceName,
				Unit:         c.unit,
				Address:      pfx.IP.String(),
				PrefixLength: ones,
				LeaseTime:    int(prefix.ValidLifetime.Seconds()),
			}
			c.publishDHCP(topic, payload)

			logger.Info("iface dhcp v6: prefix delegated",
				"iface", c.ifaceName, "prefix", pfx.String(),
				"valid", prefix.ValidLifetime)
		}
	}
}

// removeV6Addrs removes all IA_NA addresses obtained from the DHCPv6 reply.
func (c *DHCPClient) removeV6Addrs(msg *dhcpv6.Message) {
	logger := loggerPtr.Load()

	link, err := netlink.LinkByName(c.ifaceName)
	if err != nil {
		logger.Debug("iface dhcp v6: link lookup for removal",
			"iface", c.ifaceName, "err", err)
		return
	}

	const maxAddrs = 16
	count := 0
	for _, iana := range msg.Options.IANA() {
		for _, iaAddr := range iana.Options.Addresses() {
			if count >= maxAddrs {
				break
			}
			count++
			ip := iaAddr.IPv6Addr
			if ip == nil {
				continue
			}
			cidr := fmt.Sprintf("%s/128", ip.String())
			addr, err := netlink.ParseAddr(cidr)
			if err != nil {
				continue
			}
			if err := netlink.AddrDel(link, addr); err != nil {
				logger.Debug("iface dhcp v6: addr removal failed",
					"iface", c.ifaceName, "addr", cidr, "err", err)
			}
		}
	}
}

// publishV6Expired publishes lease-expired events for all IA_NA addresses.
func (c *DHCPClient) publishV6Expired(msg *dhcpv6.Message) {
	const maxAddrs = 16
	count := 0
	for _, iana := range msg.Options.IANA() {
		for _, iaAddr := range iana.Options.Addresses() {
			if count >= maxAddrs {
				break
			}
			count++
			ip := iaAddr.IPv6Addr
			if ip == nil {
				continue
			}
			payload := DHCPPayload{
				Name:         c.ifaceName,
				Unit:         c.unit,
				Address:      ip.String(),
				PrefixLength: 128,
				LeaseTime:    0,
			}
			c.publishDHCP(TopicDHCPLeaseExpired, payload)
		}
	}
}

// v6Timers extracts T1, T2, and valid lifetime from IA_NA options.
// Falls back to reasonable defaults if not present.
func (c *DHCPClient) v6Timers(msg *dhcpv6.Message) (t1, t2, validLft time.Duration) {
	const (
		defaultT1       = 30 * time.Minute
		defaultT2       = 50 * time.Minute
		defaultValidLft = time.Hour
	)

	t1 = defaultT1
	t2 = defaultT2
	validLft = defaultValidLft

	ianas := msg.Options.IANA()
	if len(ianas) == 0 {
		return t1, t2, validLft
	}

	iana := ianas[0]
	if iana.T1 > 0 {
		t1 = iana.T1
	}
	if iana.T2 > 0 {
		t2 = iana.T2
	}
	for _, iaAddr := range iana.Options.Addresses() {
		if iaAddr.ValidLifetime > 0 {
			validLft = iaAddr.ValidLifetime
			break
		}
	}

	return t1, t2, validLft
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
