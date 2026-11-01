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
	if !servesReference(pluginName, declared, filterTypes, ins.Filter) {
		return insertionError(pluginName, ins, "which the plugin does not serve; insert <plugin>:<filter> of a "+
			"filter it declares, or <type>:<instance> of a filter type it owns")
	}
	if ins.Direction != rpc.FilterImport {
		if ins.Direction != rpc.FilterExport {
			return insertionError(pluginName, ins, "with a direction other than import or export; declare one "+
				"insertion per list")
		}
	}
	if ins.Priority < 0 {
		return insertionError(pluginName, ins, priorityRange)
	}
	if ins.Priority > rpc.FilterInsertionPriorityMax {
		return insertionError(pluginName, ins, priorityRange)
	}
	if len(ins.Config) == 0 {
		return insertionError(pluginName, ins, "with no condition; name the config path (config) that makes "+
			"a peer need it")
	}
	if slices.Contains(ins.Config, "") {
		return insertionError(pluginName, ins, "whose condition path holds an empty word")
	}
	if ins.Required && ins.Reason == "" {
		return insertionError(pluginName, ins, "required with no reason; give the reason the operator reads "+
			"when an edit is refused")
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

// priorityRange is the refusal of a priority outside 0 to FilterInsertionPriorityMax.
var priorityRange = "with a priority outside 0-" + strconv.Itoa(rpc.FilterInsertionPriorityMax)

// servesReference reports whether ref is a chain reference the plugin serves:
// <plugin>:<filter> of its own declarations, or <type>:<instance> of a type it
// owns.
func servesReference(pluginName string, declared []DeclaredFilter, filterTypes []string, ref string) bool {
	prefix, name, found := strings.Cut(ref, ":")
	if !found {
		return false
	}
	if name == "" {
		return false
	}
	if prefix == pluginName {
		for i := range declared {
			if declared[i].Covers(name) {
				return true
			}
		}
		return false
	}
	return slices.Contains(filterTypes, prefix)
}

func insertionError(pluginName string, ins *rpc.FilterInsertion, rule string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("plugin ").Quoted(pluginName).Str(" declares the filter insertion ").
		Quoted(ins.Filter).Str(" ").Str(rule).Str(" (docs/architecture/api/process-protocol.md, \"Filter Insertion\")").String())
}
