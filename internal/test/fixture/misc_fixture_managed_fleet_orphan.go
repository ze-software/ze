// Design: docs/architecture/fleet-config.md -- the hub's copy of the configuration a client no serving block declares
// Related: register_managed_fleet_orphan.go -- registers the driver
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun, fleetHoldScenario, the hub and the holding device this driver reuses
// Related: misc_fixture_managed_fleet_divergence.go -- requireDivergenceRow, the divergence list answer and its decoder
// Related: misc_fixture_managed_fleet_doctor.go -- fleetDoctorReport and fleetDoctorDaemonReport, ze doctor against the hub

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// The texts of the hub's rows the verdicts read (internal/component/managed,
// doctor.go, diagnoseHeldClient and diagnoseHeldOrphan; internal/component/
// plugin/doctor, check_managed_runtime.go, managedRejectedRow;
// internal/component/config/storage, rename_config.go, heldRenameRefusal, and
// storage/cli, data_rpc_rename.go, the served-client refusal), the key the hub keeps the copy under (pkg/zefs KeyConfigHeld), and the names
// this scenario gives.
const (
	fleetOrphanDivergedCode = "doctor-managed-client-diverged"
	fleetOrphanDeclaredText = "the managed client " + caTrustClientName + " holds"
	fleetOrphanText         = "no plugin hub server block declares " + caTrustClientName + ", so the copy is orphaned"
	fleetOrphanFixDeclare   = "declare client " + caTrustClientName + " again"
	// What the fix says before the command it names, and the copy's key.
	fleetOrphanFixRemove = "remove the copy with the hub stopped: "
	fleetOrphanHeldKey   = "meta/config/client-" + caTrustClientName + ".conf/held"
	// The row of a connection that claimed a name no serving block declares.
	fleetOrphanUnknownText  = " is not a client the hub serves"
	fleetOrphanRenameText   = "cannot be renamed: a managed client holds a configuration of its own"
	fleetOrphanRenameTarget = caTrustClientName + "-renamed.conf"
	// The refusal of a rename of the configuration a declared client fetches,
	// which is judged before the held copy is: the client's identity.
	fleetOrphanServedRenameText = "the managed client configuration identity cannot be renamed"
	fleetOrphanServedRenameWhy  = "managed client " + caTrustClientName + " is configured to fetch "
	// The client the hub declares in place of the device's, so the managed
	// listener still serves and the device's connection still reaches it.
	fleetOrphanOtherClient = "edge-02"
)

// fleetOrphanHub stops and starts the hub of a fleetHoldScenario between the
// steps that need its store free. It is not safe for concurrent use.
type fleetOrphanHub struct {
	s *fleetHoldScenario
	// earlier is the output of the hub's runs before the current one, which
	// each start replaces.
	earlier strings.Builder
}

// managedFleetDivergenceOrphanDriver answers one question: does the hub keep
// the copy of a client that holds once no serving block declares that client,
// report it orphaned with the fix that names the way out, never serve it, and
// lose it only to the command that fix names (spec-fleet-7 AC-18, AC-19, A-8)?
//
// Method: the hub and the device of fleetHoldRun. The device takes the hub's
// configuration, is given another one with its daemon stopped, and holds: the
// hub lists its copy for a declared client, `ze doctor` on the hub reports it
// as held, not orphaned, and `ze config rename` of the client's configuration
// is refused as the identity of a client the hub serves, leaving the copy and
// the configuration in place. The hub is then stopped, given its own
// configuration with another client in place of the device's, and started.
// `ze doctor` on the hub MUST report the copy orphaned under
// doctor-managed-client-diverged with a fix that declares the client again or
// removes the copy with the hub stopped, and `show fleet divergence list` MUST
// list it undeclared, with the copy unchanged. The same rename, with no serving
// block declaring the client, MUST be refused because the hub keeps its copy,
// and leave the copy unchanged. The device, started
// again, MUST be refused as a client the hub does not serve, keep its own
// configuration, and leave the copy as it was. With the hub stopped, the
// command the fix names MUST remove the copy, and the hub started again MUST
// report and list nothing for the client.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetDivergenceOrphanDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-divergence-orphan", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		hub := &fleetOrphanHub{s: s}
		// fleetHoldRun checked the three ports before it called this.
		err := hub.run(ctx, args[0], args[1])
		if err != nil && hub.earlier.Len() > 0 {
			err = fmt.Errorf("%w\nhub output of its earlier runs:\n%s", err, hub.earlier.String())
		}
		return err
	})
}

// run is the scenario of managedFleetDivergenceOrphanDriver over the hub's SSH
// and plugin ports.
func (h *fleetOrphanHub) run(ctx context.Context, sshPort, pluginPort string) error {
	s := h.s
	device, served := s.version(fleetHoldLocalRouterID), s.version(restoreClientServedRouterID)
	if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
		return s.requireFollows(ctx, daemon, restoreClientServedRouterID, "the first connection")
	}); err != nil {
		return err
	}
	if err := s.editStopped(ctx, fleetHoldLocalRouterID); err != nil {
		return err
	}
	var first fleetDivergenceRow
	if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
		var err error
		first, err = s.requireDivergenceRow(ctx, daemon, device, served, "on connecting")
		return err
	}); err != nil {
		return err
	}
	if err := h.requireHeldDeclared(ctx, device); err != nil {
		return err
	}
	if err := h.requireRenameRefused(ctx, "while a serving block declares the client",
		fleetOrphanServedRenameText, fleetOrphanServedRenameWhy+s.hubConfigName()); err != nil {
		return err
	}
	row, err := h.requireListed(ctx, "after the refused rename of a declared client")
	if err != nil {
		return err
	}
	if row != first {
		return fmt.Errorf("the refused rename changed what the hub lists for %s: %+v, before it %+v",
			caTrustClientName, row, first)
	}

	orphanConfig, err := fleetOrphanHubConfig(sshPort, pluginPort, s.managedPort)
	if err != nil {
		return err
	}
	h.stop()
	if err := caTrustImportConfig(ctx, s.hub, caTrustHubName+".conf", orphanConfig); err != nil {
		return fmt.Errorf("give the stopped hub a configuration that declares %s in place of %s: %w",
			fleetOrphanOtherClient, caTrustClientName, err)
	}
	if err := h.start(ctx); err != nil {
		return err
	}
	removal, err := h.requireOrphanReported(ctx, device)
	if err != nil {
		return err
	}
	if err := h.requireCopyKept(ctx, first, "once no serving block declares the client"); err != nil {
		return err
	}
	// No served-client rule refuses the rename now, so the held copy is the
	// reason it is refused.
	if err := h.requireRenameRefused(ctx, "once no serving block declares the client",
		fleetOrphanRenameText, fleetOrphanHeldKey); err != nil {
		return err
	}
	if err := h.requireCopyKept(ctx, first, "after the refused rename of an undeclared client"); err != nil {
		return err
	}
	if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
		return h.requireNeverServed(ctx, daemon)
	}); err != nil {
		return err
	}
	if err := h.requireCopyKept(ctx, first, "after the device's refused connection"); err != nil {
		return err
	}

	h.stop()
	if err := h.removeWithFix(ctx, removal); err != nil {
		return err
	}
	if err := h.start(ctx); err != nil {
		return err
	}
	if err := h.requireNothingHeld(ctx); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "OK: the hub kept the copy of a client no serving block declares, reported it orphaned, "+ //nolint:errcheck // fixture progress line
		"never served it, and lost it only to the command its fix names, device:", device)
	return nil
}

// fleetOrphanHubConfig is the hub's configuration of fleetHoldRun with
// fleetOrphanOtherClient declared in place of the device's client.
func fleetOrphanHubConfig(sshPort, pluginPort, managedPort string) (string, error) {
	declared := restoreClientHubConfig(sshPort, pluginPort, managedPort)
	entry := "client " + caTrustClientName + " {"
	if count := strings.Count(declared, entry); count != 1 {
		return "", fmt.Errorf("the hub's configuration declares %q %d times, want once:\n%s", entry, count, declared)
	}
	return strings.Replace(declared, entry, "client "+fleetOrphanOtherClient+" {", 1), nil
}

// stop stops the hub. The command is replaced by one with no process, so the
// stop fleetHoldRun defers returns at once when the scenario ends with the hub
// stopped.
func (h *fleetOrphanHub) stop() {
	stopManagedProcess(h.s.hub.command, h.s.hub.done)
	h.s.hub.command = &exec.Cmd{}
}

// start starts the stopped hub on its store and waits until its SSH answers.
// The output of the run before it is kept in earlier.
func (h *fleetOrphanHub) start(ctx context.Context) error {
	if h.s.hub.output != nil {
		h.earlier.WriteString(h.s.hub.output.String())
	}
	if err := caTrustStart(ctx, h.s.hub); err != nil {
		return err
	}
	exited := false
	var answer string
	answered := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
		select {
		case reaped := <-h.s.hub.done:
			h.s.hub.done <- reaped
			exited = true
			return true
		default:
		}
		var err error
		answer, err = storageLiveRequest(ctx, h.s.hubOperator, "show version")
		return err == nil
	})
	if exited {
		return fmt.Errorf("the hub stopped as it started again:\n%s", h.s.hub.output.String())
	}
	if !answered {
		return fmt.Errorf("the hub started again never answered over SSH, the last answer:\n%s", answer)
	}
	return nil
}

// requireHeldDeclared requires `ze doctor` on the running hub to report the
// copy of the declared client as held, at deviceVersion, and not as orphaned:
// the orphan wording is tied to a client no serving block declares.
func (h *fleetOrphanHub) requireHeldDeclared(ctx context.Context, deviceVersion string) error {
	row, err := h.divergedRow(ctx, "while a serving block declares the client")
	if err != nil {
		return err
	}
	message, err := doctorConfigClaimsField(row, fieldMessage)
	if err != nil {
		return err
	}
	if !strings.Contains(message, fleetOrphanDeclaredText) {
		return fmt.Errorf("the hub's %s row for a declared client lacks %q: %s", fleetOrphanDivergedCode, fleetOrphanDeclaredText, message)
	}
	if !strings.Contains(message, deviceVersion) {
		return fmt.Errorf("the hub's %s row for a declared client lacks the version %s: %s", fleetOrphanDivergedCode, deviceVersion, message)
	}
	if strings.Contains(message, fleetOrphanText) {
		return fmt.Errorf("the hub reports the copy of a declared client as orphaned: %s", message)
	}
	return nil
}

// requireRenameRefused requires `ze config rename` of the client's
// configuration, beside the running hub, to be refused, in state, with a
// refusal that carries every text of wants. The caller requires the copy and
// the configuration to be left as they were.
func (h *fleetOrphanHub) requireRenameRefused(ctx context.Context, state string, wants ...string) error {
	s := h.s
	stdout, stderr, code, err := rawCommandStreams(ctx, s.hub.dir, s.hubImporter,
		"ze", "config", "rename", s.hubConfigName(), fleetOrphanRenameTarget)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("%s: ze config rename %s %s was not refused while the hub keeps the copy of %s:\n%s%s",
			state, s.hubConfigName(), fleetOrphanRenameTarget, caTrustClientName, stdout, stderr)
	}
	for _, want := range wants {
		if !strings.Contains(stderr, want) {
			return fmt.Errorf("%s: ze config rename %s was refused without %q:\n%s%s", state, s.hubConfigName(), want, stdout, stderr)
		}
	}
	return nil
}

// requireOrphanReported requires `ze doctor` on the running hub to report the
// copy of the undeclared client as orphaned, at deviceVersion, with a fix that
// declares the client again or removes the copy with the hub stopped, and
// answers the removal command that fix names.
func (h *fleetOrphanHub) requireOrphanReported(ctx context.Context, deviceVersion string) (string, error) {
	row, err := h.divergedRow(ctx, "once no serving block declares the client")
	if err != nil {
		return "", err
	}
	const fieldFix = "fix"
	wants := []struct{ field, text string }{
		{fieldSeverity, severityWarning},
		{fieldMessage, fleetOrphanText},
		{fieldMessage, "version " + deviceVersion},
		{fieldFix, fleetOrphanFixDeclare},
		{fieldFix, fleetOrphanFixRemove + "ze data rm " + fleetOrphanHeldKey},
	}
	for _, want := range wants {
		got, fieldErr := doctorConfigClaimsField(row, want.field)
		if fieldErr != nil {
			return "", fmt.Errorf("the hub's %s row: %w", fleetOrphanDivergedCode, fieldErr)
		}
		if !strings.Contains(got, want.text) {
			return "", fmt.Errorf("the %s of the hub's %s row lacks %q: %s", want.field, fleetOrphanDivergedCode, want.text, got)
		}
	}
	fix, err := doctorConfigClaimsField(row, fieldFix)
	if err != nil {
		return "", err
	}
	_, removal, _ := strings.Cut(fix, fleetOrphanFixRemove)
	return removal, nil
}

// divergedRow answers the one doctor-managed-client-diverged row of the
// running hub's `ze doctor` report.
func (h *fleetOrphanHub) divergedRow(ctx context.Context, state string) (doctorConfigClaimsDiagnostic, error) {
	report, err := fleetDoctorDaemonReport(ctx, h.s.hubImporter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", state, err)
	}
	rows, err := doctorConfigClaimsFilter(report, fleetOrphanDivergedCode)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", state, err)
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("%s: want one %s row on the hub, got %d: %s",
			state, fleetOrphanDivergedCode, len(rows), doctorConfigClaimsJSON(report))
	}
	return rows[0], nil
}

// requireCopyKept requires `show fleet divergence list` on the running hub to
// list the client undeclared, with the copy first listed: the same device
// version and the same time it was received.
func (h *fleetOrphanHub) requireCopyKept(ctx context.Context, first fleetDivergenceRow, state string) error {
	row, err := h.requireListed(ctx, state)
	if err != nil {
		return err
	}
	if row.Declared {
		return fmt.Errorf("%s: %q lists %s as declared by a server block: %+v",
			state, fleetDivergenceListCommand, caTrustClientName, row)
	}
	// The client's configuration stays on the hub, so only the declaration
	// differs from the row first listed: the same copy, the same hub version.
	kept := first
	kept.Declared = false
	if row != kept {
		return fmt.Errorf("%s: the hub's copy of %s changed: %+v, first %+v", state, caTrustClientName, row, first)
	}
	return nil
}

// requireListed waits until `show fleet divergence list` on the running hub
// lists the client and answers its row.
func (h *fleetOrphanHub) requireListed(ctx context.Context, state string) (fleetDivergenceRow, error) {
	var row fleetDivergenceRow
	var answer string
	listed := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
		var found bool
		row, found, answer = h.listRow(ctx)
		return found
	})
	if !listed {
		return row, fmt.Errorf("%s: %q on the hub never listed %s, the last answer:\n%s",
			state, fleetDivergenceListCommand, caTrustClientName, answer)
	}
	return row, nil
}

// listRow asks `show fleet divergence list` once and answers the client's
// row, whether the answer listed it, and the answer. An answer that is no
// list does not list it.
func (h *fleetOrphanHub) listRow(ctx context.Context) (fleetDivergenceRow, bool, string) {
	answer, _ := storageLiveRequest(ctx, h.s.hubOperator, fleetDivergenceListCommand) //nolint:errcheck // an answer that is no list lists nothing; the caller polls
	var list fleetDivergenceList
	if fleetDivergenceDecode(answer, &list) != nil {
		return fleetDivergenceRow{}, false, answer
	}
	row, found := fleetOrphanRowOf(list)
	return row, found, answer
}

// fleetOrphanRowOf answers the client's row of list, and whether list has one.
func fleetOrphanRowOf(list fleetDivergenceList) (fleetDivergenceRow, bool) {
	for _, row := range list.Clients {
		if row.Client == caTrustClientName {
			return row, true
		}
	}
	return fleetDivergenceRow{}, false
}

// requireNeverServed is the device running while no serving block declares
// it: the hub MUST refuse its connection as a client it does not serve, and
// the device MUST still run its own configuration.
func (h *fleetOrphanHub) requireNeverServed(ctx context.Context, daemon *cachedBootDaemon) error {
	want := "managed client " + strconv.Quote(caTrustClientName) + fleetOrphanUnknownText
	var last []doctorConfigClaimsDiagnostic
	refused := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
		report, err := fleetDoctorReport(ctx, h.s.hubImporter)
		if err != nil {
			return false
		}
		last = report
		rows, err := doctorConfigClaimsFilter(report, fleetHoldRejectedCode)
		if err != nil {
			return false
		}
		for _, row := range rows {
			message, fieldErr := doctorConfigClaimsField(row, fieldMessage)
			if fieldErr != nil {
				continue // A row with no message is not the refusal looked for.
			}
			if strings.Contains(message, want) {
				return true
			}
		}
		return false
	})
	if !refused {
		return fmt.Errorf("the hub never reported the device's connection refused as %q, or the device stopped; the last report: %s",
			want, doctorConfigClaimsJSON(last))
	}
	active, err := caTrustActiveConfig(ctx, h.s.device)
	if err != nil {
		return err
	}
	if !strings.Contains(active, "router-id "+fleetHoldLocalRouterID) {
		return fmt.Errorf("the device the hub does not serve no longer runs its own configuration (router-id %s):\n%s",
			fleetHoldLocalRouterID, active)
	}
	return nil
}

// removeWithFix runs removal, the command the orphan row's fix names, beside
// the stopped hub, as an operator types it, and requires the copy gone from
// the hub's store.
func (h *fleetOrphanHub) removeWithFix(ctx context.Context, removal string) error {
	words := strings.Fields(removal)
	if len(words) < 2 {
		return fmt.Errorf("the orphan row's fix names no command after %q: %q", fleetOrphanFixRemove, removal)
	}
	if words[0] != "ze" {
		return fmt.Errorf("the orphan row's fix names %q after %q, not a ze command", removal, fleetOrphanFixRemove)
	}
	hub := h.s.hub
	stdout, stderr, code, err := rawCommandStreams(ctx, hub.dir, hub.env, words[0], words[1:]...)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("%q, the command the orphan row's fix names, exited %d with the hub stopped:\n%s%s",
			removal, code, stdout, stderr)
	}
	stdout, stderr, code, err = rawCommandStreams(ctx, hub.dir, hub.env, "ze", "data", "--path", hub.dbPath, "cat", fleetOrphanHeldKey)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("%q exited 0 and the hub's store still holds %s:\n%s%s", removal, fleetOrphanHeldKey, stdout, stderr)
	}
	return nil
}

// requireNothingHeld requires the hub started after the removal to report no
// doctor-managed-client-diverged row and to list no copy of the client. The
// report is the running hub's, and the list answer is a list, so neither
// silence is a failure to look.
func (h *fleetOrphanHub) requireNothingHeld(ctx context.Context) error {
	report, err := fleetDoctorDaemonReport(ctx, h.s.hubImporter)
	if err != nil {
		return fmt.Errorf("after the removal: %w", err)
	}
	rows, err := doctorConfigClaimsFilter(report, fleetOrphanDivergedCode)
	if err != nil {
		return fmt.Errorf("after the removal: %w", err)
	}
	if len(rows) != 0 {
		return fmt.Errorf("after the removal the hub still reports %s: %s", fleetOrphanDivergedCode, doctorConfigClaimsJSON(rows))
	}
	var answer string
	var listed bool
	decoded := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
		answer, _ = storageLiveRequest(ctx, h.s.hubOperator, fleetDivergenceListCommand) //nolint:errcheck // an answer that is no list is "not yet"
		var list fleetDivergenceList
		if fleetDivergenceDecode(answer, &list) != nil {
			return false
		}
		_, listed = fleetOrphanRowOf(list)
		return true
	})
	if !decoded {
		return errors.New("after the removal " + strconv.Quote(fleetDivergenceListCommand) + " on the hub never answered a list:\n" + answer)
	}
	if listed {
		return fmt.Errorf("after the removal %q still lists %s:\n%s", fleetDivergenceListCommand, caTrustClientName, answer)
	}
	return nil
}
