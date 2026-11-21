// Design: docs/architecture/fleet-config.md -- the hub's resolution of a managed client that holds
// Related: register_managed_fleet_divergence_revert.go -- registers the two drivers
// Related: misc_fixture_managed_fleet_divergence_adopt.go -- holdWithCopy, the adopt drivers these copy
// Related: misc_fixture_managed_fleet_divergence.go -- requireDivergenceRow, divergedConfig, the list and show commands
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun, fleetHoldScenario, the hub and the holding device

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// The audit query of the reverts the hub recorded, and the revert states the
// show answers (internal/component/managed, cmd_divergence_show.go).
const (
	fleetRevertAuditCommand = "show audit action fleet-divergence-revert"
	fleetRevertNone         = "none"
	fleetRevertPending      = "pending"
	fleetRevertVoid         = "void"
	// fleetRevertNewerRouterID is the router-id of the device's second local
	// edit, made after the hub's operator decided a revert of the first.
	fleetRevertNewerRouterID = "5.5.5.5"
)

// fleetRevertDetail is the part of `show fleet divergence name <client>` the
// revert verdicts read.
type fleetRevertDetail struct {
	Client        string `json:"client"`
	DeviceVersion string `json:"device-version"`
	HubVersion    string `json:"hub-version"`
	Revert        string `json:"revert"`
	RevertVoid    string `json:"revert-void"`
}

// fleetDivergenceRevertCommand is the revert of the scenario's client at
// device hash version and hub hash hub, as an operator types it from the show.
func fleetDivergenceRevertCommand(version, hub string) string {
	return "request fleet divergence name " + caTrustClientName + " revert version " + version + " hub-version " + hub
}

// fleetRevertHubWrittenRefusal is what the hub's refusal of a revert says
// when the hub hash it names is no longer what the hub serves the client
// (cmd_divergence.go, revertHeldLocked).
const fleetRevertHubWrittenRefusal = "it was written after that version was shown, so no decision was stored"

// managedFleetDivergenceRevertDriver answers one question: does revert on the
// hub make a holding device drop the configuration it runs and take its
// hub's, through the push path, whether the device is connected when the
// revert is decided or connects later (spec-fleet-7 AC-13, AC-14, user story
// 4)?
//
// Method: the hub and the device of fleetHoldRun, the device serving SSH so
// its daemon answers `show fleet status`. The device takes the hub's
// configuration, is given another one with its daemon stopped, and is started
// again: it holds and the hub lists its copy. The revert over the hub's SSH
// with the copy's hash MUST be answered, the device MUST run the hub's
// configuration with its hash as the baseline, the list MUST no longer name
// the client, and the hub MUST audit the revert of the client. The device is
// then given a second configuration with its daemon stopped, started until
// the hub lists the copy, and stopped: the revert is decided while it is not
// connected, and the show MUST report it pending. Started again, the device
// MUST take the hub's configuration at its first fetch.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetDivergenceRevertDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-divergence-revert", args, true, func(ctx context.Context, s *fleetHoldScenario) error {
		served := s.version(restoreClientServedRouterID)
		device, err := s.holdWithCopy(ctx)
		if err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			if _, err := s.requireDivergenceRow(ctx, daemon, device, served, "on connecting"); err != nil {
				return err
			}
			if err := s.requireReverted(ctx, device, served); err != nil {
				return err
			}
			if err := s.requireFollows(ctx, daemon, restoreClientServedRouterID, "after the revert"); err != nil {
				return err
			}
			return s.requireRevertCleared(ctx, "after the revert")
		}); err != nil {
			return err
		}
		if err := s.requireRevertAudited(ctx); err != nil {
			return err
		}

		// The same device, offline when the revert is decided (AC-14).
		offline := s.divergedConfig(fleetRevertNewerRouterID)
		if err := s.editStoppedTo(ctx, offline); err != nil {
			return err
		}
		again := managedVersionHash([]byte(offline))
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			_, err := s.requireDivergenceRow(ctx, daemon, again, served, "on connecting with a second local edit")
			return err
		}); err != nil {
			return err
		}
		if err := s.requireReverted(ctx, again, served); err != nil {
			return err
		}
		if err := s.requireRevertState(ctx, again, served, fleetRevertPending, "with the device not connected"); err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			if err := s.requireFollows(ctx, daemon, restoreClientServedRouterID, "after a revert decided while the device was away"); err != nil {
				return err
			}
			return s.requireRevertCleared(ctx, "after the offline revert")
		}); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: revert on the hub made a holding device take its hub's configuration, connected and offline, device:", device, again) //nolint:errcheck // fixture progress line
		return nil
	})
}

// managedFleetDivergenceRevertVoidDriver answers one question: does a revert
// decision that went stale do nothing, so that a newer local edit is kept and
// a hub configuration the operator did not see is not applied (spec-fleet-7
// AC-15, AC-16, user story 5)?
//
// Method: as managedFleetDivergenceRevertDriver, until the hub lists the copy
// D; the device is stopped and the revert of D is decided. The device is then
// given a newer configuration D2 with its daemon stopped, and started: it
// MUST keep D2 and hold, the hub MUST list D2, and the show MUST report no
// decision, because the newer copy replaced the one it was taken on. The
// device is stopped, the revert of D2 is decided, and the hub's configuration
// for the client is written again (V2). A revert naming V, the hub hash shown
// before, MUST be refused (E-13), and the show MUST report the decision void
// with the revert as its fix. Started again, the device MUST fetch, be
// told the hub holds D2, offered no revert, and keep running D2.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetDivergenceRevertVoidDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-divergence-revert-void", args, true, func(ctx context.Context, s *fleetHoldScenario) error {
		served := s.version(restoreClientServedRouterID)
		device, err := s.holdWithCopy(ctx)
		if err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			_, err := s.requireDivergenceRow(ctx, daemon, device, served, "on connecting")
			return err
		}); err != nil {
			return err
		}
		if err := s.requireReverted(ctx, device, served); err != nil {
			return err
		}

		// A newer local edit wins over the decision taken on the older one.
		newer := s.divergedConfig(fleetRevertNewerRouterID)
		if err := s.editStoppedTo(ctx, newer); err != nil {
			return err
		}
		newerVersion := managedVersionHash([]byte(newer))
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			if _, err := s.requireDivergenceRow(ctx, daemon, newerVersion, served, "after a local edit newer than the decision"); err != nil {
				return err
			}
			if err := s.requireRevertState(ctx, newerVersion, served, fleetRevertNone, "after a local edit newer than the decision"); err != nil {
				return err
			}
			return s.requireKeeps(ctx, newerVersion, device, "after a local edit newer than the decision")
		}); err != nil {
			return err
		}

		// A hub write after the decision voids it.
		if err := s.requireReverted(ctx, newerVersion, served); err != nil {
			return err
		}
		if err := s.hubImport(ctx, fleetHoldFurtherRouterID); err != nil {
			return err
		}
		written := s.version(fleetHoldFurtherRouterID)
		// The operator's hub hash is now stale: the revert is refused and
		// stores nothing, so the decision stays the void one (E-13).
		if err := s.requireRevertRefused(ctx, newerVersion, served, fleetRevertHubWrittenRefusal); err != nil {
			return err
		}
		if err := s.requireRevertState(ctx, newerVersion, written, fleetRevertVoid, "after a hub write newer than the decision"); err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireKeeps(ctx, newerVersion, newerVersion, "after a hub write newer than the decision")
		}); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: a revert decision went void on a newer local edit and on a newer hub write, device:", newerVersion) //nolint:errcheck // fixture progress line
		return nil
	})
}

// requireReverted runs the revert of the copy whose hash is device, to the hub
// configuration whose hash is hub, over the hub's SSH, the daemon route every
// surface takes: it MUST be answered.
func (s *fleetHoldScenario) requireReverted(ctx context.Context, device, hub string) error {
	revert := fleetDivergenceRevertCommand(device, hub)
	answer, err := storageLiveRequest(ctx, s.hubOperator, revert)
	if err != nil {
		return fmt.Errorf("%q on the hub failed: %w\n%s", revert, err, answer)
	}
	if !strings.Contains(answer, device) {
		return fmt.Errorf("%q on the hub answered no device version %s:\n%s", revert, device, answer)
	}
	return nil
}

// requireRevertRefused runs the revert of device to hub over the hub's SSH:
// it MUST be refused, saying why.
func (s *fleetHoldScenario) requireRevertRefused(ctx context.Context, device, hub, why string) error {
	return storageLiveRefused(ctx, s.hubOperator, fleetDivergenceRevertCommand(device, hub), why)
}

// requireRevertCleared waits until the hub's list no longer names the client:
// the device's ok:true ack removed the copy.
func (s *fleetHoldScenario) requireRevertCleared(ctx context.Context, state string) error {
	var list string
	var err error
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
			return fmt.Errorf("%s: the hub's copy list failed: %w:\n%s", state, err, list)
		}
		return fmt.Errorf("%s: the hub still lists the copy of %s, or its list did not decode:\n%s", state, caTrustClientName, list)
	}
	return nil
}

// requireRevertAudited requires the hub's audit log to name a revert of the
// client.
func (s *fleetHoldScenario) requireRevertAudited(ctx context.Context) error {
	audit, err := storageLiveRequest(ctx, s.hubOperator, fleetRevertAuditCommand)
	if err != nil {
		return fmt.Errorf("%q on the hub failed: %w\n%s", fleetRevertAuditCommand, err, audit)
	}
	if !strings.Contains(audit, caTrustClientName) {
		return fmt.Errorf("%q names no revert of %s:\n%s", fleetRevertAuditCommand, caTrustClientName, audit)
	}
	return nil
}

// requireRevertState requires the show of the client to answer both hashes
// and the revert state want; a void state MUST carry its reason, ending with
// the revert of the copy over the two hashes the show answers as its fix.
func (s *fleetHoldScenario) requireRevertState(ctx context.Context, device, hub, want, state string) error {
	answer, err := storageLiveRequest(ctx, s.hubOperator, fleetDivergenceShowCommand)
	if err != nil {
		return fmt.Errorf("%s: %q on the hub failed: %w\n%s", state, fleetDivergenceShowCommand, err, answer)
	}
	var detail fleetRevertDetail
	if err := fleetDivergenceDecode(answer, &detail); err != nil {
		return fmt.Errorf("%s: %q answered no detail: %w\n%s", state, fleetDivergenceShowCommand, err, answer)
	}
	if detail.DeviceVersion != device || detail.HubVersion != hub || detail.Revert != want {
		return fmt.Errorf("%s: %q answered device %s, hub %s, revert %q; want %s, %s, %q:\n%s",
			state, fleetDivergenceShowCommand, detail.DeviceVersion, detail.HubVersion, detail.Revert, device, hub, want, answer)
	}
	if want != fleetRevertVoid {
		return nil
	}
	if !strings.HasSuffix(strings.TrimSpace(detail.RevertVoid), fleetDivergenceRevertCommand(device, hub)) {
		return fmt.Errorf("%s: the void revert's reason does not end with the revert as its fix:\n%s", state, detail.RevertVoid)
	}
	return nil
}

// requireKeeps requires the running device to have fetched from its hub, been
// told the hub holds the copy heldBy, been offered no revert, and to still
// run version with it differing from its baseline: it holds. heldBy is the
// copy the hub held when the device fetched, which the copy of version
// replaces only after that answer.
func (s *fleetHoldScenario) requireKeeps(ctx context.Context, version, heldBy, state string) error {
	status, err := s.deviceOperator.askDaemonUntil(ctx, "held by the hub as "+heldBy+" and no revert offered", func(status fleetStatusAnswer) bool {
		return status.HeldByHub == heldBy && status.RevertOffered == fleetRevertNone
	})
	if err != nil {
		return fmt.Errorf("%s: %w", state, err)
	}
	if status.ActiveHash != version || !status.Diverged {
		return fmt.Errorf("%s: the device runs %s (diverged %t), want %s and diverged: a stale revert changed it",
			state, status.ActiveHash, status.Diverged, version)
	}
	return nil
}
