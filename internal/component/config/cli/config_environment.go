// Design: docs/architecture/config/environment.md -- environment validation authority.
// Related: cmd_validate.go -- the environment-only local invocation.

package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// environmentValidationAnswer uses the same registered-type and recorded-range
// authority as doctor. Secret values are never copied into the result.
func environmentValidationAnswer(command.ValidatedArgs) command.LocalAnswer {
	invalid := env.InvalidValues()
	diags := make([]diagnostic.Diagnostic, 0, len(invalid))
	var reason textbuf.Buffer
	for _, value := range invalid {
		var message, fix textbuf.Buffer
		message.Str("Environment variable ").Str(value.Key).Str(" is not a valid ").Str(value.Type)
		if value.Kind == env.InvalidOutOfRange {
			message.Str(" in the range ").Str(value.Range)
		}
		message.Str("; Ze cannot use the configured value.")
		fix.Str("env ").Str(value.Key).Byte('=').Str(value.Example).
			Str(" ze config validate --limit environment")
		diags = append(diags, diagnostic.Diagnostic{
			Code: diagnostic.CodeDoctorEnvInvalidValue, Severity: diagnostic.SeverityError,
			Message: message.String(), Path: value.Key, Expected: value.Type,
			Fix: fix.String(),
		})
		reason.Str(message.String()).Byte('\n').Str(fix.String()).Byte('\n')
	}
	valid := len(diags) == 0
	answer := command.LocalAnswer{Payload: diagnostic.NewValidateResult("environment", valid, diags, nil)}
	if !valid {
		answer.Code = exitInvalid
		answer.Diagnostic = reason.String()
	}
	return answer
}
