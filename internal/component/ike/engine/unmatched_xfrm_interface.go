// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: unmatched.go -- the catch-all these entries are ranked above
// Related: established.go -- resolveIfID, the if_id a bound peer's Child SA policies carry
// Related: dynamic_wan.go -- the address worker that re-reads the interfaces
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the SPD selection function of a multi-SPD implementation

package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// The DISCARD catch-all (unmatched.go) sits at if_id 0, and a route-based peer's Child
// SA policies sit at the if_id of the XFRM interface it binds (resolveIfID). Linux keeps
// one policy database for each if_id and looks a packet up in the one its path names:
//
//   - IPv6 host output is looked up at if_id 0 before the packet reaches any device
//     (ip6_dst_lookup_flow calls xfrm_lookup_route with if_id 0), and the flow's oif
//     is still 0 unless the socket chose a device. A ping6 routed into vti0 met the
//     catch-all there and was dropped: the lab scenario vti-ipv6-inner-ipv4-outer lost
//     every packet.
//   - IPv4 host output is looked up at if_id 0 too, with the flow's oif already set
//     to the route's device (ip_route_output_key_hash_rcu fills flowi4_oif).
//   - Forwarded traffic of both families is checked at if_id 0, in the fwd and the out
//     direction, with the oif taken from the route's device (decode_session4 and
//     decode_session6).
//
// Only once the packet reaches the XFRM interface does xfrmi_xmit look it up again at
// the interface's own if_id, where the Child SA policies are. So under a DISCARD
// catch-all every packet routed into a bound interface is dropped before that second
// lookup, unless an if_id 0 entry above the catch-all lets it through. These entries
// are that exemption, and they are the SPD selection function RFC 4301 asks of an
// implementation with more than one SPD: they hand the packet on to the interface's
// SPD instead of deciding it in the node-wide one.

// xfrmInterfaceFacts is one bound XFRM interface as the iface resolver reads it: the
// kernel name and index of its device, the if_id that names its SPD, and the addresses
// it holds. readXFRMInterface never answers an ifID of 0 (boundInterfaceIfID).
type xfrmInterfaceFacts struct {
	device    string
	ifIndex   int
	ifID      uint32
	addresses []netip.Addr
}

// xfrmInterfaceEntry is one installed SPD entry Ze holds for a bound XFRM interface,
// the interface it was built for and the job it does, so a failed read of that
// interface keeps its entries rather than removing them, and install and prune run
// the roles in their order (xfrmInterfaceRoleOrder). Maps hold it by pointer, so a
// range over them copies no SPParams; an entry is never changed once built, which is
// what lets a plan and the installed set share one.
type xfrmInterfaceEntry struct {
	iface  string
	role   dataplane.PolicyRole
	params dataplane.SPParams
}

// xfrmInterfaceRoleOrder is the order the roles of a bound interface's entries are
// installed in; removal runs it backwards. The interface's own catch-all goes in
// first, so its SPD has its final entry before the if_id 0 exemption hands any packet
// on to it, and it comes out last, after the exemption stopped handing packets over.
var xfrmInterfaceRoleOrder = [...]dataplane.PolicyRole{
	dataplane.PolicyRoleInterfaceCatchAll,
	dataplane.PolicyRoleInterfaceExemption,
}

// xfrmExemptionDirections are the directions the interface-index entry covers. In is
// absent: a packet that arrives through an XFRM interface is checked at that
// interface's if_id (xfrmi_decode_session), never at if_id 0.
var xfrmExemptionDirections = [...]dataplane.SADir{dataplane.SADirOut, dataplane.SADirFwd}

// xfrmInterfaceExemptions builds the if_id 0 entries that let traffic routed into one
// bound XFRM interface reach the interface's own SPD.
//
// RFC 4301 Section 4.4.1: "However, if an implementation supports multiple SPDs, then
// it MUST include an explicit SPD selection function that is invoked to select the
// appropriate SPD for outbound traffic processing." Each if_id is one SPD, and the
// packet's output device is what selects it, so the entries match that device:
//
//   - out and fwd, both families, any address, the selector naming the interface's
//     index: forwarded traffic and IPv4 host traffic, whose flow carries the device.
//   - out, IPv6 only, each IPv6 address the interface holds as the source: IPv6 host
//     traffic, whose flow carries no device at the if_id 0 lookup. IPv4 needs no
//     such entry, and one would only widen the bypass described below.
//
// Every entry is a BYPASS ranked at dataplane.PriorityUnmatchedXFRMInterface, one
// place above the catch-all, so Child SA policies and operator entries still decide
// first. The source-address entry is wider than the device it stands for: an IPv6
// packet sourced from the interface's address but routed out of another interface
// meets it too. The tunnel-source guard (source_guard.go) closes that path: a source
// entry is installed only while its address is guarded, so a packet from the address
// leaves only through the interface, where its own SPD decides it.
func xfrmInterfaceExemptions(facts xfrmInterfaceFacts) []dataplane.SPParams {
	out := make([]dataplane.SPParams, 0, len(spdFamilies)*len(xfrmExemptionDirections)+len(facts.addresses))
	for _, family := range spdFamilies {
		anyNet := anyNetFor(family)
		for _, dir := range xfrmExemptionDirections {
			out = append(out, dataplane.SPParams{
				Src:      anyNet,
				Dst:      anyNet,
				Dir:      dir,
				Action:   dataplane.SPActionBypass,
				Role:     dataplane.PolicyRoleInterfaceExemption,
				Priority: dataplane.PriorityUnmatchedXFRMInterface,
				SrcPort:  dataplane.AnyPortMatch(),
				DstPort:  dataplane.AnyPortMatch(),
				IfIndex:  facts.ifIndex,
			})
		}
	}
	for _, addr := range facts.addresses {
		if !addr.Is6() || addr.Is4In6() {
			continue
		}
		out = append(out, dataplane.SPParams{
			Src:      &net.IPNet{IP: addr.AsSlice(), Mask: net.CIDRMask(128, 128)},
			Dst:      anyNetFor(net.IPv6zero),
			Dir:      dataplane.SADirOut,
			Action:   dataplane.SPActionBypass,
			Role:     dataplane.PolicyRoleInterfaceExemption,
			Priority: dataplane.PriorityUnmatchedXFRMInterface,
			SrcPort:  dataplane.AnyPortMatch(),
			DstPort:  dataplane.AnyPortMatch(),
		})
	}
	return out
}

// interfaceSPDCatchAll builds the final entry of one XFRM interface's SPD: in, out
// and fwd for both families, at the interface's if_id, ranked last.
//
// RFC 4301 Section 4.4.1: "Every SPD SHOULD have a nominal, final entry that matches
// anything that is otherwise unmatched, and discards it." RFC 4301 Section 5.2:
// "Every SPD SHOULD have a nominal, final entry that catches anything that is
// otherwise unmatched, and discards it." Each if_id is one SPD, so the interface's
// SPD needs its own entry: the if_id 0 catch-all is never consulted for a packet
// looked up at the interface's if_id.
//
// It discards under both values of `unmatched`. A BYPASS at an interface if_id
// carries no cleartext: xfrmi_xmit drops a packet whose lookup returns no state, and
// __xfrm_policy_check rejects a tunnel-mode packet a template-free entry matched, so a
// bypass entry would be listed as bypass while the kernel drops what it matches.
// Ranked at dataplane.PriorityUnmatched, it sits below every Child SA policy of the
// interface (dataplane.PriorityChildSA), which the kernel searches first.
func interfaceSPDCatchAll(ifID uint32) []dataplane.SPParams {
	out := make([]dataplane.SPParams, 0, len(spdFamilies)*len(unmatchedDirections))
	for _, family := range spdFamilies {
		out = append(out, spdCatchAllEntries(family, dataplane.SPActionDiscard, ifID)...)
	}
	return out
}

// boundXFRMInterfaces answers the interfaces the configuration's peers bind under
// `vti bind`, each once, sorted so the reads run in a stable order.
func boundXFRMInterfaces(cfg *ipsec.IPsecConfig) []string {
	var names []string
	for name := range cfg.Peers {
		bind := cfg.Peers[name].VTIBind
		if bind == "" {
			continue
		}
		if slices.Contains(names, bind) {
			continue
		}
		names = append(names, bind)
	}
	slices.Sort(names)
	return names
}

// readXFRMInterfaceFn is the read planXFRMInterfaceEntries takes, so a test stands fixed
// facts in its place without an interface backend. Production never replaces it.
//
//nolint:gochecknoglobals // Injection seam, replaced only by tests.
var readXFRMInterfaceFn = readXFRMInterface

// readXFRMInterface reads one bound interface through the shared iface resolver, which
// maps the logical name to its device (docs/architecture/iface/logical-name-resolution.md).
// An address the resolver reports in a form netip cannot parse is an error, never a
// skipped address: a missing source entry drops that address's IPv6 traffic. A zone
// is dropped, because an XFRM selector carries none. The if_id comes from the same
// resolution the peer's Child SA policies use (boundInterfaceIfID), so the interface's
// catch-all sits in the SPD that holds them.
func readXFRMInterface(name string) (xfrmInterfaceFacts, error) {
	// EnsureBackend first, for the reason resolveInterfaceAddr gives: without an
	// `interfaces` block nothing else loads one, and every read then fails.
	_ = iface.EnsureBackend()
	index, err := iface.ResolveIndex(name)
	if err != nil {
		return xfrmInterfaceFacts{}, err
	}
	device, err := iface.ResolveDevice(name)
	if err != nil {
		return xfrmInterfaceFacts{}, err
	}
	ifID, err := boundInterfaceIfID(name)
	if err != nil {
		return xfrmInterfaceFacts{}, err
	}
	addrs, err := iface.Addresses(name)
	if err != nil {
		return xfrmInterfaceFacts{}, err
	}
	facts := xfrmInterfaceFacts{device: device, ifIndex: index, ifID: ifID, addresses: make([]netip.Addr, 0, len(addrs))}
	for _, a := range addrs {
		addr, err := netip.ParseAddr(a.Address)
		if err != nil {
			return xfrmInterfaceFacts{}, fmt.Errorf("interface %s reports address %q, which does not parse: %w", name, a.Address, err)
		}
		facts.addresses = append(facts.addresses, addr.WithZone(""))
	}
	return facts, nil
}

// xfrmInterfacePlan is what the bound XFRM interfaces of a configuration need now: their
// SPD entries, and under a DISCARD catch-all the tunnel-source guard of each interface
// that has an IPv6 address to exempt.
type xfrmInterfacePlan struct {
	entries map[spdSelector]*xfrmInterfaceEntry
	guards  map[string]sourceGuardSpec
}

// planXFRMInterfaceEntries answers the SPD entries the bound XFRM interfaces of cfg
// need now, and their guards. Every bound interface gets its own SPD's catch-all (interfaceSPDCatchAll),
// whatever `unmatched` says. Only while the if_id 0 catch-all discards does it also get
// the if_id 0 exemptions (xfrmInterfaceExemptions): under a bypass catch-all the
// traffic reaches the interface without them.
//
// A read that fails is not an interface with nothing to hold, so the entries held for
// that interface are kept, the exemptions only while the catch-all still discards.
// Removing them for a transient lookup fault would drop the tunnel's traffic, which is
// what followAddress refuses for the IKE address too. An interface that does not exist
// yet is the same failure; the address worker reads it again when its addresses or its
// link appear. A read answering an if_id of 0 is refused like a failed read, because an
// entry built for it would sit in the node-wide SPD. A failed read keeps the interface's
// guard as held too, while the catch-all discards. The caller holds applyMu.
func (s *ikeEngineState) planXFRMInterfaceEntries(cfg *ipsec.IPsecConfig) xfrmInterfacePlan {
	next := make(map[spdSelector]*xfrmInterfaceEntry)
	plan := xfrmInterfacePlan{entries: next, guards: make(map[string]sourceGuardSpec)}
	discards := cfg.Unmatched == dataplane.SPActionDiscard
	for _, name := range boundXFRMInterfaces(cfg) {
		facts, err := readXFRMInterfaceFn(name)
		if err == nil && facts.ifID == 0 {
			err = errors.New("the interface reports if_id 0, which is the node-wide SPD")
		}
		if err != nil {
			s.log.Warn("ike: cannot read a bound XFRM interface, keeping its SPD entries as they were; "+
				"until it reads, a new interface SPD has no final entry, and under `unmatched discard` traffic routed into it may meet the DISCARD catch-all; "+
				"fix: make sure the interface `vti bind` names exists and is an XFRM interface (`show interface "+name+"`)",
				"interface", name, "error", err)
			for sel, held := range s.xfrmEntries {
				if held.iface != name {
					continue
				}
				if held.role == dataplane.PolicyRoleInterfaceExemption && !discards {
					continue
				}
				next[sel] = held
			}
			if spec, held := guards.heldSpec(name); held && discards {
				plan.guards[name] = spec
			}
			continue
		}
		// RFC 4301 Section 4.4.1, Section 5.2: the interface SPD's final entry.
		catchAll := interfaceSPDCatchAll(facts.ifID)
		for i := range catchAll {
			p := &catchAll[i]
			// The catch-all names its SPD, and every other entry at this if_id (the
			// Child SA policies) is named from it when the dataplane is read.
			p.SPD = name
			next[selectorOf(p)] = &xfrmInterfaceEntry{iface: name, role: dataplane.PolicyRoleInterfaceCatchAll, params: *p}
		}
		if !discards {
			continue
		}
		// RFC 4301 Section 4.4.1: the SPD selection function, by output interface.
		exemptions := xfrmInterfaceExemptions(facts)
		for i := range exemptions {
			p := &exemptions[i]
			next[selectorOf(p)] = &xfrmInterfaceEntry{iface: name, role: dataplane.PolicyRoleInterfaceExemption, params: *p}
		}
		if sources := guardSources(name, facts, cfg, s.log); len(sources) > 0 {
			plan.guards[name] = sourceGuardSpec{iface: name, device: facts.device, ifIndex: facts.ifIndex, ifID: facts.ifID, sources: sources}
		}
	}
	return plan
}

// exemptionSource answers the source address of an if_id 0 source exemption entry, and
// false for an interface-index entry, which carries no source.
func exemptionSource(p *dataplane.SPParams) (netip.Addr, bool) {
	if p.IfIndex != 0 {
		return netip.Addr{}, false
	}
	addr, ok := netip.AddrFromSlice(p.Src.IP)
	if !ok {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// installXFRMInterfaceEntries installs every entry of next the engine does not hold
// yet, role by role in xfrmInterfaceRoleOrder, and records each one the backend took.
// One that failed is left out of the record, so the next apply or address event tries
// it again. A failed exemption is logged at error level because it fails closed: the
// bound tunnel's traffic meets the DISCARD catch-all.
//
// A failed interface catch-all is an error the caller returns, as installUnmatched's is
// for the node-wide SPD: the interface's SPD then has no final entry. The kernel drops
// most of what reaches such an SPD unmatched for its own reasons (xfrmi_xmit finds no
// state; __xfrm_policy_check rejects a tunnel-mode secpath no entry matched), but not
// all of it: __xfrm_policy_check accepts an unmatched inbound packet whose secpath holds
// only transport-mode states, or a crypto-offload state (net/xfrm/xfrm_policy.c, the
// `if (!pol)` branch). RFC 4301 Section 4.4.1: "Every SPD SHOULD have a nominal, final
// entry that matches anything that is otherwise unmatched, and discards it." So the
// apply fails closed rather than report applied an SPD without it, and no exemption is
// installed for that interface in the same pass: an exemption would hand traffic on to
// the SPD that lacks its final entry.
//
// A backend that declares it holds no SPD entry bound to no interface
// (catchAllUnsupported, the VPP backend) gets no interface catch-all: such a backend
// keeps no if_id SPD for the entry to close.
//
// The tunnel-source guards go in between the interface catch-alls and the exemptions,
// and a source exemption whose address the guard does not hold is not installed: no
// instant holds a source BYPASS without its guard (source_guard.go). The caller holds
// applyMu.
//
// This is the form a reload and the address worker use. The daemon's start calls
// placeXFRMInterfaceEntries itself and reports a refused interface catch-all instead of
// returning it (owner decision 2026-11-20, 5c; recordInterfaceCatchAllAtStart).
func (s *ikeEngineState) installXFRMInterfaceEntries(dp dataplane.Dataplane, plan xfrmInterfacePlan) error {
	return interfaceCatchAllRefused(s.placeXFRMInterfaceEntries(dp, plan))
}

// placeXFRMInterfaceEntries is the install pass of installXFRMInterfaceEntries. It
// answers each interface whose SPD catch-all the backend refused in this pass, with the
// backend's answer; nil or empty when there is none. An interface the start recorded
// (interfaceCatchAllRefused) that this pass did not refuse again leaves the record: its
// catch-all is in, or no peer binds it any more. The caller holds applyMu.
func (s *ikeEngineState) placeXFRMInterfaceEntries(dp dataplane.Dataplane, plan xfrmInterfacePlan) map[string]error {
	if dp == nil {
		// refuse-by-default: guards installUnmatched -- every apply refuses a missing dataplane there
		return nil
	}
	next := plan.entries
	if s.xfrmEntries == nil {
		s.xfrmEntries = make(map[spdSelector]*xfrmInterfaceEntry)
	}
	noIfIDSPD := catchAllUnsupported(dp)
	// noFinalEntry names each interface whose catch-all the backend refused in this pass.
	noFinalEntry := make(map[string]error)
	// noExemption names each interface with an exemption the backend refused in this
	// pass, with the first refusal.
	noExemption := make(map[string]*opererr.Error)
	for _, role := range xfrmInterfaceRoleOrder {
		if role == dataplane.PolicyRoleInterfaceCatchAll && noIfIDSPD != nil {
			s.warnNoInterfaceCatchAll(next, noIfIDSPD)
			continue
		}
		var guarded map[netip.Addr]bool
		if role == dataplane.PolicyRoleInterfaceExemption {
			guarded = guards.install(plan.guards, s.log)
		}
		for sel, entry := range next {
			if entry.role != role {
				continue
			}
			if interfaceForwardDefault(dp, entry) {
				continue
			}
			if _, held := s.xfrmEntries[sel]; held {
				continue
			}
			if source, isSource := exemptionSource(&entry.params); isSource && role == dataplane.PolicyRoleInterfaceExemption && !guarded[source] {
				continue
			}
			if role == dataplane.PolicyRoleInterfaceExemption {
				// The catch-all role ran first, so noFinalEntry is complete for this pass.
				if _, refused := noFinalEntry[entry.iface]; refused {
					continue
				}
			}
			err := dp.InstallPolicy(entry.params)
			if err == nil {
				s.xfrmEntries[sel] = entry
				continue
			}
			if role == dataplane.PolicyRoleInterfaceCatchAll {
				// Answered, not reported here: each caller reports the interface once, the
				// reload as its refusal (interfaceCatchAllRefused), the start at Error
				// (recordInterfaceCatchAllAtStart) and the address worker at Error
				// (followAddress). This line keeps the direction for a debug log.
				s.log.Debug("ike: the backend refused an SPD catch-all entry of a bound XFRM interface",
					"interface", entry.iface, "dir", entry.params.Dir, "error", err)
				noFinalEntry[entry.iface] = err
				continue
			}
			s.log.Error("ike: could not install an SPD exemption of a bound XFRM interface, so under `unmatched discard` "+
				"the traffic routed into that interface is dropped at the node-wide catch-all; "+
				"the daemon needs CAP_NET_ADMIN to install it, and the next commit or address change retries it",
				"interface", entry.iface, "dir", entry.params.Dir, "error", err,
				"code", diagnostic.CodeDoctorIPsecSPDExemptionMissing)
			if _, seen := noExemption[entry.iface]; !seen {
				noExemption[entry.iface] = interfaceExemptionRefusal(entry, err)
			}
		}
	}
	for name := range s.interfaceCatchAllRefused {
		if _, still := noFinalEntry[name]; !still {
			delete(s.interfaceCatchAllRefused, name)
		}
	}
	// Every pass retries each exemption it does not hold, so this pass's refusals are the
	// whole record: an exemption that went in, or that no plan names any more, is gone
	// from it. One skipped because its interface has no final entry is reported by that
	// interface's catch-all finding instead.
	s.interfaceExemptionRefused = noExemption
	return noFinalEntry
}

// interfaceExemptionRefusal is the operator error for an if_id 0 exemption of a bound
// XFRM interface the backend refused, which the ipsec-spd-install doctor check reports
// under diagnostic.CodeDoctorIPsecSPDExemptionMissing. A reload is not refused for it,
// because the entry fails closed: the traffic it would hand on is dropped, never passed.
func interfaceExemptionRefusal(entry *xfrmInterfaceEntry, cause error) *opererr.Error {
	return opererr.Wrap(cause,
		"Ze could not install the SPD exemption that hands traffic routed into XFRM interface "+entry.iface+
			" on to that interface (direction "+dirLabel(entry.params.Dir)+")",
		"Under `unmatched discard` that traffic meets the node-wide catch-all and is dropped, so the tunnel bound to "+
			entry.iface+" carries none of it. The next commit or address change retries the entry. "+
			"Remove the cause the dataplane names (the daemon needs CAP_NET_ADMIN), then restart Ze",
		spdInstallFixRestart())
}

// recordInterfaceCatchAllAtStart is what the daemon's start does with the interfaces
// whose SPD catch-all the backend refused (placeXFRMInterfaceEntries), where a reload
// refuses the commit: each is logged at Error and recorded for the ipsec-spd-install
// doctor check, in name order, and the apply goes on (owner decision 2026-11-20, 5c,
// the start-up rule of decision 3 (a)). The pass has already installed every other
// entry it could and no exemption of a refused interface. The caller holds applyMu.
func (s *ikeEngineState) recordInterfaceCatchAllAtStart(noFinalEntry map[string]error) {
	for _, name := range slices.Sorted(maps.Keys(noFinalEntry)) {
		refusal := interfaceCatchAllStartRefusal(name, noFinalEntry[name])
		s.log.LogAttrs(context.Background(), slog.LevelError, "ike: "+refusal.Problem(),
			refusal.LogAttrs(slog.String("code", diagnostic.CodeDoctorIPsecSPDCatchAllMissing))...)
		if s.interfaceCatchAllRefused == nil {
			s.interfaceCatchAllRefused = make(map[string]*opererr.Error)
		}
		s.interfaceCatchAllRefused[name] = refusal
	}
}

// interfaceCatchAllStartRefusal is the operator error for one XFRM interface whose SPD
// catch-all, always a discard (interfaceSPDCatchAll), the start could not install. The
// fix is a restart for the reason unmatchedStartRefusal gives.
func interfaceCatchAllStartRefusal(name string, cause error) *opererr.Error {
	return opererr.Wrap(cause,
		"The default discard of XFRM interface "+name+" is NOT in force: Ze could not install that interface's SPD catch-all when the daemon started",
		"Traffic that no entry of that interface's SPD matches meets no final entry, and the kernel accepts some of it "+
			"(an inbound packet under a transport-mode or offloaded state). RFC 4301 Section 5 requires it dropped: "+
			"\"If no policy is found in the SPD that matches a packet (for either inbound or outbound traffic), the packet MUST be discarded.\" "+
			"The final discard entry that ensures it is a SHOULD (Section 4.4.1). "+
			"A start never refuses the IPsec block, so Ze installed the rest of the configuration and no entry hands traffic on to that interface. "+
			"Remove the cause the dataplane names (the daemon needs CAP_NET_ADMIN), then restart Ze",
		spdInstallFixRestart())
}

// A namespace FWD default also closes every if_id's forward lookup. An explicit
// interface final entry would prevent the ICMP payload lookup before that default.
func interfaceForwardDefault(dp dataplane.Dataplane, entry *xfrmInterfaceEntry) bool {
	_, usesDefault := forwardDefaultInstaller(dp)
	return usesDefault && entry.role == dataplane.PolicyRoleInterfaceCatchAll && entry.params.Dir == dataplane.SADirFwd
}

// interfaceCatchAllRefused answers the error installXFRMInterfaceEntries returns for
// the interfaces whose SPD catch-all the backend refused, naming the first in sorted
// order and how many there are, or nil when there is none.
func interfaceCatchAllRefused(noFinalEntry map[string]error) error {
	if len(noFinalEntry) == 0 {
		return nil
	}
	names := make([]string, 0, len(noFinalEntry))
	for name := range noFinalEntry {
		names = append(names, name)
	}
	slices.Sort(names)
	return fmt.Errorf("the SPD of XFRM interface %s (one of %d refused) has no final entry: "+
		"its catch-all could not be installed: %w; the daemon needs CAP_NET_ADMIN to install it, then commit again",
		names[0], len(names), noFinalEntry[names[0]])
}

// warnNoInterfaceCatchAll says, once per install pass that has one to place, that the
// backend keeps no if_id SPD, so no interface catch-all is installed.
func (s *ikeEngineState) warnNoInterfaceCatchAll(next map[spdSelector]*xfrmInterfaceEntry, reason error) {
	for _, entry := range next {
		if entry.role != dataplane.PolicyRoleInterfaceCatchAll {
			continue
		}
		s.log.Warn("ike: this dataplane keeps no SPD per XFRM interface, so no interface SPD catch-all is installed",
			"interface", entry.iface, "error", reason)
		return
	}
}

// pruneXFRMInterfaceEntries removes every held entry next no longer names, role by role
// in the reverse of xfrmInterfaceRoleOrder: an interface no peer binds, an address the
// interface lost, a device that moved to another index or if_id, or an exemption under
// a catch-all that no longer discards. A removal is reported to the SPD ledger
// (removeZePolicy), so a failed one keeps the if_id 0 catch-all in place at exit. The
// caller holds applyMu. The tunnel-source guards come out after the exemptions they
// guard and before the interface catch-alls.
func (s *ikeEngineState) pruneXFRMInterfaceEntries(dp dataplane.Dataplane, plan xfrmInterfacePlan) {
	next := plan.entries
	for i := len(xfrmInterfaceRoleOrder) - 1; i >= 0; i-- {
		role := xfrmInterfaceRoleOrder[i]
		if role == dataplane.PolicyRoleInterfaceCatchAll {
			guards.prune(plan.guards, s.log)
		}
		for sel, held := range s.xfrmEntries {
			if held.role != role {
				continue
			}
			if entry, kept := next[sel]; kept && !interfaceForwardDefault(dp, entry) {
				continue
			}
			if dp != nil {
				err := removeZePolicy(dp, held.params)
				if err != nil && !policyGone(err) {
					s.log.Warn("ike: could not remove an SPD entry of an XFRM interface; "+
						"it keeps acting on what it matches until removed",
						"interface", held.iface, "role", held.role.String(), "dir", held.params.Dir, "error", err)
				}
			}
			delete(s.xfrmEntries, sel)
		}
	}
}
