package rfc

// VALIDATES: the priority tree over the real corpus (spec-rfc-priority-tree,
// functional test TestRFCTreeOverRealCorpus): every profile resolves, every
// summarized node names a declared aggregate, and `next` answers for every
// release profile; the two actions answer through the action table.
// PREVENTS: a profile the corpus cannot close, and an action registered with
// no working body.

import (
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestRFCTreeOverRealCorpus loads the checkout once, walks every profile and
// aggregate, then runs `rfc tree profile route-server` and `rfc next` through
// Answer, the entry point `./le` dispatches to. Each Answer loads the corpus
// again, which is minutes under -race, so the refusal of an unknown profile
// is held by the fixture test instead.
func TestRFCTreeOverRealCorpus(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	corpus, err := loadPriorityCorpus(root)
	if err != nil {
		t.Fatalf("load the corpus: %v", err)
	}
	names := append(sortedKeysOf(corpus.profiles.UseCases), sortedKeysOf(corpus.profiles.Aggregates)...)
	for _, name := range names {
		report, err := corpus.tree(name)
		if err != nil {
			t.Errorf("tree profile %s: %v", name, err)
			continue
		}
		if len(report.Rows) == 0 {
			t.Errorf("tree profile %s has no row", name)
		}
		for _, row := range report.Rows {
			if !row.Summary {
				continue
			}
			if _, held := corpus.profiles.Aggregates[row.Aggregate]; !held {
				t.Errorf("tree profile %s: %s names undeclared aggregate %q", name, row.Stem, row.Aggregate)
			}
		}
	}
	for _, name := range append([]string{""}, sortedKeysOf(corpus.profiles.UseCases)...) {
		report, err := corpus.next(name)
		if err != nil {
			t.Errorf("next profile %q: %v", name, err)
			continue
		}
		if !report.Complete && len(report.Gaps) == 0 {
			t.Errorf("next profile %q names no gap and is not complete", name)
		}
	}
	for _, args := range [][]string{{"tree", "profile", "route-server"}, {"next"}} {
		answer, code := Answer(args)
		if code != 0 {
			t.Fatalf("rfc %v exited %d", args, code)
		}
		prose, isProse := answer.(interface{ Text() string })
		if !isProse {
			t.Fatalf("rfc %v answered %T, which renders no prose", args, answer)
		}
		t.Logf("rfc %v:\n%s", args, prose.Text())
	}
}
