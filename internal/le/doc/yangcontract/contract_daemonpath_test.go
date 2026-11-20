// Design: docs/architecture/api/commands.md -- local-data commands at an offline root, served by the daemon too

package docyangcontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestADaemonPathThatIsNotALiteralIsRefusedWhereItIsWritten: a registration
// whose registry.Meta.DaemonPath is a name, not a string literal, is refused,
// and the refusal names the file, the line and the registration, so the
// reader goes straight to it. Method: scan one source file holding such a
// registration on its sixth line, then one whose DaemonPath is a literal.
func TestADaemonPathThatIsNotALiteralIsRefusedWhereItIsWritten(t *testing.T) {
	dir := t.TempDir()
	named := filepath.Join(dir, "named.go")
	source := "package probe\n\nconst daemonPath = \"show probe list\"\n\nfunc init() {\n" +
		"\tcommand.MustRegisterLocalData(\"probe\", nil, registry.Meta{DaemonPath: daemonPath}, nil)\n}\n"
	if err := os.WriteFile(named, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	err := collectLocalHandlersFromFile(named, map[string]string{})
	if err == nil {
		t.Fatal("a DaemonPath spelled as a name was accepted")
	}
	for _, want := range []string{named + ":6:", `registration "probe"`, "must be a string literal"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}

	literal := filepath.Join(dir, "literal.go")
	source = strings.Replace(source, "DaemonPath: daemonPath", `DaemonPath: "show probe list"`, 1)
	if err := os.WriteFile(literal, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	if err := collectLocalHandlersFromFile(literal, paths); err != nil {
		t.Fatalf("a literal DaemonPath was refused: %v", err)
	}
	if paths["probe"] != "show probe list" {
		t.Errorf("daemon path of probe = %q, want %q", paths["probe"], "show probe list")
	}
}
