package testdeployment

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: AC-8 for the deployment verbs: SIGINT or SIGTERM delivered while a
// container-starting verb runs cancels the context the verb was handed.
// METHOD: the test holds its own subscription to both signals, so a root that
// failed to cancel leaves the process alive and the run answers "not canceled"
// instead of the default action killing the test binary.
// PREVENTS: a verb rooted on context.Background, whose Ctrl-C kills the client
// and leaves its `sleep infinity` container running.
func TestDeploymentVerbsCancelOnSignal(t *testing.T) {
	held := make(chan os.Signal, 2)
	signal.Notify(held, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(held)

	for _, delivered := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		run := signalRooted(func(ctx context.Context, _ string) (any, int) {
			if err := syscall.Kill(os.Getpid(), delivered); err != nil {
				t.Errorf("signal self: %v", err)
				return "not signaled", 0
			}
			select {
			case <-ctx.Done():
				return "canceled", 1
			case <-time.After(5 * time.Second):
				return "not canceled", 0
			}
		})
		payload, code := run()
		if payload != "canceled" || code != 1 {
			t.Errorf("%v: payload %v code %d, want the root context canceled", delivered, payload, code)
		}
	}
}

// VALIDATES: AC-8, AC-11: the proof's container is removed on a context the
// verb's cancellation does not reach, and a removal that fails is named in
// the report text with the command that removes it, and fails the verb.
// PREVENTS: a SIGINT that skips the removal, or a removal error discarded.
func TestContainerRemovalSurvivesCancelAndIsReported(t *testing.T) {
	recorder := &argvRecorder{}
	docker := interoplab.NewFixtureDocker(recorder)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if removal := removeContainer(canceled, docker, "ze-l2tp-evidence-1"); removal != "" {
		t.Fatalf("removal under a canceled root failed: %s", removal)
	}
	if want := []string{interoplab.DockerExecutable, "rm", "-f", "ze-l2tp-evidence-1"}; !slices.Equal(recorder.commands[0], want) {
		t.Fatalf("removal argv = %v, want %v", recorder.commands[0], want)
	}

	recorder.answer = func(argv []string) (interoplab.ProcessResult, error) {
		if argv[1] == "rm" {
			return interoplab.ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: device or resource busy"}, nil
		}
		return interoplab.ProcessResult{}, errors.New("daemon not answering")
	}
	removal := removeContainer(t.Context(), docker, "ze-l2tp-evidence-1")
	if removal == "" {
		t.Fatal("a failed removal was reported as done")
	}
	report := L2TPReport{Peer: PeerName, Container: "ze-l2tp-evidence-1", Established: true, ContainerRemoval: removal}
	text := report.Text()
	if !strings.Contains(text, "ze-l2tp-evidence-1 was not removed") {
		t.Errorf("report text does not name the leftover:\n%s", text)
	}
	if !strings.Contains(text, "./le test integration cleanup") {
		t.Errorf("report text does not name the command that removes it:\n%s", text)
	}
	if code := l2tpExitCode(report); code != 1 {
		t.Errorf("a leftover container exited %d, want 1", code)
	}
}

// VALIDATES: G-2 for the deployment proofs: a container start that docker
// issued and that failed still answers the removal the proof defers, and the
// removal names the container; a start the daemon refused as name-taken, or a
// spec that never rendered, answers none.
// METHOD: the fixture answers `docker run -d` with the daemon's refusal of a
// missing device, the refusal a host without /dev/vhost-net gives vpp-test
// after the daemon has created the container (observed 2026-10-19, left
// `ze-vpp-evidence-2578373` in Created state), then with the name-taken
// refusal.
// PREVENTS: a proof that registers its removal only after a successful start,
// so a refused start or a Ctrl-C during `docker run -d` leaves the created
// container for the next sweep.
func TestFailedProofStartStillRemovesItsContainer(t *testing.T) {
	const name = "ze-vpp-evidence-1"
	spec := interoplab.DetachedContainer{Name: name, Image: "ligato/vpp-base:24.02", Command: []string{"sleep", "infinity"}}
	refusal := "docker: Error response from daemon: error gathering device information while adding custom device \"/dev/vhost-net\": no such file or directory"
	recorder := &argvRecorder{answer: func(argv []string) (interoplab.ProcessResult, error) {
		if argv[1] == "run" {
			return interoplab.ProcessResult{ExitCode: 127, Stderr: refusal}, nil
		}
		return interoplab.ProcessResult{}, nil
	}}
	docker := interoplab.NewFixtureDocker(recorder)

	remove, err := startProofContainer(t.Context(), docker, spec)
	if err == nil {
		t.Fatal("a refused start answered no error")
	}
	if remove == nil {
		t.Fatal("a start docker issued answered no removal, so its created container is left behind")
	}
	if removal := remove(); removal != "" {
		t.Fatalf("removal failed: %s", removal)
	}
	if want := []string{interoplab.DockerExecutable, "rm", "-f", name}; !slices.Equal(recorder.commands[len(recorder.commands)-1], want) {
		t.Fatalf("removal argv = %v, want %v", recorder.commands[len(recorder.commands)-1], want)
	}

	refusal = "docker: Error response from daemon: Conflict. The container name \"/" + name + "\" is already in use by container \"abc\"."
	remove, err = startProofContainer(t.Context(), docker, spec)
	if err == nil {
		t.Fatal("a name-taken start answered no error")
	}
	if remove != nil {
		t.Fatal("a name another container holds answered a removal of that container")
	}

	remove, err = startProofContainer(t.Context(), docker, interoplab.DetachedContainer{Name: name})
	if err == nil {
		t.Fatal("a spec with no image answered no error")
	}
	if remove != nil {
		t.Fatal("a spec docker never ran answered a removal")
	}
}
