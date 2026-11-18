// Design: docs/architecture/api/commands.md -- one declaration of how a client's
// argument values become the text command the dispatcher reads. The web admin form and the MCP tool call both
// hold a name-to-value map and the command's ArgDefs, and both used to spell
// the placement themselves; the MCP copy put an anchored value in keyword form
// and every command that requires a selector refused the one value the client
// supplied (plan/journal/guard-demands-what-the-model-cannot-supply.md).
// Related: usage.go renders the same placement for a reader (usageAnchor), and
// matchCommandTokens in internal/component/plugin/server binds it.
package command

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// WriteInvocation writes the command at path with the values a client
// supplied for its arguments, in the form the dispatcher binds
// (internal/component/plugin/server).
//
// An argument a container ABOVE the command declares carries that container's
// name as its Anchor, and matchCommandTokens binds it from the bare token that
// follows that keyword (anchoredDef): `send bgp <selector> withdraw all` reads
// the selector after `bgp`, so the value is written there, with no keyword in
// front of it. A keyword form after the command would pass ValidateArgs
// and still leave the selector unbound, and every command that requires one
// would answer "requires a selector" for the one value the client supplied.
// A nonflag own leaf whose name is a path keyword also follows that keyword
// bare, matching the dispatcher's explicit id/name selector binding. A named
// anchor takes precedence over the own name. Every other declared argument
// follows the command as the keyword the leaf is
// named by and then the value (ai/rules/cli.md: keyword before value), in
// declaration order, which is the form the dispatcher's keyword phase binds.
// A value whose name no definition declares trails them, in name order, in the
// same keyword form: the dispatcher hands an unmatched pair to the handler,
// and dropping it here would make the daemon ignore a client's input in
// silence.
//
// A missing map entry is omitted. An explicitly supplied empty value is
// written as a quoted empty token, so the model judges that value instead of
// silently treating it as missing. An unset flag remains omitted; it takes
// no value of its own. HTML adapters omit blank optional controls themselves.
// Literal values use WriteInvocationValue's operational quoting, as read by
// ScanInvocation. A quoted quote or pipe remains data rather than changing the
// token or pipeline boundary.
//
// The path and the definitions come from the model this binary carries, and
// the values from one request, so every loop below is bounded.
func WriteInvocation(tb *textbuf.Buffer, path []string, defs []InvocationArg, values map[string]string) {
	placed := make(map[string]bool, len(values))
	for i, segment := range path {
		if i > 0 {
			tb.Byte(' ')
		}
		tb.Str(segment)
		for j := range defs {
			def := &defs[j]
			if def.Flag {
				continue
			}
			if placed[def.Name] {
				continue
			}
			value, found := values[def.Name]
			if !found {
				continue
			}
			if invocationAnchor(def.Name, def.Anchor) != segment {
				continue
			}
			placed[def.Name] = true
			tb.Byte(' ')
			WriteInvocationValue(tb, value)
		}
	}
	for j := range defs {
		name := defs[j].Name
		if placed[name] {
			continue
		}
		value, found := values[name]
		if !found {
			continue
		}
		placed[name] = true
		if defs[j].Flag {
			if value == "" {
				continue
			}
		}
		tb.Byte(' ').Str(name)
		if defs[j].Flag {
			// Any non-empty form value sets the flag; the keyword is the whole argument.
			continue
		}
		tb.Byte(' ')
		WriteInvocationValue(tb, value)
	}

	undeclared := make([]string, 0, len(values))
	for name := range values {
		if placed[name] {
			continue
		}
		undeclared = append(undeclared, name)
	}
	slices.Sort(undeclared)
	for _, name := range undeclared {
		tb.Byte(' ').Str(name).Byte(' ')
		WriteInvocationValue(tb, values[name])
	}
}

// WriteInvocationValue writes one value as the dispatcher's tokenizer reads
// it: bare, or double-quoted when a byte would change its token or pipe boundary.
// Escapes inside quotes preserve quotes, backslashes and line/control spacing.
func WriteInvocationValue(tb *textbuf.Buffer, value string) {
	if value == "" {
		tb.Str(`""`)
		return
	}
	if !strings.ContainsAny(value, " \t\r\n|\"'\\") && strings.TrimSpace(value) == value {
		tb.Str(value)
		return
	}
	tb.Byte('"')
	for i := range len(value) {
		c := value[i]
		switch c {
		case '"', '\\':
			tb.Byte('\\').Byte(c)
		case '\n':
			tb.Str(`\n`)
		case '\r':
			tb.Str(`\r`)
		case '\t':
			tb.Str(`\t`)
		default:
			tb.Byte(c)
		}
	}
	tb.Byte('"')
}
