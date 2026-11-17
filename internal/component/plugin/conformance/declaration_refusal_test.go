package conformance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rawDeclarationLine is a Stage 1 request line carrying params verbatim, as a
// plugin written in another language would print it.
func rawDeclarationLine(params string) string {
	return "#1 " + rpc.MethodDeclareRegistration + " " + params + "\n"
}

// TestRunFailsTheDecodeRowForTheRetiredSchemaKey proves the checker reads a
// declaration as the daemon does: a declaration that carries its module under
// the retired "schema" key fails the decode row with the daemon's refusal,
// which names the plugin and the "schemas" form to send, and the Stage 1 rows
// after it are not judged, never passed. The same module under "schemas"
// passes both, so the refusal is tied to the key and not to the module.
//
// Method: a shell program that prints the line, run through Run.
func TestRunFailsTheDecodeRowForTheRetiredSchemaKey(t *testing.T) {
	const module = `{"module":"acme-conf","namespace":"urn:acme","yang-text":"module acme-conf {}"}`
	stage1Checks := []string{"protocol-version", "help-texts", "command-names"}

	t.Run("schema is refused", func(t *testing.T) {
		report := runTarget(t, writes(rawDeclarationLine(`{"protocol-version":1,"schema":`+module+`}`)), testBudget)

		row := rowNamed(t, report, "declaration-decodes")
		require.Equal(t, VerdictFail, row.Verdict, "reason %q", row.Reason)
		assert.Equal(t, stageDeclare, row.Stage)
		assert.Contains(t, row.Reason, `plugin "conformance-subject" sent its YANG module under "schema", a key ze no longer reads`)
		assert.Contains(t, row.Reason, `send "schemas":[{"module":...,"namespace":...,"yang-text":...}] in place of "schema":{...}`)
		assert.NotEmpty(t, row.Fix, "a failed row names the change that fixes it")
		for _, check := range stage1Checks {
			later := rowNamed(t, report, check)
			assert.Equal(t, VerdictNotJudged, later.Verdict, "%s: %s", check, later.Reason)
			assert.Equal(t, "the declaration was refused before any check read it", later.Reason, check)
		}
		assert.Equal(t, VerdictFail, report.Verdict)
		assert.Equal(t, ExitViolation, report.ExitCode())
	})

	t.Run("schemas is read", func(t *testing.T) {
		report := runTarget(t, writes(rawDeclarationLine(`{"protocol-version":1,"schemas":[`+module+`]}`)), testBudget)

		for _, check := range []string{"declaration-decodes", "protocol-version"} {
			row := rowNamed(t, report, check)
			assert.Equal(t, VerdictPass, row.Verdict, "%s: %s", check, row.Reason)
		}
	})
}
