package testdeployment

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: review round 7 ISSUE 3: a proof container this run could not
// remove is reported with the cleanup command exactly once, whether the
// removal error already names it (a container still settling after a refused
// start) or not (a plain refused removal).
// METHOD: the fixture refuses `docker run -d` the way a host without
// /dev/vhost-net does, so the start leaves the unsettled mark; the first
// `docker rm -f` answers the name absent, the second is refused busy, and the
// inspect finds the container, which yields the settling removal error. The
// plain case refuses every `docker rm -f` busy with the container present.
// The L2TP report text is rendered and the cleanup command counted.
// PREVENTS: the fix line printed twice, once by the removal error and once by
// the caller that reports it.
func TestContainerRemovalNamesTheFixOnce(t *testing.T) {
	const name = "ze-vpp-evidence-1"
	refusal := "docker: Error response from daemon: error gathering device information while adding custom device \"/dev/vhost-net\": no such file or directory"
	busy := "Error response from daemon: device or resource busy"
	spec := interoplab.DetachedContainer{Name: name, Image: "ligato/vpp-base:24.02", Command: []string{"sleep", "infinity"}}

	for _, row := range []struct {
		name     string
		settling bool
	}{
		{name: "settling after a refused start", settling: true},
		{name: "plain refused removal", settling: false},
	} {
		t.Run(row.name, func(t *testing.T) {
			removals := 0
			recorder := &argvRecorder{answer: func(argv []string) (interoplab.ProcessResult, error) {
				switch argv[1] {
				case "run":
					return interoplab.ProcessResult{ExitCode: 125, Stderr: refusal}, nil
				case "rm":
					removals++
					if row.settling && removals == 1 {
						// Docker 29.9 answers an absent name with exit 0 and no output.
						return interoplab.ProcessResult{}, nil
					}
					return interoplab.ProcessResult{ExitCode: 1, Stderr: busy}, nil
				}
				// The container inspect finds the container still present.
				return interoplab.ProcessResult{Stdout: "abc\n"}, nil
			}}
			docker := interoplab.NewFixtureDocker(recorder)

			var removal string
			if row.settling {
				remove, err := startProofContainer(t.Context(), docker, spec)
				if err == nil {
					t.Fatal("a refused start answered no error")
				}
				if remove == nil {
					t.Fatal("a start docker issued answered no removal")
				}
				removal = remove()
			} else {
				removal = removeContainer(t.Context(), docker, name)
			}
			if removal == "" {
				t.Fatal("a failed removal was reported as done")
			}
			report := L2TPReport{Peer: PeerName, Container: name, Established: true, ContainerRemoval: removal}
			text := report.Text()
			if !strings.Contains(text, name+" was not removed") {
				t.Errorf("report text does not name the leftover:\n%s", text)
			}
			// The settling row is only a proof when the removal error is the
			// settling one, which carries its own fix line.
			if settled := strings.Contains(text, "was created after its start failed"); settled != row.settling {
				t.Fatalf("settling removal error = %v, want %v:\n%s", settled, row.settling, text)
			}
			if got := strings.Count(text, cleanupCommand); got != 1 {
				t.Errorf("report text names %q %d times, want once:\n%s", cleanupCommand, got, text)
			}
		})
	}
}
