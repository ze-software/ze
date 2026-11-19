// Design: docs/architecture/fleet-config.md -- the freeze of a managed device's own configuration
// Related: register_managed_fleet_freeze.go -- registers the driver
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun and fleetHoldScenario, the hub and the device
// Related: misc_fixture_managed_fleet_leave.go -- requestFleet, requireLinked, deviceImport, requireDeviceRuns

package fixture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The texts of the store's refusal (internal/component/config/storage,
// freeze.go, ErrConfigFrozen) that the verdicts read.
const (
	fleetFreezeProblem = "the configuration was not changed: this device is managed by a hub"
	fleetFreezeFix     = "fix: request fleet disable"
)

// managedFleetFreezeRefusesCommitDriver answers one question: does a managed
// device refuse an operator's change to its own configuration, naming
// `request fleet disable`, on a running daemon and on its stopped store, while
// its hub's push still applies, and accept the same change once it left the
// fleet (spec-fleet-6 AC-16, AC-17, user story 2)?
//
// Method: one real hub and one real device that serves SSH, both `ze start`.
// The device takes the hub's configuration. An operator beside the running
// device then imports another configuration (`ze data import`, which the
// daemon receives as `update data config`) and asks for a rollback
// (`request data config rollback`): both MUST be refused with the freeze's
// text, and the device's active configuration and every key of its store that
// names the configuration MUST be as before. The hub's copy changes, and the
// device MUST take it. With the daemon stopped, `ze data import` and
// `ze config import` of the device's own stored configuration MUST be refused
// the same way. Started again, the
// device leaves the fleet with `request fleet disable`, and the import that
// was refused MUST become the configuration it runs.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetFreezeRefusesCommitDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-freeze-refuses-commit", args, true, func(ctx context.Context, s *fleetHoldScenario) error {
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireFrozenWhileRunning(ctx, daemon)
		}); err != nil {
			return err
		}
		if err := s.requireFrozenWhileStopped(ctx); err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireChangeAfterLeaving(ctx, daemon)
		}); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: a managed device refused a local change to its configuration, running and stopped, naming request fleet disable, "+ //nolint:errcheck // fixture progress line
			"took its hub's push meanwhile, and accepted the same change once it left the fleet, version:", s.version(fleetHoldLocalRouterID))
		return nil
	})
}

// requireFrozenWhileRunning is the running device that follows its hub: two
// operator routes through its daemon are refused and change nothing, and a
// hub push then applies.
func (s *fleetHoldScenario) requireFrozenWhileRunning(ctx context.Context, daemon *cachedBootDaemon) error {
	if err := s.requireFollows(ctx, daemon, restoreClientServedRouterID, "the first connection"); err != nil {
		return err
	}
	if _, err := s.requireLinked(ctx, daemon, "before the operator's change"); err != nil {
		return err
	}
	before, err := s.ownConfigKeys(ctx)
	if err != nil {
		return err
	}

	path, err := s.localChangeFile()
	if err != nil {
		return err
	}
	stdout, stderr, code, err := rawCommandStreams(ctx, s.device.dir, s.deviceOperator.env,
		"ze", "data", "--path", s.device.dbPath, "import", path)
	if err != nil {
		return err
	}
	if err := fleetFreezeRequireRefusal(stdout, stderr, code, "ze data import beside the running managed device"); err != nil {
		return err
	}
	rollback := "request data config rollback name " + caTrustClientName + ".conf revision 1"
	stdout, stderr, code, err = rawCommandStreams(ctx, s.device.dir, s.deviceOperator.env, "ze", "cli", "-c", rollback)
	if err != nil {
		return err
	}
	if err := fleetFreezeRequireRefusal(stdout, stderr, code, "ze cli -c \""+rollback+"\" on the running managed device"); err != nil {
		return err
	}
	if err := s.requireUnchanged(ctx, before, restoreClientServedRouterID, "after the daemon refused two local changes"); err != nil {
		return err
	}

	// The hub's push is the one change a managed device exists to take.
	if err := s.hubImport(ctx, fleetHoldChangedRouterID); err != nil {
		return err
	}
	return s.requireFollows(ctx, daemon, fleetHoldChangedRouterID, "a hub push to the frozen device")
}

// requireFrozenWhileStopped is the same device with no daemon: the tools that
// open its store themselves are refused and change nothing.
func (s *fleetHoldScenario) requireFrozenWhileStopped(ctx context.Context) error {
	before, err := s.ownConfigKeys(ctx)
	if err != nil {
		return err
	}
	path, err := s.localChangeFile()
	if err != nil {
		return err
	}
	stdout, stderr, code, err := rawCommandStreams(ctx, s.device.dir, s.device.env,
		"ze", "data", "--path", s.device.dbPath, "import", path)
	if err != nil {
		return err
	}
	if err := fleetFreezeRequireRefusal(stdout, stderr, code, "ze data import on the stopped managed device"); err != nil {
		return err
	}
	// `ze config import` replaces a stored configuration through
	// storage.RestoreConfig (cmdImportWrite), the operator's staging step. Not
	// `ze config rollback <N> <file>`: with no daemon that command rolls back
	// the disk file it names (cmdRollbackThrough, a loose file editor) and never
	// touches the store, so the freeze has nothing to refuse.
	stdout, stderr, code, err = rawCommandStreams(ctx, s.device.dir, s.device.env,
		"ze", "config", "import", "--dir", s.device.dir, "--yes", path)
	if err != nil {
		return err
	}
	if err := fleetFreezeRequireRefusal(stdout, stderr, code, "ze config import on the stopped managed device"); err != nil {
		return err
	}
	return s.requireUnchanged(ctx, before, fleetHoldChangedRouterID, "after the stopped store refused two local changes")
}

// requireChangeAfterLeaving is the device started again: it leaves the fleet,
// and the import its daemon refused becomes the configuration it runs.
func (s *fleetHoldScenario) requireChangeAfterLeaving(ctx context.Context, daemon *cachedBootDaemon) error {
	if _, err := s.requireLinked(ctx, daemon, "the start before the device left"); err != nil {
		return err
	}
	if err := s.requestFleet(ctx, fleetLeaveDisable, fleetLeaveNoteClosed); err != nil {
		return err
	}
	if err := s.deviceImport(ctx, s.config(fleetHoldLocalRouterID)); err != nil {
		return fmt.Errorf("the device left the fleet and still refused router-id %s: %w", fleetHoldLocalRouterID, err)
	}
	if err := s.requireDeviceRuns(ctx, s.version(fleetHoldLocalRouterID), "the local change of a device that left the fleet"); err != nil {
		return err
	}
	active, err := caTrustActiveConfig(ctx, s.device)
	if err != nil {
		return err
	}
	if !strings.Contains(active, "router-id "+fleetHoldLocalRouterID) {
		return fmt.Errorf("the local change is the active version and the stored configuration lacks router-id %s:\n%s",
			fleetHoldLocalRouterID, active)
	}
	return nil
}

// localChangeFile writes config(fleetHoldLocalRouterID), the operator's
// change, as a file named as the device's own configuration, and answers its
// path: `ze data import` and `ze config import` name a configuration by the
// base name of the file they are given.
func (s *fleetHoldScenario) localChangeFile() (string, error) {
	path := filepath.Join(s.device.dir, caTrustClientName+".conf")
	if err := os.WriteFile(path, []byte(s.config(fleetHoldLocalRouterID)), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// ownConfigKeys lists, with `ze data list` beside the device, every key of its
// store that names the device's own configuration: the stored file, each
// version, and the pointers, the candidate among them.
func (s *fleetHoldScenario) ownConfigKeys(ctx context.Context) (string, error) {
	stdout, stderr, code, err := rawCommandStreams(ctx, s.device.dir, s.device.env,
		"ze", "data", "--path", s.device.dbPath, "list")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("ze data list exited %d: %s", code, stderr)
	}
	var keys []string
	for line := range strings.SplitSeq(stdout, "\n") {
		if strings.Contains(line, caTrustClientName+".conf") {
			keys = append(keys, strings.TrimSpace(line))
		}
	}
	if len(keys) == 0 {
		return "", fmt.Errorf("ze data list names no key of %s.conf:\n%s", caTrustClientName, stdout)
	}
	return strings.Join(keys, "\n"), nil
}

// requireUnchanged requires what a refused change must leave: the keys that
// name the device's configuration are before, and the stored configuration
// still carries routerID.
func (s *fleetHoldScenario) requireUnchanged(ctx context.Context, before, routerID, state string) error {
	after, err := s.ownConfigKeys(ctx)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("%s the store's keys of %s.conf changed.\nbefore:\n%s\nafter:\n%s", state, caTrustClientName, before, after)
	}
	active, err := caTrustActiveConfig(ctx, s.device)
	if err != nil {
		return err
	}
	if !strings.Contains(active, "router-id "+routerID) {
		return fmt.Errorf("%s the device's configuration lacks router-id %s:\n%s", state, routerID, active)
	}
	return nil
}

// fleetFreezeRequireRefusal requires the freeze's refusal of route, on the
// stream that carries it: a failed command, the problem, and the fix alone on
// the last line.
func fleetFreezeRequireRefusal(stdout, stderr string, code int, route string) error {
	if code == 0 {
		return fmt.Errorf("%s exited zero, want the freeze's refusal:\nstdout:\n%s\nstderr:\n%s", route, stdout, stderr)
	}
	refusal := stderr
	if strings.Contains(stdout, fleetFreezeProblem) {
		refusal = stdout
	}
	if !strings.Contains(refusal, fleetFreezeProblem) {
		return fmt.Errorf("%s was not refused with %q:\nstdout:\n%s\nstderr:\n%s", route, fleetFreezeProblem, stdout, stderr)
	}
	lines := strings.Split(strings.TrimSpace(refusal), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last != fleetFreezeFix {
		return fmt.Errorf("%s: the last line of the refusal is %q, want %q:\n%s", route, last, fleetFreezeFix, refusal)
	}
	return nil
}
