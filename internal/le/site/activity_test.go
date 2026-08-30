package site

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// activityFixtureDay is the day every test in this file builds on. A window is
// measured backwards from it, so a fixed day makes the grid, the range pill and
// the month labels the same on every run.
var activityFixtureDay = time.Date(2026, time.August, 25, 11, 30, 0, 0, time.UTC)

// activityFixturePaths answers a Paths whose repository is a throwaway git tree
// with one commit, and whose source is this checkout's own website directory.
//
// The two roots differ on purpose. The measurement walks the repository, and
// walking this checkout would read every tracked .go file for every test in
// this file. The page sidebar and the header come from website/data, which only
// the real tree carries.
func activityFixturePaths(t *testing.T) Paths {
	t.Helper()

	// git rev-parse --show-toplevel answers the symlink-resolved path, and on
	// macOS t.TempDir() hands back /var/... for a real /private/var/... So the
	// fixture root is resolved here, or the measurement reports another path.
	repository, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	activityFixtureGit(t, repository, "init", "-q")
	activityFixtureGit(t, repository, "config", "user.name", "Fixture")
	activityFixtureGit(t, repository, "config", "user.email", "fixture@example.test")
	source := "package main\n\n// One comment line.\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(repository, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	activityFixtureGit(t, repository, "add", ".")
	commit := exec.CommandContext(t.Context(), "git", "-C", repository,
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture")
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2026-08-20T12:00:00Z", "GIT_COMMITTER_DATE=2026-08-20T12:00:00Z")
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}

	previous := buildClock
	buildClock = func() time.Time { return activityFixtureDay }
	t.Cleanup(func() { buildClock = previous })

	return Paths{
		Repository: repository,
		Source:     filepath.Join(repositoryRoot(t), "website"),
		Output:     t.TempDir(),
	}
}

func activityFixtureGit(t *testing.T, repository string, arguments ...string) {
	t.Helper()
	argv := append([]string{"-C", repository}, arguments...)
	if output, err := exec.CommandContext(t.Context(), "git", argv...).CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
}

// activityRenderedBody renders the page over the fixture repository and answers
// what it wrote under <main>.
func activityRenderedBody(t *testing.T, paths Paths) string {
	t.Helper()
	routes, err := renderActivityPage(paths)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(routes) != 1 || routes[0] != activityRoute {
		t.Fatalf("routes = %v, want [%s]", routes, activityRoute)
	}
	page, err := os.ReadFile(filepath.Join(paths.Output, filepath.FromSlash(activityDest)))
	if err != nil {
		t.Fatal(err)
	}
	body, err := extractMain(string(page))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// activityNumbers matches every run of digits, with the commas and hyphens that
// group a count or a date. A parity comparison replaces each run, because the
// fixture repository has one commit where the published page has 6,862.
var activityNumbers = regexp.MustCompile(`\d[\d,\-]*`)

// activityWording answers the words of one page fragment with every number
// removed, which is what "reads the same" means for a page whose whole content
// is a measurement.
func activityWording(fragment string) string {
	return activityNumbers.ReplaceAllString(visibleText(fragment), "#")
}

// TestTheActivityPageReadsAsThePublishedPage compares every word the page shows
// against the page published at gh-pages 2fa8fa2ad, with the measured numbers
// removed. The goal is rendered parity: a reader meets the same headings, the
// same labels and the same captions.
//
// VALIDATES: every word the activity page shows is the word the published page
// showed, once the measured numbers are set aside.
func TestTheActivityPageReadsAsThePublishedPage(t *testing.T) {
	paths := activityFixturePaths(t)
	got := activityWording(activityRenderedBody(t, paths))

	// The fixture is everything the published page holds inside <main>, the
	// page sidebar included, so this compares the chrome the shell writes as
	// well as the body this producer renders.
	want := activityWording(readFixture(t, "published-activity-main.html"))
	if got != want {
		t.Errorf("page wording\n got: %s\nwant: %s", got, want)
	}
}

// TestTheActivityPageCarriesTheDayCellGrid proves the calendar reaches the
// published page: one cell for each day of each week column, spelled the way
// the published page spells one.
//
// VALIDATES: the calendar the page exists to show reaches the artifact, with
// one cell for each day and the measured counts on the day that carried them.
func TestTheActivityPageCarriesTheDayCellGrid(t *testing.T) {
	paths := activityFixturePaths(t)
	body := activityRenderedBody(t, paths)

	grid, found := activitySliceBetween(body, `<div class="activity-grid"`, "</div>\n                </div>")
	if !found {
		t.Fatal("page carries no activity-grid")
	}
	cells := strings.Count(grid, `<div class="day-cell`)
	if cells == 0 || cells%7 != 0 {
		t.Errorf("grid holds %d day cells, want a non-zero multiple of seven", cells)
	}

	// The one commit landed on 2026-08-20 and added four lines, so that day is
	// the only colored square on the grid and it carries both counts.
	const commitDay = `<div class="day-cell" data-date="2026-08-20" data-date-label="Thu 20 Aug 2026"` +
		` data-lines="4" data-lines-display="4" data-lines-level="5" data-commits="1"` +
		` data-commits-display="1" data-commits-level="5" data-level="5"` +
		` aria-label="Thu 20 Aug 2026: 4 lines added, 1 commits" tabindex="0"></div>`
	if !strings.Contains(grid, commitDay) {
		t.Errorf("grid carries no cell for the commit day; want\n%s", commitDay)
	}
}

// TestTheActivityPageStatesTheLabelsAReaderCannotSee checks the attributes a
// visible-text comparison is blind to. A landmark that loses its name is a
// regression no wording test can witness.
//
// VALIDATES: every aria-labelledby, aria-label, aria-hidden and role the page
// carries, which a visible-text comparison is blind to.
func TestTheActivityPageStatesTheLabelsAReaderCannotSee(t *testing.T) {
	paths := activityFixturePaths(t)
	body := activityRenderedBody(t, paths)

	for _, want := range []string{
		`<section class="activity-page" aria-labelledby="activity-title">`,
		`<h1 id="activity-title">Development activity</h1>`,
		`<div class="activity-widget reveal" aria-label="Activity heatmap">`,
		`<section class="stats" aria-label="Summary">`,
		`<div class="metric-switch" aria-label="Activity metric">`,
		`<button type="button" data-metric="lines" aria-pressed="true">Added Lines</button>`,
		`<button type="button" data-metric="commits" aria-pressed="false">Commits</button>`,
		`<div class="months" aria-hidden="true">`,
		`<div class="weekday-labels" aria-hidden="true">`,
		`<div class="activity-grid" role="img" aria-label="Daily activity from 2025-08-26 to 2026-08-25">`,
		`<section class="panel go-panel" aria-label="Go code stats">`,
		`<div id="activity-tooltip" class="activity-tooltip" role="tooltip" hidden>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not carry %s", want)
		}
	}
}

// TestTheActivityPageCarriesTheSharedShell proves the page is published through
// the same chrome as every other route rather than as a standalone document.
//
// VALIDATES: AC-3, the shared shell: title, canonical, stylesheet, the main
// class the sidebar decides, the sidebar itself, the deferred script, and the
// footer stamp.
func TestTheActivityPageCarriesTheSharedShell(t *testing.T) {
	paths := activityFixturePaths(t)
	if _, err := renderActivityPage(paths); err != nil {
		t.Fatalf("render: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(paths.Output, filepath.FromSlash(activityDest)))
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	for _, want := range []string{
		`<title>Development Activity - Ze</title>`,
		`<link rel="canonical" href="https://ze-software.net/project/activity/" />`,
		`<link rel="stylesheet" href="../../assets/site.css" />`,
		`<main id="top" class="has-page-sidebar" tabindex="-1">`,
		`<aside class="page-sidebar" aria-label="Related page links">`,
		`<a class="page-sidebar-link" href="../../project/changes/">`,
		`<script src="../../assets/site.js" defer></script>`,
		"<footer>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page does not carry %s", want)
		}
	}
}

// TestTheActivityMirrorReadsAsThePublishedMirror compares the Markdown beside
// the page against the mirror published at gh-pages 2fa8fa2ad, with the
// measured numbers removed.
//
// The mirror is converted back from the rendered body rather than written by
// hand, so this also proves the conversion still reaches every panel: a section
// that stops rendering disappears from both files at once.
//
// VALIDATES: AC-5, the Markdown mirror beside the route, in the shape the
// published mirror holds.
func TestTheActivityMirrorReadsAsThePublishedMirror(t *testing.T) {
	paths := activityFixturePaths(t)
	if _, err := renderActivityPage(paths); err != nil {
		t.Fatalf("render: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(paths.Output, activityDirectory, pageMirrorFile))
	if err != nil {
		t.Fatal(err)
	}
	got := activityMirrorShape(string(raw))
	want := activityMirrorShape(readFixture(t, "published-activity.md"))
	if got != want {
		t.Errorf("mirror shape\n got:\n%s\nwant:\n%s", got, want)
	}
}

// activityMirrorShape answers the mirror's headings and its bullet labels, with
// every measured value removed. The published mirror carries the lead the page
// showed on the day it was written, so the lead is not part of the shape.
func activityMirrorShape(mirror string) string {
	var shape strings.Builder
	for line := range strings.SplitSeq(mirror, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "#"):
			shape.WriteString(activityNumbers.ReplaceAllString(line, "#") + "\n")
		case strings.HasPrefix(line, "- "):
			label, _, _ := strings.Cut(line[2:], ":")
			shape.WriteString("- " + label + "\n")
		}
	}
	return shape.String()
}

// TestTwoActivityBuildsOverOneHistoryWriteTheSameBytes proves the page is
// derived from the history and the build clock alone.
//
// The window ends on the build clock rather than on the wall clock, so a second
// build minutes after the first draws the same days. Without that, every build
// would republish this page and its mirror whether or not anything changed.
//
// VALIDATES: AC-14, a second build over an unchanged tree writes the same page.
func TestTwoActivityBuildsOverOneHistoryWriteTheSameBytes(t *testing.T) {
	paths := activityFixturePaths(t)
	first := activityRenderedBody(t, paths)

	paths.Output = t.TempDir()
	second := activityRenderedBody(t, paths)
	if first != second {
		t.Error("two builds over one history wrote different pages")
	}
}

// TestExactlyOneProducerClaimsTheActivityRoute proves the registry answers for
// the route once. A second claimant would mean two writers and a page whose
// content depends on registration order.
//
// VALIDATES: AC-1 for the last published route no producer claimed, 1 of 712.
func TestExactlyOneProducerClaimsTheActivityRoute(t *testing.T) {
	// A producer that writes a page nobody published is as wrong as a
	// published page nobody writes, so the claim is checked against the
	// artifact the retired renderers left.
	if !slices.Contains(publishedArtifactRoutes(t), activityRoute) {
		t.Errorf("%s is not a route the site published", activityRoute)
	}

	// The whole registry is asked, not this producer alone. The failure this
	// guards is a SECOND writer of one route, and a second writer carries its
	// own name, so counting the producers named for this page would answer one
	// however many of them wrote the page.
	// The sweep runs over the throwaway repository, so it costs under a
	// second. Over this checkout it costs two minutes and examines three more
	// producers, which is not what those two minutes are worth on every run:
	// `./le site check` reads a real build and answers for the whole registry,
	// and TestCheckRefusesARouteTwoProducersWrote covers that arithmetic. What
	// is bought here is the cheap half, run on every commit.
	paths := activityFixturePaths(t)
	var claimants, refused []string
	for _, producer := range allProducers() {
		paths.Output = t.TempDir()
		routes, err := producer.Render(paths)
		if err != nil {
			// A producer this checkout cannot run is named rather than
			// counted, so a reader sees which population was examined.
			// website/assets/demos/ is generated and absent here, so the docs
			// producer and the homepage refuse (R-8 of the spec).
			refused = append(refused, producer.Name)
			continue
		}
		if slices.Contains(routes, activityRoute) {
			claimants = append(claimants, producer.Name)
		}
	}
	if len(claimants) != 1 || claimants[0] != activityProducer {
		t.Errorf("%s is claimed by %v, want [%s]; producers that could not run here: %v",
			activityRoute, claimants, activityProducer, refused)
	}
}

// activitySliceBetween answers the text between one opening marker and the
// first closing marker after it.
func activitySliceBetween(body, opening, closing string) (string, bool) {
	start := strings.Index(body, opening)
	if start < 0 {
		return "", false
	}
	end := strings.Index(body[start:], closing)
	if end < 0 {
		return "", false
	}
	return body[start : start+end], true
}
