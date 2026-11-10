// Design: docs/architecture/testing/qemu-integration.md -- One command in Ze's own system
// Related: dockerlab.go -- the shared image, cidata, mounts and VM ownership
// Related: run_guestcommand.go -- the command's interrupt, teardown and join

package testqemu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	setupdockerhost "github.com/ze-software/ze/internal/le/setup/docker/host"
	"github.com/ze-software/ze/internal/thirdparty/fat"
)

// dockerLabKernel records the explicit request and its matching image runtime
// identity. Only dockerLabCommandKernel constructs one for execution.
type dockerLabKernel struct {
	Requested string `json:"requested"`
	Selected  string `json:"selected"`
	Runtime   string `json:"runtime"`
	SHA256    string `json:"sha256"`
	Release   string `json:"module-release"`
}

// dockerLabMode refuses incompatible modes before any image or build work.
func dockerLabMode(args leaction.Arguments) error {
	if args.Has(keywordCommand) {
		if args.Has(keywordLab) {
			return errors.New("docker-lab command and lab are mutually exclusive. Fix: name only one")
		}
		if strings.TrimSpace(args.One(keywordCommand)) == "" {
			return errors.New("docker-lab command is empty. Fix: supply a guest shell command")
		}
		if args.One(keywordKernel) == "" {
			return errors.New("docker-lab command requires kernel <vmlinuz-path>. Fix: supply the image's matching runtime kernel")
		}
		return nil
	}
	if args.Has(keywordKernel) {
		return errors.New("docker-lab kernel is only valid with command. Fix: supply command or omit kernel for a Docker lab")
	}
	if args.Has(KeywordHarness) {
		return errors.New("docker-lab harness is only valid with command. Fix: supply the guest command it prepares for")
	}
	if args.Has(KeywordRoot) {
		return errors.New("docker-lab root is only valid with command. Fix: supply the guest command to run as root")
	}
	return nil
}

// dockerLabCommandHost holds the host steps of the command branch: a kernel, an
// image and a guest le are each minutes of work, and the boot needs QEMU. A test
// replaces them to read what the branch hands each one.
type dockerLabCommandHost struct {
	runtimeTree func(root, arch string) (string, error)
	ensureImage func(ctx context.Context, root, arch string) (string, error)
	buildGuest  func(ctx context.Context, root, arch string) error
	boot        func(ctx context.Context, run *Run, image string, kernel *dockerLabKernel) (DockerLabReport, error)
}

// dockerLabHost is the command branch's steps, a variable for the reason runSelf
// is. Nothing writes it outside a test.
var dockerLabHost = dockerLabCommandHost{
	runtimeTree: func(root, arch string) (string, error) {
		return appliance.ImageRuntimeKernelTree(root, arch, dockerLabImageBuild)
	},
	ensureImage: setupdockerhost.EnsureImage,
	buildGuest:  buildGuestLe,
	boot: func(ctx context.Context, run *Run, image string, kernel *dockerLabKernel) (DockerLabReport, error) {
		return run.dockerLabCommand(ctx, image, kernel)
	},
}

// runDockerLabCommand resolves the same immutable runtime tree EnsureImage uses,
// refusing a mismatched request before an image build can start.
func runDockerLabCommand(ctx context.Context, root, arch string, options RunOptions) (any, int) {
	tree, err := dockerLabHost.runtimeTree(root, arch)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	kernel, err := dockerLabCommandKernel(options.Kernel, tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	image, err := dockerLabHost.ensureImage(ctx, root, arch)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if options.GuestHarness {
		// After the caller's own builds, under the environment it restored, so
		// the guest le never carries a discrimination overlay.
		if err := dockerLabHost.buildGuest(ctx, root, arch); err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
	}
	if options.guestCommandAsRoot() {
		options.Command = dockerLabRootCommand(options.Command)
	}
	options.Kernel = kernel.Selected
	report, err := dockerLabHost.boot(ctx, NewRun(root, options), image, kernel)
	if err != nil {
		leaction.ReportError(err)
		return &report, 1
	}
	return &report, report.CommandExit
}

// dockerLabImageBuild answers the command that builds the image for arch, which
// a refusal for a kernel this host cannot build names.
func dockerLabImageBuild(arch string) string {
	return "./le setup docker host build arch " + arch
}

// GuestRuntimeKernel answers the vmlinuz of the runtime kernel the image for arch
// boots, for a caller with no kernel of its own to name: the bytes the docker-lab
// command branch accepts, resolved or built the way EnsureImage resolves them.
func GuestRuntimeKernel(root, arch string) (string, error) {
	tree, err := appliance.ImageRuntimeKernelTree(root, arch, dockerLabImageBuild)
	if err != nil {
		return "", err
	}
	return filepath.Join(tree, "vmlinuz"), nil
}

// dockerLabHarnessSetup answers the setup step of a harness command: the checkout
// bound at GuestWorkspace, where `test qemu all` requires it, then a guest-only
// tmpfs over the patched-tree cache, as guestPatchedTree mounts for the same
// reason, and the guest's working directory. The image carries GuestWorkspace as
// an empty directory because its root is a read-only squashfs. The mounts need
// root and the session does not hold it, so each step elevates alone.
func dockerLabHarnessSetup(tree string) string {
	var b textbuf.Buffer
	return b.Str("sudo -n mount -o bind ").Str(shellQuote(tree)).Byte(' ').Str(GuestWorkspace).
		Str(" && sudo -n mkdir -p ").Str(guestStdpatchCache).
		Str(" && sudo -n mount -t tmpfs tmpfs ").Str(guestStdpatchCache).
		Str(" && cd ").Str(GuestWorkspace).String()
}

// The caches a harness command gets by default, and the scratch it works in, on
// the guest's own /tmp: `test qemu all` refuses an unnamed cache, and a selected
// .ci compiles nothing. A caller that compiles in the guest assigns its own in
// the command.
const (
	guestHarnessGoCache    = "/tmp/ze-guest-gocache"
	guestHarnessModuleDir  = "/tmp/ze-guest-gomodcache"
	guestHarnessScratchDir = "/tmp"
)

// dockerLabRootCommand answers command as the guest runs it under the harness or
// KeywordRoot: as root, because `test qemu all` prepares kernel state and refuses
// anything else, and a root-declared Go unit loads what only root may, under the
// environment the action reads. sudo resets the environment, so
// everything travels through env, the job stand-in included
// (guestRunParentSetup). HOME and TMPDIR name the guest's own /tmp: the root is
// read-only, and a work directory on the share would carry the host's UID.
// GOFLAGS drops the VCS stamp because the image carries no git.
func dockerLabRootCommand(command string) string {
	var b textbuf.Buffer
	return b.Str("sudo -n env ").Str(job.ParentVariable).Str("=\"$").Str(job.ParentVariable).Str("\"").
		Str(" HOME=").Str(guestHarnessScratchDir).Str(" TMPDIR=").Str(guestHarnessScratchDir).
		Str(" GOCACHE=").Str(guestHarnessGoCache).Str(" GOMODCACHE=").Str(guestHarnessModuleDir).
		Str(" GOFLAGS=-buildvcs=false CGO_ENABLED=0 sh -c ").Str(shellQuote(command)).String()
}

// dockerLabCommandKernel checks bytes rather than the requested filename or
// release string. Unknown module sets cannot become an executable selection.
func dockerLabCommandKernel(requested, tree string) (*dockerLabKernel, error) {
	selected, err := filepath.Abs(requested)
	if err != nil {
		return nil, err
	}
	want, err := dockerLabKernelSHA(filepath.Join(tree, "vmlinuz"))
	if err != nil {
		return nil, fmt.Errorf("read image runtime kernel: %w", err)
	}
	got, err := dockerLabKernelSHA(selected)
	if err != nil {
		return nil, fmt.Errorf("read requested kernel: %w", err)
	}
	if got != want {
		return nil, fmt.Errorf("docker-lab requested kernel %s has SHA256 %s, not image runtime kernel %s (%s). Fix: supply that runtime tree's vmlinuz; custom kernel/module sets are not supported", selected, got, tree, want)
	}
	modules := filepath.Join(tree, "lib", "modules")
	entries, err := os.ReadDir(modules)
	if err != nil {
		return nil, fmt.Errorf("read image runtime modules %s: %w", modules, err)
	}
	if len(entries) != 1 {
		return nil, fmt.Errorf("image runtime modules %s has %d releases, not one known set. Fix: rebuild the runtime kernel and image", modules, len(entries))
	}
	if !entries[0].IsDir() {
		return nil, fmt.Errorf("image runtime modules %s has no release directory. Fix: rebuild the runtime kernel and image", modules)
	}
	release := entries[0].Name()
	index := filepath.Join(modules, release, "modules.dep")
	info, err := os.Stat(index)
	if err != nil {
		return nil, fmt.Errorf("image runtime module index %s: %w", index, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("image runtime module index %s is not a regular file", index)
	}
	return &dockerLabKernel{Requested: requested, Selected: selected, Runtime: tree, SHA256: want, Release: release}, nil
}

// dockerLabKernelSHA streams a kernel once, without copying its bytes into memory.
func dockerLabKernelSHA(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // an explicit local kernel or its resolved immutable cache entry
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // read-only
	return dockerLabReaderSHA(file)
}

// dockerLabReaderSHA also checks the kernel embedded in the cached image.
func dockerLabReaderSHA(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// dockerLabImageBoot reuses the existing GPT and FAT readers to retain the
// image's own root PARTUUID, not a guessed virtio partition number. It also
// checks that this image really carries the selected runtime kernel.
func dockerLabImageBoot(image string, kernel *dockerLabKernel) (string, error) {
	entries, err := installGPTEntries(image)
	if err != nil {
		return "", err
	}
	if len(entries) < 4 {
		return "", fmt.Errorf("docker-host image %s lacks its boot/root-A/root-B/perm partitions", image)
	}
	file, err := os.Open(image) //nolint:gosec // EnsureImage's immutable cached image
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // read-only
	boot := entries[0]
	if boot.Last < boot.First {
		return "", errors.New("docker-host boot partition ends before it starts")
	}
	partition := io.NewSectionReader(file, int64(boot.First)*512, int64(boot.Last-boot.First+1)*512) //nolint:gosec // the existing GPT reader bounds the table; image comes from EnsureImage
	reader, err := fat.NewReader(partition)
	if err != nil {
		return "", fmt.Errorf("read docker-host boot FAT: %w", err)
	}
	at, size, err := reader.Extents("/vmlinuz")
	if err != nil {
		return "", err
	}
	embedded, err := dockerLabReaderSHA(io.NewSectionReader(partition, at, size))
	if err != nil {
		return "", err
	}
	if embedded != kernel.SHA256 {
		return "", fmt.Errorf("docker-host image %s embeds kernel SHA256 %s, not selected runtime %s. Fix: rebuild the image for the selected runtime kernel", image, embedded, kernel.SHA256)
	}
	at, size, err = reader.Extents("/cmdline.txt")
	if err != nil {
		return "", err
	}
	if size > 4096 {
		return "", errors.New("docker-host boot command line exceeds 4096 bytes")
	}
	line, err := io.ReadAll(io.NewSectionReader(partition, at, size))
	if err != nil {
		return "", err
	}
	var root string
	for word := range strings.FieldsSeq(string(line)) {
		if strings.HasPrefix(word, "root=") {
			if root != "" {
				return "", errors.New("docker-host boot command line has multiple root devices")
			}
			root = word
		}
	}
	if !strings.HasPrefix(root, "root=PARTUUID=") {
		return "", errors.New("docker-host image has no root PARTUUID in cmdline.txt. Fix: rebuild the image with the runtime kernel package")
	}
	return strings.TrimSpace(string(line)) + " console=ttyS0 console=hvc0", nil
}

// dockerLabKernelPayload acquires the requested bytes, never the matching cache
// artifact. The caller MUST acquire the payload before starting QEMU, retain
// dir through VM shutdown, and remove it after the VM stops or on any refusal.
// Hashing the copied stream binds validation to QEMU's input.
func dockerLabKernelPayload(kernel *dockerLabKernel, dir string) (*dockerLabKernel, error) {
	requested, err := os.Open(kernel.Selected) //nolint:gosec // the explicitly requested local kernel
	if err != nil {
		return nil, fmt.Errorf("open requested kernel payload: %w", err)
	}
	defer requested.Close() //nolint:errcheck // read-only input
	path := filepath.Join(dir, "kernel-payload")
	payload, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // fresh file inside the run-owned private directory
	if err != nil {
		return nil, fmt.Errorf("create private kernel payload: %w", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(payload, hash), requested)
	if err := errors.Join(copyErr, payload.Close()); err != nil {
		return nil, fmt.Errorf("acquire requested kernel payload: %w", err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != kernel.SHA256 {
		return nil, fmt.Errorf("requested kernel payload changed: copied SHA256 %s, expected %s", actual, kernel.SHA256)
	}
	selected := *kernel
	selected.Selected = path
	return &selected, nil
}

// dockerLabCommand owns exactly the shared VM lifetime, then delegates command
// interruption and SSH join to the existing guest-command lifecycle.
func (r *Run) dockerLabCommand(ctx context.Context, image string, kernel *dockerLabKernel) (DockerLabReport, error) {
	report := DockerLabReport{Image: image, Kernel: kernel}
	cmdline, err := dockerLabImageBoot(image, kernel)
	if err != nil {
		return report, err
	}
	vm, dir, err := r.dockerLabPrepare(ctx, image, "")
	if dir != "" {
		// This defer MUST run after stop below: QEMU owns the selected payload
		// until VM shutdown. A refusal before boot MUST remove partial output.
		defer os.RemoveAll(dir) //nolint:errcheck // a disposable run-owned directory
	}
	if err != nil {
		return report, err
	}
	selected, err := dockerLabKernelPayload(kernel, dir)
	if err != nil {
		return report, err
	}
	report.Kernel = selected
	vm.kernel, vm.cmdline = selected.Selected, cmdline
	report.Accelerator, report.Console = vm.accelerator, vm.console
	stop, err := r.dockerLabBoot(vm, dir)
	if err != nil {
		return report, err
	}
	defer stop()
	session := dockerLabSession{run: r, key: filepath.Join(dir, "id_ed25519"), port: vm.sshPort}
	boot, endBoot := context.WithTimeout(ctx, r.Options.Boot)
	defer endBoot()
	if err := session.waitSSH(boot, r.Options.Boot); err != nil {
		return report, dockerLabBootFailure(err, dir, vm.console)
	}
	for _, identity := range []struct{ name, command string }{
		{"kernel release", "release=$(uname -r) && printf '%s\\n' \"$release\" && test \"$release\" = " + shellQuote(kernel.Release)},
		{"PID 1 executable", "executable=$(sudo -n readlink /proc/1/exe) && printf '%s\\n' \"$executable\" && test \"$executable\" = /gokrazy/init"},
		{"root filesystem", "filesystem=$(stat -f -c %T /) && printf '%s\\n' \"$filesystem\" && test \"$filesystem\" = squashfs"},
		{"runtime modules", "test -d " + shellQuote("/lib/modules/"+kernel.Release)},
		{"Docker socket absent", "test ! -S /var/run/docker.sock"},
		// gokrazy's init mounts cgroup2 at boot and only logs a failure, so a
		// command that attaches a cgroup program, or a Docker lab, would meet its
		// absence as an unexplained red.
		{"cgroup2 mounted", "grep -qs ' - cgroup2 ' /proc/self/mountinfo"},
	} {
		observed, err := session.remote(boot, identity.command, nil)
		if err != nil {
			return report, fmt.Errorf("own-system identity check %s failed before the guest command: %w", identity.name, err)
		}
		if observed.Code != 0 {
			return report, fmt.Errorf("own-system identity refused: %s (exit %d, output %q, error %q). "+
				"Fix: inspect %s and correct the image before rerunning the docker-lab command", identity.name,
				observed.Code, strings.TrimSpace(observed.Stdout), strings.TrimSpace(observed.Stderr), report.Console)
		}
	}
	setup := guestRunParentSetup(guestRunJobParent)
	if r.Options.GuestHarness {
		setup += " && " + dockerLabHarnessSetup(r.Tree)
	}
	command := RunReport{Plan: RunPlan{SSHPort: vm.sshPort, SetupCommand: setup, commandSession: &session, rootCommand: r.Options.guestCommandAsRoot()}}
	ended, err := r.runGuestCommand(ctx, &command)
	report.CommandExit = ended.GuestExitCode
	report.Interrupted = ctx.Err() != nil
	return report, err
}

// guestSessionArgs selects one transport for both the command and its interrupt.
// The legacy Alpine branch remains root without a key; own-system commands use
// only the per-run key and the cidata user, never a privileged SSH connection.
func (r *Run) guestSessionArgs(plan *RunPlan, extra ...string) []string {
	if plan.commandSession != nil {
		return plan.commandSession.args(extra...)
	}
	args := append(r.sshOptions(plan.SSHPort), extra...)
	return append(args, "root@"+forwardHost)
}
