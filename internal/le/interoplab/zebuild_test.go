package interoplab

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/featuretags"
	"github.com/ze-software/ze/internal/le/lepath"
)

// buildRunner answers the two commands a staging preflight issues: the daemon
// architecture query, and one cross-compile per declared binary. It answers the
// query with architecture and every build with success, so a test reads what
// the producer ASKED FOR rather than what a compiler did.
type buildRunner struct {
	recordingRunner
	architecture string
}

func newBuildRunner(architecture string) *buildRunner {
	runner := &buildRunner{architecture: architecture}
	runner.run = func(command processCommand) (processResult, error) {
		if len(command.Arguments) > 1 && command.Arguments[1] == "version" {
			return processResult{Stdout: runner.architecture + "\n"}, nil
		}
		return processResult{}, nil
	}
	return runner
}

// builds answers every cross-compile the runner recorded, dropping the daemon
// architecture query.
func (r *buildRunner) builds() []processCommand {
	out := make([]processCommand, 0, len(r.commands))
	for _, command := range r.commands {
		if len(command.Arguments) > 0 && command.Arguments[0] == "go" {
			out = append(out, command)
		}
	}
	return out
}

// tagsOf answers the -tags value of one recorded build command.
func tagsOf(t *testing.T, command processCommand) string {
	t.Helper()
	for index, argument := range command.Arguments {
		if argument == "-tags" && index+1 < len(command.Arguments) {
			return command.Arguments[index+1]
		}
	}
	t.Fatalf("the build command carries no -tags: %v", command.Arguments)
	return ""
}

// outputOf answers the -o value of one recorded build command.
func outputOf(t *testing.T, command processCommand) string {
	t.Helper()
	for index, argument := range command.Arguments {
		if argument == "-o" && index+1 < len(command.Arguments) {
			return command.Arguments[index+1]
		}
	}
	t.Fatalf("the build command carries no -o: %v", command.Arguments)
	return ""
}

// VALIDATES: every binary a lab declares gets its own cross-compile, with its own tag base and its own output path.
// PREVENTS: a lab that declares two personalities shipping an image with one of them, which is how a scenario meets "ze-test: not found".
func TestPreflightBuildsEveryDeclaredBinary(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	declared := []LabBinary{
		{Name: "ze", Base: featuretags.DaemonBase, Output: "test/interop/ze-linux"},
		{Name: "ze-test", Base: "ze_test", Output: "test/interop/ze-test-linux"},
	}

	runner := newBuildRunner("amd64")
	if err := StageBinaries(root, false, declared...)(context.Background(), newDocker(runner)); err != nil {
		t.Fatalf("StageBinaries returned an error: %v", err)
	}

	builds := runner.builds()
	if len(builds) != len(declared) {
		t.Fatalf("the preflight ran %d builds, want %d", len(builds), len(declared))
	}
	for index, binary := range declared {
		wantTags, tagErr := featuretags.DaemonBuildTags(root, binary.Base)
		if tagErr != nil {
			t.Fatalf("read the daemon tags for %s: %v", binary.Name, tagErr)
		}
		if got := tagsOf(t, builds[index]); got != wantTags {
			t.Errorf("%s was built with tags %q, want %q", binary.Name, got, wantTags)
		}
		if got, want := outputOf(t, builds[index]), filepath.Join(root, binary.Output); got != want {
			t.Errorf("%s was written to %q, want %q", binary.Name, got, want)
		}
		if builds[index].Directory != root {
			t.Errorf("%s was built in %q, want the checkout root %q", binary.Name, builds[index].Directory, root)
		}
		if builds[index].Timeout != labBuildTimeout {
			t.Errorf("%s was bounded at %s, want %s", binary.Name, builds[index].Timeout, labBuildTimeout)
		}
	}
}

// VALIDATES: the staged binary is a static Linux binary at the DAEMON's architecture, never the host's.
// PREVENTS: `exec format error` on a remote Docker context, and a dynamically linked binary that cannot run on the musl alpine base.
func TestLabCrossBuildIsStaticLinuxAtTheDaemonArch(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	// An architecture this host is not, so a host-derived GOARCH cannot pass.
	const daemonArchitecture = "arm64"

	runner := newBuildRunner(daemonArchitecture)
	binary := LabBinary{Name: "ze", Base: featuretags.DaemonBase, Output: "test/interop/ze-linux"}
	if err := StageBinaries(root, false, binary)(context.Background(), newDocker(runner)); err != nil {
		t.Fatalf("StageBinaries returned an error: %v", err)
	}

	builds := runner.builds()
	if len(builds) != 1 {
		t.Fatalf("the preflight ran %d builds, want 1", len(builds))
	}
	for _, want := range []string{"GOOS=linux", "GOARCH=" + daemonArchitecture, "CGO_ENABLED=0"} {
		if !slices.Contains(builds[0].Environment, want) {
			t.Errorf("the cross-compile environment does not carry %s", want)
		}
	}
	if slices.Contains(builds[0].Environment, "CGO_ENABLED=1") {
		t.Error("the cross-compile environment turns CGO on, which links the binary against the host libc")
	}
}

// VALIDATES: an architecture the preflight cannot read is an error naming the query and the answer, before any build runs.
// PREVENTS: a guessed GOARCH, whose only symptom is a container that exits with `exec format error`.
func TestPreflightRefusesAnUnreadableDaemonArchitecture(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	binary := LabBinary{Name: "ze", Base: featuretags.DaemonBase, Output: "test/interop/ze-linux"}

	cases := []struct {
		name   string
		answer processResult
		fail   error
	}{
		{name: "empty answer", answer: processResult{Stdout: "\n"}},
		{name: "a diagnostic sentence", answer: processResult{Stdout: "Cannot connect to the Docker daemon\n"}},
		{name: "a shell fragment", answer: processResult{Stdout: "amd64; rm -rf /\n"}},
		{name: "the daemon refused", answer: processResult{ExitCode: 1}, fail: errors.New("daemon unreachable")},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			runner := &recordingRunner{run: func(processCommand) (processResult, error) {
				return one.answer, one.fail
			}}
			err := StageBinaries(root, false, binary)(context.Background(), newDocker(runner))
			if err == nil {
				t.Fatal("StageBinaries accepted an architecture it could not read")
			}
			for _, command := range runner.commands {
				if len(command.Arguments) > 0 && command.Arguments[0] == "go" {
					t.Fatalf("a build ran after an unreadable architecture: %v", command.Arguments)
				}
			}
			if one.fail != nil {
				return
			}
			if !strings.Contains(err.Error(), serverArchitectureFormat) {
				t.Errorf("the refusal does not name what it asked: %v", err)
			}
			if !strings.Contains(err.Error(), strings.TrimSpace(one.answer.Stdout)) {
				t.Errorf("the refusal does not name what it got: %v", err)
			}
		})
	}
}

// VALIDATES: NO_BUILD performs no build and asks the daemon nothing.
// PREVENTS: a run that reuses yesterday's images paying for a cross-compile it will not use.
func TestPreflightSkippedUnderNoBuild(t *testing.T) {
	runner := newBuildRunner("amd64")
	binary := LabBinary{Name: "ze", Base: featuretags.DaemonBase, Output: "test/interop/ze-linux"}
	if err := StageBinaries("/nowhere", true, binary)(context.Background(), newDocker(runner)); err != nil {
		t.Fatalf("StageBinaries returned an error under NO_BUILD: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("NO_BUILD ran %d commands, want none: %v", len(runner.commands), runner.commands)
	}
}

// VALIDATES: an incomplete declaration and a non-positive bound are both refused rather than run.
// PREVENTS: a build with no tags, which produces a daemon that answers `start` with "unknown command".
func TestStagingRefusesAnIncompleteDeclarationAndANonPositiveBound(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	incomplete := []LabBinary{
		{Name: "ze", Output: "test/interop/ze-linux"},
		{Name: "ze", Base: featuretags.DaemonBase},
		{Base: featuretags.DaemonBase, Output: "test/interop/ze-linux"},
	}
	for _, binary := range incomplete {
		runner := newBuildRunner("amd64")
		if err := StageBinaries(root, false, binary)(context.Background(), newDocker(runner)); err == nil {
			t.Errorf("StageBinaries accepted the incomplete declaration %+v", binary)
		}
	}
	if err := StageBinaries(root, false)(context.Background(), newDocker(newBuildRunner("amd64"))); err == nil {
		t.Error("StageBinaries accepted a lab that declares no binary at all")
	}

	if labBuildTimeout <= 0 {
		t.Fatalf("labBuildTimeout is %s, and a build bound must be positive", labBuildTimeout)
	}
	_, err = systemProcessRunner{}.Run(context.Background(), processCommand{Arguments: []string{"true"}})
	if err == nil {
		t.Error("a process with a zero bound ran; a non-positive bound must be refused before the command starts")
	}
}

// VALIDATES: the architecture token guard accepts every GOARCH spelling Docker reports and refuses everything else.
// PREVENTS: a daemon answer reaching GOARCH as an environment value when it is not an architecture name at all.
func TestArchitectureTokenAcceptsAGOARCHAndRefusesTheRest(t *testing.T) {
	for _, good := range []string{"amd64", "arm64", "arm", "386", "riscv64", "loong64", "s390x", "ppc64le"} {
		if !architectureToken(good) {
			t.Errorf("architectureToken refused %q, which is a GOARCH Go names", good)
		}
	}
	for _, bad := range []string{"", " ", "amd64 ", "AMD64", "amd64;true", "amd/64", "amd_64", strings.Repeat("a", architectureTokenMax+1)} {
		if architectureToken(bad) {
			t.Errorf("architectureToken accepted %q", bad)
		}
	}
}
