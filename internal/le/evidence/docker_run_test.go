// Related: docker_run.go -- the Docker evidence lifecycle these tests call

package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/le/leaction"
)

type dockerRunRecorder struct {
	commands      []dockerHostCommand
	inspectCode   int
	pullCode      int
	buildCode     int
	containerCode int
	installCode   int
	innerCode     int
	startErr      error
	moduleMount   bool
	innerBlock    bool
	innerStarted  chan struct{}
	innerStopped  chan struct{}
	signals       []os.Signal
	treeEffect    string
}

func (rec *dockerRunRecorder) runner(t *testing.T, options DockerRunOptions) *DockerRun {
	t.Helper()
	tree := dockerRunFixture(t)
	run := NewDockerRun(tree, options)
	run.Environment = []string{"PATH=/bin", "ZE_FIRST=one", "ze.dotted=a:b=c", "OTHER=no", "ZE_LAST=old"}
	run.Stdout = &strings.Builder{}
	run.Stderr = &strings.Builder{}
	run.ops = dockerRunOps{
		lookPath: func(string) error { return nil },
		run: func(_ context.Context, command dockerHostCommand) (dockerHostResult, error) {
			rec.commands = append(rec.commands, cloneDockerHostCommand(command))
			switch {
			case command.program == "go":
				if rec.buildCode == 0 {
					output := command.arguments[slices.Index(command.arguments, "-o")+1]
					if err := os.WriteFile(output, []byte("ze"), 0o755); err != nil {
						t.Fatalf("write fake ze: %v", err)
					}
				}
				return dockerHostResult{code: rec.buildCode}, nil
			case slices.Equal(command.arguments[:min(3, len(command.arguments))], []string{"image", "inspect", options.Image}):
				return dockerHostResult{code: rec.inspectCode}, nil
			case len(command.arguments) > 0 && command.arguments[0] == "pull":
				return dockerHostResult{code: rec.pullCode}, nil
			case len(command.arguments) > 0 && command.arguments[0] == "run":
				return dockerHostResult{code: rec.containerCode, stdout: "run-out\n", stderr: "run-err\n"}, nil
			case slices.Contains(command.arguments, "apk"):
				return dockerHostResult{code: rec.installCode}, nil
			case len(command.arguments) > 0 && command.arguments[0] == "rm":
				return dockerHostResult{}, nil
			default:
				t.Fatalf("unexpected command: %s %v", command.program, command.arguments)
				return dockerHostResult{}, nil
			}
		},
		start: func(command dockerHostCommand) (dockerHostProcess, error) {
			rec.commands = append(rec.commands, cloneDockerHostCommand(command))
			if rec.startErr != nil {
				return nil, rec.startErr
			}
			if rec.treeEffect != "" {
				if err := os.WriteFile(filepath.Join(tree, rec.treeEffect), []byte("inner\n"), 0o600); err != nil {
					t.Fatalf("write inner tree effect: %v", err)
				}
			}
			proc := &dockerRunFakeProcess{recorder: rec, code: rec.innerCode, done: make(chan struct{})}
			if rec.innerStarted != nil {
				close(rec.innerStarted)
			}
			if !rec.innerBlock {
				close(proc.done)
			}
			return proc, nil
		},
		modulesDir: func() bool { return rec.moduleMount },
		pid:        func() int { return 42 },
	}
	return run
}

type dockerRunFakeProcess struct {
	recorder *dockerRunRecorder
	code     int
	done     chan struct{}
	once     sync.Once
}

func (proc *dockerRunFakeProcess) Wait() (dockerHostResult, error) {
	<-proc.done
	return dockerHostResult{code: proc.code}, nil
}

func (proc *dockerRunFakeProcess) Signal(caught os.Signal) error {
	proc.recorder.signals = append(proc.recorder.signals, caught)
	proc.once.Do(func() {
		close(proc.done)
		if proc.recorder.innerStopped != nil {
			close(proc.recorder.innerStopped)
		}
	})
	return nil
}

func cloneDockerHostCommand(command dockerHostCommand) dockerHostCommand {
	command.arguments = slices.Clone(command.arguments)
	command.environment = slices.Clone(command.environment)
	return command
}

func dockerRunFixture(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "feature-gates.txt"), []byte("ze_bgp default\nze_l2tp default\n"), 0o600); err != nil {
		t.Fatalf("write feature gates: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tree, "scripts", "evidence"), 0o755); err != nil {
		t.Fatalf("make evidence directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tree, "scripts", "evidence", "proof.py"), []byte("print('proof')\n"), 0o600); err != nil {
		t.Fatalf("write evidence script: %v", err)
	}
	return tree
}

func dockerRunOptions() DockerRunOptions {
	return DockerRunOptions{
		Script: "scripts/evidence/proof.py", Packages: []string{"iproute2", "python3"},
		Environment: []string{"ZE_LAST=new", "PUNCT=x:y=z,[]"}, Image: DockerEvidenceImageDefault,
		Platform: DockerEvidencePlatformDefault, Goarch: DockerEvidenceGoarchDefault,
	}
}

// VALIDATES: the action boundary requires script and decodes the two optional ordered values.
// PREVENTS: free positional values or malformed JSON reaching Docker.
func TestDockerRunGrammarAndJSONBoundaries(t *testing.T) {
	environ := []string{
		"ZE_DOCKER_EVIDENCE_IMAGE=custom:image", "ZE_DOCKER_EVIDENCE_PLATFORM=linux/arm64",
		"ZE_DOCKER_EVIDENCE_GOARCH=arm64",
	}
	got, err := ParseDockerRunArguments(leaction.Arguments{
		"script": "scripts/evidence/proof.py", "packages": "  iproute2   ppp  ",
		"environment": `["A=one:two","A=last","ze.dot=x=y,[]"]`,
	}, environ)
	if err != nil {
		t.Fatalf("parse valid arguments: %v", err)
	}
	if got.Image != "custom:image" || got.Platform != "linux/arm64" || got.Goarch != "arm64" {
		t.Fatalf("environment defaults = %#v", got)
	}
	if !slices.Equal(got.Packages, []string{"iproute2", "ppp"}) {
		t.Fatalf("packages = %v", got.Packages)
	}
	wantEnv := []string{"A=one:two", "A=last", "ze.dot=x=y,[]"}
	if !slices.Equal(got.Environment, wantEnv) {
		t.Fatalf("environment = %v, want %v", got.Environment, wantEnv)
	}
	defaults, err := ParseDockerRunArguments(leaction.Arguments{"script": "proof.py"}, nil)
	if err != nil {
		t.Fatalf("parse default arguments: %v", err)
	}
	if defaults.Image != DockerEvidenceImageDefault ||
		defaults.Platform != DockerEvidencePlatformDefault ||
		defaults.Goarch != DockerEvidenceGoarchDefault {
		t.Fatalf("defaults = %#v", defaults)
	}

	bad := []leaction.Arguments{
		{},
		{"script": "proof.py", "packages": " \t "},
		{"script": "proof.py", "environment": `null`},
		{"script": "proof.py", "environment": `{}`},
		{"script": "proof.py", "environment": `["NO-EQUALS"]`},
		{"script": "proof.py", "environment": `["=missing-key"]`},
	}
	for _, args := range bad {
		if _, parseErr := ParseDockerRunArguments(args, nil); parseErr == nil {
			t.Errorf("accepted invalid arguments: %#v", args)
		}
	}
}

// VALIDATES: missing and outside-tree scripts fail before a process executes.
// PREVENTS: image pulls or builds before a path error the producer can determine locally.
func TestDockerRunValidatesTheScriptBeforeEffects(t *testing.T) {
	tree := dockerRunFixture(t)
	outside := filepath.Join(t.TempDir(), "proof.py")
	if err := os.WriteFile(outside, []byte("pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{"missing.py", outside} {
		effects := 0
		run := NewDockerRun(tree, dockerRunOptions())
		run.Options.Script = script
		run.ops.lookPath = func(string) error { return nil }
		run.ops.run = func(context.Context, dockerHostCommand) (dockerHostResult, error) {
			effects++
			return dockerHostResult{}, nil
		}
		if _, err := run.Execute(context.Background()); err == nil {
			t.Errorf("script %q produced no error", script)
		}
		if effects != 0 {
			t.Errorf("script %q caused %d effects", script, effects)
		}
	}
}

// VALIDATES: the cached-image path inspects once and never pulls.
// PREVENTS: every local run requiring network access despite a cached image.
func TestDockerRunUsesTheCachedImage(t *testing.T) {
	rec := &dockerRunRecorder{}
	run := rec.runner(t, dockerRunOptions())
	report, err := run.Execute(context.Background())
	if err != nil {
		t.Fatalf("run evidence: %v", err)
	}
	if report.Verdict != DockerRunVerdictPass || report.Code != 0 || !report.Cleanup {
		t.Fatalf("report = %#v", report)
	}
	if got := dockerCommandCount(rec.commands, "pull"); got != 0 {
		t.Fatalf("pull count = %d, want 0", got)
	}
	if len(rec.commands) != 6 {
		t.Fatalf("absolute command count = %d, want 6", len(rec.commands))
	}
	if report.Plan.ModuleMounted || slices.Contains(rec.commands[2].arguments, "/lib/modules:/lib/modules:ro") {
		t.Fatal("a host without /lib/modules still mounted it")
	}
}

// VALIDATES: an inspect miss pulls exactly once before the build.
// PREVENTS: a missing image reaching docker run or a redundant pull after success.
func TestDockerRunPullsAnUncachedImage(t *testing.T) {
	rec := &dockerRunRecorder{inspectCode: 1}
	run := rec.runner(t, dockerRunOptions())
	if _, err := run.Execute(context.Background()); err != nil {
		t.Fatalf("run evidence: %v", err)
	}
	if got := dockerCommandCount(rec.commands, "pull"); got != 1 {
		t.Fatalf("pull count = %d, want 1", got)
	}
	if rec.commands[0].program != "docker" || rec.commands[1].arguments[0] != "pull" || rec.commands[2].program != "go" {
		t.Fatalf("inspect/pull/build order = %#v", rec.commands[:3])
	}
}

// VALIDATES: the build uses the complete derived tags and Python's environment replacement rules.
// PREVENTS: a Linux ze without a feature gate, the chosen architecture, or CGO disabled.
func TestDockerRunBuildArgsTagsAndEnvironment(t *testing.T) {
	rec := &dockerRunRecorder{}
	run := rec.runner(t, dockerRunOptions())
	run.Environment = []string{"PATH=/bin", "GOARCH=old", "GOCACHE=/cache", "CGO_ENABLED=1"}
	report, err := run.Execute(context.Background())
	if err != nil {
		t.Fatalf("run evidence: %v", err)
	}
	if report.Plan.BuildTags != "ze_core ze_distro ze_bgp ze_l2tp" {
		t.Fatalf("tags = %q", report.Plan.BuildTags)
	}
	build := rec.commands[1]
	wantArgv := []string{"build", "-tags", report.Plan.BuildTags, "-o", filepath.Join(report.Plan.Tree, filepath.FromSlash(report.Plan.ZeBinary)), "./cmd/ze"}
	if build.program != "go" || !slices.Equal(build.arguments, wantArgv) {
		t.Fatalf("build = %s %v", build.program, build.arguments)
	}
	wantEnv := []string{"PATH=/bin", "GOARCH=amd64", "GOCACHE=/cache", "CGO_ENABLED=0", "GOOS=linux"}
	if !slices.Equal(build.environment, wantEnv) {
		t.Fatalf("build environment = %v, want %v", build.environment, wantEnv)
	}
}

// VALIDATES: module mounts, package order, forwarded keys, and explicit duplicates reach exact argv slots.
// PREVENTS: dotted keys passing through dash, environment reordering, or package sorting.
func TestDockerRunContainerMountPackagesAndEnvironmentOrder(t *testing.T) {
	rec := &dockerRunRecorder{moduleMount: true}
	run := rec.runner(t, dockerRunOptions())
	report, err := run.Execute(context.Background())
	if err != nil {
		t.Fatalf("run evidence: %v", err)
	}
	container := rec.commands[2].arguments
	wantContainer := []string{"run", "--rm", "--detach", "--privileged", "--platform", "linux/amd64", "--name", "ze-evidence-42", "-v", mountPair(report.Plan.Tree, "/src"), "-v", "/lib/modules:/lib/modules:ro", "-w", "/src", "alpine:3.20", "sleep", "infinity"}
	if !slices.Equal(container, wantContainer) {
		t.Fatalf("container argv = %v, want %v", container, wantContainer)
	}
	wantInstall := []string{"exec", "ze-evidence-42", "apk", "add", "--no-cache", "iproute2", "python3"}
	if !slices.Equal(rec.commands[3].arguments, wantInstall) {
		t.Fatalf("install argv = %v", rec.commands[3].arguments)
	}
	wantForwarded := []string{
		"ZE_EVIDENCE_ZE_BINARY=/src/tmp/evidence/bin/ze-linux-amd64", "ZE_FIRST=one",
		"ze.dotted=a:b=c", "ZE_LAST=old", "ZE_LAST=new", "PUNCT=x:y=z,[]",
	}
	if !slices.Equal(report.Plan.Environment, wantForwarded) {
		t.Fatalf("forwarded environment = %v, want %v", report.Plan.Environment, wantForwarded)
	}
	wantExec := []string{"exec"}
	for _, item := range wantForwarded {
		wantExec = append(wantExec, "--env", item)
	}
	wantExec = append(wantExec, "ze-evidence-42", "python3", "/src/scripts/evidence/proof.py")
	if !slices.Equal(rec.commands[4].arguments, wantExec) {
		t.Fatalf("inner argv = %v, want %v", rec.commands[4].arguments, wantExec)
	}
}

// VALIDATES: the inner exit is report data and the action code, while cleanup runs once.
// PREVENTS: flattening an inner exit or removing the container twice.
func TestDockerRunInnerExitAndTreeEffect(t *testing.T) {
	rec := &dockerRunRecorder{innerCode: 7, treeEffect: "inner-result.txt"}
	run := rec.runner(t, dockerRunOptions())
	report, err := run.Execute(context.Background())
	if err != nil {
		t.Fatalf("run evidence: %v", err)
	}
	if report.Verdict != DockerRunVerdictFail || report.InnerExitCode != 7 || report.Code != 7 {
		t.Fatalf("report = %#v", report)
	}
	if dockerRunExitCode(report) != 7 || dockerCommandCount(rec.commands, "rm") != 1 {
		t.Fatalf("exit/cleanup = %d/%d", dockerRunExitCode(report), dockerCommandCount(rec.commands, "rm"))
	}
	data, readErr := os.ReadFile(filepath.Join(report.Plan.Tree, "inner-result.txt"))
	if readErr != nil || string(data) != "inner\n" {
		t.Fatalf("tree effect = %q, %v", data, readErr)
	}
}

// VALIDATES: SIGTERM removes the container, signals the active docker exec, and answers 143.
// PREVENTS: an orphan container or a generic exit 1 on the producer's TERM path.
func TestDockerRunSIGTERMCleanupAndExit(t *testing.T) {
	rec := &dockerRunRecorder{innerBlock: true, innerStarted: make(chan struct{}), innerStopped: make(chan struct{})}
	run := rec.runner(t, dockerRunOptions())
	signals := make(chan os.Signal, 1)
	result := make(chan struct {
		answer any
		code   int
	}, 1)
	go func() {
		answer, code := runDockerAction(run, signals)
		result <- struct {
			answer any
			code   int
		}{answer, code}
	}()
	<-rec.innerStarted
	signals <- syscall.SIGTERM
	got := <-result
	report, ok := got.answer.(DockerRunReport)
	if !ok {
		t.Fatalf("signal answer type = %T", got.answer)
	}
	if got.code != 143 || report.Code != 143 || report.Signal != int(syscall.SIGTERM) || report.Verdict != DockerRunVerdictSignal {
		t.Fatalf("signal report/code = %#v/%d", report, got.code)
	}
	if dockerCommandCount(rec.commands, "rm") != 2 || !slices.Equal(rec.signals, []os.Signal{syscall.SIGTERM}) {
		t.Fatalf("cleanup/signals = %d/%v", dockerCommandCount(rec.commands, "rm"), rec.signals)
	}
}

// VALIDATES: a started container is removed after install or inner-start failure, but never after start fails.
// PREVENTS: leaked containers and cleanup commands aimed at containers that never existed.
func TestDockerRunCleanupBoundaries(t *testing.T) {
	cases := []struct {
		name         string
		recorder     dockerRunRecorder
		cleanupCount int
	}{
		{name: "container start", recorder: dockerRunRecorder{containerCode: 1}, cleanupCount: 0},
		{name: "package install", recorder: dockerRunRecorder{installCode: 1}, cleanupCount: 1},
		{name: "inner start", recorder: dockerRunRecorder{startErr: errors.New("start refused")}, cleanupCount: 1},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			run := one.recorder.runner(t, dockerRunOptions())
			report, err := run.Execute(context.Background())
			if err == nil {
				t.Fatal("operating failure returned no error")
			}
			if got := dockerCommandCount(one.recorder.commands, "rm"); got != one.cleanupCount {
				t.Fatalf("cleanup count = %d, want %d (report %#v)", got, one.cleanupCount, report)
			}
		})
	}
}

// VALIDATES: command availability, pull, build, and container-start failures stop at their owner.
// PREVENTS: an operating failure continuing into a later effect or removing an unstarted container.
func TestDockerRunOperatingFailureBoundaries(t *testing.T) {
	t.Run("missing docker", func(t *testing.T) {
		rec := &dockerRunRecorder{}
		run := rec.runner(t, dockerRunOptions())
		run.ops.lookPath = func(name string) error {
			if name == "docker" {
				return errors.New("missing docker")
			}
			return nil
		}
		if _, err := run.Execute(context.Background()); err == nil {
			t.Fatal("missing docker returned no error")
		}
		if len(rec.commands) != 0 {
			t.Fatalf("missing docker ran %d commands", len(rec.commands))
		}
	})
	t.Run("missing go after image inspection", func(t *testing.T) {
		rec := &dockerRunRecorder{}
		run := rec.runner(t, dockerRunOptions())
		run.ops.lookPath = func(name string) error {
			if name == "go" {
				return errors.New("missing go")
			}
			return nil
		}
		if _, err := run.Execute(context.Background()); err == nil {
			t.Fatal("missing go returned no error")
		}
		if len(rec.commands) != 1 || rec.commands[0].arguments[0] != "image" {
			t.Fatalf("missing go commands = %#v", rec.commands)
		}
	})
	for _, one := range []struct {
		name     string
		recorder dockerRunRecorder
		count    int
	}{
		{name: "pull", recorder: dockerRunRecorder{inspectCode: 1, pullCode: 2}, count: 2},
		{name: "build", recorder: dockerRunRecorder{buildCode: 2}, count: 2},
		{name: "container", recorder: dockerRunRecorder{containerCode: 2}, count: 3},
	} {
		t.Run(one.name, func(t *testing.T) {
			run := one.recorder.runner(t, dockerRunOptions())
			if _, err := run.Execute(context.Background()); err == nil {
				t.Fatal("operating failure returned no error")
			}
			if len(one.recorder.commands) != one.count {
				t.Fatalf("command count = %d, want %d", len(one.recorder.commands), one.count)
			}
			if dockerCommandCount(one.recorder.commands, "rm") != 0 {
				t.Fatal("an unstarted container was removed")
			}
			if one.name == "container" {
				got := run.Stderr.(*strings.Builder).String()
				if got != "run-out\nrun-err\n" {
					t.Fatalf("start failure stream = %q", got)
				}
			}
		})
	}
}

// VALIDATES: the report uses kebab-case structured data and refuses an unspecified verdict.
// PREVENTS: a zero verdict rendering as a plausible success or pipe-incompatible prose.
func TestDockerRunReportShapeAndZeroVerdict(t *testing.T) {
	if _, err := json.Marshal(DockerRunReport{}); err == nil {
		t.Fatal("an unspecified verdict marshaled without error")
	}
	report := DockerRunReport{
		Verdict: DockerRunVerdictFail, Plan: DockerRunPlan{
			Tree: "/tree", Script: "proof.py", Packages: []string{}, Environment: []string{},
			Image: "image", Platform: "platform", Goarch: "arch", ZeBinary: "ze",
			Container: "container", BuildTags: "tags", Commands: []DockerRunCommand{},
		}, InnerExitCode: 3, Code: 3, Cleanup: true,
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	wantKeys := []string{"cleanup", "code", "inner-exit-code", "plan", "verdict"}
	keys := make([]string, 0, len(got))
	for key := range got {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("report keys = %v, want %v", keys, wantKeys)
	}
}

func dockerCommandCount(commands []dockerHostCommand, firstArgument string) int {
	count := 0
	for _, command := range commands {
		if len(command.arguments) > 0 && command.arguments[0] == firstArgument {
			count++
		}
	}
	return count
}
