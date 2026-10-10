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
