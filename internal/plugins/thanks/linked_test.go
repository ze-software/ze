package thanks_test

import (
	"errors"
	"runtime/debug"
	"testing"

	// The product's composition root: this test binary links the packages a
	// shipped ze links under the same feature tags, and the presence
	// declarations of every carrying package run in its init().
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/core/credits"
)

// TestEveryLinkedModuleHasACredit is the in-binary completeness proof (spec
// AC-13): every module this test binary's build information lists has a
// record in the REAL embedded data, and every one of its licence texts is
// there. Every presence declaration the linked packages made has already run
// at init, where an undeclared name panics, so reaching the test proves each
// names a declared work. Method: the test binary links the composition root
// and no third-party test helper, so its Deps are modules product code links;
// require each to be a recorded row, and its licence document to answer.
// Run under the feature tags (`./le test unit pkg path`), it links what the
// shipped flavors link.
func TestEveryLinkedModuleHasACredit(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("this test binary carries no build information")
	}
	if len(info.Deps) == 0 {
		t.Fatal("the build information lists no module, so the test proves nothing")
	}
	works, err := credits.Works()
	if unrecorded, isUnrecorded := errors.AsType[*credits.UnrecordedError](err); isUnrecorded {
		t.Fatalf("linked with no credits record: %v (declare each in website/data/dependencies.json, then ./le deps credits update)", unrecorded.Modules)
	}
	if err != nil {
		t.Fatalf("Works() error = %v", err)
	}
	listed := make(map[string]credits.Work, len(works))
	for _, work := range works {
		listed[work.Name] = work
	}
	for _, dep := range info.Deps {
		work, found := listed[dep.Path]
		if !found {
			t.Errorf("%s is linked and not listed", dep.Path)
			continue
		}
		if work.Version == "" {
			t.Errorf("%s is listed with no version", dep.Path)
		}
		document, err := credits.LicenceOf(dep.Path)
		if err != nil || len(document) == 0 {
			t.Errorf("%s: no licence document: %v", dep.Path, err)
		}
	}
}
