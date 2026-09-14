// Design: docs/architecture/cli/command-namespacing.md -- canonical CLI verb vocabulary

package command

import "slices"

// verbRole classifies what a command verb does. The role is load-bearing for the
// grammar gate: only RoleMutation verbs (set, delete) may target objects that live
// in the config YANG tree (ai/rules/cli.md "Engine-Owned Tree Mutation");
// everything else is a runtime action or a read.
type verbRole uint8

const (
	// RoleUnspecified is the zero value, and it means the token is not a verb.
	// A miss on Verbs therefore answers "no role" rather than a role that reads
	// as a legitimate one, so a caller that skips the comma-ok form does not
	// classify every unknown first word as a read (ai/rules/principles.md).
	RoleUnspecified verbRole = iota
	// RoleRead reads state without changing it (show, monitor, resolve).
	RoleRead
	// RoleMutation mutates the config YANG tree via engine path form (set, delete).
	RoleMutation
	// RoleAction performs a runtime operational action (clear, request, ...).
	RoleAction
)

// The canonical verb spellings. A surface that means one of these words
// references the constant rather than writing the word again, so the CLI
// vocabulary is declared once and a rename reaches every surface through the
// compiler (ai/rules/principles.md).
const (
	VerbShow    = "show"
	VerbMonitor = "monitor"
	VerbResolve = "resolve"
	VerbSet     = "set"
	VerbDelete  = "delete"
	VerbClear   = "clear"
	VerbRequest = "request"
	VerbCommit  = "commit"
	VerbUpdate  = "update"
	VerbCache   = "cache"
	VerbCreate  = "create"
	VerbSend    = "send"
	VerbDebug   = "debug"
)

// Verbs is the single canonical source of truth for the CLI command vocabulary.
// Every command's first token MUST be one of these keys unless the command is
// category-exempt (see the grammar gate exemptions). Both the grammar gate and the
// plugin registration gate (validateCommandName in
// internal/component/plugin/server/command_registry.go) derive their verb set from
// this map -- there is no second hardcoded list (ai/rules/evidence.md).
//
// The vocabulary was agreed as verb-first: show, monitor, clear, set, request,
// resolve, commit and update, plus the engine mutation verb delete,
// the runtime cache verb, the runtime-lifecycle verb create, the diagnostic verb
// debug, and the wire verb send. Adding a verb here is a deliberate vocabulary
// decision, not a convenience: a small, learnable verb set is the point.
var Verbs = map[string]verbRole{
	// Reads.
	VerbShow:    RoleRead,
	VerbMonitor: RoleRead,
	VerbResolve: RoleRead,
	// Engine config-tree mutation (path form: set <path> <value> / delete <path>).
	VerbSet:    RoleMutation,
	VerbDelete: RoleMutation,
	// Runtime operational actions.
	VerbClear:   RoleAction,
	VerbRequest: RoleAction,
	VerbCommit:  RoleAction,
	VerbUpdate:  RoleAction,
	VerbCache:   RoleAction,
	// Runtime resource lifecycle. create/delete manipulate live kernel resources
	// (e.g. netlink interfaces) immediately; this is distinct from config-tree
	// mutation (which is set/delete on a config path). delete is the RoleMutation
	// above and serves both the config-tree and runtime-resource senses.
	VerbCreate: RoleAction,
	// Bytes the operator supplies leave the router for a destination the operator
	// names: send <protocol> <selector> <form>. It is a verb of its own and not a
	// noun under request, because request changes THIS system and a send puts a
	// message OUTSIDE it (docs/architecture/cli/command-verbs.md, row L-8).
	// The protocol keyword comes before the destination, so a closed set stands in
	// front of the one free-form slot and says how to read it.
	VerbSend: RoleAction,
	// Diagnostic actions that PERTURB live protocol state for testing/introspection
	// (e.g. OSPF ext-14 crafted-LSA injection). Distinct from show/monitor, which
	// only read state: a debug command changes what the router does. Double-gated by
	// authz + an explicit enablement (see internal/plugins/ospf/debug_enable.go).
	// When to pick debug vs show: ai/rules/cli.md "Choosing the Verb".
	VerbDebug: RoleAction,
}

// IsVerb reports whether tok is a canonical command verb. A token is a verb
// when the registry gives it a role, so RoleUnspecified is the answer that
// decides here rather than a comma-ok nobody reads. An entry written with
// RoleUnspecified would name a verb that promises nothing, which is not a verb
// a command may root at; TestNoVerbCarriesRoleUnspecified refuses one.
func IsVerb(tok string) bool {
	return Verbs[tok] != RoleUnspecified
}

// IsReadOnlyVerb reports whether tok promises to change nothing. It is the
// registry's own answer: show, monitor and resolve carry RoleRead, and every
// other verb changes something (docs/architecture/cli/command-verbs.md).
func IsReadOnlyVerb(tok string) bool {
	return Verbs[tok] == RoleRead
}

// VerbList returns the sorted verb names, for error messages and diagnostics.
// Derived from Verbs so a new verb appears everywhere automatically.
func VerbList() []string {
	verbs := make([]string, 0, len(Verbs))
	for v := range Verbs {
		verbs = append(verbs, v)
	}
	slices.Sort(verbs)
	return verbs
}
