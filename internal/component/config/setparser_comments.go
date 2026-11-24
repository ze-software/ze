// Design: docs/architecture/config/syntax.md -- set and set-meta comment parsing.
// Overview: setparser.go -- set-format parser core.
// Related: serialize_set_comments.go -- inverse comment serializer.
// Related: setparser_meta.go -- metadata-annotated set parser.

package config

// commentLinePathTokens extracts the configuration path tokens from the
// arguments following the `comment` verb (slot, optional path, optional member,
// and trailing text). Returns (nil, false) when tokens do not carry a valid
// comment slot or have no path tokens (for example, a root head/closing comment).
func commentLinePathTokens(tokens []string) ([]string, bool) {
	if len(tokens) < 2 {
		return nil, false
	}
	slot := tokens[0]
	switch slot {
	case commentSlotHead, commentSlotOpening, commentSlotClosing:
		if len(tokens) == 2 {
			return nil, false
		}
		return tokens[1 : len(tokens)-1], true
	case commentSlotBlockLeading, commentSlotBlockTrailing, commentSlotLeading, commentSlotTrailing:
		if len(tokens) < 3 {
			return nil, false
		}
		return tokens[1 : len(tokens)-1], true
	case commentSlotMemberLeading, commentSlotMemberTrailing:
		if len(tokens) < 4 {
			return nil, false
		}
		return tokens[1 : len(tokens)-2], true
	default:
		return nil, false
	}
}

// parseComment parses a set-format `comment <slot> [<path...>] <text>` line
// and attaches the comment text to the addressed Tree slot.
func (p *SetParser) parseComment(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 2 {
		return lineErrorf(lineNum, "comment requires a slot and text")
	}

	slot := tokens[0]
	text := tokens[len(tokens)-1]
	pathTokens := tokens[1 : len(tokens)-1]

	switch slot {
	case commentSlotHead, commentSlotOpening, commentSlotClosing:
		target := tree
		if len(pathTokens) > 0 {
			// lineAt and refusal builders expect every walked slice to be a
			// suffix of p.lineTokens; bind p.lineTokens to the path portion
			// without the leading slot or trailing comment text.
			p.lineTokens = pathTokens
			var skipped bool
			var err error
			target, skipped, err = p.resolveCommentBlock(tree, p.schema.root, pathTokens, lineNum)
			if err != nil {
				return err
			}
			if skipped {
				return nil
			}
		}
		attachBlockComment(target, slot, text)
		return nil

	case commentSlotBlockLeading, commentSlotBlockTrailing:
		if len(pathTokens) == 0 {
			return lineErrorf(lineNum, "comment %s requires a block path", slot)
		}
		p.lineTokens = pathTokens
		target, skipped, err := p.resolveCommentBlock(tree, p.schema.root, pathTokens, lineNum)
		if err != nil {
			return err
		}
		if skipped {
			return nil
		}
		attachBlockComment(target, slot, text)
		return nil

	case commentSlotLeading, commentSlotTrailing:
		if len(pathTokens) == 0 {
			return lineErrorf(lineNum, "comment %s requires a leaf path", slot)
		}
		p.lineTokens = pathTokens
		owner, leafName, skipped, err := p.resolveCommentLeaf(tree, p.schema.root, pathTokens, lineNum)
		if err != nil {
			return err
		}
		if skipped {
			return nil
		}
		attachLeafComment(owner, leafName, "", slot, text)
		return nil

	case commentSlotMemberLeading, commentSlotMemberTrailing:
		if len(pathTokens) < 2 {
			return lineErrorf(lineNum, "comment %s requires a leaf-list path and member", slot)
		}
		member := pathTokens[len(pathTokens)-1]
		leafPath := pathTokens[:len(pathTokens)-1]
		p.lineTokens = leafPath
		owner, leafName, skipped, err := p.resolveCommentMemberLeaf(tree, p.schema.root, leafPath, lineNum)
		if err != nil {
			return err
		}
		if skipped {
			return nil
		}
		attachLeafComment(owner, leafName, member, slot, text)
		return nil

	default:
		return lineErrorf(lineNum, "unknown comment slot %q", slot)
	}
}

// attachBlockComment stores text in the addressed block comment slot.
func attachBlockComment(target *Tree, slot, text string) {
	switch slot {
	case commentSlotHead:
		target.addBlockHead([]string{text})
	case commentSlotOpening:
		target.addBlockOpening(text)
	case commentSlotClosing:
		target.addBlockClosing([]string{text})
	case commentSlotBlockLeading:
		target.commentSlotOfBlock().addLeading([]string{text})
	case commentSlotBlockTrailing:
		target.commentSlotOfBlock().addTrailing(text)
	}
}

// attachLeafComment stores text in the addressed leaf or member comment slot.
func attachLeafComment(owner *Tree, name, member, slot, text string) {
	switch slot {
	case commentSlotLeading, commentSlotMemberLeading:
		owner.commentSlotOfLeaf(name, member).addLeading([]string{text})
	case commentSlotTrailing, commentSlotMemberTrailing:
		owner.commentSlotOfLeaf(name, member).addTrailing(text)
	}
}

// resolveCommentBlock walks tokens through the schema to the addressed block
// Tree (container, list entry, freeform block, flex container/entry, or
// inline-list entry), creating missing structural nodes as set replay does.
// skipped is true only when pre-migration mode recorded and skipped an unknown
// field.
func (p *SetParser) resolveCommentBlock(tree *Tree, parent Node, tokens []string, lineNum int) (*Tree, bool, error) {
	name := tokens[0]
	rest := tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		if p.preMigration {
			p.skipUnknownField(lineNum, name)
			return nil, true, nil
		}
		return nil, false, p.unknownField(lineNum, name, parent, rest)
	}

	switch n := node.(type) {
	case *ContainerNode:
		child := tree.GetOrCreateContainer(name)
		if len(rest) == 0 {
			return child, false, nil
		}
		return p.resolveCommentBlock(child, n, rest, lineNum)

	case *FreeformNode:
		if len(rest) != 0 {
			return nil, false, lineErrorf(lineNum, "unexpected tokens after freeform block %s", name)
		}
		return tree.GetOrCreateContainer(name), false, nil

	case *ListNode:
		entry, addr, err := p.resolveCommentListEntry(tree, n, name, rest, lineNum)
		if err != nil {
			return nil, false, err
		}
		if len(rest) == addr.Tokens {
			return entry, false, nil
		}
		return p.resolveCommentBlock(entry, n, rest[addr.Tokens:], lineNum)

	case *FlexNode:
		if len(rest) == 0 {
			return tree.GetOrCreateContainer(name), false, nil
		}
		if n.Get(rest[0]) != nil {
			child := tree.GetOrCreateContainer(name)
			return p.resolveCommentBlock(child, n, rest, lineNum)
		}
		entry := ensureFlexListEntry(tree, name, rest[0])
		if len(rest) == 1 {
			return entry, false, nil
		}
		return p.resolveCommentBlock(entry, n, rest[1:], lineNum)

	case *InlineListNode:
		if len(rest) == 0 {
			return nil, false, lineErrorf(lineNum, "comment on inline-list %s requires a key", name)
		}
		entry := ensureInlineListEntry(tree, name, rest[0])
		if len(rest) == 1 {
			return entry, false, nil
		}
		return p.resolveCommentBlock(entry, n, rest[1:], lineNum)

	default:
		return nil, false, lineErrorf(lineNum, "block comment target %s is not a block", name)
	}
}

// resolveCommentLeaf walks tokens through the schema to the owner Tree and leaf
// name of a leaf, multi-leaf, leaf-list, presence-flag, flex-value or
// unknown-allowed leaf statement.
func (p *SetParser) resolveCommentLeaf(tree *Tree, parent Node, tokens []string, lineNum int) (*Tree, string, bool, error) {
	name := tokens[0]
	rest := tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		if container, ok := parent.(*ContainerNode); ok && container.AllowUnknown {
			if len(rest) == 0 {
				return tree, name, false, nil
			}
		}
		if p.preMigration {
			p.skipUnknownField(lineNum, name)
			return nil, "", true, nil
		}
		return nil, "", false, p.unknownField(lineNum, name, parent, rest)
	}

	switch n := node.(type) {
	case *LeafNode, *MultiLeafNode, *BracketLeafListNode, *ValueOrArrayNode:
		if len(rest) != 0 {
			return nil, "", false, lineErrorf(lineNum, "unexpected tokens after leaf %s", name)
		}
		return tree, name, false, nil

	case *ContainerNode:
		if len(rest) == 0 {
			if n.Presence {
				return tree, name, false, nil
			}
			return nil, "", false, lineErrorf(lineNum, "leaf comment target %s is a container, not a leaf", name)
		}
		child := tree.GetOrCreateContainer(name)
		return p.resolveCommentLeaf(child, n, rest, lineNum)

	case *ListNode:
		entry, addr, err := p.resolveCommentListEntry(tree, n, name, rest, lineNum)
		if err != nil {
			return nil, "", false, err
		}
		if len(rest) == addr.Tokens {
			return nil, "", false, lineErrorf(lineNum, "leaf comment on list %s requires a child leaf", name)
		}
		return p.resolveCommentLeaf(entry, n, rest[addr.Tokens:], lineNum)

	case *FlexNode:
		if len(rest) == 0 {
			return tree, name, false, nil
		}
		if n.Get(rest[0]) != nil {
			child := tree.GetOrCreateContainer(name)
			return p.resolveCommentLeaf(child, n, rest, lineNum)
		}
		if len(rest) < 2 {
			return nil, "", false, lineErrorf(lineNum, "leaf comment on flex entry %s %s requires a leaf name", name, rest[0])
		}
		entry := ensureFlexListEntry(tree, name, rest[0])
		return p.resolveCommentLeaf(entry, n, rest[1:], lineNum)

	case *InlineListNode:
		if len(rest) < 2 {
			return nil, "", false, lineErrorf(lineNum, "leaf comment on inline-list %s requires a key and leaf name", name)
		}
		entry := ensureInlineListEntry(tree, name, rest[0])
		return p.resolveCommentLeaf(entry, n, rest[1:], lineNum)

	case *FreeformNode:
		if len(rest) != 1 {
			return nil, "", false, lineErrorf(lineNum, "leaf comment on freeform %s requires a single key", name)
		}
		return tree.GetOrCreateContainer(name), rest[0], false, nil

	default:
		return nil, "", false, lineErrorf(lineNum, "leaf comment not supported on %T (%s)", node, name)
	}
}

// resolveCommentMemberLeaf walks tokens to a leaf-list node (ValueOrArrayNode
// or BracketLeafListNode) for a member-leading / member-trailing comment.
func (p *SetParser) resolveCommentMemberLeaf(tree *Tree, parent Node, tokens []string, lineNum int) (*Tree, string, bool, error) {
	name := tokens[0]
	rest := tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		if p.preMigration {
			p.skipUnknownField(lineNum, name)
			return nil, "", true, nil
		}
		return nil, "", false, p.unknownField(lineNum, name, parent, rest)
	}

	switch n := node.(type) {
	case *BracketLeafListNode, *ValueOrArrayNode:
		if len(rest) != 0 {
			return nil, "", false, lineErrorf(lineNum, "unexpected tokens after leaf-list %s", name)
		}
		return tree, name, false, nil

	case *ContainerNode:
		if len(rest) == 0 {
			return nil, "", false, lineErrorf(lineNum, "member comment target %s is a container, not a leaf-list", name)
		}
		child := tree.GetOrCreateContainer(name)
		return p.resolveCommentMemberLeaf(child, n, rest, lineNum)

	case *ListNode:
		entry, addr, err := p.resolveCommentListEntry(tree, n, name, rest, lineNum)
		if err != nil {
			return nil, "", false, err
		}
		if len(rest) == addr.Tokens {
			return nil, "", false, lineErrorf(lineNum, "member comment on list %s requires a leaf-list child", name)
		}
		return p.resolveCommentMemberLeaf(entry, n, rest[addr.Tokens:], lineNum)

	case *FlexNode:
		if len(rest) == 0 {
			return nil, "", false, lineErrorf(lineNum, "member comment target %s is not a leaf-list", name)
		}
		if n.Get(rest[0]) != nil {
			child := tree.GetOrCreateContainer(name)
			return p.resolveCommentMemberLeaf(child, n, rest, lineNum)
		}
		if len(rest) < 2 {
			return nil, "", false, lineErrorf(lineNum, "member comment on flex entry %s %s requires a leaf-list child", name, rest[0])
		}
		entry := ensureFlexListEntry(tree, name, rest[0])
		return p.resolveCommentMemberLeaf(entry, n, rest[1:], lineNum)

	case *InlineListNode:
		if len(rest) < 2 {
			return nil, "", false, lineErrorf(lineNum, "member comment on inline-list %s requires a key and leaf-list child", name)
		}
		entry := ensureInlineListEntry(tree, name, rest[0])
		return p.resolveCommentMemberLeaf(entry, n, rest[1:], lineNum)

	default:
		return nil, "", false, lineErrorf(lineNum, "member comment target %s is not a leaf-list", name)
	}
}

// resolveCommentListEntry parses a list address and returns or creates the
// addressed entry through the shared EnsureListEntry creator.
func (p *SetParser) resolveCommentListEntry(tree *Tree, list *ListNode, name string, tokens []string, lineNum int) (*Tree, ListAddress, error) {
	if len(tokens) == 0 {
		return nil, ListAddress{}, lineErrorf(lineNum, "comment on list %s requires a key", name)
	}
	if err := p.lineKeyRefusal(lineNum, list, name, tokens); err != nil {
		return nil, ListAddress{}, err
	}
	addr, err := ParseListAddress(name, list, tokens)
	if err != nil {
		return nil, ListAddress{}, p.addressLineRefusal(lineNum, tokens, err)
	}
	entry, err := EnsureListEntry(tree, p.lineAt(tokens), list, addr)
	if err != nil {
		return nil, ListAddress{}, p.keyLineRefusal(lineNum, name, tokens, err)
	}
	return entry, addr, nil
}

// ensureInlineListEntry returns the named inline-list entry, creating it if absent.
func ensureInlineListEntry(tree *Tree, name, key string) *Tree {
	entries := tree.GetList(name)
	if entries != nil {
		if entry := entries[key]; entry != nil {
			return entry
		}
	}
	entry := NewTree()
	tree.AddListEntry(name, key, entry)
	return entry
}
