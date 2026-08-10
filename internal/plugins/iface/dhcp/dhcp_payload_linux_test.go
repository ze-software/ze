// VALIDATES: the DHCP option-to-payload translators — v4Payload (subnet-mask ->
// prefix length with /24 fallback, router, full ordered DNS list, NTP servers,
// lease time), v4LeaseTime default, v6Timers (defaults vs IANA-supplied T1/T2 and
// address ValidLifetime), publishV6Expired (one /128 expiry event per IA_NA
// address, 16-address cap), and publishDHCP name/unit backfill + unknown-topic
// no-emit.
// PREVENTS: a lease being installed with the wrong prefix length, DNS/NTP servers
// dropped or reordered, expiry events missing addresses, or an event leaking with
// an empty interface name.

//go:build linux

package ifacedhcp

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"

	"github.com/ze-software/ze/internal/component/iface"
)

// leases decodes every payload the bus recorded into DHCPPayload values.
func (b *recordingBus) leases(t *testing.T) []iface.DHCPPayload {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]iface.DHCPPayload, 0, len(b.events))
	for _, ev := range b.events {
		var p iface.DHCPPayload
		if err := json.Unmarshal([]byte(ev.payload), &p); err != nil {
			t.Fatalf("unmarshal %q: %v", ev.payload, err)
		}
		out = append(out, p)
	}
	return out
}

func newTestClient(t *testing.T, bus *recordingBus, v4, v6 bool) *DHCPClient {
	t.Helper()
	c, err := newDHCPClient("eth0", "0", bus, v4, v6, dHCPConfig{})
	if err != nil {
		t.Fatalf("NewDHCPClient: %v", err)
	}
	return c
}

func TestV4Payload(t *testing.T) {
	c := newTestClient(t, &recordingBus{}, true, false)

	ack, err := dhcpv4.New()
	if err != nil {
		t.Fatalf("dhcpv4.New: %v", err)
	}
	ack.YourIPAddr = net.ParseIP("192.0.2.50")
	ack.UpdateOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32)))
	ack.UpdateOption(dhcpv4.OptRouter(net.ParseIP("192.0.2.1")))
	ack.UpdateOption(dhcpv4.OptDNS(net.ParseIP("192.0.2.53"), net.ParseIP("192.0.2.54")))
	ack.UpdateOption(dhcpv4.OptNTPServers(net.ParseIP("192.0.2.123")))
	ack.UpdateOption(dhcpv4.OptIPAddressLeaseTime(2 * time.Hour))

	p := c.v4Payload(ack)
	if p.Name != "eth0" || p.Unit != "0" {
		t.Errorf("identity = %q/%q, want eth0/0", p.Name, p.Unit)
	}
	if p.Address != "192.0.2.50" {
		t.Errorf("Address = %q, want 192.0.2.50", p.Address)
	}
	if p.PrefixLength != 24 {
		t.Errorf("PrefixLength = %d, want 24", p.PrefixLength)
	}
	if p.Router != "192.0.2.1" {
		t.Errorf("Router = %q, want 192.0.2.1", p.Router)
	}
	if p.DNS != "192.0.2.53" {
		t.Errorf("DNS = %q, want 192.0.2.53 (first)", p.DNS)
	}
	if len(p.DNSAll) != 2 || p.DNSAll[0] != "192.0.2.53" || p.DNSAll[1] != "192.0.2.54" {
		t.Errorf("DNSAll = %v, want [192.0.2.53 192.0.2.54] in order", p.DNSAll)
	}
	if len(p.NTPServers) != 1 || p.NTPServers[0] != "192.0.2.123" {
		t.Errorf("NTPServers = %v, want [192.0.2.123]", p.NTPServers)
	}
	if p.LeaseTime != int((2 * time.Hour).Seconds()) {
		t.Errorf("LeaseTime = %d, want %d", p.LeaseTime, int((2 * time.Hour).Seconds()))
	}
}

func TestV4PayloadPrefixFallback(t *testing.T) {
	c := newTestClient(t, &recordingBus{}, true, false)

	ack, err := dhcpv4.New()
	if err != nil {
		t.Fatalf("dhcpv4.New: %v", err)
	}
	ack.YourIPAddr = net.ParseIP("10.0.0.7")
	// No subnet mask option: prefix length must fall back to /24.

	p := c.v4Payload(ack)
	if p.PrefixLength != 24 {
		t.Errorf("PrefixLength = %d, want 24 fallback when mask absent", p.PrefixLength)
	}
}

func TestV4LeaseTime(t *testing.T) {
	c := newTestClient(t, &recordingBus{}, true, false)

	withLease, err := dhcpv4.New()
	if err != nil {
		t.Fatalf("dhcpv4.New: %v", err)
	}
	withLease.UpdateOption(dhcpv4.OptIPAddressLeaseTime(7200 * time.Second))
	if got := c.v4LeaseTime(withLease); got != 2*time.Hour {
		t.Errorf("v4LeaseTime(7200s) = %v, want 2h", got)
	}

	noLease, err := dhcpv4.New()
	if err != nil {
		t.Fatalf("dhcpv4.New: %v", err)
	}
	if got := c.v4LeaseTime(noLease); got != time.Hour {
		t.Errorf("v4LeaseTime(absent) = %v, want 1h default", got)
	}
}

func TestV6Timers(t *testing.T) {
	c := newTestClient(t, &recordingBus{}, false, true)

	// No IANA: all defaults.
	empty := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeReply}
	t1, t2, valid := c.v6Timers(empty)
	if t1 != 30*time.Minute || t2 != 50*time.Minute || valid != time.Hour {
		t.Errorf("defaults = %v/%v/%v, want 30m/50m/1h", t1, t2, valid)
	}

	// IANA with T1/T2 and an address ValidLifetime overrides the defaults.
	addr := &dhcpv6.OptIAAddress{IPv6Addr: net.ParseIP("2001:db8::1"), ValidLifetime: 900 * time.Second}
	iana := &dhcpv6.OptIANA{IaId: [4]byte{1, 2, 3, 4}, T1: 100 * time.Second, T2: 200 * time.Second}
	iana.Options.Add(addr)
	msg := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeReply}
	msg.AddOption(iana)

	t1, t2, valid = c.v6Timers(msg)
	if t1 != 100*time.Second || t2 != 200*time.Second || valid != 900*time.Second {
		t.Errorf("IANA timers = %v/%v/%v, want 100s/200s/900s", t1, t2, valid)
	}
}

func TestPublishV6Expired(t *testing.T) {
	bus := &recordingBus{}
	c := newTestClient(t, bus, false, true)

	addr1 := &dhcpv6.OptIAAddress{IPv6Addr: net.ParseIP("2001:db8::1")}
	addr2 := &dhcpv6.OptIAAddress{IPv6Addr: net.ParseIP("2001:db8::2")}
	iana := &dhcpv6.OptIANA{IaId: [4]byte{1, 2, 3, 4}}
	iana.Options.Add(addr1)
	iana.Options.Add(addr2)
	msg := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeReply}
	msg.AddOption(iana)

	c.publishV6Expired(msg)

	events := bus.leases(t)
	if len(events) != 2 {
		t.Fatalf("expired events = %d, want 2", len(events))
	}
	for _, lease := range events {
		if lease.PrefixLength != 128 {
			t.Errorf("PrefixLength = %d, want 128", lease.PrefixLength)
		}
		if lease.LeaseTime != 0 {
			t.Errorf("LeaseTime = %d, want 0 for expiry", lease.LeaseTime)
		}
	}
}

func TestPublishV6ExpiredCap(t *testing.T) {
	bus := &recordingBus{}
	c := newTestClient(t, bus, false, true)

	iana := &dhcpv6.OptIANA{IaId: [4]byte{1, 2, 3, 4}}
	for range 20 {
		iana.Options.Add(&dhcpv6.OptIAAddress{IPv6Addr: net.ParseIP("2001:db8::").To16()})
	}
	msg := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeReply}
	msg.AddOption(iana)

	c.publishV6Expired(msg)

	if got := len(bus.leases(t)); got != 16 {
		t.Errorf("expired events = %d, want 16 (cap)", got)
	}
}

func TestPublishDHCPBackfillAndUnknownTopic(t *testing.T) {
	bus := &recordingBus{}
	c := newTestClient(t, bus, true, false)

	// Known topic with empty name/unit: publishDHCP must backfill from the client.
	c.publishDHCP(iface.TopicDHCPLeaseAcquired, iface.DHCPPayload{Address: "192.0.2.9"})
	acquired := bus.leases(t)
	// Filter for the acquired payload we just emitted.
	var backfilled *iface.DHCPPayload
	for i := range acquired {
		if acquired[i].Address == "192.0.2.9" {
			backfilled = &acquired[i]
		}
	}
	if backfilled == nil {
		t.Fatal("no event emitted for a known topic")
	}
	if backfilled.Name != "eth0" || backfilled.Unit != "0" {
		t.Errorf("backfill = %q/%q, want eth0/0", backfilled.Name, backfilled.Unit)
	}

	// Unknown topic: nothing emitted.
	before := len(bus.events)
	c.publishDHCP("not-a-real-topic", iface.DHCPPayload{Address: "203.0.113.1"})
	if len(bus.events) != before {
		t.Error("unknown topic should not emit an event")
	}
}
