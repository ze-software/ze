// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: dockerkernel.go -- the area this verb belongs to
// Related: ../../../interoplab/apparmor.go -- the profile registry, and the load command its refusals name
// Related: ../../../../component/kernelcap/apparmor.go -- DockerDefaultProfile, the base of every profile
//
// `le setup docker kernel apparmor` loads a Ze AppArmor profile on a Linux
// Docker host whose daemon applies AppArmor (D-7), and on macOS into the
// Colima VM that runs the daemon (dockerkernel_apparmor_colima.go). A lab container that needs
// more than Docker's docker-default profile grants runs under a Ze profile that
// grants exactly that, never unconfined: the Docker kernel check's probe under
// ze-kernel-probe, a lab peer that writes its network sysctls at run time (the
// VRRP scenarios' ze among them) under ze-lab-net-sysctl. Every profile
// registers in interoplab, so this action loads any of them and lists them all.
// The profile goes into /etc/apparmor.d of that machine, so it loads again at
// boot, and is loaded with apparmor_parser through sudo, one printed step at a
// time, only after the operator confirms the profile by name.

package dockerkernel

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/dockerhost"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const (
	dockerKernelAppArmorVerb = "apparmor"
	// appArmorEnabledPath reads Y when the kernel's AppArmor module is enabled.
	appArmorEnabledPath = "/sys/module/apparmor/parameters/enabled"
	appArmorProfileDir  = "/etc/apparmor.d"
	appArmorParser      = "apparmor_parser"
)

// appArmorParserPaths are where distributions install apparmor_parser, which
// sits in sbin and so is often outside an operator's PATH.
var appArmorParserPaths = []string{"/usr/sbin/apparmor_parser", "/sbin/apparmor_parser"}

// runDockerKernelAppArmor runs appArmorLoad on this machine, with its real
// commands and its root steps on the terminal.
func runDockerKernelAppArmor(args leaction.Arguments) (any, int) {
	env := appArmorEnv{goos: runtime.GOOS, run: runVMCommand, exists: dockerhost.FileExists, step: runInstallStep}
	result, err := appArmorLoad(context.Background(), args.One(dockerKernelConfirmKeyword), env)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return result, 0
}

// appArmorEnv is what appArmorLoad acts through: the client's system, the
// runner for the questions to the Docker client and the VM, the file test that
// recognizes a Colima home, and the runner for one root step. A struct so a
// test scripts every one of them.
type appArmorEnv struct {
	goos   string
	run    vmRun
	exists func(string) bool
	step   func(context.Context, installStep) error
}

// appArmorLoad refuses off Linux and macOS, on a host that cannot load a
// profile, and without the profile confirmed, before any step; then it
// installs and loads the registered profile the operator confirmed on the
// machine whose kernel runs the daemon's containers. Without one it prints
// every registered profile's steps and runs none.
func appArmorLoad(ctx context.Context, confirm string, env appArmorEnv) (map[string]string, error) {
	if err := appArmorLoadPlatform(env.goos); err != nil {
		return nil, err
	}
	host, err := appArmorHostFor(ctx, env)
	if err != nil {
		return nil, err
	}
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(filepath.Join(root, "tmp"), "apparmor-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // a staging directory under tmp/, nothing reads it after the steps

	profile, confirmErr := appArmorLoadConfirmed(confirm)
	if confirmErr != nil {
		if planErr := printAppArmorLoadPlans(dir, host); planErr != nil {
			return nil, errors.Join(planErr, confirmErr)
		}
		return nil, confirmErr
	}
	staged, err := stageAppArmorProfile(dir, profile)
	if err != nil {
		return nil, err
	}
	if err := appArmorStagedVisible(ctx, host, staged, env.run); err != nil {
		return nil, err
	}
	for _, step := range appArmorLoadSteps(profile, staged, host) {
		// This verb holds no signal root: its root steps run in le's own
		// process group, so a Ctrl-C at the terminal reaches them directly.
		if err := env.step(ctx, step); err != nil {
			return nil, err
		}
	}
	return map[string]string{
		"profile": profile.Name,
		"file":    filepath.Join(appArmorProfileDir, profile.Name),
		"next":    profile.Next,
	}, nil
}

// printAppArmorLoadPlans stages every registered profile in dir and prints the
// root steps that would load it, so the refusal that follows shows what
// confirming each one runs.
func printAppArmorLoadPlans(dir string, host appArmorHost) error {
	for _, name := range interoplab.AppArmorProfileNames() {
		profile, registered := interoplab.LookupAppArmorProfile(name)
		if !registered {
			panic("BUG: AppArmorProfileNames listed " + name + ", which LookupAppArmorProfile does not hold")
		}
		staged, err := stageAppArmorProfile(dir, profile)
		if err != nil {
			return err
		}
		if err := printInstallPlan(appArmorLoadSteps(profile, staged, host)); err != nil {
			return err
		}
	}
	return nil
}

// appArmorLoadPlatform refuses every platform but Linux, where the profile
// loads into this machine's kernel, and macOS, where it loads into Colima's VM,
// naming the QEMU route instead (D-6).
func appArmorLoadPlatform(goos string) error {
	if goos == osLinux {
		return nil
	}
	if goos == osDarwin {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("`le setup docker kernel apparmor` loads an AppArmor profile into a Linux Docker host's kernel, or on macOS into Colima's VM, and runs on Linux and macOS only. ").
		Str(interoplab.DockerKernelRoute(goos, nil, nil)).String())
}

// appArmorHostFor answers the machine that loads the profile for a client on
// goos, once it shows it can: this Linux machine, or on macOS the Colima VM
// behind the client's Docker endpoint (appArmorDarwinHost).
func appArmorHostFor(ctx context.Context, env appArmorEnv) (appArmorHost, error) {
	if env.goos == osDarwin {
		return appArmorDarwinHost(ctx, env.run, env.exists)
	}
	enabled, err := os.ReadFile(appArmorEnabledPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return appArmorHost{}, err
	}
	parser := findAppArmorParser()
	if err := appArmorHostReady(string(enabled), parser); err != nil {
		return appArmorHost{}, err
	}
	return appArmorHost{parser: parser}, nil
}

// findAppArmorParser answers apparmor_parser's path, empty when absent.
func findAppArmorParser() string {
	if path, err := exec.LookPath(appArmorParser); err == nil {
		return path
	}
	for _, path := range appArmorParserPaths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// appArmorHostReady refuses a host whose kernel has AppArmor disabled (enabled
// is the module parameter, empty when the module is absent) or which has no
// apparmor_parser (parser empty).
func appArmorHostReady(enabled, parser string) error {
	if strings.TrimSpace(enabled) != "Y" {
		var tb textbuf.Buffer
		return errors.New(tb.Str("AppArmor is not enabled in this kernel (").Str(appArmorEnabledPath).
			Str(" does not read Y), so Docker applies no AppArmor profile and the probe needs none: run ./le setup docker kernel check").String())
	}
	if parser == "" {
		return errors.New("AppArmor is enabled but apparmor_parser is not installed (Debian and Ubuntu: the apparmor package), so no profile can be loaded")
	}
	return nil
}

// stageAppArmorProfile writes profile's registered text into dir for the
// install step, and answers its path.
func stageAppArmorProfile(dir string, profile interoplab.AppArmorProfile) (string, error) {
	path := filepath.Join(dir, profile.Name)
	if err := os.WriteFile(path, []byte(profile.Text()), 0o644); err != nil { //nolint:gosec // a profile is world-readable in /etc/apparmor.d too
		return "", err
	}
	return path, nil
}

// appArmorLoadSteps answers the root steps on host, each behind host.via:
// install the profile where it loads at boot, then replace the loaded copy and
// write the parser's cache.
func appArmorLoadSteps(profile interoplab.AppArmorProfile, staged string, host appArmorHost) []installStep {
	target := filepath.Join(appArmorProfileDir, profile.Name)
	install := append(slices.Clone(host.via), sudoBin, installBinary, "-m", "0644", staged, target)
	load := append(slices.Clone(host.via), sudoBin, host.parser, "-r", "-W", target)
	return []installStep{
		{Why: "install Ze's profile " + profile.Name + " where AppArmor loads it at boot", Argv: install},
		{Why: "load it now, replacing an earlier copy, and write its cache", Argv: load},
	}
}

// appArmorLoadConfirmed answers the registered profile the operator named
// after `confirm`, or refuses naming the command that loads each registered
// profile.
func appArmorLoadConfirmed(value string) (interoplab.AppArmorProfile, error) {
	if profile, registered := interoplab.LookupAppArmorProfile(value); registered {
		return profile, nil
	}
	var tb textbuf.Buffer
	tb.Str("the steps above run as root and none ran; to run them, confirm the profile to load:")
	for _, name := range interoplab.AppArmorProfileNames() {
		tb.Str("\n  ").Str(interoplab.AppArmorLoadCommandFor(name))
	}
	return interoplab.AppArmorProfile{}, errors.New(tb.String())
}
