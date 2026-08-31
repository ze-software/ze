// Design: website/AI.md -- deploy one expanded stylesheet and one script
package site

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var cssImport = regexp.MustCompile(`(?m)^\s*@import\s+url\(["']?([^"')]+)["']?\);\s*$`)
var blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
var cssSpace = regexp.MustCompile(`\s+`)
var cssPunctuationSpace = regexp.MustCompile(`\s*([{},;])\s*`)
var cssColonSpace = regexp.MustCompile(`:\s*`)

// siteStylesheet answers the deployable stylesheet: every local import of
// site.css expanded once, minified.
//
// Two surfaces read it. renderCSS publishes it as assets/site.css, and
// renderActivity inlines it into the talk embed, which is served as an iframe
// srcdoc and can resolve no link to that file.
func siteStylesheet(source string) ([]byte, error) {
	entry := filepath.Join(source, "assets", "css", "site.css")
	content, err := expandCSS(entry, make(map[string]bool))
	if err != nil {
		return nil, err
	}
	return minifyCSS(content), nil
}

// renderCSS writes the deployable stylesheet into the artifact.
func renderCSS(source, output string) error {
	content, err := siteStylesheet(source)
	if err != nil {
		return err
	}
	path := filepath.Join(output, "assets", "site.css")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	return os.WriteFile(path, content, 0o644) //nolint:gosec // published web content: a web server, often another account, serves these bytes
}

func expandCSS(path string, active map[string]bool) ([]byte, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if active[absolute] {
		return nil, fmt.Errorf("circular CSS import: %s", absolute)
	}
	active[absolute] = true
	defer delete(active, absolute)
	content, err := os.ReadFile(absolute) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return nil, err
	}
	matches := cssImport.FindAllSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil
	}
	var out bytes.Buffer
	offset := 0
	for _, match := range matches {
		out.Write(content[offset:match[0]])
		name := string(content[match[2]:match[3]])
		if strings.Contains(name, "://") {
			out.Write(content[match[0]:match[1]])
		} else {
			nested, nestedErr := expandCSS(filepath.Join(filepath.Dir(absolute), filepath.FromSlash(name)), active)
			if nestedErr != nil {
				return nil, nestedErr
			}
			out.Write(nested)
		}
		offset = match[1]
	}
	out.Write(content[offset:])
	return out.Bytes(), nil
}

func minifyCSS(content []byte) []byte {
	text := blockComment.ReplaceAllString(string(content), "")
	text = cssSpace.ReplaceAllString(text, " ")
	text = cssPunctuationSpace.ReplaceAllString(text, "$1")
	text = cssColonSpace.ReplaceAllString(text, ":")
	text = strings.ReplaceAll(text, ";}", "}")
	return []byte(strings.TrimSpace(text))
}

// renderJS writes the authored script without source-only trailing whitespace.
func renderJS(source, output string) error {
	content, err := os.ReadFile(filepath.Join(source, "assets", "js", "site.js")) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " \t")
	}
	content = []byte(strings.TrimSpace(strings.Join(lines, "\n")) + "\n")
	path := filepath.Join(output, "assets", "site.js")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	return os.WriteFile(path, content, 0o644) //nolint:gosec // published web content: a web server, often another account, serves these bytes
}
