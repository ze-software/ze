// Design: docs/architecture/fleet-config.md -- request fleet disable and enable: a managed client leaves its hub and rejoins it
// Related: register_managed_fleet_leave.go -- registers the two drivers
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun and fleetHoldScenario, the hub and the device both drivers run on
// Related: misc_fixture_managed_fleet_status.go -- fleetStatusScenario, show fleet status beside the device
// Related: misc_fixture_managed_cached_boot.go -- cachedBootStart and cachedBootWait, the device's daemon

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// The commands, the texts the product answers and the lines the hub logs that
// the verdicts read.
const (
	fleetLeaveDisable = "request fleet disable"
	fleetLeaveEnable  = "request fleet enable"
	// The note of each answer (internal/component/managed, cmd_fleet.go).
	fleetLeaveNoteClosed      = "this device left the fleet: its hub connection was closed"
	fleetLeaveNoteNoClient    = "this device left the fleet: it ran no hub client, so there was no hub connection to close"
	fleetLeaveNoteStopped     = "this device left the fleet: no daemon runs, and the next ze start runs no hub client"
	fleetLeaveNoteStarted     = "this device rejoined the fleet: its hub client started and fetches from the hub"
	fleetLeaveNoteRunning     = "this device is in the fleet: its hub client already runs, so none was started"
	fleetLeaveNoteJoinsAtBoot = "this device rejoins the fleet at the next ze start: no daemon runs"
	// The refusal of a rejoin on a device whose configuration names no hub
	// (cmd_fleet.go, noDeclaredHub), and the fix on its last line
	// (doctor.go, hubClientMissingFix).
	fleetLeaveNoHubProblem = "request fleet enable: the configuration of this device names no hub"
	fleetLeaveNoHubFix     = "ze init --managed --force"
	// The lines the hub's managed listener writes when it admits a client and
	// when that client's session ends (internal/component/plugin/server,
	// managed_serve.go, handleConn).
	fleetLeaveHubOpened = "managed client connected"
	fleetLeaveHubClosed = "managed client disconnected"
	// The configuration given to the device itself once it left the fleet,
	// which names no hub.
	fleetLeaveAloneRouterID = "7.7.7.7"
	// The hub reads the close of a socket the device closed before it answered
	// the command. The budget, 10 seconds, is far inside the 90 seconds after
	// which the hub would give up on a silent client, so a hub that learns of
	// the loss by that timeout fails the wait.
	fleetLeaveCloseAttempts = 40
)

// fleetLeaveSessions is what the hub's log says of the device's sessions: how
// many the hub admitted and how many of those ended.
type fleetLeaveSessions struct {
	opened int
	closed int
}

// managedFleetDisableSeversDriver answers one question: does `request fleet
// disable` on a connected managed device take it out of the fleet at once,
// with the hub seeing the connection close, and leave the device running its
// configuration and open to a local change (spec-fleet-6 AC-10)?
//
// Method: one real hub and one real device that serves SSH, both `ze start`.
// The device takes the hub's configuration. `request fleet disable` runs as
// `ze cli -c` beside the device. The verdicts are the command's answer, the
// stored flag, the sessions the hub's own log counts, `show fleet status`
// from the device's daemon, and the device's store after `ze data import`
// beside it gave it another configuration.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetDisableSeversDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-disable-severs", args, true, func(ctx context.Context, s *fleetHoldScenario) error {
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireDisableSevers(ctx, daemon)
		}); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: request fleet disable closed the hub connection of a managed device at once, "+ //nolint:errcheck // fixture progress line
			"the device kept its configuration, and a local change then became active, version:", s.version(fleetHoldLocalRouterID))
		return nil
	})
}

// managedFleetEnableRejoinsDriver answers one question: does `request fleet
// enable` put a device that left the fleet back on its hub with no restart,
// refuse while the device's configuration names no hub, and store the flag
// when no daemon runs (spec-fleet-6 AC-11, AC-13, AC-14)?
//
// Method: the hub and the device of the driver above. The device leaves, the
// hub's copy of its configuration changes meanwhile, and the device rejoins:
// the same `ze start` process MUST connect again and take the change. The
// device then leaves once more and is given a configuration with no hub
// block, and the rejoin MUST be refused with the flag left false, by the
// daemon and then by `ze request fleet enable` with the daemon stopped. With
// a configuration that names the hub again, the stopped store MUST take the
// flag from both commands, and the next `ze start` MUST follow it.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetEnableRejoinsDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-enable-rejoins", args, true, func(ctx context.Context, s *fleetHoldScenario) error {
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireRejoinsThenRefuses(ctx, daemon)
		}); err != nil {
			return err
		}
		if err := s.requireStoppedFlagChanges(ctx); err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			_, err := s.requireLinked(ctx, daemon, "the start after ze request fleet enable")
			return err
		}); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: request fleet enable put a managed device back on its hub with no restart and it took the hub's change, "+ //nolint:errcheck // fixture progress line
			"a configuration with no hub was refused, and a stopped device stored the flag, version:", s.version(fleetHoldChangedRouterID))
		return nil
	})
}

// requireDisableSevers is the running device that follows its hub: it leaves
// the fleet, and is then changed locally.
func (s *fleetHoldScenario) requireDisableSevers(ctx context.Context, daemon *cachedBootDaemon) error {
	if err := s.requireFollows(ctx, daemon, restoreClientServedRouterID, "the first connection"); err != nil {
		return err
	}
	before, err := s.requireLinked(ctx, daemon, "before the device left")
	if err != nil {
		return err
	}
	lost := fleetHoldLogLines(daemon.output.String(), cachedBootReconnectMessage)

	if err := s.requestFleet(ctx, fleetLeaveDisable, fleetLeaveNoteClosed); err != nil {
		return err
	}
	if err := s.requireHubSawClose(ctx, daemon, before); err != nil {
		return err
	}
	if err := s.requireLeft(ctx, "after request fleet disable"); err != nil {
		return err
	}
	// The device keeps the configuration its hub gave it, and the record of it.
	active, err := caTrustActiveConfig(ctx, s.device)
	if err != nil {
		return err
	}
	if !strings.Contains(active, "router-id "+restoreClientServedRouterID) {
		return fmt.Errorf("the device left the fleet and no longer runs the configuration it ran (router-id %s):\n%s",
			restoreClientServedRouterID, active)
	}
	baseline, err := s.baseline(ctx)
	if err != nil {
		return err
	}
	if baseline != s.version(restoreClientServedRouterID) {
		return fmt.Errorf("the device left the fleet and its baseline is %q, want %q, the hash of the configuration its hub gave it",
			baseline, s.version(restoreClientServedRouterID))
	}
	// Leaving twice is not an error: there is no client left to stop.
	if err := s.requestFleet(ctx, fleetLeaveDisable, fleetLeaveNoteNoClient); err != nil {
		return err
	}

	// A local change, by the import an operator beside the device runs.
	if err := s.deviceImport(ctx, s.config(fleetHoldLocalRouterID)); err != nil {
		return fmt.Errorf("give the device that left the fleet router-id %s: %w", fleetHoldLocalRouterID, err)
	}
	if err := s.requireDeviceRuns(ctx, s.version(fleetHoldLocalRouterID), "the local change of a device that left the fleet"); err != nil {
		return err
	}
	active, err = caTrustActiveConfig(ctx, s.device)
	if err != nil {
		return err
	}
	if !strings.Contains(active, "router-id "+fleetHoldLocalRouterID) {
		return fmt.Errorf("the local change is the active version and the stored configuration lacks router-id %s:\n%s",
			fleetHoldLocalRouterID, active)
	}

	// The local change became active after the device left, so what follows
	// is read after everything the leave could still have caused.
	if after := fleetHoldLogLines(daemon.output.String(), cachedBootReconnectMessage); after != lost {
		return fmt.Errorf("the device logged %q %d times before it left the fleet and %d after: "+
			"a device that left MUST NOT try its hub again", cachedBootReconnectMessage, lost, after)
	}
	sessions := s.hubSessions()
	if sessions.opened != before.opened {
		return fmt.Errorf("the hub admitted the device %d times before it left the fleet and %d after: "+
			"a device that left MUST NOT connect again", before.opened, sessions.opened)
	}
	if sessions.closed != sessions.opened {
		return fmt.Errorf("the hub admitted the device %d times and saw %d of those sessions end: one is still open after the device left",
			sessions.opened, sessions.closed)
	}
	return s.requireLeft(ctx, "after the local change")
}

// requireRejoinsThenRefuses is the running device that follows its hub: it
// leaves, rejoins and takes what the hub changed meanwhile, then leaves again,
// is given a configuration that names no hub, and is refused the rejoin.
func (s *fleetHoldScenario) requireRejoinsThenRefuses(ctx context.Context, daemon *cachedBootDaemon) error {
	if err := s.requireFollows(ctx, daemon, restoreClientServedRouterID, "the first connection"); err != nil {
		return err
	}
	before, err := s.requireLinked(ctx, daemon, "before the device left")
	if err != nil {
		return err
	}
	connections := fleetHoldLogLines(daemon.output.String(), firstStartConnectedMessage, s.hubAddress)

	if err := s.requestFleet(ctx, fleetLeaveDisable, fleetLeaveNoteClosed); err != nil {
		return err
	}
	if err := s.requireHubSawClose(ctx, daemon, before); err != nil {
		return err
	}
	// The hub may write a declared client's configuration while it is away.
	if err := s.hubImport(ctx, fleetHoldChangedRouterID); err != nil {
		return err
	}

	if err := s.requestFleet(ctx, fleetLeaveEnable, fleetLeaveNoteStarted); err != nil {
		return err
	}
	if err := s.requireFlag(ctx, valueTrue); err != nil {
		return err
	}
	// The device changed nothing while it was away, so it follows its hub.
	if err := s.requireFollows(ctx, daemon, fleetHoldChangedRouterID, "after request fleet enable"); err != nil {
		return err
	}
	rejoined, err := s.requireLinked(ctx, daemon, "after request fleet enable")
	if err != nil {
		return err
	}
	if rejoined.opened != before.opened+1 {
		return fmt.Errorf("the hub admitted the device %d times before it left and %d after it rejoined, want one more",
			before.opened, rejoined.opened)
	}
	// daemon is the process started before the leave, and cachedBootWait
	// fails once it exits: a second connection in its own output is a rejoin
	// with no restart.
	if again := fleetHoldLogLines(daemon.output.String(), firstStartConnectedMessage, s.hubAddress); again != connections+1 {
		return fmt.Errorf("the daemon that left the fleet logged %q for %s %d times before and %d after it rejoined, want one more",
			firstStartConnectedMessage, s.hubAddress, connections, again)
	}
	// Rejoining twice is not an error: the client already runs.
	if err := s.requestFleet(ctx, fleetLeaveEnable, fleetLeaveNoteRunning); err != nil {
		return err
	}

	// A device whose configuration names no hub has nothing to rejoin.
	if err := s.requestFleet(ctx, fleetLeaveDisable, fleetLeaveNoteClosed); err != nil {
		return err
	}
	if err := s.requireHubSawClose(ctx, daemon, rejoined); err != nil {
		return err
	}
	alone := s.configWithoutHub(fleetLeaveAloneRouterID)
	if err := s.deviceImport(ctx, alone); err != nil {
		return fmt.Errorf("give the device that left the fleet a configuration with no hub: %w", err)
	}
	if err := s.requireDeviceRuns(ctx, managedVersionHash([]byte(alone)), "a configuration with no hub"); err != nil {
		return err
	}
	stdout, stderr, _, err := rawCommandStreams(ctx, s.device.dir, s.deviceOperator.env, "ze", "cli", "-c", fleetLeaveEnable)
	if err != nil {
		return err
	}
	if err := fleetLeaveRequireNoHubRefusal(stdout, stderr, "request fleet enable through the daemon"); err != nil {
		return err
	}
	return s.requireLeft(ctx, "after the refused request fleet enable")
}

// requireStoppedFlagChanges is the stopped device, out of the fleet, whose
// configuration names no hub: `ze request fleet enable` MUST refuse it. Given
// a configuration that names the hub, both commands MUST store the flag, and
// the device is left managed for its next start. The configuration is the one
// the hub holds and the device's baseline records, so that start does not
// hold.
func (s *fleetHoldScenario) requireStoppedFlagChanges(ctx context.Context) error {
	stdout, stderr, code, err := rawCommandStreams(ctx, s.device.dir, s.deviceOperator.env, "ze", "request", "fleet", "enable")
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("ze request fleet enable exited 0 on a stopped device whose configuration names no hub:\n%s%s", stdout, stderr)
	}
	if err := fleetLeaveRequireNoHubRefusal(stdout, stderr, "ze request fleet enable with the daemon stopped"); err != nil {
		return err
	}
	if err := s.requireFlag(ctx, "false"); err != nil {
		return fmt.Errorf("after the refused ze request fleet enable: %w", err)
	}

	// The flag is false, so this import is the change of a device that is out
	// of the fleet.
	if err := caTrustImportConfig(ctx, s.device, caTrustClientName+".conf", s.config(fleetHoldChangedRouterID)); err != nil {
		return err
	}
	if err := s.requestFleetStopped(ctx, "enable", fleetLeaveNoteJoinsAtBoot, valueTrue); err != nil {
		return err
	}
	if err := s.requestFleetStopped(ctx, "disable", fleetLeaveNoteStopped, "false"); err != nil {
		return err
	}
	return s.requestFleetStopped(ctx, "enable", fleetLeaveNoteJoinsAtBoot, valueTrue)
}

// requestFleet runs command as `ze cli -c` beside the running device and
// requires the answer to carry note, the sentence that says what the daemon
// did.
func (s *fleetHoldScenario) requestFleet(ctx context.Context, command, note string) error {
	stdout, stderr, code, err := rawCommandStreams(ctx, s.device.dir, s.deviceOperator.env, "ze", "cli", "-c", command)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ze cli -c %q exited %d:\n%s%s", command, code, stdout, stderr)
	}
	if !strings.Contains(stdout, note) {
		return fmt.Errorf("ze cli -c %q did not answer %q:\n%s%s", command, note, stdout, stderr)
	}
	return nil
}

// requestFleetStopped runs `ze request fleet <verb>` beside the stopped device
// and requires the answer to carry note and to say that no daemon runs, and
// the store to hold flag.
func (s *fleetHoldScenario) requestFleetStopped(ctx context.Context, verb, note, flag string) error {
	stdout, stderr, code, err := rawCommandStreams(ctx, s.device.dir, s.deviceOperator.env, "ze", "request", "fleet", verb)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ze request fleet %s exited %d with the daemon stopped:\n%s%s", verb, code, stdout, stderr)
	}
	if !strings.Contains(stdout, note) {
		return fmt.Errorf("ze request fleet %s did not answer %q with the daemon stopped:\n%s%s", verb, note, stdout, stderr)
	}
	if !strings.Contains(stdout, fleetStatusNoDaemon) {
		return fmt.Errorf("ze request fleet %s does not say %q with the daemon stopped:\n%s%s", verb, fleetStatusNoDaemon, stdout, stderr)
	}
	if err := s.requireFlag(ctx, flag); err != nil {
		return fmt.Errorf("after ze request fleet %s with the daemon stopped: %w", verb, err)
	}
	return nil
}

// fleetLeaveRequireNoHubRefusal requires the refusal of a rejoin on a device
// whose configuration names no hub, on the stream that carries it: the
// problem, and the fix on the last line.
func fleetLeaveRequireNoHubRefusal(stdout, stderr, route string) error {
	refusal := stderr
	if strings.Contains(stdout, fleetLeaveNoHubProblem) {
		refusal = stdout
	}
	if !strings.Contains(refusal, fleetLeaveNoHubProblem) {
		return fmt.Errorf("%s was not refused with %q:\nstdout:\n%s\nstderr:\n%s", route, fleetLeaveNoHubProblem, stdout, stderr)
	}
	lines := strings.Split(strings.TrimSpace(refusal), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasSuffix(last, fleetLeaveNoHubFix) {
		return fmt.Errorf("%s: the last line of the refusal is %q, want the fix %q:\n%s", route, last, fleetLeaveNoHubFix, refusal)
	}
	return nil
}

// requireFlag requires meta/instance/managed in the device's store to hold
// want.
func (s *fleetHoldScenario) requireFlag(ctx context.Context, want string) error {
	flag, err := s.storeKey(ctx, fleetStatusKeyManaged)
	if err != nil {
		return err
	}
	if flag != want {
		return fmt.Errorf("%s is %q, want %q", fleetStatusKeyManaged, flag, want)
	}
	return nil
}

// requireLeft requires the store and the running daemon to agree that the
// device is out of the fleet: the flag is false, and `show fleet status` from
// the daemon reports it unmanaged with its hub client stopped, no connection
// and nothing held.
func (s *fleetHoldScenario) requireLeft(ctx context.Context, state string) error {
	if err := s.requireFlag(ctx, "false"); err != nil {
		return fmt.Errorf("%s: %w", state, err)
	}
	status, err := s.deviceOperator.askDaemon(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", state, err)
	}
	if status.Managed {
		return fmt.Errorf("%s: show fleet status reports the device managed: %+v", state, status)
	}
	if status.Frozen {
		return fmt.Errorf("%s: show fleet status reports the device frozen: %+v", state, status)
	}
	return fleetStatusRequireClient(status, state, fleetStatusClientStopped, fleetStatusNotConnected, fleetStatusNotHolding)
}

// requireLinked waits until the hub holds one open session of the device,
// then requires `show fleet status` from the device's daemon to report it
// managed, with its hub client running and connected, holding nothing and not
// diverged. It answers the hub's session counts at that moment.
func (s *fleetHoldScenario) requireLinked(ctx context.Context, daemon *cachedBootDaemon, state string) (fleetLeaveSessions, error) {
	var sessions fleetLeaveSessions
	open := cachedBootWait(ctx, daemon, fleetHoldLogAttempts, func() bool {
		sessions = s.hubSessions()
		return sessions.opened == sessions.closed+1
	})
	if !open {
		return fleetLeaveSessions{}, fmt.Errorf("%s: the hub holds no open session of %s (it admitted %d and saw %d end), or the device stopped",
			state, caTrustClientName, sessions.opened, sessions.closed)
	}
	// The hub logs the session before the device's client reports it.
	status, err := s.deviceOperator.askDaemonUntil(ctx, "a connected hub client", func(answer fleetStatusAnswer) bool {
		return answer.Connection == fleetStatusConnected
	})
	if err != nil {
		return fleetLeaveSessions{}, fmt.Errorf("%s: %w", state, err)
	}
	if !status.Managed {
		return fleetLeaveSessions{}, fmt.Errorf("%s: show fleet status reports a connected device as not managed: %+v", state, status)
	}
	if status.Diverged {
		return fleetLeaveSessions{}, fmt.Errorf("%s: show fleet status reports a device that follows its hub as diverged: %+v", state, status)
	}
	if err := fleetStatusRequireClient(status, state, fleetStatusClientRunning, fleetStatusConnected, fleetStatusNotHolding); err != nil {
		return fleetLeaveSessions{}, err
	}
	return sessions, nil
}

// requireHubSawClose waits until the hub's own log says every session it
// admitted for the device has ended, and requires that it admitted none since
// before. The wait is on what the hub saw, within a budget the hub's timeout
// for a silent client cannot meet.
func (s *fleetHoldScenario) requireHubSawClose(ctx context.Context, daemon *cachedBootDaemon, before fleetLeaveSessions) error {
	var sessions fleetLeaveSessions
	closed := cachedBootWait(ctx, daemon, fleetLeaveCloseAttempts, func() bool {
		sessions = s.hubSessions()
		return sessions.closed == sessions.opened
	})
	if !closed {
		return fmt.Errorf("the hub did not see the session of %s end after request fleet disable (it admitted %d and saw %d end), "+
			"or the device stopped", caTrustClientName, sessions.opened, sessions.closed)
	}
	if sessions.opened != before.opened {
		return fmt.Errorf("the hub admitted %s %d times before request fleet disable and %d after it: the device connected again",
			caTrustClientName, before.opened, sessions.opened)
	}
	return nil
}

// requireDeviceRuns waits until `show fleet status` from the device's daemon
// reports version as the hash of the configuration the device runs.
func (s *fleetHoldScenario) requireDeviceRuns(ctx context.Context, version, state string) error {
	_, err := s.deviceOperator.askDaemonUntil(ctx, "active hash "+version, func(answer fleetStatusAnswer) bool {
		return answer.ActiveHash == version
	})
	if err != nil {
		return fmt.Errorf("%s did not become the configuration the device runs: %w", state, err)
	}
	return nil
}

// hubSessions counts, in the hub's output so far, the sessions its managed
// listener admitted for the device and the ones it saw end.
func (s *fleetHoldScenario) hubSessions() fleetLeaveSessions {
	return fleetLeaveCountSessions(s.hub.output.String(), caTrustClientName)
}

// fleetLeaveCountSessions counts, in a hub's output, the sessions its managed
// listener admitted for client and the ones it saw end. A line that names
// another client, or another event of this one, counts as neither.
func fleetLeaveCountSessions(output, client string) fleetLeaveSessions {
	var sessions fleetLeaveSessions
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, client) {
			continue
		}
		if strings.Contains(line, fleetLeaveHubOpened) {
			sessions.opened++
		}
		if strings.Contains(line, fleetLeaveHubClosed) {
			sessions.closed++
		}
	}
	return sessions
}

// deviceImport gives the running device content as its own configuration,
// with `ze data import` beside it.
func (s *fleetHoldScenario) deviceImport(ctx context.Context, content string) error {
	return fleetHoldImportBeside(ctx, s.deviceOperator.env, s.device, caTrustClientName+".conf", []byte(content))
}

// configWithoutHub is a configuration of the device that names no hub: its
// listener and a BGP section, with no plugin hub client block.
func (s *fleetHoldScenario) configWithoutHub(routerID string) string {
	return s.deviceListener + "bgp {\n    router-id " + routerID + "\n    session { asn { local 65000; } }\n}\n"
}
