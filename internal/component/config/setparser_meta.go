// Design: docs/architecture/config/syntax.md -- Set-command parser with metadata annotations
// Related: setparser.go -- base set/delete/inactive parsing

package config

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ParseWithMeta parses set-format input with optional metadata prefixes.
// Returns both the config Tree and a MetaTree with authorship information.
//
// Metadata tokens are consumed before the set/delete command:
//   - #user -> MetaEntry.User
//   - @source -> MetaEntry.Source (connection origin)
//   - %ISO8601 -> MetaEntry.Time (session start time)
//   - "# text" (hash + space) -> comment, line skipped
func (p *SetParser) ParseWithMeta(input string) (*Tree, *MetaTree, error) {
	tree := NewTree()
	meta := NewMetaTree()

	scanner := bufio.NewScanner(strings.NewReader(input))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := trimTokenSpace(scanner.Text())

		if line == "" {
			continue
		}

		// "# text" (hash + space) is a comment
		if strings.HasPrefix(line, "# ") {
			continue
		}

		// Extract metadata tokens and the remaining command
		entry, cmdLine := extractMeta(line)

		// After stripping metadata, we may have an empty line
		if cmdLine == "" {
			continue
		}

		if err := p.parseLineWithMeta(tree, meta, cmdLine, entry, lineNum); err != nil {
			return nil, nil, err
		}
	}

	return tree, meta, scanner.Err()
}

// maxMetaFieldLen caps metadata field values from change files to prevent abuse.
const maxMetaFieldLen = 256

// capMetaField truncates a metadata field value to maxMetaFieldLen bytes.
func capMetaField(s string) string {
	if len(s) > maxMetaFieldLen {
		return s[:maxMetaFieldLen]
	}
	return s
}

// extractMeta consumes metadata tokens from the beginning of a line.
// Returns the MetaEntry and the remaining command string.
func extractMeta(line string) (MetaEntry, string) {
	var entry MetaEntry
	remaining := line

	for remaining != "" {
		if remaining[0] == '#' && len(remaining) > 1 && remaining[1] != ' ' {
			// User metadata: #user (capped at 256 bytes to prevent abuse)
			// Legacy detection: #user@origin contains '@' -- split into User + Source.
			end := strings.IndexByte(remaining, ' ')
			var raw string
			if end == -1 {
				raw = remaining[1:]
				remaining = ""
			} else {
				raw = remaining[1:end]
				remaining = trimTokenSpace(remaining[end+1:])
			}
			if at := strings.IndexByte(raw, '@'); at > 0 && entry.Source == "" {
				entry.User = capMetaField(raw[:at])
				entry.Source = capMetaField(raw[at+1:])
			} else {
				entry.User = capMetaField(raw)
			}
			continue
		}

		if remaining[0] == '@' {
			// Source metadata: @origin (e.g., "local", "192.168.1.5") (capped at 256 bytes)
			// Legacy: if Source already set (from #user@origin split) and value looks
			// like ISO 8601, treat as Time instead of overwriting Source.
			end := strings.IndexByte(remaining, ' ')
			var val string
			if end == -1 {
				val = remaining[1:]
				remaining = ""
			} else {
				val = remaining[1:end]
				remaining = trimTokenSpace(remaining[end+1:])
			}
			if entry.Source == "" {
				entry.Source = capMetaField(val)
			} else {
				// Source already populated (legacy #user@origin split).
				// Try parsing as time -- old format used @ISO8601 for timestamp.
				if t, err := time.Parse(time.RFC3339, val); err == nil {
					entry.Time = t
				} else if t, err := time.Parse("2006-01-02T15:04:05Z", val); err == nil {
					entry.Time = t
				}
				// If not parseable as time, discard (don't overwrite Source).
			}
			continue
		}

		if remaining[0] == '%' {
			// Time metadata: %ISO8601 (session start time)
			end := strings.IndexByte(remaining, ' ')
			var timeStr string
			if end == -1 {
				timeStr = remaining[1:]
				remaining = ""
			} else {
				timeStr = remaining[1:end]
				remaining = trimTokenSpace(remaining[end+1:])
			}
			if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
				entry.Time = t
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", timeStr); err == nil {
				entry.Time = t
			} else if t, err := time.Parse("2006-01-02T15:04:05", timeStr); err == nil {
				entry.Time = t
			}
			continue
		}

		if remaining[0] == '^' {
			// Previous value metadata: ^value or ^"multi word value".
			// Quoted form supports backslash escapes: \" for quote, \\ for backslash.
			if len(remaining) > 2 && remaining[1] == '"' {
				// Quoted: find unescaped closing quote.
				var prev textbuf.Buffer
				i := 2
				for i < len(remaining) {
					if remaining[i] == '\\' && i+1 < len(remaining) {
						next := remaining[i+1]
						if next == '"' || next == '\\' {
							prev.Byte(next)
							i += 2
							continue
						}
					}
					if remaining[i] == '"' {
						break
					}
					prev.Byte(remaining[i])
					i++
				}
				entry.Previous = capMetaField(prev.String())
				if i < len(remaining) {
					remaining = trimTokenSpace(remaining[i+1:])
				} else {
					remaining = ""
				}
			} else {
				// Unquoted: ^value
				end := strings.IndexByte(remaining, ' ')
				if end == -1 {
					entry.Previous = capMetaField(remaining[1:])
					remaining = ""
				} else {
					entry.Previous = capMetaField(remaining[1:end])
					remaining = trimTokenSpace(remaining[end+1:])
				}
			}
			continue
		}

		// Not a metadata token, stop consuming
		break
	}

	return entry, remaining
}

// parseLineWithMeta parses a set/delete command and records metadata.
func (p *SetParser) parseLineWithMeta(tree *Tree, meta *MetaTree, line string, entry MetaEntry, lineNum int) error {
	tokens, err := splitQuotedTokens(line)
	if err != nil {
		return fmt.Errorf("line %d: %w", lineNum, err)
	}
	if len(tokens) == 0 {
		return nil
	}

	cmd := tokens[0]
	tokens = tokens[1:]

	switch cmd {
	case cmdSet:
		return p.parseSetWithMeta(tree, meta, entry, tokens, lineNum)
	case cmdNop:
		p.nopLine = true
		err := p.parseSetWithMeta(tree, meta, entry, tokens, lineNum)
		p.nopLine = false
		if err != nil {
			return err
		}
		p.markNopInactive(tree, p.schema.root, tokens)
		return nil
	case cmdDelete:
		if err := p.parseDelete(tree, tokens, lineNum); err != nil {
			return err
		}
		if err := p.recordDeleteMeta(meta, entry, tokens); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
		return nil
	case cmdInactive:
		return p.parseInactive(tree, tokens, lineNum)
	}

	return fmt.Errorf("line %d: unknown command: %s (expected set/nop/delete/inactive)", lineNum, cmd)
}

// parseSetWithMeta handles: set <path...> <value> and records metadata along the path.
// Tolerates structural-only commands (incomplete config) like parseSet.
func (p *SetParser) parseSetWithMeta(tree *Tree, meta *MetaTree, entry MetaEntry, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: set requires at least a path", lineNum)
	}

	return p.setSyntaxRefusal(tokens, p.walkAndSetWithMeta(tree, meta, p.schema.root, entry, tokens, lineNum))
}

// walkAndSetWithMeta walks the path, sets the value, and records metadata at the leaf.
//
//nolint:cyclop // schema node dispatch mirrors walkAndSet
func (p *SetParser) walkAndSetWithMeta(tree *Tree, meta *MetaTree, parent Node, entry MetaEntry, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return nil // structural-only: container/entry exists, no leaf to set
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
		return fmt.Errorf("line %d: unknown field: %s%s", lineNum, name, p.unknownFieldHint(name, parent))
	}

	hasMetadata := entry.User != "" || !entry.Time.IsZero() || entry.Source != ""

	// setLeafMeta records metadata and sets the value for a leaf-like node.
	setLeafMeta := func(value string) {
		tree.Set(name, value)
		if hasMetadata {
			entry.Value = value
			meta.SetEntry(name, entry)
		}
	}

	if leaf, ok := node.(*LeafNode); ok {
		if len(tokens) != 1 {
			return fmt.Errorf("line %d: leaf %s expects exactly one value", lineNum, name)
		}
		// A $9$ key decodes as it does in the hierarchical form (parseLeaf).
		// The session entry holds the decoded key too, as the tree does: the
		// editor replays and compares entries against tree values.
		value, err := decodeSecretLeaf(tree, name, leaf, tokens[0])
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
		// The ^previous key is written in the $9$ form too (encodeSecretMeta),
		// and the editor compares it with the committed key.
		entry.Previous, _, err = DecodeSecretText(name, leaf, entry.Previous)
		if err != nil {
			return fmt.Errorf("line %d: previous value: %w", lineNum, err)
		}
		if err := ValidateLeafValue(leaf, value); err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
		}
		setLeafMeta(NormalizeLeafValue(leaf.Type, value))
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
		setLeafMeta(value)
		return nil
	}

	if bracket, ok := node.(*BracketLeafListNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: no value to set
		}
		// A member deactivation is not a value of the leaf: no metadata.
		if handled, err := p.nopBracketMember(tree, bracket, name, tokens); handled {
			return err
		}
		// Each token is one member, a quoted one included, as walkAndSet
		// reads it. A contested entry's line spells its value in brackets
		// (bracketValue), so it reads back as members too.
		items, err := bracketMembers(bracket, tokens)
		if err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
		}
		// Members plus the scalar mirror, as walkAndSet stores them. A later
		// line for the same leaf replaces an earlier one: a committed file
		// carries a leaf's earlier value as its own line, before the current.
		if err := setBracketLine(tree, name, items); err != nil {
			return fmt.Errorf("line %d: %s: %w", lineNum, name, err)
		}
		if hasMetadata {
			entry.Value = JoinLeafListMembers(items)
			meta.SetEntry(name, entry)
		}
		return nil
	}

	if valueOrArray, ok := node.(*ValueOrArrayNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: no value to set
		}
		items := bracketItems(tokens)
		if err := validateValueOrArrayItems(valueOrArray, name, items, lineNum); err != nil {
			return err
		}
		for _, item := range items {
			tree.AddMultiValueMember(name, item)
		}
		if hasMetadata {
			if entry.Source == "" {
				// Committed annotation (no @source): one entry for the leaf,
				// matching the bracket-form line buildCommitMeta writes. No
				// Value: the tree's member list is the source of truth, and a
				// joined Value would be re-emitted as one quoted token by the
				// contested-leaf serializer when annotations accumulate.
				meta.SetEntry(name, entry)
			} else {
				// Session line: one entry per member so concurrent sessions
				// can add/remove independent members without contesting the
				// whole leaf.
				for _, item := range items {
					memberEntry := entry
					memberEntry.Member = item
					memberEntry.Value = item
					meta.SetEntry(name, memberEntry)
				}
			}
		}
		return nil
	}

	if container, ok := node.(*ContainerNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		childMeta := meta.GetOrCreateContainer(name)
		return p.walkAndSetWithMeta(child, childMeta, container, entry, tokens, lineNum)
	}

	if list, ok := node.(*ListNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: list node declared, no entries
		}
		if err := ValidateListKey(list, tokens[0]); err != nil {
			return fmt.Errorf("line %d: invalid key for %s: %w", lineNum, name, err)
		}
		addr, err := ParseListAddress(name, list, tokens)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
		treeEntry := EnsureListEntry(tree, name, addr)
		if len(tokens) == addr.Tokens {
			// Address only: create the entry (incomplete config tolerance).
			return nil
		}
		listMeta := meta.GetOrCreateContainer(name)
		entryMeta := listMeta.GetOrCreateListEntry(addr.Key)
		return p.walkAndSetWithMeta(treeEntry, entryMeta, list, entry, tokens[addr.Tokens:], lineNum)
	}

	if _, ok := node.(*FreeformNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: no key to set
		}
		if err := setFreeformValue(tree, name, tokens, lineNum); err != nil {
			return err
		}
		if hasMetadata && len(tokens) > 0 {
			childMeta := meta.GetOrCreateContainer(name)
			entry.Value = tokens[len(tokens)-1]
			childMeta.SetEntry(tokens[0], entry)
		}
		return nil
	}

	if flex, ok := node.(*FlexNode); ok {
		// If first token matches a child, recurse into container form.
		if len(tokens) > 0 && flex.Get(tokens[0]) != nil {
			child := tree.GetOrCreateContainer(name)
			childMeta := meta.GetOrCreateContainer(name)
			return p.walkAndSetWithMeta(child, childMeta, flex, entry, tokens, lineNum)
		}
		// A named block entry, as walkAndSet reads it (setFlexValue).
		if isFlexListEntry(flex, tokens) {
			key := tokens[0]
			entryMeta := meta.GetOrCreateContainer(name).GetOrCreateListEntry(key)
			return p.walkAndSetWithMeta(ensureFlexListEntry(tree, name, key), entryMeta, flex, entry, tokens[1:], lineNum)
		}
		// Otherwise treat as value/flag leaf.
		value := configTrue
		if len(tokens) > 0 {
			value = textbuf.Join(tokens, " ")
		}
		setLeafMeta(value)
		return nil
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
		key := tokens[0]
		tokens = tokens[1:]
		entries := tree.GetList(name)
		if entries == nil {
			entries = make(map[string]*Tree)
		}
		treeEntry := entries[key]
		if treeEntry == nil {
			treeEntry = NewTree()
			tree.AddListEntry(name, key, treeEntry)
		}
		listMeta := meta.GetOrCreateContainer(name)
		entryMeta := listMeta.GetOrCreateListEntry(key)
		return p.walkAndSetWithMeta(treeEntry, entryMeta, il, entry, tokens, lineNum)
	}

	return fmt.Errorf("line %d: unknown node type %T for %s", lineNum, node, name)
}

// recordDeleteMeta walks the meta tree along the same path as a delete command
// and records metadata at the leaf. Called after parseDelete has already applied
// the deletion to the tree.
//
// It refuses a ^previous field in the $9$ form that does not decode, and a
// list address the codec refuses.
func (p *SetParser) recordDeleteMeta(meta *MetaTree, entry MetaEntry, tokens []string) error {
	hasMetadata := entry.User != "" || !entry.Time.IsZero() || entry.Source != ""
	if !hasMetadata || len(tokens) == 0 {
		return nil
	}
	return p.walkAndRecordDeleteMeta(meta, p.schema.root, entry, tokens)
}

// walkAndRecordDeleteMeta navigates the meta tree in parallel with the schema
// to find the correct position for recording delete metadata. A field the
// schema does not hold records nothing, because parseDelete refused it; a
// list address the codec refuses is an error.
//
//nolint:cyclop // exhaustive node type dispatch mirrors walkAndSetWithMeta
func (p *SetParser) walkAndRecordDeleteMeta(meta *MetaTree, parent Node, entry MetaEntry, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	// refuse-by-default: guards parseDelete -- it refused an unknown field before this walk
	if node == nil {
		return nil
	}

	// Leaf-list member delete: record which member the delete targets, so a
	// replay removes that member and never the whole leaf.
	if isMemberLeafList(node) && len(tokens) == 1 {
		entry.Member = tokens[0]
		meta.SetEntry(name, entry)
		return nil
	}

	// Leaf-like types: record metadata at this position. The ^previous key
	// of a ze:sensitive leaf is written in the $9$ form (encodeSecretMeta),
	// and the editor compares it with the committed key.
	if isLeafLike(node) {
		leaf, _ := node.(*LeafNode) // A leaf-like node that is no LeafNode holds no secret.
		previous, _, err := DecodeSecretText(name, leaf, entry.Previous)
		if err != nil {
			return fmt.Errorf("previous value: %w", err)
		}
		entry.Previous = previous
		meta.SetEntry(name, entry)
		return nil
	}

	// Container: navigate into child.
	if container, ok := node.(*ContainerNode); ok {
		if len(tokens) == 0 {
			return nil // Container-level delete, no leaf metadata to record.
		}
		childMeta := meta.GetOrCreateContainer(name)
		return p.walkAndRecordDeleteMeta(childMeta, container, entry, tokens)
	}

	// List: navigate to specific entry.
	if list, ok := node.(*ListNode); ok {
		if len(tokens) == 0 {
			return nil // List-level delete, no leaf metadata.
		}
		// parseDelete stops at a container the tree does not hold, before
		// it reaches this address, so the walk is the first to parse it: an
		// address the codec refuses would drop the session's delete record.
		addr, err := ParseListAddress(name, list, tokens)
		if err != nil {
			return fmt.Errorf("delete metadata for %s: %w", name, err)
		}
		if len(tokens) == addr.Tokens {
			return nil // Entry-level delete, no leaf metadata.
		}
		listMeta := meta.GetOrCreateContainer(name)
		entryMeta := listMeta.GetOrCreateListEntry(addr.Key)
		return p.walkAndRecordDeleteMeta(entryMeta, list, entry, tokens[addr.Tokens:])
	}

	// Flex: navigate into child.
	if flex, ok := node.(*FlexNode); ok {
		if len(tokens) == 0 {
			return nil
		}
		childMeta := meta.GetOrCreateContainer(name)
		return p.walkAndRecordDeleteMeta(childMeta, flex, entry, tokens)
	}

	// InlineList: navigate to specific entry.
	if il, ok := node.(*InlineListNode); ok {
		if len(tokens) < 2 {
			return nil
		}
		key := tokens[0]
		tokens = tokens[1:]
		listMeta := meta.GetOrCreateContainer(name)
		entryMeta := listMeta.GetOrCreateListEntry(key)
		return p.walkAndRecordDeleteMeta(entryMeta, il, entry, tokens)
	}
	return nil
}
