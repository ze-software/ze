// Design: docs/contributing/spec-workflow.md -- the roadmap Profiles column.

package rfccorpus

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/spec/roadmap"
)

// TestRoadmapProfilesOverRealCorpus holds AC-16 over the committed tree: the
// roadmap of HEAD reads HEAD's own rfc/ corpus, and every spec that declares
// an `RFC gaps` item is placed in at least one profile with no unjudged or
// unknown-gap diagnostic. Method: Collect over this checkout's HEAD; the time
// the corpus read adds is logged, because it is paid on every roadmap run.
func TestRoadmapProfilesOverRealCorpus(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	snapshot, err := roadmap.Collect(t.Context(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("roadmap of HEAD with its rfc/ corpus: %v", time.Since(start))
	for _, item := range snapshot.Items {
		if len(item.RFCGaps) == 0 {
			continue
		}
		t.Logf("%s: gaps %v, profiles %v", item.Name, item.RFCGaps, item.Profiles)
		for _, diagnostic := range item.Diagnostics {
			if strings.Contains(diagnostic, "RFC gaps") {
				t.Errorf("%s: %s", item.Path, diagnostic)
			}
		}
	}
}
