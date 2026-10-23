// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: parser.go — config parser core
// Related: serialize_set.go — set-format serialization (inverse of this file)
// Related: meta.go — MetaTree for metadata-aware parsing
// Detail: setparser_inline.go — inline arg parsing (ParseInlineArgs)
// Related: setparser_meta.go — metadata-annotated parsing (ParseWithMeta)

package config

import (
	"bufio"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The keywords a set-format line starts with. set and delete are the canonical
// CLI verbs in their engine path form, so their spelling comes from the verb
// registry (internal/component/command). nop and inactive belong to this file
// format alone.
const (
	cmdSet      = command.VerbSet
	cmdNop      = "nop"
	cmdDelete   = command.VerbDelete
	cmdInactive = "inactive"
)

// SetParser parses set-style configuration.
//
// Format:
//
//	set <path> <value>
//	delete <path>
//
// Examples:
//
//	set router-id 1.2.3.4
//	set neighbor 192.0.2.1 local-as 65000
//	set neighbor 192.0.2.1 family ipv4/unicast true
//	delete neighbor 192.0.2.1 peer-as
type SetParser struct {
	schema       *Schema
	preMigration bool     // collect unknown fields as warnings for migration (not final parse)
	warnings     []string // unknown field warnings from pre-migration parse
	// nopLine is true while a `nop` line is walked, so a bracket leaf-list
	// reads `nop <path> <member>` as the deactivation of a member the list
	// line already set (nopBracketMember), never as a new list.
	nopLine bool
}

// NewSetParser creates a new set-style parser with the given schema.
func NewSetParser(schema *Schema) *SetParser {
	return &SetParser{schema: schema}
}

// SetPreMigration enables pre-migration mode: unknown fields are collected
// as warnings instead of causing parse errors. This allows parsing configs
// with stale fields so that tree-level migrations can remove them.
func (p *SetParser) SetPreMigration(v bool) {
	p.preMigration = v
}

// Warnings returns warnings collected during pre-migration parsing.
func (p *SetParser) Warnings() []string {
	return p.warnings
}

// Parse parses the input string into a config tree.
func (p *SetParser) Parse(input string) (*Tree, error) {
	tree := NewTree()

	scanner := bufio.NewScanner(strings.NewReader(input))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if err := p.parseLine(tree, line, lineNum); err != nil {
			return nil, err
		}
	}

	return tree, scanner.Err()
}

// parseLine parses a single set/delete line.
func (p *SetParser) parseLine(tree *Tree, line string, lineNum int) error {
	tokens := p.tokenizeLine(line)
	if len(tokens) == 0 {
		return nil
	}

	cmd := tokens[0]
	tokens = tokens[1:]

	switch cmd {
	case cmdSet:
		return p.parseSet(tree, tokens, lineNum)
	case cmdNop:
		return p.parseNop(tree, tokens, lineNum)
	case cmdDelete:
		return p.parseDelete(tree, tokens, lineNum)
	case cmdInactive:
		return p.parseInactive(tree, tokens, lineNum)
	default:
		return fmt.Errorf("line %d: unknown command: %s (expected set/nop/delete/inactive)", lineNum, cmd)
	}
}

// bracketMembers answers the members a set-format line gives a bracket
// leaf-list, each token one member, every member validated against the leaf's
// type.
//
// One older spelling is still read: a draft written before each token became a
// member spelled a contested entry as one quoted value, `address "a/24 b/30"`.
// When a lone value fails as one member but every word of it passes, it is
// that spelling, and it answers the words. A value that is one valid member,
// such as a quoted description holding a space, stays one member.
func bracketMembers(bracket *BracketLeafListNode, tokens []string) ([]string, error) {
	items := bracketItems(tokens)
	var firstErr error
	for _, item := range items {
		if err := validateValuePatterns(bracket.Type, bracket.Patterns, item); err != nil {
			firstErr = err
			break
		}
	}
	if firstErr == nil {
		return items, nil
	}
	if len(tokens) != 1 {
		return nil, firstErr
	}
	words := strings.Fields(tokens[0])
	if len(words) < 2 {
		return nil, firstErr
	}
	for _, word := range words {
		if err := validateValuePatterns(bracket.Type, bracket.Patterns, word); err != nil {
			return nil, firstErr
		}
	}
	return words, nil
}

// isMemberLeafList reports whether node is a leaf-list whose members a
// `delete <path> <member>` or a `nop <path> <member>` line names one by one: a
// plain leaf-list or a bracket one.
func isMemberLeafList(node Node) bool {
	switch node.(type) {
	case *ValueOrArrayNode, *BracketLeafListNode:
		return true
	}
	return false
}

// tokenizeLine splits a line into tokens, respecting quotes.
func (p *SetParser) tokenizeLine(line string) []string {
	var tokens []string
	var current textbuf.Buffer
	inQuote := false
	quoteChar := byte(0)

	for i := range len(line) {
		ch := line[i]

		if inQuote {
			if ch == quoteChar {
				inQuote = false
				tokens = append(tokens, current.String())
			} else {
				current.Byte(ch)
			}
			continue
		}

		if ch == '"' || ch == '\'' {
			inQuote = true
			quoteChar = ch
			continue
		}

		if ch == ' ' || ch == '\t' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
			}
			continue
		}

		current.Byte(ch)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseSet handles: set <path...> <value>.
// Also tolerates structural-only commands (e.g., "set bgp peer foo") that create
// containers or list entries without setting a leaf value. This allows incomplete
// configs (work-in-progress editing) to load without failing.
func (p *SetParser) parseSet(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: set requires at least a path", lineNum)
	}

	// Walk the schema to find where to set the value
	return p.setSyntaxRefusal(tokens, p.walkAndSet(tree, p.schema.root, tokens, lineNum))
}

// walkAndSet walks the path and sets the value at the leaf.
// Returns nil when tokens are exhausted at a non-leaf node, which means
// the caller already created the container or list entry at this level.
func (p *SetParser) walkAndSet(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return nil // structural-only: container/entry exists, no leaf to set
	}

	name := tokens[0]
	tokens = tokens[1:]

	var node Node
	if parent == nil {
		// Start from schema root
		node = p.schema.Get(name)
	} else {
		switch n := parent.(type) {
		case *ContainerNode:
			node = n.Get(name)
		case *ListNode:
			node = n.Get(name)
		default:
			return fmt.Errorf("line %d: cannot traverse %T", lineNum, parent)
		}
	}

	if node == nil {
		if p.preMigration {
			var bw textbuf.Buffer
			p.warnings = append(p.warnings, bw.Reset().Str("line ").Int(int64(lineNum)).Str(": unknown field: ").Str(name).Str(" (needs migration)").String())
			return nil
		}
		return fmt.Errorf("line %d: unknown field: %s%s", lineNum, name, UnknownKeywordHint(name, schemaChildNames(p.schema, parent)))
	}

	//nolint:gocritic // if-else chain preferred over type switch for exhaustive node handling
	if leaf, ok := node.(*LeafNode); ok {
		if leaf.Type == TypeEmpty {
			// "type empty" leaves are presence flags: "set no-default-route".
			if len(tokens) != 0 {
				return fmt.Errorf("line %d: leaf %s is a flag and takes no value", lineNum, name)
			}
			tree.Set(name, configTrue)
			return nil
		}
		if len(tokens) != 1 {
			return fmt.Errorf("line %d: leaf %s expects exactly one value", lineNum, name)
		}
		value := tokens[0]
		if err := ValidateLeafValue(leaf, value); err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
		}
		tree.Set(name, NormalizeLeafValue(leaf.Type, value))
		return nil
	}

	if multi, ok := node.(*MultiLeafNode); ok {
		if len(tokens) < 1 {
			return fmt.Errorf("line %d: multi-leaf %s expects at least one value", lineNum, name)
		}
		value := textbuf.Join(tokens, " ")
		if err := validateValuePatterns(multi.Type, multi.Patterns, value); err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
		}
		tree.Set(name, value)
		return nil
	}

	if bracket, ok := node.(*BracketLeafListNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: no value to set
		}
		if handled, err := p.nopBracketMember(tree, bracket, name, tokens); handled {
			return err
		}
		// Each token is one member, a quoted one included, as the hierarchical
		// parser reads `[ a "b c" ]`, so the two parsers agree on the members.
		items, err := bracketMembers(bracket, tokens)
		if err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
		}
		// Members plus the scalar mirror, the store the hierarchical parser
		// fills (storeBracketLeafList), so a set-format load hands the editor
		// a leaf-list it merges member by member exactly as a hierarchical
		// one. A set line states the whole list, so a later line for the same
		// leaf replaces an earlier one.
		if err := setBracketLine(tree, name, items); err != nil {
			return fmt.Errorf("line %d: %s: %w", lineNum, name, err)
		}
		return nil
	}

	if valueOrArray, ok := node.(*ValueOrArrayNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: no value to set
		}
		// Normalize an inline "inactive:" member prefix into the out-of-band
		// marker (same accepted input form as the hierarchical parser), so the
		// stored member value stays clean and is validated clean.
		clean, deactivated := stripInactiveMemberPrefix(bracketItems(tokens))
		if err := validateValueOrArrayItems(valueOrArray, name, clean, lineNum); err != nil {
			return err
		}
		// Add-member merge: each line appends missing members (JunOS set
		// semantics); members land in the multi-value store the serializers
		// read, with the scalar map kept in sync for Get() callers.
		for _, item := range clean {
			tree.AddMultiValueMember(name, item)
		}
		for _, member := range deactivated {
			_ = tree.DeactivateMultiValue(name, member)
		}
		return nil
	}

	if container, ok := node.(*ContainerNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		return p.walkAndSet(child, container, tokens, lineNum)
	}

	if list, ok := node.(*ListNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: list node declared, no entries
		}
		if len(tokens) == 1 {
			// Key-only: create empty list entry (incomplete config tolerance).
			key := tokens[0]
			if err := ValidateListKey(list, key); err != nil {
				return fmt.Errorf("line %d: invalid key for %s: %w", lineNum, name, err)
			}
			entries := tree.GetList(name)
			if entries == nil || entries[key] == nil {
				tree.AddListEntry(name, key, NewTree())
			}
			return nil
		}
		return p.walkAndSetListEntry(tree, list, name, tokens, lineNum)
	}

	if _, ok := node.(*FreeformNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: no key to set
		}
		return setFreeformValue(tree, name, tokens, lineNum)
	}

	if flex, ok := node.(*FlexNode); ok {
		return p.setFlexValue(tree, flex, name, tokens, lineNum)
	}

	if il, ok := node.(*InlineListNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: inline-list declared, no entries
		}
		if len(tokens) == 1 {
			// Key-only: create empty inline-list entry (incomplete config tolerance).
			key := tokens[0]
			entries := tree.GetList(name)
			if entries == nil || entries[key] == nil {
				tree.AddListEntry(name, key, NewTree())
			}
			return nil
		}
		return p.walkAndSetInlineListEntry(tree, il, name, tokens, lineNum)
	}

	return fmt.Errorf("line %d: unknown node type %T for %s", lineNum, node, name)
}

// parseNop handles: nop <path...> [<value>].
// Sets the value (like parseSet) and marks the resolved node inactive in one step.
func (p *SetParser) parseNop(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: nop requires at least a path", lineNum)
	}
	p.nopLine = true
	err := p.walkAndSet(tree, p.schema.root, tokens, lineNum)
	p.nopLine = false
	if err != nil {
		return err
	}
	p.markNopInactive(tree, p.schema.root, tokens)
	return nil
}

// nopBracketMember handles `nop <path> <member>` on a bracket leaf-list, the
// set-format spelling of one deactivated member, as for a plain leaf-list. The
// serializer writes it after the line that sets the whole list, where the
// member is there and is deactivated. A line written before the list line, or
// naming a value the list does not hold yet, is still one member: the member
// is added deactivated, and a later list line that names it keeps it
// deactivated (setBracketLine). It answers false for any other line, which
// then takes the whole-leaf meaning: the value is the list, written in
// brackets or as the legacy joined value, and markNopInactive deactivates the
// leaf.
func (p *SetParser) nopBracketMember(tree *Tree, bracket *BracketLeafListNode, name string, tokens []string) (bool, error) {
	if !p.nopLine || len(tokens) != 1 || tokens[0] == "[" {
		return false, nil
	}
	present, inactive := tree.MultiValueMemberState(name, tokens[0])
	if inactive {
		return true, nil
	}
	if !present {
		// An invalid value, or a joined value that holds several members,
		// is left to the list path, which reports or reads it.
		items, err := bracketMembers(bracket, tokens)
		if err != nil {
			return false, nil //nolint:nilerr // The list path reports the error with its line number.
		}
		if len(items) != 1 {
			return false, nil
		}
		tree.AddMultiValueMember(name, items[0])
	}
	return true, tree.DeactivateMultiValue(name, tokens[0])
}

// setBracketLine stores the members a bracket leaf-list's set line states. The
// line states the whole list, so it replaces the members an earlier line
// stored, as it always has in this format, but a member a `nop <path>
// <member>` line deactivated stays deactivated while the new list names it:
// the member line says which member is deactivated, whichever line comes
// first.
func setBracketLine(tree *Tree, name string, items []string) error {
	var deactivated []string
	for _, state := range tree.GetMultiValuesState(name) {
		if state.Inactive && slices.Contains(items, state.Value) {
			deactivated = append(deactivated, state.Value)
		}
	}
	tree.SetSlice(name, items)
	for _, member := range deactivated {
		if err := tree.DeactivateMultiValue(name, member); err != nil {
			return err
		}
	}
	return nil
}

// markNopInactive walks the schema path and marks the terminal node
// inactive. Unlike walkAndMarkInactive, it understands that leaf value tokens
// are part of the token stream (consumed by the prior walkAndSet) and stops
// at the leaf name rather than trying to descend into the value.
//
//nolint:cyclop // exhaustive node-type handling mirrors walkAndMarkInactive
func (p *SetParser) markNopInactive(tree *Tree, parent Node, tokens []string) {
	if len(tokens) == 0 {
		return
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		return
	}

	if _, ok := node.(*LeafNode); ok {
		tree.SetLeafInactive(name, true)
		return
	}
	if _, ok := node.(*MultiLeafNode); ok {
		tree.SetLeafInactive(name, true)
		return
	}
	if _, ok := node.(*BracketLeafListNode); ok {
		// A member line was handled by nopBracketMember, which left the member
		// deactivated; only a list line deactivates the whole leaf.
		if len(tokens) == 1 {
			if present, inactive := tree.MultiValueMemberState(name, tokens[0]); present && inactive {
				return
			}
		}
		tree.SetLeafInactive(name, true)
		return
	}

	if _, ok := node.(*ValueOrArrayNode); ok {
		for _, item := range bracketItems(tokens) {
			_ = tree.DeactivateMultiValue(name, item)
		}
		return
	}

	if container, ok := node.(*ContainerNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			return
		}
		if len(tokens) == 0 {
			child.SetInactive(true)
			return
		}
		p.markNopInactive(child, container, tokens)
		return
	}

	if list, ok := node.(*ListNode); ok {
		if len(tokens) == 0 {
			return
		}
		addr, err := ParseListAddress(name, list, tokens)
		if err != nil {
			return // An address the codec refuses names no entry, so nothing is marked.
		}
		entry := tree.GetList(name)[addr.Key]
		if entry == nil {
			return
		}
		if len(tokens) == addr.Tokens {
			entry.SetInactive(true)
			return
		}
		p.markNopInactive(entry, list, tokens[addr.Tokens:])
		return
	}

	if flex, ok := node.(*FlexNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		if len(tokens) == 0 {
			child.SetInactive(true)
			return
		}
		p.markNopInactive(child, flex, tokens)
	}
}

// parseInactive handles: inactive <path...>.
//
// Single-keyword design: the presence of `inactive <path>` declares the
// node at <path> inactive. There is no symmetric "active" or
// "deactivate"/"activate" verb pair -- absence of the line means the
// node is active. Removing the inactive declaration (manually, or by
// the editor's CLI/TUI activate path which re-serializes without it)
// re-activates the node.
//
// The path resolves to a leaf, container, list entry, or single
// leaf-list value; dispatch mirrors the block-format `inactive:`
// prefix and the one-shot CLI verbs.
func (p *SetParser) parseInactive(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: inactive requires a path", lineNum)
	}
	return p.walkAndMarkInactive(tree, p.schema.root, tokens, lineNum)
}

// walkAndMarkInactive walks the path through schema and tree and sets
// the inactive marker at the resolved node.
//
//nolint:cyclop // exhaustive node-type handling, mirrors walkAndDelete
func (p *SetParser) walkAndMarkInactive(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return fmt.Errorf("line %d: incomplete inactive path", lineNum)
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		// In pre-migration mode unknown fields are warnings rather than
		// errors so that a renamed YANG path can still parse and the
		// migration step can rewrite it. Mirror the set / delete paths.
		if p.preMigration {
			var bw textbuf.Buffer
			p.warnings = append(p.warnings, bw.Reset().Str("line ").Int(int64(lineNum)).Str(": unknown field: ").Str(name).Str(" (needs migration)").String())
			return nil
		}
		return fmt.Errorf("line %d: unknown field: %s%s", lineNum, name, UnknownKeywordHint(name, schemaChildNames(p.schema, parent)))
	}

	if _, ok := node.(*LeafNode); ok {
		if len(tokens) != 0 {
			return fmt.Errorf("line %d: unexpected tokens after leaf %s", lineNum, name)
		}
		tree.SetLeafInactive(name, true)
		return nil
	}
	if _, ok := node.(*MultiLeafNode); ok {
		tree.SetLeafInactive(name, true)
		return nil
	}
	// A bracket leaf-list takes the same two forms as a plain one below. The
	// serializer writes `nop <path> <member>` now; `inactive <path> <member>`
	// is the older spelling of the same thing, and the member MUST already be
	// in the list.
	if _, ok := node.(*BracketLeafListNode); ok {
		switch len(tokens) {
		case 0:
			tree.SetLeafInactive(name, true)
			return nil
		case 1:
			if present, _ := tree.MultiValueMemberState(name, tokens[0]); !present {
				return fmt.Errorf("line %d: inactive %s %s names no member of %s; set the member in the %s list first", lineNum, name, tokens[0], name, name)
			}
			return tree.DeactivateMultiValue(name, tokens[0])
		}
		return fmt.Errorf("line %d: inactive on leaf-list expects either no value or one value, got %d", lineNum, len(tokens))
	}

	// ValueOrArray: a final token denotes a single leaf-list value
	// (deactivate one element); no token denotes the whole leaf-list.
	if _, ok := node.(*ValueOrArrayNode); ok {
		if len(tokens) == 1 {
			return tree.DeactivateMultiValue(name, tokens[0])
		}
		if len(tokens) == 0 {
			tree.SetLeafInactive(name, true)
			return nil
		}
		return fmt.Errorf("line %d: inactive on leaf-list expects either no value or one value, got %d", lineNum, len(tokens))
	}

	// Container: mark via the Tree-level inactive bool.
	if container, ok := node.(*ContainerNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		if len(tokens) == 0 {
			child.SetInactive(true)
			return nil
		}
		return p.walkAndMarkInactive(child, container, tokens, lineNum)
	}

	// List: a key token selects an entry; further tokens descend
	// into the entry's fields.
	if list, ok := node.(*ListNode); ok {
		if len(tokens) == 0 {
			return fmt.Errorf("line %d: inactive on list %s requires a key", lineNum, name)
		}
		addr, err := ParseListAddress(name, list, tokens)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
		entry := EnsureListEntry(tree, name, addr)
		if len(tokens) == addr.Tokens {
			entry.SetInactive(true)
			return nil
		}
		return p.walkAndMarkInactive(entry, list, tokens[addr.Tokens:], lineNum)
	}

	if flex, ok := node.(*FlexNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		if len(tokens) == 0 {
			child.SetInactive(true)
			return nil
		}
		return p.walkAndMarkInactive(child, flex, tokens, lineNum)
	}

	return fmt.Errorf("line %d: inactive not supported on %T (%s)", lineNum, node, name)
}

// parseDelete handles: delete <path...>.
func (p *SetParser) parseDelete(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: delete requires path", lineNum)
	}

	return p.walkAndDelete(tree, p.schema.root, tokens, lineNum)
}

// walkAndDelete walks the path and deletes the target.
//
//nolint:cyclop // exhaustive node type handling
func (p *SetParser) walkAndDelete(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return fmt.Errorf("line %d: incomplete delete path", lineNum)
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		if p.preMigration {
			var bw textbuf.Buffer
			p.warnings = append(p.warnings, bw.Reset().Str("line ").Int(int64(lineNum)).Str(": unknown field: ").Str(name).Str(" (needs migration)").String())
			return nil
		}
		return fmt.Errorf("line %d: unknown field: %s%s", lineNum, name, UnknownKeywordHint(name, schemaChildNames(p.schema, parent)))
	}

	// Leaf-list member delete: `delete <path> <member>` removes one member, of
	// a plain or a bracket leaf-list alike.
	if isMemberLeafList(node) && len(tokens) == 1 {
		tree.RemoveMultiValueMember(name, tokens[0])
		return nil
	}

	// Leaf-like types: delete the value directly.
	if isLeafLike(node) {
		if len(tokens) != 0 {
			return fmt.Errorf("line %d: unexpected tokens after leaf %s", lineNum, name)
		}
		tree.Delete(name)
		return nil
	}

	if container, ok := node.(*ContainerNode); ok {
		if len(tokens) == 0 {
			tree.DeleteContainer(name)
			return nil
		}
		child := tree.GetContainer(name)
		if child == nil {
			return nil // Already doesn't exist
		}
		return p.walkAndDelete(child, container, tokens, lineNum)
	}

	if list, ok := node.(*ListNode); ok {
		return p.deleteFromList(tree, list, name, tokens, lineNum)
	}

	if _, ok := node.(*FreeformNode); ok {
		return deleteFreeformEntry(tree, name, tokens, lineNum)
	}

	if flex, ok := node.(*FlexNode); ok {
		if len(tokens) == 0 {
			// Delete the flex value/container.
			tree.Delete(name)
			tree.DeleteContainer(name)
			return nil
		}
		child := tree.GetContainer(name)
		if child == nil {
			return nil
		}
		return p.walkAndDelete(child, flex, tokens, lineNum)
	}

	if il, ok := node.(*InlineListNode); ok {
		return p.deleteFromInlineList(tree, il, name, tokens, lineNum)
	}

	return fmt.Errorf("line %d: unknown node type %T for %s", lineNum, node, name)
}

// isLeafLike returns true for terminal node types that store a single value.
func isLeafLike(node Node) bool {
	switch node.(type) {
	case *LeafNode, *MultiLeafNode, *BracketLeafListNode, *ValueOrArrayNode:
		return true
	}
	return false
}

// validateValueOrArrayItems checks every leaf-list item against the node's
// enum values and type patterns. Shared by the plain and metadata-aware
// set parsers.
func validateValueOrArrayItems(node *ValueOrArrayNode, name string, items []string, lineNum int) error {
	for _, item := range items {
		if node.ValidValues != nil && !containsString(node.ValidValues, item) {
			return fmt.Errorf("line %d: invalid value for %s: %q (valid: %s)", lineNum, name, item, textbuf.Join(node.ValidValues, ", "))
		}
		if err := validateValuePatterns(node.Type, node.Patterns, item); err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
		}
	}
	return nil
}

// deleteFromList handles delete for ListNode (entire list, entry, or field within entry).
//
// A whole-entry delete goes through RemoveListEntry, which is the only mutator
// that drops the key from the recorded entry order as well as from the list.
// GetList hands out the live map, so deleting from it directly leaves the order
// naming a key the list no longer holds (Tree.RemoveListEntry).
func (p *SetParser) deleteFromList(tree *Tree, list *ListNode, name string, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		tree.DeleteList(name)
		return nil
	}
	addr, err := ParseListAddress(name, list, tokens)
	if err != nil {
		return fmt.Errorf("line %d: %w", lineNum, err)
	}
	tokens = tokens[addr.Tokens:]
	entries := tree.GetList(name)
	if entries == nil {
		return nil
	}
	if len(tokens) == 0 {
		tree.RemoveListEntry(name, addr.Key)
		return nil
	}
	entry := entries[addr.Key]
	if entry == nil {
		return nil
	}
	return p.walkAndDelete(entry, list, tokens, lineNum)
}

// deleteFreeformEntry handles delete for FreeformNode entries.
func deleteFreeformEntry(tree *Tree, name string, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		tree.DeleteContainer(name)
		return nil
	}
	if len(tokens) != 1 {
		return fmt.Errorf("line %d: freeform delete expects 0 or 1 key after %s", lineNum, name)
	}
	child := tree.GetContainer(name)
	if child == nil {
		return nil
	}
	child.Delete(tokens[0])
	return nil
}

// deleteFromInlineList handles delete for InlineListNode entries. A whole-entry
// delete goes through RemoveListEntry for the reason deleteFromList states.
func (p *SetParser) deleteFromInlineList(tree *Tree, il *InlineListNode, name string, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		tree.DeleteList(name)
		return nil
	}
	key := tokens[0]
	tokens = tokens[1:]
	entries := tree.GetList(name)
	if entries == nil {
		return nil
	}
	if len(tokens) == 0 {
		tree.RemoveListEntry(name, key)
		return nil
	}
	entry := entries[key]
	if entry == nil {
		return nil
	}
	return p.walkAndDelete(entry, il, tokens, lineNum)
}

// Delete removes a leaf value, its multi-value members, and its
// insertion-order entry. Clearing multiValues matters for leaf-lists: the
// serializers read the multi-value store, so leaving it populated would
// resurrect a deleted leaf-list on the next serialize.
// No-op if the key does not exist.
func (t *Tree) Delete(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.multiValues, name)
	delete(t.mirrored, name)
	delete(t.inactiveMembers, name)
	if _, exists := t.values[name]; !exists {
		return
	}
	delete(t.values, name)

	// Remove from valuesOrder
	for i, k := range t.valuesOrder {
		if k == name {
			t.valuesOrder = append(t.valuesOrder[:i], t.valuesOrder[i+1:]...)
			break
		}
	}
}

// DeleteContainer removes a container from the tree.
func (t *Tree) DeleteContainer(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.containers, name)
}

// DeleteList removes an entire list from the tree, and the entry order recorded
// for it. Leaving the order behind makes t.lists and t.listOrder disagree, and
// a list re-added afterwards would be lowered in the order the DELETED list had
// (Tree.ToPluginMap, entryOrderLocked).
func (t *Tree) DeleteList(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.lists, name)
	delete(t.listOrder, name)
}

// resolveSchemaNode looks up a child node from the parent, handling the root case.
// Supports all parent types that have children: Container, List, Flex, InlineList.
func resolveSchemaNode(schema *Schema, parent Node, name string) Node {
	if parent == nil {
		return schema.Get(name)
	}
	if c, ok := parent.(*ContainerNode); ok {
		return c.Get(name)
	}
	if l, ok := parent.(*ListNode); ok {
		return l.Get(name)
	}
	if f, ok := parent.(*FlexNode); ok {
		return f.Get(name)
	}
	if il, ok := parent.(*InlineListNode); ok {
		return il.Get(name)
	}
	return nil
}

// schemaChildNames lists the keys valid under parent (the schema root when
// parent is nil), for the closest-key hint of an unknown-field refusal. A node
// with no children answers nil, and the refusal then carries no hint.
func schemaChildNames(schema *Schema, parent Node) []string {
	if parent == nil {
		return schema.Children()
	}
	if c, ok := parent.(ChildProvider); ok {
		return c.Children()
	}
	return nil
}

// NormalizeLeafValue rewrites an accepted leaf value into the one spelling the
// tree stores. Every reader downstream then sees one form, whichever spelling
// the operator typed.
//
// It runs AFTER validation on every path that writes a leaf:
//
//   - the config file, parser.go
//   - `set`, setparser.go
//   - the inline attribute form, setparser_inline.go
//   - the meta form, setparser_meta.go
//   - the editor behind the CLI and the web form, cli.Editor.SetValue
//
// A value that does not parse is returned unchanged. Validation refused it on
// every caller's path already, and a value invented here would hide which
// spelling was rejected.
func NormalizeLeafValue(typ ValueType, value string) string {
	//exhaustive:ignore // Only boolean and ASN leaves need canonical spelling conversion.
	switch typ {
	case TypeBool:
		return NormalizeBool(value)
	case TypeASN:
		// An AS number is stored as its decimal value, whichever notation the
		// operator typed. The readers of local-as, peer-as and every other AS
		// number leaf therefore parse one form.
		number, err := asn.Parse(value)
		if err != nil {
			return value
		}
		return asn.Text(number, asn.NotationPlain)
	default:
		return value
	}
}

func bracketItems(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	if tokens[0] == "[" && tokens[len(tokens)-1] == "]" {
		return tokens[1 : len(tokens)-1]
	}
	return tokens
}

// setFreeformValue stores a freeform key-value pair.
// Format: set <path> <freeform-name> <key> [value]
// Stored as container[name] -> Tree with key=value (or key="true" for flags).
func setFreeformValue(tree *Tree, name string, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: freeform %s requires at least a key", lineNum, name)
	}
	child := tree.GetOrCreateContainer(name)
	key := tokens[0]
	value := configTrue
	if len(tokens) > 1 {
		value = textbuf.Join(tokens[1:], " ")
	}
	child.Set(key, value)
	return nil
}

// setFlexValue handles FlexNode's multiple forms: flag, value, or container with children.
// If remaining tokens match a known child name, recurse. Otherwise treat as a leaf value.
func (p *SetParser) setFlexValue(tree *Tree, flex *FlexNode, name string, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		// Flag form: just the name with no value.
		tree.Set(name, configTrue)
		return nil
	}

	// If the first token matches a child in the flex schema, recurse into container form.
	if flex.Get(tokens[0]) != nil {
		child := tree.GetOrCreateContainer(name)
		return p.walkAndSet(child, flex, tokens, lineNum)
	}

	// Otherwise treat as a simple value.
	tree.Set(name, textbuf.Join(tokens, " "))
	return nil
}

// walkAndSetListEntry creates or finds a list entry and recurses.
func (p *SetParser) walkAndSetListEntry(tree *Tree, list *ListNode, name string, tokens []string, lineNum int) error {
	if err := ValidateListKey(list, tokens[0]); err != nil {
		return fmt.Errorf("line %d: invalid key for %s: %w", lineNum, name, err)
	}
	addr, err := ParseListAddress(name, list, tokens)
	if err != nil {
		return fmt.Errorf("line %d: %w", lineNum, err)
	}
	entry := EnsureListEntry(tree, name, addr)
	return p.walkAndSet(entry, list, tokens[addr.Tokens:], lineNum)
}

// walkAndSetInlineListEntry creates or finds an inline list entry and recurses.
func (p *SetParser) walkAndSetInlineListEntry(tree *Tree, il *InlineListNode, name string, tokens []string, lineNum int) error {
	key := tokens[0]
	tokens = tokens[1:]
	entries := tree.GetList(name)
	if entries == nil {
		entries = make(map[string]*Tree)
	}
	entry := entries[key]
	if entry == nil {
		entry = NewTree()
		tree.AddListEntry(name, key, entry)
	}
	return p.walkAndSet(entry, il, tokens, lineNum)
}

// Inline arg parsing moved to setparser_inline.go
