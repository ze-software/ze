// Design: docs/architecture/testing/qemu-integration.md -- Docker labs on the docker-host image

package testqemu

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/appliance/dockerhost"
)

// VALIDATES: spec AC-9's boot, TestDockerLabBootsTheDockerHostImage's row:
// QEMU boots the docker-host image (snapshot=on, so the cached file is never
// written), then the cidata volume as the last virtio disk; arm64 names its
// UEFI firmware and amd64 none; every share is exported at its host path; and
// no Alpine kernel, initramfs or ISO is on the line.
// PREVENTS: a boot that writes the shared image, an arm64 guest with no
// firmware, a share the guest mounts somewhere a bind mount does not look, and
// the Alpine medium creeping back.
func TestDockerLabBootsTheDockerHostImage(t *testing.T) {
	for _, goarch := range []string{ArchARM64, "amd64"} {
		t.Run(goarch, func(t *testing.T) {
			vm := dockerLabVM{
				goarch: goarch, accelerator: "hvf", firmware: "/fw/edk2-aarch64-code.fd",
				image: "/cache/docker-host.img", cidata: "/run/cidata.iso", console: "/run/console.log",
				memory: "4096", cpus: "4", sshPort: 2222,
				shares: []dockerLabShare{{tag: "zelab0", point: "/Users/someone/ze"}, {tag: "zelab1", point: "/scratch/tmp"}},
			}
			argv := strings.Join(vm.argv(), " ")
			image := strings.Index(argv, "-drive file=/cache/docker-host.img,format=raw,if=virtio,snapshot=on")
			cidata := strings.Index(argv, "-drive file=/run/cidata.iso,format=raw,if=virtio,readonly=on")
			if image < 0 || cidata < image {
				t.Errorf("want the image drive with snapshot=on, then the read-only cidata drive:\n%s", argv)
			}
			if hasFirmware := strings.Contains(argv, "-bios /fw/edk2-aarch64-code.fd"); hasFirmware != (goarch == ArchARM64) {
				t.Errorf("firmware on the line = %v for %s:\n%s", hasFirmware, goarch, argv)
			}
			for _, want := range []string{
				"-virtfs local,path=/Users/someone/ze,mount_tag=zelab0,",
				"-virtfs local,path=/scratch/tmp,mount_tag=zelab1,",
				"hostfwd=tcp:127.0.0.1:2222-:22",
				"virtconsole,chardev=console0",
			} {
				if !strings.Contains(argv, want) {
					t.Errorf("argv lacks %q:\n%s", want, argv)
				}
			}
			for _, banned := range []string{"-kernel", "-initrd", "-cdrom", "alpine"} {
				if strings.Contains(argv, banned) {
					t.Errorf("argv carries %q:\n%s", banned, argv)
				}
			}
		})
	}
}

// VALIDATES: the cidata le writes passes the guest's own parser, and every
// share is a 9p mount at its host path (spec R-7).
// METHOD: write the file set to a directory and run dockerhost.ParseCidata,
// the function the guest runs at boot.
// PREVENTS: a volume the guest refuses at boot, which shows only on the
// guest's console, and a checkout mounted where the host le's paths do not
// resolve.
func TestDockerLabCidataPassesTheGuestParser(t *testing.T) {
	guest := dockerLabGuest{
		instanceID: "docker-lab-42-1761040000", publicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest ze-docker-lab",
		shares: []dockerLabShare{{tag: "zelab0", point: "/Users/someone/Code/ze"}},
	}
	dir := t.TempDir()
	for _, file := range guest.cidataFiles() {
		if err := os.WriteFile(filepath.Join(dir, file.name), file.body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	parsed, err := dockerhost.ParseCidata(dir)
	if err != nil {
		t.Fatalf("the guest refuses le's cidata: %v", err)
	}
	if parsed.User != dockerLabUser || parsed.Home != dockerLabHome || parsed.UID != 1001 {
		t.Errorf("user %q home %q uid %d", parsed.User, parsed.Home, parsed.UID)
	}
	if !slices.Equal(parsed.AuthorizedKeys, []string{guest.publicKey}) {
		t.Errorf("authorized keys %q, want the run's key alone", parsed.AuthorizedKeys)
	}
	want := []dockerhost.Mount{{Tag: "zelab0", Point: "/Users/someone/Code/ze", FSType: "9p", Options: ninePOptions}}
	if !slices.Equal(parsed.Mounts, want) {
		t.Errorf("mounts %+v, want %+v", parsed.Mounts, want)
	}
}

// VALIDATES: the host le runs the kernel check probing with the linux ze,
// then the lab as `le <words>`, under `env` with the lab's assignments only,
// and both reach the guest's daemon through DOCKER_HOST.
// PREVENTS: a lab run against the host's own Docker, and a selector that
// reaches the check rather than the lab.
func TestDockerLabHostLeTargetsTheForwardedSocket(t *testing.T) {
	check := dockerLabCheckArgv("/ze", "/ze/tmp/qemu/linux-arm64/ze")
	if got := strings.Join(check, " "); got != "/ze/le setup docker-kernel check ze /ze/tmp/qemu/linux-arm64/ze" {
		t.Errorf("check argv %q", got)
	}
	lab := dockerLabHostArgv("/ze", []string{"test", "integration", "interop-ipsec"}, []string{"IPSEC_INTEROP_SCENARIO=mobike-initiator"})
	if got := strings.Join(lab, " "); got != "env IPSEC_INTEROP_SCENARIO=mobike-initiator /ze/le test integration interop-ipsec" {
		t.Errorf("lab argv %q", got)
	}
	environ := dockerLabEnviron([]string{"HOME=/h", "DOCKER_HOST=unix:///Users/h/.colima/ze/docker.sock"}, "/ze/tmp/qemu/docker-lab-1/docker.sock")
	if !slices.Contains(environ, "DOCKER_HOST=unix:///ze/tmp/qemu/docker-lab-1/docker.sock") || slices.Contains(environ, "DOCKER_HOST=unix:///Users/h/.colima/ze/docker.sock") {
		t.Errorf("environ %q does not name the forwarded socket alone", environ)
	}
	if _, err := dockerLabEnvironment("FOO=bar", false); err == nil {
		t.Error("env without a lab was accepted")
	}
	if _, err := dockerLabEnvironment("lower=bar", true); err == nil {
		t.Error("a lower-case env name was accepted")
	}
}

// VALIDATES: the action is registered with its keywords, so `./le test qemu`
// lists it and the grammar accepts `lab`, `env` and `timeout`.
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

// Fakes for TestDockerLabForwardsOneInterruptThenStopsTheVM. Each appends
// what it saw to $LOG, so the test reads the order of events. The ssh fake
// creates the socket file under umask 177, as ssh does under
// StreamLocalBindMask=0177, so the file is 0600 from creation: a create then
// chmod left a window in which forward's poll saw 0644 and failed the run.
const (
	fakeDockerLabQEMU = `#!/bin/sh
trap 'echo "qemu stopped" >> "$LOG"; exit 0' INT TERM
while :; do sleep 0.1; done
`
	fakeDockerLabSSH = `#!/bin/sh
while [ $# -gt 0 ]; do [ "$1" = -L ] && sock="${2%%:*}"; shift; done
(umask 177 && : > "$sock")
trap 'exit 0' TERM
while :; do sleep 0.1; done
`
	fakeDockerLabLe = `#!/bin/sh
echo "le $* DOCKER_HOST=$DOCKER_HOST" >> "$LOG"
[ "$1" = setup ] && exit 0
trap 'echo "le teardown" >> "$LOG"; exit 130' INT
while :; do sleep 0.1; done
`
)

// VALIDATES: spec AC-10's order, over fakes. The host le runs the check, then
// the lab, both with DOCKER_HOST on the forwarded socket; one interrupt of the
// run reaches the lab le once and its own teardown runs; le then lists the
// guest's leftovers, "none", and only after that is the VM stopped; the run
// answers interrupted, exit 1.
// METHOD: dockerLab over a fake QEMU and ssh forward on PATH, a fake le at the
// checkout root, and ops.Run answering ssh-keygen and the guest commands; the
// root context is canceled once the lab le logs its start.
// PREVENTS: a VM stopped under a lab that is still removing its containers,
// a leftover listing that never runs after an interrupt, and an interrupt
// that ends the run with exit 0.
func TestDockerLabForwardsOneInterruptThenStopsTheVM(t *testing.T) {
	run, tree, logPath := fakeDockerLabRun(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan DockerLabReport, 1)
	go func() {
		report, err := run.dockerLab(ctx, dockerLabRequest{image: "/img", linuxZe: "/ze", lab: []string{"test", "lab"}})
		if err != nil {
			t.Errorf("dockerLab: %v", err)
			appendLog(t, logPath, "le test lab (failed)")
		}
		finished <- report
	}()
	for deadline := time.Now().Add(20 * time.Second); !strings.Contains(readLog(t, logPath), "le test lab"); {
		if time.Now().After(deadline) {
			t.Fatalf("the lab le never started; events:\n%s", readLog(t, logPath))
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	report := <-finished

	if !report.Interrupted || dockerLabExitCode(&report, true) != 1 || report.LeftBehind != "none" {
		t.Errorf("report %+v: want interrupted, exit 1, left behind none", report)
	}
	events := strings.Split(strings.TrimSpace(readLog(t, logPath)), "\n")
	if len(events) != 5 {
		t.Fatalf("events %q, want check, lab, teardown, listing, qemu stopped", events)
	}
	socket := "DOCKER_HOST=unix://" + filepath.Join(tree, "tmp", "qemu")
	for i, want := range []string{"le setup docker-kernel check ze /ze " + socket, "le test lab " + socket, "le teardown", "listing", "qemu stopped"} {
		if !strings.HasPrefix(events[i], want) {
			t.Errorf("event %d = %q, want it to start %q", i, events[i], want)
		}
	}
}

// fakeDockerLabRun answers a Run whose QEMU and ssh forward are the fakes on
// PATH and whose checkout root holds the fake le, with ops.Run answering
// ssh-keygen and the guest's leftover listing, and the checkout and the event
// log it writes to.
func fakeDockerLabRun(t *testing.T) (*Run, string, string) {
	t.Helper()
	tree, err := os.MkdirTemp("/tmp", "zdl") // short: the socket path must fit sun_path
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tree) }) //nolint:errcheck // test cleanup
	if err := os.Mkdir(filepath.Join(tree, "tmp"), 0o700); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	for name, body := range map[string]string{runQEMUBinary("amd64"): fakeDockerLabQEMU, sshProgram: fakeDockerLabSSH} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // an executable test fixture
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tree, "le"), []byte(fakeDockerLabLe), 0o700); err != nil { //nolint:gosec // an executable test fixture
		t.Fatal(err)
	}
	logPath := filepath.Join(tree, "events.log")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LOG", logPath)

	run := NewRun(tree, RunOptions{Memory: "512", CPUs: "1", Boot: 10 * time.Second, Timeout: time.Minute, SSHPort: 2222, HardwareOnly: true})
	// The fake guest mounts nothing; the tree sits under /tmp for sun_path.
	run.shareMountRoots = append(run.shareMountRoots, "/tmp")
	run.ops.GOARCH = "amd64"
	run.ops.Accelerator = func() string { return acceleratorKVM }
	run.ops.Sleep = func(context.Context, time.Duration) error { return nil }
	run.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		if spec.Name == "ssh-keygen" {
			key := spec.Args[len(spec.Args)-1]
			if err := os.WriteFile(key+".pub", []byte("ssh-ed25519 AAAAtest ze-docker-lab\n"), 0o600); err != nil {
				return commandResult{}, err
			}
			return commandResult{}, nil
		}
		remote := spec.Args[len(spec.Args)-1]
		if strings.HasPrefix(remote, "left=") {
			appendLog(t, logPath, "listing")
			spec.Stdout.Write([]byte("docker-lab: left behind: none\n")) //nolint:errcheck // a strings.Builder
		}
		return commandResult{}, nil
	}
	return run, tree, logPath
}

// VALIDATES: with no lab the run ends at the host le's kernel check: no lab
// le starts, the VM stops, and the run answers the check's exit, 0.
// METHOD: dockerLab over the fakes of TestDockerLabForwardsOneInterruptThenStopsTheVM
// with an empty lab, reading the event log.
// PREVENTS: an empty lab running `le` bare, which lists actions and exits 0,
// read as a passing lab.
func TestDockerLabWithoutALabEndsAtTheCheck(t *testing.T) {
	run, tree, logPath := fakeDockerLabRun(t)
	report, err := run.dockerLab(t.Context(), dockerLabRequest{image: "/img", linuxZe: "/ze"})
	if err != nil {
		t.Fatalf("dockerLab: %v", err)
	}
	if report.Interrupted || report.CheckExit != 0 || dockerLabExitCode(&report, false) != 0 {
		t.Errorf("report %+v: want the check's exit 0, not interrupted", report)
	}
	events := strings.Split(strings.TrimSpace(readLog(t, logPath)), "\n")
	socket := "DOCKER_HOST=unix://" + filepath.Join(tree, "tmp", "qemu")
	if len(events) == 0 || !strings.HasPrefix(events[0], "le setup docker-kernel check ze /ze "+socket) {
		t.Fatalf("events %q, want the kernel check first", events)
	}
	for _, event := range events[1:] {
		if strings.HasPrefix(event, "le ") {
			t.Errorf("a host le ran after the check with no lab: %q", event)
		}
	}
	if events[len(events)-1] != "qemu stopped" {
		t.Errorf("events %q, want the VM stopped last", events)
	}
}

func appendLog(t *testing.T, path, line string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the test's own file
	if err != nil {
		t.Error(err)
		return
	}
	defer file.Close()            //nolint:errcheck // test fixture
	file.WriteString(line + "\n") //nolint:errcheck // test fixture
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // the test's own file
	if err != nil {
		return ""
	}
	return string(data)
}

// TestDockerLabRefusesAShareTheGuestCannotMount checks share points under
// each writable guest root, on a root itself and outside every root.
//
// VALIDATES: review R1-4: a share outside /Users, /home and /mnt is refused
// before the VM boots, naming the share, why, and the fix.
// PREVENTS: a checkout under /opt or /private/tmp failing to mount inside
// the guest, reported only on the guest's console.
func TestDockerLabRefusesAShareTheGuestCannotMount(t *testing.T) {
	for _, point := range []string{"/Users/someone/Code/ze", "/home/runner/work/ze", "/mnt/cache"} {
		if err := refuseUnmountableShare(point, dockerhost.ShareMountRoots()); err != nil {
			t.Errorf("refused %s: %v", point, err)
		}
	}
	for _, point := range []string{"/opt/ze", "/private/tmp/ze", "/Users", "/homework/ze"} {
		err := refuseUnmountableShare(point, dockerhost.ShareMountRoots())
		if err == nil {
			t.Errorf("accepted %s, which the guest cannot mount", point)
			continue
		}
		for _, want := range []string{point, "read-only root", "/Users, /home", "Fix:"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal of %s lacks %q: %v", point, want, err)
			}
		}
	}
}

// TestDockerLabRunRefusesAShareBeforeQEMUStarts proves the share refusal is
// reached through the run's entry point, not only through the helper.
//
// VALIDATES: review R2-3: dockerLab over a checkout outside every writable
// guest root (the fake run's tree under /tmp, with the production roots)
// answers the share refusal, and neither ssh-keygen nor QEMU ever runs.
// METHOD: the fakes of TestDockerLabForwardsOneInterruptThenStopsTheVM with
// shareMountRoots reset to dockerhost.ShareMountRoots(), ops.Run and
// ops.Start recording every call.
// PREVENTS: a refactor of dockerLabPrepare that drops the dockerLabShares
// call or moves it after the boot, which the helper's own test cannot see.
func TestDockerLabRunRefusesAShareBeforeQEMUStarts(t *testing.T) {
	run, tree, logPath := fakeDockerLabRun(t)
	run.shareMountRoots = dockerhost.ShareMountRoots()
	var started []string
	run.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		started = append(started, spec.Name)
		return commandResult{}, nil
	}
	run.ops.Start = func(command *exec.Cmd) error {
		started = append(started, command.Path)
		return errors.New("QEMU started after the share refusal should have stopped the run")
	}

	_, err := run.dockerLab(t.Context(), dockerLabRequest{image: "/img", linuxZe: "/ze"})
	if err == nil {
		t.Fatalf("dockerLab over %s outside every guest root succeeded, want the share refusal", tree)
	}
	for _, want := range []string{"docker-lab cannot share " + tree, "read-only root", "Fix:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("dockerLab error lacks %q: %v", want, err)
		}
	}
	if len(started) != 0 {
		t.Errorf("commands ran before the share refusal: %q", started)
	}
	if events := readLog(t, logPath); events != "" {
		t.Errorf("the run reached the guest: events %q", events)
	}
}
