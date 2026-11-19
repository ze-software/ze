// Design: website/AI.md -- deploy one expanded stylesheet and one script
package site

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

var cssImport = regexp.MustCompile(`(?m)^\s*@import\s+url\(["']?([^"')]+)["']?\);\s*$`)
var cssSpace = regexp.MustCompile(`\s+`)
var cssPunctuationSpace = regexp.MustCompile(`\s*([{},;])\s*`)
var cssColonSpace = regexp.MustCompile(`:\s*`)

// RenderAssets publishes the current stylesheet and authored scripts using the
// same transformations as a full build. Static vendor files remain the caller's
// staging responsibility.
func RenderAssets(paths Paths) error {
	if err := renderCSS(paths.Source, paths.Output); err != nil {
		return err
	}
	return renderJS(paths.Source, paths.Output)
}

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
	return minifyCSS(content)
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

// minifyCSS treats strings as opaque bytes. Comment syntax outside a string
// must be balanced before renderCSS can replace the published stylesheet.
func minifyCSS(content []byte) ([]byte, error) {
	var out, code bytes.Buffer
	flush := func() {
		text := cssSpace.ReplaceAllString(code.String(), " ")
		text = cssPunctuationSpace.ReplaceAllString(text, "$1")
		text = cssColonSpace.ReplaceAllString(text, ":")
		out.WriteString(strings.ReplaceAll(text, ";}", "}"))
		code.Reset()
	}
	// Each step consumes at least one source byte; quotes and comments consume
	// at most the remaining input, so malformed source cannot extend this scan.
	for offset := 0; offset < len(content); {
		character := content[offset]
		if character == '"' || character == '\'' {
			flush()
			start := offset
			offset++
			closed := false
			for offset < len(content) {
				if content[offset] == '\\' {
					offset += min(2, len(content)-offset)
					continue
				}
				if content[offset] == character {
					offset++
					closed = true
					break
				}
				offset++
			}
			if !closed {
				return nil, opererr.New(fmt.Sprintf("stylesheet has an unclosed string at expanded byte %d; close the string in website/assets/css/", start),
					"Publication stopped to preserve the stylesheet's rules.", "./le site build")
			}
			out.Write(content[start:offset])
			continue
		}
		if bytes.HasPrefix(content[offset:], []byte("/*")) {
			end := bytes.Index(content[offset+2:], []byte("*/"))
			if end == -1 {
				return nil, opererr.New(fmt.Sprintf("stylesheet has an unclosed comment at expanded byte %d; close the comment in website/assets/css/", offset),
					"Publication stopped to preserve the stylesheet's rules.", "./le site build")
			}
			offset += end + 4
			continue
		}
		if bytes.HasPrefix(content[offset:], []byte("*/")) {
			return nil, opererr.New(fmt.Sprintf("stylesheet has an unmatched comment close at expanded byte %d; repair the comment in website/assets/css/", offset),
				"Publication stopped to preserve the stylesheet's rules.", "./le site build")
		}
		code.WriteByte(character)
		offset++
	}
	flush()
	return bytes.TrimSpace(out.Bytes()), nil
}

// renderJS writes every authored script under website/assets/js/ to
// assets/<name>, without source-only trailing whitespace. site.js is required:
// every page loads it, so a checkout without it is refused rather than
// published scriptless. Other scripts (rfc-figures.js) are published beside it
// by being there, with no list to edit.
func renderJS(source, output string) error {
	directory := filepath.Join(source, "assets", "js")
	if _, err := os.Stat(filepath.Join(directory, "site.js")); err != nil {
		return err
	}
	scripts, err := filepath.Glob(filepath.Join(directory, "*.js"))
	if err != nil {
		return err
	}
	for _, script := range scripts {
		if err := renderScript(script, filepath.Join(output, "assets", filepath.Base(script))); err != nil {
			return err
		}
	}
	return nil
}

// renderScript writes one authored script without trailing whitespace.
func renderScript(source, path string) error {
	content, err := os.ReadFile(source) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " \t")
	}
	content = []byte(strings.TrimSpace(strings.Join(lines, "\n")) + "\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	return os.WriteFile(path, content, 0o644) //nolint:gosec // published web content: a web server, often another account, serves these bytes
}
