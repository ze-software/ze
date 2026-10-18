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
