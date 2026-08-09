//go:build linux

// Design: docs/features/interfaces.md AC-6 -- DHCPv6-PD lease flow. This is
// the native lease-flow test: a synthetic DHCPv6 Reply carrying an IA_PD prefix
// drives handleV6Reply (the producing function for the lease event), so the
// prefix-delegation lease flow is exercised in CI without a DHCPv6 server or
// QEMU. A full server-in-the-loop QEMU run is described in the spec runbook.

package ifacedhcp

import (
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

// recordingBus records every DHCP lease event emitted by the client.
type recordingBus struct {
	mu     sync.Mutex
	events []struct {
		eventType string
		payload   string
	}
}

func (b *recordingBus) Emit(_, eventType string, payload any) (int, error) {
	b.mu.Lock()
	s, _ := payload.(string)
	b.events = append(b.events, struct {
		eventType string
		payload   string
	}{eventType, s})
	b.mu.Unlock()
	return 0, nil
}

func (b *recordingBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

// VALIDATES: AC-6 -- a DHCPv6 Reply carrying an IA_PD prefix produces a lease
// event with the delegated prefix, its length, and the valid lifetime.
func TestDHCPv6PDLeaseFlow(t *testing.T) {
	bus := &recordingBus{}
	c, err := NewDHCPClient("eth0", "", bus, false, true, DHCPConfig{})
	if err != nil {
		t.Fatalf("NewDHCPClient: %v", err)
	}

	prefix := &dhcpv6.OptIAPrefix{
		PreferredLifetime: 1800 * time.Second,
		ValidLifetime:     3600 * time.Second,
		Prefix:            &net.IPNet{IP: net.ParseIP("2001:db8:dddd::"), Mask: net.CIDRMask(56, 128)},
	}
	iapd := &dhcpv6.OptIAPD{IaId: [4]byte{1, 2, 3, 4}}
	iapd.Options.Add(prefix)
	msg := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeReply}
	msg.AddOption(iapd)

	c.handleV6Reply(msg, iface.TopicDHCPLeaseAcquired)

	var payload string
	found := 0
	bus.mu.Lock()
	for _, ev := range bus.events {
		if ev.eventType == ifaceevents.EventDHCPAcquired {
			payload = ev.payload
			found++
		}
	}
	bus.mu.Unlock()

	if found != 1 {
		t.Fatalf("expected 1 DHCP-acquired lease event for the IA_PD prefix, got %d", found)
	}
	var lease iface.DHCPPayload
	if err := json.Unmarshal([]byte(payload), &lease); err != nil {
		t.Fatalf("unmarshal lease payload %q: %v", payload, err)
	}
	if lease.Address != "2001:db8:dddd::" {
		t.Errorf("lease address = %q, want 2001:db8:dddd::", lease.Address)
	}
	if lease.PrefixLength != 56 {
		t.Errorf("lease prefix-length = %d, want 56", lease.PrefixLength)
	}
	if lease.LeaseTime != 3600 {
		t.Errorf("lease time = %d, want 3600", lease.LeaseTime)
	}
}
