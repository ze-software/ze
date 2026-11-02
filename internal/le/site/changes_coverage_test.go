// Design: website/AI.md -- coverage dates belong to the authored update.
package site

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestUpdateCoverageReachesEverySurface drives the publisher from a post whose
// filename differs from its coverage, then checks each reader-facing output.
func TestUpdateCoverageReachesEverySurface(t *testing.T) {
	paths := Paths{Repository: repositoryRoot(t), Source: t.TempDir(), Output: t.TempDir()}
	writeArtifactFile(t, paths.Source, topicsVocabularyFile, `{"tags":{"BGP":"routing"}}`)
	copyFixture(t, filepath.Join(paths.Repository, "website/data/page-links.json"), filepath.Join(paths.Source, "data/page-links.json"))
	writeArtifactFile(t, paths.Source, changesSourceDirectory+"/2026-10-26.md", "---\ncovers: 2026-10-12 .. 2026-10-25\ntags: BGP\n---\n**Update**\n\nThe full introduction.\n\n**Routing**\nA change.\n")
	if _, err := renderChanges(paths); err != nil {
		t.Fatal(err)
	}
	const label = "2026-10-12 – 2026-10-25 (2 weeks)"
	for _, dest := range []string{changesIndexDest, changesDirectory + "/index.md", changesDirectory + "/2026-10-12/index.html", changesDirectory + "/2026-10-12/index.md", changesFeedDest, changesLegacyFeedDest} {
		if !strings.Contains(readArtifact(t, paths.Output, dest), label) {
			t.Errorf("%s omits coverage %q", dest, label)
		}
	}
	var records []map[string]any
	if err := json.Unmarshal([]byte(readArtifact(t, paths.Output, changesIndexFile)), &records); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{"slug": "2026-10-12", "coverage_start": "2026-10-12", "coverage_end": "2026-10-25", "duration_days": float64(14), "duration": "2 weeks"} {
		if got := records[0][key]; got != want {
			t.Errorf("JSON %s = %v, want %v", key, got, want)
		}
	}
	weeks, err := loadChangeWeeks(paths.Source, testVocabulary)
	if err != nil {
		t.Fatal(err)
	}
	news, err := whatsNewBand(&whatsNewData{Link: &audienceLink{Href: "project/changes/", Label: "All updates"}}, nil, weeks)
	if err != nil {
		t.Fatal(err)
	}
	for name, surface := range map[string]string{"teasers": blogTeaserCards(weeks), "news": news} {
		if !strings.Contains(surface, label) {
			t.Errorf("%s omits coverage", name)
		}
	}
	page := readArtifact(t, paths.Output, changesDirectory+"/2026-10-12/index.html")
	for _, want := range []string{"Development update · 2 weeks", `<meta name="description" content="` + label, `<title>` + label + " - Ze</title>"} {
		if !strings.Contains(page, want) {
			t.Errorf("page omits %q", want)
		}
	}
}

// TestAuthoredUpdateCoverageLoads checks every repository post through the same
// loader the build uses, including the two-week report that exposed this defect.
func TestAuthoredUpdateCoverageLoads(t *testing.T) {
	source := filepath.Join(repositoryRoot(t), "website")
	vocabulary, err := loadTopicVocabulary(source)
	if err != nil {
		t.Fatal(err)
	}
	weeks, err := loadChangeWeeks(source, vocabulary)
	if err != nil {
		t.Fatal(err)
	}
	for _, week := range weeks {
		if week.Slug == "2026-10-12" {
			if got := week.label(); got != "2026-10-12 – 2026-10-25 (2 weeks)" {
				t.Fatalf("authored coverage = %q", got)
			}
			return
		}
	}
	t.Fatal("the October two-week update is missing")
}

// TestUpdateCoverageCalendarBounds checks inclusive calendar days, legacy time
// syntax and filename ranges without guessing a boundary from another post.
func TestUpdateCoverageCalendarBounds(t *testing.T) {
	for _, test := range []struct{ name, covers, label string }{
		{"week", "2026-10-12 .. 2026-10-18", "2026-10-12 – 2026-10-18 (1 week)"},
		{"partial", "2026-07-01 .. 2026-07-06", "2026-07-01 – 2026-07-06 (6 days)"},
		{"legacy-time", "2026-06-25 21:10 .. 2026-07-01", "2026-06-25 – 2026-07-01 (1 week)"},
		{"leap", "2024-02-28 .. 2024-03-01", "2024-02-28 – 2024-03-01 (3 days)"},
		{"day", "2026-10-12", "2026-10-12 – 2026-10-12 (1 day)"},
		{"filename", "", "2026-10-12 – 2026-10-25 (2 weeks)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			week, err := writeWeek(t, "2026-10-12..2026-10-25.md", "---\ncovers: "+test.covers+"\ntags: BGP\n---\n**Update**\nIntro.\n")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(changeWeekMirror(week), test.label) {
				t.Errorf("mirror omits %q", test.label)
			}
		})
	}
}

// TestUpdateCoverageRefusesInvalidBounds proves malformed ends cannot silently
// become a plausible weekly label.
func TestUpdateCoverageRefusesInvalidBounds(t *testing.T) {
	for _, covers := range []string{"2026-10-12 ..", "2026-10-12 .. nope", "2026-10-12 .. 2026-02-30", "2026-10-12 .. 2026-10-11", "2026-10-12 .. 2026-10-25 .. 2026-11-01", "2026-10-12 junk .. 2026-10-25", "2026-10-12 25:10 .. 2026-10-25", "nope .. 2026-10-25"} {
		t.Run(covers, func(t *testing.T) {
			_, err := writeWeek(t, "2026-10-12.md", "---\ncovers: "+covers+"\ntags: BGP\n---\n**Update**\nIntro.\n")
			if err == nil {
				t.Fatal("invalid coverage accepted")
			}
		})
	}
}
