// Design: docs/guide/configuration.md -- conntrack helper assignment
// Related: engine.go -- the config transaction that verifies, installs and withdraws these tables
// Related: cmd_show_conntrack.go -- show system conntrack lists what this assigned
// Related: ../config/system/conntrack_helpers.go -- the helper declaration table

package firewall

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// configRootConntrack is the `system conntrack` subtree the firewall engine
// takes part in the config transaction for. The engine owns only the helper
// assignment of its `module` leaf-list; the hub still loads the modules and
// writes the conntrack sysctls from the same subtree (cmd/ze/hub/main_system.go).
// The path is nested, so a delivered section is wrapped as
// {"system":{"conntrack":{...}}}.
const configRootConntrack = "system/conntrack"

// conntrackHelperOwner is the table registry owner of the helper tables.
const conntrackHelperOwner = "conntrack-helpers"

// The two helper tables. A helper the kernel registers for both families is
// assigned from the inet table; one it registers for IPv4 alone is assigned
// from the ip table, so no IPv6 rule ever names an object with no IPv6 helper
// behind it.
const (
	conntrackHelperTableInet = tableNamePrefix + "conntrack_helpers"
	conntrackHelperTableIP   = tableNamePrefix + "conntrack_helpers_ip"
)

// conntrackHelperPriority is filter - 5: assignment runs before the
// operator's filter chains at priority filter (0), and after conntrack itself
// (-200), which must have created the entry the helper is attached to.
const conntrackHelperPriority int32 = -5

// conntrackSectionModules answers the helper modules the delivered
// `system conntrack` section lists, and whether the delivery carried that
// section at all. A reload delivers a root only when it changed, so a missing
// section means "unchanged". A block the operator deleted arrives as an empty
// section, which lists no module: its tables are withdrawn, never kept.
func conntrackSectionModules(sections []sdk.ConfigSection) ([]string, bool, error) {
	for _, s := range sections {
		if s.Root != configRootConntrack {
			continue
		}
		if strings.TrimSpace(s.Data) == "" {
			return nil, true, nil
		}
		var root map[string]any
		if err := json.Unmarshal([]byte(s.Data), &root); err != nil {
			return nil, true, fmt.Errorf("system conntrack config: unmarshal: %w", err)
		}
		cc := system.ExtractConntrackFromMap(root)
		return cc.Modules, true, nil
	}
	return nil, false, nil
}

// ConntrackHelperTables lowers the configured helper modules to the tables
// that declare one ct helper object per kernel helper and transport, and
// assign it by destination port in prerouting and output. An empty list
// answers no table: nothing is assigned (RFC 4787 Section 7 REQ-10, "NAT ALGs
// for UDP-based protocols SHOULD be turned off", holds by default).
func ConntrackHelperTables(modules []string) ([]Table, error) {
	if len(modules) == 0 {
		return nil, nil
	}
	inet := newConntrackHelperTable(conntrackHelperTableInet, FamilyInet)
	ip := newConntrackHelperTable(conntrackHelperTableIP, FamilyIP)
	for _, name := range modules {
		helper, err := system.ConntrackHelperByName(name)
		if err != nil {
			return nil, err
		}
		switch helper.Family {
		case system.ConntrackHelperBothFamilies:
			addConntrackHelper(&inet, &helper)
		case system.ConntrackHelperIPv4Only:
			addConntrackHelper(&ip, &helper)
		default:
			return nil, fmt.Errorf("conntrack: helper %q declares no address family", name)
		}
	}
	var tables []Table
	if len(inet.CtHelpers) > 0 {
		tables = append(tables, inet)
	}
	if len(ip.CtHelpers) > 0 {
		tables = append(tables, ip)
	}
	return tables, nil
}

// newConntrackHelperTable builds an empty helper table with its two base
// chains: prerouting for forwarded and inbound connections, output for
// locally originated ones.
func newConntrackHelperTable(name string, family TableFamily) Table {
	return Table{
		Name:   name,
		Family: family,
		Chains: []Chain{
			{Name: "prerouting", IsBase: true, Type: ChainFilter, Hook: HookPrerouting,
				Priority: conntrackHelperPriority, Policy: PolicyAccept},
			{Name: "output", IsBase: true, Type: ChainFilter, Hook: HookOutput,
				Priority: conntrackHelperPriority, Policy: PolicyAccept},
		},
	}
}

// addConntrackHelper adds one object per rule of helper to t, and one term
// per port to each base chain of t.
func addConntrackHelper(t *Table, helper *system.ConntrackHelper) {
	for _, rule := range helper.Rules {
		object := helper.Name + "-" + rule.Protocol
		t.CtHelpers = append(t.CtHelpers, CtHelper{Name: object, Helper: rule.KernelHelper, Protocol: rule.Protocol})
		for _, port := range rule.Ports {
			term := Term{
				Name: object + "-" + strconv.Itoa(int(port)),
				Matches: []Match{
					MatchProtocol{Protocol: rule.Protocol},
					MatchDestinationPort{Ranges: []PortRange{{Lo: port, Hi: port}}},
				},
				Actions: []Action{SetCtHelper{Name: object}},
			}
			for i := range t.Chains {
				t.Chains[i].Terms = append(t.Chains[i].Terms, term)
			}
		}
	}
}

// conntrackHelperApplyError explains a reconcile that failed while helpers
// were listed. The kernel refuses a ct helper object whose helper it does not
// provide, and the reconcile cannot tell which owner's table it refused, so
// the message names that cause as a possibility and gives both fixes.
func conntrackHelperApplyError(modules []string, err error) error {
	if len(modules) == 0 {
		return err
	}
	var b textbuf.Buffer
	b.Str("\nconntrack helpers listed: ").Str(textbuf.Join(modules, ", "))
	b.Str("\nIf the kernel refused a ct helper object, it does not provide that helper,")
	b.Str(" so FTP, SIP and the other listed protocols do not work through NAT.")
	b.Str("\nLoad the helper, then commit again:")
	for _, name := range modules {
		b.Str("\n  modprobe nf_conntrack_").Str(strings.ReplaceAll(name, "-", "_"))
	}
	b.Str("\nOr stop assigning it:")
	for _, name := range modules {
		b.Str("\n  delete system conntrack module ").Str(name)
	}
	return fmt.Errorf("%w%s", err, b.String())
}

// assignedConntrackHelpers answers one row per ct helper object the backend
// last applied, for show system conntrack. It reads LastApplied, the set the
// kernel accepted, so a refused apply, a flush and a closed backend each show
// what the kernel holds rather than what was asked for.
func assignedConntrackHelpers() []map[string]any {
	rows := []map[string]any{}
	applied := LastApplied()
	for i := range applied {
		tbl := &applied[i]
		if tbl.Name != conntrackHelperTableInet && tbl.Name != conntrackHelperTableIP {
			continue
		}
		families := "ipv4 ipv6"
		if tbl.Family == FamilyIP {
			families = "ipv4"
		}
		for _, h := range tbl.CtHelpers {
			rows = append(rows, map[string]any{
				"module":        strings.TrimSuffix(h.Name, "-"+h.Protocol),
				"kernel-helper": h.Helper,
				"protocol":      h.Protocol,
				"ports":         conntrackHelperPorts(tbl, h.Name),
				"families":      families,
			})
		}
	}
	return rows
}

// conntrackHelperPorts answers the destination ports the first chain of tbl
// assigns the ct helper object to. Both chains carry the same terms
// (addConntrackHelper).
func conntrackHelperPorts(tbl *Table, object string) []int {
	var ports []int
	if len(tbl.Chains) == 0 {
		return ports
	}
	for _, term := range tbl.Chains[0].Terms {
		if !slices.ContainsFunc(term.Actions, func(a Action) bool {
			set, ok := a.(SetCtHelper)
			return ok && set.Name == object
		}) {
			continue
		}
		for _, m := range term.Matches {
			if dport, ok := m.(MatchDestinationPort); ok {
				for _, r := range dport.Ranges {
					ports = append(ports, int(r.Lo))
				}
			}
		}
	}
	return ports
}
