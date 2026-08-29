// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- Accept_Mode dataplane enforcement
// RFC: rfc/short/rfc9568.md (VRRPv3) -- Section 6.1 Accept_Mode, Section 6.4.3 Active
// Related: instance.go -- doInstallVIPs and doRemoveVIPs own this filter's lifetime
//
// RFC 9568 Section 6.4.3 states what an Active router does with a packet sent to
// a virtual address: "It MUST accept packets addressed to the IPvX address(es)
// associated with the Virtual Router if it is the IPvX address owner or if
// Accept_Mode is True. Otherwise, it MUST NOT accept these packets."
//
// The address stays installed either way. The same section requires the Active
// router to answer ARP for an IPv4 virtual address, and to be a member of the
// Solicited-Node multicast group and answer Neighbor Solicitations for an IPv6
// one. On Linux all three follow from the address being configured on the
// virtual-MAC macvlan, so suppression cannot be "do not install the address":
// that answers one MUST NOT by breaking two MUSTs, and the gateway stops
// resolving. What is suppressed is LOCAL DELIVERY alone, at the input hook.
// Forwarding for the virtual MAC runs at the forward hook and is untouched, and
// ARP is not carried by the inet family at all, so no rule here can reach it.
//
// One exception is written into the rule order. Section 6.1 and Section 6.4.3
// both state that IPv6 Neighbor Solicitations and Neighbor Advertisements MUST
// NOT be dropped when Accept_Mode is False, so ICMPv6 types 135 and 136 are
// accepted by this chain before any address is examined.
//
// The rules reach the kernel through the firewall component's table registry,
// under one owner for the whole plugin, as copp, ddos-local and
// flowspec-firewall already do. VRRP therefore gains no nftables code, no
// second writer of the kernel firewall and no platform-specific file: the
// firewall backend owns the kernel, its host-namespace safety gate covers these
// rules too, and `show firewall ruleset` lists them beside every other ze table.
package vrrp

import (
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// acceptFilterTableName carries the "ze_" ownership prefix the firewall
	// backend uses to recognize a ze-managed kernel table. RegisterTables
	// refuses a name without it.
	acceptFilterTableName = "ze_vrrp"
	// acceptFilterChainName is the base chain at the input hook. Local delivery
	// is what RFC 9568 Section 6.4.3 calls accepting the packet.
	acceptFilterChainName = "input"
	// acceptFilterOwner is this plugin's identity in the firewall table
	// registry. One owner for the whole plugin, because every group's rules
	// share one table and one base chain: two owners registering a chain of the
	// same name into the same table would collide when the registry merges them.
	acceptFilterOwner = "vrrp"
)

// The two ICMPv6 types RFC 9568 Section 6.1 keeps out of this filter.
const (
	icmpv6TypeNeighborSolicit = 135
	icmpv6TypeNeighborAdvert  = 136
)

// acceptFilterState holds, for each VRRP instance, the virtual addresses whose
// local delivery must be suppressed. An instance has an entry only while it
// holds addresses it is not allowed to accept traffic for; every other instance
// withdraws its entry, and an empty map withdraws the table.
//
// Process-wide, because one kernel table carries every group's rules. Bounded
// by the configured group count, and each entry by that group's virtual-address
// count. Both are operator configuration, so the rule count is bounded by the
// config the operator wrote and never by anything a peer sends.
var (
	acceptFilterMu    sync.Mutex
	acceptFilterState = map[string][]netip.Addr{}
)

// setAcceptFilter records one instance's RFC 9568 Section 6.4.3 acceptance
// decision and reconciles the kernel. accept true withdraws the instance's
// suppression, which covers both of the section's accepting cases: the address
// owner, and Accept_Mode True.
//
// instanceOwner is the caller's address-owner identity (ownerString), so the
// entry is keyed the same way the addresses themselves are.
func setAcceptFilter(instanceOwner string, vips []netip.Addr, accept bool) error {
	acceptFilterMu.Lock()
	defer acceptFilterMu.Unlock()

	if accept {
		return dropAcceptFilterEntryLocked(instanceOwner)
	}
	if current, ok := acceptFilterState[instanceOwner]; ok && slices.Equal(current, vips) {
		return nil
	}
	acceptFilterState[instanceOwner] = slices.Clone(vips)
	return reconcileAcceptFilterLocked()
}

// clearAcceptFilter withdraws one instance's suppression entry. The caller MUST
// call it when the instance gives its virtual addresses up, or a Backup leaves
// a drop rule standing over an address the next Active is allowed to answer on.
func clearAcceptFilter(instanceOwner string) error {
	acceptFilterMu.Lock()
	defer acceptFilterMu.Unlock()

	return dropAcceptFilterEntryLocked(instanceOwner)
}

// dropAcceptFilterEntryLocked removes one instance's entry, and reconciles only
// if the entry was there to remove.
//
// The early return is what keeps this feature off a deployment that does not use
// it. Every promotion calls setAcceptFilter, so without it an accept-mode-true
// group would reach firewall.ApplyAll on every transition, load the nftables
// backend on a box that configured no firewall at all, and pay a kernel round
// trip to publish a set that has not changed.
func dropAcceptFilterEntryLocked(instanceOwner string) error {
	if _, ok := acceptFilterState[instanceOwner]; !ok {
		return nil
	}
	delete(acceptFilterState, instanceOwner)
	return reconcileAcceptFilterLocked()
}

// reconcileAcceptFilterLocked publishes the current suppression set to the
// firewall table registry and reconciles the kernel.
//
// The registration and the apply both run under acceptFilterMu so that two
// instances changing state at the same moment cannot apply their snapshots out
// of order and leave the kernel holding the older one. The firewall registry
// takes its own process-wide reconcile lock inside ApplyAll, so this adds no
// new lock ordering: nothing under internal/component/firewall calls back here.
//
// The apply is SYNCHRONOUS, and the caller holds its instance lock across it, so
// a wedged kernel firewall delays that instance's next advertisement. That cost
// is paid deliberately. RFC 9568 Section 6.4.3 states the prohibition over the
// router's whole time in the Active state, and only an ordered install gives it:
// a filter published on a worker could land after the address it governs, which
// is the window the rule exists to close. The cost is bounded by how often it is
// paid -- once per promotion, per demotion and per accept-mode change, never per
// advertisement -- and internal/component/firewall times every apply and reports
// a slow one (metrics.go, observeApply).
func reconcileAcceptFilterLocked() error {
	return acceptFilterPublish(acceptFilterTables(suppressedAddressesLocked()))
}

// acceptFilterPublish hands the desired tables to the firewall component, which
// owns the kernel. It is a var so a test can watch what this package publishes
// without a kernel to publish it to, which is the seam dataplane_linux.go
// already uses for its sysctl reads and writes.
var acceptFilterPublish = func(tables []firewall.Table) error {
	if err := firewall.RegisterTables(acceptFilterOwner, tables); err != nil {
		return err
	}
	return firewall.ApplyAll()
}

// suppressedAddressesLocked flattens the per-instance entries into one sorted,
// deduplicated address list. Sorted because map iteration order is not stable
// and the kernel rules would otherwise be rewritten on every apply.
// Deduplicated because two groups suppressing one address owe one rule, and
// because the term name is derived from the address.
func suppressedAddressesLocked() []netip.Addr {
	var addrs []netip.Addr
	for _, vips := range acceptFilterState {
		addrs = append(addrs, vips...)
	}
	slices.SortFunc(addrs, func(a, b netip.Addr) int { return a.Compare(b) })
	return slices.Compact(addrs)
}

// acceptFilterTables builds the firewall table that stops the kernel accepting
// packets addressed to addrs.
//
// It returns no table when addrs is empty, so the last instance to stop
// suppressing withdraws the kernel table instead of leaving an empty one behind.
func acceptFilterTables(addrs []netip.Addr) []firewall.Table {
	if len(addrs) == 0 {
		return nil
	}

	// The two carve-out terms come first. A packet takes the verdict of the
	// first rule it matches, so a Neighbor Solicitation or Advertisement sent to
	// a suppressed address is accepted before any drop below can see it
	// (RFC 9568 Section 6.1: they MUST NOT be dropped when Accept_Mode is False).
	terms := make([]firewall.Term, 0, 2+len(addrs))
	terms = append(terms,
		firewall.Term{
			Name:    "nd-neighbor-solicit",
			Matches: []firewall.Match{firewall.MatchICMPv6Type{Type: icmpv6TypeNeighborSolicit}},
			Actions: []firewall.Action{firewall.Accept{}},
		},
		firewall.Term{
			Name:    "nd-neighbor-advert",
			Matches: []firewall.Match{firewall.MatchICMPv6Type{Type: icmpv6TypeNeighborAdvert}},
			Actions: []firewall.Action{firewall.Accept{}},
		},
	)
	for _, addr := range addrs {
		terms = append(terms, firewall.Term{
			Name: acceptFilterTermName(addr),
			// A host route: the rule names this address and no other.
			Matches: []firewall.Match{
				firewall.MatchDestinationAddress{Prefix: netip.PrefixFrom(addr, addr.BitLen())},
			},
			Actions: []firewall.Action{firewall.Drop{}},
		})
	}

	return []firewall.Table{{
		Name:   acceptFilterTableName,
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:   acceptFilterChainName,
			IsBase: true,
			Type:   firewall.ChainFilter,
			// The input hook is local delivery, which is the "accept" of
			// RFC 9568 Section 6.4.3. The same section's forwarding requirement
			// runs at the forward hook and is never reached from here.
			Hook: firewall.HookInput,
			// Ordinary filter priority. The position is not load-bearing: a base
			// chain that accepts a packet does not stop another base chain at
			// the same hook from running, so this chain's drop stands wherever
			// it sits relative to the operator's own rules.
			Priority: 0,
			// Accept, so this table decides only what it names. A drop policy
			// would refuse every packet the two carve-outs and the address rules
			// do not mention.
			Policy: firewall.PolicyAccept,
			Terms:  terms,
		}},
	}}
}

// acceptFilterTermName names the drop rule for one virtual address.
//
// A firewall name accepts letters, digits, '-', '_' and '.', and refuses a
// leading '-' or '.' (firewall.ValidateName). So the colons of an IPv6 address
// become hyphens, and the "drop-" prefix is not decoration: it guarantees the
// first character is a letter whatever the address looks like.
func acceptFilterTermName(addr netip.Addr) string {
	text := textbuf.StringAddr(addr)

	var tb textbuf.Buffer
	tb.Str("drop-")
	for i := range len(text) {
		c := text[i]
		if c == ':' {
			c = '-'
		}
		tb.Byte(c)
	}
	return tb.String()
}
