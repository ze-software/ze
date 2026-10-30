package site

import (
	"slices"
	"testing"
)

// plainFigureSnapshot states a snapshot with one value of each shape the guard
// matches, one under the raw threshold, and a source sentence it must ignore.
const plainFigureSnapshot = `{
  "_sources": {"cli_commands": "1,234 commands counted by the binary"},
  "cli_commands": 490,
  "config_sections": 99,
  "github_stars": 100,
  "interop": {"scenarios": 137, "scenarios_display": "130+"},
  "rfc": {"requirements_display": "4,744"},
  "tests": {"unit": 23712, "unit_display": "23,700+"}
}`

// VALIDATES: AC-7, the wiring row for ./le site check. An artifact whose page
// shows a snapshot display string, or a value of 100 or more beside its unit
// word, outside a marker is refused, naming the page, the figure and the key;
// the same artifact with every figure marked passes.
//
// The method lays the artifact down twice and asks checkPlainFigures, which
// runCheck reports as plain-figures, and the report's exit status for each.
func TestSiteCheckRefusesAPublishedValueOutsideItsMarker(t *testing.T) {
	plain := t.TempDir()
	writeArtifactFile(t, plain, factsFile, plainFigureSnapshot)
	writeArtifactFile(t, plain, "quality/index.html", `<!doctype html><html><head>
<title>Ze runs 23,700+ unit tests</title>
<meta name="description" content="130+ interop scenarios">
</head><body><main>
<p>Ze answers 490 commands and 23,712 unit tests.</p>
<p>It holds 4,744 requirements and 100 stars.</p>
<p>In 2026 Ze had 490 of them, 1490 commands, 99 sections, 34,744 rows and v4,744.1.</p>
<pre>490 commands</pre><code>23,700+</code><script>"130+"</script>
<p><span data-ze-stat="cli_commands">490</span> commands, <span data-ze-rfc="rfc4271:proven">4,744</span>.</p>
</main></body></html>`)

	findings, err := checkPlainFigures(plain)
	if err != nil {
		t.Fatal(err)
	}
	want := []plainFigure{
		{Page: "quality/index.html", Figure: "23,700+", Key: "tests.unit_display"},
		{Page: "quality/index.html", Figure: "130+", Key: "interop.scenarios_display"},
		{Page: "quality/index.html", Figure: "490", Key: "cli_commands"},
		{Page: "quality/index.html", Figure: "23,712", Key: "tests.unit"},
		{Page: "quality/index.html", Figure: "100", Key: "github_stars"},
		{Page: "quality/index.html", Figure: "4,744", Key: "rfc.requirements_display"},
	}
	if !slices.Equal(findings, want) {
		t.Errorf("the check found\n%v\nwant\n%v", findings, want)
	}
	if (checkReport{PlainFigures: findings}).exit() != 1 {
		t.Error("a page showing a plain published value must fail the site check")
	}

	marked := t.TempDir()
	writeArtifactFile(t, marked, factsFile, plainFigureSnapshot)
	writeArtifactFile(t, marked, "quality/index.html", `<!doctype html><html><head>
<title>Ze unit tests</title>
</head><body><main>
<p>Ze answers <span data-ze-stat="cli_commands">490</span> commands and
<span data-ze-stat="tests.unit_display"><b>23,700+</b></span> unit tests.</p>
<p>It holds <span data-ze-rfc="rfc4271:requirements">4,744</span> requirements.</p>
</main></body></html>`)
	findings, err = checkPlainFigures(marked)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("a page with every figure marked was refused: %v", findings)
	}
	if (checkReport{PlainFigures: findings}).exit() != 0 {
		t.Error("a page with every figure marked must pass the site check")
	}
}
