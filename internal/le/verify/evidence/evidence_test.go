// Related: evidence.go -- the release-candidate run these tests call as a function

package verifyevidence

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/grantcheck"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/le/human"
)

// recorder is a Runner whose two outside effects are captured rather than made:
// what git was asked, and what docker was started with. It is the fixture
// Docker client's process boundary, so the argv it records is the one
// interoplab.Docker.RunOneShot renders.
type recorder struct {
	asked   [][]string
	started [][]string
	// bounds holds each docker command's timeout, index for index with started.
	bounds  []time.Duration
	missing string
	status  string
	fail    error
	code    int
	// stderr is what docker wrote to stderr before it exited.
	stderr string
	// kernelErr is what the Docker kernel check answers; kernelChecks counts
	// the checks made.
	kernelErr    error
	kernelChecks int
}

// Run answers one docker command: the container exits rec.code after writing
// rec.stderr, and a command whose context has ended is killed the way the
// system runner kills it.
func (rec *recorder) Run(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	rec.started = append(rec.started, command.Arguments)
	rec.bounds = append(rec.bounds, command.Timeout)
	if err := ctx.Err(); err != nil {
		return interoplab.ProcessResult{ExitCode: 124}, err
	}
	if command.Stderr != nil {
		io.WriteString(command.Stderr, rec.stderr) //nolint:errcheck // the fixture's stderr is a test buffer
	}
	if len(command.Arguments) > 2 && command.Arguments[1] == "rm" {
		// Docker prints the name of a container it removed; an empty answer
		// is its absent answer (interoplab forceRemoveContainer).
		return interoplab.ProcessResult{ExitCode: rec.code, Stdout: command.Arguments[len(command.Arguments)-1] + "\n"}, nil
	}
	return interoplab.ProcessResult{ExitCode: rec.code}, nil
}

// checkout is the tree every case in this file judges. Its value never
// matters; what matters is that both halves of a comparison use one.
const checkout = "/checkout"

func (rec *recorder) runner() *Runner {
	return &Runner{
		Tree:     checkout,
		Image:    "golang:1.27",
		Platform: "linux/amd64",
		Look: func(name string) error {
			if name == rec.missing {
				return errors.New("not on PATH")
			}
			return nil
		},
		Ask: func(args ...string) (string, error) {
			rec.asked = append(rec.asked, args)
			return rec.status, rec.fail
		},
		Docker:   interoplab.NewFixtureDocker(rec),
		Progress: io.Discard,
		Kernel: func(context.Context) error {
			rec.kernelChecks++
			return rec.kernelErr
		},
	}
}

// VALIDATES: the release-candidate run checks the Docker daemon's kernel once
// before it starts the container, and a refusal starts no container and
// carries the refusal.
// PREVENTS: the verify gate's ze functional tests, which run inside the
// container on the daemon's kernel, passing or failing on a kernel that lacks
// a feature Ze enrolls (owner D-4: every Docker run that runs Ze).
func TestRunChecksTheDockerKernelBeforeTheContainer(t *testing.T) {
	rec := &recorder{}
	if _, err := rec.runner().Run(t.Context()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if rec.kernelChecks != 1 || len(rec.started) != 1 {
		t.Fatalf("kernel checks = %d, containers = %d, want one of each", rec.kernelChecks, len(rec.started))
	}

	refused := &recorder{kernelErr: errors.New("the Docker daemon's kernel lacks CONFIG_XFRM_MIGRATE")}
	_, err := refused.runner().Run(t.Context())
	if err == nil || !strings.Contains(err.Error(), "CONFIG_XFRM_MIGRATE") {
		t.Errorf("err = %v, want the kernel refusal", err)
	}
	if len(refused.started) != 0 {
		t.Errorf("a container started after the kernel refused: %v", refused.started)
	}
}

// VALIDATES: a clean tree starts one container, mounting the tree read-only and
// handing bash the container script.
// PREVENTS: the gate running against the developer's own tree, which is the one
// thing this check exists to make impossible.
func TestACleanTreeStartsOneContainer(t *testing.T) {
	rec := &recorder{}

	report, err := rec.runner().Run(t.Context())
	if err != nil {
		t.Fatalf("a clean tree answered an error: %v", err)
	}
	if !report.Passed {
		t.Error("a container that exited 0 was not reported as passed")
	}
	if len(rec.started) != 1 {
		t.Fatalf("the run started %d containers, want 1", len(rec.started))
	}

	argv := rec.started[0]
	line := strings.Join(argv, " ")
	for _, want := range []string{
		"run --rm", "--label ze.lab=1", "--platform linux/amd64",
		"-v " + checkout + ":/host:ro", "golang:1.27", "bash -lc",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the docker argv does not carry %q:\n%s", want, line)
		}
	}
	// spec-lab-containers-least-privilege, site 6: the container holds
	// exactly the capabilities the caps= vocabulary names and the devices
	// runner.ContainerDevicePaths lists, never SYS_TIME, never --privileged, never a relaxed security
	// profile, a host namespace, the host network or a device cgroup rule,
	// and mounts only the checkout read-only and the three cache volumes.
	// grantcheck reads both flag spellings. The want lists are written out
	// here, independent of the runner's tables, so a stray grant fails.
	want := grantcheck.Grants{
		Capabilities: []string{"BPF", "NET_ADMIN", "NET_BIND_SERVICE", "NET_RAW"},
		Devices:      []string{"/dev/ppp"},
		Mounts: []string{
			checkout + ":/host:ro",
			"ze-apt-cache:/var/cache/apt",
			"ze-gobuild-cache:/root/.cache/go-build",
			"ze-gomod-cache:/go/pkg/mod",
		},
	}
	for _, problem := range grantcheck.Problems(grantcheck.Parse(nil, argv), want) {
		t.Errorf("the release-candidate container %s:\n%s", problem, line)
	}
	if argv[len(argv)-1] != ContainerScript {
		t.Error("the last argument is not the container script")
	}
}

// VALIDATES: a dirty tree refuses BEFORE anything is started, and names every
// path that made it dirty.
// PREVENTS: the refusal reaching the operator without the paths, which is the
// one thing they need in order to act on it.
func TestADirtyTreeStartsNothing(t *testing.T) {
	rec := &recorder{status: " M internal/a.go\n?? tmp/b.txt\n"}

	report, err := rec.runner().Run(t.Context())
	if !errors.Is(err, ErrDirtyTree) {
		t.Fatalf("a dirty tree answered %v, want ErrDirtyTree", err)
	}
	if len(rec.started) != 0 {
		t.Errorf("a dirty tree started %d containers, want 0", len(rec.started))
	}
	if len(report.Dirty) != 2 {
		t.Fatalf("the report names %v, want both dirty paths", report.Dirty)
	}
	if report.Dirty[0] != " M internal/a.go" || report.Dirty[1] != "?? tmp/b.txt" {
		t.Errorf("the report names %v, want git's own two lines", report.Dirty)
	}
	if report.Passed {
		t.Error("a dirty tree was reported as passed")
	}

	text := report.Text()
	if !strings.Contains(text, "clean git worktree") {
		t.Errorf("the rendering does not say what is required:\n%s", text)
	}
	if !strings.Contains(text, "internal/a.go") {
		t.Errorf("the rendering does not name the dirty path:\n%s", text)
	}
}

// VALIDATES: a missing external command stops the run before it asks git
// anything.
// PREVENTS: a confusing git or docker failure standing in for the plain fact
// that the tool is not installed.
func TestAMissingCommandStopsTheRun(t *testing.T) {
	for _, name := range requiredCommandNames() {
		t.Run(name, func(t *testing.T) {
			rec := &recorder{missing: name}

			_, err := rec.runner().Run(t.Context())
			if err == nil {
				t.Fatal("a missing command answered no error")
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("the error is %q, want it to name %q", err, name)
			}
			if len(rec.asked) != 0 || len(rec.started) != 0 {
				t.Error("a missing command still ran something")
			}
		})
	}
}

// VALIDATES: the container's own exit status is the command's exit status.
// PREVENTS: a flattened 1, which would tell a caller that the gate failed
// without saying which stage of it did (AC-8).
func TestTheContainerExitStatusIsTheAnswer(t *testing.T) {
	for _, code := range []int{0, 1, 2, 3, 125} {
		rec := &recorder{code: code}

		report, err := rec.runner().Run(t.Context())
		if err != nil {
			t.Fatalf("the run answered an error: %v", err)
		}
		if report.Code != code {
			t.Errorf("the report carries %d, want the container's own %d", report.Code, code)
		}
		if report.Passed != (code == 0) {
			t.Errorf("a container that exited %d reports passed=%v", code, report.Passed)
		}
	}
}

// VALIDATES: git failing to answer is an error rather than a clean tree.
// PREVENTS: the fail-open where git cannot read the tree, answers nothing, and
// the empty answer is read as "no changes".
func TestGitFailingToAnswerIsNotACleanTree(t *testing.T) {
	rec := &recorder{fail: errors.New("not a git repository")}

	if _, err := rec.runner().Run(t.Context()); err == nil {
		t.Fatal("git failing to answer was read as a clean tree")
	}
	if len(rec.started) != 0 {
		t.Error("a container was started after git failed")
	}
}

// VALIDATES: the container script clones the mounted tree and runs the verify
// gate inside the clone.
// PREVENTS: a rewrite that runs the gate over the read-only mount, which is the
// developer's own tree by another name.
func TestTheContainerScriptClonesAndVerifies(t *testing.T) {
	for _, want := range []string{
		"set -euo pipefail",
		"git clone --no-local /host /work/src",
		"cd /work/src",
		"./le verify current",
	} {
		if !strings.Contains(ContainerScript, want) {
			t.Errorf("the container script does not carry %q", want)
		}
	}
}

// VALIDATES: the answer is structured data with kebab-case keys, so | json,
// | yaml and | table each render it (AC-7).
// PREVENTS: a tool that answers finished text, which no operator can pipe.
func TestTheReportIsStructuredData(t *testing.T) {
	rec := &recorder{status: " M a.go\n", code: 1}

	report, _ := rec.runner().Run(t.Context())

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("the report does not encode: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the encoded report does not decode: %v", err)
	}
	for _, key := range []string{"image", "platform", "tree", "dirty", "passed", "code"} {
		if _, ok := back[key]; !ok {
			t.Errorf("the report has no %q key: %s", key, raw)
		}
	}
	for key := range back {
		if strings.ContainsAny(key, "_ ") || strings.ToLower(key) != key {
			t.Errorf("the key %q is not kebab-case", key)
		}
	}
}

// VALIDATES: when Docker refuses to start the container because the Docker
// host has no device node the container is given, the run answers an error
// that names the node and the host command that creates it, on a line of its
// own; any other docker failure stays the report's exit code.
// PREVENTS: a host without /dev/ppp reported as a failed gate with exit 125
// and nothing to do (spec-lab-containers-least-privilege, review round 2).
func TestAHostWithoutAContainerDeviceNamesTheModule(t *testing.T) {
	rec := &recorder{code: 125, stderr: "docker: Error response from daemon: error gathering device " +
		"information while adding custom device \"/dev/ppp\": no such file or directory.\n" +
		"Run 'docker run --help' for more information\n"}
	report, err := rec.runner().Run(t.Context())
	if err == nil {
		t.Fatal("a host without /dev/ppp answered no error")
	}
	if !strings.HasSuffix(err.Error(), "\n  sudo modprobe ppp_generic") {
		t.Errorf("refusal %q does not end with the host command on a line of its own", err)
	}
	if !strings.Contains(err.Error(), "has no /dev/ppp") {
		t.Errorf("refusal %q does not name the missing node", err)
	}
	if report.Code != 125 || report.Passed {
		t.Errorf("report = %+v, want the docker exit 125 and not passed", report)
	}

	rec = &recorder{code: 125, stderr: "docker: Error response from daemon: pull access denied\n"}
	if _, err := rec.runner().Run(t.Context()); err != nil {
		t.Errorf("an unrelated docker failure answered an error: %v", err)
	}
}

// VALIDATES: the release-candidate container runs under the four-hour bound
// (owner D-1), far past any gate run, so only a wedged daemon reaches it.
// PREVENTS: the run going unbounded again, or a bound short enough to kill a
// normal 25 to 53 minute gate.
func TestTheContainerRunsUnderTheFourHourBound(t *testing.T) {
	rec := &recorder{}
	if _, err := rec.runner().Run(t.Context()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(rec.bounds) != 1 || rec.bounds[0] != 4*time.Hour {
		t.Errorf("container bounds = %v, want one of 4h", rec.bounds)
	}
}

// VALIDATES: a run whose root context ends is an error, not a verdict, and the
// container is removed by name while the three cache volumes stay.
// PREVENTS: a Ctrl-C leaving the privileged build container running inside
// the daemon, or being reported as a failed gate.
func TestAnInterruptedRunRemovesTheContainerAndKeepsTheCaches(t *testing.T) {
	rec := &recorder{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	report, err := rec.runner().Run(ctx)
	if err == nil {
		t.Fatal("an interrupted run answered no error")
	}
	if report.Passed || report.Code != 0 {
		t.Errorf("report = %+v, an interrupted run is no verdict", report)
	}
	if len(rec.started) != 2 {
		t.Fatalf("commands = %v, want the run and its removal", rec.started)
	}
	removal := strings.Join(rec.started[1], " ")
	if !strings.Contains(removal, "rm") || !strings.Contains(removal, "ze-oneshot-fixture") {
		t.Errorf("second command = %q, want the container removed by name", removal)
	}
	for _, argv := range rec.started {
		if strings.Contains(strings.Join(argv, " "), "volume") {
			t.Errorf("a command touched a volume: %v", argv)
		}
	}
}

// VALIDATES: AC-8 for the host side of `le verify evidence release-candidate`:
// with no terminal on stdin or stdout the verb is refused with exit 2 before
// its body runs, so no docker command, kernel probe or container starts. The
// only text on stderr is the gate's refusal.
// PREVENTS: an agent's tool call starting the hour-long container run.
func TestEvidenceRefusesBeforeDockerWithoutTerminal(t *testing.T) {
	var asked []string
	restore := leaction.SetWholeTreeAdmission(func(line string) error {
		asked = append(asked, line)
		return human.Check(func(*os.File) (bool, error) { return false, nil }, os.Stdin, os.Stdout, line)
	})
	defer restore()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = write
	_, code := Answer([]string{"release-candidate"})
	os.Stderr = saved
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	captured, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	stderr := string(captured)

	const given = "./le verify evidence release-candidate"
	if code != 2 {
		t.Fatalf("release-candidate answered %d, want 2; stderr:\n%s", code, stderr)
	}
	if len(asked) != 1 || asked[0] != given {
		t.Fatalf("the gate was asked about %q, want exactly %q", asked, given)
	}
	for _, want := range []string{"a release check only Thomas starts", "\n    " + given + "\n", human.ScopedCommand} {
		if !strings.Contains(stderr, want) {
			t.Errorf("refusal does not carry %q:\n%s", want, stderr)
		}
	}
	for _, body := range []string{"docker", "container", "kernel", "dirty"} {
		if strings.Contains(stderr, body) {
			t.Errorf("refusal carries %q, so the body ran:\n%s", body, stderr)
		}
	}
}

// VALIDATES: A-7 -- the container is started with `-t`, so the inner
// `./le verify current`, itself a whole-tree verb, meets a terminal on stdin and
// stdout and passes the gate the host side already passed.
// PREVENTS: the release-candidate run being refused inside its own container.
func TestEvidenceContainerHasTerminal(t *testing.T) {
	rec := &recorder{}
	if _, err := rec.runner().Run(t.Context()); err != nil {
		t.Fatalf("a clean tree answered an error: %v", err)
	}
	if len(rec.started) != 1 {
		t.Fatalf("the run started %d containers, want 1", len(rec.started))
	}
	argv := rec.started[0]
	image := slices.Index(argv, "golang:1.27")
	if image < 0 {
		t.Fatalf("the docker argv names no image:\n%s", strings.Join(argv, " "))
	}
	if !slices.Contains(argv[:image], "-t") {
		t.Errorf("the docker argv carries no -t before the image:\n%s", strings.Join(argv, " "))
	}
}
