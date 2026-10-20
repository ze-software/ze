// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: dockerkernel.go -- runDockerKernelCheck, the signal root these tests drive.

package dockerkernel

import (
	"context"
	"debug/elf"
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

// cancelingRecorder answers the daemon's architecture live, then ends the
// root: every later command is the kernel probe, and it records whether the
// probe was issued under a live context.
type cancelingRecorder struct {
	cancel context.CancelFunc
	live   int
}

func (r *cancelingRecorder) Run(ctx context.Context, _ interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	if err := ctx.Err(); err != nil {
		return interoplab.ProcessResult{}, err
	}
	r.live++
	r.cancel()
	return interoplab.ProcessResult{Stdout: "amd64\n"}, nil
}

// VALIDATES: the kernel probe itself runs under the verb's root, not only the
// architecture query before it.
// METHOD: an operator-named amd64 ELF skips the build; the double answers the
// architecture query live and then cancels the root. A probe bounded from
// context.Background issues its commands live after the cancel; one under the
// root issues none, so exactly one command ran live.
// PREVENTS: dockerKernelCheck bounding the probe from context.Background,
// which TestDockerKernelCheckRunsUnderTheVerbRoot cannot see because its root
// is canceled before the architecture query.
func TestDockerKernelProbeRunsUnderTheVerbRoot(t *testing.T) {
	named := writeELF(t, elf.EM_X86_64)
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	recorder := &cancelingRecorder{cancel: cancel}

	if _, code := dockerKernelCheck(root, interoplab.NewFixtureDocker(recorder), named); code != 1 {
		t.Fatalf("a check whose root ended after the architecture query answered %d, want 1", code)
	}
	if recorder.live != 1 {
		t.Fatalf("the check issued %d docker commands live, want 1 (the architecture query alone)", recorder.live)
	}
}

// VALIDATES: the linux ze the check builds for the daemon is built under the
// verb's root, so a Ctrl-C that lands during the cross-compile ends it.
// METHOD: no ze is named, so the check builds one; the double answers the
// architecture query live and then cancels the root, and the builder records
// whether its ctx had ended. A build closure that passes
// context.Background builds under a live ctx.
// PREVENTS: the cross-compile behind `setup docker-kernel check` running on
// in its own process group after the operator stopped the check.
func TestDockerKernelCheckBuildsUnderTheVerbRoot(t *testing.T) {
	saved := linuxZe
	t.Cleanup(func() { linuxZe = saved })
	var builds, live int
	linuxZe = func(ctx context.Context, _, _ string) (string, error) {
		builds++
		if ctx.Err() == nil {
			live++
		}
		return "", context.Canceled
	}
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	recorder := &cancelingRecorder{cancel: cancel}

	if _, code := dockerKernelCheck(root, interoplab.NewFixtureDocker(recorder), ""); code != 1 {
		t.Fatalf("a check whose root ended before its build answered %d, want 1", code)
	}
	if builds != 1 {
		t.Fatalf("the check asked for %d linux ze builds, want 1", builds)
	}
	if live != 0 {
		t.Fatal("the check built its linux ze under a live ctx after its root ended")
	}
}
