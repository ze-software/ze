// Related: dockerkernel.go -- runDockerKernelCheck, the signal root these tests drive.

package dockerkernel

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// ctxRecorder is the check's docker process boundary: it records whether
// each command was issued under a live context, and answers like the process
// runner does for an ended one.
type ctxRecorder struct {
	live int
}

func (r *ctxRecorder) Run(ctx context.Context, _ interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	if err := ctx.Err(); err != nil {
		return interoplab.ProcessResult{}, err
	}
	r.live++
	return interoplab.ProcessResult{Stdout: "amd64\n"}, nil
}

// VALIDATES: `./le setup docker-kernel check` runs its docker commands under
// the verb's root, so the root ending (a Ctrl-C reaching the signal root)
// ends them.
// METHOD: the check runs under a canceled root against a recording double;
// a check rooted on context.Background issues its daemon query live.
// PREVENTS: a Ctrl-C that the docker client, in a process group of its own,
// never sees, leaving the check running to its bound.
func TestDockerKernelCheckRunsUnderTheVerbRoot(t *testing.T) {
	recorder := &ctxRecorder{}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	if _, code := dockerKernelCheck(canceled, interoplab.NewFixtureDocker(recorder), ""); code != 1 {
		t.Fatalf("a check under a canceled root answered %d, want 1", code)
	}
	if recorder.live != 0 {
		t.Fatalf("a check under a canceled root issued %d docker commands live, want 0", recorder.live)
	}
}
