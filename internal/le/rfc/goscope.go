// Design: docs/architecture/core-design.md -- the tagged unit, defined once
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// goscope.go is the single definition of "the tagged unit", the text one
// `RFC requirement:` tag governs.
//
// Native hooks, commit preparation, spec lifecycle checks, and the RFC gate
// consume the exported functions below. Offsets are byte offsets and remain
// inside this package from production through consumption.
package rfc

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ScopeKind states how a tagged unit or path is resolved.
type ScopeKind string

// ScopeGo means a path admits function-span resolution; ScopeFunc and ScopeFile
// describe the unit a particular lookup selected.
const (
	ScopeGo   ScopeKind = "go"
	ScopeFunc ScopeKind = "func"
	ScopeFile ScopeKind = "file"

	scopeFunc = string(ScopeFunc)
	scopeFile = string(ScopeFile)
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

var tagCarrierSuffixes = [...]string{"_test.go", ".ci", ".et"}

// ScopeReader answers how a path's tagged unit can be resolved.
//
// Go files admit top-level function spans. Every other shape is file scoped by
// declaration, which can over-trigger a re-read but cannot leave changed
// evidence looking fresh.
func ScopeReader(path string) ScopeKind {
	if goScoped(path) {
		return ScopeGo
	}
	return ScopeFile
}

// IsTagCarrier reports whether the RFC scanner treats path as test evidence.
func IsTagCarrier(path string) bool {
	if strings.HasPrefix(filepath.ToSlash(path), "internal/le/interoplab/") &&
		strings.HasSuffix(path, ".go") {
		return true
	}
	for _, suffix := range tagCarrierSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

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

// FunctionUnit is one top-level Go function, including its contiguous doc
// comment and ending immediately after its closing brace.
type FunctionUnit struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// FunctionUnits answers every top-level Go function in file order.
func FunctionUnits(content string) []FunctionUnit {
	spans := goFuncSpans(content)
	units := make([]FunctionUnit, 0, len(spans))
	for _, one := range spans {
		text := content[one.begin:one.end]
		units = append(units, FunctionUnit{Name: funcNameIn(text), Text: text})
	}
	return units
}

// functionNameAt answers the top-level Go function enclosing a 1-based line.
// An empty answer means the whole file is the unit.
func functionNameAt(path, content string, line int) string {
	return newScopeIndex().funcNameAt(path, content, line)
}

// functionText answers the text of the one top-level function declared name.
//
// It refuses absent and duplicate names alike. Two receiver methods can share
// a name in one file, and picking either would fingerprint text nobody chose.
func functionText(content, name string) (string, bool) {
	found := newScopeIndex().funcTexts(content, name)
	if len(found) != 1 {
		return "", false
	}
	return found[0], true
}

// TaggedUnit is the text one RFC tag governs and how it was resolved.
type TaggedUnit struct {
	Text  string    `json:"text"`
	Scope ScopeKind `json:"scope"`
}

// UnitAt answers the tagged unit at a 1-based line.
//
// A line outside exactly one Go function resolves to the whole file. A narrow
// guess could leave changed evidence falsely fresh; file scope can only ask for
// an extra re-read.
func UnitAt(path, content string, line int) TaggedUnit {
	if ScopeReader(path) != ScopeGo {
		return TaggedUnit{Text: content, Scope: ScopeFile}
	}
	at := lineOffset(content, line)
	var hit span
	found := 0
	for _, one := range goFuncSpans(content) {
		if one.begin <= at && at < one.end {
			hit = one
			found++
		}
	}
	if found != 1 {
		return TaggedUnit{Text: content, Scope: ScopeFile}
	}
	return TaggedUnit{Text: content[hit.begin:hit.end], Scope: ScopeFunc}
}

// EditHunk is the old text one edit replaces and whether every occurrence is
// replaced.
type EditHunk struct {
	Old        string `json:"old"`
	ReplaceAll bool   `json:"replace-all"`
}

// tagScope widens edit hunks to the text whose RFC tags govern them.
//
// The false answer means no widening is needed. An unlocatable hunk, a hunk
// outside every function, or a tag outside every function returns the whole
// file, because every narrower answer would be a guess.
func tagScope(path, content string, hunks []EditHunk, tag *regexp.Regexp) (string, bool) {
	if len(hunks) == 0 || tag == nil || !tag.MatchString(content) {
		return "", false
	}
	if ScopeReader(path) != ScopeGo {
		return content, true
	}

	spans := goFuncSpans(content)
	for _, at := range tag.FindAllStringIndex(content, -1) {
		inside := false
		for _, one := range spans {
			if one.begin <= at[0] && at[0] < one.end {
				inside = true
				break
			}
		}
		if !inside {
			return content, true
		}
	}

	picked := map[span]bool{}
	for _, hunk := range hunks {
		if hunk.Old == "" {
			continue
		}
		found := false
		start := strings.Index(content, hunk.Old)
		for start >= 0 {
			found = true
			end := start + len(hunk.Old)
			hit := false
			for _, one := range spans {
				if one.begin < end && start < one.end {
					picked[one] = true
					hit = true
				}
			}
			if !hit {
				return content, true
			}
			if !hunk.ReplaceAll {
				break
			}
			next := strings.Index(content[start+1:], hunk.Old)
			if next < 0 {
				break
			}
			start += next + 1
		}
		if !found {
			return content, true
		}
	}
	if len(picked) == 0 {
		return "", false
	}
	ordered := make([]span, 0, len(picked))
	for one := range picked {
		ordered = append(ordered, one)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].begin != ordered[j].begin {
			return ordered[i].begin < ordered[j].begin
		}
		return ordered[i].end < ordered[j].end
	})
	units := make([]string, 0, len(ordered))
	for _, one := range ordered {
		units = append(units, content[one.begin:one.end])
	}
	return strings.Join(units, "\n"), true
}
