// Design: docs/architecture/testing/qemu-integration.md -- the host QEMU harness
// Overview: actions.go -- the area table that reaches this run
// Detail: run_exec.go -- the bounded VM lifecycle
// Detail: run_iso.go -- Alpine ISO and initramfs cache behavior
// Detail: run_report.go -- the structured plan and run answer
//
// This file owns the native host QEMU harness. It starts QEMU directly and
// never starts Python. The command string remains guest input and crosses the
// SSH boundary unchanged.
package qemu

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
)

const (
	AlpineVersion = "3.21"
	AlpineMinor   = "3"
	GoVersion     = "1.25.9"
)

const (
	DefaultRunMemory      = "16384"
	DefaultRunCPUs        = "8"
	DefaultBootTimeout    = 300 * time.Second
	DefaultCommandTimeout = 1200 * time.Second
	ScratchMountTag       = "zescratch"
	sshPollInterval       = 2 * time.Second
	sshReadyTimeout       = 30 * time.Second
	bootstrapTimeout      = 90 * time.Second
	bootstrapAttempts     = 3
	serialBufferMax       = 20000
	serialBufferKeep      = 10000
	serialReadSize        = 4096
)

// goosDarwin is the one runtime.GOOS this harness branches on. It has no
// /dev/kvm, so it selects the Apple hypervisor, and its QEMU firmware and
// tooling arrive through a Homebrew prefix rather than a system path.
const goosDarwin = "darwin"

// The QEMU accelerators the harness asks for. hvf is the Apple hypervisor, kvm
// is the Linux one, and tcg is the software emulator that both fall back to.
const (
	acceleratorHVF = "hvf"
	acceleratorKVM = "kvm"
	acceleratorTCG = "tcg"
)

// The QEMU system emulator that boots each target architecture.
const (
	qemuSystemARM64 = "qemu-system-aarch64"
	qemuSystemAMD64 = "qemu-system-x86_64"
)

// envTypeString is the env.EnvEntry Type of a setting this package uses as
// written, with no parse (internal/core/env/registry.go, EnvEntry.Type).
const envTypeString = "string"

const (
	runMemoryKey  = "ze.qemu.memory"
	runCPUsKey    = "ze.qemu.cpus"
	runBootKey    = "ze.qemu.boot.timeout"
	runSSHPortKey = "ze.qemu.ssh.port"
	runBinaryKey  = "ze.qemu.bin"
	runTestBinKey = "ze.qemu.test.bin"
)

func runSetting(key, fallback, description string) env.EnvEntry {
	return env.MustRegister(env.EnvEntry{
		Key: key, Type: envTypeString, Default: fallback, Description: description, Private: true,
	})
}

var (
	runMemoryEntry = runSetting(runMemoryKey, DefaultRunMemory,
		"the memory in MiB that the QEMU guest receives")
	runCPUsEntry = runSetting(runCPUsKey, DefaultRunCPUs,
		"the processor count that the QEMU guest receives")
	runBootEntry = runSetting(runBootKey, "300s",
		"the maximum time for the QEMU serial login prompt")
	runSSHPortEntry = runSetting(runSSHPortKey, "",
		"the host port forwarded to the QEMU guest SSH server")
	runBinaryEntry = runSetting(runBinaryKey, "bin/ze-linux-arm64",
		"the guest ze binary used in keep-alive instructions")
	runTestBinEntry = runSetting(runTestBinKey, "bin/ze-test-linux-arm64",
		"the guest ze-test binary used in keep-alive instructions")
)

// RunOptions is the invocation after the closed keyword grammar is parsed.
type RunOptions struct {
	Command   string
	Packages  []string
	Timeout   time.Duration
	Kernel    string
	KeepAlive bool
	Memory    string
	CPUs      string
	Boot      time.Duration
	SSHPort   int
}

// parseRunArguments validates the qemu run keyword values.
func parseRunArguments(args leaction.Arguments) (RunOptions, error) {
	options := RunOptions{
		Command: args["command"], Packages: strings.Fields(args["packages"]),
		Timeout: DefaultCommandTimeout, Kernel: args["kernel"],
		KeepAlive: args.Has("keep-alive"), Memory: settingOr(runMemoryEntry.Key, DefaultRunMemory),
		CPUs: settingOr(runCPUsEntry.Key, DefaultRunCPUs), Boot: DefaultBootTimeout,
	}
	hasRunMode := options.Command != "" || options.KeepAlive
	if !hasRunMode {
		return options, errors.New("qemu run requires command <value> or keep-alive")
	}
	if named := args["timeout"]; named != "" {
		timeout, err := positiveWholeSeconds("timeout", named)
		if err != nil {
			return options, err
		}
		options.Timeout = timeout
	}
	if named := env.Get(runBootEntry.Key); named != "" {
		boot, err := bootTimeout(named)
		if err != nil {
			return options, err
		}
		options.Boot = boot
	}
	if named := env.Get(runSSHPortEntry.Key); named != "" {
		port, err := parsePort(named)
		if err != nil {
			return options, err
		}
		options.SSHPort = port
	}
	return options, nil
}

func bootTimeout(value string) (time.Duration, error) {
	seconds, err := strconv.Atoi(value)
	if err == nil {
		if seconds <= 0 {
			return 0, fmt.Errorf("%s must be greater than zero, got %q", runBootEntry.Key, value)
		}
		return time.Duration(seconds) * time.Second, nil
	}
	return positiveWholeSeconds(runBootEntry.Key, value)
}

func positiveWholeSeconds(name, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not a duration: %w", name, value, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero, got %q", name, value)
	}
	if duration%time.Second != 0 {
		return 0, fmt.Errorf("%s must use whole seconds, got %q", name, value)
	}
	return duration, nil
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not a port: %w", runSSHPortEntry.Key, value, err)
	}
	if port < 1 {
		return 0, fmt.Errorf("%s must be in 1..65535, got %d", runSSHPortEntry.Key, port)
	}
	if port > 65535 {
		return 0, fmt.Errorf("%s must be in 1..65535, got %d", runSSHPortEntry.Key, port)
	}
	return port, nil
}

// Run is one host-side QEMU invocation. It is not safe for concurrent use.
type Run struct {
	Tree    string
	Options RunOptions
	ops     runOps
}

// NewRun answers a run over tree with production host operations.
func NewRun(tree string, options RunOptions) *Run {
	return &Run{Tree: tree, Options: options, ops: productionRunOps()}
}

type commandSpec struct {
	Name   string
	Args   []string
	Dir    string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type commandResult struct {
	Code   int
	Stdout string
	Stderr string
}

type runFileSystem interface {
	Lstat(string) (os.FileInfo, error)
	Stat(string) (os.FileInfo, error)
	MkdirAll(string, os.FileMode) error
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, os.FileMode) error
	Open(string) (*os.File, error)
	CreateTemp(string, string) (*os.File, error)
	MkdirTemp(string, string) (string, error)
	Rename(string, string) error
	Remove(string) error
	RemoveAll(string) error
	Readlink(string) (string, error)
	EvalSymlinks(string) (string, error)
}

type osRunFS struct{}

func (osRunFS) Lstat(name string) (os.FileInfo, error)       { return os.Lstat(name) }
func (osRunFS) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }
func (osRunFS) MkdirAll(name string, mode os.FileMode) error { return os.MkdirAll(name, mode) }
func (osRunFS) ReadFile(name string) ([]byte, error) {
	// #nosec G304 -- callers provide paths assembled by this package under the run tree or its fixture-owned cache.
	return os.ReadFile(name)
}
func (osRunFS) WriteFile(name string, data []byte, mode os.FileMode) error {
	return os.WriteFile(name, data, mode)
}
func (osRunFS) Open(name string) (*os.File, error) {
	// #nosec G304 -- callers provide ISO paths assembled by this package under the fixture-owned cache.
	return os.Open(name)
}
func (osRunFS) CreateTemp(dir, pattern string) (*os.File, error) { return os.CreateTemp(dir, pattern) }
func (osRunFS) MkdirTemp(dir, pattern string) (string, error)    { return os.MkdirTemp(dir, pattern) }
func (osRunFS) Rename(oldPath, newPath string) error             { return os.Rename(oldPath, newPath) }
func (osRunFS) Remove(name string) error                         { return os.Remove(name) }
func (osRunFS) RemoveAll(name string) error                      { return os.RemoveAll(name) }
func (osRunFS) Readlink(name string) (string, error)             { return os.Readlink(name) }
func (osRunFS) EvalSymlinks(name string) (string, error)         { return filepath.EvalSymlinks(name) }

type runOps struct {
	FS      runFileSystem
	Look    func(string) (string, error)
	Run     func(context.Context, commandSpec) (commandResult, error)
	Start   func(*exec.Cmd) error
	Port    func() (int, error)
	Now     func() time.Time
	Sleep   func(context.Context, time.Duration) error
	GOOS    string
	GOARCH  string
	Getenv  func(string) string
	Home    func() (string, error)
	Environ func() []string
}

func productionRunOps() runOps {
	return runOps{
		FS: osRunFS{}, Look: exec.LookPath, Run: runHostCommand,
		Start: func(command *exec.Cmd) error { return command.Start() },
		Port:  freeRunSSHPort, Now: time.Now, Sleep: sleepContext,
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Home: os.UserHomeDir,
		Environ: os.Environ, Getenv: os.Getenv,
	}
}

func runHostCommand(ctx context.Context, spec commandSpec) (commandResult, error) {
	var result commandResult
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	// #nosec G204 -- program choices and argv structure are closed; the operator command is one opaque ssh operand and never enters a host shell.
	command := exec.CommandContext(ctx, spec.Name, spec.Args...)
	command.Dir = spec.Dir
	command.Env = spec.Env
	command.Stdin = spec.Stdin
	command.Stdout = &stdout
	command.Stderr = &stderr
	if spec.Stdout != nil {
		command.Stdout = io.MultiWriter(spec.Stdout, &stdout)
	}
	if spec.Stderr != nil {
		command.Stderr = io.MultiWriter(spec.Stderr, &stderr)
	}
	err := command.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return result, fmt.Errorf("command %s exceeded its deadline: %w", spec.Name, ctx.Err())
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		result.Code = exit.ExitCode()
		return result, nil
	}
	return result, fmt.Errorf("start command %s: %w", spec.Name, err)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func freeRunSSHPort() (int, error) {
	for port := 2222; port <= 2321; port++ {
		listener, err := netListenWildcard(port)
		if err != nil {
			continue
		}
		if err := listener.Close(); err != nil {
			return 0, fmt.Errorf("release SSH port %d: %w", port, err)
		}
		return port, nil
	}
	return 0, errors.New("no free host port in 2222..2321 for the QEMU SSH forward")
}

func netListenWildcard(port int) (io.Closer, error) {
	var config net.ListenConfig
	return config.Listen(context.Background(), "tcp", textbuf.StrIntStr(":", int64(port), ""))
}

// Plan resolves the cache, kernel, port, firmware, and complete QEMU argv.
func (r *Run) Plan(ctx context.Context) (RunPlan, error) {
	var plan RunPlan
	if _, err := r.ops.Look(runQEMUBinary(r.ops.GOARCH)); err != nil {
		return plan, fmt.Errorf("missing required command %s: install QEMU: %w", runQEMUBinary(r.ops.GOARCH), err)
	}
	if _, err := r.ops.Look("ssh"); err != nil {
		return plan, fmt.Errorf("missing required command ssh: install OpenSSH: %w", err)
	}
	if err := r.ensureScratch(); err != nil {
		return plan, err
	}
	iso, err := r.ensureISO(ctx, runAlpineArch(r.ops.GOARCH))
	if err != nil {
		return plan, err
	}
	kernel, err := r.kernelPath()
	if err != nil {
		return plan, err
	}
	port := r.Options.SSHPort
	if port == 0 {
		port, err = r.ops.Port()
		if err != nil {
			return plan, err
		}
	}
	argv, err := r.qemuArgs(ctx, iso, kernel, port)
	if err != nil {
		return plan, err
	}
	setup, err := r.setupCommand()
	if err != nil {
		return plan, err
	}
	plan = RunPlan{
		Tree: r.Tree, AlpineVersion: AlpineVersion, AlpineMinor: AlpineMinor,
		AlpineArch: runAlpineArch(r.ops.GOARCH), GoVersion: GoVersion,
		QEMUBinary: runQEMUBinary(r.ops.GOARCH), ISO: iso, Kernel: kernel,
		Memory: r.Options.Memory, CPUs: r.Options.CPUs, SSHPort: port,
		BootTimeoutSeconds:    int64(r.Options.Boot / time.Second),
		CommandTimeoutSeconds: int64(r.Options.Timeout / time.Second),
		Command:               r.Options.Command, Packages: append([]string(nil), r.Options.Packages...),
		KeepAlive: r.Options.KeepAlive, QEMUArgv: argv,
		BootstrapCommand: runBootstrapCommand, SetupCommand: setup,
	}
	return plan, nil
}

func runAlpineArch(goarch string) string {
	if goarch == ArchARM64 {
		return "aarch64"
	}
	return "x86_64"
}

func runQEMUBinary(goarch string) string {
	if goarch == ArchARM64 {
		return qemuSystemARM64
	}
	return qemuSystemAMD64
}

func (r *Run) ensureScratch() error {
	for _, name := range []string{"", "go-dl", "go-cache", "gomodcache"} {
		dir := filepath.Join(r.Tree, "tmp", "qemu", name)
		if err := r.ops.FS.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create QEMU cache directory %s: %w", dir, err)
		}
	}
	return nil
}

func (r *Run) kernelPath() (string, error) {
	if r.Options.Kernel == "" {
		return "", nil
	}
	kernel := r.Options.Kernel
	if !filepath.IsAbs(kernel) {
		kernel = filepath.Join(r.Tree, kernel)
	}
	info, err := r.ops.FS.Stat(kernel)
	if err != nil {
		return "", fmt.Errorf("kernel not found at %s: %w", kernel, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("kernel path %s is not a regular file", kernel)
	}
	return kernel, nil
}

func (r *Run) qemuArgs(ctx context.Context, iso, kernel string, port int) ([]string, error) {
	argv := []string{runQEMUBinary(r.ops.GOARCH)}
	if r.ops.GOARCH == ArchARM64 {
		bios, err := r.arm64BIOS()
		if err != nil {
			return nil, err
		}
		argv = append(argv, "-machine", "virt,highmem=on,accel=hvf:tcg", "-cpu", "max")
		if kernel == "" {
			argv = append(argv, "-bios", bios)
		}
	} else {
		argv = append(argv, "-machine", "accel=hvf:kvm:tcg")
	}
	var b textbuf.Buffer
	forward := b.Str("user,id=net0,hostfwd=tcp::").Int(int64(port)).Str("-:22").String()
	argv = append(argv,
		"-smp", r.Options.CPUs, "-m", r.Options.Memory,
		"-cdrom", iso, "-boot", "d", "-nographic", "-serial", "mon:stdio",
		"-netdev", forward, "-device", "virtio-net-pci,netdev=net0",
	)
	argv = append(argv, r.virtfsArgs()...)
	if kernel != "" {
		initrd, err := r.extractAlpineInitramfs(ctx, iso)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "-kernel", kernel, "-initrd", initrd, "-append",
			"console=ttyAMA0 alpine_dev=cdrom modules=loop,squashfs quiet")
	}
	return argv, nil
}

func (r *Run) arm64BIOS() (string, error) {
	candidates := append(r.brewFiles("share/qemu/edk2-aarch64-code.fd"),
		"/usr/share/qemu/edk2-aarch64-code.fd")
	for _, candidate := range candidates {
		info, err := r.ops.FS.Stat(candidate)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", errors.New("cannot find aarch64 UEFI firmware edk2-aarch64-code.fd")
}

func (r *Run) scratchShare() (string, string, bool, error) {
	tmp := filepath.Join(r.Tree, "tmp")
	info, err := r.ops.FS.Lstat(tmp)
	if err != nil {
		return "", "", false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", "", false, nil
	}
	link, err := r.ops.FS.Readlink(tmp)
	if err != nil {
		return "", "", false, err
	}
	host, err := r.ops.FS.EvalSymlinks(tmp)
	if err != nil {
		return "", "", false, err
	}
	guest := link
	if !filepath.IsAbs(link) {
		guest = path.Clean(path.Join("/workspace", filepath.ToSlash(link)))
	}
	return host, guest, true, nil
}

func (r *Run) virtfsArgs() []string {
	var b textbuf.Buffer
	workspace := b.Str("local,path=").Str(r.Tree).Str(",mount_tag=workspace,security_model=none,id=ws0,readonly=off").String()
	argv := make([]string, 0, 4)
	argv = append(argv, "-virtfs", workspace)
	host, _, shared, err := r.scratchShare()
	if err != nil {
		return argv
	}
	if !shared {
		return argv
	}
	b.Reset()
	scratch := b.Str("local,path=").Str(host).Str(",mount_tag=").Str(ScratchMountTag).
		Str(",security_model=none,id=ws1,readonly=off").String()
	return append(argv, "-virtfs", scratch)
}

const runBootstrapCommand = "setup-interfaces -a 2>/dev/null; ifup eth0 2>/dev/null; ifup lo 2>/dev/null; echo nameserver 8.8.8.8 > /etc/resolv.conf; apk add --no-cache openssh; echo PermitRootLogin yes >> /etc/ssh/sshd_config; echo PermitEmptyPasswords yes >> /etc/ssh/sshd_config; passwd -d root; ssh-keygen -t ed25519 -f /etc/ssh/ssh_host_ed25519_key -N '' 2>/dev/null; ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key -N '' 2>/dev/null; /usr/sbin/sshd; echo SSHD_READY"

func shellQuote(value string) string {
	var b textbuf.Buffer
	b.Byte('\'')
	for _, character := range value {
		if character == '\'' {
			b.Str("'\\''")
			continue
		}
		b.Str(string(character))
	}
	return b.Byte('\'').String()
}

func (r *Run) setupCommand() (string, error) {
	arch := ArchAMD64
	if runAlpineArch(r.ops.GOARCH) == "aarch64" {
		arch = ArchARM64
	}
	var b textbuf.Buffer
	repositories := b.Str("printf 'https://dl-cdn.alpinelinux.org/alpine/v").Str(AlpineVersion).
		Str("/main\\nhttps://dl-cdn.alpinelinux.org/alpine/v").Str(AlpineVersion).
		Str("/community\\n' > /etc/apk/repositories").String()
	parts := []string{
		"set -e", repositories, "apk update",
		"apk add --no-cache git curl musl-dev",
	}
	if len(r.Options.Packages) != 0 {
		b.Reset()
		parts = append(parts, b.Str("apk add --no-cache ").Join(r.Options.Packages, " ").String())
	}
	parts = append(parts,
		"modprobe ppp_generic 2>/dev/null || true",
		"modprobe l2tp_ppp 2>/dev/null || true",
		"modprobe l2tp_netlink 2>/dev/null || true",
		"modprobe nft_chain_nat 2>/dev/null || true",
		"modprobe nf_conntrack 2>/dev/null || true",
		"modprobe nf_conntrack_netlink 2>/dev/null || true",
		"[ -w /proc/sys/net/netfilter/nf_conntrack_acct ] && echo 1 > /proc/sys/net/netfilter/nf_conntrack_acct || true",
		"nft add table inet ztrack 2>/dev/null || true",
		"nft 'add chain inet ztrack out { type filter hook output priority -150 ; policy accept ; }' 2>/dev/null || true",
		"nft add rule inet ztrack out ct state new,established,related counter 2>/dev/null || true",
		"echo \"CONNTRACK-SETUP: acct=$(cat /proc/sys/net/netfilter/nf_conntrack_acct 2>/dev/null || echo MISSING) rules=$(nft list ruleset 2>/dev/null | grep -c 'ct state' || echo 0)\"",
		"mkdir -p /workspace",
		"mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 workspace /workspace",
	)
	_, guest, shared, err := r.scratchShare()
	if err != nil {
		return "", err
	}
	if shared {
		b.Reset()
		mkdir := b.Str("mkdir -p ").Str(shellQuote(guest)).String()
		b.Reset()
		mount := b.Str("mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 ").
			Str(ScratchMountTag).Byte(' ').Str(shellQuote(guest)).String()
		parts = append(parts, mkdir, mount)
	}
	b.Reset()
	goTar := b.Str("GO_TAR=\"/workspace/tmp/qemu/go-dl/go").Str(GoVersion).
		Str(".linux-").Str(arch).Str(".tar.gz\"").String()
	b.Reset()
	goURL := b.Str("https://go.dev/dl/go").Str(GoVersion).Str(".linux-").Str(arch).Str(".tar.gz").String()
	parts = append(parts,
		"cd /workspace", "mkdir -p /workspace/tmp/qemu/go-dl", goTar,
		b.Reset().Str("[ -f \"$GO_TAR\" ] || curl -fsSL -o \"$GO_TAR\" \"").Str(goURL).Str("\"").String(),
		"tar -C /usr/local -xzf \"$GO_TAR\"",
		"export PATH=\"/usr/local/go/bin:$PATH\"",
		"export GOROOT=\"/usr/local/go\"",
		"export GOCACHE=\"/workspace/tmp/qemu/go-cache\"",
		"export GOMODCACHE=\"/workspace/tmp/qemu/gomodcache\"",
		"export GOFLAGS=\"-buildvcs=false\"", "export CGO_ENABLED=\"0\"",
		"export HOME=\"/root\"", "export TMPDIR=\"/tmp\"",
		"git config --global --add safe.directory '*' 2>/dev/null || true",
		"mkdir -p /workspace/tmp/evidence", "mount -t tmpfs tmpfs /workspace/tmp/evidence",
	)
	return strings.Join(parts, " && "), nil
}
