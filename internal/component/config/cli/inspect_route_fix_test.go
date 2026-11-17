package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// producerRefusal is a refusal as the dump producer words one for content it
// was supplied: it holds no file name, so its fix names standard input.
const producerRefusal = "Error resolving config\n" +
	"The supplied configuration could not be resolved; no configuration dump was produced.: missing required bgp { } block\n" +
	"fix: ze config validate -"

// TestFixNamingFileNamesTheFileTheOperatorGave proves the fix line of a
// producer's refusal is rewritten only when it is the last line, names
// standard input, and the operator gave a file.
func TestFixNamingFileNamesTheFileTheOperatorGave(t *testing.T) {
	rows := []struct {
		name string
		diag string
		path string
		want string
	}{
		{"a file, no final newline", "error: refused\nfix: ze config validate -", "first.conf", "error: refused\nfix: ze config validate first.conf"},
		{"a file, final newline kept", "error: refused\nfix: ze config validate -\n", "etc/first.conf", "error: refused\nfix: ze config validate etc/first.conf\n"},
		{"the fix is the only line", "fix: ze config validate -\n", "first.conf", "fix: ze config validate first.conf\n"},
		{"standard input keeps the dash", "error: refused\nfix: ze config validate -\n", "-", "error: refused\nfix: ze config validate -\n"},
		{"no path keeps the dash", "error: refused\nfix: ze config validate -\n", "", "error: refused\nfix: ze config validate -\n"},
		{"another fix is left alone", "error: refused\nfix: ze show doctor\n", "first.conf", "error: refused\nfix: ze show doctor\n"},
		{"a fix that names a file is left alone", "error: refused\nfix: ze config validate other.conf\n", "first.conf", "error: refused\nfix: ze config validate other.conf\n"},
		{"the words inside a line are left alone", "error: see the prefix: ze config validate -\n", "first.conf", "error: see the prefix: ze config validate -\n"},
		{"a fix that is not the last line is left alone", "fix: ze config validate -\nmore\n", "first.conf", "fix: ze config validate -\nmore\n"},
		{"nothing to read", "", "first.conf", ""},
	}
	for _, row := range rows {
		assert.Equal(t, row.want, fixNamingFile(row.diag, row.path), row.name)
	}
}

// TestInspectRouteNamesTheOperatorsFileInTheDaemonsFix: the running ze judged
// content it was sent, with no file name, so the fix of its refusal names
// standard input. The command that read the operator's file draws that
// refusal with the file in the fix. Method: a stub ask that refuses as the
// dump producer does, through the registered handlers of the three commands
// that read one file.
//
// VALIDATES: the daemon's lines are drawn whole inside one operator error, and
// the last line, the one fix, is `fix: ze config validate <the file the
// operator gave>`.
// PREVENTS: `fix: ze config validate -` for an operator who piped nothing.
func TestInspectRouteNamesTheOperatorsFileInTheDaemonsFix(t *testing.T) {
	for _, route := range inspectionRoutes() {
		if route.name == "diff" {
			// Two files: a refusal of the comparison names neither in its fix.
			continue
		}
		t.Run(route.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			_, path, marker := routedConfig(t)
			recorder := &askRecorder{verdict: daemonVerdict{refused: true, refusal: producerRefusal, address: daemonAddress}}

			code, answered, diagnostic := route.answer(t, path, recorder.ask)

			require.Equal(t, 1, recorder.calls)
			assert.NotEqual(t, exitOK, code)
			assert.False(t, answered)
			// MUTATION: return diag unchanged from fixNamingFile, and the last
			// line is `fix: ze config validate -`.
			assert.Equal(t, "error: the running ze refused config "+path+"\n"+
				"It checked the config with the schemas of the plugins it started. What it answered: "+
				strings.TrimSuffix(producerRefusal, "\nfix: ze config validate -")+"\n"+
				"fix: ze config validate "+path+"\n", diagnostic)
			requireNotStarted(t, marker)
		})
	}
}
