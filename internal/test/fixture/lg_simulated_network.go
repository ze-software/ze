// Design: docs/architecture/chaos-web-dashboard.md -- a simulated network over real sessions, read back through the looking glass
// Related: plugin_fixture_09.go -- the registration
// Related: lg_covering_lookup.go -- lgCoveringRequest, the HTTP reader this file shares

package fixture

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	chaospeer "github.com/ze-software/ze/internal/chaos/peer"
	chaosscenario "github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/test/runner"
)

// lgSimulatedNetworkParams is the network the fixture runs: five routers of
// one AS, two of them border routers and two of them clients of the one route
// reflector, six external neighbours, eight prefixes from outside and one
// originated inside. It is the shape TestTopologyHasMultiHopTrace
// (internal/chaos/scenario/topology_test.go) proves for seeds 1 to 25, with the
// same order between the two address ranges.
//
// The addresses are documentation ranges (RFC 5737), so no host the test runs
// on holds one: ze ignores a route whose next hop is an address of its own.
// The speakers do not connect from these addresses. Each one reaches ze on a
// listen port of its own, and ze knows the session by the address its config
// gives the peer, which is the address the model writes as next hop.
var lgSimulatedNetworkParams = chaosscenario.TopologyParams{
	Seed:             7,
	LocalAS:          65000,
	Routers:          5,
	Borders:          2,
	ReflectorClients: 2,
	Neighbours:       6,
	Prefixes:         8,
	RouterFirst:      netip.MustParseAddr("198.51.100.1"),
	NeighbourFirst:   netip.MustParseAddr("203.0.113.1"),
}

// What the generator builds for every seed, written out here and never asked
// of the model: the routers and the neighbours each fill one range, the first
// prefix from outside has two exits, the second has one, and one prefix
// originates inside the AS (generateTopologySpec,
// internal/chaos/scenario/topology_generate.go).
var (
	lgSimulatedRouterRange    = netip.MustParsePrefix("198.51.100.0/24")
	lgSimulatedNeighbourRange = netip.MustParsePrefix("203.0.113.0/24")
	lgSimulatedPrefixTwoExits = netip.MustParsePrefix("198.18.0.0/24")
	lgSimulatedPrefixOneExit  = netip.MustParsePrefix("198.18.1.0/24")
	lgSimulatedPrefixInside   = netip.MustParsePrefix("198.19.0.0/24")
)

// lgSimulatedCollectorID is the collector's own BGP identifier, outside both
// ranges, so no speaker shares it.
var lgSimulatedCollectorID = netip.MustParseAddr("192.0.2.254")

const (
	// lgSimulatedPollDelay is the pause between two looks at a state the fixture
	// waits for.
	lgSimulatedPollDelay = 250 * time.Millisecond
	// lgSimulatedEventDepth is the depth of the simulators' event channel. One
	// goroutine empties it for the whole run, so a full channel only delays a
	// simulator, and RunSimulator then waits for room.
	lgSimulatedEventDepth = 1024
	// lgSimulatedStopWait is how long ze gets to stop after SIGTERM before it
	// is killed.
	lgSimulatedStopWait = 10 * time.Second

	lgSimulatedLoopback = "127.0.0.1"
)

// lgSimulatedCheck is one question asked of the looking glass about the
// running network, answered from the model.
type lgSimulatedCheck struct {
	name string
	ask  func(ctx context.Context, base string, topology *chaosscenario.Topology) error
}

// lgSimulatedNetworkChecks are the questions, in the order they are asked,
// once every speaker's End-of-RIB is on the wire. Another question about the
// same network is one more entry: a function that reads a page and compares it
// with what the model answers.
var lgSimulatedNetworkChecks = []lgSimulatedCheck{
	{name: "peers page", ask: lgSimulatedPeersPage},
	{name: "search", ask: lgSimulatedSearch},
	{name: "AS path graph", ask: lgSimulatedASPathGraph},
	{name: "next-hop graph", ask: lgSimulatedNextHopGraph},
	{name: "trace graph", ask: lgSimulatedTraceGraph},
}

// lgSimulatedNetworkPlan is one network the fixture runs and the questions it asks of
// it. The questions belong to the network: each list refuses a run whose
// network held none of the cases that list exists to prove.
type lgSimulatedNetworkPlan struct {
	topology *chaosscenario.Topology
	checks   []lgSimulatedCheck
}

// lgSimulatedNetworkFixture runs a generated network against a real ze and asks the
// looking glass what the model computed.
//
// Arguments: the first port of the test's own span, and the looking glass
// port. The speakers' listen ports are reserved past the span.
func lgSimulatedNetworkFixture(ctx context.Context, args []string) error {
	options, err := lgSimulatedOptions("plugin/lg-simulated-network", args)
	if err != nil {
		return err
	}
	return RunLGSimulatedNetwork(ctx, options, nil)
}

// lgSimulatedOptions reads the two arguments every simulated network fixture
// takes: the first port of the test's own span, and the looking glass port.
// fixture is the name the usage line shows.
func lgSimulatedOptions(fixture string, args []string) (LGSimulatedNetworkOptions, error) {
	if len(args) != 2 {
		return LGSimulatedNetworkOptions{}, errors.New("usage: " + fixture + " <first-test-port> <looking-glass-port>")
	}
	portFirst, err := strconv.Atoi(args[0])
	if err != nil {
		return LGSimulatedNetworkOptions{}, fmt.Errorf("first-test-port %q: %w", args[0], err)
	}
	lgPort, err := strconv.Atoi(args[1])
	if err != nil {
		return LGSimulatedNetworkOptions{}, fmt.Errorf("looking-glass-port %q: %w", args[1], err)
	}
	// The runner puts the ze under test first in PATH, and ze runs in the
	// test's own directory.
	return LGSimulatedNetworkOptions{Binary: "ze", PortFirst: portFirst, LGPort: lgPort}, nil
}

// LGSimulatedNetworkOptions says which ze one run of the simulated network
// starts and which ports it owns.
type LGSimulatedNetworkOptions struct {
	// Binary is the ze to start: a name looked up in PATH, or a path.
	Binary string
	// WorkDir is the directory ze runs in. Empty is the caller's own.
	WorkDir string
	// PortFirst is the first port of a span of runner.TestPortSpan ports the
	// caller owns. The speakers' listen ports are reserved past that span.
	PortFirst int
	// LGPort is the port the looking glass listens on, inside that span.
	LGPort int
}

// LGSimulatedNetwork is the running network RunLGSimulatedNetwork hands to its
// caller: every speaker has sent its End-of-RIB and every check has passed.
type LGSimulatedNetwork struct {
	// Base is the looking glass, as a URL with no path.
	Base string
	// Params generated Topology, so Params.Seed names the network.
	Params   chaosscenario.TopologyParams
	Topology *chaosscenario.Topology
}

// RunLGSimulatedNetwork runs the generated network against a real ze, asks the
// looking glass every question of lgSimulatedNetworkChecks, then hands the
// network, still running, to use. A nil use ends the run after the checks.
//
// Both sides come from the one model: the daemon's config, with one peer per
// speaker, and the speakers themselves, each a peer simulator announcing the
// routes the model computed for it over a real BGP session. It starts ze
// itself, because the config does not exist before the model is built.
//
// The fixture and the website's looking glass recorder
// (internal/le/site/lgreplay) both call it, so a recording is read from a
// network the checks have proven against the model. A session lost while use
// ran fails the run, because what use read is then meaningless.
func RunLGSimulatedNetwork(ctx context.Context, options LGSimulatedNetworkOptions, use func(context.Context, LGSimulatedNetwork) error) error {
	topology, err := chaosscenario.GenerateTopology(lgSimulatedNetworkParams)
	if err != nil {
		return err
	}
	plan := lgSimulatedNetworkPlan{topology: topology, checks: lgSimulatedNetworkChecks}
	if use == nil {
		return lgSimulatedRunNetworkPlan(ctx, options, plan, nil)
	}
	return lgSimulatedRunNetworkPlan(ctx, options, plan, func(ctx context.Context, base string) error {
		return use(ctx, LGSimulatedNetwork{Base: base, Params: lgSimulatedNetworkParams, Topology: topology})
	})
}

// lgSimulatedRunNetworkPlan runs the network of plan against a real ze, asks the
// looking glass every question of plan, then hands the looking glass's base
// URL, with the network still running, to use. A nil use ends the run after
// the checks.
//
// Both sides come from the one model, as RunLGSimulatedNetwork states. A
// hand-written network (lg_simulated_next_hop_unchanged.go) runs through here with its
// own questions, on the same daemon, sessions and waits as the generated one.
func lgSimulatedRunNetworkPlan(ctx context.Context, options LGSimulatedNetworkOptions, plan lgSimulatedNetworkPlan, use func(ctx context.Context, base string) error) error {
	topology := plan.topology
	profiles, err := chaosscenario.GenerateFromTopology(topology)
	if err != nil {
		return err
	}

	// One listen port per speaker, past the span the caller owns. The
	// reservation is released last, after ze and every simulator have stopped.
	ports, _, err := runner.ReservePorts(options.PortFirst+runner.TestPortSpan, len(profiles))
	if err != nil {
		return fmt.Errorf("reserve %d BGP listen ports: %w", len(profiles), err)
	}
	defer ports.Release()
	for index := range profiles {
		profiles[index].ZePort = ports.Start + index
	}
	// The reservation is advisory: it keeps other test runs away and cannot
	// see a process that is no test run. A listener already on a speaker's
	// port would take the speaker's connection in place of the ze started
	// below, and the run would end as a handshake timeout with nothing to say
	// why.
	if busyErr := lgSimulatedPortsIdle(ctx, ports.Start, len(profiles)); busyErr != nil {
		return busyErr
	}

	daemon, err := lgSimulatedStartDaemon(ctx, options, chaosscenario.GenerateConfig(chaosscenario.ConfigParams{
		LocalAS:     topology.LocalAS(),
		RouterID:    lgSimulatedCollectorID,
		LocalAddr:   lgSimulatedLoopback,
		Profiles:    profiles,
		Collector:   true,
		LGPort:      options.LGPort,
		LGPlaintext: true,
	}))
	if err != nil {
		return err
	}
	defer daemon.stop()

	speakers, err := lgSimulatedStartSpeakers(ctx, daemon, profiles)
	if err != nil {
		return daemon.failed(err)
	}
	defer speakers.stop()

	if waitErr := speakers.waitEndOfRIB(ctx, daemon); waitErr != nil {
		return daemon.failed(waitErr)
	}

	base := "http://" + net.JoinHostPort(lgSimulatedLoopback, strconv.Itoa(options.LGPort))
	for _, check := range plan.checks {
		if askErr := check.ask(ctx, base, topology); askErr != nil {
			return daemon.failed(fmt.Errorf("simulated network, %s: %w", check.name, askErr))
		}
		// A session lost while a page was read makes its answer meaningless.
		if _, failure := speakers.sessions.state(); failure != "" {
			return daemon.failed(fmt.Errorf("simulated network, after %s: %s", check.name, failure))
		}
	}
	if use == nil {
		return nil
	}
	if useErr := use(ctx, base); useErr != nil {
		return daemon.failed(useErr)
	}
	if _, failure := speakers.sessions.state(); failure != "" {
		return daemon.failed(fmt.Errorf("simulated network, while its caller read it: %s", failure))
	}
	return nil
}

// lgSimulatedDaemon is the ze the fixture started. Its output is kept and
// shown only when the fixture fails.
type lgSimulatedDaemon struct {
	command *exec.Cmd
	// config is what ze was started on, shown when the run fails.
	config string
	output syncBuffer02
	// exit receives the result of the one Wait. A reader that only looks puts
	// the value back, so every later look still finds it.
	exit chan error
}

// lgSimulatedStartDaemon starts `ze -` on config, as the binary and in the
// directory options names. The caller MUST call stop.
func lgSimulatedStartDaemon(ctx context.Context, options LGSimulatedNetworkOptions, config string) (*lgSimulatedDaemon, error) {
	daemon := &lgSimulatedDaemon{exit: make(chan error, 1), config: config}
	// The binary is the fixture's literal "ze", or the path the recorder's
	// caller named on its own command line.
	daemon.command = exec.CommandContext(ctx, options.Binary, "-") //nolint:gosec // a test and build-host tool starting the ze its caller named
	daemon.command.Dir = options.WorkDir
	daemon.command.Stdin = strings.NewReader(config)
	daemon.command.Stdout = &daemon.output
	daemon.command.Stderr = &daemon.output
	if err := daemon.command.Start(); err != nil {
		return nil, fmt.Errorf("start ze: %w", err)
	}
	go func() { daemon.exit <- daemon.command.Wait() }()
	return daemon, nil
}

// hasEnded reports whether ze has ended.
func (d *lgSimulatedDaemon) hasEnded() bool {
	_, isEnded := d.endResult()
	return isEnded
}

// endResult answers the result of ze's one Wait, and whether ze has ended.
func (d *lgSimulatedDaemon) endResult() (result error, isEnded bool) {
	select {
	case result = <-d.exit:
		d.exit <- result
		return result, true
	default:
		return nil, false
	}
}

// state says in one line whether ze is running or how it ended, for the
// message of a failed run.
func (d *lgSimulatedDaemon) state() string {
	result, isEnded := d.endResult()
	if !isEnded {
		return "ze is still running"
	}
	if result == nil {
		return "ze has ended with exit status 0"
	}
	return "ze has ended: " + result.Error()
}

// stop ends ze and reaps it. It MUST be called after lgSimulatedStartDaemon
// succeeded, and it is safe to call once only.
func (d *lgSimulatedDaemon) stop() {
	if d.hasEnded() {
		return
	}
	if err := d.command.Process.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "simulated network: signal ze: %v\n", err)
	}
	select {
	case <-d.exit:
	case <-time.After(lgSimulatedStopWait):
		if err := d.command.Process.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "simulated network: kill ze: %v\n", err)
		}
		<-d.exit
	}
}

// The files a failed run keeps, in a directory beside ze's working directory.
// The runner shows the head and the tail of what a fixture prints and elides
// the middle, so nothing long is printed: the reason is one line, and the
// evidence is in these files.
const (
	lgSimulatedReasonFile = "lg-simulated-failure.txt"
	lgSimulatedConfigFile = "lg-simulated-ze.conf"
	lgSimulatedOutputFile = "lg-simulated-ze.log"
	// lgSimulatedEvidencePrefix opens the name of the directory a failed run
	// keeps its files in; the name of ze's working directory follows it.
	lgSimulatedEvidencePrefix = "lg-simulated-evidence-"
	// lgSimulatedTailLines and lgSimulatedTailWidth bound what the reason
	// quotes of ze's output: its last lines, each cut to this many bytes.
	lgSimulatedTailLines = 3
	lgSimulatedTailWidth = 200
)

// failed answers the reason of a failed run as ONE line, and it is the only
// thing the failure path writes: the first line of err, whether ze is running
// or how it ended, the last lines of what ze wrote, and the directory holding
// the whole reason, the config ze was started on and everything ze wrote.
//
// The answer does not wrap err: a wrapped error prints its whole text, and the
// text of a page that differs from the model runs to hundreds of lines.
func (d *lgSimulatedDaemon) failed(err error) error {
	written := d.output.String()
	kept := lgSimulatedKeep(d.command.Dir, map[string]string{
		lgSimulatedReasonFile: err.Error() + "\n",
		lgSimulatedConfigFile: d.config,
		lgSimulatedOutputFile: written,
	})

	reason, _, _ := strings.Cut(err.Error(), "\n")
	return errors.New(reason + "; " + d.state() +
		"; ze's last lines: " + lgSimulatedTail(written) + "; " + kept)
}

// lgSimulatedKeep writes the files of a failed run and answers, in one
// sentence, where they are or why they could not be written.
//
// They go in a directory BESIDE workDir, named after it, never inside it: the
// runner removes a test's directory when the test ends (runOrchestrated,
// internal/test/runner/runner_exec.go), so the files a failure of 2026-11-18
// named were gone before anyone could read them.
func lgSimulatedKeep(workDir string, files map[string]string) string {
	work, err := filepath.Abs(workDir)
	if err != nil {
		return "the evidence could not be written, ze's working directory " + workDir + " has no absolute path: " + err.Error()
	}
	directory := filepath.Join(filepath.Dir(work), lgSimulatedEvidencePrefix+filepath.Base(work))
	if mkdirErr := os.MkdirAll(directory, 0o700); mkdirErr != nil {
		return "the evidence could not be written to " + directory + ": " + mkdirErr.Error()
	}
	for name, content := range files {
		if writeErr := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); writeErr != nil {
			return "the evidence could not be written to " + directory + ": " + writeErr.Error()
		}
	}
	return "its whole reason, ze's config and ze's output are kept in " + directory +
		" (" + lgSimulatedReasonFile + ", " + lgSimulatedConfigFile + ", " + lgSimulatedOutputFile + ")"
}

// lgSimulatedTail answers the last lines of what ze wrote, on one line and
// bounded, or that ze wrote nothing.
func lgSimulatedTail(written string) string {
	lines := strings.Split(strings.TrimRight(written, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return "(ze wrote nothing)"
	}
	if len(lines) > lgSimulatedTailLines {
		lines = lines[len(lines)-lgSimulatedTailLines:]
	}
	for index, line := range lines {
		if len(line) > lgSimulatedTailWidth {
			lines[index] = line[:lgSimulatedTailWidth] + "..."
		}
	}
	return strings.Join(lines, " | ")
}

// lgSimulatedPortsIdle waits until none of the count speaker ports from first
// has a listener, before ze is started, and refuses the run when one still has
// after the wait. The wait is on that state: a daemon of the invocation before
// this one may still be closing its listeners, and a probe that refused on the
// first answer would fail a run that only had to wait.
//
// The lease on these ports (runner.ReservePorts) is the one every test's ports
// are leased through, and it binds nothing, so a connection answered here is
// answered by a process, never by the lease.
func lgSimulatedPortsIdle(ctx context.Context, first, count int) error {
	dialer := net.Dialer{Timeout: time.Second}
	busy := ""
	Poll(ctx, WaitAttempts(30, lgSimulatedPollDelay, 120), lgSimulatedPollDelay, func() bool {
		busy = ""
		for port := first; port < first+count; port++ {
			target := net.JoinHostPort(lgSimulatedLoopback, strconv.Itoa(port))
			conn, err := dialer.DialContext(ctx, "tcp", target)
			if err != nil {
				continue
			}
			busy = target
			if closeErr := conn.Close(); closeErr != nil {
				busy = target + " (closing the probe: " + closeErr.Error() + ")"
			}
			return false
		}
		return true
	})
	if busy == "" {
		return nil
	}
	_, port, _ := strings.Cut(busy, ":")
	port, _, _ = strings.Cut(port, " ")
	return errors.New("simulated network: " + busy + " still has a listener before ze is started, so a speaker would reach " +
		"that process and not the ze under test; find it with: lsof -nP -iTCP:" + port + " -sTCP:LISTEN")
}

// lgSimulatedSessions is what the simulators reported, kept by the one
// goroutine that empties their event channel. Safe for concurrent use.
type lgSimulatedSessions struct {
	mu       sync.Mutex
	eorSent  map[int]bool
	failures []string
	// closing says the fixture is ending the sessions itself, so a
	// disconnection is no longer a failure.
	closing bool
}

func (s *lgSimulatedSessions) record(event chaospeer.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.Type == chaospeer.EventEORSent {
		s.eorSent[event.PeerIndex] = true
	}
	if event.Type == chaospeer.EventError {
		s.failures = append(s.failures, fmt.Sprintf("speaker %d: %v", event.PeerIndex, event.Err))
	}
	if event.Type == chaospeer.EventDisconnected {
		if !s.closing {
			s.failures = append(s.failures, fmt.Sprintf("speaker %d lost its session", event.PeerIndex))
		}
	}
}

// state answers how many speakers sent End-of-RIB, and the failures so far as
// one text, empty when there is none.
func (s *lgSimulatedSessions) state() (eorSent int, failure string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.eorSent), strings.Join(s.failures, "; ")
}

func (s *lgSimulatedSessions) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closing = true
}

// lgSimulatedSpeakers is the running network: one simulator goroutine per
// speaker for the life of its session, and one goroutine emptying their
// events. Start with lgSimulatedStartSpeakers, then stop MUST be called.
type lgSimulatedSpeakers struct {
	count    int
	sessions *lgSimulatedSessions
	cancel   context.CancelFunc
	running  sync.WaitGroup
	drained  chan struct{}
}

// lgSimulatedStartSpeakers connects one simulator per profile to the listen
// port ze opened for it. On an error every simulator already started is
// stopped before it returns.
func lgSimulatedStartSpeakers(ctx context.Context, daemon *lgSimulatedDaemon, profiles []chaosscenario.PeerProfile) (*lgSimulatedSpeakers, error) {
	sessionCtx, cancel := context.WithCancel(ctx)
	speakers := &lgSimulatedSpeakers{
		count:    len(profiles),
		sessions: &lgSimulatedSessions{eorSent: make(map[int]bool, len(profiles))},
		cancel:   cancel,
		drained:  make(chan struct{}),
	}
	events := make(chan chaospeer.Event, lgSimulatedEventDepth)
	go func() {
		defer close(speakers.drained)
		for event := range events {
			speakers.sessions.record(event)
		}
	}()

	var dialErr error
	for index := range profiles {
		profile := profiles[index]
		conn, err := lgSimulatedDial(sessionCtx, daemon, profile.ZePort)
		if err != nil {
			dialErr = fmt.Errorf("simulated network: speaker %d (%s): %w", profile.Index, profile.Address, err)
			break
		}
		// The simulator owns conn from here and closes it when its session ends.
		speakers.running.Add(1)
		go func() {
			defer speakers.running.Done()
			chaospeer.RunSimulator(sessionCtx, chaospeer.SimulatorConfig{
				Profile: chaospeer.SimProfile{
					Index:      profile.Index,
					ASN:        profile.ASN,
					RouterID:   profile.RouterID,
					IsIBGP:     profile.IsIBGP,
					HoldTime:   profile.HoldTime,
					RouteCount: profile.RouteCount,
					TotalPeers: len(profiles),
					Families:   profile.Families,
					RouteSet:   profile.RouteSet,
				},
				Conn:   conn,
				Events: events,
				Quiet:  true,
			})
		}()
	}
	go func() {
		speakers.running.Wait()
		close(events)
	}()

	if dialErr != nil {
		speakers.stop()
		return nil, dialErr
	}
	return speakers, nil
}

// stop ends every session and returns once every simulator has ended and its
// last event was read.
func (s *lgSimulatedSpeakers) stop() {
	s.sessions.close()
	s.cancel()
	<-s.drained
}

// waitEndOfRIB returns once every speaker has sent its routes and its
// End-of-RIB, and fails as soon as one speaker reports a failure.
func (s *lgSimulatedSpeakers) waitEndOfRIB(ctx context.Context, daemon *lgSimulatedDaemon) error {
	var sent int
	var failure string
	daemonEnded := false
	settled := Poll(ctx, WaitAttempts(30, lgSimulatedPollDelay, 120), lgSimulatedPollDelay, func() bool {
		// A ze that ended will never answer: stop at once, before any speaker
		// reaches its own handshake timeout.
		if daemon.hasEnded() {
			daemonEnded = true
			return true
		}
		sent, failure = s.sessions.state()
		if failure != "" {
			return true
		}
		return sent == s.count
	})
	if daemonEnded {
		return fmt.Errorf("simulated network: ze ended after %d of %d speakers sent End-of-RIB", sent, s.count)
	}
	if failure != "" {
		return fmt.Errorf("simulated network: %s", failure)
	}
	if !settled {
		return fmt.Errorf("simulated network: %d of %d speakers sent End-of-RIB", sent, s.count)
	}
	return nil
}

// lgSimulatedDial connects to the listen port ze opens for one peer, retrying
// while ze is not listening yet. It stops as soon as ze has ended.
func lgSimulatedDial(ctx context.Context, daemon *lgSimulatedDaemon, port int) (net.Conn, error) {
	target := net.JoinHostPort(lgSimulatedLoopback, strconv.Itoa(port))
	dialer := net.Dialer{Timeout: 2 * time.Second}
	var conn net.Conn
	var lastErr error
	daemonEnded := false
	Poll(ctx, WaitAttempts(30, lgSimulatedPollDelay, 120), lgSimulatedPollDelay, func() bool {
		if daemon.hasEnded() {
			daemonEnded = true
			return true
		}
		dialed, err := dialer.DialContext(ctx, "tcp", target)
		if err != nil {
			lastErr = err
			return false
		}
		conn = dialed
		return true
	})
	if daemonEnded {
		return nil, fmt.Errorf("ze ended before it listened on %s", target)
	}
	if conn == nil {
		return nil, fmt.Errorf("ze never listened on %s: %w", target, lastErr)
	}
	return conn, nil
}

// lgSimulatedPeersPage waits until the peers page lists every speaker of the
// model as an established session holding the model's route count (AC-19).
// The wait is on that state: ze reads a speaker's routes after the speaker
// sent them, so the first read may still show a lower count.
func lgSimulatedPeersPage(ctx context.Context, base string, topology *chaosscenario.Topology) error {
	speakers := topology.Speakers()
	problem := "the peers page was never read"
	if !Poll(ctx, WaitAttempts(30, lgSimulatedPollDelay, 120), lgSimulatedPollDelay, func() bool {
		page, err := lgCoveringRequest(ctx, http.MethodGet, base+"/lg/peers", "")
		if err != nil {
			problem = err.Error()
			return false
		}
		problem = lgSimulatedPeersProblem(page, speakers)
		return problem == ""
	}) {
		return errors.New(problem)
	}
	fmt.Fprintf(os.Stderr, "OK: every simulated speaker is established with the route count the model computed: %d speakers\n", len(speakers))
	return nil
}

// lgSimulatedPeersProblem answers how the peers page differs from the model's
// speakers, or "" when it lists exactly them, each established, with its AS
// and its route count.
func lgSimulatedPeersProblem(page string, speakers []chaosscenario.Speaker) string {
	table, err := lgSimulatedReadTable(page)
	if err != nil {
		return "peers page: " + err.Error()
	}
	rows := make(map[string][]string, len(table.rows))
	for _, row := range table.rows {
		address, cellErr := table.cell(row, "Peer")
		if cellErr != nil {
			return "peers page: " + cellErr.Error()
		}
		rows[address] = row
	}
	if len(table.rows) != len(speakers) {
		return fmt.Sprintf("peers page lists %d peers, and the model has %d speakers", len(table.rows), len(speakers))
	}
	for _, speaker := range speakers {
		row, listed := rows[speaker.Address.String()]
		if !listed {
			return fmt.Sprintf("peers page does not list the speaker %s", speaker.Address)
		}
		want := map[string]string{
			"State":     "established",
			"Remote AS": strconv.FormatUint(uint64(speaker.AS), 10),
			"Received":  strconv.Itoa(len(speaker.Announcements)),
		}
		for _, column := range []string{"State", "Remote AS", "Received"} {
			got, cellErr := table.cell(row, column)
			if cellErr != nil {
				return "peers page: " + cellErr.Error()
			}
			if got != want[column] {
				return fmt.Sprintf("peers page shows %s %q for the speaker %s, and the model computed %q",
					column, got, speaker.Address, want[column])
			}
		}
	}
	return ""
}

// lgSimulatedRoute is one row of a search result: the peer that sent the
// route, and the next hop and AS path it sent.
type lgSimulatedRoute struct {
	peer    string
	nextHop string
	asPath  string
}

// lgSimulatedSearch asks the search page for every prefix of the model and
// compares the rows with the model: every speaker announcing the prefix, with
// the next hop and the AS path the model computed for it, and nothing else. The
// external neighbours among the rows are compared with Topology.Announcers on
// their own (AC-20). Three prefixes are then checked against what the
// generator states it builds, with no value taken from the model.
func lgSimulatedSearch(ctx context.Context, base string, topology *chaosscenario.Topology) error {
	external := map[string]bool{}
	for _, speaker := range topology.Speakers() {
		if speaker.Session == chaosscenario.SessionKindExternal {
			external[speaker.Address.String()] = true
		}
	}

	listed := map[netip.Prefix][]lgSimulatedRoute{}
	for _, prefix := range topology.Prefixes() {
		got, err := lgSimulatedSearchRows(ctx, base, prefix)
		if err != nil {
			return err
		}
		listed[prefix] = got
		if want := lgSimulatedModelRoutes(topology, prefix); !slices.Equal(got, want) {
			return fmt.Errorf("search for %s listed %v, and the model computed %v", prefix, got, want)
		}

		announcers, err := topology.Announcers(prefix)
		if err != nil {
			return err
		}
		wantAnnouncers := make([]string, len(announcers))
		for index, announcer := range announcers {
			wantAnnouncers[index] = announcer.String()
		}
		slices.Sort(wantAnnouncers)
		gotAnnouncers := []string{}
		for _, route := range got {
			if external[route.peer] {
				gotAnnouncers = append(gotAnnouncers, route.peer)
			}
		}
		if !slices.Equal(gotAnnouncers, wantAnnouncers) {
			return fmt.Errorf("search for %s listed the external neighbours %v, and the model's announcers are %v",
				prefix, gotAnnouncers, wantAnnouncers)
		}
	}

	if err := lgSimulatedHandChecked(listed); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: every search listed the announcers and the paths the model computed: %d prefixes\n", len(listed))
	return nil
}

// lgSimulatedModelRoutes answers the rows the model expects for prefix, in the
// order lgSimulatedSearchRows answers them.
func lgSimulatedModelRoutes(topology *chaosscenario.Topology, prefix netip.Prefix) []lgSimulatedRoute {
	routes := []lgSimulatedRoute{}
	for _, speaker := range topology.Speakers() {
		for _, announcement := range speaker.Announcements {
			if announcement.Prefix != prefix {
				continue
			}
			path := make([]string, len(announcement.ASPath))
			for index, as := range announcement.ASPath {
				path[index] = strconv.FormatUint(uint64(as), 10)
			}
			routes = append(routes, lgSimulatedRoute{
				peer:    speaker.Address.String(),
				nextHop: announcement.NextHop.String(),
				asPath:  strings.Join(path, " "),
			})
		}
	}
	lgSimulatedSortRoutes(routes)
	return routes
}

// lgSimulatedSortRoutes orders rows by peer. A prefix has one row per peer.
func lgSimulatedSortRoutes(routes []lgSimulatedRoute) {
	slices.SortFunc(routes, func(a, b lgSimulatedRoute) int { return strings.Compare(a.peer, b.peer) })
}

// lgSimulatedSearchRows posts one exact prefix search and answers its rows,
// ordered by peer. A row for another prefix is an error: the search is exact.
func lgSimulatedSearchRows(ctx context.Context, base string, prefix netip.Prefix) ([]lgSimulatedRoute, error) {
	page, err := lgCoveringRequest(ctx, http.MethodPost, base+"/lg/search",
		url.Values{columnPrefix: {prefix.String()}}.Encode())
	if err != nil {
		return nil, err
	}
	if strings.Contains(page, `class="error-banner"`) {
		return nil, fmt.Errorf("search for %s answered an error: %.600q", prefix, page)
	}
	table, err := lgSimulatedReadTable(page)
	if err != nil {
		return nil, fmt.Errorf("search for %s: %w", prefix, err)
	}
	routes := make([]lgSimulatedRoute, 0, len(table.rows))
	for _, row := range table.rows {
		cells := map[string]string{}
		for _, column := range []string{"Prefix", "Peer", "Next Hop", "AS Path"} {
			value, cellErr := table.cell(row, column)
			if cellErr != nil {
				return nil, fmt.Errorf("search for %s: %w", prefix, cellErr)
			}
			cells[column] = value
		}
		if cells["Prefix"] != prefix.String() {
			return nil, fmt.Errorf("search for %s listed a route for %s", prefix, cells["Prefix"])
		}
		routes = append(routes, lgSimulatedRoute{peer: cells["Peer"], nextHop: cells["Next Hop"], asPath: cells["AS Path"]})
	}
	lgSimulatedSortRoutes(routes)
	return routes, nil
}

// lgSimulatedHandChecked compares three prefixes with what the generator
// states it builds for every seed, so the model and the looking glass cannot
// agree on a shared mistake. The first prefix from outside has two exits: two
// external neighbours announce it, each with its own address as next hop, and
// each of the five routers announces it with a router's loopback as next hop,
// never a neighbour's address, because a border router rewrites the next hop to
// itself. The second has one exit. The inside prefix has no external announcer
// and an empty AS path from every router.
func lgSimulatedHandChecked(listed map[netip.Prefix][]lgSimulatedRoute) error {
	twoExits, err := lgSimulatedSplitRoutesByPeerRange(lgSimulatedPrefixTwoExits, listed)
	if err != nil {
		return err
	}
	if len(twoExits.neighbours) != 2 {
		return fmt.Errorf("search for %s listed %d external neighbours, and the prefix has two exits: %v",
			lgSimulatedPrefixTwoExits, len(twoExits.neighbours), twoExits.neighbours)
	}
	for _, route := range twoExits.neighbours {
		if route.nextHop != route.peer {
			return fmt.Errorf("search for %s: the external neighbour %s announces the next hop %s, and it announces its own address",
				lgSimulatedPrefixTwoExits, route.peer, route.nextHop)
		}
	}
	if len(twoExits.routers) != lgSimulatedNetworkParams.Routers {
		return fmt.Errorf("search for %s listed %d routers, and each of the %d routers announces its best path: %v",
			lgSimulatedPrefixTwoExits, len(twoExits.routers), lgSimulatedNetworkParams.Routers, twoExits.routers)
	}
	for _, route := range twoExits.routers {
		nextHop, parseErr := netip.ParseAddr(route.nextHop)
		if parseErr != nil {
			return fmt.Errorf("search for %s: router %s: next hop %q: %w", lgSimulatedPrefixTwoExits, route.peer, route.nextHop, parseErr)
		}
		if !lgSimulatedRouterRange.Contains(nextHop) {
			return fmt.Errorf("search for %s: the router %s announces the next hop %s, which is no router's loopback",
				lgSimulatedPrefixTwoExits, route.peer, route.nextHop)
		}
	}

	oneExit, err := lgSimulatedSplitRoutesByPeerRange(lgSimulatedPrefixOneExit, listed)
	if err != nil {
		return err
	}
	if len(oneExit.neighbours) != 1 {
		return fmt.Errorf("search for %s listed %d external neighbours, and the prefix has one exit: %v",
			lgSimulatedPrefixOneExit, len(oneExit.neighbours), oneExit.neighbours)
	}

	inside, err := lgSimulatedSplitRoutesByPeerRange(lgSimulatedPrefixInside, listed)
	if err != nil {
		return err
	}
	if len(inside.neighbours) != 0 {
		return fmt.Errorf("search for %s listed the external neighbours %v, and the prefix originates inside the AS",
			lgSimulatedPrefixInside, inside.neighbours)
	}
	if len(inside.routers) != lgSimulatedNetworkParams.Routers {
		return fmt.Errorf("search for %s listed %d routers, and each of the %d routers announces it: %v",
			lgSimulatedPrefixInside, len(inside.routers), lgSimulatedNetworkParams.Routers, inside.routers)
	}
	for _, route := range inside.routers {
		if route.asPath != "" {
			return fmt.Errorf("search for %s: the router %s announces the AS path %q, and a prefix originated inside the AS has none",
				lgSimulatedPrefixInside, route.peer, route.asPath)
		}
	}
	return nil
}

// lgSimulatedRoutesByPeerRange is the rows of one prefix, split by the range of the peer.
type lgSimulatedRoutesByPeerRange struct {
	routers    []lgSimulatedRoute
	neighbours []lgSimulatedRoute
}

// lgSimulatedSplitRoutesByPeerRange splits the rows listed for prefix into those a router
// sent and those an external neighbour sent. A prefix that was not searched,
// and a peer in neither range, are errors.
func lgSimulatedSplitRoutesByPeerRange(prefix netip.Prefix, listed map[netip.Prefix][]lgSimulatedRoute) (lgSimulatedRoutesByPeerRange, error) {
	routes, searched := listed[prefix]
	if !searched {
		return lgSimulatedRoutesByPeerRange{}, fmt.Errorf("the model holds no prefix %s, which every generated network has", prefix)
	}
	var split lgSimulatedRoutesByPeerRange
	for _, route := range routes {
		peer, err := netip.ParseAddr(route.peer)
		if err != nil {
			return lgSimulatedRoutesByPeerRange{}, fmt.Errorf("search for %s: peer %q: %w", prefix, route.peer, err)
		}
		inRouters := lgSimulatedRouterRange.Contains(peer)
		inNeighbours := lgSimulatedNeighbourRange.Contains(peer)
		if inRouters {
			split.routers = append(split.routers, route)
		}
		if inNeighbours {
			split.neighbours = append(split.neighbours, route)
		}
		if !inRouters {
			if !inNeighbours {
				return lgSimulatedRoutesByPeerRange{}, fmt.Errorf("search for %s listed the peer %s, which is neither a router nor an external neighbour of the network",
					prefix, route.peer)
			}
		}
	}
	return split, nil
}

// The cells of a looking glass table. A header cell holds text alone. A body
// cell may hold markup after its text (the AS name beside an AS number), and
// only the text before it is read.
var (
	lgSimulatedHeaderCell = regexp.MustCompile(`<th[^>]*>([^<]*)</th>`)
	lgSimulatedBodyCell   = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
)

// lgSimulatedTable is the first table of a looking glass page, read by column
// name, so a column added to the page moves no assertion.
type lgSimulatedTable struct {
	columns map[string]int
	rows    [][]string
}

// lgSimulatedReadTable reads the first table of page. A page with no table,
// which is what a search with no result renders, is an error.
func lgSimulatedReadTable(page string) (lgSimulatedTable, error) {
	headStart := strings.Index(page, "<thead>")
	if headStart < 0 {
		return lgSimulatedTable{}, fmt.Errorf("the page renders no table: %.400q", page)
	}
	headEnd := strings.Index(page[headStart:], "</thead>")
	if headEnd < 0 {
		return lgSimulatedTable{}, fmt.Errorf("the page renders an unclosed table header: %.400q", page[headStart:])
	}
	table := lgSimulatedTable{columns: map[string]int{}}
	for index, cell := range lgSimulatedHeaderCell.FindAllStringSubmatch(page[headStart:headStart+headEnd], -1) {
		table.columns[html.UnescapeString(strings.TrimSpace(cell[1]))] = index
	}

	rest := page[headStart+headEnd:]
	bodyStart := strings.Index(rest, "<tbody")
	if bodyStart < 0 {
		return lgSimulatedTable{}, fmt.Errorf("the page renders a table with no body: %.400q", rest)
	}
	bodyEnd := strings.Index(rest[bodyStart:], "</tbody>")
	if bodyEnd < 0 {
		return lgSimulatedTable{}, fmt.Errorf("the page renders an unclosed table body: %.400q", rest[bodyStart:])
	}
	for _, row := range strings.Split(rest[bodyStart:bodyStart+bodyEnd], "<tr")[1:] {
		cells := lgSimulatedBodyCell.FindAllStringSubmatch(row, -1)
		values := make([]string, len(cells))
		for index, cell := range cells {
			text := cell[1]
			if markup := strings.IndexByte(text, '<'); markup >= 0 {
				text = text[:markup]
			}
			values[index] = html.UnescapeString(strings.TrimSpace(text))
		}
		table.rows = append(table.rows, values)
	}
	return table, nil
}

// cell answers the text of row under the named column. A column the page does
// not have, and a row shorter than the header, are errors.
func (t lgSimulatedTable) cell(row []string, column string) (string, error) {
	index, known := t.columns[column]
	if !known {
		return "", fmt.Errorf("the table has no column %q: its columns are %v", column, t.columns)
	}
	if index >= len(row) {
		return "", fmt.Errorf("a row holds %d cells and the column %q is cell %d: %v", len(row), column, index+1, row)
	}
	return row[index], nil
}
