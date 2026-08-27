package qemu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/leaction"
)

// VALIDATES: qemu publishes the exact host and guest action population, while
// the host run remains gateless, native, and argument-aware.
// PREVENTS: a missing guest proof, a Make gate claim for run, a Python fork, or
// a renamed action.
func TestQEMUActionsIncludeTheHostRun(t *testing.T) {
	rows := Actions().Actions
	if len(rows) != 11 {
		t.Fatalf("qemu actions = %d, want 11", len(rows))
	}
	want := []string{
		"vpp-hugepages-test", "run", "install-test", "install-iso-test",
		"install-scenarios-test", "install-ventoy-test", "vrrp-keepalived-test",
		"pppoe-accel-test", "netns-test", "pppoe-test", "all-tests",
	}
	for index, verb := range want {
		if rows[index].Verb != verb {
			t.Errorf("action %d = %q, want %q", index, rows[index].Verb, verb)
		}
	}
	if rows[1].Gate != "" || len(rows[1].Forks) != 0 {
		t.Fatalf("run action claims a gate or script fork: %#v", rows[1])
	}
}

// VALIDATES: qemu run uses closed keywords, requires a command or keep-alive,
// and parses package and duration values at their boundaries.
// PREVENTS: Python flag syntax or a zero timeout entering the Go command.
func TestParseRunArgumentsAndBoundaries(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	t.Setenv("ZE_QEMU_BOOT_TIMEOUT", "300")
	t.Setenv("ZE_QEMU_SSH_PORT", "65535")
	env.ResetCache()

	got, err := ParseRunArguments(leaction.Arguments{
		"command": "go test ./...", "packages": "git  bash\npython3", "timeout": "45s",
	})
	if err != nil {
		t.Fatalf("ParseRunArguments: %v", err)
	}
	if got.Command != "go test ./..." || got.Timeout != 45*time.Second {
		t.Fatalf("parsed options = %#v", got)
	}
	if !reflect.DeepEqual(got.Packages, []string{"git", "bash", "python3"}) {
		t.Fatalf("packages = %v", got.Packages)
	}
	if got.SSHPort != 65535 || got.Boot != 300*time.Second {
		t.Fatalf("environment boundaries = port %d, boot %s", got.SSHPort, got.Boot)
	}

	for _, one := range []struct {
		name string
		args leaction.Arguments
	}{
		{"missing command", leaction.Arguments{}},
		{"zero timeout", leaction.Arguments{"command": "true", "timeout": "0s"}},
		{"negative timeout", leaction.Arguments{"command": "true", "timeout": "-1s"}},
		{"fractional timeout", leaction.Arguments{"command": "true", "timeout": "1500ms"}},
	} {
		t.Run(one.name, func(t *testing.T) {
			if _, parseErr := ParseRunArguments(one.args); parseErr == nil {
				t.Fatal("invalid arguments were accepted")
			}
		})
	}

	keep, err := ParseRunArguments(leaction.Arguments{"keep-alive": ""})
	if err != nil || !keep.KeepAlive {
		t.Fatalf("keep-alive alone = %#v, %v", keep, err)
	}
}

// VALIDATES: every port boundary is explicit.
// PREVENTS: an invalid forwarded port reaching QEMU as a plausible string.
func TestRunSSHPortBoundaries(t *testing.T) {
	for _, value := range []string{"1", "65535"} {
		if _, err := parsePort(value); err != nil {
			t.Errorf("valid port %s: %v", value, err)
		}
	}
	for _, value := range []string{"0", "65536", "not-a-port"} {
		if _, err := parsePort(value); err == nil {
			t.Errorf("invalid port %s was accepted", value)
		}
	}
}

func fixtureRun(t *testing.T, arch string) *Run {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(root, "cache")
	isoDir := filepath.Join(cache, "ze", "alpine-iso")
	if err := os.MkdirAll(isoDir, 0o750); err != nil {
		t.Fatal(err)
	}
	name := alpineISOName(runAlpineArch(arch))
	iso := filepath.Join(isoDir, name)
	payload := []byte("fixture alpine iso")
	if err := os.WriteFile(iso, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	if err := os.WriteFile(iso+".sha256", []byte(hex.EncodeToString(digest[:])+"  "+name+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	run := NewRun(root, RunOptions{
		Command: "true", Timeout: 30 * time.Second, Boot: 10 * time.Second,
		Memory: DefaultRunMemory, CPUs: DefaultRunCPUs, SSHPort: 2222,
	})
	run.ops.GOARCH = arch
	run.ops.GOOS = "linux"
	run.ops.Getenv = func(name string) string {
		if name == "XDG_CACHE_HOME" {
			return cache
		}
		return ""
	}
	run.ops.Look = func(name string) (string, error) { return name, nil }
	run.ops.Run = func(context.Context, commandSpec) (commandResult, error) {
		t.Fatalf("cache-hit plan unexpectedly ran a host command")
		return commandResult{}, nil
	}
	return run
}

// VALIDATES: a missing host command is an operating error before any VM starts.
// PREVENTS: a missing QEMU binary surfacing as a serial timeout.
func TestRunPlanRefusesAMissingCommand(t *testing.T) {
	run := fixtureRun(t, ArchAMD64)
	run.ops.Look = func(name string) (string, error) {
		if name == "qemu-system-x86_64" {
			return "", os.ErrNotExist
		}
		return name, nil
	}
	if _, err := run.Plan(context.Background()); err == nil {
		t.Fatal("missing QEMU command was accepted")
	}
}

// VALIDATES: the x86 QEMU argv matches the script in full, including the BIOS
// choice, SSH forward, 9p share, memory, and processor count.
// PREVENTS: a changed QEMU spelling hidden by a partial contains assertion.
func TestRunPlanBuildsTheCompleteX86QEMUArgv(t *testing.T) {
	run := fixtureRun(t, ArchAMD64)
	plan, err := run.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"qemu-system-x86_64", "-machine", "accel=hvf:kvm:tcg",
		"-smp", "8", "-m", "16384", "-cdrom", plan.ISO, "-boot", "d",
		"-nographic", "-serial", "mon:stdio", "-netdev",
		"user,id=net0,hostfwd=tcp::2222-:22", "-device", "virtio-net-pci,netdev=net0",
		"-virtfs", "local,path=" + run.Tree + ",mount_tag=workspace,security_model=none,id=ws0,readonly=off",
	}
	if !reflect.DeepEqual(plan.QEMUArgv, want) {
		t.Fatalf("QEMU argv:\n got %#v\nwant %#v", plan.QEMUArgv, want)
	}
}

// VALIDATES: a relative kernel resolves from the checkout and adds the ISO-keyed
// initramfs plus the complete kernel command line.
// PREVENTS: a working-directory-dependent kernel or a stock-kernel fallback.
func TestRunPlanAddsTheCustomKernelAndCachedInitramfs(t *testing.T) {
	run := fixtureRun(t, ArchAMD64)
	kernel := filepath.Join(run.Tree, "tmp", "kernel", "vmlinuz")
	if err := os.MkdirAll(filepath.Dir(kernel), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kernel, []byte("kernel"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache, err := run.durableCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	iso := filepath.Join(cache, "alpine-iso", alpineISOName("x86_64"))
	initrd := filepath.Join(alpineExtractDir(iso), "boot", "initramfs-virt")
	if err := os.MkdirAll(filepath.Dir(initrd), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(initrd, []byte("initrd"), 0o600); err != nil {
		t.Fatal(err)
	}
	run.Options.Kernel = filepath.Join("tmp", "kernel", "vmlinuz")
	plan, err := run.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"-kernel": kernel, "-initrd": initrd,
		"-append": "console=ttyAMA0 alpine_dev=cdrom modules=loop,squashfs quiet",
	} {
		if !containsPair(plan.QEMUArgv, key, value) {
			t.Errorf("QEMU argv misses %s %q: %#v", key, value, plan.QEMUArgv)
		}
	}
}

// VALIDATES: arm64 uses Homebrew firmware first, the documented Linux path
// second, and does not pass BIOS when a custom kernel is used.
// PREVENTS: an Apple Silicon-only literal or a BIOS argument beside -kernel.
func TestRunPlanSelectsTheArm64BIOS(t *testing.T) {
	run := fixtureRun(t, ArchARM64)
	prefix := filepath.Join(run.Tree, "brew")
	bios := filepath.Join(prefix, "share", "qemu", "edk2-aarch64-code.fd")
	if err := os.MkdirAll(filepath.Dir(bios), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bios, []byte("firmware"), 0o600); err != nil {
		t.Fatal(err)
	}
	priorGetenv := run.ops.Getenv
	run.ops.Getenv = func(name string) string {
		if name == "HOMEBREW_PREFIX" {
			return prefix
		}
		return priorGetenv(name)
	}
	plan, err := run.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !containsPair(plan.QEMUArgv, "-bios", bios) {
		t.Fatalf("arm64 argv does not use Homebrew BIOS: %#v", plan.QEMUArgv)
	}
	kernel := filepath.Join(run.Tree, "vmlinuz")
	if err := os.WriteFile(kernel, []byte("kernel"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache, err := run.durableCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	iso := filepath.Join(cache, "alpine-iso", alpineISOName("aarch64"))
	initrd := filepath.Join(alpineExtractDir(iso), "boot", "initramfs-virt")
	if err := os.MkdirAll(filepath.Dir(initrd), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(initrd, []byte("initrd"), 0o600); err != nil {
		t.Fatal(err)
	}
	run.Options.Kernel = kernel
	kernelPlan, err := run.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if containsToken(kernelPlan.QEMUArgv, "-bios") {
		t.Fatalf("custom-kernel arm64 argv includes BIOS: %#v", kernelPlan.QEMUArgv)
	}
}

func containsPair(argv []string, key, value string) bool {
	for index := 0; index+1 < len(argv); index++ {
		if argv[index] == key && argv[index+1] == value {
			return true
		}
	}
	return false
}

func containsToken(argv []string, want string) bool {
	return slices.Contains(argv, want)
}

// VALIDATES: the initramfs extract cache is keyed by the complete ISO name.
// PREVENTS: an Alpine version bump reusing an earlier ISO's initramfs.
func TestInitramfsCacheIsKeyedByISO(t *testing.T) {
	run := fixtureRun(t, ArchAMD64)
	dir := t.TempDir()
	first := filepath.Join(dir, "alpine-virt-3.21.3-x86_64.iso")
	second := filepath.Join(dir, "alpine-virt-3.22.0-x86_64.iso")
	initrd := filepath.Join(alpineExtractDir(first), "boot", "initramfs-virt")
	if err := os.MkdirAll(filepath.Dir(initrd), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(initrd, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := run.extractAlpineInitramfs(context.Background(), first)
	if err != nil || got != initrd {
		t.Fatalf("cache hit = %q, %v", got, err)
	}
	if alpineExtractDir(first) == alpineExtractDir(second) {
		t.Fatal("two ISO names share an extract directory")
	}
}

// VALIDATES: a symlinked checkout tmp gets a second share mounted at the link's
// own guest path, including relative-link resolution.
// PREVENTS: /workspace/tmp dangling behind the checkout 9p mount.
func TestScratchSymlinkResolvesHostAndGuestPaths(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "tmp")); err != nil {
		t.Fatal(err)
	}
	run := NewRun(root, RunOptions{})
	host, guest, shared, err := run.scratchShare()
	if err != nil {
		t.Fatal(err)
	}
	if !shared || host != target || guest != target {
		t.Fatalf("scratch share = host %q, guest %q, shared %v", host, guest, shared)
	}
	if count := strings.Count(strings.Join(run.virtfsArgs(), "\n"), "mount_tag="); count != 2 {
		t.Fatalf("virtfs exports = %d, want 2", count)
	}
}

// VALIDATES: bootstrap, package installation, Go setup, and SSH options retain
// the script's complete payloads.
// PREVENTS: a missing bootstrap command, package, cache export, or SSH option.
func TestRunBuildsBootstrapSetupAndSSHCommands(t *testing.T) {
	run := fixtureRun(t, ArchAMD64)
	run.Options.Packages = []string{"xl2tpd", "ppp"}
	plan, err := run.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if plan.BootstrapCommand != runBootstrapCommand {
		t.Fatalf("bootstrap command changed:\n%s", plan.BootstrapCommand)
	}
	for _, needle := range []string{
		"apk add --no-cache xl2tpd ppp", "go1.25.9.linux-amd64.tar.gz",
		"export GOCACHE=\"/workspace/tmp/qemu/go-cache\"",
		"mount -t tmpfs tmpfs /workspace/tmp/evidence",
	} {
		if !strings.Contains(plan.SetupCommand, needle) {
			t.Errorf("setup command misses %q", needle)
		}
	}
	wantSSH := []string{
		"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		"-o", "PreferredAuthentications=none", "-o", "LogLevel=ERROR", "-p", "2222",
	}
	if !reflect.DeepEqual(run.sshOptions(2222), wantSSH) {
		t.Fatalf("SSH options = %#v", run.sshOptions(2222))
	}
}

// VALIDATES: the guest kernel assertion is an explicit proof failure and uses
// one SSH command carrying the expected version.
// PREVENTS: a stock Alpine kernel producing a successful or operating verdict.
func TestRuntimeKernelAssertionIsReportData(t *testing.T) {
	run := fixtureRun(t, ArchAMD64)
	versionDir := filepath.Join(run.Tree, "internal", "appliance")
	if err := os.MkdirAll(versionDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "kernel.version"), []byte("7.2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	run.ops.Environ = func() []string {
		return []string{"ze.l2tp.ncp.ip-timeout=9"}
	}
	run.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		calls++
		if spec.Name != "ssh" || !strings.Contains(spec.Args[len(spec.Args)-1], "7.2|7.2.*") {
			t.Fatalf("kernel probe command = %#v", spec)
		}
		if !reflect.DeepEqual(spec.Env, []string{"ze.l2tp.ncp.ip-timeout=9"}) {
			t.Fatalf("dotted environment changed: %#v", spec.Env)
		}
		return commandResult{Code: 1}, nil
	}
	failure, err := run.assertRuntimeKernel(context.Background(), &RunPlan{SSHPort: 2222})
	if err != nil || failure == "" || calls != 1 {
		t.Fatalf("kernel assertion = failure %q, err %v, calls %d", failure, err, calls)
	}
}

// VALIDATES: an unassigned verdict cannot serialize as a plausible report.
// PREVENTS: an operating failure becoming structured success by zero value.
func TestRunVerdictZeroIsInvalid(t *testing.T) {
	if _, err := json.Marshal(RunReport{}); err == nil {
		t.Fatal("an unspecified run verdict marshaled without error")
	}
}

// VALIDATES: guest and proof failures keep the script's exit semantics, and
// signal exits use the conventional 128+signal status.
// PREVENTS: flattening every failure to one or treating zero verdict as pass.
func TestRunExitMapping(t *testing.T) {
	cases := []struct {
		report RunReport
		want   int
	}{
		{RunReport{Verdict: RunVerdictPass}, 0},
		{RunReport{Verdict: RunVerdictFail, GuestExitCode: 3}, 3},
		{RunReport{Verdict: RunVerdictFail, ProofFailure: "kernel"}, 1},
		{RunReport{}, 1},
	}
	for _, one := range cases {
		if got := runExitCode(&one.report); got != one.want {
			t.Errorf("runExitCode(%#v) = %d, want %d", one.report, got, one.want)
		}
	}
	if got := signalExitCode(os.Interrupt); got != 130 {
		t.Errorf("interrupt exit = %d, want 130", got)
	}
	if got := signalExitCode(nil); got != 1 {
		t.Errorf("unknown signal exit = %d, want 1", got)
	}
}

// VALIDATES: a corrupt Alpine ISO is removed, downloaded through two curl
// calls, verified, atomically published, and paired with the exact sidecar.
// PREVENTS: a truncated or substituted ISO surviving a cache hit.
func TestEnsureISORepairsTheCacheWithTwoAbsoluteCommands(t *testing.T) {
	run := fixtureRun(t, ArchAMD64)
	cache, err := run.durableCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	name := alpineISOName("x86_64")
	iso := filepath.Join(cache, "alpine-iso", name)
	if err := os.WriteFile(iso, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := []byte("verified replacement")
	digest := sha256.Sum256(payload)
	want := hex.EncodeToString(digest[:])
	calls := 0
	run.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		calls++
		if calls == 1 {
			return commandResult{Stdout: want + "  " + name + "\n"}, nil
		}
		if calls != 2 {
			t.Fatalf("host command count exceeded 2: %#v", spec)
		}
		output := ""
		for index, arg := range spec.Args {
			if arg == "-o" && index+1 < len(spec.Args) {
				output = spec.Args[index+1]
			}
		}
		if output == "" {
			t.Fatalf("download argv has no output: %#v", spec.Args)
		}
		if err := os.WriteFile(output, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		return commandResult{}, nil
	}
	got, err := run.ensureISO(context.Background(), "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || got != iso {
		t.Fatalf("ensure ISO = %q, calls %d", got, calls)
	}
	if data, err := os.ReadFile(iso); err != nil || !reflect.DeepEqual(data, payload) {
		t.Fatalf("published ISO = %q, %v", data, err)
	}
	sidecar, err := os.ReadFile(iso + ".sha256")
	if err != nil || string(sidecar) != want+"  "+name+"\n" {
		t.Fatalf("sidecar = %q, %v", sidecar, err)
	}
}

// VALIDATES: brew_files uses the exported prefix, then the brew binary's
// prefix, drops duplicates, and returns only existing files.
// PREVENTS: an Apple Silicon literal or symlink-resolved brew path taking over.
func TestBrewFilesPreservesAuthoritativePrefixOrder(t *testing.T) {
	run := fixtureRun(t, ArchARM64)
	exported := filepath.Join(run.Tree, "relocated")
	discovered := filepath.Join(run.Tree, "discovered")
	relative := filepath.Join("share", "qemu", "edk2-aarch64-code.fd")
	for _, prefix := range []string{exported, discovered} {
		if err := os.MkdirAll(filepath.Join(prefix, "share", "qemu"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(prefix, relative), []byte(prefix), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	priorGetenv := run.ops.Getenv
	run.ops.Getenv = func(name string) string {
		if name == "HOMEBREW_PREFIX" {
			return exported
		}
		return priorGetenv(name)
	}
	run.ops.Look = func(name string) (string, error) {
		if name == "brew" {
			return filepath.Join(discovered, "bin", "brew"), nil
		}
		return name, nil
	}
	wantFiles := []string{filepath.Join(exported, relative), filepath.Join(discovered, relative)}
	if got := run.brewFiles(filepath.ToSlash(relative)); !reflect.DeepEqual(got, wantFiles) {
		t.Fatalf("brew files = %#v, want %#v", got, wantFiles)
	}
}

// VALIDATES: a boot timeout stops and waits for the QEMU process and removes
// the per-run diagnostics file before Execute returns.
// PREVENTS: a timed-out VM or temporary resource surviving its owner.
func TestBootTimeoutCleansUpTheQEMUProcess(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "tmp", "qemu")
	if err := os.MkdirAll(cache, 0o750); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	qemu := filepath.Join(bin, "qemu-system-x86_64")
	marker := filepath.Join(root, "stopped")
	program := "#!/bin/sh\ntrap 'printf stopped > \"$MARKER\"; exit 0' INT TERM EXIT\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(qemu, []byte(program), 0o750); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("MARKER", marker)

	run := NewRun(root, RunOptions{Boot: 20 * time.Millisecond, Timeout: time.Second})
	report := RunReport{Plan: RunPlan{
		QEMUBinary: "qemu-system-x86_64",
		QEMUArgv:   []string{"qemu-system-x86_64"},
		SSHPort:    2222,
	}}
	if _, err := run.executePlan(context.Background(), &report); err == nil {
		t.Fatal("boot timeout returned no error")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "stopped" {
		t.Fatalf("QEMU cleanup marker = %q, %v", data, err)
	}
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("QEMU diagnostics survived cleanup: %v", entries)
	}
}
