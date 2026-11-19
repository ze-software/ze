// Design: docs/architecture/fleet-config.md -- the hub's config-changed push to a connected client
// Related: register_managed_config_change_live.go -- registers the driver
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun, fleetHoldScenario, the hub and the device
// Related: misc_fixture_managed_fleet_leave.go -- fleetLeaveCountSessions, the hub's session lines
// Related: misc_fixture_managed_fleet_status.go -- fleetStatusAnswer, askDaemonUntil

package fixture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// The line the hub writes when a client acknowledges a configuration it
	// applied (internal/component/plugin/server, managed_serve.go,
	// handleConfigAck), with the client's name and the applied version.
	fleetConfigChangeLiveApplied = "managed client applied config"
	// The budget from the import's answer to the device running the imported
	// configuration with it as its baseline (spec-managed-server-hardening
	// AC-4). It is well inside the 30 seconds of the managed heartbeat, so a
	// device that learned of the change by anything slower than the push
	// fails it.
	fleetConfigChangeLiveBound = 10 * time.Second
)

// managedFleetConfigChangeLiveDriver answers one question: does a client
// configuration imported on a running hub reach its connected device, which
// fetches and applies it on the session it already holds
// (spec-managed-server-hardening AC-4; docs/guide/fleet-config.md, "Change a
// Client's Configuration")?
//
// Method: the hub and the device of fleetHoldRun, the device serving SSH so
// its daemon answers `show fleet status`. The device first takes the hub's
// configuration: it runs it, records it as its baseline and does not diverge,
// and the hub has admitted one session for it. A changed configuration is
// then given to the running hub with `ze data import client-edge-01.conf`
// beside it, which MUST answer that it imported that key. Within
// fleetConfigChangeLiveBound the device MUST report the imported bytes' hash as
// both its active and its baseline hash, with no divergence. The hub MUST then
// log one applied line for the client naming that hash, and its log MUST hold
// exactly one admitted session and no ended one for the client while the
// device still runs: the change came over the session the device already had.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetConfigChangeLiveDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-config-change-live", args, true, func(ctx context.Context, s *fleetHoldScenario) error {
		return s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			if err := s.requireFollows(ctx, daemon, restoreClientServedRouterID, "the first connection"); err != nil {
				return err
			}
			if err := s.requireAppliedOnOneSession(ctx, restoreClientServedRouterID, "before the import"); err != nil {
				return err
			}
			if err := s.requireImportAnswered(ctx, fleetHoldChangedRouterID); err != nil {
				return err
			}
			if err := s.requirePushApplied(ctx, daemon, fleetHoldChangedRouterID); err != nil {
				return err
			}
			if err := s.requireAppliedOnOneSession(ctx, fleetHoldChangedRouterID, "after the import"); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "OK: a configuration imported on the running hub reached its connected device, "+ //nolint:errcheck // fixture progress line
				"which applied it on the same session, active and baseline:", s.version(fleetHoldChangedRouterID))
			return nil
		})
	})
}

// requireAppliedOnOneSession requires the device's daemon to report
// config(routerID)'s hash as its active and baseline hash with no divergence,
// and the hub's log to hold exactly one admitted session for the client and
// no ended one.
func (s *fleetHoldScenario) requireAppliedOnOneSession(ctx context.Context, routerID, state string) error {
	want := s.version(routerID)
	if _, err := s.deviceOperator.askDaemonUntil(ctx, "active and baseline hash "+want+" and no divergence", func(status fleetStatusAnswer) bool {
		return status.ActiveHash == want && status.BaselineHash == want && !status.Diverged
	}); err != nil {
		return fmt.Errorf("%s: %w", state, err)
	}
	sessions := s.hubSessions()
	if sessions.opened != 1 {
		return fmt.Errorf("%s: the hub logged %q %d times for %s, want exactly 1: the device MUST keep the session it connected on",
			state, fleetLeaveHubOpened, sessions.opened, caTrustClientName)
	}
	if sessions.closed != 0 {
		return fmt.Errorf("%s: the hub logged %q %d times for %s, want 0: the device's session MUST NOT end",
			state, fleetLeaveHubClosed, sessions.closed, caTrustClientName)
	}
	return nil
}

// requireImportAnswered gives the running hub config(routerID) as the
// device's configuration with `ze data import` beside the hub, the command
// the guide names, and requires the answer to name the imported key.
func (s *fleetHoldScenario) requireImportAnswered(ctx context.Context, routerID string) error {
	path := filepath.Join(s.hub.dir, s.hubConfigName())
	if err := os.WriteFile(path, []byte(s.config(routerID)), 0o600); err != nil {
		return err
	}
	answer, err := runCommandProcess04(ctx, s.hubImporter, nil, "data", "--path", s.hub.dbPath, "import", path)
	if err != nil {
		return fmt.Errorf("give the running hub router-id %s for %s: %w", routerID, caTrustClientName, err)
	}
	imported := "imported file/active/" + s.hubConfigName()
	if !strings.Contains(answer, imported) {
		return fmt.Errorf("ze data import %s answered without %q:\n%s", s.hubConfigName(), imported, answer)
	}
	return nil
}

// requirePushApplied requires the device to report config(routerID)'s hash as
// its active and baseline hash, with no divergence, within
// fleetConfigChangeLiveBound of the import, and then the hub to log exactly
// one applied line for the client naming that hash.
func (s *fleetHoldScenario) requirePushApplied(ctx context.Context, daemon *cachedBootDaemon, routerID string) error {
	want := s.version(routerID)
	pushCtx, cancel := context.WithTimeout(ctx, fleetConfigChangeLiveBound)
	_, err := s.deviceOperator.askDaemonUntil(pushCtx, "active and baseline hash "+want+" and no divergence", func(status fleetStatusAnswer) bool {
		return status.ActiveHash == want && status.BaselineHash == want && !status.Diverged
	})
	cancel()
	if err != nil {
		return fmt.Errorf("within %s of the import the device did not run router-id %s as its baseline: "+
			"the hub MUST push config-changed on the import, and the device MUST fetch and apply it: %w",
			fleetConfigChangeLiveBound, routerID, err)
	}

	var applied int
	logged := cachedBootWait(ctx, daemon, fleetHoldLogAttempts, func() bool {
		applied = fleetConfigChangeLiveAppliedLines(s.hub.output.String(), want)
		return applied > 0
	})
	if !logged {
		return fmt.Errorf("the hub never logged %q for %s with version %s, or the device stopped",
			fleetConfigChangeLiveApplied, caTrustClientName, want)
	}
	if applied != 1 {
		return fmt.Errorf("the hub logged %q for %s with version %s %d times, want exactly 1",
			fleetConfigChangeLiveApplied, caTrustClientName, want, applied)
	}
	return nil
}

// fleetConfigChangeLiveAppliedLines counts, in a hub's output, the applied
// lines that name the scenario's client and version. A line for another
// client or another version counts as neither.
func fleetConfigChangeLiveAppliedLines(output, version string) int {
	count := 0
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, fleetConfigChangeLiveApplied) {
			continue
		}
		if !strings.Contains(line, caTrustClientName) {
			continue
		}
		if strings.Contains(line, version) {
			count++
		}
	}
	return count
}
