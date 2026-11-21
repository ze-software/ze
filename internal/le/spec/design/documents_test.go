// VALIDATES: `le spec design documents spec <path>` derives every design page
// a spec's files link and every decision row those pages carry
// (spec-design-needs-owner-approval, AC-13).
// PREVENTS: a spec that names a file and never gives the decision on that
// file's design page a verdict.
package specdesign_test

import (
	"strings"
	"testing"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// TestDesignDocumentsDerivation is the wiring row "`./le spec design documents
// spec <path>`". Method: run the registered command over a spec naming one
// code file; the answer lists the file's // Design: page, its decision row and
// the covering door, each with what derived it.
func TestDesignDocumentsDerivation(t *testing.T) {
	widgetRoot(t)
	answer, code := specdesign.Answer([]string{"documents", "spec", widgetSpec})
	if code != 0 {
		t.Fatalf("documents answered (%#v, %d), want exit 0", answer, code)
	}
	requireContains(t, "the derivation", answerText(t, answer), widgetPage, widgetDecision, widgetDoor, widgetCode)
}

// TestDesignDocumentsTakeEveryPageOfABullet is the defect the design judge
// met: a Files to Modify bullet naming several pages derived only the first.
// Method: the widget spec gains a bullet naming two design pages whose
// description cites another, and a bullet naming a third page whose
// parenthesis cites it too; the derivation lists the three named pages as
// pages the spec modifies, and never the cited one.
func TestDesignDocumentsTakeEveryPageOfABullet(t *testing.T) {
	const (
		firstPage  = "docs/architecture/first.md"
		secondPage = "docs/architecture/second.md"
		thirdPage  = "docs/architecture/third.md"
		citedPage  = "docs/architecture/cited.md"
	)
	root := widgetRoot(t)
	codeBullet := "- `" + widgetCode + "` - render in the surface\n"
	pagesBullet := "- `" + firstPage + "`, `" + secondPage + "` - as `" + citedPage + "` says\n" +
		"- `" + thirdPage + "` (Design: `" + citedPage + "`) - the third\n"
	writeTree(t, root, map[string]string{
		firstPage:  "# First\n",
		secondPage: "# Second\n",
		thirdPage:  "# Third\n",
		citedPage:  "# Cited\n",
		widgetSpec: strings.Replace(widgetSpecText, codeBullet, codeBullet+pagesBullet, 1),
	})
	answer, code := specdesign.Answer([]string{"documents", "spec", widgetSpec})
	if code != 0 {
		t.Fatalf("documents answered (%#v, %d), want exit 0", answer, code)
	}
	text := answerText(t, answer)
	requireContains(t, "the derivation", text, firstPage, secondPage, thirdPage, "a page this spec modifies")
	if strings.Contains(text, citedPage) {
		t.Errorf("the derivation lists %s, which only a bullet's description or parenthesis cites:\n%s", citedPage, text)
	}
}

// TestDesignDocumentsReadTheWholeHeader pins that the derivation reads the
// whole header, as `./le doc consistency` does, and not a fixed count of
// lines. Method: the code file's `// Design:` line is line 27, after a 26-line
// package comment, the shape of internal/component/command/registry/registry.go;
// the answer still lists the page as that file's // Design: page, and its
// decision row.
func TestDesignDocumentsReadTheWholeHeader(t *testing.T) {
	root := widgetRoot(t)
	deep := strings.Repeat("// Package widget is described at length.\n", 25) + "//\n" +
		"// Design: " + widgetPage + " -- the widget\npackage widget\n"
	writeTree(t, root, map[string]string{widgetCode: deep})
	answer, code := specdesign.Answer([]string{"documents", "spec", widgetSpec})
	if code != 0 {
		t.Fatalf("documents answered (%#v, %d), want exit 0", answer, code)
	}
	requireContains(t, "the derivation", answerText(t, answer), widgetPage, widgetDecision, "the // Design: page of "+widgetCode)
}
