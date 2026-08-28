// Design: docs/architecture/core-design.md -- the perf nudge, as one command
// Detail: report.go -- what the nudge answers
// Detail: actions.go -- the two things this area does
//
// Package perfbench reports BGP dataplane changes since the last performance run.
// The command is a nudge, not a gate, and always exits 0.
// It does not block a build.
//
// The performance suite needs Docker and several minutes, so developers do not run it after every edit.
// This nudge identifies dataplane changes that otherwise remain untested between performance runs.
//
// This local nudge complements CI.
// The scheduled Docker-free check validates the committed NDJSON history each night.
// The nudge requests a complete Docker performance run when local hot-path code changed.
//
// Detection uses the changed-package selector with the last performance run as its baseline.
// A .go file is uncovered if the working tree or a later commit changed it.
package perfbench

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area names the command namespace.
const area = "perf-bench"

// recordVerb writes the marker after a performance run.
const recordVerb = "record"

// HotPathPrefixes are the packages the Docker perf DUT actually measures (BGP
// convergence, throughput, p99) plus the harness itself.
//
// This list is a suggestion filter, not a gate: a wrong entry only over- or
// under-suggests, and it can never fail a build. But the failure that matters
// for an ADVISORY is UNDER-coverage, a silent nudge on a real regression, so
// the list is anchored to what the sole ze perf config exercises rather than to
// a guess.
//
// That config sets `rs-fast-path enable` on every peer, so the measured path
// runs through the route-server plugin's batch forwarder. Omitting plugins/rs/
// left the nudge silent on exactly the throughput regression the run measures.
// The engine RIB, store, fsm and adj_rib_in sit on the convergence path the
// same config drives.
//
// Add a prefix when a new dataplane package joins the measured path.
// The alloc-gate benchmarks enforce a narrower companion set.
var HotPathPrefixes = []string{
	"internal/component/bgp/reactor/",
	"internal/component/bgp/wireu/",
	"internal/component/bgp/message/",
	"internal/component/bgp/attrpool/",
	// The route-server fast path, which the perf config enables.
	"internal/component/bgp/plugins/rs/",
	"internal/component/bgp/plugins/rib/",
	"internal/component/bgp/plugins/adj_rib_in/",
	// The engine RIB: commit, incoming, update.
	"internal/component/bgp/rib/",
	// Per-attribute dedup interning.
	"internal/component/bgp/store/",
	// Session establishment and convergence.
	"internal/component/bgp/fsm/",
	// Wire, attribute, capability, nlri.
	"internal/core/bgp/",
	"internal/perf/",
}

// MarkerPath is where the last perf run's SHA is recorded, relative to the
// checkout. The three benchmark verbs in bench.go write it, so a real perf run
// clears the suggestion.
const MarkerPath = "tmp/.ze-perf-lastrun"

// unknownSHA marks a record operation when HEAD is unreadable.
// It distinguishes that event from a performance run that never occurred.
const unknownSHA = "unknown"

// Three of the four working-tree queries share these words.
// Naming them once avoids repeated literals and makes the query list read as a table.
const (
	diffQuery = "diff"
	nameOnly  = "--name-only"
)

// namedFiles is how many uncovered paths the nudge names before it counts the
// rest.
const namedFiles = 12

// shortSHA is how much of a commit the nudge prints.
const shortSHA = 12

// Runner is one checkout the nudge reads.
//
// The git command is a field so tests can simulate an absent git or a nonzero result.
// The tests do not need a checkout with those faults.
type Runner struct {
	Root string
	// Git runs one git command in Root and answers its stdout and whether it
	// succeeded. A command that failed answers false, and the caller decides
	// whether that is fatal to the answer or expected.
	Git func(args ...string) (string, bool)
}

// New answers a runner over one checkout, using the git on PATH.
func New(root string) *Runner {
	run := &Runner{Root: root}
	run.Git = run.gitHere
	return run
}

// gitHere runs one git command in the checkout.
//
// workingtree.Changed explains why this local query has no timeout.
// It has no network access or external lock.
// A deadline would misreport a slow filesystem as no changes.
func (r *Runner) gitHere(args ...string) (string, bool) {
	cmd := exec.Command("git", args...) //nolint:gosec,noctx // a build tool queries the checkout it was pointed at
	cmd.Dir = r.Root
	out, err := cmd.Output()
	return string(out), err == nil
}

// isHot reports whether a path is a perf-sensitive Go file.
func isHot(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	for _, prefix := range HotPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// Head returns the current commit, or an empty string when git does not return one.
func (r *Runner) Head() string {
	out, ok := r.Git("rev-parse", "HEAD")
	if !ok {
		return ""
	}
	return strings.TrimSpace(out)
}

// marker answers the absolute path of the recorded-run marker.
func (r *Runner) marker() string { return filepath.Join(r.Root, filepath.FromSlash(MarkerPath)) }

// Record writes the current HEAD as "perf ran here".
func (r *Runner) Record() (Report, int) {
	sha := r.Head()
	if sha == "" {
		sha = unknownSHA
	}
	path := r.marker()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return Report{Recorded: sha, Error: err.Error()}, 1
	}
	var tb textbuf.Buffer
	if err := os.WriteFile(path, []byte(tb.Str(sha).Byte('\n').String()), 0o600); err != nil {
		return Report{Recorded: sha, Error: err.Error()}, 1
	}
	return Report{Recorded: sha}, 0
}

// reachable reports whether a SHA names a commit this checkout holds.
func (r *Runner) reachable(sha string) bool {
	var tb textbuf.Buffer
	_, ok := r.Git("cat-file", "-e", tb.Str(sha).Str("^{commit}").String())
	return ok
}

// Origin records the source of a baseline.
// Both the nudge message and report readers need this fact directly.
type Origin string

const (
	// OriginLastRun is the marker: perf ran here, at that commit.
	OriginLastRun Origin = "last-perf-run"
	// OriginMergeBase marks branch work that has never had a performance test.
	// It prevents committed hot-path changes from disappearing before any performance run.
	OriginMergeBase Origin = "merge-base"
	// OriginWorkingTree is no trusted point at all, so committed work is not
	// measured. Quiet and safe on a detached HEAD or an untracked branch, and
	// never noisy on a fresh clone.
	OriginWorkingTree Origin = "working-tree"
)

// Baseline returns the commit for committed-change comparisons and its origin.
// It prefers the recorded run, then the upstream merge base, then no baseline.
func (r *Runner) Baseline() (string, Origin) {
	if raw, err := os.ReadFile(r.marker()); err == nil {
		sha := strings.TrimSpace(string(raw))
		if sha != "" && sha != unknownSHA && r.reachable(sha) {
			return sha, OriginLastRun
		}
	}
	upstream, ok := r.Git("rev-parse", "--abbrev-ref", "@{upstream}")
	if ok && strings.TrimSpace(upstream) != "" {
		base, found := r.Git("merge-base", "HEAD", strings.TrimSpace(upstream))
		sha := strings.TrimSpace(base)
		if found && sha != "" && r.reachable(sha) {
			return sha, OriginMergeBase
		}
	}
	return "", OriginWorkingTree
}

// changedHot answers the hot-path Go files changed in the working tree, or
// committed since base, sorted and without duplicates.
//
// Suggest also needs the resolved baseline for its message.
// Passing the baseline here avoids duplicate merge-base and cat-file queries.
//
// A failed git query returns an error instead of "nothing changed".
// The script used an empty string for both a failed command and an empty result.
// Missing git or an unreadable repository CAN therefore silence this advisory.
// That silence hides the regression that the nudge exists to expose.
func (r *Runner) changedHot(base string) ([]string, error) {
	queries := [][]string{
		{diffQuery, nameOnly},
		{diffQuery, "--cached", nameOnly},
		{"ls-files", "--others", "--exclude-standard"},
	}
	if base != "" {
		var tb textbuf.Buffer
		queries = append(queries, []string{diffQuery, nameOnly, tb.Str(base).Str("..HEAD").String()})
	}

	seen := make(map[string]bool)
	for _, query := range queries {
		out, ok := r.Git(query...)
		if !ok {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("git ").Join(query, " ").Str(" could not read ").Str(r.Root).String())
		}
		for line := range strings.SplitSeq(out, "\n") {
			if line != "" && isHot(line) {
				seen[line] = true
			}
		}
	}

	hot := make([]string, 0, len(seen))
	for path := range seen {
		hot = append(hot, path)
	}
	sort.Strings(hot)
	return hot, nil
}

// Suggest answers the nudge over this checkout. The code is ALWAYS 0: this is
// advisory, and a caller that started blocking on it would be a different tool.
func (r *Runner) Suggest() (Report, int) {
	base, origin := r.Baseline()
	hot, err := r.changedHot(base)
	if err != nil {
		return Report{Baseline: base, Origin: origin, Error: err.Error()}, 0
	}
	return Report{Baseline: base, Origin: origin, Uncovered: hot}, 0
}
