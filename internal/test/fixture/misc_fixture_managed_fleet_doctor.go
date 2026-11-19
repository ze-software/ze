// Design: docs/architecture/doctor-and-health-checks.md -- the managed-diverged check through ze doctor, daemon stopped and running
// Related: register_managed_fleet_doctor.go -- registers the driver
// Related: misc_fixture_managed_fleet_status.go -- fleetStatusSeed and fleetStatusScenario, the device this driver seeds and starts
// Related: ui_fixture_doctor_config_claims.go -- doctorConfigClaimsFilter and doctorConfigClaimsField, the reads of a doctor report

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

// The codes the verdicts read in a `ze doctor --json` report, and the texts of
// the managed-diverged row (internal/component/managed, doctor.go).
const (
	fleetDoctorDivergedCode = "doctor-managed-diverged"
	// The row the local route adds when no daemon answered
	// (internal/component/doctor, local_route.go). Its absence means the report
	// is a running daemon's.
	fleetDoctorNotCheckedCode = "doctor-runtime-not-checked"
	// The row of the hub-unreachable check of the same component, which reads
	// the configuration tree. The device's hub is a port nothing listens on, so
	// a report that carries this row read this device's configuration.
	fleetDoctorHubUnreachableCode = "doctor-hub-unreachable"
	fleetDoctorDivergedMessage    = "differs from the last one its hub sent"
	// The two ways out that the fix names: one on the hub, one on the device.
	fleetDoctorFixKeep    = "ze data import client-"
	fleetDoctorFixDiscard = "request fleet disable"
)

// managedFleetDivergedDoctorDriver answers one question: does `ze doctor`
// report a managed device whose configuration is not the last one its hub
// sent, with the daemon stopped and with it running, and say nothing of the
// same difference on a device that is not managed (spec-fleet-6 AC-8)?
//
// Method: `ze init --managed` and an imported config, as the fleet-status
// driver seeds them. `ze doctor --json` runs as the operator's own process in
// four states: managed with no baseline (silent: a device with no baseline is
// clean), managed with a baseline no configuration hashes to and the daemon
// stopped, the same with the daemon running and answering, and the same store
// with the managed flag set to false. Each silent state MUST still carry the
// hub-unreachable row, so the report is known to be this device's.
func managedFleetDivergedDoctorDriver(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("managed/fleet-diverged-doctor", flag.ContinueOnError)
	pluginPort := flags.Int("plugin-port", 0, "the device's own plugin transport port")
	hubPort := flags.Int("hub-port", 0, "the hub port the config names, which nothing listens on")
	sshPort := flags.Int("ssh-port", 0, "the port the daemon's SSH listener takes, which `ze doctor` dials")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *pluginPort == 0 {
		return errors.New("plugin-port is required")
	}
	if *hubPort == 0 {
		return errors.New("hub-port is required")
	}
	if *sshPort == 0 {
		return errors.New("ssh-port is required")
	}

	scenario, err := fleetStatusSeed(ctx, "managed-fleet-doctor-", *pluginPort, *hubPort, *sshPort)
	if err != nil {
		return err
	}
	defer os.RemoveAll(scenario.dir) //nolint:errcheck // fixture cleanup

	// Managed, no baseline: nothing to compare with, so nothing to report.
	if err := fleetDoctorRequireSilent(ctx, scenario.env, "managed, no baseline, daemon stopped", fleetDoctorHubUnreachableCode); err != nil {
		return err
	}

	if err := scenario.writeKey(ctx, fleetStatusKeyBaseline, fleetStatusOtherBaseline); err != nil {
		return err
	}
	stopped, err := fleetDoctorReport(ctx, scenario.env)
	if err != nil {
		return err
	}
	local, err := fleetDoctorCount(stopped, fleetDoctorNotCheckedCode)
	if err != nil {
		return err
	}
	if local == 0 {
		return fmt.Errorf("managed, diverged, daemon stopped: the report lacks %s, so it is not known to be the local route's: %s",
			fleetDoctorNotCheckedCode, doctorConfigClaimsJSON(stopped))
	}
	if err := fleetDoctorJudgeDiverged(stopped, "managed, diverged, daemon stopped", fleetStatusOtherBaseline); err != nil {
		return err
	}

	if err := scenario.withDaemon(ctx, func() error {
		running, err := fleetDoctorDaemonReport(ctx, scenario.env)
		if err != nil {
			return err
		}
		return fleetDoctorJudgeDiverged(running, "managed, diverged, daemon running", fleetStatusOtherBaseline)
	}); err != nil {
		return err
	}

	// The same store, the same difference, and no fleet: nothing to report.
	if err := scenario.writeKey(ctx, fleetStatusKeyManaged, "false"); err != nil {
		return err
	}
	if err := fleetDoctorRequireSilent(ctx, scenario.env, "not managed, same difference, daemon stopped", fleetDoctorHubUnreachableCode); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "OK: ze doctor reported doctor-managed-diverged with its fix on a managed device, daemon stopped and running, "+ //nolint:errcheck // fixture progress line
		"and nothing on a device with no baseline or one that is not managed")
	return nil
}

// fleetDoctorReport runs `ze doctor --json` once with env and answers the
// diagnostics of its report. The exit code is not judged: a report that
// carries an error exits non-zero, and the rows are what the callers read.
func fleetDoctorReport(ctx context.Context, env []string) ([]doctorConfigClaimsDiagnostic, error) {
	_, stdout, stderr, err := runCaptured(ctx, env, "", "ze", "doctor", "--json")
	if err != nil {
		return nil, fmt.Errorf("run ze doctor --json: %w", err)
	}
	var report struct {
		Diagnostics []doctorConfigClaimsDiagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		return nil, fmt.Errorf("ze doctor --json did not answer a report: %w:\n%s%s", err, stdout, stderr)
	}
	if report.Diagnostics == nil {
		return nil, fmt.Errorf("the ze doctor --json report has no diagnostics list:\n%s", stdout)
	}
	return report.Diagnostics, nil
}

// fleetDoctorDaemonReport asks until the report is the running daemon's: one
// without the row the local route adds when no daemon answered. The ready file
// does not order the SSH listener, so a local report is "not yet", within a
// bounded number of attempts.
func fleetDoctorDaemonReport(ctx context.Context, env []string) ([]doctorConfigClaimsDiagnostic, error) {
	var last []doctorConfigClaimsDiagnostic
	var lastErr error
	answered := Poll(ctx, fleetStatusAnswerAttempts, fleetStatusPollDelay, func() bool {
		last, lastErr = fleetDoctorReport(ctx, env)
		if lastErr != nil {
			return false
		}
		var local int
		local, lastErr = fleetDoctorCount(last, fleetDoctorNotCheckedCode)
		if lastErr != nil {
			return false
		}
		return local == 0
	})
	if lastErr != nil {
		return nil, lastErr
	}
	if !answered {
		return nil, fmt.Errorf("the running daemon never answered ze doctor: the last report is the local route's: %s", doctorConfigClaimsJSON(last))
	}
	return last, nil
}

// fleetDoctorCount counts the rows of report that carry code. A report with a
// row that has no code cannot be counted, which is an error and never zero.
func fleetDoctorCount(report []doctorConfigClaimsDiagnostic, code string) (int, error) {
	matched, err := doctorConfigClaimsFilter(report, code)
	if err != nil {
		return 0, fmt.Errorf("count %s rows: %w: %s", code, err, doctorConfigClaimsJSON(report))
	}
	return len(matched), nil
}

// fleetDoctorRequireDiverged runs `ze doctor --json` with env and judges its
// report with fleetDoctorJudgeDiverged.
func fleetDoctorRequireDiverged(ctx context.Context, env []string, state, baseline string) error {
	report, err := fleetDoctorReport(ctx, env)
	if err != nil {
		return err
	}
	return fleetDoctorJudgeDiverged(report, state, baseline)
}

// fleetDoctorJudgeDiverged requires report to carry exactly one
// doctor-managed-diverged row: a warning that says the configuration differs
// from the last one the hub sent, names baseline, and gives both ways out.
func fleetDoctorJudgeDiverged(report []doctorConfigClaimsDiagnostic, state, baseline string) error {
	rows, err := doctorConfigClaimsFilter(report, fleetDoctorDivergedCode)
	if err != nil {
		return fmt.Errorf("%s: %w", state, err)
	}
	if len(rows) != 1 {
		return fmt.Errorf("%s: want one %s row, got %d: %s", state, fleetDoctorDivergedCode, len(rows), doctorConfigClaimsJSON(report))
	}
	const fieldFix = "fix"
	wants := []struct{ field, text string }{
		{"severity", severityWarning},
		{fieldMessage, fleetDoctorDivergedMessage},
		{fieldMessage, baseline},
		{fieldFix, fleetDoctorFixKeep},
		{fieldFix, fleetDoctorFixDiscard},
	}
	for _, want := range wants {
		got, fieldErr := doctorConfigClaimsField(rows[0], want.field)
		if fieldErr != nil {
			return fmt.Errorf("%s: the %s row: %w", state, fleetDoctorDivergedCode, fieldErr)
		}
		if !strings.Contains(got, want.text) {
			return fmt.Errorf("%s: the %s of the %s row lacks %q: %s", state, want.field, fleetDoctorDivergedCode, want.text, got)
		}
	}
	return nil
}

// fleetDoctorRequireSilent runs `ze doctor --json` with env and requires a
// report with no doctor-managed-diverged row. When control names a code, the
// report MUST carry a row of it: without one, a doctor that read no
// configuration of this device would pass. A caller that proved the row
// present on the same store before passes no control.
func fleetDoctorRequireSilent(ctx context.Context, env []string, state, control string) error {
	report, err := fleetDoctorReport(ctx, env)
	if err != nil {
		return err
	}
	if control != "" {
		controls, countErr := fleetDoctorCount(report, control)
		if countErr != nil {
			return countErr
		}
		if controls == 0 {
			return fmt.Errorf("%s: the report lacks %s, so it is not known to have read this device's configuration: %s",
				state, control, doctorConfigClaimsJSON(report))
		}
	}
	diverged, err := fleetDoctorCount(report, fleetDoctorDivergedCode)
	if err != nil {
		return err
	}
	if diverged != 0 {
		return fmt.Errorf("%s: want no %s row, got %d: %s", state, fleetDoctorDivergedCode, diverged, doctorConfigClaimsJSON(report))
	}
	return nil
}
