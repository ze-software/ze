// Design: docs/guide/pppoe.md -- IPv6 on the PPPoE client
// Related: pppoe_client_ipv6.go -- the parser and the link-local move under test

package iface

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

// TestPPPoEClientIPv6Parse checks the ipv6 container of a pppoe-client: absent
// is the zero value, enabled carries the sysctls and the DHCPv6 block, and
// leaves without enabled are refused with the fix.
func TestPPPoEClientIPv6Parse(t *testing.T) {
	got, err := parsePPPoEClientIPv6(map[string]any{})
	if err != nil {
		t.Fatalf("absent: %v", err)
	}
	if got != (PPPoEClientIPv6{}) {
		t.Fatalf("absent container gave %+v, want the zero value", got)
	}

	got, err = parsePPPoEClientIPv6(map[string]any{"ipv6": map[string]any{
		"enabled": "true", "accept-ra": "2", "forwarding": "true", "autoconf": "true",
		"dhcpv6": map[string]any{"enabled": "true", "pd": map[string]any{"length": "56"}},
	}})
	if err != nil {
		t.Fatalf("enabled: %v", err)
	}
	if !got.Enabled {
		t.Fatal("enabled true parsed as disabled")
	}
	want := [pppoeClientSysctlMax]pppoeClientSysctl{
		{Suffix: "autoconf", Value: "1"}, {Suffix: "accept_ra", Value: "2"}, {Suffix: "forwarding", Value: "1"},
	}
	if got.Sysctls != want {
		t.Fatalf("sysctls %+v, want %+v", got.Sysctls, want)
	}
	if got.AcceptRA != 2 || !got.acceptsRA() {
		t.Fatalf("accept-ra parsed as %d (acceptsRA %v), want 2 (true)", got.AcceptRA, got.acceptsRA())
	}
	if !got.DHCPv6.Enabled || !got.DHCPv6.PDRequested || got.DHCPv6.PDLength != 56 {
		t.Fatalf("dhcpv6 %+v, want enabled with pd length 56", got.DHCPv6)
	}

	if _, err := parsePPPoEClientIPv6(map[string]any{"ipv6": map[string]any{"accept-ra": "2"}}); err == nil {
		t.Fatal("ipv6 leaves without enabled true were accepted")
	}
	if _, err := parsePPPoEClientIPv6(map[string]any{"ipv6": map[string]any{"enabled": "true", "accept-ra": "3"}}); err == nil {
		t.Fatal("accept-ra 3 was accepted")
	}
}

// TestPPPoEClientDHCPv6RunsOnSessionDevice checks AC-6's wiring: a
// pppoe-client with ipv6 dhcpv6 enabled gets a DHCPv6-only client on its own
// name, with the prefix-delegation hint and the source interface as the
// DUID device; a client without ipv6 gets none.
func TestPPPoEClientDHCPv6RunsOnSessionDevice(t *testing.T) {
	type call struct {
		v4, v6, pd bool
		pdLength   int
		duidDevice string
	}
	calls := map[string]call{}
	previous := dhcpClientFactory
	SetDHCPClientFactory(func(ifaceName, unit string, _ ze.EventBus, v4, v6 bool, _, _ string, pd bool, pdLength int, _, duidDevice string, _ int) (DHCPStopper, error) {
		calls[ifaceName+"/"+unit] = call{v4: v4, v6: v6, pd: pd, pdLength: pdLength, duidDevice: duidDevice}
		return noopStopper{}, nil
	})
	t.Cleanup(func() { dhcpClientFactory = previous })

	cfg := &ifaceConfig{PPPoEClient: []pppoeClientEntry{
		{Name: "pppoe0", SourceInterface: "eth1", IPv6: PPPoEClientIPv6{Enabled: true,
			DHCPv6: dhcpv6UnitConfig{Enabled: true, PDRequested: true, PDLength: 56}}},
		{Name: "pppoe1", SourceInterface: "eth2"},
	}}
	reconcileDHCP(cfg, nil, map[dhcpUnitKey]dhcpEntry{}, slogutil.DiscardLogger())

	got, ok := calls["pppoe0/"+pppoeClientDHCPUnit]
	if !ok {
		t.Fatalf("no DHCPv6 client started on pppoe0; calls %+v", calls)
	}
	if got != (call{v6: true, pd: true, pdLength: 56, duidDevice: "eth1"}) {
		t.Fatalf("pppoe0 client %+v, want DHCPv6 only, pd 56, DUID from eth1", got)
	}
	if len(calls) != 1 {
		t.Fatalf("calls %+v, want only pppoe0", calls)
	}
}

// linkLocalBackend records the address calls applyPPPoELinkLocal makes.
type linkLocalBackend struct {
	Backend
	added, removed []string
}

func (b *linkLocalBackend) AddAddress(_, cidr string) error {
	b.added = append(b.added, cidr)
	return nil
}

func (b *linkLocalBackend) RemoveAddress(_, cidr string) error {
	b.removed = append(b.removed, cidr)
	return nil
}

// TestRFC5072LinkLocalInstalledOnSessionDevice checks AC-1's install step:
// the link-local IPv6CP publishes is added as a /64 on the device, replaced
// when it changes and removed when IPv6CP closes.
func TestRFC5072LinkLocalInstalledOnSessionDevice(t *testing.T) {
	b := &linkLocalBackend{}
	first := netip.MustParseAddr("fe80::5054:ff:fe12:3456")
	second := netip.MustParseAddr("fe80::77")

	applied, err := applyPPPoELinkLocal(b, "pppoe0", netip.Addr{}, first)
	if err != nil || applied != first {
		t.Fatalf("install: applied %v err %v", applied, err)
	}
	if applied, err = applyPPPoELinkLocal(b, "pppoe0", applied, second); err != nil || applied != second {
		t.Fatalf("replace: applied %v err %v", applied, err)
	}
	if applied, err = applyPPPoELinkLocal(b, "pppoe0", applied, netip.Addr{}); err != nil || applied.IsValid() {
		t.Fatalf("withdraw: applied %v err %v", applied, err)
	}
	wantAdded := []string{"fe80::5054:ff:fe12:3456/64", "fe80::77/64"}
	wantRemoved := []string{"fe80::5054:ff:fe12:3456/64", "fe80::77/64"}
	if len(b.added) != 2 || b.added[0] != wantAdded[0] || b.added[1] != wantAdded[1] {
		t.Fatalf("added %v, want %v", b.added, wantAdded)
	}
	if len(b.removed) != 2 || b.removed[0] != wantRemoved[0] || b.removed[1] != wantRemoved[1] {
		t.Fatalf("removed %v, want %v", b.removed, wantRemoved)
	}
}
