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
	return &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return ProcessResult{Stdout: options}, nil
		}
		return ProcessResult{}, nil
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
	requireCommandOnItsOwnLine(t, err, AppArmorLoadCommandFor(testLabProfile))
	for _, want := range []string{testLabProfile, "the test lab's router"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal lacks %q: %v", want, err)
		}
	}
	if argv := runArgv(runner); argv != nil {
		t.Errorf("the container ran: %v", argv)
	}
}

// VALIDATES: review round 3, NOTE 5. A daemon applying AppArmor whose host
// profile list is unreadable (a non-root user) runs the container, and when
// runc cannot apply the profile the refusal names the profile and the command
// that loads it, on a line of its own.
// PREVENTS: runc's raw error as the only answer for a lab container under an
// unloaded Ze profile, the half the kernel probe already maps.
func TestRunContainerMapsAnUnappliedProfileToItsLoadCommand(t *testing.T) {
	registerTestLabProfile(t)
	original := appArmorProfiles
	t.Cleanup(func() { appArmorProfiles = original })
	appArmorProfiles = func() (string, bool) { return "", false }
	stderr := "docker: Error response from daemon: failed to create task: unable to apply apparmor profile: " +
		"apparmor failed to apply profile: write /proc/self/attr/apparmor/exec: no such file or directory"
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if slices.Contains(command.Arguments, dockerSecurityOptionsFormat) {
			return ProcessResult{Stdout: withAppArmor}, nil
		}
		if slices.Contains(command.Arguments, "run") {
			return ProcessResult{ExitCode: 127, Stderr: stderr}, nil
		}
		return ProcessResult{}, nil
	}}
	err := newDocker(runner).runContainer(t.Context(), labNetwork(), labPeer())
	if err == nil {
		t.Fatal("a container whose profile the daemon could not apply was reported started")
	}
	requireCommandOnItsOwnLine(t, err, AppArmorLoadCommandFor(testLabProfile))
	for _, want := range []string{testLabProfile, "the test lab's router", "unable to apply apparmor profile"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal lacks %q: %v", want, err)
		}
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

// VALIDATES: owner decision 2026-10-21 ("find the colima issue so that setup
// can have it working"). On a darwin client every AppArmor refusal names
// `./le setup docker-kernel apparmor confirm <profile>` on its own line, says
// that on macOS it loads the profile into the Colima VM that runs the daemon,
// and still names the QEMU docker-lab route and the daemon's kernel release,
// because the profile adds no kernel feature; on a Linux client the texts are
// unchanged.
// METHOD: the kernel check runs over a scripted daemon whose client is set
// to darwin, once with the probe's profile unloaded and once with a denied
// row; the lab refusal builder runs for both client systems.
// PREVENTS: a Mac user on Colima sent only to the QEMU guest when the one
// command that lets the probe answer works from the Mac.
func TestAppArmorRefusalOnDarwinNamesTheColimaLoad(t *testing.T) {
	route := DockerKernelRoute(hostGOOSDarwin, leWords(), selectorEnvironment())
	denied := `{"ready": false, "capabilities": [
 {"subsystem": "mpls-transit-mtu", "kernel": "CONFIG_MPLS_IP_MTU", "state": "denied",
  "reason": "make the probe's mounts private: permission denied"}]}`
	for name, profiles := range map[string]string{
		"unloaded":   "docker-default (enforce)\n",
		"denied row": kernelcap.ProbeAppArmorProfileName + " (enforce)\n",
	} {
		answer := allPresent
		if name == "denied row" {
			answer = denied
		}
		docker := newDocker(appArmorKernel(t, answer, profiles, true))
		docker.owner.goos = hostGOOSDarwin
		err := checkDockerKernel(t.Context(), docker, kernelCheckZe)
		if err == nil {
			t.Fatalf("%s: the kernel check proceeded", name)
		}
		if !strings.Contains(err.Error(), "\n  "+AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName)) {
			t.Errorf("%s: a darwin refusal lacks the load command on its own line:\n%v", name, err)
		}
		if !strings.Contains(err.Error(), appArmorColimaLoad) {
			t.Errorf("%s: a darwin refusal does not say the command loads into Colima's VM:\n%v", name, err)
		}
		if !strings.Contains(err.Error(), route) {
			t.Errorf("%s: a darwin refusal lacks the QEMU route %q:\n%v", name, route, err)
		}
		if !strings.Contains(err.Error(), "6.8.0-117-generic") {
			t.Errorf("%s: a darwin refusal lacks the daemon's kernel release:\n%v", name, err)
		}
		if strings.Contains(err.Error(), "from darwin") {
			t.Errorf("%s: a darwin refusal names the system as darwin, not macOS:\n%v", name, err)
		}
	}

	probe := appArmorRefusal(hostGOOSLinux, "6.8.0-117-generic", "the daemon could not apply (x)")
	wantProbe := "the Docker daemon applies AppArmor, and the kernel probe runs under Ze's profile ze-kernel-probe," +
		" which the daemon could not apply (x). Docker's docker-default profile denies the probe's mount and its" +
		" /proc/sys write. Load Ze's profile with:\n  ./le setup docker-kernel apparmor confirm ze-kernel-probe\n" +
		"The profile lets the probe answer and adds no kernel feature: every feature the daemon's kernel" +
		" (6.8.0-117-generic) lacks, MOBIKE's xfrm migrate for example, is then named.\n" +
		DockerKernelRoute(hostGOOSLinux, leWords(), selectorEnvironment())
	if probe.Error() != wantProbe {
		t.Errorf("the Linux probe refusal changed:\n%v\nwant\n%v", probe, wantProbe)
	}

	registerTestLabProfile(t)
	profile, _ := LookupAppArmorProfile(testLabProfile)
	darwin := labAppArmorRefusal(hostGOOSDarwin, profile, "the daemon could not apply (x)")
	if !strings.Contains(darwin.Error(), "\n  "+AppArmorLoadCommandFor(testLabProfile)) {
		t.Errorf("the darwin lab refusal lacks the load command on its own line:\n%v", darwin)
	}
	if !strings.Contains(darwin.Error(), appArmorColimaLoad) {
		t.Errorf("the darwin lab refusal does not say the command loads into Colima's VM:\n%v", darwin)
	}
	if !strings.Contains(darwin.Error(), route) {
		t.Errorf("the darwin lab refusal lacks the QEMU route %q:\n%v", route, darwin)
	}
	linux := labAppArmorRefusal(hostGOOSLinux, profile, "the daemon could not apply (x)")
	want := "the Docker daemon applies AppArmor, and the test lab's router runs under Ze's profile ze-test-lab," +
		" which the daemon could not apply (x). Docker's docker-default profile denies its sysctl writes." +
		" Load Ze's profile with:\n  ./le setup docker-kernel apparmor confirm ze-test-lab"
	if linux.Error() != want {
		t.Errorf("the Linux lab refusal changed:\n%v\nwant\n%v", linux, want)
	}
}

// VALIDATES: on a client that is neither Linux nor macOS, where `./le setup
// docker-kernel apparmor` refuses (appArmorLoadPlatform), the probe refusal,
// the lab refusal and a denied row's fix never name the load command, never
// claim the Colima load, and point at the QEMU docker-lab route.
// METHOD: each refusal builder runs for freebsd.
// PREVENTS: the macOS branch leaking the load command to a system it refuses.
func TestAppArmorRefusalElsewhereNamesOnlyTheQEMURoute(t *testing.T) {
	const freebsd = "freebsd"
	route := DockerKernelRoute(freebsd, leWords(), selectorEnvironment())
	registerTestLabProfile(t)
	profile, _ := LookupAppArmorProfile(testLabProfile)
	for name, text := range map[string]string{
		"probe": appArmorRefusal(freebsd, "6.8.0-117-generic", "the daemon could not apply (x)").Error(),
		"lab":   labAppArmorRefusal(freebsd, profile, "the daemon could not apply (x)").Error(),
	} {
		if strings.Contains(text, AppArmorLoadCommandPrefix) {
			t.Errorf("%s: a freebsd refusal names the load command:\n%s", name, text)
		}
		if strings.Contains(text, appArmorColimaLoad) {
			t.Errorf("%s: a freebsd refusal claims the Colima load:\n%s", name, text)
		}
		if !strings.Contains(text, route) {
			t.Errorf("%s: a freebsd refusal lacks the QEMU route %q:\n%s", name, route, text)
		}
		if !strings.Contains(text, appArmorLoadUnreachable(freebsd)) {
			t.Errorf("%s: a freebsd refusal does not say why the command is not named:\n%s", name, text)
		}
	}
	fix := deniedFix(freebsd, true, false)
	if strings.Contains(fix, AppArmorLoadCommandPrefix) {
		t.Errorf("a freebsd denied fix names the load command: %s", fix)
	}
	if !strings.Contains(fix, "QEMU guest route") {
		t.Errorf("a freebsd denied fix does not point at the QEMU route: %s", fix)
	}
}
