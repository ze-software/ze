package interoplab

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

const kernelCheckZe = "/checkout/test/interop-ipsec/ze-linux"

// scriptedKernel scripts the daemon: `docker info` answers the kernel release and
// the probe container answers stdout with exit.
func scriptedKernel(stdout string, exit int, runErr error) *recordingRunner {
	return &recordingRunner{run: func(command processCommand) (processResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return processResult{Stdout: noAppArmor}, nil
		}
		if slices.Contains(command.Arguments, "info") {
			return processResult{Stdout: "6.8.0-117-generic\n"}, nil
		}
		return processResult{Stdout: stdout, ExitCode: exit}, runErr
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
	for _, want := range []string{"ipsec-mobike", "CONFIG_XFRM_MIGRATE", "invalid argument", "fake-later-enrolment", "CONFIG_FAKE", "6.8.0-117-generic", DockerKernelRoute(runtime.GOOS)} {
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
func kernelAnswering(run func(processCommand) (processResult, error)) func(processCommand) (processResult, error) {
	return func(command processCommand) (processResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return processResult{Stdout: noAppArmor}, nil
		}
		if slices.Contains(command.Arguments, "{{.KernelVersion}}") {
			return processResult{Stdout: "6.8.0-117-generic\n"}, nil
		}
		if slices.Contains(command.Arguments, "kernel-capabilities") {
			return processResult{Stdout: allPresent}, nil
		}
		if run == nil {
			return processResult{}, nil
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
	for _, want := range []string{"ipsec-mobike", "CONFIG_XFRM_MIGRATE", "6.8.0-117-generic", DockerKernelRoute(runtime.GOOS)} {
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
// macOS, where that install refuses.
// PREVENTS: a Mac developer sent to an action that cannot run there.
func TestDockerKernelRouteNamesEachPlatformsNextStep(t *testing.T) {
	for goos, wants := range map[string][]string{
		"linux":  {"./le setup docker-kernel install", "reboot"},
		"darwin": {"./le test qemu docker-lab"},
	} {
		route := DockerKernelRoute(goos)
		for _, want := range wants {
			if !strings.Contains(route, want) {
				t.Errorf("%s route %q does not name %q", goos, route, want)
			}
		}
	}
	if strings.Contains(DockerKernelRoute("darwin"), "docker-kernel install") {
		t.Errorf("the darwin route names the Linux-only install: %s", DockerKernelRoute("darwin"))
	}
}

// appArmorKernel scripts a daemon that applies AppArmor and a probe answering
// stdout, and fakes the local profile list.
func appArmorKernel(t *testing.T, stdout string, profiles string, readable bool) *recordingRunner {
	t.Helper()
	original := appArmorProfiles
	t.Cleanup(func() { appArmorProfiles = original })
	appArmorProfiles = func() (string, bool) { return profiles, readable }
	return &recordingRunner{run: func(command processCommand) (processResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return processResult{Stdout: withAppArmor}, nil
		}
		if slices.Contains(command.Arguments, "info") {
			return processResult{Stdout: "6.8.0-117-generic\n"}, nil
		}
		return processResult{Stdout: stdout}, nil
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
	for _, want := range []string{"AppArmor", "ze-kernel-probe", AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName)} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
	if argv := probeArgv(runner); argv != "" {
		t.Errorf("a container ran before the refusal: %s", argv)
	}
}

// VALIDATES: AC-17. A denied row refuses as any missing feature does, its
// reason in the text, plus the command that loads Ze's profile.
// PREVENTS: a policy denial read as a kernel fault, and a refusal with no fix.
func TestDockerKernelDeniedRowNamesTheProfileCommand(t *testing.T) {
	answer := `{"ready": false, "capabilities": [
 {"subsystem": "mpls-transit-mtu", "kernel": "CONFIG_MPLS_IP_MTU", "state": "denied",
  "reason": "make the probe's mounts private: permission denied; AppArmor profile docker-default (enforce) refused it"}]}`
	err := checkDockerKernel(context.Background(), newDocker(scriptedKernel(answer, 1, nil)), kernelCheckZe)
	if err == nil {
		t.Fatal("a denied row proceeded")
	}
	for _, want := range []string{"mpls-transit-mtu", "denied", "docker-default (enforce)", AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName)} {
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
	for name, answer := range map[string]processResult{
		"not JSON": {Stdout: "permission denied\n"},
		"failed":   {Stdout: "", ExitCode: 1, Stderr: "daemon gone"},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{run: func(command processCommand) (processResult, error) {
				if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
					return answer, nil
				}
				return processResult{Stdout: allPresent}, nil
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
	runner.run = func(command processCommand) (processResult, error) {
		if slices.Contains(command.Arguments, "run") {
			return processResult{ExitCode: 127, Stderr: runcAppArmorRefusal}, nil
		}
		return probe(command)
	}
	err := checkDockerKernel(context.Background(), newDocker(runner), kernelCheckZe)
	if err == nil {
		t.Fatal("a profile the daemon could not apply proceeded")
	}
	for _, want := range []string{"ze-kernel-probe", AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName), "6.8.0-117-generic", DockerKernelRoute(runtime.GOOS)} {
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
	for _, want := range []string{"6.8.0-117-generic", DockerKernelRoute(runtime.GOOS)} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}
