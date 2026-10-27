// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- doctor check
//
// The check compares the kernel with what this configuration asks for. The
// expected table is not listed here: it is translatePolicy's answer for the
// configuration the doctor runner loaded, parsed by the same parseCoppSections
// the plugin's verify runs, so a term added to the ACL is checked with no edit
// to this file. A table the kernel holds is read back through the firewall
// backend, which preserves each term's name.

package copp

import (
	"errors"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const doctorCoppCode = "doctor-copp-missing"

// restoreHint is the command that installs the ze_copp table again when the
// kernel lost part of it. A reload is not it: a reload that changes neither
// root copp reads does not reach copp. A restart delivers the configuration
// to copp's start handler, which installs the whole table.
const restoreHint = "Run `ze signal restart` so the daemon installs the table again, then rerun `ze doctor`."

// readKernelTables reads the ze-owned tables the kernel holds, through the
// firewall backend this process loaded. It is a var so the unit test can stand
// in a kernel; nothing else assigns it.
var readKernelTables = func() ([]firewall.Table, error) {
	backend := firewall.GetBackend()
	if backend == nil {
		return nil, errNoFirewallBackend
	}
	return backend.ListTables()
}

var errNoFirewallBackend = errors.New("no firewall backend is loaded in this process")

func checkCoppInputChain(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	cppTree := tree.GetContainer(configRoot)
	if cppTree == nil {
		return nil
	}
	if cppTree.GetContainer("bgp") == nil {
		return nil
	}

	want, err := expectedTable(tree)
	if err != nil {
		return []rpc.DoctorCheckDiagnostic{coppDiagnostic(textbuf.New().
			Str("copp: the control-plane-protection bgp configuration does not parse, so the ze_copp table cannot be built: ").
			Err(err).Str(". Fix the configuration, then check it with `ze config validate <config-file>`.").String())}
	}

	tables, err := readKernelTables()
	if err != nil {
		return []rpc.DoctorCheckDiagnostic{coppDiagnostic(textbuf.New().
			Str("copp: the kernel firewall tables could not be read (").Err(err).
			Str("), so whether the ze_copp table and its unknown-source-drop term are installed is unknown. ").
			Str(restoreHint).String())}
	}

	missing := missingFromKernel(want, tables)
	if len(missing) == 0 {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{coppDiagnostic(textbuf.New().
		Str("copp: the kernel does not hold ").Join(missing, ", ").
		Str(" of the ze_copp table, so port ").Str(protectedPortsText(want)).
		Str(" is not protected as configured. ").Str(restoreHint).String())}
}

// expectedTable builds the table this configuration installs, from the same
// sections the engine delivers to the plugin: control-plane-protection, and
// the bgp root copp reads for its neighbors.
func expectedTable(tree *config.Tree) (firewall.Table, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return firewall.Table{}, err
	}
	// PruneInactive modifies in place, and the doctor runner shares one tree
	// across every later check.
	clone := tree.Clone()
	config.PruneInactive(clone, schema)

	sections, err := config.BuildPluginConfigSections(clone.ToPluginMap(), []string{configRoot, bgpConfigRoot})
	if err != nil {
		return firewall.Table{}, err
	}
	for _, section := range sections {
		if section.Root != configRoot {
			continue
		}
		policy, found, err := parseCoppSections(sections, section.Data)
		if err != nil {
			return firewall.Table{}, err
		}
		if !found {
			return firewall.Table{}, errNoCoppPolicy
		}
		return translatePolicy(policy), nil
	}
	return firewall.Table{}, errNoCoppPolicy
}

var errNoCoppPolicy = errors.New("the control-plane-protection section holds no bgp policy")

// missingFromKernel names each part of want the kernel tables do not hold:
// the table itself, a chain, a term of a chain by its name, or a set.
func missingFromKernel(want firewall.Table, tables []firewall.Table) []string {
	var have *firewall.Table
	for i := range tables {
		if tables[i].Name == want.Name && tables[i].Family == want.Family {
			have = &tables[i]
			break
		}
	}
	if have == nil {
		return []string{"the whole table (inet " + want.Name + ")"}
	}

	var missing []string
	for _, chain := range want.Chains {
		haveTerms, found := chainTermNames(have, chain.Name)
		if !found {
			missing = append(missing, "chain "+chain.Name)
			continue
		}
		for _, term := range chain.Terms {
			if _, held := haveTerms[term.Name]; !held {
				missing = append(missing, "term "+term.Name+" in chain "+chain.Name)
			}
		}
	}
	for _, set := range want.Sets {
		if !holdsSet(have, set.Name) {
			missing = append(missing, "set "+set.Name)
		}
	}
	return missing
}

func chainTermNames(table *firewall.Table, chainName string) (map[string]struct{}, bool) {
	for _, chain := range table.Chains {
		if chain.Name != chainName {
			continue
		}
		names := make(map[string]struct{}, len(chain.Terms))
		for _, term := range chain.Terms {
			names[term.Name] = struct{}{}
		}
		return names, true
	}
	return nil, false
}

func holdsSet(table *firewall.Table, setName string) bool {
	for _, set := range table.Sets {
		if set.Name == setName {
			return true
		}
	}
	return false
}

// protectedPortsText reads the protected ports back from the drop term, the
// one term every built table carries, so the message names what the operator
// configured.
func protectedPortsText(table firewall.Table) string {
	tb := textbuf.New()
	for _, chain := range table.Chains {
		for _, term := range chain.Terms {
			if term.Name != unknownSourceDrop {
				continue
			}
			for _, match := range term.Matches {
				ports, ok := match.(firewall.MatchDestinationPort)
				if !ok {
					continue
				}
				for i, r := range ports.Ranges {
					if i > 0 {
						tb.Str(",")
					}
					tb.Uint(uint64(r.Lo))
				}
			}
		}
	}
	return tb.String()
}

func coppDiagnostic(message string) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     doctorCoppCode,
		Severity: "warning",
		Message:  message,
	}
}
