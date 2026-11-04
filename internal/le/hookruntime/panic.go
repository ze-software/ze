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

// forbiddenPanic scans the complete proposed content. Scanner tokens keep
// comments and quoted examples from becoming calls; each call earns its own
// exemption. A real panic expression must also parse, even when its prefix is
// permitted, so malformed calls cannot borrow that exemption.
func forbiddenPanic(content string) bool {
	if !strings.Contains(content, "panic") {
		return false
	}
	var scan scanner.Scanner
	files := token.NewFileSet()
	file := files.AddFile("edit.go", files.Base(), len(content))
	scan.Init(file, []byte(content), func(token.Position, string) {}, 0)
	depth := 0
	start := 0
	for {
		position, kind, literal := scan.Scan()
		if kind == token.EOF {
			if scan.ErrorCount != 0 {
				return true
			}
			return depth != 0
		}
		if kind == token.LPAREN {
			if depth != 0 {
				depth++
			}
			continue
		}
		if kind == token.RPAREN {
			if depth != 0 {
				depth--
				if depth == 0 {
					if _, err := parser.ParseExpr(content[start : file.Offset(position)+1]); err != nil {
						return true
					}
				}
			}
			continue
		}
		if kind != token.IDENT {
			continue
		}
		if literal != "panic" {
			continue
		}
		_, kind, _ = scan.Scan()
		if kind != token.LPAREN {
			continue
		}
		if depth == 0 {
			start = file.Offset(position)
		}
		depth++
		_, kind, literal = scan.Scan()
		if kind != token.STRING {
			return true
		}
		reason, err := strconv.Unquote(literal)
		if err != nil {
			return true
		}
		allowed := false
		for _, prefix := range []string{"unreachable", "not implemented", "unimplemented", "TODO", "BUG", "impossible"} {
			if strings.HasPrefix(reason, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return true
		}
	}
}
