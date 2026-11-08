// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the IPv6 tunnel-source guard
// Related: unmatched_xfrm_interface.go -- the source BYPASS entries this guard makes sound
// Related: established.go -- the Child SA route events that fill the guard tables
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the SPD selection function of a multi-SPD implementation

package engine

import (
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/iprule"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The IPv6 source BYPASS of a bound XFRM interface (xfrmInterfaceExemptions) selects
// the interface's SPD by the packet's source address, because IPv6 host output carries
// no output device into the if_id 0 lookup. A source address is wider than the device
// it stands for: a packet from the tunnel address routed out of another interface meets
// the same BYPASS and would leave in the clear. The guard closes that path in two parts:
//
//   - Routing: one rule `from <tunnel address>/128 lookup <table>` per guarded address,
//     and a table per interface holding a route into the interface for each Child SA
//     remote traffic selector bound to it, ended by `unreachable ::/0`. A socket that
//     chose the tunnel address as its source is steered into the tunnel or refused.
//   - Filtering: the nft table ze_ipsec_source_guard drops, at postrouting, IPv6 whose
//     source is a guarded address and whose output device is not its interface. When
//     the kernel chose the source, ip6_dst_lookup_tail keeps the route it looked up with
//     no source, so no source rule is consulted; the drop is what stops that packet.
//
// A source BYPASS is installed only while its address is guarded (installXFRMInterfaceEntries),
// and the guard is removed only after the BYPASS (pruneXFRMInterfaceEntries).

const (
	// sourceGuardOwner is the firewall registry owner of the guard's nft table.
	sourceGuardOwner = "ike-source-guard"
	// sourceGuardTableName is the guard's nft table; the ze_ prefix is the ownership
	// mark the firewall registry requires (firewall.RegisterTables).
	sourceGuardTableName = "ze_ipsec_source_guard"
	sourceGuardChainName = "postrouting"
)

// sourceGuardSpec is what one bound interface's guard must hold: the kernel device the
// nft term names, the link its tunnel routes use, the if_id its Child SAs carry, and
// the IPv6 addresses whose source BYPASS it guards.
type sourceGuardSpec struct {
	iface   string
	device  string
	ifIndex int
	ifID    uint32
	sources []netip.Addr
}

// heldSourceGuard is one interface's guard as installed: its table, and the rules and
// tunnel routes the kernel took.
type heldSourceGuard struct {
	spec      sourceGuardSpec
	table     uint32
	rules     map[netip.Addr]bool
	routes    map[netip.Prefix]bool
	unreached bool
}

// guardKernel is the kernel side of the guard: fib rules, routes and the nft table. A
// test stands a recorder in its place; production never replaces kernelGuard.
type guardKernel interface {
	addRule(rule iprule.Source) error
	deleteRule(rule iprule.Source) error
	replaceRoute(route iprule.Route) error
	deleteRoute(route iprule.Route) error
	publish(tables []firewall.Table) error
}

type kernelGuard struct{}

func (kernelGuard) addRule(rule iprule.Source) error      { return rule.Add() }
func (kernelGuard) deleteRule(rule iprule.Source) error   { return rule.Delete() }
func (kernelGuard) replaceRoute(route iprule.Route) error { return route.Replace() }
func (kernelGuard) deleteRoute(route iprule.Route) error  { return route.Delete() }

// publish registers the guard's tables and reconciles the firewall. The registry is
// process-local, which holds: the IKE engine is a component and never runs forked.
func (kernelGuard) publish(tables []firewall.Table) error {
	if err := firewall.RegisterTables(sourceGuardOwner, tables); err != nil {
		return err
	}
	return firewall.ApplyAll()
}

// guardRouteKey names one tunnel route of a guard table: the if_id of the Child SAs
// that carry it, and their remote traffic selector.
type guardRouteKey struct {
	ifID   uint32
	prefix netip.Prefix
}

// sourceGuards is the guard state. The apply path (under applyMu) and the Child SA
// workers (guardTunnelRoute) both reach it, so mu guards every field. Safe for
// concurrent use.
type sourceGuards struct {
	mu   sync.Mutex
	kern guardKernel
	held map[string]*heldSourceGuard
	// published is the set of addresses the nft table drops for, as the last publish
	// that succeeded left it.
	published map[netip.Addr]bool
	// tunnels holds, per tunnel route, the peers whose Child SA carries it: the route
	// leaves a guard table when its last peer goes.
	tunnels map[guardRouteKey]map[string]bool
}

//nolint:gochecknoglobals // One guard per process: the nft table and the fib rules are per namespace.
var guards = newSourceGuards(kernelGuard{})

func newSourceGuards(kern guardKernel) *sourceGuards {
	return &sourceGuards{
		kern:      kern,
		held:      make(map[string]*heldSourceGuard),
		published: make(map[netip.Addr]bool),
		tunnels:   make(map[guardRouteKey]map[string]bool),
	}
}

// guardSources answers the addresses of one interface that need a guard: the IPv6
// addresses xfrmInterfaceExemptions gives a source BYPASS, minus any address a peer
// uses as its IKE local address. refuseTunnelAddressAsIKEEndpoint refuses such a
// configuration at verify; this is the paired check on the apply path, for an address
// the interface gained after the commit. A rule from that address would steer IKE into
// the tunnel, so it gets no guard, and so no source BYPASS either.
func guardSources(name string, facts xfrmInterfaceFacts, cfg *ipsec.IPsecConfig, log *slog.Logger) []netip.Addr {
	locals := ikeLocalAddresses(cfg)
	var out []netip.Addr
	for _, addr := range facts.addresses {
		if !addr.Is6() || addr.Is4In6() {
			continue
		}
		if _, local := locals[addr]; local {
			log.Warn("ike: an address of a bound XFRM interface is also a peer's IKE local address, so it gets no tunnel-source guard and no source exemption; "+
				"IPv6 host traffic from it routed into the interface meets the DISCARD catch-all; "+
				"fix: set the peer's `local-address` to an address of the WAN interface",
				"interface", name, "address", addr.String())
			continue
		}
		out = append(out, addr)
	}
	return out
}

// ikeLocalAddresses answers the IKE local address of every peer that names one, with
// the peer's name.
func ikeLocalAddresses(cfg *ipsec.IPsecConfig) map[netip.Addr]string {
	out := make(map[netip.Addr]string)
	for name := range cfg.Peers {
		addr, err := netip.ParseAddr(cfg.Peers[name].LocalAddress)
		if err != nil {
			// refuse-by-default: guards guardSources -- a peer with no literal local address sends from the address its route selects, never a tunnel address the guard holds
			continue
		}
		out[addr.WithZone("")] = name
	}
	return out
}

// refuseTunnelAddressAsIKEEndpoint refuses, at verify, a peer whose IKE local address
// is an address of an XFRM interface some peer binds (AC-12). The guard steers every
// packet from a tunnel address into the tunnel, so IKE sent from it could never reach
// its peer. An interface that cannot be read yet (it does not exist before its own
// commit) is not judged here: guardSources applies the same check on the apply path
// once it reads.
func refuseTunnelAddressAsIKEEndpoint(cfg *ipsec.IPsecConfig) error {
	locals := ikeLocalAddresses(cfg)
	if len(locals) == 0 {
		return nil
	}
	for _, ifName := range boundXFRMInterfaces(cfg) {
		facts, err := readXFRMInterfaceFn(ifName)
		if err != nil {
			// refuse-by-default: guards guardSources -- the apply path judges the interface once it reads
			continue
		}
		for _, addr := range facts.addresses {
			peer, clash := locals[addr.WithZone("")]
			if !clash {
				continue
			}
			return errors.New("peer " + peer + " uses local-address " + addr.String() +
				", which is an address of XFRM interface " + ifName + " that `vti bind` names; " +
				"traffic from a tunnel address is steered into the tunnel, so IKE from it could never reach the peer; " +
				"fix: set vpn ipsec site-to-site peer " + peer + " local-address to an address of the WAN interface")
		}
	}
	return nil
}

// install brings every interface of want into the guard, keeping what is held beyond
// it: its source BYPASS may still be installed, and prune removes it after that BYPASS.
// It answers the addresses that are guarded now: in the published nft table and
// holding their source rule. The order is the nft table first, then the table's
// routes, then the rules, so no rule steers into a table that is not complete.
func (g *sourceGuards) install(want map[string]sourceGuardSpec, log *slog.Logger) map[netip.Addr]bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(want) == 0 {
		// Nothing to bring in: publishing here would load the firewall backend on every
		// apply of a configuration that guards nothing.
		return g.guardedLocked()
	}

	for _, name := range sortedNames(want) {
		spec := want[name]
		h, held := g.held[name]
		if !held {
			table, err := g.allocateTable()
			if err != nil {
				log.Error("ike: no routing table is free for the tunnel-source guard; the interface's IPv6 source exemptions stay uninstalled",
					"interface", name, "error", err)
				continue
			}
			h = &heldSourceGuard{table: table, rules: make(map[netip.Addr]bool), routes: make(map[netip.Prefix]bool)}
			g.held[name] = h
		}
		sources := h.spec.sources
		for _, addr := range spec.sources {
			if !slices.Contains(sources, addr) {
				sources = append(sources, addr)
			}
		}
		h.spec = spec
		h.spec.sources = sources
	}

	g.publishHeld(log)
	for _, name := range sortedNames(want) {
		h, held := g.held[name]
		if !held {
			continue
		}
		g.installRoutes(h, log)
		for _, addr := range want[name].sources {
			if h.rules[addr] {
				continue
			}
			if err := g.kern.addRule(guardRule(addr, h.table)); err != nil {
				log.Error("ike: could not install the tunnel-source rule; the address's IPv6 source exemption stays uninstalled, "+
					"so IPv6 host traffic from it routed into the interface meets the DISCARD catch-all; "+
					"the daemon needs CAP_NET_ADMIN, and the next commit or address change retries it",
					"interface", name, "address", addr.String(), "table", h.table, "error", err)
				continue
			}
			h.rules[addr] = true
		}
	}
	return g.guardedLocked()
}

// prune removes every held source want no longer names and every interface it no longer
// names: rules first, then routes, then the nft terms. The caller removed the source
// BYPASS entries they guarded first.
func (g *sourceGuards) prune(want map[string]sourceGuardSpec, log *slog.Logger) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.held) == 0 && len(g.published) == 0 {
		return
	}

	for _, name := range sortedNames(g.held) {
		h := g.held[name]
		spec, wanted := want[name]
		var kept []netip.Addr
		for _, addr := range h.spec.sources {
			if wanted && slices.Contains(spec.sources, addr) {
				kept = append(kept, addr)
				continue
			}
			if !h.rules[addr] {
				continue
			}
			if err := g.kern.deleteRule(guardRule(addr, h.table)); err != nil {
				log.Warn("ike: could not remove a tunnel-source rule; it keeps steering traffic from the address into its table until removed; "+
					"fix: ip -6 rule del from "+addr.String()+" table "+textbuf.StringUint32(h.table),
					"interface", name, "error", err)
			}
			delete(h.rules, addr)
		}
		h.spec.sources = kept
		if wanted {
			continue
		}
		g.removeRoutes(name, h, log)
		delete(g.held, name)
	}
	g.publishHeld(log)
}

// heldSpec answers the guard held for one interface, for a plan whose read of that
// interface failed and so keeps what it holds.
func (g *sourceGuards) heldSpec(name string) (sourceGuardSpec, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	h, held := g.held[name]
	if !held {
		return sourceGuardSpec{}, false
	}
	spec := h.spec
	spec.sources = slices.Clone(h.spec.sources)
	return spec, true
}

// guardRule is the source rule of one guarded address.
func guardRule(addr netip.Addr, table uint32) iprule.Source {
	return iprule.Source{Priority: iprule.IPsecSourceGuardPriority, From: netip.PrefixFrom(addr, 128), Table: table}
}

// guardUnreachable is the default route that ends a guard table: a packet from a tunnel
// address that no tunnel route covers is refused rather than handed to the main table.
func guardUnreachable(table uint32) iprule.Route {
	return iprule.Route{Table: table, Dst: netip.PrefixFrom(netip.IPv6Unspecified(), 0), Unreachable: true, Protocol: rtproto.IPsecSourceGuard}
}

// installRoutes makes h's table hold the unreachable default and a route into its
// interface for every tunnel route its if_id carries, and no other tunnel route. A
// route that failed is retried on the next pass.
func (g *sourceGuards) installRoutes(h *heldSourceGuard, log *slog.Logger) {
	if err := g.kern.replaceRoute(guardUnreachable(h.table)); err != nil {
		log.Error("ike: could not install the tunnel-source table's unreachable default; "+
			"a packet from a tunnel address with no tunnel route falls through to the main table and is dropped at postrouting",
			"interface", h.spec.iface, "table", h.table, "error", err)
	} else {
		h.unreached = true
	}
	for key := range g.tunnels {
		if key.ifID != h.spec.ifID {
			continue
		}
		route := iprule.Route{Table: h.table, Dst: key.prefix, LinkIndex: h.spec.ifIndex, Protocol: rtproto.IPsecSourceGuard}
		if err := g.kern.replaceRoute(route); err != nil {
			log.Error("ike: could not install a tunnel route in the tunnel-source table; traffic from the tunnel address to it is refused as unreachable",
				"interface", h.spec.iface, "prefix", key.prefix.String(), "error", err)
			continue
		}
		h.routes[key.prefix] = true
	}
	for prefix := range h.routes {
		if _, carried := g.tunnels[guardRouteKey{ifID: h.spec.ifID, prefix: prefix}]; carried {
			continue
		}
		g.deleteRoute(h, prefix, log)
	}
}

// removeRoutes removes every route of h's table.
func (g *sourceGuards) removeRoutes(name string, h *heldSourceGuard, log *slog.Logger) {
	for prefix := range h.routes {
		g.deleteRoute(h, prefix, log)
	}
	if !h.unreached {
		return
	}
	if err := g.kern.deleteRoute(guardUnreachable(h.table)); err != nil {
		log.Warn("ike: could not remove the tunnel-source table's unreachable default; "+
			"fix: ip -6 route flush table "+textbuf.StringUint32(h.table),
			"interface", name, "error", err)
	}
	h.unreached = false
}

func (g *sourceGuards) deleteRoute(h *heldSourceGuard, prefix netip.Prefix, log *slog.Logger) {
	route := iprule.Route{Table: h.table, Dst: prefix, LinkIndex: h.spec.ifIndex, Protocol: rtproto.IPsecSourceGuard}
	if err := g.kern.deleteRoute(route); err != nil {
		log.Warn("ike: could not remove a tunnel route from the tunnel-source table; "+
			"fix: ip -6 route del "+prefix.String()+" table "+textbuf.StringUint32(h.table),
			"interface", h.spec.iface, "error", err)
	}
	delete(h.routes, prefix)
}

// publishHeld publishes the nft table for every held source, or withdraws it when no
// source is held. A failed publish leaves the last one in the kernel and in published.
func (g *sourceGuards) publishHeld(log *slog.Logger) {
	var terms []firewall.Term
	next := make(map[netip.Addr]bool)
	for _, name := range sortedNames(g.held) {
		h := g.held[name]
		sources := slices.Clone(h.spec.sources)
		slices.SortFunc(sources, netip.Addr.Compare)
		for _, addr := range sources {
			terms = append(terms, sourceGuardTerms(len(terms)/2, addr, h.spec.device)...)
			next[addr] = true
		}
	}
	var tables []firewall.Table
	if len(terms) > 0 {
		tables = []firewall.Table{sourceGuardTable(terms)}
	}
	if err := g.kern.publish(tables); err != nil {
		log.Error("ike: could not publish the tunnel-source guard's nft table; source exemptions it does not cover stay uninstalled; "+
			"the daemon needs CAP_NET_ADMIN, and the next commit or address change retries it",
			"table", sourceGuardTableName, "error", err)
		return
	}
	g.published = next
}

// guardedLocked answers the addresses in the published nft table that hold their rule.
func (g *sourceGuards) guardedLocked() map[netip.Addr]bool {
	out := make(map[netip.Addr]bool)
	for _, h := range g.held {
		for addr := range h.rules {
			if g.published[addr] {
				out[addr] = true
			}
		}
	}
	return out
}

// allocateTable answers the lowest table of iprule.IPsecSourceGuardTables no held guard
// uses. The range bounds the loop.
func (g *sourceGuards) allocateTable() (uint32, error) {
	r := iprule.IPsecSourceGuardTables
	for table := r.First; table <= r.Last; table++ {
		if !g.tableUsed(table) {
			return table, nil
		}
	}
	return 0, errors.New("every table of the IPsec source-guard range " +
		textbuf.StringUint32(r.First) + "-" + textbuf.StringUint32(r.Last) + " is in use")
}

func (g *sourceGuards) tableUsed(table uint32) bool {
	for _, h := range g.held {
		if h.table == table {
			return true
		}
	}
	return false
}

// sourceGuardTerms is the pair of terms for one guarded address: pass it through its
// own device, drop it through any other. A term name accepts letters, digits, '-', '_'
// and '.', so the terms are numbered rather than named after the address.
func sourceGuardTerms(index int, addr netip.Addr, device string) []firewall.Term {
	source := firewall.MatchSourceAddress{Prefix: netip.PrefixFrom(addr, 128)}
	n := strconv.Itoa(index)
	return []firewall.Term{
		{
			Name:    "tunnel-source-" + n + "-own-device",
			Matches: []firewall.Match{source, firewall.MatchOutputInterface{Name: device}},
			Actions: []firewall.Action{firewall.Accept{}},
		},
		{
			Name:    "tunnel-source-" + n + "-other-device",
			Matches: []firewall.Match{source},
			Actions: []firewall.Action{firewall.Drop{}},
		},
	}
}

// sourceGuardTable is the guard's ip6 table: one postrouting base chain that accepts by
// policy, so it decides only what its terms name.
func sourceGuardTable(terms []firewall.Term) firewall.Table {
	return firewall.Table{
		Name:   sourceGuardTableName,
		Family: firewall.FamilyIP6,
		Chains: []firewall.Chain{{
			Name:     sourceGuardChainName,
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookPostrouting,
			Priority: 0,
			Policy:   firewall.PolicyAccept,
			Terms:    terms,
		}},
	}
}

// guardTunnelRoute records that peer's Child SA carries (add) or no longer carries its
// remote traffic selectors, and keeps the guard table of the interface whose if_id the
// Child SA carries in step. Only an IPv6 selector of a Child SA bound to an XFRM
// interface (IfID not 0) has a guard table to enter. It runs where the Child SA's route
// events are emitted (emitRouteAdd, emitRouteRemove).
func guardTunnelRoute(add bool, peer string, child *ChildSA, log *slog.Logger) {
	if child == nil {
		return
	}
	if child.IfID == 0 {
		return
	}
	for _, prefix := range childRemotePrefixes(child) {
		if !prefix.Addr().Is6() {
			continue
		}
		guards.tunnelRoute(add, peer, guardRouteKey{ifID: child.IfID, prefix: prefix}, log)
	}
}

// childRemotePrefixes answers the remote traffic selectors child installs policies for,
// masked and each listed once: the remote half of every negotiated pair that installs a
// policy (childPolicyPair), or TSRemote alone for a Child SA that negotiated no selector
// set, the same choice appendChildPolicies makes. TSRemote holds only the first pair, so
// a Child SA that negotiated IPv4 first and IPv6 second carries an IPv6 selector that
// TSRemote never shows; reading TSRemote alone left that selector out of the guard table,
// and the guard's rule then refused every packet from the tunnel address to it.
func childRemotePrefixes(child *ChildSA) []netip.Prefix {
	out := make([]netip.Prefix, 0, max(1, len(child.Selectors)))
	if len(child.Selectors) == 0 {
		if child.TSRemote == nil {
			return out
		}
		if prefix, ok := netIPNetToPrefix(child.TSRemote); ok {
			out = append(out, prefix.Masked())
		}
		return out
	}
	for _, pair := range child.Selectors {
		pp, ok := childPolicyPair(child, pair)
		if !ok {
			continue
		}
		if pp.remote == nil {
			continue
		}
		prefix, ok := netIPNetToPrefix(pp.remote)
		if !ok {
			continue
		}
		prefix = prefix.Masked()
		if slices.Contains(out, prefix) {
			continue
		}
		out = append(out, prefix)
	}
	return out
}

func (g *sourceGuards) tunnelRoute(add bool, peer string, key guardRouteKey, log *slog.Logger) {
	g.mu.Lock()
	defer g.mu.Unlock()

	peers := g.tunnels[key]
	if add {
		if peers == nil {
			peers = make(map[string]bool)
			g.tunnels[key] = peers
		}
		peers[peer] = true
	} else {
		delete(peers, peer)
		if len(peers) == 0 {
			delete(g.tunnels, key)
		}
	}
	for _, name := range sortedNames(g.held) {
		h := g.held[name]
		if h.spec.ifID == key.ifID {
			g.installRoutes(h, log)
		}
	}
}

// sortedNames answers the keys of m in order, so passes over the guards run stably.
func sortedNames[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
