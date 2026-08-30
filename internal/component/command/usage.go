// Design: docs/architecture/api/commands.md -- the generated invocation form
// Related: node.go -- the tree and the argument definitions this reads
// Related: help.go -- the operator surface that prints the rendered line
//
// usage.go turns a command path and its node into the form an operator types.
// It reads the MODEL and nothing else. No description reaches it, so a sentence
// written in prose can never influence a generated line.
//
// One rule explains where every value sits: a leaf ANCHORED to a container on
// the path belongs immediately after that container's keyword. A leaf is
// anchored by the container that declares it when that container is not the
// command itself, and by its own name when the command declares it and the name
// repeats a keyword. Every other leaf follows the last keyword, the ones the
// command needs first.

package command

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// UsageKind says what an operator does with one token of an invocation form.
type UsageKind uint8

const (
	// UsageUnspecified is the zero value. It names no token, so a token built
	// by mistake never reads as a valid one.
	UsageUnspecified UsageKind = iota
	// UsageKeyword is a word the operator types exactly.
	UsageKeyword
	// UsageValue is a value the operator supplies, and the command needs it.
	UsageValue
	// UsageOption is a value the operator supplies after its own keyword, and
	// the command runs without it.
	UsageOption
	// UsageGroup is a keyword and the values that belong to it, supplied
	// together or not at all.
	UsageGroup
	// UsageGroupRepeat is a UsageGroup the operator repeats.
	UsageGroupRepeat
	// UsageChoice is a closed set of words the operator types one of, or none
	// at all. The words ARE the tokens, so nothing follows the one chosen.
	UsageChoice
)

// usageKindNames names each kind for a reader and for the published catalog.
// String, MarshalJSON and UnmarshalJSON all read this one table, so a writer
// and a reader of the catalog cannot disagree about what a kind is called.
var usageKindNames = [...]string{
	UsageUnspecified: "unspecified",
	UsageKeyword:     "keyword",
	UsageValue:       "value",
	UsageOption:      "option",
	UsageGroup:       "group",
	UsageGroupRepeat: "group-repeat",
	UsageChoice:      "choice",
}

// String names the kind. A value outside the declared set names itself as
// unspecified rather than as a number.
func (k UsageKind) String() string {
	if int(k) < len(usageKindNames) {
		return usageKindNames[k]
	}
	return usageKindNames[UsageUnspecified]
}

// MarshalJSON publishes the kind as its word, so a catalog reader never has to
// map a number onto a meaning.
func (k UsageKind) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(k.String())), nil
}

// UnmarshalJSON reads a kind back from its word. A word the table does not hold
// is an error: a kind that fell back to the zero value would read as a token
// nobody meant to write (ai/rules/evidence.md).
func (k *UsageKind) UnmarshalJSON(data []byte) error {
	name, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("usage kind is not a string: %w", err)
	}
	for i, candidate := range usageKindNames {
		if candidate != name {
			continue
		}
		// i is a range index over usageKindNames, so the conversion cannot
		// reach a kind the table does not hold, however the table grows.
		*k = UsageKind(i) //nolint:gosec // i indexes usageKindNames itself
		return nil
	}
	return fmt.Errorf("unknown usage kind %q", name)
}

// Modifier says whether a child container is an optional trailing group of its
// parent's command, and how many times an operator may supply it.
type Modifier uint8

const (
	// ModifierNone is the zero value: the node is not a modifier group. A child
	// container is a subcommand unless its module says otherwise.
	ModifierNone Modifier = iota
	// ModifierOnce is a group the operator supplies at most once.
	ModifierOnce
	// ModifierRepeat is a group the operator repeats.
	ModifierRepeat
	// ModifierRequired is a group the operator MUST supply: its keyword and
	// every value it declares. It is the mandatory sibling of ModifierOnce,
	// and it is what states a keyword that introduces a value the leaf name
	// alone cannot spell, such as `id <opaque-id>`.
	ModifierRequired
	// ModifierChoice is a closed set of words the operator types one of, or
	// none. The container names the choice for a machine reader; the operator
	// never types that name.
	ModifierChoice
)

// modifierNames names each kind for a reader and for the YANG argument that
// selects it. ParseModifier reads this one table, so a module and this package
// cannot disagree about what a modifier is called.
var modifierNames = [...]string{
	ModifierNone:     "",
	ModifierOnce:     "once",
	ModifierRepeat:   "repeat",
	ModifierRequired: "required",
	ModifierChoice:   "choice",
}

// ParseModifier answers the modifier a ze:modifier argument names, and false
// for a word the table does not hold. A word nobody declared must not fall back
// to a valid-looking group (ai/rules/evidence.md).
func ParseModifier(argument string) (Modifier, bool) {
	if argument == "" {
		return ModifierNone, false
	}
	for i, name := range modifierNames {
		if name == argument {
			// i is a range index over modifierNames, so the conversion cannot
			// reach an occurrence the table does not hold.
			return Modifier(i), true //nolint:gosec // i indexes modifierNames itself
		}
	}
	return ModifierNone, false
}

// String names the modifier. A value outside the declared set names itself as
// none rather than as a number.
func (m Modifier) String() string {
	if int(m) < len(modifierNames) {
		return modifierNames[m]
	}
	return modifierNames[ModifierNone]
}

// UsageToken is one element of a command's invocation form. It is published in
// the command catalog, so its JSON keys are part of that contract.
type UsageToken struct {
	// Text is the keyword the operator types, or the name of the value the
	// operator supplies.
	Text string `json:"text"`
	// Values is the closed set the leaf's type states, and it is empty when
	// the type states none.
	Values []string `json:"values,omitempty"`
	// Group is what a UsageGroup or UsageGroupRepeat token holds: the values
	// that belong to Text, in declaration order. It is empty for every other
	// kind.
	Group []UsageToken `json:"group,omitempty"`
	// Kind says whether the operator types Text or supplies a value for it.
	Kind UsageKind `json:"kind"`
}

// Usage builds the invocation form of the node at path, as an ordered token
// list. It answers nil for a node that runs no command, because a grouping node
// has no invocation form.
//
// The path and the argument definitions both come from the YANG this binary
// carries, so the loops below are bounded by the model rather than by input.
func Usage(path []string, node *Node) []UsageToken {
	if node == nil || node.WireMethod == "" {
		return nil
	}

	tokens := make([]UsageToken, 0, len(path)+len(node.ArgDefs))
	anchored := make(map[string]bool, len(node.ArgDefs))

	for _, segment := range path {
		tokens = append(tokens, UsageToken{Text: segment, Kind: UsageKeyword})
		for i := range node.ArgDefs {
			def := &node.ArgDefs[i]
			if anchored[def.Name] || usageAnchor(def) != segment {
				continue
			}
			anchored[def.Name] = true
			tokens = append(tokens, usageToken(def, UsageValue))
		}
	}

	// Two passes over the definitions, so every value the command needs comes
	// before every value it runs without.
	tokens = appendLeafTokens(tokens, node, anchored, true)
	tokens = appendLeafTokens(tokens, node, anchored, false)

	// The groups follow in the order the module DECLARES them, needed and
	// optional alike. A module that wants `filter` read before `update` says
	// so by declaring it first, which is the only place that reading order is
	// known: `[filter <name>] update <hex>` names the chain before the payload
	// it feeds through the chain, and no property of the two containers can
	// tell a renderer that.
	for _, child := range modifierChildren(node) {
		tokens = appendGroupTokens(tokens, child)
	}

	return tokens
}

// appendLeafTokens adds the node's own values, either the ones the command
// needs or the ones it runs without. A value already placed after the path
// keyword that anchors it is never placed a second time.
func appendLeafTokens(tokens []UsageToken, node *Node, anchored map[string]bool, wantMandatory bool) []UsageToken {
	kind := UsageValue
	if !wantMandatory {
		kind = UsageOption
	}
	for i := range node.ArgDefs {
		def := &node.ArgDefs[i]
		if anchored[def.Name] || def.Mandatory != wantMandatory {
			continue
		}
		tokens = append(tokens, usageToken(def, kind))
	}
	return tokens
}

// appendGroupTokens adds one modifier child's tokens to the line.
//
// A REQUIRED group is not bracketed and is not one token: the operator types
// its keyword and then every value it declares, which is the same shape a path
// keyword and its anchored leaf already produce. Only a group the command runs
// without needs a bracket to say so.
func appendGroupTokens(tokens []UsageToken, node *Node) []UsageToken {
	if node.Modifier == ModifierRequired {
		tokens = append(tokens, UsageToken{Text: node.Name, Kind: UsageKeyword})
		for i := range node.ArgDefs {
			tokens = append(tokens, usageToken(&node.ArgDefs[i], UsageValue))
		}
		return tokens
	}
	if node.Modifier == ModifierChoice {
		return append(tokens, usageChoiceToken(node))
	}
	return append(tokens, usageGroupToken(node))
}

// modifierChildren lists the node's optional trailing groups, in the order the
// module declares them.
//
// A child that runs a command of its own is a SUBCOMMAND and is never a group,
// whatever else it carries: it has an invocation form of its own, on its own
// line. Children is a map, so the order comes from ModifierOrder, and a name
// breaks a tie no module should produce.
//
// Nothing is hoisted for being required. A module that declares its groups in
// the order an operator types them already answers the question, and a
// required-first sort could only override that answer with a worse one.
func modifierChildren(node *Node) []*Node {
	groups := make([]*Node, 0, len(node.Children))
	for _, child := range node.Children {
		if child == nil || child.Modifier == ModifierNone || child.WireMethod != "" {
			continue
		}
		groups = append(groups, child)
	}
	sort.Slice(groups, func(i, j int) bool {
		left, right := groups[i], groups[j]
		if left.ModifierOrder != right.ModifierOrder {
			return left.ModifierOrder < right.ModifierOrder
		}
		return left.Name < right.Name
	})
	return groups
}

// usageGroupToken builds one group token: the group's keyword and every value
// it declares, so a reader cannot supply one half of a pair.
func usageGroupToken(node *Node) UsageToken {
	kind := UsageGroup
	if node.Modifier == ModifierRepeat {
		kind = UsageGroupRepeat
	}
	group := make([]UsageToken, 0, len(node.ArgDefs))
	for i := range node.ArgDefs {
		def := &node.ArgDefs[i]
		valueKind := UsageValue
		if !def.Mandatory {
			valueKind = UsageOption
		}
		group = append(group, usageToken(def, valueKind))
	}
	return UsageToken{Text: node.Name, Group: group, Kind: kind}
}

// usageChoiceToken builds one choice token: the closed set of words the
// operator types one of. The container's name is carried for a machine reader
// and never reaches the line, because the words themselves are the tokens.
func usageChoiceToken(node *Node) UsageToken {
	values := make([]string, 0, len(node.ArgDefs))
	for i := range node.ArgDefs {
		values = append(values, usageValues(&node.ArgDefs[i])...)
	}
	return UsageToken{Text: node.Name, Values: values, Kind: UsageChoice}
}

// UsageLine renders a token list as the line an operator types. It answers ""
// for an empty list.
func UsageLine(tokens []UsageToken) string {
	var tb textbuf.Buffer
	for i := range tokens {
		if i > 0 {
			tb.Byte(' ')
		}
		writeUsageToken(&tb, &tokens[i])
	}
	return tb.String()
}

// writeUsageToken renders one token. It is separate from UsageLine because a
// group holds tokens of its own and renders them the same way.
func writeUsageToken(tb *textbuf.Buffer, token *UsageToken) {
	switch token.Kind {
	case UsageKeyword:
		tb.Str(token.Text)
	case UsageValue:
		tb.Byte('<')
		writeUsageValue(tb, token)
		tb.Byte('>')
	case UsageOption:
		// The keyword introduces the value, so the operator never supplies a
		// bare optional positional (ai/rules/cli.md).
		tb.Byte('[').Str(token.Text).Str(" <")
		writeUsageValue(tb, token)
		tb.Str(">]")
	case UsageGroup, UsageGroupRepeat:
		// The keyword and its values are one unit, so the bracket closes after
		// the last of them and the operator reads them as supplied together.
		tb.Byte('[').Str(token.Text)
		for i := range token.Group {
			tb.Byte(' ')
			writeUsageToken(tb, &token.Group[i])
		}
		if token.Kind == UsageGroupRepeat {
			tb.Str(" ...")
		}
		tb.Byte(']')
	case UsageChoice:
		// The members ARE the words the operator types, so they carry no angle
		// brackets: nothing is supplied after the one chosen.
		tb.Byte('[')
		writeChoiceMembers(tb, token)
		tb.Byte(']')
	case UsageUnspecified:
		panic("BUG: usage token with no kind")
	default:
		panic("BUG: usage token with an unknown kind")
	}
}

// writeChoiceMembers writes the words an operator types one of. A choice that
// states no closed set names itself, so a token decoded from a published
// catalog can never render as an empty bracket pair.
func writeChoiceMembers(tb *textbuf.Buffer, token *UsageToken) {
	if len(token.Values) == 0 {
		tb.Str(token.Text)
		return
	}
	tb.Join(token.Values, "|")
}

// writeUsageValue writes what the operator supplies: the closed value set when
// the type states one, and the value's name when it does not.
func writeUsageValue(tb *textbuf.Buffer, token *UsageToken) {
	if len(token.Values) == 0 {
		tb.Str(token.Text)
		return
	}
	tb.Join(token.Values, "|")
}

// usageToken builds one value token from an argument definition.
func usageToken(def *ArgDef, kind UsageKind) UsageToken {
	return UsageToken{Text: def.Name, Values: usageValues(def), Kind: kind}
}

// usageValues returns the closed set of answers the definition's type states,
// or nil when the type states none.
//
// A union states the member forms: each enumerated member contributes its own
// values, and every other member contributes the leaf's name, once.
func usageValues(def *ArgDef) []string {
	switch def.Kind {
	case ArgEnum:
		return def.EnumValues
	case ArgUnion:
		return unionForms(def)
	case ArgString, ArgUint:
		return nil
	default:
		return nil
	}
}

// unionForms lists a union's member forms in declaration order. It answers nil
// when no member states a value set, because the leaf name alone is the form.
func unionForms(def *ArgDef) []string {
	forms := make([]string, 0, len(def.UnionDefs))
	named := false
	for i := range def.UnionDefs {
		member := &def.UnionDefs[i]
		if member.Kind == ArgEnum {
			forms = append(forms, member.EnumValues...)
			continue
		}
		if named {
			continue
		}
		named = true
		forms = append(forms, def.Name)
	}
	if len(forms) == 1 && forms[0] == def.Name {
		return nil
	}
	return forms
}

// usageAnchor names the path keyword a value follows.
//
// A leaf a container ABOVE the command declares carries that container's name,
// because its own name says nothing about where the operator types it:
// `request interface <name> down` and `request peer <selector> flush` both put
// the value after the container that declares it.
//
// A leaf the command itself declares carries no anchor, and its own name is the
// answer: it follows the keyword it repeats, and trails every keyword when it
// repeats none.
func usageAnchor(def *ArgDef) string {
	if def.Anchor != "" {
		return def.Anchor
	}
	return def.Name
}
