// Design: plan/spec-iface-0-umbrella.md — DHCPv4 client lifecycle
// Overview: dhcp_linux.go — DHCP client types and lifecycle

package iface

import (
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/vishvananda/netlink"
)

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
