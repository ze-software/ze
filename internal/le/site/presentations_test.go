// Design: website/AI.md -- a published deck is a snapshot of the day it was given
// Detail: presentations.go -- renderActivity measures the window these assert

package site

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// activityRepository builds a checkout with one commit inside the deck's window
// and commits after it. Both tests measure the same commits, so both build them
// here. The checkout carries no website sources, because the embed carries its
// own stylesheet and reads nothing from the tree but its git history.
func activityRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	run := func(name string, argument ...string) {
		t.Helper()
		command := exec.CommandContext(t.Context(), name, argument...)
		command.Dir = repository
		command.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s %s: %v: %s", name, strings.Join(argument, " "), err, output)
		}
	}
	commitOn := func(day, file string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repository, file), []byte("package main\n\nfunc "+strings.TrimSuffix(file, ".go")+"() {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		run("git", "add", file)
		stamp := day + "T12:00:00+00:00"
		command := exec.CommandContext(t.Context(), "git", "commit", "-m", file)
		command.Dir = repository
		command.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test",
			"GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("commit %s: %v: %s", day, err, output)
		}
	}

	run("git", "init", "-q", "-b", "main")
	commitOn("2026-06-10", "before.go")
	// The day after the talk falls inside the grid's last week, which is drawn
	// whole, and the day in August falls outside the grid entirely. The two
	// reach a frozen render by different routes, so both are committed here.
	commitOn("2026-06-12", "tomorrow.go")
	commitOn("2026-08-28", "after.go")
	return repository
}

// renderDeckActivity renders the deck embed for the frozen talk date and
// answers the published page.
func renderDeckActivity(t *testing.T, repository string) string {
	t.Helper()
	output := filepath.Join(t.TempDir(), "activity.html")
	if err := renderActivity(ActivityOptions{
		Repository: repository,
		Output:     output,
		Days:       365,
		Today:      time.Date(2026, time.June, 11, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("render activity: %v", err)
	}
	page, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	return string(page)
}

// activityCell answers the two metrics the grid draws one day at.
func activityCell(t *testing.T, page, day string) (added, commits string) {
	t.Helper()
	cell := regexp.MustCompile(`data-date="` + day + `"[^>]*data-lines="(\d+)"[^>]*data-commits="(\d+)"`).FindStringSubmatch(page)
	if cell == nil {
		t.Fatalf("the grid draws no cell for %s\n%s", day, page)
	}
	return cell[1], cell[2]
}

// VALIDATES: renderActivity closes its window at Today, so a deck frozen at a
// past date does not gain the commits made after it.
//
// The method builds a repository with commits before, one day after, and ten
// weeks after the frozen date, renders at that date, and asserts that neither
// later commit reaches the page: the August day is absent from it, and the cell
// the grid draws for the day after the talk is empty. Deleting the --until
// bound in sourcerewrite.collectActivity reddens the last assertion, because
// the render then counts a day that had not happened yet.
func TestRenderActivityStopsAtToday(t *testing.T) {
	page := renderDeckActivity(t, activityRepository(t))
	if !strings.Contains(page, "2026-06-10") {
		t.Errorf("the day inside the window is absent\n%s", page)
	}
	if strings.Contains(page, "2026-08-28") {
		t.Errorf("a day after Today reached a frozen snapshot\n%s", page)
	}
	if added, commits := activityCell(t, page, "2026-06-10"); added == "0" || commits == "0" {
		t.Errorf("the day inside the window carries %s added lines and %s commits, want the commit it was given", added, commits)
	}
	if added, commits := activityCell(t, page, "2026-06-12"); added != "0" || commits != "0" {
		t.Errorf("the day after Today carries %s added lines and %s commits, want an empty cell", added, commits)
	}
}

// VALIDATES: the deck embed carries the calendar heatmap widget, the styles
// that draw it, and the deck's own presentation of them rather than the
// website's.
//
// The method renders the embed and asserts what a bare table cannot have: a day
// cell for every day of the window, the heat level each cell is drawn at, and
// the widget's own layout rules. It then asserts the three marks that separate
// this rendering from the published page, each of which the site rendering
// fails: the dark palette the deck is projected in, the absence of the page's
// hero, and the absence of the site stylesheet the embed cannot resolve.
func TestRenderActivityEmbedsHeatmap(t *testing.T) {
	page := renderDeckActivity(t, activityRepository(t))

	if cells := strings.Count(page, `class="day-cell`); cells < 365 {
		t.Errorf("the embed draws %d day cells, want one for at least the 365 measured days", cells)
	}
	if !strings.Contains(page, `class="activity-grid"`) {
		t.Error("the embed carries no activity grid")
	}
	if !strings.Contains(page, `data-level="`) {
		t.Error("the embed carries no heat level")
	}
	if !strings.Contains(page, ".activity-widget .day-cell") {
		t.Error("the embed carries no widget stylesheet, so its grid has no layout")
	}
	if !strings.Contains(page, "--level-5: #bef86a") {
		t.Error("the embed carries no presentation palette, so its cells have no color")
	}
	if !strings.Contains(page, "color-scheme: dark") {
		t.Error("the embed states no color scheme, so a projected deck meets a white page")
	}
	if !strings.Contains(page, `class="activity-slide"`) {
		t.Error("the embed is not rooted in the slide shell, so it is sized for a page")
	}
	if strings.Contains(page, "activity-hero") {
		t.Error("the embed carries the published page's hero, which spends the slide's height")
	}
	if strings.Contains(page, "--mint-base:") {
		t.Error("the embed inlines the light site stylesheet, which the deck's own rules would have to fight")
	}
}
