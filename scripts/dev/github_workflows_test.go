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
//   - verify.yml is a push + pull_request gate that runs `make ze-verify` and
//     nothing heavy or scheduled (the merge gate stays fast).
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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
	advisory bool // carries `continue-on-error: true` as a DIRECT key of this job
}

// jobBlocks parses `jobs:` into per-job blocks and reports, for each, whether
// `continue-on-error: true` is one of that job's own keys.
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
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				continue
			}
			ind := indentOf(next)
			if len(ind) <= len(jobIndent) {
				break // back at job level: this job's block ended
			}
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
// VALIDATES: verify.yml runs `make ze-verify` on push and pull_request, and adds
// no scheduled or heavy work (the merge gate stays fast).
// PREVENTS: (a) the fast gate silently becoming scheduled-only or dropping a
// trigger; (b) a heavy suite being bolted onto every merge.
func TestVerifyWorkflowIsTheFastMergeGate(t *testing.T) {
	src := workflowFile(t, "verify.yml")
	if !slices.Contains(makeTargetsInWorkflow(t, "verify.yml"), "ze-verify") {
		t.Error("verify.yml must run `make ze-verify`")
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
		"ze-mutation-test", "ze-release-evidence",
	} {
		if strings.Contains(src, heavy) {
			t.Errorf("verify.yml must not run %q: it is a scheduled/heavy suite, and the merge gate stays fast", heavy)
		}
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
// (ai/rules/no-parking.md); (b) the VM run creeping onto push/pull_request, where
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

// TestCapabilityGatedTestsHaveAQemuHome
//
// VALIDATES: whenever a `.ci` test declares `option=needs-linux:caps=...`, the
// workflow set runs `ze-qemu-needs-linux-test` SPECIFICALLY -- the one target
// whose ZE_QEMU_LINUX_ONLY pass selects those tests -- and that no gated test
// also carries a `skip-env` that would exclude it from that very run.
// PREVENTS: the silent-coverage-deletion failure mode. A caps= option makes a
// test skip on every host lacking the capability, INCLUDING the verify runner,
// so marking tests with it and having no privileged runner would turn a red
// suite into a green one by removing the coverage (ai/rules/no-parking.md).
//
// The earlier version of this test accepted ANY `ze-qemu-*` target, which was
// fail-open: `ze-qemu-debug`, `ze-qemu-l2tp-ppp-test` and friends satisfy that
// prefix and run no `.ci` from the plugin or reload suites. It also could not
// see the second half of the problem, which is what actually bit:
// test/plugin/show-policy-routes.ci carried `skip-env:var=ZE_QEMU` from an era
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
// deterministic allocs/op gate is ze-alloc-gate, inside ze-verify).
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
// VALIDATES: the make targets the gh-pages deploy invokes BY NAME across the
// branch boundary still exist in this branch's Makefile set.
// PREVENTS: a rename on main breaking the website publish with nothing on main
// noticing. TestWorkflowMakeTargetsExist only globs THIS branch's
// .github/workflows, and main's pages.yml -- which used to invoke these two --
// was deleted so that gh-pages owns the deploy. The invocations did not go away
// with it: gh-pages/.github/workflows/pages.yml still runs
// `make -C main ze-terminal-demo-tools` and `make -C main
// ze-terminal-demos-release` against a checkout of THIS branch. That is a real
// dependency no glob on main can see, so it is pinned here explicitly.
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

	// Invoked by gh-pages/.github/workflows/pages.yml, which runs on the
	// gh-pages branch against a `main` checkout.
	for _, target := range []string{"ze-terminal-demo-tools", "ze-terminal-demos-release"} {
		if !strings.Contains(corpus, "\n"+target+":") {
			t.Errorf("the gh-pages deploy runs `make -C main %s`, but no such target exists in Makefile or mk/*.mk: "+
				"the website publish would fail with `No rule to make target` and nothing on main would report it", target)
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
