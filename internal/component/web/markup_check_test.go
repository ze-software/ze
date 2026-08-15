// Related: internal/test/markupcheck -- the scans this calls
// Related: view_typed_test.go -- the sibling guard, over what a component TAKES

package web

import (
	"io/fs"
	"testing"

	"github.com/ze-software/ze/internal/test/markupcheck"
)

// webMarkupExempt names each Go file in this package allowed to build markup.
//
// It is EMPTY, and that is the claim: every panel, page, fragment and
// out-of-band swap the web interface renders is written in a .templ file.
//
// The table is fail-closed both ways. A file that starts building markup is a
// finding. An entry that stops explaining one is a finding too, so no exemption
// outlives the markup it was written for. webMarkupFloors states the size, so a
// third way of going green, adding an entry beside markup that is really there,
// is a finding as well.
var webMarkupExempt = map[string]string{}

// webMarkupFloors is what a pass of TestNoGoFileBuildsMarkup must clear.
//
// Literals is a floor rather than the exact count, so adding a file does not
// red it. The walk read 4033 string literals on 2026-08-15, and half of that is
// a walk that has lost the tree rather than one that shrank.
//
// Exempt is EXACT. See webMarkupExempt.
var webMarkupFloors = markupcheck.Floors{Literals: 2000, Exempt: 0}

// webTemplFileFloor is the least number of .templ sources the CSP scan and the
// asset scan must read. The package holds 59, and a filter that matches none of
// them passes over everything and reports green.
const webTemplFileFloor = 40

// webAssetRefFloor is the least number of /assets/ paths the templ sources must
// name. They name 23 on 2026-08-15.
const webAssetRefFloor = 15

// TestNoGoFileBuildsMarkup requires every HTML tag in this package to live in a
// .templ file.
//
// It is the evidence for AC-7 of plan/spec-web-templ-migration.md. The
// criterion used to be a grep in a Deliverables checklist, which nothing ran.
//
// The scan reads Go string LITERALS. A tag in a comment is therefore not a
// finding, and a tag split across a Str(...) chain is. It reads the FORM of a
// tag, so `usage: set <leaf> <value>` is not a finding either. This package
// holds fourteen such strings, and a name-based check would red on every one.
//
// VALIDATES: no markup is built in Go here.
// PREVENTS: the return of the failure this migration removed. Nothing checks
// markup in a Go string. A renamed view-model field renders a blank panel and
// reports success, which is what html/template did and what templ makes a
// compile error.
func TestNoGoFileBuildsMarkup(t *testing.T) {
	markupcheck.AssertNoMarkup(t, ".", webMarkupExempt, webMarkupFloors)
}

// TestTemplAssetsResolve requires every asset a .templ names to exist in the
// filesystem the handler serves.
//
// It resolves against the SUB-filesystem NewRenderer builds, not the embed
// root, so it answers the question a browser asks. page_snapshot.templ names
// /assets/snapshot-live.js, and without this test a rename of that file leaves
// the IS-IS and OSPF live views dead with every gate green.
//
// VALIDATES: each src and href under /assets/ resolves to a served file.
// PREVENTS: a 404 that only a browser sees. The server renders the page and
// answers 200 either way.
func TestTemplAssetsResolve(t *testing.T) {
	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		t.Fatalf("sub-FS for the served assets: %v", err)
	}

	markupcheck.AssertAssetsResolve(t, ".", "/assets/", assets, webAssetRefFloor)
}
