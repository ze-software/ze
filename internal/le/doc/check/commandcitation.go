// Design: docs/architecture/core-design.md -- native documentation verifier actions
// Related: retired.go -- the sweep whose per-line walk hands each line here
// Related: ../../le/root/citation.go -- JudgeCitation, the resolver dispatch runs
//
// A tracked line that cites an le command is judged against what le holds
// (spec-le-cli-usability, AC-25): the area, or the namespace, and the first
// verb after it. The sweep finds the citation in the three forms
// commandPattern compiles for a retired form, and asks dispatch's own
// resolver whether le holds it, so a page cannot keep a command le never had
// or lost without a rename row.

package doccheck

import (
	"errors"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// citationStart finds where a cited le line begins. A line cites le when it
// writes it as a command: opening a code span or a quoted string (`le doc`,
// "le doc"), after a shell prompt (`$ le doc`), or run by its path (`./le`,
// `bin/le`, `$CLAUDE_PROJECT_DIR/le`, `bin/le-<name>/le`), each optionally as
// the `ze le` personality; a Go argv whose program is the literal `"le"` or a
// variable named `le`; and a helper named for le whose first literal argument
// is the command. A sentence that names le as a word ("le refuses the line")
// cites nothing. Prose ends in the blank after le; the two argv forms end in
// the opening quote of the first literal.
var citationStart = regexp.MustCompile(edgeBeforeLe + "(?:[`\"']|\\$[ \t]+)" + citationProgram +
	`|(?:\.|bin|\$\{?[A-Za-z_]+\}?|le-[A-Za-z0-9_-]+)/` + citationProgram +
	`|` + edgeBefore + `(?:"le"|le),[ \t]*"` +
	`|` + edgeBefore + leCallee + `(?:[^()"]*,[ \t]*)?"`)

// citationProgram is le as a cited command's program, alone or as the `ze le`
// personality, followed by the blank before its first word.
const citationProgram = `(?:ze[ \t]+)?le[ \t]+`

// citationWord is the shape of a command word: a keyword, a verb, a value
// written as one word, a launcher word with its value. A placeholder
// (`<area>`), a pipe or a sentence word cut by punctuation is not one, and it
// ends the cited line.
var citationWord = regexp.MustCompile(`^-{0,2}[A-Za-z0-9][A-Za-z0-9_.=/-]*`)

// citationWordsMax bounds the words read from one citation. The judge reads
// the area, a namespace word and the first verb, after the launcher words and
// a help word, so a longer line carries nothing it reads.
const citationWordsMax = 8

// citationException is one line that hands le a word it does not hold on
// purpose: a test proving le refuses an unknown command.
type citationException struct {
	file string
	// lines are the lines the exception covers, each written as its text with
	// the indentation trimmed. A line is covered when it holds one of them, so
	// a stale citation elsewhere in the same file is still found.
	lines []string
	why   string
}

// citationFixtureWhy is the reason of the sweep's own test, whose fixture
// cites words le does not hold so the sweep is seen to refuse them.
const citationFixtureWhy = "the citation sweep's test cites words le does not hold to prove each one is refused"

// citationFixtureLines are the lines of the sweep's test that cite a word le
// does not hold. The exception table writes the same fragments out, so its
// lines are covered by them too.
var citationFixtureLines = []string{
	"`le doc check no-such-verb`",
	"`le no-such-area`",
	`"le", "doc", "check", "absent"`,
	`Cited: "le `,
	`"unheld docs/verb.md:2: le doc check no-such-verb"`,
}

// citationExceptions are the declared exceptions of AC-25: each names the
// file, the lines that cite a word le does not hold, and why that is the
// line's purpose rather than a stale citation.
var citationExceptions = [...]citationException{
	{file: "internal/le/doc/check/commandcitation_test.go", lines: citationFixtureLines, why: citationFixtureWhy},
	{
		file:  "internal/le/doc/check/commandcitation.go",
		lines: citationFixtureLines,
		why:   "the exception table writes out the fragments of the sweep's test that it covers",
	},
	{
		file: "docs/architecture/core-design.md",
		lines: []string{
			"So `le interface` is an unknown command",
			"`le go lint`, not `le go-lint`",
			"`le cli stdio`, not `le cli-stdio`",
			"as in `le spec nosuch`, is a mistake",
		},
		why: citationRefusedWhy,
	},
	{file: "docs/architecture/cli/command-namespacing.md", lines: []string{"`le go lint`, not `le go-lint`"}, why: citationRefusedWhy},
	{file: "docs/architecture/testing/qemu-integration.md", lines: []string{"`le test plugin <name>` names no suite"}, why: citationRefusedWhy},
	{file: "docs/guide/developer-setup.md", lines: []string{"`./le --update` is gone and says so"}, why: citationRefusedWhy},
}

// citationRefusedWhy is the reason of a page that cites a line le refuses to
// show the reader the refusal, or the spelling it is not.
const citationRefusedWhy = "the page cites a line le refuses, as the example of what le does not hold"

// UnheldCitation is one tracked line that cites an le command le does not
// hold, with the words cited and the resolver's answer.
type UnheldCitation struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Cited  string `json:"cited"`
	Reason string `json:"reason"`
}

// commandCitationCandidate answers whether a line can hold a citation, without
// a regular expression, by the bytes each form of citationStart must carry
// around its `le`. The tree is too large to run citationStart over every
// line, so a line that can match none of its forms is never handed to it:
//
//   - a command or an argv: `le` followed by a blank, a comma or a quote, after
//     a quote, a backtick, a blank or a slash, or at the line start;
//   - a helper named for le: `le` opening a word, or `Le` or `LE` after a
//     lower-case letter, then `(` or the next capitalized part of the name.
func commandCitationCandidate(line string) bool {
	for index := 0; index+2 < len(line); index++ {
		if line[index+1] != 'e' && line[index+1] != 'E' {
			continue
		}
		char := line[index]
		if char != 'l' && char != 'L' {
			continue
		}
		next := line[index+2]
		if char == 'l' && line[index+1] == 'e' && citesProgram(line, index, next) {
			return true
		}
		if namesHelper(line, index, next) {
			return true
		}
	}
	return false
}

// citesProgram reports whether the lower-case `le` at index can be a cited
// command's program or an argv's: a blank, a comma or a quote after it, and
// a quote, a backtick, a blank or a slash before it, or nothing.
func citesProgram(line string, index int, next byte) bool {
	if strings.IndexByte(" \t,\"", next) < 0 {
		return false
	}
	if index == 0 {
		return true
	}
	return strings.IndexByte("`\"' \t/", line[index-1]) >= 0
}

// opensHelperPart reports whether next, the byte after `le`, `Le` or `LE`,
// continues a helper named for le: its call, or its next capitalized part.
func opensHelperPart(next byte) bool {
	switch {
	case next == '(', next == '_':
		return true
	case next >= 'A' && next <= 'Z':
		return true
	}
	return next >= '0' && next <= '9'
}

// namesHelper reports whether the `le` at index can open or continue a helper
// named for le (leCallee): `le` opening a word, or `Le` or `LE` after a
// lower-case letter, followed by `(` or by an upper-case letter, a digit or
// an underscore.
func namesHelper(line string, index int, next byte) bool {
	if !opensHelperPart(next) {
		return false
	}
	if line[index] == 'l' {
		return index == 0 || !isWordByte(line[index-1])
	}
	return index != 0 && line[index-1] >= 'a' && line[index-1] <= 'z'
}

// isWordByte reports whether char continues a Go identifier.
func isWordByte(char byte) bool {
	switch {
	case char >= 'a' && char <= 'z':
		return true
	case char >= 'A' && char <= 'Z':
		return true
	case char >= '0' && char <= '9':
		return true
	}
	return char == '_'
}

// citationsOf answers the le lines a tracked line of the file rel cites, each
// as its words after the program. A citation whose first word is no command
// word (`le <area>`, a bare `le`) answers no words and is not judged.
//
// A citation opened by a backtick or a quote is judged only as a whole code
// span or string: it needs its closing mark, and everything before it must be
// command words, placeholders or a pipe (spanWords). A sentence that opens
// with le, such as an error string "le cannot find Colima: ...", is then not
// read as a command. An undelimited citation (`$ le`, a bare `./le` in a
// shell script or a code block) is read up to its first word that is not a
// command word when it opens the line, as a code block or a script writes a
// command; anywhere else, and anywhere in Go, an undelimited one is a sentence
// of a comment ("run ./le again"). A third-party file is not Ze's to cite le.
func citationsOf(rel, line string) [][]string {
	if citationThirdParty(rel) {
		return nil
	}
	if !commandCitationCandidate(line) {
		return nil
	}
	found := citationStart.FindAllStringIndex(line, -1)
	if len(found) == 0 {
		return nil
	}
	cited := make([][]string, 0, len(found))
	for _, span := range found {
		var words []string
		opener := strings.IndexAny(line[span[0]:span[1]], "`\"'")
		switch {
		case line[span[1]-1] == '"':
			words = argvWords(line[span[1]-1:])
		case opener >= 0:
			words = spanWords(line[span[1]:], line[span[0]+opener])
		case strings.HasSuffix(rel, ".go"):
			continue
		case !opensLine(line[:span[0]]):
			continue
		default:
			words = proseWords(line[span[1]:])
		}
		if len(words) != 0 {
			cited = append(cited, words)
		}
	}
	return cited
}

// citationThirdPartyPaths are the third-party sources Ze carries unmodified:
// a word there spelled `le` is that project's, never a citation. A path that
// ends in a slash takes every file under it.
var citationThirdPartyPaths = [...]string{
	"third_party/",
	"internal/component/api/rest/assets/swagger-ui-bundle.js",
}

// citationThirdParty reports whether rel is one of citationThirdPartyPaths.
func citationThirdParty(rel string) bool {
	for _, path := range &citationThirdPartyPaths {
		if rel == path {
			return true
		}
		if strings.HasSuffix(path, "/") && strings.HasPrefix(rel, path) {
			return true
		}
	}
	return false
}

// opensLine reports whether the text before an undelimited citation leaves it
// opening the line: blanks, and the start of a shell prompt, which
// citationStart matches itself.
func opensLine(before string) bool {
	return strings.TrimLeft(before, " \t") == ""
}

// spanWords answers the command words of a code span or a string, text being
// the rest of the line after its `le ` and closer the mark that opened it.
// Without the closing mark, or with anything but command words, placeholders
// (`<file>`, `[...]`), an ellipsis or a pipe before it, the span is prose and
// answers no words.
func spanWords(text string, closer byte) []string {
	span, _, closed := strings.Cut(text, string(closer))
	if !closed {
		return nil
	}
	words := make([]string, 0, 4)
	placeholder := false
	for token := range strings.FieldsSeq(span) {
		if citationWord.FindString(token) == token && !placeholder {
			if len(words) < citationWordsMax {
				words = append(words, token)
			}
			continue
		}
		if !citationFiller(token) {
			return nil
		}
		placeholder = true
	}
	return words
}

// citationFiller reports whether a token of a code span stands for words
// rather than being one: a placeholder, an optional part, an ellipsis or a
// pipe. A span may carry them and still be a command; it may not carry a
// sentence word cut by punctuation.
func citationFiller(token string) bool {
	switch {
	case strings.HasPrefix(token, "<"):
		return true
	case strings.HasPrefix(token, "["):
		return true
	case strings.HasPrefix(token, "{"):
		return true
	case strings.HasPrefix(token, "..."):
		return true
	case token == "|", token == "'|'":
		return true
	}
	return citationWord.FindString(token) == token
}

// proseWords answers the command words that open text, the rest of a line
// after `le `. A word cut by anything a command word cannot carry (a backtick,
// a quote, a parenthesis) is the last one, and so is a word a sentence ends
// with: its trailing `.`, `,`, `:` or `;` is punctuation, not the word.
func proseWords(text string) []string {
	words := make([]string, 0, 4)
	for len(words) < citationWordsMax {
		text = strings.TrimLeft(text, " \t")
		token := text
		if end := strings.IndexAny(text, " \t"); end >= 0 {
			token = text[:end]
		}
		word := citationWord.FindString(token)
		if word == "" {
			return words
		}
		trimmed := strings.TrimRight(word, ".,:;")
		if trimmed == "" {
			return words
		}
		words = append(words, trimmed)
		if len(trimmed) != len(token) {
			return words
		}
		text = text[len(token):]
	}
	return words
}

// argvWords answers the command words of a Go argv, text starting at the
// opening quote of its first literal: each literal that is a whole command
// word, up to the first that is not a literal or not a command word.
func argvWords(text string) []string {
	words := make([]string, 0, 4)
	for len(words) < citationWordsMax {
		if !strings.HasPrefix(text, `"`) {
			return words
		}
		end := strings.IndexByte(text[1:], '"')
		if end < 0 {
			return words
		}
		literal := text[1 : end+1]
		if literal == "" || citationWord.FindString(literal) != literal {
			return words
		}
		words = append(words, literal)
		text = strings.TrimLeft(strings.TrimPrefix(text[end+2:], ","), " \t")
	}
	return words
}

// citationExempt reports whether a line of a file is declared to cite a word
// le does not hold on purpose: an exception of that file lists the line.
func citationExempt(rel, line string) bool {
	for _, exception := range &citationExceptions {
		if exception.file != rel {
			continue
		}
		for _, covered := range exception.lines {
			if strings.Contains(line, covered) {
				return true
			}
		}
	}
	return false
}

// unheldCitations answers the citations of one line that le does not
// hold. A citation of a retired spelling is the retired-name sweep's own
// finding, which names the successor, so it is not reported a second time.
func unheldCitations(rel string, lineNo int, line string) []UnheldCitation {
	cited := citationsOf(rel, line)
	if len(cited) == 0 {
		return nil
	}
	if citationExempt(rel, line) {
		return nil
	}
	var findings []UnheldCitation
	for _, words := range cited {
		err := leroot.JudgeCitation(words)
		if err == nil {
			continue
		}
		if errors.Is(err, leroot.ErrCitationRetired) {
			continue
		}
		var tb textbuf.Buffer
		findings = append(findings, UnheldCitation{
			File: rel, Line: lineNo,
			Cited:  tb.Str("le ").Join(words, " ").String(),
			Reason: err.Error(),
		})
	}
	return findings
}
