// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Overview: dhcp_linux.go -- DHCP client types and lifecycle

//go:build linux

package ifacedhcp

import (
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
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
		c.handleV4Lease(ack, iface.TopicDHCPLeaseAcquired)

		leaseTime := c.v4LeaseTime(ack)
		t1 := leaseTime / 2
		t2 := leaseTime * 7 / 8

		if !c.sleepOrStop(t1) {
			c.removeV4Addr(ack)
			return
		}

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

		remainingLease := leaseTime - t2
		if remainingLease > 0 {
			if !c.sleepOrStop(remainingLease) {
				c.removeV4Addr(ack)
				return
			}
		}

		c.removeV4Addr(ack)
		c.publishDHCP(iface.TopicDHCPLeaseExpired, c.v4Payload(ack))
	}
}

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

	c.handleV4Lease(renewed.ACK, iface.TopicDHCPLeaseRenewed)
	return renewed, true
}

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
	lftSec := int(leaseTime.Seconds())

	if err := iface.ReplaceAddressWithLifetime(c.ifaceName, cidr, lftSec, lftSec); err != nil {
		logger.Warn("iface dhcp v4: addr replace failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
		return
	}

	payload := c.v4Payload(ack)
	c.publishDHCP(topic, payload)

	logger.Info("iface dhcp v4: lease obtained",
		"iface", c.ifaceName, "addr", cidr, "lease", leaseTime)
}

func (c *DHCPClient) removeV4Addr(ack *dhcpv4.DHCPv4) {
	ip := ack.YourIPAddr
	mask := ack.SubnetMask()
	ones, _ := mask.Size()
	if ones == 0 {
		ones = 24
	}

	cidr := fmt.Sprintf("%s/%d", ip.String(), ones)

	if err := iface.RemoveAddress(c.ifaceName, cidr); err != nil {
		loggerPtr.Load().Debug("iface dhcp v4: addr removal failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
	}
}

func (c *DHCPClient) v4LeaseTime(ack *dhcpv4.DHCPv4) time.Duration {
	leaseTime := ack.IPAddressLeaseTime(time.Hour)
	if leaseTime <= 0 {
		leaseTime = time.Hour
	}
	return leaseTime
}

func (c *DHCPClient) v4Payload(ack *dhcpv4.DHCPv4) iface.DHCPPayload {
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

	return iface.DHCPPayload{
		Name:         c.ifaceName,
		Unit:         c.unit,
		Address:      ip.String(),
		PrefixLength: ones,
		Router:       router,
		DNS:          dns,
		LeaseTime:    int(c.v4LeaseTime(ack).Seconds()),
	}
}
