// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: dockerkernel_apparmor.go -- the action this file serves on macOS
// Related: ../../dockerhost/dockerhost.go -- ColimaProfile and ColimaSSH
//
// On macOS the Docker daemon runs in a Linux VM. Colima's Ubuntu VM applies
// AppArmor, and its kernel holds only the
// profiles loaded inside it, so the kernel probe's ze-kernel-probe is absent
// there until something loads it: runc then fails writing
// attr/apparmor/exec, which is what the kernel answers for a profile it has
// not loaded (colima 29.5.2, Ubuntu 24.04, 6.8.0-117-generic, 2026-10-21).
// `le setup docker-kernel apparmor` therefore runs on macOS too: it finds the
// Colima profile behind the client's Docker endpoint, asks that VM whether it
// can load a profile, and runs the same two root steps inside it through
// `colima ssh`. The staged profile reaches the VM through Colima's mount of
// the home directory, which places it at the same path inside the VM.

package dockerkernel

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/dockerhost"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// osDarwin is the one platform besides Linux the AppArmor action runs on,
// against Colima's VM.
const osDarwin = "darwin"

// vmCommandTimeout bounds one question to the Docker client or the VM, so a
// VM that accepts ssh and never answers stops the action rather than hanging it.
const vmCommandTimeout = 30 * time.Second

// dockerEndpointArgv asks the Docker client which endpoint it talks to. It
// honors DOCKER_HOST and the current context alike, so it names the daemon
// every lab reaches, under the environment the labs run docker with.
var dockerEndpointArgv = []string{"docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}"}

// dockerEndpointCommand is dockerEndpointArgv as a refusal names it.
var dockerEndpointCommand = strings.Join(dockerEndpointArgv, " ")

// appArmorHost is the machine whose kernel runs the Docker daemon's
// containers, ready to load a profile: via is the argv prefix that runs a
// command there (none on Linux, where it is this machine; `colima ssh` on
// macOS), parser is apparmor_parser's path on it, and profile is the Colima
// profile, empty on Linux.
type appArmorHost struct {
	via     []string
	parser  string
	profile string
}

// vmResult is what one command answered: its output and exit status.
type vmResult struct {
	Stdout string
	Stderr string
	Exit   int
}

// vmRun runs argv and answers its result; the error is for a command that
// could not run at all. It is a function type, so a test scripts the client
// and the VM.
type vmRun func(ctx context.Context, argv []string) (vmResult, error)

// runVMCommand runs argv under the environment the labs run docker with
// (dockerhost.Inherited), with no terminal, within vmCommandTimeout.
func runVMCommand(ctx context.Context, argv []string) (vmResult, error) {
	ctx, cancel := context.WithTimeout(ctx, vmCommandTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // argv is this file's own question to the Docker client or the VM
	command.Env = dockerhost.Inherited()
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if errors.Is(err, exec.ErrNotFound) {
		return vmResult{}, vmCommandMissing(argv)
	}
	if ctx.Err() != nil {
		return vmResult{}, vmCommandHung(argv)
	}
	result := vmResult{Stdout: stdout.String(), Stderr: strings.TrimSpace(stderr.String())}
	var exited *exec.ExitError
	if errors.As(err, &exited) {
		result.Exit = exited.ExitCode()
		return result, nil
	}
	if err != nil {
		var tb textbuf.Buffer
		return vmResult{}, errors.New(tb.Str(strings.Join(argv, " ")).Str(": ").Err(err).String())
	}
	return result, nil
}

// vmCommandMissing is the refusal for a question whose program, colima or
// docker, is not installed, naming the Homebrew formula that installs it.
func vmCommandMissing(argv []string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("`").Str(argv[0]).Str("` is not installed, and `le setup docker-kernel apparmor` asks it which VM runs the Docker daemon. Install it with:\n  brew install ").
		Str(argv[0]).String())
}

// vmCommandHung is the refusal for a question that did not answer within
// vmCommandTimeout: it names the command, and the command that shows the
// state of the Colima profile it asked, or of every profile when it asked the
// Docker client.
func vmCommandHung(argv []string) error {
	status := "colima list"
	if profile := colimaProfileOf(argv); profile != "" {
		status = "colima status --profile " + profile
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("`").Str(strings.Join(argv, " ")).Str("` did not answer within ").
		Str(vmCommandTimeout.String()).Str(", so the VM's state is unknown. Check it with:\n  ").Str(status).String())
}

// colimaProfileOf answers the profile a `colima ssh --profile P --` argv names,
// empty for any other argv.
func colimaProfileOf(argv []string) string {
	for i, word := range argv {
		if word == "--" {
			return ""
		}
		if word == "--profile" && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	return ""
}

// appArmorDarwinHost answers the Colima VM behind the client's Docker endpoint,
// once that VM answers, shows AppArmor enabled and has apparmor_parser, or a
// refusal naming why not and the fix. exists recognizes a Colima home
// (dockerhost.ColimaProfile).
func appArmorDarwinHost(ctx context.Context, run vmRun, exists func(string) bool) (appArmorHost, error) {
	answer, err := run(ctx, dockerEndpointArgv)
	if err != nil {
		return appArmorHost{}, err
	}
	if answer.Exit != 0 {
		var tb textbuf.Buffer
		return appArmorHost{}, errors.New(tb.Str("le could not ask the Docker client which daemon it uses: `").
			Str(dockerEndpointCommand).Str("` exit ").Str(strconv.Itoa(answer.Exit)).Str(": ").Str(answer.Stderr).
			Str(". The profile is loaded into the VM behind that endpoint, so the client must name one: start Colima with:\n  colima start").String())
	}
	endpoint := strings.TrimSpace(answer.Stdout)
	profile, colima := dockerhost.ColimaProfile(endpoint, exists)
	if !colima {
		var tb textbuf.Buffer
		return appArmorHost{}, errors.New(tb.Str("the Docker client uses ").Str(endpoint).
			Str(", which is not a Colima VM's socket. On macOS `le setup docker-kernel apparmor` loads a profile only into the Colima VM that runs the Docker daemon. ").
			Str(interoplab.DockerKernelRoute(osDarwin, nil, nil)).String())
	}
	via := dockerhost.ColimaSSH(profile)

	reached, err := run(ctx, append(slices.Clone(via), "true"))
	if err != nil {
		return appArmorHost{}, err
	}
	if reached.Exit != 0 {
		var tb textbuf.Buffer
		return appArmorHost{}, errors.New(tb.Str("le could not reach the Colima VM of profile ").Str(profile).
			Str(" (exit ").Str(strconv.Itoa(reached.Exit)).Str(": ").Str(reached.Stderr).
			Str("). The profile is loaded inside that VM, so it must be running. Start it with:\n  colima start --profile ").
			Str(profile).String())
	}
	// The VM answers, so a failed read is the kernel's: no AppArmor module,
	// which appArmorHostReady refuses as it does an absent file on Linux.
	enabled, err := run(ctx, append(slices.Clone(via), "cat", appArmorEnabledPath))
	if err != nil {
		return appArmorHost{}, err
	}
	enabledText := enabled.Stdout
	if enabled.Exit != 0 {
		enabledText = ""
	}
	parser, err := vmAppArmorParser(ctx, via, run)
	if err != nil {
		return appArmorHost{}, err
	}
	if err := appArmorHostReady(enabledText, parser); err != nil {
		return appArmorHost{}, err
	}
	return appArmorHost{via: via, parser: parser, profile: profile}, nil
}

// vmAppArmorParser answers the first of appArmorParserPaths the VM can
// execute, empty when it has none.
func vmAppArmorParser(ctx context.Context, via []string, run vmRun) (string, error) {
	for _, path := range appArmorParserPaths {
		result, err := run(ctx, append(slices.Clone(via), "test", "-x", path))
		if err != nil {
			return "", err
		}
		if result.Exit == 0 {
			return path, nil
		}
	}
	return "", nil
}

// appArmorStagedVisible refuses, before any root step, a staged profile the
// VM cannot read: the install step inside the VM copies it from the path it
// has on the Mac, which Colima's mount of the home directory makes the same.
// On Linux the file is on the machine that installs it, so nothing is asked.
func appArmorStagedVisible(ctx context.Context, host appArmorHost, staged string, run vmRun) error {
	if len(host.via) == 0 {
		return nil
	}
	result, err := run(ctx, append(slices.Clone(host.via), "test", "-r", staged))
	if err != nil {
		return err
	}
	if result.Exit == 0 {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("the Colima VM cannot read the staged profile ").Str(staged).
		Str(", so installing it there would fail. Colima mounts the home directory into the VM by default; this checkout is outside every mount of the profile ").
		Str(host.profile).Str(". Add its directory under `mounts:` in the profile's configuration, then rerun, with:\n  colima start --profile ").
		Str(host.profile).Str(" --edit").String())
}
