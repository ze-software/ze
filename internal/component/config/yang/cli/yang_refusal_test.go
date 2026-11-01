package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
)

// TestYANGDataRefusalsAreReturned proves `show yang tree --help` answers the
// help page and a refused option answers its error line, both in the
// LocalAnswer rather than written by the parser or the handler.
// PREVENTS: the flag set's Usage hook printing help on the process's stderr
// from inside the parser, which on a daemon route reaches the daemon's stderr.
// Method: the help request compared with the page rendered without colour (the
// test's stderr is no terminal), and an unknown option's line checked whole.
func TestYANGDataRefusalsAreReturned(t *testing.T) {
	var page bytes.Buffer
	treeHelpPage().WriteTo(&page, false)

	help := dataTree(commandtest.Args("--help"))
	if help.Payload != nil || help.Code != 0 {
		t.Errorf("--help = payload %v, code %d; want nil, 0", help.Payload, help.Code)
	}
	if help.Diagnostic != page.String() {
		t.Errorf("--help diagnostic:\n got %q\nwant %q", help.Diagnostic, page.String())
	}

	refused := dataTree(commandtest.Args("--unknown"))
	if refused.Payload != nil || refused.Code != 1 {
		t.Errorf("--unknown = payload %v, code %d; want nil, 1", refused.Payload, refused.Code)
	}
	if !strings.HasPrefix(refused.Diagnostic, "error: ") || !strings.HasSuffix(refused.Diagnostic, "\n") ||
		!strings.Contains(refused.Diagnostic, "-unknown") {
		t.Errorf("--unknown diagnostic %q, want one error line naming -unknown", refused.Diagnostic)
	}
	if strings.Count(refused.Diagnostic, "\n") != 1 {
		t.Errorf("--unknown answered more than its error line: %q", refused.Diagnostic)
	}
}
