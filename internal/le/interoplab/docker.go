// Design: docs/architecture/testing/interop.md -- Docker lab lifecycle and failure contracts
// Related: process.go -- the bounded host process boundary.
// Related: lab.go -- scenario lifecycle built from these Docker operations.
package interoplab

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/dockerhost"
)

// The docker CLI words several command lines in this package repeat.
const (
	dockerFormatFlag      = "--format"
	dockerFilterFlag      = "--filter"
	dockerNoTruncFlag     = "--no-trunc"
	dockerInspectVerb     = "inspect"
	dockerImageObject     = "image"
	dockerNetworkObject   = "network"
	dockerContainerObject = "container"
	dockerExecVerb        = "exec"
)

// DockerExecutable is the program every lab Docker command runs. grantcheck
// skips it when a whole argv opens with it.
const DockerExecutable = "docker"

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

// serverArchitectureFormat is the Go template that asks the daemon, rather than
// the client, which architecture it runs. `docker version` reports both, and
// only the server's decides what an image built without --platform is for.
const serverArchitectureFormat = "{{.Server.Arch}}"

// dockerBuildTimeoutDefault bounds one docker build on a machine that declares
// nothing. The bound exists to stop a wedged Docker daemon, not to budget a
// build, so it is deliberately generous: a build that finishes returns at once,
// and the only cost of a large bound is a slower report of a daemon that hung.
//
// The number covers the slowest build measured, and the spread is the point.
// It was measured when every lab image compiled ze inside the container: on
// 2026-09-04 test/interop/Dockerfile.ze took 40m39s on a colima VM of 2 CPUs and
// 2 GB whose host disk was full and whose guest was thrashing, and 2m48s on the
// same VM once the disk was repaired. The 10 minutes this constant held until
// then killed the first of those, which the machine completed by hand.
//
// No lab image compiles anything now: the binaries are cross-compiled on the
// host and copied in (zebuild.go, StageBinaries), which took the same image to
// 54.7s on a 32-core workstation on 2026-09-06. The bound is unchanged, because
// a pull of a third-party peer image on a bad link is still the slow case.
//
// So the bound cannot be the thing that decides a verdict: it has to survive the
// bad day, and no constant is right for every machine. BUILD_TIMEOUT overrides
// this one in both directions, so a slower machine raises it and a build host
// that wants a wedge reported in minutes lowers it.
const dockerBuildTimeoutDefault = 90 * time.Minute

// Docker runs the closed set of Docker operations that an interop leaf needs.
// It is safe for concurrent use when its process runner is safe for concurrent use.
type Docker struct {
	runner       ProcessRunner
	buildTimeout time.Duration
	// owner is the label set every create argv carries, and judge decides a
	// labeled resource's owner; both are client-side facts (orphans.go).
	owner ownerIdentity
	judge ownerJudge
	// oneShotName names each one-shot container, so a canceled or timed-out
	// one can be removed by name: killing the docker client stops nothing the
	// daemon runs, and --rm fires only when the container's command ends.
	oneShotName func() string
	// environ is the environment every docker command runs under: this
	// process's with the Colima socket selected (dockerhost.Inherited), so a
	// macOS host reaches the same daemon `docker` on its PATH would.
	environ []string
	// unsettled names the containers whose `docker run` this client issued
	// and that failed or was canceled (D-e, 2026-10-19). The daemon finishes
	// a create it accepted even after the client is gone, so RemoveContainer
	// does not take one early "No such container" answer for such a name as
	// proof that nothing is left.
	unsettled unsettledNames
}

// ImageBuild identifies one image that a suite builds before it starts scenarios.
type ImageBuild struct {
	Name       string   `json:"name"`
	Tag        string   `json:"tag"`
	Dockerfile string   `json:"dockerfile"`
	Context    string   `json:"context"`
	BuildArgs  []string `json:"build-args,omitempty"`
	// NamedContexts are the additional build contexts a Dockerfile reads
	// with `COPY --from=<name>`, such as a sibling source checkout.
	NamedContexts []NamedBuildContext `json:"named-contexts,omitempty"`
	// Timeout lengthens the bound of this one build past the machine budget,
	// which dockerBuildTimeoutDefault answers for and BUILD_TIMEOUT overrides.
	// It only ever lengthens it: zero or a value below the machine budget
	// keeps the machine budget.
	Timeout  time.Duration `json:"timeout,omitempty"`
	Required bool          `json:"required"`
	Pull     bool          `json:"pull"`
}

// NamedBuildContext is one `--build-context <name>=<path>` of a build: a
// directory the Dockerfile reads by name rather than through the main context.
type NamedBuildContext struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ImageResult records the immutable reference that containers in this run use.
type ImageResult struct {
	Name      string `json:"name"`
	Tag       string `json:"tag"`
	Reference string `json:"reference"`
	Built     bool   `json:"built"`
	Error     string `json:"error,omitempty"`

	retainedTag string
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

// ReadyProbe asks one peer command until it succeeds and stdout contains every
// Contains string. An empty Contains keeps exit-only readiness. A caller that
// needs configured protocol state MUST name that state, not process existence.
// Environment applies only to the probe command. Timeout and Interval MUST be
// positive; the suite MUST finish readiness before starting the next peer.
type ReadyProbe struct {
	Command     []string              `json:"command"`
	Environment []EnvironmentVariable `json:"environment,omitempty"`
	Contains    []string              `json:"contains,omitempty"`
	Timeout     time.Duration         `json:"timeout"`
	Interval    time.Duration         `json:"interval"`
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
	// AppArmorProfile names the registered Ze profile the container runs
	// under on a daemon that applies AppArmor; empty keeps docker-default.
	AppArmorProfile string `json:"apparmor-profile,omitempty"`
}

// OneShotContainer is a bounded foreground container: a host preflight, the
// kernel probe, a release-candidate gate run or a terminal-demo render.
type OneShotContainer struct {
	Image string `json:"image"`
	// Network names the network the container joins (`--network`); empty
	// keeps Docker's default bridge. "host" puts a client container in the
	// host's namespace, where it reaches a listener a test opened on the host
	// loopback.
	Network string `json:"network,omitempty"`
	// Mounts are bind mounts, rendered as `-v` like a detached container's.
	Mounts    []Mount       `json:"mounts,omitempty"`
	Arguments []string      `json:"arguments,omitempty"`
	Command   []string      `json:"command,omitempty"`
	Timeout   time.Duration `json:"timeout"`
	// Stdin, Stdout and Stderr attach the container to the caller's streams
	// (ProcessCommand); a nil Stdout or Stderr is captured into the result.
	Stdin  io.Reader `json:"-"`
	Stdout io.Writer `json:"-"`
	Stderr io.Writer `json:"-"`
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
	// Timeout is the bound the command ran under. The text names it when the
	// bound is what ended the command, so the reader knows it hung, not failed.
	Timeout time.Duration `json:"-"`
}

func (e *commandError) Error() string {
	var tb textbuf.Buffer
	tb.Str(e.Operation)
	switch {
	case errors.Is(e.Cause, context.DeadlineExceeded):
		tb.Str(" did not finish within ").Str(e.Timeout.String())
	case errors.Is(e.Cause, context.Canceled):
		// A cancel is no failure of the command: le stopped it, so the
		// reader is told why rather than shown a code le's kill produced.
		tb.Str(" was canceled before it finished, by an interrupt or by the run stopping")
	default:
		tb.Str(" failed")
	}
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
	owner, judge := processIdentity()
	return &Docker{runner: systemProcessRunner{}, buildTimeout: machineBuildTimeout(os.LookupEnv), owner: owner, judge: judge, oneShotName: randomOneShotName, environ: dockerhost.Inherited()}
}

// NewFixtureDocker is the test double's client for a package outside this
// one: it runs every command through runner, under the fixed owner and a host
// on which every pid is absent, so a recorded argv is deterministic. Code that
// talks to a real daemon MUST call NewDocker: this client's labels name an
// owner that does not exist.
func NewFixtureDocker(runner ProcessRunner) *Docker {
	return newDocker(runner)
}

// newDocker is the test double's client: a fixed owner and a host on which
// every pid is absent, so a test argv is deterministic.
func newDocker(runner ProcessRunner) *Docker {
	return &Docker{runner: runner, buildTimeout: dockerBuildTimeoutDefault, owner: fixtureOwner(), judge: fixtureJudge(), oneShotName: fixtureOneShotName}
}

// oneShotNamePrefix starts every one-shot container's name.
const oneShotNamePrefix = "ze-oneshot-"

// randomOneShotName answers a name no other one-shot on any client holds.
func randomOneShotName() string {
	return oneShotNamePrefix + strings.ToLower(rand.Text())
}

// fixtureOneShotName is the test double's fixed one-shot name.
func fixtureOneShotName() string {
	return oneShotNamePrefix + "fixture"
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
	_, err := d.command(ctx, dockerInfoTimeout, DockerExecutable, "info")
	return err
}

// ServerArchitecture answers the GOARCH the Docker daemon builds images FOR.
//
// The DAEMON is asked rather than the host assumed, and that is a fact about
// Build rather than a convention: Build passes no --platform, so the image
// takes the daemon's own platform. The two agree on every machine used so far
// and differ under a remote context or DOCKER_DEFAULT_PLATFORM, where a
// host-derived GOARCH stages a binary the container cannot exec and the only
// symptom is `exec format error`, which names nothing about its cause.
//
// An answer this method cannot read is an ERROR naming what it asked and what
// it got. It never falls back to the host architecture, because a guess here is
// the failure this method exists to remove.
func (d *Docker) ServerArchitecture(ctx context.Context) (string, error) {
	result, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, "version", dockerFormatFlag, serverArchitectureFormat)
	if err != nil {
		return "", err
	}
	architecture := strings.TrimSpace(result.Stdout)
	if !architectureToken(architecture) {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("`docker version --format ").Str(serverArchitectureFormat).
			Str("` answered ").Quoted(architecture).
			Str(", which is not an architecture name; a GOARCH is never guessed from the host").String())
	}
	return architecture, nil
}

// architectureTokenMax bounds the answer this client accepts. The longest
// GOARCH Go names is `loong64`, at seven characters, so sixteen leaves room for
// a spelling nobody has written yet and still refuses a paragraph.
const architectureTokenMax = 16

// architectureToken reports whether an answer has the SHAPE of an architecture
// name: one to architectureTokenMax lowercase letters and digits.
//
// The shape is judged here and the SPELLING is judged by the toolchain. Go's
// own architecture list is the authority on which names exist, and a copy of it
// here would go stale the first time Go adds one, so an answer with the right
// shape is handed to the cross-compile, which refuses an unknown GOARCH loudly
// and before any image is built. What this guard removes is an answer that is
// not a token at all: an empty string, a diagnostic sentence, or anything
// carrying a character an environment variable should never receive.
func architectureToken(value string) bool {
	if value == "" || len(value) > architectureTokenMax {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character >= 'a' && character <= 'z' {
			continue
		}
		if character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

// Build retains a built image until ReleaseImage drops its run-owned tag:
// Suite.Run does that for a lab, and any other caller MUST call ReleaseImage
// after a successful Build. The shared Tag stays.
func (d *Docker) Build(ctx context.Context, build ImageBuild) (ImageResult, error) {
	image := ImageResult{Name: build.Name, Tag: build.Tag}
	if build.Timeout < 0 {
		return image, errors.New("Docker build timeout must not be negative")
	}
	// The image's field only ever lengthens the machine bound, the same rule
	// Pull applies to its caller's bound.
	timeout := max(build.Timeout, d.buildTimeout)
	if timeout <= 0 {
		return image, errors.New("Docker build timeout must be positive")
	}
	image.retainedTag = retainedTagRepository + ":" + rand.Text()
	arguments := []string{DockerExecutable, "build"}
	arguments = append(arguments, d.owner.labelArguments(resourceImageTag)...)
	arguments = append(arguments, "-t", build.Tag)
	for _, argument := range build.BuildArgs {
		arguments = append(arguments, "--build-arg", argument)
	}
	for _, named := range build.NamedContexts {
		if named.Name == "" {
			return image, errors.New("Docker build context name is empty")
		}
		if named.Path == "" {
			return image, errors.New("Docker build context path is empty")
		}
		var tb textbuf.Buffer
		arguments = append(arguments, "--build-context", tb.Str(named.Name).Byte('=').Str(named.Path).String())
	}
	arguments = append(arguments, "-t", image.retainedTag, "-f", build.Dockerfile, build.Context, "-q")
	result, err := d.command(ctx, timeout, arguments...)
	if err != nil {
		return image, err
	}
	lines := strings.Fields(result.Stdout)
	if len(lines) == 0 {
		return image, errors.New("docker build printed no image id")
	}
	image.Reference = lines[len(lines)-1]
	image.Built = true
	return image, nil
}

// ReleaseImage removes only the tag this build owns, never the shared cache tag.
// It MUST be called once after each successful Build.
func (d *Docker) ReleaseImage(ctx context.Context, image ImageResult) error {
	if image.retainedTag == "" {
		return nil
	}
	_, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, dockerImageObject, "rm", "--no-prune", image.retainedTag)
	if err == nil {
		return nil
	}
	if commandErr, ok := errors.AsType[*commandError](err); ok {
		var tb textbuf.Buffer
		absent := tb.Str("No such image: ").Str(image.retainedTag).String()
		for line := range strings.SplitSeq(commandErr.Stderr, "\n") {
			if strings.HasSuffix(strings.TrimSpace(line), absent) {
				return nil
			}
		}
	}
	return d.confirmRemoval(ctx, resourceImageTag, image.retainedTag, err)
}

// Pull makes one external image available before the scenarios start.
// boundMin only ever lengthens the wrapper's dockerPullTimeout: zero or a
// shorter bound keeps the default, so no caller can make a pull on a slow
// mirror fail sooner than every other pull would.
func (d *Docker) Pull(ctx context.Context, image string, boundMin time.Duration) error {
	bound := max(boundMin, dockerPullTimeout)
	_, err := d.command(ctx, bound, DockerExecutable, "pull", "-q", image)
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
	name := d.oneShotName()
	argv, err := d.oneShotArguments(spec, name)
	if err != nil {
		return CommandResult{}, err
	}
	result, err := d.runner.Run(ctx, ProcessCommand{
		Arguments: argv, Timeout: spec.Timeout, Environment: d.environ,
		Stdin: spec.Stdin, Stdout: spec.Stdout, Stderr: spec.Stderr,
	})
	if err != nil {
		// The bound or the caller's context ended the docker client, which
		// leaves the container running its command, or still being created.
		// Remove it by name, on a context the cancellation does not reach, and
		// report both.
		d.unsettled.mark(name)
		removeErr := d.RemoveContainer(context.WithoutCancel(ctx), name)
		return CommandResult(result), errors.Join(d.oneShotError(argv, spec.Timeout, result, err), removeErr)
	}
	if result.ExitCode != 0 {
		return CommandResult(result), d.oneShotError(argv, spec.Timeout, result, nil)
	}
	return CommandResult(result), nil
}

// oneShotError names a one-shot that did not exit 0, the way commandAs names
// every other docker command.
func (d *Docker) oneShotError(argv []string, timeout time.Duration, result ProcessResult, cause error) error {
	return &commandError{
		Operation: commandOperation(argv),
		Command:   argv,
		ExitCode:  result.ExitCode,
		Stderr:    strings.TrimSpace(result.Stderr),
		Cause:     cause,
		Timeout:   timeout,
	}
}

// OneShotDidNotExit reports a RunOneShot error that is not the container's own
// exit code: the arguments were refused, or the docker client failed or was
// stopped. A one-shot that exited non-zero is judged on its answer, as a
// probe's AppArmor refusal and a client's refused request need.
func OneShotDidNotExit(err error) bool {
	if err == nil {
		return false
	}
	commandErr, ok := errors.AsType[*commandError](err)
	if !ok {
		return true
	}
	return commandErr.Cause != nil
}

// oneShotArguments is every one-shot container's argv, the kernel probe's
// included, so each carries the owner labels and the name a canceled run is
// removed by.
func (d *Docker) oneShotArguments(spec OneShotContainer, name string) ([]string, error) {
	if err := refuseOwnedArguments("OneShotContainer.Arguments", "the client's own one-shot name", spec.Arguments); err != nil {
		return nil, err
	}
	labels := d.owner.labelArguments(resourceContainer)
	argv := make([]string, 0, 8+2*len(spec.Mounts)+len(spec.Arguments)+len(labels)+len(spec.Command))
	argv = append(argv, DockerExecutable, "run", "--rm", "--name", name)
	argv = append(argv, labels...)
	if spec.Network != "" {
		argv = append(argv, "--network", spec.Network)
	}
	for _, mount := range spec.Mounts {
		value, err := mountArgument(mount)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "-v", value)
	}
	argv = append(argv, spec.Arguments...)
	argv = append(argv, spec.Image)
	return append(argv, spec.Command...), nil
}

// CreateNetwork tries each declared candidate once. Only overlap moves to the next candidate.
func (d *Docker) CreateNetwork(ctx context.Context, spec NetworkSpec) (Network, error) {
	if spec.Name == "" {
		return Network{}, errors.New("Docker network name is empty")
	}
	if len(spec.Candidates) == 0 {
		return Network{}, errors.New("Docker network has no subnet candidates")
	}
	var last *commandError
	for _, candidate := range spec.Candidates {
		argv, err := networkCreateArguments(spec.Name, candidate, d.owner.labelArguments(resourceNetwork))
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
		// A network of that name is another run's (a lab's preClean claimed
		// the name, so one made since is concurrent): this run neither joins
		// nor removes it, so its teardown cannot remove a live run's (R-1).
		if strings.Contains(last.Stderr, "already exists") {
			var refusal textbuf.Buffer
			return Network{}, errors.New(refusal.Str("docker network ").Str(spec.Name).
				Str(" already exists and this run did not create it, so this run neither uses nor removes it.").
				Str(" Fix: run `").Str(cleanupCommand).Str("`, which removes it once its owner is gone,").
				Str(" or give each concurrent run its own suffix").String())
		}
		if !strings.Contains(strings.ToLower(last.Stderr), "overlap") {
			return Network{}, err
		}
	}
	return Network{}, fmt.Errorf("docker network create exhausted %d candidates: %w", len(spec.Candidates), last)
}

// RemoveNetwork removes a network. A name-specific not-found answer is
// success, and any other failure is settled by confirmRemoval.
func (d *Docker) RemoveNetwork(ctx context.Context, name string) error {
	_, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, dockerNetworkObject, "rm", name)
	if err == nil {
		return nil
	}
	if commandErr, ok := errors.AsType[*commandError](err); ok {
		var tb textbuf.Buffer
		absent := tb.Str("network ").Str(name).Str(" not found").String()
		if strings.Contains(commandErr.Stderr, absent) {
			return nil
		}
	}
	return d.confirmRemoval(ctx, resourceNetwork, name, err)
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
	if err := refuseOwnedArguments("PeerConfig.Arguments", "PeerConfig.Container", peer.Arguments); err != nil {
		return err
	}
	ipv4, err := addressAtHost(network.IPv4, peer.Host)
	if err != nil {
		return err
	}
	argvCapacity := 26 + 2*len(peer.Capabilities) + 2*len(peer.Mounts) +
		2*len(peer.Environment) + len(peer.Arguments) + len(peer.Command)
	if network.IPv6.IsValid() {
		argvCapacity += 2
	}
	argv := make([]string, 0, argvCapacity)
	argv = append(argv, DockerExecutable, "run", "-d", "--name", peer.Container,
		"--network", network.Name, "--ip", ipv4.String())
	if network.IPv6.IsValid() {
		ipv6, addrErr := addressAtHost(network.IPv6, peer.Host)
		if addrErr != nil {
			return addrErr
		}
		argv = append(argv, "--ip6", ipv6.String())
	}
	argv = append(argv, d.owner.labelArguments(resourceContainer)...)
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
	confined := false
	if peer.AppArmorProfile != "" {
		option, optionErr := d.appArmorSecurityOption(ctx, peer.AppArmorProfile)
		if optionErr != nil {
			return optionErr
		}
		argv = append(argv, option...)
		confined = len(option) > 0
	}
	argv = append(argv, peer.Arguments...)
	argv = append(argv, peer.Image)
	argv = append(argv, peer.Command...)
	_, err = d.command(ctx, dockerRunTimeout, argv...)
	if err == nil {
		return nil
	}
	issued := &issuedRunError{err: d.runContainerFailure(peer, confined, err)}
	d.markUnsettled(peer.Container, issued)
	return issued
}

// runContainerFailure names a failed `docker run` of peer: under a Ze
// AppArmor profile the run itself is where an unloaded profile shows, because
// the profile list was unreadable or the daemon's kernel differs from this one.
func (d *Docker) runContainerFailure(peer PeerConfig, confined bool, err error) error {
	if !confined {
		return err
	}
	profile, registered := LookupAppArmorProfile(peer.AppArmorProfile)
	if !registered {
		return err
	}
	if unapplied := labAppArmorUnapplied(profile, err); unapplied != nil {
		return unapplied
	}
	return err
}

// issuedRunError is a `docker run -d` that was issued and failed. The daemon
// may have created the container before the failure, so the name may be the
// caller's to remove; an error of any other type came before docker ran.
type issuedRunError struct{ err error }

func (e *issuedRunError) Error() string { return e.err.Error() }

func (e *issuedRunError) Unwrap() error { return e.err }

// ContainerMayExist reports whether a detached run that answered err
// (RunDetached, or a lab peer's run) may have left a container under its name,
// so the caller owes its removal: true on success, and on a failure of the
// issued `docker run` itself unless the daemon refused the name as another
// container's. A refusal before docker ran created nothing and answers false.
func ContainerMayExist(err error) bool {
	if err == nil {
		return true
	}
	if _, issued := errors.AsType[*issuedRunError](err); !issued {
		return false
	}
	return !containerNameTaken(err)
}

// dockerFlagNamed reports whether argument is flag, as a separate word or in
// its --flag=value form.
func dockerFlagNamed(argument, flag string) bool {
	return argument == flag || strings.HasPrefix(argument, flag+"=")
}

// refuseOwnedArguments refuses a free-form docker argument that would replace
// what the client owns on a container: its name, which a canceled or torn-down
// run removes the container by, and its ze.lab owner labels, which a later run
// and `./le test integration cleanup` judge the owner by. A label file is
// refused too, because what it carries cannot be read here. field names the
// free-form field and nameSource the typed source of the container's name.
// Either form is a Ze defect in the caller, never an operator input.
func refuseOwnedArguments(field, nameSource string, arguments []string) error {
	var problem textbuf.Buffer
	for index, argument := range arguments {
		if dockerFlagNamed(argument, "--name") {
			return errors.New(problem.Str("BUG: ").Str(field).Str(" carries --name, which would replace the name").
				Str(" the container is removed by: the name comes from ").Str(nameSource).Str(", so drop it").String())
		}
		if dockerFlagNamed(argument, "--label-file") {
			return errors.New(problem.Str("BUG: ").Str(field).Str(" carries --label-file, which could forge a ").
				Str(labelMarker).Str(" owner label: pass each label as --label <key>=<value> outside ").
				Str(labelMarker).String())
		}
		label, isLabel := labelFlagValue(arguments, index)
		if !isLabel {
			continue
		}
		key, _, _ := strings.Cut(label, "=")
		if key != labelMarker && !strings.HasPrefix(key, labelMarker+".") {
			continue
		}
		return errors.New(problem.Str("BUG: ").Str(field).Str(" carries the owner label ").Str(key).
			Str(", which would override the one this client writes from its own identity: drop it, and name").
			Str(" any label the container needs outside ").Str(labelMarker).String())
	}
	return nil
}

// labelFlagValue answers the label arguments[index] sets, in every spelling
// docker accepts: --label <v>, --label=<v>, and -l as the last flag of a
// short-flag cluster, which takes <v> as the next argument, after an =, or
// glued on (-l <v>, -l=<v>, -l<v>, -dl <v>, -dl=<v>, -dl<v>).
func labelFlagValue(arguments []string, index int) (string, bool) {
	argument := arguments[index]
	if argument == "--label" {
		return nextArgument(arguments, index), true
	}
	if value, found := strings.CutPrefix(argument, "--label="); found {
		return value, true
	}
	if strings.HasPrefix(argument, "--") {
		return "", false
	}
	cluster, found := strings.CutPrefix(argument, "-")
	if !found {
		return "", false
	}
	return clusterLabelValue(cluster, arguments, index)
}

// runBooleanShorthands is every short flag of `docker run` and `docker create`
// that takes no value (-d -i -P -q -t), so a cluster reads on past it. Any other
// letter takes the rest of the cluster as its value, as docker's flag parser
// (spf13/pflag) reads one. --name has no short form.
const runBooleanShorthands = "diPqt"

// clusterLabelValue walks one short-flag cluster (the argument without its
// leading -) the way pflag does: past each value-free flag, and stopping at the
// first value-taking one. It answers the label when that flag is l, and false
// when another flag, or an = after a value-free flag, takes the rest.
func clusterLabelValue(cluster string, arguments []string, index int) (string, bool) {
	for position := range len(cluster) {
		letter := cluster[position]
		rest := cluster[position+1:]
		if letter == 'l' {
			if rest == "" {
				return nextArgument(arguments, index), true
			}
			return strings.TrimPrefix(rest, "="), true
		}
		if !strings.ContainsRune(runBooleanShorthands, rune(letter)) {
			return "", false
		}
		if strings.HasPrefix(rest, "=") {
			return "", false
		}
	}
	return "", false
}

// nextArgument is the value a flag at arguments[index] takes from the next
// argument: empty when none follows, which docker refuses on its own.
func nextArgument(arguments []string, index int) string {
	if index+1 < len(arguments) {
		return arguments[index+1]
	}
	return ""
}

// containerNameTaken reports whether a failed `docker run` is the daemon
// refusing a name another container holds, in which case the run created
// nothing and the container is not its to remove. nil is not taken.
func containerNameTaken(err error) bool {
	commandErr, ok := errors.AsType[*commandError](err)
	if !ok {
		return false
	}
	return strings.Contains(commandErr.Stderr, "is already in use by container")
}

// RemoveContainer force-removes one peer. Docker's name-specific
// absent-container answer is an empty pre-clean state, and any other failure
// is settled by confirmRemoval. After a start of name that failed or was
// canceled, one absent answer is not enough: removeSettlingContainer looks
// again over createSettleWindow (startsettle.go).
func (d *Docker) RemoveContainer(ctx context.Context, name string) error {
	absent, err := d.forceRemoveContainer(ctx, name)
	if !absent {
		return err
	}
	if !d.unsettled.take(name) {
		return nil
	}
	return d.removeSettlingContainer(ctx, name)
}

// forceRemoveContainer runs one `docker rm -f name`. It answers true when
// docker says no container holds the name, and otherwise false with the
// removal's outcome as confirmRemoval settles it.
func (d *Docker) forceRemoveContainer(ctx context.Context, name string) (bool, error) {
	_, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, "rm", "-f", name)
	if err == nil {
		return false, nil
	}
	if commandErr, ok := errors.AsType[*commandError](err); ok {
		var tb textbuf.Buffer
		absent := tb.Str("No such container: ").Str(name).String()
		for line := range strings.SplitSeq(commandErr.Stderr, "\n") {
			if strings.HasSuffix(strings.TrimSpace(line), absent) {
				return true, nil
			}
		}
	}
	return false, d.confirmRemoval(ctx, resourceContainer, name, err)
}

// confirmRemoval settles a removal call that failed by observing the resource
// again. A removal can outlast the client bound while the daemon completes it
// (the 2026-10-18 cleanup under load ~30 reported three removed resources as
// failed), so the failed call alone does not say the resource stays. It answers
// nil when the resource is gone, removeErr when it is still there, and a
// removalUnknownError carrying both errors when the observation fails too.
func (d *Docker) confirmRemoval(ctx context.Context, kind resourceKind, ref string, removeErr error) error {
	present, checkErr := d.resourcePresent(ctx, kind, ref)
	if checkErr != nil {
		return &removalUnknownError{removal: removeErr, check: checkErr}
	}
	if present {
		return removeErr
	}
	return nil
}

// removalUnknownError is a removal the daemon answered neither way: the
// removal call failed and the observation that would settle it failed too.
type removalUnknownError struct {
	removal error
	check   error
}

func (e *removalUnknownError) Error() string {
	var tb textbuf.Buffer
	return tb.Str("the removal failed (").Err(e.removal).
		Str(") and the check for whether it took effect failed too (").Err(e.check).Str(")").String()
}

func (e *removalUnknownError) Unwrap() []error { return []error{e.removal, e.check} }

// pauseContainer freezes every process in one peer's cgroup.
func (d *Docker) pauseContainer(ctx context.Context, name string) error {
	_, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, "pause", name)
	return err
}

// unpauseContainer resumes every process in one paused peer's cgroup.
func (d *Docker) unpauseContainer(ctx context.Context, name string) error {
	_, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, "unpause", name)
	return err
}

// startContainer restarts the same stopped container and preserves its accumulated state.
func (d *Docker) startContainer(ctx context.Context, name string) error {
	_, err := d.command(ctx, dockerRunTimeout, DockerExecutable, "start", name)
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
		DockerExecutable, "stop", "-t", strconv.Itoa(timeoutSeconds), name)
	return err
}

// Signal sends a signal to container PID 1.
func (d *Docker) Signal(ctx context.Context, name, signal string) error {
	if signal == "" {
		signal = "TERM"
	}
	_, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, "kill", "--signal", signal, name)
	return err
}

// Exec runs one command in a peer container. A non-zero exit always returns an error.
func (d *Docker) Exec(ctx context.Context, name string, argv []string, environ []EnvironmentVariable) (CommandResult, error) {
	arguments, err := dockerExecArguments([]string{DockerExecutable, dockerExecVerb}, name, argv, environ)
	if err != nil {
		return CommandResult{}, err
	}
	result, err := d.commandAs(ctx, dockerCommandTimeout, execOperation("docker exec", name, argv), arguments...)
	return CommandResult(result), err
}

// ExecDetached starts one command in a peer container and returns after Docker accepts it.
func (d *Docker) ExecDetached(ctx context.Context, name string, argv []string, environ []EnvironmentVariable) error {
	arguments, err := dockerExecArguments([]string{DockerExecutable, dockerExecVerb, "-d"}, name, argv, environ)
	if err != nil {
		return err
	}
	_, err = d.commandAs(ctx, dockerCommandTimeout, execOperation("docker exec -d", name, argv), arguments...)
	return err
}

// execOperationCommandMax bounds, in bytes, how much of the command run inside a
// container an error names. A checker's shell script would otherwise fill the line.
const execOperationCommandMax = 160

// execOperation names an exec by its container and the command it ran inside it, so
// a failure or a timeout says which of a checker's commands it was. Environment
// flags stay out: they are not the command, and a value can be a secret. A command
// is cut at its first newline and at execOperationCommandMax bytes, and " ..."
// marks the cut.
func execOperation(verb, container string, argv []string) string {
	var command textbuf.Buffer
	for i, word := range argv {
		if i > 0 {
			command.Byte(' ')
		}
		command.Str(word)
	}
	text := command.String()
	cut := false
	if line, _, found := strings.Cut(text, "\n"); found {
		text = line
		cut = true
	}
	if len(text) > execOperationCommandMax {
		text = text[:execOperationCommandMax]
		cut = true
	}
	var tb textbuf.Buffer
	tb.Str(verb).Byte(' ').Str(container).Byte(' ').Str(text)
	if cut {
		tb.Str(" ...")
	}
	return tb.String()
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
		DockerExecutable, "logs", name, "--tail", strconv.Itoa(lines))
	if err != nil {
		return LogResult{}, err
	}
	var tb textbuf.Buffer
	return LogResult{Text: tb.Str(result.Stdout).Str(result.Stderr).String(), Available: true}, nil
}

// containerPID returns the positive host PID of one container's PID 1.
func (d *Docker) containerPID(ctx context.Context, name string) (int, error) {
	result, err := d.command(ctx, dockerCommandTimeout,
		DockerExecutable, dockerInspectVerb, dockerFormatFlag, "{{.State.Pid}}", name)
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

func (d *Docker) command(ctx context.Context, timeout time.Duration, argv ...string) (ProcessResult, error) {
	return d.commandAs(ctx, timeout, commandOperation(argv), argv...)
}

// commandAs runs argv and names it operation in any error it returns.
func (d *Docker) commandAs(ctx context.Context, timeout time.Duration, operation string, argv ...string) (ProcessResult, error) {
	result, err := d.runner.Run(ctx, ProcessCommand{Arguments: argv, Timeout: timeout, Environment: d.environ})
	if err != nil {
		return result, &commandError{
			Operation: operation,
			Command:   argv,
			ExitCode:  result.ExitCode,
			Stderr:    strings.TrimSpace(result.Stderr),
			Cause:     err,
			Timeout:   timeout,
		}
	}
	if result.ExitCode != 0 {
		return result, &commandError{
			Operation: operation,
			Command:   argv,
			ExitCode:  result.ExitCode,
			Stderr:    strings.TrimSpace(result.Stderr),
			Timeout:   timeout,
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

func networkCreateArguments(name string, subnet Subnet, labels []string) ([]string, error) {
	if !subnet.IPv4.IsValid() {
		return nil, errors.New("Docker network IPv4 subnet is invalid")
	}
	if subnet.IPv4.Bits() != 24 {
		return nil, errors.New("Docker network IPv4 subnet must be a /24")
	}
	var tb textbuf.Buffer
	argv := []string{DockerExecutable, dockerNetworkObject, "create"}
	argv = append(argv, labels...)
	argv = append(argv, tb.Str("--subnet=").Str(subnet.IPv4.String()).String())
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
