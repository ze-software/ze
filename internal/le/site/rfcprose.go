// Design: website/AI.md -- the authored ledger prose, rendered rather than rewritten
// Overview: rfcdetail.go -- the section these items are published under
//
// The public ledger's Coverage and Remaining cells are editorial prose an
// author wrote, and they run to fourteen thousand characters. They MUST NOT be
// rewritten, paraphrased, summarized or truncated: they are the disclosure, and
// a clause lost here is disclosure lost (owner ruling, 2026-09-01). What this
// file changes is the RENDERING.
//
// The prose already carries structure the renderer was throwing away. A
// Coverage cell is a semicolon-chained list of claims. A Remaining cell is a
// lead sentence followed by themed groups, each "Theme: body". Both are split
// on those boundaries and published as items.
//
// Every split here is TOTAL and LOSSLESS: rejoining the items reproduces the
// input byte for byte, and anything this file cannot read confidently is
// returned whole rather than dropped. TestEveryLedgerCellSplitsWithoutLoss
// holds that over the real corpus.
package site

import (
	"html"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// rfcProseSplit answers one cell's top-level items, split at the semicolons
// that are not inside a bracket, a quotation or a code span.
//
// A naive split cuts inside "(... ; ...)" and inside a `{ med; }` directive,
// both of which this corpus carries, and the result is a mangled clause. The
// depth counter is what keeps them whole. A cell with no top-level semicolon
// answers itself, as one item.
func rfcProseSplit(prose string) []string {
	if prose == "" {
		return nil
	}
	var out []string
	var item textbuf.Buffer
	depth, quoted, coded := 0, false, false
	for _, letter := range prose {
		switch {
		case letter == '`':
			coded = !coded
		case letter == '"' && !coded:
			quoted = !quoted
		case quoted || coded:
		case letter == '(' || letter == '[' || letter == '{':
			depth++
		case letter == ')' || letter == ']' || letter == '}':
			if depth > 0 {
				depth--
			}
		case letter == ';' && depth == 0:
			out = append(out, item.String())
			continue
		}
		item.WriteRune(letter)
	}
	// The tail is appended even when it is empty. A cell ending in a semicolon
	// leaves nothing after the last cut, and dropping that empty tail loses the
	// semicolon on rejoin -- one character of the disclosure (independent
	// review, 2026-09-01). An empty item is punctuation rather than a claim, so
	// rfcClaimsHTML skips it when it renders the list.
	out = append(out, item.String())
	return out
}

// rfcProseJoin is the inverse of rfcProseSplit, and exists so a test can prove
// the split loses nothing.
func rfcProseJoin(items []string) string { return strings.Join(items, ";") }

// rfcProseBalanced answers whether one item closes every bracket, quotation and
// code span it opens.
//
// Losslessness alone does not prove a split is in the RIGHT places: a naive
// splitter that cuts at every semicolon still rejoins byte for byte. What it
// cannot do is leave every item balanced, because a cut inside "(... ; ...)"
// leaves one item with an unclosed bracket and the next with a stray close.
// That is the property this answers, and a test holds it over the corpus.
func rfcProseBalanced(item string) bool {
	depth, quoted, coded := 0, false, false
	for _, letter := range item {
		switch {
		case letter == '`':
			coded = !coded
		case letter == '"' && !coded:
			quoted = !quoted
		case quoted || coded:
		case letter == '(' || letter == '[' || letter == '{':
			depth++
		case letter == ')' || letter == ']' || letter == '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0 && !quoted && !coded
}

// rfcThemeLabel matches the label a themed group opens with: a capitalized
// phrase followed by a colon and a space.
var rfcThemeLabel = regexp.MustCompile(`^([A-Z][A-Za-z0-9 /,+-]{2,40}): `)

// rfcProseTheme is one themed group of a Remaining cell: the label the author
// wrote and the body under it. Label is empty for the lead prose that opens the
// cell, and for a cell that carries no themes at all.
type rfcProseTheme struct {
	Label string
	Body  string
}

// rfcProseThemes splits a Remaining cell into its lead and its themed groups.
//
// A theme opens at the start of the cell or after a sentence stop, at depth
// zero, so a colon inside a citation or a code span never opens one. A cell
// with no theme answers one part carrying the whole text, which is what "render
// it whole rather than invent headings" means.
func rfcProseThemes(prose string) []rfcProseTheme {
	starts := rfcThemeStarts(prose)
	if len(starts) == 0 {
		return []rfcProseTheme{{Body: prose}}
	}
	out := make([]rfcProseTheme, 0, len(starts)+1)
	if lead := prose[:starts[0]]; strings.TrimSpace(lead) != "" {
		out = append(out, rfcProseTheme{Body: lead})
	}
	for index, start := range starts {
		end := len(prose)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		part := prose[start:end]
		label := rfcThemeLabel.FindStringSubmatch(part)
		out = append(out, rfcProseTheme{Label: label[1],
			Body: strings.TrimPrefix(part, label[0])})
	}
	return out
}

// rfcThemeStarts answers every offset a themed group opens at.
func rfcThemeStarts(prose string) []int {
	var starts []int
	depth, quoted, coded := 0, false, false
	for index, letter := range prose {
		open := depth == 0 && !quoted && !coded &&
			(index == 0 || (index >= 2 && prose[index-2:index] == ". "))
		if open && rfcThemeLabel.MatchString(prose[index:]) {
			starts = append(starts, index)
		}
		switch {
		case letter == '`':
			coded = !coded
		case letter == '"' && !coded:
			quoted = !quoted
		case quoted || coded:
		case letter == '(' || letter == '[' || letter == '{':
			depth++
		case letter == ')' || letter == ']' || letter == '}':
			if depth > 0 {
				depth--
			}
		}
	}
	return starts
}

// rfcProseThemesJoin is the inverse of rfcProseThemes, for the same reason
// rfcProseJoin exists.
func rfcProseThemesJoin(themes []rfcProseTheme) string {
	var out textbuf.Buffer
	for _, theme := range themes {
		if theme.Label != "" {
			out.Str(theme.Label).Str(": ")
		}
		out.Str(theme.Body)
	}
	return out.String()
}

// rfcRequirementID matches a requirement id as the authored prose writes it.
var rfcRequirementID = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-\d+(?:\.\d+)*-\d+\b`)

// rfcRepositoryPath matches a slash-separated path with a file extension.
var rfcRepositoryPath = regexp.MustCompile(`\b[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+\.[a-z]{1,5}\b`)

// rfcRepositoryRoots are the top-level directories a path must open with before
// it is linked.
//
// The prose also cites RELATIVE paths -- `engine/rekey.go` names a file under
// internal/component/ike -- and a link built from one would address a path that
// does not exist. 65 of this corpus's 446 cited paths are that shape, so the
// root is what separates a link from a guess.
// TestEveryLinkedPathExistsInTheTree holds it over the real corpus.
var rfcRepositoryRoots = map[string]bool{
	"ai": true, "cmd": true, "docs": true, "examples": true, "gokrazy": true,
	rfcInternalRoot: true, "pkg": true, "plan": true, "rfc": true, "test": true,
	"website": true,
}

// rfcInternalRoot is where nearly every cited producer lives.
const rfcInternalRoot = "internal"

// rfcLinkablePath answers whether one cited path is addressable from the
// repository root.
func rfcLinkablePath(path string) bool {
	root, _, held := strings.Cut(path, "/")
	return held && rfcRepositoryRoots[root]
}

// rfcProseHTML renders one run of authored prose: every character escaped,
// every code span marked up, every requirement id this RFC declares linked to
// its own row, and every repository path linked to its file.
//
// The text is never altered. A requirement id this RFC does not declare, and a
// path that is not addressable from the repository root, are left as the author
// wrote them: a link nobody can follow is worse than none (owner ruling,
// 2026-09-01).
func rfcProseHTML(prose string, declared map[string]bool) string {
	if !rfcProseCodeSpansClose(prose) {
		return rfcProseLinksHTML(prose, declared)
	}
	var out textbuf.Buffer
	for index, run := range strings.Split(prose, "`") {
		if index%2 == 1 {
			out.Str(rfcProseCodeHTML(run))
			continue
		}
		out.Str(rfcProseLinksHTML(run, declared))
	}
	return out.String()
}

// rfcProseCodeSpansClose answers whether every code span the author opened is
// closed.
//
// The renderer reads the odd runs between backticks as code spans. An odd
// NUMBER of backticks means the last one opens a span nothing closes, and the
// split then reads the tail as code and deletes that backtick from the page:
// one character of the disclosure lost, silently (independent review,
// 2026-09-01). Prose in that shape is published as plain text, every character
// of it, backticks included.
func rfcProseCodeSpansClose(prose string) bool {
	return strings.Count(prose, "`")%2 == 0
}

// rfcProseCodeHTML renders one code span, linked when it names a file.
func rfcProseCodeHTML(run string) string {
	if rfcRepositoryPath.MatchString(run) && rfcLinkablePath(run) &&
		rfcRepositoryPath.FindString(run) == run {
		return `<a href="` + html.EscapeString(repositoryBlobURL(run)) +
			`" target="_blank" rel="noopener"><code>` + html.EscapeString(run) + "</code></a>"
	}
	return "<code>" + html.EscapeString(run) + "</code>"
}

// rfcProseLinksHTML links the ids and the paths of one run of plain prose.
func rfcProseLinksHTML(run string, declared map[string]bool) string {
	var out textbuf.Buffer
	rest := run
	for rest != "" {
		next := rfcNextToken(rest)
		if !next.Found {
			out.Str(html.EscapeString(rest))
			break
		}
		out.Str(html.EscapeString(rest[:next.Start]))
		token := rest[next.Start:next.End]
		switch {
		case next.IsID && declared[token]:
			out.Str(rfcRequirementRefHTML(token, ""))
		case !next.IsID && rfcLinkablePath(token):
			out.Str(`<a href="`).Str(html.EscapeString(repositoryBlobURL(token))).
				Str(`" target="_blank" rel="noopener"><code>`).Str(html.EscapeString(token)).
				Str("</code></a>")
		default:
			out.Str(html.EscapeString(token))
		}
		rest = rest[next.End:]
	}
	return out.String()
}

// rfcProseToken is the next linkable thing in one run of prose: where it sits,
// and whether it is a requirement id or a repository path.
type rfcProseToken struct {
	Start, End int
	IsID       bool
	Found      bool
}

// rfcNextToken answers whichever of the two shapes starts first.
//
// A path that starts where an id does is answered as the id, because an id is
// the more specific shape and the two overlap on nothing else.
func rfcNextToken(run string) rfcProseToken {
	id := rfcRequirementID.FindStringIndex(run)
	path := rfcRepositoryPath.FindStringIndex(run)
	switch {
	case len(id) == 0 && len(path) == 0:
		return rfcProseToken{}
	case len(id) == 0:
		return rfcProseToken{Start: path[0], End: path[1], Found: true}
	case len(path) == 0 || id[0] <= path[0]:
		return rfcProseToken{Start: id[0], End: id[1], IsID: true, Found: true}
	default:
		return rfcProseToken{Start: path[0], End: path[1], Found: true}
	}
}

// rfcProseMirror states the same run in Markdown: the text unchanged, with the
// same ids and paths linked.
func rfcProseMirror(prose string, declared map[string]bool) string {
	if !rfcProseCodeSpansClose(prose) {
		return rfcProseLinksMirror(prose, declared)
	}
	var out textbuf.Buffer
	for index, run := range strings.Split(prose, "`") {
		if index%2 == 1 {
			out.Str(rfcProseCodeMirror(run))
			continue
		}
		out.Str(rfcProseLinksMirror(run, declared))
	}
	return out.String()
}

// rfcProseCodeMirror states one code span, linked when it names a file.
func rfcProseCodeMirror(run string) string {
	if rfcRepositoryPath.MatchString(run) && rfcLinkablePath(run) &&
		rfcRepositoryPath.FindString(run) == run {
		return "[`" + run + "`](" + repositoryBlobURL(run) + ")"
	}
	return "`" + run + "`"
}

// rfcProseLinksMirror links the ids and the paths of one run of plain prose.
func rfcProseLinksMirror(run string, declared map[string]bool) string {
	var out textbuf.Buffer
	rest := run
	for rest != "" {
		next := rfcNextToken(rest)
		if !next.Found {
			out.Str(rest)
			break
		}
		out.Str(rest[:next.Start])
		token := rest[next.Start:next.End]
		switch {
		case next.IsID && declared[token]:
			out.Str(rfcRequirementRefMirror(token, ""))
		case !next.IsID && rfcLinkablePath(token):
			out.Str("[`").Str(token).Str("`](").Str(repositoryBlobURL(token)).Byte(')')
		default:
			out.Str(token)
		}
		rest = rest[next.End:]
	}
	return out.String()
}
