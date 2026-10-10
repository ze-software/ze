// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: apparmor.go -- the profile registry and the container confinement these tests drive
//
// A lab container that needs more than docker-default grants runs under a Ze
// profile, never unconfined (owner decision D-7). These tests script the
// daemon and the local profile list; no profile is loaded anywhere.

package interoplab

import (
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

const testLabProfile = "ze-test-lab"

// registerTestLabProfile registers testLabProfile for one test.
func registerTestLabProfile(t *testing.T) {
	t.Helper()
	RegisterAppArmorProfile(AppArmorProfile{
		Name: testLabProfile, Runs: "the test lab's router", Needs: "its sysctl writes",
		Text: func() string { return "profile ze-test-lab {}\n" }, Next: "./le test integration interop",
	})
	t.Cleanup(func() { delete(appArmorRegistry, testLabProfile) })
}

// labDaemon scripts a daemon whose security options are options and fakes the
// local profile list.
func labDaemon(t *testing.T, options, profiles string) *recordingRunner {
	t.Helper()
	original := appArmorProfiles
	t.Cleanup(func() { appArmorProfiles = original })
	appArmorProfiles = func() (string, bool) { return profiles, true }
	return &recordingRunner{run: func(command processCommand) (processResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return processResult{Stdout: options}, nil
		}
		return processResult{}, nil
	}}
}

func labPeer() PeerConfig {
	return PeerConfig{Name: "ze", Container: "ze-lab", Image: "ze", Host: 2, AppArmorProfile: testLabProfile}
}

func labNetwork() Network {
	return Network{Name: "lab", IPv4: netip.MustParsePrefix("172.31.22.0/24")}
}

// runArgv answers the argv of the recorded `docker run`, or nil.
func runArgv(runner *recordingRunner) []string {
	for _, command := range runner.commands {
		if slices.Contains(command.Arguments, "run") {
			return command.Arguments
		}
	}
	return nil
}

// VALIDATES: D-7. On a daemon applying AppArmor, a peer naming a loaded Ze
// profile starts under it.
// PREVENTS: the profile named and never applied.
func TestRunContainerAppliesItsAppArmorProfile(t *testing.T) {
	registerTestLabProfile(t)
	runner := labDaemon(t, withAppArmor, testLabProfile+" (enforce)\ndocker-default (enforce)\n")
	if err := newDocker(runner).runContainer(t.Context(), labNetwork(), labPeer()); err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(runArgv(runner), " ")
	if !strings.Contains(argv, "--security-opt apparmor="+testLabProfile+" ze") {
		t.Errorf("the container does not run under %s: %s", testLabProfile, argv)
	}
}

// VALIDATES: D-7. On a daemon that applies no AppArmor there is nothing to
// select, so no option is passed.
func TestRunContainerWithoutAppArmorPassesNoProfile(t *testing.T) {
	registerTestLabProfile(t)
	runner := labDaemon(t, `["name=seccomp,profile=builtin"]`, "")
	if err := newDocker(runner).runContainer(t.Context(), labNetwork(), labPeer()); err != nil {
		t.Fatal(err)
	}
	if argv := strings.Join(runArgv(runner), " "); strings.Contains(argv, "apparmor") {
		t.Errorf("a daemon without AppArmor was given a profile: %s", argv)
	}
}

// VALIDATES: D-7. A daemon applying AppArmor whose host lacks the profile is
// refused before the container starts, naming the command that loads it.
// PREVENTS: a lab container that fails later with Docker's own opaque error,
// or one that falls back to unconfined.
func TestRunContainerRefusesAnUnloadedProfile(t *testing.T) {
	registerTestLabProfile(t)
	runner := labDaemon(t, withAppArmor, "docker-default (enforce)\n")
	err := newDocker(runner).runContainer(t.Context(), labNetwork(), labPeer())
	if err == nil {
		t.Fatal("a container started under a profile the host has not loaded")
	}
	for _, want := range []string{testLabProfile, "the test lab's router", AppArmorLoadCommandFor(testLabProfile)} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal lacks %q: %v", want, err)
		}
	}
	if argv := runArgv(runner); argv != nil {
		t.Errorf("the container ran: %v", argv)
	}
}

// VALIDATES: D-7. A peer naming a profile no lab registered is refused.
func TestRunContainerRefusesAnUnregisteredProfile(t *testing.T) {
	runner := labDaemon(t, withAppArmor, "")
	err := newDocker(runner).runContainer(t.Context(), labNetwork(), labPeer())
	if err == nil || !strings.Contains(err.Error(), testLabProfile) {
		t.Fatalf("an unregistered profile was not refused by name: %v", err)
	}
	if argv := runArgv(runner); argv != nil {
		t.Errorf("the container ran: %v", argv)
	}
}

// VALIDATES: D-7. The kernel probe's profile is registered, so the load action
// can load it with kernelcap's text, and its load command confirms that name.
func TestKernelProbeProfileIsRegistered(t *testing.T) {
	profile, registered := LookupAppArmorProfile("ze-kernel-probe")
	if !registered {
		t.Fatal("ze-kernel-probe is not registered")
	}
	if profile.Text() != kernelcap.ProbeAppArmorProfile() {
		t.Error("the registered text is not kernelcap's probe profile")
	}
	if got, want := AppArmorLoadCommandFor(profile.Name), "./le setup docker-kernel apparmor confirm ze-kernel-probe"; got != want {
		t.Errorf("load command %q, want %q", got, want)
	}
	if !slices.Contains(AppArmorProfileNames(), profile.Name) {
		t.Errorf("AppArmorProfileNames lacks %s: %v", profile.Name, AppArmorProfileNames())
	}
}
