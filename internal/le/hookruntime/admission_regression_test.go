// Design: docs/contributing/spec-workflow.md -- admission through actual editing tools.
// VALIDATES: whole proposed Go, shipped bridge proof and atomic WIP publication.
// PREVENTS: fragment false positives, MultiEdit bypass and unlocked slot admission.
package hookruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/spec"
)

// TestPanicJudgmentReconstructsActualEditTools sends tool payloads to the
// registered write consumer, rather than calling the lexical helper.
func TestPanicJudgmentReconstructsActualEditTools(t *testing.T) {
	const original = "package probe\nfunc Message() string { return \"before\" }\n"
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		for _, test := range []struct {
			name        string
			replacement string
			want        int
		}{
			{"quoted-apostrophe", "panic: can't recover", 0},
			{"quoted-call", "panic(err)", 0},
		} {
			t.Run(tool+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				const file = "internal/probe/message.go"
				writeHookFixture(t, root, file, original)
				input := map[string]any{"file_path": filepath.Join(root, file)}
				switch tool {
				case toolWrite:
					input["content"] = strings.Replace(original, "before", test.replacement, 1)
				case toolEdit:
					input["old_string"], input["new_string"] = "before", test.replacement
				case toolMultiEdit:
					input["edits"] = []any{map[string]any{"old_string": "before", "new_string": test.replacement}}
				}
				code, message, found := Probe("writeGoPatterns", Payload{ToolName: tool, ToolInput: input}, root)
				if !found || code != test.want {
					t.Fatalf("%s = %d, found %v, want %d: %s", tool, code, found, test.want, message)
				}
			})
		}
		for _, test := range []struct{ name, body string }{
			{"operating-error", "panic(err)"},
			{"mixed", "panic(\"BUG: invariant\"); panic(err)"},
			{"malformed", "panic(\"BUG: invariant\""},
		} {
			t.Run(tool+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				const file = "internal/probe/panic.go"
				const before = "package probe\nfunc Run() { return }\n"
				writeHookFixture(t, root, file, before)
				input := map[string]any{"file_path": filepath.Join(root, file)}
				switch tool {
				case toolWrite:
					input["content"] = strings.Replace(before, "return", test.body, 1)
				case toolEdit:
					input["old_string"], input["new_string"] = "return", test.body
				case toolMultiEdit:
					input["edits"] = []any{map[string]any{"old_string": "return", "new_string": test.body}}
				}
				code, message, found := Probe("writeGoPatterns", Payload{ToolName: tool, ToolInput: input}, root)
				if !found || code != 2 || !strings.Contains(message, "panic") {
					t.Fatalf("disallowed call = %d, found %v: %s", code, found, message)
				}
			})
		}
	}
}

// TestShippedBridgeProofReachesEveryEditTool exercises the registered prewrite
// consumer and postwrite hook over the shipped standalone bridge dispatch.
func TestShippedBridgeProofReachesEveryEditTool(t *testing.T) {
	const source = "internal/exabgp/bridge/bridge.go"
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		for _, proof := range []string{"internal/exabgp/bridge/bridge_test.go", "test/ui/script-status.ci"} {
			t.Run(tool+"/"+proof, func(t *testing.T) {
				root := t.TempDir()
				writeHookFixture(t, root, source, "// Design: docs/bridge.md -- standalone bridge dispatch\npackage bridge\n")
				writeHookFixture(t, root, "docs/bridge.md", "# Bridge\n\n<!-- source: internal/exabgp/bridge/bridge.go -- producer -->\n")
				body := strings.Replace(specFixture(source), "## Files to Modify\n\nOne.", "## Files to Modify\n\n- `"+source+"`", 1)
				body = strings.Replace(body, "## Data Flow", "## Existing Mechanisms\n\n| Mechanism | Source | Decision |\n|---|---|---|\n| Script bridge | `"+source+"` | Extend the existing dispatch |\n\n## Data Flow", 1)
				body += "\n## Proof\n`" + proof + "`\n\n## Design documentation\n`docs/bridge.md`\n" // <!-- doc-links: ignore (a synthetic path in a test fixture) -->
				const file = "plan/post-release/engineering/spec-bridge-proof.md"
				before := strings.ReplaceAll(body, source, "internal/le/probe/probe.go")
				writeHookFixture(t, root, file, before)
				input := map[string]any{"file_path": filepath.Join(root, file)}
				switch tool {
				case toolWrite:
					input["content"] = body
				case toolEdit:
					input["old_string"], input["new_string"] = before, body
				case toolMultiEdit:
					input["edits"] = []any{map[string]any{"old_string": before, "new_string": body}}
				}
				want := 2
				if strings.HasPrefix(proof, "test/") {
					want = 0
				}
				code, message, found := Probe("writeSpecMechanisms", Payload{ToolName: tool, ToolInput: input}, root)
				if !found || code != want {
					t.Errorf("prewrite = %d, found %v, want %d: %s", code, found, want, message)
				}
				if want == 2 && !strings.Contains(message, "functional test") {
					t.Errorf("prewrite did not judge daemon proof: %s", message)
				}
				writeHookFixture(t, root, file, body)
				code, _, message = runHook(t, root, "validate-spec", map[string]any{"tool_name": tool, "tool_input": input})
				if code != want {
					t.Errorf("postwrite = %d, want %d: %s", code, want, message)
				}
				if want == 2 && !strings.Contains(message, "functional test") {
					t.Errorf("postwrite did not judge daemon proof: %s", message)
				}
			})
		}
	}
}

// TestProofPopulationPreservesOnlyAffirmativeExemptions drives validation over
// known nonproduction roots and a new runtime root that has no exemption.
func TestProofPopulationPreservesOnlyAffirmativeExemptions(t *testing.T) {
	for _, test := range []struct {
		source string
		want   int
	}{
		{"internal/le/probe/probe.go", 0},
		{"internal/test/fixture/probe.go", 0},
		{"internal/chaos/probe.go", 0},
		{"internal/perf/probe.go", 0},
		{"internal/component/probe/probe_test.go", 0},
		{"internal/component/probe/testdata/probe.go", 0},
		{"test/ui/probe.ci", 0},
		{"runtime-new/probe.go", 2},
	} {
		t.Run(test.source, func(t *testing.T) {
			root := t.TempDir()
			writeHookFixture(t, root, "internal/le/probe/docownership.go",
				"// Design: docs/population.md -- neutral documentation prerequisite\npackage probe\n")
			writeHookFixture(t, root, "docs/population.md",
				"# Population fixture\n\n<!-- source: internal/le/probe/docownership.go -- the neutral documentation prerequisite -->\n")
			body := strings.Replace(specFixture(test.source), "## Files to Modify\n\nOne.", "## Files to Modify\n\n- `"+test.source+"`", 1)
			body += "\n## Design documentation\n`docs/population.md`\n" // <!-- doc-links: ignore (a synthetic path in a test fixture) -->
			const file = "plan/post-release/engineering/spec-population.md"
			writeHookFixture(t, root, file, body)
			code, _, message := runHook(t, root, "validate-spec", map[string]any{
				"tool_name": toolWrite, "tool_input": map[string]any{"file_path": filepath.Join(root, file)},
			})
			if code != test.want {
				t.Fatalf("population = %d, want %d: %s", code, test.want, message)
			}
			if test.want == 2 && !strings.Contains(message, "functional test") {
				t.Fatalf("unknown source escaped proof judgment: %s", message)
			}
		})
	}
}

// TestDirectSpecAdmissionCannotRaceNativeClaim deterministically schedules two
// external preflights, one native claim publication, then permitted external
// publications: the adverse interleaving a precheck lock cannot prevent.
func TestDirectSpecAdmissionCannotRaceNativeClaim(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("ZE_SPEC_WIP_CAP", "1")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "native-publication")
	t.Setenv("ZE_AGENT_ID", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	const ready = "| Field | Value |\n|---|---|\n| Status | ready |\n| Updated | 2026-11-04 |\n"
	active := strings.Replace(ready, "ready", "in-progress", 1)
	const dir = "plan/post-release/engineering/"
	for _, name := range []string{"spec-direct-a.md", "spec-direct-b.md", "spec-native.md"} {
		writeHookFixture(t, root, dir+name, ready)
	}
	var permitted []string
	for index, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		name := []string{"spec-direct-a.md", "spec-direct-b.md", "spec-direct-a.md"}[index]
		input := map[string]any{"file_path": filepath.Join(root, dir+name)}
		switch tool {
		case toolWrite:
			input["content"] = active
		case toolEdit:
			input["old_string"], input["new_string"] = ready, active
		case toolMultiEdit:
			input["edits"] = []any{map[string]any{"old_string": ready, "new_string": active}}
		}
		code, message, found := Probe("writeSpecWIP", Payload{ToolName: tool, ToolInput: input}, root)
		if !found || code != 2 || !strings.Contains(message, "./le spec claim "+name) {
			t.Errorf("%s transition = %d, found %v; want exact native claim: %s", tool, code, found, message)
		}
		if code == 0 {
			permitted = append(permitted, name)
		}
	}
	answer, code := spec.Claim([]string{"spec-native.md"})
	report, ok := answer.(spec.ClaimReport)
	if code != 0 || !ok || !report.Transitioned {
		t.Fatalf("native claim failed: code %d, answer %#v", code, answer)
	}
	for _, name := range permitted {
		writeHookFixture(t, root, dir+name, active)
	}
	count := 0
	for _, name := range []string{"spec-direct-a.md", "spec-direct-b.md", "spec-native.md"} {
		body, err := os.ReadFile(filepath.Join(root, dir+name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "| Status | in-progress |") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("external preflight/native claim race published %d active specs at cap 1", count)
	}
}

// TestCanonicalSpecValidationRoutesEveryEditTool applies the canonical matcher
// before invoking its native hook, so a manually invoked MultiEdit cannot hide
// a missing installed event route.
func TestCanonicalSpecValidationRoutesEveryEditTool(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(body, &settings); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		routed := false
		for _, event := range settings.Hooks["PostToolUse"] {
			matcher, err := regexp.Compile("^(?:" + event.Matcher + ")$")
			if err != nil {
				t.Fatal(err)
			}
			if !matcher.MatchString(tool) {
				continue
			}
			for _, hook := range event.Hooks {
				if !strings.HasSuffix(hook.Command, " ai hooks validate-spec") {
					continue
				}
				routed = true
				root := t.TempDir()
				const file = "plan/post-release/engineering/spec-invalid.md"
				writeHookFixture(t, root, file, "# Invalid spec\n")
				code, _, message := runHook(t, root, "validate-spec", map[string]any{
					"tool_name": tool, "tool_input": map[string]any{"file_path": filepath.Join(root, file)},
				})
				if code != 2 {
					t.Errorf("%s routed validation = %d, want refusal: %s", tool, code, message)
				}
			}
		}
		if !routed {
			t.Errorf("canonical PostToolUse settings do not route %s to validate-spec", tool)
		}
	}
}
