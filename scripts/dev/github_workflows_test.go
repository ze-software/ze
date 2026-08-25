package main

// Guards for the .github/workflows/ CI set (plan/spec-fixit-ci-schedule-evidence.md).
//
// CI is the only thing that runs ze's gates for everyone rather than for whoever
// happened to type a make target, so what CI does is worth pinning. Validation
// runs on GitHub Actions (the repo is pushed to both codeberg.org and
// github.com/ze-software/ze; the heavy suites and the fast merge gate both live
// here). These tests replace the former .woodpecker/ guards when validation moved
// off Codeberg's shared Woodpecker runners.
//
// The shape being pinned:
//   - verify.yml is a push + pull_request gate that runs every stage of
//     `make ze-precommit-verify` across its shards, and nothing heavy or scheduled
//     (the merge gate stays fast). The shards derive their stages from
//     `make ze-precommit-verify-list`, so their union is the whole stage list.
//   - evidence-nightly.yml is scheduled-only, advisory, and runs fuzz AND the
//     kernel integration suite by make-target name -- but never the QEMU target.
//   - perf-nightly.yml is scheduled-only.
//   - every `make <target>` any workflow names actually exists.
//   - no .woodpecker pipeline remains (validation is not on Codeberg).
//
// These tests are string-based rather than YAML-parsed on purpose:
// gopkg.in/yaml.v3 is an INDIRECT dependency (go.mod), and importing it here
// would promote it to direct and churn go.mod/go.sum -- a shared file, and one
// plan/spec-fixit-supply-chain-hardening.md is specifically about. Comments are
// stripped before matching so a commented-out command cannot satisfy a check.

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

const workflowsDir = ".github/workflows"

// stripComments removes `#` line comments so a commented-out command can never
// satisfy a positive check, nor a word in a comment trip a negative one.
func stripComments(src string) string {
	var kept []string
	for ln := range strings.SplitSeq(src, "\n") {
		if i := strings.Index(ln, "#"); i >= 0 {
			ln = ln[:i]
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

func readFileOrFail(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// workflowFile returns the named workflow with `#` comments stripped.
func workflowFile(t *testing.T, name string) string {
	t.Helper()
	return stripComments(readFileOrFail(t, filepath.Join(repoRoot(t), workflowsDir, name)))
}

// topLevelBlockBody returns the indented body of a top-level `key:` line (the
// lines below it, up to the next column-0 line). Fatals -- never returns "" --
// when the key is missing or its body is blank, so a check built on it can never
// pass vacuously.
func topLevelBlockBody(t *testing.T, name, key string) string {
	t.Helper()
	lines := strings.Split(workflowFile(t, name), "\n")
	want := strings.TrimRight(key, " \t")
	for i, ln := range lines {
		if strings.TrimRight(ln, " \t") != want {
			continue
		}
		var out []string
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				out = append(out, next)
				continue
			}
			if !strings.HasPrefix(next, " ") && !strings.HasPrefix(next, "\t") {
				break // back to column 0: the block ended
			}
			out = append(out, next)
		}
		// Blank lines are APPENDED, so a body of only blanks has len(out) != 0;
		// judge on CONTENT to keep the fail-open shape out.
		if strings.TrimSpace(strings.Join(out, "")) == "" {
			t.Fatalf("%s has an empty %q block; this test must not pass vacuously", name, key)
		}
		return strings.Join(out, "\n")
	}
	t.Fatalf("%s has no top-level %q block; this test must not pass vacuously", name, key)
	return ""
}

// onBlock returns just the top-level `on:` block of a workflow.
//
// Trigger checks must NOT scan the whole file: a step command containing
// `git push`, or a path containing "pull_request", would trip a whole-file
// substring test on legitimate content. Scoping to `on:` keeps the check honest.
func onBlock(t *testing.T, name string) string {
	t.Helper()
	return topLevelBlockBody(t, name, "on:")
}

// jobBlock is one entry of a workflow's `jobs:` mapping.
type jobBlock struct {
	name     string
	advisory bool   // carries `continue-on-error: true` as a DIRECT key of this job
	body     string // every line under this job, comments stripped
}

// jobBlocks parses `jobs:` into per-job blocks and reports, for each, whether
// `continue-on-error: true` is one of that job's own keys, plus the job's own
// lines. The body lets a check ask whether ONE job does two things. A
// precondition and the target that needs it must run on the same runner. A
// whole-file scan cannot tell that from two jobs that each do one.
//
// Indentation-agnostic by construction: a job's key level is derived from the
// first key inside that job, not assumed. Anything indented deeper (a steps:
// list item, an env: value) is NOT a direct key and cannot satisfy the check.
func jobBlocks(t *testing.T, name string) []jobBlock {
	t.Helper()
	lines := strings.Split(topLevelBlockBody(t, name, "jobs:"), "\n")

	indentOf := func(ln string) string {
		return ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))]
	}

	var jobIndent string
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		jobIndent = indentOf(ln)
		break
	}
	if jobIndent == "" {
		t.Fatalf("%s: could not determine job indentation; this test must not pass vacuously", name)
	}

	var out []jobBlock
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" || indentOf(ln) != jobIndent {
			continue
		}
		cur := jobBlock{name: strings.TrimSuffix(strings.TrimSpace(ln), ":")}
		var keyIndent string
		var body []string
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				continue
			}
			ind := indentOf(next)
			if len(ind) <= len(jobIndent) {
				break // back at job level: this job's block ended
			}
			body = append(body, next)
			if keyIndent == "" {
				keyIndent = ind // first key inside the job defines its key level
			}
			if ind != keyIndent {
				continue // deeper: a value, not a direct key
			}
			trimmed := strings.TrimSpace(next)
			if trimmed == "continue-on-error: true" || trimmed == `continue-on-error: "true"` {
				cur.advisory = true
			}
		}
		cur.body = strings.Join(body, "\n")
		out = append(out, cur)
	}
	if len(out) == 0 {
		t.Fatalf("parsed no jobs from %s; the file's shape changed. This test must not pass vacuously.", name)
	}
	return out
}

// parseMakeTargets returns every make target a workflow (or Makefile fragment)
// invokes, one per bare word after `make`.
//
// LIMIT: this models shell command lines, not a shell. `$(MAKE)`, `bash -c "..."`,
// backticks and `(subshells)` are NOT parsed. What IS covered: every target on a
// line (not just the first), surrounding quotes, `&&`/`;`/`||`/`|` chains,
// `sudo`/`env`/`then`/`do`/`VAR=` and leading command flags (`-E`, `--foo`)
// before `make`, and `-C`/`-f`/`-j`/`-l`/`-o`/`-W` with separate or attached args.
func parseMakeTargets(src string) []string {
	var out []string
	for ln := range strings.SplitSeq(src, "\n") {
		cmd := strings.TrimSpace(ln)
		cmd = strings.TrimPrefix(cmd, "- ")
		// `- "make ze-integration-test"` is a quoted YAML scalar, not a
		// different command. Unquote so it parses like the bare form.
		cmd = strings.Trim(cmd, `"'`)
		for _, sep := range []string{"&&", ";", "||", "|"} {
			cmd = strings.ReplaceAll(cmd, sep, "\x00")
		}
		for frag := range strings.SplitSeq(cmd, "\x00") {
			fields := strings.Fields(frag)
			if len(fields) > 0 && fields[0] == "-" {
				fields = fields[1:]
			}
			// `run: make ...` -- the YAML command key is not part of the command.
			if len(fields) > 0 && fields[0] == "run:" {
				fields = fields[1:]
			}
			// Strip command prefixes that sit before `make`: wrappers
			// (sudo/env), shell keywords (then/do), VAR=value assignments, and
			// their flags. `sudo -E env "PATH=$PATH" make X` must reach `make X`;
			// an earlier version stopped at `-E` and missed the target entirely.
			for len(fields) > 0 && (fields[0] == "sudo" || fields[0] == "env" ||
				fields[0] == "then" || fields[0] == "do" ||
				strings.HasPrefix(fields[0], "-") || strings.Contains(fields[0], "=")) {
				fields = fields[1:]
			}
			if len(fields) < 2 || fields[0] != "make" {
				continue
			}
			args := fields[1:]
			for i := 0; i < len(args); i++ {
				a := args[i]
				if a == "-C" || a == "-f" || a == "-j" || a == "-l" || a == "-o" || a == "-W" {
					i++ // this flag takes a SEPARATE argument; skip it too
					continue
				}
				if strings.HasPrefix(a, "-") || strings.Contains(a, "=") {
					continue
				}
				// EVERY bare word is a target: `make a b` invokes both.
				out = append(out, a)
			}
		}
	}
	return out
}

// makeTargetsInWorkflow returns every make target the named workflow invokes.
func makeTargetsInWorkflow(t *testing.T, name string) []string {
	t.Helper()
	return parseMakeTargets(workflowFile(t, name))
}

// TestVerifyWorkflowIsTheFastMergeGate
//
// VALIDATES: verify.yml reads the verify stage list on push and pull_request, and adds
// no scheduled or heavy work (the merge gate stays fast).
// PREVENTS: (a) the fast gate silently becoming scheduled-only or dropping a
// trigger; (b) a heavy suite being bolted onto every merge.
//
// The gate runs its stages one shard at a time now, so the target it names is
// `ze-precommit-verify-list` -- the command each shard reads its stages from. That the
// shards then run all of them is TestWorkflowShardsCoverEveryStage.
func TestVerifyWorkflowIsTheFastMergeGate(t *testing.T) {
	src := workflowFile(t, "verify.yml")
	if !slices.Contains(makeTargetsInWorkflow(t, "verify.yml"), "ze-precommit-verify-list") {
		t.Error("verify.yml must read its stages from `make ze-precommit-verify-list`")
	}
	on := onBlock(t, "verify.yml")
	for _, want := range []string{"push", "pull_request"} {
		if !strings.Contains(on, want) {
			t.Errorf("verify.yml must trigger on %q; its `on:` block is:\n%s", want, on)
		}
	}
	for _, forbidden := range []string{"schedule", "cron"} {
		if strings.Contains(on, forbidden) {
			t.Errorf("verify.yml must not be %s-triggered; scheduled work lives in the nightly workflows", forbidden)
		}
	}
	// "no added latency" has to be enforced, not merely assumed: the heavy suites
	// must not appear anywhere in the fast gate.
	for _, heavy := range []string{
		"ze-fuzz-test", "ze-integration-test", "ze-qemu-integration-test",
		"ze-mutation-test", "ze-evidence-release-verify",
	} {
		if strings.Contains(src, heavy) {
			t.Errorf("verify.yml must not run %q: it is a scheduled/heavy suite, and the merge gate stays fast", heavy)
		}
	}
}

func activeStaticcheckInstallCounts(src, module, install string) (int, int) {
	active := stripComments(src)
	return strings.Count(active, install), strings.Count(active, module)
}

func TestPinnedStaticcheckInstallIgnoresCommentedCommands(t *testing.T) {
	const module = "honnef.co/go/tools/cmd/staticcheck"
	const install = "CGO_ENABLED=0 go install " + module + "@2026.1"
	installs, moduleRefs := activeStaticcheckInstallCounts("# "+install, module, install)
	if installs != 0 || moduleRefs != 0 {
		t.Fatalf("commented install counted as active: installs=%d module references=%d", installs, moduleRefs)
	}
}

// TestVerifyInstallsPinnedStaticcheck
//
// VALIDATES: CI, the agent workstation, and isolated verification evidence
// install the same cgo-free Staticcheck release that supports the repository Go version.
// PREVENTS: duplicate, unpinned, or native-CGO installs making the matrix verdict
// depend on which verification bootstrap prepared the host.
func TestVerifyInstallsPinnedStaticcheck(t *testing.T) {
	const module = "honnef.co/go/tools/cmd/staticcheck"
	const nonSudoInstall = "CGO_ENABLED=0 go install " + module + "@2026.1"
	for _, tc := range []struct {
		path    string
		install string
	}{
		{path: ".github/workflows/verify.yml", install: nonSudoInstall},
		{
			path:    "scripts/dev/setup_claude_server.sh",
			install: `sudo -u "$TARGET_USER" env CGO_ENABLED=0 /usr/local/go/bin/go install ` + module + "@2026.1",
		},
		{path: "scripts/evidence/effective-verify.sh", install: nonSudoInstall},
	} {
		src := readFileOrFail(t, filepath.Join(repoRoot(t), tc.path))
		installs, moduleRefs := activeStaticcheckInstallCounts(src, module, tc.install)
		if installs != 1 {
			t.Errorf("%s must contain exactly one active %q", tc.path, tc.install)
		}
		if moduleRefs != 1 {
			t.Errorf("%s must contain exactly one active pinned Staticcheck install", tc.path)
		}
	}
}

// TestVerifyWorkflowProvisionsTheLoopbackAddress
//
// VALIDATES: verify.yml adds fd00::2 to lo before any shard runs its stages.
// PREVENTS: the merge gate reddening on every functional fixture that needs a
// second IPv6 address. The runner cannot add one (CAP_NET_ADMIN), so the
// workflow is the only place it can happen on this host, and a step that quietly
// goes away takes the whole plugin suite with it.
//
// Every shard runs the step, because any shard can hold ze-functional-test: which one
// does is derived from the live stage list, not written down here.
func TestVerifyWorkflowProvisionsTheLoopbackAddress(t *testing.T) {
	src := workflowFile(t, "verify.yml")
	const add = "ip -6 addr add fd00::2/128 dev lo"
	addAt := strings.Index(src, add)
	if addAt < 0 {
		t.Fatalf("verify.yml must add the loopback address the fixtures bind: %q", add)
	}
	verifyAt := strings.Index(src, "make ze-precommit-verify-list")
	if verifyAt < 0 {
		t.Fatal("verify.yml must read its stages from `make ze-precommit-verify-list`")
	}
	if addAt > verifyAt {
		t.Error("verify.yml must add fd00::2 BEFORE a shard runs its stages; after it the gate has already run")
	}
}

// ─── verify.yml shards (plan/spec-verify-scope-4-suite-budget-and-ci.md) ────

// yamlBlockKeys returns the direct keys of the block under a `key:` line, judging
// membership by indentation the way jobBlocks does. Fatals when the key is absent, so
// a check built on it can never pass vacuously.
func yamlBlockKeys(t *testing.T, src, key string) []string {
	t.Helper()
	lines := strings.Split(src, "\n")
	indentOf := func(ln string) int { return len(ln) - len(strings.TrimLeft(ln, " ")) }
	for i, ln := range lines {
		if strings.TrimSpace(ln) != key+":" {
			continue
		}
		outer := indentOf(ln)
		keyIndent := -1
		var keys []string
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				continue
			}
			ind := indentOf(next)
			if ind <= outer {
				break // back at the outer level: the block ended
			}
			if keyIndent < 0 {
				keyIndent = ind // the first key inside defines the block's key level
			}
			if ind != keyIndent {
				continue // deeper: a value, not a direct key
			}
			name, _, ok := strings.Cut(strings.TrimSpace(next), ":")
			if ok {
				keys = append(keys, name)
			}
		}
		return keys
	}
	t.Fatalf("verify.yml has no %q block; this test must not pass vacuously", key)
	return nil
}

var shardMatrixRE = regexp.MustCompile(`(?m)^\s*shard:\s*\[([0-9,\s]+)]\s*$`)

// shardIndices returns the shard numbers verify.yml fans out over, and fatals unless
// they are exactly 1..N.
//
// The count is stated ONCE, in this list: the workflow passes `strategy.job-total`
// (the size of this matrix) to the selection as SHARD_TOTAL. Contiguous 1..N is what
// makes the round-robin below cover every residue class, so it is checked here rather
// than assumed by the simulation.
func shardIndices(t *testing.T, src string) []int {
	t.Helper()
	if keys := yamlBlockKeys(t, src, "matrix"); !slices.Equal(keys, []string{"shard"}) {
		t.Fatalf("verify.yml's matrix must have `shard` as its only dimension, got %v: "+
			"a second dimension multiplies strategy.job-total and breaks the shard arithmetic", keys)
	}
	m := shardMatrixRE.FindStringSubmatch(src)
	if m == nil {
		t.Fatal("verify.yml must declare `shard: [1, 2, ...]`; this test must not pass vacuously")
	}
	var idx []int
	for f := range strings.SplitSeq(m[1], ",") {
		var n int
		if _, err := fmt.Sscan(strings.TrimSpace(f), &n); err != nil {
			t.Fatalf("shard matrix entry %q is not a number: %v", f, err)
		}
		idx = append(idx, n)
	}
	for i, n := range idx {
		if n != i+1 {
			t.Fatalf("verify.yml's shard matrix must be 1..N in order, got %v", idx)
		}
	}
	if len(idx) < 2 {
		t.Fatalf("verify.yml must fan out over more than one shard, got %v", idx)
	}
	if !strings.Contains(src, "SHARD_TOTAL: ${{ strategy.job-total }}") {
		t.Error("verify.yml must pass SHARD_TOTAL: ${{ strategy.job-total }}: the shard count " +
			"belongs in the matrix only, and a second spelling of it can disagree with it")
	}
	return idx
}

var shardSelectRE = regexp.MustCompile(`(?m)^\s*(awk -v i=.*)$`)

// shardSelectCommand returns the one command line verify.yml selects a shard's stages
// with. The test RUNS this line rather than reimplementing it: a copy would agree with
// itself while the workflow drifted.
func shardSelectCommand(t *testing.T, src string) string {
	t.Helper()
	m := shardSelectRE.FindAllStringSubmatch(src, -1)
	if len(m) != 1 {
		t.Fatalf("verify.yml must select its shard's stages with exactly one `awk -v i=...` line, found %d", len(m))
	}
	return strings.TrimSpace(m[0][1])
}

// verifyStageList returns the live verify stage list, one stage per line, as the
// workflow reads it. stagesForMode (scripts/status/verify_run.go) is its only source.
//
// ZE_VERIFY_MODE is passed as a make ARGUMENT, exactly as verify.yml passes it,
// and the reason is that the name carries two unrelated meanings. The suites read
// "1" from the environment, and execStage (scripts/status/verify_run.go) exports
// that value to every stage it runs. This target reads a MODE NAME from the same
// name ($(or $(ZE_VERIFY_MODE),ze-precommit-verify) in the Makefile), so a run
// under verify would hand it "1" and the runner would refuse with `unknown mode
// "1"`. A command-line assignment beats the environment, so this test reads the
// stage list whether or not a verify run is what invoked it.
func verifyStageList(t *testing.T) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "make", "--no-print-directory", "ze-precommit-verify-list", "ZE_VERIFY_MODE=ze-precommit-verify")
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("make ze-precommit-verify-list: %v", err)
	}
	var stages []string
	for ln := range strings.SplitSeq(string(out), "\n") {
		if s := strings.TrimSpace(ln); s != "" {
			stages = append(stages, s)
		}
	}
	if len(stages) == 0 {
		t.Fatal("make ze-precommit-verify-list printed no stage; this test must not pass vacuously")
	}
	return stages
}

// TestVerifyStageListIgnoresTheInheritedVerifyMode
//
// VALIDATES: verifyStageList reads the stage list with ZE_VERIFY_MODE=1 already
// in the environment, which is what every stage of a verify run inherits.
// PREVENTS: this file's tests passing on a developer's shell and failing inside
// `make ze-precommit-verify`. execStage (scripts/status/verify_run.go) exports
// ZE_VERIFY_MODE=1 to every stage, ZE_PACKAGES puts ./scripts/dev inside
// ze-unit-test-cached, and ze-precommit-verify-list reads that same name as a
// MODE NAME, so an inherited "1" makes the runner exit 2 with `unknown mode "1"`.
// The red then lands only in the gate, which is where it costs the most to read.
func TestVerifyStageListIgnoresTheInheritedVerifyMode(t *testing.T) {
	t.Setenv("ZE_VERIFY_MODE", "1")
	verifyStageList(t)
}

// TestWorkflowShardsCoverEveryStage
//
// VALIDATES: the union of verify.yml's shards is EXACTLY the stage list
// `make ze-precommit-verify-list` prints, each stage on exactly one shard, and no stage
// named in the workflow itself.
// PREVENTS: the failure sharding introduces and nothing else catches -- a gate that
// stops running while CI stays green. A stage added to stagesForMode must land in a
// shard with no second file to edit, so the workflow may hold a count and an
// arithmetic rule, never a stage name.
func TestWorkflowShardsCoverEveryStage(t *testing.T) {
	src := workflowFile(t, "verify.yml")
	shards := shardIndices(t, src)
	selectCmd := shardSelectCommand(t, src)
	stages := verifyStageList(t)

	// A stage NAME in the workflow is the second list this design refuses. Comments
	// are already stripped, so the header may still explain which stages are heavy.
	for _, st := range stages {
		if strings.Contains(src, st) {
			t.Errorf("verify.yml names the stage %q: the shards must derive every stage from "+
				"`make ze-precommit-verify-list`, so no stage name belongs in the workflow", st)
		}
	}
	if !strings.Contains(src, "fail-fast: false") {
		t.Error("verify.yml must set `fail-fast: false`: a canceled sibling shard leaves its " +
			"stages unrun and unreported, which is the failure this test exists to refuse")
	}

	dir := t.TempDir()
	stageList := filepath.Join(dir, "stages.txt")
	if err := os.WriteFile(stageList, []byte(strings.Join(stages, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write stage list: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	seen := map[string]int{}
	for _, idx := range shards {
		shardList := filepath.Join(dir, fmt.Sprintf("shard-%d.txt", idx))
		cmd := exec.CommandContext(ctx, "sh", "-c", selectCmd)
		cmd.Env = append(os.Environ(),
			"STAGE_LIST="+stageList,
			"SHARD_LIST="+shardList,
			fmt.Sprintf("SHARD_INDEX=%d", idx),
			fmt.Sprintf("SHARD_TOTAL=%d", len(shards)),
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("shard %d selection (%s) failed: %v\n%s", idx, selectCmd, err, out)
		}
		body, err := os.ReadFile(shardList) //nolint:gosec // path is this test's temp dir
		if err != nil {
			t.Fatalf("read shard %d selection: %v", idx, err)
		}
		got := 0
		for ln := range strings.SplitSeq(string(body), "\n") {
			if s := strings.TrimSpace(ln); s != "" {
				seen[s]++
				got++
			}
		}
		if got == 0 {
			t.Errorf("shard %d selects no stage out of %d; the workflow's own `test -s` would "+
				"fail the step, and the shard's share of the gate would not run", idx, len(stages))
		}
	}

	for _, st := range stages {
		switch seen[st] {
		case 1:
		case 0:
			t.Errorf("no shard runs %q: it is in the stage list and would run NOWHERE in CI", st)
		default:
			t.Errorf("%d shards run %q; each stage must run exactly once", seen[st], st)
		}
		delete(seen, st)
	}
	for st := range seen {
		t.Errorf("a shard selects %q, which is not in the stage list", st)
	}
}

// TestVerifyShardsRunStagesTheWayTheVerifyRunnerDoes
//
// VALIDATES: a shard runs one `make` per stage with ZE_VERIFY_MODE=1, which is what
// execStage (scripts/status/verify_run.go) does for every stage of a local
// `make ze-precommit-verify`.
// PREVENTS: CI running a WEAKER gate than the developer's. The functional runner reads
// that variable (VerifyModeEnabled, internal/test/runner/parallel.go) to turn a silent
// environment skip into a hard failure; without it a shard passes on a suite that
// skipped itself, and nothing in the log says so.
func TestVerifyShardsRunStagesTheWayTheVerifyRunnerDoes(t *testing.T) {
	src := workflowFile(t, "verify.yml")
	if !strings.Contains(src, `ZE_VERIFY_MODE: "1"`) {
		t.Error(`verify.yml must run its stages with ZE_VERIFY_MODE: "1", as execStage does`)
	}
	// One make per stage, and every stage attempted: `make a b c` would stop at the
	// first red and leave the rest of the shard's gates unrun.
	if !strings.Contains(src, `xargs -a "$SHARD_LIST" -n1 -t make --no-print-directory`) {
		t.Error("verify.yml must run one `make --no-print-directory <stage>` per selected stage, " +
			"so a red stage does not stop the ones after it")
	}
}

// TestEvidenceNightlyIsScheduled
//
// VALIDATES: the heavy-evidence pipeline is triggered by a schedule (cron) and
// NOT by push/pull_request.
// PREVENTS: it degrading into a per-merge pipeline, which would put a multi-minute
// fuzz + privileged integration run in front of every merge.
func TestEvidenceNightlyIsScheduled(t *testing.T) {
	on := onBlock(t, "evidence-nightly.yml")
	for _, want := range []string{"schedule", "cron"} {
		if !strings.Contains(on, want) {
			t.Errorf("evidence-nightly.yml must be scheduled (%q); its `on:` block is:\n%s", want, on)
		}
	}
	for _, forbidden := range []string{"push", "pull_request"} {
		if strings.Contains(on, forbidden) {
			t.Errorf("evidence-nightly.yml must NOT trigger on %q: the sweep is scheduled-only", forbidden)
		}
	}
}

// TestEvidenceNightlyRunsFuzzAndIntegration
//
// VALIDATES: the nightly invokes ze-fuzz-test AND ze-integration-test by make
// target name (AC-2, now met on GitHub, whose ubuntu-latest runner grants the
// CAP_NET_ADMIN / CAP_NET_BIND_SERVICE the suite needs via root -- impossible on
// Codeberg's shared Woodpecker instance, where it needed the banned `privileged`).
// PREVENTS: (a) the integration suite silently disappearing again; (b)
// ze-qemu-integration-test creeping in -- it additionally needs nested virt / KVM,
// which GitHub-hosted runners do not reliably provide, and a pipeline that cannot
// start catches nothing.
func TestEvidenceNightlyRunsFuzzAndIntegration(t *testing.T) {
	targets := makeTargetsInWorkflow(t, "evidence-nightly.yml")
	for _, want := range []string{"ze-fuzz-test", "ze-integration-test"} {
		if !slices.Contains(targets, want) {
			t.Errorf("evidence-nightly.yml must run `make %s`; targets found: %v", want, targets)
		}
	}
	if slices.Contains(targets, "ze-qemu-integration-test") {
		t.Error("evidence-nightly.yml must NOT run ze-qemu-integration-test: it needs nested virt / KVM, " +
			"which GitHub-hosted runners do not reliably provide")
	}
}

// TestEvidenceNightlyRunsInterop
//
// VALIDATES: the nightly invokes `make ze-interop-test` by name, from its OWN
// job, and that job is advisory (plan/spec-rfcgate-2-evidence.md AC-2, AC-3).
// PREVENTS: the interop suite going back to having no automated caller at all.
// Until this landed its only caller was ze-evidence-release-verify, a manual
// release-time target -- so the 104 BGP scenarios ran when somebody remembered.
// That is the condition that makes an interop `RFC requirement:` tag inadmissible
// (rfc_requirements.py CARRIERS, tier `unrun`): a tag is only evidence if
// something executes the test. Its own job, not a step bolted onto `integration`,
// because Docker-lab failures must not be attributed to the kernel suite, and
// because jobBlocks reads `continue-on-error` as a DIRECT job key.
func TestEvidenceNightlyRunsInterop(t *testing.T) {
	targets := makeTargetsInWorkflow(t, "evidence-nightly.yml")
	if !slices.Contains(targets, "ze-interop-test") {
		t.Errorf("evidence-nightly.yml must run `make ze-interop-test`; targets found: %v", targets)
	}
	jobs := jobBlocks(t, "evidence-nightly.yml")
	var found *jobBlock
	for i, j := range jobs {
		if j.name == "interop" {
			found = &jobs[i]
			break
		}
	}
	if found == nil {
		names := make([]string, 0, len(jobs))
		for _, j := range jobs {
			names = append(names, j.name)
		}
		t.Fatalf("evidence-nightly.yml has no `interop` job; jobs found: %v", names)
	}
	if !found.advisory {
		t.Error("the interop job must carry job-level `continue-on-error: true`: it ships advisory-first, " +
			"like every other job in this workflow")
	}
}

// TestEvidenceNightlyIsAdvisory
//
// VALIDATES: every job is non-blocking (`continue-on-error: true`).
// PREVENTS: a known-red heavy suite marking the whole run failed while the matrix
// is stabilized; it ships advisory-first and flips to blocking after a green
// baseline.
func TestEvidenceNightlyIsAdvisory(t *testing.T) {
	for _, j := range jobBlocks(t, "evidence-nightly.yml") {
		if !j.advisory {
			t.Errorf("evidence-nightly.yml job %q has no job-level `continue-on-error: true`; every job must be advisory", j.name)
		}
	}
}

// TestQemuNightlyIsScheduledAdvisoryAndRunsTheLinuxOnlySuite
//
// VALIDATES: qemu-nightly.yml is scheduled-only, every job is advisory, and it
// invokes `make ze-qemu-needs-linux-test` by name.
// PREVENTS: (a) the Linux-only functional surface silently having no automated
// home again -- `option=needs-linux:caps=net-admin` makes those tests SKIP on the
// unprivileged verify runner, so this workflow is the only thing that executes
// them, and a lost target here converts a redirection into a deletion
// (ai/rules/completion.md); (b) the VM run creeping onto push/pull_request, where
// a 3600s boot-plus-suite budget would sit in front of every merge.
func TestQemuNightlyIsScheduledAdvisoryAndRunsTheLinuxOnlySuite(t *testing.T) {
	on := onBlock(t, "qemu-nightly.yml")
	for _, want := range []string{"schedule", "cron"} {
		if !strings.Contains(on, want) {
			t.Errorf("qemu-nightly.yml must be scheduled (%q); its `on:` block is:\n%s", want, on)
		}
	}
	for _, forbidden := range []string{"push", "pull_request"} {
		if strings.Contains(on, forbidden) {
			t.Errorf("qemu-nightly.yml must NOT trigger on %q: a VM boot plus the full suite is not a merge gate", forbidden)
		}
	}
	if targets := makeTargetsInWorkflow(t, "qemu-nightly.yml"); !slices.Contains(targets, "ze-qemu-needs-linux-test") {
		t.Errorf("qemu-nightly.yml must run `make ze-qemu-needs-linux-test`; targets found: %v", targets)
	}
	for _, j := range jobBlocks(t, "qemu-nightly.yml") {
		if !j.advisory {
			t.Errorf("qemu-nightly.yml job %q has no job-level `continue-on-error: true`; "+
				"KVM availability on hosted runners is not guaranteed, so this reports rather than wedges", j.name)
		}
	}
}

// makefileRecipes maps every target in a make fragment to its recipe: the
// tab-indented lines below its `name:` line. That is where make itself ends a
// recipe. Blank and comment lines below a recipe are NOT part of it, so a
// comment quoting a command cannot satisfy a check built on this.
//
// One line can declare SEVERAL targets (`a b: deps`). Each of them owns the
// recipe below it. Reading only the first word drops the rest, so a check built
// on this would miss a guard a consolidated target line carries. A derived set
// that silently shrinks is the fail-open shape. The vacuity Fatal in the caller
// fires only when it shrinks all the way to zero.
type makefileTarget struct {
	recipe  string
	prereqs []string
}

func makefileRecipes(src string) map[string]makefileTarget {
	out := map[string]makefileTarget{}
	var cur []string
	var prereqs []string
	var body []string
	flush := func() {
		for _, name := range cur {
			out[name] = makefileTarget{recipe: strings.Join(body, "\n"), prereqs: prereqs}
		}
		cur, prereqs, body = nil, nil, nil
	}
	for ln := range strings.SplitSeq(src, "\n") {
		if strings.HasPrefix(ln, "\t") {
			if len(cur) > 0 {
				body = append(body, ln)
			}
			continue
		}
		flush()
		head, after, ok := strings.Cut(ln, ":")
		if !ok || strings.ContainsAny(head, "#=") || strings.HasPrefix(after, "=") {
			continue // a comment, or an assignment (`FOO = x`, `FOO := x`), never a rule
		}
		for name := range strings.FieldsSeq(head) {
			if strings.HasPrefix(name, ".") {
				continue // .PHONY and friends declare nothing this reads
			}
			cur = append(cur, name)
		}
		prereqs = strings.Fields(after)
	}
	flush()
	return out
}

// TestQemuKernelPreconditionIsMetInTheSameJob
//
// VALIDATES: any workflow JOB that runs a QEMU target guarded by
// ze-qemu-kernel-guard also runs, in that same job, a target that stages
// tmp/kernel/vmlinuz.
// PREVENTS: the failure that ran for four nights unseen. b38706464 put both QEMU
// functional targets on ze's own runtime kernel and made the staged vmlinuz a
// hard precondition. qemu-nightly.yml was not updated. Every scheduled run from
// 2026-08-08 died in about a minute on "tmp/kernel/vmlinuz not found", and
// job-level continue-on-error reported each of those runs as `success`.
//
// Two guards were blind to it.
// TestQemuNightlyIsScheduledAdvisoryAndRunsTheLinuxOnlySuite pins that the
// workflow CALLS the target. scripts/evidence/qemu_kernel_wiring_test.go pins
// that the target CARRIES the guard. Neither asks whether the caller can
// satisfy what the guard demands.
//
// Both sides are DERIVED from the make fragments. A target that adopts the
// guard later, and a renamed staging target, are covered with no edit here.
// Same job, not same file: a precondition met in a different job runs on a
// different runner with a different filesystem, which is no precondition.
func TestQemuKernelPreconditionIsMetInTheSameJob(t *testing.T) {
	root := repoRoot(t)
	integration := readFileOrFail(t, filepath.Join(root, "mk", "test-integration.mk"))
	// build-gokrazy.mk, not gokrazy.mk. 72d2f0d59 renamed every makefile
	// fragment by what it does and left this path behind, so the read failed and
	// this check stopped checking. It is the only one that asks whether a
	// workflow job can satisfy the kernel precondition its targets demand.
	gokrazy := readFileOrFail(t, filepath.Join(root, "mk", "build-gokrazy.mk"))

	intRecipes := makefileRecipes(integration)
	var guarded []string
	for name, target := range intRecipes {
		if !strings.Contains(target.recipe, "$(ze-qemu-kernel-guard)") {
			continue
		}
		guarded = append(guarded, name)
	}
	// The job-admission wrapper splits a heavy target in two: the public name
	// calls scripts/dev/ze-run.sh, and an `-impl` target carries the recipe body.
	// The guard went with the body. A workflow calls the PUBLIC name and the guard
	// still runs, one level down, so the public name is guarded too. Without this
	// the scan collects impl names alone, no workflow ever names one, and the
	// vacuity Fatal below fires on a tree where every guard is in place -- which
	// is what happened when the wrapper landed on 2026-08-18.
	//
	// Credited by reading what the recipe INVOKES, not by unwrapping the `_<name>-impl`
	// spelling. The convention is a convention: a wrapper written with a differently
	// named body would silently stop crediting its public target and put this back
	// where it was. makeInvocations answers what make will actually run.
	for name, target := range intRecipes {
		for _, called := range makeInvocations(target.recipe) {
			if slices.Contains(guarded, called) && !slices.Contains(guarded, name) {
				guarded = append(guarded, name)
			}
		}
	}
	if len(guarded) == 0 {
		t.Fatal("no target in mk/test-integration.mk runs $(ze-qemu-kernel-guard); the guard or the layout moved, and this test must not pass vacuously")
	}

	// A stager either copies the kernel into place itself, or depends on one that
	// does. The second case is what keeps `make ze-kernel-build` a valid answer here.
	//
	// The copy and the path must be on the SAME line. Over a joined recipe the
	// two substrings meet without the staging command existing. ze-kernel-vmlinuz-stage's
	// cache branches both run `cp -R`, and its echo names the path. So deleting
	// the one line that stages the kernel left the target still reading as a
	// stager. A guard that survives the deletion of what it checks for is not one.
	recipes := makefileRecipes(gokrazy)
	stagers := map[string]bool{}
	for name, target := range recipes {
		for ln := range strings.SplitSeq(target.recipe, "\n") {
			if strings.Contains(ln, "tmp/kernel/vmlinuz") && strings.Contains(ln, "cp ") {
				stagers[name] = true
			}
		}
	}
	if len(stagers) == 0 {
		t.Fatal("no target in mk/build-gokrazy.mk stages tmp/kernel/vmlinuz; this test must not pass vacuously")
	}
	// To a fixpoint, because map iteration is unordered. One pass resolves a
	// direct prerequisite. It resolves a two-link chain only when the map
	// happens to yield it in order.
	for grew := true; grew; {
		grew = false
		for name := range recipes {
			if stagers[name] {
				continue
			}
			for _, prereq := range recipes[name].prereqs {
				if stagers[prereq] {
					stagers[name] = true
					grew = true
				}
			}
		}
	}

	workflows, err := filepath.Glob(filepath.Join(root, workflowsDir, "*.yml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("glob %s/*.yml: %v (%d found)", workflowsDir, err, len(workflows))
	}
	checked := 0
	for _, wf := range workflows {
		for _, j := range jobBlocks(t, filepath.Base(wf)) {
			targets := parseMakeTargets(j.body)
			var needsKernel []string
			for _, target := range targets {
				if slices.Contains(guarded, target) {
					needsKernel = append(needsKernel, target)
				}
			}
			if len(needsKernel) == 0 {
				continue
			}
			checked++
			if !slices.ContainsFunc(targets, func(target string) bool { return stagers[target] }) {
				t.Errorf("%s job %q runs %v, which requires a staged tmp/kernel/vmlinuz (ze-qemu-kernel-guard), "+
					"but the job runs no target that stages one (any of %v). The guard denies before the VM starts, "+
					"and on an advisory job that reads as a green run.",
					filepath.Base(wf), j.name, needsKernel, slices.Sorted(maps.Keys(stagers)))
			}
		}
	}
	if checked == 0 {
		t.Fatalf("no workflow job runs a kernel-guarded QEMU target (%v); the Linux-only surface lost its home, or the scan broke", guarded)
	}
}

// TestCapabilityGatedTestsHaveAQemuHome
//
// VALIDATES: whenever a `.ci` test declares `option=needs-linux:caps=...`, the
// workflow set runs `ze-qemu-needs-linux-test` SPECIFICALLY -- the one target
// whose ZE_QEMU_LINUX_ONLY pass selects those tests -- and that no gated test
// also carries a `skip-env` that would exclude it from that very run.
// PREVENTS: the silent-coverage-deletion failure mode. A caps= option makes a
// test skip on every host lacking the capability, INCLUDING the verify runner,
// so marking tests with it and having no privileged runner would turn a red
// suite into a green one by removing the coverage (ai/rules/completion.md).
//
// The earlier version of this test accepted ANY `ze-qemu-*` target, which was
// fail-open: `ze-qemu-debug`, `ze-qemu-l2tp-ppp-test` and friends satisfy that
// prefix and run no `.ci` from the plugin or reload suites. It also could not
// see the second half of the problem, which is what actually bit:
// test/plugin/policy-routes-show.ci carried `skip-env:var=ZE_QEMU` from an era
// when QEMU could not do nftables, and qemu-all-tests.sh exports ZE_QEMU=1 --
// so the caps gate skipped it everywhere else and the env gate skipped it in the
// one place with the capability. It ran nowhere, and every gate stayed green.
func TestCapabilityGatedTestsHaveAQemuHome(t *testing.T) {
	root := repoRoot(t)
	var gated []string
	err := filepath.Walk(filepath.Join(root, "test"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".ci" {
			return err //nolint:wrapcheck // walk callback: propagate as-is
		}
		b, rerr := os.ReadFile(path) //nolint:gosec // path from a repo-local walk
		if rerr != nil {
			return rerr //nolint:wrapcheck // walk callback: propagate as-is
		}
		for ln := range strings.SplitSeq(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(ln), "option=needs-linux:caps=") {
				rel, _ := filepath.Rel(root, path)
				gated = append(gated, rel)
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk test/: %v", err)
	}
	if len(gated) == 0 {
		t.Fatal("no .ci test declares option=needs-linux:caps=; the marker or the tree moved. This test must not pass vacuously.")
	}

	// A gated test that ALSO opts out of the QEMU run is excluded from the only
	// place its capability exists, so the marker deletes it rather than moving it.
	for _, rel := range gated {
		body := readFileOrFail(t, filepath.Join(root, rel))
		for ln := range strings.SplitSeq(body, "\n") {
			if strings.HasPrefix(strings.TrimSpace(ln), "option=skip-env:var=ZE_QEMU") {
				t.Errorf("%s declares a caps= gate AND %q: the caps gate skips it wherever the capability is absent, "+
					"and qemu-all-tests.sh exports ZE_QEMU=1, so it runs in NO environment at all", rel, strings.TrimSpace(ln))
			}
		}
	}

	workflows, err := filepath.Glob(filepath.Join(root, workflowsDir, "*.yml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("glob %s/*.yml: %v (%d found)", workflowsDir, err, len(workflows))
	}
	// By NAME, not by `ze-qemu-` prefix. Only ze-qemu-needs-linux-test passes
	// ZE_QEMU_LINUX_ONLY=1, which is what makes the VM run select these tests.
	const home = "ze-qemu-needs-linux-test"
	for _, wf := range workflows {
		if slices.Contains(parseMakeTargets(stripComments(readFileOrFail(t, wf))), home) {
			return
		}
	}
	t.Errorf("%d .ci test(s) declare option=needs-linux:caps=... (e.g. %s) but no workflow runs `make %s`: "+
		"those tests skip everywhere, which deletes the coverage instead of relocating it", len(gated), gated[0], home)
}

// TestPerfNightlyIsScheduled
//
// VALIDATES: the perf-regression check is scheduled-only, never a merge gate.
// PREVENTS: machine-dependent timing metrics gating push/pull_request merges (the
// deterministic allocs/op gate is ze-alloc-check, inside ze-precommit-verify).
func TestPerfNightlyIsScheduled(t *testing.T) {
	on := onBlock(t, "perf-nightly.yml")
	for _, want := range []string{"schedule", "cron"} {
		if !strings.Contains(on, want) {
			t.Errorf("perf-nightly.yml must be scheduled (%q); its `on:` block is:\n%s", want, on)
		}
	}
	for _, forbidden := range []string{"push", "pull_request"} {
		if strings.Contains(on, forbidden) {
			t.Errorf("perf-nightly.yml must NOT trigger on %q: timing metrics are scheduled-only", forbidden)
		}
	}
}

// TestWorkflowMakeTargetsExist
//
// VALIDATES: every `make <target>` any workflow invokes is a real target in the
// Makefile or an mk/*.mk fragment.
// PREVENTS: a typo'd or renamed target in a file nobody runs locally -- a
// scheduled pipeline is exactly where that rots unnoticed: it fires at night, and
// `make: *** No rule to make target` looks like an infrastructure blip.
func TestWorkflowMakeTargetsExist(t *testing.T) {
	root := repoRoot(t)
	frags, err := filepath.Glob(filepath.Join(root, "mk", "*.mk"))
	if err != nil || len(frags) == 0 {
		t.Fatalf("glob mk/*.mk: %v (%d found); layout changed. This test must not pass vacuously.", err, len(frags))
	}
	var sb strings.Builder
	sb.WriteString(readFileOrFail(t, filepath.Join(root, "Makefile")))
	for _, f := range frags {
		sb.WriteString("\n")
		sb.WriteString(readFileOrFail(t, f))
	}
	corpus := sb.String()

	var workflows []string
	for _, pat := range []string{"*.yml", "*.yaml"} {
		m, err := filepath.Glob(filepath.Join(root, workflowsDir, pat))
		if err != nil {
			t.Fatalf("glob %s: %v", pat, err)
		}
		workflows = append(workflows, m...)
	}
	if len(workflows) == 0 {
		t.Fatalf("found no workflow files under %s. This test must not pass vacuously.", workflowsDir)
	}

	checked := 0
	for _, wf := range workflows {
		for _, target := range parseMakeTargets(stripComments(readFileOrFail(t, wf))) {
			checked++
			if !strings.Contains(corpus, "\n"+target+":") {
				t.Errorf("%s invokes `make %s`, but no such target exists in Makefile or mk/*.mk",
					filepath.Base(wf), target)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("found no `make <target>` invocations across %s/*.yml; the scan is broken. This test must not pass vacuously.", workflowsDir)
	}
}

// TestCrossBranchDemoTargetsExist
//
// VALIDATES: the make targets the website publish invokes BY NAME, from
// outside this branch's own workflow glob, still exist in this branch's
// Makefile set.
// PREVENTS: a rename on main breaking the website publish with nothing on main
// noticing. TestWorkflowMakeTargetsExist only globs THIS branch's
// .github/workflows, and main's pages.yml was deleted so that gh-pages owns
// the deploy. `ze-terminal-demo-release-render-all` is named as the operator's
// command by docs/contributing/gh-pages.md and by website/AI.md, and it is the
// aggregate mk/build-terminal-demo.mk says release preparation calls before
// tagging. None of those is a file a glob on main can see, so the name is
// pinned here explicitly.
//
// The list held a second target until 2026-08-24. `ze-terminal-demo-tools-install`
// installed native VHS, ttyd and ffmpeg through demos/terminal/install-vhs.sh;
// the renderer records with its own PTY now, so the script and the target were
// deleted with VHS. The gh-pages branch carries no .github directory at all
// since `b0430c2a9 Remove website sources from pages branch`, so no workflow
// there invokes either name today.
func TestCrossBranchDemoTargetsExist(t *testing.T) {
	root := repoRoot(t)
	frags, err := filepath.Glob(filepath.Join(root, "mk", "*.mk"))
	if err != nil || len(frags) == 0 {
		t.Fatalf("glob mk/*.mk: %v (%d found)", err, len(frags))
	}
	var sb strings.Builder
	sb.WriteString(readFileOrFail(t, filepath.Join(root, "Makefile")))
	for _, f := range frags {
		sb.WriteString("\n")
		sb.WriteString(readFileOrFail(t, f))
	}
	corpus := sb.String()

	// Named by docs/contributing/gh-pages.md and website/AI.md as the command
	// that republishes the demo media, and by mk/build-terminal-demo.mk as the
	// aggregate release preparation calls.
	for _, target := range []string{"ze-terminal-demo-release-render-all"} {
		if !strings.Contains(corpus, "\n"+target+":") {
			t.Errorf("the website publish runs `make %s`, but no such target exists in Makefile or mk/*.mk: "+
				"the publish would fail with `No rule to make target` and nothing on main would report it", target)
		}
	}
}

// TestValidationIsNotOnWoodpecker
//
// VALIDATES: no .woodpecker pipeline remains -- validation moved to GitHub Actions
// and does not run on Codeberg's donated shared runners.
// PREVENTS: a .woodpecker pipeline being reintroduced (and silently double-running
// the heavy suites on a free shared instance) after the migration.
func TestValidationIsNotOnWoodpecker(t *testing.T) {
	root := repoRoot(t)
	for _, pat := range []string{"*.yml", "*.yaml"} {
		m, err := filepath.Glob(filepath.Join(root, ".woodpecker", pat))
		if err != nil {
			t.Fatalf("glob .woodpecker/%s: %v", pat, err)
		}
		if len(m) > 0 {
			t.Errorf(".woodpecker pipelines still present (%v); validation moved to %s and must not run on Codeberg", m, workflowsDir)
		}
	}
}

// TestEvidenceNightlyRunsIpsecInterop
//
// VALIDATES: the nightly invokes `make ze-interop-ipsec-test` by name, from its
// OWN advisory job, and that the job sets up Go
// (plan/spec-rfcgate-2-deferred-unrun-interop-trees.md AC-1).
// PREVENTS: the 12 strongSwan scenarios going back to having no automated caller.
// That is the condition under which rfc_requirements.py refuses an
// `RFC requirement:` tag in the tree as tier `unrun`, so deleting this job now
// DOWNGRADES the carrier rather than passing unnoticed.
//
// setup-go is asserted, not assumed boilerplate: test/interop-ipsec/run.py
// build_images() cross-compiles ze on the HOST before Docker COPYs it, so without
// a host toolchain the job dies at image build and reads as a scenario failure.
func TestEvidenceNightlyRunsIpsecInterop(t *testing.T) {
	targets := makeTargetsInWorkflow(t, "evidence-nightly.yml")
	if !slices.Contains(targets, "ze-interop-ipsec-test") {
		t.Errorf("evidence-nightly.yml must run `make ze-interop-ipsec-test`; targets found: %v", targets)
	}
	jobs := jobBlocks(t, "evidence-nightly.yml")
	var found *jobBlock
	for i, j := range jobs {
		if j.name == "ipsec-interop" {
			found = &jobs[i]
			break
		}
	}
	if found == nil {
		names := make([]string, 0, len(jobs))
		for _, j := range jobs {
			names = append(names, j.name)
		}
		t.Fatalf("evidence-nightly.yml has no `ipsec-interop` job; jobs found: %v", names)
	}
	if !found.advisory {
		t.Error("the ipsec-interop job must carry job-level `continue-on-error: true`: it ships advisory-first, " +
			"like every other job in this workflow")
	}
	if !strings.Contains(workflowFile(t, "evidence-nightly.yml"), "actions/setup-go") {
		t.Error("evidence-nightly.yml must set up Go: test/interop-ipsec/run.py cross-compiles ze on the host " +
			"before Docker COPYs it, so the ipsec-interop job needs a host toolchain")
	}
}

// TestWorkflowTargetExtractorsAgree
//
// VALIDATES: the Go parseMakeTargets above and the Python make_targets_in
// (scripts/dev/rfc_requirements.py) return the same make targets for every
// workflow file, and agree on which workflows are scheduled
// (plan/spec-rfcgate-2-deferred-unrun-interop-trees.md AC-7).
// PREVENTS: the two extractors drifting. The Python one decides the interop
// evidence tier from whether a SCHEDULED workflow names a tree's runner; if it
// saw a target this one does not (or missed one it does), the gate would believe
// in a caller CI does not have -- silently, and in the green direction.
func TestWorkflowTargetExtractorsAgree(t *testing.T) {
	root := repoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "rfc_requirements.py", "--workflow-targets")
	cmd.Dir = filepath.Join(root, "scripts", "dev")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running rfc_requirements.py --workflow-targets: %v", err)
	}
	var got map[string]struct {
		Targets   []string `json:"targets"`
		Scheduled bool     `json:"scheduled"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decoding --workflow-targets output: %v\n%s", err, out)
	}
	if len(got) == 0 {
		t.Fatal("the Python reader found no workflow files; this test must not pass vacuously")
	}
	entries, err := os.ReadDir(filepath.Join(root, workflowsDir))
	if err != nil {
		t.Fatalf("read %s: %v", workflowsDir, err)
	}
	seen := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		seen++
		py, ok := got[name]
		if !ok {
			t.Errorf("the Python reader did not report %s; it decides the interop tier and must see every workflow", name)
			continue
		}
		want := parseMakeTargets(workflowFile(t, name))
		if want == nil {
			want = []string{}
		}
		pyTargets := py.Targets
		if pyTargets == nil {
			pyTargets = []string{}
		}
		if !slices.Equal(want, pyTargets) {
			t.Errorf("%s: extractors disagree.\n  Go:     %v\n  Python: %v", name, want, pyTargets)
		}
		wantScheduled := strings.Contains(onBlock(t, name), "schedule")
		if wantScheduled != py.Scheduled {
			t.Errorf("%s: schedule classification disagrees: Go says %v, Python says %v", name, wantScheduled, py.Scheduled)
		}
	}
	if seen != len(got) {
		t.Errorf("workflow file count disagrees: Go saw %d, Python reported %d", seen, len(got))
	}
}

// ─── Orphaned QEMU / interop targets ────────────────────────────────────────

// quotedSpan matches a double- or single-quoted shell string.
//
// A recipe that PRINTS a command is not a recipe that RUNS one. mk/test-release.mk
// ends with a `printf "  make ze-interop-test\n"` hint per failed category, and a
// scan that reads those as calls would report every target named in a help or
// remediation line as wired. Dropping quoted spans before looking for `make`
// removes the whole class, `@echo "  ze-qemu-test-all  FULL suite"` included.
var quotedSpan = regexp.MustCompile(`"[^"]*"|'[^']*'`)

// makeInvocations returns the make targets a Makefile recipe body actually runs.
//
// parseMakeTargets models a workflow's command lines and documents that `$(MAKE)`
// is out of scope. Inside a make fragment `$(MAKE)` is the NORMAL spelling, and
// the aggregate that runs the heavy suites (ze-evidence-release-verify) reaches them
// through a shell function: `run_if_qemu qemu $(MAKE) --no-print-directory
// ze-qemu-integration-test`. So three things happen before the shared parser is
// handed the fragment: quoted spans go (see quotedSpan), `$(MAKE)` becomes `make`,
// and each fragment is re-anchored at its `make` token so a wrapper word in front
// of it does not hide the call.
//
// Comment lines are dropped. makefileRecipes already keeps only tab-indented
// lines, so this covers a `\t# ...` line inside a recipe.
func makeInvocations(recipe string) []string {
	var out []string
	for ln := range strings.SplitSeq(recipe, "\n") {
		ln = strings.TrimSpace(ln)
		ln = strings.TrimPrefix(ln, "@")
		ln = strings.TrimPrefix(ln, "-")
		if strings.HasPrefix(ln, "#") {
			continue
		}
		ln = quotedSpan.ReplaceAllString(ln, " ")
		ln = strings.ReplaceAll(ln, "$(MAKE)", "make")
		ln = strings.ReplaceAll(ln, "${MAKE}", "make")
		for _, sep := range []string{"&&", ";", "||", "|", "`"} {
			ln = strings.ReplaceAll(ln, sep, "\x00")
		}
		for frag := range strings.SplitSeq(ln, "\x00") {
			fields := strings.Fields(frag)
			for i, f := range fields {
				if f != "make" {
					continue
				}
				out = append(out, parseMakeTargets(strings.Join(fields[i:], " "))...)
				break
			}
		}
	}
	return out
}

// isQemuOrInteropTarget reports whether a make target belongs to the class this
// guard covers: the QEMU VM labs and the containerised interop runners. Both are
// expensive, both live outside `make ze-precommit-verify`, and both are therefore invisible
// to every gate a developer runs -- which is exactly the population where a
// working target can sit for months with nothing calling it.
func isQemuOrInteropTarget(name string) bool {
	if !strings.HasSuffix(name, "-test") {
		return false
	}
	if strings.HasPrefix(name, "ze-qemu-") {
		return true
	}
	return strings.HasPrefix(name, "ze-") && strings.Contains(name, "-interop-")
}

// manualQemuTargets names the targets of that class that deliberately have no
// automated caller, each with the reason. A row here is a DECISION and must say
// why no pipeline runs it -- "expensive" alone describes every target in the
// class. An entry whose target no longer exists fails the test, so the list
// cannot rot into an unread allowlist.
var manualQemuTargets = map[string]string{
	"ze-qemu-test-all": "runs the same driver as ze-qemu-needs-linux-test " +
		"(scripts/evidence/qemu-all-tests.sh) without ZE_QEMU_LINUX_ONLY, so nightly " +
		"coverage is a strict superset of what this adds; scheduling both would pay a " +
		"second ~90-minute VM run for no test that the first does not already execute. " +
		"It is the developer's full-suite pass before a release.",
	"ze-qemu-netns-test": "developer-only proof of the per-test netns launch path. " +
		"ze-qemu-integration-test already executes the host-netns refusal guard, while " +
		"this target adds a full cross-build and VM run for the same safety boundary.",
	"ze-qemu-vpp-hugepages-test": "needs a host with usable KVM access plus QEMU, " +
		"sshpass and e2fsprogs; ordinary hosted runners self-skip it, so scheduling it " +
		"there would report no boot evidence.",
	"ze-qemu-install-test": "needs an external ZE_INSTALL_KERNEL carrying the PXE, " +
		"virtio and ext4 symbols; the checkout and hosted runners do not carry that " +
		"artifact, so the target deliberately self-skips outside a prepared release host.",
	"ze-qemu-install-iso-test": "needs an external ZE_INSTALL_KERNEL with ISO9660 and " +
		"optical-drive support in addition to the installer symbols; no hosted workflow " +
		"provides that kernel, so the target is prepared-host release evidence.",
	"ze-qemu-install-scenarios-test": "needs the same external installer kernel as the " +
		"full-chain proof and exercises destructive failure, pin and rescue branches on " +
		"a prepared release host; ordinary runners would only record its self-skip.",
	"ze-qemu-install-ventoy-test": "needs the external installer kernel plus " +
		"grub-mkstandalone, xorriso and mtools; those release-host prerequisites are not " +
		"present in the hosted workflow, where this target deliberately self-skips.",
}

// TestQemuAndInteropTargetsHaveACaller
//
// VALIDATES: every `ze-qemu-*-test` and `ze-*-interop-test` target in the make
// fragments is invoked by something that runs on its own -- a workflow job, a
// script, or another make target -- or is listed in manualQemuTargets with a
// reason.
// PREVENTS: the defect class this repository has now met four times: a target
// that exists, works, and is called by nothing. `make ze-qemu-ldp-frr-test` drove
// a real FRR ldpd peer in a VM and was named only by its own `.PHONY` line, the
// `make help` text, and a comment explaining that internal/plugins/ldp was
// EXCLUDED from ze-qemu-integration-test in its favor. The coverage existed, was
// correct, and executed nowhere. Nothing in `make ze-precommit-verify` can see this: these
// targets are outside it by design, so their rot is silent by construction.
//
// Callers are derived from INVOCATION, never from mention. The three escapes that
// would otherwise make the guard vacuous:
//   - a `.PHONY:` line naming every target in the file (makefileRecipes attributes
//     no recipe and no prerequisite to a `.`-prefixed rule, so it declares nothing
//     this reads);
//   - a target's own recipe naming itself (self-calls are skipped below);
//   - a printed hint or help line (`printf "  make ze-x-test\n"`), which
//     makeInvocations drops with the quoted span it sits in.
//
// Documentation is deliberately NOT a caller. docs/functional-tests.md names all
// ten QEMU targets; eight of them ran nowhere while it said so.
func TestQemuAndInteropTargetsHaveACaller(t *testing.T) {
	root := repoRoot(t)

	frags, err := filepath.Glob(filepath.Join(root, "mk", "*.mk"))
	if err != nil || len(frags) == 0 {
		t.Fatalf("glob mk/*.mk: %v (%d found); layout changed. This test must not pass vacuously.", err, len(frags))
	}
	var sb strings.Builder
	sb.WriteString(readFileOrFail(t, filepath.Join(root, "Makefile")))
	for _, f := range frags {
		sb.WriteString("\n")
		sb.WriteString(readFileOrFail(t, f))
	}
	recipes := makefileRecipes(sb.String())

	var targets []string
	for name := range recipes {
		if isQemuOrInteropTarget(name) {
			targets = append(targets, name)
		}
	}
	slices.Sort(targets)
	if len(targets) == 0 {
		t.Fatal("no ze-qemu-*-test or ze-*-interop-test target found in Makefile or mk/*.mk; " +
			"the naming convention or the layout moved, and this test must not pass vacuously")
	}

	// callers[target] = the places that invoke it, for the failure message.
	callers := map[string][]string{}
	note := func(target, where string) {
		if isQemuOrInteropTarget(target) {
			callers[target] = append(callers[target], where)
		}
	}

	// (1) Another make target: a recipe that runs it, or a prerequisite list that
	// declares it. A target never counts as its own caller.
	for name, target := range recipes {
		for _, called := range append(makeInvocations(target.recipe), target.prereqs...) {
			if called != name {
				note(called, "make target "+name)
			}
		}
	}

	// (2) A workflow job.
	workflows, err := filepath.Glob(filepath.Join(root, workflowsDir, "*.y*ml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("glob %s/*.y*ml: %v (%d found)", workflowsDir, err, len(workflows))
	}
	for _, wf := range workflows {
		for _, called := range parseMakeTargets(stripComments(readFileOrFail(t, wf))) {
			note(called, "workflow "+filepath.Base(wf))
		}
	}

	// (3) A script. Comments are stripped first: scripts/evidence/qemu-all-tests.sh
	// opens with "# Invoked by `make ze-qemu-test-all`", which is a description of
	// its caller, not a call.
	scriptCount := 0
	err = filepath.Walk(filepath.Join(root, "scripts"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err //nolint:wrapcheck // walk callback: propagate as-is
		}
		if ext := filepath.Ext(path); ext != ".sh" && ext != ".py" {
			return nil
		}
		scriptCount++
		rel, _ := filepath.Rel(root, path)
		for _, called := range parseMakeTargets(stripComments(readFileOrFail(t, path))) {
			note(called, "script "+rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk scripts/: %v", err)
	}
	if scriptCount == 0 {
		t.Fatal("walked scripts/ and found no .sh or .py file; this test must not pass vacuously")
	}

	// The caller scan must find SOMETHING in this class, or it is broken rather
	// than reporting a repository with no wiring at all.
	if len(callers) == 0 {
		t.Fatalf("no caller found for ANY of %v; the invocation scan is broken. This test must not pass vacuously.", targets)
	}

	for _, target := range targets {
		reason, manual := manualQemuTargets[target]
		if len(callers[target]) > 0 {
			if manual {
				t.Errorf("%s is listed in manualQemuTargets (%q) but IS invoked by %v; "+
					"delete the entry -- a stale row makes the list unreadable as a set of decisions",
					target, reason, callers[target])
			}
			continue
		}
		if manual {
			continue
		}
		t.Errorf("%s is invoked by no workflow, no script and no other make target: it exists, it works, "+
			"and it runs nowhere. Give it a caller (a job in .github/workflows/, or an aggregate target), "+
			"or add it to manualQemuTargets with the reason no pipeline runs it. A mention in docs/ is not a caller.",
			target)
	}

	for target, reason := range manualQemuTargets {
		if _, ok := recipes[target]; !ok {
			t.Errorf("manualQemuTargets names %q (%q), which is no longer a target in Makefile or mk/*.mk: "+
				"the row is stale and the list must stay a set of live decisions", target, reason)
		}
	}
}

// TestMakeInvocationRefusesMentionsThatAreNotCalls
//
// VALIDATES: makeInvocations reads a call and refuses the three shapes that
// merely NAME a target -- a `.PHONY` declaration, a printed help or remediation
// line, and a comment -- while still seeing a call made through a shell wrapper
// with `$(MAKE)`.
// PREVENTS: TestQemuAndInteropTargetsHaveACaller degrading into a rubber stamp.
// Every orphaned target it exists to catch is already named by a `.PHONY` line
// and by `make help`; an extractor that counted either would report the whole
// class as wired and never fail for any reason.
func TestMakeInvocationRefusesMentionsThatAreNotCalls(t *testing.T) {
	const fragment = `.PHONY: ze-qemu-phony-test ze-qemu-wrapped-test

ze-qemu-phony-test:
	@echo "Running the phony lab..."
	python3 scripts/evidence/qemu-run.py --run 'go test ./...'

ze-qemu-wrapped-test:
	python3 scripts/evidence/qemu-run.py --run 'go test ./...'

ze-help-target:
	@echo "    ze-qemu-phony-test        a lab nothing runs"
	@printf "  make ze-qemu-phony-test\n"
	# make ze-qemu-phony-test

ze-aggregate-target:
	run_if_qemu qemu $(MAKE) --no-print-directory ze-qemu-wrapped-test; \
	true
`
	recipes := makefileRecipes(fragment)
	for _, want := range []string{"ze-qemu-phony-test", "ze-qemu-wrapped-test", "ze-help-target", "ze-aggregate-target"} {
		if _, ok := recipes[want]; !ok {
			t.Fatalf("the fixture no longer parses: %q missing from %v", want, slices.Sorted(maps.Keys(recipes)))
		}
	}
	if got := recipes["ze-help-target"]; len(makeInvocations(got.recipe)) != 0 {
		t.Errorf("a help echo, a printf hint and a comment must not read as calls; got %v",
			makeInvocations(got.recipe))
	}
	if got := makeInvocations(recipes["ze-aggregate-target"].recipe); !slices.Contains(got, "ze-qemu-wrapped-test") {
		t.Errorf("a $(MAKE) call behind a shell-function wrapper must read as a call; got %v", got)
	}
	// The .PHONY line names both targets and must contribute neither a recipe nor
	// a prerequisite: makefileRecipes skips `.`-prefixed rule heads entirely.
	for _, target := range recipes {
		if slices.Contains(target.prereqs, "ze-qemu-phony-test") {
			t.Error(".PHONY must not make a target read as a prerequisite of anything")
		}
	}
}
