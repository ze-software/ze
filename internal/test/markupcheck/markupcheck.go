// Design: docs/architecture/web-components.md -- markup lives in .templ, never in Go
// Related: internal/test/templcheck -- the sibling guard over component PARAMETERS
// Related: spec-web-templ-migration -- AC-7, the criterion this enforces

// Package markupcheck holds the static checks over a package that renders
// markup. It asks three questions, one per exported assertion. Is markup
// written where it belongs. Does it carry what a strict CSP refuses. Do the
// assets it names resolve.
//
// AssertNoMarkup reports every Go string literal that builds an HTML or SVG
// tag. It reads LITERALS rather than lines, so a tag inside a comment is not a
// finding, and a tag split across a Str(...) chain is one literal per part.
//
// It reads the FORM of a tag rather than its name. `usage: set <leaf> <value>`
// is therefore not a finding.
//
// Markup takes one of four forms. A close tag, a self-closing tag, an open tag
// carrying an attribute, or a void element. A bare `<name>` completes the
// fourth and none of the first three.
//
// The first three need no list of element names. Such a list is a hole as well
// as a chore. `<path>` is an SVG element and also how a usage string names a
// path. The fourth needs one, because nothing else tells `<br>` from `<leaf>`.
// voidElements holds it and states what it leaves out.
//
// AssertNoInlineScriptOrStyle reads the .templ sources instead, and refuses the
// three things `script-src 'self'` rejects at the browser.
//
// AssertAssetsResolve reads the same sources and resolves each asset path
// against the embedded FS that serves it.
package markupcheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Site is one string literal holding a markup tag.
type Site struct {
	// File is the path relative to the scanned root.
	File string
	// Line is the line the literal starts on.
	Line int
	// Tag is the element name that made this a finding.
	Tag string
	// Text is the literal, shortened for a message.
	Text string
}

// maxSiteText is how much of a literal a finding quotes. A whole panel of
// markup on one line tells the reader nothing the first tag did not.
const maxSiteText = 72

// Report returns every markup site under root, and the number of string
// literals it read. The count is what makes a vacuous walk visible. A scan that
// read nothing found nothing for a reason unrelated to markup.
func Report(root string) ([]Site, int, error) {
	var (
		sites    []Site
		literals int
	)

	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !readable(info.Name()) {
			return nil
		}

		src, readErr := os.ReadFile(path) //nolint:gosec // a test walks a package directory
		if readErr != nil {
			return readErr
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}

		found, count := scan(rel, string(src))
		sites = append(sites, found...)
		literals += count

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return sites, literals, nil
}

// readable answers whether a file is one a person writes. A *_templ.go IS the
// generated markup and a test CAN assert on a tag, so neither is scanned.
func readable(name string) bool {
	return strings.HasSuffix(name, ".go") &&
		!strings.HasSuffix(name, "_test.go") &&
		!strings.HasSuffix(name, "_templ.go")
}

// Findings returns one message per file that builds markup without a reason,
// plus one per exemption that exempts nothing.
//
// A stale exemption is a finding for the reason webPortTemplates gives. An
// entry that outlives what it explained is a license nobody reviewed.
func Findings(root string, exempt map[string]string) ([]string, error) {
	sites, _, err := Report(root)
	if err != nil {
		return nil, err
	}

	var (
		findings []string
		used     = make(map[string]bool, len(exempt))
		tb       textbuf.Buffer
	)

	for _, s := range sites {
		if _, ok := exempt[s.File]; ok {
			used[s.File] = true

			continue
		}

		tb.Reset()
		findings = append(findings, tb.Str(s.File).Byte(':').Int(int64(s.Line)).
			Str(" builds a <").Str(s.Tag).Str("> in Go: ").Quoted(s.Text).String())
	}

	names := make([]string, 0, len(exempt))
	for name := range exempt {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		if used[name] {
			continue
		}

		tb.Reset()
		findings = append(findings, tb.Str(name).
			Str(" is exempted and builds no markup, so the entry is stale").String())
	}

	return findings, nil
}

// Floors are the counts a caller states so a pass says something. A scan that
// reads nothing reports nothing. A green over an empty set is the vacuous pass
// every walk of a tree is one typo away from.
type Floors struct {
	// Literals is the least number of Go string literals the walk must read. It
	// is a floor rather than the exact number, so adding a file does not red it.
	Literals int
	// Exempt is the EXACT size of the exemption table. Widening an exemption
	// needs a reader as much as retiring one does, so a ceiling is not enough.
	// Findings cannot see a new entry over a file that does build markup.
	Exempt int
}

// ExemptionDrift returns a message when exempt does not hold exactly want
// entries, and the empty string otherwise.
//
// Findings alone cannot see this. It reports an entry that exempts nothing, so
// a RETIRED builder is caught. It says nothing about a NEW entry over a file
// that does build markup. Widening the table is the one edit that makes the
// gate greener, and it is the edit a reader most needs to see.
func ExemptionDrift(exempt map[string]string, want int) string {
	if len(exempt) == want {
		return ""
	}

	var tb textbuf.Buffer

	return tb.Str("exempts ").Int(int64(len(exempt))).Str(" files from building markup, want exactly ").
		Int(int64(want)).Str("; give the change its reason here").String()
}

// AssertNoMarkup fails on any of four conditions. A Go file under root builds
// markup that exempt does not name. An entry in exempt names a file that builds
// none. The walk read fewer string literals than want.Literals. The exemption
// table is not exactly want.Exempt entries.
func AssertNoMarkup(t *testing.T, root string, exempt map[string]string, want Floors) {
	t.Helper()

	_, literals, err := Report(root)
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}

	if short := Shortfall("string literals", literals, want.Literals); short != "" {
		t.Fatalf("scan %s %s", root, short)
	}

	if drift := ExemptionDrift(exempt, want.Exempt); drift != "" {
		t.Errorf("scan %s %s", root, drift)
	}

	findings, err := Findings(root, exempt)
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}

	for _, f := range findings {
		t.Errorf("%s", f)
	}
}

// scan returns the markup sites in one file, and how many string literals it
// read.
func scan(file, src string) ([]Site, int) {
	var (
		sites    []Site
		literals int
	)

	for _, lit := range literalsOf(src) {
		literals++

		tag, ok := firstTag(lit.text)
		if !ok {
			continue
		}

		sites = append(sites, Site{File: file, Line: lit.line, Tag: tag, Text: shorten(lit.text)})
	}

	return sites, literals
}

// literal is one string constant and the line it starts on.
type literal struct {
	line int
	text string
}

// literalsOf returns every string literal in src, raw and interpreted, with
// comments and rune constants skipped.
//
// It is a scanner rather than a parse. A file that does not compile must still
// be readable, because several sessions edit this tree at once.
func literalsOf(src string) []literal {
	var out []literal

	line, i, n := 1, 0, len(src)

	for i < n {
		switch {
		case src[i] == '\n':
			line++
			i++
		case strings.HasPrefix(src[i:], "//"):
			i = endOfLine(src, i)
		case strings.HasPrefix(src[i:], "/*"):
			end := blockCommentEnd(src, i)
			line += strings.Count(src[i:end], "\n")
			i = end
		case src[i] == '`':
			end := strings.IndexByte(src[i+1:], '`')
			if end < 0 {
				return out
			}

			end += i + 1
			out = append(out, literal{line: line, text: src[i+1 : end]})
			line += strings.Count(src[i:end], "\n")
			i = end + 1
		case src[i] == '"':
			text, end := quotedLiteral(src, i)
			out = append(out, literal{line: line, text: text})
			i = end
		case src[i] == '\'':
			i = runeEnd(src, i)
		default:
			i++
		}
	}

	return out
}

// endOfLine returns the index of the newline ending the line comment at i.
func endOfLine(src string, i int) int {
	if j := strings.IndexByte(src[i:], '\n'); j >= 0 {
		return i + j
	}

	return len(src)
}

// blockCommentEnd returns the index just past the block comment starting at i.
func blockCommentEnd(src string, i int) int {
	if j := strings.Index(src[i+2:], "*/"); j >= 0 {
		return i + 2 + j + 2
	}

	return len(src)
}

// quotedLiteral reads the interpreted string starting at i and returns its
// contents and the index just past it. An escape is kept as written: no escape
// Go allows can spell a tag the raw text does not.
func quotedLiteral(src string, i int) (string, int) {
	var b textbuf.Buffer

	j := i + 1
	for j < len(src) && src[j] != '"' && src[j] != '\n' {
		if src[j] == '\\' && j+1 < len(src) {
			b.Str(src[j : j+2])
			j += 2

			continue
		}

		b.Byte(src[j])
		j++
	}

	return b.String(), j + 1
}

// runeEnd returns the index just past the rune constant starting at i.
func runeEnd(src string, i int) int {
	j := i + 1
	for j < len(src) && src[j] != '\'' {
		if src[j] == '\\' {
			j += 2

			continue
		}

		j++
	}

	return j + 1
}

// firstTag returns the first HTML or SVG element name the text opens or closes.
func firstTag(text string) (string, bool) {
	for i := 0; i+1 < len(text); i++ {
		if text[i] != '<' {
			continue
		}

		name, ok := tagName(text[i+1:])
		if ok {
			return name, true
		}
	}

	return "", false
}

// voidElements are the HTML elements that never close, so a bare `<br>` IS a
// complete element while a bare `<name>` is a placeholder. HTML fixes the set,
// and templ keeps the same one at vendor/github.com/a-h/templ/parser/v2/types.go.
//
// This list changes the BARE case only. Every void element is already a
// finding the moment it carries an attribute. That is the only way most of
// them do any work. `<img>`, `<link>` and `<meta>` say nothing without one.
//
// Two of HTML's sixteen are left out, because ze writes both as CLI text.
// `execListEntryOp` labels the copy verb's argument `<source>`, and the run
// verb answers `usage: run <command>`, both in
// internal/component/web/cli_terminal.go. HTML has also retired `command`.
// Reading a usage string as markup teaches the next reader to ignore this
// check, which is the failure the form rule above exists to avoid.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "keygen": true, "link": true,
	"meta": true, "param": true, "track": true, "wbr": true,
}

// tagName reads the tag just past a '<' and answers whether it is markup.
//
// Four forms are markup and a usage placeholder is none of them. A close tag
// closes an element something opened. A self-closing tag is an element written
// whole. An open tag carrying an attribute is an element being configured. A
// bare void element is an element written whole too, because nothing closes it.
// Any other bare `<word>` is left alone, because that is what
// `usage: delete <name>` looks like.
func tagName(rest string) (string, bool) {
	closing := strings.HasPrefix(rest, "/")
	rest = strings.TrimPrefix(rest, "/")

	end := 0
	for end < len(rest) && isNameByte(rest[end]) {
		end++
	}

	if end == 0 || end >= len(rest) {
		return "", false
	}

	name := strings.ToLower(rest[:end])

	// A name must be followed by whitespace, a close, or a self-close. Anything
	// else is prose that happens to hold an angle bracket.
	if !isTagBoundary(rest[end]) {
		return "", false
	}

	if closing || voidElements[name] {
		return name, true
	}

	return name, hasAttributeOrSelfCloses(rest[end:])
}

// hasAttributeOrSelfCloses answers whether what follows an element name is an
// attribute list or a self-close, rather than an immediate '>'.
func hasAttributeOrSelfCloses(rest string) bool {
	i := 0
	for i < len(rest) && isSpaceByte(rest[i]) {
		i++
	}

	if i >= len(rest) {
		return false
	}

	if rest[i] == '/' {
		return true
	}

	return rest[i] != '>' && i > 0
}

// isNameByte answers whether c can appear in an element name.
func isNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
}

// isSpaceByte answers whether c separates an element name from an attribute.
func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// isTagBoundary answers whether c can follow an element name.
func isTagBoundary(c byte) bool {
	return c == '>' || c == '/' || isSpaceByte(c)
}

// shorten trims a literal to what a message can carry.
func shorten(text string) string {
	if len(text) <= maxSiteText {
		return text
	}

	var tb textbuf.Buffer

	return tb.Str(text[:maxSiteText]).Str("...").String()
}
