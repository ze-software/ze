// Design: website/AI.md -- the footer carries the page publication stamp
// Related: build.go stamps the artifact, pages.go names the pages that carry one
package site

import (
	"bytes"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// buildClock answers the publication time of one build. It is a variable so a
// test can state a time rather than read the wall clock, the same way
// liveCommandCatalog lets a test state a command catalog.
var buildClock = time.Now

// footerBlock matches the whole footer element of an authored page, from the
// indentation of its opening tag to its closing tag. Footer markup never nests
// a second footer, so the first closing tag ends the first opening one and no
// balanced-tag count is needed.
var footerBlock = regexp.MustCompile(`(?s)[ \t]*<footer>.*?</footer>`)

// publicationStamp matches the one span a page carries the stamp in.
// carryPublicationStamps finds the stamp by this markup, so footerHTML must
// keep writing the span exactly as it is written here.
var publicationStamp = regexp.MustCompile(`<span class="footer-published">[^<]*</span>`)

// publishedDisplay formats a publication time the way a reader sees it in the
// footer. The footer prints UTC, so the time is converted rather than trusted
// to already be there: a page that says UTC over a local clock reading states a
// wrong published fact.
func publishedDisplay(published time.Time) string {
	return published.UTC().Format("2 January 2006 15:04") + " UTC"
}

// pageRoot answers the relative prefix that reaches the site root from a page,
// given the page's artifact-relative HTML path. The root page gets the empty
// prefix, so a link from it spells no leading "./".
func pageRoot(htmlPath string) string {
	directory := path.Dir(filepath.ToSlash(htmlPath))
	if directory == "." || directory == "" {
		return ""
	}
	return strings.Repeat("../", strings.Count(directory, "/")+1)
}

// footerHTML renders the whole footer for a page at the given root prefix: the
// license line and the publication stamp. The footer is not a sitemap and not a
// second call to action, so nothing else belongs in it.
func footerHTML(root string, published time.Time) string {
	return "        <footer>\n" +
		"            <div class=\"footer-inner\">\n" +
		"                <div class=\"footer-bottom\">\n" +
		"                    <a href=\"" + html.EscapeString(root+"license/") + "\">Ze is AGPLv3 open source.</a>\n" +
		"                    <span class=\"footer-published\">Published " + html.EscapeString(publishedDisplay(published)) + "</span>\n" +
		"                </div>\n" +
		"            </div>\n" +
		"        </footer>"
}

// patchFooter replaces the footer of an already authored page with a freshly
// built one. It reports whether the page carried a footer to replace: a page
// without one is left as it is, and stampArtifact counts it.
func patchFooter(page []byte, root string, published time.Time) ([]byte, bool) {
	location := footerBlock.FindIndex(page)
	if location == nil {
		return page, false
	}
	replacement := footerHTML(root, published)
	patched := make([]byte, 0, len(page)-(location[1]-location[0])+len(replacement))
	patched = append(patched, page[:location[0]]...)
	patched = append(patched, replacement...)
	patched = append(patched, page[location[1]:]...)
	return patched, true
}

// stampArtifact writes this build's publication stamp into every public page of
// the artifact, and answers the pages that carry no footer to stamp.
//
// The stamp is written into every page, including the pages this build did not
// otherwise change. carryPublicationStamps then gives an unchanged page its
// previous stamp back, so the line reads as when that page last changed rather
// than as when a build last ran.
func stampArtifact(root string, published time.Time) ([]string, error) {
	pages, err := pageRegistry(root)
	if err != nil {
		return nil, err
	}
	var unstamped []string
	for _, page := range pages {
		name := filepath.Join(root, filepath.FromSlash(page.HTML))
		content, readErr := os.ReadFile(name) //nolint:gosec // a site build reads the artifact it just wrote
		if readErr != nil {
			return nil, readErr
		}
		patched, found := patchFooter(content, pageRoot(page.HTML), published)
		if !found {
			unstamped = append(unstamped, page.HTML)
			continue
		}
		if bytes.Equal(patched, content) {
			continue
		}
		if err := os.WriteFile(name, patched, 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
			return nil, err
		}
	}
	return unstamped, nil
}

// carryPublicationStamps gives every page this build did not otherwise change
// the stamp it was published with, and answers how many it carried.
//
// Without it a build that changed three pages rewrites all of them, and the
// three real changes are lost in the noise of a new timestamp on every file.
// The carried line is also the more useful one to read: it says when THIS page
// last changed, where a build stamp says only that a build ran.
//
// previous names the artifact as it was last published. An empty name means
// there was none, and every page then keeps this build's stamp.
func carryPublicationStamps(previous, next string) (int, error) {
	if previous == "" {
		return 0, nil
	}
	carried := 0
	err := filepath.WalkDir(next, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == gitMetadataDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || filepath.Ext(name) != ".html" {
			return nil
		}
		relative, err := filepath.Rel(next, name)
		if err != nil {
			return err
		}
		fresh, err := os.ReadFile(name) //nolint:gosec // a site build reads the artifact it just wrote
		if err != nil {
			return err
		}
		published, err := publishedPage(filepath.Join(previous, relative))
		if err != nil {
			return err
		}
		restored, ok := restoreStamp(published, fresh)
		if !ok {
			return nil
		}
		if err := os.WriteFile(name, restored, 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
			return err
		}
		carried++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("carry publication stamps: %w", err)
	}
	return carried, nil
}

// publishedPage reads one page of the previously published artifact. A page
// that is absent, or that is a symlink rather than a file, answers nil: it has
// no stamp to carry and is not an error.
func publishedPage(name string) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return nil, nil //nolint:nilerr // an absent previous page has no stamp to carry
	}
	return os.ReadFile(name) //nolint:gosec // a site build reads the artifact it last published
}

// restoreStamp answers the freshly built page carrying the previously published
// stamp, and whether the two pages differ by nothing else.
//
// One stamp must stand on each side, or there is no single value to carry. When
// the pages differ anywhere outside the stamp, this build changed the page and
// its new stamp is the true one.
func restoreStamp(published, fresh []byte) ([]byte, bool) {
	if published == nil || bytes.Equal(published, fresh) {
		return nil, false
	}
	stamps := publicationStamp.FindAll(published, -1)
	if len(stamps) != 1 || len(publicationStamp.FindAll(fresh, -1)) != 1 {
		return nil, false
	}
	if !bytes.Equal(publicationStamp.ReplaceAllLiteral(published, nil), publicationStamp.ReplaceAllLiteral(fresh, nil)) {
		return nil, false
	}
	return publicationStamp.ReplaceAllLiteral(fresh, stamps[0]), true
}
