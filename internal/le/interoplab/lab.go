// Design: docs/architecture/testing/interop.md -- reusable scenario lifecycle
// Related: discover.go -- scenario names and typed checker callbacks.
// Related: docker.go -- image, network, container, and peer operations.
// Related: wait.go -- bounded readiness and query waits.
package interoplab

import (
	"context"
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

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

// Suite builds shared images, then runs each scenario and keeps every verdict.
type Suite struct {
	Docker    *Docker
	Preflight PreflightCheck
	Images    []ImageBuild
	Scenarios []ScenarioPlan
	NoBuild   bool
}

// SuiteReport is the structured result for one gate invocation.
type SuiteReport struct {
	Images      []ImageResult    `json:"images,omitempty"`
	Scenarios   []ScenarioResult `json:"scenarios,omitempty"`
	Passed      int              `json:"passed"`
	Failed      int              `json:"failed"`
	FailedNames []string         `json:"failed-names,omitempty"`
	SetupError  string           `json:"setup-error,omitempty"`
	Code        int              `json:"code"`
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
}

// Lab exposes peer commands, fail-closed queries, and bounded log reads.
type Lab struct {
	docker *Docker
	peers  map[string]PeerConfig
}

var _ CheckerLab = (*Lab)(nil)

// Run probes Docker once, prepares images once, and runs every scenario.
func (s Suite) Run(ctx context.Context) SuiteReport {
	report := SuiteReport{}
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
	if s.Preflight != nil {
		if err := s.Preflight(ctx, s.Docker); err != nil {
			report.SetupError = err.Error()
			report.Code = 1
			return report
		}
	}

	images, references, err := s.prepareImages(ctx)
	report.Images = images
	if err != nil {
		report.SetupError = err.Error()
		report.Code = 1
		return report
	}
	for _, plan := range s.Scenarios {
		result := s.runScenario(ctx, plan, references)
		report.Scenarios = append(report.Scenarios, result)
		if result.Passed {
			report.Passed++
			continue
		}
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
			err := s.Docker.Pull(ctx, build.Tag)
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
	var hostCleanup func() error
	defer func() {
		result.CleanupErrors = lab.cleanup(cleanupCtx, plan.Network.Name, cleanupPeers)
		if hostCleanup == nil {
			return
		}
		if err := hostCleanup(); err != nil {
			result.CleanupErrors = append(result.CleanupErrors, err.Error())
		}
	}()
	if err := lab.preClean(ctx, plan.Network.Name, cleanupPeers); err != nil {
		result.Error = err.Error()
		return result
	}
	network, err := s.Docker.createNetwork(ctx, plan.Network)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if plan.Prepare != nil {
		prepared, prepareErr := plan.Prepare(ctx, PrepareContext{Source: plan.Source, Network: network})
		hostCleanup = prepared.Cleanup
		peers = resolvedPeers(prepared.Peers, references)
		cleanupPeers = mergeCleanupPeers(cleanupPeers, peers)
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
		if err := s.Docker.runContainer(ctx, network, *peer); err != nil {
			result.Error = err.Error()
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
	check := &CheckContext{Source: plan.Source, Network: network, Lab: lab}
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

func (l *Lab) preClean(ctx context.Context, network string, peers []PeerConfig) error {
	errorsFound := l.cleanup(ctx, network, peers)
	if len(errorsFound) == 0 {
		return nil
	}
	return errors.New(errorsFound[0])
}

func (l *Lab) cleanup(ctx context.Context, network string, peers []PeerConfig) []string {
	problems := make([]string, 0)
	for index := range peers {
		peer := &peers[index]
		if err := l.docker.RemoveContainer(ctx, peer.Container); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if network == "" {
		return problems
	}
	if err := l.docker.removeNetwork(ctx, network); err != nil {
		problems = append(problems, err.Error())
	}
	return problems
}

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
		return l.docker.Exec(probeCtx, peer.Container, ready.Command, nil)
	}, func(CommandResult) bool { return true })
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
	var tb textbuf.Buffer
	return PeerConfig{}, errors.New(tb.Str("unknown peer ").Str(name).String())
}
