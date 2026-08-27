// Design: docs/architecture/core-design.md -- the tagged unit, defined once
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// goscope.go is scripts/dev/rfc_tagged_scope.py, ported: the single definition
// of "the tagged unit", the text one `RFC requirement:` tag governs.
//
// The Python module STAYS where it is, and this is not a second copy of a rule.
// `.claude/hooks/pretool-writeedit.py` imports it to widen an edit hunk to its
// enclosing unit, a hook cannot call Go, and the gate must not import from
// `.claude/`. What moves here is the half the GATE needs -- which top-level
// function encloses a tag, and what that function's text is -- and
// TestTheTwoTaggedScopeReadersAgree compares the two over every tagged file in
// the checkout, so a drift reddens rather than re-sealing a verdict against a
// hash the other half does not compute.
//
// Offsets here are BYTE offsets where the Python's are CHARACTER offsets.
// Nothing compares one against the other: every offset is produced and consumed
// inside this file, and the substrings the two languages cut are the same bytes.
package rfc

import (
	"regexp"
	"strings"
)

// How a fingerprint key resolved. `func` is one top-level Go function span;
// `file` is the whole file. Recorded so a reader can tell a narrow answer from
// the fallback, and so a file-scoped verdict is never read as an unresolved one.
const (
	scopeFunc = "func"
	scopeFile = "file"
)

// The three shapes the span finder reads. Go is the only carrier with a
// machine-readable unit boundary cheap enough to trust, so every other shape is
// file-scoped BY DECLARATION: file scope is strictly MORE sensitive than
// function scope, so declaring it can only over-trigger a re-read.
var (
	goFuncStartRE = regexp.MustCompile(`(?m)^func\b`)
	goFuncEndRE   = regexp.MustCompile(`(?m)^\}`)
	goFuncDeclRE  = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)`)
)

// goScoped reports whether this path's unit is a Go function span.
func goScoped(path string) bool { return strings.HasSuffix(path, ".go") }

// span is one top-level func as a half-open [doc comment, past closing brace)
// range of the file it was found in.
type span struct{ begin, end int }

// lineOffset answers the offset of the start of a 1-based line.
//
// Clamps to the end of the content rather than refusing: a tag line past EOF
// means the file shrank under the run, and the caller's fallback is file scope,
// which is the safe answer rather than a crash.
func lineOffset(content string, line int) int {
	if line <= 1 {
		return 0
	}
	off := 0
	for i := 1; i < line; i++ {
		next := strings.IndexByte(content[off:], '\n')
		if next < 0 {
			return len(content)
		}
		off += next + 1
	}
	return off
}

// docCommentStart walks back from a `func` offset over its contiguous `//` doc
// comment, so a tag on the line above the declaration is inside the span.
func docCommentStart(content string, at int) int {
	lineStart := at
	for lineStart > 0 {
		prev := strings.LastIndexByte(content[:lineStart-1], '\n') + 1
		if !strings.HasPrefix(pyLStrip(content[prev:lineStart]), "//") {
			break
		}
		lineStart = prev
	}
	return lineStart
}

// goFuncSpans answers each top-level func as a [doc comment .. closing brace)
// span, in file order.
//
// Two boundaries decide what a tag governs, and both were wrong in the Python
// in turn. The END is the func's own closing brace, CAPPED at the next func's
// doc comment: running to the next `func` keyword swallowed the following
// function's doc comment, where tags live, and running to the next doc comment
// alone left the spans contiguous, which re-homed a tag sitting in the gap
// between one brace and the next comment onto the preceding function. The spans
// are therefore NOT a partition of the file, and a tag outside all of them is
// answered with the whole file: more checking, never less.
//
// Column 0 for the closing brace is gofmt's guarantee for a top-level func. A
// one-line func has none, so the cap keeps its span at the safe boundary.
func goFuncSpans(content string) []span {
	starts := goFuncStartRE.FindAllStringIndex(content, -1)
	ends := goFuncEndRE.FindAllStringIndex(content, -1)
	out := make([]span, 0, len(starts))
	for i, match := range starts {
		at := match[0]
		limit := len(content)
		if i+1 < len(starts) {
			limit = docCommentStart(content, starts[i+1][0])
		}
		end := limit
		for _, closing := range ends {
			if closing[0] > at {
				// +2 to run past the brace and its newline.
				if closing[0]+2 < limit {
					end = closing[0] + 2
				}
				break
			}
		}
		if end < at+1 {
			end = at + 1
		}
		out = append(out, span{begin: docCommentStart(content, at), end: end})
	}
	return out
}

// funcNameIn answers the name a span DECLARES, or "" when no declaration is
// visible. It reads the SPAN, not the file: a span starts at the doc comment,
// so the declaration line is already inside the text the caller holds. Both Go
// shapes are read, and a generic `func Name[T any](` too, because the name is
// taken before the parameter list rather than after it.
func funcNameIn(text string) string {
	match := goFuncDeclRE.FindStringSubmatch(text)
	if match == nil {
		return ""
	}
	return match[1]
}

// scopeIndex memoises the span walk for one command run.
//
// The same content is queried once per tag, and the checkout carries 3602 of
// them. The Python holds a 128-entry LRU for the same reason; this is keyed by
// the exact content string, so an edited file can never inherit stale spans,
// and it is discarded when the run ends.
type scopeIndex struct{ spans map[string][]span }

func newScopeIndex() *scopeIndex { return &scopeIndex{spans: map[string][]span{}} }

func (s *scopeIndex) of(content string) []span {
	found, held := s.spans[content]
	if !held {
		found = goFuncSpans(content)
		s.spans[content] = found
	}
	return found
}

// funcNameAt answers the name of the top-level func enclosing a 1-based line,
// or "" for file scope.
//
// "" comes back exactly where the unit is the whole file, so a caller minting a
// key writes the bare path there and states "the file is the unit" instead of
// guessing a narrower answer. A line inside no span, or inside more than one,
// is file scope: a wrong guess is a false FRESH, the one catastrophic outcome.
func (s *scopeIndex) funcNameAt(path, content string, line int) string {
	if !goScoped(path) {
		return ""
	}
	at := lineOffset(content, line)
	var hit span
	found := 0
	for _, one := range s.of(content) {
		if one.begin <= at && at < one.end {
			hit, found = one, found+1
		}
	}
	if found != 1 {
		return ""
	}
	return funcNameIn(content[hit.begin:hit.end])
}

// funcTexts answers the text of every top-level func in content declared name.
//
// The caller refuses anything but exactly one. A name no span declares and a
// name two spans declare (two methods with different receivers can share one
// name in one file) are both refusals: picking either of two same-named
// functions would fingerprint text nobody chose.
func (s *scopeIndex) funcTexts(content, name string) []string {
	var found []string
	for _, one := range s.of(content) {
		text := content[one.begin:one.end]
		if funcNameIn(text) == name {
			found = append(found, text)
		}
	}
	return found
}
