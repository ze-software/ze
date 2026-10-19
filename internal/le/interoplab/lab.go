// Design: docs/architecture/testing/interop.md -- reusable scenario lifecycle
// Related: discover.go -- scenario names and typed checker callbacks.
// Related: docker.go -- image, network, container, and peer operations.
// Related: wait.go -- bounded readiness and query waits.
package interoplab

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// RenderedConfigDirectory holds Docker-bound files under the checkout's tmp directory.
const RenderedConfigDirectory = "interop-rendered"

// Preparer renders configuration after Docker selects the network. If Cleanup
// is non-nil, the core always calls it, including when Preparer returns an error.
type Preparer func(context.Context, PrepareContext) (PreparedScenario, error)

// PrepareContext carries only protocol-neutral selected state.
type PrepareContext struct {
	Source  ScenarioSource `json:"source"`
	Network Network        `json:"network"`
}

// PreparedScenario contains peers in dependency order and optional host cleanup.
type PreparedScenario struct {
	Peers   []PeerConfig `json:"peers"`
	Cleanup func() error `json:"-"`
}

// ScenarioPlan is protocol-neutral. Containers lists every resource that
// pre-clean must remove before a dynamic Preparer runs.
type ScenarioPlan struct {
	Source     ScenarioSource `json:"source"`
	Network    NetworkSpec    `json:"network"`
	Peers      []PeerConfig   `json:"peers,omitempty"`
	Containers []string       `json:"containers,omitempty"`
	Prepare    Preparer       `json:"-"`
}

// PreflightCheck proves host capabilities before image builds consume time.
type PreflightCheck func(context.Context, *Docker) error

// Preflights answers one check that runs each check in order and stops at the
// first failure.
//
// The order is what a lab declares, and it is a cost decision: a lab that
// probes the kernel and also stages a binary puts the probe first, so a machine
// that cannot run the lab at all refuses before a cross-compile spends time on
// it. A nil check is skipped rather than called.
func Preflights(checks ...PreflightCheck) PreflightCheck {
	return func(ctx context.Context, docker *Docker) error {
		for _, check := range checks {
			if check == nil {
				continue
			}
			if err := check(ctx, docker); err != nil {
				return err
			}
		}
		return nil
	}
}

// Suite builds shared images, then runs each scenario and keeps every verdict.
type Suite struct {
	Docker    *Docker
	Preflight PreflightCheck
	// StagedZe is the absolute path of the linux ze Preflight stages for the
	// daemon's architecture (StagedZePath). Run probes the daemon's kernel with
	// it before any image build (DockerKernel), and refuses when it is empty:
	// every suite runs Ze on that kernel, so the check fails and never skips.
	StagedZe  string
	Images    []ImageBuild
	Scenarios []ScenarioPlan
	NoBuild   bool
	// Checkout is the repository root the suite runs from. Each CheckContext
	// carries it, so a checker that retains proof resolves the session scratch
	// against the checkout rather than against the process working directory,
	// which `go test` sets to the package directory inside the source tree.
	Checkout string
	// SuffixVariable is the environment variable that sets this lab's
	// container and network name suffix (Environment.SuffixVariable). A
	// pre-clean refusal names it as the fix; empty when the lab has none.
	SuffixVariable string
	// Progress receives one line as each phase starts and as each scenario
	// ends, WHILE the suite runs. The report prints only after the last
	// scenario, so without these lines a suite that hangs is silent until its
	// caller's deadline kills it: the BGP lab in evidence-nightly ran 89
	// minutes with no output before `command ssh exceeded its deadline`, and
	// the log could not say which scenario held it. Nil writes to stderr,
	// which is the job log in CI and the ssh stream under docker-lab.
	Progress io.Writer
}

// progress writes one progress line. A failed write is not a suite failure:
// the line is a narration of the run, and the report still carries every
// verdict.
func (s Suite) progress(line *textbuf.Buffer) {
	sink := s.Progress
	if sink == nil {
		sink = os.Stderr
	}
	line.Byte('\n')
	//nolint:errcheck // progress narration; the report carries every verdict
	sink.Write(line.Bytes())
}

// SuiteReport is the structured result for one gate invocation.
type SuiteReport struct {
	Images        []ImageResult    `json:"images,omitempty"`
	Scenarios     []ScenarioResult `json:"scenarios,omitempty"`
	Passed        int              `json:"passed"`
	Failed        int              `json:"failed"`
	FailedNames   []string         `json:"failed-names,omitempty"`
	SetupError    string           `json:"setup-error,omitempty"`
	CleanupErrors []string         `json:"cleanup-errors,omitempty"`
	Code          int              `json:"code"`
}

// Text renders the report for a terminal, so a failing `./le test integration interop`
// prints the scenario that failed and the assertion that failed it under the
// "Failed: interop" summary rather than the summary alone. The sweep renders a
// failing native answer only when it can render itself
// (plan/journal/failing-gate-prints-no-cause.md); a passing report is not printed.
func (r SuiteReport) Text() string {
	var out textbuf.Buffer
	if r.SetupError != "" {
		out.Str("interop: setup: ").Str(r.SetupError).Byte('\n')
	}
	for index := range r.Scenarios {
		scenario := &r.Scenarios[index]
		if scenario.Passed {
			continue
		}
		out.Str("interop: ").Str(scenario.Name).Str(": FAIL: ").Str(scenario.Error).Byte('\n')
		for _, cleanup := range scenario.CleanupErrors {
			out.Str("interop: ").Str(scenario.Name).Str(": cleanup: ").Str(cleanup).Byte('\n')
		}
	}
	for _, cleanup := range r.CleanupErrors {
		out.Str("interop: image cleanup: ").Str(cleanup).Byte('\n')
	}
	out.Str("interop: ").Int(int64(r.Passed)).Str(" passed, ").Int(int64(r.Failed)).Str(" failed")
	if len(r.FailedNames) != 0 {
		out.Str(": ").Join(r.FailedNames, " ")
	}
	return out.Byte('\n').String()
}

// ScenarioResult records the checker verdict separately from cleanup diagnostics.
type ScenarioResult struct {
	Name          string   `json:"name"`
	Passed        bool     `json:"passed"`
	Error         string   `json:"error,omitempty"`
	CleanupErrors []string `json:"cleanup-errors,omitempty"`
}

// CheckerLab is the protocol-neutral surface a leaf checker consumes. A leaf
// test can implement this interface without starting Docker.
type CheckerLab interface {
	Exec(context.Context, string, []string, []EnvironmentVariable) (CommandResult, error)
	ExecDetached(context.Context, string, []string, []EnvironmentVariable) error
	Query(context.Context, string, []string, []EnvironmentVariable) (string, error)
	Logs(context.Context, string, int) (LogResult, error)
	PeerPID(context.Context, string) (int, error)
	Signal(context.Context, string, string) error
	Pause(context.Context, string) error
	Unpause(context.Context, string) error
	Start(context.Context, string) error
	Stop(context.Context, string, int) error
}

// CheckContext is the typed state handed to each protocol leaf checker.
type CheckContext struct {
	Source  ScenarioSource
	Network Network
	Lab     CheckerLab
	// Checkout is the root a checker that retains proof writes under, through
	// that root's session scratch. Empty means the caller declared none, and
	// such a checker MUST refuse rather than fall back to the working directory.
	Checkout string
}

// Lab exposes peer commands, fail-closed queries, and bounded log reads.
type Lab struct {
	docker *Docker
	peers  map[string]PeerConfig
}

var _ CheckerLab = (*Lab)(nil)

// ErrUnknownPeer is wrapped by every Lab call that names a peer the scenario did not
// start, so a caller that walks a suite's whole peer set can skip the absent ones.
var ErrUnknownPeer = errors.New("unknown peer")

// Run probes Docker once, removes what earlier runs left behind, checks the
// daemon's kernel, prepares images once, and runs every scenario.
func (s Suite) Run(ctx context.Context) (report SuiteReport) {
	if s.Docker == nil {
		report.SetupError = "interop suite has no Docker client"
		report.Code = 1
		return report
	}
	if len(s.Scenarios) == 0 {
		report.SetupError = "interop suite selected no scenarios"
		report.Code = 1
		return report
	}
	if err := s.Docker.Probe(ctx); err != nil {
		report.SetupError = err.Error()
		report.Code = 1
		return report
	}
	// The orphan cleanup runs before Preflight and before anything is
	// created, so a killed run's leftovers go before this run's names are
	// claimed. Its per-resource outcomes are progress lines: a dead run's
	// leftover it could not remove is not this run's failure, and preClean
	// still refuses a name it collides with.
	orphans, err := s.Docker.CleanupOrphans(ctx, false)
	if err != nil {
		report.SetupError = err.Error()
		report.Code = 1
		return report
	}
	var line textbuf.Buffer
	for _, text := range orphans.Lines() {
		s.progress(line.Reset().Str("interop: ").Str(text))
	}
	if s.Preflight != nil {
		if err := s.Preflight(ctx, s.Docker); err != nil {
			report.SetupError = err.Error()
			report.Code = 1
			return report
		}
	}
	// The kernel check follows Preflight because it runs the ze Preflight
	// stages, and precedes the first image build so a host that cannot run
	// the lab costs no build.
	if err := DockerKernel(s.StagedZe)(ctx, s.Docker); err != nil {
		report.SetupError = err.Error()
		report.Code = 1
		return report
	}

	line.Reset()
	s.progress(line.Str("interop: preparing ").Int(int64(len(s.Images))).Str(" image(s)"))
	preparing := time.Now()
	images, references, err := s.prepareImages(ctx)
	report.Images = images
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dockerCommandTimeout)
		defer cancel()
		for _, image := range images {
			if err := s.Docker.ReleaseImage(cleanupCtx, image); err != nil {
				report.CleanupErrors = append(report.CleanupErrors, err.Error())
				report.Code = 1
			}
		}
	}()
	if err != nil {
		report.SetupError = err.Error()
		report.Code = 1
		return report
	}
	line.Reset()
	s.progress(line.Str("interop: images ready (").Str(time.Since(preparing).Round(time.Second).String()).Byte(')'))
	for index, plan := range s.Scenarios {
		line.Reset()
		s.progress(scenarioLine(&line, index, len(s.Scenarios), plan.Source.Name).Str(": running"))
		started := time.Now()
		result := s.runScenario(ctx, plan, references)
		report.Scenarios = append(report.Scenarios, result)
		line.Reset()
		scenarioLine(&line, index, len(s.Scenarios), result.Name)
		elapsed := time.Since(started).Round(time.Second).String()
		if result.Passed {
			s.progress(line.Str(": pass (").Str(elapsed).Byte(')'))
			report.Passed++
			continue
		}
		s.progress(line.Str(": FAIL (").Str(elapsed).Str("): ").Str(result.Error))
		report.Failed++
		report.FailedNames = append(report.FailedNames, result.Name)
		if ctx.Err() != nil {
			break
		}
	}
	if report.Failed > 0 {
		report.Code = 1
	}
	return report
}

func (s Suite) prepareImages(ctx context.Context) ([]ImageResult, map[string]string, error) {
	results := make([]ImageResult, 0, len(s.Images))
	references := make(map[string]string, len(s.Images))
	for _, build := range s.Images {
		if build.Name == "" {
			return results, references, errors.New("suite image has no logical name")
		}
		if build.Tag == "" {
			return results, references, errors.New("suite image has no tag")
		}
		if s.NoBuild {
			result := ImageResult{Name: build.Name, Tag: build.Tag, Reference: build.Tag}
			results = append(results, result)
			references[build.Name] = build.Tag
			continue
		}
		if build.Pull {
			err := s.Docker.Pull(ctx, build.Tag, 0)
			if err == nil {
				result := ImageResult{Name: build.Name, Tag: build.Tag, Reference: build.Tag}
				results = append(results, result)
				references[build.Name] = build.Tag
				continue
			}
			result := ImageResult{Name: build.Name, Tag: build.Tag, Reference: build.Tag, Error: err.Error()}
			results = append(results, result)
			if build.Required {
				return results, references, err
			}
			references[build.Name] = build.Tag
			continue
		}
		result, err := s.Docker.Build(ctx, build)
		if err == nil {
			results = append(results, result)
			references[build.Name] = result.Reference
			continue
		}
		result.Error = err.Error()
		result.Reference = build.Tag
		results = append(results, result)
		if build.Required {
			return results, references, err
		}
		references[build.Name] = build.Tag
	}
	return results, references, nil
}

// scenarioLine starts the progress line of scenario index of total.
func scenarioLine(line *textbuf.Buffer, index, total int, name string) *textbuf.Buffer {
	return line.Str("interop: [").Int(int64(index + 1)).Byte('/').Int(int64(total)).Str("] ").Str(name)
}

func (s Suite) runScenario(ctx context.Context, plan ScenarioPlan, references map[string]string) (result ScenarioResult) {
	result.Name = plan.Source.Name
	if plan.Source.Name == "" {
		result.Error = "scenario has no name"
		return result
	}
	if plan.Source.Checker == nil {
		result.Error = "scenario has no checker"
		return result
	}

	peers := resolvedPeers(plan.Peers, references)
	cleanupPeers := declaredCleanupPeers(plan, peers)
	if len(cleanupPeers) == 0 {
		result.Error = "scenario declares no containers to clean"
		return result
	}
	if plan.Prepare == nil {
		if err := validatePeers(peers); err != nil {
			result.Error = err.Error()
			return result
		}
	}
	lab := newLab(s.Docker, peers)
	cleanupCtx := context.WithoutCancel(ctx)
	// The teardown removes only what this run created. A name preClean
	// refused, or a create that found the name taken, belongs to another run,
	// so a resource is recorded only once this run's create made it.
	var created createdResources
	var hostCleanup func() error
	defer func() {
		result.CleanupErrors = lab.cleanup(cleanupCtx, created)
		if hostCleanup == nil {
			return
		}
		if err := hostCleanup(); err != nil {
			result.CleanupErrors = append(result.CleanupErrors, err.Error())
		}
	}()
	if err := lab.preClean(ctx, plan.Network.Name, cleanupPeers, s.SuffixVariable); err != nil {
		result.Error = err.Error()
		return result
	}
	network, err := s.Docker.CreateNetwork(ctx, plan.Network)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	created.network = network.Name
	if plan.Prepare != nil {
		prepared, prepareErr := plan.Prepare(ctx, PrepareContext{Source: plan.Source, Network: network})
		hostCleanup = prepared.Cleanup
		peers = resolvedPeers(prepared.Peers, references)
		lab.setPeers(peers)
		if prepareErr != nil {
			result.Error = prepareErr.Error()
			return result
		}
	}
	if plan.Prepare != nil {
		if err := validatePeers(peers); err != nil {
			result.Error = err.Error()
			return result
		}
	}
	for index := range peers {
		peer := &peers[index]
		runErr := s.Docker.runContainer(ctx, network, *peer)
		// A failed `docker run -d` can leave the container it created, so the
		// name is this run's to remove unless the daemon refused it as taken.
		if !containerNameTaken(runErr) {
			created.containers = append(created.containers, peer.Container)
		}
		if runErr != nil {
			result.Error = runErr.Error()
			return result
		}
		if peer.Ready == nil {
			continue
		}
		if err := lab.waitPeer(ctx, *peer); err != nil {
			result.Error = err.Error()
			return result
		}
	}
	check := &CheckContext{Source: plan.Source, Network: network, Lab: lab, Checkout: s.Checkout}
	if err := plan.Source.Checker(ctx, check); err != nil {
		result.Error = err.Error()
		return result
	}
	result.Passed = true
	return result
}

func resolvedPeers(peers []PeerConfig, references map[string]string) []PeerConfig {
	resolved := make([]PeerConfig, len(peers))
	copy(resolved, peers)
	for index := range resolved {
		if reference, ok := references[resolved[index].Image]; ok {
			resolved[index].Image = reference
		}
	}
	return resolved
}

func declaredCleanupPeers(plan ScenarioPlan, peers []PeerConfig) []PeerConfig {
	declared := make([]PeerConfig, 0, len(plan.Containers)+len(peers))
	for _, container := range plan.Containers {
		declared = append(declared, PeerConfig{Name: container, Container: container})
	}
	return mergeCleanupPeers(declared, peers)
}

func mergeCleanupPeers(first, second []PeerConfig) []PeerConfig {
	merged := make([]PeerConfig, 0, len(first)+len(second))
	seen := make(map[string]struct{}, len(first)+len(second))
	for index := range first {
		peer := &first[index]
		if peer.Container == "" {
			continue
		}
		if _, exists := seen[peer.Container]; exists {
			continue
		}
		seen[peer.Container] = struct{}{}
		merged = append(merged, *peer)
	}
	for index := range second {
		peer := &second[index]
		if peer.Container == "" {
			continue
		}
		if _, exists := seen[peer.Container]; exists {
			continue
		}
		seen[peer.Container] = struct{}{}
		merged = append(merged, *peer)
	}
	return merged
}

func validatePeers(peers []PeerConfig) error {
	if len(peers) == 0 {
		return errors.New("scenario has no peers")
	}
	names := make(map[string]struct{}, len(peers))
	containers := make(map[string]struct{}, len(peers))
	for index := range peers {
		peer := &peers[index]
		if peer.Name == "" {
			return errors.New("scenario peer has no name")
		}
		if _, exists := names[peer.Name]; exists {
			var tb textbuf.Buffer
			return errors.New(tb.Str("scenario repeats peer name ").Str(peer.Name).String())
		}
		names[peer.Name] = struct{}{}
		if peer.Container == "" {
			var tb textbuf.Buffer
			return errors.New(tb.Str("peer ").Str(peer.Name).Str(" has no container name").String())
		}
		if _, exists := containers[peer.Container]; exists {
			var tb textbuf.Buffer
			return errors.New(tb.Str("scenario repeats container name ").Str(peer.Container).String())
		}
		containers[peer.Container] = struct{}{}
	}
	return nil
}

func newLab(docker *Docker, peers []PeerConfig) *Lab {
	byName := make(map[string]PeerConfig, len(peers))
	for index := range peers {
		peer := &peers[index]
		byName[peer.Name] = *peer
	}
	return &Lab{docker: docker, peers: byName}
}

func (l *Lab) setPeers(peers []PeerConfig) {
	l.peers = make(map[string]PeerConfig, len(peers))
	for index := range peers {
		peer := &peers[index]
		l.peers[peer.Name] = *peer
	}
}

// preClean clears this scenario's container and network names before it
// creates them. A same-name resource goes only when its owner labels prove the
// owner gone; a live, foreign, unjudgeable or unlabeled one fails the scenario
// with the owner and the fix (Docker.claimName), which names suffixVariable
// when the lab has one.
func (l *Lab) preClean(ctx context.Context, network string, peers []PeerConfig, suffixVariable string) error {
	for index := range peers {
		if err := l.docker.claimName(ctx, resourceContainer, peers[index].Container, suffixVariable); err != nil {
			return err
		}
	}
	if network == "" {
		return nil
	}
	return l.docker.claimName(ctx, resourceNetwork, network, suffixVariable)
}

// createdResources is what one scenario's own creates made, and so all its
// teardown may remove. Not safe for concurrent use: runScenario owns it.
type createdResources struct {
	containers []string
	network    string
}

// cleanup removes the containers, then the network, this scenario created.
// A name it did not create is never removed here, whoever holds it: preClean
// refused it, or another run took it first (R-1).
func (l *Lab) cleanup(ctx context.Context, created createdResources) []string {
	problems := make([]string, 0)
	for _, container := range created.containers {
		if err := l.docker.RemoveContainer(ctx, container); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if created.network == "" {
		return problems
	}
	if err := l.docker.RemoveNetwork(ctx, created.network); err != nil {
		problems = append(problems, err.Error())
	}
	return problems
}

// waitPeer MUST satisfy the probe's exit and stdout requirements before the suite
// starts the next peer. It passes the declared environment to every attempt.
func (l *Lab) waitPeer(ctx context.Context, peer PeerConfig) error {
	ready := peer.Ready
	if ready.Timeout <= 0 {
		return errors.New("peer readiness timeout must be positive")
	}
	if ready.Interval <= 0 {
		return errors.New("peer readiness interval must be positive")
	}
	_, _, err := Wait(ctx, WaitOptions{
		Timeout:     ready.Timeout,
		Interval:    ready.Interval,
		Description: peer.Name,
	}, func(probeCtx context.Context) (CommandResult, error) {
		return l.docker.Exec(probeCtx, peer.Container, ready.Command, ready.Environment)
	}, func(answer CommandResult) bool {
		for _, required := range ready.Contains {
			if !strings.Contains(answer.Stdout, required) {
				return false
			}
		}
		return true
	})
	return err
}

// Exec runs one command in the named peer and returns separate output streams.
func (l *Lab) Exec(ctx context.Context, peer string, argv []string, environ []EnvironmentVariable) (CommandResult, error) {
	config, err := l.peer(peer)
	if err != nil {
		return CommandResult{}, err
	}
	if len(argv) == 0 {
		return CommandResult{}, errors.New("peer command is empty")
	}
	return l.docker.Exec(ctx, config.Container, argv, environ)
}

// ExecDetached starts one command in the named peer and returns after Docker accepts it.
func (l *Lab) ExecDetached(ctx context.Context, peer string, argv []string, environ []EnvironmentVariable) error {
	config, err := l.peer(peer)
	if err != nil {
		return err
	}
	if len(argv) == 0 {
		return errors.New("detached peer command is empty")
	}
	return l.docker.ExecDetached(ctx, config.Container, argv, environ)
}

// Query runs a peer command and rejects empty stdout as an unmeasured answer.
func (l *Lab) Query(ctx context.Context, peer string, argv []string, environ []EnvironmentVariable) (string, error) {
	result, err := l.Exec(ctx, peer, argv, environ)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		return result.Stdout, nil
	}
	var tb textbuf.Buffer
	return "", errors.New(tb.Str("peer ").Str(peer).Str(" query returned no output").String())
}

// Logs reads a bounded peer log tail and distinguishes readable empty logs.
func (l *Lab) Logs(ctx context.Context, peer string, lines int) (LogResult, error) {
	config, err := l.peer(peer)
	if err != nil {
		return LogResult{}, err
	}
	return l.docker.Logs(ctx, config.Container, lines)
}

// PeerPID returns the positive host PID for the named peer's PID 1.
func (l *Lab) PeerPID(ctx context.Context, peer string) (int, error) {
	config, err := l.peer(peer)
	if err != nil {
		return 0, err
	}
	return l.docker.containerPID(ctx, config.Container)
}

// Signal sends a signal to the named peer's PID 1.
func (l *Lab) Signal(ctx context.Context, peer, signal string) error {
	config, err := l.peer(peer)
	if err != nil {
		return err
	}
	return l.docker.Signal(ctx, config.Container, signal)
}

// Pause freezes every process in the named peer's cgroup.
func (l *Lab) Pause(ctx context.Context, peer string) error {
	config, err := l.peer(peer)
	if err != nil {
		return err
	}
	return l.docker.pauseContainer(ctx, config.Container)
}

// Unpause resumes every process in the named paused peer's cgroup.
func (l *Lab) Unpause(ctx context.Context, peer string) error {
	config, err := l.peer(peer)
	if err != nil {
		return err
	}
	return l.docker.unpauseContainer(ctx, config.Container)
}

// Start restarts the named stopped peer without replacing its container.
func (l *Lab) Start(ctx context.Context, peer string) error {
	config, err := l.peer(peer)
	if err != nil {
		return err
	}
	return l.docker.startContainer(ctx, config.Container)
}

// Stop asks Docker to stop the named peer within the grace period.
func (l *Lab) Stop(ctx context.Context, peer string, timeoutSeconds int) error {
	config, err := l.peer(peer)
	if err != nil {
		return err
	}
	return l.docker.stopContainer(ctx, config.Container, timeoutSeconds)
}

func (l *Lab) peer(name string) (PeerConfig, error) {
	peer, ok := l.peers[name]
	if ok {
		return peer, nil
	}
	return PeerConfig{}, fmt.Errorf("%w %s", ErrUnknownPeer, name)
}
