// Design: website/AI.md -- one page per live command, carrying both halves of its help
package site

import (
	"strings"
	"testing"
)

// VALIDATES: the per-command detail page takes the summary as its lede and the
// long form as its Description body (AC-9).
//
// Before this split the lede was one hard-coded sentence naming the command,
// which told a reader nothing the heading above it had not already said, and
// the Description body carried whatever the one authored string held. The two
// halves are now declared, so each surface reads its own field. The page and
// its Markdown mirror are both asserted: a mirror that drops the long form
// publishes less than the page it mirrors.
func TestPublishedCommandDetailUsesBothForms(t *testing.T) {
	paths := commandSurfacePaths(t)
	writeCatalog(t, paths.Output, twoFormCommandCatalog)
	writeEquivalentMapping(t, paths.Output, twoFormCommandMapping)
	if _, err := renderCommandEquivalents(paths); err != nil {
		t.Fatal(err)
	}

	directory := equivalentsDirectory + "/" + commandSlug("show test") + "/"
	page := readArtifact(t, paths.Output, directory+pageIndexFile)
	mirror := readArtifact(t, paths.Output, directory+pageMirrorFile)

	if strings.Contains(page, "Command details and vendor equivalents for show test.") {
		t.Error("the page still leads with the hard-coded sentence rather than the declared summary")
	}
	lede, _, found := strings.Cut(visibleText(mainContent(t, page)), "Ze command")
	if !found {
		t.Fatalf("the page carries no Ze command card:\n%s", page)
	}
	if !strings.Contains(lede, "Show the rows of the test table.") {
		t.Errorf("the page lede is %q, want the declared summary", lede)
	}
	if strings.Contains(lede, "since the last clear") {
		t.Errorf("the page lede carries the long form: %q", lede)
	}
	if !strings.Contains(page, "since the last clear") {
		t.Error("the page never carries the long form, which is what its Description body renders")
	}

	if !strings.Contains(mirror, "Show the rows of the test table.") {
		t.Error("the mirror does not carry the declared summary")
	}
	if !strings.Contains(mirror, "since the last clear") {
		t.Error("the mirror does not carry the long form")
	}
}

// VALIDATES: a command that declares no long form publishes no empty body.
//
// 291 of the 601 command nodes declare a summary and no long form, so the
// absent half is the common case rather than the exception. An empty
// Description heading would read as "this command has nothing to explain",
// which is a claim the model never made.
func TestPublishedCommandDetailOmitsAnUndeclaredLongForm(t *testing.T) {
	paths := commandSurfacePaths(t)
	writeCatalog(t, paths.Output,
		`[{"path":"show test","description":"Show the rows of the test table.","mode":"read-only"}]`)
	writeEquivalentMapping(t, paths.Output, twoFormCommandMapping)
	if _, err := renderCommandEquivalents(paths); err != nil {
		t.Fatal(err)
	}

	directory := equivalentsDirectory + "/" + commandSlug("show test") + "/"
	page := readArtifact(t, paths.Output, directory+pageIndexFile)
	if strings.Contains(page, "<h3>Description</h3>") {
		t.Error("the page publishes a Description heading for a command that declares no long form")
	}
	if !strings.Contains(visibleText(mainContent(t, page)), "Show the rows of the test table.") {
		t.Error("the page dropped the summary along with the absent long form")
	}
}
