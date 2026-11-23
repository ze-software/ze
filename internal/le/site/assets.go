// Design: website/AI.md -- deploy one expanded stylesheet and one script
package site

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// renderScript minifies one authored script while preserving string, template,
// and regular expression literals and line breaks for automatic semicolon insertion.
func renderScript(source, path string) error {
	content, err := os.ReadFile(source) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return err
	}
	minified, err := minifyJS(content)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(source), err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	return os.WriteFile(path, minified, 0o644) //nolint:gosec // published web content: a web server, often another account, serves these bytes
}

// minifyJS strips comments, indentation, blank lines, and redundant horizontal
// whitespace from a JavaScript file while preserving strings, template
// literals, regex literals, and statement-terminating newlines.
func minifyJS(content []byte) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(len(content))

	// templateBraces tracks nested brace depth inside each active `${...}`
	// interpolation so closing `}` resumes the enclosing template literal.
	var templateBraces []int
	prevKind := jsTokStart
	prevWord := ""
	pendingSpace := false
	pendingNewline := false

	emitSep := func(next byte, nextIsWord bool) {
		if out.Len() == 0 {
			pendingSpace = false
			pendingNewline = false
			return
		}
		if pendingNewline {
			out.WriteByte('\n')
			pendingSpace = false
			pendingNewline = false
			return
		}
		if !pendingSpace {
			return
		}
		pendingSpace = false
		last := out.Bytes()[out.Len()-1]
		if jsNeedsSpace(last, next, nextIsWord) {
			out.WriteByte(' ')
		}
	}

	for i := 0; i < len(content); {
		ch := content[i]

		// Inside a template literal's `${...}` interpolation, closing `}` at depth 0
		// resumes scanning the template literal text.
		if ch == '}' && len(templateBraces) > 0 && templateBraces[len(templateBraces)-1] == 0 {
			templateBraces = templateBraces[:len(templateBraces)-1]
			pendingSpace = false
			pendingNewline = false
			out.WriteByte('}')
			i++
			next, done, err := scanJSTemplateChunk(content, i, &out)
			if err != nil {
				return nil, err
			}
			i = next
			if done {
				prevKind = jsTokString
				prevWord = ""
			} else {
				templateBraces = append(templateBraces, 0)
				prevKind = jsTokPunct
				prevWord = "{"
			}
			continue
		}

		// Whitespace: preserve newlines for ASI; defer horizontal spaces.
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\f' || ch == '\v' {
			pendingSpace = true
			i++
			continue
		}
		if ch == '\n' {
			pendingNewline = true
			pendingSpace = false
			i++
			continue
		}

		// Line comment.
		if ch == '/' && i+1 < len(content) && content[i+1] == '/' {
			i += 2
			for i < len(content) && content[i] != '\n' {
				i++
			}
			continue
		}

		// Block comment.
		if ch == '/' && i+1 < len(content) && content[i+1] == '*' {
			end := bytes.Index(content[i+2:], []byte("*/"))
			if end == -1 {
				return nil, opererr.New("script has an unclosed block comment at byte "+strconv.Itoa(i),
					"Publication stopped to preserve the script's syntax.", "./le site build")
			}
			if bytes.IndexByte(content[i+2:i+2+end], '\n') >= 0 {
				pendingNewline = true
				pendingSpace = false
			} else {
				pendingSpace = true
			}
			i += end + 4
			continue
		}

		// Quoted string literal.
		if ch == '"' || ch == '\'' {
			emitSep(ch, false)
			start := i
			i++
			closed := false
			for i < len(content) {
				if content[i] == '\\' {
					i += min(2, len(content)-i)
					continue
				}
				if content[i] == ch {
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, opererr.New("script has an unclosed string at byte "+strconv.Itoa(start),
					"Publication stopped to preserve the script's syntax.", "./le site build")
			}
			out.Write(content[start:i])
			prevKind = jsTokString
			prevWord = ""
			continue
		}

		// Template literal.
		if ch == '`' {
			emitSep(ch, false)
			out.WriteByte('`')
			i++
			next, done, err := scanJSTemplateChunk(content, i, &out)
			if err != nil {
				return nil, err
			}
			i = next
			if done {
				prevKind = jsTokString
				prevWord = ""
			} else {
				templateBraces = append(templateBraces, 0)
				prevKind = jsTokPunct
				prevWord = "{"
			}
			continue
		}

		// Regex literal (when `/` cannot be division).
		if ch == '/' && jsRegexCanFollow(prevKind, prevWord) {
			emitSep(ch, false)
			start := i
			i++
			inClass := false
			closed := false
			for i < len(content) {
				c := content[i]
				if c == '\\' {
					i += min(2, len(content)-i)
					continue
				}
				if c == '\n' {
					break
				}
				if c == '/' && !inClass {
					i++
					for i < len(content) && isJSIdentPart(content[i]) {
						i++
					}
					closed = true
					break
				}
				switch c {
				case '[':
					inClass = true
				case ']':
					inClass = false
				}
				i++
			}
			if !closed {
				return nil, opererr.New("script has an unclosed regex literal at byte "+strconv.Itoa(start),
					"Publication stopped to preserve the script's syntax.", "./le site build")
			}
			out.Write(content[start:i])
			prevKind = jsTokRegex
			prevWord = ""
			continue
		}

		// Identifiers, keywords, and numbers.
		if isJSIdentStart(ch) || (ch >= '0' && ch <= '9') {
			emitSep(ch, true)
			start := i
			if ch >= '0' && ch <= '9' {
				for i < len(content) && (isJSIdentPart(content[i]) || content[i] == '.') {
					if (content[i] == 'e' || content[i] == 'E') && i+1 < len(content) && (content[i+1] == '+' || content[i+1] == '-') {
						i += 2
						continue
					}
					i++
				}
				prevKind = jsTokNumber
				prevWord = ""
			} else {
				for i < len(content) && isJSIdentPart(content[i]) {
					i++
				}
				prevKind = jsTokWord
				prevWord = string(content[start:i])
			}
			out.Write(content[start:i])
			continue
		}

		// Multi-character `++` / `--` must keep their token kind so `/` after them
		// is parsed as division rather than a regex literal.
		if (ch == '+' || ch == '-') && i+1 < len(content) && content[i+1] == ch {
			emitSep(ch, false)
			out.Write(content[i : i+2])
			prevKind = jsTokIncDec
			prevWord = string(content[i : i+2])
			i += 2
			continue
		}

		// Punctuation and operators.
		emitSep(ch, false)
		out.WriteByte(ch)
		if ch == '{' && len(templateBraces) > 0 {
			templateBraces[len(templateBraces)-1]++
		} else if ch == '}' && len(templateBraces) > 0 {
			templateBraces[len(templateBraces)-1]--
		}
		switch ch {
		case ')':
			prevKind = jsTokCloseParen
		case ']':
			prevKind = jsTokCloseBracket
		case '}':
			prevKind = jsTokCloseBrace
		default:
			prevKind = jsTokPunct
		}
		prevWord = string(ch)
		i++
	}

	if out.Len() > 0 {
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

const (
	jsTokStart = iota
	jsTokPunct
	jsTokWord
	jsTokNumber
	jsTokString
	jsTokRegex
	jsTokCloseParen
	jsTokCloseBracket
	jsTokCloseBrace
	jsTokIncDec
)

// scanJSTemplateChunk copies raw template characters until an unescaped closing
// backtick (done=true) or `${` interpolation start (done=false).
func scanJSTemplateChunk(content []byte, i int, out *bytes.Buffer) (int, bool, error) {
	start := i
	for i < len(content) {
		c := content[i]
		if c == '\\' {
			end := min(i+2, len(content))
			out.Write(content[i:end])
			i = end
			continue
		}
		if c == '`' {
			out.WriteByte('`')
			return i + 1, true, nil
		}
		if c == '$' && i+1 < len(content) && content[i+1] == '{' {
			out.WriteString("${")
			return i + 2, false, nil
		}
		out.WriteByte(c)
		i++
	}
	return i, false, opererr.New("script has an unclosed template literal at byte "+strconv.Itoa(start),
		"Publication stopped to preserve the script's syntax.", "./le site build")
}

func isJSIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '$' || ch >= 0x80
}

func isJSIdentPart(ch byte) bool {
	return isJSIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func jsRegexCanFollow(kind int, word string) bool {
	switch kind {
	case jsTokStart, jsTokPunct, jsTokCloseBrace:
		return true
	case jsTokWord:
		switch word {
		case "return", "case", "throw", "in", "instanceof", "of", "typeof", "void", "delete", "else", "do", "yield", "await", "new":
			return true
		}
	}
	return false
}

// jsNeedsSpace keeps a single separating space whenever omitting it would merge
// two identifiers/numbers or form an unintended compound operator (`++`, `--`, `//`, `/*`).
func jsNeedsSpace(prev, next byte, nextIsWord bool) bool {
	prevIsWord := isJSIdentPart(prev)
	if prevIsWord && nextIsWord {
		return true
	}
	if prev == '+' && next == '+' {
		return true
	}
	if prev == '-' && next == '-' {
		return true
	}
	if prev == '/' && (next == '/' || next == '*') {
		return true
	}
	if prev == '<' && next == '!' {
		return true
	}
	if prev == '-' && next == '>' {
		return true
	}
	return false
}
