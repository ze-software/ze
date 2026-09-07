// VALIDATES: the worktree lifecycle refuses a half-named cut, and gives two
// pieces of one commit two identities in the job registry.
// PREVENTS: a second piece attaching to the first piece's verdict and recording
// stages as proven that no run started.
package verify

import (
	"slices"
	"testing"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// TestTwoPiecesOfOneCommitAreTwoJobs holds the admission identity apart per
// piece. The registry shares a running holder's verdict with a run whose argv
// matches, so an argv that named only the commit would hand piece two the exit
// code of piece one.
func TestTwoPiecesOfOneCommitAreTwoJobs(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	whole := worktreeArgv(sha, verifyengine.Uncut())
	first := worktreeArgv(sha, verifyengine.Part{Index: 1, Of: 6})
	second := worktreeArgv(sha, verifyengine.Part{Index: 2, Of: 6})

	if slices.Equal(first, second) {
		t.Errorf("part 1 and part 2 of %s share one argv %v", sha[:12], first)
	}
	if slices.Equal(first, whole) {
		t.Errorf("part 1 of 6 and the uncut run share one argv %v", whole)
	}
	if !slices.Equal(whole, []string{"le", "verify", actionName, "commit", sha}) {
		t.Errorf("the uncut run's argv moved to %v", whole)
	}
}

// TestACutNeedsBothNumbers holds the pair rule at the lifecycle boundary: one
// number alone cannot say which stages run, and a guess would report the wrong
// piece proven.
func TestACutNeedsBothNumbers(t *testing.T) {
	uncut, err := (Options{}).part()
	if err != nil || uncut != verifyengine.Uncut() {
		t.Errorf("no numbers answered (%#v, %v), want the uncut run", uncut, err)
	}
	cut, err := (Options{Part: 3, Parts: 6}).part()
	if err != nil || cut != (verifyengine.Part{Index: 3, Of: 6}) {
		t.Errorf("part 3 of 6 answered (%#v, %v)", cut, err)
	}
	for name, options := range map[string]Options{
		"an index with no count": {Part: 3},
		"a count with no index":  {Parts: 6},
		"an index past the cut":  {Part: 7, Parts: 6},
		"a negative index":       {Part: -1, Parts: 6},
	} {
		if _, err := options.part(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
