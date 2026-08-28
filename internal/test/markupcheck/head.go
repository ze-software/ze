// Design: docs/architecture/web-components.md -- markup lives in .templ, never in Go
// Overview: markupcheck.go -- the package doc and the Go-literal scan
// Related: assets.go -- the sibling scan, which reads the sources rather than the captures

package markupcheck

import (
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two vendored files a rendered attribute can need. The names repeat what
// internal/le/webassets/webassets.go carries, and the repetition is the point: that
// generator over-approximates from the sources while this reads the captured
// output, so neither may take its answer from the other.
const (
	htmxAsset = "htmx.min.js"
	sseAsset  = "hx-sse.min.js"
)

// headMarker is what makes a capture a whole page rather than a fragment. A
// fragment is swapped into a page that has already loaded its assets, so it
// states nothing about what it needs.
const headMarker = "<head>"

// htmxAttribute finds one htmx attribute in rendered markup: a whitespace byte,
// the attribute name, then the equals sign a value follows. Requiring both ends
// keeps a file name such as sse-client.js out of the match. htmx 4 names an
// extension's attributes with a colon, as hx-sse:connect does.
var htmxAttribute = regexp.MustCompile(`\s(hx-[a-z:-]+)=`)

// assetImplementing names the vendored file one rendered attribute needs, or "".
//
// An extension attribute names the extension: the core is what every other
// htmx attribute on the page already needs, and a page carrying an extension
// attribute alone still fails here when its head loads no extension.
func assetImplementing(attribute string) string {
	switch {
	case strings.HasPrefix(attribute, "hx-sse:"):
		return sseAsset
	case strings.HasPrefix(attribute, "hx-"):
		return htmxAsset
	default:
		return ""
	}
}

// HeadCoverageFindings returns one message per captured page that renders an
// htmx attribute its own head loads no asset for, plus the number of pages it
// read.
//
// It reads the fixtures rather than the sources, so it answers what a browser
// receives. It is the UNDER-approximating half of the pair: a branch no fixture
// exercises is invisible here, and the walk over the sources covers that
// direction. A page loading MORE than it renders is not a finding, because the
// generator is entitled to over-approximate.
func HeadCoverageFindings(root, prefix string) ([]string, int, error) {
	loaded := regexp.MustCompile(`<script src="` + regexp.QuoteMeta(prefix) + `([^"]+)"`)

	var (
		findings []string
		pages    int
		tb       textbuf.Buffer
	)

	fsys := os.DirFS(root)

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		body, readErr := fs.ReadFile(fsys, path)
		if readErr != nil {
			return readErr
		}

		if !strings.Contains(string(body), headMarker) {
			return nil
		}

		pages++

		findings = append(findings, headFindings(&tb, path, string(body), loaded)...)

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return findings, pages, nil
}

// headFindings compares one captured page's attributes against its own head.
func headFindings(tb *textbuf.Buffer, path, body string, loaded *regexp.Regexp) []string {
	var (
		findings []string
		head     = map[string]bool{}
		reported = map[string]bool{}
	)

	for _, m := range loaded.FindAllStringSubmatch(body, -1) {
		head[m[1]] = true
	}

	for _, m := range htmxAttribute.FindAllStringSubmatch(body, -1) {
		want := assetImplementing(m[1])
		if want == "" || head[want] || reported[m[1]] {
			continue
		}

		reported[m[1]] = true

		tb.Reset()
		findings = append(findings, tb.Str(path).Str(" renders ").Str(m[1]).
			Str(", which ").Str(want).Str(" implements, and its head does not load it").String())
	}

	return findings
}
