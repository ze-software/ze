// Design: docs/contributing/ze-go-style.md -- every panic states a programmer invariant
// Related: panic.go -- the judgment of the panic calls an edit adds
package hookruntime

import (
	"path/filepath"
	"strings"
	"testing"
)

// Goal: a panic call already in a file does not refuse an unrelated edit to
// it, while every call the edit adds is still judged, a second copy of an
// existing call included.
// Method: a file that re-raises a recovered value, edited through each tool
// payload the registered write consumer reconstructs.
// MUTATION: judging the whole proposed file again refuses "unrelated";
// ignoring call counts admits "duplicate".
func TestPanicJudgmentSkipsCallsAlreadyInFile(t *testing.T) {
	const original = "package probe\n\nfunc Run() {\n\tdefer func() {\n\t\tif failure := recover(); failure != nil {\n\t\t\tpanic(failure)\n\t\t}\n\t}()\n\treturn\n}\n"
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		for _, test := range []struct {
			name        string
			replacement string
			want        int
		}{
			{"unrelated", "_ = 1\n\treturn", 0},
			{"allowed-addition", "panic(\"BUG: invariant\")", 0},
			{"new-call", "panic(err)", 2},
			{"duplicate", "panic(failure)", 2},
			{"malformed", "panic(\"BUG: invariant\"", 2},
		} {
			t.Run(tool+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				const file = "internal/probe/recover.go"
				writeHookFixture(t, root, file, original)
				input := map[string]any{"file_path": filepath.Join(root, file)}
				switch tool {
				case toolWrite:
					input["content"] = strings.Replace(original, "\treturn\n", "\t"+test.replacement+"\n", 1)
				case toolEdit:
					input["old_string"], input["new_string"] = "\treturn\n", "\t"+test.replacement+"\n"
				case toolMultiEdit:
					input["edits"] = []any{map[string]any{"old_string": "\treturn\n", "new_string": "\t" + test.replacement + "\n"}}
				}
				code, message, found := Probe("writeGoPatterns", Payload{ToolName: tool, ToolInput: input}, root)
				if !found || code != test.want {
					t.Fatalf("%s = %d, found %v, want %d: %s", tool, code, found, test.want, message)
				}
			})
		}
	}
}

// Goal: a file the check cannot read is refused for every tool, a Write
// included, because the calls it already holds cannot be known.
// Method: a directory at the target path, which os.ReadFile refuses with
// EISDIR whoever runs the test.
func TestPanicJudgmentRefusesUnreadableCurrentFile(t *testing.T) {
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		t.Run(tool, func(t *testing.T) {
			root := t.TempDir()
			const file = "internal/probe/dir.go"
			writeHookFixture(t, root, file+"/placeholder", "x")
			input := map[string]any{"file_path": filepath.Join(root, file)}
			switch tool {
			case toolWrite:
				input["content"] = "package probe\n"
			case toolEdit:
				input["old_string"], input["new_string"] = "a", "b"
			case toolMultiEdit:
				input["edits"] = []any{map[string]any{"old_string": "a", "new_string": "b"}}
			}
			code, message, found := Probe("writeGoPatterns", Payload{ToolName: tool, ToolInput: input}, root)
			if !found || code != 2 || !strings.Contains(message, "cannot reconstruct") {
				t.Fatalf("%s = %d, found %v: %s", tool, code, found, message)
			}
		})
	}
}
