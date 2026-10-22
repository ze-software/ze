// Design: docs/architecture/testing/qemu-integration.md -- the host QEMU harness
// Overview: actions.go -- the area table that reaches this run
// Detail: run_exec.go -- the bounded VM lifecycle
// Detail: run_iso.go -- Alpine ISO and initramfs cache behavior
// Detail: run_report.go -- the structured plan and run answer
//
// This file owns the native host QEMU harness. It starts QEMU directly and
// never starts Python. The command string remains guest input and crosses the
// SSH boundary unchanged.
package testqemu

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/appliance/dockerhost"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/tmplink"
	goversionpin "github.com/ze-software/ze/internal/le/go/versionpin"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

const (
	AlpineVersion = "3.24"
	AlpineMinor   = "3"
)

const (
	DefaultRunMemory      = "16384"
	DefaultRunCPUs        = "8"
	DefaultBootTimeout    = 300 * time.Second
	DefaultCommandTimeout = 1200 * time.Second
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
)

// GuestArch answers the architecture of the QEMU guest: QEMU_GOARCH when a
// caller names one, else the host's, because the guest runs without emulation.
func GuestArch() string {
	if named := os.Getenv("QEMU_GOARCH"); named != "" {
		return named
	}
	if runtime.GOARCH == ArchARM64 {
		return ArchARM64
	}
	return ArchAMD64
}

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
	// HardwareOnly demands the host's hypervisor, HVF on macOS or KVM on
	// Linux, with no software fallback. A run that measures sets it: QEMU
	// otherwise drops to TCG in silence, and a stress run under TCG measures
	// the emulator rather than Ze.
	HardwareOnly bool
}

// parseRunArguments validates the qemu run keyword values.
func parseRunArguments(args leaction.Arguments) (RunOptions, error) {
	options := RunOptions{
		Command: args.One(keywordCommand), Packages: strings.Fields(args.One(keywordPackages)),
		Timeout: DefaultCommandTimeout, Kernel: args.One(keywordKernel),
		KeepAlive: args.Has("keep-alive"), Memory: settingOr(runMemoryEntry.Key, DefaultRunMemory),
		CPUs: settingOr(runCPUsEntry.Key, DefaultRunCPUs), Boot: DefaultBootTimeout,
	}
	hasRunMode := options.Command != "" || options.KeepAlive
	if !hasRunMode {
		return options, errors.New("qemu run requires command <value> or keep-alive")
	}
	if named := args.One(keywordTimeout); named != "" {
		timeout, err := positiveWholeSeconds(keywordTimeout, named)
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
	// teardownGrace bounds the wait for the guest command's own teardown
	// after the run forwards it SIGINT (stopGuestCommand).
	teardownGrace time.Duration
	// shareMountRoots are the guest directories a docker-lab share may sit
	// under (dockerhost.ShareMountRoots); nil refuses every share.
	shareMountRoots []string
}

// NewRun answers a run over tree with production host operations.
func NewRun(tree string, options RunOptions) *Run {
	return &Run{Tree: tree, Options: options, ops: productionRunOps(), teardownGrace: guestTeardownGrace, shareMountRoots: dockerhost.ShareMountRoots()}
}

type commandSpec struct {
	Name   string
	Args   []string
	Dir    string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// OwnGroup starts the child as the leader of its own process group. A
	// Ctrl-C at the terminal signals the whole foreground group, and a guest
	// ssh session or QEMU that took that signal would end under the guest
	// command before le could forward the interrupt to it, so the run's signal
	// root alone decides how such a child stops. On Linux the child also
	// dies with le (childProcAttr).
	OwnGroup bool
}

type commandResult struct {
	Code   int
	Stdout string
	Stderr string
	// StreamNote says when the operator's output stream stopped accepting
	// writes during the command (terminalWriter), and is empty otherwise.
	StreamNote string
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
	// Accelerator probes the hypervisor this host can open (accelerator).
	Accelerator func() string
	// Interrupts subscribes to SIGINT, SIGTERM and SIGHUP once the run's signal root
	// has already taken the first one, so a second one can end the wait for
	// the guest command's teardown. The caller MUST call the release it
	// answers.
	Interrupts func() (<-chan os.Signal, func())
	// StartTime answers when pid started, or errProcessGone when no process
	// holds it, so a VM record is never matched to a reused pid (vmrecord.go).
	StartTime func(pid int) (int64, error)
	// Kill stops the QEMU of a record whose le is gone (reapOrphanVMs).
	Kill func(pid int) error
}

func productionRunOps() runOps {
	return runOps{
		FS: osRunFS{}, Look: exec.LookPath, Run: runHostCommand,
		Start: func(command *exec.Cmd) error { return command.Start() },
		Port:  freeRunSSHPort, Now: time.Now, Sleep: sleepContext,
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Home: os.UserHomeDir,
		Environ: os.Environ, Getenv: os.Getenv, Accelerator: accelerator,
		Interrupts: notifyInterrupts, StartTime: processStartTime, Kill: killProcess,
	}
}

// notifyInterrupts is the production Interrupts: a channel SIGINT, SIGTERM and
// SIGHUP reach from now on, and its release. The root's signal.NotifyContext
// still holds all three, so none ends le by its default action while this
// subscription waits.
func notifyInterrupts() (<-chan os.Signal, func()) {
	again := make(chan os.Signal, 1)
	signal.Notify(again, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return again, func() { signal.Stop(again) }
}

// terminalWriter streams a child's output to the operator's terminal and
// never fails the copy: once a write to the terminal fails (EIO after a
// hangup, ENOSPC on a job log), it stops writing there, keeps that first
// error and its time for note, and reports every write as done. The copy
// then keeps draining the child, so a guest ssh session survives a closed
// terminal and its guest command's teardown runs (stopGuestCommand), rather
// than ssh dying of EPIPE and the guest le of SIGPIPE. The captured copy in
// runHostCommand still holds every byte. Not safe for concurrent use; each
// stream has its own.
type terminalWriter struct {
	target io.Writer
	err    error
	at     time.Time
}

func (w *terminalWriter) Write(data []byte) (int, error) {
	if w.err == nil {
		if _, err := w.target.Write(data); err != nil {
			w.err = err
			w.at = time.Now()
		}
	}
	return len(data), nil
}

// note answers the line that says the stream was lost, or "" when it never
// was. The caller MUST read it only after the command has ended.
func (w *terminalWriter) note(stream string) string {
	if w == nil || w.err == nil {
		return ""
	}
	var b textbuf.Buffer
	return b.Str("le's ").Str(stream).Str(" stopped accepting writes at ").Str(w.at.Format(time.TimeOnly)).
		Str(" (").Str(w.err.Error()).Str("), so the command's later output is missing there; ").
		Str("the run's captured output is intact").String()
}

// joinStreamNotes joins the notes of the two streams that are not empty.
func joinStreamNotes(output, errorOutput string) string {
	if output == "" {
		return errorOutput
	}
	if errorOutput == "" {
		return output
	}
	return output + "; " + errorOutput
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
	if spec.OwnGroup {
		command.SysProcAttr = childProcAttr()
	}
	command.Stdout = &stdout
	command.Stderr = &stderr
	var streamOut, streamErr *terminalWriter
	if spec.Stdout != nil {
		streamOut = &terminalWriter{target: spec.Stdout}
		command.Stdout = io.MultiWriter(streamOut, &stdout)
	}
	if spec.Stderr != nil {
		streamErr = &terminalWriter{target: spec.Stderr}
		command.Stderr = io.MultiWriter(streamErr, &stderr)
	}
	err := command.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.StreamNote = joinStreamNotes(streamOut.note("output"), streamErr.note("error output"))
	if result.StreamNote != "" {
		var b textbuf.Buffer
		b.Str("  ").Str(result.StreamNote).Byte('\n').StdErr() //nolint:errcheck // said once, where it may still reach somebody
	}
	if err == nil {
		return result, nil
	}
	// A deadline and a cancel are told apart: a Ctrl-C cancels the run's signal
	// root, and naming that a timeout sends the operator after a bound they
	// never hit.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, fmt.Errorf("command %s exceeded its deadline: %w", spec.Name, ctx.Err())
	}
	if ctx.Err() != nil {
		return result, fmt.Errorf("command %s was canceled before it finished, by an interrupt or by the run stopping: %w", spec.Name, ctx.Err())
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

// freeRunSSHPort answers a host port for the QEMU SSH forward. The port is
// released before QEMU binds it, so two runs probing in the same instant can
// still pick the same one. When both forwards bind the same address, the
// second QEMU refuses the forward at start. When they differ, a run or
// install-test wildcard forward beside a docker-lab 127.0.0.1 forward, macOS
// lets both bind (slirp sets SO_REUSEADDR). Every ssh client reaches the
// forward over IPv4 loopback: docker-lab, hugepages and install-test dial
// 127.0.0.1, and run dials localhost, which falls back to 127.0.0.1 because
// slirp binds IPv4 only. By BSD's most-specific-bind lookup the docker-lab
// 127.0.0.1 forward then takes every loopback connection: the wildcard run's
// ssh reaches the docker-lab VM, whichever QEMU started first. This is
// reasoned from the lookup, not measured.
func freeRunSSHPort() (int, error) {
	return freeSSHPortIn(2222, 2321)
}

// freeSSHPortIn answers the first port in first..last that sshPortFree finds
// free on every address a forward binds.
func freeSSHPortIn(first, last int) (int, error) {
	for port := first; port <= last; port++ {
		free, err := sshPortFree(port)
		if err != nil {
			return 0, err
		}
		if free {
			return port, nil
		}
	}
	var b textbuf.Buffer
	return 0, errors.New(b.Str("no free host port in ").Int(int64(first)).Str("..").Int(int64(last)).
		Str(" for the QEMU SSH forward: every one is held, most likely by other QEMU runs. ").
		Str("Fix: stop a VM you no longer need, or set ze.qemu.ssh.port (ZE_QEMU_SSH_PORT) to a free port.").String())
}

// sshForwardHosts are the addresses this package's hostfwd strings bind:
// docker-lab forwards on 127.0.0.1, run and install-test on the wildcard.
var sshForwardHosts = [...]string{"127.0.0.1", "0.0.0.0"}

// sshPortFree binds port on every forward address, IPv4 as slirp binds, with
// SO_REUSEADDR cleared. With it set, as Go sets it on every listener, macOS
// lets the wildcard bind sit beside another VM's 127.0.0.1 forward, and Linux
// lets it sit beside a forward bound and not yet listening; without it, any
// overlapping holder refuses the bind on both. An error answers a probe that
// could not be released, never a held port.
func sshPortFree(port int) (bool, error) {
	config := net.ListenConfig{Control: clearReuseAddress}
	for _, host := range sshForwardHosts {
		listener, err := config.Listen(context.Background(), "tcp4", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			return false, nil //nolint:nilerr // a refused bind is the answer "held", not a failure
		}
		if err := listener.Close(); err != nil {
			return false, fmt.Errorf("release SSH port probe %s:%d: %w. A fixed SSH port skips this probe. Fix: set ze.qemu.ssh.port (ZE_QEMU_SSH_PORT) for run and docker-lab, or ze.install.ssh.port (ZE_INSTALL_SSH_PORT) for install-test, to a free port", host, port, err)
		}
	}
	return true, nil
}

// clearReuseAddress runs after Go has set SO_REUSEADDR and before the bind.
func clearReuseAddress(_, _ string, raw syscall.RawConn) error {
	var optionErr error
	err := raw.Control(func(fd uintptr) {
		optionErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
	})
	if err != nil {
		return err
	}
	return optionErr
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
	goRelease, err := goversionpin.DeclaredRelease(r.Tree)
	if err != nil {
		return plan, fmt.Errorf("read the Go release the guest must install: %w", err)
	}
	setup, err := r.setupCommand(goRelease)
	if err != nil {
		return plan, err
	}
	plan = RunPlan{
		Tree: r.Tree, AlpineVersion: AlpineVersion, AlpineMinor: AlpineMinor,
		AlpineArch: runAlpineArch(r.ops.GOARCH), GoVersion: goRelease,
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
	acceleration, err := r.acceleration()
	if err != nil {
		return nil, err
	}
	var b textbuf.Buffer
	if r.ops.GOARCH == ArchARM64 {
		bios, err := r.arm64BIOS()
		if err != nil {
			return nil, err
		}
		argv = append(argv, "-machine", b.Str("virt,highmem=on,accel=").Str(acceleration).String(), "-cpu", "max")
		if kernel == "" {
			argv = append(argv, "-bios", bios)
		}
	} else {
		argv = append(argv, "-machine", b.Str("accel=").Str(acceleration).String())
	}
	b.Reset()
	forward := b.Str("user,id=net0,hostfwd=tcp::").Int(int64(port)).Str("-:22").String()
	argv = append(argv,
		"-smp", r.Options.CPUs, "-m", r.Options.Memory,
		"-cdrom", iso, "-boot", "d", "-nographic", "-serial", "mon:stdio",
		"-netdev", forward, "-device", "virtio-net-pci,netdev=net0",
	)
	virtfs, err := r.virtfsArgs()
	if err != nil {
		return nil, err
	}
	argv = append(argv, virtfs...)
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

// acceleration answers the QEMU accel list. By default it is every hypervisor
// the architecture's machine accepts, then TCG, and QEMU takes the first that
// opens. HardwareOnly names the one hypervisor this host opens and refuses a
// host that has none, so the run never falls back to emulation unseen.
func (r *Run) acceleration() (string, error) {
	if !r.Options.HardwareOnly {
		if r.ops.GOARCH == ArchARM64 {
			return "hvf:tcg", nil
		}
		return "hvf:kvm:tcg", nil
	}
	named := r.ops.Accelerator()
	if !Hardware(named) {
		return "", errors.New("this run needs a hypervisor and the host offers none (HVF on macOS," +
			" read-write /dev/kvm on Linux: `./le setup check` reports kvm-access); under " + named +
			" software emulation it would measure the emulator")
	}
	return named, nil
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

// scratchShares answers the 9p exports the guest needs to follow the checkout
// symlinks it meets under /workspace: tmp itself, or the relocated children
// of a real tmp (internal/core/tmplink, Shares), and the cache link the Go
// build cache lives behind (cacheShare).
func (r *Run) scratchShares() ([]tmplink.Share, error) {
	shares, err := tmplink.Shares(r.Tree, "/workspace")
	if err != nil {
		return nil, fmt.Errorf("find the tmp directories relocated out of the checkout: %w", err)
	}
	cache, linked, err := cacheShare(r.Tree)
	if err != nil {
		return nil, fmt.Errorf("find the build cache relocated out of the checkout: %w", err)
	}
	if linked {
		shares = append(shares, cache)
	}
	return shares, nil
}

// cacheShareTag is the virtfs mount_tag of the cache export. tmplink's tags
// are "zescratch" and "zescratch-<Migratable child>", and "cache" is not one.
const cacheShareTag = "zescratch-cache"

// cacheShare answers the export a guest needs to follow the checkout's `cache`
// symlink (`./le scratch links-ensure` points it out of tree), and false when
// cache is absent or a real directory, which the workspace share already
// carries. Every le-driven Go build sets GOCACHE to cache/go-cache
// (internal/le/go/toolchain, GoCache) whatever the guest exported, so a lab
// that cross-compiles inside the guest (a `./le test qemu run` command) meets the
// link dangling and fails with "mkdir /workspace/cache: file exists".
//
// It lives here rather than in tmplink because the kernel builder imports
// tmplink and runs no le-driven build in its guest: an edit there changes the
// runtime-kernel cache identity for no behavior of the kernel's.
func cacheShare(tree string) (tmplink.Share, bool, error) {
	link := filepath.Join(tree, "cache")
	info, err := os.Lstat(link)
	if errors.Is(err, os.ErrNotExist) {
		return tmplink.Share{}, false, nil
	}
	if err != nil {
		return tmplink.Share{}, false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return tmplink.Share{}, false, nil
	}
	target, err := os.Readlink(link)
	if err != nil {
		return tmplink.Share{}, false, err
	}
	host, err := filepath.EvalSymlinks(link)
	if err != nil {
		return tmplink.Share{}, false, fmt.Errorf("resolve %s: %w", link, err)
	}
	guest := target
	if !filepath.IsAbs(target) {
		guest = path.Clean(path.Join(GuestWorkspace, filepath.ToSlash(target)))
	}
	return tmplink.Share{Tag: cacheShareTag, Host: host, Guest: guest}, true, nil
}

func (r *Run) virtfsArgs() ([]string, error) {
	var b textbuf.Buffer
	workspace := b.Str("local,path=").Str(r.Tree).Str(",mount_tag=workspace,security_model=none,id=ws0,readonly=off").String()
	shares, err := r.scratchShares()
	if err != nil {
		return nil, err
	}
	argv := make([]string, 0, 2+2*len(shares))
	argv = append(argv, "-virtfs", workspace)
	for _, share := range shares {
		b.Reset()
		export := b.Str("local,path=").Str(share.Host).Str(",mount_tag=").Str(share.Tag).
			Str(",security_model=none,id=").Str(share.Tag).Str(",readonly=off").String()
		argv = append(argv, "-virtfs", export)
	}
	return argv, nil
}

// guestPatchedTree selects Ze's patched standard library in a guest shell
// whose cwd is /workspace. The guest's own go (GOROOT=/usr/local/go) runs the
// generator, which needs git, and its answer becomes GOROOT, with
// GOTOOLCHAIN=local and the tree's bin first on PATH, so every later go command
// compiles against the patched crypto/tls
// (docs/contributing/patching-the-standard-library.md). It is one line so a
// refusal stops the setup chain, and in the keep-alive login profile leaves
// the real GOROOT in place, where a build of internal/core/eap then fails on
// the compile-time marker instead of building unpatched.
//
// The line first mounts a guest-only tmpfs over guestStdpatchCache, so the
// tree the generator writes, and the one the guest le's own activation
// writes, never reach the host through the 9p share. Every link in that tree
// names the guest's /usr/local/go, which dangles on the host: a host walk of
// the checkout then failed, and the pretool hook refused every Bash read under
// ai/ (plan/journal/gate-verdict-depends-on-the-machine.md, 2026-10-15). The
// marker file lives inside the tmpfs, so the keep-alive profile, which runs at
// every login, mounts it once and later logins answer the same tree.
const guestPatchedTree = `mkdir -p ` + guestStdpatchCache + ` && { [ -e ` + guestStdpatchMarker + ` ] || { mount -t tmpfs tmpfs ` +
	guestStdpatchCache + ` && touch ` + guestStdpatchMarker + `; }; } && ` +
	`tree="$(go run ./internal/le/go/stdpatch/goroot)" && export GOROOT="$tree" GOTOOLCHAIN=local PATH="$tree/bin:$PATH"`

// guestStdpatchCache is the directory stdpatch's CacheDir answers for the
// checkout the guest mounts at /workspace: where both the generator and the
// guest le's activation write their tree.
const guestStdpatchCache = "/workspace/cache/stdpatch"

// guestStdpatchMarker exists only inside the guest's tmpfs over
// guestStdpatchCache, so its presence says the mount is in place.
const guestStdpatchMarker = guestStdpatchCache + "/.guest-only"

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

// setupCommand answers the one shell line the guest runs before the caller's
// command. goRelease is the Go release the guest unpacks, read from the go
// directive of go.mod by the caller. It is a parameter rather than a constant
// here because a copy of that declaration drifted once already, and a guest a
// minor behind downloads a second toolchain on its first `go` command
// (internal/le/go/versionpin, DeclaredRelease).
func (r *Run) setupCommand(goRelease string) (string, error) {
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
	shares, err := r.scratchShares()
	if err != nil {
		return "", err
	}
	for _, share := range shares {
		b.Reset()
		mkdir := b.Str("mkdir -p ").Str(shellQuote(share.Guest)).String()
		b.Reset()
		mount := b.Str("mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 ").
			Str(share.Tag).Byte(' ').Str(shellQuote(share.Guest)).String()
		parts = append(parts, mkdir, mount)
	}
	b.Reset()
	goTar := b.Str("GO_TAR=\"/workspace/tmp/qemu/go-dl/go").Str(goRelease).
		Str(".linux-").Str(arch).Str(".tar.gz\"").String()
	b.Reset()
	goURL := b.Str("https://go.dev/dl/go").Str(goRelease).Str(".linux-").Str(arch).Str(".tar.gz").String()
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
		guestPatchedTree,
		"mkdir -p /workspace/tmp/evidence", "mount -t tmpfs tmpfs /workspace/tmp/evidence",
	)
	return strings.Join(parts, " && "), nil
}
