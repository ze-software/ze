// Design: docs/architecture/doctor-and-health-checks.md -- retained full-store recovery sources.
// Related: doctor_checks.go -- registration before configuration loading.
package doctor

import (
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorCheckRecoveryPending reads the same directory authority as admission;
// it never consumes a retained source. The opener reports a missing store.
func doctorCheckRecoveryPending(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if ctx.Store == nil {
		return nil
	}
	stage, command, err := storage.PendingRecoveryCommand(ctx.Store)
	var message textbuf.Buffer
	if err != nil && stage == "" {
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorStorageRecoveryPending,
			Severity: diagnostic.SeverityError,
			Message:  message.Str("retained full-store recovery sources could not be inspected: ").Err(err).String(),
			Path:     ctx.ConfigDir,
			Fix:      fixAskActiveConfig,
		}}
	}
	if stage == "" {
		return nil
	}
	if err != nil {
		message.Str("the retained directory's recovery ownership is not proven: ").Err(err).Str("; ")
		var fix textbuf.Buffer
		fix.Str("Stop all Ze store owners; verify the intended live store, then move the whole directory ").
			Str(configtext.QuoteCommandWord(stage)).Str(" outside ").Str(configtext.QuoteCommandWord(ctx.ConfigDir)).
			Str(" without deleting bytes or overwriting a destination; then run ze doctor")
		command = fix.String()
	}
	return []diagnostic.Diagnostic{{
		Code:     diagnostic.CodeDoctorStorageRecoveryPending,
		Severity: diagnostic.SeverityWarning,
		Message: message.Str("an unresolved full-store recovery source remains at ").Str(stage).
			Str("; another source cannot be accepted until it is resolved").String(),
		Path: stage,
		Fix:  command,
	}}
}
