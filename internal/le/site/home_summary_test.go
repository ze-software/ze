// Design: website/AI.md -- homepage summaries stay separate from weekly article content.
package site

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestTheHomepageUsesAnAuthoredWeeklySummary exercises the file-to-homepage path
// and compares the full weekly page before and after adding teaser metadata.
func TestTheHomepageUsesAnAuthoredWeeklySummary(t *testing.T) {
	paths := homeFixture(t)
	const slug = "2026-08-17"
	const intro = "The full opening explains the configuration changes.\n\n" +
		"The release queue has a separate paragraph with its evidence and qualifications."
	source := strings.Replace(weekSource(slug, "BGP, CLI"), "The intro.", intro, 1)
	file := changesSourceDirectory + "/" + slug + markdownExtension
	writeArtifactFile(t, paths.Source, file, source)
	vocabulary := map[string]string{"BGP": categoryRouting, "CLI": categoryOperate}
	before, err := readChangeWeek(filepath.Join(paths.Source, file), vocabulary)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderChangeWeek(paths.Output, before, pageLinks{}); err != nil {
		t.Fatal(err)
	}
	pageBefore := readArtifact(t, paths.Output, before.dest())
	mirrorBefore := changeWeekMirror(before)

	const summary = "Configuration <checks> & safer commits."
	writeArtifactFile(t, paths.Source, file,
		strings.Replace(source, "---\n", "---\nhome-summary: "+summary+"\n", 1))
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, homeDest)
	card := sliceBetween(t, page, `<article class="card card-post home-update-card `, "</article>")
	const heading = `<h3><a href="project/changes/2026-08-17/">Configuration &lt;checks&gt; &amp; safer commits.</a></h3>`
	if !strings.Contains(card, heading) {
		t.Errorf("the newest card lost its escaped authored summary: %s", card)
	}
	if strings.Contains(card, "The full opening") {
		t.Error("the newest card used the full intro instead of its summary")
	}
	if strings.Contains(card, "release queue") {
		t.Error("the newest card contains the article's second paragraph")
	}
	after, err := readChangeWeek(filepath.Join(paths.Source, file), vocabulary)
	if err != nil {
		t.Fatal(err)
	}
	if after.Intro != intro {
		t.Errorf("teaser metadata changed the full introduction: %q", after.Intro)
	}
	if err := renderChangeWeek(paths.Output, after, pageLinks{}); err != nil {
		t.Fatal(err)
	}
	if pageAfter := readArtifact(t, paths.Output, after.dest()); pageAfter != pageBefore {
		t.Error("adding a homepage summary changed the full weekly HTML page")
	}
	if changeWeekMirror(after) != mirrorBefore {
		t.Error("adding a homepage summary changed the full weekly Markdown mirror")
	}
}

// TestTheHomepageBoundsWeeklyIntroFallbacks checks cards without authored
// summaries against independent expected text, including paragraph boundaries.
func TestTheHomepageBoundsWeeklyIntroFallbacks(t *testing.T) {
	for _, test := range []struct {
		name  string
		intro string
		want  string
	}{
		{
			name:  "long opening",
			intro: homeLongWeekIntro + "\n\nThe release queue stays in the article.",
			want: "One two three four five six seven eight nine ten eleven twelve " +
				"thirteen fourteen fifteen sixteen seventeen…",
		},
		{
			name:  "short opening",
			intro: "A short opening.\n\nThe release queue stays in the article.",
			want:  "A short opening.",
		},
		{
			name:  "wrapped opening",
			intro: "A short\nopening with <checks> & care.\n\nThe release queue stays in the article.",
			want:  "A short opening with &lt;checks&gt; &amp; care.",
		},
		{
			name:  "space separator",
			intro: "A short opening.\n \nThe release queue stays in the article.",
			want:  "A short opening.",
		},
		{
			name:  "tab separator",
			intro: "A short opening.\n\t\nThe release queue stays in the article.",
			want:  "A short opening.",
		},
		{
			name:  "CRLF separator",
			intro: "A short opening.\r\n\r\nThe release queue stays in the article.",
			want:  "A short opening.",
		},
		{
			name: "empty opening",
			want: "Weekly update",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			card := blogTeaserCards([]changeWeek{{Slug: "2026-08-17", Intro: test.intro}})
			want := `<h3><a href="project/changes/2026-08-17/">` + test.want + `</a></h3>`
			if !strings.Contains(card, want) {
				t.Errorf("the weekly teaser does not carry %q: %s", want, card)
			}
		})
	}
}
