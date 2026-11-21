// Design: docs/architecture/fleet-config.md -- the hub's resolution of a managed client that holds
// Related: register_managed_fleet_divergence_adopt.go -- registers the two drivers
// Related: misc_fixture_managed_fleet_divergence.go -- requireDivergenceRow, requireDivergenceDetail, divergedConfig
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun, fleetHoldScenario, the hub and the holding device

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// fleetDivergenceAuditCommand lists the adopts the hub audited.
const fleetDivergenceAuditCommand = "show audit action fleet-divergence-adopt"

// fleetDivergenceAdoptCommand is the adopt of the scenario's client at
// version, as an operator types it.
func fleetDivergenceAdoptCommand(version string) string {
	return "update fleet divergence name " + caTrustClientName + " adopt version " + version
}

// managedFleetDivergenceAdoptDriver answers one question: does adopt on the
// hub end a hold by making the configuration the device runs the one its hub
// serves it, after refusing a hash the operator did not see (spec-fleet-7
// AC-11, AC-12, user story 3)?
//
// Method: the hub and the device of fleetHoldRun, the device serving SSH so
// its daemon answers `show fleet status`. The device takes the hub's
// configuration, is given another one with its daemon stopped, and is started
// again: it holds and the hub lists its copy. `ze update fleet divergence ...
// adopt` beside the hub, the local route the store's owner answers, with the
// hash of the hub's own configuration MUST be refused naming the copy's hash,
// and the copy MUST still be listed. The same adopt over the hub's SSH with
// the copy's hash MUST be answered, the device MUST record that hash as its
// baseline and stop diverging, the list MUST no longer name the client, and
// the hub MUST audit one adopt of the client.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetDivergenceAdoptDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-divergence-adopt", args, true, func(ctx context.Context, s *fleetHoldScenario) error {
		device, err := s.holdWithCopy(ctx)
		if err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			if _, err := s.requireDivergenceRow(ctx, daemon, device, s.version(restoreClientServedRouterID), "on connecting"); err != nil {
				return err
			}
			if err := s.requireStaleAdoptRefused(ctx, daemon, device); err != nil {
				return err
			}
			return s.requireAdopted(ctx, device)
		}); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: adopt on the hub ended the hold of a managed device, after refusing a stale hash, device:", device) //nolint:errcheck // fixture progress line
		return nil
	})
}

// managedFleetDivergenceHubRestartDriver answers one question: does the hub's
// copy of a held configuration survive a hub restart, so that the hub still
// lists and shows it and adopt still ends the hold (spec-fleet-7 AC-17, R-8)?
//
// Method: as managedFleetDivergenceAdoptDriver, until the hub lists the copy.
// The hub is then stopped and started again on the same store while the
// device runs. After the restart the list MUST name the same two hashes and
// the same received time (the copy came from the store, and the device, told
// that its hub holds its hash, sent no second one), the show MUST answer the
// same hashes, and adopt with the copy's hash MUST end the hold.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetDivergenceHubRestartDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-divergence-hub-restart", args, true, func(ctx context.Context, s *fleetHoldScenario) error {
		device, err := s.holdWithCopy(ctx)
		if err != nil {
			return err
		}
		served := s.version(restoreClientServedRouterID)
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			before, err := s.requireDivergenceRow(ctx, daemon, device, served, "before the hub restart")
			if err != nil {
				return err
			}
			if err := s.restartHub(ctx); err != nil {
				return err
			}
			after, err := s.requireDivergenceRow(ctx, daemon, device, served, "after the hub restart")
			if err != nil {
				return err
			}
			if after.ReceivedAt != before.ReceivedAt {
				return fmt.Errorf("after the hub restart the copy of %s was received at %q, before it at %q: "+
					"the hub MUST keep the copy in its store, and a device whose hub holds its hash MUST NOT send it again",
					caTrustClientName, after.ReceivedAt, before.ReceivedAt)
			}
			if err := s.requireDivergenceDetail(ctx, device, served); err != nil {
				return fmt.Errorf("after the hub restart: %w", err)
			}
			return s.requireAdopted(ctx, device)
		}); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: the hub kept the copy of a held configuration across its restart, and adopt ended the hold, device:", device) //nolint:errcheck // fixture progress line
		return nil
	})
}

// holdWithCopy brings the device to a hold: it takes the hub's configuration,
// then is given another one with its daemon stopped. It answers the hash of
// that configuration, the one the device sends its hub.
func (s *fleetHoldScenario) holdWithCopy(ctx context.Context) (string, error) {
	if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
		return s.requireFollows(ctx, daemon, restoreClientServedRouterID, "the first connection")
	}); err != nil {
		return "", err
	}
	diverged := s.divergedConfig(fleetHoldLocalRouterID)
	if err := s.editStoppedTo(ctx, diverged); err != nil {
		return "", err
	}
	return managedVersionHash([]byte(diverged)), nil
}

// requireStaleAdoptRefused runs the adopt with the hash of the hub's own
// configuration, which is not the copy's, with `ze update` beside the running
// hub: it MUST be refused naming the copy's hash and the show, and the copy
// MUST still be listed.
func (s *fleetHoldScenario) requireStaleAdoptRefused(ctx context.Context, daemon *cachedBootDaemon, device string) error {
	stale := s.version(restoreClientServedRouterID)
	answer, err := runCommandProcess04(ctx, s.hubImporter, nil, strings.Fields(fleetDivergenceAdoptCommand(stale))...)
	if err == nil {
		return fmt.Errorf("ze %s was accepted although the hub's copy is version %s:\n%s",
			fleetDivergenceAdoptCommand(stale), device, answer)
	}
	if !strings.Contains(answer, device) {
		return fmt.Errorf("the refusal of ze %s does not name the copy's version %s:\n%s",
			fleetDivergenceAdoptCommand(stale), device, answer)
	}
	if !strings.Contains(answer, "show fleet divergence name "+caTrustClientName) {
		return fmt.Errorf("the refusal of ze %s does not name the show as its fix:\n%s", fleetDivergenceAdoptCommand(stale), answer)
	}
	_, err = s.requireDivergenceRow(ctx, daemon, device, stale, "after the refused adopt")
	return err
}

// requireAdopted runs the adopt with the copy's hash over the hub's SSH, the
// daemon route every surface takes. The device MUST record that hash as its
// baseline and stop diverging, the list MUST no longer name the client, and
// the hub MUST audit the adopt of the client.
func (s *fleetHoldScenario) requireAdopted(ctx context.Context, device string) error {
	adopt := fleetDivergenceAdoptCommand(device)
	answer, err := storageLiveRequest(ctx, s.hubOperator, adopt)
	if err != nil {
		return fmt.Errorf("%q on the hub failed: %w\n%s", adopt, err, answer)
	}
	if _, err := s.deviceOperator.askDaemonUntil(ctx, "baseline "+device+" and no divergence", func(status fleetStatusAnswer) bool {
		return status.BaselineHash == device && !status.Diverged
	}); err != nil {
		return fmt.Errorf("after %q: %w", adopt, err)
	}

	var list string
	cleared := Poll(ctx, fleetHoldDoctorAttempts, 200*time.Millisecond, func() bool {
		list, err = storageLiveRequest(ctx, s.hubOperator, fleetDivergenceListCommand)
		if err != nil {
			return false
		}
		var rows fleetDivergenceList
		if fleetDivergenceDecode(list, &rows) != nil {
			return false
		}
		for _, row := range rows.Clients {
			if row.Client == caTrustClientName {
				return false
			}
		}
		return true
	})
	if !cleared {
		// The last poll's request error, when there is one, is why the list
		// never cleared; without one the list still named the copy or did not
		// decode.
		if err != nil {
			return fmt.Errorf("after %q the hub's copy list failed: %w:\n%s", adopt, err, list)
		}
		return fmt.Errorf("after %q the hub still lists the copy of %s, or its list did not decode:\n%s", adopt, caTrustClientName, list)
	}

	audit, err := storageLiveRequest(ctx, s.hubOperator, fleetDivergenceAuditCommand)
	if err != nil {
		return fmt.Errorf("%q on the hub failed: %w\n%s", fleetDivergenceAuditCommand, err, audit)
	}
	if !strings.Contains(audit, caTrustClientName) {
		return fmt.Errorf("%q names no adopt of %s:\n%s", fleetDivergenceAuditCommand, caTrustClientName, audit)
	}
	return nil
}

// restartHub stops the running hub and starts it again on the same store,
// then waits until its SSH answers again.
func (s *fleetHoldScenario) restartHub(ctx context.Context) error {
	stopManagedProcess(s.hub.command, s.hub.done)
	if err := caTrustStart(ctx, s.hub); err != nil {
		return err
	}
	var last string
	answered := Poll(ctx, 50, 200*time.Millisecond, func() bool {
		output, err := storageLiveRequest(ctx, s.hubOperator, "show version")
		last = output
		return err == nil
	})
	if !answered {
		return fmt.Errorf("the hub did not answer over SSH after its restart: %s\nhub output:\n%s", last, s.hub.output.String())
	}
	return nil
}
