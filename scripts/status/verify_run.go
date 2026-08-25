// Design: docs/functional-tests.md -- verify failure routing protocol
// Related: scripts/dev/verify-status.sh -- freshness status consumer
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/hostload"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	combinedLogPath    = "tmp/ze-verify.log"
	failuresLogPath    = "tmp/ze-verify-failures.log"
	failuresJSONPath   = "tmp/ze-verify-failures.json"
	fullVerifyJSONPath = "tmp/ze-verify-full.json"
	statusPath         = "tmp/ze-verify.status"
	manifestPath       = "tmp/ze-verify-manifest.txt"
	stageLogDir        = "tmp/verify"

	// scopePackagesFile holds this run's change-set answer, inside the run's own
	// artifact directory. It is NOT published under a documented tmp/ path: two
	// sessions verify this checkout at once, and a shared name would hand one
	// run's scoped stages the other run's tree
	// (plan/spec-verify-scope-2-change-set-selector.md).
	scopePackagesFile = "verify-scope-packages.txt"

	// scopePackagesEnv names that file to every stage the run starts.
	// scripts/dev/changed-pkgs.sh reads it, so _ze-lint-changed-impl and
	// _ze-unit-test-changed-impl share the one answer instead of each paying for
	// its own `go list`.
	scopePackagesEnv = "ZE_VERIFY_SCOPE_PACKAGES"

	// scopeTagsFile holds this run's feature-tag answer, beside the package
	// answer and isolated for the same reason
	// (plan/spec-verify-scope-3-selector-consumers.md).
	scopeTagsFile = "verify-scope-tags.txt"

	// scopeTagsEnv names that file to every stage the run starts.
	// scripts/checks/staticcheck_feature_matrix.go reads it and judges only the
	// matrix rows the change set can move; 38 rows cost 874s, and the rows a
	// tag-local change cannot move are the bulk of it.
	//
	// It is set ONLY when the selection succeeded. Absence is what the matrix
	// reads as "judge every row", and an EMPTY answer is a real answer meaning
	// "no Go package changed", so a failed selection must not publish one
	// (ai/rules/evidence.md).
	scopeTagsEnv = "ZE_VERIFY_SCOPE_TAGS"

	// scopePackagesSection and scopeTagsSection are the headers the selector
	// writes under --print=both. Reading both answers from ONE selector run is
	// what keeps the per-run cost at one graph walk.
	scopePackagesSection = "# packages"
	scopeTagsSection     = "# tags"

	// everyPackageWord is the widest change-set answer, and the one the runner
	// writes when the selector cannot answer at all. An EMPTY answer would read
	// as "nothing to lint or test", which turns the selector's fail-open into a
	// fail-closed gate (ai/rules/evidence.md).
	everyPackageWord = "./..."

	maxInlineMembers = 8
	maxExcerptLines  = 20

	// runDirPrefix names a per-run artifact directory under stageLogDir. It is
	// also the selector pruneRunDirs uses, so nothing else living in
	// tmp/verify -- mk/test-alloc.mk writes alloc-gate-bench.txt there -- can
	// be mistaken for a run.
	runDirPrefix = "run-"

	// maxRetainedRunDirs bounds the artifact directories kept under
	// stageLogDir. A run older than the ten most recent ones finished hours
	// ago: a full verify takes 20 to 30 minutes, and the box runs one or two
	// at a time. Pruning by count rather than by age needs no clock and never
	// deletes the directory of a run that is still writing.
	maxRetainedRunDirs = 10

	// scopeSelectorDeadline bounds the one change-set selection. The selector's
	// own budget is 30s (AC-6 of the spec named above) and it measures 2.4 to
	// 2.9s; the slack covers `go run` compiling it first. The deadline exists so
	// a wedged toolchain fails the selection, and widens, rather than hanging
	// the run before its first stage.
	scopeSelectorDeadline = 5 * time.Minute

	// scopeSelectorSource is the one producer of the change-set answer. It is
	// `go run` rather than a built binary for the same reason the make target is
	// (ze-verify-scope-selector): the file carries //go:build ignore, so nothing
	// compiles it into a package, and go's build cache makes the second run of a
	// session cost nothing.
	scopeSelectorSource = "scripts/checks/verify_scope_selector.go"
)

type stage struct {
	Name    string
	Command []string
	Rerun   string
	Env     []string
}

type failureGroup struct {
	Stage     string   `json:"stage"`
	GroupID   string   `json:"group-id"`
	Kind      string   `json:"kind"`
	Related   []string `json:"related"`
	Summary   string   `json:"summary"`
	Rerun     string   `json:"rerun"`
	DetailLog string   `json:"detail-log"`
	Parallel  string   `json:"parallel"`
	Excerpt   []string `json:"excerpt,omitempty"`
}

type stageResult struct {
	Stage     string         `json:"stage"`
	ExitCode  int            `json:"exit-code"`
	DetailLog string         `json:"detail-log"`
	Groups    []failureGroup `json:"groups,omitempty"`
}

type verifyIndex struct {
	Mode        string        `json:"mode"`
	ExitCode    int           `json:"exit-code"`
	RunDir      string        `json:"run-dir"`
	CombinedLog string        `json:"combined-log"`
	GeneratedAt string        `json:"generated-at"`
	Stages      []stageResult `json:"stages"`
}

// runArtifacts is one run's own artifact set. Several sessions share this
// checkout and start verify runs at the same moment, so every file a run writes
// lives under Dir and no two runs ever write the same path. Each field is
// relative to the repository root and uses forward slashes, because these paths
// are published in the failure index that other tools read.
//
// The documented paths (combinedLogPath and the three failure artifacts) are
// SYMLINKS into the directory of the run that owns them: commit_helper.py reads
// tmp/ze-verify-full.json and tmp/ze-verify-failures.json, and every rule tells
// a session to read tmp/ze-verify.log. Isolation must not cost those readers
// their path.
type runArtifacts struct {
	Dir          string
	CombinedLog  string
	FailuresLog  string
	FailuresJSON string
	FullJSON     string
	// ScopePackages is the change-set answer this run's scoped stages read. It
	// is deliberately NOT published under a documented tmp/ path: it is an input
	// to the stages of THIS run, and a shared name would let a second session's
	// run rewrite it between two stages of the first.
	ScopePackages string
	// ScopeTags is the feature-tag half of that answer, isolated the same way.
	ScopeTags string
}

// newRunArtifacts creates the directory for one run and returns its paths.
//
// os.MkdirTemp is what makes the name unique: two runs starting in the same
// second, in one process or in two, cannot collide on it. The timestamp and the
// mode in the prefix are for a human reading `ls tmp/verify`, and the
// chronological sort they give is the order pruneRunDirs deletes in.
func newRunArtifacts(root, mode string, now time.Time) (runArtifacts, error) {
	parent := filepath.Join(root, filepath.FromSlash(stageLogDir))
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return runArtifacts{}, fmt.Errorf("create stage log dir: %w", err)
	}
	var tb textbuf.Buffer
	tb.Str(runDirPrefix).Str(now.UTC().Format("20060102T150405Z")).Byte('-').Str(safeStageLogName(mode)).Byte('-')
	// MkdirTemp creates the directory 0700, and umask can only take bits away, so
	// the mode needs no second step. Several sessions share this checkout under
	// one uid; nothing outside it reads a run's artifacts.
	dir, err := os.MkdirTemp(parent, tb.String())
	if err != nil {
		return runArtifacts{}, fmt.Errorf("create run artifact dir: %w", err)
	}
	rel := path.Join(stageLogDir, filepath.Base(dir))
	return runArtifacts{
		Dir:           rel,
		CombinedLog:   path.Join(rel, "ze-verify.log"),
		FailuresLog:   path.Join(rel, "ze-verify-failures.log"),
		FailuresJSON:  path.Join(rel, "ze-verify-failures.json"),
		FullJSON:      path.Join(rel, "ze-verify-full.json"),
		ScopePackages: path.Join(rel, scopePackagesFile),
		ScopeTags:     path.Join(rel, scopeTagsFile),
	}, nil
}

// publishArtifact points a documented path at this run's own file.
//
// The link is created under a unique staging directory and renamed into place,
// so a reader following the documented path sees either the previous run's file
// or this one, never a missing path. The link target is relative, so moving the
// checkout does not break it.
func publishArtifact(root, publishedRel, targetRel string) error {
	published := filepath.Join(root, filepath.FromSlash(publishedRel))
	target := filepath.Join(root, filepath.FromSlash(targetRel))
	linkTarget, err := filepath.Rel(filepath.Dir(published), target)
	if err != nil {
		linkTarget = target
	}
	staged, err := os.MkdirTemp(filepath.Dir(published), ".publish-")
	if err != nil {
		return fmt.Errorf("stage published path %s: %w", publishedRel, err)
	}
	defer os.RemoveAll(staged) //nolint:errcheck // best-effort cleanup of the staging directory
	link := filepath.Join(staged, filepath.Base(published))
	if err := os.Symlink(linkTarget, link); err != nil {
		return fmt.Errorf("link published path %s: %w", publishedRel, err)
	}
	// The link target is resolved against the DESTINATION directory, which the
	// rename below makes the parent of the link, so the staging directory never
	// has to resolve it.
	if err := os.Rename(link, published); err != nil {
		return fmt.Errorf("publish path %s: %w", publishedRel, err)
	}
	return nil
}

// pinnedRunDirs names the run directories a published path still points into.
// They are never pruned, however old they are: a published path whose target is
// gone reads as a missing artifact to every consumer of it.
//
// tmp/ze-verify-full.json is the case that forces this. Only a full run writes
// it (writeFailureArtifacts, guarded on modeFullVerify), so pruning by age
// alone would delete the full run's directory after maxRetainedRunDirs cheaper
// ze-precommit-verify-changed runs -- a day's work in a shared checkout.
// full_verify_coverage in scripts/dev/commit_helper.py reads that file and
// returns "uncovered" when it cannot, so the dangling link would refuse every
// Go-carrying commit until somebody spent 20 minutes on a fresh full run. That
// is the exact failure the full-mode copy exists to prevent.
//
// The link is read rather than resolved, so a target that is already missing
// still pins its name and this never depends on the file existing.
func pinnedRunDirs(root string) map[string]struct{} {
	pinned := make(map[string]struct{}, 4)
	for _, published := range []string{combinedLogPath, failuresLogPath, failuresJSONPath, fullVerifyJSONPath} {
		link := filepath.Join(root, filepath.FromSlash(published))
		target, err := os.Readlink(link)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(link), target)
		}
		pinned[filepath.Base(filepath.Dir(target))] = struct{}{}
	}
	return pinned
}

// pruneRunDirs removes the oldest run artifact directories, keeping at most
// maxRetainedRunDirs of them: this run's, every directory a published path
// points into, and the newest of what is left. Before per-run directories
// existed each run overwrote the previous one's files, so the artifact
// footprint was bounded by a single run; keeping every run instead would grow
// tmp/verify without limit.
//
// The retention COUNT is deliberately not the answer to the pinning problem: a
// count is a guess about how many changed-mode runs happen between two full
// ones, and nothing in this checkout bounds that number.
//
// Failures are ignored on purpose: another session can be reading or deleting
// the same directory, and housekeeping must not fail a verify run.
func pruneRunDirs(root, keepDir string) {
	parent := filepath.Join(root, filepath.FromSlash(stageLogDir))
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	pinned := pinnedRunDirs(root)
	pinned[keepDir] = struct{}{}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), runDirPrefix) {
			continue
		}
		if _, held := pinned[e.Name()]; held {
			continue
		}
		names = append(names, e.Name())
	}
	budget := max(maxRetainedRunDirs-len(pinned), 0)
	if len(names) <= budget {
		return
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names[budget:] {
		os.RemoveAll(filepath.Join(parent, name)) //nolint:errcheck // housekeeping never fails a run
	}
}

type verifyConfig struct {
	Root        string
	Mode        string
	Stages      []stage
	Now         func() time.Time
	Out         io.Writer
	RunStage    func(context.Context, string, stage, io.Writer) int
	WriteStatus func(root string, exitCode int, mode, skipped string, start treeSnapshot, now time.Time) error
	// SelectScope answers, once per run, which Go packages the change set
	// reaches and which features it can move. It is injected so a test can drive
	// the runner without paying for a real `go list` over the tree; production
	// leaves it nil and gets selectChangeSet.
	SelectScope func(root string, log io.Writer) (changeSetAnswer, error)
}

// changeSetAnswer is what the selector answers for one run.
type changeSetAnswer struct {
	// Packages holds ./-prefixed package directories the change reaches.
	Packages []string
	// Tags holds the feature tags the change can move. Empty is an answer: no
	// changed path is compiled by a gated package, so only the two rows that
	// omit no tag can move.
	Tags []string
}

// selectChangeSet runs the change-set selector once and returns both halves of
// its answer: the packages to retest and the feature tags to judge.
//
// The selector is the single producer of that answer
// (scripts/checks/verify_scope_selector.go). Running it HERE, before the first
// stage, is what makes it a per-run cost instead of a per-stage one: every
// scoped stage then reads a file the caller writes, through
// scripts/dev/changed-pkgs.sh for the packages and ZE_VERIFY_SCOPE_TAGS for the
// tags. --print=both is one run for two consumers, so adding the second
// consumer added no graph walk.
//
// Its stderr goes to the run log rather than being swallowed. That stream
// carries the paths no rule classifies and what the depth bound dropped, which
// is the evidence a reader needs to add a rule (ai/rules/repo-maintenance.md
// refuses a silent cap).
func selectChangeSet(root string, log io.Writer) (changeSetAnswer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), scopeSelectorDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", scopeSelectorSource, "--print=both") //nolint:gosec // every argument is a repository constant
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stderr = log
	out, err := cmd.Output()
	if err != nil {
		return changeSetAnswer{}, fmt.Errorf("run %s: %w", scopeSelectorSource, err)
	}
	return parseChangeSetAnswer(string(out))
}

// parseChangeSetAnswer reads the sectioned document --print=both writes.
//
// A missing section is an error rather than an empty half: the two halves mean
// different things when empty -- no package to retest, and no feature to judge
// beyond the shipped combinations -- so a truncated answer must widen the run
// instead of narrowing it silently.
func parseChangeSetAnswer(out string) (changeSetAnswer, error) {
	var answer changeSetAnswer
	seen := make(map[string]bool, 2)
	section := ""
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if line != scopePackagesSection && line != scopeTagsSection {
				return changeSetAnswer{}, fmt.Errorf("%s named an unknown section %q", scopeSelectorSource, line)
			}
			section = line
			seen[line] = true
			continue
		}
		switch section {
		case scopePackagesSection:
			answer.Packages = append(answer.Packages, line)
		case scopeTagsSection:
			answer.Tags = append(answer.Tags, line)
		default:
			return changeSetAnswer{}, fmt.Errorf("%s answered %q before naming a section", scopeSelectorSource, line)
		}
	}
	if !seen[scopePackagesSection] || !seen[scopeTagsSection] {
		return changeSetAnswer{}, fmt.Errorf(
			"%s answered %d of the 2 sections, so half the change set is unknown",
			scopeSelectorSource,
			len(seen),
		)
	}
	return answer, nil
}

// makeNoExecFlags are the GNU make short options under which recipes are not
// really executed, so every stage would "succeed" without doing anything:
//
//	n  -n / --dry-run / --just-print / --recon -- prints recipes, runs nothing
//	t  -t / --touch                            -- touches targets; a .PHONY
//	                                              target reports "Nothing to be
//	                                              done" and exits 0
//	q  -q / --question                         -- stage sub-makes no-op and
//	                                              exit 1 (the top-level make
//	                                              still runs a $(MAKE) line)
//
// All three share the property that matters here: make still EXECUTES recipe
// lines containing $(MAKE) (that is how recursive make participates in these
// modes), so this runner really starts, while the stage sub-makes it invokes do
// nothing. -n and -t then report success; -q reports failure, which is not a
// forgery but would still overwrite a good verify record with a false red.
const makeNoExecFlags = "ntq"

// makeDryRun reports whether the invoking make was run in one of those modes,
// by inspecting MAKEFLAGS.
//
// GNU make puts the concatenated single-letter flags in the FIRST whitespace
// -separated field, with no leading dash ("n", "rRn", "Bn",
// "n -j4 --jobserver-auth=…", "n -- FOO=bar"). If that field starts with a dash
// there are no short flags at all -- that is the case for "-j8 …" and for
// long-only options such as "--no-print-directory", both of which must NOT
// match. Only that first field is examined, so a target name or a variable
// value containing an "n"/"t"/"q" cannot cause a false positive.
//
// Verified against real GNU make 4.3 output rather than assumed; the observed
// strings are pinned in TestMakeDryRunDetectsDashN.
func makeDryRun(makeflags string) bool {
	fields := strings.Fields(makeflags)
	if len(fields) == 0 || strings.HasPrefix(fields[0], "-") {
		return false
	}
	// A short-flags word never contains '='. GNU make 3.81 (the macOS system
	// make) writes a command-line variable override as the FIRST word with no
	// `--` separator (`make ze-precommit-verify ZE_VERIFY_LOG=tmp/x.log` ->
	// MAKEFLAGS="ZE_VERIFY_LOG=tmp/x.log"); reading that as flags would refuse
	// the run on the 't' in "tmp". Newer makes separate overrides with `--`,
	// already handled by the leading-dash check above.
	if strings.ContainsRune(fields[0], '=') {
		return false
	}
	return strings.ContainsAny(fields[0], makeNoExecFlags)
}

// modeFullVerify is the default mode name, shared by main, --list and the
// Makefile targets.
const modeFullVerify = "ze-precommit-verify"

func main() {
	mode := modeFullVerify
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	// `--list` prints the stage list and runs nothing. It exists so that
	// "what does ze-precommit-verify run?" has a SAFE answer -- see the dry-run refusal
	// below for why `make -n ze-precommit-verify` is not one.
	if mode == "--list" {
		listMode := modeFullVerify
		if len(os.Args) > 2 {
			listMode = os.Args[2]
		}
		stages := stagesForMode(listMode, "make")
		if len(stages) == 0 {
			fmt.Fprintf(os.Stderr, "verify runner: unknown mode %q (want ze-precommit-verify or ze-precommit-verify-changed)\n", listMode)
			os.Exit(2)
		}
		for _, st := range stages {
			fmt.Println(st.Name) //nolint:errcheck // stdout
		}
		return
	}

	// REFUSE to run under make's no-execute modes (-n, -t, -q). This is a
	// fail-closed guard on the commit gate, not a convenience.
	//
	// `ze-precommit-verify`'s recipe contains $(MAKE), and GNU make executes such lines
	// even in those modes (it is how recursive make participates in them). So
	// `make -n ze-precommit-verify` really starts this program, with the flag propagated
	// through MAKEFLAGS to every stage sub-make -- and each stage then does
	// nothing. Under -n it echoes its recipe and exits 0; under -t a .PHONY
	// stage prints "Nothing to be done" and exits 0, which is quieter still.
	// Without this guard the run would collect a full sweep of green stages, write an
	// all-green tmp/ze-verify-failures.json, and write tmp/ze-verify.status with
	// exit=0 and the CURRENT tree hash -- after which
	// scripts/dev/verify-status.sh reports FRESH and commit_helper.py sees no
	// structural-gate reds. One `make -n` or `make -t` would certify a
	// completely unverified tree as fully verified. Refuse loudly instead.
	if makeDryRun(os.Getenv("MAKEFLAGS")) {
		fmt.Fprintln(os.Stderr, "verify runner: refusing to run under `make -n` / -t / -q (no-execute modes).")
		fmt.Fprintln(os.Stderr, "  This recipe contains $(MAKE), so make executes it even in those modes, while")
		fmt.Fprintln(os.Stderr, "  every stage sub-make does nothing. Under -n and -t the stages all report")
		fmt.Fprintln(os.Stderr, "  success, so writing tmp/ze-verify.status would forge a FRESH verify record")
		fmt.Fprintln(os.Stderr, "  for a completely unverified tree.")
		fmt.Fprintln(os.Stderr, "  To see the stage list without running anything: make ze-precommit-verify-list")
		os.Exit(2)
	}

	cfg := defaultVerifyConfig(mode, os.Stdout)
	code, err := runVerify(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify runner failed: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func defaultVerifyConfig(mode string, out io.Writer) verifyConfig {
	makeCmd := os.Getenv("ZE_VERIFY_MAKE")
	if makeCmd == "" {
		makeCmd = "make"
	}
	return verifyConfig{
		Root:        ".",
		Mode:        mode,
		Stages:      stagesForMode(mode, makeCmd),
		Now:         time.Now,
		Out:         out,
		RunStage:    execStage,
		WriteStatus: writeVerifyStatus,
	}
}

// stagesForMode is the SINGLE SOURCE OF TRUTH for what `make ze-precommit-verify` and
// `make ze-precommit-verify-changed` run. Both Makefile targets shell out to this runner,
// and .github/workflows/verify.yml runs every stage of this list too: each of its
// shards reads the list at run time with `make ze-precommit-verify-list` and takes
// every Nth line. So a gate that is not listed here runs NOWHERE, in CI or locally,
// and a gate ADDED here needs no edit to the workflow.
//
// Add a new gate to BOTH branches. The two lists are hand-duplicated on
// purpose (the changed-mode variants differ), which is precisely how they drift;
// TestStagesForModeMatchesGolden pins each against a committed golden so a
// one-branch edit fails loudly.
//
// A pair of duplicate `_ze-verify-impl` / `_ze-verify-changed-impl` Makefile
// targets used to shadow this list. They had zero callers, drifted for an
// unknown period, and were deleted by plan/spec-fixit-verify-stage-ssot.md.
// Do not reintroduce a second copy.
func stagesForMode(mode, makeCmd string) []stage {
	mk := func(name string) stage {
		return stage{
			Name:    name,
			Command: []string{makeCmd, "--no-print-directory", name},
			Rerun:   "make " + shellQuote(name),
		}
	}
	switch mode {
	case "ze-precommit-verify-changed":
		return []stage{
			mk("ze-lint-changed"),
			mk("ze-tier-check"),
			mk("ze-rfc-check"),
			mk("ze-iface-resolution-check"),
			mk("ze-plugin-boundary-check"),
			mk("ze-config-coercion-check"),
			mk("ze-fs-persistence-check"),
			mk("ze-dash-stdio-check"),
			mk("ze-port-defaults-check"),
			mk("ze-config-claims-check"),
			mk("ze-test-sensitivity-check"),
			mk("ze-test-weakened-check"),
			mk("ze-staticcheck-feature-matrix-check"),
			mk("ze-repository-tracked-build-check"),
			mk("ze-platform-vet"),
			mk("ze-doc-wiring-check"),
			mk("ze-doc-verify"),
			mk("ze-doc-links-check"),
			mk("ze-repository-tree-check"),
			mk("ze-generated-files-check"),
			mk("ze-vendor-web-check"),
			mk("ze-htmx-upgrade-check"),
			mk("ze-unit-hook-test"),
			mk("ze-dependency-vulnerability-check"),
			mk("ze-unit-test-changed"),
			mk("ze-functional-test"),
			mk("ze-functional-exabgp-test"),
		}
	case modeFullVerify:
		return []stage{
			mk("ze-lint"),
			mk("ze-tier-check"),
			mk("ze-rfc-check"),
			mk("ze-iface-resolution-check"),
			mk("ze-plugin-boundary-check"),
			mk("ze-config-coercion-check"),
			mk("ze-fs-persistence-check"),
			mk("ze-dash-stdio-check"),
			mk("ze-port-defaults-check"),
			mk("ze-config-claims-check"),
			mk("ze-test-sensitivity-check"),
			mk("ze-test-weakened-check"),
			mk("ze-staticcheck-feature-matrix-check"),
			mk("ze-repository-tracked-build-check"),
			mk("ze-platform-vet"),
			mk("ze-doc-wiring-check"),
			mk("ze-doc-verify"),
			mk("ze-doc-links-check"),
			mk("ze-repository-tree-check"),
			mk("ze-generated-files-check"),
			mk("ze-vendor-web-check"),
			mk("ze-htmx-upgrade-check"),
			mk("ze-evidence-vet"),
			mk("ze-unit-hook-test"),
			mk("ze-dependency-vulnerability-check"),
			mk("ze-unit-test-cached"),
			mk("ze-unit-test-race-changed"),
			mk("ze-alloc-check"),
			mk("ze-functional-test"),
			mk("ze-functional-exabgp-test"),
		}
	default:
		// FAIL CLOSED on an unknown mode. This used to be the `default` branch
		// returning the FULL list, so a typo (`ze-verify-chnaged`) silently ran
		// full verify and then wrote mode=ze-verify-chnaged into
		// tmp/ze-verify.status, which verify-status.sh rendered as
		// FRESH(ze-verify-chnaged) -- a verified-looking record for a mode
		// nobody asked for. runVerify turns the empty list into exit 2 with
		// "no verify stages configured".
		return nil
	}
}

func runVerify(ctx context.Context, cfg verifyConfig) (int, error) {
	if cfg.Root == "" {
		cfg.Root = "."
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.RunStage == nil {
		cfg.RunStage = execStage
	}
	if cfg.WriteStatus == nil {
		cfg.WriteStatus = writeVerifyStatus
	}
	if cfg.SelectScope == nil {
		cfg.SelectScope = selectChangeSet
	}
	if len(cfg.Stages) == 0 {
		return 2, errors.New("no verify stages configured")
	}

	if err := os.MkdirAll(filepath.Join(cfg.Root, "tmp"), 0o750); err != nil {
		return 2, fmt.Errorf("create tmp dir: %w", err)
	}

	started := cfg.Now()
	art, err := newRunArtifacts(cfg.Root, cfg.Mode, started)
	if err != nil {
		return 2, err
	}
	pruneRunDirs(cfg.Root, filepath.Base(art.Dir))

	combinedPath := filepath.Join(cfg.Root, filepath.FromSlash(art.CombinedLog))
	combined, err := openControlledFile(combinedPath)
	if err != nil {
		return 2, fmt.Errorf("open combined log: %w", err)
	}
	defer combined.Close() //nolint:errcheck // best-effort close at process exit

	// The combined log is published as the run STARTS, not when it ends: a
	// waiting session follows tmp/ze-verify.log for progress
	// (ai/rules/git-safety.md), and the run it follows is the newest one.
	if err := publishArtifact(cfg.Root, combinedLogPath, art.CombinedLog); err != nil {
		return 2, err
	}

	generatedAt := started.UTC().Format(time.RFC3339)
	out := io.MultiWriter(cfg.Out, combined)
	writef(out, "Ze verify protocol run: %s\nMode: %s\nRun directory: %s\nCombined log: %s (published at %s)\n\n",
		generatedAt, cfg.Mode, art.Dir, art.CombinedLog, combinedLogPath)

	if warn := contendedWarning(); warn != "" {
		writef(out, "WARNING: %s\n\n", warn)
	}

	// The change set, computed ONCE for the whole run. Every scoped stage reads
	// the file this writes, so a run pays for one graph walk however many scoped
	// stages it holds, and no two stages can scope to different trees.
	scopeAnswer, err := filepath.Abs(filepath.Join(cfg.Root, filepath.FromSlash(art.ScopePackages)))
	if err != nil {
		return 2, fmt.Errorf("place the change-set answer: %w", err)
	}
	tagAnswer, err := filepath.Abs(filepath.Join(cfg.Root, filepath.FromSlash(art.ScopeTags)))
	if err != nil {
		return 2, fmt.Errorf("place the feature-tag answer: %w", err)
	}
	answer, scopeErr := cfg.SelectScope(cfg.Root, out)
	if scopeErr != nil {
		// Widen. An unanswered selection must not reach a stage as an empty
		// package list: `make ze-lint-changed` reads that as "nothing to lint"
		// and reports success having linted nothing.
		writef(out, "WARNING: the change set could not be selected (%v), so every scoped stage widens to %s\n", scopeErr, everyPackageWord)
		answer = changeSetAnswer{Packages: []string{everyPackageWord}}
	}
	if err := writeScopeAnswer(scopeAnswer, answer.Packages); err != nil {
		return 2, err
	}
	writef(out, "Change set: %d package pattern(s), in %s\n", len(answer.Packages), art.ScopePackages)

	var scopeEnvBuf textbuf.Buffer
	scopeEnvBuf.Str(scopePackagesEnv).Byte('=').Str(scopeAnswer)
	scopeEnv := []string{scopeEnvBuf.String()}

	// The feature-tag half is published only when the selection ANSWERED. The
	// staticcheck matrix reads an absent variable as "judge every row", and an
	// empty file as "no feature is reachable, so judge the two shipped
	// combinations". A failed selection means the second, written down, would be
	// a fail-closed gate wearing a valid answer's shape.
	if scopeErr == nil {
		if err := writeScopeAnswer(tagAnswer, answer.Tags); err != nil {
			return 2, err
		}
		var tagEnvBuf textbuf.Buffer
		tagEnvBuf.Str(scopeTagsEnv).Byte('=').Str(tagAnswer)
		scopeEnv = append(scopeEnv, tagEnvBuf.String())
		writef(out, "Feature scope: %d feature tag(s), in %s\n\n", len(answer.Tags), art.ScopeTags)
	} else {
		writef(out, "Feature scope: unanswered, so every matrix row is judged\n\n")
	}

	// The tree as it stands BEFORE any stage runs. The certificate must name
	// what was verified, and the stages verify this tree: fingerprinting after
	// the loop stamps whatever the tree became, which in a shared checkout is a
	// different tree than the early stages judged.
	start := snapshotTree(cfg.Root)

	results := make([]stageResult, 0, len(cfg.Stages))
	exitCode := 0
	for i, st := range cfg.Stages {
		logRel := stageLogPath(art.Dir, i+1, st.Name)
		logPath := filepath.Join(cfg.Root, filepath.FromSlash(logRel))
		stageLog, openErr := openControlledFile(logPath)
		if openErr != nil {
			return 2, fmt.Errorf("open stage log %s: %w", logRel, openErr)
		}

		writer := io.MultiWriter(cfg.Out, combined, stageLog)
		writef(writer, "\n### Stage %02d/%02d: %s\n", i+1, len(cfg.Stages), st.Name)
		writef(writer, "$ %s\n", strings.Join(quoteCommand(st.Command), " "))

		// EXTEND the stage's own environment, never replace it: st is a copy of
		// the element, so the caller's slice keeps its length, and every variable
		// the stage list already carried survives.
		st.Env = append(st.Env, scopeEnv...)

		code := cfg.RunStage(ctx, cfg.Root, st, writer)
		writef(writer, "\n### Stage result: %s exit=%d\n", st.Name, code)
		stageLog.Close() //nolint:errcheck // subsequent read reports the real error if close failed

		res := stageResult{Stage: st.Name, ExitCode: code, DetailLog: logRel}
		if code != 0 {
			if exitCode == 0 {
				exitCode = 1
			}
			content, readErr := readControlledFile(logPath)
			if readErr != nil {
				res.Groups = []failureGroup{genericGroup(st, logRel, fmt.Sprintf("stage failed and log could not be read: %v", readErr), nil)}
			} else {
				res.Groups = classifyStage(st, logRel, string(content))
			}
		}
		results = append(results, res)
	}

	index := verifyIndex{
		Mode:        cfg.Mode,
		ExitCode:    exitCode,
		RunDir:      art.Dir,
		CombinedLog: art.CombinedLog,
		GeneratedAt: generatedAt,
		Stages:      results,
	}
	if err := writeFailureArtifacts(cfg.Root, art, index); err != nil {
		return 2, err
	}
	if err := cfg.WriteStatus(cfg.Root, exitCode, cfg.Mode, os.Getenv("ZE_SKIP_SUITES"), start, cfg.Now()); err != nil {
		return 2, fmt.Errorf("write verify status: %w", err)
	}

	printFinalSummary(io.MultiWriter(cfg.Out, combined), art, index)
	return exitCode, nil
}

// writeScopeAnswer writes the change set as one package pattern per line, the
// shape scripts/dev/changed-pkgs.sh emits and the make recipes expand as an
// unquoted word list.
//
// An empty answer writes an EMPTY file rather than no file at all. The
// difference is load-bearing for the reader: a missing file means no run
// published an answer and the caller must select its own, while an empty file
// is an answer, and it says no changed path is compiled or read by a Go package.
// writeScopeAnswer publishes one half of the change-set answer, one entry per
// line. An empty answer writes an empty file, which is a statement: the half was
// selected and reached nothing.
func writeScopeAnswer(path string, entries []string) error {
	var body strings.Builder
	for _, entry := range entries {
		body.WriteString(entry)
		body.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(body.String()), 0o600); err != nil {
		return fmt.Errorf("write the change-set answer: %w", err)
	}
	return nil
}

func execStage(ctx context.Context, root string, st stage, w io.Writer) int {
	if len(st.Command) == 0 {
		writeln(w, "stage has no command")
		return 2
	}
	cmd := exec.CommandContext(ctx, st.Command[0], st.Command[1:]...) //nolint:gosec // command list is fixed by repository code
	cmd.Dir = root
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Env = append(os.Environ(), "ZE_VERIFY_MODE=1")
	cmd.Env = append(cmd.Env, st.Env...)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		writef(w, "stage command failed: %v\n", err)
		return 1
	}
	return 0
}

func writeFailureArtifacts(root string, art runArtifacts, index verifyIndex) error {
	var text strings.Builder
	writef(&text, "# Ze verify failure index\n")
	writef(&text, "Generated: %s\n", index.GeneratedAt)
	writef(&text, "Mode: %s\n", index.Mode)
	writef(&text, "Run directory: %s\n", index.RunDir)
	writef(&text, "Combined log: %s\n\n", index.CombinedLog)

	failedStages := 0
	for i := range index.Stages {
		st := &index.Stages[i]
		if st.ExitCode == 0 {
			continue
		}
		failedStages++
		writef(&text, "## Stage: %s\n", st.Stage)
		writef(&text, "Exit: %d\n", st.ExitCode)
		writef(&text, "Detail log: %s\n\n", st.DetailLog)
		for j := range st.Groups {
			g := &st.Groups[j]
			writef(&text, "### Group: %s\n", g.GroupID)
			writef(&text, "Stage: %s\n", g.Stage)
			writef(&text, "Kind: %s\n", g.Kind)
			writef(&text, "Related: %s\n", formatInlineMembers(g.Related))
			writef(&text, "Summary: %s\n", g.Summary)
			writef(&text, "Rerun: %s\n", g.Rerun)
			writef(&text, "Detail log: %s\n", g.DetailLog)
			writef(&text, "Parallel: %s\n", g.Parallel)
			if len(g.Excerpt) > 0 {
				writeln(&text, "Excerpt:")
				for _, line := range cappedStrings(g.Excerpt, maxExcerptLines) {
					writef(&text, "  %s\n", line)
				}
				if len(g.Excerpt) > maxExcerptLines {
					writef(&text, "  ... %d more line(s); see detail log\n", len(g.Excerpt)-maxExcerptLines)
				}
			}
			writeln(&text)
		}
	}
	if failedStages == 0 {
		writeln(&text, "No failures.")
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(art.FailuresLog)), []byte(text.String()), 0o600); err != nil {
		return fmt.Errorf("write failure log: %w", err)
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal failure json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(art.FailuresJSON)), data, 0o600); err != nil {
		return fmt.Errorf("write failure json: %w", err)
	}
	// The full gate keeps its own copy, because a Go-carrying commit is gated on
	// having RUN it (ai/rules/git-safety.md, owner directive 2026-08-17) and
	// several sessions share this checkout: any one of them running the cheaper
	// ze-precommit-verify-changed republishes failuresJSONPath and would
	// otherwise erase the evidence that this session's full run ever happened.
	if index.Mode == modeFullVerify {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(art.FullJSON)), data, 0o600); err != nil {
			return fmt.Errorf("write full-verify json: %w", err)
		}
	}

	// Publish LAST, and only once each artifact is complete on disk. The
	// documented paths are what commit_helper.py and verify-status.sh read, so
	// a reader following one of them gets a whole index of one run, never a
	// half-written file and never two runs mixed.
	if err := publishArtifact(root, failuresLogPath, art.FailuresLog); err != nil {
		return err
	}
	if err := publishArtifact(root, failuresJSONPath, art.FailuresJSON); err != nil {
		return err
	}
	if index.Mode == modeFullVerify {
		if err := publishArtifact(root, fullVerifyJSONPath, art.FullJSON); err != nil {
			return err
		}
	}
	return nil
}

// printFinalSummary names this run's OWN artifacts before the documented ones.
// A concurrent run republishes tmp/ze-verify-failures.log the moment it
// finishes, so a summary that named only the documented path would send a
// reader to another run's failures.
func printFinalSummary(w io.Writer, art runArtifacts, index verifyIndex) {
	failed := 0
	for _, st := range index.Stages {
		if st.ExitCode != 0 {
			failed++
		}
	}
	writeln(w)
	writeln(w, "════════════════════════════════════════")
	if failed == 0 {
		writef(w, "PASS  all %d verify stage(s)\n", len(index.Stages))
		writef(w, "Artifacts: %s, %s, %s\n", art.CombinedLog, art.FailuresLog, statusPath)
		writef(w, "Published: %s, %s\n", combinedLogPath, failuresLogPath)
		return
	}
	writef(w, "FAIL  %d verify stage(s) failed\n", failed)
	writef(w, "Read first: %s (published at %s)\n", art.FailuresLog, failuresLogPath)
	for i := range index.Stages {
		st := &index.Stages[i]
		if st.ExitCode == 0 {
			continue
		}
		writef(w, "  %s: %d group(s), detail %s\n", st.Stage, len(st.Groups), st.DetailLog)
		for j := range st.Groups {
			g := &st.Groups[j]
			writef(w, "    %s: %s\n", g.GroupID, g.Rerun)
		}
	}
}

func classifyStage(st stage, detailLog, text string) []failureGroup {
	groups := classifiedGroups(st, detailLog, text)
	if truncated, ok := truncatedLogGroup(detailLog, splitLines(text)); ok {
		groups = append(groups, truncated)
	}
	return normalizedGroups(st, detailLog, groups)
}

// functionalStage is the one stage whose classifier reads the declared groups
// ITSELF. classifiedGroups names it twice for that reason, and
// TestTheFunctionalStageKeepsItsSummaryReconciliation holds the two uses
// together.
const functionalStage = "ze-functional-test"

func classifiedGroups(st stage, detailLog, text string) []failureGroup {
	// A producer that named its own failures beats any classifier that reads its
	// prose, and every stage is asked, not only the six named below: the others
	// fall through to genericGroup, whose Kind the commit helper refuses to read
	// paths from, so nothing they print can be attributed today.
	//
	// The functional stage is the exception, and it is asked through its own
	// classifier instead. classifyFunctional reads the declared groups with the
	// same parser and then reconciles them against the FAIL summary, which names
	// every suite that failed. That is a STRONGER completeness statement than the
	// count, so the shortcut here would silently replace it the day a functional
	// producer starts printing a terminator.
	if st.Name != functionalStage {
		if declared, complete := parseDeclaredGroups(detailLog, text); complete && len(declared) > 0 {
			return declared
		}
	}
	var groups []failureGroup
	switch st.Name {
	case "ze-unit-test-cached", "ze-unit-test-race-changed", "ze-unit-test-changed":
		groups = classifyGoTest(st, detailLog, text)
	case "ze-lint", "ze-lint-changed":
		groups = classifyLint(st, detailLog, text)
	case "ze-evidence-vet", "ze-platform-vet":
		groups = classifyVet(st, detailLog, text)
	case "ze-doc-wiring-check":
		groups = classifyWiringDocs(st, detailLog, text)
	case functionalStage:
		groups = classifyFunctional(detailLog, text)
	case "ze-functional-exabgp-test":
		groups = classifyExabgp(st, detailLog, text)
	}
	if len(groups) == 0 {
		groups = []failureGroup{genericGroup(st, detailLog, firstUsefulLine(text), excerptFromText(text))}
	}
	return groups
}

// truncatedLogGroup is the group a stage earns when its log could not be read to
// the end, and false when it could.
//
// splitLines appends logTruncatedMarker and stops there, so every failure the
// lost tail reported is missing from the classification. An incomplete
// classification is exactly what a failure group says, and no other reader
// carries the marker: no classifier's regex matches it, and the excerpt cannot
// either, because excerptFromText keeps the first maxExcerptLines+1 non-empty
// lines while a log holding a line over maxLogLineBytes is by definition longer
// than that.
//
// The kind is subcheck, which PATH_BEARING_GROUP_KINDS (scripts/dev/commit_helper.py)
// does not hold, so the gate is CHARGED. What the unread tail said cannot be attributed
// to anyone, and an unattributable failure is charged, never dropped
// (ai/rules/evidence.md).
func truncatedLogGroup(detailLog string, lines []string) (failureGroup, bool) {
	if len(lines) == 0 || !strings.HasPrefix(lines[len(lines)-1], logTruncatedMarker) {
		return failureGroup{}, false
	}
	marker := lines[len(lines)-1]
	return failureGroup{
		GroupID:   "subcheck:stage-log-truncated",
		Kind:      "subcheck",
		Related:   []string{"stage-log-truncated"},
		Summary:   marker,
		DetailLog: detailLog,
		Parallel:  wholeStage,
		Excerpt:   []string{marker},
	}, true
}

// normalizedGroups fills in what a group left empty and caps what it left long,
// so every group in the failure index carries the same fields whichever
// classifier or producer built it.
func normalizedGroups(st stage, detailLog string, groups []failureGroup) []failureGroup {
	for i := range groups {
		if groups[i].Stage == "" {
			groups[i].Stage = st.Name
		}
		if groups[i].DetailLog == "" {
			groups[i].DetailLog = detailLog
		}
		if groups[i].Rerun == "" {
			groups[i].Rerun = st.Rerun
		}
		if groups[i].Parallel == "" {
			groups[i].Parallel = wholeStage
		}
		groups[i].Related = uniqueStrings(groups[i].Related)
		groups[i].Excerpt = cappedStrings(groups[i].Excerpt, maxExcerptLines+1)
	}
	return groups
}

// wholeStage is how both failureGroup.Kind and failureGroup.Parallel spell a
// failure that belongs to the whole stage rather than to one group inside it.
// The commit helper keeps this token OUT of PATH_BEARING_GROUP_KINDS
// (scripts/dev/commit_helper.py), so a group of this kind is charged rather than
// attributed.
const wholeStage = "stage"

// declaredGroupPrefix introduces one JSON failureGroup a producer declared for
// itself. declaredCompletePrefix introduces that producer's count of them.
const (
	declaredGroupPrefix    = "VERIFY FAILURE GROUP:"
	declaredCompletePrefix = "VERIFY FAILURE GROUPS COMPLETE:"
)

// parseDeclaredGroups reads the failure groups a producer declared for itself,
// and whether it declared one for EVERY failure it reported.
//
// A producer knows which files its failure is about, and a classifier reading
// its prose is guessing. So any stage MAY print declaredGroupPrefix and one JSON
// failureGroup at the point of failure, then finish with declaredCompletePrefix
// and the number of groups it printed.
//
// complete is that number agreeing with what this parser read, and it is the
// safety property of the protocol. classifyStage replaces its groups with
// genericGroup only when the slice is EMPTY, so a run that declared groups for
// some of its failures and not for the rest would fill the slice and take the
// missed failures out of the failure index with it. A producer that declares
// nothing, that dies before its count, or that has another program's group line
// relayed into its log therefore reports false here, and the caller keeps the
// classifier it would have used anyway.
//
// A line that does not parse becomes a group of its own. The producer printed it
// because something failed, so skipping it deletes that failure from the index.
func parseDeclaredGroups(detailLog, text string) (groups []failureGroup, complete bool) {
	var counts []string
	for _, line := range splitLines(text) {
		// The group line is read first: a producer whose summary or path quotes
		// declaredCompletePrefix must not be read as a count.
		_, payload, isGroup := strings.Cut(line, declaredGroupPrefix)
		if !isGroup {
			if _, count, isCount := strings.Cut(line, declaredCompletePrefix); isCount {
				counts = append(counts, strings.TrimSpace(count))
			}
			continue
		}
		var g failureGroup
		if err := json.Unmarshal([]byte(strings.TrimSpace(payload)), &g); err != nil {
			groups = append(groups, unparsedGroup(len(groups), detailLog, line, err))
			continue
		}
		if g.DetailLog == "" {
			g.DetailLog = detailLog
		}
		if g.Parallel == "" {
			g.Parallel = "group"
		}
		groups = append(groups, g)
	}
	if len(counts) != 1 {
		return groups, false
	}
	declared, err := strconv.Atoi(counts[0])
	if err != nil {
		return groups, false
	}
	return groups, declared == len(groups)
}

// unparsedGroup is what a declaredGroupPrefix line gets when its JSON does not
// parse. Its kind is outside PATH_BEARING_GROUP_KINDS (scripts/dev/commit_helper.py),
// so group_related_paths reads no path out of it whatever the checkout holds and
// the commit helper charges the gate: a failure nobody could attribute is
// charged, never dropped (ai/rules/evidence.md).
func unparsedGroup(index int, detailLog, line string, err error) failureGroup {
	var tb textbuf.Buffer
	groupID := tb.Str("unparsed-group:").Int(int64(index)).String()
	tb.Reset()
	return failureGroup{
		GroupID:   groupID,
		Kind:      "unparsed",
		Related:   []string{"unparsed-group"},
		Summary:   tb.Str("a declared failure group line did not parse: ").Err(err).String(),
		DetailLog: detailLog,
		Parallel:  "group",
		Excerpt:   []string{line},
	}
}

func classifyGoTest(st stage, detailLog, text string) []failureGroup {
	lines := splitLines(text)
	groupsByPkg := map[string]*failureGroup{}
	order := []string{}
	var pendingTests []string
	var pendingExcerpt []string
	var compilePkg string
	pkgHeaderRE := regexp.MustCompile(`^#\s+(\S+)`)
	failPkgRE := regexp.MustCompile(`^FAIL\s+(\S+)(?:\s|$)`)
	testRE := regexp.MustCompile(`^--- FAIL: ([^\s(]+)`) // standard go test failure line
	for _, line := range lines {
		if m := pkgHeaderRE.FindStringSubmatch(line); m != nil {
			compilePkg = m[1]
			appendCapped(&pendingExcerpt, line)
			continue
		}
		if m := testRE.FindStringSubmatch(line); m != nil {
			pendingTests = append(pendingTests, m[1])
			appendCapped(&pendingExcerpt, line)
			continue
		}
		if strings.HasPrefix(line, "    ") || strings.Contains(line, ".go:") || strings.Contains(line, "Error") {
			appendCapped(&pendingExcerpt, line)
		}
		m := failPkgRE.FindStringSubmatch(line)
		if len(m) != 2 || m[1] == "FAIL" {
			continue
		}
		pkg := m[1]
		if compilePkg != "" {
			pkg = compilePkg
		}
		const kindBuild = "build"
		kind := "package"
		if strings.Contains(line, "[build failed]") || compilePkg != "" {
			kind = kindBuild
		}
		g, ok := groupsByPkg[pkg]
		if !ok {
			g = &failureGroup{Stage: st.Name, GroupID: "package:" + pkg, Kind: kind, Summary: strings.TrimSpace(line), DetailLog: detailLog, Parallel: "group"}
			groupsByPkg[pkg] = g
			order = append(order, pkg)
		}
		if kind == kindBuild {
			g.Kind = kind
		}
		if len(pendingTests) == 0 {
			g.Related = append(g.Related, pkg)
		} else {
			g.Related = append(g.Related, pendingTests...)
		}
		g.Excerpt = append(g.Excerpt, pendingExcerpt...)
		g.Excerpt = append(g.Excerpt, line)
		g.Rerun = goTestRerun(st.Name, pkg, pendingTests)
		pendingTests = nil
		pendingExcerpt = nil
		compilePkg = ""
	}
	groups := make([]failureGroup, 0, len(order))
	for _, pkg := range order {
		groups = append(groups, *groupsByPkg[pkg])
	}
	return groups
}

// classifyLint splits a lint red into one group per package and linter, whose
// Related members are the .go files golangci-lint named.
//
// The kind is the constant word, never the linter: PATH_BEARING_GROUP_KINDS
// (scripts/dev/commit_helper.py) is an allowlist, and it can only be an
// allowlist while Kind is a CLOSED vocabulary. golangci-lint owns the linter
// names, so putting one in Kind would hand that consumer a set nobody here can
// enumerate. The linter survives where it is read from: the group id
// (`lint:<dir>:<linter>`) and the rerun command.
func classifyLint(st stage, detailLog, text string) []failureGroup {
	const kindLint = "lint"
	lineRE := regexp.MustCompile(`^([^:\s][^:]*\.go):\d+:\d+:\s*(.*?)(?:\s+\(([^)]+)\))?$`)
	groups := map[string]*failureGroup{}
	order := []string{}
	for _, line := range splitLines(text) {
		m := lineRE.FindStringSubmatch(stripANSI(line))
		if m == nil {
			continue
		}
		file := filepath.ToSlash(m[1])
		linter := m[3]
		if linter == "" {
			linter = "lint"
		}
		dir := filepath.ToSlash(filepath.Dir(file))
		if dir == "." {
			dir = "."
		}
		key := dir + ":" + linter
		g, ok := groups[key]
		if !ok {
			pkg := "./" + dir
			if dir == "." {
				pkg = "."
			}
			var rerun textbuf.Buffer
			rerun.Str("golangci-lint run ").Str(shellQuote(pkg))
			g = &failureGroup{Stage: st.Name, GroupID: groupID("lint", key), Kind: kindLint, Summary: strings.TrimSpace(m[2]), Rerun: rerun.String(), DetailLog: detailLog, Parallel: "group"}
			groups[key] = g
			order = append(order, key)
		}
		g.Related = append(g.Related, file)
		g.Excerpt = append(g.Excerpt, line)
	}
	return orderedGroups(groups, order)
}

func classifyVet(st stage, detailLog, text string) []failureGroup {
	pkgRE := regexp.MustCompile(`^#\s+(\S+)`)
	groups := map[string]*failureGroup{}
	order := []string{}
	current := "./scripts/evidence/..."
	for _, line := range splitLines(text) {
		if m := pkgRE.FindStringSubmatch(line); m != nil {
			current = importPathToPattern(m[1])
		}
		if !strings.Contains(line, ".go:") && !strings.HasPrefix(line, "# ") {
			continue
		}
		g, ok := groups[current]
		if !ok {
			g = &failureGroup{Stage: st.Name, GroupID: "vet:" + current, Kind: "package", Summary: strings.TrimSpace(line), Rerun: "GOOS=linux go vet " + shellQuote(current), DetailLog: detailLog, Parallel: "group"}
			groups[current] = g
			order = append(order, current)
		}
		g.Related = append(g.Related, current)
		g.Excerpt = append(g.Excerpt, line)
	}
	return orderedGroups(groups, order)
}

func classifyWiringDocs(st stage, detailLog, text string) []failureGroup {
	var groups []failureGroup
	current := ""
	runningRE := regexp.MustCompile(`^Running ([A-Za-z0-9_-]+)\.{3}`)
	failedRE := regexp.MustCompile(`([A-Za-z0-9_-]+) failed`)
	for _, line := range splitLines(text) {
		if strings.Contains(line, "Wiring check FAILED") {
			groups = append(groups, failureGroup{Stage: st.Name, GroupID: "subcheck:wiring", Kind: "subcheck", Related: []string{"wiring"}, Summary: strings.TrimSpace(line), Rerun: "python3 scripts/dev/verify_wiring_docs.py", DetailLog: detailLog, Parallel: "group", Excerpt: []string{line}})
			current = "wiring"
			continue
		}
		if m := runningRE.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if m := failedRE.FindStringSubmatch(line); m != nil {
			target := m[1]
			if current != "" && current != target {
				target = current
			}
			groups = append(groups, failureGroup{Stage: st.Name, GroupID: "subcheck:" + target, Kind: "subcheck", Related: []string{target}, Summary: strings.TrimSpace(line), Rerun: "make " + shellQuote(target), DetailLog: detailLog, Parallel: "group", Excerpt: []string{line}})
		}
	}
	return groups
}

// classifyFunctional groups the functional stage's failures.
//
// The suite runner declares its own groups, so this reads them with the shared
// parser. It ignores the completeness count, because the functional stage states
// completeness a second way that predates the count and is stronger: the FAIL
// summary names every suite that failed, and the loop below adds a group for
// each suite no declared group covered.
func classifyFunctional(detailLog, text string) []failureGroup {
	groups, _ := parseDeclaredGroups(detailLog, text)
	seenSuite := map[string]bool{}
	for i := range groups {
		seenSuite[groups[i].Stage] = true
	}

	summaryRE := regexp.MustCompile(`FAIL\s+\d+ suite\(s\) failed:\s+(.+)`)
	for _, line := range splitLines(stripANSI(text)) {
		m := summaryRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for suite := range strings.FieldsSeq(m[1]) {
			if seenSuite[suite] {
				continue
			}
			groups = append(groups, failureGroup{Stage: suite, GroupID: groupID("suite", suite), Kind: "suite", Related: []string{suite}, Summary: "functional suite failed", Rerun: functionalSuiteRerun(suite), DetailLog: detailLog, Parallel: wholeStage, Excerpt: []string{line}})
			seenSuite[suite] = true
		}
	}
	return groups
}

// functionalSuiteRerun returns the command that re-runs one functional suite.
// Every gating suite (GATING, scripts/le/application/functional.py) has a target
// of this name, and TestFunctionalSuiteRerunNamesARealMakeTarget holds that true: a
// failure report earns its place by naming a command the reader can type, and a
// name make answers with `No rule to make target` costs the reader twice, once
// for the failure and once for finding out the advice was wrong.
func functionalSuiteRerun(suite string) string {
	var tb textbuf.Buffer
	return tb.Str("make ze-functional-").Str(suite).Str("-test").String()
}

func classifyExabgp(st stage, detailLog, text string) []failureGroup {
	lines := splitLines(stripANSI(text))
	var failed []string
	var timedOut []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "failed") && strings.Contains(line, "[") {
			failed = append(failed, parseBracketMembers(line)...)
		}
		if strings.HasPrefix(line, "timed out") && strings.Contains(line, "[") {
			timedOut = append(timedOut, parseBracketMembers(line)...)
		}
	}
	var groups []failureGroup
	if len(failed) > 0 {
		groups = append(groups, failureGroup{Stage: st.Name, GroupID: "exabgp:failed", Kind: "mismatch", Related: failed, Summary: fmt.Sprintf("%d ExaBGP encoding test(s) failed", len(failed)), Rerun: exabgpRerun(failed), DetailLog: detailLog, Parallel: "group", Excerpt: excerptFromText(text)})
	}
	if len(timedOut) > 0 {
		groups = append(groups, failureGroup{Stage: st.Name, GroupID: "exabgp:timeout", Kind: "timeout", Related: timedOut, Summary: fmt.Sprintf("%d ExaBGP encoding test(s) timed out", len(timedOut)), Rerun: exabgpRerun(timedOut), DetailLog: detailLog, Parallel: "group", Excerpt: excerptFromText(text)})
	}
	return groups
}

func genericGroup(st stage, detailLog, summary string, excerpt []string) failureGroup {
	if summary == "" {
		summary = "stage failed"
	}
	return failureGroup{Stage: st.Name, GroupID: groupID("stage", st.Name), Kind: wholeStage, Related: []string{st.Name}, Summary: summary, Rerun: st.Rerun, DetailLog: detailLog, Parallel: wholeStage, Excerpt: excerpt}
}

func goTestRerun(stageName, pkg string, tests []string) string {
	pkgPattern := importPathToPattern(pkg)
	args := []string{"go", "test"}
	if stageName == "ze-unit-test-race-changed" {
		args = append(args, "-race")
	}
	args = append(args, pkgPattern)
	if len(tests) == 1 {
		args = append(args, "-run", "^"+regexp.QuoteMeta(tests[0])+"$")
	}
	return strings.Join(quoteCommand(args), " ")
}

func exabgpRerun(nicks []string) string {
	args := make([]string, 0, maxInlineMembers+9)
	args = append(args, "uv", "run", "--with", "psutil", "--with", "paramiko", "./test/exabgp-compat/bin/functional", "encoding", "--timeout", "180")
	args = append(args, cappedStrings(nicks, maxInlineMembers)...)
	return strings.Join(quoteCommand(args), " ")
}

func importPathToPattern(pkg string) string {
	const module = "github.com/ze-software/ze"
	if pkg == module {
		return "."
	}
	if suffix, ok := strings.CutPrefix(pkg, module+"/"); ok {
		return "./" + suffix
	}
	return pkg
}

// stageLogPath names one stage's log inside a run's artifact directory. The
// number is zero padded to two digits so `ls` sorts the stages in run order.
func stageLogPath(runDir string, number int, stage string) string {
	var tb textbuf.Buffer
	tb.Str(runDir).Byte('/')
	if number < 10 {
		tb.Byte('0')
	}
	tb.Int(int64(number)).Byte('-').Str(safeStageLogName(stage)).Str(".log")
	return tb.String()
}

func safeStageLogName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "stage"
	}
	return b.String()
}

// splitLines splits captured stage output into lines.
//
// The reader is a strings.Reader, so Read returns only io.EOF. Every caller
// shapes the failure REPORT of a stage that already failed; the run's verdict
// is the stage exit code collected in runVerify, never anything derived here.
//
// One stop is NOT an io.EOF: a line over bufio.MaxScanTokenSize ends the scan
// and takes the rest of the log with it. So a producer that declares its own
// groups keeps each line short rather than relying on the reader
// (RELATED_PER_GROUP, scripts/dev/verify_wiring_docs.py). The repo-wide record
// of this scanner class is plan/journal/silent-fall-through.md, 2026-08-12.
// splitLines is the reader every classifier in this file goes through, so a
// truncated read here loses failure GROUPS rather than log text.
//
// `bufio.Scanner` stops on a line above its token limit, and `Scan` returns
// false for that exactly as it does for EOF. With the DEFAULT 64 KiB limit and
// no `Err()` check, one over-long line ended the loop and discarded that line
// AND EVERY LINE AFTER IT, in silence: the classifier then saw a prefix of the
// stage log and reported whichever failures that prefix happened to contain.
// Stage logs here reach 900 KB and a single line can be arbitrarily long, so
// the exemption the 2026-08-12 sweep granted a `strings.Reader` over "this
// process's own short output" (plan/journal/silent-fall-through.md) does not
// cover this caller.
//
// The limit is raised, and a read that still ends early appends a MARKER rather
// than vanishing. The marker reaches a reader through ONE route, and it is
// classifyStage turning it into `truncatedLogGroup`: a log with an over-long
// line is longer than any excerpt, so the marker is always past the excerpt cap,
// and no classifier's regex matches it. Being a group of its own is also what
// charges it, which a rendered line would not do. `parseDeclaredGroups` counts
// group lines against a declared total on top of that, so a read truncated
// before a producer's count falls back instead of trusting a partial set
// (ai/rules/evidence.md).
const (
	maxLogLineBytes    = 4 << 20
	logTruncatedMarker = "### verify runner: stage log read ended early, so what follows was not classified"
)

func splitLines(text string) []string {
	s := bufio.NewScanner(strings.NewReader(text))
	s.Buffer(make([]byte, 0, 64<<10), maxLogLineBytes)
	lines := []string{}
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		var tb textbuf.Buffer
		lines = append(lines, tb.Str(logTruncatedMarker).Str(": ").Err(err).String())
	}
	return lines
}

func firstUsefulLine(text string) string {
	for _, line := range splitLines(stripANSI(text)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "$ ") || strings.HasPrefix(trimmed, "### Stage") {
			continue
		}
		return trimmed
	}
	return "stage failed"
}

func excerptFromText(text string) []string {
	var excerpt []string
	for _, line := range splitLines(stripANSI(text)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "$ ") || strings.HasPrefix(trimmed, "### Stage") {
			continue
		}
		appendCapped(&excerpt, trimmed)
	}
	return excerpt
}

func appendCapped(dst *[]string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if len(*dst) < maxExcerptLines+1 {
		*dst = append(*dst, value)
	}
}

func cappedStrings(values []string, cap int) []string {
	if len(values) <= cap {
		return values
	}
	out := make([]string, cap)
	copy(out, values[:cap])
	return out
}

func formatInlineMembers(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	shown := cappedStrings(values, maxInlineMembers)
	text := strings.Join(shown, ", ")
	if len(values) > maxInlineMembers {
		text += fmt.Sprintf(", +%d more", len(values)-maxInlineMembers)
	}
	return text
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := values[:0]
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func orderedGroups(groups map[string]*failureGroup, order []string) []failureGroup {
	out := make([]failureGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *groups[key])
	}
	return out
}

func parseBracketMembers(line string) []string {
	start := strings.IndexByte(line, '[')
	end := strings.LastIndexByte(line, ']')
	if start < 0 || end <= start {
		return nil
	}
	parts := strings.Split(line[start+1:end], ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if nick := strings.TrimSpace(part); nick != "" {
			out = append(out, nick)
		}
	}
	return out
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// groupID builds a failure group's identifier, "<kind>:<key>". One helper, so
// the three call sites do not each write a `+` between two strings: that
// operator allocates a backing array and copies both sides, and the performance
// rule bans it in new code.
func groupID(kind, key string) string {
	var tb textbuf.Buffer
	return tb.Str(kind).Byte(':').Str(key).String()
}

var safeShellRE = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

func shellQuote(s string) string {
	if safeShellRE.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func quoteCommand(args []string) []string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return quoted
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) //nolint:errcheck // output
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...) //nolint:errcheck // output
}

// treeMovedSentinel is written as the tree_hash when the tree changed while the
// run was in flight. verify-status.sh reports FRESH only when the live hash
// EQUALS the recorded one, so a value no tree can hash to reports STALE. That is
// the fail-closed answer: when the stages judged more than one tree, no single
// hash is true for the run, and asserting one would certify content that was
// never verified.
//
// It survives at WHOLE-TREE granularity alone, where the question genuinely has
// no answer: an edit reverted before the check leaves the start hash matching
// again while the stages read content nobody verified. The per-path question is
// answered by the manifest instead, where only the paths that moved carry
// movedDuringRun and every other path keeps a fingerprint a later check matches.
// One session's edit therefore costs the run that session's paths, not the run.
const treeMovedSentinel = "tree-moved-during-run"

// movedDuringRun replaces a path's fingerprint in the manifest when the path's
// content at the run's END snapshot differs from its content at the START
// snapshot. No file content hashes to the marker, so verify-status.sh reports
// STALE for that path however the tree looks now -- including after an edit
// reverted AFTER the run ended, where the fingerprints alone would agree again
// while the stages read content nobody verified. It is the same fail-closed
// answer as treeMovedSentinel, charged to the paths that earned it instead of to
// the whole run.
//
// What two snapshots cannot see is an edit that begins and ends BETWEEN them,
// and that window has two shapes. A path clean at run start, written, and
// restored appears in neither snapshot. A path already dirty at run start,
// written, and restored to its start content appears in both with the same
// fingerprint, so it keeps a real one and answers FRESH. The whole-tree hash has
// the identical hole for the identical reason, and closing either needs a third
// observation of the tree rather than a different marker
// (docs/architecture/testing/verify-freshness-scope.md).
const movedDuringRun = "MOVED-DURING-RUN"

func writeVerifyStatus(root string, exitCode int, mode, skipped string, start treeSnapshot, now time.Time) error {
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		return err
	}
	// The record names the tree the stages READ, not the tree that happens to be
	// on disk now. The two agree only when nothing edited the checkout during
	// the run; where they differ the run certifies nothing about the paths that
	// moved, and everything it always did about the paths that held still.
	end := snapshotTree(root)
	hash := start.hash
	if end.hash != start.hash {
		hash = treeMovedSentinel
	}
	if err := writeVerifyManifest(root, start.manifest, end.manifest); err != nil {
		return err
	}
	sha := gitOutput(root, "rev-parse", "HEAD")
	if strings.TrimSpace(sha) == "" {
		sha = "unknown"
	}
	var content strings.Builder
	writef(&content, "exit=%d\n", exitCode)
	writef(&content, "timestamp=%s\n", now.UTC().Format(time.RFC3339))
	// mode distinguishes FRESH(ze-precommit-verify) from the weaker FRESH(ze-precommit-verify-changed);
	// skipped records ZE_SKIP_SUITES so a partial pass cannot read as a full one.
	writef(&content, "mode=%s\n", mode)
	writef(&content, "skipped=%s\n", skipped)
	writef(&content, "git_sha=%s\n", strings.TrimSpace(sha))
	writef(&content, "tree_hash=%s\n", hash)
	return os.WriteFile(filepath.Join(root, statusPath), []byte(content.String()), 0o600)
}

// treeSnapshot is the tree at one instant: one whole-tree hash, and one
// fingerprint per path that differs from HEAD.
//
// Both granularities are recorded because two different questions are asked of
// them. "Is the whole checkout the one that was verified" has a single answer,
// and a checkout several sessions share stops answering yes within seconds of a
// PASS. "Was MY file the one that was verified" is the question a commit asks,
// and it stays answerable while another session works.
type treeSnapshot struct {
	hash     string
	manifest map[string]string
}

func snapshotTree(root string) treeSnapshot {
	return treeSnapshot{hash: computeTreeHash(root), manifest: computeDirtyManifest(root)}
}

// writeVerifyManifest records the per-path fingerprint of the tree the stages
// read, and names the paths that moved underneath them.
//
// A path whose fingerprint differs between the two snapshots was read by some
// stages at one content and by the rest at another, so no stage judged the file
// the checkout now holds. It is recorded as movedDuringRun rather than with
// either fingerprint, which keeps that path STALE even after the edit is
// reverted. Every other path keeps the fingerprint the stages actually read --
// including a path that was edited and put back before the END snapshot, which
// the comparison cannot see either way (see movedDuringRun).
func writeVerifyManifest(root string, start, end map[string]string) error {
	paths := make([]string, 0, len(start)+len(end))
	for rel := range start {
		paths = append(paths, rel)
	}
	for rel := range end {
		if _, both := start[rel]; !both {
			paths = append(paths, rel)
		}
	}
	sort.Strings(paths)

	var content strings.Builder
	for _, rel := range paths {
		fingerprint := start[rel]
		if end[rel] != fingerprint {
			fingerprint = movedDuringRun
		}
		writef(&content, "%s %s\n", fingerprint, rel)
	}
	return os.WriteFile(filepath.Join(root, manifestPath), []byte(content.String()), 0o600)
}

// computeDirtyManifest fingerprints every path that differs from HEAD, keyed by
// the path git prints for it.
//
// It mirrors dirty_manifest in scripts/dev/verify-status.sh, which computes the
// LIVE side of the same comparison. A row here MUST be byte-identical to the row
// that function produces for a file nobody touched, or the scoped check reports
// STALE for a path that never changed.
//
// Only differing paths are recorded, a few hundred rather than every tracked
// file. A path in neither side is identical to HEAD in both, which is the same
// answer as two matching fingerprints.
func computeDirtyManifest(root string) map[string]string {
	tracked := nonEmptyLines(gitOutput(root, "diff", "HEAD", "--name-only"))
	untracked := nonEmptyLines(gitOutput(root, "ls-files", "-o", "--exclude-standard"))

	manifest := make(map[string]string, len(tracked)+len(untracked))
	for _, rel := range slices.Concat(tracked, untracked) {
		if _, seen := manifest[rel]; seen {
			continue
		}
		// A deleted tracked file, and a path git names that is not a regular
		// file, both read as MISSING -- the same word the shell prints for them.
		data, err := readControlledFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			manifest[rel] = "MISSING"
			continue
		}
		sum := sha256.Sum256(data)
		manifest[rel] = hex.EncodeToString(sum[:])
	}
	return manifest
}

func computeTreeHash(root string) string {
	h := sha256.New()
	io.WriteString(h, gitOutput(root, "rev-parse", "HEAD")) //nolint:errcheck // hash writer never fails
	io.WriteString(h, gitOutput(root, "diff", "HEAD"))      //nolint:errcheck // hash writer never fails
	untracked := nonEmptyLines(gitOutput(root, "ls-files", "-o", "--exclude-standard"))
	sort.Strings(untracked)
	for _, rel := range untracked {
		io.WriteString(h, rel+"\n") //nolint:errcheck // hash writer never fails
		data, err := readControlledFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			io.WriteString(h, "MISSING\n") //nolint:errcheck // hash writer never fails
			continue
		}
		fileHash := sha256.Sum256(data)
		io.WriteString(h, hex.EncodeToString(fileHash[:])+"\n") //nolint:errcheck // hash writer never fails
	}
	return hex.EncodeToString(h.Sum(nil))
}

func gitOutput(root string, args ...string) string {
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // fixed binary and caller-controlled arguments
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func nonEmptyLines(text string) []string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func readControlledFile(path string) ([]byte, error) {
	//nolint:gosec // path is generated by the verify runner or enumerated by git within the repo root
	return os.ReadFile(path)
}

func openControlledFile(path string) (*os.File, error) {
	//nolint:gosec // path is generated by the verify runner within the repo root
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
}

// contendedWarning warns when the verify run is CPU-contended. It uses the same
// hostload.Contended definition as the functional-test runner (single source of
// truth), so the two surfaces cannot disagree: process concurrency ALONE is not
// enough -- load must also exceed CPU count before the run is called contended.
func contendedWarning() string {
	load := hostload.Snapshot()
	if !load.Contended() {
		return ""
	}
	var tb textbuf.Buffer
	tb.Str("contended run detected: ")
	tb.Int(int64(load.ZeProcs))
	tb.Str(" ze-test, ")
	tb.Int(int64(load.GoTestProcs))
	tb.Str(" go-test processes; results may include environmental failures")
	return tb.String()
}
