// Design: docs/architecture/cli/command-namespacing.md -- CLI command grammar
//
// Package grammar mechanizes the CLI command-syntax rules of ai/rules/cli.md
// as pure functions, so the grammar gate (scripts/checks/cli_grammar.go), the plugin
// registration check (validateCommandName), and the runtime audit all enforce the
// SAME rules from one place.
//
// The authoritative prose is ai/rules/cli.md. Path-level rules (R1, R2, R3,
// R7) need only the command name; structural rules (R5, R6, R8) need the command.Node
// with its ArgDefs; the sibling rule (R9) needs the sibling token names at one tree
// level, so only the static gate (which walks the tree) can run it.
package grammar

import (
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Finding is one grammar violation on one command.
type Finding struct {
	Command string // the full command path, e.g. "show interface name detail"
	Rule    string // "R1".."R8"
	Message string // human-readable reason naming the fix
}

// mutationTokens are operational mutation words that must never appear as a command
// token: mutating a config-tree object uses engine set/delete path form, not an
// operational sub-action (ai/rules/cli.md "Engine-Owned Tree Mutation", R7).
// set/delete/create are legitimate verbs (command.Verbs) and are not here. `del` is
// NOT here either: it is only the auto-completed prefix of `delete` (the full verb),
// never a command in its own right. What remains are the genuine mutation words that
// are not a prefix of any verb.
var mutationTokens = map[string]bool{
	"add":    true,
	"remove": true,
}

// selectorKinds are the typed selector keywords that address one member of a set
// (ai/rules/cli.md "Typed Selectors"). A value captured on one of these
// nodes is already keyword-typed, so it may legitimately precede a sub-action
// (`... name <n> unit ...`), the correct `show interface name <name> detail` shape.
var selectorKinds = map[string]bool{
	"name":    true,
	"id":      true,
	"index":   true,
	"address": true,
	"type":    true,
	"key":     true,
}

// CheckName applies the path-level rules (R1 verb-first, R2 token form, R3 no flag,
// R7 no mutation token) to a command name. It is the check plugin registration runs
// and the gate runs per command path. The caller is responsible for skipping
// category-exempt commands first (see ExemptCategory).
func CheckName(name string) []Finding {
	if name == "" {
		return []Finding{{Command: name, Rule: "R2", Message: "empty command name"}}
	}

	var out []Finding
	tokens := strings.Split(name, " ")

	// R2/R3: per-token form.
	for _, tok := range tokens {
		switch {
		case tok == "":
			out = append(out, Finding{name, "R2", "repeated or edge whitespace in command name"})
		case strings.HasPrefix(tok, "-"):
			out = append(out, Finding{name, "R3", msg("flag-style token ", tok, " -- filters are keyword grammar, never --flags")})
		case !validToken(tok):
			out = append(out, Finding{name, "R2", msg("token ", tok, " is not lowercase ASCII / digits / interior-hyphen kebab")})
		}
	}

	// R1: first token must be a canonical verb.
	if tokens[0] != "" && !command.IsVerb(tokens[0]) {
		out = append(out, Finding{name, "R1", msg("first token ", tokens[0], " is not a verb (valid: ", strings.Join(command.VerbList(), ", "), ")")})
	}

	// R7: no operational mutation token anywhere.
	for _, tok := range tokens {
		if mutationTokens[tok] {
			out = append(out, Finding{name, "R7", msg("mutation token ", tok, " -- config-tree objects mutate via set/delete path form, not operational verbs")})
		}
	}

	return out
}

// CheckNode applies the structural rules (R5 keyword-before-value, R6 value-before-
// keyword ordering, R8 string identifiers) to one executable command node, reading
// its typed value slots (ArgDefs) and keyword children. Only meaningful for
// YANG-backed commands, where the structure exists.
func CheckNode(path string, node *command.Node) []Finding {
	if node == nil {
		return nil
	}

	var out []Finding
	mandatoryFreeform := false
	for i := range node.ArgDefs {
		def := &node.ArgDefs[i]
		if def.Kind != command.ArgString && def.Kind != command.ArgUint {
			continue // enum values are themselves closed keywords -- fine.
		}
		if def.Mandatory {
			mandatoryFreeform = true
		}

		// R5: a free-form value must be preceded by a keyword. In the YANG tree the
		// leaf name IS that keyword, so it must be a valid keyword token.
		if def.Name == "" || !validToken(def.Name) {
			out = append(out, Finding{path, "R5", msg("free-form value with no keyword (arg name \"", def.Name, "\") -- a value must be typed by a selector keyword")})
		}

		// R8: identifier-typed values are strings, never numeric, to avoid
		// numeric-keyword ambiguity.
		if def.Kind == command.ArgUint && looksLikeID(def.Name) {
			out = append(out, Finding{path, "R8", msg("identifier ", def.Name, " is numeric; identifiers must be string-typed")})
		}
	}

	// R6: a node that captures a MANDATORY free-form value AND still has keyword
	// children puts a required value before those keywords (`<resource> <value>
	// <action>`); the action keyword must precede the identifier. Two things are
	// NOT violations: an OPTIONAL value coexisting with subcommands (e.g. `show route
	// [<cidr>]` alongside `show route lookup`, an object-rooted fork), and a value
	// captured on a typed SELECTOR node (`... name <n> unit ...`), where the value is
	// already keyword-typed by the selector -- exactly the correct `show interface
	// name <name> detail` shape.
	if mandatoryFreeform && len(node.Children) > 0 && !selectorKinds[node.Name] {
		out = append(out, Finding{path, "R6", "mandatory free-form value on a non-selector node precedes keyword children -- type the value with a selector keyword (name/id/...) or move the action keyword before the identifier"})
	}

	return out
}

// CheckSiblings applies R9 (sibling namespace collision): among the child tokens at a
// single tree level, a hyphenated token whose left segment is itself a sibling name is a
// namespace member masquerading as a compound name. With a `traffic` sibling and a
// `traffic-stat` sibling, `traffic-stat` should be the two-token path `traffic stat`, so
// the object roots the tree and completion can enumerate members (ai/rules/cli.md
// "Compound Token vs Namespace Split"; docs/architecture/cli/command-namespacing.md).
//
// R9 is the one grammar rule that needs sibling context, so only the static gate (which
// walks the whole YANG command tree) runs it; per-command registration cannot see
// siblings. It is deliberately conservative: it fires ONLY when the colliding namespace
// literally exists as a sibling, so a genuine compound (`as-set` with no `as` sibling) is
// never flagged. Deeper judgement calls (a flat `foo-bar` with no bare `foo`
// sibling at that level) are left to human review against the prose rule.
//
// parentPath is the command path of the level's parent ("" at the root); names are the
// child token names at this level. The returned Command is the full path of the offending
// token.
func CheckSiblings(parentPath string, names []string) []Finding {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	var out []Finding
	for _, n := range names {
		b := strings.IndexByte(n, '-')
		for b > 0 {
			left := n[:b]
			if set[left] {
				out = append(out, Finding{
					Command: joinPath(parentPath, n),
					Rule:    "R9",
					Message: msg("token ", n, " collides with sibling namespace ", left,
						" -- split into two tokens (", left, " ", n[b+1:],
						"); reserve the hyphen for one indivisible name"),
				})
				break
			}
			next := strings.IndexByte(n[b+1:], '-')
			if next < 0 {
				break
			}
			b += 1 + next
		}
	}
	return out
}

// joinPath returns "parent tok", or just "tok" at the root (empty parent).
func joinPath(parent, tok string) string {
	if parent == "" {
		return tok
	}
	var tb textbuf.Buffer
	return tb.Str(parent).Byte(' ').Str(tok).String()
}

// CheckRootNamespace applies R9 to the ROOT command namespace across CLI
// surfaces. CheckSiblings fires only when the colliding namespace is a sibling at
// the same YANG-tree level; but root handlers never pass through the YANG tree
// (ai/rules/cli.md), so their compound-vs-namespace collisions live on a
// surface CheckSiblings cannot see. This is the feeder that governs them: a root
// whose left hyphen-segment names a namespace that exists on ANOTHER surface -- a
// YANG verb or a YANG object container -- is a namespace member masquerading as a
// compound root. `traffic-control` collides with the `traffic` container,
// `update-serve` with the `update` verb; both must split into `traffic control` /
// `update serve` so the object roots the tree and completion can enumerate the
// members.
//
// roots are the registered root command names; namespaces is the set of names
// that exist as a namespace on another surface (every YANG verb plus every YANG
// container). A root whose left segment is in that set is the violation. A leading
// '-' (a flag-shaped root such as `--plugins`) has an empty left segment and is
// never flagged. The multi-hyphen walk mirrors CheckSiblings so `a-b-c` is tested
// at each boundary.
func CheckRootNamespace(roots []string, namespaces map[string]bool) []Finding {
	var out []Finding
	for _, n := range roots {
		b := strings.IndexByte(n, '-')
		for b > 0 {
			left := n[:b]
			if namespaces[left] {
				out = append(out, Finding{
					Command: n,
					Rule:    "R9",
					Message: msg("root ", n, " collides with the ", left,
						" namespace on another surface -- split into two tokens (", left, " ", n[b+1:],
						"); reserve the hyphen for one indivisible name"),
				})
				break
			}
			next := strings.IndexByte(n[b+1:], '-')
			if next < 0 {
				break
			}
			b += 1 + next
		}
	}
	return out
}

// bridgeSurface is the text-bridge compatibility surface: announce/withdraw/peer/help
// commands that deliberately mirror a legacy line protocol and are the one operator
// surface intentionally not verb-first (E1). A documented set, not an ad-hoc
// allowlist -- each is a line-protocol verb whose grammar is fixed by that protocol.
var bridgeSurface = map[string]bool{
	"ze-bgp:announce":    true,
	"ze-bgp:withdraw":    true,
	"ze-bgp:peer-raw":    true,
	"ze-bgp:peer-update": true,
	"ze-bgp:help":        true,
}

// ExemptCategory reports whether a command (identified by its handler wire method)
// belongs to a category that is intentionally exempt from the verb-first grammar,
// and which category. Exemptions are keyed on the structural identity of the handler
// namespace, never on a per-command string allowlist (AC-7):
//
//	E1 bridge        : the text-bridge compatibility verbs (announce/withdraw/peer/help)
//	E2 wire-protocol : plugin/system process-boundary directives (ze-plugin:, ze-system:, ze-bgp:plugin-)
//	E3 editor        : editor mode switches (ze-editor:)
func ExemptCategory(wireMethod string) (string, bool) {
	switch {
	case bridgeSurface[wireMethod]:
		return "bridge", true
	case strings.HasPrefix(wireMethod, "ze-plugin:"),
		strings.HasPrefix(wireMethod, "ze-system:"),
		strings.HasPrefix(wireMethod, "ze-bgp:plugin-"):
		return "wire-protocol", true
	case strings.HasPrefix(wireMethod, "ze-editor:"):
		return "editor", true
	}
	return "", false
}

// validToken reports whether tok is lowercase ASCII letters, digits, and interior
// hyphens, with no leading or trailing hyphen. Mirrors validateCommandToken in
// command_registry.go (the two derive the same shape; see that file's tests).
func validToken(tok string) bool {
	if tok == "" || tok[0] == '-' || tok[len(tok)-1] == '-' {
		return false
	}
	for _, r := range tok {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return true
}

// looksLikeID reports whether an argument name denotes an identifier that must be
// string-typed (id, session-id, tunnel-id, ...).
func looksLikeID(name string) bool {
	return name == "id" || strings.HasSuffix(name, "-id")
}

// msg concatenates parts through a pooled text buffer (ai/rules/performance.md).
func msg(parts ...string) string {
	var tb textbuf.Buffer
	for _, p := range parts {
		tb.Str(p)
	}
	return tb.String()
}
