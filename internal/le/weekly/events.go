// Design: docs/architecture/core-design.md -- the weekly report of tooling events
// Related: answer.go -- the report action that runs these sources
// Related: report.go -- the publication's answer, which renders the same way
//
// events.go is `le weekly report from <rev> to <rev>`: the week's tooling
// events, each section answered by a source the package that owns the records
// registered. The proven RFC figure lives in internal/le/rfc and the approval
// trailers in internal/le/commit; both register here from their own init(), so
// this package imports neither and its tests link neither, as
// roadmap.RegisterCorpusReader already does for the roadmap.
//
// The run resolves both revisions to full commits, lists the first-parent
// commits of the range once, materializes both endpoints, and hands every
// source the same EventRange. A source that fails answers its section as
// failed with the reason, never as an empty section: an empty section reads as
// a week with no events.

package weekly

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/repo/export"
	repoobjects "github.com/ze-software/ze/internal/le/repo/objects"
)

// EventRange is what every source reads: one release range, resolved and
// materialized once for all of them.
type EventRange struct {
	// Repo is the checkout whose object store holds both commits, for a
	// source that reads commit messages or blobs rather than a tree.
	Repo string
	// FromCommit and ToCommit are the full commit IDs the two revisions
	// resolved to.
	FromCommit string
	ToCommit   string
	// FromDir and ToDir hold each endpoint's tree, exported from git, so a
	// producer that takes a directory root judges what that commit held. A
	// source MUST NOT write into them: every source reads the same two trees.
	FromDir string
	ToDir   string
	// Commits are the first-parent commits after FromCommit up to and
	// including ToCommit, oldest first, as full commit IDs.
	Commits []string
}

// Fact is one line of a section: a figure, with the identifiers behind it.
type Fact struct {
	// Name says what is counted, in a few words.
	Name string `json:"name"`
	// Value is the figure: a count, or proven over total.
	Value string `json:"value"`
	// Items are the identifiers the figure counts (requirement ids, scenario
	// names, commit IDs), in the order the source chose.
	Items []string `json:"items,omitempty"`
}

// SectionBody is what a source answers.
type SectionBody struct {
	// Note states what the section counts and what it leaves out, where a
	// reader could otherwise assume more (approvals that landed, not every
	// verdict a judge gave).
	Note string
	// Facts MUST hold at least one entry: a source with no events states a
	// zero count, so an answered section never reads as a silent one.
	Facts []Fact
}

// EventSource answers one named section of the report.
type EventSource struct {
	// Section is the section's name, kebab-case, unique across sources. The
	// report orders its sections by this name, never by registration order,
	// which follows import order and so changes with an unrelated import.
	Section string
	// Answer reads the range and answers the section, or the reason it could
	// not. ctx bounds every child process the source starts.
	Answer func(ctx context.Context, events EventRange) (SectionBody, error)
}

// eventSources are the registered sources, in registration order; reports read
// them through registeredSources, which orders them.
var eventSources []EventSource

// RegisterEventSource adds one section to `le weekly report`. It MUST be
// called from an init() in the owner's register.go, once per section.
func RegisterEventSource(source EventSource) {
	if source.Section == "" {
		panic("BUG: weekly.RegisterEventSource: a source with no section name")
	}
	if source.Answer == nil {
		panic("BUG: weekly.RegisterEventSource: a source with no Answer")
	}
	for _, registered := range eventSources {
		if registered.Section == source.Section {
			panic("BUG: weekly.RegisterEventSource: two sources answer section " + source.Section)
		}
	}
	eventSources = append(eventSources, source)
}

// registeredSources answers the registered sources ordered by section name.
func registeredSources() []EventSource {
	sources := slices.Clone(eventSources)
	slices.SortFunc(sources, func(a, b EventSource) int { return cmp.Compare(a.Section, b.Section) })
	return sources
}

// SectionStatus says whether a source answered its section.
type SectionStatus uint8

// The section states. Zero is no state, so a section built without one cannot
// pass for an answered section.
const (
	SectionUnspecified SectionStatus = iota
	SectionAnswered
	SectionFailed
)

// MarshalText spells the state for every renderer.
func (s SectionStatus) MarshalText() ([]byte, error) {
	switch s {
	case SectionAnswered:
		return []byte("answered"), nil
	case SectionFailed:
		return []byte("failed"), nil
	case SectionUnspecified:
		return nil, errors.New("weekly report: a section with no status")
	}
	return nil, errors.New("weekly report: a section status outside the declared set")
}

// Section is one row of the report.
type Section struct {
	Name   string        `json:"name"`
	Status SectionStatus `json:"status"`
	// Reason says why a failed section has no facts, and is absent on an
	// answered one.
	Reason string `json:"reason,omitempty"`
	Note   string `json:"note,omitempty"`
	Facts  []Fact `json:"facts"`
}

// Endpoint is one end of the range as the operator named it and as git
// resolved it.
type Endpoint struct {
	Revision string `json:"revision"`
	Commit   string `json:"commit"`
}

// EventReport is the whole answer of `le weekly report`.
//
// Sections is its only row set, so `| match`, `| count` and `| table` act on
// the sections.
type EventReport struct {
	From Endpoint `json:"from"`
	To   Endpoint `json:"to"`
	// Commits counts the first-parent commits of the range.
	Commits int `json:"commits"`
	// RunDir is the scratch directory both endpoints were exported into. It
	// is removed before the answer renders, and named so a reader can check.
	RunDir   string    `json:"run-dir"`
	Sections []Section `json:"sections"`
}

// Complete reports whether every section answered. An incomplete report
// exits nonzero, so a caller cannot take a failed section for a quiet week.
func (r EventReport) Complete() bool {
	for _, section := range r.Sections {
		if section.Status != SectionAnswered {
			return false
		}
	}
	return true
}

// Text renders the report for a person, and ends in a newline. Every pipe
// operator bypasses it and renders the data.
func (r EventReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("weekly report ").Str(r.From.Commit).Str("..").Str(r.To.Commit).
		Str(" (").Int(int64(r.Commits)).Str(" first-parent commits)\n")
	tb.Str("endpoints exported into ").Str(r.RunDir).Str(", removed\n")
	if len(r.Sections) == 0 {
		tb.Str("no event source is linked into this binary\n")
	}
	for _, section := range r.Sections {
		tb.Byte('\n').Str("== ").Str(section.Name)
		if section.Status != SectionAnswered {
			tb.Str(" == FAILED: ").Str(section.Reason).Byte('\n')
			continue
		}
		tb.Str(" ==\n")
		if section.Note != "" {
			tb.Str(section.Note).Byte('\n')
		}
		for _, fact := range section.Facts {
			tb.Str(fact.Name).Str(": ").Str(fact.Value).Byte('\n')
			for _, item := range fact.Items {
				tb.Str("  ").Str(item).Byte('\n')
			}
		}
	}
	return tb.String()
}

// exportedRange is both endpoints materialized, and how to remove them.
type exportedRange struct {
	// runDir holds both exports, and nothing outside it is written.
	runDir  string
	fromDir string
	toDir   string
	// remove deletes runDir, and MUST be called on every exit path once the
	// export answered.
	remove func() error
}

// endpointExporter materializes the two commits, one after the other, never in
// parallel (one CPU-intensive run per session). On an error it MUST leave
// nothing behind, because the caller has no range to remove.
type endpointExporter func(ctx context.Context, repo, fromCommit, toCommit string) (exportedRange, error)

// reportRunsDir is the subdirectory of this session's scratch that holds one
// run directory per report.
const reportRunsDir = "weekly-report"

// reportScratch answers the directory a report creates its run directory in.
// A test points it at a temporary directory, so the run does not resolve a
// session inside a fixture repository.
var reportScratch = sessionReportScratch

// sessionReportScratch answers this session's report directory, created. It is
// under the session's scratch (`./le session scratch ensure`), so two sessions
// exporting the same range never share a directory.
func sessionReportScratch(root string) (string, error) {
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return "", fmt.Errorf("weekly report: resolve this session's scratch directory, where the endpoints are exported: %w", err)
	}
	directory := filepath.Join(root, paths.Scratch, reportRunsDir)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", fmt.Errorf("weekly report: create %s: %w", directory, err)
	}
	return directory, nil
}

// exportEndpoints creates one run directory and exports from, then to, into
// its two subdirectories. Exactly two exports exist per run, and the run
// directory is removed whole, so a failed run leaves nothing to find later.
func exportEndpoints(ctx context.Context, repo, fromCommit, toCommit string) (exportedRange, error) {
	base, err := reportScratch(repo)
	if err != nil {
		return exportedRange{}, err
	}

	runDir, err := os.MkdirTemp(base, "run-")
	if err != nil {
		return exportedRange{}, fmt.Errorf("weekly report: create the run directory under %s: %w", base, err)
	}
	exported := exportedRange{
		runDir:  runDir,
		fromDir: filepath.Join(runDir, "from"),
		toDir:   filepath.Join(runDir, "to"),
		remove:  func() error { return os.RemoveAll(runDir) },
	}

	// One after the other, never in parallel: each is a CPU-intensive job.
	if err := exportEndpoint(ctx, repo, "from", fromCommit, exported.fromDir); err != nil {
		return exportedRange{}, removeFailedRun(err, exported)
	}
	if err := exportEndpoint(ctx, repo, "to", toCommit, exported.toDir); err != nil {
		return exportedRange{}, removeFailedRun(err, exported)
	}
	return exported, nil
}

// exportEndpoint exports one commit into dir, which it creates empty, as
// export.Commit requires.
func exportEndpoint(ctx context.Context, repo, keyword, commit, dir string) error {
	if err := os.Mkdir(dir, 0o750); err != nil {
		return fmt.Errorf("weekly report: create the %s export directory: %w", keyword, err)
	}
	if err := export.Commit(ctx, repo, commit, dir); err != nil {
		return fmt.Errorf("weekly report: export the %s commit %s: %w", keyword, commit, err)
	}
	return nil
}

// removeFailedRun removes a run whose export failed, and keeps both errors.
func removeFailedRun(exportErr error, exported exportedRange) error {
	if removeErr := exported.remove(); removeErr != nil {
		return errors.Join(exportErr, fmt.Errorf("weekly report: remove the run directory %s: %w", exported.runDir, removeErr))
	}
	return exportErr
}

// reportRun is one report over one checkout.
type reportRun struct {
	repo    string
	export  endpointExporter
	sources []EventSource
	// diagnostics receives the stack of a source that panicked; the section
	// itself carries only the panic value.
	diagnostics io.Writer
}

// answer resolves the range, materializes it, and asks every source.
//
// Both revisions resolve before anything is exported, so a refused revision
// costs no export (AC-7). Once the export answered, its removal is deferred,
// so it runs on every later exit: a returned error, and a panic outside a
// source as it unwinds. A panic inside a source is recovered by answerSection
// into a failed section, because one owner's defect must not hide the other
// sections (AC-8).
func (run reportRun) answer(ctx context.Context, from, to string) (report EventReport, err error) {
	fromCommit, err := resolveEndpoint(ctx, run.repo, "from", from)
	if err != nil {
		return EventReport{}, err
	}
	toCommit, err := resolveEndpoint(ctx, run.repo, "to", to)
	if err != nil {
		return EventReport{}, err
	}
	commits, err := firstParentCommits(ctx, run.repo, fromCommit, toCommit)
	if err != nil {
		return EventReport{}, err
	}

	exported, err := run.export(ctx, run.repo, fromCommit, toCommit)
	if err != nil {
		return EventReport{}, err
	}
	defer func() {
		if removeErr := exported.remove(); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("weekly report: remove the run directory %s: %w", exported.runDir, removeErr))
		}
	}()

	events := EventRange{
		Repo:       run.repo,
		FromCommit: fromCommit,
		ToCommit:   toCommit,
		FromDir:    exported.fromDir,
		ToDir:      exported.toDir,
		Commits:    commits,
	}
	report = EventReport{
		From:     Endpoint{Revision: from, Commit: fromCommit},
		To:       Endpoint{Revision: to, Commit: toCommit},
		Commits:  len(commits),
		RunDir:   exported.runDir,
		Sections: make([]Section, 0, len(run.sources)),
	}
	for _, source := range run.sources {
		report.Sections = append(report.Sections, answerSection(ctx, source, events, run.diagnostics))
	}
	return report, nil
}

// answerSection asks one source, and turns its failure, or an answer with no
// facts, into a failed section that names the reason.
//
// A panicking source is a defect in its owner package. It answers its section
// failed with the panic value, and its stack goes to diagnostics, so the other
// sections still answer and the exit code still says the report is incomplete.
func answerSection(ctx context.Context, source EventSource, events EventRange, diagnostics io.Writer) (section Section) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		var tb textbuf.Buffer
		tb.Str("weekly report: source ").Str(source.Section).Str(" panicked\n")
		diagnostics.Write([]byte(tb.String())) //nolint:errcheck // a diagnostic beside the failed section, which carries the reason
		diagnostics.Write(debug.Stack())       //nolint:errcheck // same diagnostic
		tb.Reset()
		tb.Str("the source panicked, a defect in the package that registered it: ").Str(fmt.Sprint(recovered))
		section = Section{Name: source.Section, Status: SectionFailed, Reason: tb.String(), Facts: []Fact{}}
	}()

	body, err := source.Answer(ctx, events)
	if err != nil {
		return Section{Name: source.Section, Status: SectionFailed, Reason: err.Error(), Facts: []Fact{}}
	}
	if len(body.Facts) == 0 {
		return Section{Name: source.Section, Status: SectionFailed, Facts: []Fact{},
			Reason: "the source answered no facts; a source with no events states a zero count"}
	}
	return Section{Name: source.Section, Status: SectionAnswered, Note: body.Note, Facts: body.Facts}
}

// resolveEndpoint resolves one revision through the resolver le's
// committed-source readers share, and names the revision and the fix when it
// cannot.
func resolveEndpoint(ctx context.Context, repo, keyword, revision string) (string, error) {
	commit, err := repoobjects.Resolve(ctx, repo, revision)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("weekly report: ").Str(keyword).Str(" revision ").Quoted(revision).
			Str(" does not name a commit in this checkout; pass the full sha the post's front matter records (")
		tb.Str("release-from for from, release-to for to), after git fetch if the commit is not local")
		return "", fmt.Errorf("%s: %w", tb.String(), err)
	}
	return commit, nil
}

// firstParentCommits lists the first-parent commits after from up to to,
// oldest first. Both are full commit IDs repoobjects.Resolve answered, so
// neither can be read as an option.
func firstParentCommits(ctx context.Context, repo, from, to string) ([]string, error) {
	var tb textbuf.Buffer
	span := tb.Str(from).Str("..").Str(to).String()
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "rev-list", "--first-parent", "--reverse", span) //nolint:gosec // two resolved commit IDs, one argv element
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("weekly report: list the commits of %s: %w", span, err)
	}
	return strings.Fields(string(output)), nil
}
