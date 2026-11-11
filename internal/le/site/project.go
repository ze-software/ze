// Design: website/AI.md -- the project overview derives recent work from canonical sources.
// Related: docs.go supplies the shared HTML shell and Markdown mirror.
package site

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	projectSource        = "website/project/index.md"
	projectDestination   = "project/index.html"
	projectLatestSlot    = "{project_latest}"
	projectLandmarksSlot = "{project_landmarks}"
	projectLandmarksMax  = 3
)

var projectSlotPattern = regexp.MustCompile(`\{project_[^{}]+\}`)

// renderProject owns the overview while the shared documentation renderer owns
// its publishing format. Reports and landmarks retain their existing sources.
func renderProject(paths Paths) ([]string, error) {
	source, err := os.ReadFile(filepath.Join(paths.Repository, projectSource)) //nolint:gosec // The build reads the authored page from its selected repository.
	if err != nil {
		return nil, err
	}
	vocabulary, err := loadTopicVocabulary(paths.Source)
	if err != nil {
		return nil, err
	}
	weeks, err := loadChangeWeeks(paths.Source, vocabulary)
	if err != nil {
		return nil, err
	}
	var landmarks milestoneData
	if err := readSourceJSON(paths.Source, milestonesDataFile, &landmarks); err != nil {
		return nil, err
	}
	sourceText, err := projectOverviewContent(string(source), weeks, landmarks.Milestones)
	if err != nil {
		return nil, err
	}
	renderer, err := newDocsRenderer(paths)
	if err != nil {
		return nil, err
	}
	page := sitePage{
		Source:  projectSource,
		Dest:    projectDestination,
		Desc:    "Where Ze stands, recent development, release work, demonstrations, and ways to contribute.",
		Journey: "Project",
	}
	if err := renderer.renderSource(page, []byte(sourceText)); err != nil {
		return nil, err
	}
	return []string{"/" + page.destinationDirectory() + "/"}, nil
}

// projectOverviewContent fills only this page's two slots. Validation precedes
// insertion so a report containing slot-like text remains ordinary text.
// Weeks MUST arrive newest first from loadChangeWeeks, as its callers require.
func projectOverviewContent(source string, weeks []changeWeek, landmarks []milestone) (string, error) {
	for _, slot := range []string{projectLatestSlot, projectLandmarksSlot} {
		if strings.Count(source, slot) != 1 {
			return "", fmt.Errorf("%s needs exactly one %s slot; restore that slot before running ./le site build", projectSource, slot)
		}
	}
	unfilled := strings.NewReplacer(projectLatestSlot, "", projectLandmarksSlot, "").Replace(source)
	if slot := projectSlotPattern.FindString(unfilled); slot != "" {
		return "", fmt.Errorf("%s contains unknown slot %s; remove it before running ./le site build", projectSource, slot)
	}
	latest, err := projectLatestUpdate(weeks)
	if err != nil {
		return "", err
	}
	history, err := projectLandmarkList(landmarks)
	if err != nil {
		return "", err
	}
	return strings.NewReplacer(projectLatestSlot, latest, projectLandmarksSlot, history).Replace(source), nil
}

// projectLatestUpdate uses the same authored summary and clipping rule as the
// homepage, with the report's complete coverage label alongside it.
func projectLatestUpdate(weeks []changeWeek) (string, error) {
	if len(weeks) == 0 {
		return "", fmt.Errorf("the project overview has no update report; add a post under website/changes/posts before running ./le site build")
	}
	week := weeks[0]
	summary := week.homeSummary
	if summary == "" {
		summary = week.Intro
	}
	if strings.TrimSpace(summary) == "" {
		return "", fmt.Errorf("update report %s has no introduction or home-summary; add one before running ./le site build", week.Slug)
	}
	var body textbuf.Buffer
	body.Reset().Str(`<p class="project-report-date">`).Str(html.EscapeString(week.label())).Str("</p>\n").
		Str("<p>").Str(html.EscapeString(clipSummary(summary))).Str("</p>\n").
		Str(`<a class="project-text-link" href="../`).Str(html.EscapeString(week.route())).
		Str(`">Read the latest update <span aria-hidden="true">→</span></a>`)
	return body.String(), nil
}

// projectLandmarkList shares the timeline's date/category validation and stable
// ordering. A short or empty history is displayed without invented entries.
func projectLandmarkList(landmarks []milestone) (string, error) {
	months, err := groupMilestones(landmarks, nil)
	if err != nil {
		return "", err
	}
	if len(landmarks) == 0 {
		return "<p>No project milestones have been recorded yet.</p>", nil
	}
	var body textbuf.Buffer
	body.Reset().Str(`<ol class="project-landmarks">`)
	shown := 0
	for _, month := range months {
		for _, item := range month.Items {
			if shown == projectLandmarksMax {
				break
			}
			if strings.TrimSpace(item.Title) == "" {
				return "", fmt.Errorf("data/%s has a milestone dated %s with no title; add its title before running ./le site build", milestonesDataFile, item.Date)
			}
			body.Str("\n<li><time datetime=\"").Str(html.EscapeString(item.Date)).Str(`">`).
				Str(html.EscapeString(item.Date)).Str("</time><h3>").Str(html.EscapeString(item.Title)).
				Str("</h3><p>").Str(html.EscapeString(item.Blurb)).Str("</p></li>")
			shown++
		}
		if shown == projectLandmarksMax {
			break
		}
	}
	body.Str("\n</ol>")
	return body.String(), nil
}
