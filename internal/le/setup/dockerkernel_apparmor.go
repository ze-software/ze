// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: dockerkernel.go -- the area this verb belongs to
// Related: ../interoplab/kernelcheck.go -- AppArmorLoadCommand, the command its refusals name
// Related: ../../component/kernelcap/apparmor.go -- ProbeAppArmorProfile, the profile it loads
//
// `le setup docker-kernel apparmor` loads Ze's probe profile on a Linux Docker
// host whose daemon applies AppArmor (D-7). Docker's docker-default profile
// denies the MPLS probe's mount and its /proc/sys write, so the Docker kernel
// check runs the probe under ze-kernel-probe, which grants exactly those. The
// profile goes into /etc/apparmor.d, so it loads again at boot, and is loaded
// with apparmor_parser through sudo, one printed step at a time, only after the
// operator confirms the profile by name.

package setup

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// runDockerKernelAppArmor refuses off Linux, on a host that cannot load a
// profile, and without the profile confirmed, before any step; then it
// installs and loads Ze's probe profile.
func runDockerKernelAppArmor(args leaction.Arguments) (any, int) {
	if err := appArmorLoadPlatform(runtime.GOOS); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	enabled, err := os.ReadFile(appArmorEnabledPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		leaction.ReportError(err)
		return nil, 1
	}
	parser := findAppArmorParser()
	if err := appArmorHostReady(string(enabled), parser); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	dir, err := os.MkdirTemp(filepath.Join(root, "tmp"), "apparmor-")
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	defer os.RemoveAll(dir) //nolint:errcheck // a staging directory under tmp/, nothing reads it after the steps

	staged, err := stageAppArmorProfile(dir)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	steps := appArmorLoadSteps(staged, parser)
	if err := appArmorLoadConfirmed(args.One(dockerKernelConfirmKeyword)); err != nil {
		if planErr := printInstallPlan(steps); planErr != nil {
			leaction.ReportError(planErr)
		}
		leaction.ReportError(err)
		return nil, 1
	}
	for _, step := range steps {
		if err := runInstallStep(step); err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
	}
	return map[string]string{
		"profile": kernelcap.ProbeAppArmorProfileName,
		"file":    filepath.Join(appArmorProfileDir, kernelcap.ProbeAppArmorProfileName),
		"next":    "./le setup docker-kernel check ze <linux ze>",
	}, 0
}

// appArmorLoadPlatform refuses every platform but Linux, naming the route the
// Mac takes instead (D-6).
func appArmorLoadPlatform(goos string) error {
	if goos == osLinux {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("`le setup docker-kernel apparmor` loads an AppArmor profile into a Linux Docker host's kernel and runs on Linux only. ").
		Str(interoplab.DockerKernelRoute(goos)).String())
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
			Str(" does not read Y), so Docker applies no AppArmor profile and the probe needs none: run ./le setup docker-kernel check ze <linux ze>").String())
	}
	if parser == "" {
		return errors.New("AppArmor is enabled but apparmor_parser is not installed (Debian and Ubuntu: the apparmor package), so no profile can be loaded")
	}
	return nil
}

// stageAppArmorProfile writes kernelcap's profile into dir for the install
// step, and answers its path.
func stageAppArmorProfile(dir string) (string, error) {
	path := filepath.Join(dir, kernelcap.ProbeAppArmorProfileName)
	if err := os.WriteFile(path, []byte(kernelcap.ProbeAppArmorProfile()), 0o644); err != nil { //nolint:gosec // a profile is world-readable in /etc/apparmor.d too
		return "", err
	}
	return path, nil
}

// appArmorLoadSteps answers the root steps: install the profile where it loads
// at boot, then replace the loaded copy and write the parser's cache.
func appArmorLoadSteps(staged, parser string) []installStep {
	target := filepath.Join(appArmorProfileDir, kernelcap.ProbeAppArmorProfileName)
	return []installStep{
		{Why: "install Ze's probe profile where AppArmor loads it at boot", Argv: []string{"sudo", "install", "-m", "0644", staged, target}},
		{Why: "load it now, replacing an earlier copy, and write its cache", Argv: []string{"sudo", parser, "-r", "-W", target}},
	}
}

// appArmorLoadConfirmed refuses unless the operator named the profile after
// `confirm`, and names the command that does.
func appArmorLoadConfirmed(value string) error {
	if value == kernelcap.ProbeAppArmorProfileName {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("the steps above run as root and none ran; to run them, confirm the profile: ").
		Str(interoplab.AppArmorLoadCommand).String())
}
