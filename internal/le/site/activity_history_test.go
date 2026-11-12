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
// view stands outside a history marker, so the page script can refresh every
// size the page states and none goes stale.
//
// The method walks every text node and every text attribute of the rendered
// size view and refuses a digit outside an element marked data-ze-history; a
// positive control requires every marker to hold a figure.
//
// VALIDATES: AC-10: each size figure is a history-marked element.
func TestEveryActivitySizeFigureIsAHistoryMarkedValue(t *testing.T) {
	document := activityParse(t, activityRenderedBody(t, activityFixturePaths(t)))
	view := activityElementByID(t, document, "activity-size")
	markers := 0
	stack := []*xhtml.Node{view}
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
			t.Errorf("the size view states %q outside a history marker", strings.TrimSpace(node.Data))
		}
		if node.Type == xhtml.ElementNode {
			for _, attribute := range node.Attr {
				if figureAttribute(node.Data, attribute.Key) && strings.ContainsFunc(attribute.Val, unicode.IsDigit) {
					t.Errorf("the size view's %s %s states %q outside a history marker", node.Data, attribute.Key, attribute.Val)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			stack = append(stack, child)
		}
	}
	if markers < len(activityHistoryNewest) {
		t.Errorf("the size view carries %d markers, fewer than the %d figures it states", markers, len(activityHistoryNewest))
	}
}

// TestActivityPageKeepsCalendarAndCommitViews proves the calendar view keeps
// everything the page showed around the calendar, and that the tabs which
// leave it stand outside both views.
//
// VALIDATES: AC-10, R-12 and R-16: cells, metric switch, four cards, the note
// on added lines and the commits curve sit in the calendar view; the first card
// says its lines were added by commits; the tabs reach both views and sit in
// neither; the cumulative added-lines curve and the working-tree block are gone.
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
		"Lines added by commits in this window",
		"Added lines count additions, not net repository size.",
		`id="activity-commits-chart"`, `id="activity-commits-day"`, `id="activity-commits-readout"`,
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
