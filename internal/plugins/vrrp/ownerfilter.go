// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the address owner's physical-MAC answers
// RFC: rfc/short/rfc9568.md (VRRPv3) -- Sections 8.1.2 and 8.2.2
// Related: acceptfilter.go -- the same firewall-registry route for the Accept_Mode filter
// Related: instance.go -- doInstallVIPs and doRemoveVIPs own this filter's lifetime
//
// The address owner's virtual address is also a real address of the parent
// interface, so the kernel answers ARP and Neighbor Solicitations for it on the
// parent, with the parent's physical MAC. The virtual-MAC macvlan holds the
// same address and answers too, with the virtual MAC. `arp_ignore` cannot
// muzzle the parent, because the address is local to it.
//
// RFC 3768 Section 8.2 and RFC 9568 Sections 8.1.2 and 8.2.2 forbid the
// physical-MAC answer. This file drops it on its way out: an ARP reply leaving
// the parent with the virtual address as its sender, at the arp family's output
// hook, and a Neighbor Advertisement leaving the parent for the virtual address
// as its target, at the ip6 output hook. The macvlan's own answers leave through
// the macvlan, so no rule here names them.
//
// Only replies and advertisements are dropped. An ARP request the router sends
// from the parent while resolving a neighbor is its own traffic, and dropping it
// would stop the router reaching the LAN.
//
// The rules reach the kernel through the firewall component's table registry,
// under an owner separate from the Accept_Mode filter's, because the two sets
// change at different moments and one owner registration replaces the other.
package vrrp

import (
	"net/netip"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// ownerFilterARPTableName and ownerFilterNDTableName carry the "ze_"
	// ownership prefix RegisterTables requires.
	ownerFilterARPTableName = "ze_vrrp_owner_arp"
	ownerFilterNDTableName  = "ze_vrrp_owner_nd"
	// ownerFilterChainName is the base chain at the output hook, where a reply
	// the kernel generated names the interface it leaves by.
	ownerFilterChainName = "output"
	// ownerFilterOwner is this filter's identity in the firewall table
	// registry, distinct from acceptFilterOwner.
	ownerFilterOwner = "vrrp-owner"
)

// ownerFilterEntry is one instance's parent device and the virtual addresses
// that parent also holds as real addresses.
type ownerFilterEntry struct {
	parent string
	vips   []netip.Addr
}

// ownerFilterState holds, for each VRRP instance that has installed virtual
// addresses it owns, the entry whose answers the parent must not send.
//
// Process-wide, because one kernel table per family carries every group's
// rules. Bounded by the configured group count and each entry by that group's
// virtual-address count, both operator configuration.
var (
	ownerFilterMu    sync.Mutex
	ownerFilterState = map[string]ownerFilterEntry{}
)

// ownedVIPs returns the virtual addresses among vips that the parent also holds
// as real addresses. Those are the addresses the parent answers for with its
// physical MAC, and the only ones the owner filter names.
func (s GroupSpec) ownedVIPs(vips []netip.Addr) []netip.Addr {
	var owned []netip.Addr
	for _, vip := range vips {
		if slices.Contains(s.realAddresses, vip) {
			owned = append(owned, vip)
		}
	}
	return owned
}

// setOwnerFilter records that parent MUST NOT answer for vips, and reconciles
// the kernel. An empty vips withdraws the instance's entry, which is the case
// of every group that is not the address owner. The caller MUST call
// clearOwnerFilter when the instance gives its addresses up, or the parent
// stays silent for an address it is again the only holder of.
func setOwnerFilter(instanceOwner, parent string, vips []netip.Addr) error {
	ownerFilterMu.Lock()
	defer ownerFilterMu.Unlock()

	if len(vips) == 0 {
		return dropOwnerFilterEntryLocked(instanceOwner)
	}
	if current, ok := ownerFilterState[instanceOwner]; ok && current.parent == parent && slices.Equal(current.vips, vips) {
		return nil
	}
	ownerFilterState[instanceOwner] = ownerFilterEntry{parent: parent, vips: slices.Clone(vips)}
	return reconcileOwnerFilterLocked()
}

// clearOwnerFilter withdraws one instance's entry. It MUST be called after
// setOwnerFilter once the virtual-MAC device stops holding the addresses.
func clearOwnerFilter(instanceOwner string) error {
	ownerFilterMu.Lock()
	defer ownerFilterMu.Unlock()

	return dropOwnerFilterEntryLocked(instanceOwner)
}

// dropOwnerFilterEntryLocked removes one instance's entry, and reconciles only
// if the entry was there to remove, so a group that owns nothing never loads
// the firewall backend.
func dropOwnerFilterEntryLocked(instanceOwner string) error {
	if _, ok := ownerFilterState[instanceOwner]; !ok {
		return nil
	}
	delete(ownerFilterState, instanceOwner)
	return reconcileOwnerFilterLocked()
}

// reconcileOwnerFilterLocked publishes the current entries and reconciles the
// kernel. Synchronous under ownerFilterMu for the reason
// reconcileAcceptFilterLocked gives: two instances applying snapshots out of
// order would leave the kernel holding the older one.
func reconcileOwnerFilterLocked() error {
	entries := make([]ownerFilterEntry, 0, len(ownerFilterState))
	for _, entry := range ownerFilterState {
		entries = append(entries, entry)
	}
	return ownerFilterPublish(ownerFilterTables(entries))
}

// ownerFilterPublish hands the desired tables to the firewall component. It is
// a var so a test can watch what this package publishes without a kernel.
var ownerFilterPublish = func(tables []firewall.Table) error {
	if err := firewall.RegisterTables(ownerFilterOwner, tables); err != nil {
		return err
	}
	return firewall.ApplyAll()
}

// ownerFilterRule is one (parent, address) pair the filter drops answers for.
type ownerFilterRule struct {
	parent string
	vip    netip.Addr
}

// ownerFilterTables builds the arp table for the IPv4 pairs and the ip6 table
// for the IPv6 pairs. A family with no pair gets no table, so the last owner to
// leave withdraws both instead of leaving empty tables behind.
//
// The rules are sorted and deduplicated: map iteration order is not stable,
// and the kernel rules would otherwise be rewritten on every apply.
func ownerFilterTables(entries []ownerFilterEntry) []firewall.Table {
	var rules []ownerFilterRule
	for _, entry := range entries {
		for _, vip := range entry.vips {
			rules = append(rules, ownerFilterRule{parent: entry.parent, vip: vip})
		}
	}
	slices.SortFunc(rules, func(a, b ownerFilterRule) int {
		if c := strings.Compare(a.parent, b.parent); c != 0 {
			return c
		}
		return a.vip.Compare(b.vip)
	})
	rules = slices.Compact(rules)

	var arpTerms, ndTerms []firewall.Term
	for _, rule := range rules {
		if rule.vip.Is4() {
			arpTerms = append(arpTerms, ownerARPReplyTerm(rule))
			continue
		}
		ndTerms = append(ndTerms, ownerNDAdvertTerm(rule))
	}

	var tables []firewall.Table
	if len(arpTerms) > 0 {
		tables = append(tables, ownerFilterTable(ownerFilterARPTableName, firewall.FamilyARP, arpTerms))
	}
	if len(ndTerms) > 0 {
		tables = append(tables, ownerFilterTable(ownerFilterNDTableName, firewall.FamilyIP6, ndTerms))
	}
	return tables
}

// ownerARPReplyTerm drops an ARP reply that leaves the parent with the virtual
// address as its sender protocol address: the physical-MAC answer RFC 9568
// Section 8.1.2 forbids.
func ownerARPReplyTerm(rule ownerFilterRule) firewall.Term {
	// RFC 9568 Section 8.1.2: "The Active Router MUST NOT respond with its
	// physical MAC address in the ARP response."
	return firewall.Term{
		Name: ownerFilterTermName("arp-", rule),
		Matches: []firewall.Match{
			firewall.MatchOutputInterface{Name: rule.parent},
			firewall.MatchARPOperation{Operation: firewall.ARPOperationReply},
			firewall.MatchARPSenderAddress{Addr: rule.vip},
		},
		Actions: []firewall.Action{firewall.Drop{}},
	}
}

// ownerNDAdvertTerm drops a Neighbor Advertisement that leaves the parent for
// the virtual address, solicited or not: the physical-MAC answer RFC 9568
// Section 8.2.2 forbids.
func ownerNDAdvertTerm(rule ownerFilterRule) firewall.Term {
	// RFC 9568 Section 8.2.2: "The Active Router MUST NOT respond with its
	// physical MAC address."
	return firewall.Term{
		Name: ownerFilterTermName("nd-", rule),
		Matches: []firewall.Match{
			firewall.MatchOutputInterface{Name: rule.parent},
			firewall.MatchICMPv6Type{Type: icmpv6TypeNeighborAdvert},
			firewall.MatchNDTargetAddress{Addr: rule.vip},
		},
		Actions: []firewall.Action{firewall.Drop{}},
	}
}

// ownerFilterTable is one family's table: a single output base chain that
// accepts by policy, so it decides only what its terms name.
func ownerFilterTable(name string, family firewall.TableFamily, terms []firewall.Term) firewall.Table {
	return firewall.Table{
		Name:   name,
		Family: family,
		Chains: []firewall.Chain{{
			Name:     ownerFilterChainName,
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookOutput,
			Priority: 0,
			Policy:   firewall.PolicyAccept,
			Terms:    terms,
		}},
	}
}

// ownerFilterTermName names one rule after its kind, parent and address. A
// firewall name accepts letters, digits, '-', '_' and '.' (firewall.ValidateName),
// so the colons of an IPv6 address become hyphens. The kind prefix makes the
// first character a letter.
func ownerFilterTermName(kind string, rule ownerFilterRule) string {
	text := textbuf.StringAddr(rule.vip)

	var tb textbuf.Buffer
	tb.Str(kind).Str(rule.parent).Byte('-')
	for i := range len(text) {
		c := text[i]
		if c == ':' {
			c = '-'
		}
		tb.Byte(c)
	}
	return tb.String()
}
