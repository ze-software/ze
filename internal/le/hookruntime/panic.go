// Design: docs/contributing/ze-go-style.md -- every panic states a programmer invariant
// Related: writeedit.go -- registered writeGoPatterns consumer
package hookruntime

import (
	"go/parser"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"
)

// addedForbiddenPanic reports whether proposed holds a panic call naming no
// programmer invariant that current does not already hold. Calls are compared
// by their text, so an edit may keep or move a call already in the file, such
// as a recovered value re-raised after cleanup: judging it would refuse every
// unrelated edit to that file. A call with new text, or another copy of an
// existing one, is refused, and so is proposed content that cannot be judged,
// whoever wrote the malformed part.
func addedForbiddenPanic(current, proposed string) bool {
	added, malformed := forbiddenPanics(proposed)
	if malformed {
		return true
	}
	if len(added) == 0 {
		return false
	}
	// A malformed current file still holds the calls scanned before its
	// error; the edit is judged on proposed's own syntax above.
	existing, _ := forbiddenPanics(current)
	held := make(map[string]int, len(existing))
	for _, call := range existing {
		held[call]++
	}
	for _, call := range added {
		if held[call] == 0 {
			return true
		}
		held[call]--
	}
	return false
}

// forbiddenPanics scans complete content and answers the text of each
// outermost panic call that names no programmer invariant. Scanner tokens keep
// comments and quoted examples from becoming calls; each call earns its own
// exemption. A real panic expression must also parse, even when its prefix is
// permitted, so malformed calls cannot borrow that exemption: malformed
// reports content whose calls cannot be judged.
func forbiddenPanics(content string) (calls []string, malformed bool) {
	if !strings.Contains(content, "panic") {
		return nil, false
	}
	var scan scanner.Scanner
	files := token.NewFileSet()
	file := files.AddFile("edit.go", files.Base(), len(content))
	scan.Init(file, []byte(content), func(token.Position, string) {}, 0)
	depth := 0      // parenthesis depth inside the outermost panic call, 0 outside one
	start := 0      // offset of the outermost panic call
	allowed := true // every panic call within the outermost one names an invariant
	for {
		position, kind, literal := scan.Scan()
		if kind == token.EOF {
			if scan.ErrorCount != 0 {
				return calls, true
			}
			return calls, depth != 0
		}
		if kind == token.IDENT && literal == "panic" {
			_, kind, _ = scan.Scan()
			if kind != token.LPAREN {
				continue
			}
			if depth == 0 {
				start = file.Offset(position)
				allowed = true
			}
			depth++
			// The argument token is judged here and still counts below,
			// because it can itself open or close a parenthesis.
			position, kind, literal = scan.Scan()
			if !namesInvariant(kind, literal) {
				allowed = false
			}
		}
		if kind == token.LPAREN {
			if depth != 0 {
				depth++
			}
			continue
		}
		if kind != token.RPAREN {
			continue
		}
		if depth == 0 {
			continue
		}
		depth--
		if depth != 0 {
			continue
		}
		call := content[start : file.Offset(position)+1]
		if _, err := parser.ParseExpr(call); err != nil {
			return calls, true
		}
		if !allowed {
			calls = append(calls, call)
		}
	}
}

// namesInvariant reports whether a panic's first argument token is a string
// literal opening with a permitted invariant prefix.
func namesInvariant(kind token.Token, literal string) bool {
	if kind != token.STRING {
		return false
	}
	reason, err := strconv.Unquote(literal)
	if err != nil {
		return false
	}
	for _, prefix := range []string{"unreachable", "not implemented", "unimplemented", "TODO", "BUG", "impossible"} {
		if strings.HasPrefix(reason, prefix) {
			return true
		}
	}
	return false
}
