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

// assetRef matches a src or href whose value is a quoted literal. templ writes
// a computed attribute as `href={ expr }`, which no static check can resolve
// and which this pattern therefore does not match.
var assetRef = regexp.MustCompile(`(?:src|href)="([^"]*)"`)

// AssetFindings returns one message per asset path a .templ under root names
// that assets cannot answer, plus the number of paths it resolved.
//
// assets is the SERVED filesystem, which is the sub-FS the handler builds, not
// the embed root. Resolving against the embed root would pass a path the file
// server answers 404 for.
//
// prefix is the URL prefix that filesystem is mounted at. A src or href naming
// some other asset tree is a finding rather than a skip. It resolves against
// nothing, and a scan that matched its own prefix alone would pass over it.
func AssetFindings(root, prefix string, assets fs.FS) ([]string, int, error) {
	var (
		findings []string
		refs     int
		tb       textbuf.Buffer
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

		for _, m := range assetRef.FindAllStringSubmatch(string(body), -1) {
			value := m[1]

			if !strings.HasPrefix(value, prefix) {
				if !strings.Contains(value, "assets/") {
					continue
				}

				tb.Reset()
				findings = append(findings, tb.Str(path).Str(" names ").Quoted(value).
					Str(", which is not under ").Quoted(prefix).
					Str("; this package serves no other asset tree").String())

				continue
			}

			refs++

			if _, statErr := fs.Stat(assets, strings.TrimPrefix(value, prefix)); statErr != nil {
				tb.Reset()
				findings = append(findings, tb.Str(path).Str(" names ").Quoted(value).
					Str(", which the served filesystem does not hold").String())
			}
		}

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return findings, refs, nil
}

// AssertAssetsResolve fails when a .templ under root names an asset the served
// filesystem cannot answer, and when the walk resolved fewer than minRefs of
// them.
//
// A page whose script or stylesheet 404s still renders, and the server reports
// success. Only the browser sees the failure, so a rename kills a live view
// silently. This assertion is what makes the rename a red test.
func AssertAssetsResolve(t *testing.T, root, prefix string, assets fs.FS, minRefs int) {
	t.Helper()

	findings, refs, err := AssetFindings(root, prefix, assets)
	if err != nil {
		t.Fatalf("scan %s for asset references: %v", root, err)
	}

	if short := Shortfall("asset references", refs, minRefs); short != "" {
		t.Errorf("scan %s %s", root, short)
	}

	for _, f := range findings {
		t.Errorf("%s", f)
	}
}
