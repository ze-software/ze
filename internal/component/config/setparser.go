// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: parser.go — config parser core
// Related: serialize_set.go — set-format serialization (inverse of this file)
// Related: meta.go — MetaTree for metadata-aware parsing
// Detail: setparser_inline.go — inline arg parsing (ParseInlineArgs)
// Detail: setparser_entries.go — Flex, freeform, list and inline-list entry writers
// Related: setparser_meta.go — metadata-annotated parsing (ParseWithMeta)

package config

import (
	"bufio"
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
	preMigration bool // skip unknown fields and record them, for the refusal that names them all (not a final parse)
	// unknown holds each field the pre-migration parse skipped, typed with
	// its line, so parseSetConfig names each line once (unknownFieldsError).
	unknown []*LineError
	// nopLine is true while a `nop` line is walked, so a bracket leaf-list
	// reads `nop <path> <member>` as the deactivation of a member the list
	// line already set (nopBracketMember), never as a new list.
	nopLine bool
	// inactiveLine is true while an `inactive` line is walked, so a refusal's
	// fix is an `inactive` line, the line the operator typed (lineVerb).
	inactiveLine bool
	// lineTokens is the path of the line being parsed, its verb left out, so
	// an unknown keyword's refusal can name the keywords before it
	// (keywordsBefore) and ask KeywordHint whether a compiled-out plugin owns
	// that position.
	lineTokens []string
}

// NewSetParser creates a new set-style parser with the given schema.
func NewSetParser(schema *Schema) *SetParser {
	return &SetParser{schema: schema}
}

// SetPreMigration enables pre-migration mode: an unknown field is skipped
// and recorded (UnknownFields) instead of ending the parse, so one parse
// finds every unknown field of a config.
func (p *SetParser) SetPreMigration(v bool) {
	p.preMigration = v
}

// UnknownFields answers each field the pre-migration parse skipped, in the
// order of the config's lines, each carrying its own line.
func (p *SetParser) UnknownFields() []*LineError {
	return p.unknown
}

// skipUnknownField records name, unknown on line lineNum, and lets the
// pre-migration parse go on.
func (p *SetParser) skipUnknownField(lineNum int, name string) {
	p.unknown = append(p.unknown, unknownFieldLine(lineNum, name))
}

// Parse parses the input string into a config tree.
func (p *SetParser) Parse(input string) (*Tree, error) {
	tree := NewTree()

	scanner := bufio.NewScanner(strings.NewReader(input))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := trimTokenSpace(scanner.Text())

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
	tokens, err := splitQuotedTokens(line)
	if err != nil {
		return newLineError(lineNum, err)
	}
	if len(tokens) == 0 {
		return nil
	}

	cmd := tokens[0]
	tokens = tokens[1:]
	p.lineTokens = tokens

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
		return lineErrorf(lineNum, "unknown command: %s (expected set/nop/delete/inactive)", cmd)
	}
}

// bracketMembers answers the members a set-format line gives a bracket
// leaf-list, each token one member, every member validated against the leaf's
// type. path is the setting path of the leaf-list, which a refusal names.
//
// One older spelling is still read: a draft written before each token became a
// member spelled a contested entry as one quoted value, `address "a/24 b/30"`.
// When a lone value fails as one member but every word of it passes, it is
// that spelling, and it answers the words. A value that is one valid member,
// such as a quoted description holding a space, stays one member.
func bracketMembers(bracket *BracketLeafListNode, path, tokens []string) ([]string, error) {
	items := bracketItems(tokens)
	member := bracket.memberLeaf()
	firstErr := validateMembers(member, path, items)
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
		if err := ValidateLeafValue(member, path, word); err != nil {
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

// parseSet handles: set <path...> <value>.
// Also tolerates structural-only commands (e.g., "set bgp peer foo") that create
// containers or list entries without setting a leaf value. This allows incomplete
// configs (work-in-progress editing) to load without failing.
func (p *SetParser) parseSet(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return lineErrorf(lineNum, "set requires at least a path")
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

	// The children of a Flex block or an InlineList entry are reached here
	// too, as walkAndSetWithMeta reaches them: setFlexValue and
	// walkAndSetInlineListEntry recurse with that node as the parent.
	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		if p.preMigration {
			p.skipUnknownField(lineNum, name)
			return nil
		}
		return p.unknownField(lineNum, name, parent, tokens)
	}

	//nolint:gocritic // if-else chain preferred over type switch for exhaustive node handling
	if leaf, ok := node.(*LeafNode); ok {
		if leaf.Type == TypeEmpty {
			// "type empty" leaves are presence flags: "set no-default-route".
			if len(tokens) != 0 {
				return lineErrorf(lineNum, "leaf %s is a flag and takes no value", name)
			}
			tree.Set(name, configTrue)
			return nil
		}
		if len(tokens) != 1 {
			return lineErrorf(lineNum, "leaf %s expects exactly one value", name)
		}
		// A $9$ key decodes as it does in the hierarchical form (parseLeaf).
		value, err := decodeSecretLeaf(tree, name, leaf, tokens[0])
		if err != nil {
			return newLineError(lineNum, err)
		}
		if err := ValidateLeafValue(leaf, p.lineAt(tokens), value); err != nil {
			return p.valueLineRefusal(lineNum, name, tokens, err)
		}
		tree.Set(name, NormalizeLeafValue(leaf.Type, value))
		return nil
	}

	if multi, ok := node.(*MultiLeafNode); ok {
		if len(tokens) < 1 {
			return lineErrorf(lineNum, "multi-leaf %s expects at least one value", name)
		}
		value := textbuf.Join(tokens, " ")
		if err := ValidateLeafValue(multi.memberLeaf(), p.lineAt(tokens), value); err != nil {
			return p.valueLineRefusal(lineNum, name, tokens, err)
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
		items, err := bracketMembers(bracket, p.lineAt(tokens), tokens)
		if err != nil {
			return p.valueLineRefusal(lineNum, name, tokens, err)
		}
		// Members plus the scalar mirror, the store the hierarchical parser
		// fills (storeBracketLeafList), so a set-format load hands the editor
		// a leaf-list it merges member by member exactly as a hierarchical
		// one. A set line states the whole list, so a later line for the same
		// leaf replaces an earlier one.
		if err := setBracketLine(tree, name, items); err != nil {
			return lineErrorf(lineNum, "%s: %w", name, err)
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
		if err := p.validateValueOrArrayItems(valueOrArray, name, tokens, clean, lineNum); err != nil {
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
			// Key-only: create the empty entry (incomplete config tolerance)
			// through the one creator. The typed key is judged first, as the
			// line types it, whether or not its entry is stored.
			if err := p.lineKeyRefusal(lineNum, list, name, tokens); err != nil {
				return err
			}
			addr, err := ParseListAddress(name, list, tokens)
			if err != nil {
				return p.addressLineRefusal(lineNum, tokens, err)
			}
			if _, err := EnsureListEntry(tree, p.lineAt(tokens), list, addr); err != nil {
				return p.keyLineRefusal(lineNum, name, tokens, err)
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

	return lineErrorf(lineNum, "unknown node type %T for %s", node, name)
}

// parseNop handles: nop <path...> [<value>].
// Sets the value (like parseSet) and marks the resolved node inactive in one step.
func (p *SetParser) parseNop(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return lineErrorf(lineNum, "nop requires at least a path")
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
		items, err := bracketMembers(bracket, p.lineAt(tokens), tokens)
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
		return lineErrorf(lineNum, "inactive requires a path")
	}
	p.inactiveLine = true
	err := p.walkAndMarkInactive(tree, p.schema.root, tokens, lineNum)
	p.inactiveLine = false
	return err
}

// walkAndMarkInactive walks the path through schema and tree and sets
// the inactive marker at the resolved node.
//
//nolint:cyclop // exhaustive node-type handling, mirrors walkAndDelete
func (p *SetParser) walkAndMarkInactive(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return lineErrorf(lineNum, "incomplete inactive path")
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		// In pre-migration mode an unknown field is skipped and recorded
		// rather than ending the parse, so the refusal names every unknown
		// field. Mirror the set / delete paths.
		if p.preMigration {
			p.skipUnknownField(lineNum, name)
			return nil
		}
		return p.unknownField(lineNum, name, parent, tokens)
	}

	if _, ok := node.(*LeafNode); ok {
		if len(tokens) != 0 {
			return lineErrorf(lineNum, "unexpected tokens after leaf %s", name)
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
				return lineErrorf(lineNum, "inactive %s %s names no member of %s; set the member in the %s list first", name, tokens[0], name, name)
			}
			return tree.DeactivateMultiValue(name, tokens[0])
		}
		return lineErrorf(lineNum, "inactive on leaf-list expects either no value or one value, got %d", len(tokens))
	}

	// valueOrArray: a final token denotes a single leaf-list value
	// (deactivate one element); no token denotes the whole leaf-list.
	if _, ok := node.(*ValueOrArrayNode); ok {
		if len(tokens) == 1 {
			return tree.DeactivateMultiValue(name, tokens[0])
		}
		if len(tokens) == 0 {
			tree.SetLeafInactive(name, true)
			return nil
		}
		return lineErrorf(lineNum, "inactive on leaf-list expects either no value or one value, got %d", len(tokens))
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
			return lineErrorf(lineNum, "inactive on list %s requires a key", name)
		}
		addr, err := ParseListAddress(name, list, tokens)
		if err != nil {
			return p.addressLineRefusal(lineNum, tokens, err)
		}
		entry, err := EnsureListEntry(tree, p.lineAt(tokens), list, addr)
		if err != nil {
			return p.keyLineRefusal(lineNum, name, tokens, err)
		}
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

	return lineErrorf(lineNum, "inactive not supported on %T (%s)", node, name)
}

// parseDelete handles: delete <path...>.
func (p *SetParser) parseDelete(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return lineErrorf(lineNum, "delete requires path")
	}

	return p.walkAndDelete(tree, p.schema.root, tokens, lineNum)
}

// walkAndDelete walks the path and deletes the target.
//
//nolint:cyclop // exhaustive node type handling
func (p *SetParser) walkAndDelete(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return lineErrorf(lineNum, "incomplete delete path")
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		if p.preMigration {
			p.skipUnknownField(lineNum, name)
			return nil
		}
		return p.unknownField(lineNum, name, parent, tokens)
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
			return lineErrorf(lineNum, "unexpected tokens after leaf %s", name)
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
			// Delete the flex value/container and its named block entries.
			tree.Delete(name)
			tree.DeleteContainer(name)
			tree.DeleteList(name)
			return nil
		}
		// A named block entry, as setFlexValue stores it.
		if entry := tree.GetList(name)[tokens[0]]; entry != nil && flex.Get(tokens[0]) == nil {
			if len(tokens) == 1 {
				tree.RemoveListEntry(name, tokens[0])
				return nil
			}
			return p.walkAndDelete(entry, flex, tokens[1:], lineNum)
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

	return lineErrorf(lineNum, "unknown node type %T for %s", node, name)
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
func (p *SetParser) validateValueOrArrayItems(node *ValueOrArrayNode, name string, rest, items []string, lineNum int) error {
	if err := validateMembers(node.memberLeaf(), p.lineAt(rest), items); err != nil {
		return p.valueLineRefusal(lineNum, name, rest, err)
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
		return p.addressLineRefusal(lineNum, tokens, err)
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
		return lineErrorf(lineNum, "freeform delete expects 0 or 1 key after %s", name)
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

// unknownField refuses a name the schema does not hold under parent: as a node
// of another OS when the build pruned it for its ze:os, else as an unknown
// field with unknownFieldHint's hint. rest is what follows name on the line.
func (p *SetParser) unknownField(lineNum int, name string, parent Node, rest []string) error {
	if parent == nil {
		parent = p.schema.root
	}
	if err := nodeOfAnotherOS(parent, parentLabel(p.schema, parent), name, lineNum); err != nil {
		return err
	}
	return lineErrorf(lineNum, "unknown field: %s%s", name, p.unknownFieldHint(name, parent, rest))
}

// parentLabel names a schema node in a refusal: "the top level" for the
// root, and "" below it, where the set parser walks a path and holds no name.
func parentLabel(schema *Schema, parent Node) string {
	if container, ok := parent.(*ContainerNode); ok && container == schema.root {
		return "the top level"
	}
	return ""
}

// unknownFieldHint answers what the refusal of an unknown field adds after its
// name: KeywordHint over the keywords the line holds before it, so a keyword a
// compiled-out plugin owns there names its build tag, at the top level and
// below it. When the line does not hold name where rest says
// (keywordsBefore), the position is unknown and the hint is the closest key or
// a retired keyword's replacement alone (UnknownKeywordHint): a guessed
// position could name a tag for a plugin that does not own the keyword.
func (p *SetParser) unknownFieldHint(name string, parent Node, rest []string) string {
	known := schemaChildNames(p.schema, parent)
	before, ok := p.keywordsBefore(name, rest)
	if !ok {
		return UnknownKeywordHint(name, known)
	}
	return KeywordHint(before, name, known)
}

// keywordsBefore answers the tokens of the line being parsed that come before
// name, when rest is what follows name on that line, and false when the line
// does not hold name at that position. A list key among them makes the path
// match no gated root, which is the answer for a keyword inside a list entry.
func (p *SetParser) keywordsBefore(name string, rest []string) ([]string, bool) {
	at := len(p.lineTokens) - len(rest) - 1
	if at < 0 {
		return nil, false
	}
	if p.lineTokens[at] != name {
		return nil, false
	}
	return p.lineTokens[:at], true
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
	// Only boolean and ASN leaves need canonical spelling conversion.
	//exhaustive:ignore
	switch typ {
	case TypeBool:
		return normalizeEnableBool(value)
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

// The entry writers (Flex, freeform, list and inline-list entries) are in
// setparser_entries.go, and inline arg parsing is in setparser_inline.go.
