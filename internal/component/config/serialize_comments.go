// Design: docs/architecture/config/syntax.md — comments attached to statements
// Overview: serialize.go — the hierarchical printer that calls these helpers
// Related: tree_comments.go — where the parser left the comments

package config

import "github.com/ze-software/ze/internal/core/textbuf"

// The printer writes every comment the tree carries beside the statement it
// belongs to, in the places parser_comments.go reads them from, so a second
// parse attaches each comment to the same statement and a second print writes
// the same bytes:
//
//   - leading comments on their own lines above the statement, at its indent;
//   - the trailing comment after the statement on its last line;
//   - a block's opening comment after its `{`, its head first in the body and
//     followed by one blank line, its closing comments last before its `}`.
//
// A statement the parser merged (a leaf set twice, two blocks of one
// container) can carry more trailing comments than its line has room for, and
// a merged block more opening comments. The last one, written beside the
// occurrence whose value the tree kept, stays on the line. Each earlier one
// goes on its own line where the parser reads it back into the same statement:
// a trailing comment above the statement, an opening comment at the top of the
// body.

// writeCommentLines writes each comment on its own line at prefix.
func writeCommentLines(b *textbuf.Buffer, prefix string, texts []string) {
	for _, text := range texts {
		b.Str(prefix).Byte('#').Str(text).Byte('\n')
	}
}

// writeCommentsAbove writes the own-line comments of a statement, before the
// caller writes the statement at prefix: its leading comments, then every
// trailing comment but the last.
func writeCommentsAbove(b *textbuf.Buffer, prefix string, comments statementComments) {
	writeCommentLines(b, prefix, comments.Leading)
	writeCommentLines(b, prefix, allButLast(comments.Trailing))
}

// writeCommentAfter writes the last of texts after the statement on its line,
// before the caller ends the line. The others are written by writeCommentsAbove
// or, for a block's opening comments, by writeBlock.
func writeCommentAfter(b *textbuf.Buffer, texts []string) {
	if len(texts) == 0 {
		return
	}
	b.Str(" #").Str(texts[len(texts)-1])
}

// allButLast answers texts without its last element, nil for one or none.
func allButLast(texts []string) []string {
	if len(texts) < 2 {
		return nil
	}
	return texts[:len(texts)-1]
}

// writeBlock writes a block statement at prefix with every comment it carries:
// open writes the words before ` {`, body writes the statements inside one tab
// deeper.
func writeBlock(b *textbuf.Buffer, prefix string, comments blockComments, open, body func(*textbuf.Buffer)) {
	writeCommentsAbove(b, prefix, comments.Statement)
	b.Str(prefix)
	open(b)
	b.Str(" {")
	writeCommentAfter(b, comments.Opening)
	b.Byte('\n')
	inner := prefix + "\t"
	writeCommentLines(b, inner, allButLast(comments.Opening))
	writeBlockBody(b, inner, comments, body)
	b.Str(prefix).Byte('}')
	writeCommentAfter(b, comments.Statement.Trailing)
	b.Byte('\n')
}

// writeBlockBody writes the inside of a block, the file included: the head
// comments, the statements body writes, then the closing comments.
//
// The blank line after the head is what detaches it from the first statement,
// so it is written only when a statement follows. With none, the parser reads
// the head as closing comments, and a blank line written there would vanish on
// the next format.
func writeBlockBody(b *textbuf.Buffer, inner string, comments blockComments, body func(*textbuf.Buffer)) {
	if len(comments.Head) == 0 {
		body(b)
		writeCommentLines(b, inner, comments.Closing)
		return
	}
	var statements textbuf.Buffer
	body(&statements)
	writeCommentLines(b, inner, comments.Head)
	if statements.Len() > 0 {
		b.Byte('\n')
	}
	b.Str(statements.String())
	writeCommentLines(b, inner, comments.Closing)
}
