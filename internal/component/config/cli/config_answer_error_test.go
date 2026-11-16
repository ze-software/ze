// Design: docs/architecture/api/commands.md -- actionable configuration inspection failures.
package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestGraphParseRefusalPreservesCauseAndRecovery proves an invalid source
// produces a failure rather than a graph, with its parse evidence and next step.
func TestGraphParseRefusalPreservesCauseAndRecovery(t *testing.T) {
	answer := graphBytesAnswer([]byte("bgp {"))
	if answer.Code == 0 || answer.Payload != nil {
		t.Fatalf("invalid graph input answered %+v", answer)
	}
	if !strings.Contains(answer.Diagnostic, "could not be parsed") {
		t.Fatalf("graph refusal lost the parsing explanation: %q", answer.Diagnostic)
	}
	if !strings.HasSuffix(answer.Diagnostic, "fix: ze config validate -\n") {
		t.Fatalf("graph refusal has no final validation command: %q", answer.Diagnostic)
	}
}

// TestSuppliedDumpParseRefusalKeepsRecovery exercises the registered supplied
// route, so its refusal cannot lose the shared parser's recovery information.
func TestSuppliedDumpParseRefusalKeepsRecovery(t *testing.T) {
	answer := suppliedConfigAnswer(t, "dump", suppliedConfigTokens("bgp {"))
	if answer.Code != exitError || answer.Payload != nil {
		t.Fatalf("invalid dump input answered %+v", answer)
	}
	if !strings.Contains(answer.Diagnostic, "Error parsing config") {
		t.Fatalf("dump refusal lost its parsing evidence: %q", answer.Diagnostic)
	}
	if !strings.HasSuffix(answer.Diagnostic, "fix: ze config validate -\n") {
		t.Fatalf("dump refusal has no final validation command: %q", answer.Diagnostic)
	}
}

// TestFmtWriteRefusalKeepsEarlierDiagnostics exercises a missing destination
// directory and requires its useful earlier warning to precede the typed failure.
func TestFmtWriteRefusalKeepsEarlierDiagnostics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "config.conf")
	var diag textbuf.Buffer
	diag.Str("existing password warning\n")
	answer := fmtWriteAnswer(&diag, path, "bgp {}\n", true)
	if answer.Code != exitError || answer.Payload != nil {
		t.Fatalf("unwritable destination answered %+v", answer)
	}
	if !strings.HasPrefix(answer.Diagnostic, "existing password warning\nerror: The formatted configuration could not be saved to "+path) {
		t.Fatalf("write refusal lost warning or destination: %q", answer.Diagnostic)
	}
	if !strings.HasSuffix(answer.Diagnostic, "fix: ze config fmt -w '"+path+"'\n") {
		t.Fatalf("write refusal has no final recovery command: %q", answer.Diagnostic)
	}
}
