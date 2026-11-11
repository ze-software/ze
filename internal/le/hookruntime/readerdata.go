// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashScratch, the guard whose text scan this blanks
// Related: readerline.go -- judgeReaderWalk, the reader-line walk this asks with writes recorded
package hookruntime

import (
	"mvdan.cc/sh/v3/syntax"
)

// withoutReaderData answers command with the body of every here-document
// that is only data blanked out, for the text scan of bashScratch, which
// would otherwise read a `> tmp/out.log` written in a note as a write. A
// body is data when the line, past the caller's own identity line, is a
// reader line in every respect but its file writes (readerLineButWrites): a
// reader prints what it is handed and runs none of it, and no command of
// such a line can run what a reader printed, because a shell, `eval`,
// `exec`, `source` or a script is a command outside the reader set, and so
// is any program a reader's output could be piped or redirected into.
//
// A body that holds a command substitution, a process substitution or
// arithmetic keeps its text (runsWhenExpanded): bash runs what those hold
// when it expands the body, and their redirections are writes the scan must
// see. A line that does not parse, or that holds any command outside the
// reader set, is answered whole, so `bash <<'EOF'` and `python3 - <<'EOF'`
// keep their bodies scanned. The line's redirection targets are never
// blanked, so `cat > tmp/out.log <<'EOF'` is still refused. Each blanked byte
// becomes a space and each newline stays, so the scan reads no new word.
//
// A panic inside the walk answers the command whole, the text the scan
// judged before this blanking existed: a line the hook failed to read keeps
// every refusal its text gives. readWithoutReaderData is the walk without
// the recover, which the fuzz target calls.
func withoutReaderData(command string, parse commandParse, identity string) (scanned string) {
	defer func() {
		if recover() != nil {
			scanned = command
		}
	}()
	return readWithoutReaderData(command, parse, identity)
}

// readWithoutReaderData is withoutReaderData without its recover.
func readWithoutReaderData(command string, parse commandParse, identity string) string {
	if !readerLineButWrites(command, parse, identity) {
		return command
	}
	text := []byte(command)
	blanked := false
	syntax.Walk(parse.file, func(node syntax.Node) bool {
		redirect, ok := node.(*syntax.Redirect)
		if !ok {
			return true
		}
		if redirect.Op != syntax.Hdoc && redirect.Op != syntax.DashHdoc {
			return true
		}
		if redirect.Hdoc == nil {
			return true
		}
		if runsWhenExpanded(redirect.Hdoc) {
			return true
		}
		start, end := redirect.Hdoc.Pos().Offset(), redirect.Hdoc.End().Offset()
		if start > end {
			return true
		}
		if end > uint(len(text)) {
			return true
		}
		for index := start; index < end; index++ {
			if text[index] != '\n' {
				text[index] = ' '
			}
		}
		blanked = true
		return true
	})
	if !blanked {
		return command
	}
	return string(text)
}

// readerLineButWrites reports whether a line is a reader line once its file
// writes are set aside: the reader-line predicate (judgeReaderWalk) with
// every write recorded rather than refused (readerWalk.recordWrites), so
// every node is of an admitted kind, every here-document has a plain
// delimiter (readerWalk.hereDocument), and every simple command is a reader
// its judge admits, with no assignment in front (readerAdmit). The
// control-byte, trailing-backslash, parse and misread-tree checks of the
// predicate hold as they are, and a panic in the walk answers false.
func readerLineButWrites(command string, parse commandParse, identity string) bool {
	_, outside := judgeReaderWalk(command, parse, identity, readerWalk{source: command, admit: readerAdmit, recordWrites: true})
	return outside == ""
}

// runsWhenExpanded reports whether a here-document body holds a part bash
// runs a command for when it expands the body: a command or process
// substitution, or arithmetic, which can run one through an array
// subscript. A quoted delimiter makes the body one literal, which holds
// none.
func runsWhenExpanded(body *syntax.Word) bool {
	runs := false
	syntax.Walk(body, func(node syntax.Node) bool {
		switch node.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst, *syntax.ArithmExp:
			runs = true
			return false
		}
		return !runs
	})
	return runs
}
