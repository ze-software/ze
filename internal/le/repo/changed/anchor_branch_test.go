// VALIDATES: the no-upstream refusal names the fix for the case git reports: a
// branch gets its upstream set, a detached HEAD gets origin/HEAD set, and a
// failed branch query is reported as a git error with the command that shows
// it (review round 2, finding 4).
// PREVENTS: a git that could not answer being reported as the fact "HEAD is
// detached", sending the operator to a fix for a case that does not hold.
package repochanged

import (
	"errors"
	"strings"
	"testing"
)

// TestNoUpstreamNamesTheFixForWhatGitReported drives ResolveAnchor with a git
// whose upstream lookups name nothing and whose branch query answers a branch,
// a detached HEAD, nothing, or an error, and checks each refusal.
//
// Method: a fake query answers every rev-parse --verify with an error, so no
// upstream resolves, and answers rev-parse --abbrev-ref HEAD with the case's
// output and error. Every refusal must still be ErrNoUpstream.
func TestNoUpstreamNamesTheFixForWhatGitReported(t *testing.T) {
	broken := errors.New("git exited 128: fatal: not a git repository")
	for _, check := range []struct {
		name, out string
		err       error
		want      []string
		never     []string
	}{
		{"branch", "main\n", nil,
			[]string{"git branch --set-upstream-to=origin/main"},
			[]string{"detached", "git could not say"}},
		{"detached", "HEAD\n", nil,
			[]string{"HEAD is detached", "git remote set-head origin --auto"},
			[]string{"set-upstream-to", "git could not say"}},
		{"git error", "", broken,
			[]string{"git could not say whether HEAD is on a branch (" + broken.Error() + ")",
				"git -C /checkout rev-parse --abbrev-ref HEAD"},
			[]string{"detached", "set-upstream-to", "set-head"}},
		{"no answer", "", nil,
			[]string{"git answered no branch and no detached HEAD", "git -C /checkout rev-parse --abbrev-ref HEAD"},
			[]string{"HEAD is detached", "set-upstream-to", "set-head"}},
	} {
		t.Run(check.name, func(t *testing.T) {
			query := func(_ string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --abbrev-ref HEAD":
					return check.out, check.err
				default:
					return "", errors.New("exit status 1")
				}
			}
			_, err := ResolveAnchor("/checkout", query)
			if !errors.Is(err, ErrNoUpstream) {
				t.Fatalf("ResolveAnchor err = %v, want ErrNoUpstream", err)
			}
			for _, want := range check.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal lacks %q:\n%s", want, err)
				}
			}
			for _, never := range check.never {
				if strings.Contains(err.Error(), never) {
					t.Errorf("refusal says %q, which is not the case git reported:\n%s", never, err)
				}
			}
		})
	}
}
