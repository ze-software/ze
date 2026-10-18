// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- sFlow agent address advice
// Related: register.go -- validateSFlowCollectors refuses an agent address that cannot identify the agent

package sflow

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// agentPlacement says where the router holds the sFlow agent address.
type agentPlacement uint8

const (
	agentPlacementUnspecified agentPlacement = iota
	// agentPlacementLoopback: a loopback or dummy interface holds it.
	agentPlacementLoopback
	// agentPlacementSingleHomed: a routed interface holds it, and the router
	// has only one routed interface.
	agentPlacementSingleHomed
	// agentPlacementRouted: a routed interface holds it, and the router has
	// more than one routed interface.
	agentPlacementRouted
	// agentPlacementAbsent: no interface holds it.
	agentPlacementAbsent
)

// String names the placement for test failures.
func (p agentPlacement) String() string {
	switch p {
	case agentPlacementUnspecified:
		return "unspecified"
	case agentPlacementLoopback:
		return "loopback"
	case agentPlacementSingleHomed:
		return "single-homed"
	case agentPlacementRouted:
		return "routed"
	case agentPlacementAbsent:
		return "absent"
	}
	return "invalid"
}

// agentLocation is the classification of one agent address: where it sits,
// and the name of the interface holding it (empty when absent).
type agentLocation struct {
	placement agentPlacement
	owner     string
}

// adviseSFlowCollectors answers the warnings the sflow collectors owe the
// operator. validateSFlowCollectors already refused a config whose collectors
// name a missing or different agent address, so the first collector's address
// is the agent's. An empty answer means nothing to report.
func adviseSFlowCollectors(collectors []flowexport.CollectorConfig) []string {
	agent := agentAddress(collectors[0])
	ifs, err := iface.ListInterfaces()
	if err != nil {
		return []string{"sflow agent-address " + agent.String() +
			" was not checked against the router's interfaces, because the interface list is unavailable (" +
			err.Error() + "). Collectors identify this router by that address, so it should be held by a loopback interface." +
			" Check it with: show interface"}
	}
	return agentAddressAdvice(agent, classifyAgentAddress(agent, ifs))
}

// sFlow v5 Section 4.3: "The IP address associated with this agent. In the
// case of a multi-homed agent, this should be the loopback address of the
// agent."
//
// classifyAgentAddress finds the interface holding the agent address and
// counts the router's routed interfaces. A routed interface is one that is
// neither a loopback nor a dummy and holds an address other than a
// link-local one; more than one makes the router multi-homed. A dummy
// interface counts as a loopback, because on Linux it is the interface that
// carries a router's stable loopback addresses besides lo itself.
func classifyAgentAddress(agent netip.Addr, ifs []iface.InterfaceInfo) agentLocation {
	agent = agent.Unmap()
	owner := -1
	routed := 0
	for i := range ifs {
		holds, global := scanInterfaceAddresses(agent, &ifs[i])
		if holds {
			owner = i
		}
		if isLoopbackInterface(&ifs[i]) {
			continue
		}
		if global {
			routed++
		}
	}
	if owner < 0 {
		return agentLocation{placement: agentPlacementAbsent}
	}
	name := ifs[owner].Name
	if isLoopbackInterface(&ifs[owner]) {
		return agentLocation{placement: agentPlacementLoopback, owner: name}
	}
	if routed > 1 {
		return agentLocation{placement: agentPlacementRouted, owner: name}
	}
	return agentLocation{placement: agentPlacementSingleHomed, owner: name}
}

// scanInterfaceAddresses reports whether the interface holds the agent
// address, and whether it holds any address that is not link-local.
// An address the backend reports that does not parse holds nothing.
func scanInterfaceAddresses(agent netip.Addr, info *iface.InterfaceInfo) (holds, global bool) {
	for i := range info.Addresses {
		addr, err := netip.ParseAddr(info.Addresses[i].Address)
		if err != nil {
			continue
		}
		addr = addr.Unmap()
		if addr == agent {
			holds = true
		}
		if !addr.IsLinkLocalUnicast() {
			global = true
		}
	}
	return holds, global
}

// isLoopbackInterface reports whether the interface is the kernel loopback or
// a dummy interface.
func isLoopbackInterface(info *iface.InterfaceInfo) bool {
	kind := iface.CanonicalInterfaceType(info)
	if kind == "loopback" {
		return true
	}
	return kind == iface.TypeDummy
}

// agentAddressAdvice turns a classification into the warnings it owes. A
// loopback address and a single-homed router owe none.
func agentAddressAdvice(agent netip.Addr, loc agentLocation) []string {
	address := agent.String()
	fix := " Fix: put the address on a loopback interface, for example: create interface dummy name agent0 address " +
		address + "/" + hostPrefixLength(agent) + ", or set agent-address to an address a loopback interface already holds."
	switch loc.placement {
	case agentPlacementLoopback, agentPlacementSingleHomed:
		return nil
	case agentPlacementRouted:
		return []string{"sflow agent-address " + address + " is on routed interface " + loc.owner +
			", not on a loopback interface. This router has more than one routed interface, so collectors" +
			" can lose this agent's identity when " + loc.owner + " goes down or is renumbered." + fix}
	case agentPlacementAbsent:
		return []string{"sflow agent-address " + address + " is not configured on any interface." +
			" Collectors identify this router by that address, and the router does not hold it." + fix}
	case agentPlacementUnspecified:
	}
	panic("BUG: sflow agent address classified with no placement")
}

// hostPrefixLength answers the host prefix length of the address family.
func hostPrefixLength(addr netip.Addr) string {
	if addr.Is4() {
		return "32"
	}
	return "128"
}
