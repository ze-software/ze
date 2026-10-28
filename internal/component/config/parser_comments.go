// Design: docs/architecture/config/syntax.md — comments attached to statements
// Overview: parser.go — config parser core
// Related: tree_comments.go — where the comments are kept
// Related: tokenizer.go — comments arrive as trivia on the token after them

package config

import "strings"

// commentScope attaches the comments of one block (the file, or the body of a
// container or list entry) to the statements in it. The loop that parses the
// block calls begin before each statement and at its closing `}` or EOF, and
// end after each statement:
//
//   - a comment after a token on that token's line trails the statement it
//     ends (a `}` included), or the block's opening `{` before any statement;
//   - own-line comments lead the statement after them;
//   - before the block's first statement, a group a blank line detaches from it
//     is the block's head, and so is a first-line schema stamp;
//   - own-line comments before the closing `}` or EOF close the block;
//   - a comment inside a statement (after `[`, between its words) leads it.
//
// Every comment the tokenizer read reaches exactly one of these places, and
// none is dropped: a comment read inside a block no scope parses (a freeform
// or flex body) is consumed with its token and leads the enclosing statement.
type commentScope struct {
	parser  *Parser
	block   *Tree
	prev    commentSlot
	hasPrev bool
	started bool
	leading []string
	mark    int
}

// newCommentScope starts the scope of a block whose statements land in block.
func (p *Parser) newCommentScope(block *Tree) *commentScope {
	return &commentScope{parser: p, block: block}
}

// begin peeks the next token, attaches the comments before it, and answers
// it. On a statement's first token it keeps the leading comments for end.
func (s *commentScope) begin() token {
	tok, comments := s.parser.tok.peekTrivia()
	if len(comments) > 0 && comments[0].sameLine {
		if s.hasPrev {
			s.prev.addTrailing(comments[0].text)
		} else {
			s.block.addBlockOpening(comments[0].text)
		}
		comments = comments[1:]
	}
	if closesBlock(tok.kind) {
		s.block.addBlockClosing(commentTexts(comments))
		return tok
	}
	if !s.started {
		head := headCount(comments, tok)
		s.block.addBlockHead(commentTexts(comments[:head]))
		comments = comments[head:]
	}
	s.started = true
	s.leading = commentTexts(comments)
	s.mark = s.parser.tok.takenMark()
	return tok
}

// end attaches the leading comments kept by begin, then those read inside the
// statement, to the statement just parsed: the container or list entry the
// parser recorded in stmtTree, else leaf name (or its member line) in tree.
func (s *commentScope) end(tree *Tree, name, member string) {
	slot := tree.commentSlotOfLeaf(name, member)
	if s.parser.stmtTree != nil {
		slot = s.parser.stmtTree.commentSlotOfBlock()
		s.parser.stmtTree = nil
	}
	slot.addLeading(s.leading)
	slot.addLeading(commentTexts(s.parser.tok.takeConsumed(s.mark)))
	s.leading = nil
	s.prev, s.hasPrev = slot, true
}

// closesBlock reports whether a token of kind ends the block a scope covers:
// the `}` of a container or list entry, or the end of the file.
func closesBlock(kind tokenType) bool {
	return kind == tokenRBrace || kind == tokenEOF
}

// headCount answers how many of the comments before a block's first statement
// form its head: those up to the last one a blank line separates from what
// follows it. A schema stamp on the first line of the file is head even with
// no blank line after it, so fmt keeps it on line 1 for ScanStampRelease.
func headCount(comments []comment, first token) int {
	head := 0
	if len(comments) > 0 && comments[0].line == 1 && strings.HasPrefix("#"+comments[0].text, schemaStampPrefix) {
		head = 1
	}
	for i := range comments {
		blankAfter := first.blankBefore
		if i+1 < len(comments) {
			blankAfter = comments[i+1].blankBefore
		}
		if blankAfter {
			head = i + 1
		}
	}
	return head
}

// commentTexts answers the text of each comment, nil for none.
func commentTexts(comments []comment) []string {
	if len(comments) == 0 {
		return nil
	}
	texts := make([]string, len(comments))
	for i, c := range comments {
		texts[i] = c.text
	}
	return texts
}
