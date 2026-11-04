// Design: docs/architecture/testing/test-health.md -- exact Go unit boundaries.
// Related: scope.go -- grouping, structural attribution and semantic comparison.
package testweakened

import (
	"go/parser"
	"go/scanner"
	"go/token"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type goLexeme struct {
	kind token.Token
	at   int
	text string
}

type goComment struct {
	start int
	end   int
}

// goFunctionUnits reads declaration boundaries, not Go syntax validity. This
// also accepts package-less edit fragments. An incomplete unit carries no text;
// an unnamed incomplete entry records lexical uncertainty outside declarations.
// Neither can be mistaken for an exact span ending at EOF or the next func.
func goFunctionUnits(content string) []namedUnit {
	lexemes, comments, failures := scanGoUnits(content)
	var units []namedUnit
	var delimiters []token.Token
	for index := 0; index < len(lexemes); index++ {
		lexeme := lexemes[index]
		declarationStart := index == 0
		if index > 0 {
			previous := lexemes[index-1].kind
			declarationStart = previous == token.SEMICOLON || previous == token.RBRACE
		}
		if lexeme.kind == token.FUNC && len(delimiters) == 0 && declarationStart {
			name, end := goDeclarationEnd(lexemes, index)
			// Only a named declaration can omit its body. A receiver fragment
			// or anonymous signature must remain unknown at a semicolon.
			if name == "" && end >= 0 {
				if lexemes[end].kind == token.SEMICOLON {
					end = -1
				}
			}
			if end < 0 {
				units = append(units, namedUnit{name: name, incomplete: true})
				break
			}
			if name != "" {
				unit := namedUnit{name: name}
				stop := lexemes[end].at + 1
				if lexemes[end].kind == token.SEMICOLON {
					// A semicolon can close an external declaration, or follow a
					// truncated result type. Validate only this signature, never
					// the whole edit fragment, before accepting its exact end.
					last := lexemes[end-1]
					width := len(last.kind.String())
					if last.text != "" {
						width = len(last.text)
					}
					stop = last.at + width
					if _, err := parser.ParseFile(token.NewFileSet(), "", "package p\n"+content[lexeme.at:stop], 0); err != nil {
						units = append(units, namedUnit{name: name, incomplete: true})
						break
					}
				}
				start := goDocStart(content, comments, lexeme.at)
				for _, failure := range failures {
					if failure >= start && failure < stop {
						unit.incomplete = true
					}
				}
				if !unit.incomplete {
					unit.text = content[start:stop]
					unit.start = start
					unit.end = stop
				}
				units = append(units, unit)
			}
			index = end
			continue
		}
		if closing := goClosing(lexeme.kind); closing != token.ILLEGAL {
			delimiters = append(delimiters, closing)
			continue
		}
		if len(delimiters) != 0 {
			if lexeme.kind == delimiters[len(delimiters)-1] {
				delimiters = delimiters[:len(delimiters)-1]
			}
		}
	}
	if len(failures) != 0 || len(delimiters) != 0 {
		units = append(units, namedUnit{incomplete: true})
	}
	return units
}

// goUnresolvedText retains spans that cannot be compared as named units on both
// sides. In particular, a complete baseline stays when its replacement is
// incomplete, so assertion loss remains visible. Scanner offsets prevent an
// identical comment or literal from being removed instead of the declaration.
func goUnresolvedText(content string, other []namedUnit) string {
	var remaining textbuf.Buffer
	start := 0
	for _, unit := range goFunctionUnits(content) {
		if unit.incomplete {
			continue
		}
		if !unitBoundaryKnown(other, unit.name) {
			continue
		}
		remaining.Str(content[start:unit.start]).Byte('\n')
		start = unit.end
	}
	return remaining.Str(content[start:]).String()
}

func scanGoUnits(content string) ([]goLexeme, []goComment, []int) {
	file := token.NewFileSet().AddFile("", -1, len(content))
	var lexer scanner.Scanner
	var failures []int
	lexer.Init(file, []byte(content), func(position token.Position, _ string) {
		failures = append(failures, position.Offset)
	}, scanner.ScanComments)
	var lexemes []goLexeme
	var comments []goComment
	for {
		position, kind, literal := lexer.Scan()
		if kind == token.EOF {
			break
		}
		at := file.Offset(position)
		if kind == token.COMMENT {
			// Scanner removes CR bytes from comment literals; offsets must use
			// the original source so CRLF comments retain their exact extent.
			end := len(content)
			if strings.HasPrefix(content[at:], "//") {
				if newline := strings.IndexByte(content[at:], '\n'); newline >= 0 {
					end = at + newline
				}
			} else if closing := strings.Index(content[at:], "*/"); closing >= 0 {
				end = at + closing + 2
			}
			comments = append(comments, goComment{start: at, end: end})
			continue
		}
		lexemes = append(lexemes, goLexeme{kind: kind, at: at, text: literal})
	}
	return lexemes, comments, failures
}

// goDeclarationEnd returns a named declaration's matching body brace or a
// signature-ending semicolon, or -1 when delimiters are incomplete. A semicolon
// still needs signature validation. Anonymous funcs have no unit name.
func goDeclarationEnd(lexemes []goLexeme, start int) (string, int) {
	index := start + 1
	if index == len(lexemes) {
		return "", -1
	}
	if lexemes[index].kind == token.LPAREN {
		end := goDelimitedEnd(lexemes, index)
		if end < 0 {
			return "", -1
		}
		// A receiver is followed by a method name and parameter list,
		// unlike an anonymous func result.
		index = end + 1
		if index+1 >= len(lexemes) {
			return "", goBodyEnd(lexemes, index)
		}
		if lexemes[index].kind != token.IDENT {
			return "", goBodyEnd(lexemes, index)
		}
		if lexemes[index+1].kind != token.LPAREN {
			return "", goBodyEnd(lexemes, index)
		}
	}
	if index >= len(lexemes) {
		return "", -1
	}
	if lexemes[index].kind != token.IDENT {
		return "", -1
	}
	name := lexemes[index].text
	index++
	if index < len(lexemes) && lexemes[index].kind == token.LBRACK {
		end := goDelimitedEnd(lexemes, index)
		if end < 0 {
			return name, -1
		}
		index = end + 1
	}
	if index >= len(lexemes) {
		return name, -1
	}
	if lexemes[index].kind != token.LPAREN {
		return name, -1
	}
	end := goDelimitedEnd(lexemes, index)
	if end < 0 {
		return name, -1
	}
	return name, goBodyEnd(lexemes, end+1)
}

// goBodyEnd skips result-type delimiters before selecting the body brace.
// struct/interface braces belong to the type; parentheses and brackets can
// contain further type braces. A returned function type follows the same rule.
func goBodyEnd(lexemes []goLexeme, index int) int {
	for index < len(lexemes) {
		kind := lexemes[index].kind
		if kind == token.SEMICOLON {
			return index
		}
		if kind == token.LBRACE {
			return goDelimitedEnd(lexemes, index)
		}
		if kind == token.STRUCT || kind == token.INTERFACE {
			index++
			if index == len(lexemes) {
				return -1
			}
			if lexemes[index].kind != token.LBRACE {
				return -1
			}
			kind = token.LBRACE
		}
		if goClosing(kind) != token.ILLEGAL {
			end := goDelimitedEnd(lexemes, index)
			if end < 0 {
				return -1
			}
			index = end + 1
			continue
		}
		if kind == token.RPAREN || kind == token.RBRACK || kind == token.RBRACE {
			return -1
		}
		index++
	}
	return -1
}

// goDelimitedEnd walks at most the remaining tokens, with a stack bounded by
// their count. A mismatch never becomes a guessed boundary.
func goDelimitedEnd(lexemes []goLexeme, start int) int {
	var delimiters []token.Token
	for index := start; index < len(lexemes); index++ {
		kind := lexemes[index].kind
		if closing := goClosing(kind); closing != token.ILLEGAL {
			delimiters = append(delimiters, closing)
			continue
		}
		if kind != token.RPAREN && kind != token.RBRACK && kind != token.RBRACE {
			continue
		}
		if kind != delimiters[len(delimiters)-1] {
			return -1
		}
		delimiters = delimiters[:len(delimiters)-1]
		if len(delimiters) == 0 {
			return index
		}
	}
	return -1
}

func goClosing(kind token.Token) token.Token {
	// Only opening delimiters have a matching closing token.
	//exhaustive:ignore
	switch kind {
	case token.LPAREN:
		return token.RPAREN
	case token.LBRACK:
		return token.RBRACK
	case token.LBRACE:
		return token.RBRACE
	default:
		return token.ILLEGAL
	}
}

func goDocStart(content string, comments []goComment, start int) int {
	for index := len(comments) - 1; index >= 0; index-- {
		comment := comments[index]
		if comment.end > start {
			continue
		}
		gap := content[comment.end:start]
		if strings.TrimSpace(gap) != "" {
			break
		}
		if strings.Count(gap, "\n") > 1 {
			break
		}
		line := strings.LastIndexByte(content[:comment.start], '\n') + 1
		if strings.TrimSpace(content[line:comment.start]) != "" {
			break
		}
		start = comment.start
	}
	return start
}
