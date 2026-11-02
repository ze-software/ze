// Design: docs/architecture/api/commands.md — a declared leaf typed by name or by position

package cli

import (
	"strings"
	"testing"
)

// fileLeafConfig is a configuration validation accepts, so the answers
// compared are verdicts of a valid file rather than two refusals.
const fileLeafConfig = "bgp {\n    router-id 203.0.113.1;\n    session { asn { local 65000; } }\n}\n"

// TestFileLeafKeyedFormReadsOneFile proves a command whose model declares a
// `file` leaf reads the same one file whether the leaf is typed by name or by
// position.
//
// Goal: `show config validation file f` and `show config graph file f`, the
// spelling a generated MCP tool sends, answer exactly what the positional
// `... f` answers, on the local and the daemon route, and that answer is a
// success, so two equal refusals cannot pass. The keyword was counted as a
// second file: validation refused "takes one config file, got 2" and graph
// read a file named "file".
// Method: each spelling through command.ValidateModelArgs and the registered
// handler (answerOn), comparing code, payload and diagnostic.
func TestFileLeafKeyedFormReadsOneFile(t *testing.T) {
	file := confinedConfigDir(t, fileLeafConfig)

	for _, path := range []string{"show config validation", "show config graph"} {
		for _, remote := range []bool{false, true} {
			positional := answerOn(t, path, []string{file}, remote)
			if positional.Code != 0 {
				t.Fatalf("%s %s (remote %v) refused a valid config, so this test proves nothing: %s",
					path, file, remote, encodedAnswer(t, positional))
			}
			keyed := answerOn(t, path, []string{"file", file}, remote)
			if encodedAnswer(t, keyed) != encodedAnswer(t, positional) {
				t.Errorf("%s file %s (remote %v) answered\n%s\nthe positional form\n%s",
					path, file, remote, encodedAnswer(t, keyed), encodedAnswer(t, positional))
			}
		}
	}
}

// TestValidationKeyedFileWithSecondFileRefused proves the keyed form does not
// hide a second file: `show config validation file a b` names two files and
// is refused with the count, as `show config validation a b` is.
func TestValidationKeyedFileWithSecondFileRefused(t *testing.T) {
	file := confinedConfigDir(t, fileLeafConfig)

	refusal := answerOn(t, "show config validation", []string{"file", file, file}, false)
	if refusal.Code == 0 {
		t.Fatalf("validation of two files succeeded: %s", encodedAnswer(t, refusal))
	}
	if !strings.Contains(refusal.Diagnostic, "takes one config file, got 2") {
		t.Errorf("refusal %q does not name the two files", refusal.Diagnostic)
	}
}
