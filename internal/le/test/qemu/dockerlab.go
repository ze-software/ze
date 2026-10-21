// Design: docs/architecture/testing/qemu-integration.md -- Docker labs on the docker-host image
// Detail: cidata.go -- the cidata volume the guest reads at boot
// Related: run.go -- the QEMU acceleration, firmware and VM records this boot reuses
// Related: ../../setup/dockerhost/actions.go -- EnsureImage, the image this boot runs
// Related: ../../setup/dockerkernel/dockerkernel.go -- `setup docker-kernel check`, run by the host le
//
// dockerlab.go is the one path the Mac (HVF, arm64) and the scheduled nightly
// (KVM, amd64) share for a Docker lab that runs Ze (spec D-6): boot Ze's
// docker-host image for the host's architecture, the same image the ze Colima
// profile runs, with a cidata volume le writes; forward the guest's Docker
// socket to a socket under the checkout's tmp/qemu; then the HOST le runs the
// Docker kernel check and the lab against it through DOCKER_HOST. The guest
// runs dockerd and nothing of le's. The checkout is shared over 9p at its own
// absolute path, so a bind mount the host le names resolves in the guest
// (spec R-7).

package testqemu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/linuxle"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	setupdockerhost "github.com/ze-software/ze/internal/le/setup/dockerhost"
)

const (
	dockerLabVerb = "docker-lab"
	keywordLab    = "lab"
	// keywordEnv carries NAME=value assignments the lab runs under, such as
	// IPSEC_INTEROP_SCENARIO=mobike-initiator: the suites select one scenario
	// from the environment.
	keywordEnv = "env"
	// dockerLabUser is the cidata user le logs in as. Its uid is not the
	// docker gid (1000), so the user keeps its own primary group and joins
	// docker as a member (dockerhost groupLines).
	dockerLabUser = "ze"
	dockerLabUID  = "1001"
	dockerLabHome = "/home/ze.linux"
	// dockerLabHostname is the guest's host name.
	dockerLabHostname = "docker-lab"
	// dockerLabGuestSocket is dockerd's socket in the guest (dockerhost
	// docker.go, the docker.service argv).
	dockerLabGuestSocket = "/var/run/docker.sock"
	// dockerReadySeconds bounds the wait for dockerd to answer after boot.sh
	// starts it.
	dockerReadySeconds = "120"
	// dockerLabForwardWait bounds the wait for ssh to create the local socket.
	dockerLabForwardWait = 15 * time.Second
	// dockerLabForwardPoll is how often that wait looks for the socket.
	dockerLabForwardPoll = 100 * time.Millisecond
	// dockerLabTeardownGrace bounds a host le's own teardown after the
	// interrupt le forwards to it: a lab removes its containers, networks and
	// images, which takes seconds; past this it is killed.
	dockerLabTeardownGrace = 5 * time.Minute
	// dockerLabListingTimeout bounds the leftover listing, which runs after
	// the run's own root may already have ended.
	dockerLabListingTimeout = 2 * time.Minute
	// unixSocketPathMax is the longest socket path both hosts bind: macOS
	// holds 104 octets in sun_path, Linux 108, each with its terminating NUL.
	unixSocketPathMax = 103
	// ninePOptions are the 9p mount options of every share: virtio
	// transport, the Linux dialect, and no guest cache, so a file the host
	// writes is what the guest's dockerd bind-mounts.
	ninePOptions = "trans=virtio,version=9p2000.L,msize=262144,cache=none"
)

// dockerLabBootScript is the boot.sh on the cidata volume. Under Lima it is
// Lima's own, which runs Lima's provisioning; the QEMU guest has no Lima host
// agent and no Colima, so le's boot.sh starts what Colima's provisioning
// starts, containerd then dockerd, through the systemctl shim.
const dockerLabBootScript = `#!/bin/sh
# Written by ./le test qemu docker-lab (internal/le/test/qemu/dockerlab.go).
set -eu
systemctl start containerd.service
systemctl start docker.service
touch /run/lima-boot-done
`

// dockerLabShare is one 9p export, mounted in the guest at point, which is
// also its host path.
type dockerLabShare struct {
	tag   string
	point string
}

// dockerLabGuest is what the cidata volume tells the guest.
type dockerLabGuest struct {
	instanceID string
	// publicKey is the one authorized key, the run's own.
	publicKey string
	shares    []dockerLabShare
}

// cidataFiles answers Lima's file set for the guest: lima.env, meta-data,
// user-data and boot.sh, the files dockerhost ParseCidata and BootScript read.
func (g dockerLabGuest) cidataFiles() []cidataFile {
	var limaEnv textbuf.Buffer
	limaEnv.Str("LIMA_CIDATA_IID=").Str(g.instanceID).Byte('\n').
		Str("LIMA_CIDATA_NAME=").Str(dockerLabHostname).Byte('\n').
		Str("LIMA_CIDATA_USER=").Str(dockerLabUser).Byte('\n').
		Str("LIMA_CIDATA_UID=").Str(dockerLabUID).Byte('\n').
		Str("LIMA_CIDATA_COMMENT=ze docker-lab\n").
		Str("LIMA_CIDATA_HOME=").Str(dockerLabHome).Byte('\n').
		Str("LIMA_CIDATA_SHELL=/bin/sh\n").
		Str("LIMA_CIDATA_VSOCK_PORT=0\n")
	var meta textbuf.Buffer
	meta.Str("instance-id: ").Str(g.instanceID).Str("\nlocal-hostname: ").Str(dockerLabHostname).Byte('\n')
	var user textbuf.Buffer
	user.Str("#cloud-config\nmounts:\n")
	for _, share := range g.shares {
		user.Str("- [").Str(strconv.Quote(share.tag)).Str(", ").Str(strconv.Quote(share.point)).
			Str(", \"9p\", \"").Str(ninePOptions).Str("\", \"0\", \"0\"]\n")
	}
	user.Str("users:\n  - name: ").Str(strconv.Quote(dockerLabUser)).
		Str("\n    ssh-authorized-keys:\n      - ").Str(strconv.Quote(g.publicKey)).Byte('\n')
	return []cidataFile{
		{name: "boot.sh", body: []byte(dockerLabBootScript)},
		{name: "lima.env", body: []byte(limaEnv.String())},
		{name: "meta-data", body: []byte(meta.String())},
		{name: "user-data", body: []byte(user.String())},
	}
}

// dockerLabVM is one boot of the docker-host image.
type dockerLabVM struct {
	goarch      string
	accelerator string
	// firmware is the arm64 UEFI firmware; amd64 boots SeaBIOS, QEMU's own.
	firmware string
	image    string
	cidata   string
	console  string
	memory   string
	cpus     string
	sshPort  int
	shares   []dockerLabShare
}

// argv answers QEMU's argv. The image is opened with snapshot=on, so the
// guest's writes to /perm go to a temporary overlay and the cached image is
// never changed (spec AC-9). The cidata volume is the last virtio disk, where
// Lima attaches it. The console is a virtio console (hvc0, the image's last
// console=), written to console.
func (vm dockerLabVM) argv() []string {
	var b textbuf.Buffer
	argv := []string{runQEMUBinary(vm.goarch)}
	if vm.goarch == ArchARM64 {
		argv = append(argv, "-machine", b.Str("virt,highmem=on,accel=").Str(vm.accelerator).String(),
			"-cpu", "max", "-bios", vm.firmware)
	} else {
		argv = append(argv, "-machine", b.Str("q35,accel=").Str(vm.accelerator).String())
	}
	b.Reset()
	image := b.Str("file=").Str(vm.image).Str(",format=raw,if=virtio,snapshot=on").String()
	b.Reset()
	cidata := b.Str("file=").Str(vm.cidata).Str(",format=raw,if=virtio,readonly=on").String()
	b.Reset()
	forward := b.Str("user,id=net0,hostfwd=tcp:127.0.0.1:").Int(int64(vm.sshPort)).Str("-:22").String()
	b.Reset()
	console := b.Str("file,id=console0,path=").Str(vm.console).String()
	argv = append(argv,
		"-smp", vm.cpus, "-m", vm.memory,
		"-drive", image, "-drive", cidata,
		"-netdev", forward, "-device", "virtio-net-pci,netdev=net0",
		"-display", "none", "-monitor", "none", "-serial", "none",
		"-device", "virtio-serial-pci", "-chardev", console, "-device", "virtconsole,chardev=console0",
	)
	for _, share := range vm.shares {
		b.Reset()
		export := b.Str("local,path=").Str(share.point).Str(",mount_tag=").Str(share.tag).
			Str(",security_model=none,id=").Str(share.tag).String()
		argv = append(argv, "-virtfs", export)
	}
	return argv
}

// dockerLabRequest is one docker-lab run: the image, the linux ze the kernel
// check probes with, and the lab with its environment, empty for none.
type dockerLabRequest struct {
	image       string
	linuxZe     string
	lab         []string
	environment []string
}

// DockerLabReport is what a docker-lab run answers.
type DockerLabReport struct {
	Image       string `json:"image"`
	Accelerator string `json:"accelerator"`
	Console     string `json:"console"`
	CheckExit   int    `json:"check-exit"`
	LabExit     int    `json:"lab-exit"`
	Interrupted bool   `json:"interrupted"`
	LeftBehind  string `json:"left-behind,omitempty"`
}

// runDockerLabHere runs the verb over this checkout under its signal root, so
// a SIGINT, SIGTERM or SIGHUP ends every phase: the image build, the linux ze
// build, the boot, and the host le, which takes the interrupt and runs its own
// teardown before the VM stops.
func runDockerLabHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	ctx, cancel := guestContext()
	defer cancel()

	return runDockerLab(ctx, root, args)
}

// runDockerLab is the verb over the checkout at root, under its signal root
// ctx.
func runDockerLab(ctx context.Context, root string, args leaction.Arguments) (any, int) {
	goarch := GuestArch()
	lab := strings.Fields(args.One(keywordLab))
	environment, err := dockerLabEnvironment(args.One(keywordEnv), len(lab) > 0)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	runArgs := leaction.Arguments{keywordCommand: {dockerLabCommandLine(root, lab, environment)}}
	if timeout := args.One(keywordTimeout); timeout != "" {
		runArgs[keywordTimeout] = []string{timeout}
	}
	options, err := parseRunArguments(runArgs)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	// A lab under TCG measures the emulator and times out; HVF or KVM or nothing.
	options.HardwareOnly = true
	image, err := setupdockerhost.EnsureImage(ctx, root, goarch)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	linuxZe, err := BuildLinuxZe(ctx, root, goarch)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	report, err := NewRun(root, options).dockerLab(ctx, dockerLabRequest{
		image: image, linuxZe: linuxZe, lab: lab, environment: environment,
	})
	if err != nil {
		leaction.ReportError(err)
		return &report, 1
	}
	return &report, dockerLabExitCode(&report, len(lab) > 0)
}

// dockerLabExitCode is 1 for an interrupted run (spec AC-10), then the
// check's exit status, then the lab's.
func dockerLabExitCode(report *DockerLabReport, lab bool) int {
	if report.Interrupted {
		return 1
	}
	if report.CheckExit != 0 {
		return report.CheckExit
	}
	if lab {
		return report.LabExit
	}
	return 0
}

// dockerLabCommandLine is the host le line a run executes, for the report:
// `le <lab>` under its environment, or the kernel check with no lab.
func dockerLabCommandLine(root string, lab, environment []string) string {
	if len(lab) == 0 {
		return strings.Join(dockerLabCheckArgv(root, GuestZeRel(GuestArch())), " ")
	}
	return strings.Join(dockerLabHostArgv(root, lab, environment), " ")
}

// dockerLabCheckArgv is the host le's Docker kernel check, probing with the
// linux ze at linuxZe.
func dockerLabCheckArgv(root, linuxZe string) []string {
	return []string{filepath.Join(root, "le"), "setup", "docker-kernel", "check", "ze", linuxZe}
}

// dockerLabHostArgv is the host le running the lab, under `env` with the
// lab's assignments when it has any. DOCKER_HOST reaches it through its
// environment (dockerLabEnviron), not its argv.
func dockerLabHostArgv(root string, lab, environment []string) []string {
	argv := make([]string, 0, 2+len(environment)+len(lab))
	if len(environment) > 0 {
		argv = append(argv, "env")
		argv = append(argv, environment...)
	}
	argv = append(argv, filepath.Join(root, "le"))
	return append(argv, lab...)
}

// dockerLabEnviron is the host le's environment: the caller's, with
// DOCKER_HOST naming the forwarded socket.
func dockerLabEnviron(environ []string, socket string) []string {
	return setEnv(environ, "DOCKER_HOST", "unix://"+socket)
}

// dockerLab boots the image, forwards its Docker socket and runs the host le
// against it. Operating failures return an error; the check's and the lab's
// exit statuses are report data.
func (r *Run) dockerLab(ctx context.Context, request dockerLabRequest) (DockerLabReport, error) {
	report := DockerLabReport{Image: request.image}
	vm, dir, err := r.dockerLabPrepare(ctx, request.image)
	if dir != "" {
		defer os.RemoveAll(dir) //nolint:errcheck // a per-run directory; the next run makes its own
	}
	if err != nil {
		return report, err
	}
	report.Accelerator, report.Console = vm.accelerator, vm.console
	socket := filepath.Join(dir, "docker.sock")

	stopVMOnce, err := r.dockerLabBoot(vm, dir)
	if err != nil {
		return report, err
	}
	defer stopVMOnce()
	session := dockerLabSession{run: r, key: filepath.Join(dir, "id_ed25519"), port: vm.sshPort}
	if err := session.waitReady(ctx, r.Options.Boot); err != nil {
		if ctx.Err() != nil {
			// The root ended the boot: say so, not "context canceled".
			report.Interrupted = true
			return report, fmt.Errorf("docker-lab stopped by an interrupt or SIGTERM while the guest booted, before any check or lab ran;"+
				" the VM is stopped. Rerun the same command to try again (guest console: %s)", vm.console)
		}
		return report, fmt.Errorf("%w; the guest console is %s", err, vm.console)
	}
	stopForward, err := session.forward(socket)
	if err != nil {
		return report, err
	}
	defer stopForward()

	environ := dockerLabEnviron(r.ops.Environ(), socket)
	report.CheckExit, report.Interrupted, err = r.runHostLe(ctx, dockerLabCheckArgv(r.Tree, request.linuxZe), environ)
	if err != nil {
		return report, err
	}
	if report.Interrupted || report.CheckExit != 0 || len(request.lab) == 0 {
		return report, nil
	}
	labCtx, cancel := context.WithTimeout(ctx, r.Options.Timeout)
	defer cancel()
	report.LabExit, report.Interrupted, err = r.runHostLe(labCtx, dockerLabHostArgv(r.Tree, request.lab, request.environment), environ)
	if err != nil {
		return report, err
	}
	report.LeftBehind = session.leftBehind()
	return report, nil
}

// dockerLabPrepare answers the VM to boot and the run's directory under the
// checkout's tmp/qemu, holding the run's ssh key, the cidata volume, the
// forwarded socket and QEMU's diagnostics. The caller MUST remove the
// directory when it is not empty, error or not.
func (r *Run) dockerLabPrepare(ctx context.Context, image string) (dockerLabVM, string, error) {
	vm := dockerLabVM{goarch: r.ops.GOARCH, image: image, memory: r.Options.Memory, cpus: r.Options.CPUs}
	if _, err := r.ops.Look(runQEMUBinary(vm.goarch)); err != nil {
		return vm, "", fmt.Errorf("missing required command %s: install QEMU (brew install qemu, apt-get install qemu-system): %w",
			runQEMUBinary(vm.goarch), err)
	}
	var err error
	if vm.accelerator, err = r.acceleration(); err != nil {
		return vm, "", err
	}
	if vm.goarch == ArchARM64 {
		if vm.firmware, err = r.arm64BIOS(); err != nil {
			return vm, "", err
		}
	}
	if vm.sshPort = r.Options.SSHPort; vm.sshPort == 0 {
		if vm.sshPort, err = r.ops.Port(); err != nil {
			return vm, "", err
		}
	}
	if vm.shares, err = r.dockerLabShares(); err != nil {
		return vm, "", err
	}
	parent := r.vmRecordDir()
	if err := r.ops.FS.MkdirAll(parent, 0o750); err != nil {
		return vm, "", fmt.Errorf("create %s: %w", parent, err)
	}
	dir, err := r.ops.FS.MkdirTemp(parent, "docker-lab-")
	if err != nil {
		return vm, "", fmt.Errorf("create the docker-lab directory under %s: %w", parent, err)
	}
	if socket := filepath.Join(dir, "docker.sock"); len(socket) > unixSocketPathMax {
		return vm, dir, fmt.Errorf("the forwarded Docker socket %s is %d bytes, above the %d a unix socket path holds."+
			" Fix: run from a checkout whose path is at least %d bytes shorter", socket, len(socket), unixSocketPathMax, len(socket)-unixSocketPathMax)
	}
	key := filepath.Join(dir, "id_ed25519")
	generated, err := r.runCommand(ctx, commandSpec{
		Name: "ssh-keygen", Args: []string{"-q", "-t", "ed25519", "-N", "", "-C", "ze-docker-lab", "-f", key},
		Env: r.ops.Environ(), Stdin: strings.NewReader(""),
	})
	if err != nil {
		return vm, dir, fmt.Errorf("generate the run's ssh key: %w", err)
	}
	if generated.Code != 0 {
		return vm, dir, fmt.Errorf("ssh-keygen exited %d generating the run's key: %s", generated.Code, generated.Stderr)
	}
	public, err := r.ops.FS.ReadFile(key + ".pub")
	if err != nil {
		return vm, dir, fmt.Errorf("read the run's ssh key: %w", err)
	}
	var id textbuf.Buffer
	guest := dockerLabGuest{
		instanceID: id.Str("docker-lab-").Int(int64(os.Getpid())).Byte('-').Int(r.ops.Now().Unix()).String(),
		publicKey:  strings.TrimSpace(string(public)), shares: vm.shares,
	}
	vm.cidata = filepath.Join(dir, "cidata.iso")
	if err := writeCidataISO(vm.cidata, guest.cidataFiles()); err != nil {
		return vm, dir, err
	}
	var b textbuf.Buffer
	vm.console = filepath.Join(parent, b.Str("docker-lab-console-").Int(int64(vm.sshPort)).Str(".log").String())
	return vm, dir, nil
}

// dockerLabShares answers the 9p exports: the checkout, then every directory
// its tmp and cache links lead out of it (scratchShares), each mounted at its
// own host path, so a path the host le resolves is the same file in the guest.
func (r *Run) dockerLabShares() ([]dockerLabShare, error) {
	shares := []dockerLabShare{{tag: "zelab0", point: r.Tree}}
	linked, err := r.scratchShares()
	if err != nil {
		return nil, err
	}
	for i, share := range linked {
		shares = append(shares, dockerLabShare{tag: "zelab" + strconv.Itoa(i+1), point: share.Host})
	}
	return shares, nil
}

// dockerLabBoot starts QEMU and records it for the reap a later run does if
// this le dies. It answers the stop, which the caller MUST call once.
func (r *Run) dockerLabBoot(vm dockerLabVM, dir string) (func(), error) {
	r.reapOrphanVMs(os.Stderr)
	errorLog, err := os.Create(filepath.Join(dir, "qemu-errors.log")) //nolint:gosec // the run's own directory
	if err != nil {
		return nil, fmt.Errorf("create the QEMU diagnostics file: %w", err)
	}
	argv := vm.argv()
	var b textbuf.Buffer
	fmt.Fprintln(os.Stderr, b.Str("Booting the docker-host image under ").Str(vm.accelerator). //nolint:errcheck // progress output
													Str(" (ssh forwarded on 127.0.0.1:").Int(int64(vm.sshPort)).Str(", console ").Str(vm.console).Str(")...").String())
	command := exec.Command(argv[0], argv[1:]...) //nolint:gosec,noctx // argv is dockerLabVM.argv's; stopVM owns the stop
	command.SysProcAttr = childProcAttr()
	command.Env = r.ops.Environ()
	command.Stdout = errorLog
	command.Stderr = errorLog
	if err := r.ops.Start(command); err != nil {
		errorLog.Close() //nolint:errcheck // nothing was written
		return nil, fmt.Errorf("start %s: %w", argv[0], err)
	}
	removeRecord, err := r.recordVM(command.Process.Pid)
	if err != nil {
		stopVM(command)
		errorLog.Close() //nolint:errcheck // the VM is stopped
		return nil, fmt.Errorf("record the VM so a later run can stop it if this le dies: %w", err)
	}
	return func() {
		stopVM(command)
		removeRecord()
		errorLog.Close()                                   //nolint:errcheck // the VM is stopped
		fmt.Fprintln(os.Stderr, "docker-lab: VM stopped.") //nolint:errcheck // progress output
	}, nil
}

// runHostLe runs argv, a host le, with environ, streaming its output, and
// answers its exit status. The child leads its own process group, so a
// Ctrl-C at the terminal reaches this le alone; when ctx ends, le sends the
// child's group one SIGINT, the interrupt a terminal would have sent, and
// waits up to dockerLabTeardownGrace for the child's own teardown before it
// kills the group. interrupted reports that ctx ended the child.
func (r *Run) runHostLe(ctx context.Context, argv, environ []string) (int, bool, error) {
	command := exec.Command(argv[0], argv[1:]...) //nolint:gosec,noctx // a host le argv this file builds; the stop is the forwarded SIGINT
	command.Dir = r.Tree
	command.Env = environ
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := r.ops.Start(command); err != nil {
		return 0, false, fmt.Errorf("start %s: %w", argv[0], err)
	}
	done := make(chan struct{})
	go waitChild(command, done)

	select {
	case <-done:
		return command.ProcessState.ExitCode(), false, nil
	case <-ctx.Done():
	}
	fmt.Fprintln(os.Stderr, "docker-lab: interrupt forwarded to the host le; waiting for its teardown...") //nolint:errcheck // progress output
	syscall.Kill(-command.Process.Pid, syscall.SIGINT)                                                     //nolint:errcheck // a group already gone has nothing to tear down
	select {
	case <-done:
	case <-time.After(dockerLabTeardownGrace):
		syscall.Kill(-command.Process.Pid, syscall.SIGKILL) //nolint:errcheck // a group already gone needs no kill
		<-done
	}
	return command.ProcessState.ExitCode(), true, nil
}

// waitChild owns a child's one Wait and closes done when it returns.
func waitChild(command *exec.Cmd, done chan<- struct{}) {
	defer close(done)
	command.Wait() //nolint:errcheck // the exit status is read from ProcessState
}

// dockerLabSession reaches the guest as the cidata user over the forwarded
// ssh port, with the run's own key.
type dockerLabSession struct {
	run  *Run
	key  string
	port int
}

// args answers ssh's options and destination, before any command.
func (s dockerLabSession) args(extra ...string) []string {
	args := []string{
		"-i", s.key, "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR", "-o", "ConnectTimeout=5", "-p", strconv.Itoa(s.port),
	}
	args = append(args, extra...)
	return append(args, dockerLabUser+"@127.0.0.1")
}

// remote runs command in the guest under ctx and answers its result.
func (s dockerLabSession) remote(ctx context.Context, command string, stdout io.Writer) (commandResult, error) {
	return s.run.runCommand(ctx, commandSpec{
		Name: sshProgram, Args: append(s.args(), command), Env: s.run.ops.Environ(),
		Stdin: strings.NewReader(""), Stdout: stdout, OwnGroup: true,
	})
}

// waitReady waits up to boot for the guest's sshd, then a bounded time for
// its dockerd to answer.
func (s dockerLabSession) waitReady(ctx context.Context, boot time.Duration) error {
	deadline := s.run.ops.Now().Add(boot)
	for {
		result, err := s.remote(ctx, "true", nil)
		if err != nil {
			return err
		}
		if result.Code == 0 {
			break
		}
		if !s.run.ops.Now().Before(deadline) {
			return fmt.Errorf("the docker-host guest's sshd did not answer within %s", boot)
		}
		if err := s.run.ops.Sleep(ctx, sshPollInterval); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, "  guest up; waiting for dockerd...") //nolint:errcheck // progress output
	result, err := s.remote(ctx, "timeout "+dockerReadySeconds+" sh -c 'until docker info >/dev/null 2>&1; do sleep 1; done'", nil)
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return errors.New("the guest's dockerd did not answer within " + dockerReadySeconds + "s of sshd")
	}
	return nil
}

// forward starts ssh forwarding socket, mode 0600, to the guest's Docker
// socket, waits for it, and answers the stop, which the caller MUST call.
// The ssh client leads its own group, so a Ctrl-C leaves the forward up for
// the host le's teardown.
func (s dockerLabSession) forward(socket string) (func(), error) {
	args := s.args("-N", "-o", "ExitOnForwardFailure=yes", "-o", "StreamLocalBindMask=0177",
		"-o", "StreamLocalBindUnlink=yes", "-L", socket+":"+dockerLabGuestSocket)
	command := exec.Command(sshProgram, args...) //nolint:gosec,noctx // ssh with argv this file builds; the stop is below
	command.Env = s.run.ops.Environ()
	command.Stderr = os.Stderr
	command.SysProcAttr = childProcAttr()
	if err := s.run.ops.Start(command); err != nil {
		return nil, fmt.Errorf("start the Docker socket forward: %w", err)
	}
	done := make(chan struct{})
	go waitChild(command, done)
	stop := func() {
		command.Process.Signal(syscall.SIGTERM) //nolint:errcheck // a forward already gone needs no stop
		<-done
	}
	for waited := time.Duration(0); ; waited += dockerLabForwardPoll {
		info, err := os.Stat(socket)
		if err == nil {
			if info.Mode().Perm() != 0o600 {
				stop()
				return nil, fmt.Errorf("the forwarded Docker socket %s has mode %o, not 0600", socket, info.Mode().Perm())
			}
			return stop, nil
		}
		if waited >= dockerLabForwardWait {
			stop()
			return nil, fmt.Errorf("ssh did not create the forwarded Docker socket %s within %s", socket, dockerLabForwardWait)
		}
		time.Sleep(dockerLabForwardPoll)
	}
}

// dockerLabLeftoverListing is the guest shell line listing what the guest
// dockerd still holds of a lab: containers and networks carrying the ze.lab
// label, and the run-owned ze-interop-retained image tags. The DUT image kept
// under its cache tag is allowed to stay (spec AC-6), so it is not listed. A
// listing dockerd does not answer is reported as unknown, never as none.
const dockerLabLeftoverListing = "left=$(docker ps -a --filter label=ze.lab --format 'container {{.Names}}' && " +
	"docker network ls --filter label=ze.lab --format 'network {{.Name}}' && " +
	"docker images --filter reference=ze-interop-retained --format 'image {{.Repository}}:{{.Tag}}') || " +
	"{ echo 'docker-lab: left behind: unknown, because the guest dockerd did not answer the listing; " +
	"whatever the lab left goes when the VM stops'; exit 0; }; " +
	`left=$(printf '%s\n' "$left" | awk 'NF { printf "%s%s", sep, $0; sep = ", " }'); ` +
	`echo "docker-lab: left behind: ${left:-none}"`

// leftBehind prints and answers the guest's leftover listing. It runs after
// the lab, interrupted or not, under its own bound, because the run's root may
// have ended.
func (s dockerLabSession) leftBehind() string {
	ctx, cancel := context.WithTimeout(context.Background(), dockerLabListingTimeout)
	defer cancel()
	var out strings.Builder
	if _, err := s.remote(ctx, dockerLabLeftoverListing, &out); err != nil {
		out.WriteString("docker-lab: left behind: unknown, because the listing did not run: " + err.Error())
	}
	line := strings.TrimSpace(out.String())
	fmt.Fprintln(os.Stderr, line) //nolint:errcheck // progress output
	return strings.TrimPrefix(line, "docker-lab: left behind: ")
}

// GuestZeRel answers where the host writes the linux ze the Docker kernel check
// probes with, relative to the checkout. The nightly's informational check on
// the runner's own Docker probes with the same file, and so does `le setup
// docker-kernel check` on a Linux host when it is given no ze.
func GuestZeRel(goarch string) string {
	return filepath.Join("tmp", "qemu", "linux-"+goarch, "ze")
}

// BuildLinuxZe cross-builds the linux ze for goarch at GuestZeRel(goarch),
// through job admission, with the daemon's feature tags, and answers its
// absolute path. It is the one producer of the ze the Docker kernel check
// probes with: docker-lab builds it for the guest's daemon, and `le setup
// docker-kernel check` builds it for the daemon in hand. The check runs
// `ze doctor kernel-capabilities` in a container, and a ze_le build cannot
// stand in: started under the name ze, it answers "unknown command: doctor"
// (first HVF boot, 2026-10-10).
//
// ctx is the calling verb's signal root: a done ctx ends the wait for a slot
// and the build (RunContext).
func BuildLinuxZe(ctx context.Context, root, goarch string) (string, error) {
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return "", err
	}
	tags, err := repofeaturetags.DaemonBuildTags(root, repofeaturetags.DaemonBase)
	if err != nil {
		return "", err
	}
	admission, err := job.NewIn(root)
	if err != nil {
		return "", err
	}
	admission.Out = os.Stderr

	output := filepath.Join(root, GuestZeRel(goarch))
	environment := append(toolchain.EnvironmentContext(ctx, gotoolchain.EnvOptions{GOOS: guestOS, GOARCH: goarch}),
		linuxle.Overrides(goarch)...)
	argv := []string{"go", "build", tagsFlag, tags, "-o", output, zeMainPackage}
	if _, code := admission.RunContext(ctx, "qemu-build-ze", argv, root, environment); code != 0 {
		if err := ctx.Err(); err != nil {
			// The root ended the build or the wait for its slot: say so, and
			// not the code, which reads as a failed compile.
			return "", fmt.Errorf("build the linux ze for linux/%s: stopped by an interrupt or SIGTERM: %w", goarch, err)
		}
		return "", fmt.Errorf("build the linux ze for linux/%s exited %d", goarch, code)
	}
	return output, nil
}

// dockerLabEnvironment answers the NAME=value assignments of the `env` value,
// separated by spaces, and refuses an assignment whose name is not an upper-case
// shell variable name, or any assignment when no lab runs to read it.
func dockerLabEnvironment(value string, lab bool) ([]string, error) {
	assignments := strings.Fields(value)
	if len(assignments) == 0 {
		return nil, nil
	}
	if !lab {
		return nil, errors.New("docker-lab env names the environment of a lab, and no lab is named to run under it")
	}
	for _, assignment := range assignments {
		name, _, found := strings.Cut(assignment, "=")
		if !found || !environmentName(name) {
			return nil, fmt.Errorf("docker-lab env takes NAME=value with an upper-case NAME, not %q", assignment)
		}
	}
	return assignments, nil
}

// environmentName reports whether name is an upper-case shell variable name:
// a letter or underscore, then letters, digits or underscores.
func environmentName(name string) bool {
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return false
	}
	for i := range len(name) {
		c := name[i]
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}
