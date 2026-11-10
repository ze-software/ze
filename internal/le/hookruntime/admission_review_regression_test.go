// Design: docs/contributing/spec-workflow.md -- canonical proof and unambiguous admission.
// VALIDATES: registered edit consumers classify resolved source identities and metadata.
// PREVENTS: tooling aliases or missing metadata headers granting publication permission.
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/spec"
)

func admissionReviewInput(tool, file, current, proposed string) map[string]any {
	input := map[string]any{"file_path": file}
	switch tool {
	case toolWrite:
		input["content"] = proposed
	case toolEdit:
		input["old_string"], input["new_string"] = current, proposed
	case toolMultiEdit:
		input["edits"] = []any{map[string]any{"old_string": current, "new_string": proposed}}
	}
	return input
}

// TestDaemonProofUsesCanonicalSourceBeforeExemptions drives actual registered
// prewrite consumers over runtime aliases and genuinely planned tooling paths.
func TestDaemonProofUsesCanonicalSourceBeforeExemptions(t *testing.T) {
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		for _, test := range []struct {
			name, source string
			want         int
			diagnostic   string
		}{
			{"dot-runtime", "internal/le/../exabgp/bridge/bridge_status.go", 2, "functional test"},
			{"symlink-runtime", "internal/le/runtime-alias/bridge_status.go", 2, "functional test"},
			{"planned-runtime-alias", "internal/le/runtime-alias/planned/new.go", 2, "functional test"},
			{"planned-tooling", "internal/le/planned/new.go", 0, ""},
			{"canonical-runtime", "internal/exabgp/bridge/bridge_status.go", 2, "functional test"},
			{"outside-tooling-alias", "internal/le/outside-alias/source.go", 2, "outside"},
			{"broken-tooling-alias", "internal/le/broken-alias/source.go", 2, "resolve"},
		} {
			t.Run(tool+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				writeHookFixture(t, root, "internal/le/neutral.go", "package neutral\n")
				writeHookFixture(t, root, "internal/exabgp/bridge/bridge_status.go", "package bridge\n")
				for name, target := range map[string]string{"runtime-alias": "../exabgp/bridge", "outside-alias": t.TempDir(), "broken-alias": "../absent-runtime"} {
					if err := os.Symlink(target, filepath.Join(root, "internal", "le", name)); err != nil {
						t.Fatal(err)
					}
				}
				body := strings.Replace(specFixture(test.source), "## Files to Modify\n\nOne.", "## Files to Modify\n\n- `"+test.source+"`", 1)
				body = strings.Replace(body, "## Data Flow", "## Existing Mechanisms\n\n| Mechanism | Source | Decision |\n|---|---|---|\n| Existing dispatch | `internal/le/neutral.go` | Extend |\n\n## Data Flow", 1)
				const file = "plan/post-release/engineering/spec-canonical-proof.md"
				const before = "# Before\n"
				writeHookFixture(t, root, file, before)
				code, message, found := Probe("writeSpecMechanisms", Payload{ToolName: tool, ToolInput: admissionReviewInput(tool, filepath.Join(root, file), before, body)}, root)
				if !found || code != test.want {
					t.Fatalf("canonical proof = %d, found %v, want %d: %s", code, found, test.want, message)
				}
				if test.diagnostic != "" && !strings.Contains(message, test.diagnostic) {
					t.Fatalf("wrong canonical refusal: %s", message)
				}
			})
		}
	}
}

// TestDirectAdmissionRequiresUnambiguousMetadata drives every edit tool, keeping
// valid existing-active and nontransition controls beside malformed proposals.
func TestDirectAdmissionRequiresUnambiguousMetadata(t *testing.T) {
	const ready = "| Field | Value |\n|---|---|\n| Status | ready |\n"
	const active = "| Field | Value |\n|---|---|\n| Status | in-progress |\n"
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		for _, test := range []struct {
			name, current, proposed string
			want                    int
		}{
			{"headerless-active", ready, "| Status | in-progress |\n", 2},
			{"removed-header-active", active, "| Status | in-progress |\n", 2},
			{"headerless-ready", ready, "| Status | ready |\n", 2},
			{"unknown-status", ready, strings.Replace(ready, "ready", "not-a-status", 1), 2},
			{"duplicate-status", ready, ready + "| Status | in-progress |\n", 2},
			{"ambiguous-existing-active", active + "| Status | ready |\n", active, 2},
			{"existing-active", active, active + "\nUnrelated detail.\n", 0},
			{"ready-detail", ready, ready + "\nUnrelated detail.\n", 0},
			{"legacy-spacing", "|Field|Value|\n|---|---|\n|Status|in-progress|\n", "|Field|Value|\n|---|---|\n|Status|in-progress|\n\nDetail.\n", 0},
		} {
			t.Run(tool+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				const file = "plan/post-release/engineering/spec-metadata.md"
				writeHookFixture(t, root, file, test.current)
				code, message, found := Probe("writeSpecWIP", Payload{ToolName: tool, ToolInput: admissionReviewInput(tool, filepath.Join(root, file), test.current, test.proposed)}, root)
				if !found || code != test.want {
					t.Fatalf("metadata admission = %d, found %v, want %d: %s", code, found, test.want, message)
				}
				if test.want == 2 && !strings.Contains(strings.ToLower(message), "metadata") {
					t.Fatalf("wrong metadata refusal: %s", message)
				}
			})
		}
	}
}

// TestValidationAndSourceAdmissionShareMetadataGrammar runs the actual validator
// and the registered source-edit gate against the same claimed-spec contents.
func TestValidationAndSourceAdmissionShareMetadataGrammar(t *testing.T) {
	for _, test := range []struct {
		name      string
		transform func(string) string
		want      int
	}{
		{"valid-active", func(s string) string { return s }, 0},
		{"legacy-spacing", func(s string) string {
			return strings.ReplaceAll(strings.ReplaceAll(s, "| Field | Value |", "|Field|Value|"), "| Status | in-progress |", "|Status|in-progress|")
		}, 0},
		{"missing-header", func(s string) string { return strings.Replace(s, "| Field | Value |\n", "", 1) }, 2},
		{"duplicate-status", func(s string) string {
			return strings.Replace(s, "| Status | in-progress |", "| Status | in-progress |\n| Status | ready |", 1)
		}, 2},
		{"unknown-status", func(s string) string {
			return strings.Replace(s, "| Status | in-progress |", "| Status | not-a-status |", 1)
		}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "metadata-proof")
			t.Setenv("ZE_AGENT_ID", "")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			const source = "internal/le/probe/metadata.go"
			writeHookFixture(t, root, source, "// Design: docs/metadata.md -- the fixture producer\npackage probe\n")
			writeHookFixture(t, root, "docs/metadata.md", "# Metadata\n\n<!-- source: internal/le/probe/metadata.go -- fixture producer -->\n")
			body := strings.Replace(specFixture(source), "| Status | design |", "| Status | in-progress |", 1) + "\n## Design documentation\n`docs/metadata.md`\n"
			const file = "plan/post-release/engineering/spec-metadata-proof.md"
			writeHookFixture(t, root, file, body)
			if answer, code := spec.Claim([]string{"spec-metadata-proof.md"}); code != 0 {
				t.Fatalf("native fixture claim = %d: %#v", code, answer)
			}
			writeHookFixture(t, root, file, test.transform(body))
			code, _, message := runHook(t, root, "validate-spec", map[string]any{"tool_name": toolMultiEdit, "tool_input": map[string]any{"file_path": filepath.Join(root, file)}})
			if code != test.want {
				t.Errorf("metadata validation = %d, want %d: %s", code, test.want, message)
			}
			code, message, found := Probe("writeSpecStatus", Payload{SessionID: "metadata-proof", ToolName: toolWrite, ToolInput: map[string]any{"file_path": filepath.Join(root, source), "content": "package probe\n"}}, root)
			if !found || code != test.want {
				t.Errorf("claimed source admission = %d, found %v, want %d: %s", code, found, test.want, message)
			}
		})
	}
}

// TestFunctionalProofUsesCanonicalTestIdentity prevents a test/ spelling from
// granting functional proof when its resolved target belongs elsewhere.
func TestFunctionalProofUsesCanonicalTestIdentity(t *testing.T) {
	for _, tool := range []string{toolWrite, toolEdit, toolMultiEdit} {
		for _, test := range []struct {
			name, proof string
			want        int
			diagnostic  string
		}{
			{"planned-valid", "test/ui/planned.ci", 0, ""},
			{"dot-escape", "test/../internal/le/fake.ci", 2, "functional test"},
			{"symlink-escape", "test/tooling-alias/fake.ci", 2, "functional test"},
			{"outside-proof", "test/outside-alias/fake.ci", 2, "outside"},
		} {
			t.Run(tool+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				const source = "internal/exabgp/bridge/bridge_status.go"
				writeHookFixture(t, root, source, "package bridge\n")
				writeHookFixture(t, root, "internal/le/neutral.go", "package neutral\n")
				writeHookFixture(t, root, "test/neutral.txt", "neutral fixture\n")
				for name, target := range map[string]string{"tooling-alias": "../internal/le", "outside-alias": t.TempDir()} {
					if err := os.Symlink(target, filepath.Join(root, "test", name)); err != nil {
						t.Fatal(err)
					}
				}
				body := strings.Replace(specFixture(source), "## Files to Modify\n\nOne.", "## Files to Modify\n\n- `"+source+"`", 1)
				body = strings.Replace(body, "## Data Flow", "## Existing Mechanisms\n\n| Mechanism | Source | Decision |\n|---|---|---|\n| Bridge | `"+source+"` | Extend |\n\n## Data Flow", 1)
				body += "\n## Proof\n`" + test.proof + "`\n"
				const file = "plan/post-release/engineering/spec-canonical-test-proof.md"
				const before = "# Before\n"
				writeHookFixture(t, root, file, before)
				code, message, found := Probe("writeSpecMechanisms", Payload{ToolName: tool, ToolInput: admissionReviewInput(tool, filepath.Join(root, file), before, body)}, root)
				if !found || code != test.want {
					t.Fatalf("canonical test proof = %d, found %v, want %d: %s", code, found, test.want, message)
				}
				if test.diagnostic != "" && !strings.Contains(message, test.diagnostic) {
					t.Fatalf("wrong canonical proof refusal: %s", message)
				}
			})
		}
	}
}

// TestSourceEditRefusedWhileClaimedSpecIsNotActive asks the registered
// source-edit gate about a Go write under a claimed spec whose metadata table
// is well formed. A spec still in design refuses the edit; one in verification
// admits it, as in-progress does.
//
// MUTATION: deleting the `if !oneOf(status, "in-progress", "verification")`
// check from writeSpecStatus admits the design spec, so the design case goes
// red; narrowing it to in-progress alone turns the verification case red.
func TestSourceEditRefusedWhileClaimedSpecIsNotActive(t *testing.T) {
	for _, test := range []struct {
		status string
		want   int
	}{
		{"design", 2},
		{"verification", 0},
	} {
		t.Run(test.status, func(t *testing.T) {
			const session = "status-proof"
			root := t.TempDir()
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", session)
			t.Setenv("ZE_AGENT_ID", "")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			writeHookFixture(t, root, "tmp/session/.session-"+session, "spec-status-proof.md\n")
			writeHookFixture(t, root, "plan/post-release/engineering/spec-status-proof.md",
				"| Field | Value |\n|---|---|\n| Status | "+test.status+" |\n")
			code, message, found := Probe("writeSpecStatus", Payload{
				SessionID: session, ToolName: toolWrite,
				ToolInput: map[string]any{"file_path": filepath.Join(root, "internal", "probe", "probe.go"), "content": "package probe\n"},
			}, root)
			if !found || code != test.want {
				t.Fatalf("source edit under a %s spec = %d, found %v, want %d: %s", test.status, code, found, test.want, message)
			}
			if test.want == 2 && !strings.Contains(message, "is not in-progress") {
				t.Fatalf("the refusal does not name the inactive status: %s", message)
			}
		})
	}
}
