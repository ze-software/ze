// Design: docs/guide/authentication.md -- local recovery credential diagnostics.
// Related: main_servers.go -- usersFromStore is the only local credential reader.
// Related: register_bootstrap_credentials.go -- owned pre-config and event registrations.

package hub

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const localAdminCredentialSource = "hub"
const localAdminCredentialSubject = "local-admin"
const localAdminCredentialRestore = "ze data --path <database> write <credential-key> <credential-backup-file>"

// localAdminCredentialFailure interprets a canonical credential read. A nil
// error or explicit admin disable is healthy and returns nil. Every other
// failure retains its cause without exposing the stored credential bytes.
func localAdminCredentialFailure(err error) *opererr.Error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errAdminDisabledInZefs) {
		return nil
	}
	return opererr.Wrap(err, "stored local recovery credentials are unavailable", "This account cannot log in. From the console or another working login, stop Ze, restore the affected local credential frame from a known backup, and restart Ze", localAdminCredentialRestore)
}

func localAdminCredentialDiagnostic(failure *opererr.Error) diagnostic.Diagnostic {
	var message textbuf.Buffer
	diag := diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorLocalAdminCredentials,
		Severity: diagnostic.SeverityWarning,
		Message:  message.Str(failure.Problem()).Str(". ").Str(failure.Why()).String(),
		Fix:      failure.Fix(),
	}
	if cause := failure.Unwrap(); cause != nil {
		diag.Actual = cause.Error()
	}
	return diag
}

// checkLocalAdminCredentials uses the same reader as daemon authentication.
// A missing store cannot establish whether the credentials are usable and is
// reported as an unavailable probe rather than as a healthy local account.
func checkLocalAdminCredentials(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if ctx.Store == nil {
		failure := opererr.New("local recovery credentials could not be checked", "No owned credential store was supplied; check the initialized device database", "ze data --path <database> check")
		return []diagnostic.Diagnostic{localAdminCredentialDiagnostic(failure)}
	}
	_, err := usersFromStore(ctx.Store)
	failure := localAdminCredentialFailure(err)
	if failure == nil {
		return nil
	}
	diag := localAdminCredentialDiagnostic(failure)
	diag.Path = ctx.ConfigDir
	return []diagnostic.Diagnostic{diag}
}

// reportLocalAdminCredentialRead keeps one active problem for the boot reader.
// A healthy read, including explicit disable, clears only this owned issue.
// Reporting is independent of the optional logger supplied by the caller.
func reportLocalAdminCredentialRead(log *slog.Logger, err error) {
	failure := localAdminCredentialFailure(err)
	if failure == nil {
		report.ClearWarning(localAdminCredentialSource, diagnostic.CodeDoctorLocalAdminCredentials, localAdminCredentialSubject)
		return
	}
	diag := localAdminCredentialDiagnostic(failure)
	report.RaiseWarningFix(localAdminCredentialSource, diagnostic.CodeDoctorLocalAdminCredentials, localAdminCredentialSubject, diag.Message, diag.Fix, map[string]any{"cause": err.Error()})
	if log != nil {
		log.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(), failure.LogAttrs(slog.String("code", diagnostic.CodeDoctorLocalAdminCredentials))...)
	}
}
