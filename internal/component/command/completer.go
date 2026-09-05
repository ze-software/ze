// Design: docs/architecture/api/commands.md — command completion
// Overview: node.go — command tree types

package command

import (
	"sort"
	"strings"
)

// Suggestion represents a single completion suggestion from the command completer.
// The editor maps these to its own Completion type for display.
type Suggestion struct {
	Text        string
	Description string
	Type        string // one of the suggestion kinds below
}

// The kinds a Suggestion carries in its Type field. They are exported because
// a caller in another package builds Suggestions too, and the editor renders
// each kind differently: a suggestion that names the wrong kind is shown wrong.
const (
	SuggestionCommand  = "command"
	SuggestionPipe     = "pipe"
	SuggestionValue    = "value"
	SuggestionSelector = "selector"
)

// TreeCompleter provides completions for operational commands from a Node tree.
// Used by both the CLI and the editor's command mode.
type TreeCompleter struct {
	root           *Node
	activeBackends map[string]string // component root name -> active backend name
}

// NewTreeCompleter creates a completer from a command tree root.
func NewTreeCompleter(root *Node) *TreeCompleter {
	if root == nil {
		return &TreeCompleter{root: &Node{}}
	}
	return &TreeCompleter{root: root}
}

// SetActiveBackends sets the per-component active backend map.
// Keys are component root paths (e.g. "interface", "firewall", "traffic/control"),
// values are backend names (e.g. "netlink", "vpp", "nft").
func (c *TreeCompleter) SetActiveBackends(backends map[string]string) {
	c.activeBackends = backends
}

// PipeOperators lists the available pipe operators for completion, derived
// from the catalog so completion and the parser cannot drift apart. The list
// used to be hand-written here, and completer_test.go compared it against
// itself, so a name added to the parser was silently absent from completion.
var PipeOperators = func() []Suggestion {
	out := make([]Suggestion, 0, len(pipeCatalog))
	for _, op := range pipeCatalog {
		out = append(out, Suggestion{Text: op.Name, Description: op.Description, Type: SuggestionPipe})
	}
	return out
}()

// pipeSubArgs maps pipe operators to their sub-argument completions.
var pipeSubArgs = map[string][]Suggestion{
	"json": {
		{Text: "compact", Description: "Single-line JSON", Type: SuggestionPipe},
		{Text: "pretty", Description: "Indented JSON (default)", Type: SuggestionPipe},
	},
	"fill": {
		{Text: fillWayAlpha, Description: "Remaining columns by field name", Type: SuggestionPipe},
		{Text: fillWordReverse, Description: "Flip the order in force", Type: SuggestionPipe},
	},
}

// CompletePipe returns global pipe operator completions matching the partial input.
// When a pipe operator is fully matched (e.g., "json "), returns sub-argument
// completions instead of repeating the operator.
func CompletePipe(partial string) []Suggestion {
	return completePipe("", partial, pipeExtras(""))
}

// completePipeForCommand returns global pipe completions plus the names the
// resolved command adds to them.
func completePipeForCommand(command, partial string) []Suggestion {
	return completePipe(command, partial, pipeExtras(command))
}

// pipeExtras returns the names a command answers to beside the global
// operators: its aliases first, then the filters it owns. A name is never in
// both, because RegisterAliases and RegisterPipeFilters each refuse the
// collision.
func pipeExtras(command string) []Suggestion {
	return append(aliasSuggestions(command), filterSuggestions(command)...)
}

// completePipe completes one pipe segment: its operator name, or the argument
// of an operator already named.
//
// An argument is matched on the LAST token typed, not on everything after the
// operator name. `| display address st` asks about "st". Matching the whole
// tail "address st" against a field name answers nothing after the first
// field.
func completePipe(command, partial string, extras []Suggestion) []Suggestion {
	trimmed := strings.TrimSpace(partial)
	fields := strings.Fields(trimmed)
	naming := len(fields) > 1 || (len(fields) == 1 && strings.HasSuffix(partial, " "))

	if naming {
		typed := fields[1:]
		last := ""
		if !strings.HasSuffix(partial, " ") {
			last = typed[len(typed)-1]
			typed = typed[:len(typed)-1]
		}
		return completePipeArg(command, fields[0], typed, last)
	}

	// Prefix matching against operators.
	var completions []Suggestion
	for _, op := range PipeOperators {
		if trimmed == "" || strings.HasPrefix(op.Text, trimmed) {
			completions = append(completions, op)
		}
	}
	for _, op := range extras {
		if pipeSuggestionExists(completions, op.Text) {
			continue
		}
		if trimmed == "" || strings.HasPrefix(op.Text, trimmed) {
			completions = append(completions, op)
		}
	}
	return completions
}

// completePipeArg completes the argument of the named operator. typed holds the
// argument tokens already complete, and last holds the token being typed.
func completePipeArg(command, opName string, typed []string, last string) []Suggestion {
	// knownPipeOps is the one list of operator names, so the completer reads
	// the kind rather than repeating a spelling. An unknown name maps to the
	// zero kind, which is not this one.
	//
	// `| display` takes field names, which are per-command and live in the
	// column registry. Every other operator takes keywords, which are the same
	// for every command and sit in pipeSubArgs. Neither set is ever mixed with
	// the other, because a token that is a field name in one position and a
	// keyword in another is a token nobody can complete.
	if knownPipeOps[opName] == pipeDisplay {
		return completeDisplayFields(command, typed, last)
	}

	subs, ok := pipeSubArgs[opName]
	if !ok {
		return nil // the operator takes no argument
	}
	var completions []Suggestion
	for _, s := range subs {
		if last == "" || strings.HasPrefix(s.Text, last) {
			completions = append(completions, s)
		}
	}
	return completions
}

// completeDisplayFields offers the field names the command declared, which is
// the only in-process list of what its answer carries. A command that declared
// no column order can offer none, and the operator types the names by hand.
//
// A name already typed is not offered again, because naming a field twice
// displays it once.
func completeDisplayFields(command string, typed []string, last string) []Suggestion {
	named := make(map[string]bool, len(typed))
	for _, name := range typed {
		named[strings.ToLower(name)] = true
	}

	var completions []Suggestion
	offered := make(map[string]bool)
	for _, order := range ColumnsForCommand(command) {
		for _, name := range order {
			if named[name] || offered[name] {
				continue
			}
			if last != "" && !strings.HasPrefix(name, last) {
				continue
			}
			offered[name] = true
			completions = append(completions, Suggestion{Text: name, Description: "Column", Type: SuggestionValue})
		}
	}
	return completions
}

func pipeSuggestionExists(items []Suggestion, text string) bool {
	for _, item := range items {
		if item.Text == text {
			return true
		}
	}
	return false
}

// resolve walks the words from the root and returns the node they name. It
// returns nil when a word names neither a static child nor a dynamic selector.
//
// Complete and Explain both read this one walk. The path a candidate is offered
// for and the path an explanation is answered for cannot drift apart.
func (c *TreeCompleter) resolve(words []string) *Node {
	current := c.root
	for _, word := range words {
		if current.Children == nil {
			return nil
		}

		child, ok := current.Children[word]
		if !ok {
			// Word is not a static child. If parent has DynamicChildren,
			// the word might be a dynamic selector (e.g., peer name).
			// Skip it and stay on the same node.
			if current.DynamicChildren != nil {
				continue
			}
			return nil
		}
		current = child
	}
	return current
}

// Complete returns completions for the given input.
func (c *TreeCompleter) Complete(input string) []Suggestion {
	// After a pipe character, complete pipe operators.
	if pipeIdx := strings.LastIndex(input, "|"); pipeIdx >= 0 {
		base, _, _ := strings.Cut(input, "|")
		return completePipeForCommand(base, input[pipeIdx+1:])
	}

	if c.root == nil || c.root.Children == nil {
		return nil
	}

	input = strings.TrimLeft(input, " ")
	words := strings.Fields(input)

	// A last word with no space after it is what the operator still types. It
	// is the prefix to match rather than a step on the path.
	var partial string
	if len(words) > 0 && !strings.HasSuffix(input, " ") {
		partial = words[len(words)-1]
		words = words[:len(words)-1]
	}

	current := c.resolve(words)
	if current == nil {
		return nil
	}

	return c.matchChildren(current, partial)
}

// Explain returns the long explanation the command the input names declares. It
// returns false when the input names no command, and when the command declares
// none.
//
// The caller MUST NOT invent text for a false answer. An empty explanation reads
// on screen as a command whose author wrote nothing, which is a different claim.
//
// Every typed word is a step on the path. That holds for the last word, with or
// without a space after it. Complete treats the last word as a prefix, because
// it offers what comes next. Explain answers about the command in hand.
func (c *TreeCompleter) Explain(input string) (string, bool) {
	if c.root == nil {
		return "", false
	}

	// A pipe operator explains nothing about the command, so read the part
	// before the first one, as Complete does.
	base, _, _ := strings.Cut(input, "|")
	words := strings.Fields(base)
	if len(words) == 0 {
		return "", false
	}

	node := c.resolve(words)
	if node == nil {
		return "", false
	}
	if node.LongHelp == "" {
		return "", false
	}
	return node.LongHelp, true
}

// GhostText returns the best single completion for inline display.
func (c *TreeCompleter) GhostText(input string) string {
	if input == "" || c.root == nil {
		return ""
	}

	if strings.HasSuffix(input, " ") {
		return ""
	}

	completions := c.Complete(input)
	if len(completions) == 0 {
		return ""
	}

	// For pipe completions, extract the last word after the pipe.
	// When pipe has sub-args (e.g., "| json c"), lastWord should be "c".
	var lastWord string
	if pipeIdx := strings.LastIndex(input, "|"); pipeIdx >= 0 {
		fields := strings.Fields(strings.TrimSpace(input[pipeIdx+1:]))
		if len(fields) > 0 {
			lastWord = fields[len(fields)-1]
		}
	} else {
		words := strings.Fields(input)
		if len(words) == 0 {
			return ""
		}
		lastWord = words[len(words)-1]
	}

	if lastWord == "" {
		return ""
	}

	var matches []Suggestion
	for _, comp := range completions {
		if strings.HasPrefix(comp.Text, lastWord) {
			matches = append(matches, comp)
		}
	}

	if len(matches) == 1 {
		return matches[0].Text[len(lastWord):]
	}

	if len(matches) > 1 {
		common := matches[0].Text
		for _, m := range matches[1:] {
			common = commonPrefix(common, m.Text)
		}
		if len(common) > len(lastWord) {
			return common[len(lastWord):]
		}
	}

	return ""
}

// choiceSuggestions offers the words a choice group's leaf declares. The group
// carries one leaf whose type states a closed set, so the loop is bounded by
// that set.
func choiceSuggestions(node *Node, prefix string) []Suggestion {
	var out []Suggestion
	for i := range node.ArgDefs {
		for _, value := range node.ArgDefs[i].EnumValues {
			if prefix != "" && !strings.HasPrefix(value, prefix) {
				continue
			}
			out = append(out, Suggestion{Text: value, Description: node.Description, Type: SuggestionValue})
		}
	}
	return out
}

// oneOfSuggestions offers the KEYWORDS a one-of group's members declare, sorted
// by name. The group holds a closed set of member containers, so the loop is
// bounded by the model.
//
// The member's own name is what the operator types, and its values follow it,
// so a member is offered exactly as a plain trailing group would be. The group
// container's name is offered by nothing, here or in help.
func (c *TreeCompleter) oneOfSuggestions(node *Node, prefix string) []Suggestion {
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Suggestion, 0, len(names))
	for _, name := range names {
		member := node.Children[name]
		if member == nil || !c.backendAllowed(member) {
			continue
		}
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		out = append(out, Suggestion{Text: name, Description: member.Description, Type: SuggestionCommand})
	}
	return out
}

// matchChildren returns sorted completions for children matching prefix.
// Includes both static children and dynamic suggestions from DynamicChildren callback.
func (c *TreeCompleter) matchChildren(node *Node, prefix string) []Suggestion {
	if node == nil {
		return nil
	}

	var completions []Suggestion

	// Static children from tree.
	if node.Children != nil {
		keys := make([]string, 0, len(node.Children))
		for k := range node.Children {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			child := node.Children[name]
			if !c.backendAllowed(child) {
				continue
			}
			// A choice group's own name is never typed: the WORDS its leaf
			// declares are the tokens (usage.go, ModifierChoice). Offering the
			// container would complete a token the handler rejects.
			if child.Modifier == ModifierChoice {
				completions = append(completions, choiceSuggestions(child, prefix)...)
				continue
			}
			// A one-of group's own name is never typed either. Its MEMBERS are
			// the keywords, so the operator is offered those (usage.go,
			// ModifierOneOf).
			if child.Modifier == ModifierOneOf {
				completions = append(completions, c.oneOfSuggestions(child, prefix)...)
				continue
			}
			if prefix == "" || strings.HasPrefix(name, prefix) {
				completions = append(completions, Suggestion{
					Text:        name,
					Description: child.Description,
					Type:        SuggestionCommand,
				})
			}
		}
	}

	// Dynamic children (e.g., peer names/IPs).
	if node.DynamicChildren != nil {
		for _, s := range node.DynamicChildren() {
			if prefix == "" || strings.HasPrefix(s.Text, prefix) {
				completions = append(completions, s)
			}
		}
	}

	// Value hints (terminal argument values like families, log levels).
	if node.ValueHints != nil {
		for _, s := range node.ValueHints() {
			if prefix == "" || strings.HasPrefix(s.Text, prefix) {
				completions = append(completions, s)
			}
		}
	}

	// YANG-declared argument definitions: enum values as value suggestions,
	// leaf names as keyword suggestions. Deduplicate against ValueHints.
	var seen map[string]bool
	if node.ValueHints != nil && len(node.ArgDefs) > 0 {
		seen = make(map[string]bool, len(completions))
		for _, s := range completions {
			seen[s.Text] = true
		}
	}
	for i := range node.ArgDefs {
		def := &node.ArgDefs[i]
		for _, v := range def.EnumValues {
			if seen != nil && seen[v] {
				continue
			}
			if prefix == "" || strings.HasPrefix(v, prefix) {
				completions = append(completions, Suggestion{
					Text: v,
					Type: SuggestionValue,
				})
			}
		}
		// An ENUM the command runs without is reached through its own keyword,
		// so the keyword is offered beside its values. The renderer states the
		// same fact from the same field: a mandatory leaf is a bare positional
		// and an optional one is `[<name> <value>]` (Usage, appendLeafTokens).
		// Without this the completer offered `open` for `send bgp <selector>
		// raw`, where a bare `open` is refused and only `type open` is accepted.
		enumNeedsItsKeyword := def.Kind == ArgEnum && !def.Mandatory
		if def.Kind == ArgUint || def.Kind == ArgString || enumNeedsItsKeyword {
			if seen != nil && seen[def.Name] {
				continue
			}
			if prefix == "" || strings.HasPrefix(def.Name, prefix) {
				completions = append(completions, Suggestion{
					Text: def.Name,
					Type: SuggestionValue,
				})
			}
		}
	}

	return completions
}

// backendAllowed returns true if the node should be shown given the active backends.
// Nil Backend means unrestricted (always shown). When Backend is set, at least one
// entry must match an active backend value.
func (c *TreeCompleter) backendAllowed(node *Node) bool {
	if node.Backend == nil || c.activeBackends == nil {
		return true
	}
	for _, allowed := range node.Backend {
		for _, active := range c.activeBackends {
			if allowed == active {
				return true
			}
		}
	}
	return false
}

// commonPrefix returns the longest common prefix of two strings.
func commonPrefix(a, b string) string {
	minLen := min(len(b), len(a))
	for i := range minLen {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:minLen]
}
