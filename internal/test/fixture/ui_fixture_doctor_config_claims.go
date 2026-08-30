package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const doctorConfigClaimsUnclaimed = "doctor-config-root-unclaimed"

func init() {
	Register("ui/doctor-config-claims", uiDriver(doctorConfigClaims))
}

type doctorConfigClaimsDiagnostic map[string]any

type doctorConfigClaimsCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func doctorConfigClaims(ctx context.Context) error {
	unclaimedDiagnostics, err := doctorConfigClaimsDoctor(ctx, "unclaimed.conf")
	if err != nil {
		return err
	}
	unclaimed, err := doctorConfigClaimsFilter(unclaimedDiagnostics, doctorConfigClaimsUnclaimed)
	if err != nil {
		return doctorConfigClaimsFail("inspect unclaimed.conf diagnostics: %v", err)
	}

	// Constructing the original assertion message ran doctor a second time,
	// regardless of whether the assertion passed. Keep that invocation and its
	// ordering intact.
	allUnclaimedDiagnostics, err := doctorConfigClaimsDoctor(ctx, "unclaimed.conf")
	if err != nil {
		return err
	}
	if len(unclaimed) != 1 {
		return doctorConfigClaimsFail(
			"want one %s for the pki root, got %s -- all diagnostics: %s",
			doctorConfigClaimsUnclaimed,
			doctorConfigClaimsJSON(unclaimed),
			doctorConfigClaimsJSON(allUnclaimedDiagnostics),
		)
	}

	message, err := doctorConfigClaimsField(unclaimed[0], "message")
	if err != nil {
		return doctorConfigClaimsFail("inspect %s message: %v", doctorConfigClaimsUnclaimed, err)
	}
	if !bytes.Contains([]byte(message), []byte("config under pki")) {
		return doctorConfigClaimsFail("the diagnostic must name the subtree: %s", message)
	}

	severity, err := doctorConfigClaimsField(unclaimed[0], "severity")
	if err != nil {
		return doctorConfigClaimsFail("inspect %s severity: %v", doctorConfigClaimsUnclaimed, err)
	}
	if severity != severityWarning {
		return doctorConfigClaimsFail("want a warning, got %s", severity)
	}

	// A root this build does claim stays silent, so the check is not reporting
	// every root it sees.
	claimedDiagnostics, err := doctorConfigClaimsDoctor(ctx, "claimed.conf")
	if err != nil {
		return err
	}
	claimed, err := doctorConfigClaimsFilter(claimedDiagnostics, doctorConfigClaimsUnclaimed)
	if err != nil {
		return doctorConfigClaimsFail("inspect claimed.conf diagnostics: %v", err)
	}
	if len(claimed) != 0 {
		return doctorConfigClaimsFail(
			"the static root is claimed and must not be reported: %s",
			doctorConfigClaimsJSON(claimed),
		)
	}

	// The code an operator sees is one they can look up.
	explain, err := doctorConfigClaimsRun(ctx, "ze-stripped", "explain", doctorConfigClaimsUnclaimed)
	if err != nil {
		return doctorConfigClaimsFail("run ze explain %s: %v", doctorConfigClaimsUnclaimed, err)
	}
	if explain.exitCode != 0 {
		return doctorConfigClaimsFail(
			"ze explain %s failed: %s",
			doctorConfigClaimsUnclaimed,
			explain.stderr,
		)
	}
	if !bytes.Contains([]byte(explain.stdout), []byte("Config subtree delivered to nobody")) {
		return doctorConfigClaimsFail("ze explain does not describe the code: %s", explain.stdout)
	}

	if _, err := fmt.Fprintln(os.Stdout, "OK"); err != nil {
		return doctorConfigClaimsFail("write success output: %v", err)
	}
	return nil
}

func doctorConfigClaimsDoctor(ctx context.Context, path string) ([]doctorConfigClaimsDiagnostic, error) {
	// A non-zero status is intentionally accepted: the placeholder certificate
	// also triggers doctor-pki-cert. This fixture judges the diagnostics list.
	result, err := doctorConfigClaimsRun(ctx, "ze-stripped", "doctor", "--json", path)
	if err != nil {
		return nil, doctorConfigClaimsFail("run ze doctor for %s: %v", path, err)
	}
	if len(bytes.TrimSpace([]byte(result.stdout))) == 0 {
		return nil, doctorConfigClaimsFail("ze doctor produced no output: %s", result.stderr)
	}

	var report struct {
		Diagnostics []doctorConfigClaimsDiagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &report); err != nil {
		return nil, doctorConfigClaimsFail("decode ze doctor output for %s: %v", path, err)
	}
	if report.Diagnostics == nil {
		return nil, doctorConfigClaimsFail("ze doctor output for %s has no diagnostics list", path)
	}
	return report.Diagnostics, nil
}

func doctorConfigClaimsRun(ctx context.Context, name string, args ...string) (doctorConfigClaimsCommandResult, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := doctorConfigClaimsCommandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	if cmd.ProcessState != nil {
		result.exitCode = cmd.ProcessState.ExitCode()
	}
	if err == nil {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}

	// Product exit failures are returned to the caller as data, matching
	// check-disabled command execution. Start and wait failures remain errors.
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return result, nil
	}
	return result, err
}

func doctorConfigClaimsFilter(diagnostics []doctorConfigClaimsDiagnostic, code string) ([]doctorConfigClaimsDiagnostic, error) {
	matched := make([]doctorConfigClaimsDiagnostic, 0)
	for i, diagnostic := range diagnostics {
		got, err := doctorConfigClaimsField(diagnostic, "code")
		if err != nil {
			return nil, fmt.Errorf("diagnostic %d: %w", i, err)
		}
		if got == code {
			matched = append(matched, diagnostic)
		}
	}
	return matched, nil
}

func doctorConfigClaimsField(diagnostic doctorConfigClaimsDiagnostic, name string) (string, error) {
	value, ok := diagnostic[name]
	if !ok {
		return "", fmt.Errorf("missing %q field", name)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("field %q is %T, not a string", name, value)
	}
	return text, nil
}

func doctorConfigClaimsJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("<JSON encoding failed: %v>", err)
	}
	return string(encoded)
}

func doctorConfigClaimsFail(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}
