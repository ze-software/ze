// Design: docs/architecture/policyroute/policy-routing.md -- a @set reference resolves to a firewall table's set
// Related: translate.go -- translate copies each resolved set into the ze_pr table
// Related: register.go -- ConfigReads delivers the firewall section beside policy/route

package policyroute

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/firewall"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// parsePolicySections reads the policies of one delivery and resolves every
// @set reference they make against the firewall section of the same delivery.
//
// Because firewall is a read root, the engine delivers it whole beside
// policy/route whenever either root changes (reloadConfigSections,
// internal/component/plugin/server/reload_compensation.go), and so does
// `ze config validate` (ConfigSectionRoots). A delivery without a policy/route
// section therefore means the configuration holds no policy route, and the
// answer is no policies. A delivery without a firewall section means the
// configuration holds no firewall section, so no set can resolve.
func parsePolicySections(sections []sdk.ConfigSection) ([]PolicyRoute, error) {
	var policies []PolicyRoute
	firewallData := ""
	for _, section := range sections {
		switch section.Root {
		case configRoot:
			parsed, err := parsePolicyConfig(section.Data)
			if err != nil {
				return nil, err
			}
			policies = parsed
		case firewall.ConfigRoot:
			firewallData = section.Data
		}
	}
	if err := resolveSetReferences(policies, firewallData); err != nil {
		return nil, err
	}
	return policies, nil
}

// verifyPolicySections is what both verifiers run: the parse and the set
// resolution, then the translation the apply will run. The translation runs
// on a fresh allocator, which holds the only state it writes, so it touches
// nothing the plugin or the kernel holds. Running it here is what makes a
// refusal only translate can give (a tcp-flags or port value it cannot read,
// the mark or table ranges running out) a refusal at commit, not at apply.
func verifyPolicySections(sections []sdk.ConfigSection) ([]PolicyRoute, error) {
	policies, err := parsePolicySections(sections)
	if err != nil {
		return nil, err
	}
	if _, err := newAllocator().translate(policies); err != nil {
		return nil, err
	}
	return policies, nil
}

// declaredSet is one set a firewall table declares, with the table's name as
// the configuration spells it.
type declaredSet struct {
	table string
	set   firewall.Set
}

// setLeaf is one leaf of a policy rule's from block that can name a set: what
// the operator wrote, where the resolved set goes, and whether the leaf reads
// ports or addresses, which decides the set type it accepts.
type setLeaf struct {
	name   string
	value  string
	target **firewall.Set
	port   bool
}

func ruleSetLeaves(match *PolicyMatch) [4]setLeaf {
	return [4]setLeaf{
		{name: "source-address", value: match.SourceAddress, target: &match.SourceSet},
		{name: "destination-address", value: match.DestinationAddress, target: &match.DestinationSet},
		{name: "source-port", value: match.SourcePort, target: &match.SourcePortSet, port: true},
		{name: "destination-port", value: match.DestinationPort, target: &match.DestinationPortSet, port: true},
	}
}

// resolveSetReferences attaches to every @name address or port match the set
// a firewall table declares under that name, and refuses a reference that
// resolves to no set, to more than one, or to a set the leaf cannot read. It
// runs at verify, so the operator meets the refusal at commit rather than as a
// failed nftables apply.
//
// An nftables set resolves only inside its own table, so the ze_pr chain
// cannot read the firewall table's set. translate copies the resolved set into
// ze_pr instead, and this resolution is the only place the copy is taken from.
//
// The firewall section is parsed only when a reference exists, so a policy that
// names no set does not refuse a firewall section the firewall engine itself
// refuses at the same verify, with its own message.
func resolveSetReferences(policies []PolicyRoute, firewallData string) error {
	if !referencesASet(policies) {
		return nil
	}
	catalog, err := firewallSetCatalog(firewallData)
	if err != nil {
		return err
	}
	for i := range policies {
		for j := range policies[i].Rules {
			rule := &policies[i].Rules[j]
			for _, leaf := range ruleSetLeaves(&rule.Match) {
				if err := resolveLeaf(catalog, leaf); err != nil {
					return fmt.Errorf("policy route %q: rule %q: %w", policies[i].Name, rule.Name, err)
				}
			}
		}
	}
	return nil
}

func referencesASet(policies []PolicyRoute) bool {
	for i := range policies {
		for j := range policies[i].Rules {
			for _, leaf := range ruleSetLeaves(&policies[i].Rules[j].Match) {
				if isSetReference(leaf.value) {
					return true
				}
			}
		}
	}
	return false
}

func isSetReference(value string) bool {
	return strings.HasPrefix(value, "@")
}

// firewallSetCatalog indexes every set the firewall section declares by its
// name. One name can be declared by several tables, because a set name is
// table-local in the firewall section; the catalog keeps every declaration so
// resolveLeaf can refuse the ambiguity instead of picking one.
func firewallSetCatalog(firewallData string) (map[string][]declaredSet, error) {
	catalog := make(map[string][]declaredSet)
	if firewallData == "" {
		return catalog, nil
	}
	tables, err := firewall.ParseFirewallConfig(firewallData)
	if err != nil {
		return nil, fmt.Errorf("reading the firewall sets a policy route references: %w", err)
	}
	for i := range tables {
		table := firewall.StripZeTablePrefix(tables[i].Name)
		for _, set := range tables[i].Sets {
			catalog[set.Name] = append(catalog[set.Name], declaredSet{table: table, set: set})
		}
	}
	return catalog, nil
}

// resolveLeaf resolves one leaf. A value that is not a @set reference is a
// prefix or a port list, which translate reads; it leaves the target nil.
func resolveLeaf(catalog map[string][]declaredSet, leaf setLeaf) error {
	if !isSetReference(leaf.value) {
		return nil
	}
	name := leaf.value[1:]
	declared := catalog[name]
	if len(declared) == 0 {
		return undeclaredSetError(leaf, name)
	}
	if len(declared) > 1 {
		tables := make([]string, 0, len(declared))
		for _, d := range declared {
			tables = append(tables, d.table)
		}
		slices.Sort(tables)
		return fmt.Errorf("%s %s: firewall tables %s each declare a set named %q, and a policy route cannot tell which one it means; "+
			"rename the set in all but one of those tables",
			leaf.name, leaf.value, strings.Join(tables, ", "), name)
	}
	found := declared[0]
	if err := checkSetType(leaf, found); err != nil {
		return err
	}
	set := found.set
	*leaf.target = &set
	return nil
}

// undeclaredSetError explains a name no firewall table declares. A name another
// plugin fills at run time (an IRR or a domain-group set) is not declared
// anywhere, and declaring it is not the fix, so that case says so.
func undeclaredSetError(leaf setLeaf, name string) error {
	if plugin, firewallLeaf, provided := firewall.ProvidedSetOwner(name); provided {
		return fmt.Errorf("%s %s: set %q is filled at run time by the %s plugin, not declared in a firewall table, "+
			"and a policy route can copy only a set a firewall table declares; "+
			"match that traffic with %s in a firewall table term instead, "+
			"or declare a firewall table set holding the elements and reference that",
			leaf.name, leaf.value, name, plugin, firewallLeaf)
	}
	example := "type ipv4; element <address> {}"
	if leaf.port {
		example = "type inet-service; element <port> {}"
	}
	return fmt.Errorf("%s %s: no firewall table declares a set named %q, and a policy route matches only a set a firewall table declares; "+
		"declare it with `firewall { table <table> { set %s { %s } } }`",
		leaf.name, leaf.value, name, name, example)
}

// checkSetType refuses a set whose elements the leaf cannot read: an address
// leaf reads an ipv4 or ipv6 set, a port leaf an inet-service set. The ze_pr
// table is inet, where both address families are valid
// (firewall.validateSetFamilyCompat).
func checkSetType(leaf setLeaf, found declaredSet) error {
	setType := found.set.Type
	if leaf.port {
		if setType != firewall.SetTypeInetService {
			return fmt.Errorf("%s %s: set %q in firewall table %q holds %s elements, and a port match reads only an inet-service set; "+
				"reference a set declared with `type inet-service`",
				leaf.name, leaf.value, found.set.Name, found.table, setType)
		}
		return nil
	}
	if setType != firewall.SetTypeIPv4 && setType != firewall.SetTypeIPv6 {
		return fmt.Errorf("%s %s: set %q in firewall table %q holds %s elements, and an address match reads only an ipv4 or ipv6 set; "+
			"reference a set declared with `type ipv4` or `type ipv6`",
			leaf.name, leaf.value, found.set.Name, found.table, setType)
	}
	return nil
}
