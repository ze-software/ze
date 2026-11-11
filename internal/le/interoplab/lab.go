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
	// Links are the scenario's VLAN segments (Link). Their members name the
	// planned peers, the prepared ones when Prepare is set, and runScenario
	// refuses a link it cannot build before any container starts.
	Links   []Link   `json:"links,omitempty"`
	Prepare Preparer `json:"-"`
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
	// ImagesOnDemand prepares only images used by the final peer plans, after
	// dynamic preparation. Every peer MUST name a declared image in this mode.
	// A full suite leaves this false to retain its eager image preparation.
	ImagesOnDemand bool
	// Checkout is the repository root the suite runs from. Each CheckContext
	// carries it, so a checker that retains proof resolves the session scratch
	// against the checkout rather than against the process working directory,
	// which `go test` sets to the package directory inside the source tree.
	Checkout string
	// Recordings is the directory each lab's checker recordings go under
	// (record.go). Empty resolves it as Checkout's session scratch joined
	// with RecordingDirectory, which is what every lab run uses; a test sets
	// it to a directory of its own.
	Recordings string
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
	Images      []ImageResult    `json:"images,omitempty"`
	Scenarios   []ScenarioResult `json:"scenarios,omitempty"`
	Passed      int              `json:"passed"`
	Failed      int              `json:"failed"`
	FailedNames []string         `json:"failed-names,omitempty"`
	// NotRun names the scenarios an early stop skipped, in suite order. None
	// of them is counted as passed or failed.
	NotRun []string `json:"not-run,omitempty"`
	// Stopped says why the run stopped before its last scenario and how to
	// fix it; empty when every scenario ran.
	Stopped       string   `json:"stopped,omitempty"`
	SetupError    string   `json:"setup-error,omitempty"`
	CleanupErrors []string `json:"cleanup-errors,omitempty"`
	Code          int      `json:"code"`
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
		if !scenario.Passed {
			inPhase(out.Str("interop: ").Str(scenario.Name).Str(": FAIL"), scenario.Phase).
				Str(": ").Str(scenario.Error).Byte('\n')
		}
		for _, diagnostic := range scenario.Diagnostics {
			out.Str("interop: ").Str(scenario.Name).Str(": ").Str(diagnostic)
		}
		for _, cleanup := range scenario.CleanupErrors {
			out.Str("interop: ").Str(scenario.Name).Str(": cleanup: ").Str(cleanup).Byte('\n')
		}
	}
	for _, cleanup := range r.CleanupErrors {
		out.Str("interop: image cleanup: ").Str(cleanup).Byte('\n')
	}
	if r.Stopped != "" {
		out.Str("interop: stopped: ").Str(r.Stopped).Byte('\n')
	}
	out.Str("interop: ").Int(int64(r.Passed)).Str(" passed, ").Int(int64(r.Failed)).Str(" failed")
	if len(r.NotRun) != 0 {
		out.Str(", ").Int(int64(len(r.NotRun))).Str(" not run")
	}
	if len(r.FailedNames) != 0 {
		out.Str(": ").Join(r.FailedNames, " ")
	}
	return out.Byte('\n').String()
}

// ScenarioResult records the checker verdict separately from cleanup diagnostics.
type ScenarioResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
	// Phase is the step the failed run reached (runScenario); empty on a pass.
	Phase         ScenarioPhase `json:"phase,omitempty"`
	CleanupErrors []string      `json:"cleanup-errors,omitempty"`
	// Diagnostics holds one block per container a failed run started
	// (diagnosePeers), read before the teardown removes them, so the report
	// names the cause the verdict could not: an exited container, the
	// daemon's last log lines, a missing device.
	Diagnostics []string `json:"diagnostics,omitempty"`
}

// CheckerLab is the protocol-neutral surface a leaf checker consumes. A leaf
// test can implement this interface without starting Docker. Every capability
// a checker uses is a method here, never an optional interface a checker
// type-asserts its Lab for: the recorder Suite.Run puts in front of the Lab,
// and the replay lab, implement exactly this set, so an asserted method would
// vanish behind them.
type CheckerLab interface {
	Exec(context.Context, string, []string, []EnvironmentVariable) (CommandResult, error)
	ExecDetached(context.Context, string, []string, []EnvironmentVariable) error
	Query(context.Context, string, []string, []EnvironmentVariable) (string, error)
	Logs(context.Context, string, int) (LogResult, error)
	PeerPID(context.Context, string) (int, error)
	// PeerImage answers the image ID the peer's running container actually
	// consumed, never the tag the scenario declared for it.
	PeerImage(context.Context, string) (string, error)
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
	// Links are the scenario's declared links, and Memberships what each
	// member holds on them, by peer name, so a checker reads a member's
	// device and address from the declaration rather than from a literal.
	Links       []Link
	Memberships map[string][]LinkMembership
}

// Member answers what peer holds on link, and refuses a pair the scenario
// did not declare, so a checker never probes an address nobody assigned.
func (c *CheckContext) Member(peer, link string) (LinkMembership, error) {
	for _, membership := range c.Memberships[peer] {
		if membership.Link == link {
			return membership, nil
		}
	}
	var reason textbuf.Buffer
	return LinkMembership{}, errors.New(reason.Str(c.Source.Name).Str(" declares no link ").Str(link).
		Str(" with member ").Str(peer).Str("; name a link and member the scenario's Links declare").String())
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

// Run probes Docker once, reports what earlier runs left behind, checks the
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
	// An exited owner does not make its resources this run's. Inspect its
	// leftovers without removing them. preClean reclaims only exact current
	// owner labels and refuses every other name collision. Print the summary
	// even when the inspection met nothing, so the operator knows it ran.
	var line textbuf.Buffer
	orphans, err := s.Docker.CleanupOrphans(ctx, true)
	if err != nil {
		report.SetupError = line.Str("the pre-run inspection could not list the labeled lab resources (").Err(err).
			Str("), so their ownership could not be reported. Fix: run the failing docker command by hand to").
			Str(" see why the daemon refuses it. Correct that refusal, then retry the inspection:\n`").
			Str(cleanupCommand).Str(" dry`\nRun this suite again").String()
		report.Code = 1
		return report
	}
	for _, text := range orphans.Lines() {
		s.progress(line.Reset().Str("interop: ").Str(text))
	}
	s.progress(line.Reset().Str("interop: ").Str(orphans.Summary()))
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

	var references map[string]string
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dockerCommandTimeout)
		defer cancel()
		for _, image := range report.Images {
			if err := s.Docker.ReleaseImage(cleanupCtx, image); err != nil {
				report.CleanupErrors = append(report.CleanupErrors, err.Error())
				report.Code = 1
			}
		}
	}()
	if s.ImagesOnDemand {
		references = make(map[string]string, len(s.Images))
	} else {
		line.Reset()
		s.progress(line.Str("interop: preparing ").Int(int64(len(s.Images))).Str(" image(s)"))
		preparing := time.Now()
		report.Images, references, err = s.prepareImages(ctx, s.Images)
		if err != nil {
			report.SetupError = err.Error()
			report.Code = 1
			return report
		}
		line.Reset()
		s.progress(line.Str("interop: images ready (").Str(time.Since(preparing).Round(time.Second).String()).Byte(')'))
	}
	starts := imageStarts{start: make(map[string]ImageStart), probed: make(map[string]bool)}
	// A run that cannot record still runs: recording serves a later replay
	// test, never this run's verdict. It says so once, here, and a failed
	// scenario then names no recording.
	recordings, err := s.recordingPlan()
	if err != nil {
		s.progress(line.Reset().Str("interop: checker recordings are off: ").Err(err))
	}
	if err == nil && recordings.keepAllReason != nil {
		s.progress(line.Reset().Str("interop: checker recordings: keeping every recording because the lab could not list its scenarios: ").
			Err(recordings.keepAllReason))
	}
	// beforeChecker holds the names of the scenarios that failed in a row
	// before their checker ran; a pass or a checker failure empties it.
	var beforeChecker []string
	for index := range s.Scenarios {
		plan := &s.Scenarios[index]
		line.Reset()
		s.progress(scenarioLine(&line, index, len(s.Scenarios), plan.Source.Name).Str(": running"))
		started := time.Now()
		result := s.runScenario(ctx, *plan, references, &report.Images, starts, recordings)
		report.Scenarios = append(report.Scenarios, result)
		for _, cleanup := range result.CleanupErrors {
			report.Code = 1
			s.progress(scenarioLine(line.Reset(), index, len(s.Scenarios), result.Name).
				Str(": cleanup: ").Str(cleanup))
		}
		line.Reset()
		scenarioLine(&line, index, len(s.Scenarios), result.Name)
		elapsed := time.Since(started).Round(time.Second).String()
		if result.Passed {
			s.progress(line.Str(": pass (").Str(elapsed).Byte(')'))
			report.Passed++
			beforeChecker = beforeChecker[:0]
			continue
		}
		// The phase sits after the elapsed time, so the `: FAIL (` marker and
		// the checker's `scenario <name> assertion N:` text that the RFC
		// recorder reads (internal/le/rfc) are both unchanged.
		s.progress(inPhase(line.Str(": FAIL (").Str(elapsed).Byte(')'), result.Phase).
			Str(": ").Str(result.Error))
		report.Failed++
		report.FailedNames = append(report.FailedNames, result.Name)
		if ctx.Err() != nil {
			break
		}
		if !result.Phase.beforeChecker() {
			beforeChecker = beforeChecker[:0]
			continue
		}
		beforeChecker = append(beforeChecker, result.Name)
		remaining := s.Scenarios[index+1:]
		if len(beforeChecker) < setupFailuresBeforeStop {
			continue
		}
		if len(remaining) == 0 {
			continue
		}
		for skipped := range remaining {
			report.NotRun = append(report.NotRun, remaining[skipped].Source.Name)
		}
		report.Stopped = s.stopReason(beforeChecker, report.NotRun)
		s.progress(line.Reset().Str("interop: stopped: ").Str(report.Stopped))
		break
	}
	if report.Failed > 0 {
		report.Code = 1
	}
	return report
}

func (s Suite) prepareImages(ctx context.Context, builds []ImageBuild) ([]ImageResult, map[string]string, error) {
	results := make([]ImageResult, 0, len(builds))
	references := make(map[string]string, len(builds))
	for _, build := range builds {
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
			var line textbuf.Buffer
			s.progress(line.Str("interop: image ").Str(result.Name).Str(": reference ").Str(result.Reference))
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

// imageStarts caches, by resolved image reference and for one Suite.Run, the
// start each wrapped peer's image runs and which images passed the link
// probe. The two are kept apart because an image first wrapped for a
// Prestart alone is inspected but never probed, and a later scenario that
// makes it a link member still owes the probe and its named refusal. Not
// safe for concurrent use: Run runs its scenarios one at a time.
type imageStarts struct {
	start  map[string]ImageStart
	probed map[string]bool
}

// runScenario runs one scenario. starts caches what wrapStarts reads from
// each image, so each image is inspected and probed at most once for the run.
// recordings, when not nil, records every call the checker makes to the lab;
// nil is the run Suite.Run announced it cannot record.
func (s Suite) runScenario(ctx context.Context, plan ScenarioPlan, references map[string]string, images *[]ImageResult, starts imageStarts, recordings *recordingPlan) (result ScenarioResult) {
	// phase is the step the run has reached. Each step sets it before it
	// starts, and this deferred tag, registered first so it runs last, copies
	// it onto a failed result: every return below is labeled by where it
	// happened, never by matching its error text.
	phase := PhaseSetup
	defer func() {
		if result.Passed {
			return
		}
		result.Phase = phase
	}()
	result.Name = plan.Source.Name
	if plan.Source.Name == "" {
		result.Error = "scenario has no name"
		return result
	}
	if plan.Source.Checker == nil {
		result.Error = "scenario has no checker"
		return result
	}

	peers := plan.Peers
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
		if err := ValidateLinks(plan.Source.Name, plan.Links, peers); err != nil {
			result.Error = err.Error()
			return result
		}
	}
	lab := &Lab{docker: s.Docker}
	cleanupCtx := context.WithoutCancel(ctx)
	// The teardown removes only what this run created. A name preClean
	// refused, or a create that found the name taken, belongs to another run,
	// so a resource is recorded only once this run's create made it.
	var created createdResources
	var hostCleanup func() error
	defer func() {
		if result.Error != "" {
			result.Diagnostics = append(result.Diagnostics, diagnosePeers(cleanupCtx, s.Docker, peers, created.containers)...)
		}
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
		peers = prepared.Peers
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
		if err := ValidateLinks(plan.Source.Name, plan.Links, peers); err != nil {
			result.Error = err.Error()
			return result
		}
	}
	if s.ImagesOnDemand {
		if err := s.preparePeerImages(ctx, peers, references, images); err != nil {
			result.Error = err.Error()
			return result
		}
	}
	declared := peers
	peers = resolvedPeers(peers, references)
	memberships, err := s.wrapStarts(ctx, plan.Links, declared, peers, starts)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	lab.setPeers(peers)
	for index := range peers {
		peer := &peers[index]
		phase = PhaseSetup
		runErr := s.Docker.runContainer(ctx, network, *peer)
		// A failed `docker run -d` can leave the container it created, so the
		// name is this run's to remove once docker ran, unless the daemon
		// refused it as taken; a failure before docker ran created nothing.
		if ContainerMayExist(runErr) {
			created.containers = append(created.containers, peer.Container)
		}
		if runErr != nil {
			result.Error = runErr.Error()
			return result
		}
		if peer.Ready == nil {
			continue
		}
		phase = PhaseReadiness
		if err := lab.waitPeer(ctx, *peer); err != nil {
			result.Error = err.Error()
			return result
		}
	}
	phase = PhaseChecker
	check := &CheckContext{Source: plan.Source, Network: network, Lab: lab, Checkout: s.Checkout,
		Links: plan.Links, Memberships: memberships}
	// A nil plan is the run Suite.Run announced it cannot record.
	if recordings == nil {
		if err := plan.Source.Checker(ctx, check); err != nil {
			result.Error = err.Error()
			return result
		}
		result.Passed = true
		return result
	}
	recording, err := recordings.recordChecker(ctx, plan.Source.Checker, check)
	if err != nil {
		result.Error = err.Error()
		if recording != "" {
			result.Diagnostics = append(result.Diagnostics, recording+"\n")
		}
		return result
	}
	if recording != "" {
		var line textbuf.Buffer
		s.progress(line.Str("interop: ").Str(result.Name).Str(": ").Str(recording))
	}
	result.Passed = true
	return result
}

// wrapStarts gives every peer that joins a link or carries a Prestart one
// start wrapper (wrapStart): its links first, then its Prestart, then the
// image's own start read from the resolved image. A link member's image is
// probed for the wrapper's shell and ip first, so an image that cannot build
// a link is refused, naming its Dockerfile, before any container starts.
// declared holds the peers before image resolution, whose Image is the
// suite's logical name, so a refusal names the declaration.
func (s Suite) wrapStarts(ctx context.Context, links []Link, declared, peers []PeerConfig, starts imageStarts) (map[string][]LinkMembership, error) {
	memberships := make(map[string][]LinkMembership, len(peers))
	for index := range peers {
		peer := &peers[index]
		joined := LinkMemberships(links, peer.Name, peer.Host)
		if len(joined) == 0 && peer.Prestart == "" {
			continue
		}
		memberships[peer.Name] = joined
		if len(joined) > 0 && !starts.probed[peer.Image] {
			if err := s.Docker.probeLinkTools(ctx, peer.Image); err != nil {
				return memberships, linkToolRefusal(peer.Image, peer.Name, s.imageDeclaration(declared[index].Image), err)
			}
			starts.probed[peer.Image] = true
		}
		start, known := starts.start[peer.Image]
		if !known {
			read, err := s.Docker.ImageStart(ctx, peer.Image)
			if err != nil {
				return memberships, err
			}
			start = read
			starts.start[peer.Image] = start
		}
		*peer = ComposeStart(*peer, links, start)
	}
	return memberships, nil
}

// imageDeclaration answers the suite's declaration of the image named name,
// or a declaration carrying only that name as its tag when the suite runs an
// image it does not declare (a pulled reference).
func (s Suite) imageDeclaration(name string) ImageBuild {
	for index := range s.Images {
		if s.Images[index].Name == name {
			return s.Images[index]
		}
	}
	return ImageBuild{Name: name, Tag: name}
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

// preClean reclaims same-name resources only when every owner label equals
// this process's own (ownerIdentity.created). Every other owner is refused,
// including an exited one. The refusal names suffixVariable when available.
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

// EmptyAnswerError reports a peer command that ran to its end, exited 0 and
// printed nothing on stdout. It is distinct from an exec that failed or was cut
// short: the peer answered, and its answer was empty. FRR's vtysh prints nothing
// for an empty route table, so a wait for a route that never arrives meets this
// error on every probe; a caller that can read empty as a measured state
// (waitContains in the BGP checkers) tests for it with errors.As.
type EmptyAnswerError struct {
	Peer string
}

func (e *EmptyAnswerError) Error() string {
	var tb textbuf.Buffer
	return tb.Str("peer ").Str(e.Peer).Str(" query returned no output").String()
}

// Query runs a peer command and rejects empty stdout as an unmeasured answer,
// with an *EmptyAnswerError so a caller can tell it from a failed exec.
func (l *Lab) Query(ctx context.Context, peer string, argv []string, environ []EnvironmentVariable) (string, error) {
	result, err := l.Exec(ctx, peer, argv, environ)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		return result.Stdout, nil
	}
	return "", &EmptyAnswerError{Peer: peer}
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
