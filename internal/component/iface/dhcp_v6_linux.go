// Design: plan/spec-iface-0-umbrella.md — DHCPv6 client lifecycle
// Overview: dhcp_linux.go — DHCP client types and lifecycle

package iface

import (
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/nclient6"
	"github.com/vishvananda/netlink"
)

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
