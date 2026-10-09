//go:build linux

// Design: docs/functional-tests.md -- independent implementation namespace proof.
// Related: plugin_fixture_rfc2545_joint_subnet_linux.go -- shared topology/readback.
// Related: plugin_fixture_rfc2545_joint_subnet_frr_oracle_linux.go -- FRR JSON oracle.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netns"
)

// jointSubnetFRRPlan names every external executable explicitly. The execution
// owner resolves installed Alpine paths; this fixture never assumes a package
// version or filesystem layout. runtimeDir MUST be a fresh local filesystem
// directory for Unix sockets, output MUST be a fresh retained evidence directory.
// Only parseJointSubnetFRRPlan produces usable plans; nil and zero plans are
// uninitialized. Callers MUST keep a parsed plan immutable for the run.
type jointSubnetFRRPlan struct {
	prefix, runtimeDir, output          string
	ze, le, zebra, bgpd, vtysh, tcpdump string
	port                                uint16
	scenario                            jointSubnetFRRScenario
}

// jointSubnetFRRScenario is the fixture's finite case set. Its zero is
// uninitialized; nextHop rejects it and out-of-range values without a fallback.
type jointSubnetFRRScenario uint8

const (
	jointSubnetFRRUnspecified jointSubnetFRRScenario = iota
	jointSubnetFRRSameLink
	jointSubnetFRRSplitLink
)

// String is only for human output and the legacy topology configuration sink.
func (scenario jointSubnetFRRScenario) String() string {
	switch scenario {
	case jointSubnetFRRUnspecified:
		return "unspecified"
	case jointSubnetFRRSameLink:
		return "same-link"
	case jointSubnetFRRSplitLink:
		return "split-link"
	default:
		// Named scalars admit arbitrary conversions; never call them split-link.
		return "invalid"
	}
}

func (scenario jointSubnetFRRScenario) nextHop() (netip.Addr, error) {
	switch scenario {
	case jointSubnetFRRSameLink:
		return netip.AddrFrom16([16]byte{0x20, 1, 0x0d, 0xb8, 0, 0x0a, 15: 9}), nil
	case jointSubnetFRRSplitLink:
		return netip.AddrFrom16([16]byte{0x20, 1, 0x0d, 0xb8, 0, 0x0b, 15: 9}), nil
	case jointSubnetFRRUnspecified:
		return netip.Addr{}, fmt.Errorf("joint-subnet-frr: uninitialized scenario")
	default:
		// Named scalars admit arbitrary conversions; reject unsupported input.
		return netip.Addr{}, fmt.Errorf("joint-subnet-frr: unsupported scenario %d", scenario)
	}
}

func parseJointSubnetFRRPlan(args []string) (*jointSubnetFRRPlan, error) {
	var plan jointSubnetFRRPlan
	var scenarioArgument, portArgument string
	fields := []struct {
		name  string
		value *string
	}{
		{"case", &scenarioArgument}, {"netns", &plan.prefix}, {"port", &portArgument},
		{"runtime", &plan.runtimeDir}, {"output", &plan.output},
		{"ze", &plan.ze}, {"le", &plan.le}, {"zebra", &plan.zebra},
		{"bgpd", &plan.bgpd}, {"vtysh", &plan.vtysh}, {"tcpdump", &plan.tcpdump},
	}
	if len(args) != 2*len(fields) {
		return nil, fmt.Errorf("joint-subnet-frr: expected case NAME netns PREFIX port PORT runtime DIR output DIR ze PATH le PATH zebra PATH bgpd PATH vtysh PATH tcpdump PATH")
	}
	for i, field := range fields {
		if args[2*i] != field.name {
			return nil, fmt.Errorf("joint-subnet-frr: expected %s, got %q", field.name, args[2*i])
		}
		if args[2*i+1] == "" {
			return nil, fmt.Errorf("joint-subnet-frr: empty %s", field.name)
		}
		*field.value = args[2*i+1]
	}
	switch scenarioArgument {
	case "same-link":
		plan.scenario = jointSubnetFRRSameLink
	case "split-link":
		plan.scenario = jointSubnetFRRSplitLink
	default:
		return nil, fmt.Errorf("joint-subnet-frr: only same-link and split-link are defined")
	}
	port, err := strconv.ParseUint(portArgument, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("joint-subnet-frr port: %w", err)
	}
	if port == 0 {
		return nil, fmt.Errorf("joint-subnet-frr: port must be a leased nonzero port")
	}
	plan.port = uint16(port)
	for _, directory := range []*string{&plan.runtimeDir, &plan.output} {
		absolute, err := filepath.Abs(*directory)
		if err != nil {
			return nil, err
		}
		*directory = absolute
	}
	if len(plan.runtimeDir) > 70 {
		return nil, fmt.Errorf("joint-subnet-frr: runtime directory too long for Unix sockets")
	}
	return &plan, nil
}

// jointSubnetFRRDriver runs a bounded independent FRR recipient, not a generic
// interop backend. It preserves JSON, daemon logs, version and recipient capture.
// The exact-wire native peer carrier remains a separate, unchanged proof.
func jointSubnetFRRDriver(parent context.Context, args []string) error {
	plan, err := parseJointSubnetFRRPlan(args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	if err := os.Mkdir(plan.output, 0o755); err != nil {
		return fmt.Errorf("fresh evidence directory: %w", err)
	}
	if err := os.Mkdir(plan.runtimeDir, 0o755); err != nil {
		return fmt.Errorf("fresh local runtime directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(plan.runtimeDir); err != nil {
			netnsSay("joint-subnet-frr runtime cleanup: ", err.Error())
		}
	}()
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		return err
	}
	defer orig.Close() //nolint:errcheck // best-effort descriptor close at exit
	var topology jointSubnetTopology
	defer topology.path.remove()
	if err := topology.create(plan.prefix, orig); err != nil {
		return err
	}
	if err := topology.configure(plan.scenario.String(), orig); err != nil {
		return fmt.Errorf("FRR topology setup: %w", err)
	}
	if err := topology.assertOwnership(); err != nil {
		return fmt.Errorf("FRR topology ownership: %w", err)
	}
	netnsSay("TOPOLOGY-PASSED: FRR ", plan.scenario.String())
	if err := writeJointSubnetFRRFiles(plan); err != nil {
		return err
	}
	version, err := jointSubnetFRRCommand(ctx, []string{plan.bgpd, "--version"}, topology.path.router, orig)
	if err != nil {
		return fmt.Errorf("installed FRR version: %w", err)
	}
	if err := os.WriteFile(filepath.Join(plan.output, "version.txt"), version, 0o644); err != nil {
		return err
	}
	return runJointSubnetFRR(ctx, plan, &topology, orig)
}

// runJointSubnetFRR owns four long-lived processes and one injector. All starts
// happen on the locked thread, and stop MUST join every child before namespaces
// or runtime sockets disappear. Capture is stopped with SIGINT to flush pcap.
func runJointSubnetFRR(ctx context.Context, plan *jointSubnetFRRPlan, topology *jointSubnetTopology, orig netns.NsHandle) (result error) {
	if plan == nil {
		return fmt.Errorf("joint-subnet-frr: uninitialized plan")
	}
	if plan.port == 0 {
		return fmt.Errorf("joint-subnet-frr: uninitialized plan")
	}
	switch plan.scenario {
	case jointSubnetFRRSameLink, jointSubnetFRRSplitLink:
	case jointSubnetFRRUnspecified:
		return fmt.Errorf("joint-subnet-frr: uninitialized scenario")
	default:
		// Same-package literals can bypass the CLI parser; refuse invalid plans.
		return fmt.Errorf("joint-subnet-frr: unsupported scenario %d", plan.scenario)
	}
	watched := &watchedProcesses{exits: make(chan processExit, 5)}
	defer func() {
		result = finishJointSubnetFRR(ctx, watched, result)
		if result == nil {
			netnsSay("FRR-PASSED: ", plan.scenario.String(), " exact subject next hops and community65001:7 completion fence; capture retained")
		}
	}()
	portArgument := strconv.Itoa(int(plan.port))
	zserv := filepath.Join(plan.runtimeDir, "zserv.api")
	common := []string{"-f", filepath.Join(plan.output, "frr.conf"), "-z", zserv, "--vty_socket", plan.runtimeDir, "-P", "0", "-u", "root", "-g", "root"}
	zebra := append([]string{plan.zebra}, common...)
	zebra = append(zebra, "-i", filepath.Join(plan.runtimeDir, "zebra.pid"))
	if _, err := startJointSubnetFRRProcess(ctx, zebra, topology.path.router, orig, plan.output, "zebra", watched); err != nil {
		return err
	}
	if err := pollJointSubnetFRR(ctx, watched, nil, func() (bool, error) { return peerFileExists(zserv) }); err != nil {
		return fmt.Errorf("FRR zebra socket setup: %w", err)
	}
	bgpd := append([]string{plan.bgpd}, common...)
	bgpd = append(bgpd, "-n", "-p", portArgument, "-i", filepath.Join(plan.runtimeDir, "bgpd.pid"))
	if _, err := startJointSubnetFRRProcess(ctx, bgpd, topology.path.router, orig, plan.output, "bgpd", watched); err != nil {
		return err
	}
	captureWatch := &watchedProcesses{exits: make(chan processExit, 1)}
	capture, err := startJointSubnetFRRProcess(context.WithoutCancel(ctx), []string{plan.tcpdump, "-i", "p0", "-nn", "-s", "0", "--immediate-mode", "-U", "-w", filepath.Join(plan.output, "recipient.pcap"), "tcp", "port", portArgument}, topology.path.router, orig, plan.output, "capture", captureWatch)
	if err != nil {
		return err
	}
	// Capture is deliberately independent of the execution deadline. Flush on
	// every return, before killing the other children, even after semantic red.
	defer func() {
		if err := capture.flush(); err != nil {
			if result == nil {
				result = err
			} else {
				netnsSay("FRR capture cleanup: ", err.Error())
			}
		}
	}()
	if err := pollJointSubnetFRR(ctx, watched, captureWatch, func() (bool, error) {
		data, err := os.ReadFile(filepath.Join(plan.output, "capture.log"))
		if err != nil {
			return false, err
		}
		return strings.Contains(string(data), "listening on p0"), nil
	}); err != nil {
		return fmt.Errorf("FRR capture setup: %w", err)
	}
	if _, err := startJointSubnetFRRProcess(ctx, []string{plan.ze, "start", filepath.Join(plan.output, "ze.conf")}, topology.path.sender, orig, plan.output, "ze", watched); err != nil {
		return err
	}
	if err := pollJointSubnetFRR(ctx, watched, captureWatch, func() (bool, error) {
		return jointSubnetFRRReady(ctx, plan, topology.path.router, orig)
	}); err != nil {
		return fmt.Errorf("FRR recipient establishment setup: %w", err)
	}
	if _, err := startJointSubnetFRRProcess(ctx, []string{plan.le, "test", lePeerVerb, "--port", portArgument, filepath.Join(plan.output, "inject.peer")}, topology.path.far, orig, plan.output, "inject", watched); err != nil {
		return err
	}
	if err := pollJointSubnetFRR(ctx, watched, captureWatch, func() (bool, error) {
		// RFC 2545 Section 3: the independent recipient's retained addresses
		// must match the same-link pair or the split-link global-only field.
		return jointSubnetFRRReceived(ctx, plan, topology.path.router, orig)
	}); err != nil {
		return err
	}
	// FRR receipt does not fence libpcap's kernel queue. Observe both complete
	// UPDATEs in the savefile itself before the deferred SIGINT/flush/join.
	var captureSize int64
	if err := pollJointSubnetFRR(ctx, watched, captureWatch, func() (bool, error) {
		return jointSubnetFRRCaptureReady(plan, &captureSize)
	}); err != nil {
		return fmt.Errorf("FRR capture completion: %w", err)
	}
	return nil
}

// jointSubnetFRRProcess is a successfully started child and its sole join owner.
// Only startJointSubnetFRRProcess constructs it. Nil and zero handles are invalid.
// It is not safe for concurrent use; callers MUST call stopJointSubnetFRRProcesses
// for its watcher, or flush for capture. Copies share the watcher, which records
// a consumed join so a stale alias cannot wait a second time.
type jointSubnetFRRProcess struct {
	command *exec.Cmd
	watched *watchedProcesses
}

// flush MUST be called for a started capture on every exit path, before stopping
// its peers. It has its own teardown budget, never the expired execution context.
// A forced kill is an evidence failure, not a clean flush.
func (capture *jointSubnetFRRProcess) flush() error {
	if capture == nil {
		return fmt.Errorf("cannot flush an uninitialized FRR process")
	}
	if capture.command == nil {
		return fmt.Errorf("cannot flush an uninitialized FRR process")
	}
	watched := capture.watched
	if watched.pending == 0 {
		// The poll already reported and joined this premature capture exit.
		return nil
	}
	signalErr := capture.command.Process.Signal(syscall.SIGINT)
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case exit := <-watched.exits:
		watched.pending--
		if signalErr != nil {
			return fmt.Errorf("flush recipient capture: %w", signalErr)
		}
		if exit.err != nil {
			return fmt.Errorf("capture flush: %w", exit.err)
		}
		return nil
	case <-timer.C:
	}
	killErr := capture.command.Process.Kill()
	timer.Reset(2 * time.Second)
	select {
	case <-watched.exits:
		watched.pending--
		return fmt.Errorf("capture flush exceeded three seconds; forced termination: %v", killErr)
	case <-timer.C:
		return fmt.Errorf("capture failed to join after forced termination: %v", killErr)
	}
}

// finishJointSubnetFRR decides the verdict only after owned cleanup. The
// execution deadline can expire during capture flush or child teardown; that
// cancellation MUST remain a failure even if the earlier polling fence passed.
func finishJointSubnetFRR(ctx context.Context, watched *watchedProcesses, result error) error {
	cleanup := stopJointSubnetFRRProcesses(watched)
	return errors.Join(result, cleanup, ctx.Err())
}

// stopJointSubnetFRRProcesses MUST signal and join the watcher before namespace
// teardown. A forced kill or a failed join is an evidence failure, never success.
func stopJointSubnetFRRProcesses(watched *watchedProcesses) error {
	var result error
	for _, child := range watched.started {
		if err := child.Process.Signal(syscall.SIGTERM); err != nil {
			if !errors.Is(err, os.ErrProcessDone) {
				result = errors.Join(result, fmt.Errorf("FRR child termination signal: %w", err))
			}
		}
	}
	if err := joinJointSubnetFRRProcesses(watched, 3*time.Second); err != nil {
		result = errors.Join(result, err)
		for _, child := range watched.started {
			if err := child.Process.Kill(); err != nil {
				if !errors.Is(err, os.ErrProcessDone) {
					result = errors.Join(result, fmt.Errorf("FRR child forced termination: %w", err))
				}
			}
		}
		return errors.Join(result, joinJointSubnetFRRProcesses(watched, 2*time.Second))
	}
	return result
}

// joinJointSubnetFRRProcesses MUST be called after signaling these children.
// Only the watcher's owner consumes exits; its waiter channels hold one result
// per child, so a deadline cannot strand a goroutine on an exit-channel send.
func joinJointSubnetFRRProcesses(watched *watchedProcesses, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for watched.pending > 0 {
		select {
		case <-watched.exits:
			watched.pending--
		case <-timer.C:
			return fmt.Errorf("FRR children failed to join within %s: %d pending", timeout, watched.pending)
		}
	}
	return nil
}

// pollJointSubnetFRR bounds every setup/state wait by the driver's deadline and
// detects any daemon's premature exit. A timeout is explicitly not a semantic red.
func pollJointSubnetFRR(ctx context.Context, watched, capture *watchedProcesses, predicate func() (bool, error)) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var captureExits <-chan processExit
	if capture != nil {
		captureExits = capture.exits
	}
	for {
		ready, err := predicate()
		if err != nil {
			return err
		}
		// A ready predicate cannot hide an already queued process exit or an
		// expired deadline. Only the ordinary pending case waits for a tick.
		select {
		case exit := <-watched.exits:
			watched.pending--
			return fmt.Errorf("FRR fixture process %s exited: %v", exit.label, exit.err)
		case exit := <-captureExits:
			capture.pending--
			return fmt.Errorf("FRR recipient capture exited prematurely: %v", exit.err)
		case <-ctx.Done():
			return fmt.Errorf("FRR fixture setup or completion deadline, not a protocol verdict: %w", ctx.Err())
		default:
		}
		if ready {
			return nil
		}
		select {
		case exit := <-watched.exits:
			watched.pending--
			return fmt.Errorf("FRR fixture process %s exited: %v", exit.label, exit.err)
		case exit := <-captureExits:
			capture.pending--
			return fmt.Errorf("FRR recipient capture exited prematurely: %v", exit.err)
		case <-ticker.C:
		case <-ctx.Done():
			return fmt.Errorf("FRR fixture setup or completion deadline, not a protocol verdict: %w", ctx.Err())
		}
	}
}

// startJointSubnetFRRProcess preserves each child's complete log while reusing
// the locked-thread start and shared child watcher. On success the caller MUST
// call stopJointSubnetFRRProcesses, or flush for capture's separately owned watcher.
func startJointSubnetFRRProcess(ctx context.Context, argv []string, namespace *testNetns, orig netns.NsHandle, output, label string, watched *watchedProcesses) (jointSubnetFRRProcess, error) {
	log, err := os.OpenFile(filepath.Join(output, label+".log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return jointSubnetFRRProcess{}, err
	}
	defer log.Close() //nolint:errcheck // the child inherits its own file descriptor
	if err := netns.Set(namespace.ns); err != nil {
		return jointSubnetFRRProcess{}, err
	}
	command := newJointSubnetFRRChild(ctx, argv)
	command.Stdout, command.Stderr = log, log
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	startErr := command.Start()
	if startErr == nil {
		watched.watch(command, label, false)
	}
	if err := netns.Set(orig); err != nil {
		if startErr == nil {
			return jointSubnetFRRProcess{}, errors.Join(err, stopJointSubnetFRRProcesses(watched))
		}
		return jointSubnetFRRProcess{}, err
	}
	if startErr != nil {
		return jointSubnetFRRProcess{}, startErr
	}
	return jointSubnetFRRProcess{command: command, watched: watched}, nil
}

// newJointSubnetFRRChild leaves termination exclusively to the fixture's bounded
// signal/join owner. The caller MUST register every successful start with its
// watcher and MUST stop that watcher, even after execution-context cancellation.
func newJointSubnetFRRChild(ctx context.Context, argv []string) *exec.Cmd {
	return exec.CommandContext(context.WithoutCancel(ctx), argv[0], argv[1:]...)
}
