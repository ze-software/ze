// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Overview: dhcp_linux.go -- DHCP client types and lifecycle

//go:build linux

package ifacedhcp

import (
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// runV4 is the long-lived DHCPv4 worker. It performs DORA (Discover-Offer-
// Request-Ack), installs the leased address, and renews at T1/T2 intervals.
func (c *DHCPClient) runV4() {
	logger := loggerPtr.Load()

	for !c.stopped() {
		binding, err := iface.Resolve(c.ifaceName)
		if err != nil {
			logger.Warn("iface dhcp v4: interface lookup failed",
				"iface", c.ifaceName, "err", err)
			if !c.sleepOrStop(5 * time.Second) {
				return
			}
			continue
		}

		// Bind the DHCP client socket to the resolved kernel device, so a
		// logical name that differs from its os device (os-name / mac-match
		// selectors) still binds to the right interface.
		client, err := nclient4.New(binding.OsName)
		if err != nil {
			logger.Warn("iface dhcp v4: client creation failed",
				"iface", c.ifaceName, "err", err)
			if !c.sleepOrStop(5 * time.Second) {
				return
			}
			continue
		}

		ctx, ctxCancel := c.stoppableContext()
		lease, err := client.Request(ctx, c.v4RequestModifiers()...)
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
		if leaseTime >= v4LeaseInfinite {
			c.holdInfiniteV4(ack)
			return
		}
		t1, t2 := v4RenewalTimes(leaseTime)

		if !c.sleepOrStop(t1) {
			c.removeV4Addr(ack)
			return
		}

		newLease, renewed := c.renewV4(lease)
		if renewed {
			lease = newLease
			ack = lease.ACK
			leaseTime = c.v4LeaseTime(ack)
			if leaseTime >= v4LeaseInfinite {
				c.holdInfiniteV4(ack)
				return
			}
			t1, t2 = v4RenewalTimes(leaseTime)
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
				if leaseTime >= v4LeaseInfinite {
					c.holdInfiniteV4(ack)
					return
				}
				_, t2 = v4RenewalTimes(leaseTime)
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

// holdInfiniteV4 keeps an infinite lease until the client stops, then
// removes it.
//
// RFC 2131 Section 3.3: "A client acquires a lease for a network address for a
// fixed period of time (which may be infinite)." An infinite lease never runs
// out, so there is no T1, no T2 and no expiry to wait for.
func (c *DHCPClient) holdInfiniteV4(ack *dhcpv4.DHCPv4) {
	<-c.stop
	c.removeV4Addr(ack)
}

// v4RenewalTimes returns T1 and T2 for a finite lease.
//
// RFC 2131 Section 4.4.5: "T1 defaults to (0.5 * duration_of_lease).  T2
// defaults to (0.875 * duration_of_lease)." T2 is computed as the lease less
// an eighth, never as seven eighths of it: leaseTime * 7 overflows a
// time.Duration for a lease above 41 years.
func v4RenewalTimes(leaseTime time.Duration) (t1, t2 time.Duration) {
	return leaseTime / 2, leaseTime - leaseTime/8
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
	// RFC 2131 Section 2: the renewal copies the ACK's flags word, so its
	// reserved bits are cleared here as for acquisition.
	renewed, err := client.Renew(ctx, lease, clearReservedFlags)
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

	var bCidr textbuf.Buffer
	cidr := bCidr.Reset().Str(ip.String()).Byte('/').Int(int64(ones)).String()
	leaseTime := c.v4LeaseTime(ack)
	lifetime := v4AddressLifetime(leaseTime)

	// Registered before it is installed, so the address reconcile never sees
	// the lease unregistered (iface.RegisterLeasedAddresses).
	if err := c.claimV4Address(cidr); err != nil {
		logger.Warn("iface dhcp v4: lease not installed, the address is not this client's to hold",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
		return
	}
	// The lifetime is the lease, refreshed on every renewal: the kernel
	// removes the address when the lease runs out unless the client renewed.
	if err := iface.ReplaceAddressWithLifetime(c.ifaceName, cidr, lifetime, lifetime); err != nil {
		logger.Warn("iface dhcp v4: addr replace failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
		c.settleV4Address(c.v4Address)
		return
	}
	if c.v4Address != "" && c.v4Address != cidr {
		// The server granted another address: the old one is no longer leased.
		c.removeV4Address(c.v4Address)
	}
	c.v4Address = cidr
	c.settleV4Address(cidr)

	payload := c.v4Payload(ack)

	// Install default route from DHCP Router option (RFC 2132 Section 3.5).
	if payload.Router != "" {
		if err := iface.AddRoute(c.ifaceName, "0.0.0.0/0", payload.Router, c.config.RouteMetric, rtproto.Iface); err != nil {
			logger.Warn("iface dhcp v4: route install failed",
				"iface", c.ifaceName, "gw", payload.Router, "err", err)
		}
	}

	// Write DNS servers from DHCP to resolv.conf (RFC 2132 Section 3.8).
	// Static name-servers (system { name-server }) take priority.
	if len(payload.DNSAll) > 0 && c.config.ResolvConfPath != "" && !c.config.HasStaticNameServers {
		if err := writeResolvConfTo(c.config.ResolvConfPath, payload.DNSAll); err != nil {
			logger.Warn("iface dhcp v4: resolv.conf write failed",
				"iface", c.ifaceName, "path", c.config.ResolvConfPath, "err", err)
		}
	}

	c.publishDHCP(topic, payload)

	logger.Info("iface dhcp v4: lease obtained",
		"iface", c.ifaceName, "addr", cidr, "lease", leaseTime)
}

func (c *DHCPClient) removeV4Addr(ack *dhcpv4.DHCPv4) {
	logger := loggerPtr.Load()

	// Removed before it is unregistered, so the address reconcile never sees
	// the lease unregistered (iface.RegisterLeasedAddresses).
	if c.v4Address != "" {
		c.removeV4Address(c.v4Address)
		c.v4Address = ""
	}
	c.settleV4Address("")

	// Remove default route installed from this lease.
	routerIP := dhcpv4.GetIP(dhcpv4.OptionRouter, ack.Options)
	if routerIP != nil {
		if err := iface.RemoveRoute(c.ifaceName, "0.0.0.0/0", routerIP.String(), c.config.RouteMetric, rtproto.Iface); err != nil {
			logger.Debug("iface dhcp v4: route removal failed",
				"iface", c.ifaceName, "gw", routerIP.String(), "err", err)
		}
	}

	// resolv.conf is NOT cleared on individual lease expiry. When multiple
	// interfaces have DHCP enabled, each writes DNS on acquire/renew (last
	// writer wins). Clearing here would remove DNS that another active
	// lease provided. Stale DNS is better than no DNS; the next lease
	// acquisition overwrites with fresh servers.
}

// claimV4Address registers cidr as this client's lease. The address it
// replaces, when it differs, stays registered until it is removed.
func (c *DHCPClient) claimV4Address(cidr string) error {
	addrs := []string{cidr}
	if c.v4Address != "" && c.v4Address != cidr {
		addrs = append(addrs, c.v4Address)
	}
	return c.registerV4Addresses(addrs)
}

// settleV4Address makes the registration name exactly cidr, the address the
// client holds, or nothing when cidr is empty.
func (c *DHCPClient) settleV4Address(cidr string) {
	if cidr == "" {
		iface.UnregisterLeasedAddresses(c.v4Owner)
		return
	}
	if err := c.registerV4Addresses([]string{cidr}); err != nil {
		loggerPtr.Load().Warn("iface dhcp v4: lease registration failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
	}
}

// registerV4Addresses records addrs as this client's leases on the kernel
// device the logical interface name resolves to, the key the reconcile uses.
func (c *DHCPClient) registerV4Addresses(addrs []string) error {
	osName, err := iface.ResolveDevice(c.ifaceName)
	if err != nil {
		return err
	}
	return iface.RegisterLeasedAddresses(osName, c.v4Owner, addrs)
}

// removeV4Address removes one leased address from the kernel.
func (c *DHCPClient) removeV4Address(cidr string) {
	if err := iface.RemoveAddress(c.ifaceName, cidr); err != nil {
		loggerPtr.Load().Debug("iface dhcp v4: addr removal failed",
			"iface", c.ifaceName, "cidr", cidr, "err", err)
	}
}

// v4LeaseInfinite is the lease time RFC 2131 reserves for a lease that never
// runs out, as v4LeaseTime decodes it.
//
// RFC 2131 Section 3.3: "The time value of 0xffffffff is reserved to
// represent "infinity"."
const v4LeaseInfinite = 0xffffffff * time.Second

// kernelLifetimeInfinite is the address lifetime the kernel holds as forever
// (INFINITY_LIFE_TIME): an address installed with it is permanent.
const kernelLifetimeInfinite = 0xffffffff

// v4AddressLifetime is the valid and preferred lifetime, in seconds, the
// leased address is installed with: the lease itself.
//
// RFC 2131 Section 3.3: "A client acquires a lease for a network address for a
// fixed period of time (which may be infinite)."
func v4AddressLifetime(leaseTime time.Duration) int {
	if leaseTime >= v4LeaseInfinite {
		return kernelLifetimeInfinite
	}
	return int(leaseTime / time.Second)
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

	dnsAll := make([]string, 0, len(dnsServers))
	for _, s := range dnsServers {
		dnsAll = append(dnsAll, s.String())
	}

	ntpIPs := dhcpv4.GetIPs(dhcpv4.OptionNTPServers, ack.Options)
	ntpServers := make([]string, 0, len(ntpIPs))
	for _, s := range ntpIPs {
		ntpServers = append(ntpServers, s.String())
	}

	return iface.DHCPPayload{
		Name:         c.ifaceName,
		Unit:         c.unit,
		Address:      ip.String(),
		PrefixLength: ones,
		Router:       router,
		DNS:          dns,
		DNSAll:       dnsAll,
		NTPServers:   ntpServers,
		LeaseTime:    int(c.v4LeaseTime(ack).Seconds()),
	}
}

// flagsBroadcast is bit 0, the most significant, of the flags field: the only
// bit RFC 2131 Section 2 defines.
const flagsBroadcast = 0x8000

// clearReservedFlags zeroes flags bits 1 to 15 of a message the client sends.
// The vendored dhcpv4.NewRequestFromOffer and NewRenewFromAck fill the flags
// word from the server's OFFER or ACK (WithReply), so a server that sets a
// reserved bit would otherwise have it sent back. nclient4 applies the caller's
// modifiers after its own, so this runs last.
func clearReservedFlags(d *dhcpv4.DHCPv4) {
	// RFC 2131 Section 2: "The remaining bits of the flags field are reserved
	// for future use. They MUST be set to zero by clients and ignored by
	// servers and relay agents."
	d.Flags &= flagsBroadcast
}

// v4RequestModifiers builds dhcpv4 packet modifiers from the client config.
// Adds hostname (option 12) and client-id (option 61) when configured, and
// always clears the reserved flags bits.
// RFC 2132 Section 3.14 (hostname), Section 9.14 (client-id).
func (c *DHCPClient) v4RequestModifiers() []dhcpv4.Modifier {
	// RFC 2131 Section 2
	mods := []dhcpv4.Modifier{clearReservedFlags}
	if c.config.Hostname != "" {
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptHostName(c.config.Hostname)))
	}
	if c.config.ClientID != "" {
		mods = append(mods, dhcpv4.WithOption(dhcpv4.OptClientIdentifier([]byte(c.config.ClientID))))
	}
	return mods
}
