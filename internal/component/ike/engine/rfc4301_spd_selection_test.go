// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: unmatched_xfrm_interface.go -- xfrmInterfaceExemptions, the selection entries under test
// Related: unmatched_xfrm_interface_test.go -- stubXFRMInterfaceReads, vtiBoundConfig, vti0IfID
// Related: source_guard_test.go -- guardRecorder, the guard a source entry waits for
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.1-5
//
// RFC 4301 Section 4.4.1: "However, if an implementation supports multiple SPDs, then
// it MUST include an explicit SPD selection function that is invoked to select the
// appropriate SPD for outbound traffic processing."
//
// Ze holds one SPD per XFRM if_id: the node-wide SPD at if_id 0 and one per interface
// a peer binds with `vti bind`. Linux looks an outbound packet up at if_id 0 first and,
// only when that lookup lets it through, again at the if_id of the XFRM interface the
// route sends it to (xfrmi_xmit). The selection function is therefore the set of if_id
// 0 entries that let a packet routed into a bound interface through, so the
// interface's own SPD decides it, while every other packet is decided by the node-wide
// SPD. These tests read the entries Ze holds and replay that two-step lookup over them.

package engine

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// selectionProbe is one outbound packet as the kernel's if_id 0 lookup sees it: the
// direction it is checked in, its addresses and protocol, the oif the flow carries at
// that lookup (0 when the flow carries none, as for IPv6 host output), and the if_id of
// the XFRM interface the route sends it to (0 when the route leaves through an
// ordinary device).
type selectionProbe struct {
	name     string
	dir      dataplane.SADir
	src, dst netip.Addr
	proto    uint8
	oif      int
	routedTo uint32
}

// matchesProbe answers whether entry p, of the SPD at ifID, matches the probe the way
// xfrm_policy_match does: the same if_id, the same direction, both prefixes holding the
// addresses, the protocol and ports either wildcard or the probe's, and the selector's
// ifindex either 0 (any device) or the flow's oif.
func matchesProbe(p *dataplane.SPParams, ifID uint32, probe *selectionProbe) bool {
	if p.IfID != ifID {
		return false
	}
	if p.Dir != probe.dir {
		return false
	}
	if p.Src == nil || !p.Src.Contains(net.IP(probe.src.AsSlice())) {
		return false
	}
	if p.Dst == nil || !p.Dst.Contains(net.IP(probe.dst.AsSlice())) {
		return false
	}
	if p.UpperProto != 0 && p.UpperProto != probe.proto {
		return false
	}
	// The probe is ICMP and carries no port, so only a wildcard port selector matches.
	if !p.SrcPort.IsAny() {
		return false
	}
	if !p.DstPort.IsAny() {
		return false
	}
	return p.IfIndex == 0 || p.IfIndex == probe.oif
}

// lookupSPD answers the entry that decides probe in the SPD at ifID: the matching entry
// with the lowest priority value, which XFRM searches first. False when none matches.
func lookupSPD(entries []dataplane.SPParams, ifID uint32, probe *selectionProbe) (dataplane.SPParams, bool) {
	var winner dataplane.SPParams
	found := false
	for i := range entries {
		p := &entries[i]
		if !matchesProbe(p, ifID, probe) {
			continue
		}
		if found && p.Priority >= winner.Priority {
			continue
		}
		winner, found = *p, true
	}
	return winner, found
}

// selectSPD replays the kernel's outbound selection for probe over entries and answers
// the if_id of the SPD that decides it, with the entry that decided the if_id 0 lookup.
// A BYPASS at if_id 0 for a packet routed into an XFRM interface hands it on to that
// interface's SPD (xfrmi_xmit); any other if_id 0 outcome is the node-wide SPD's own
// decision.
func selectSPD(t *testing.T, entries []dataplane.SPParams, probe *selectionProbe) (uint32, dataplane.SPParams) {
	t.Helper()
	node, ok := lookupSPD(entries, 0, probe)
	if !ok {
		t.Fatalf("%s: no if_id 0 entry matches, so the node-wide SPD has no final entry", probe.name)
	}
	if node.Action == dataplane.SPActionBypass && probe.routedTo != 0 {
		return probe.routedTo, node
	}
	return 0, node
}

// liveEntries answers every SPD entry the fake backend holds now.
func liveEntries(dp *catchAllDP) []dataplane.SPParams {
	out := make([]dataplane.SPParams, 0, len(dp.live))
	for key := range dp.live {
		out = append(out, dp.live[key])
	}
	return out
}

// nodeCatchAll answers the node-wide SPD's DISCARD catch-all, the final entry
// `unmatched discard` installs at if_id 0 (unmatched.go).
func nodeCatchAll() []dataplane.SPParams {
	out := make([]dataplane.SPParams, 0, len(ikeBypassFamilies)*len(unmatchedDirections))
	for _, family := range ikeBypassFamilies {
		out = append(out, spdCatchAllEntries(family, dataplane.SPActionDiscard, 0)...)
	}
	return out
}

// vti0Selection is the bound interface both tests use: vti0, ifindex 7, if_id 9, one
// IPv4 and one IPv6 tunnel address. eth0 (ifindex 3) is the WAN device.
const (
	vti0IfIndex = 7
	eth0IfIndex = 3
	icmpProto   = 1
	icmp6Proto  = 58
)

func vti0SelectionFacts(t *testing.T) xfrmInterfaceFacts {
	t.Helper()
	return xfrmInterfaceFacts{
		device:    "vti0",
		ifIndex:   vti0IfIndex,
		ifID:      vti0IfID,
		addresses: []netip.Addr{mustAddr(t, "10.255.0.1"), mustAddr(t, "2001:db8:ff::1")},
	}
}

// VALIDATES: RFC 4301 Section 4.4.1, the positive half: under `unmatched discard`, with
// one peer bound to vti0, every outbound path that routes a packet into vti0 is handed
// to vti0's own SPD (if_id 9) by an if_id 0 BYPASS ranked above the node-wide DISCARD
// catch-all, and that SPD exists with its own final entry. The paths are the ones the
// kernel looks up at if_id 0 before xfrmi_xmit: forwarded IPv4 and IPv6 in fwd and out
// with the oif of vti0, IPv4 host output with the oif of vti0, and IPv6 host output
// from vti0's IPv6 address, whose flow carries no oif.
// Method: apply the configuration over the fake backend and a stubbed iface read, then
// replay the two-step lookup over the entries the backend holds.
// PREVENTS: traffic routed into a bound XFRM interface being decided by the node-wide
// SPD, where the DISCARD catch-all drops it before the interface's SPD sees it.
//
// RFC requirement: RFC4301-4.4.1-5 positive -- with a peer bound to an XFRM interface under `unmatched discard`, the entries Ze installs hand forwarded IPv4 and IPv6 packets (fwd and out) and IPv4 host packets routed into the interface, and IPv6 host packets from its tunnel address, past the if_id 0 DISCARD catch-all to the interface's own SPD at its if_id, which holds a final entry.
func TestRFC4301SPDSelectionByOutputInterface(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = vti0SelectionFacts(t)
	state := applyTestState(t)

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	entries := liveEntries(dp)

	probes := []selectionProbe{
		{name: "forwarded IPv4, fwd", dir: dataplane.SADirFwd, src: mustAddr(t, "192.0.2.10"), dst: mustAddr(t, "10.20.0.1"), proto: icmpProto, oif: vti0IfIndex, routedTo: vti0IfID},
		{name: "forwarded IPv4, out", dir: dataplane.SADirOut, src: mustAddr(t, "192.0.2.10"), dst: mustAddr(t, "10.20.0.1"), proto: icmpProto, oif: vti0IfIndex, routedTo: vti0IfID},
		{name: "forwarded IPv6, fwd", dir: dataplane.SADirFwd, src: mustAddr(t, "2001:db8:1::10"), dst: mustAddr(t, "2001:db8:20::1"), proto: icmp6Proto, oif: vti0IfIndex, routedTo: vti0IfID},
		{name: "forwarded IPv6, out", dir: dataplane.SADirOut, src: mustAddr(t, "2001:db8:1::10"), dst: mustAddr(t, "2001:db8:20::1"), proto: icmp6Proto, oif: vti0IfIndex, routedTo: vti0IfID},
		{name: "IPv4 host output", dir: dataplane.SADirOut, src: mustAddr(t, "10.255.0.1"), dst: mustAddr(t, "10.20.0.1"), proto: icmpProto, oif: vti0IfIndex, routedTo: vti0IfID},
		{name: "IPv6 host output from the tunnel address", dir: dataplane.SADirOut, src: mustAddr(t, "2001:db8:ff::1"), dst: mustAddr(t, "2001:db8:20::1"), proto: icmp6Proto, oif: 0, routedTo: vti0IfID},
	}
	for i := range probes {
		probe := &probes[i]
		spd, decided := selectSPD(t, entries, probe)
		if spd != vti0IfID {
			t.Errorf("%s: decided by the SPD at if_id %d (if_id 0 entry: action %d, priority %d, ifindex %d), want vti0's SPD at if_id %d",
				probe.name, spd, decided.Action, decided.Priority, decided.IfIndex, vti0IfID)
			continue
		}
		if decided.Priority >= int(dataplane.PriorityUnmatched) {
			t.Errorf("%s: the hand-off entry has priority %d, which does not outrank the node-wide catch-all (%d)",
				probe.name, decided.Priority, int(dataplane.PriorityUnmatched))
		}
		final, ok := lookupSPD(entries, spd, probe)
		if !ok {
			t.Errorf("%s: handed to the SPD at if_id %d, which holds no entry for it: no SPD was selected", probe.name, spd)
			continue
		}
		if final.Action != dataplane.SPActionDiscard {
			t.Errorf("%s: vti0's SPD decides it with action %d, want its final DISCARD entry (no Child SA is up)", probe.name, final.Action)
		}
	}
}

// VALIDATES: RFC 4301 Section 4.4.1, the negative half: the selection function selects
// vti0's SPD only for traffic routed into vti0. A packet routed out of eth0, forwarded
// or host, in either family, is still decided by the node-wide SPD's DISCARD catch-all;
// and an IPv6 packet sourced from vti0's tunnel address but routed out of eth0 is
// decided there too while the address has no tunnel-source guard, because the source
// entry that would let it through waits for its guard.
// Method: two legs. The device leg plans vti0's entries (planXFRMInterfaceEntries)
// beside the node-wide DISCARD catch-all and replays the lookup for eth0 packets. The
// guard leg refuses the guard's rule install, applies the configuration over the fake
// backend, and replays the lookup for the IPv6 packet from the tunnel address.
// PREVENTS: an exemption that selects any device (cleartext out of eth0 past the
// DISCARD catch-all), and a source BYPASS live with nothing keeping its address on vti0.
//
// RFC requirement: RFC4301-4.4.1-5 negative -- packets routed out of a device that is not the bound XFRM interface (forwarded and host, IPv4 and IPv6) are decided by the if_id 0 DISCARD catch-all, never handed on by a selection entry; and while a tunnel address has no source guard, an IPv6 packet from it routed out of that device is decided by the DISCARD catch-all too.
func TestRFC4301SPDSelectionLeavesOtherTrafficInTheNodeSPD(t *testing.T) {
	t.Run("device", func(t *testing.T) {
		stub := stubXFRMInterfaceReads(t)
		stub.facts["vti0"] = vti0SelectionFacts(t)
		state := applyTestState(t)

		plan := state.planXFRMInterfaceEntries(vtiBoundConfig(dataplane.SPActionDiscard))
		entries := nodeCatchAll()
		exemptions := 0
		for _, entry := range plan.entries {
			entries = append(entries, entry.params)
			if entry.role == dataplane.PolicyRoleInterfaceExemption {
				exemptions++
			}
		}
		if exemptions == 0 {
			t.Fatal("the plan holds no exemption for vti0, so the leg below would pass for no reason")
		}

		probes := []selectionProbe{
			{name: "forwarded IPv4 out eth0, fwd", dir: dataplane.SADirFwd, src: mustAddr(t, "192.0.2.10"), dst: mustAddr(t, "198.51.100.1"), proto: icmpProto, oif: eth0IfIndex},
			{name: "forwarded IPv4 out eth0, out", dir: dataplane.SADirOut, src: mustAddr(t, "192.0.2.10"), dst: mustAddr(t, "198.51.100.1"), proto: icmpProto, oif: eth0IfIndex},
			{name: "forwarded IPv6 out eth0, fwd", dir: dataplane.SADirFwd, src: mustAddr(t, "2001:db8:1::10"), dst: mustAddr(t, "2001:db8:99::1"), proto: icmp6Proto, oif: eth0IfIndex},
			{name: "forwarded IPv6 out eth0, out", dir: dataplane.SADirOut, src: mustAddr(t, "2001:db8:1::10"), dst: mustAddr(t, "2001:db8:99::1"), proto: icmp6Proto, oif: eth0IfIndex},
			{name: "IPv4 host output out eth0", dir: dataplane.SADirOut, src: mustAddr(t, "192.0.2.1"), dst: mustAddr(t, "198.51.100.1"), proto: icmpProto, oif: eth0IfIndex},
		}
		assertNodeCatchAllDecides(t, entries, probes)
	})

	t.Run("guard", func(t *testing.T) {
		dp := &catchAllDP{}
		loadFailClosedBackend(t, dp)
		stub := stubXFRMInterfaceReads(t)
		stub.facts["vti0"] = vti0SelectionFacts(t)
		stub.guard.failRule = errors.New("test: operation not permitted")
		state := applyTestState(t)

		if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
			t.Fatalf("discard apply: %v", err)
		}
		entries := liveEntries(dp)
		handOffs := 0
		for i := range entries {
			if entries[i].Priority == dataplane.PriorityUnmatchedXFRMInterface && entries[i].IfIndex == vti0IfIndex {
				handOffs++
			}
		}
		if handOffs == 0 {
			t.Fatal("the apply installed no interface-index exemption, so the apply did not run the selection at all")
		}

		probes := []selectionProbe{
			{name: "IPv6 host output from the unguarded tunnel address out eth0", dir: dataplane.SADirOut, src: mustAddr(t, "2001:db8:ff::1"), dst: mustAddr(t, "2001:db8:99::1"), proto: icmp6Proto, oif: 0},
		}
		assertNodeCatchAllDecides(t, entries, probes)
	})
}

// assertNodeCatchAllDecides requires each probe to be decided by the node-wide SPD's
// DISCARD catch-all: the if_id 0 lookup's winner is the entry ranked last, and it
// discards.
func assertNodeCatchAllDecides(t *testing.T, entries []dataplane.SPParams, probes []selectionProbe) {
	t.Helper()
	for i := range probes {
		probe := &probes[i]
		spd, decided := selectSPD(t, entries, probe)
		if spd != 0 {
			t.Errorf("%s: handed to the SPD at if_id %d, want the node-wide SPD", probe.name, spd)
			continue
		}
		if decided.Priority != int(dataplane.PriorityUnmatched) {
			t.Errorf("%s: decided at if_id 0 by an entry of priority %d (ifindex %d, source %s), want the catch-all at %s",
				probe.name, decided.Priority, decided.IfIndex, decided.Src, strconv.FormatUint(uint64(dataplane.PriorityUnmatched), 10))
			continue
		}
		if decided.Action != dataplane.SPActionDiscard {
			t.Errorf("%s: the node-wide catch-all acts %d, want discard", probe.name, decided.Action)
		}
	}
}
