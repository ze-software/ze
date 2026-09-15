// Design: one declaration of how a client's argument values become the text
// command the dispatcher reads. The web admin form and the MCP tool call both
// hold a name-to-value map and the command's ArgDefs, and both used to spell
// the placement themselves; the MCP copy put an anchored value in keyword form
// and every command that requires a selector refused the one value the client
// supplied (plan/journal/guard-demands-what-the-model-cannot-supply.md).
// Related: usage.go renders the same placement for a reader (usageAnchor), and
// matchCommandTokens in internal/component/plugin/server binds it.
package command

import (
	"fmt"
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
// front of it. A keyword form after the command would pass validateCommandArgs
// and still leave the selector unbound, and every command that requires one
// would answer "requires a selector" for the one value the client supplied.
// Every other declared argument follows the command as the keyword the leaf is
// named by and then the value (ai/rules/cli.md: keyword before value), in
// declaration order, which is the form the dispatcher's keyword phase binds.
// A value whose name no definition declares trails them, in name order, in the
// same keyword form: the dispatcher hands an unmatched pair to the handler,
// and dropping it here would make the daemon ignore a client's input in
// silence.
//
// An empty value is left out rather than sent: the dispatcher's own mandatory
// check then names the missing argument, and an optional one is simply absent.
// A value holding a space is quoted, which is the dispatcher's grouping rule
// (tokenize). Its grammar carries no escape for a double quote, so a value
// holding one is refused: sent, it would split into tokens nobody typed.
//
// The path and the definitions come from the model this binary carries, and
// the values from one request, so every loop below is bounded.
func WriteInvocation(tb *textbuf.Buffer, path []string, defs []ArgDef, values map[string]string) error {
	for name, value := range values {
		if strings.ContainsRune(value, '"') {
			return fmt.Errorf("argument %s: a value cannot hold a double quote", name)
		}
	}

	placed := make(map[string]bool, len(values))
	for i, segment := range path {
		if i > 0 {
			tb.Byte(' ')
		}
		tb.Str(segment)
		for j := range defs {
			def := &defs[j]
			if placed[def.Name] || values[def.Name] == "" || def.Anchor != segment {
				continue
			}
			placed[def.Name] = true
			tb.Byte(' ')
			writeInvocationValue(tb, values[def.Name])
		}
	}
	for j := range defs {
		name := defs[j].Name
		if placed[name] || values[name] == "" {
			continue
		}
		placed[name] = true
		tb.Byte(' ').Str(name).Byte(' ')
		writeInvocationValue(tb, values[name])
	}

	undeclared := make([]string, 0, len(values))
	for name, value := range values {
		if placed[name] || value == "" {
			continue
		}
		undeclared = append(undeclared, name)
	}
	slices.Sort(undeclared)
	for _, name := range undeclared {
		tb.Byte(' ').Str(name).Byte(' ')
		writeInvocationValue(tb, values[name])
	}
	return nil
}

// writeInvocationValue writes one value as the dispatcher's tokenizer reads
// it: bare, or double-quoted when it holds a space.
func writeInvocationValue(tb *textbuf.Buffer, value string) {
	if strings.ContainsAny(value, " \t") {
		tb.Byte('"').Str(value).Byte('"')
		return
	}
	tb.Str(value)
}
