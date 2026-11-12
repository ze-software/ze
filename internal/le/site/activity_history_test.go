// Design: docs/architecture/site-facts.md -- the activity page's size view reads the committed size history
// Related: activityhistory.go -- the marker and the fill these tests drive
package site

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	xhtml "golang.org/x/net/html"

	sitehistory "github.com/ze-software/ze/internal/le/site/history"
)

// activityHistoryNewest is what the fixture history states on its newest
// recorded day, counted by hand from testdata/repo-history.json: each marker
// the size view carries, and the text the build must write inside it.
var activityHistoryNewest = map[string]string{
	"day:date":                       "2026-08-23",
	"day:modules":                    "3",
	"total:lines":                    "2,344",
	"third-party-go:lines":           "650",
	"go-tests:lines":                 "175",
	"go-production:lines":            "190",
	"plans:lines":                    "95",
	"other:lines":                    "1,234",
	"go-production+go-tests:files":   "10",
	"go-production+go-tests:lines":   "365",
	"go-production+go-tests:code":    "276",
	"go-production+go-tests:blank":   "46",
	"go-production+go-tests:comment": "43",
	"go-production:files":            "4",
	"go-production:code":             "148",
	"go-production:blank":            "21",
	"go-production:comment":          "21",
	"go-tests:files":                 "6",
	"go-tests:code":                  "128",
	"go-tests:blank":                 "25",
	"go-tests:comment":               "22",
	"third-party-go:files":           "5",
	"third-party-go:code":            "520",
	"third-party-go:blank":           "65",
	"third-party-go:comment":         "65",

	// All Go, third-party included: the three Go sections of the newest day,
	// 650 + 175 + 190, summed by hand.
	"third-party-go+go-tests+go-production:lines": "1,015",
}

// writeActivityHistoryFixture lays the hand-written size history down where
// renderActivityPage reads it: the history file under repository. It is the
// fixture every activity page test builds on, so no site test measures size
// from git.
func writeActivityHistoryFixture(t *testing.T, repository string) {
	t.Helper()
	path := filepath.Join(repository, filepath.FromSlash(sitehistory.HistoryFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(readFixture(t, "repo-history.json")), 0o600); err != nil {
		t.Fatal(err)
	}
}

// activityHistoryFigures reads the fixture history through the tool's own
// reader and answers its newest day's figures.
func activityHistoryFigures(t *testing.T) historyFigures {
	t.Helper()
	root := t.TempDir()
	writeActivityHistoryFixture(t, root)
	history, err := sitehistory.ReadHistory(root)
	if err != nil {
		t.Fatalf("the fixture history does not read: %v", err)
	}
	figures, err := newHistoryFigures(history)
	if err != nil {
		t.Fatal(err)
	}
	return figures
}

// activityElementByID answers the element of document whose id is id.
func activityElementByID(t *testing.T, document *xhtml.Node, id string) *xhtml.Node {
	t.Helper()
	stack := []*xhtml.Node{document}
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Type == xhtml.ElementNode && activityAttribute(node, "id") == id {
			return node
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			stack = append(stack, child)
		}
	}
	t.Fatalf("the page holds no element with id %q", id)
	return nil
}

// activityAttribute answers the value of node's attribute key, or "".
func activityAttribute(node *xhtml.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

// activityWithin reports whether node lies inside ancestor.
func activityWithin(node, ancestor *xhtml.Node) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent == ancestor {
			return true
		}
	}
	return false
}

// activityOuterHTML renders one element as the page carries it.
func activityOuterHTML(t *testing.T, node *xhtml.Node) string {
	t.Helper()
	var out bytes.Buffer
	if err := xhtml.Render(&out, node); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// activityParse parses a rendered body.
func activityParse(t *testing.T, body string) *xhtml.Node {
	t.Helper()
	document, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return document
}

// TestActivityPagePublishesSizeHistory proves the registered producer builds
// the size view from the committed file, the way a build does: over a source
// tree that holds the history, staged and trimmed as Build stages it.
//
// VALIDATES: AC-10 and AC-12: the size view comes first, each marked figure
// states the fixture's newest day, the artifact carries data/repo-history.json
// byte for byte, and the mirror names every section with its lines.
func TestActivityPagePublishesSizeHistory(t *testing.T) {
	paths := activityFixturePaths(t)
	// The sources are the fixture repository's own website directory, so the
	// file the producer reads is the file the staging publishes.
	paths.Source = filepath.Join(paths.Repository, "website")
	files, err := trackedAndUntrackedSourceFiles(paths)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageSources(paths.Source, paths.Output, files); err != nil {
		t.Fatal(err)
	}
	var routes []string
	for _, producer := range allProducers() {
		if producer.Name != activityProducer {
			continue
		}
		routes, err = producer.Render(paths)
		if err != nil {
			t.Fatalf("the registered activity producer refused: %v", err)
		}
	}
	if !slices.Equal(routes, []string{activityRoute}) {
		t.Fatalf("the registered activity producer answered %v, want [%s]", routes, activityRoute)
	}
	if err := removeSourceOnly(paths.Output); err != nil {
		t.Fatal(err)
	}

	published, err := os.ReadFile(filepath.Join(paths.Output, filepath.FromSlash(historyDataFile)))
	if err != nil {
		t.Fatalf("the artifact does not carry %s: %v", historyDataFile, err)
	}
	if string(published) != readFixture(t, "repo-history.json") {
		t.Errorf("the artifact's %s is not the committed file", historyDataFile)
	}
	if slices.Contains(checkNamedArtifacts(paths.Output), historyDataFile) {
		t.Errorf("the artifact check reports %s missing", historyDataFile)
	}

	raw, err := os.ReadFile(filepath.Join(paths.Output, filepath.FromSlash(activityDest)))
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	size, calendar := strings.Index(page, `id="activity-size"`), strings.Index(page, `id="activity-calendar"`)
	if size < 0 || calendar < 0 || calendar < size {
		t.Errorf("the size view must precede the calendar view: size at %d, calendar at %d", size, calendar)
	}
	for mark, want := range activityHistoryNewest {
		if !strings.Contains(page, `<span data-ze-history="`+mark+`">`+want+`</span>`) {
			t.Errorf("the page does not state %s as %q", mark, want)
		}
	}
	if !strings.Contains(page, `data-history="`+activityRoot+historyDataFile+`"`) {
		t.Errorf("the size view does not name the published file %s", historyDataFile)
	}

	mirror, err := os.ReadFile(filepath.Join(paths.Output, activityDirectory, pageMirrorFile))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(mirror)), " ")
	for _, want := range []string{
		"2,344", "2026-08-23",
		"Third-party Go:** 650", "Go tests:** 175", "Go production:** 190", "Plans:** 95", "Other:** 1,234",
		"All Go files, third-party included, hold **1,015** lines.",
		strings.Join(strings.Fields(activitySizeMeasureText), " "),
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the mirror does not state %q", want)
		}
	}
}

// TestActivityPageRefusesMissingHistory proves a build over a checkout with no
// size history refuses, rather than publishing a page with no size or
// measuring one.
//
// VALIDATES: AC-12: the refusal says the history has not been recorded and
// ends with the command that records it; no page is written.
func TestActivityPageRefusesMissingHistory(t *testing.T) {
	paths := activityFixturePaths(t)
	if err := os.Remove(filepath.Join(paths.Repository, filepath.FromSlash(sitehistory.HistoryFile))); err != nil {
		t.Fatal(err)
	}
	_, err := renderActivityPage(paths)
	if err == nil {
		t.Fatal("the activity page built with no size history")
	}
	if !errors.Is(err, sitehistory.ErrNoHistory) {
		t.Errorf("the refusal does not carry ErrNoHistory: %v", err)
	}
	message := err.Error()
	if !strings.Contains(message, "has not been recorded") {
		t.Errorf("the refusal does not say the history has not been recorded: %s", message)
	}
	if !strings.HasSuffix(message, "./le site history update") {
		t.Errorf("the refusal does not end with ./le site history update: %s", message)
	}
	if _, statErr := os.Stat(filepath.Join(paths.Output, filepath.FromSlash(activityDest))); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a refused build still wrote the page: %v", statErr)
	}
}

// TestEveryActivitySizeFigureIsAHistoryMarkedValue proves no number in the size
// view or in the Go composition stands outside a history marker, so the page
// script can refresh every size the page states and none goes stale.
//
// The method walks every text node and every text attribute of the rendered
// size view and of the Go composition, which sits in the calendar view, and
// refuses a digit outside an element marked data-ze-history; a positive
// control requires every marker to hold a figure, and the two together to
// carry at least one marker for each figure the fixture's newest day states.
//
// VALIDATES: AC-10: each size figure is a history-marked element.
func TestEveryActivitySizeFigureIsAHistoryMarkedValue(t *testing.T) {
	document := activityParse(t, activityRenderedBody(t, activityFixturePaths(t)))
	views := []*xhtml.Node{activityElementByID(t, document, "activity-size"), activityElementByClass(t, document, "go-panel")}
	markers := 0
	stack := views
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Type == xhtml.ElementNode && activityAttribute(node, "data-ze-history") != "" {
			markers++
			if node.FirstChild == nil || strings.TrimSpace(node.FirstChild.Data) == "" {
				t.Errorf("the marker %s holds no figure", activityAttribute(node, "data-ze-history"))
			}
			continue
		}
		if node.Type == xhtml.TextNode && strings.ContainsFunc(node.Data, unicode.IsDigit) {
			t.Errorf("a size figure %q stands outside a history marker", strings.TrimSpace(node.Data))
		}
		if node.Type == xhtml.ElementNode {
			for _, attribute := range node.Attr {
				if figureAttribute(node.Data, attribute.Key) && strings.ContainsFunc(attribute.Val, unicode.IsDigit) {
					t.Errorf("the %s %s states %q outside a history marker", node.Data, attribute.Key, attribute.Val)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			stack = append(stack, child)
		}
	}
	if markers < len(activityHistoryNewest) {
		t.Errorf("the size view and the Go composition carry %d markers, fewer than the %d figures they state", markers, len(activityHistoryNewest))
	}
}

// TestActivityPageKeepsCalendarAndCommitViews proves the calendar view keeps
// everything the page showed around the calendar, and that the tabs which
// leave it stand outside both views.
//
// VALIDATES: AC-10, R-12 and R-16: cells, metric switch, four cards, the note
// on added lines, the Go composition and the cumulative curve sit in the
// calendar view; the first card says its lines were added by commits; the
// tabs reach both views and sit in neither; the cumulative added-lines curve
// of the old growth view and the working-tree block are gone.
func TestActivityPageKeepsCalendarAndCommitViews(t *testing.T) {
	body := activityRenderedBody(t, activityFixturePaths(t))
	document := activityParse(t, body)
	calendar := activityElementByID(t, document, "activity-calendar")
	size := activityElementByID(t, document, "activity-size")
	view := activityOuterHTML(t, calendar)
	for _, want := range []string{
		`class="activity-grid"`, `class="day-cell`,
		`data-metric="lines"`, `data-metric="commits"`,
		`id="total-value"`, `id="active-value"`, `id="peak-value"`, "Days shown",
		"Total commits",
		"The lines count every file committed outside the third-party trees",
		"added 4 lines and deleted 0 lines, a net change of 4 lines",
		`class="panel go-panel"`,
		`id="activity-cumulative-chart"`, `id="activity-cumulative-day"`, `id="activity-cumulative-readout"`,
		`class="legend"`,
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the calendar view does not carry %s", want)
		}
	}
	for _, gone := range []string{"activity-growth-lines-chart", "Cumulative added lines", "Historical category totals", "Total added lines"} {
		if strings.Contains(body, gone) {
			t.Errorf("the page still carries %q", gone)
		}
	}
	for _, id := range []string{"activity-tab-size", "activity-tab-calendar"} {
		tab := activityElementByID(t, document, id)
		if activityWithin(tab, calendar) || activityWithin(tab, size) {
			t.Errorf("the tab %s sits inside a view, so hiding that view would strand the reader", id)
		}
		if target := activityAttribute(tab, "aria-controls"); target != "activity-size" && target != "activity-calendar" {
			t.Errorf("the tab %s controls %q, not one of the two views", id, target)
		}
	}
	if activityAttribute(activityElementByID(t, document, "activity-tab-size"), "aria-selected") != "true" {
		t.Error("the size tab is not the one the page opens on")
	}
	if _, hidden := activityAttributeSet(calendar, "hidden"); hidden {
		t.Error("the calendar view ships hidden, so a reader with no JavaScript cannot read it")
	}
	if _, hidden := activityAttributeSet(size, "hidden"); hidden {
		t.Error("the size view ships hidden, so a reader with no JavaScript cannot read it")
	}
	// The tabs ship as a bare hidden attribute, which the script removes; a
	// valued one such as hidden="false" still hides them in HTML.
	if value, hidden := activityAttributeSet(activityElementByID(t, document, "activity-tab-size").Parent, "hidden"); !hidden || value != "" {
		t.Errorf("the tabs must ship with a bare hidden attribute, so a reader with no JavaScript meets no dead controls; hidden=%q set=%v", value, hidden)
	}
}

// activityAttributeSet reports whether node carries attribute key at all.
func activityAttributeSet(node *xhtml.Node, key string) (string, bool) {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val, true
		}
	}
	return "", false
}

// TestTheActivityMirrorStartsEveryBlockOnItsOwnLine proves the Markdown mirror
// of the activity page keeps the calendar view's first card a bullet of its
// own, and holds no bullet or heading marker with nothing after it.
//
// The method renders the fixture page, reads the mirror the render wrote, as
// TestTheActivityMirrorReadsAsThePublishedMirror does, and reads its lines
// rather than its shape. The first card's label must open its line, after
// nothing but spaces, as "- Total commits": when the
// element before the calendar view ends in inline text (the size view's day
// selector), the card's bullet joins that text's line and stops being a list
// item. No line may be a bare "-", "*" or "+", or a run of "#" alone.
//
// VALIDATES: the mirror a reader with no browser gets carries every card of
// the calendar view as a bullet, and every heading and bullet it opens says
// something.
func TestTheActivityMirrorStartsEveryBlockOnItsOwnLine(t *testing.T) {
	paths := activityFixturePaths(t)
	if _, err := renderActivityPage(paths); err != nil {
		t.Fatalf("render: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(paths.Output, activityDirectory, pageMirrorFile))
	if err != nil {
		t.Fatal(err)
	}
	const firstCard = "- Total commits"
	label := strings.TrimPrefix(firstCard, "- ")
	opens, holder := false, ""
	for number, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " "), firstCard) {
			opens = true
		}
		if holder == "" && strings.Contains(line, label) {
			holder = line
		}
		marker := strings.TrimSpace(line)
		if marker == "-" || marker == "*" || marker == "+" {
			t.Errorf("mirror line %d is a bullet with nothing after its marker: %q", number+1, line)
		}
		if marker != "" && strings.Trim(marker, "#") == "" {
			t.Errorf("mirror line %d is a heading with nothing after its marker: %q", number+1, line)
		}
	}
	if !opens {
		t.Errorf("the first calendar card does not start its own line as %q; the line that holds its label is %q", firstCard, holder)
	}
}

// TestAllGoNamesEveryGoSection proves the all-Go figure sums every section the
// file splits into Go lines, and that a file with no such section refuses the
// panel rather than publishing a sum of nothing.
//
// VALIDATES: AC-10: the Go composition keeps the old page's all-Go total; the
// fixture's three Go sections, in the file's order, are the summed subject.
func TestAllGoNamesEveryGoSection(t *testing.T) {
	subject, err := activityHistoryFigures(t).goSubject()
	if err != nil {
		t.Fatal(err)
	}
	if subject != "third-party-go+go-tests+go-production" {
		t.Errorf("the all-Go subject is %q, want the fixture's three Go sections", subject)
	}
	noGo := historyFigures{sections: []sitehistory.Section{{ID: "plans", Label: "Plans"}}}
	if _, err := noGo.goSubject(); err == nil || !strings.HasSuffix(err.Error(), "./le site history update all") {
		t.Errorf("a file with no Go section did not refuse with the repair command: %v", err)
	}
	if _, err := activityHistoryGoPanelHTML(noGo); err == nil {
		t.Error("the Go composition rendered from a file with no Go section")
	}
}

// TestHistoryFillRefusesUnknownMarker proves the build refuses, by name, every
// history marker it cannot fill, so no size figure publishes empty.
//
// VALIDATES: AC-12: an unknown subject, an unknown field, a Go field on a
// section without the split, a day field the day lacks and a marker outside
// the helper's shape each refuse; a well-formed marker fills.
func TestHistoryFillRefusesUnknownMarker(t *testing.T) {
	figures := activityHistoryFigures(t)
	filled, err := figures.fillHTML("<p>" + historyValueHTML("total", "lines") + "</p>")
	if err != nil {
		t.Fatalf("a well-formed marker was refused: %v", err)
	}
	if filled != `<p><span data-ze-history="total:lines">2,344</span></p>` {
		t.Errorf("the fill wrote %s", filled)
	}
	for _, refused := range []struct {
		name, page string
		names      []string
	}{
		{"unknown section", historyValueHTML("no-such-section", "lines"), []string{"no-such-section"}},
		{"unknown part of a sum", historyValueHTML("go-tests+no-such-section", "lines"), []string{"no-such-section"}},
		{"unknown field", historyValueHTML("plans", "widgets"), []string{"plans:widgets", "widgets"}},
		{"Go field without the split", historyValueHTML("plans", "code"), []string{"plans:code", "Go split"}},
		{"Go field on the total", historyValueHTML("total", "blank"), []string{"total:blank"}},
		{"unknown day field", historyValueHTML("day", "size"), []string{"day:size"}},
		{"another attribute", `<span data-ze-history="total:lines" class="figure"></span>`, []string{`data-ze-history="total:lines" class="figure"`}},
		{"another tag", `<div data-ze-history="total:lines"></div>`, []string{`<div data-ze-history="total:lines">`}},
		{"text already inside", `<span data-ze-history="total:lines">7</span>`, []string{`data-ze-history="total:lines">7`}},
		{"no field", `<span data-ze-history="total"></span>`, []string{`data-ze-history="total"`}},
	} {
		_, err := figures.fillHTML("<p>" + refused.page + "</p>")
		if err == nil {
			t.Errorf("%s: the fill accepted %s", refused.name, refused.page)
			continue
		}
		for _, name := range refused.names {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("%s: the refusal does not name %q: %v", refused.name, name, err)
			}
		}
	}
}

// TestActivityGoCompositionSitsInTheCalendarView proves the Go composition
// sits in the Commit calendar view, after the calendar and before the
// cumulative curve, and not in the Repository size view, and that it still
// states the committed size history's newest day.
//
// The method parses the rendered body, finds the one panel by its class,
// checks its ancestors and its place in document order, then requires a
// marker of each kind the panel carries to hold the fixture's value.
//
// VALIDATES: the owner's correction that the Go composition belongs to the
// calendar view; AC-10: its figures stay history markers read from the file.
func TestActivityGoCompositionSitsInTheCalendarView(t *testing.T) {
	document := activityParse(t, activityRenderedBody(t, activityFixturePaths(t)))
	size := activityElementByID(t, document, "activity-size")
	calendar := activityElementByID(t, document, "activity-calendar")
	panel := activityElementByClass(t, document, "go-panel")
	if activityWithin(panel, size) {
		t.Error("the Go composition still sits in the size view")
	}
	if !activityWithin(panel, calendar) {
		t.Error("the Go composition does not sit in the calendar view")
	}
	order := activityDocumentOrder(document)
	grid := activityElementByClass(t, document, "activity-grid")
	curve := activityElementByClass(t, document, "activity-cumulative")
	if order[panel] < order[grid] {
		t.Error("the Go composition comes before the calendar, so a reader with no JavaScript meets it first")
	}
	if order[curve] < order[panel] {
		t.Error("the Go composition comes after the cumulative curve; it belongs between the calendar and the curve")
	}
	view := activityOuterHTML(t, panel)
	if !strings.Contains(view, "As committed on ") {
		t.Error("the Go composition no longer says which committed day it states")
	}
	for _, mark := range []string{
		"day:date", "day:modules",
		"third-party-go+go-tests+go-production:lines",
		"go-production+go-tests:files", "go-production:code", "go-tests:blank", "third-party-go:comment",
	} {
		want := `<span data-ze-history="` + mark + `">` + activityHistoryNewest[mark] + `</span>`
		if !strings.Contains(view, want) {
			t.Errorf("the Go composition does not state %s as %q", mark, activityHistoryNewest[mark])
		}
	}
}

// TestActivityCumulativeCurveCarriesBothWordings proves the cumulative curve
// ships able to say which of the switch's two metrics it shows. It opens on
// the Commits wording, the metric the cards open on, and every element whose
// words change carries the Added Lines wording beside its own, which names the
// net lines, added minus deleted; the chart's accessible name and the slider's
// unit carry both as well. No wording calls the lines a size.
//
// VALIDATES: the owner's correction that the curve follows the Added Lines
// and Commits switch; AC-11: commit activity stays labeled as activity.
func TestActivityCumulativeCurveCarriesBothWordings(t *testing.T) {
	document := activityParse(t, activityRenderedBody(t, activityFixturePaths(t)))
	curve := activityElementByClass(t, document, "activity-cumulative")
	if !activityWithin(curve, activityElementByID(t, document, "activity-calendar")) {
		t.Error("the cumulative curve does not sit in the calendar view")
	}
	wordings := map[string]string{}
	stack := []*xhtml.Node{curve}
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			stack = append(stack, child)
		}
		lines, carries := activityAttributeSet(node, "data-text-lines")
		if !carries {
			continue
		}
		commits := activityAttribute(node, "data-text-commits")
		wordings[lines] = commits
		text := node.FirstChild
		if text == nil || text.Type != xhtml.TextNode || text.NextSibling != nil || text.Data != commits {
			t.Errorf("the element worded %q under Commits does not ship stating it", commits)
		}
	}
	for lines, commits := range map[string]string{
		"Lines added minus lines deleted by commits, accumulated since the calendar's first day. " +
			"The curve falls on a day whose commits deleted more lines than they added. " +
			"A second line draws the same figure over every committed file, the third-party trees included.": "Commits accumulated since the calendar's first day.",
		"Net lines": "Commits",
	} {
		if wordings[lines] != commits {
			t.Errorf("the curve words %q as %q under Commits, want %q", lines, wordings[lines], commits)
		}
	}
	if len(wordings) != 2 {
		t.Errorf("the curve carries %d switchable wordings, want the sentence and the readout label: %v", len(wordings), wordings)
	}
	chart := activityElementByID(t, document, "activity-cumulative-chart")
	if label := activityAttribute(chart, "aria-label"); label != activityAttribute(chart, "data-aria-commits") {
		t.Errorf("the chart ships named %q, not by its Commits wording", label)
	}
	if !strings.HasPrefix(activityAttribute(chart, "data-aria-lines"), "Cumulative lines added minus lines deleted by commits.") {
		t.Errorf("the chart's Added Lines name is %q", activityAttribute(chart, "data-aria-lines"))
	}
	if !strings.HasPrefix(activityAttribute(chart, "data-aria-commits"), "Cumulative commits.") {
		t.Errorf("the chart's Commits name is %q", activityAttribute(chart, "data-aria-commits"))
	}
	slider := activityElementByID(t, document, "activity-cumulative-day")
	if activityAttribute(slider, "data-unit-lines") != "net lines" || activityAttribute(slider, "data-unit-commits") != "commits" {
		t.Errorf("the slider reads out %q and %q, want \"net lines\" and \"commits\"",
			activityAttribute(slider, "data-unit-lines"), activityAttribute(slider, "data-unit-commits"))
	}
	if strings.Contains(strings.ToLower(activityOuterHTML(t, curve)), "size") {
		t.Error("the cumulative curve's wording calls the added lines a size")
	}
}

// activityElementByClass answers the one element of document whose class list
// holds class, and refuses a document with none or with several.
func activityElementByClass(t *testing.T, document *xhtml.Node, class string) *xhtml.Node {
	t.Helper()
	var found []*xhtml.Node
	stack := []*xhtml.Node{document}
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Type == xhtml.ElementNode && slices.Contains(strings.Fields(activityAttribute(node, "class")), class) {
			found = append(found, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			stack = append(stack, child)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the page holds %d elements of class %q, want one", len(found), class)
	}
	return found[0]
}

// activityDocumentOrder numbers every node of document in the order a reader
// meets it, which is the order a page with no JavaScript shows it in.
func activityDocumentOrder(document *xhtml.Node) map[*xhtml.Node]int {
	order := map[*xhtml.Node]int{}
	stack := []*xhtml.Node{document}
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		order[node] = len(order)
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return order
}
