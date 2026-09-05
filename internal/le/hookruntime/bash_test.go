// Related: bash.go -- the pretool-bash checks
//
// VALIDATES: the governed-document guard sees an in-place edit however its short
// flags are spelled, and still leaves a read of the same tree alone.
// PREVENTS: a clustered flag such as `perl -0pi` writing to plan/ or ai/rules/
// with none of the Write/Edit checks having run.
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGovernedWriteCatchesEveryInPlaceFlagSpelling drives the pretool-bash hook
// with one command for each spelling of the in-place flag that sed and perl
// accept. The method is the hook's own entry point rather than the regexp, so a
// change to the guard's wiring cannot leave this green.
func TestGovernedWriteCatchesEveryInPlaceFlagSpelling(t *testing.T) {
	blocked := []struct {
		name    string
		command string
	}{
		{"perl-bare-i", `perl -i -pe 's/a/b/' plan/spec-x.md`},
		{"perl-clustered-pi", `perl -pi -e 's/a/b/' plan/spec-x.md`},
		{"perl-clustered-0pi", `perl -0pi -e 's/a/b/' plan/spec-x.md`},
		{"perl-i-suffix", `perl -i.bak -pe 's/a/b/' plan/spec-x.md`},
		{"sed-bare-i", `sed -i 's/a/b/' plan/spec-x.md`},
		{"sed-clustered-Ei", `sed -Ei 's/a/b/' plan/spec-x.md`},
		{"sed-i-suffix", `sed -i.bak 's/a/b/' ai/rules/commands.md`},
	}
	for _, test := range blocked {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			code, _, message := runHook(t, root, "pretool-bash", map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": test.command},
			})
			if code != 2 {
				t.Fatalf("code = %d, want 2 for %q", code, test.command)
			}
			if !strings.Contains(message, "shell write to plan/ or ai/rules/") {
				t.Errorf("message did not name the guard: %s", message)
			}
		})
	}
}

// TestSpecCurrentBehaviorAcceptsEverySourceKindZeHas drives validateSpec with a
// minimal spec whose Current Behavior section lists one source file, once per
// file kind the repository actually contains. The method is the hook entry
// point, so the extension set and the section walk are exercised together.
//
// It exists because the set was go|sh|rs|ts|js|mk, which is the set a Go and
// shell repository has. Ze is a protocol router: a spec that read a YANG module,
// a .ci fixture or a templ template listed its sources correctly and was still
// refused for listing none.
func TestSpecCurrentBehaviorAcceptsEverySourceKindZeHas(t *testing.T) {
	kinds := []string{
		"internal/component/bgp/reactor/peer.go",
		"internal/component/iface/yang/ze-iface-conf.yang",
		"test/parse/iface-vpp-rejects-dhcp.ci",
		"internal/component/web/page.templ",
		"third_party/vpp-linux-cp/src/lcp_interface.c",
		"api/proto/ze.proto",
		"docs/architecture/core-design.md",
	}
	for _, source := range kinds {
		t.Run(filepath.Ext(source), func(t *testing.T) {
			root := t.TempDir()
			body := specFixture(source)
			path := filepath.Join(root, "plan", "spec-kind.md")
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, _, message := runHook(t, root, "validate-spec", map[string]any{
				"tool_name":  "Edit",
				"tool_input": map[string]any{"file_path": path, "new_string": "x"},
			})
			if strings.Contains(message, "Current Behavior section must list") {
				t.Errorf("a spec reading %s was refused for listing no source: %s", source, message)
			}
		})
	}
}

// TestSpecCurrentBehaviorStillRefusesASectionThatListsNothing pins the negative
// half, so the widened extension set cannot be mistaken for dropping the check.
func TestSpecCurrentBehaviorStillRefusesASectionThatListsNothing(t *testing.T) {
	root := t.TempDir()
	body := strings.Replace(specFixture("internal/component/bgp/reactor/peer.go"),
		"- [ ] `internal/component/bgp/reactor/peer.go`", "It was all read carefully.", 1)
	path := filepath.Join(root, "plan", "spec-kind.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, message := runHook(t, root, "validate-spec", map[string]any{
		"tool_name":  "Edit",
		"tool_input": map[string]any{"file_path": path, "new_string": "x"},
	})
	if !strings.Contains(message, "Current Behavior section must list") {
		t.Fatalf("a Current Behavior section naming no source was accepted: %s", message)
	}
}

// specFixture builds the smallest spec validateSpec accepts, with one source
// bullet in its Current Behavior section.
func specFixture(source string) string {
	return "# Spec: kind\n\n" +
		"| Field | Value |\n|-------|-------|\n| Status | design |\n| Updated | 2026-08-28 |\n\n" +
		"## Task\n\nOne sentence.\n\n" +
		"## Required Reading\n\nNothing.\n\n" +
		"## Current Behavior\n\n- [ ] `" + source + "` - read\n\n" +
		"## Data Flow\n\n### Entry Point\nHere.\n\n### Transformation Path\nThen.\n\n" +
		"### Boundaries Crossed\nOne.\n\n### Integration Points\nNone.\n\n" +
		"## Wiring Test\n\n| Entry | -> | Test |\n|---|---|---|\n| a | -> | b |\n\n" +
		"## 🧪 TDD Test Plan\n\n### Unit Tests\n\n| Test | File |\n|---|---|\n| a | b |\n\n" +
		"## Files to Modify\n\nOne.\n\n## Implementation Steps\n\nOne.\n\n" +
		"## Checklist\n\n- [ ] Tests written\n- [ ] Tests FAIL\n- [ ] Tests PASS\n" +
		"- [ ] `./le verify worktree` passes\n"
}

// TestNolintGuardAllowsTheFormTheStyleGuideRequires drives the write hook with
// each spelling of a nolint directive. The method is the hook entry point, and
// the file path is an ordinary Go source path so writeGoPatterns actually runs.
//
// The allowed half is what matters: `//nolint:<linter> // <reason>` is the form
// `docs/contributing/ze-go-style.md` requires for a deliberate discard, and the
// guard refused it, so the rule blocked the only spelling it exists to permit.
func TestNolintGuardAllowsTheFormTheStyleGuideRequires(t *testing.T) {
	const goPath = "/repo/internal/component/bgp/reactor/peer.go"
	cases := []struct {
		name    string
		content string
		blocked bool
	}{
		{"specific-linter-and-reason", "f, _ := open() //nolint:errcheck // the caller owns the handle\n", false},
		{"two-linters-and-reason", "x := y //nolint:gosec,errcheck // the path is tracked\n", false},
		{"bare", "f, _ := open() //nolint\n", true},
		{"bare-with-prose", "f, _ := open() //nolint // it is fine\n", true},
		{"linter-but-no-reason", "f, _ := open() //nolint:errcheck\n", true},
		{"linter-and-empty-reason", "f, _ := open() //nolint:errcheck //\n", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			code, _, message := runHook(t, root, "pretool-writeedit", map[string]any{
				"tool_name":  "Edit",
				"tool_input": map[string]any{"file_path": goPath, "new_string": test.content},
			})
			refused := strings.Contains(message, "nolint without a specific linter and reason")
			if refused != test.blocked {
				t.Fatalf("content %q: refused = %v (code %d), want %v: %s",
					test.content, refused, code, test.blocked, message)
			}
		})
	}
}

// TestGovernedWriteLeavesReadsAlone pins the negative half. A read of a governed
// tree carries no in-place flag, and the guard must not claim it.
func TestGovernedWriteLeavesReadsAlone(t *testing.T) {
	allowed := []struct {
		name    string
		command string
	}{
		{"sed-n-print", `sed -n '1,40p' plan/spec-x.md`},
		{"grep", `grep -n Status plan/spec-x.md`},
		{"perl-no-in-place", `perl -ne 'print if /Status/' plan/spec-x.md`},
	}
	for _, test := range allowed {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			_, _, message := runHook(t, root, "pretool-bash", map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": test.command},
			})
			if strings.Contains(message, "shell write to plan/ or ai/rules/") {
				t.Errorf("read was refused as a write: %q -> %s", test.command, message)
			}
		})
	}
}

// TestLossyPipeReadsTheTwoWordArea drives the pretool-bash hook with one piped
// command for each side of the area boundary. The method is the hook's own
// entry point, so the tokenizer, heavyArea and bashLossyPipe are judged
// together.
//
// It exists because `le verify-status` became `le verify status` when the
// commands were grouped, and the guard read only the first word: a certificate
// read piped into `tail` was refused with the message the verification gate
// owns, while nothing in the tree said the area had changed meaning.
func TestLossyPipeReadsTheTwoWordArea(t *testing.T) {
	tests := []struct {
		name    string
		command string
		blocked bool
	}{
		{"verify-gate", `./le verify current mode full | tail -5`, true},
		{"verify-lint", `./le verify lint run | grep issues`, true},
		{"verify-deps", `./le verify deps unit-cached | head -20`, true},
		{"verify-status", `./le verify status check | tail -1`, false},
		{"verify-summary", `./le verify summary append | head -2`, false},
		{"ze-le-verify-gate", `ze le verify current mode full | tail -5`, true},
		{"ze-le-verify-status", `ze le verify status check | tail -1`, false},
		{"functional-run", `./le functional gating | tail -5`, true},
		{"functional-listing", `./le functional | head -20`, false},
		{"functional-listing-redirected", `./le functional 2>&1 | head -20`, false},
		{"functional-run-redirected", `./le functional gating 2>&1 | tail -5`, true},
		{"unit-run", `./le test-unit all | tail -5`, true},
		{"unit-listing", `./le test-unit | head -20`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			code, _, message := runHook(t, root, "pretool-bash", map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": test.command},
			})
			refused := strings.Contains(message, "piping an expensive command's output")
			if refused != test.blocked {
				t.Fatalf("%q: refused = %t code = %d, want refused %t: %s",
					test.command, refused, code, test.blocked, message)
			}
		})
	}
}

// TestSpecValidationReachesEveryReleaseBucket is the fail-open the release
// buckets created.
//
// VALIDATES: a spec written under plan/immediate/ or plan/pre-release/ is
// validated exactly as one under plan/.
// PREVENTS: the silent version of no validation at all. The predicate was a
// regex spelling plan/ alone, and a path it did not match returned 0 with no
// output, so this hook reported success over every spec in two of the three
// buckets. The refusal below is what proves the hook READ the file.
func TestSpecValidationReachesEveryReleaseBucket(t *testing.T) {
	for _, bucket := range []string{"plan", "plan/immediate", "plan/pre-release"} {
		t.Run(bucket, func(t *testing.T) {
			root := t.TempDir()
			body := strings.Replace(specFixture("internal/component/bgp/reactor/peer.go"),
				"- [ ] `internal/component/bgp/reactor/peer.go`", "It was all read carefully.", 1)
			path := filepath.Join(root, filepath.FromSlash(bucket), "spec-kind.md")
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, _, message := runHook(t, root, "validate-spec", map[string]any{
				"tool_name":  "Edit",
				"tool_input": map[string]any{"file_path": path, "new_string": "x"},
			})
			if !strings.Contains(message, "Current Behavior section must list") {
				t.Fatalf("a spec under %s was written with no validation: %q", bucket, message)
			}
		})
	}
}
