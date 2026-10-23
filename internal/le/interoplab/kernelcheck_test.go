package interoplab

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
	buildhostdriver "github.com/ze-software/ze/internal/le/build/hostdriver"
)

// kernelCheckZe is a staged ze stand-in: a regular file, because the check
// refuses a path that holds none. TestMain writes it.
var kernelCheckZe string

// TestMain writes the staged ze stand-in every kernel check test probes with,
// and removes it after the run.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "interoplab-kernelcheck")
	if err != nil {
		panic("BUG: no temporary directory for the staged ze stand-in: " + err.Error())
	}
	kernelCheckZe = filepath.Join(dir, "ze-linux")
	if err := os.WriteFile(kernelCheckZe, []byte("stand-in"), 0o600); err != nil {
		panic("BUG: write the staged ze stand-in: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(dir) //nolint:errcheck // a leftover temporary directory changes no verdict
	os.Exit(code)
}

// scriptedKernel scripts the daemon: `docker info` answers the kernel release and
// the probe container answers stdout with exit.
func scriptedKernel(stdout string, exit int, runErr error) *recordingRunner {
	return &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return ProcessResult{Stdout: noAppArmor}, nil
		}
		if slices.Contains(command.Arguments, "info") {
			return ProcessResult{Stdout: "6.8.0-117-generic\n"}, nil
		}
		// The orphan cleanup's labeled listings find nothing on this daemon.
		if slices.Contains(command.Arguments, "label="+labelMarker) {
			return ProcessResult{}, nil
		}
		return ProcessResult{Stdout: stdout, ExitCode: exit}, runErr
	}}
}

// noAppArmor and withAppArmor are `docker info` security options as a daemon
// prints them; withAppArmor is colima's, read 2026-10-10.
const (
	noAppArmor   = `["name=seccomp,profile=builtin","name=cgroupns"]` + "\n"
	withAppArmor = `["name=apparmor","name=seccomp,profile=builtin","name=cgroupns"]` + "\n"
)

const allPresent = `{"ready": true, "capabilities": [
 {"subsystem": "ipsec", "kernel": "CONFIG_XFRM_USER", "state": "present"},
 {"subsystem": "l2tp", "kernel": "CONFIG_L2TP", "state": "present"}]}`

// VALIDATES: AC-5. Every row present proceeds; the probe runs the staged ze's
// kernel-capabilities mode in a container granted NET_ADMIN and SYS_ADMIN (the
// xfrm-interface probe unshares a network namespace), with the binary mounted.
// PREVENTS: a probe container whose missing privilege turns present into unknown.
func TestDockerKernelAllPresentProceeds(t *testing.T) {
	runner := scriptedKernel(allPresent, 0, nil)
	if err := checkDockerKernel(context.Background(), newDocker(runner), kernelCheckZe); err != nil {
		t.Fatalf("all present refused: %v", err)
	}
	var probe []string
	for _, command := range runner.commands {
		if slices.Contains(command.Arguments, "run") {
			probe = command.Arguments
		}
	}
	joined := strings.Join(probe, " ")
	for _, want := range []string{"--cap-add NET_ADMIN", "--cap-add SYS_ADMIN", kernelCheckZe + ":/ze:ro", "doctor --json kernel-capabilities"} {
		if !strings.Contains(joined, want) {
			t.Errorf("probe argv lacks %q: %s", want, joined)
		}
	}
}

// VALIDATES: AC-3. A daemon kernel lacking features refuses, naming EVERY absent
// row by subsystem and CONFIG_ symbol, the kernel release, and both routes.
// AC-11: the check holds no feature name; the rows come from the probe.
// PREVENTS: fifteen minutes of scenario failures that are a host fact, and a
// repair loop that meets one missing feature per run.
func TestDockerKernelRefusesNamingEveryMissingFeature(t *testing.T) {
	answer := `{"ready": false, "capabilities": [
 {"subsystem": "ipsec", "kernel": "CONFIG_XFRM_USER", "state": "present"},
 {"subsystem": "ipsec-mobike", "kernel": "CONFIG_XFRM_MIGRATE", "state": "absent", "reason": "invalid argument"},
 {"subsystem": "fake-later-enrolment", "kernel": "CONFIG_FAKE", "state": "absent"}]}`
	err := checkDockerKernel(context.Background(), newDocker(scriptedKernel(answer, 1, nil)), kernelCheckZe)
	if err == nil {
		t.Fatal("a kernel lacking features was accepted")
	}
	for _, want := range []string{"ipsec-mobike", "CONFIG_XFRM_MIGRATE", "invalid argument", "fake-later-enrolment", "CONFIG_FAKE", "6.8.0-117-generic", DockerKernelRoute(hostGOOSLinux, leWords(), selectorEnvironment())} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "CONFIG_XFRM_USER") {
		t.Errorf("refusal names a present feature: %v", err)
	}
}

// VALIDATES: rows that all read present still refuse when the probe's own
// verdict disagrees: a non-zero exit, or `ready` false. The refusal names the
// exit code and the ready field it read.
// PREVENTS: the check re-deriving a pass from the rows alone and proceeding on
// a kernel the producer itself (doctor writeKernelCapabilities) did not call
// ready.
func TestDockerKernelHonoursTheProbeVerdict(t *testing.T) {
	notReady := `{"ready": false, "capabilities": [
 {"subsystem": "ipsec", "kernel": "CONFIG_XFRM_USER", "state": "present"}]}`
	for name, tc := range map[string]struct {
		stdout string
		exit   int
		want   string
	}{
		"present rows, exit 1":      {allPresent, 1, "exit 1"},
		"present rows, ready false": {notReady, 0, "ready false"},
	} {
		err := checkDockerKernel(context.Background(), newDocker(scriptedKernel(tc.stdout, tc.exit, nil)), kernelCheckZe)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: refusal does not name %q: %v", name, tc.want, err)
		}
	}
}

// VALIDATES: AC-4. An unknown row, a failed probe container, unreadable JSON and
// an empty answer each refuse, naming what ran and what was read.
// PREVENTS: a check that proceeds because it could not ask.
func TestDockerKernelUnknownRefuses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stdout string
		exit   int
		runErr error
		want   string
	}{
		{"unknown row", `{"ready": false, "capabilities": [{"subsystem": "xfrm-interface", "kernel": "CONFIG_XFRM_INTERFACE", "state": "unknown", "reason": "needs CAP_SYS_ADMIN"}]}`, 1, nil, "needs CAP_SYS_ADMIN"},
		{"container failed", "", 125, nil, "exit 125"},
		{"runner error", "", 0, errors.New("docker vanished"), "docker vanished"},
		{"not json", "exec format error", 1, nil, "exec format error"},
		{"empty answer", `{"ready": false, "capabilities": []}`, 1, nil, "no capability"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkDockerKernel(context.Background(), newDocker(scriptedKernel(tc.stdout, tc.exit, tc.runErr)), kernelCheckZe)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("refusal does not name %q: %v", tc.want, err)
			}
			if !strings.Contains(err.Error(), "kernel-capabilities") {
				t.Errorf("refusal does not name what ran: %v", err)
			}
		})
	}
}

// kernelAnswering wraps a scripted runner so the Docker kernel check that
// Suite.Run performs reads a kernel with every feature present; every other
// command reaches run. A Suite test that is about something else uses it, and
// the check adds three commands (docker info twice, the probe container) before the
// first image build.
func kernelAnswering(run func(ProcessCommand) (ProcessResult, error)) func(ProcessCommand) (ProcessResult, error) {
	return func(command ProcessCommand) (ProcessResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return ProcessResult{Stdout: noAppArmor}, nil
		}
		if slices.Contains(command.Arguments, "{{.KernelVersion}}") {
			return ProcessResult{Stdout: "6.8.0-117-generic\n"}, nil
		}
		if slices.Contains(command.Arguments, "kernel-capabilities") {
			return ProcessResult{Stdout: allPresent}, nil
		}
		if run == nil {
			return ProcessResult{}, nil
		}
		return run(command)
	}
}

// kernelSuite is a suite that declares one image to build and one scenario, so
// a refusal that lets either through is visible in the recorded argv.
func kernelSuite(runner *recordingRunner, stagedZe string, preflight PreflightCheck) Suite {
	return Suite{
		Docker:    newDocker(runner),
		Preflight: preflight,
		StagedZe:  stagedZe,
		Images:    []ImageBuild{{Name: "ze", Tag: "ze-kernel-test", Dockerfile: "/checkout/Dockerfile.ze", Context: "/checkout", Required: true}},
		Scenarios: []ScenarioPlan{{Source: ScenarioSource{Name: "unreached", Checker: func(context.Context, *CheckContext) error { return nil }}}},
	}
}

// VALIDATES: AC-3 through Suite.Run, the entry point every interop lab and the
// docker-* deployment proofs reach. A daemon kernel lacking a feature refuses
// with the feature named, after Preflight staged ze and before any image build,
// and no scenario is counted.
// PREVENTS: the check existing as a function no suite calls.
func TestSuiteRefusesDockerHostMissingAKernelFeature(t *testing.T) {
	answer := `{"ready": false, "capabilities": [
 {"subsystem": "ipsec-mobike", "kernel": "CONFIG_XFRM_MIGRATE", "state": "absent", "reason": "invalid argument"}]}`
	runner := scriptedKernel(answer, 1, nil)
	staged := false
	report := kernelSuite(runner, kernelCheckZe, func(context.Context, *Docker) error {
		staged = true
		return nil
	}).Run(t.Context())

	if report.Code != 1 {
		t.Fatalf("Suite code = %d, want 1: %+v", report.Code, report)
	}
	if !staged {
		t.Error("the kernel check ran without the Preflight that stages ze")
	}
	for _, want := range []string{"ipsec-mobike", "CONFIG_XFRM_MIGRATE", "6.8.0-117-generic", DockerKernelRoute(hostGOOSLinux, leWords(), selectorEnvironment())} {
		if !strings.Contains(report.SetupError, want) {
			t.Errorf("setup error does not name %q: %s", want, report.SetupError)
		}
	}
	if report.Passed != 0 || report.Failed != 0 || len(report.Scenarios) != 0 {
		t.Errorf("a refused host counted scenarios: %+v", report)
	}
	joined := recordedCommands(runner.commands)
	if strings.Contains(joined, "docker build") {
		t.Errorf("an image build followed the refusal:\n%s", joined)
	}
	if !strings.Contains(joined, "kernel-capabilities") {
		t.Errorf("Suite.Run never probed the kernel:\n%s", joined)
	}
}

// VALIDATES: AC-5 through Suite.Run. Every row present proceeds to the image
// build, and the probe comes before the build.
// PREVENTS: a check that refuses a passing host, or runs after the build it guards.
func TestSuiteProbesTheKernelBeforeTheFirstBuild(t *testing.T) {
	runner := &recordingRunner{run: kernelAnswering(nil)}
	report := kernelSuite(runner, kernelCheckZe, nil).Run(t.Context())
	joined := recordedCommands(runner.commands)
	if strings.Contains(report.SetupError, "kernel") {
		t.Fatalf("a kernel with every feature was refused: %s", report.SetupError)
	}
	assertOrder(t, joined, "doctor --json kernel-capabilities", "docker build")
}

// VALIDATES: D-4. A suite that names no staged ze refuses before any image
// build: the check fails, it never skips.
// PREVENTS: a lab added without the staged ze running on an unchecked kernel.
func TestSuiteWithoutStagedZeRefuses(t *testing.T) {
	runner := &recordingRunner{run: kernelAnswering(nil)}
	report := kernelSuite(runner, "", nil).Run(t.Context())
	if report.Code != 1 {
		t.Fatalf("Suite code = %d, want 1: %+v", report.Code, report)
	}
	if !strings.Contains(report.SetupError, "staged ze") {
		t.Errorf("setup error does not say what is missing: %s", report.SetupError)
	}
	if joined := recordedCommands(runner.commands); strings.Contains(joined, "docker build") {
		t.Errorf("an image build followed the refusal:\n%s", joined)
	}
}

// VALIDATES: every lab names the ze it stages as the binary the check runs.
// PREVENTS: a lab whose StagedZe points at a path its Preflight never writes.
func TestStagedZePathIsTheZeOutput(t *testing.T) {
	binaries := []LabBinary{
		{Name: "le", Base: "ze_le", Output: "test/interop/le-linux"},
		{Name: "ze", Base: "ze_core", Output: "test/interop/ze-linux"},
	}
	if got, want := StagedZePath("/checkout", binaries), "/checkout/test/interop/ze-linux"; got != want {
		t.Errorf("StagedZePath = %q, want %q", got, want)
	}
	if got := StagedZePath("/checkout", binaries[:1]); got != "" {
		t.Errorf("a lab that stages no ze answered %q; Suite.Run refuses the empty answer", got)
	}
}

// VALIDATES: AC-3 under D-6, the refusal names the next step the reader's
// platform takes: the install action on Linux, the Ze-kernel QEMU guest on
// macOS, where that install refuses. Review round 3, ISSUE 2: each command
// stands on a line of its own and is the exact command, never a template: on
// macOS it reruns the refused lab's own le words inside the guest.
// Review round 4, ISSUE 3: a run whose scenario a selector variable chose
// reruns under the same assignment, through the guest's `env` keyword.
// PREVENTS: a Mac developer sent to an action that cannot run there, a route
// the reader must edit (`"<le words>"`) before it runs, and a route that reruns
// the whole suite in place of the one scenario that was refused.
func TestDockerKernelRouteNamesEachPlatformsNextStep(t *testing.T) {
	lab := []string{"test", "integration", "interop", "ipsec"}
	selected := []string{"IPSEC_INTEROP_SCENARIO=mobike-responder"}
	for _, tc := range []struct {
		goos        string
		lab         []string
		environment []string
		command     string
	}{
		{"linux", lab, nil, "./le setup docker-kernel install"},
		{"linux", lab, selected, "./le setup docker-kernel install"},
		{"darwin", lab, selected, "./le setup docker-host start"},
		{"darwin", nil, nil, "./le setup docker-host start"},
		{"freebsd", lab, nil, "./le test qemu docker-lab lab 'test integration interop ipsec'"},
		{"freebsd", lab, selected, "./le test qemu docker-lab lab 'test integration interop ipsec' env 'IPSEC_INTEROP_SCENARIO=mobike-responder'"},
		{"freebsd", lab, []string{"A=1", "B=2"}, "./le test qemu docker-lab lab 'test integration interop ipsec' env 'A=1 B=2'"},
		// An assignment the guest's env keyword cannot carry intact would rerun
		// the whole suite, so the route falls back to the kernel check alone.
		{"freebsd", lab, []string{"IPSEC_INTEROP_SCENARIO=it's"}, "./le test qemu docker-lab"},
		{"freebsd", nil, nil, "./le test qemu docker-lab"},
		{"freebsd", nil, selected, "./le test qemu docker-lab"},
		{"freebsd", []string{"perf", "run", "name", "two words"}, nil, "./le test qemu docker-lab"},
		// Review round 5, R5-N3: the guest splits its lab and env values with
		// strings.Fields, which breaks on every unicode.IsSpace rune, so a word
		// holding any of them falls back to the check alone.
		{"freebsd", []string{"perf", "run", "a\rb"}, nil, "./le test qemu docker-lab"},
		{"freebsd", []string{"perf", "run", "a\vb"}, nil, "./le test qemu docker-lab"},
		{"freebsd", []string{"perf", "run", "a\fb"}, nil, "./le test qemu docker-lab"},
		{"freebsd", []string{"perf", "run", "a b"}, nil, "./le test qemu docker-lab"},
		{"freebsd", lab, []string{"IPSEC_INTEROP_SCENARIO=a b"}, "./le test qemu docker-lab"},
	} {
		route := DockerKernelRoute(tc.goos, tc.lab, tc.environment)
		requireCommandOnItsOwnLine(t, errors.New(route), tc.command)
		if strings.Contains(route, "<") {
			t.Errorf("%s route %q holds a template, not a command", tc.goos, route)
		}
	}
	if strings.Contains(DockerKernelRoute("darwin", lab, nil), "docker-kernel install") {
		t.Errorf("the darwin route names the Linux-only install: %s", DockerKernelRoute("darwin", lab, nil))
	}
}

// VALIDATES: review round 3, ISSUE 2. The le words the route reruns are this
// process's own command line, for `le` and for `ze le`, and none for a
// process that is not le.
// PREVENTS: a route naming a test binary's flags as a lab.
func TestInvokedLeWordsReadTheCommandLine(t *testing.T) {
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"/repo/bin/le", "test", "perf"}, "test perf"},
		{[]string{"ze", "le", "test", "perf"}, "test perf"},
		{[]string{"/build/interoplab.test", "-test.run", "X"}, ""},
	} {
		os.Args = tc.args
		if got := strings.Join(invokedLeWords(), " "); got != tc.want {
			t.Errorf("invokedLeWords() under %v = %q, want %q", tc.args, got, tc.want)
		}
	}
}

// VALIDATES: review round 4, NOTE 2, after the guest variable left with the
// Alpine guest (spec ze-image-docker-host R-11). A refusal from a daemon whose
// kernel release is Ze's names the runtime kernel's own fix: its config files
// and the kernel cache rebuild, each command on a line of its own, on Linux
// and on any other client system; a daemon on another kernel keeps the
// platform route, and macOS keeps the profile's route.
// METHOD: checkDockerKernel over a scripted daemon answering a "-ze" release
// and one absent row, then dockerKernelRouteFor for each client system.
// PREVENTS: an install and a reboot prescribed for a daemon that already runs
// Ze's kernel, which changes nothing.
func TestDockerKernelRouteForZesKernelNamesTheRuntimeConfig(t *testing.T) {
	const release = "7.2.9" + zeKernelReleaseSuffix
	answer := `{"ready": false, "capabilities": [
 {"subsystem": "ipsec-mobike", "kernel": "CONFIG_XFRM_MIGRATE", "state": "absent"}]}`
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return ProcessResult{Stdout: noAppArmor}, nil
		}
		if slices.Contains(command.Arguments, "info") {
			return ProcessResult{Stdout: release + "\n"}, nil
		}
		if slices.Contains(command.Arguments, "label="+labelMarker) {
			return ProcessResult{}, nil
		}
		return ProcessResult{Stdout: answer, ExitCode: 1}, nil
	}}
	docker := newDocker(runner)
	docker.owner.goos = hostGOOSLinux
	err := checkDockerKernel(context.Background(), docker, kernelCheckZe)
	if err == nil {
		t.Fatal("a Ze kernel lacking a feature was accepted")
	}
	if strings.Contains(err.Error(), "docker-kernel install") {
		t.Errorf("the refusal prescribes installing Ze's kernel on a daemon that runs it: %v", err)
	}
	for _, want := range []string{release, "gokrazy/kernel/runtime.config", "gokrazy/kernel/runtime.require"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %s: %v", want, err)
		}
	}
	for line := range strings.SplitSeq(buildhostdriver.RuntimeKernelBuild(runtime.GOARCH, ""), "\n") {
		requireCommandOnItsOwnLine(t, err, strings.TrimSpace(line))
	}
	lab := []string{"test", "integration", "interop-ipsec"}
	if got := dockerKernelRouteFor("freebsd", release, lab, nil); !strings.Contains(got, "runtime.config") {
		t.Errorf("a Ze-kernel daemon reached from freebsd is not sent to runtime.config: %s", got)
	}
	if got, want := dockerKernelRouteFor(hostGOOSLinux, "6.8.0-117-generic", lab, nil), DockerKernelRoute(hostGOOSLinux, lab, nil); got != want {
		t.Errorf("another kernel's route = %q, want the Linux route %q", got, want)
	}
	if got, want := dockerKernelRouteFor(hostGOOSDarwin, release, lab, nil), DockerKernelRoute(hostGOOSDarwin, lab, nil); got != want {
		t.Errorf("the macOS route = %q, want the profile's route %q", got, want)
	}
}

// VALIDATES: zeKernelReleaseSuffix is the release suffix the runtime kernel
// config declares, so a refusal recognizes the kernel Ze builds today.
// METHOD: read CONFIG_LOCALVERSION from gokrazy/kernel/runtime.config, from the
// package directory the test runs in.
// PREVENTS: a renamed localversion that leaves every Ze-kernel daemon told to
// install Ze's kernel.
func TestZeKernelReleaseSuffixIsTheRuntimeConfigs(t *testing.T) {
	config, err := os.ReadFile(filepath.Join("..", "..", "..", "gokrazy", "kernel", "runtime.config"))
	if err != nil {
		t.Fatalf("read the runtime kernel config: %v", err)
	}
	want := `CONFIG_LOCALVERSION="` + zeKernelReleaseSuffix + `"`
	if !slices.Contains(strings.Split(string(config), "\n"), want) {
		t.Errorf("gokrazy/kernel/runtime.config holds no line %s", want)
	}
}

// VALIDATES: review round 4, ISSUE 3. The selector assignments the route
// carries are the variables a suite declared to ReadEnvironment, read from
// this process's environment, sorted, and none that is unset or blank.
// PREVENTS: a refused IPSEC_INTEROP_SCENARIO run rerun in the guest as the
// whole suite.
func TestSelectorEnvironmentCarriesTheDeclaredSelectors(t *testing.T) {
	t.Setenv("ZE_TEST_ROUTE_SELECTOR_B", "mobike-responder")
	t.Setenv("ZE_TEST_ROUTE_SELECTOR_A", " ebgp ")
	t.Setenv("ZE_TEST_ROUTE_SELECTOR_BLANK", " ")
	t.Setenv("ZE_TEST_ROUTE_UNDECLARED", "x")
	for _, name := range []string{"ZE_TEST_ROUTE_SELECTOR_B", "ZE_TEST_ROUTE_SELECTOR_A", "ZE_TEST_ROUTE_SELECTOR_BLANK", "ZE_TEST_ROUTE_SELECTOR_UNSET"} {
		ReadEnvironment(EnvironmentOptions{SelectorVariable: name, Lookup: func(string) (string, bool) { return "", false }})
	}
	var got []string
	for _, assignment := range invokedSelectorEnvironment() {
		if strings.HasPrefix(assignment, "ZE_TEST_ROUTE_") {
			got = append(got, assignment)
		}
	}
	want := "ZE_TEST_ROUTE_SELECTOR_A=ebgp ZE_TEST_ROUTE_SELECTOR_B=mobike-responder"
	if strings.Join(got, " ") != want {
		t.Errorf("invokedSelectorEnvironment() = %q, want %q", got, want)
	}
}

// VALIDATES: review round 3, ISSUE 2, on the refusal itself: a kernel lacking
// a feature names the platform's route command on a line of its own, and on
// macOS that command reruns the refused lab. Both client systems run on
// every host, so the verdict does not depend on the machine.
func TestDockerKernelRefusalNamesTheExactRouteCommand(t *testing.T) {
	original, originalEnvironment := leWords, selectorEnvironment
	t.Cleanup(func() { leWords, selectorEnvironment = original, originalEnvironment })
	leWords = func() []string { return []string{"test", "integration", "interop-ipsec"} }
	selectorEnvironment = func() []string { return []string{"IPSEC_INTEROP_SCENARIO=mobike-responder"} }
	answer := `{"ready": false, "capabilities": [
 {"subsystem": "ipsec-mobike", "kernel": "CONFIG_XFRM_MIGRATE", "state": "absent", "reason": "invalid argument"}]}`
	for goos, command := range map[string]string{
		hostGOOSLinux:  "./le setup docker-kernel install",
		hostGOOSDarwin: "./le setup docker-host start",
		"freebsd":      "./le test qemu docker-lab lab 'test integration interop-ipsec' env 'IPSEC_INTEROP_SCENARIO=mobike-responder'",
	} {
		docker := newDocker(scriptedKernel(answer, 1, nil))
		docker.owner.goos = goos
		err := checkDockerKernel(context.Background(), docker, kernelCheckZe)
		if err == nil {
			t.Fatalf("%s: a kernel lacking a feature proceeded", goos)
		}
		requireCommandOnItsOwnLine(t, err, command)
	}
}

// appArmorKernel scripts a daemon that applies AppArmor and a probe answering
// stdout, and fakes the local profile list.
func appArmorKernel(t *testing.T, stdout, profiles string, readable bool) *recordingRunner {
	t.Helper()
	original := appArmorProfiles
	t.Cleanup(func() { appArmorProfiles = original })
	appArmorProfiles = func() (string, bool) { return profiles, readable }
	return &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return ProcessResult{Stdout: withAppArmor}, nil
		}
		if slices.Contains(command.Arguments, "info") {
			return ProcessResult{Stdout: "6.8.0-117-generic\n"}, nil
		}
		return ProcessResult{Stdout: stdout}, nil
	}}
}

// probeArgv answers the probe container's argv among the recorded commands.
func probeArgv(runner *recordingRunner) string {
	for _, command := range runner.commands {
		if slices.Contains(command.Arguments, "run") {
			return strings.Join(command.Arguments, " ")
		}
	}
	return ""
}

// VALIDATES: AC-18 (D-7). A daemon that applies AppArmor runs the probe under
// Ze's profile, whether or not the local profile list is readable; a daemon that
// does not runs it unchanged.
// PREVENTS: the probe meeting docker-default (deny mount) and answering denied on
// a host whose operator already loaded Ze's profile.
func TestDockerKernelProbesUnderZeProfileWhenTheDaemonAppliesAppArmor(t *testing.T) {
	loaded := "docker-default (enforce)\nze-kernel-probe (enforce)\n"
	for name, readable := range map[string]bool{"profile listed": true, "list unreadable": false} {
		t.Run(name, func(t *testing.T) {
			runner := appArmorKernel(t, allPresent, loaded, readable)
			if err := checkDockerKernel(context.Background(), newDocker(runner), kernelCheckZe); err != nil {
				t.Fatalf("all present refused: %v", err)
			}
			if argv := probeArgv(runner); !strings.Contains(argv, "--security-opt apparmor=ze-kernel-probe") {
				t.Errorf("the probe does not run under ze-kernel-probe: %s", argv)
			}
		})
	}

	runner := scriptedKernel(allPresent, 0, nil)
	if err := checkDockerKernel(context.Background(), newDocker(runner), kernelCheckZe); err != nil {
		t.Fatalf("all present refused: %v", err)
	}
	if argv := probeArgv(runner); strings.Contains(argv, "--security-opt") {
		t.Errorf("a daemon without AppArmor got a security option: %s", argv)
	}
}

// VALIDATES: AC-18. A local profile list that lacks Ze's profile refuses before
// any container runs, naming the command that loads it.
// PREVENTS: a probe that docker refuses to start ("profile not found") and a
// reader who has to work out which profile and how to load it.
func TestDockerKernelRefusesAnUnloadedProfileNamingTheFix(t *testing.T) {
	runner := appArmorKernel(t, allPresent, "docker-default (enforce)\n", true)
	err := checkDockerKernel(context.Background(), newDocker(runner), kernelCheckZe)
	if err == nil {
		t.Fatal("an unloaded profile proceeded")
	}
	requireCommandOnItsOwnLine(t, err, AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName))
	for _, want := range []string{"AppArmor", "ze-kernel-probe"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
	if argv := probeArgv(runner); argv != "" {
		t.Errorf("a container ran before the refusal: %s", argv)
	}
}

// VALIDATES: AC-17. On a daemon applying AppArmor, a denied row refuses as any
// missing feature does, its reason in the text, plus the command that loads
// Ze's profile.
// PREVENTS: a policy denial read as a kernel fault, and a refusal with no fix.
func TestDockerKernelDeniedRowNamesTheProfileCommand(t *testing.T) {
	answer := `{"ready": false, "capabilities": [
 {"subsystem": "mpls-transit-mtu", "kernel": "CONFIG_MPLS_IP_MTU", "state": "denied",
  "reason": "make the probe's mounts private: permission denied; AppArmor profile docker-default (enforce) refused it"}]}`
	runner := appArmorKernel(t, answer, kernelcap.ProbeAppArmorProfileName+" (enforce)\n", true)
	err := checkDockerKernel(context.Background(), newDocker(runner), kernelCheckZe)
	if err == nil {
		t.Fatal("a denied row proceeded")
	}
	requireCommandOnItsOwnLine(t, err, AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName))
	for _, want := range []string{"mpls-transit-mtu", "denied", "docker-default (enforce)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

// VALIDATES: review round 3, ISSUE 1. On a daemon applying no AppArmor (an
// SELinux host), a denied row names the grant the row's own policy owes and
// the command that shows the SELinux denial, and never the AppArmor load
// action, which refuses on such a host.
// PREVENTS: an operator sent to load an AppArmor profile on a host without
// AppArmor.
func TestDockerKernelDeniedRowWithoutAppArmorNamesThePolicyGrant(t *testing.T) {
	answer := `{"ready": false, "capabilities": [
 {"subsystem": "mpls-transit-mtu", "kernel": "CONFIG_MPLS_IP_MTU", "state": "denied",
  "reason": "make the probe's mounts private: permission denied; SELinux context system_u:system_r:container_t:s0:c1,c2 refused it; the probe needs a mount of /proc/sys in a private mount namespace, which this host's policy must grant"}]}`
	err := checkDockerKernel(context.Background(), newDocker(scriptedKernel(answer, 1, nil)), kernelCheckZe)
	if err == nil {
		t.Fatal("a denied row proceeded")
	}
	if strings.Contains(err.Error(), AppArmorLoadCommandPrefix) {
		t.Errorf("a daemon without AppArmor was told to load an AppArmor profile: %v", err)
	}
	requireCommandOnItsOwnLine(t, err, seLinuxDenialsCommand)
	for _, want := range []string{"mpls-transit-mtu", "container_t", "applies no AppArmor", "must grant"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

// VALIDATES: AC-4 for the new question. Security options the check cannot read
// refuse: the probe's argv depends on them.
// PREVENTS: a probe run under the wrong profile because a failed query read as
// "no AppArmor".
func TestDockerKernelUnreadableSecurityOptionsRefuse(t *testing.T) {
	for name, answer := range map[string]ProcessResult{
		"not JSON": {Stdout: "permission denied\n"},
		"failed":   {Stdout: "", ExitCode: 1, Stderr: "daemon gone"},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
				if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
					return answer, nil
				}
				return ProcessResult{Stdout: allPresent}, nil
			}}
			if err := checkDockerKernel(context.Background(), newDocker(runner), kernelCheckZe); err == nil {
				t.Fatal("unreadable security options proceeded")
			}
			if argv := probeArgv(runner); argv != "" {
				t.Errorf("a container ran: %s", argv)
			}
		})
	}
}

// VALIDATES: the profile list is read by name and mode, so a profile whose name
// merely starts with Ze's is not taken for it.
func TestAppArmorProfileLoaded(t *testing.T) {
	for profiles, want := range map[string]bool{
		"ze-kernel-probe (enforce)\n":     true,
		"docker-default (enforce)\n":      false,
		"ze-kernel-probe-old (enforce)\n": false,
		"":                                false,
		"a (enforce)\nze-kernel-probe (complain)": true,
	} {
		if got := appArmorProfileLoaded(profiles, "ze-kernel-probe"); got != want {
			t.Errorf("appArmorProfileLoaded(%q) = %v, want %v", profiles, got, want)
		}
	}
}

// runcAppArmorRefusal is Docker's answer, verbatim from the Ubuntu 6.8 host on
// 2026-10-10, when the daemon applies AppArmor and ze-kernel-probe is not loaded.
const runcAppArmorRefusal = "docker: Error response from daemon: failed to create task for container: " +
	"failed to create shim task: OCI runtime create failed: runc create failed: unable to start container " +
	"process: error during container init: unable to apply apparmor profile: apparmor failed to apply " +
	"profile: write fsmount:fscontext:proc/thread-self/attr/apparmor/exec: no such file or directory"

// VALIDATES: AC-18 for a host whose profile list the check cannot read (Ubuntu
// as a non-root user). The unread list is not taken for a loaded profile: the
// probe's own failure to apply the profile refuses, naming the load command and
// the kernel route, and never reads as an unreadable kernel answer.
// PREVENTS: the owner's 2026-10-10 run, where the check started the container
// and reported "unexpected end of JSON input" for a profile nobody had loaded.
func TestDockerKernelUnloadedProfileOnAnUnreadableListNamesTheFix(t *testing.T) {
	runner := appArmorKernel(t, "", "", false)
	probe := runner.run
	runner.run = func(command ProcessCommand) (ProcessResult, error) {
		if slices.Contains(command.Arguments, "run") {
			return ProcessResult{ExitCode: 127, Stderr: runcAppArmorRefusal}, nil
		}
		return probe(command)
	}
	err := checkDockerKernel(context.Background(), newDocker(runner), kernelCheckZe)
	if err == nil {
		t.Fatal("a profile the daemon could not apply proceeded")
	}
	requireCommandOnItsOwnLine(t, err, AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName))
	for _, want := range []string{"ze-kernel-probe", "6.8.0-117-generic", DockerKernelRoute(hostGOOSLinux, leWords(), selectorEnvironment())} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "unreadable answer") {
		t.Errorf("the profile refusal reads as an unreadable probe answer: %v", err)
	}
}

// VALIDATES: item 3 of the owner's 2026-10-10 report. The missing-profile
// refusal also names the daemon's kernel and the route to Ze's kernel, because
// loading the profile lets the probe answer and changes no kernel feature.
// PREVENTS: an operator who loads the profile, reruns, and only then learns the
// stock kernel needs replacing too.
func TestDockerKernelUnloadedProfileNamesTheKernelRoute(t *testing.T) {
	runner := appArmorKernel(t, allPresent, "docker-default (enforce)\n", true)
	err := checkDockerKernel(context.Background(), newDocker(runner), kernelCheckZe)
	if err == nil {
		t.Fatal("an unloaded profile proceeded")
	}
	for _, want := range []string{"6.8.0-117-generic", DockerKernelRoute(hostGOOSLinux, leWords(), selectorEnvironment())} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

// VALIDATES: review round 2, finding 3. A staged ze path that holds no file, or
// holds a directory, refuses naming the path, before any docker command runs.
// PREVENTS: a probe that bind-mounts a missing path, which makes the daemon
// create a root-owned directory there, and a refusal that names no cause.
func TestDockerKernelRefusesAMissingStagedZe(t *testing.T) {
	for name, path := range map[string]string{
		"missing":   filepath.Join(t.TempDir(), "ze-linux"),
		"directory": t.TempDir(),
	} {
		t.Run(name, func(t *testing.T) {
			runner := scriptedKernel(allPresent, 0, nil)
			err := checkDockerKernel(context.Background(), newDocker(runner), path)
			if err == nil {
				t.Fatal("the check proceeded with no staged ze")
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("the refusal does not name %s: %v", path, err)
			}
			if len(runner.commands) != 0 {
				t.Errorf("docker ran before the refusal: %v", runner.commands)
			}
		})
	}
}

// requireCommandOnItsOwnLine fails unless the refusal carries command as a
// line of its own, so an operator can copy it without editing a sentence
// (user-facing-errors.md: the fix command on a line of its own).
func requireCommandOnItsOwnLine(t *testing.T, err error, command string) {
	t.Helper()
	for line := range strings.SplitSeq(err.Error(), "\n") {
		if strings.TrimSpace(line) == command {
			return
		}
	}
	t.Errorf("the refusal does not carry %q on a line of its own: %v", command, err)
}

// VALIDATES: review round 3, NOTE 4. A missing staged ze names, on a line of
// its own, the exact command that stages it: the run's own le words, with
// NO_BUILD unset when the run skipped its build, and as they were otherwise
// (perf stages without NO_BUILD). A word the shell would split is quoted.
// PREVENTS: an inline "run the same command again without NO_BUILD=1" that
// does not fit a run that never set it.
func TestStagedZeMissingNamesTheCommandThatStagesIt(t *testing.T) {
	original := leWords
	t.Cleanup(func() { leWords = original })
	leWords = func() []string { return []string{"perf", "run", "note", "two words"} }
	for _, tc := range []struct {
		noBuild string
		command string
	}{
		{"1", "env -u NO_BUILD ./le perf run note 'two words'"},
		{"", "./le perf run note 'two words'"},
	} {
		lookup := func(name string) (string, bool) {
			if name == "NO_BUILD" && tc.noBuild != "" {
				return tc.noBuild, true
			}
			return "", false
		}
		requireCommandOnItsOwnLine(t, errors.New(stageZeAgain(lookup)), tc.command)
	}
}

// VALIDATES: AC-8 (b) text: on macOS the kernel route names the `ze` Colima
// profile, the image rebuild, and `./le setup docker-host start` on a line of
// its own, never Colima's `default` profile or the QEMU guest; the Linux route
// is unchanged, word for word.
// PREVENTS: a Mac developer sent to a daemon the owner's standing decisions rule out.
func TestKernelCheckRouteNamesDockerHostSetup(t *testing.T) {
	darwin := DockerKernelRoute(hostGOOSDarwin, []string{"test", "integration", "interop-ipsec"}, nil)
	requireCommandOnItsOwnLine(t, errors.New(darwin), "./le setup docker-host start")
	for _, want := range []string{"the `ze` Colima profile", "./le setup docker-host build", "unset it"} {
		if !strings.Contains(darwin, want) {
			t.Errorf("darwin route lacks %q:\n%s", want, darwin)
		}
	}
	for _, banned := range []string{"default", "docker-lab", "docker-kernel install"} {
		if strings.Contains(darwin, banned) {
			t.Errorf("darwin route names %q:\n%s", banned, darwin)
		}
	}
	const linux = "Run Docker on a kernel with every feature, or install Ze's kernel and reboot." +
		" The install prints its steps and runs none until you confirm the release:\n" +
		"  ./le setup docker-kernel install"
	if got := DockerKernelRoute(hostGOOSLinux, nil, nil); got != linux {
		t.Errorf("linux route changed:\n%s", got)
	}
}
