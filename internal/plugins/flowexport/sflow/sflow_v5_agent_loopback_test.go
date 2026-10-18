// VALIDATES: sFlow v5 Section 4.3 SHOULD: a multi-homed agent is warned when its
// agent-address is not on a loopback interface, and an address held by no
// interface is warned on any router.
// PREVENTS: an agent address that changes as a routed interface goes down being
// accepted in silence.

package sflow

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// agentTestInterface builds one interface of the given kernel type holding
// the given addresses, the shape iface.ListInterfaces answers on Linux.
func agentTestInterface(name, kind string, addresses ...string) iface.InterfaceInfo {
	info := iface.InterfaceInfo{Name: name, Type: kind}
	for _, a := range addresses {
		family := "ipv4"
		if strings.Contains(a, ":") {
			family = "ipv6"
		}
		info.Addresses = append(info.Addresses, iface.AddrInfo{Address: a, Family: family})
	}
	return info
}

// multiHomedRouter is a router with two routed interfaces, the kernel
// loopback and one dummy interface used as a loopback.
func multiHomedRouter() []iface.InterfaceInfo {
	return []iface.InterfaceInfo{
		agentTestInterface("lo", "device", "127.0.0.1", "::1", "198.51.100.1"),
		agentTestInterface("agent0", "dummy", "198.51.100.2", "2001:db8::2"),
		agentTestInterface("eth0", "device", "203.0.113.1", "fe80::1"),
		agentTestInterface("eth1", "device", "192.0.2.5", "fe80::2"),
	}
}

// RFC requirement: SFLOW-V5-4.3-2 positive -- on a router with two routed interfaces, an agent address held by eth0 is classified multi-homed-not-loopback and the advice names the address, the interface and the loopback fix.
// RFC requirement: SFLOW-V5-4.3-2 negative -- on the same router, an agent address held by lo or by a dummy interface (also in another IPv6 spelling) is classified on-loopback and draws no advice.
func TestSFlowV5AgentAddressOnLoopbackWhenMultiHomed(t *testing.T) {
	// Goal: the multi-homed agent is told when its agent address is not a
	// loopback address, and is not told when it is.
	// Method: classify each address against a fixed interface table.
	ifs := multiHomedRouter()
	tests := []struct {
		name  string
		agent string
		want  agentPlacement
		owner string
	}{
		{"kernel loopback", "198.51.100.1", agentPlacementLoopback, "lo"},
		{"dummy loopback v4", "198.51.100.2", agentPlacementLoopback, "agent0"},
		{"dummy loopback v6 other spelling", "2001:DB8:0::2", agentPlacementLoopback, "agent0"},
		{"routed interface", "203.0.113.1", agentPlacementRouted, "eth0"},
		{"second routed interface", "192.0.2.5", agentPlacementRouted, "eth1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := netip.MustParseAddr(tt.agent)
			got := classifyAgentAddress(agent, ifs)
			if got.placement != tt.want {
				t.Fatalf("placement = %v, want %v", got.placement, tt.want)
			}
			if got.owner != tt.owner {
				t.Fatalf("owner = %q, want %q", got.owner, tt.owner)
			}
			advice := agentAddressAdvice(agent, got)
			if tt.want == agentPlacementLoopback {
				if len(advice) != 0 {
					t.Fatalf("advice for a loopback address: %q", advice)
				}
				return
			}
			if len(advice) != 1 {
				t.Fatalf("advice = %q, want one warning", advice)
			}
			for _, part := range []string{"agent-address " + tt.agent, tt.owner, "loopback", "create interface dummy"} {
				if !strings.Contains(advice[0], part) {
					t.Errorf("advice %q does not name %q", advice[0], part)
				}
			}
		})
	}
}

// RFC requirement: SFLOW-V5-4.3-2 negative -- a router with one routed interface (the second carries only a link-local address) is not multi-homed, so an agent address on that interface is classified single-homed and draws no advice.
func TestSFlowV5AgentAddressSingleHomedNoAdvice(t *testing.T) {
	// Goal: the SHOULD binds a multi-homed agent only.
	// Method: one routed interface, a link-local-only interface, and lo.
	ifs := []iface.InterfaceInfo{
		agentTestInterface("lo", "device", "127.0.0.1"),
		agentTestInterface("eth0", "device", "203.0.113.1", "fe80::1"),
		agentTestInterface("eth1", "device", "fe80::2"),
	}
	agent := netip.MustParseAddr("203.0.113.1")
	got := classifyAgentAddress(agent, ifs)
	if got.placement != agentPlacementSingleHomed {
		t.Fatalf("placement = %v, want single-homed", got.placement)
	}
	if advice := agentAddressAdvice(agent, got); len(advice) != 0 {
		t.Fatalf("advice for a single-homed agent: %q", advice)
	}
}

// TestSFlowAgentAddressAbsentIsAdvised checks that an agent address no
// interface holds draws a warning on any router, single-homed or not.
func TestSFlowAgentAddressAbsentIsAdvised(t *testing.T) {
	// Goal: an address the router does not hold cannot identify it.
	// Method: classify an address no interface of either table carries.
	agent := netip.MustParseAddr("192.0.2.99")
	tables := map[string][]iface.InterfaceInfo{
		"multi-homed":  multiHomedRouter(),
		"single-homed": {agentTestInterface("eth0", "device", "203.0.113.1")},
	}
	for name, ifs := range tables {
		got := classifyAgentAddress(agent, ifs)
		if got.placement != agentPlacementAbsent {
			t.Fatalf("%s: placement = %v, want absent", name, got.placement)
		}
		advice := agentAddressAdvice(agent, got)
		if len(advice) != 1 {
			t.Fatalf("%s: advice = %q, want one warning", name, advice)
		}
		if !strings.Contains(advice[0], "not configured on any interface") {
			t.Errorf("%s: advice %q does not say the address is on no interface", name, advice[0])
		}
	}
}
