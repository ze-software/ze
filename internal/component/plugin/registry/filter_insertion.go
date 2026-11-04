// Design: docs/architecture/api/process-protocol.md -- Filter Insertion
// Related: filter_declaration.go -- the filters an insertion may reference

package registry

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CheckFilterInsertions judges a plugin's declared auto-insertions: each
// names a reference the plugin itself serves, one direction, a bounded
// priority, a condition, and a reason when it is required; no (filter,
// direction) pair repeats. filters are the plugin's own declarations and
// filterTypes the filter types it owns. The rules are static, so the Stage 1
// handshake and config load (for a compiled-in registration) judge alike.
func CheckFilterInsertions(pluginName string, filters []rpc.FilterDecl, filterTypes []string, insertions []rpc.FilterInsertion) error {
	if len(insertions) == 0 {
		return nil
	}
	declared, err := CompileFilterDeclarations(pluginName, filters)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(insertions))
	for i := range insertions {
		ins := &insertions[i]
		if err := checkFilterInsertion(pluginName, declared, filterTypes, ins); err != nil {
			return err
		}
		key := ins.Filter + " " + ins.Direction.String()
		if seen[key] {
			return insertionError(pluginName, ins, "twice for the same direction; declare it once")
		}
		seen[key] = true
	}
	return nil
}

func checkFilterInsertion(pluginName string, declared []DeclaredFilter, filterTypes []string, ins *rpc.FilterInsertion) error {
	filter, served := insertionDeclaration(pluginName, declared, filterTypes, ins.Filter)
	if !served {
		return insertionError(pluginName, ins, "which the plugin does not serve; insert <plugin>:<filter> of a "+
			"filter it declares, or <type>:<instance> of a filter type it owns")
	}
	if ins.Direction != rpc.FilterImport {
		if ins.Direction != rpc.FilterExport {
			return insertionError(pluginName, ins, "with a direction other than import or export; declare one "+
				"insertion per list")
		}
	}
	if filter != nil {
		if !filter.AllowsDirection(ins.Direction) {
			return insertionError(pluginName, ins, "for direction "+ins.Direction.String()+
				", but its filter declares direction "+filter.Direction.String()+
				"; insert it in that direction or declare the filter for both")
		}
	}
	if ins.Priority < 0 {
		return insertionError(pluginName, ins, priorityRange)
	}
	if ins.Priority > rpc.FilterInsertionPriorityMax {
		return insertionError(pluginName, ins, priorityRange)
	}
	if err := checkInsertionScope(pluginName, ins); err != nil {
		return err
	}
	if err := checkInsertionCondition(pluginName, ins); err != nil {
		return err
	}
	if ins.Required && ins.Reason == "" {
		return insertionError(pluginName, ins, "required with no reason; give the reason the operator reads "+
			"when an edit is refused")
	}
	return nil
}

// checkInsertionCondition judges the insertion's one condition: a config
// path with no empty word, or a session condition whose every fact some owner
// registered (RegisterSessionFact), each named once.
func checkInsertionCondition(pluginName string, ins *rpc.FilterInsertion) error {
	if len(ins.Config) == 0 {
		if len(ins.Session) == 0 {
			return insertionError(pluginName, ins, "with no condition; name the config path (config) or the "+
				"session facts (session) that make a peer need it")
		}
		return checkSessionCondition(pluginName, ins)
	}
	if len(ins.Session) > 0 {
		return insertionError(pluginName, ins, "with two conditions, config and session; declare exactly one")
	}
	if slices.Contains(ins.Config, "") {
		return insertionError(pluginName, ins, "whose condition path holds an empty word")
	}
	return nil
}

// checkInsertionScope refuses a scope no one defined, and any-peer where it
// cannot mean anything: on the import list (a source-keyed procedure acts on
// the routes it exports to others), and beside a session condition, whose
// facts a dynamic peer has only once its OPEN arrives, so no load can judge
// it on any peer.
func checkInsertionScope(pluginName string, ins *rpc.FilterInsertion) error {
	switch ins.Scope {
	case rpc.FilterScopePeer:
		return nil
	case rpc.FilterScopeAnyPeer:
		if ins.Direction == rpc.FilterImport {
			return insertionError(pluginName, ins, "with scope any-peer on the import list; any-peer places the "+
				"member on every export list, so declare it for export")
		}
		if len(ins.Session) > 0 {
			return insertionError(pluginName, ins, "with scope any-peer beside a session condition; any-peer "+
				"judges a config condition, so name the config path that marks the source peer")
		}
		return nil
	}
	// The scope arrives over the wire from any plugin, so the set is open.
	return insertionError(pluginName, ins, "with scope "+strconv.Quote(string(ins.Scope))+
		", which is not a scope; omit it for the peer itself, or give any-peer")
}

func checkSessionCondition(pluginName string, ins *rpc.FilterInsertion) error {
	if len(ins.Values) > 0 {
		return insertionError(pluginName, ins, "with values beside a session condition; give each fact its "+
			"own values")
	}
	for i := range ins.Session {
		fact := ins.Session[i].Fact
		if !SessionFactRegistered(fact) {
			return insertionError(pluginName, ins, "whose session condition names "+strconv.Quote(fact)+
				", which no owner registered as a session fact; name a fact of the peer the daemon "+
				"knows (ibgp, rs-client, peer-as, ...)")
		}
		for j := range i {
			if ins.Session[j].Fact == fact {
				return insertionError(pluginName, ins, "whose session condition names "+strconv.Quote(fact)+
					" twice; list each fact once with all its values")
			}
		}
	}
	return nil
}

// OwnedFilterTypes answers the filter types the compiled registration of
// pluginName owns. A plugin with no compiled registration (a third-party one)
// owns none: a filter type is a YANG list the daemon compiles, and an external
// plugin's own YANG is not compiled (docs/plugin-development/schema.md).
func OwnedFilterTypes(pluginName string) []string {
	reg := Lookup(pluginName)
	if reg == nil {
		return nil
	}
	return reg.FilterTypes
}

// OwnedYANG answers the YANG module text of the compiled registration of
// pluginName, empty for a plugin with no compiled registration or no module.
// The insertion path judge reads it to tell a compiled-in plugin, whose
// conditions must name config its own module adds, from a third-party one.
func OwnedYANG(pluginName string) string {
	reg := Lookup(pluginName)
	if reg == nil {
		return ""
	}
	return reg.YANG
}

// priorityRange is the refusal of a priority outside 0 to FilterInsertionPriorityMax.
var priorityRange = "with a priority outside 0-" + strconv.Itoa(rpc.FilterInsertionPriorityMax)

// insertionDeclaration answers the declaration an insertion names, with exact
// names taking precedence over patterns. A served filter-type reference has no
// Stage 1 declaration, so it answers nil, true and is judged by its type.
func insertionDeclaration(pluginName string, declared []DeclaredFilter, filterTypes []string, ref string) (*DeclaredFilter, bool) {
	prefix, name, found := strings.Cut(ref, ":")
	if !found {
		return nil, false
	}
	if name == "" {
		return nil, false
	}
	if prefix == pluginName {
		return FindDeclaredFilter(declared, name)
	}
	return nil, slices.Contains(filterTypes, prefix)
}

func insertionError(pluginName string, ins *rpc.FilterInsertion, rule string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("plugin ").Quoted(pluginName).Str(" declares the filter insertion ").
		Quoted(ins.Filter).Str(" ").Str(rule).Str(" (docs/architecture/api/process-protocol.md, \"Filter Insertion\")").String())
}
