package commit

import (
	"strings"
	"testing"
)

// TestAnsweredNamesACommitItCouldNotRead proves that a commit whose facts git
// could not read keeps its obligation open AND names why. Method: a commit cache
// that refused the commit, asked for a review and an RFC approval; each answer
// must be "not answered", and the answering must hold one problem per ask that
// carries the cache's refusal, so the push refuses with the cause rather than
// listing an item whose reason the operator cannot act on.
func TestAnsweredNamesACommitItCouldNotRead(t *testing.T) {
	const commit = "0123456789abcdef0123456789abcdef01234567"
	cache := newCommitCache()
	cache.refused[commit] = "git show printed nothing"
	acts := personActs{attested: map[string]bool{}, approved: map[string]bool{}}
	answers := answering{root: t.TempDir(), cache: cache, acts: &acts, patchIDs: map[string]string{}}

	if answers.answered(commit, owedItemReview, "spec-x") {
		t.Fatal("a review on an unreadable commit read as answered")
	}
	if answers.answered(commit, owedItemRFCApproval, "RFC1-1-1") {
		t.Fatal("an RFC approval on an unreadable commit read as answered")
	}
	if len(answers.problems) != 2 {
		t.Fatalf("problems = %q, want one per unreadable ask", answers.problems)
	}
	for _, problem := range answers.problems {
		if !strings.Contains(problem, commit) || !strings.Contains(problem, "git show printed nothing") {
			t.Errorf("problem %q does not name the commit and the read failure", problem)
		}
	}
}
