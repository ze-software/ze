// Design: docs/architecture/testing/interop.md -- Docker lab lifecycle and failure contracts
// Related: process.go -- the bounded host process boundary.
// Related: lab.go -- scenario lifecycle built from these Docker operations.
package interoplab

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const dockerExecutable = "docker"

const (
	dockerInfoTimeout    = 15 * time.Second
	dockerCommandTimeout = 30 * time.Second
	dockerRunTimeout     = 60 * time.Second
	dockerPullTimeout    = 10 * time.Minute
	dockerLogsTimeout    = 15 * time.Second
	dockerStopSecondsMax = 300
)

// dockerBuildTimeoutVariable names the machine's build budget in whole seconds.
const dockerBuildTimeoutVariable = "BUILD_TIMEOUT"

// dockerBuildTimeoutDefault bounds one docker build on a machine that declares
// nothing. The bound exists to stop a wedged Docker daemon, not to budget a
// build, so it is deliberately generous: a build that finishes returns at once,
// and the only cost of a large bound is a slower report of a daemon that hung.
//
// The number covers the slowest build measured, and the spread is the point.
// test/interop/Dockerfile.ze copies the whole tree and compiles ze twice with no
// cache mount. On 2026-09-04 the same image took 40m39s on a colima VM of 2 CPUs
// and 2 GB whose host disk was full and whose guest was thrashing, and 2m48s on
// the same VM once the disk was repaired. The 10 minutes this constant held
// until then killed the first of those, which the machine completed by hand.
//
// So the bound cannot be the thing that decides a verdict: it has to survive the
// bad day, and no constant is right for every machine. BUILD_TIMEOUT overrides
// this one in both directions, so a slower machine raises it and a build host
// that wants a wedge reported in minutes lowers it.
const dockerBuildTimeoutDefault = 90 * time.Minute

// Docker runs the closed set of Docker operations that an interop leaf needs.
// It is safe for concurrent use when its process runner is safe for concurrent use.
type Docker struct {
	runner       processRunner
	buildTimeout time.Duration
}

// ImageBuild identifies one image that a suite builds before it starts scenarios.
type ImageBuild struct {
	Name       string   `json:"name"`
	Tag        string   `json:"tag"`
	Dockerfile string   `json:"dockerfile"`
	Context    string   `json:"context"`
	BuildArgs  []string `json:"build-args,omitempty"`
	// Timeout bounds this one build. Zero takes the machine budget, which
	// dockerBuildTimeoutDefault answers for and BUILD_TIMEOUT overrides.
	Timeout  time.Duration `json:"timeout,omitempty"`
	Required bool          `json:"required"`
	Pull     bool          `json:"pull"`
}

// ImageResult records the immutable reference that containers in this run use.
type ImageResult struct {
	Name      string `json:"name"`
	Tag       string `json:"tag"`
	Reference string `json:"reference"`
	Built     bool   `json:"built"`
	Error     string `json:"error,omitempty"`
}

// Subnet is one network candidate. IPv6 is invalid when the network is IPv4-only.
type Subnet struct {
	IPv4 netip.Prefix `json:"ipv4"`
	IPv6 netip.Prefix `json:"ipv6,omitzero"`
}

// NetworkSpec declares a network name and a bounded list of candidate subnets.
type NetworkSpec struct {
	Name       string   `json:"name"`
	Candidates []Subnet `json:"candidates"`
}

// Network is the candidate that Docker accepted for one scenario.
type Network struct {
	Name string       `json:"name"`
	IPv4 netip.Prefix `json:"ipv4"`
	IPv6 netip.Prefix `json:"ipv6,omitzero"`
}

// Mount is one host-to-container bind mount.
type Mount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read-only"`
}

// EnvironmentVariable is one ordered container or exec environment entry.
type EnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ReadyProbe asks one peer command until it exits successfully.
type ReadyProbe struct {
	Command  []string      `json:"command"`
	Timeout  time.Duration `json:"timeout"`
	Interval time.Duration `json:"interval"`
}

// PeerConfig declares one participant without assigning it a protocol role.
type PeerConfig struct {
	Name         string                `json:"name"`
	Container    string                `json:"container"`
	Image        string                `json:"image"`
	Host         uint8                 `json:"host"`
	Mounts       []Mount               `json:"mounts,omitempty"`
	Capabilities []string              `json:"capabilities,omitempty"`
	Environment  []EnvironmentVariable `json:"environment,omitempty"`
	Arguments    []string              `json:"arguments,omitempty"`
	Command      []string              `json:"command,omitempty"`
	Ready        *ReadyProbe           `json:"ready,omitempty"`
}

// OneShotContainer is a bounded foreground container used for host preflight.
type OneShotContainer struct {
	Image     string        `json:"image"`
	Arguments []string      `json:"arguments,omitempty"`
	Command   []string      `json:"command,omitempty"`
	Timeout   time.Duration `json:"timeout"`
}

// CommandResult records a peer command without merging output streams.
type CommandResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit-code"`
}

// LogResult distinguishes a readable empty log from a log that was not read.
type LogResult struct {
	Text      string `json:"text"`
	Available bool   `json:"available"`
}

// commandError records the failed operation and the Docker exit status.
type commandError struct {
	Operation string   `json:"operation"`
	Command   []string `json:"command"`
	ExitCode  int      `json:"exit-code"`
	Stderr    string   `json:"stderr,omitempty"`
	Cause     error    `json:"-"`
}

func (e *commandError) Error() string {
	var tb textbuf.Buffer
	tb.Str(e.Operation).Str(" failed")
	if e.ExitCode != 0 {
		tb.Str(" (exit ").Int(int64(e.ExitCode)).Byte(')')
	}
	if e.Stderr != "" {
		tb.Str(": ").Str(e.Stderr)
	}
	if e.Cause != nil {
		tb.Str(": ").Err(e.Cause)
	}
	return tb.String()
}

func (e *commandError) Unwrap() error { return e.Cause }

// NewDocker returns the real Docker client used by interop suites. It reads the
// machine's build budget once, so no suite carries that knob through its plan.
func NewDocker() *Docker {
	return &Docker{runner: systemProcessRunner{}, buildTimeout: machineBuildTimeout(os.LookupEnv)}
}

func newDocker(runner processRunner) *Docker {
	return &Docker{runner: runner, buildTimeout: dockerBuildTimeoutDefault}
}

// machineBuildTimeout reads the machine's build budget in whole seconds. A value that
// does not parse, or that is not positive, keeps the shipped default rather
// than inventing a different fallback, which is how ReadEnvironment answers an
// unreadable SESSION_TIMEOUT.
func machineBuildTimeout(lookup func(string) (string, bool)) time.Duration {
	value, ok := lookup(dockerBuildTimeoutVariable)
	if !ok {
		return dockerBuildTimeoutDefault
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return dockerBuildTimeoutDefault
	}
	if seconds <= 0 {
		return dockerBuildTimeoutDefault
	}
	return time.Duration(seconds) * time.Second
}

// Probe fails when Docker is absent, unresponsive, or refuses the current context.
func (d *Docker) Probe(ctx context.Context) error {
	_, err := d.command(ctx, dockerInfoTimeout, dockerExecutable, "info")
	return err
}

// Build builds one image and returns the image ID printed by docker build -q.
func (d *Docker) Build(ctx context.Context, build ImageBuild) (ImageResult, error) {
	timeout := build.Timeout
	if timeout == 0 {
		timeout = d.buildTimeout
	}
	if timeout <= 0 {
		return ImageResult{Name: build.Name, Tag: build.Tag}, errors.New("Docker build timeout must be positive")
	}
	arguments := []string{dockerExecutable, "build", "-t", build.Tag}
	for _, argument := range build.BuildArgs {
		arguments = append(arguments, "--build-arg", argument)
	}
	arguments = append(arguments, "-f", build.Dockerfile, build.Context, "-q")
	result, err := d.command(ctx, timeout, arguments...)
	if err != nil {
		return ImageResult{Name: build.Name, Tag: build.Tag}, err
	}
	lines := strings.Fields(result.Stdout)
	if len(lines) == 0 {
		return ImageResult{Name: build.Name, Tag: build.Tag}, errors.New("docker build printed no image id")
	}
	return ImageResult{
		Name:      build.Name,
		Tag:       build.Tag,
		Reference: lines[len(lines)-1],
		Built:     true,
	}, nil
}

// Pull makes one external image available before the scenarios start.
func (d *Docker) Pull(ctx context.Context, image string) error {
	_, err := d.command(ctx, dockerPullTimeout, dockerExecutable, "pull", "-q", image)
	return err
}

// RunOneShot runs a foreground container and removes it after the command exits.
func (d *Docker) RunOneShot(ctx context.Context, spec OneShotContainer) (CommandResult, error) {
	if spec.Image == "" {
		return CommandResult{}, errors.New("one-shot container image is empty")
	}
	if spec.Timeout <= 0 {
		return CommandResult{}, errors.New("one-shot container timeout must be positive")
	}
	argv := make([]string, 0, 4+len(spec.Arguments)+len(spec.Command))
	argv = append(argv, dockerExecutable, "run", "--rm")
	argv = append(argv, spec.Arguments...)
	argv = append(argv, spec.Image)
	argv = append(argv, spec.Command...)
	result, err := d.command(ctx, spec.Timeout, argv...)
	return CommandResult(result), err
}

// createNetwork tries each declared candidate once. Only overlap moves to the next candidate.
func (d *Docker) createNetwork(ctx context.Context, spec NetworkSpec) (Network, error) {
	if spec.Name == "" {
		return Network{}, errors.New("Docker network name is empty")
	}
	if len(spec.Candidates) == 0 {
		return Network{}, errors.New("Docker network has no subnet candidates")
	}
	var last *commandError
	for _, candidate := range spec.Candidates {
		argv, err := networkCreateArguments(spec.Name, candidate)
		if err != nil {
			return Network{}, err
		}
		_, err = d.command(ctx, dockerCommandTimeout, argv...)
		if err == nil {
			return Network{Name: spec.Name, IPv4: candidate.IPv4, IPv6: candidate.IPv6}, nil
		}
		if !errors.As(err, &last) {
			return Network{}, err
		}
		if strings.Contains(last.Stderr, "already exists") {
			return Network{Name: spec.Name, IPv4: candidate.IPv4, IPv6: candidate.IPv6}, nil
		}
		if !strings.Contains(strings.ToLower(last.Stderr), "overlap") {
			return Network{}, err
		}
	}
	return Network{}, fmt.Errorf("docker network create exhausted %d candidates: %w", len(spec.Candidates), last)
}

// removeNetwork removes a network. A name-specific not-found answer is success.
func (d *Docker) removeNetwork(ctx context.Context, name string) error {
	_, err := d.command(ctx, dockerCommandTimeout, dockerExecutable, "network", "rm", name)
	if err == nil {
		return nil
	}
	var commandErr *commandError
	if !errors.As(err, &commandErr) {
		return err
	}
	var tb textbuf.Buffer
	absent := tb.Str("network ").Str(name).Str(" not found").String()
	if strings.Contains(commandErr.Stderr, absent) {
		return nil
	}
	return err
}

// runContainer starts one peer at a deterministic address on the selected network.
func (d *Docker) runContainer(ctx context.Context, network Network, peer PeerConfig) error {
	if peer.Name == "" {
		return errors.New("peer name is empty")
	}
	if peer.Container == "" {
		return errors.New("peer container name is empty")
	}
	if peer.Image == "" {
		return errors.New("peer image is empty")
	}
	ipv4, err := addressAtHost(network.IPv4, peer.Host)
	if err != nil {
		return err
	}
	argvCapacity := 10 + 2*len(peer.Capabilities) + 2*len(peer.Mounts) +
		2*len(peer.Environment) + len(peer.Arguments) + len(peer.Command)
	if network.IPv6.IsValid() {
		argvCapacity += 2
	}
	argv := make([]string, 0, argvCapacity)
	argv = append(argv, dockerExecutable, "run", "-d", "--name", peer.Container,
		"--network", network.Name, "--ip", ipv4.String())
	if network.IPv6.IsValid() {
		ipv6, addrErr := addressAtHost(network.IPv6, peer.Host)
		if addrErr != nil {
			return addrErr
		}
		argv = append(argv, "--ip6", ipv6.String())
	}
	for _, capability := range peer.Capabilities {
		argv = append(argv, "--cap-add", capability)
	}
	for _, mount := range peer.Mounts {
		value, mountErr := mountArgument(mount)
		if mountErr != nil {
			return mountErr
		}
		argv = append(argv, "-v", value)
	}
	for _, variable := range peer.Environment {
		value, variableErr := environmentArgument(variable)
		if variableErr != nil {
			return variableErr
		}
		argv = append(argv, "-e", value)
	}
	argv = append(argv, peer.Arguments...)
	argv = append(argv, peer.Image)
	argv = append(argv, peer.Command...)
	_, err = d.command(ctx, dockerRunTimeout, argv...)
	return err
}

// RemoveContainer force-removes one peer. It accepts only Docker's
// name-specific absent-container answer as an empty pre-clean state.
func (d *Docker) RemoveContainer(ctx context.Context, name string) error {
	_, err := d.command(ctx, dockerCommandTimeout, dockerExecutable, "rm", "-f", name)
	if err == nil {
		return nil
	}
	var commandErr *commandError
	if !errors.As(err, &commandErr) {
		return err
	}
	var tb textbuf.Buffer
	absent := tb.Str("No such container: ").Str(name).String()
	for line := range strings.SplitSeq(commandErr.Stderr, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), absent) {
			return nil
		}
	}
	return err
}

// pauseContainer freezes every process in one peer's cgroup.
func (d *Docker) pauseContainer(ctx context.Context, name string) error {
	_, err := d.command(ctx, dockerCommandTimeout, dockerExecutable, "pause", name)
	return err
}

// unpauseContainer resumes every process in one paused peer's cgroup.
func (d *Docker) unpauseContainer(ctx context.Context, name string) error {
	_, err := d.command(ctx, dockerCommandTimeout, dockerExecutable, "unpause", name)
	return err
}

// startContainer restarts the same stopped container and preserves its accumulated state.
func (d *Docker) startContainer(ctx context.Context, name string) error {
	_, err := d.command(ctx, dockerRunTimeout, dockerExecutable, "start", name)
	return err
}

// stopContainer asks Docker to stop a container within the declared grace period.
func (d *Docker) stopContainer(ctx context.Context, name string, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		return errors.New("Docker stop timeout must be positive")
	}
	if timeoutSeconds > dockerStopSecondsMax {
		return errors.New("Docker stop timeout must not exceed 300 seconds")
	}
	timeout := time.Duration(timeoutSeconds)*time.Second + dockerCommandTimeout
	_, err := d.command(ctx, timeout,
		dockerExecutable, "stop", "-t", strconv.Itoa(timeoutSeconds), name)
	return err
}

// Signal sends a signal to container PID 1.
func (d *Docker) Signal(ctx context.Context, name, signal string) error {
	if signal == "" {
		signal = "TERM"
	}
	_, err := d.command(ctx, dockerCommandTimeout, dockerExecutable, "kill", "--signal", signal, name)
	return err
}

// Exec runs one command in a peer container. A non-zero exit always returns an error.
func (d *Docker) Exec(ctx context.Context, name string, argv []string, environ []EnvironmentVariable) (CommandResult, error) {
	arguments, err := dockerExecArguments([]string{dockerExecutable, "exec"}, name, argv, environ)
	if err != nil {
		return CommandResult{}, err
	}
	result, err := d.command(ctx, dockerCommandTimeout, arguments...)
	return CommandResult(result), err
}

// ExecDetached starts one command in a peer container and returns after Docker accepts it.
func (d *Docker) ExecDetached(ctx context.Context, name string, argv []string, environ []EnvironmentVariable) error {
	arguments, err := dockerExecArguments([]string{dockerExecutable, "exec", "-d"}, name, argv, environ)
	if err != nil {
		return err
	}
	_, err = d.command(ctx, dockerCommandTimeout, arguments...)
	return err
}

func dockerExecArguments(arguments []string, name string, argv []string, environ []EnvironmentVariable) ([]string, error) {
	for _, variable := range environ {
		value, err := environmentArgument(variable)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, "-e", value)
	}
	arguments = append(arguments, name)
	arguments = append(arguments, argv...)
	return arguments, nil
}

// Logs reads a bounded tail. Available is true even when a readable log is empty.
func (d *Docker) Logs(ctx context.Context, name string, lines int) (LogResult, error) {
	if lines <= 0 {
		return LogResult{}, errors.New("Docker log line count must be positive")
	}
	result, err := d.command(ctx, dockerLogsTimeout,
		dockerExecutable, "logs", name, "--tail", strconv.Itoa(lines))
	if err != nil {
		return LogResult{}, err
	}
	var tb textbuf.Buffer
	return LogResult{Text: tb.Str(result.Stdout).Str(result.Stderr).String(), Available: true}, nil
}

// containerPID returns the positive host PID of one container's PID 1.
func (d *Docker) containerPID(ctx context.Context, name string) (int, error) {
	result, err := d.command(ctx, dockerCommandTimeout,
		dockerExecutable, "inspect", "--format", "{{.State.Pid}}", name)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(result.Stdout)
	if value == "" {
		return 0, errors.New("docker inspect returned no container PID")
	}
	pid, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("docker inspect returned invalid container PID %q: %w", value, err)
	}
	if pid <= 0 {
		return 0, errors.New("docker inspect returned a non-positive container PID")
	}
	return pid, nil
}

func (d *Docker) command(ctx context.Context, timeout time.Duration, argv ...string) (processResult, error) {
	result, err := d.runner.Run(ctx, processCommand{Arguments: argv, Timeout: timeout})
	if err != nil {
		return result, &commandError{
			Operation: commandOperation(argv),
			Command:   argv,
			ExitCode:  result.ExitCode,
			Stderr:    strings.TrimSpace(result.Stderr),
			Cause:     err,
		}
	}
	if result.ExitCode != 0 {
		return result, &commandError{
			Operation: commandOperation(argv),
			Command:   argv,
			ExitCode:  result.ExitCode,
			Stderr:    strings.TrimSpace(result.Stderr),
		}
	}
	return result, nil
}

func commandOperation(argv []string) string {
	if len(argv) < 2 {
		return "docker command"
	}
	var tb textbuf.Buffer
	tb.Str("docker ").Str(argv[1])
	if len(argv) > 2 {
		tb.Byte(' ').Str(argv[2])
	}
	return tb.String()
}

func networkCreateArguments(name string, subnet Subnet) ([]string, error) {
	if !subnet.IPv4.IsValid() {
		return nil, errors.New("Docker network IPv4 subnet is invalid")
	}
	if subnet.IPv4.Bits() != 24 {
		return nil, errors.New("Docker network IPv4 subnet must be a /24")
	}
	var tb textbuf.Buffer
	argv := []string{dockerExecutable, "network", "create", tb.Str("--subnet=").Str(subnet.IPv4.String()).String()}
	if subnet.IPv6.IsValid() {
		if subnet.IPv6.Bits() != 64 {
			return nil, errors.New("Docker network IPv6 subnet must be a /64")
		}
		argv = append(argv, "--ipv6", tb.Reset().Str("--subnet=").Str(subnet.IPv6.String()).String())
	}
	return append(argv, name), nil
}

func addressAtHost(prefix netip.Prefix, host uint8) (netip.Addr, error) {
	if host == 0 {
		return netip.Addr{}, errors.New("peer host number must be between 1 and 255")
	}
	if !prefix.IsValid() {
		return netip.Addr{}, errors.New("peer subnet is invalid")
	}
	base := prefix.Masked().Addr()
	if base.Is4() {
		octets := base.As4()
		octets[3] = host
		return netip.AddrFrom4(octets), nil
	}
	octets := base.As16()
	octets[15] = host
	return netip.AddrFrom16(octets), nil
}

func mountArgument(mount Mount) (string, error) {
	if mount.Source == "" {
		return "", errors.New("mount source is empty")
	}
	if mount.Target == "" {
		return "", errors.New("mount target is empty")
	}
	var tb textbuf.Buffer
	tb.Str(mount.Source).Byte(':').Str(mount.Target)
	if mount.ReadOnly {
		tb.Str(":ro")
	}
	return tb.String(), nil
}

func environmentArgument(variable EnvironmentVariable) (string, error) {
	if variable.Name == "" {
		return "", errors.New("environment variable name is empty")
	}
	if strings.Contains(variable.Name, "=") {
		return "", errors.New("environment variable name contains '='")
	}
	var tb textbuf.Buffer
	return tb.Str(variable.Name).Byte('=').Str(variable.Value).String(), nil
}
