// Design: docs/architecture/core-design.md -- repository citation grammar
// Overview: links.go -- the checks that apply this grammar.
// Related: internal/le/doc/citation/citation.go -- what a citation is.

package doccheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/le/doc/citation"
)

var ignoreMarkerRe = regexp.MustCompile(`doc-links:\s*ignore`)

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
	clean := citation.Backtick.ReplaceAllString(line, " ")
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
