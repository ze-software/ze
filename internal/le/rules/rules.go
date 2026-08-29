// Design: docs/architecture/core-design.md -- the rule corpus, as one command
// Detail: lint.go -- the format linter these spellings are printed by
// Detail: points.go -- the split these spellings are printed by
// Detail: digest.go -- the parse the two generated artifacts are built from
// Detail: index.go -- the greppable map of the corpus
// Detail: actions.go -- the eleven gates and transcript report this corpus serves
//
// Package rules owns the checks and reports for `ai/rules/`. They cover the
// rendered rule format, source point files, hook bindings, and session reads.
//
// Every consumer uses one corpus predicate. The Python scripts each defined
// which files were rules. Go uses one ruleFiles function, so this drift has no
// place to occur.
//
// Python formats answers with str.strip, repr(), and list literals. This port
// preserves those forms instead of improving them. Both implementations run
// together until cutover, and any output difference needs an explanation.
// pyRepr and pyListRepr exist only for this parity.

package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb. Every gate of this family is spelled
// `ze-rules-<verb>`, so the whole family is one area.
const area = "rules"

// rulesRel, pointsRel and retiredRel name the corpus, relative to the checkout.
// A path is joined from these rather than spelled inline, so a reader finds
// every consumer of the tree by finding the three names.
const (
	rulesRel   = "ai/rules"
	pointsRel  = "ai/rules/points"
	retiredRel = "ai/rules/points/RETIRED.md"
)

// skip names the generated aggregates that sit beside the rules and are not
// rules. It is rules_lint.SKIP, and the all-caps-stem test below covers
// TRIGGERS.md and CORE.md, which are recognized by shape.
var skip = map[string]bool{"INDEX.md": true, "CONDENSED.md": true}

// IsArtifact reports whether a file beside the rules is a generated aggregate
// rather than a rule. The Python tools rules_lint, rules_points, and
// rule_coverage each spelled this predicate separately.
//
// The all-caps stem test is what recognizes TRIGGERS.md and CORE.md, which are
// named in no list: they are generated, and a generator that adds a third
// aggregate should not have to be remembered here. That shape test is the half
// a second copy of this predicate loses. internal/le/sourcerewrite kept one --
// three names and no shape -- so its rewriter walked into both generated files
// and, on 2026-08-29, they were the only two files it would have rewritten.
// Exported so that copy could be deleted rather than corrected.
func IsArtifact(name string) bool {
	return skip[name] || isUpperStem(strings.TrimSuffix(name, ".md"))
}

// ruleFiles answers the rule corpus under rulesDir: every `*.md` that is not a
// generated artifact, sorted by name.
//
// The glob is not recursive, so `points/` stays invisible to it, which is what
// lets the point tree live inside the rules directory.
func ruleFiles(rulesDir string) ([]string, error) {
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || IsArtifact(name) {
			continue
		}
		out = append(out, filepath.Join(rulesDir, name))
	}
	sort.Strings(out)
	return out, nil
}

// isUpperStem reports Python str.isupper() for a filename stem. It requires at
// least one cased character and no lowercase characters.
//
// By repository convention, an all-caps stem names a generated artifact. This
// check prevents the linter from treating a new artifact as a malformed rule.
func isUpperStem(stem string) bool {
	cased := false
	for _, r := range stem {
		switch {
		case unicode.IsLower(r):
			return false
		case unicode.IsUpper(r) || unicode.IsTitle(r):
			cased = true
		}
	}
	return cased
}

// pyRepr answers Python repr() for a string.
//
// Eight messages interpolate repr(), whose quote is data. repr("it's") uses
// double quotes, while repr("plain") uses single quotes. This function preserves
// script output during the parallel run.
func pyRepr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}

	var tb textbuf.Buffer
	tb.Byte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote) || r == '\\':
			tb.Byte('\\').Byte(byte(r))
		case r == '\t':
			tb.Str("\\t")
		case r == '\n':
			tb.Str("\\n")
		case r == '\r':
			tb.Str("\\r")
		case r < ' ' || r == 0x7f:
			tb.Str("\\x").Str(hexPair(byte(r)))
		case r < 0x7f:
			tb.Byte(byte(r))
		case unicode.IsPrint(r):
			tb.WriteRune(r) //nolint:errcheck // textbuf never fails
		default:
			tb.Str(escapeRune(r))
		}
	}
	return tb.Byte(quote).String()
}

// hexDigits is the lowercase alphabet Python's \x, \u and \U escapes use.
const hexDigits = "0123456789abcdef"

// hexPair renders one byte as two lowercase hex digits, which is the width
// Python's \x escape always emits.
func hexPair(b byte) string {
	var tb textbuf.Buffer
	return tb.Byte(hexDigits[b>>4]).Byte(hexDigits[b&0x0f]).String()
}

// escapeRune renders a nonprintable rune in the same form as Python repr().
// It uses \xNN for values less than 0x100 and \uNNNN for values less than
// 0x10000. It uses \UNNNNNNNN for larger values.
func escapeRune(r rune) string {
	var tb textbuf.Buffer
	switch {
	case r < 0x100:
		return tb.Str("\\x").Str(hexPair(byte(r))).String()
	case r < 0x10000:
		tb.Str("\\u")
		for shift := 12; shift >= 0; shift -= 4 {
			tb.Byte(hexDigits[(r>>uint(shift))&0x0f])
		}
		return tb.String()
	default:
		tb.Str("\\U")
		for shift := 28; shift >= 0; shift -= 4 {
			tb.Byte(hexDigits[(r>>uint(shift))&0x0f])
		}
		return tb.String()
	}
}

// pyListRepr answers what Python prints for a list of str inside an f-string:
// the elements' reprs, comma-separated, in brackets.
func pyListRepr(items []string) string {
	var tb textbuf.Buffer
	tb.Byte('[')
	for i, item := range items {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(pyRepr(item))
	}
	return tb.Byte(']').String()
}

// firstRunes answers the first n runes of s, which is what Python's s[:n]
// answers. Go's slice syntax counts BYTES, so the two disagree the moment a
// rule line carries an em dash, and the corpus is full of them.
func firstRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	count := 0
	for index := range s {
		if count == n {
			return s[:index]
		}
		count++
	}
	return s
}

// lastRune answers the final rune of s as a string, like Python s[-1]. For an
// empty string, it answers "" instead of raising. Every caller rejects an empty
// value first.
func lastRune(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeLastRuneInString(s)
	if r == utf8.RuneError && size <= 1 {
		return s[len(s)-1:]
	}
	return s[len(s)-size:]
}

// sortedUnique answers the members of set, sorted, which is what Python's
// sorted(set(...)) answers.
func sortedUnique(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// relTo answers path relative to tree, in slash form.
//
// Every printed path is relative to tree. The scripts instead print absolute
// paths that they derive from __file__. A payload is data before it is a page,
// and a build-host path describes the machine, not the corpus. Step 7 of
// spec-le-is-a-ze-binary made the same choice for codegen ports.
func relTo(tree, path string) string {
	rel, err := filepath.Rel(tree, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
