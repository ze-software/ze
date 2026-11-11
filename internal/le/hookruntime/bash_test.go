// Related: bash.go -- the pretool-bash checks
//
// VALIDATES: the governed-document guard sees an in-place edit however its short
// flags are spelled, and still leaves a read of the same tree alone.
// PREVENTS: a clustered flag such as `perl -0pi` writing to plan/ or ai/rules/
// with none of the Write/Edit checks having run.
package hookruntime

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/derived"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	specpath "github.com/ze-software/ze/internal/le/spec/path"

	// The four harness commands the heaviness test reads: two runners
	// that admit, two helper tools that do not.
	_ "github.com/ze-software/ze/internal/le/mock/rpki"
	_ "github.com/ze-software/ze/internal/le/test/bgp"
	_ "github.com/ze-software/ze/internal/le/test/ospf"
	_ "github.com/ze-software/ze/internal/le/test/peer"
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
		{"perl-bare-i", `perl -i -pe 's/a/b/' plan/post-release/engineering/spec-x.md`},
		{"perl-clustered-pi", `perl -pi -e 's/a/b/' plan/post-release/engineering/spec-x.md`},
		{"perl-clustered-0pi", `perl -0pi -e 's/a/b/' plan/post-release/engineering/spec-x.md`},
		{"perl-i-suffix", `perl -i.bak -pe 's/a/b/' plan/post-release/engineering/spec-x.md`},
		{"sed-bare-i", `sed -i 's/a/b/' plan/post-release/engineering/spec-x.md`},
		{"sed-clustered-Ei", `sed -Ei 's/a/b/' plan/post-release/engineering/spec-x.md`},
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
			path := filepath.Join(root, "plan", "post-release", "engineering", "spec-kind.md")
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
	path := filepath.Join(root, "plan", "post-release", "engineering", "spec-kind.md")
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
		{"sed-n-print", `sed -n '1,40p' plan/post-release/engineering/spec-x.md`},
		{"grep", `grep -n Status plan/post-release/engineering/spec-x.md`},
		{"perl-no-in-place", `perl -ne 'print if /Status/' plan/post-release/engineering/spec-x.md`},
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

// TestGovernedWriteCatchesEveryScriptRuntime drives the pretool-bash hook with a
// script payload for each runtime the interpreter tier names. The method is the
// hook's own entry point, so the runtime list, the write-call list and the
// escape are judged together.
//
// It exists because the runtime list held perl and ruby alone, so a
// `python3 -c` or `python3 - <<EOF` payload wrote spec files with none of the
// Write/Edit checks having run, and Node had the same hole.
func TestGovernedWriteCatchesEveryScriptRuntime(t *testing.T) {
	tests := []struct {
		name    string
		command string
		blocked bool
	}{
		{"python3-c-open-write", `python3 -c "open('plan/post-release/engineering/spec-x.md','w').write('x')"`, true},
		{"python3-heredoc-write-text", "python3 - <<'EOF'\nfrom pathlib import Path\nPath('plan/post-release/engineering/spec-x.md').write_text('x')\nEOF", true},
		{"python3.12-os-replace", `python3.12 -c "import os; os.replace('scratch/x','ai/rules/commands.md')"`, true},
		{"python-shutil-copy", `python -c "import shutil; shutil.copy('x', 'plan/post-release/engineering/spec-x.md')"`, true},
		{"node-write-file-sync", `node -e "require('fs').writeFileSync('plan/post-release/engineering/spec-x.md', 'x')"`, true},
		{"node-append-file-sync", `node -e "require('fs').appendFileSync('ai/rules/commands.md', 'x')"`, true},
		{"perl-open-truncate-mode", `perl -e 'open(my $f, ">", "plan/post-release/engineering/spec-x.md"); print $f "x"'`, true},
		{"python3-read-plan", `python3 -c "print(open('plan/post-release/engineering/spec-x.md').read())"`, false},
		{"python3-read-ai-rules", `python3 -c "print(open('ai/rules/commands.md').read())"`, false},
		{"node-read-plan", `node -e "console.log(require('fs').readFileSync('plan/post-release/engineering/spec-x.md', 'utf8'))"`, false},
		{"python3-write-admitted", `ZE_ADMIT_GOVERNED_WRITE="reads plan/, writes scratch" python3 -c "open('plan/post-release/engineering/spec-x.md','w').write('x')"`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			code, _, message := runHook(t, root, "pretool-bash", map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": test.command},
			})
			refused := strings.Contains(message, "shell write to plan/ or ai/rules/")
			if refused != test.blocked {
				t.Fatalf("command %q: refused = %v (code %d), want %v: %s",
					test.command, refused, code, test.blocked, message)
			}
			if test.blocked && code != 2 {
				t.Fatalf("command %q: code = %d, want 2", test.command, code)
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
		{"verify-gate", `./le verify current | tail -5`, true},
		{"verify-lint", `./le go lint run | grep issues`, true},
		{"verify-deps", `./le verify deps unit-cached | head -20`, true},
		{"verify-status", `./le verify status check | tail -1`, false},
		{"verify-summary", `./le verify summary record | head -2`, false},
		{"ze-le-verify-gate", `ze le verify current | tail -5`, true},
		{"ze-le-verify-status", `ze le verify status check | tail -1`, false},
		{"functional-run", `./le test functional gating | tail -5`, true},
		{"functional-listing", `./le test functional | head -20`, false},
		{"functional-listing-redirected", `./le test functional 2>&1 | head -20`, false},
		{"functional-run-redirected", `./le test functional gating 2>&1 | tail -5`, true},
		{"unit-run", `./le test unit all | tail -5`, true},
		{"unit-listing", `./le test unit | head -20`, false},
		{"integration-run", `./le test integration all | tail -5`, true},
		{"qemu-run", `./le test qemu all | tail -5`, true},
		{"go-other", `./le go extract symbols | head -5`, false},
		{"verify-gate", `./le verify worktree | grep issues`, true},
		{"verify-changed", `./le verify changed | tail -5`, true},
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

// TestLossyPipeRefusalNamesTheScopedRun is the example the refusal teaches.
//
// VALIDATES: the lossy-filter refusal's example command is a scoped test run,
// the evidence an agent owes.
// PREVENTS: the refusal teaching an agent to start any `./le verify` command,
// which no agent runs (ai/META-DECISIONS.md).
func TestLossyPipeRefusalNamesTheScopedRun(t *testing.T) {
	_, _, message := runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "./le verify changed | tail -5"},
	})
	if !strings.Contains(message, `./le test unit <group> > "$dir/test.log"`) {
		t.Errorf("the refusal does not name the scoped run as its example: %s", message)
	}
	for _, full := range []string{"le verify"} {
		if strings.Contains(message, full) {
			t.Errorf("the refusal tells an agent to run %q: %s", full, message)
		}
	}
}

// TestSpecValidationReachesEveryReleaseBucket is the fail-open the release
// buckets created.
//
// VALIDATES: a spec in each declared current category is validated.
// PREVENTS: the silent version of no validation at all. The predicate was a
// regex spelling plan/ alone, and a path it did not match returned 0 with no
// output, so this hook reported success over every spec in two of the three
// buckets. The refusal below is what proves the hook READ the file.
func TestSpecValidationReachesEveryReleaseBucket(t *testing.T) {
	for _, bucket := range specpath.Dirs() {
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

// TestReadingAnAbsentArtifactMaterializesItBeforeTheCommandRuns drives the read
// half of the derived-artifact loop.
//
// A session's edits leave the artifact absent, so the grep that follows would
// read nothing. The hook runs BEFORE the command, which is what makes the
// rebuild reach the command rather than the one after it.
func TestReadingAnAbsentArtifactMaterializesItBeforeTheCommandRuns(t *testing.T) {
	root := derivedFixture(t)
	mapPath := filepath.Join(root, "ai", "PACKAGE-MAP.md")
	if err := os.Remove(mapPath); err != nil {
		t.Fatalf("clear the fixture map: %v", err)
	}

	payload := Payload{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "grep -n 'internal/core/x' ai/PACKAGE-MAP.md"},
	}
	code, message, found := Probe("preMaterializeDerived", payload, root)
	if !found {
		t.Fatal("preMaterializeDerived is not registered in nativeHookActions")
	}
	if code != 0 {
		t.Fatalf("the hook refused a grep of a registered artifact: %d %s", code, message)
	}

	body, err := os.ReadFile(mapPath) //nolint:gosec // a fixture path this test built under t.TempDir()
	if err != nil {
		t.Fatalf("the artifact the command names was not built: %v", err)
	}
	if !strings.Contains(string(body), "internal/core/x") {
		t.Errorf("the rebuilt map does not describe the fixture tree: %s", body)
	}
}

// TestArtifactWholeAsksADirectoryArtifactRatherThanStattingIt drives the
// decision the rebuild turns on, over each shape an artifact has.
//
// VALIDATES: an absent path is not whole; a present file with no question of
// its own is; a present DIRECTORY is whole only when its own Complete says the
// run that wrote it finished.
// PREVENTS: the read a stat licenses. A directory exists from the first of its
// files, so a run that stopped half way leaves one that is present and short,
// and the command that follows takes a partial set for the whole answer.
// test/runner/le-rfc-ledger-is-derived.ci drives the same decision through the
// shipped binary, over the real RFC generator.
func TestArtifactWholeAsksADirectoryArtifactRatherThanStattingIt(t *testing.T) {
	root := t.TempDir()
	writeHookFixture(t, root, "family/page.md", "# page\n")
	writeHookFixture(t, root, "family/members/one.md", "# one\n")

	file := derived.Artifact{Path: "family/page.md", Feeds: neverFeeds, Rebuild: neverRebuilds}
	absent := derived.Artifact{Path: "family/gone.md", Feeds: neverFeeds, Rebuild: neverRebuilds}
	finished := derived.Artifact{Path: "family/members", Feeds: neverFeeds, Rebuild: neverRebuilds,
		Complete: func(_ string) bool { return true }}
	unfinished := derived.Artifact{Path: "family/members", Feeds: neverFeeds, Rebuild: neverRebuilds,
		Complete: func(_ string) bool { return false }}

	for _, row := range []struct {
		name     string
		artifact derived.Artifact
		want     bool
	}{
		{"absent", absent, false},
		{"a present file answers its own presence", file, true},
		{"a finished directory", finished, true},
		{"a directory no run finished", unfinished, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			whole, err := row.artifact.Whole(root)
			if err != nil {
				t.Fatalf("Whole(%s): %v", row.artifact.Path, err)
			}
			if whole != row.want {
				t.Errorf("Whole(%s) = %v, want %v", row.artifact.Path, whole, row.want)
			}
		})
	}
}

func neverFeeds(_, _ string) bool { return false }

func neverRebuilds(_ string) error {
	return errors.New("BUG: this fixture's artifact is never rebuilt")
}

// TestMaterializingLeavesAPresentArtifactAlone is the other polarity: a read
// must not WRITE.
//
// A check that rewrote the artifact on every read would mutate the tree for
// every grep in a session, which is the `check-mode-mutates-the-tree` failure
// under a new name. The modification time is the assertion, because a rewrite
// with identical content is still a write.
func TestMaterializingLeavesAPresentArtifactAlone(t *testing.T) {
	root := derivedFixture(t)
	before := artifactStamp(t, root, "ai/PACKAGE-MAP.md")

	payload := Payload{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "grep -n 'internal/core/x' ai/PACKAGE-MAP.md"},
	}
	code, message, found := Probe("preMaterializeDerived", payload, root)
	if !found {
		t.Fatal("preMaterializeDerived is not registered in nativeHookActions")
	}
	if code != 0 {
		t.Fatalf("the hook refused a grep of a present artifact: %d %s", code, message)
	}

	if now := artifactStamp(t, root, "ai/PACKAGE-MAP.md"); !now.Equal(before) {
		t.Errorf("reading the artifact rewrote it: %v then %v", before, now)
	}
}

// TestAGrepOverTheDirectoryMaterializesTheArtifact covers the command form a
// session actually types when it does not know which index holds the answer.
//
// `grep -rn X ai/` names every artifact without spelling one, so a check that
// matches the full path alone rebuilds nothing and the recursive grep walks a
// directory the artifacts are missing from. What comes back is "no match",
// which is a wrong answer wearing the shape of a right one.
func TestAGrepOverTheDirectoryMaterializesTheArtifact(t *testing.T) {
	root := derivedFixture(t)
	mapPath := filepath.Join(root, "ai", "PACKAGE-MAP.md")
	if err := os.Remove(mapPath); err != nil {
		t.Fatalf("clear the fixture map: %v", err)
	}

	payload := Payload{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "grep -rn 'internal/core/x' ai/"},
	}
	code, message, found := Probe("preMaterializeDerived", payload, root)
	if !found {
		t.Fatal("preMaterializeDerived is not registered in nativeHookActions")
	}
	if code != 0 {
		t.Fatalf("the hook refused a recursive grep of the artifact directory: %d %s", code, message)
	}
	if _, err := os.Stat(mapPath); err != nil {
		t.Errorf("a grep of the directory holding the artifact built nothing: %v", err)
	}
}

// TestBashHookReadsNoHarnessFile proves the pretool hook holds no admission
// of a harness FILE: every harness command is `le test <name>` and no harness
// binary exists (D-8), so a command line that names one of the retired file
// names is not refused as the functional runner. That is also what ended the
// false positive on a grep or a heredoc that merely mentions the name
// (plan/journal/gate-fires-outside-its-population.md). The subject-first
// `le test <suite>` spellings still count as expensive.
//
// VALIDATES: D-8, the hook's harness-file admission is gone.
// PREVENTS: a text search for the retired name refused as a raw test run.
func TestBashHookReadsNoHarnessFile(t *testing.T) {
	refused := func(command string) bool {
		return bashRawHeavy(context{input: map[string]any{"command": command}}) != nil
	}
	for _, command := range []string{
		"grep -n 'le-test bgp' internal/le/hookruntime/bash.go",
		"ze-test bgp plugin 42",
	} {
		if refused(command) {
			t.Errorf("%q was refused as a raw harness run", command)
		}
	}

	for _, command := range []string{
		"./le test functional gating",
		"./le test integration interop",
		"./le test qemu run",
		"./le test unit all",
	} {
		if !commandExpensive(commandSegments(command)[0]) {
			t.Errorf("%q is not read as expensive", command)
		}
	}
	for _, command := range []string{"./le test functional", "./le test unit", "./le test"} {
		if commandExpensive(commandSegments(command)[0]) {
			t.Errorf("the listing %q is read as expensive", command)
		}
	}
}

// TestBashHookReadsHarnessSuitesAsHeavy proves the lossy-pipe refusal reads a
// harness runner as heavy and a helper tool as light, from the registration
// alone: `test bgp` and `test ospf` register their run as admitted
// (harnesstool.RunnerAnswer, SuiteAnswer), `test peer` and `mock rpki` do not.
// It drives bashLossyPipe, the hook's own entry point, over the command text.
//
// VALIDATES: AC-45, the hook half.
// PREVENTS: a suite piped through grep judged by its truncated output, or a
// helper tool's output refused as if it were a suite run.
func TestBashHookReadsHarnessSuitesAsHeavy(t *testing.T) {
	refused := func(command string) bool {
		return bashLossyPipe(context{input: map[string]any{"command": command}}) != nil
	}
	for _, name := range []string{"bgp", "ospf"} {
		if !leroot.Admits("test " + name) {
			t.Fatalf("test %s is not registered as admitted", name)
		}
		command := "./le test " + name + " plugin 42 | grep x"
		if !refused(command) {
			t.Errorf("%q was not refused: a runner area is heavy", command)
		}
	}
	for _, name := range []string{"test peer", "mock rpki"} {
		if leroot.Admits(name) {
			t.Fatalf("%s, a helper tool, is registered as admitted", name)
		}
		if !leroot.Forwards(name) {
			t.Fatalf("%s is not registered, so this case proves nothing", name)
		}
		command := "./le " + name + " --help | grep x"
		if refused(command) {
			t.Errorf("%q was refused: a helper tool is not heavy", command)
		}
	}
}

// testDeletionAsked drives the pretool-bash hook with one command and reports
// whether the test-deletion check asked about it.
func testDeletionAsked(t *testing.T, command string) (bool, string) {
	t.Helper()
	_, _, message := runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": command},
	})
	return strings.Contains(message, "Test deletion - user approval required"), message
}

// TestTestDeletionAsksOnlyForATestPath drives the pretool-bash hook with lines
// that delete a test and lines that do not. The method is the hook entry point,
// so the word split, the delete detection and the path test are judged together.
//
// It exists because the check matched regexps across the whole line: an
// `rm -rf bin/le-hookchk` followed by a `git status` that named
// docs/architecture/testing was asked about as a recursive test deletion. The
// fix narrows only what a test PATH is, so every way of reaching an rm that the
// line-wide check caught, through eval, a loop, a brace group or a redirect, is
// still asked about.
func TestTestDeletionAsksOnlyForATestPath(t *testing.T) {
	allowed := []string{
		`cd /Users/thomas/Code/github.com/ze-software/ze/main; rm -rf bin/le-hookchk; git status --short internal/le/commit internal/le/hookruntime docs/contributing/navigating-the-code.md docs/architecture/testing/verify-freshness-scope.md | cat; git diff --stat docs/contributing/navigating-the-code.md docs/architecture/testing/verify-freshness-scope.md`,
		`rm -rf bin/le-latest`,
		`rm -rf docs/architecture/testing/old`,
		`rm -f bin/x && go test ./internal/foo`,
		`rm -rf test/draft/wip.ci`,
		`echo 'rm internal/foo/foo_test.go'`,
		`rm -f bin/x # test/old`,
		`echo 'an unclosed quote with no delete`,
	}
	for _, command := range allowed {
		if asked, message := testDeletionAsked(t, command); asked {
			t.Errorf("%q was asked about as a test deletion: %s", command, message)
		}
	}
	asked := []string{
		`rm -rf internal/foo/foo_test.go`,
		`rm -r test/x`,
		`rm -r test/`,
		`rm -r testdata`,
		`ls; rm internal/x/testdata/y`,
		`rm 'a b_test.go'`,
		`rm "test/plugin/x.ci"`,
		`git rm test/editor/y.et`,
		`git -C /repo rm --cached internal/foo/foo_test.go`,
		`LC_ALL=C rm internal/foo/foo_test.go`,
		`rm -rf bin/x && rm -rf internal/foo/testdata`,
		`(cd internal && rm x/foo_test.go)`,
		`find . -name '*_test.go' | xargs rm`,
		`find internal -name '*_test.go' -exec rm {} +`,
		`sudo /bin/rm -rf /repo/test/x`,
		`git checkout -- internal/foo/foo_test.go`,
		`rm 'unclosed_quote`,
		`rm bin/x \`,
		// The line-wide check cannot tell a named rm from a run one, and asks.
		`grep -n rm internal/foo/foo_test.go`,
		`git log -- internal/foo/foo_test.go | grep rm`,
		// Ways of reaching an rm that a per-command parser missed.
		`eval rm internal/x/foo_test.go`,
		`if true; then rm internal/x/foo_test.go; fi`,
		`for f in internal/x/*_test.go; do rm "$f"; done`,
		`{ rm internal/x/foo_test.go; }`,
		`! rm internal/x/foo_test.go`,
		`rm internal/x/foo_test.go>/dev/null`,
		`rm 2>&1 internal/x/foo_test.go`,
		`rm -r internal/foo/*test*`,
		`find internal -name '*_test.go' -delete`,
		// Inside internal/ or test/, a directory whose name contains "test".
		`rm -rf internal/component/cli/testing`,
		`rm -rf internal/component/web/testing`,
		`rm -rf internal/le/chaos/selftest`,
		`rm -rf /repo/internal/le/chaos/selftest`,
		// A quoted rm is still the rm the shell runs.
		`'rm' internal/x/foo_test.go`,
		`"rm" -rf internal/component/cli/testing`,
	}
	for _, command := range asked {
		if asked, message := testDeletionAsked(t, command); !asked {
			t.Errorf("%q was not asked about as a test deletion: %s", command, message)
		}
	}
}

// TestSpecValidationRefusesObsoletePlacements ensures an old-layout write is
// refused before it can disappear from current discovery.
func TestSpecValidationRefusesObsoletePlacements(t *testing.T) {
	for _, directory := range []string{"plan", "plan/immediate", "plan/pre-release", "plan/to-review"} {
		root := t.TempDir()
		code, _, message := runHook(t, root, "validate-spec", map[string]any{
			"tool_name":  "Write",
			"tool_input": map[string]any{"file_path": filepath.Join(root, directory, "spec-misplaced.md")},
		})
		if code != 2 || !strings.Contains(message, "obsolete spec directory") {
			t.Fatalf("%s: code %d, message %q", directory, code, message)
		}
	}
}

// guardVerdict runs one Bash guard alone over a command and a checkout root.
func guardVerdict(check hookCheck, root, command string) *verdict {
	return check(context{root: root, input: map[string]any{"command": command}})
}

// goBuildTree writes a module under a fresh root: two library packages and one
// main package, the packages the go build tests name.
func goBuildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for path, body := range map[string]string{
		"go.mod":          "module example.com/probe\n\ngo 1.24\n",
		"internal/x/x.go": "package x\n",
		"internal/y/y.go": "package y\n",
		"cmd/z/main.go":   "package main\n\nfunc main() {}\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// guardRow is one command and whether the guard named must refuse it.
type guardRow struct {
	guard   string
	check   hookCheck
	command string
	refused bool
}

// judgeGuardRows runs each row's guard alone over the row's command.
func judgeGuardRows(t *testing.T, root string, rows []guardRow) {
	t.Helper()
	for _, row := range rows {
		result := guardVerdict(row.check, root, row.command)
		if (result != nil) != row.refused {
			message := ""
			if result != nil {
				message = result.message
			}
			t.Errorf("%s: %q refused = %t, want %t: %s", row.guard, row.command, result != nil, row.refused, message)
		}
	}
}

// TestGuardsIgnoreCommandNamesInsideQuotes drives the build, piped-command,
// raw-job and /tmp guards with a quoted grep pattern that names what each guard
// refuses, and with the real command beside it.
//
// VALIDATES: AC-7. A guard lifts its text refusal on a line of readers
// (readerLine), so a name or a separator inside quotes is text there:
// `grep -n -E 'rebuild|go build' le` is a grep.
// PREVENTS: the guards splitting on `|`, `;` and `&&` blind to quotes, or
// matching `go build` anywhere in the text, which refused every search for the
// rule they enforce. Each pass row was refused before phase 2.
func TestGuardsIgnoreCommandNamesInsideQuotes(t *testing.T) {
	root := goBuildTree(t)
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `grep -rn "go build" plan/ | head -5`, false},
		{"build", bashRootBuild, `grep -n -E 'rebuild|go build' le`, false},
		{"build", bashRootBuild, `go build ./cmd/z`, true},
		{"build", bashRootBuild, `time go build ./cmd/z`, true},
		{"pipe", bashLossyPipe, `grep -rn "go build" plan/ | head -5`, false},
		{"pipe", bashLossyPipe, `grep -n 'ok | go test ./x | head -5' f`, false},
		{"pipe", bashLossyPipe, `grep -n 'ok | golangci-lint run | head -5' f`, false},
		{"pipe", bashLossyPipe, `go test ./x | tail -5`, true},
		{"pipe", bashLossyPipe, `echo 'a|b'; go test ./x | tail -5`, true},
		{"raw", bashRawHeavy, `grep -n 'ok; go test ./x' f`, false},
		{"raw", bashRawHeavy, `grep -n 'ok | golangci-lint run' f`, false},
		{"raw", bashRawHeavy, `go test ./x`, true},
		{"raw", bashRawHeavy, `echo 'a;b'; go test ./x`, true},
		{"tmp", bashSystemTmp, `grep -n 'writes /tmp/x' f`, false},
		{"tmp", bashSystemTmp, `echo x > /tmp/y`, true},
	})
	code, _, message := runHook(t, root, "pretool-bash", map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": `grep -n -E 'rebuild|go build' le`},
	})
	if code != 0 {
		t.Errorf("the hook refused a grep for the build rule: code %d: %s", code, message)
	}
}

// TestGuardsJudgeAnUnparsableLineByItsText drives each changed guard with a
// line lineCommands cannot read whole, an unclosed quote or a command
// substitution, and with a string handed to another shell.
//
// VALIDATES: AC-8. Such a line is never a reader line, so each guard judges it
// by its text and refuses what it refused before it judged commands.
// PREVENTS: a guard clearing a command it cannot see, which is the failure that
// matters most: a guard that cannot tell where a word ends must not pass it.
func TestGuardsJudgeAnUnparsableLineByItsText(t *testing.T) {
	root := goBuildTree(t)
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `grep 'go build x`, true},
		{"build", bashRootBuild, `x="$(go build ./internal/x)"`, true},
		{"build", bashRootBuild, `bash -c 'go build ./cmd/z'`, true},
		{"pipe", bashLossyPipe, `go test ./x | tail -5 'oops`, true},
		{"pipe", bashLossyPipe, `bash -c 'true'; go test ./x | tail -5`, true},
		{"raw", bashRawHeavy, `go test ./x 'oops`, true},
		{"tmp", bashSystemTmp, `grep 'writes /tmp/x`, true},
		{"tmp", bashSystemTmp, `x="$(cat /tmp/x)"`, true},
		{"build", bashRootBuild, "cat <<EOF\nit's\nEOF\ngo build ./cmd/z", true},
	})
}

// TestGuardsSeeACommandHandedToAShell drives the guards with a command string
// handed to a shell in the shapes a fixed pattern missed: options before -c,
// and a path before the shell's name.
//
// VALIDATES: a shell is no reader, so its line keeps each guard's text
// judgment, which reads a guarded name wherever it stands, inside the quotes
// included. A scoped build handed to a shell is refused too: only a line of
// readers and builds is judged by its words.
// PREVENTS: `/bin/bash -c 'go build ./cmd/ze'` passing as one quoted word.
func TestGuardsSeeACommandHandedToAShell(t *testing.T) {
	root := goBuildTree(t)
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `bash -o pipefail -c 'go build ./cmd/z'`, true},
		{"build", bashRootBuild, `bash -e -c 'go build ./cmd/z'`, true},
		{"build", bashRootBuild, `/bin/bash -c 'go build ./cmd/z'`, true},
		{"build", bashRootBuild, `eval 'go build ./cmd/z'`, true},
		{"build", bashRootBuild, `bash -c 'go build ./internal/x'`, true},
		{"tmp", bashSystemTmp, `/bin/sh -c 'cat /tmp/x'`, true},
	})
}

// TestGuardsKeepTheTextVerdictOfHead drives the guards with the lines three
// reviews of phase 2 found cleared by an edit to the text judgment: a quoted
// GOFLAGS value, an assignment whose value names ZE_ADMIT_RAW, and a /tmp
// path after a quote that closes or opens part of a word.
//
// VALIDATES: R1 of the phase 2 brief. The text judgment of every guard is the
// one HEAD carried before phase 2, so each line it refused is refused again.
// PREVENTS: a text judgment loosened to clear a quoted name on one line and
// clearing a real violation on another.
func TestGuardsKeepTheTextVerdictOfHead(t *testing.T) {
	root := goBuildTree(t)
	judgeGuardRows(t, root, []guardRow{
		{"raw", bashRawHeavy, `GOFLAGS="-race -count=1" go test ./internal/x/`, true},
		{"raw", bashRawHeavy, `X="a ZE_ADMIT_RAW=1" go test ./...`, true},
		{"pipe", bashLossyPipe, `GOFLAGS="-a -b" go test ./x | tail`, true},
		{"tmp", bashSystemTmp, `curl -o"/tmp/f" u`, true},
		{"tmp", bashSystemTmp, `go build -o"/tmp/x" .`, true},
		{"tmp", bashSystemTmp, `cat "$EMPTY"/tmp/x`, true},
	})
}

// TestGuardsKeepTheTextVerdictBesideAReaderItsJudgeRefuses drives the guards
// with lines whose only command is a reader carrying an option or a word its
// judge does not admit, each naming a guarded command in its arguments, and
// with the same readers in the shapes their judges admit.
//
// VALIDATES: R2 of the phase 2 brief. A judge is an allow-list: an
// abbreviated long option (`--open=bash`, `--compress=sh`, `--co=sh`), an
// option that runs a program or evaluates a subscript (printf and test -v,
// sed `e`, find -delete, diff -l, rg --pre, gopls execute), a second uniq
// operand and a word the shell expands each leave the line to the text
// judgment, which refuses it.
// PREVENTS: a deny-list judge clearing a line on which the reader runs what
// its argument names.
func TestGuardsKeepTheTextVerdictBesideAReaderItsJudgeRefuses(t *testing.T) {
	root := goBuildTree(t)
	rows := make([]guardRow, 0, 32)
	for _, command := range []string{
		`git grep --open=bash 'go build ./cmd/z'`,
		`git log --ext 'go build ./cmd/z'`,
		`git log -p $_ 'go build ./cmd/z'`,
		`sort --compress=sh 'go build ./cmd/z'`,
		`sort --co=sh 'go build ./cmd/z'`,
		`printf -v 'a[$(go build ./cmd/z)]' v`,
		`printf $_ 'a[$(go build ./cmd/z)]' v`,
		`test -v 'a[$(go build ./cmd/z)]'`,
		`[ -v 'a[$(go build ./cmd/z)]' ]`,
		`sed -n '1e go build ./cmd/z' f`,
		`sed -n 's/x/go build .\/cmd\/z/e' f`,
		`find . -name 'go build' -delete`,
		`diff -l 'go build' f`,
		`diff --pag 'go build' f`,
		`rg --pre=sh 'go build' f`,
		`rg -z 'go build' f`,
		`rg -z 'go build'`,
		`rg -iz 'go build'`,
		`gopls execute 'go build ./cmd/z'`,
		`gopls -remote=auto symbols 'go build'`,
		`uniq 'go build' out`,
		`cat 'go build' > f`,
		`echo 'go build' >> f`,
		`printf 'go build' >| f`,
		`cat 'go build' &> f`,
		`cat 'go build' 2> f`,
		`echo 'go build'\>|sh`,
		`echo 'go build' 'x'\>|sh`,
		`echo "go build>"|sh`,
		`sort 'go build' ~/x`,
	} {
		rows = append(rows, guardRow{"build", bashRootBuild, command, true})
	}
	for _, command := range []string{
		`git grep -n 'go build' -- '*.go'`,
		`git log --oneline -5 --grep 'go build'`,
		`sort -t, -k2 'go build'`,
		`sed -n '/go build/p' f`,
		`sed 's/go build/x/g' f`,
		`rg -n 'go build' -g '*.go'`,
		`find . -name 'go build' -mtime -1`,
		`diff -u 'go build' f`,
		`gopls references -d 'go build'`,
		`printf '%s\n' 'go build'`,
		`test -n 'go build'`,
		`uniq -c 'go build'`,
		`cat 'go build' > /dev/null`,
		`grep 'go build' f 2>&1`,
	} {
		rows = append(rows, guardRow{"build", bashRootBuild, command, false})
	}
	judgeGuardRows(t, root, rows)
}

// TestRootBuildFollowsACdOnlyWhereItRuns drives the build guard with a cd
// before a build, joined by each separator.
//
// VALIDATES: R3 of the phase 2 brief. A cd is followed only when it starts
// the line or follows `;` or a newline, and `;`, `&&` or a newline ends it.
// A cd in a pipeline, behind `&`, after `&&` or `||`, before `||`, inside a
// group, or given -P leaves the directory unknown, so a relative -o is
// refused and only an absolute -o under the checkout's bin/ passes.
// PREVENTS: `cd cmd/z | go build -o ../../bin/z .` resolved from cmd/z while
// the cd ran in a subshell and the build writes two levels above the root.
func TestRootBuildFollowsACdOnlyWhereItRuns(t *testing.T) {
	root := goBuildTree(t)
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `cd cmd/z; go build -o ../../bin/z .`, false},
		{"build", bashRootBuild, "cd cmd/z\ngo build -o ../../bin/z .", false},
		{"build", bashRootBuild, `cd cmd/z && go build -o ../../bin/z .`, false},
		{"build", bashRootBuild, `cd cmd/z | go build -o ../../bin/z .`, true},
		{"build", bashRootBuild, `cd cmd/z & go build -o ../../bin/z .`, true},
		{"build", bashRootBuild, `true && cd cmd/z; go build -o ../../bin/z .`, true},
		{"build", bashRootBuild, `true || cd cmd/z; go build -o ../../bin/z .`, true},
		{"build", bashRootBuild, `cd cmd/z || go build -o ../../bin/z .`, true},
		{"build", bashRootBuild, `{ cd cmd/z; }; go build -o ../../bin/z .`, true},
		{"build", bashRootBuild, `cd -P cmd/z; go build -o ../../bin/z .`, true},
		{"build", bashRootBuild, `cd cmd/z | go build -o ` + root + `/bin/z .`, false},
		{"build", bashRootBuild, `cd cmd/z & go build -o ` + root + `/cmd/z/z .`, true},
	})
}

// TestRootBuildJudgesAnEnvironmentOrAReplacedInputByItsText drives the build
// guard with variables in front of a build and with the flags that hand the
// go command a program or a replacement for what the hook reads.
//
// VALIDATES: R3 of the phase 2 brief. Only GOFLAGS is read in front of a
// build; any other assignment, and -toolexec, -overlay or -modfile on the
// command line or in GOFLAGS, sends the line to the text judgment, which
// refuses a build with no output under bin/ and passes one with it.
// PREVENTS: `GOOS=windows go build ./x` judged by the packages the hook reads
// under the host's GOOS, where the files the build compiles may differ.
func TestRootBuildJudgesAnEnvironmentOrAReplacedInputByItsText(t *testing.T) {
	root := goBuildTree(t)
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `CGO_ENABLED=0 go build ./internal/x`, true},
		{"build", bashRootBuild, `GOOS=linux go build ./internal/x`, true},
		{"build", bashRootBuild, `GOOS=linux GOARCH=arm64 go build ./internal/x`, true},
		{"build", bashRootBuild, `GOEXPERIMENT=x go build ./internal/x`, true},
		{"build", bashRootBuild, `go build -overlay o.json ./internal/x`, true},
		{"build", bashRootBuild, `go build -modfile=alt.mod ./internal/x`, true},
		{"build", bashRootBuild, `GOFLAGS=-modfile=alt.mod go build ./internal/x`, true},
		{"build", bashRootBuild, `GOFLAGS=-v go build ./internal/x`, false},
		{"build", bashRootBuild, `GOOS=linux go build -o bin/z ./cmd/z`, false},
	})
}

// TestRootBuildResolvesSymbolicLinks drives the build guard with a cd through
// a link, an -o through a link, an -o through a link loop, and a bin/ that is
// itself a link out of the checkout.
//
// VALIDATES: R3 of the phase 2 brief. The working directory and the -o path
// are resolved as the kernel resolves them, every link followed, before the
// inside-the-checkout check, and an -o must land in bin/ both as typed and
// cleaned; a link that cannot be resolved refuses the build.
// PREVENTS: `cd lnk && go build -o ../bin/z .` judged as root/bin/z by text
// while the kernel climbs from the link's target and writes cmd/bin/z.
func TestRootBuildResolvesSymbolicLinks(t *testing.T) {
	root := goBuildTree(t)
	outside := t.TempDir()
	for link, target := range map[string]string{
		"lnk":  filepath.Join(root, "cmd", "z"),
		"out":  outside,
		"loop": filepath.Join(root, "loop"),
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `cd lnk && go build -o ../bin/z .`, true},
		{"build", bashRootBuild, `cd lnk && go build -o ../../bin/z .`, false},
		{"build", bashRootBuild, `go build -o out/z ./cmd/z`, true},
		{"build", bashRootBuild, `go build -o lnk/../bin/z ./cmd/z`, true},
		{"build", bashRootBuild, `go build -o loop/z ./cmd/z`, true},
		{"build", bashRootBuild, `go build -o bin/$X ./cmd/z`, true},
	})
	linked := goBuildTree(t)
	if err := os.Symlink(outside, filepath.Join(linked, "bin")); err != nil {
		t.Fatal(err)
	}
	judgeGuardRows(t, linked, []guardRow{
		{"build", bashRootBuild, `go build -o bin/z ./cmd/z`, true},
	})
}

// TestRootBuildJudgesALineThatWritesAFileByItsText drives the build guard
// with a reader+build line whose redirection writes a file, and with the
// redirections that write only to a descriptor or /dev/null.
//
// VALIDATES: a reader on a build's line could write the package the hook reads
// before the build compiles it, so a line with a build and a redirection to a
// file keeps the text judgment, which refuses a build with no output in bin/.
// PREVENTS: `echo 'package main...' > internal/x/x.go && go build ./internal/x`
// judged as a library compile and writing a binary into the working directory.
func TestRootBuildJudgesALineThatWritesAFileByItsText(t *testing.T) {
	root := goBuildTree(t)
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `echo 'package main' > internal/x/x.go && go build ./internal/x`, true},
		{"build", bashRootBuild, `go build ./internal/x > log`, true},
		{"build", bashRootBuild, `cat a >& b; go build ./internal/x`, true},
		{"build", bashRootBuild, `go build ./internal/x 2>&1`, false},
		{"build", bashRootBuild, `go build ./internal/x 2>/dev/null`, false},
		{"build", bashRootBuild, `go build ./internal/x &> /dev/null`, false},
		{"build", bashRootBuild, `grep -c x f > count && go build -o bin/z ./cmd/z`, false},
	})
}

// TestRootBuildRefusesWhatItCannotResolve drives the build guard with builds
// whose packages the hook cannot place: a build after a cd, a build whose
// packages hold no Go file under its tags, and tags set through GOFLAGS.
//
// VALIDATES: a build that resolves no package is refused, a build after an
// earlier cd is refused unless its -o resolves to the checkout's bin/ from
// the directory the cd moved to and the -C directory, an -o the hook cannot
// resolve (a cd to a variable, a cd inside a subshell) is refused, a pushd
// line is judged by its text, and GOFLAGS on the line or in the environment
// sets the tags the package is read with.
// PREVENTS: `cd cmd/ze && go build .` resolved from the checkout root, read as
// a directory with no package and cleared, while it writes cmd/ze/ze.
func TestRootBuildRefusesWhatItCannotResolve(t *testing.T) {
	root := goBuildTree(t)
	for path, body := range map[string]string{
		"internal/w/w.go":    "package w\n",
		"internal/w/main.go": "//go:build probe\n\npackage main\n\nfunc main() {}\n",
		"internal/t/t.go":    "//go:build probe\n\npackage t\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `cd cmd/z && go build .`, true},
		{"build", bashRootBuild, `cd internal && go build ./x`, true},
		{"build", bashRootBuild, `pushd cmd; go build ./z`, true},
		{"build", bashRootBuild, `go build ./internal/t`, true},
		{"build", bashRootBuild, `go build ./internal/w`, false},
		{"build", bashRootBuild, `GOFLAGS=-tags=probe go build ./internal/w`, true},
		{"build", bashRootBuild, `env GOFLAGS=-tags=probe go build ./internal/w`, true},
		{"build", bashRootBuild, `cd cmd/z && go build -o bin/z .`, true},
		{"build", bashRootBuild, `cd cmd/z && go build -o ../../bin/z .`, false},
		{"build", bashRootBuild, `go build -C cmd/z -o bin/z .`, true},
		{"build", bashRootBuild, `go build -C cmd/z -o ../../bin/z .`, false},
		{"build", bashRootBuild, `cd "$X" && go build -o bin/z .`, true},
		{"build", bashRootBuild, `(cd cmd/z) && go build -o ../../bin/z .`, true},
		{"build", bashRootBuild, `go build -o ` + root + `/bin/z ./cmd/z`, false},
		{"build", bashRootBuild, `go build -o ` + root + `/cmd/z/z ./cmd/z`, true},
	})
	t.Setenv("GOFLAGS", "-tags=probe")
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `go build ./internal/w`, true},
	})
}

// TestRootBuildRefusesAWholeTreePatternBySpelling drives the build guard with
// recursive patterns that reach the module root without spelling `./...`.
//
// VALIDATES: a `/...` pattern whose directory, cleaned, is the module root, by
// import path or by path, is a whole-tree build.
// PREVENTS: `go build github.com/ze-software/ze/...` walked as a pattern and
// cleared because it holds more than one package.
func TestRootBuildRefusesAWholeTreePatternBySpelling(t *testing.T) {
	root := goBuildTree(t)
	for _, command := range []string{
		`go build example.com/probe/...`,
		`go build .//...`,
		`go build ./cmd/../...`,
		`go build ` + root + `/...`,
	} {
		result := guardVerdict(bashRootBuild, root, command)
		if result == nil {
			t.Errorf("%q was not refused", command)
			continue
		}
		if !strings.Contains(result.message, "./le repo changed scope print both") {
			t.Errorf("%q: not refused as a whole-tree build: %s", command, result.message)
		}
	}
	if guardVerdict(bashRootBuild, root, `go build $PWD/...`) == nil {
		t.Errorf("a pattern under an unexpanded variable was cleared")
	}
}

// TestRootBuildAllowsPackagesThatProduceNoBinary drives the build guard with
// compiles that write nothing to the working directory.
//
// VALIDATES: AC-9 and owner decision D-4. The go command writes a binary only
// for one main package ("go help build"), so a non-main package and several
// packages pass, by path, by pattern and by import path, as do the -o bin/ and
// session bin/ forms.
// PREVENTS: a scoped compile check refused as a stray binary, which sent agents
// to a whole-tree build the old guard exempted.
func TestRootBuildAllowsPackagesThatProduceNoBinary(t *testing.T) {
	root := goBuildTree(t)
	rows := make([]guardRow, 0, 10)
	for _, command := range []string{
		`go build ./internal/x/`,
		`go build ./internal/x ./internal/y`,
		`go build -tags ze_bgp ./internal/x`,
		`go build ./internal/...`,
		`go build example.com/probe/internal/x`,
		`go build -C internal ./x`,
		`GOFLAGS=-v go build ./internal/x 2>&1`,
		`go build ./cmd/z ./internal/x`,
		`go build -o bin/z ./cmd/z`,
		`go build -o tmp/session/2026-08-10-abc123/bin/z ./cmd/z`,
	} {
		rows = append(rows, guardRow{"build", bashRootBuild, command, false})
	}
	judgeGuardRows(t, root, rows)
}

// TestRootBuildRefusalNamesTheCallersPackage drives the build guard with
// compiles that write a binary outside bin/, and reads the advice.
//
// VALIDATES: AC-10. One main package with no -o, or with -o outside bin/, and a
// build that names no package, are refused, and the advice carries the
// caller's own package in the `go build -o bin/<name> <package>` form.
// PREVENTS: a refusal that names a placeholder the agent must translate, and a
// package the hook cannot resolve being cleared as a library.
func TestRootBuildRefusalNamesTheCallersPackage(t *testing.T) {
	root := goBuildTree(t)
	for _, row := range []struct{ command, advice string }{
		{`go build ./cmd/z`, `go build -o bin/z ./cmd/z`},
		{`GOFLAGS=-v go build ./cmd/z`, `go build -o bin/z ./cmd/z`},
		{`go build -v ./cmd/...`, `go build -o bin/z ./cmd/z`},
		{`go build example.com/probe/cmd/z`, `go build -o bin/z example.com/probe/cmd/z`},
		{`go build -o z ./cmd/z`, `go build -o bin/z ./cmd/z`},
		{`go build ./cmd/missing`, `go build -o bin/missing ./cmd/missing`},
		{`go build`, `names no package`},
	} {
		result := guardVerdict(bashRootBuild, root, row.command)
		if result == nil {
			t.Errorf("%q was not refused", row.command)
			continue
		}
		if !strings.Contains(result.message, "go build without -o bin/") {
			t.Errorf("%q: the refusal does not say what is wrong: %s", row.command, result.message)
		}
		if !strings.Contains(result.message, row.advice) {
			t.Errorf("%q: the refusal does not carry %q: %s", row.command, row.advice, result.message)
		}
	}
}

// TestRootBuildRefusesTheWholeTree drives the build guard with whole-tree
// compiles, with and without an output under bin/.
//
// VALIDATES: AC-11 and owner decision D-4: the guard no longer exempts
// `go build ./...`, and its refusal names the command that finds the packages
// a change reaches.
// PREVENTS: a session compiling the whole tree, the release check's job.
func TestRootBuildRefusesTheWholeTree(t *testing.T) {
	root := goBuildTree(t)
	for _, command := range []string{
		`go build ./...`,
		`go build -o bin/ ./...`,
		`go build -v ./...`,
		`cd internal && go build ./...`,
	} {
		result := guardVerdict(bashRootBuild, root, command)
		if result == nil {
			t.Errorf("%q was not refused", command)
			continue
		}
		if !strings.Contains(result.message, "./le repo changed scope print both") {
			t.Errorf("%q: the refusal does not name the scoped command: %s", command, result.message)
		}
	}
}

// TestSystemTmpIgnoresATmpDirectoryUnderAnotherPath drives the /tmp guard with
// words whose path holds /tmp under another directory, and with words that
// name the system directory.
//
// VALIDATES: AC-16 as R4 of the phase 2 brief narrows it. On a reader line a
// word, quotes removed, that is /tmp or starts with /tmp/ is refused, and so
// is such a value after `=`, `:` or `,`; a /tmp under a literal parent
// passes, while one under a parent the shell expands stays refused, because
// `"$D"/tmp/x` is /tmp/x when D is empty.
// PREVENTS: `cat foo/tmp/x` refused as the system directory, and a variable
// parent cleared as another directory.
func TestSystemTmpIgnoresATmpDirectoryUnderAnotherPath(t *testing.T) {
	root := t.TempDir()
	judgeGuardRows(t, root, []guardRow{
		{"tmp", bashSystemTmp, `cat "$CLAUDE_JOB_DIR"/tmp/x`, true},
		{"tmp", bashSystemTmp, `ls "$HOME"/tmp`, true},
		{"tmp", bashSystemTmp, `cat "${v}/tmp/x"`, true},
		{"tmp", bashSystemTmp, `cat ~/tmp/x`, true},
		{"tmp", bashSystemTmp, `ls build/tmp/x`, false},
		{"tmp", bashSystemTmp, `cat foo/tmp/x`, false},
		{"tmp", bashSystemTmp, `ls ./a/tmp`, false},
		{"tmp", bashSystemTmp, `cat tmp/x`, false},
		{"tmp", bashSystemTmp, `cat /tmp/x`, true},
		{"tmp", bashSystemTmp, `cat "/tmp/x"`, true},
		{"tmp", bashSystemTmp, `ls /tmp`, true},
		{"tmp", bashSystemTmp, `cp a --target-directory=/tmp/x`, true},
		{"tmp", bashSystemTmp, `PATH=/bin:/tmp/bin ls`, true},
		{"tmp", bashSystemTmp, `cat x 2>/tmp/err`, true},
		{"tmp", bashSystemTmp, `cat x >/tmp/f`, true},
		{"tmp", bashSystemTmp, `cat x 2>>/tmp/f`, true},
		{"tmp", bashSystemTmp, `sort </tmp/f`, true},
	})
}

// writeGoFiles writes each file under root, its directories created.
func writeGoFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, body := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestGuardsJudgeALineThatCanRunAStringByItsText drives the guards with lines
// where a command other than a reader may run a string it was handed: a
// shell by -c, by stdin, by a here-string, a here-document or an rc file, a
// launcher, an interpreter, sed's `e` and `w` commands, awk, eval, a go build
// -toolexec program and a compiler variable.
//
// VALIDATES: a guard lifts its text refusal only on a line readerLine admits,
// so every such line is judged by its text and refused as the text match
// refused it, while a reader line naming the same words passes.
// PREVENTS: the defect class two reviews found: a parse that clears a command
// the text guard refused, because something on the line runs what the parser
// reads as one quoted word. Patching shell after shell does not close it;
// judging every non-reader line by its text does.
func TestGuardsJudgeALineThatCanRunAStringByItsText(t *testing.T) {
	root := goBuildTree(t)
	rows := make([]guardRow, 0, 48)
	for _, command := range []string{
		`bash -c 'go build -o bin/x ./...'`,
		`nice eval 'go build ./cmd/z'`,
		`echo 'go build ./cmd/z' | bash`,
		`echo 'go build ./cmd/z' | sh -s`,
		`bash <<< 'go build ./cmd/z'`,
		"bash <<EOF\ngo build ./cmd/z\nEOF",
		`bash --rcfile f -c 'go build ./cmd/z'`,
		`bash --init-file f -c 'go build ./cmd/z'`,
		`watch -n1 'go build ./cmd/z'`,
		`su -c 'go build ./cmd/z' builder`,
		`script -qc 'go build ./cmd/z' log`,
		`flock lock -c 'go build ./cmd/z'`,
		`ash -c 'go build ./cmd/z'`,
		`mksh -c 'go build ./cmd/z'`,
		`tcsh -c 'go build ./cmd/z'`,
		`python3 -c 'import os; os.system("go build ./cmd/z")'`,
		`sed -n '1e go build ./cmd/z' f`,
		`go build -toolexec 'bash -c "go build ./cmd/z"' ./internal/x`,
		`CC='sh -c x' go build ./internal/x`,
		`go build ./internal/x && bash -c true`,
	} {
		rows = append(rows, guardRow{"build", bashRootBuild, command, true})
	}
	for _, command := range []string{
		`grep eval -- 'go build'`,
		`grep -n 'bash -c go build ./cmd/z' f`,
		`go build ./internal/x && grep -c x f`,
		`sed -n '/go build/p' f`,
	} {
		rows = append(rows, guardRow{"build", bashRootBuild, command, false})
	}
	rows = append(rows,
		guardRow{"tmp", bashSystemTmp, `bash --rcfile f -c 'cat /tmp/x'`, true},
		guardRow{"tmp", bashSystemTmp, `awk '{print > "/tmp/x"}' f`, true},
		guardRow{"tmp", bashSystemTmp, `sed -n 'w /tmp/x' f`, true},
		guardRow{"tmp", bashSystemTmp, `python3 -c 'open("/tmp/x", "w")'`, true},
		guardRow{"tmp", bashSystemTmp, `echo x >"/tmp/y"`, true},
		guardRow{"tmp", bashSystemTmp, `mkdir -p "$D"/tmp/x`, true},
		guardRow{"pipe", bashLossyPipe, `grep -n 'go test ./x | tail' f`, false},
		guardRow{"pipe", bashLossyPipe, `echo 'a|b' | go test ./x | tail -5`, true},
		guardRow{"raw", bashRawHeavy, `grep -n 'eval go test ./x' f`, false},
	)
	judgeGuardRows(t, root, rows)
}

// TestRootBuildStaysInsideTheCheckout drives the build guard with packages
// the hook must not read: an absolute path outside the checkout, the
// filesystem root, the checkout's tmp/ and a module nested below the root.
//
// VALIDATES: a package directory outside the checkout, a `/...` pattern under
// its tmp/, and a directory in a nested module (a go.mod between it and the
// root, in the walk's start directory, or below it) are not resolved, so the
// build is refused, while an absolute path inside the checkout and two
// scratch packages named directly resolve.
// PREVENTS: `go build //...` walking the whole filesystem on a hook call,
// `/proc/self/cwd/internal/../...` slipping past the whole-tree check, and
// scratch or nested-module packages counted as the two that clear a build.
func TestRootBuildStaysInsideTheCheckout(t *testing.T) {
	root := goBuildTree(t)
	writeGoFiles(t, root, map[string]string{
		"tmp/a/a.go":        "package a\n",
		"tmp/b/b.go":        "package b\n",
		"nested/go.mod":     "module example.com/nested\n\ngo 1.24\n",
		"nested/a/a.go":     "package a\n",
		"nested/b/b.go":     "package b\n",
		"inner/mod/go.mod":  "module example.com/inner\n\ngo 1.24\n",
		"inner/mod/a/a.go":  "package a\n",
		"inner/mod/b/b.go":  "package b\n",
		"inner/lib/main.go": "package main\n\nfunc main() {}\n",
	})
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `go build /proc/self/cwd/internal/../...`, true},
		{"build", bashRootBuild, `go build //...`, true},
		{"build", bashRootBuild, `go build ` + filepath.Dir(root), true},
		{"build", bashRootBuild, `go build ./tmp/...`, true},
		{"build", bashRootBuild, `go build ./tmp/a ./tmp/b`, false},
		{"build", bashRootBuild, `go build ./nested/...`, true},
		{"build", bashRootBuild, `go build ./nested/a ./nested/b`, true},
		// The walk meets one main package, inner/lib, and must not count the
		// nested module's two packages as the second.
		{"build", bashRootBuild, `go build ./inner/...`, true},
		{"build", bashRootBuild, `go build ` + root + `/internal/x`, false},
		{"build", bashRootBuild, `go build ` + root + `/internal/...`, false},
	})
}

// TestRootBuildBoundsTheDirectoriesAPatternWalks drives the build guard with
// a `/...` pattern whose two packages lie past goBuildWalkMax directories.
//
// VALIDATES: the walk stops and the build is refused once it has visited
// goBuildWalkMax directories, and the same two packages named directly pass.
// PREVENTS: a hook call whose cost grows with the tree a pattern names.
func TestRootBuildBoundsTheDirectoriesAPatternWalks(t *testing.T) {
	root := goBuildTree(t)
	for index := range goBuildWalkMax {
		if err := os.MkdirAll(filepath.Join(root, "wide", "a"+strconv.Itoa(10000+index)), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	writeGoFiles(t, root, map[string]string{
		"wide/z1/z1.go": "package z1\n",
		"wide/z2/z2.go": "package z2\n",
	})
	result := guardVerdict(bashRootBuild, root, `go build ./wide/...`)
	if result == nil {
		t.Fatalf("a walk past %d directories was cleared", goBuildWalkMax)
	}
	if !strings.Contains(result.message, "directories before two packages") {
		t.Errorf("the refusal does not name the walk bound: %s", result.message)
	}
	if verdict := guardVerdict(bashRootBuild, root, `go build ./wide/z1 ./wide/z2`); verdict != nil {
		t.Errorf("two library packages named directly were refused: %s", verdict.message)
	}
}

// TestGuardsKeepTheTextVerdictOutsideTheModeledSyntax drives every guard with
// the lines of unmodeledSyntaxLines (session_identity_test.go), each made to
// carry the command that guard refuses by its text in place of `le x`, or
// after it.
//
// VALIDATES: a guard lifts its text verdict only on a line inside the shell
// syntax the hook models (modeledSyntax), so each such line keeps the text
// verdict, while reader lines inside the syntax still lift it.
// PREVENTS: `cat x{#}; go build ./cmd/z` cleared because the splitter read
// `#}; go build ./cmd/z` as a comment, and every other construct the
// splitter reads differently from bash.
func TestGuardsKeepTheTextVerdictOutsideTheModeledSyntax(t *testing.T) {
	root := goBuildTree(t)
	guards := []guardRow{
		{"build", bashRootBuild, "go build ./cmd/z", true},
		{"pipe", bashLossyPipe, "go test ./x | tail -5", true},
		{"raw", bashRawHeavy, "go test ./x", true},
		{"tmp", bashSystemTmp, "echo x > /tmp/y", true},
	}
	rows := make([]guardRow, 0, 4*len(unmodeledSyntaxLines)+16)
	for _, guard := range guards {
		for _, line := range unmodeledSyntaxLines {
			command := line + "\n" + guard.command
			if strings.Contains(line, "le x") {
				command = strings.ReplaceAll(line, "le x", guard.command)
			}
			rows = append(rows, guardRow{guard.guard, guard.check, command, true})
		}
	}
	for _, command := range []string{
		`grep -n "go build" "$dir/x.log"`,
		`grep -n 'go build|x' f`,
		`git log --oneline -3 --grep 'go build'`,
		`grep 'go build' f 2>&1`,
		`sed -n '1,5p' 'go build'`,
		`grep -n "go build\|x" f`,
		`grep -c 'go build' <<< x`,
		`cat a 2>&1; go build ./internal/x`,
	} {
		rows = append(rows, guardRow{"build", bashRootBuild, command, false})
	}
	judgeGuardRows(t, root, rows)
}

// VALIDATES: a build whose words the shell expands (a flag, a GOFLAGS value,
// a package) is judged by its text, because the value the go command reads is
// not the text the hook reads.
// PREVENTS: `echo 'x -toolexec=/path/evil'; go build -tags $_ ./internal/x`
// cleared: bash hands the build the last word of the echo, a -toolexec flag
// that runs a program, while the hook read the tag `$_`.
func TestRootBuildJudgesAnExpandedWordByItsText(t *testing.T) {
	root := goBuildTree(t)
	rows := make([]guardRow, 0, 8)
	for _, command := range []string{
		`echo 'x -toolexec=/path/evil'; go build -tags $_ ./internal/x`,
		`go build -tags=$TAGS ./internal/x`,
		`go build -tags "$TAGS" ./internal/x`,
		`echo -toolexec=x; GOFLAGS=$_ go build ./internal/x`,
		`echo ./cmd/z; go build $_`,
		`go build ./internal/x*`,
		`go build ~/x`,
		// An expanded -o value is refused outright: `bin/$X` leaves bin/ when
		// X holds `../`, though the text judgment allows its spelling.
		`go build -o bin/$X ./cmd/z`,
		`go build -o=bin/$X ./cmd/z`,
		`go build --o "bin/$X" ./cmd/z`,
	} {
		rows = append(rows, guardRow{"build", bashRootBuild, command, true})
	}
	// An expansion outside the -o value keeps HEAD's text verdict.
	rows = append(rows, guardRow{"build", bashRootBuild, `go build -tags "$tags" -o bin/ze ./cmd/ze`, false})
	judgeGuardRows(t, root, rows)
}

// VALIDATES: a guard keeps its text verdict on a line whose split could hand
// it other words than bash runs: a redirection before a command's words, an
// empty quoted word, and a double-quoted backslash bash keeps in a build's
// -o path, while a redirection after the words still lifts the verdict.
// PREVENTS: `<<<cat go build -o /tmp/x ./cmd/z` cleared as the reader cat
// with no build; `grep -e ” 'go build ./cmd/z'` cleared as a grep for the
// pattern `go build ./cmd/z`, while bash greps for the empty pattern; and
// `go build -o "\bin/z" ./cmd/z` cleared as a build into bin/, while bash
// writes `\bin/z` in the checkout.
func TestGuardsKeepTheTextVerdictWhereTheSplitMovesAWord(t *testing.T) {
	root := goBuildTree(t)
	rows := []guardRow{
		{"build", bashRootBuild, `<<<cat go build -o /tmp/x ./cmd/z`, true},
		{"build", bashRootBuild, `grep -e '' 'go build ./cmd/z'`, true},
		{"build", bashRootBuild, `go build -o "\bin/z" ./cmd/z`, true},
		{"build", bashRootBuild, `grep 'go build' f 2>/dev/null`, false},
		{"build", bashRootBuild, `grep 'go build' f > /dev/null`, false},
	}
	judgeGuardRows(t, root, rows)
}
