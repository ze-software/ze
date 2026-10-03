// Design: docs/architecture/core-design.md -- repository citation grammar
// Related: internal/le/doc/check/links.go -- the link sweep that applies this grammar.
// Related: internal/le/rfc/rename.go -- the rename that rewrites what this grammar answers.

// Package citation declares what a repository citation is: a path in backticks
// or a markdown link target. It is a leaf, so the link sweep in
// internal/le/doc/check and the rename in internal/le/rfc read one grammar
// without either importing the other.
package citation

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Backtick matches one backticked span; group 1 is the span's body.
var Backtick = regexp.MustCompile("`([^`]+)`")

var (
	markdownLinkRe  = regexp.MustCompile(`\]\(([^)]*)\)`)
	lineSuffixRe    = regexp.MustCompile(`:\d+(?:-\d+)?$`)
	lineRunSuffixRe = regexp.MustCompile(`(?:,\d+(?:-\d+)?)+$`)
	symbolColonRe   = regexp.MustCompile(`::?[A-Za-z_][\w.]*$`)
	symbolDotRe     = regexp.MustCompile(`\.[A-Z]\w*$`)
	braceRe         = regexp.MustCompile(`\{([^{}]+)\}`)
)

var knownRoots = map[string]bool{
	"ai": true, ".claude": true, ".codex": true, ".agents": true,
	".github": true, "internal": true, "cmd": true, "pkg": true,
	"test": true, "plan": true, "docs": true, "rfc": true, "tools": true,
	"etc": true, "examples": true, "api": true, "contrib": true,
	"gokrazy": true, "third_party": true, "parked": true, "vendor": true,
	"rules": true, "patterns": true,
}

var rootFiles = map[string]bool{
	"CLAUDE.md": true, "AGENTS.md": true, "README.md": true,
	"go.mod": true, "go.sum": true, ".gitignore": true, ".golangci.yml": true,
	"LICENSE": true, "SECURITY.md": true, "CONTRIBUTING.md": true,
}

var placeholderMarkers = [...]string{"<", ">", "$", "*", "NNN", "...", ".."}
var skipPrefixes = [...]string{"tmp/", "bin/", "~", "/", "test/tmp/"}

// Paths answers the repository paths one line cites, in backticks or as a
// markdown link target. It is the one declaration of what a citation is: the
// link sweep checks exactly what this answers and `./le rfc rename` rewrites
// exactly what this answers, so the two cannot disagree about a citation.
// Safe for concurrent use.
func Paths(root, line string) []string {
	var raw []string
	for _, found := range Backtick.FindAllStringSubmatch(line, -1) {
		raw = append(raw, found[1])
	}
	for _, found := range markdownLinkRe.FindAllStringSubmatch(line, -1) {
		if found[1] == "" {
			continue
		}
		if found[1][0] == '#' {
			continue
		}
		raw = append(raw, found[1])
	}
	out := make([]string, 0, len(raw))
	for _, token := range raw {
		if externalCitation(token) {
			continue
		}
		out = append(out, candidates(root, token)...)
	}
	return out
}

// Exists reports whether rel names a file or directory under root. A trailing
// slash is ignored. Any stat error answers false, because the grammar uses it
// only to decide whether a `.Symbol` suffix belongs to the path.
func Exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/"))))
	return err == nil
}

func candidates(root, raw string) []string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return nil
	}
	token := strings.TrimRight(fields[0], ".,;:)('\"")
	if before, _, ok := strings.Cut(token, "#"); ok {
		token = before
	}
	token = lineRunSuffixRe.ReplaceAllString(token, "")
	token = lineSuffixRe.ReplaceAllString(token, "")
	token = symbolColonRe.ReplaceAllString(token, "")
	if !Exists(root, token) {
		token = symbolDotRe.ReplaceAllString(token, "")
	}
	if token == "" {
		return nil
	}
	if !strings.Contains(token, "/") {
		if !rootFiles[token] {
			return nil
		}
	}
	if containsMarker(token) {
		return nil
	}
	if skipped(token) {
		return nil
	}
	first, _, _ := strings.Cut(token, "/")
	if !rootFiles[token] {
		if !knownRoots[first] {
			return nil
		}
	}
	expanded := expandBraces(token)
	out := expanded[:0]
	for _, path := range expanded {
		if !containsMarker(path) {
			out = append(out, path)
		}
	}
	return out
}

func externalCitation(token string) bool {
	if strings.HasPrefix(token, "http://") {
		return true
	}
	if strings.HasPrefix(token, "https://") {
		return true
	}
	return strings.HasPrefix(token, "mailto:")
}

// expandBraces recurses once per brace group in token, so its depth is bounded
// by the length of one line of repository text.
func expandBraces(token string) []string {
	match := braceRe.FindStringSubmatchIndex(token)
	if match == nil {
		return []string{token}
	}
	body := token[match[2]:match[3]]
	if !strings.Contains(body, ",") {
		return []string{token}
	}
	var out []string
	for alt := range strings.SplitSeq(body, ",") {
		expanded := token[:match[0]] + alt + token[match[1]:]
		out = append(out, expandBraces(expanded)...)
	}
	return out
}

func containsMarker(token string) bool {
	for _, marker := range placeholderMarkers {
		if strings.Contains(token, marker) {
			return true
		}
	}
	return false
}

func skipped(token string) bool {
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}
