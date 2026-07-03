// Design: docs/architecture/cli/command-namespacing.md -- canonical CLI verb vocabulary

package command

import "sort"

// VerbRole classifies what a command verb does. The role is load-bearing for the
// grammar gate: only VerbMutation verbs (set, delete) may target objects that live
// in the config YANG tree (ai/rules/cli-grammar.md "Engine-Owned Tree Mutation");
// everything else is a runtime action or a read.
type VerbRole uint8

const (
	// VerbRead reads state without changing it (show, monitor, resolve).
	VerbRead VerbRole = iota
	// VerbMutation mutates the config YANG tree via engine path form (set, delete).
	VerbMutation
	// VerbAction performs a runtime operational action (clear, request, ...).
	VerbAction
)

// Verbs is the single canonical source of truth for the CLI command vocabulary.
// Every command's first token MUST be one of these keys unless the command is
// category-exempt (see the grammar gate exemptions). Both the grammar gate and the
// plugin registration gate (validateCommandName in
// internal/component/plugin/server/command_registry.go) derive their verb set from
// this map -- there is no second hardcoded list (ai/rules/derive-not-hardcode.md).
//
// The vocabulary was agreed in plan/learned/829-command-verb-first.md (show, monitor,
// clear, set, request, resolve, commit, update) plus the engine mutation verb delete
// and the runtime cache verb. Adding a verb here is a deliberate vocabulary decision,
// not a convenience: a small, learnable verb set is the point.
var Verbs = map[string]VerbRole{
	// Reads.
	"show":    VerbRead,
	"monitor": VerbRead,
	"resolve": VerbRead,
	// Engine config-tree mutation (path form: set <path> <value> / delete <path>).
	"set":    VerbMutation,
	"delete": VerbMutation,
	// Runtime operational actions.
	"clear":   VerbAction,
	"request": VerbAction,
	"commit":  VerbAction,
	"update":  VerbAction,
	"cache":   VerbAction,
	// Runtime resource lifecycle. create/delete manipulate live kernel resources
	// (e.g. netlink interfaces) immediately; this is distinct from config-tree
	// mutation (which is set/delete on a config path). delete is the VerbMutation
	// above and serves both the config-tree and runtime-resource senses.
	"create": VerbAction,
}

// IsVerb reports whether tok is a canonical command verb.
func IsVerb(tok string) bool {
	_, ok := Verbs[tok]
	return ok
}

// VerbList returns the sorted verb names, for error messages and diagnostics.
// Derived from Verbs so a new verb appears everywhere automatically.
func VerbList() []string {
	verbs := make([]string, 0, len(Verbs))
	for v := range Verbs {
		verbs = append(verbs, v)
	}
	sort.Strings(verbs)
	return verbs
}
