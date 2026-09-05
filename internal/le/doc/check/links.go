// Design: docs/architecture/core-design.md -- native documentation link verification
// Overview: actions.go -- the callable documentation action table.
// Detail: citation.go -- the shared path-reference grammar.
// Detail: names.go -- hook and checker name resolution.
// Detail: repository.go -- bounded Git populations and ignored paths.

package doccheck

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	docwiring "github.com/ze-software/ze/internal/le/doc/wiring"
	"github.com/ze-software/ze/internal/le/spec/specpath"
)

const baselineRel = "internal/le/doc/check/testdata/doc_citation_baseline.txt"
const baselineGrowthShown = 10

var markdownGlobs = [...]string{
	"ai/INSTRUCTIONS.md",
	"ai/INDEX.md",
	"ai/NAVIGATION.md",
	"ai/rules/*.md",
	"ai/rationale/*.md",
	"ai/patterns/*.md",
	"ai/skills/*.md",
	"ai/agents/*.md",
	".claude/rules/*.md",
	".claude/README.md",
	".claude/hooks/README.md",
	"plan/README.md",
	"plan/TEMPLATE.md",
	"plan/TEMPLATE-CLOSURE.md",
	"plan/learned/RECURRING-PATTERNS.md",
	"plan/learned/HOOK-FRICTION.md",
	"plan/learned/DESIGN-HISTORY.md",
}

// citationExcludePrefixes names the trees whose prose is not policed for a
// path that no longer resolves.
//
// The plan trees are RECORDS, and that is the whole reason (owner decision,
// 2026-08-29). A spec, a journal row, a deferral and a debt row each describe
// what was true when it was written, so a path inside one is a fact about that
// moment rather than a claim about the tree today. Repointing it at whatever
// replaced the file rewrites the record into something that was never true, and
// leaving it dangling is not a defect to be repaired later: it is the record
// working. A rename that moves a package therefore owes these trees nothing.
//
// The cost of the opposite policy was measured. One package rename left 383
// dangling references across plan/, and the gate reported them as breakage of
// the same kind as a live doc pointing at a deleted file. Chasing them touched
// hundreds of historical files, each edit racing another session that was
// writing its own rows, to make records say something they had not said.
//
// Live instruction files under plan/ stay in scope and are listed by name in
// markdownGlobs: plan/README.md, the two templates, and the learned indexes.
// Those are read for what is true NOW, so a dead path in one misleads.
//
// The rule for everything else is fix-on-touch: repair a stale path in a file
// you are already editing for another reason, and leave the rest alone.
var citationExcludePrefixes = citationExcludes()

// citationExcludes joins the fixed trees to one spec prefix for each release
// bucket. The buckets come from specpath, so a spec is excluded wherever it
// sits: the list spelled plan/spec- alone, and it policed the specs of two
// buckets as live prose the moment the buckets appeared.
func citationExcludes() []string {
	trees := [...]string{
		"vendor/",
		"third_party/",
		"plan/handover/",
		"plan/journal/",
		"plan/verification-debt/",
		"plan/known-failures/",
	}
	buckets := specpath.Dirs()
	prefixes := make([]string, 0, len(trees)+len(buckets))
	prefixes = append(prefixes, trees[:]...)
	for _, dir := range buckets {
		prefixes = append(prefixes, dir+"/spec-")
	}
	return prefixes
}

type citationPair struct {
	citer  string
	target string
}

type deadCitation struct {
	citer  string
	line   int
	target string
}

type brokenReference struct {
	where  string
	target string
}

// linkReport is the ordered answer of the complete links gate.
type linkReport struct {
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
	Summary  string   `json:"summary"`
}

// Text preserves the producer's warning, fatal, and summary ordering.
func (r linkReport) Text() string {
	var out textbuf.Buffer
	for _, warning := range r.Warnings {
		out.Str(warning).Byte('\n')
	}
	for _, finding := range r.Errors {
		out.Str(finding).Byte('\n')
	}
	if len(r.Errors) > 0 {
		out.Int(int64(len(r.Errors))).
			Str(" broken reference(s) -- fix the reference, or mark the line ").
			Str("`<!-- doc-links: ignore (reason) -->`\n")
		return out.String()
	}
	out.Str(r.Summary).Byte('\n')
	return out.String()
}

// checkLinks runs all five document-link checks over root.
func checkLinks(root string) (linkReport, error) {
	broken, err := checkMarkdown(root, false)
	if err != nil {
		return linkReport{}, err
	}
	ignoredFindings, err := dropGenerated(root, broken)
	if err != nil {
		return linkReport{}, err
	}

	designFindings, err := docwiring.DesignReferences(root)
	if err != nil {
		return linkReport{}, fmt.Errorf("checking Design references: %w", err)
	}
	errorsFound := make([]string, 0, len(ignoredFindings)+len(designFindings))
	errorsFound = append(errorsFound, ignoredFindings...)
	errorsFound = append(errorsFound, designFindings...)

	nameFindings, err := checkHookNames(root, false)
	if err != nil {
		return linkReport{}, fmt.Errorf("checking hook names: %w", err)
	}
	errorsFound = append(errorsFound, nameFindings...)

	sweep, err := sweepTracked(root, false)
	if err != nil {
		return linkReport{}, err
	}
	baseline, err := loadBaseline(root)
	if err != nil {
		return linkReport{}, err
	}
	citationFindings, warnings, err := baselineFindings(root, sweep.dead, baseline, false, true)
	if err != nil {
		return linkReport{}, err
	}
	errorsFound = append(errorsFound, sweep.unreadable...)
	errorsFound = append(errorsFound, sweep.markers...)
	errorsFound = append(errorsFound, citationFindings...)

	formatFindings, err := baselineFormatProblems(root)
	if err != nil {
		return linkReport{}, err
	}
	errorsFound = append(errorsFound, formatFindings...)
	growthFindings, err := checkBaselineGrowth(root, baseline)
	if err != nil {
		return linkReport{}, err
	}
	errorsFound = append(errorsFound, growthFindings...)

	summary := "every path reference resolves"
	if len(baseline) > 0 {
		var tb textbuf.Buffer
		tb.Str(summary).Str(" (").Int(int64(len(baseline))).Str(" citation pair(s) baselined")
		if len(warnings) > 0 {
			tb.Str(", ").Int(int64(len(warnings))).Str(" stale baseline entry(s)")
		}
		summary = tb.Byte(')').String()
	}
	return linkReport{Warnings: warnings, Errors: errorsFound, Summary: summary}, nil
}

func markdownCorpus(root string) ([]string, error) {
	var files []string
	for _, pattern := range markdownGlobs {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			return nil, fmt.Errorf("expanding markdown corpus %s: %w", pattern, err)
		}
		sort.Strings(matches)
		for _, match := range matches {
			rel, err := filepath.Rel(root, match)
			if err != nil {
				return nil, fmt.Errorf("relativizing markdown corpus path %s: %w", match, err)
			}
			rel = filepath.ToSlash(rel)
			if rel != "ai/CODE-TO-DOCS.md" {
				files = append(files, rel)
			}
		}
	}
	return files, nil
}

func checkMarkdown(root string, _ bool) (broken []brokenReference, err error) {
	files, err := markdownCorpus(root)
	if err != nil {
		return nil, err
	}
	repository, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("opening repository root: %w", err)
	}
	defer func() {
		err = errors.Join(err, repository.Close())
	}()
	var tb textbuf.Buffer

	for _, rel := range files {
		body, err := repository.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			return nil, fmt.Errorf("reading markdown corpus %s: %w", rel, err)
		}
		for index, line := range strings.Split(string(body), "\n") {
			if suppressed(line) {
				continue
			}
			for _, target := range lineCitations(root, line) {
				resolves, err := pathResolves(root, target)
				if err != nil {
					return nil, fmt.Errorf("resolving %s from %s: %w", target, rel, err)
				}
				if !resolves {
					broken = append(broken, brokenReference{
						where:  tb.Reset().Str(rel).Byte(':').Int(int64(index + 1)).Str(": broken path reference").String(),
						target: target,
					})
				}
			}
		}
	}
	return broken, nil
}

func dropGenerated(root string, broken []brokenReference) ([]string, error) {
	targets := make([]string, 0, len(broken))
	for _, finding := range broken {
		targets = append(targets, finding.target)
	}
	ignored, err := checkIgnored(root, targets)
	if err != nil {
		return nil, err
	}
	findings := make([]string, 0, len(broken))
	var tb textbuf.Buffer
	for _, finding := range broken {
		if targetIgnored(ignored, finding.target) {
			continue
		}
		findings = append(findings, tb.Reset().Str(finding.where).Str(": ").Str(finding.target).String())
	}
	return findings, nil
}
func targetIgnored(ignored map[string]bool, target string) bool {
	if ignored[target] {
		return true
	}
	return ignored[strings.TrimSuffix(target, "/")]
}

type trackedSweep struct {
	unreadable []string
	markers    []string
	dead       []deadCitation
}

func sweepTracked(root string, _ bool) (result trackedSweep, err error) {
	files, err := trackedFiles(root)
	if err != nil {
		return trackedSweep{}, err
	}
	corpusFiles, err := markdownCorpus(root)
	if err != nil {
		return trackedSweep{}, err
	}
	corpus := make(map[string]bool, len(corpusFiles))
	for _, rel := range corpusFiles {
		corpus[rel] = true
	}
	repository, err := os.OpenRoot(root)
	if err != nil {
		return trackedSweep{}, fmt.Errorf("opening repository root: %w", err)
	}
	defer func() {
		err = errors.Join(err, repository.Close())
	}()

	var tb textbuf.Buffer
	for _, rel := range files {
		if sweepExcluded(rel) {
			continue
		}
		raw, readErr := repository.ReadFile(filepath.FromSlash(rel))
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			result.unreadable = append(result.unreadable, tb.Reset().
				Str(rel).Str(": cannot read for the tracked-file sweep: ").Err(readErr).
				Str(" -- every marker and every path reference it carries would go unchecked").String())
			continue
		}
		readMarkers := strings.Contains(string(raw), "doc-links: ignore")
		readCitations := !corpus[rel]
		if hasPrefix(rel, citationExcludePrefixes) {
			readCitations = false
		}
		if !readMarkers {
			if !readCitations {
				continue
			}
		}
		text := strings.ToValidUTF8(string(raw), "�")
		for index, line := range strings.Split(text, "\n") {
			lineNo := index + 1
			if readMarkers {
				for _, tail := range ignoreMarkers(line) {
					if markerReason(tail) == "" {
						result.markers = append(result.markers, tb.Reset().
							Str(rel).Byte(':').Int(int64(lineNo)).
							Str(": doc-links: ignore marker states no reason -- write it as ").
							Str("`<!-- doc-links: ignore (why this path cannot resolve) -->`, ").
							Str("or delete the marker and fix the reference it hides").String())
					}
				}
			}
			if !readCitations {
				continue
			}
			if suppressed(line) {
				continue
			}
			for _, target := range lineCitations(root, line) {
				resolves, err := pathResolves(root, target)
				if err != nil {
					return trackedSweep{}, fmt.Errorf("resolving %s from %s: %w", target, rel, err)
				}
				if !resolves {
					result.dead = append(result.dead, deadCitation{citer: rel, line: lineNo, target: target})
				}
			}
		}
	}
	return result, nil
}

func sweepExcluded(rel string) bool {
	if rel == "internal/le/doc/check/links.go" {
		return true
	}
	if rel == "internal/le/doc/check/links_test.go" {
		return true
	}
	return false
}

func loadBaseline(root string) (map[citationPair]bool, error) {
	body, err := readRepositoryFile(root, baselineRel)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[citationPair]bool), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading citation baseline: %w", err)
	}
	return parseBaseline(string(body)), nil
}

func parseBaseline(text string) map[citationPair]bool {
	pairs := make(map[citationPair]bool)
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		citer, target, _ := strings.Cut(line, "\t")
		pairs[citationPair{citer: strings.TrimSpace(citer), target: strings.TrimSpace(target)}] = true
	}
	return pairs
}

func baselineFormatProblems(root string) ([]string, error) {
	body, err := readRepositoryFile(root, baselineRel)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading citation baseline format: %w", err)
	}
	var findings []string
	var tb textbuf.Buffer
	for index, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "\t") {
			continue
		}
		findings = append(findings, tb.Reset().Str(baselineRel).Byte(':').
			Int(int64(index+1)).Str(": baseline line has no TAB: ").Str(trimmed).
			Str(" -- write it as `<citing file><TAB><target path>`, or delete it").String())
	}
	return findings, nil
}

func baselineFindings(root string, dead []deadCitation, baseline map[citationPair]bool,
	_ bool, drift bool,
) ([]string, []string, error) {
	uniqueTargets := make(map[string]bool)
	for _, citation := range dead {
		uniqueTargets[citation.target] = true
	}
	targets := make([]string, 0, len(uniqueTargets))
	for target := range uniqueTargets {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	ignored, err := checkIgnored(root, targets)
	if err != nil {
		return nil, nil, err
	}
	live := make([]deadCitation, 0, len(dead))
	var findings []string
	var tb textbuf.Buffer
	for _, citation := range dead {
		if targetIgnored(ignored, citation.target) {
			continue
		}
		live = append(live, citation)
		if baseline[citationPair{citer: citation.citer, target: citation.target}] {
			continue
		}
		findings = append(findings, tb.Reset().Str(citation.citer).Byte(':').
			Int(int64(citation.line)).Str(": dead path reference: ").Str(citation.target).
			Str(" -- repair the reference, or mark the line ").
			Str("`<!-- doc-links: ignore (why this path cannot resolve) -->`").String())
	}
	if !drift {
		return findings, nil, nil
	}
	seen := make(map[citationPair]bool, len(live))
	for _, citation := range live {
		seen[citationPair{citer: citation.citer, target: citation.target}] = true
	}
	stale := make([]citationPair, 0)
	for pair := range baseline {
		if !seen[pair] {
			stale = append(stale, pair)
		}
	}
	sort.Slice(stale, func(i, j int) bool {
		if stale[i].citer != stale[j].citer {
			return stale[i].citer < stale[j].citer
		}
		return stale[i].target < stale[j].target
	})
	warnings := make([]string, 0, len(stale))
	for _, pair := range stale {
		warnings = append(warnings, tb.Reset().Str("WARN ").Str(baselineRel).Str(": ").
			Str(pair.citer).Str(" no longer cites ").Str(pair.target).
			Str(" -- delete the entry, so the baseline shrinks").String())
	}
	return findings, warnings, nil
}

func checkBaselineGrowth(root string, current map[citationPair]bool) ([]string, error) {
	head, exists, err := headBaseline(root)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	added := make([]citationPair, 0)
	for pair := range current {
		if !head[pair] {
			added = append(added, pair)
		}
	}
	if len(added) == 0 {
		return nil, nil
	}
	sort.Slice(added, func(i, j int) bool {
		if added[i].citer != added[j].citer {
			return added[i].citer < added[j].citer
		}
		return added[i].target < added[j].target
	})
	shown := min(len(added), baselineGrowthShown)
	var tb textbuf.Buffer
	tb.Str(baselineRel).Str(": ").Int(int64(len(added))).
		Str(" baseline pair(s) are new against HEAD -- the baseline is shrink-only:\n")
	for _, pair := range added[:shown] {
		tb.Str("  ").Str(pair.citer).Str(" -> ").Str(pair.target).Byte('\n')
	}
	if len(added) > shown {
		tb.Str("  ... and ").Int(int64(len(added) - shown)).Str(" more\n")
	}
	tb.Str("Repair each citation, or mark its line ").
		Str("`<!-- doc-links: ignore (why this path cannot resolve) -->`. ").
		Str("Never regenerate the baseline to absorb it")
	return []string{tb.String()}, nil
}

func headBaseline(root string) (map[citationPair]bool, bool, error) {
	var tb textbuf.Buffer
	_, err := gitOutput(root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("checking baseline HEAD: %w", err)
	}
	_, err = gitOutput(root, "cat-file", "-e",
		tb.Str("HEAD:").Str(baselineRel).Slice())
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("checking baseline at HEAD: %w", err)
	}
	body, err := gitOutput(root, "show", tb.Reset().Str("HEAD:").Str(baselineRel).Slice())
	if err != nil {
		return nil, false, fmt.Errorf("reading baseline at HEAD: %w", err)
	}
	return parseBaseline(string(body)), true, nil
}
