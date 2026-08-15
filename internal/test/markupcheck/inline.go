// Design: docs/architecture/web-components.md -- markup lives in .templ, never in Go
// Overview: markupcheck.go -- the package doc and the Go-literal scan

package markupcheck

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// inlineHandler matches an inline event handler attribute and htmx's hx-on
// family. Both run script the page carries, so both need 'unsafe-inline'.
var inlineHandler = regexp.MustCompile(`\s(?:on[a-z]+|hx-on(?:::|:)[^=]*)=`)

// Shortfall returns a message when got is under want, and the empty string
// otherwise. It is the guard every walk in this package needs. A scan that
// reads nothing reports nothing. A filter on a suffix the tree no longer
// carries visits zero files and passes.
func Shortfall(what string, got, want int) string {
	if got >= want {
		return ""
	}

	var tb textbuf.Buffer

	return tb.Str("read ").Int(int64(got)).Byte(' ').Str(what).Str(", want at least ").
		Int(int64(want)).Str("; the walk found little to read").String()
}

// InlineFindings returns one message per .templ file under root that carries
// something a strict CSP refuses, plus the number of files it read.
//
// A response served under `script-src 'self'`, or under a bare
// `default-src 'self'` with no script-src beside it, runs none of these. The
// browser refuses them and tells the server nothing, so the feature is dead.
// This scan is what turns that silence into a finding.
//
// It reads .templ rather than the generated *_templ.go. The generated file
// holds the same markup as a Go string, so reading both reports one defect
// twice.
func InlineFindings(root string) ([]string, int, error) {
	var (
		findings []string
		files    int
	)

	fsys := os.DirFS(root)

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() || !strings.HasSuffix(path, ".templ") {
			return nil
		}

		body, readErr := fs.ReadFile(fsys, path)
		if readErr != nil {
			return readErr
		}

		files++
		findings = append(findings, inlineOf(path, string(body))...)

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return findings, files, nil
}

// inlineOf returns the messages for one file. The order is fixed, so a caller
// reads them as they are.
func inlineOf(name, content string) []string {
	var (
		found []string
		tb    textbuf.Buffer
	)

	if strings.Contains(content, "<script>") {
		found = append(found, tb.Str(name).Str(" carries an inline <script> block").String())
	}

	if strings.Contains(content, "style=") {
		tb.Reset()
		found = append(found, tb.Str(name).Str(" carries an inline style attribute").String())
	}

	if inlineHandler.MatchString(content) {
		tb.Reset()
		found = append(found, tb.Str(name).Str(" carries an inline event handler or hx-on attribute").String())
	}

	return found
}

// AssertNoInlineScriptOrStyle fails on any .templ file under root that carries
// an inline script block, an inline style attribute, or an inline event
// handler. It fails on a walk that read fewer than minFiles of them too.
func AssertNoInlineScriptOrStyle(t *testing.T, root string, minFiles int) {
	t.Helper()

	findings, files, err := InlineFindings(root)
	if err != nil {
		t.Fatalf("scan %s for .templ: %v", root, err)
	}

	if short := Shortfall("markup files", files, minFiles); short != "" {
		t.Errorf("scan %s %s", root, short)
	}

	for _, f := range findings {
		t.Errorf("%s", f)
	}
}
