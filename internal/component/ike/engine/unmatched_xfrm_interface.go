// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: unmatched.go -- the catch-all these entries are ranked above
// Related: established.go -- resolveIfID, the if_id a bound peer's Child SA policies carry
// Related: dynamic_wan.go -- the address worker that re-reads the interfaces
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the SPD selection function of a multi-SPD implementation

package engine

import (
	"fmt"
	"net"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
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
// kernel index of its device and the addresses it holds.
type xfrmInterfaceFacts struct {
	ifIndex   int
	addresses []netip.Addr
}

// xfrmExemption is one installed exemption entry and the interface it was built for,
// so a failed read of that interface keeps its entries rather than removing them.
type xfrmExemption struct {
	iface  string
	params dataplane.SPParams
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
// meets it too and crosses in the clear. docs/guide/ipsec.md names that residual.
func xfrmInterfaceExemptions(facts xfrmInterfaceFacts) []dataplane.SPParams {
	out := make([]dataplane.SPParams, 0, len(ikeBypassFamilies)*len(xfrmExemptionDirections)+len(facts.addresses))
	for _, family := range ikeBypassFamilies {
		anyNet := anyNetFor(family)
		for _, dir := range xfrmExemptionDirections {
			out = append(out, dataplane.SPParams{
				Src:      anyNet,
				Dst:      anyNet,
				Dir:      dir,
				Action:   dataplane.SPActionBypass,
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
			Priority: dataplane.PriorityUnmatchedXFRMInterface,
			SrcPort:  dataplane.AnyPortMatch(),
			DstPort:  dataplane.AnyPortMatch(),
		})
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

// readXFRMInterfaceFn is the read planXFRMExemptions takes, so a test stands fixed
// facts in its place without an interface backend. Production never replaces it.
//
//nolint:gochecknoglobals // Injection seam, replaced only by tests.
var readXFRMInterfaceFn = readXFRMInterface

// readXFRMInterface reads one bound interface through the shared iface resolver, which
// maps the logical name to its device (docs/architecture/iface/logical-name-resolution.md).
// An address the resolver reports in a form netip cannot parse is an error, never a
// skipped address: a missing source entry drops that address's IPv6 traffic. A zone
// is dropped, because an XFRM selector carries none.
func readXFRMInterface(name string) (xfrmInterfaceFacts, error) {
	// EnsureBackend first, for the reason resolveInterfaceAddr gives: without an
	// `interfaces` block nothing else loads one, and every read then fails.
	_ = iface.EnsureBackend()
	index, err := iface.ResolveIndex(name)
	if err != nil {
		return xfrmInterfaceFacts{}, err
	}
	addrs, err := iface.Addresses(name)
	if err != nil {
		return xfrmInterfaceFacts{}, err
	}
	facts := xfrmInterfaceFacts{ifIndex: index, addresses: make([]netip.Addr, 0, len(addrs))}
	for _, a := range addrs {
		addr, err := netip.ParseAddr(a.Address)
		if err != nil {
			return xfrmInterfaceFacts{}, fmt.Errorf("interface %s reports address %q, which does not parse: %w", name, a.Address, err)
		}
		facts.addresses = append(facts.addresses, addr.WithZone(""))
	}
	return facts, nil
}

// planXFRMExemptions answers the exemption entries cfg needs now: none unless the
// catch-all discards, and otherwise the entries of every bound interface.
//
// A read that fails is not an interface with nothing to exempt, so the entries held
// for that interface are kept. Removing them for a transient lookup fault would drop
// the tunnel's traffic, which is what followAddress refuses for the IKE address too.
// An interface that does not exist yet is the same failure; the address worker reads
// it again when its addresses or its link appear. The caller holds applyMu.
func (s *ikeEngineState) planXFRMExemptions(cfg *ipsec.IPsecConfig) map[spdSelector]xfrmExemption {
	next := make(map[spdSelector]xfrmExemption)
	if cfg.Unmatched != dataplane.SPActionDiscard {
		return next
	}
	for _, name := range boundXFRMInterfaces(cfg) {
		facts, err := readXFRMInterfaceFn(name)
		if err != nil {
			s.log.Warn("ike: cannot read a bound XFRM interface, keeping its SPD catch-all exemption as it was; "+
				"until it reads, traffic routed into it may meet the DISCARD catch-all; "+
				"fix: make sure the interface `vti bind` names exists (`show interface "+name+"`), "+
				"or `set vpn ipsec unmatched bypass`",
				"interface", name, "error", err)
			for sel, held := range s.xfrmExempt {
				if held.iface == name {
					next[sel] = held
				}
			}
			continue
		}
		for _, p := range xfrmInterfaceExemptions(facts) {
			next[selectorOf(&p)] = xfrmExemption{iface: name, params: p}
		}
	}
	return next
}

// installXFRMExemptions installs every entry of next the engine does not hold yet, and
// records each one the backend took. One that failed is left out of the record, so the
// next apply or address event tries it again. The failure is logged at error level
// because it fails closed: the bound tunnel's traffic meets the DISCARD catch-all.
// The caller holds applyMu.
func (s *ikeEngineState) installXFRMExemptions(dp dataplane.Dataplane, next map[spdSelector]xfrmExemption) {
	if dp == nil {
		return
	}
	if s.xfrmExempt == nil {
		s.xfrmExempt = make(map[spdSelector]xfrmExemption)
	}
	for sel, entry := range next {
		if _, held := s.xfrmExempt[sel]; held {
			continue
		}
		if err := dp.InstallPolicy(entry.params); err != nil {
			s.log.Error("ike: could not install the SPD catch-all exemption of a bound XFRM interface; "+
				"traffic routed into it meets the DISCARD catch-all and is dropped; "+
				"the daemon needs CAP_NET_ADMIN to install it, and the next commit or address change retries it",
				"interface", entry.iface, "dir", entry.params.Dir, "error", err)
			continue
		}
		s.xfrmExempt[sel] = entry
	}
}

// pruneXFRMExemptions removes every held entry next no longer names: an interface no
// peer binds, an address the interface lost, a device that moved to another index, or
// a catch-all that no longer discards. A removal is reported to the SPD ledger
// (removeZePolicy), so a failed one keeps the catch-all beneath it at exit. The
// caller holds applyMu.
func (s *ikeEngineState) pruneXFRMExemptions(dp dataplane.Dataplane, next map[spdSelector]xfrmExemption) {
	for sel, held := range s.xfrmExempt {
		if _, kept := next[sel]; kept {
			continue
		}
		if dp != nil {
			err := removeZePolicy(dp, held.params)
			if err != nil && !policyGone(err) {
				s.log.Warn("ike: could not remove the SPD catch-all exemption of an XFRM interface; "+
					"it keeps passing what it matches until removed",
					"interface", held.iface, "dir", held.params.Dir, "error", err)
			}
		}
		delete(s.xfrmExempt, sel)
	}
}
