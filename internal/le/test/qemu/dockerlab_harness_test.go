// Design: docs/architecture/testing/qemu-integration.md -- One command in Ze's own system

package testqemu

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// TestDockerLabHarnessIsOnlyValidWithCommand reaches the registered consumer and
// requires its own mode refusal. A harness with no command names a guest action
// nothing would run.
func TestDockerLabHarnessIsOnlyValidWithCommand(t *testing.T) {
	stderr, err := os.CreateTemp(t.TempDir(), "refusal-")
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = stderr
	t.Cleanup(func() {
		os.Stderr = saved
		if err := stderr.Close(); err != nil {
			t.Error(err)
		}
	})
	answer, code := Answer([]string{dockerLabVerb, KeywordHarness})
	output, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(string(output), "harness is only valid with command") {
		t.Fatalf("consumer answered %d (%v), stderr %q; want the harness mode refusal", code, answer, output)
	}
}

// TestDockerLabHarnessSetupMountsTheWorkspace pins the contract `test qemu all`
// checks before its first suite: the checkout at GuestWorkspace, and a guest-only
// tmpfs over the patched-tree cache, entered from GuestWorkspace.
// PREVENTS: a guest action refusing with ErrNotMounted, and a tree whose links
// name the guest's GOROOT reaching the host through the share.
func TestDockerLabHarnessSetupMountsTheWorkspace(t *testing.T) {
	got := dockerLabHarnessSetup("/home/a b/ze")
	want := "sudo -n mount -o bind '/home/a b/ze' /workspace" +
		" && sudo -n mkdir -p /workspace/cache/stdpatch" +
		" && sudo -n mount -t tmpfs tmpfs /workspace/cache/stdpatch" +
		" && cd /workspace"
	if got != want {
		t.Errorf("harness setup\n got %q\nwant %q", got, want)
	}
}

// TestDockerLabHarnessRunsTheCommandAsRootWithItsEnvironment executes the line
// under a real shell with a recording `sudo`, because what matters is what the
// guest action receives: sudo resets the environment, so a variable set outside
// it never arrives, and the caller's own assignment must beat the harness default.
func TestDockerLabHarnessRunsTheCommandAsRootWithItsEnvironment(t *testing.T) {
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "sudo.log")
	// Like the real one, this sudo drops what the caller's shell exported.
	fake := "#!/bin/sh\n[ \"$1\" = -n ] || exit 91\nshift\necho sudo >> " + shellQuote(record) +
		"\nexec env -i PATH=\"$PATH\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte(fake), 0o700); err != nil { //nolint:gosec // the fake sudo must be executable
		t.Fatal(err)
	}
	line := "ZE_RUN_JOB=/tmp/ze-host-job; export LOST=yes; " + dockerLabRootCommand(
		"GOCACHE=/workspace/tmp/own; printf '%s\\n' \"$ZE_RUN_JOB\" \"$HOME\" \"$TMPDIR\" \"$GOMODCACHE\" \"$GOFLAGS\" \"$GOCACHE\" \"[$LOST]\"")
	command := exec.CommandContext(t.Context(), "sh", "-c", line)
	command.Env = []string{"PATH=" + bin + ":" + os.Getenv("PATH")}
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("harness line failed: %v\n%s", err, out)
	}
	wantLines := []string{"/tmp/ze-host-job", "/tmp", "/tmp", "/tmp/ze-guest-gomodcache", "-buildvcs=false", "/workspace/tmp/own", "[]"}
	if got := strings.Fields(string(out)); strings.Join(got, " ") != strings.Join(wantLines, " ") {
		t.Errorf("the guest action saw %q, want %q", got, wantLines)
	}
	if logged, err := os.ReadFile(record); err != nil || strings.TrimSpace(string(logged)) != "sudo" { //nolint:gosec // the test's own record
		t.Errorf("the command did not run through one sudo: %q, %v", logged, err)
	}
}

// TestDockerLabHarnessInterruptReachesRootCommand runs the real interrupt sender
// over a recorded session. The guest action runs as root, so a SIGINT the ordinary
// user sends reaches only its own shell, and the run would wait out the teardown
// grace with the action still running.
func TestDockerLabHarnessInterruptReachesRootCommand(t *testing.T) {
	run := NewRun(t.TempDir(), RunOptions{Command: "true", Timeout: time.Minute})
	session := dockerLabSession{run: run, key: "/run/owned-key", port: 2222}
	var line string
	run.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		line = strings.Join(spec.Args, " ")
		return commandResult{Stdout: "42\n"}, nil
	}
	plain := RunPlan{SSHPort: 2222, commandSession: &session}
	if _, err := run.interruptGuest(t.Context(), &plain); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(line, "sudo") {
		t.Errorf("an unprivileged command's interrupt elevated: %s", line)
	}
	root := RunPlan{SSHPort: 2222, commandSession: &session, rootCommand: true}
	if _, err := run.interruptGuest(t.Context(), &root); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "sudo -n kill -INT -$pgid") {
		t.Errorf("a root command's interrupt stayed unprivileged: %s", line)
	}
}

// recordedDockerLabHost is what the command branch handed its host steps.
type recordedDockerLabHost struct {
	steps []string
	run   *Run
}

// stubDockerLabHost replaces the command branch's host steps with recorders over
// the runtime tree at tree, for this test. The boot answers exit 5.
func stubDockerLabHost(t *testing.T, tree string) *recordedDockerLabHost {
	t.Helper()
	saved := dockerLabHost
	t.Cleanup(func() { dockerLabHost = saved })
	recorded := &recordedDockerLabHost{}
	dockerLabHost = dockerLabCommandHost{
		runtimeTree: func(string, string) (string, error) { return tree, nil },
		ensureImage: func(context.Context, string, string) (string, error) {
			recorded.steps = append(recorded.steps, "image")
			return "/img", nil
		},
		buildGuest: func(context.Context, string, string) error {
			recorded.steps = append(recorded.steps, "guest-le")
			return nil
		},
		boot: func(_ context.Context, run *Run, _ string, _ *dockerLabKernel) (DockerLabReport, error) {
			recorded.steps = append(recorded.steps, "boot")
			recorded.run = run
			return DockerLabReport{CommandExit: 5}, nil
		},
	}
	return recorded
}

// TestDockerLabHarnessActionPreparesTheGuest drives the verb's body with and
// without the switch, inside a parent's slot, and executes the command the boot
// receives under a recording sudo.
// VALIDATES: the switch reaches the boot's options, the guest le is built after the
// image and before the boot, and the command the guest runs goes through sudo with
// the harness environment. Without the switch none of that happens.
// PREVENTS: a `harness` word that parses and does nothing, which every guest action
// then refuses for want of root and a guest le.
func TestDockerLabHarnessActionPreparesTheGuest(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "parent.job")
	if err := os.WriteFile(entry, []byte("LABEL=outer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nameParent(t, entry)
	tree := dockerLabRuntimeFixture(t)
	const guestAction = `printf '%s\n' "$GOFLAGS"`
	typed := func() leaction.Arguments {
		return leaction.Arguments{keywordKernel: {filepath.Join(tree, "vmlinuz")}, keywordCommand: {guestAction}}
	}

	plain := stubDockerLabHost(t, tree)
	if _, code := runDockerLab(t.Context(), t.TempDir(), typed(), ""); code != 5 {
		t.Fatalf("plain command answered %d, want the guest's 5", code)
	}
	if got := strings.Join(plain.steps, " "); got != "image boot" {
		t.Errorf("plain command ran %q, want no guest le", got)
	}
	if plain.run.Options.GuestHarness || plain.run.Options.Command != guestAction {
		t.Errorf("plain command reached the boot as harness=%v command %q", plain.run.Options.GuestHarness, plain.run.Options.Command)
	}

	withHarness := typed()
	withHarness[KeywordHarness] = nil
	harness := stubDockerLabHost(t, tree)
	if _, code := runDockerLab(t.Context(), t.TempDir(), withHarness, ""); code != 5 {
		t.Fatalf("harness command answered %d, want the guest's 5", code)
	}
	if got := strings.Join(harness.steps, " "); got != "image guest-le boot" {
		t.Errorf("harness command ran %q, want the guest le between the image and the boot", got)
	}
	if !harness.run.Options.GuestHarness {
		t.Error("the switch did not reach the boot, so the guest gets no mount and no root interrupt")
	}
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "sudo.log")
	fake := "#!/bin/sh\necho sudo >> " + shellQuote(record) + "\nshift\nexec env -i PATH=\"$PATH\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte(fake), 0o700); err != nil { //nolint:gosec // the fake sudo must be executable
		t.Fatal(err)
	}
	guest := exec.CommandContext(t.Context(), "sh", "-c", harness.run.Options.Command)
	guest.Env = []string{"PATH=" + bin + ":" + os.Getenv("PATH")}
	out, err := guest.CombinedOutput()
	if err != nil {
		t.Fatalf("the command the boot received failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "-buildvcs=false" {
		t.Errorf("the guest action saw GOFLAGS %q, want the harness's", out)
	}
	if logged, err := os.ReadFile(record); err != nil || strings.TrimSpace(string(logged)) != "sudo" { //nolint:gosec // the test's own record
		t.Errorf("the command did not run through sudo: %q, %v", logged, err)
	}
}

// TestDockerLabHarnessHoldsAHostSlot drives the verb's body with no parent over a
// scratch repository, with a recording program as the re-executed le.
// VALIDATES: the harness branch claims a slot in the host's registry and runs as a
// child in it, re-executing the words that were typed under the slot's entry, and
// answers the child's code. Nothing boots in the claiming process.
// PREVENTS: `le feature run`, `le rfc record` and the nightly booting a VM outside
// the registry while the guest's stand-in names a slot nobody holds.
func TestDockerLabHarnessHoldsAHostSlot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commitTree(t, root)
	t.Setenv(job.ParentVariable, "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	bin := t.TempDir()
	record := filepath.Join(bin, "argv")
	parent := filepath.Join(bin, "parent")
	child := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(record) +
		"\nprintf '%s' \"$" + job.ParentVariable + "\" > " + shellQuote(parent) + "\nexit 7\n"
	self := filepath.Join(bin, "le-child")
	if err := os.WriteFile(self, []byte(child), 0o700); err != nil { //nolint:gosec // the fake le must be executable
		t.Fatal(err)
	}
	savedSelf := runSelf
	t.Cleanup(func() { runSelf = savedSelf })
	runSelf = func() (string, error) { return self, nil }

	tree := dockerLabRuntimeFixture(t)
	recorded := stubDockerLabHost(t, tree)
	kernel := filepath.Join(tree, "vmlinuz")
	args := leaction.Arguments{keywordKernel: {kernel}, keywordCommand: {"guest action"}, KeywordHarness: nil, keywordTimeout: {"1500s"}}
	if _, code := runDockerLab(t.Context(), root, args, ""); code != 7 {
		t.Errorf("answered %d, want the slot's child's 7", code)
	}
	if len(recorded.steps) != 0 {
		t.Errorf("the claiming process ran %q itself, outside the child", recorded.steps)
	}
	typed, err := os.ReadFile(record) //nolint:gosec // the test's own record
	if err != nil {
		t.Fatalf("the slot's child never ran: %v", err)
	}
	want := strings.Join([]string{"test", "qemu", dockerLabVerb, keywordCommand, "guest action",
		keywordKernel, kernel, KeywordHarness, keywordTimeout, "1500s"}, "\n")
	if got := strings.TrimSpace(string(typed)); got != want {
		t.Errorf("the slot re-executed\n%s\nwant\n%s", got, want)
	}
	named, err := os.ReadFile(parent) //nolint:gosec // the test's own record
	if err != nil || len(named) == 0 {
		t.Errorf("the child runs under no slot entry: %q, %v", named, err)
	}
}

// TestDockerLabHarnessBootMountsAndInterruptsAsRoot drives the VM consumer over a
// fake QEMU and recorded ssh, with and without the harness, and interrupts the
// command while it runs.
// VALIDATES: under the harness the guest command's setup binds the checkout, and the
// forwarded SIGINT goes through the elevated sender. Without it, neither.
// PREVENTS: a harness command refused with ErrNotMounted, and an interrupt that
// reaches only the session's own shell, so the run waits out the teardown grace with
// the guest action still running.
func TestDockerLabHarnessBootMountsAndInterruptsAsRoot(t *testing.T) {
	for _, mode := range []struct{ harness, root bool }{{false, false}, {true, false}, {false, true}} {
		harness := mode.harness
		run, tree, _ := fakeDockerLabRun(t)
		runtimeTree := dockerLabRuntimeFixture(t)
		kernel, err := dockerLabCommandKernel(filepath.Join(runtimeTree, "vmlinuz"), runtimeTree)
		if err != nil {
			t.Fatal(err)
		}
		image := dockerLabImageFixture(t, "runtime-kernel", "root=PARTUUID=fixture-02 ro init=/gokrazy/init\n")
		run.Options.Kernel = kernel.Selected
		run.Options.Command = "guest-action"
		run.Options.GuestHarness = harness
		run.Options.GuestRoot = mode.root

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		interrupted := make(chan struct{})
		var command, interrupt string
		answerOthers := run.ops.Run
		run.ops.Run = func(runCtx context.Context, spec commandSpec) (commandResult, error) {
			remote := spec.Args[len(spec.Args)-1]
			switch {
			case spec.Name == sshProgram && strings.HasPrefix(remote, "sh -c "):
				command = remote
				cancel()
				select {
				case <-interrupted:
				case <-time.After(20 * time.Second):
				}
				return commandResult{Code: 130}, nil
			case spec.Name == sshProgram && strings.Contains(remote, "kill -INT"):
				interrupt = remote
				close(interrupted)
				return commandResult{Stdout: "42\n"}, nil
			}
			return answerOthers(runCtx, spec)
		}

		report, err := run.dockerLabCommand(ctx, image, kernel)
		if err == nil || !report.Interrupted {
			t.Errorf("harness=%v root=%v: an interrupted run answered %+v, %v", harness, mode.root, report, err)
		}
		setup := guestRunParentSetup(guestRunJobParent)
		wantInterrupt := guestInterruptRemote(guestCommandGroupFile)
		if harness {
			setup += " && " + dockerLabHarnessSetup(tree)
		}
		if harness || mode.root {
			// root alone binds nothing, but its interrupt must reach a root process.
			wantInterrupt = guestRootInterruptRemote(guestCommandGroupFile)
		}
		if want := "sh -c " + shellQuote(guestCommandBody(guestCommandGroupFile, setup, "guest-action")); command != want {
			t.Errorf("harness=%v root=%v: guest command\n got %q\nwant %q", harness, mode.root, command, want)
		}
		if interrupt != wantInterrupt {
			t.Errorf("harness=%v root=%v: interrupt %q, want %q", harness, mode.root, interrupt, wantInterrupt)
		}
	}
}

// TestDockerLabRootIsOnlyValidWithCommand requires the root mode refusal: root
// with no command elevates nothing, and root with a command is accepted.
func TestDockerLabRootIsOnlyValidWithCommand(t *testing.T) {
	err := dockerLabMode(leaction.Arguments{KeywordRoot: nil})
	if err == nil || !strings.Contains(err.Error(), "root is only valid with command") {
		t.Fatalf("root without command answered %v, want the root mode refusal", err)
	}
	accepted := leaction.Arguments{KeywordRoot: nil, keywordCommand: {"true"}, keywordKernel: {"vmlinuz"}}
	if err := dockerLabMode(accepted); err != nil {
		t.Fatalf("root with command and kernel refused: %v", err)
	}
}

// TestDockerLabRootRunsTheCommandAsRootWithoutTheHarness drives the verb's body
// with the root switch over the stubbed host steps.
// VALIDATES: the switch reaches the boot's options, the command the boot receives
// is the sudo wrapper, and no guest le is built: root is the elevation alone.
// PREVENTS: a root-declared guest Go unit running unprivileged and skipping.
func TestDockerLabRootRunsTheCommandAsRootWithoutTheHarness(t *testing.T) {
	entry := filepath.Join(t.TempDir(), "parent.job")
	if err := os.WriteFile(entry, []byte("LABEL=outer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nameParent(t, entry)
	tree := dockerLabRuntimeFixture(t)
	const guestAction = "sh 'guest-1.sh'"
	typed := leaction.Arguments{keywordKernel: {filepath.Join(tree, "vmlinuz")},
		keywordCommand: {guestAction}, KeywordRoot: nil}
	recorded := stubDockerLabHost(t, tree)
	if _, code := runDockerLab(t.Context(), t.TempDir(), typed, ""); code != 5 {
		t.Fatalf("root command answered %d, want the guest's 5", code)
	}
	if got := strings.Join(recorded.steps, " "); got != "image boot" {
		t.Errorf("root command ran %q, want no guest le", got)
	}
	options := recorded.run.Options
	if !options.GuestRoot {
		t.Error("the root switch did not reach the boot")
	}
	if options.GuestHarness {
		t.Error("the root switch turned on the harness")
	}
	if want := dockerLabRootCommand(guestAction); options.Command != want {
		t.Errorf("root command reached the boot as %q, want %q", options.Command, want)
	}
}
