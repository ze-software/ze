package testqemu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: D-6, AC-8 and AC-10's one guest path. The guest starts Docker,
// waits for it with a bound, runs the Docker kernel check probing with a real
// linux ze built for the guest, and only then runs the lab, as `le <lab words>`.
// PREVENTS: a lab started before the check, or on a daemon that never came up,
// whose failures would read as Ze's; and the check probing with the guest le,
// whose ze personality has no `doctor` (the first HVF boot on 2026-10-10 ended
// "unknown command: doctor").
func TestDockerLabCommandChecksTheKernelBeforeTheLab(t *testing.T) {
	command := dockerLabCommand("arm64", []string{"test", "integration", "interop-ipsec"}, nil)

	guestLe := filepath.ToSlash(filepath.Join(GuestWorkspace, GuestLeRel("arm64")))
	guestZe := filepath.ToSlash(filepath.Join(GuestWorkspace, GuestZeRel("arm64")))
	start := strings.Index(command, "rc-service docker start")
	ready := strings.Index(command, "docker info")
	check := strings.Index(command, shellQuote(guestLe)+" setup docker-kernel check ze "+shellQuote(guestZe))
	lab := strings.Index(command, shellQuote(guestLe)+" 'test' 'integration' 'interop-ipsec'")
	if start < 0 || ready < 0 || check < 0 || lab < 0 {
		t.Fatalf("guest command lacks a step (start %d, ready %d, check %d, lab %d):\n%s", start, ready, check, lab, command)
	}
	if start >= ready || ready >= check || check >= lab {
		t.Fatalf("guest command steps out of order (start %d, ready %d, check %d, lab %d):\n%s", start, ready, check, lab, command)
	}
	if !strings.HasPrefix(command, "set -e;") {
		t.Errorf("a failed step does not stop the guest command:\n%s", command)
	}
	if !strings.Contains(command, "60") {
		t.Errorf("the wait for dockerd carries no bound:\n%s", command)
	}
}

// VALIDATES: with no lab the guest command ends at the kernel check, which is
// what the boot proof (TestDockerLabGuestBootsZeKernel) runs.
// PREVENTS: an empty lab running `le` bare, which lists actions and exits 0.
func TestDockerLabCommandWithoutALabEndsAtTheCheck(t *testing.T) {
	command := dockerLabCommand("amd64", nil, nil)
	guestLe := filepath.ToSlash(filepath.Join(GuestWorkspace, GuestLeRel("amd64")))
	guestZe := filepath.ToSlash(filepath.Join(GuestWorkspace, GuestZeRel("amd64")))
	want := shellQuote(guestLe) + " setup docker-kernel check ze " + shellQuote(guestZe)
	if !strings.HasSuffix(command, want) {
		t.Fatalf("guest command does not end at the check %q:\n%s", want, command)
	}
}

// VALIDATES: `env` reaches the lab and only the lab: the suites select one
// scenario from the environment (INTEROP_SCENARIO, IPSEC_INTEROP_SCENARIO), and
// the host's environment never reaches the guest.
// PREVENTS: a one-scenario run on the Mac being impossible, so every proof runs
// a whole suite; and a malformed assignment reaching the guest shell.
func TestDockerLabEnvironmentReachesOnlyTheLab(t *testing.T) {
	environment, err := dockerLabEnvironment("IPSEC_INTEROP_SCENARIO=mobike-initiator", true)
	if err != nil {
		t.Fatalf("a well-formed assignment refused: %v", err)
	}
	command := dockerLabCommand("arm64", []string{"test", "integration", "interop-ipsec"}, environment)
	guestLe := shellQuote(filepath.ToSlash(filepath.Join(GuestWorkspace, GuestLeRel("arm64"))))
	lab := "env 'IPSEC_INTEROP_SCENARIO=mobike-initiator' " + guestLe + " 'test' 'integration' 'interop-ipsec'"
	if !strings.HasSuffix(command, lab) {
		t.Fatalf("the lab does not run under its environment, want suffix %q:\n%s", lab, command)
	}
	if strings.Count(command, "IPSEC_INTEROP_SCENARIO") != 1 {
		t.Errorf("the environment reaches more than the lab:\n%s", command)
	}

	for _, refused := range []struct {
		value string
		lab   bool
	}{
		{"IPSEC_INTEROP_SCENARIO=mobike-initiator", false},
		{"mobike-initiator", false},
		{"IPSEC_INTEROP_SCENARIO=a", false},
		{"1BAD=x", true},
		{"BAD NAME=x", true},
		{"=x", true},
		{"lower=x", true},
		{"NOVALUE", true},
	} {
		if _, err := dockerLabEnvironment(refused.value, refused.lab); err == nil {
			t.Errorf("env %q (lab %v) was accepted", refused.value, refused.lab)
		}
	}
	if environment, err := dockerLabEnvironment("", false); err != nil || environment != nil {
		t.Errorf("no env answered %v, %v; want nothing", environment, err)
	}
}

// VALIDATES: the guest boots the kernel the cache entry holds for the guest's
// architecture, and an absent entry refuses naming the build that writes it
// (R-2), before any guest boots.
// PREVENTS: a silent fall back to the Alpine ISO kernel, whose verdict would
// describe Alpine rather than Ze.
func TestDockerLabKernelComesFromTheCacheEntry(t *testing.T) {
	cache := t.TempDir()
	if _, err := dockerLabKernel(cache, "arm64"); err == nil {
		t.Fatal("an empty cache entry answered a kernel")
	} else {
		for _, want := range []string{filepath.Join(cache, "vmlinuz"), "ze appliance kernel --target runtime --arch arm64"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal does not name %q: %v", want, err)
			}
		}
	}

	if err := os.WriteFile(filepath.Join(cache, "vmlinuz"), []byte("kernel"), 0o600); err != nil {
		t.Fatalf("write the cached kernel: %v", err)
	}
	kernel, err := dockerLabKernel(cache, "arm64")
	if err != nil {
		t.Fatalf("a cache entry holding vmlinuz refused: %v", err)
	}
	if kernel != filepath.Join(cache, "vmlinuz") {
		t.Fatalf("kernel = %q, want the cache entry's vmlinuz", kernel)
	}
}

// VALIDATES: the action is registered with its keywords, so `./le test qemu`
// lists it and the grammar accepts `lab` and `timeout`.
func TestDockerLabActionIsRegistered(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Verb != dockerLabVerb {
			continue
		}
		keywords := map[string]bool{}
		for _, parameter := range action.Parameters {
			keywords[parameter.Keyword] = true
		}
		for _, want := range []string{keywordLab, keywordEnv, keywordTimeout} {
			if !keywords[want] {
				t.Errorf("%s does not take %q", dockerLabVerb, want)
			}
		}
		return
	}
	t.Fatalf("no %s action in the test qemu table", dockerLabVerb)
}
