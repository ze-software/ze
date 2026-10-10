package interoplab

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

const kernelCheckZe = "/checkout/test/interop-ipsec/ze-linux"

// scriptedKernel scripts the daemon: `docker info` answers the kernel release and
// the probe container answers stdout with exit.
func scriptedKernel(stdout string, exit int, runErr error) *recordingRunner {
	return &recordingRunner{run: func(command processCommand) (processResult, error) {
		if slices.Contains(command.Arguments, "info") {
			return processResult{Stdout: "6.8.0-117-generic\n"}, nil
		}
		return processResult{Stdout: stdout, ExitCode: exit}, runErr
	}}
}

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
	for _, want := range []string{"ipsec-mobike", "CONFIG_XFRM_MIGRATE", "invalid argument", "fake-later-enrolment", "CONFIG_FAKE", "6.8.0-117-generic", "./le setup docker-kernel install"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "CONFIG_XFRM_USER") {
		t.Errorf("refusal names a present feature: %v", err)
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
// the check adds two commands (docker info, the probe container) before the
// first image build.
func kernelAnswering(run func(processCommand) (processResult, error)) func(processCommand) (processResult, error) {
	return func(command processCommand) (processResult, error) {
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
	for _, want := range []string{"ipsec-mobike", "CONFIG_XFRM_MIGRATE", "6.8.0-117-generic", kernelInstallRoute} {
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
