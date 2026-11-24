// Design: docs/architecture/config/syntax.md -- set and set-meta comment serialization.
// Overview: serialize_set.go -- set and set-meta serializers that call these helpers.
// Related: setparser_comments.go -- inverse comment parser for SetParser.
// Related: tree_comments.go -- comment storage on Tree.

package config

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Comment slot names used by set-format `comment <slot> [<path...>] <text>` lines.
// Separate block and leaf leading/trailing slots avoid ambiguity on presence
// containers and Flex nodes, which may hold both a value/flag and a child block.
const (
	commentSlotHead           = "head"
	commentSlotOpening        = "opening"
	commentSlotClosing        = "closing"
	commentSlotBlockLeading   = "block-leading"
	commentSlotBlockTrailing  = "block-trailing"
	commentSlotLeading        = "leading"
	commentSlotTrailing       = "trailing"
	commentSlotMemberLeading  = "member-leading"
	commentSlotMemberTrailing = "member-trailing"
)

// writeSetCommentLine writes one set-format comment statement for a root, block
// or leaf target. path is empty for root comments and already formatted with
// quoted list keys otherwise.
func writeSetCommentLine(b *textbuf.Buffer, slot, path, text string) {
	b.Str(cmdComment).Byte(' ').Str(slot)
	if path != "" {
		b.Byte(' ').Str(path)
	}
	b.Byte(' ').Str(QuoteIfNeeded(text)).Byte('\n')
}

// writeSetCommentLines writes one set-format comment line per entry in texts.
func writeSetCommentLines(b *textbuf.Buffer, slot, path string, texts []string) {
	for _, text := range texts {
		writeSetCommentLine(b, slot, path, text)
	}
}

// writeSetMemberCommentLines writes one set-format comment line per entry in
// texts for an inactive leaf-list member statement.
func writeSetMemberCommentLines(b *textbuf.Buffer, slot, path, member string, texts []string) {
	for _, text := range texts {
		b.Str(cmdComment).Byte(' ').Str(slot).Byte(' ').Str(path).Byte(' ').Str(QuoteIfNeeded(member)).Byte(' ').Str(QuoteIfNeeded(text)).Byte('\n')
	}
}

// writeSetRootHeadComments writes the root Tree's head and opening comments.
// A leading `# ze-schema: ...` head entry is omitted so persistence writers
// that prepend FormatSchemaStamp() do not carry or stack a stale release stamp.
func writeSetRootHeadComments(b *textbuf.Buffer, tree *Tree) {
	comments := tree.commentsOfBlock()
	head := comments.Head
	if len(head) > 0 {
		if strings.HasPrefix("#"+head[0], schemaStampPrefix) {
			head = head[1:]
		}
	}
	writeSetCommentLines(b, commentSlotHead, "", head)
	writeSetCommentLines(b, commentSlotOpening, "", comments.Opening)
}

// writeSetRootClosingComments writes the root Tree's end-of-file comments.
func writeSetRootClosingComments(b *textbuf.Buffer, tree *Tree) {
	comments := tree.commentsOfBlock()
	writeSetCommentLines(b, commentSlotClosing, "", comments.Closing)
}

// writeSetBlockBefore writes the leading, opening and head comments of a
// container or list-entry block before its statements.
func writeSetBlockBefore(b *textbuf.Buffer, block *Tree, path string) {
	comments := block.commentsOfBlock()
	writeSetCommentLines(b, commentSlotBlockLeading, path, comments.Statement.Leading)
	writeSetCommentLines(b, commentSlotOpening, path, comments.Opening)
	writeSetCommentLines(b, commentSlotHead, path, comments.Head)
}

// writeSetBlockAfter writes the closing and trailing comments of a container or
// list-entry block after its statements.
func writeSetBlockAfter(b *textbuf.Buffer, block *Tree, path string) {
	comments := block.commentsOfBlock()
	writeSetCommentLines(b, commentSlotClosing, path, comments.Closing)
	writeSetCommentLines(b, commentSlotBlockTrailing, path, comments.Statement.Trailing)
}

// writeSetLeafLeading writes the leading comments attached to leaf name.
// Caller MUST hold tree.mu.RLock.
func writeSetLeafLeading(b *textbuf.Buffer, tree *Tree, name, path string) {
	comments := tree.leafCommentsLocked(name, "")
	writeSetCommentLines(b, commentSlotLeading, path, comments.Leading)
}

// writeSetLeafTrailing writes the trailing comments attached to leaf name.
// Caller MUST hold tree.mu.RLock.
func writeSetLeafTrailing(b *textbuf.Buffer, tree *Tree, name, path string) {
	comments := tree.leafCommentsLocked(name, "")
	writeSetCommentLines(b, commentSlotTrailing, path, comments.Trailing)
}

// writeSetMemberLeading writes the leading comments attached to an inactive
// leaf-list member statement. Caller MUST hold tree.mu.RLock.
func writeSetMemberLeading(b *textbuf.Buffer, tree *Tree, name, member, path string) {
	comments := tree.leafCommentsLocked(name, member)
	writeSetMemberCommentLines(b, commentSlotMemberLeading, path, member, comments.Leading)
}

// writeSetMemberTrailing writes the trailing comments attached to an inactive
// leaf-list member statement. Caller MUST hold tree.mu.RLock.
func writeSetMemberTrailing(b *textbuf.Buffer, tree *Tree, name, member, path string) {
	comments := tree.leafCommentsLocked(name, member)
	writeSetMemberCommentLines(b, commentSlotMemberTrailing, path, member, comments.Trailing)
}
