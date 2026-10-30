package site

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

	check, err := checkPlainFigures(ratchetSource(t, noRatchetedPages), plain)
	if err != nil {
		t.Fatal(err)
	}
	findings := check.Figures
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
	check, err = checkPlainFigures(ratchetSource(t, noRatchetedPages), marked)
	if err != nil {
		t.Fatal(err)
	}
	findings = check.Figures
	if len(findings) != 0 {
		t.Errorf("a page with every figure marked was refused: %v", findings)
	}
	if (checkReport{PlainFigures: findings}).exit() != 0 {
		t.Error("a page with every figure marked must pass the site check")
	}
}

// noRatchetedPages is a ratchet file that lists no page, the state the RFC
// section's website phase leaves.
const noRatchetedPages = `{"pages": []}`

// ratchetReason is the reason every ratcheted page carries today.
const ratchetReason = "server-written RFC figures until spec-rfc-figures-sectioned-presentation's website phase"

// ratchetSource lays down a website source tree holding one ratchet file and
// answers its root.
func ratchetSource(t *testing.T, ratchet string) string {
	t.Helper()
	source := t.TempDir()
	writeArtifactFile(t, source, plainFigureRatchetFile, ratchet)
	return source
}

// ratchetedPage is one ratchet entry as the committed file spells it.
func ratchetedPage(page string) string {
	return `{"page": "` + page + `", "reason": "` + ratchetReason + `"}`
}

// VALIDATES: AC-8. An RFC page listed in the committed ratchet file is not
// refused for its plain figures. A listed page that no longer carries one is
// refused as a stale entry, so the list only shrinks. An unlisted RFC page is
// refused like any other page. The check never writes the list, and a list
// naming a page outside the RFC section, a page twice, a page with no reason,
// or a field it does not know is refused by name, as is a missing list.
//
// The method lays down one artifact with three RFC pages and checks it against
// ratchet files of each shape, comparing the findings, the stale entries, the
// report's exit status and the ratchet bytes after the check.
func TestSiteCheckHonoursAndShrinksTheRFCRatchet(t *testing.T) {
	const (
		index   = "quality/rfc-compliance/index.html"
		detail  = "quality/rfc-compliance/rfc4271/index.html"
		profile = "quality/rfc-compliance/profiles/index.html"
	)
	output := t.TempDir()
	writeArtifactFile(t, output, factsFile, plainFigureSnapshot)
	writeArtifactFile(t, output, index, `<!doctype html><html><body><p>It holds 4,744 requirements.</p></body></html>`)
	writeArtifactFile(t, output, detail, `<!doctype html><html><body><p>Ze answers 490 commands.</p></body></html>`)
	writeArtifactFile(t, output, profile, `<!doctype html><html><body><p>No figure here.</p></body></html>`)

	ratchet := `{"pages": [` + ratchetedPage(index) + `, ` + ratchetedPage(profile) + `]}`
	source := ratchetSource(t, ratchet)
	check, err := checkPlainFigures(source, output)
	if err != nil {
		t.Fatal(err)
	}
	wantFigures := []plainFigure{{Page: detail, Figure: "490", Key: "cli_commands"}}
	if !slices.Equal(check.Figures, wantFigures) {
		t.Errorf("the check found\n%v\nwant\n%v: a listed page is not refused, an unlisted RFC page is", check.Figures, wantFigures)
	}
	if !slices.Equal(check.StaleRatchet, []string{profile}) {
		t.Errorf("stale ratchet entries %v, want [%s]: a listed page with no plain figure is stale", check.StaleRatchet, profile)
	}
	report := checkReport{PlainFigures: check.Figures, StaleRatchet: check.StaleRatchet}
	if report.exit() != 1 {
		t.Error("an unlisted RFC page with a plain figure, or a stale entry, must fail the site check")
	}
	if (checkReport{StaleRatchet: check.StaleRatchet}).exit() != 1 {
		t.Error("a stale ratchet entry alone must fail the site check")
	}
	written, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(plainFigureRatchetFile)))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != ratchet {
		t.Errorf("the check rewrote the ratchet file:\n%s", written)
	}

	writeArtifactFile(t, output, detail, `<!doctype html><html><body><p>Ze answers <span data-ze-rfc="rfc4271:commands">490</span> commands.</p></body></html>`)
	check, err = checkPlainFigures(ratchetSource(t, `{"pages": [`+ratchetedPage(index)+`]}`), output)
	if err != nil {
		t.Fatal(err)
	}
	if len(check.Figures) != 0 {
		t.Errorf("a listed page and a marked page were refused: %v", check.Figures)
	}
	if len(check.StaleRatchet) != 0 {
		t.Errorf("a listed page that still carries a plain figure was called stale: %v", check.StaleRatchet)
	}
	if (checkReport{PlainFigures: check.Figures, StaleRatchet: check.StaleRatchet}).exit() != 0 {
		t.Error("an artifact whose only plain figures sit on listed RFC pages must pass the site check")
	}

	refusals := []struct {
		name    string
		ratchet string
		names   string
	}{
		{"a page outside the RFC section", `{"pages": [` + ratchetedPage("quality/index.html") + `]}`, "quality/index.html"},
		{"the same page twice", `{"pages": [` + ratchetedPage(index) + `, ` + ratchetedPage(index) + `]}`, index},
		{"a page with no reason", `{"pages": [{"page": "` + index + `", "reason": ""}]}`, index},
		{"a field the list does not hold", `{"pages": [], "exempt": ["quality/index.html"]}`, "exempt"},
	}
	for _, refusal := range refusals {
		t.Run(refusal.name, func(t *testing.T) {
			_, err := checkPlainFigures(ratchetSource(t, refusal.ratchet), output)
			if err == nil {
				t.Fatalf("a ratchet listing %s was accepted", refusal.name)
			}
			if !strings.Contains(err.Error(), refusal.names) {
				t.Errorf("the refusal %q does not name %q", err, refusal.names)
			}
		})
	}
	_, err = checkPlainFigures(t.TempDir(), output)
	if err == nil {
		t.Fatal("a source with no ratchet file was accepted")
	}
	if !strings.Contains(err.Error(), plainFigureRatchetFile) {
		t.Errorf("the refusal %q does not name %s", err, plainFigureRatchetFile)
	}
}

// VALIDATES: AC-9. An element carrying data-ze-rfc="<stem>:<field>" is
// marked, the same as data-ze-stat: the figures inside it, nested markup and
// text attributes included, are not refused, on an RFC page and on any other.
//
// The method lays down the same two pages with and without the RFC marker and
// asks the check for each: the marked pair passes, and the unmarked pair names
// each figure, so the pass is the marker's doing.
func TestSiteCheckAcceptsTheRFCMarker(t *testing.T) {
	const (
		home   = "index.html"
		detail = "quality/rfc-compliance/rfc4271/index.html"
	)
	marked := t.TempDir()
	writeArtifactFile(t, marked, factsFile, plainFigureSnapshot)
	writeArtifactFile(t, marked, home, `<!doctype html><html><body>
<p><span data-ze-rfc="rfc4271:requirements">4,744</span> requirements.</p></body></html>`)
	writeArtifactFile(t, marked, detail, `<!doctype html><html><body>
<p><span data-ze-rfc="rfc4271:tests" title="23,700+ unit tests"><b>23,700+</b></span> unit tests.</p></body></html>`)
	check, err := checkPlainFigures(ratchetSource(t, noRatchetedPages), marked)
	if err != nil {
		t.Fatal(err)
	}
	if len(check.Figures) != 0 {
		t.Errorf("figures inside data-ze-rfc were refused: %v", check.Figures)
	}
	if (checkReport{PlainFigures: check.Figures, StaleRatchet: check.StaleRatchet}).exit() != 0 {
		t.Error("a page whose figures sit inside data-ze-rfc must pass the site check")
	}

	plain := t.TempDir()
	writeArtifactFile(t, plain, factsFile, plainFigureSnapshot)
	writeArtifactFile(t, plain, home, `<!doctype html><html><body>
<p><span data-ze-field="rfc4271:requirements">4,744</span> requirements.</p></body></html>`)
	writeArtifactFile(t, plain, detail, `<!doctype html><html><body>
<p><span title="23,700+ unit tests"><b>23,700+</b></span> unit tests.</p></body></html>`)
	check, err = checkPlainFigures(ratchetSource(t, noRatchetedPages), plain)
	if err != nil {
		t.Fatal(err)
	}
	want := []plainFigure{
		{Page: home, Figure: "4,744", Key: "rfc.requirements_display"},
		{Page: detail, Figure: "23,700+", Key: "tests.unit_display"},
		{Page: detail, Figure: "23,700+", Key: "tests.unit_display"},
	}
	if !slices.Equal(check.Figures, want) {
		t.Errorf("without the marker the check found\n%v\nwant\n%v", check.Figures, want)
	}
}
