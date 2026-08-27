// Design: scripts/evidence/docker-run.py -- the host-side Docker evidence lifecycle
// Overview: docker_run_report.go -- the structured result and complete plan
// Related: actions.go -- the argument-aware evidence action

package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/featuretags"
	"github.com/ze-software/ze/internal/le/leaction"
)

const (
	DockerEvidenceImageKey    = "ZE_DOCKER_EVIDENCE_IMAGE"
	DockerEvidencePlatformKey = "ZE_DOCKER_EVIDENCE_PLATFORM"
	DockerEvidenceGoarchKey   = "ZE_DOCKER_EVIDENCE_GOARCH"

	DockerEvidenceImageDefault    = "alpine:3.20"
	DockerEvidencePlatformDefault = "linux/amd64"
	DockerEvidenceGoarchDefault   = "amd64"

	dockerCleanupTimeout = 30 * time.Second
	dockerStopGrace      = 5 * time.Second
)

// DockerRunOptions is the validated command boundary for one run.
type DockerRunOptions struct {
	Script      string
	Packages    []string
	Environment []string
	Image       string
	Platform    string
	Goarch      string
}

// ParseDockerRunArguments validates the closed action grammar before any effect.
func ParseDockerRunArguments(args leaction.Arguments, environ []string) (DockerRunOptions, error) {
	options := DockerRunOptions{
		Script:      args["script"],
		Packages:    []string{},
		Environment: []string{},
		Image:       environmentValue(environ, DockerEvidenceImageKey, DockerEvidenceImageDefault),
		Platform:    environmentValue(environ, DockerEvidencePlatformKey, DockerEvidencePlatformDefault),
		Goarch:      environmentValue(environ, DockerEvidenceGoarchKey, DockerEvidenceGoarchDefault),
	}
	if options.Script == "" {
		return DockerRunOptions{}, errors.New("argument keyword \"script\" requires <path>")
	}
	if strings.IndexByte(options.Script, 0) >= 0 {
		return DockerRunOptions{}, errors.New("evidence script path contains a NUL byte")
	}
	if args.Has("packages") {
		options.Packages = strings.Fields(args["packages"])
		if len(options.Packages) == 0 {
			return DockerRunOptions{}, errors.New("argument keyword \"packages\" requires a non-empty space-list")
		}
	}
	if args.Has("environment") {
		if err := json.Unmarshal([]byte(args["environment"]), &options.Environment); err != nil {
			return DockerRunOptions{}, fmt.Errorf("decode environment JSON array: %w", err)
		}
		if strings.TrimSpace(args["environment"]) == "null" {
			return DockerRunOptions{}, errors.New("environment must be a JSON array of KEY=VALUE strings, not null")
		}
		for _, item := range options.Environment {
			key, _, ok := strings.Cut(item, "=")
			if !ok || key == "" || strings.IndexByte(item, 0) >= 0 {
				return DockerRunOptions{}, fmt.Errorf("environment entry %q must be KEY=VALUE", item)
			}
		}
	}
	return options, nil
}

func environmentValue(environ []string, key, fallback string) string {
	var tb textbuf.Buffer
	prefix := tb.Str(key).Byte('=').String()
	for _, item := range environ {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return fallback
}

type dockerHostCommand struct {
	program     string
	arguments   []string
	directory   string
	environment []string
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	capture     bool
	discard     bool
}

type dockerHostResult struct {
	stdout string
	stderr string
	code   int
}

// dockerHostProcess is one started docker exec. The caller MUST call Wait.
// Signal starts its bounded stop path but does not replace Wait.
type dockerHostProcess interface {
	Wait() (dockerHostResult, error)
	Signal(os.Signal) error
}

type dockerRunOps struct {
	lookPath   func(string) error
	run        func(context.Context, dockerHostCommand) (dockerHostResult, error)
	start      func(dockerHostCommand) (dockerHostProcess, error)
	modulesDir func() bool
	pid        func() int
}

// DockerRun owns one container lifecycle. Execute MUST be called at most once.
// Not safe for concurrent use.
type DockerRun struct {
	Tree        string
	Options     DockerRunOptions
	Environment []string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	ops         dockerRunOps
}

// NewDockerRun answers the production runner over tree.
func NewDockerRun(tree string, options DockerRunOptions) *DockerRun {
	return &DockerRun{
		Tree: tree, Options: options, Environment: os.Environ(),
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		ops: dockerRunOps{
			lookPath: lookDockerRunPath, run: runDockerHostCommand,
			start: startDockerHostCommand,
			modulesDir: func() bool {
				info, err := os.Stat("/lib/modules")
				return err == nil && info.IsDir()
			},
			pid: os.Getpid,
		},
	}
}

// Execute validates the plan, builds ze, runs the script, and removes the container.
func (r *DockerRun) Execute(ctx context.Context) (report DockerRunReport, runErr error) {
	if runErr = r.requiredCommand("docker"); runErr != nil {
		return report, runErr
	}
	report, runErr = r.plan()
	if runErr != nil {
		return report, runErr
	}
	if runErr = r.ensureImage(ctx, &report); runErr != nil {
		return report, runErr
	}
	if runErr = r.requiredCommand("go"); runErr != nil {
		return report, runErr
	}
	if runErr = r.buildZe(ctx, &report); runErr != nil {
		return report, runErr
	}
	if runErr = r.startContainer(ctx, &report); runErr != nil {
		return report, runErr
	}

	cleaned := false
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		r.removeContainer(&report)
	}
	defer cleanup()

	if runErr = r.installPackages(ctx, &report); runErr != nil {
		if errors.Is(runErr, context.Canceled) {
			r.removeContainer(&report)
		}
		return report, runErr
	}
	code, canceled, runErr := r.runScript(ctx, &report)
	if runErr != nil {
		return report, runErr
	}
	if canceled {
		return report, context.Canceled
	}
	report.InnerExitCode = code
	report.Code = code
	if code == 0 {
		report.Verdict = DockerRunVerdictPass
	} else {
		report.Verdict = DockerRunVerdictFail
	}
	return report, nil
}

func (r *DockerRun) plan() (DockerRunReport, error) {
	tree, err := filepath.Abs(r.Tree)
	if err != nil {
		return DockerRunReport{}, fmt.Errorf("resolve repository root %q: %w", r.Tree, err)
	}
	script := r.Options.Script
	if !filepath.IsAbs(script) {
		script = filepath.Join(tree, script)
	}
	script = filepath.Clean(script)
	info, err := os.Stat(script)
	if err != nil || !info.Mode().IsRegular() {
		return DockerRunReport{}, fmt.Errorf("evidence script not found: %s", script)
	}
	var tb textbuf.Buffer
	parentPrefix := tb.Str("..").Byte(os.PathSeparator).String()
	relScript, err := filepath.Rel(tree, script)
	if err != nil || relScript == ".." || strings.HasPrefix(relScript, parentPrefix) {
		return DockerRunReport{}, fmt.Errorf("evidence script is outside repository root: %s", script)
	}
	tb.Reset()
	zeRel := filepath.Join("tmp", "evidence", "bin", tb.Str("ze-linux-").Str(r.Options.Goarch).String())
	container := tb.Reset().Str("ze-evidence-").Int(int64(r.ops.pid())).String()
	forwarded := r.forwardedEnvironment(zeRel)
	return DockerRunReport{Plan: DockerRunPlan{
		Tree: tree, Script: filepath.ToSlash(relScript), Packages: slices.Clone(r.Options.Packages),
		Environment: forwarded, Image: r.Options.Image, Platform: r.Options.Platform,
		Goarch: r.Options.Goarch, ZeBinary: filepath.ToSlash(zeRel), Container: container,
		Commands: []DockerRunCommand{},
	}}, nil
}

func (r *DockerRun) requiredCommand(name string) error {
	return r.ops.lookPath(name)
}

func (r *DockerRun) ensureImage(ctx context.Context, report *DockerRunReport) error {
	inspect := dockerHostCommand{program: "docker", arguments: []string{"image", "inspect", report.Plan.Image}, discard: true}
	result, err := r.runCommand(ctx, report, inspect)
	if err != nil {
		return fmt.Errorf("inspect Docker image %q: %w", report.Plan.Image, err)
	}
	if result.code == 0 {
		return nil
	}
	if r.Stderr != nil {
		var tb textbuf.Buffer
		io.WriteString(r.Stderr, tb.Str("pulling ").Str(report.Plan.Image).Str("...\n").String()) //nolint:errcheck // progress output
	}
	pull := dockerHostCommand{program: "docker", arguments: []string{"pull", report.Plan.Image}, stdout: r.Stdout, stderr: r.Stderr}
	result, err = r.runCommand(ctx, report, pull)
	if err != nil {
		return err
	}
	if result.code != 0 {
		var tb textbuf.Buffer
		return errors.New(tb.Str("docker pull ").Str(report.Plan.Image).Str(" failed").String())
	}
	return nil
}

func (r *DockerRun) buildZe(ctx context.Context, report *DockerRunReport) error {
	ze := filepath.Join(report.Plan.Tree, filepath.FromSlash(report.Plan.ZeBinary))
	if err := os.MkdirAll(filepath.Dir(ze), 0o777); err != nil {
		return fmt.Errorf("create evidence binary directory: %w", err)
	}
	tags, err := featuretags.DaemonBuildTags(report.Plan.Tree, featuretags.DaemonBase)
	if err != nil {
		return err
	}
	report.Plan.BuildTags = tags
	environ := setDockerRunEnvironment(slices.Clone(r.Environment), "GOOS", "linux")
	environ = setDockerRunEnvironment(environ, "GOARCH", report.Plan.Goarch)
	environ = setDockerRunEnvironment(environ, "CGO_ENABLED", "0")
	if !hasDockerRunEnvironment(environ, "GOCACHE") {
		environ = append(environ, environmentPair("GOCACHE", filepath.Join(report.Plan.Tree, "tmp", "go-cache")))
	}
	build := dockerHostCommand{
		program: "go", arguments: []string{"build", "-tags", report.Plan.BuildTags, "-o", ze, "./cmd/ze"},
		directory: report.Plan.Tree, environment: environ, stdout: r.Stdout, stderr: r.Stderr,
	}
	result, err := r.runCommand(ctx, report, build)
	if err != nil {
		return err
	}
	if result.code != 0 {
		return errors.New("go build ./cmd/ze failed")
	}
	return nil
}

func (r *DockerRun) startContainer(ctx context.Context, report *DockerRunReport) error {
	report.Plan.ModuleMounted = r.ops.modulesDir()
	args := []string{"run", "--rm", "--detach", "--privileged", "--platform", report.Plan.Platform,
		"--name", report.Plan.Container, "-v", mountPair(report.Plan.Tree, "/src")}
	if report.Plan.ModuleMounted {
		args = append(args, "-v", "/lib/modules:/lib/modules:ro")
	}
	args = append(args, "-w", "/src", report.Plan.Image, "sleep", "infinity")
	result, err := r.runCommand(ctx, report, dockerHostCommand{program: "docker", arguments: args, capture: true})
	if err != nil {
		return err
	}
	if result.code == 0 {
		return nil
	}
	if r.Stderr != nil {
		io.WriteString(r.Stderr, result.stdout) //nolint:errcheck // producer stream parity
		io.WriteString(r.Stderr, result.stderr) //nolint:errcheck // producer stream parity
	}
	return errors.New("failed to start evidence container")
}

func (r *DockerRun) installPackages(ctx context.Context, report *DockerRunReport) error {
	if len(report.Plan.Packages) == 0 {
		return nil
	}
	args := []string{"exec", report.Plan.Container, "apk", "add", "--no-cache"}
	args = append(args, report.Plan.Packages...)
	result, err := r.runCommand(ctx, report, dockerHostCommand{
		program: "docker", arguments: args, stdin: r.Stdin, stdout: r.Stdout, stderr: r.Stderr,
	})
	if err != nil {
		return err
	}
	if result.code != 0 {
		var tb textbuf.Buffer
		return errors.New(tb.Str("apk add ").Join(report.Plan.Packages, " ").Str(" failed").String())
	}
	return nil
}

// runScript starts one docker exec and MUST wait for it before it returns.
func (r *DockerRun) runScript(ctx context.Context, report *DockerRunReport) (int, bool, error) {
	args := []string{"exec"}
	for _, item := range report.Plan.Environment {
		args = append(args, "--env", item)
	}
	args = append(args, report.Plan.Container, "python3", filepath.ToSlash(filepath.Join("/src", report.Plan.Script)))
	command := dockerHostCommand{program: "docker", arguments: args, stdin: r.Stdin, stdout: r.Stdout, stderr: r.Stderr}
	r.record(report, command)
	process, err := r.ops.start(command)
	if err != nil {
		return 0, false, fmt.Errorf("start evidence script: %w", err)
	}
	waited := make(chan dockerHostResult, 1)
	failed := make(chan error, 1)
	go func() {
		result, waitErr := process.Wait()
		if waitErr != nil {
			failed <- waitErr
			return
		}
		waited <- result
	}()
	select {
	case result := <-waited:
		return result.code, false, nil
	case waitErr := <-failed:
		return 0, false, waitErr
	case <-ctx.Done():
		// The producer removes here in its SIGTERM handler and again in its
		// finally block. Execute's deferred cleanup supplies the second call.
		r.removeContainer(report)
		process.Signal(syscall.SIGTERM) //nolint:errcheck // container removal is the primary stop path
		select {
		case <-waited:
		case <-failed:
		case <-time.After(dockerStopGrace):
			process.Signal(os.Kill) //nolint:errcheck // SIGTERM and container removal did not stop it
			select {
			case <-waited:
			case <-failed:
			case <-time.After(dockerStopGrace):
			}
		}
		return 0, true, nil
	}
}

func (r *DockerRun) removeContainer(report *DockerRunReport) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerCleanupTimeout)
	defer cancel()
	command := dockerHostCommand{program: "docker", arguments: []string{"rm", "-f", report.Plan.Container}, discard: true}
	r.runCommand(ctx, report, command) //nolint:errcheck // the inner verdict owns the answer
	report.Cleanup = true
}

func (r *DockerRun) forwardedEnvironment(zeRel string) []string {
	forwarded := []string{environmentPair("ZE_EVIDENCE_ZE_BINARY", filepath.ToSlash(filepath.Join("/src", zeRel)))}
	for _, item := range r.Environment {
		key, _, ok := strings.Cut(item, "=")
		if ok && (strings.HasPrefix(key, "ZE_") || strings.HasPrefix(key, "ze.")) {
			forwarded = append(forwarded, item)
		}
	}
	return append(forwarded, r.Options.Environment...)
}

func (r *DockerRun) runCommand(ctx context.Context, report *DockerRunReport, command dockerHostCommand) (dockerHostResult, error) {
	r.record(report, command)
	return r.ops.run(ctx, command)
}

func (r *DockerRun) record(report *DockerRunReport, command dockerHostCommand) {
	report.Plan.Commands = append(report.Plan.Commands, DockerRunCommand{
		Program: command.program, Arguments: slices.Clone(command.arguments), Directory: command.directory,
	})
}

func environmentPair(key, value string) string {
	var tb textbuf.Buffer
	return tb.Str(key).Byte('=').Str(value).String()
}

func mountPair(source, target string) string {
	var tb textbuf.Buffer
	return tb.Str(source).Byte(':').Str(target).String()
}

func hasDockerRunEnvironment(environ []string, key string) bool {
	var tb textbuf.Buffer
	prefix := tb.Str(key).Byte('=').String()
	for _, item := range environ {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

func setDockerRunEnvironment(environ []string, key, value string) []string {
	var tb textbuf.Buffer
	prefix := tb.Str(key).Byte('=').String()
	for index, item := range environ {
		if strings.HasPrefix(item, prefix) {
			environ[index] = environmentPair(key, value)
			return environ
		}
	}
	return append(environ, environmentPair(key, value))
}

func lookDockerRunPath(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("missing required command: ").Str(name).String())
	}
	return nil
}

func runDockerHostCommand(ctx context.Context, command dockerHostCommand) (dockerHostResult, error) {
	cmd := exec.CommandContext(ctx, command.program, command.arguments...) //nolint:gosec // closed grammar builds every argv
	configureDockerHostCommand(cmd, command)
	var stdout, stderr strings.Builder
	if command.capture {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}
	if command.discard {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}
	err := cmd.Run()
	result := dockerHostResult{stdout: stdout.String(), stderr: stderr.String(), code: commandExitCode(err)}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return result, err
		}
	}
	return result, nil
}

func startDockerHostCommand(command dockerHostCommand) (dockerHostProcess, error) {
	cmd := exec.Command(command.program, command.arguments...) //nolint:gosec,noctx // the caller owns cancellation and cleanup
	configureDockerHostCommand(cmd, command)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &dockerExecProcess{cmd: cmd}, nil
}

func configureDockerHostCommand(cmd *exec.Cmd, command dockerHostCommand) {
	cmd.Dir = command.directory
	cmd.Env = command.environment
	cmd.Stdin = command.stdin
	cmd.Stdout = command.stdout
	cmd.Stderr = command.stderr
}

type dockerExecProcess struct{ cmd *exec.Cmd }

func (p *dockerExecProcess) Wait() (dockerHostResult, error) {
	err := p.cmd.Wait()
	result := dockerHostResult{code: commandExitCode(err)}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return result, err
		}
	}
	return result, nil
}

func (p *dockerExecProcess) Signal(signal os.Signal) error { return p.cmd.Process.Signal(signal) }

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 1
}
