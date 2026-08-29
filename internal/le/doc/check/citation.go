// Design: docs/architecture/core-design.md -- repository citation grammar
// Overview: links.go -- the checks that apply this grammar.

package doccheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	backtickRe      = regexp.MustCompile("`([^`]+)`")
	markdownLinkRe  = regexp.MustCompile(`\]\(([^)]*)\)`)
	lineSuffixRe    = regexp.MustCompile(`:\d+(?:-\d+)?$`)
	lineRunSuffixRe = regexp.MustCompile(`(?:,\d+(?:-\d+)?)+$`)
	symbolColonRe   = regexp.MustCompile(`::?[A-Za-z_][\w.]*$`)
	symbolDotRe     = regexp.MustCompile(`\.[A-Z]\w*$`)
	braceRe         = regexp.MustCompile(`\{([^{}]+)\}`)
	ignoreMarkerRe  = regexp.MustCompile(`doc-links:\s*ignore`)
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

func lineCitations(root, line string) []string {
	var raw []string
	for _, found := range backtickRe.FindAllStringSubmatch(line, -1) {
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
		out = append(out, candidatePaths(root, token)...)
	}
	return out
}

func candidatePaths(root, raw string) []string {
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
	if !pathExists(root, token) {
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
	if hasPrefix(token, skipPrefixes[:]) {
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

func hasPrefix(token string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}

func pathExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/"))))
	return err == nil
}

func pathResolves(root, rel string) (bool, error) {
	path := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/")))
	if strings.ContainsAny(rel, "*?[") {
		matches, err := filepath.Glob(path)
		return len(matches) > 0, err
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func ignoreMarkers(line string) []string {
	clean := backtickRe.ReplaceAllString(line, " ")
	var tails []string
	for {
		start := strings.Index(clean, "<!--")
		if start < 0 {
			break
		}
		clean = clean[start+len("<!--"):]
		end := strings.Index(clean, "-->")
		if end < 0 {
			break
		}
		comment := clean[:end]
		clean = clean[end+len("-->"):]
		marker := ignoreMarkerRe.FindStringIndex(comment)
		if marker != nil {
			tails = append(tails, comment[marker[1]:])
		}
	}
	return tails
}

func markerReason(tail string) string {
	open := strings.IndexByte(tail, '(')
	if open < 0 {
		return ""
	}
	close := strings.IndexByte(tail[open+1:], ')')
	if close < 0 {
		return ""
	}
	return strings.TrimSpace(tail[open+1 : open+1+close])
}

func suppressed(line string) bool {
	for _, tail := range ignoreMarkers(line) {
		if markerReason(tail) != "" {
			return true
		}
	}
	return false
}
