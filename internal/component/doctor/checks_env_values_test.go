package doctor

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

// TestEnvValuesCheckReportsUnusableValue proves the runtime check turns the
// env record into a doctor row naming the key, the value, the type and the
// default, with the corrected export as its fix.
//
// VALIDATES: AC-35: env-values is registered at runtime declaring
// doctor-env-invalid-value; an int key holding text, judged though no reader
// read it after the reset, gives one warning row whose fix is a shell export.
// PREVENTS: an unparsable export reaching no doctor row.
func TestEnvValuesCheckReportsUnusableValue(t *testing.T) {
	const key = "ze.report.warnings.max"
	t.Setenv(key, "plenty")
	env.ResetCache()
	t.Cleanup(func() {
		os.Unsetenv(key) //nolint:errcheck // t.Setenv restores; the cache must forget too
		env.ResetCache()
	})

	registered := false
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == envValuesCheckName {
			registered = true
			if len(check.Codes) != 1 || check.Codes[0] != diagnostic.CodeDoctorEnvInvalidValue {
				t.Errorf("codes = %v, want [%s]", check.Codes, diagnostic.CodeDoctorEnvInvalidValue)
			}
		}
	}
	if !registered {
		t.Fatalf("%s is not registered at the runtime phase", envValuesCheckName)
	}

	var row *diagnostic.Diagnostic
	diags := checkEnvValues(diagnostic.DoctorCheckContext{})
	for i := range diags {
		if strings.Contains(diags[i].Message, key) {
			row = &diags[i]
		}
	}
	if row == nil {
		t.Fatalf("no row names %s; rows: %+v", key, diags)
	}
	if row.Code != diagnostic.CodeDoctorEnvInvalidValue || row.Severity != diagnostic.SeverityWarning {
		t.Errorf("row = %s %v, want %s warning", row.Code, row.Severity, diagnostic.CodeDoctorEnvInvalidValue)
	}
	for _, part := range []string{`"plenty"`, "not a valid int", "registered default 1024"} {
		if !strings.Contains(row.Message, part) {
			t.Errorf("message %q lacks %q", row.Message, part)
		}
	}
	if row.Fix != "export ZE_REPORT_WARNINGS_MAX=1024" {
		t.Errorf("fix = %q", row.Fix)
	}
}

// TestEnvValueDiagnosticWithholdsSecret proves a secret key's row says the
// value is withheld and carries none, and that a clamp names the maximum.
//
// VALIDATES: AC-35 survivor: IsSecret withholds the value; AC-42: the clamp
// row names the value used.
// PREVENTS: a token echoed into show doctor, which every API surface serves.
func TestEnvValueDiagnosticWithholdsSecret(t *testing.T) {
	secret := envValueDiagnostic(&env.InvalidValue{
		Key: "ze.x.token", Type: env.TypeInt, Secret: true,
		Kind: env.InvalidUnparsable, Read: true, Used: "3", Example: "3",
	})
	if !strings.Contains(secret.Message, "not shown because the key is secret") {
		t.Errorf("secret message %q does not say the value is withheld", secret.Message)
	}
	if strings.Contains(secret.Message, `"`) {
		t.Errorf("secret message %q quotes a value", secret.Message)
	}

	clamped := envValueDiagnostic(&env.InvalidValue{
		Key: "ze.report.errors.max", Type: env.TypeInt, Value: "30000",
		Kind: env.InvalidOutOfRange, Read: true, Used: "10000", Range: "1 to 10000", Example: "10000",
	})
	if !strings.Contains(clamped.Message, `"30000", outside the range 1 to 10000 its reader accepts, so ze uses 10000`) {
		t.Errorf("clamp message %q", clamped.Message)
	}
	if clamped.Fix != "export ZE_REPORT_ERRORS_MAX=10000" {
		t.Errorf("clamp fix = %q", clamped.Fix)
	}

	long := envValueDiagnostic(&env.InvalidValue{
		Key: "ze.metrics.interval", Type: env.TypeDuration, Value: strings.Repeat("x", 4096),
		Kind: env.InvalidUnparsable, Read: true, Used: "10s", Example: "10s",
	})
	if strings.Contains(long.Message, strings.Repeat("x", envValueShownMax+1)) {
		t.Errorf("a 4096-byte value was echoed past %d bytes", envValueShownMax)
	}
	if !strings.Contains(long.Message, strings.Repeat("x", envValueShownMax)+`..."`) {
		t.Errorf("message %q does not show the cut value", long.Message)
	}
}
