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
	return nil
}

// runDockerLabCommand resolves the same immutable runtime tree EnsureImage uses,
// refusing a mismatched request before an image build can start.
func runDockerLabCommand(ctx context.Context, root, arch string, options RunOptions) (any, int) {
	tree, err := appliance.ImageRuntimeKernelTree(root, arch, func(arch string) string {
		return "./le setup docker host build arch " + arch
	})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	kernel, err := dockerLabCommandKernel(options.Kernel, tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	image, err := setupdockerhost.EnsureImage(ctx, root, arch)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	options.Kernel = kernel.Selected
	report, err := NewRun(root, options).dockerLabCommand(ctx, image, kernel)
	if err != nil {
		leaction.ReportError(err)
		return &report, 1
	}
	return &report, report.CommandExit
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
	for _, word := range strings.Fields(string(line)) {
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
		defer os.RemoveAll(dir) //nolint:errcheck // a disposable run-owned directory
	}
	if err != nil {
		return report, err
	}
	vm.kernel, vm.cmdline = kernel.Selected, cmdline
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
	command := RunReport{Plan: RunPlan{SSHPort: vm.sshPort, SetupCommand: guestRunParentSetup(guestRunJobParent), commandSession: &session}}
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
