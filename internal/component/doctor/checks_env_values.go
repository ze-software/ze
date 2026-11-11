// Design: docs/architecture/doctor-and-health-checks.md -- environment values ze could not use
// Related: register.go -- the init() that registers envValuesCheck
// Related: registry.go -- RunRuntimeChecks, the only runner of the runtime phase
//
// A typed env getter that cannot parse a value answers its caller's default,
// and the report bus clamps a cap set above its maximum; either way ze runs on
// a value the operator did not export. internal/core/env records both and
// judges every registered entry against its type (env.InvalidValues). The env
// package is core and owns no component, so the check that reads that record
// back into show doctor lives here, as the recent-events check does for the
// report bus.

package doctor

import (
	"strings"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// envValuesCheckName names the check in the doctor registry.
	envValuesCheckName = "env-values"

	// envValuesDependency names what the check reads: the daemon's environment.
	envValuesDependency = "environment"

	// envValuesOrder prints these rows after the findings about present
	// platform state and before the recent events (recentEventsOrder).
	envValuesOrder = 2900

	// envValueShownMax is how many bytes of a value a row echoes.
	envValueShownMax = 64
)

// envValuesCheck reports each environment variable the daemon could not use
// as set. It runs in the runtime phase only: the values that matter are the
// daemon's own environment, which the local ze doctor process does not share.
var envValuesCheck = diagnostic.DoctorCheck{
	Name:         envValuesCheckName,
	Phase:        diagnostic.DoctorPhaseRuntime,
	Order:        envValuesOrder,
	Component:    doctorOwnComponent,
	Dependencies: []string{envValuesDependency},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorEnvInvalidValue},
	Check:        checkEnvValues,
}

// checkEnvValues answers one warning row per value env.InvalidValues lists,
// sorted by key. A row is a warning: ze runs, on a value the operator did not
// choose.
func checkEnvValues(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	values := env.InvalidValues()
	if len(values) == 0 {
		return nil
	}
	diags := make([]diagnostic.Diagnostic, 0, len(values))
	for i := range values {
		diags = append(diags, envValueDiagnostic(&values[i]))
	}
	return diags
}

// envValueDiagnostic writes one record as a doctor row: the key, the value
// (withheld for a secret key), the type, what ze uses instead, and the export
// that corrects it, spelled the way a shell accepts the name.
func envValueDiagnostic(v *env.InvalidValue) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	tb.Str("environment variable ").Str(v.Key).Str(" holds ")
	if v.Secret {
		tb.Str("a value not shown because the key is secret")
	} else {
		tb.Byte('"').Str(envValueShown(v.Value)).Byte('"')
	}
	if v.Kind == env.InvalidOutOfRange {
		tb.Str(", outside the range ").Str(v.Range).Str(" its reader accepts, so ze uses ").Str(v.Used).Str(" instead")
	} else {
		tb.Str(", which is not a valid ").Str(v.Type)
		writeEnvValueUsed(&tb, v)
	}
	tb.Str("; ze keeps that value until the export is corrected and ze restarts")

	var fix textbuf.Buffer
	fix.Str("export ").Str(strings.ToUpper(env.Normalize(v.Key))).Byte('=').Str(v.Example)

	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorEnvInvalidValue,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Fix:      fix.String(),
	}
}

// envValueShown bounds the value a row echoes: an environment value can be
// any length, and the row is one line. A cut value ends in "...", made valid
// UTF-8 again where the cut split a character.
func envValueShown(value string) string {
	if len(value) <= envValueShownMax {
		return value
	}
	return strings.ToValidUTF8(value[:envValueShownMax], "") + "..."
}

// writeEnvValueUsed says which default ze falls back to: the one its reader
// used, or for a key no reader has read yet the registered default, or, when
// none is registered, the default its reader will pass.
func writeEnvValueUsed(tb *textbuf.Buffer, v *env.InvalidValue) {
	if v.Read {
		tb.Str(", so ze uses the default ").Str(v.Used).Str(" instead")
		return
	}
	if v.Used != "" {
		tb.Str(", so ze uses its registered default ").Str(v.Used).Str(" when it reads it")
		return
	}
	tb.Str(", so ze uses its reader's own default when it reads it")
}
