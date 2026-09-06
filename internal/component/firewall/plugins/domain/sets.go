// Design: docs/architecture/firewall/firewall-domain-group.md -- nftables set generation from resolved addresses

package domain

import (
	"github.com/ze-software/ze/internal/component/firewall"
)

// maxAddressesPerName caps what one DNS name may contribute to one family of
// one group. DNS is attacker-influenced input and a permit rule keyed on a name
// trusts whoever controls it, so a hijacked answer carrying ten thousand
// addresses must not become ten thousand permitted sources (R-5).
const maxAddressesPerName = 64

// setNames returns the two nftables set names of a group.
//
// The names come from firewall.DomainGroupSetNames rather than from a prefix
// spelled here, because the firewall's config parser turns a rule's
// source-domain-group leaf into a match against the same names. Two spellings
// would be a future disagreement with nothing to arbitrate it.
func setNames(groupName string) (v4, v6 string) {
	return firewall.DomainGroupSetNames(groupName)
}

// buildGroupSets returns BOTH families' sets for a group, or nothing when the
// group holds no address at all.
//
// Both or neither is the rule, and it is not tidiness. The config parser emits
// an IPv6 twin term for every domain-group term
// (expandProvidedTermV6, internal/component/firewall/config.go), so a group
// answering only IPv4 would leave that twin naming a set no owner declared.
// dropTablesMissingAProvidedSet (internal/component/firewall/registry.go) then
// holds back the operator's WHOLE table and the commit still reports success,
// which is a filter silently leaving the kernel.
//
// The family that answered nothing is declared with no elements, and that is
// what its term must read: no address of that family belongs to this group.
//
// A group holding nothing at all still yields no set, so a cold cache keeps the
// table held back until addresses arrive. Verify refuses that config at commit
// time (AC-6), so an operator meets it at the terminal rather than here.
func buildGroupSets(groupName string, v4Addresses, v6Addresses []string) []firewall.Set {
	if len(v4Addresses) == 0 && len(v6Addresses) == 0 {
		return nil
	}
	v4Name, v6Name := setNames(groupName)
	return []firewall.Set{
		{
			Name:     v4Name,
			Type:     firewall.SetTypeIPv4,
			Elements: elements(v4Addresses),
		},
		{
			Name:     v6Name,
			Type:     firewall.SetTypeIPv6,
			Elements: elements(v6Addresses),
		},
	}
}

// elements turns addresses into set elements. There is no interval flag: a
// domain group holds single addresses, which is what a DNS A or AAAA record
// carries, and an interval set of /32 singletons costs the kernel more for the
// same membership test.
func elements(addresses []string) []firewall.SetElement {
	if len(addresses) == 0 {
		return nil
	}
	out := make([]firewall.SetElement, 0, len(addresses))
	for _, addr := range addresses {
		out = append(out, firewall.SetElement{Value: addr})
	}
	return out
}

// buildTables groups the sets by the table whose rules name them, and returns
// one firewall.Table per table that has at least one.
//
// The set must land in the table that references it: ApplyAll merges every
// owner's tables by NAME, so a set registered under a table name no rule uses
// leaves the rule naming a set no owner supplied.
func buildTables(cfg *domainConfig, addresses func(g group, fam family) []string) []firewall.Table {
	if cfg == nil {
		return nil
	}
	byTable := make(map[string]*firewall.Table)
	var order []string

	for _, ref := range cfg.refs {
		g, ok := cfg.groupByName(ref.Group)
		if !ok {
			// A rule naming a group no config defines. The firewall's own
			// verify refuses that config, so reaching here means the reference
			// outlived its group; registering a set for it would invent one.
			continue
		}
		sets := buildGroupSets(g.Name, addresses(g, familyV4), addresses(g, familyV6))
		if len(sets) == 0 {
			continue
		}
		tbl, ok := byTable[ref.TableName]
		if !ok {
			tbl = &firewall.Table{Name: ref.TableName, Family: firewall.FamilyInet}
			byTable[ref.TableName] = tbl
			order = append(order, ref.TableName)
		}
		if declaresSet(tbl, sets[0].Name) {
			// The same group named by two rules of one table. One declaration
			// is what nftables accepts; a second is a duplicate set name.
			continue
		}
		tbl.Sets = append(tbl.Sets, sets...)
	}

	if len(order) == 0 {
		return nil
	}
	tables := make([]firewall.Table, len(order))
	for i, name := range order {
		tables[i] = *byTable[name]
	}
	return tables
}

func declaresSet(tbl *firewall.Table, name string) bool {
	for i := range tbl.Sets {
		if tbl.Sets[i].Name == name {
			return true
		}
	}
	return false
}
