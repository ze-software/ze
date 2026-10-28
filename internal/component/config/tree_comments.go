// Design: docs/architecture/config/syntax.md — comments attached to statements
// Related: tree.go — Tree data structure
// Related: parser_comments.go — the parser attaches comments through these slots

package config

import "slices"

// statementComments holds the comments of one statement: those on their own
// lines above it and those after it on its last line. A statement the parser
// merged (a leaf set twice, two blocks of one container) holds the comments of
// every occurrence, in source order. Each entry is the text after `#`.
type statementComments struct {
	Leading  []string
	Trailing []string
}

// blockComments holds the comments of a block: the file itself or the body of
// a container or list entry. Statement is the block statement's own leading
// comments and the comment after its closing `}`; Opening the comment after
// its `{`; Head the group before its first statement that a blank line
// detaches from it (at the root, a first-line schema stamp is always head);
// Closing the comments after its last statement.
type blockComments struct {
	Statement statementComments
	Opening   []string
	Head      []string
	Closing   []string
}

// leafCommentKey names the leaf statement a comment belongs to: the leaf, or
// with member set, one `inactive: <leaf-list> <member>` line.
type leafCommentKey struct {
	name   string
	member string
}

// commentSlot is where the parser puts the comments of one statement: a leaf
// or member line in tree, or tree itself when the statement is a block.
type commentSlot struct {
	tree  *Tree
	leaf  leafCommentKey
	block bool
}

// commentSlotOfBlock answers the slot of the block statement t is the body of.
func (t *Tree) commentSlotOfBlock() commentSlot {
	return commentSlot{tree: t, block: true}
}

// commentSlotOfLeaf answers the slot of leaf name in t, or of its inactive
// member line when member is not empty.
func (t *Tree) commentSlotOfLeaf(name, member string) commentSlot {
	return commentSlot{tree: t, leaf: leafCommentKey{name: name, member: member}}
}

// addLeading appends own-line comments written above the statement.
func (s commentSlot) addLeading(texts []string) {
	if len(texts) == 0 {
		return
	}
	s.tree.mu.Lock()
	defer s.tree.mu.Unlock()
	target := s.tree.statementCommentsLocked(s)
	target.Leading = append(target.Leading, texts...)
}

// addTrailing appends the comment written after the statement on its line.
func (s commentSlot) addTrailing(text string) {
	s.tree.mu.Lock()
	defer s.tree.mu.Unlock()
	target := s.tree.statementCommentsLocked(s)
	target.Trailing = append(target.Trailing, text)
}

// statementCommentsLocked answers the comments of the statement s names,
// creating the leaf entry on first use. Caller holds t.mu for writing.
func (t *Tree) statementCommentsLocked(s commentSlot) *statementComments {
	if s.block {
		return &t.comments.Statement
	}
	if t.leafComments == nil {
		t.leafComments = make(map[leafCommentKey]*statementComments)
	}
	target := t.leafComments[s.leaf]
	if target == nil {
		target = &statementComments{}
		t.leafComments[s.leaf] = target
	}
	return target
}

// addBlockOpening appends the comment written after the block's `{`.
func (t *Tree) addBlockOpening(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.comments.Opening = append(t.comments.Opening, text)
}

// addBlockHead appends comments detached from the block's first statement.
func (t *Tree) addBlockHead(texts []string) {
	if len(texts) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.comments.Head = append(t.comments.Head, texts...)
}

// addBlockClosing appends comments written after the block's last statement.
func (t *Tree) addBlockClosing(texts []string) {
	if len(texts) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.comments.Closing = append(t.comments.Closing, texts...)
}

// commentsOfBlock answers a copy of the comments of the block t is the body of.
func (t *Tree) commentsOfBlock() blockComments {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.comments.clone()
}

// commentsOfLeaf answers a copy of the comments of leaf name.
func (t *Tree) commentsOfLeaf(name string) statementComments {
	return t.commentsOfLeafKey(leafCommentKey{name: name})
}

// commentsOfMember answers a copy of the comments of the line
// `inactive: <name> <member>`.
func (t *Tree) commentsOfMember(name, member string) statementComments {
	return t.commentsOfLeafKey(leafCommentKey{name: name, member: member})
}

func (t *Tree) commentsOfLeafKey(key leafCommentKey) statementComments {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if found := t.leafComments[key]; found != nil {
		return found.clone()
	}
	return statementComments{}
}

// leafCommentsLocked answers the comments of leaf name, or of its line
// `inactive: <name> <member>` when member is not empty, for a printer to read.
// The answer shares its slices with t. Caller holds t.mu for reading.
func (t *Tree) leafCommentsLocked(name, member string) statementComments {
	if found := t.leafComments[leafCommentKey{name: name, member: member}]; found != nil {
		return *found
	}
	return statementComments{}
}

// hasInnerCommentsLocked reports whether a comment sits inside the block t is
// the body of: after its `{`, at its head, before its `}`, or on a statement in
// it. A one-line spelling of the block (inline, flattened, positional) has no
// line for such a comment, so the printer writes the block instead. Caller
// holds t.mu for reading.
func (t *Tree) hasInnerCommentsLocked() bool {
	if len(t.comments.Opening) > 0 {
		return true
	}
	if len(t.comments.Head) > 0 {
		return true
	}
	if len(t.comments.Closing) > 0 {
		return true
	}
	return len(t.leafComments) > 0
}

// hasStatementComments reports whether the block statement t is the body of
// carries a comment above it or after its closing brace.
func (t *Tree) hasStatementComments() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.comments.Statement.empty()
}

// dropCommentsDeep removes every comment from t and from every block under it.
// prune compares two subtrees by their printed text, and a comment describes a
// statement without changing it. The recursion follows the tree's nesting,
// which the schema bounds. t MUST be unshared, such as a Clone.
func (t *Tree) dropCommentsDeep() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.comments = blockComments{}
	t.leafComments = nil
	for _, child := range t.containers {
		child.dropCommentsDeep()
	}
	for _, entries := range t.lists {
		for _, entry := range entries {
			entry.dropCommentsDeep()
		}
	}
}

// dropLeafCommentsLocked removes the comments of leaf name and of every member
// line of it: a removed statement takes its comments with it, so none
// reattaches to a later leaf of that name. Caller holds t.mu for writing.
func (t *Tree) dropLeafCommentsLocked(name string) {
	for key := range t.leafComments {
		if key.name == name {
			delete(t.leafComments, key)
		}
	}
}

// cloneCommentsLocked copies every comment of t onto target, which shares no
// slice with t afterwards. Caller holds t.mu for reading; target is unshared.
func (t *Tree) cloneCommentsLocked(target *Tree) {
	target.comments = t.comments.clone()
	if len(t.leafComments) == 0 {
		return
	}
	target.leafComments = make(map[leafCommentKey]*statementComments, len(t.leafComments))
	for key, found := range t.leafComments {
		copied := found.clone()
		target.leafComments[key] = &copied
	}
}

// mergeCommentsLocked appends the comments of other after those of t, the
// order the two occurrences had in the source. Caller holds t.mu for writing;
// other is caller-owned and unshared.
func (t *Tree) mergeCommentsLocked(other *Tree) {
	t.comments.appendFrom(other.comments)
	for key, found := range other.leafComments {
		if t.leafComments == nil {
			t.leafComments = make(map[leafCommentKey]*statementComments)
		}
		target := t.leafComments[key]
		if target == nil {
			target = &statementComments{}
			t.leafComments[key] = target
		}
		target.Leading = append(target.Leading, found.Leading...)
		target.Trailing = append(target.Trailing, found.Trailing...)
	}
}

// keepBlockCommentsOf puts the block comments of replaced, an entry a merge
// is about to replace, ahead of t's own: the entry statement stays, so its
// comments stay too. Leaf comments go with the leaves the merge replaces.
func (t *Tree) keepBlockCommentsOf(replaced *Tree) {
	replaced.mu.RLock()
	kept := replaced.comments.clone()
	replaced.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()
	kept.appendFrom(t.comments)
	t.comments = kept
}

func (c statementComments) empty() bool {
	return len(c.Leading) == 0 && len(c.Trailing) == 0
}

func (c statementComments) clone() statementComments {
	return statementComments{Leading: slices.Clone(c.Leading), Trailing: slices.Clone(c.Trailing)}
}

func (c blockComments) clone() blockComments {
	return blockComments{
		Statement: c.Statement.clone(),
		Opening:   slices.Clone(c.Opening),
		Head:      slices.Clone(c.Head),
		Closing:   slices.Clone(c.Closing),
	}
}

// appendFrom appends every part of other after the same part of c.
func (c *blockComments) appendFrom(other blockComments) {
	c.Statement.Leading = append(c.Statement.Leading, other.Statement.Leading...)
	c.Statement.Trailing = append(c.Statement.Trailing, other.Statement.Trailing...)
	c.Opening = append(c.Opening, other.Opening...)
	c.Head = append(c.Head, other.Head...)
	c.Closing = append(c.Closing, other.Closing...)
}
