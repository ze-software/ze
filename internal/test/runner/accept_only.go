// Design: docs/architecture/testing/ci-format.md — accept-only (weak) .ci predicate and annotation marker
// Related: record_parse.go — the .ci parser this reuses so "weak" has one definition

package runner

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/tmpfs"
)

// acceptOnlyBaselinePath is the repo-relative path of the accept-only ratchet
// baseline: one repo-relative .ci path per line, sorted, listing every current
// accept-only (weak) test that is NOT annotated. The lint grandfathers these so
// the class cannot GROW silently; strengthening or annotating a file removes it
// from the baseline (the ratchet only shrinks). Regenerate with
// writeAcceptOnlyBaseline (via TestRegenerateAcceptOnlyBaseline).
const acceptOnlyBaselinePath = "test/.accept-only-baseline"

// acceptOnlyAnnotation matches the documented accept-only marker: a comment line
// whose content is `accept-only:` followed by a non-empty reason, e.g.
//
//	# accept-only: unit-covered by TestParseNTPConfig
//
// A file carrying it is deliberately weak (a unit test covers the value) and is
// allowed by the lint without a baseline entry. Kept greppable (`accept-only:`).
var acceptOnlyAnnotation = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*accept-only:[ \t]*\S`)

// setEnableErrExit matches a shell line that enables `errexit` (`set -e` in any
// flag cluster, or `set -o errexit`). A tmpfs script with errexit does its own
// checking, so such a test is NOT accept-only even when its only .ci assertion is
// expect=exit:code=0 (the check lives in the script, e.g. test/managed/
// auth-reject.ci). Scanned over the raw tmpfs file bytes.
var setEnableErrExit = regexp.MustCompile(`(?m)^[ \t]*set[ \t]+(-[a-zA-Z]*e[a-zA-Z]*|-o[ \t]+errexit)\b`)

// isAcceptOnly reports whether a parsed .ci Record is "accept-only" (weak): its
// ONLY assertion is that a command exited 0, so it proves a config/command was
// ACCEPTED but never observes a produced VALUE. Such a test passes even when the
// parser stores the wrong value or drops a block — the functional-suite analog of
// the "count-only assertion" mistake class.
//
// A Record is accept-only when ALL hold:
//   - acceptance is proven by AT LEAST ONE zero exit assertion — either the
//     file-level expect=exit:code=0 OR a per-command cmd=...:exit=0. record.go
//     steers multi-command tests to the per-command form, so a per-command
//     exit=0-only test is exactly as weak and must be classified the same way;
//   - NO exit assertion is non-zero — a non-zero exit (file-level or per-command)
//     is a real negative assertion (it proves a rejection), so it makes the test
//     NOT weak;
//   - no value-observing assertion is populated (BGP/JSON messages, ze-peer
//     expects/actions, stdout/stderr/syslog contains/pattern, await fence, file
//     checks, HTTP checks, or engine steps);
//   - no reject= is present (RejectStderr/RejectSyslog/RejectStdoutRegex, and the
//     reject=stdout:contains= form that populates ExpectStdoutNotMatch);
//   - no embedded tmpfs script enables `set -e` (it would do its own checking).
//
// This is the single definition of "weak"; the lint and the baseline generator
// both call it, so "strengthened" and "flagged" can never disagree.
func isAcceptOnly(r *Record) bool {
	if r == nil {
		return false
	}
	// Acceptance is proven by at least one zero exit assertion (file-level or
	// per-command); any non-zero exit assertion is a real rejection -> not weak.
	hasZeroExit := false
	if r.ExpectExitCode != nil {
		if *r.ExpectExitCode != 0 {
			return false
		}
		hasZeroExit = true
	}
	for i := range r.RunCommands {
		ec := r.RunCommands[i].ExitCode
		if ec == nil {
			continue
		}
		if *ec != 0 {
			return false
		}
		hasZeroExit = true
	}
	if !hasZeroExit {
		// No exit assertion at all: there is no acceptance signal to weaken, so
		// this is not the accept-only class (it asserts even less, a distinct
		// problem this predicate does not target).
		return false
	}
	// Any value-observing assertion disqualifies (the test observes a value).
	if len(r.Messages) != 0 ||
		len(r.Expects) != 0 ||
		len(r.ExpectStderr) != 0 ||
		len(r.ExpectStderrMatch) != 0 ||
		len(r.ExpectSyslog) != 0 ||
		len(r.ExpectStdoutMatch) != 0 ||
		len(r.ExpectStdoutNotMatch) != 0 ||
		len(r.ExpectStdoutRegex) != 0 ||
		r.AwaitStderr != "" ||
		len(r.FileChecks) != 0 ||
		len(r.HTTPChecks) != 0 ||
		len(r.HTTPWaits) != 0 ||
		len(r.EngineSteps) != 0 {
		return false
	}
	// Any reject= disqualifies (a negative expectation observes output).
	if len(r.RejectStderr) != 0 || len(r.RejectSyslog) != 0 || len(r.RejectStdoutRegex) != 0 {
		return false
	}
	// An embedded errexit script does its own checking.
	if tmpfsHasErrExit(r.TmpfsFiles) {
		return false
	}
	return true
}

// tmpfsHasErrExit reports whether any embedded tmpfs file enables shell errexit.
func tmpfsHasErrExit(files map[string]tmpfs.File) bool {
	for _, file := range files {
		if setEnableErrExit.Match(file.Content) {
			return true
		}
	}
	return false
}

// hasAcceptOnlyAnnotation reports whether raw .ci file bytes carry the accept-only
// annotation marker. It reads the RAW file text (not a parsed Record) because the
// .ci parser strips comment lines, so the marker survives only in the source.
func hasAcceptOnlyAnnotation(raw []byte) bool {
	return acceptOnlyAnnotation.Match(raw)
}

// loadAcceptOnlyBaseline reads a baseline file into a set of repo-relative .ci
// paths (forward-slash). Blank lines and `#` comments are skipped. A missing file
// yields an empty set (nil error): the lint then reports every current accept-only
// test as a new violation, which fails loudly rather than silently passing.
func loadAcceptOnlyBaseline(path string) (map[string]struct{}, error) {
	set := make(map[string]struct{})
	f, err := os.Open(path) //nolint:gosec // caller-controlled test path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return set, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[filepath.ToSlash(line)] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return set, nil
}

// acceptOnlyParseFailure records a .ci file the generic runner parser could not
// read, with its repo-relative (forward-slash) path so the ratchet can decide
// whether the failure is an expected dialect exclusion or an unexpected one.
type acceptOnlyParseFailure struct {
	path string // repo-relative, forward-slash
	err  error
}

// acceptOnlyExcludedParsePrefixes lists directory prefixes (repo-relative,
// forward-slash) whose .ci files use a DIALECT the generic runner parser cannot
// read and which are value-asserting BY CONSTRUCTION, so the accept-only predicate
// never applies. A parse error UNDER one of these is expected and skipped; a parse
// error anywhere else fails the ratchet (fail-closed), so a future exit-only test
// in an unlisted tree cannot slip through unclassified.
var acceptOnlyExcludedParsePrefixes = []struct{ prefix, reason string }{
	{"test/decode/", "decode dialect: expect=json:json={...} without conn=/seq=; hex-to-JSON value tests, never accept-only"},
	{"test/exabgp-compat/encoding/", "legacy exabgp-compat dialect (option=file:<name>, N:raw:/N:json:) the generic parser cannot read; wire-value tests, never accept-only"},
}

// acceptOnlyExcludedParseFiles lists individual .ci files the generic parser
// rejects at discovery for a KNOWN, documented reason (not a new defect). Each key
// is a repo-relative (forward-slash) path.
var acceptOnlyExcludedParseFiles = map[string]string{
	// Deliberately-red tracked test (spec-fixit-redistribute-establishment-stall):
	// its peer1/peer2 blocks declare only expect=json, which ze-peer does not
	// consume, so validatePeerBlocks rejects it at discovery on purpose (it fails
	// its own `ze-test bgp plugin` suite too). It carries expect=json + reject=
	// stderr, so it is value-asserting and can never be accept-only regardless.
	"test/plugin/forward-mpreach-nexthop-self-two-peer.ci": "deliberately-red tracked test; value-asserting (expect=json + reject=stderr), never accept-only",
}

// acceptOnlyParseExcluded reports whether a parse failure at rel (repo-relative,
// forward-slash) is an expected dialect/known-file exclusion rather than an
// unexpected error that must fail the ratchet.
func acceptOnlyParseExcluded(rel string) bool {
	if _, ok := acceptOnlyExcludedParseFiles[rel]; ok {
		return true
	}
	for _, e := range acceptOnlyExcludedParsePrefixes {
		if strings.HasPrefix(rel, e.prefix) {
			return true
		}
	}
	return false
}

// acceptOnlyUnannotated walks repoRoot/test for *.ci files and returns the sorted
// repo-relative (forward-slash) paths of every test that isAcceptOnly and lacks
// the annotation marker. This is the canonical set the baseline must equal: the
// generator writes it, and the lint checks that nothing new escaped it and that no
// stale entry remains. parseFailures collects files that could not be parsed
// (repo-relative path + error), which the caller partitions into expected dialect
// exclusions vs unexpected fail-closed errors; walkErr is a hard filesystem error.
func acceptOnlyUnannotated(repoRoot string) (paths []string, parseFailures []acceptOnlyParseFailure, walkErr error) {
	testDir := filepath.Join(repoRoot, "test")
	walkErr = filepath.WalkDir(testDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// test/draft/ holds tests under development and is invisible to every
		// gate, this one included (test/draft/README.md). Pruned at the directory
		// so no draft file is read at all.
		if d.IsDir() && isDraftPath(testDir, p) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".ci") {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, p)
		if relErr != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)

		raw, readErr := os.ReadFile(p) //nolint:gosec // repo test tree
		if readErr != nil {
			// Record and continue: a hard return would abort discovery of every
			// other .ci in the tree. The failure is surfaced via parseFailures.
			parseFailures = append(parseFailures, acceptOnlyParseFailure{path: rel, err: readErr})
			return nil //nolint:nilerr // failure recorded in parseFailures; walk must continue
		}
		et := NewEncodingTests(filepath.Dir(p))
		rec, perr := et.parseAndAdd(p)
		if perr != nil || rec == nil {
			if perr == nil {
				perr = errors.New("parseAndAdd returned nil record")
			}
			parseFailures = append(parseFailures, acceptOnlyParseFailure{path: rel, err: perr})
			return nil //nolint:nilerr // failure recorded in parseFailures; walk must continue
		}
		if !isAcceptOnly(rec) || hasAcceptOnlyAnnotation(raw) {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	sort.Strings(paths)
	return paths, parseFailures, walkErr
}

// writeAcceptOnlyBaseline regenerates the baseline file from the current tree. It
// is the reproducible one-shot referenced in the baseline header comment. The
// output is deterministic (sorted) and only shrinks as tests are strengthened or
// annotated.
func writeAcceptOnlyBaseline(repoRoot string) error {
	paths, _, err := acceptOnlyUnannotated(repoRoot)
	if err != nil {
		return err
	}
	var b textbuf.Buffer
	b.Str("# accept-only ratchet baseline -- grandfathered weak .ci tests.\n")
	b.Str("#\n")
	b.Str("# One repo-relative path per line: each test whose ONLY assertion is\n")
	b.Str("# expect=exit:code=0 (proves ACCEPTED, not that the config parsed to the\n")
	b.Str("# CORRECT tree). The lint (internal/test/runner/accept_only_lint_test.go,\n")
	b.Str("# run under `./le verify deps unit-cached`) FAILS on a NEW unannotated accept-only .ci\n")
	b.Str("# not listed here, and on a STALE entry (one that was strengthened,\n")
	b.Str("# annotated, or removed). Strengthen a test with a readback assertion, or\n")
	b.Str("# annotate it `# accept-only: <reason>` when a unit test covers the value;\n")
	b.Str("# either way REMOVE its line here (the ratchet only shrinks). Never add a\n")
	b.Str("# new line to silence the lint.\n")
	b.Str("#\n")
	b.Str("# Regenerate: ZE_WRITE_ACCEPT_ONLY_BASELINE=1 go test -run TestRegenerateAcceptOnlyBaseline \\\n")
	b.Str("#   github.com/ze-software/ze/internal/test/runner\n")
	b.Str("#\n")
	b.Str("# merge=union in .gitattributes keeps both sides of concurrent edits.\n")
	for _, p := range paths {
		b.Str(p).Byte('\n')
	}
	target := filepath.Join(repoRoot, filepath.FromSlash(acceptOnlyBaselinePath))
	return os.WriteFile(target, []byte(b.String()), 0o644) //nolint:gosec // baseline is a checked-in text ledger
}

// diffStringSets returns members of a not in b, sorted.
func diffStringSets(a []string, b map[string]struct{}) []string {
	var out []string
	for _, s := range a {
		if _, ok := b[s]; !ok {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// baselineOrphans returns baseline entries that are no longer accept-only-and-
// unannotated (strengthened, annotated, or deleted), sorted. These are stale and
// must be removed so the ratchet keeps shrinking.
func baselineOrphans(baseline map[string]struct{}, current []string) []string {
	live := make(map[string]struct{}, len(current))
	for _, s := range current {
		live[s] = struct{}{}
	}
	var out []string
	for s := range baseline {
		if _, ok := live[s]; !ok {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// acceptOnlyLintResult holds the outcome of the accept-only ratchet check.
type acceptOnlyLintResult struct {
	// newViolations lists accept-only, unannotated .ci files not in the baseline:
	// each must be strengthened with a readback assertion or annotated, never added
	// to the baseline.
	newViolations []string
	// staleBaseline lists baseline entries that are no longer accept-only-and-
	// unannotated (strengthened, annotated, or deleted) and must be removed so the
	// ratchet keeps shrinking.
	staleBaseline []string
	// unexpectedParseErrors lists ".ci: error" for files that could not be parsed
	// and are NOT in the excluded-dialect allowlist. The ratchet FAILS on these
	// (fail-closed): an unparseable file is unclassifiable, so it could silently
	// hide a new accept-only test. Sorted.
	unexpectedParseErrors []string
	// excludedParseCount is the number of parse failures skipped because they fall
	// under a documented dialect exclusion (acceptOnlyParseExcluded). Reported, not
	// failed.
	excludedParseCount int
}

// checkAcceptOnlyRatchet walks repoRoot/test, loads the baseline at
// repoRoot/acceptOnlyBaselinePath, and reports new violations, stale baseline
// entries, and unexpected parse failures. It is the single gate the lint test
// asserts is empty.
func checkAcceptOnlyRatchet(repoRoot string) (acceptOnlyLintResult, error) {
	current, parseFailures, err := acceptOnlyUnannotated(repoRoot)
	if err != nil {
		return acceptOnlyLintResult{}, err
	}
	baseline, err := loadAcceptOnlyBaseline(filepath.Join(repoRoot, filepath.FromSlash(acceptOnlyBaselinePath)))
	if err != nil {
		return acceptOnlyLintResult{}, err
	}

	var unexpected []string
	excluded := 0
	for _, pf := range parseFailures {
		if acceptOnlyParseExcluded(pf.path) {
			excluded++
			continue
		}
		var b textbuf.Buffer
		unexpected = append(unexpected, b.Str(pf.path).Str(": ").Err(pf.err).String())
	}
	sort.Strings(unexpected)

	return acceptOnlyLintResult{
		newViolations:         diffStringSets(current, baseline),
		staleBaseline:         baselineOrphans(baseline, current),
		unexpectedParseErrors: unexpected,
		excludedParseCount:    excluded,
	}, nil
}
