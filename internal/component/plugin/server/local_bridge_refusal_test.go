package server

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestLocalRefusalKeepsTheLocalText maps a refused local-data answer onto a
// daemon response and reads what a surface prints from the returned error.
//
// VALIDATES: the returned error is a plugin.RefusalError whose Line is the
// Diagnostic the local route prints, byte for byte minus the final newline,
// for a refusal that opens with "error: " and for a usage refusal that does
// not; Response.Error stays the reason without the prefix. A refusal with no
// Diagnostic is a plain error naming the path and the code.
// PREVENTS: SSH, the web and MCP printing "error: usage: ..." where `ze
// <command>` prints "usage: ...".
func TestLocalRefusalKeepsTheLocalText(t *testing.T) {
	for _, diagnostic := range []string{"usage: show config list\n", "error: no such key\n"} {
		resp, err := localAnswerResponse("show x", command.LocalRefusal(1, diagnostic))
		line, refused := plugin.RefusalLine(err)
		if !refused {
			t.Fatalf("refusal %q: error %v is not a plugin.RefusalError", diagnostic, err)
		}
		if line+"\n" != diagnostic {
			t.Errorf("refusal %q: a surface prints %q, want the local bytes", diagnostic, line+"\n")
		}
		if resp.Error != err.Error() {
			t.Errorf("refusal %q: Response.Error %q differs from the error %q", diagnostic, resp.Error, err.Error())
		}
	}

	_, err := localAnswerResponse("show x", command.LocalRefusal(3, ""))
	if _, refused := plugin.RefusalLine(err); refused {
		t.Errorf("a refusal with no text was carried as verbatim text: %v", err)
	}
	if want := "show x failed with exit code 3 and gave no reason"; err == nil || err.Error() != want {
		t.Errorf("silent refusal error = %v, want %q", err, want)
	}
}
