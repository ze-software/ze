// Design: docs/contributing/spec-workflow.md -- source identity must not weaken shared admission
// VALIDATES: registered doors, mechanisms, WIP and weakening consumers still
// judge writable dangling links reached through an external checkout alias.
// PREVENTS: strict evidence lookup errors becoming a shared outside exemption.
package hookruntime

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedCheckoutConsumersKeepDanglingAliasAdmission(t *testing.T) {
	for _, test := range []struct {
		name    string
		check   string
		path    string
		tool    string
		blocked map[string]any
		allowed map[string]any
		message string
	}{
		{
			name: "doors", check: "writeDesignDoorsRead", path: "internal/le/new.go", tool: toolWrite,
			blocked: map[string]any{"content": "package le\n"}, allowed: map[string]any{"content": "package le\n"}, message: "system.md",
		},
		{
			name: "mechanisms", check: "writeSpecMechanisms", path: "plan/post-release/engineering/spec-new.md", tool: toolWrite,
			blocked: map[string]any{"content": "| Field | Value |\n|---|---|\n| Status | design |\n"}, allowed: map[string]any{"content": "| Field | Value |\n|---|---|\n| Status | skeleton |\n"}, message: "Existing Mechanisms",
		},
		{
			name: "WIP", check: "writeSpecWIP", path: "plan/post-release/engineering/spec-new.md", tool: toolWrite,
			blocked: map[string]any{"content": "| Field | Value |\n|---|---|\n| Status | in-progress |\n"}, allowed: map[string]any{"content": "| Field | Value |\n|---|---|\n| Status | design |\n"}, message: "./le spec claim",
		},
		{
			name: "weakening", check: "writeWeakening", path: "internal/le/new_test.go", tool: toolEdit,
			blocked: map[string]any{"old_string": "\trequire.NoError(t, err)\n", "new_string": "\tt.Skip(\"later\")\n\trequire.NoError(t, err)\n"},
			allowed: map[string]any{"content": "package le\n"}, message: "proposed path is outside the checkout",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeDoors(t, root)
			readDoor(t, root, "shared-allowed", doorSystem, nil)
			outside := filepath.Join(t.TempDir(), filepath.FromSlash(test.path))
			alias := filepath.Join(t.TempDir(), "checkout-alias")
			if err := os.Symlink(root, alias); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(root, filepath.FromSlash(test.path))
			if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(filepath.Dir(link), "not-yet-created"+filepath.Ext(link)), link); err != nil {
				t.Fatal(err)
			}
			probe := func(path, tool, session string, fields map[string]any, want int, message string) {
				t.Helper()
				input := map[string]any{"file_path": path}
				maps.Copy(input, fields)
				code, got, found := Probe(test.check, Payload{SessionID: session, ToolName: tool, ToolInput: input}, root)
				if !found || code != want || message != "" && !strings.Contains(got, message) {
					t.Fatalf("%s(%s) = %d, found %v, want %d and %q: %s", test.check, path, code, found, want, message, got)
				}
			}
			// Known-outside paths remain outside even when the proposal would be refused inside.
			probe(outside, test.tool, "shared-blocked", test.blocked, 0, "")
			// Ordinary planned files remain writable with each gate's real admission satisfied.
			planned := filepath.Join(root, "ordinary", filepath.FromSlash(test.path))
			if test.check == "writeSpecMechanisms" || test.check == "writeSpecWIP" {
				planned = filepath.Join(root, "plan", "post-release", "engineering", "spec-ordinary.md")
			}
			probe(planned, toolWrite, "shared-allowed", test.allowed, 0, "")
			// The same checkout reached through its external alias still owes the named gate.
			probe(filepath.Join(alias, filepath.FromSlash(test.path)), test.tool, "shared-blocked", test.blocked, 2, test.message)
		})
	}
}
