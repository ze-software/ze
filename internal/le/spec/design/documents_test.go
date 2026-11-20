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
