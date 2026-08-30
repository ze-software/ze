// Design: website/AI.md -- the commit calendar, measured once and drawn here
// Detail: activitystyle.go and activityscript.go hold the page's own assets.
// Related: internal/le/sourcerewrite/activitymeasure.go measures the history.
package site

import (
	"encoding/json"
	"html"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/sourcerewrite"
)

// The activity page registers from here. A build discovers it through the
// registry rather than through a call it states by name.
func init() {
	registerProducer(Producer{Name: activityProducer, Render: renderActivityPage})
}

// Where the activity page publishes, and the name it claims its route under.
//
// The producer is named for the page rather than for its directory, because
// this package already renders a second activity surface: renderActivity in
// presentations.go writes the table a talk deck embeds, under the deck's own
// directory and behind no route.
const (
	activityProducer  = "activity"
	activityDirectory = "project/activity"
	activityDest      = activityDirectory + "/" + pageIndexFile
	activityRoot      = "../../"
	activityRoute     = "/" + activityDirectory + "/"
)

// activityMetric is one series as the page's summary cards and its script read
// it: four labeled strings, and nothing the reader cannot see.
//
// The standalone dashboard names the peak day inside its own peak label. This
// page does not, because its grid already shows which square the peak is.
type activityMetric struct {
	TotalLabel     string `json:"totalLabel"`
	TotalValue     string `json:"totalValue"`
	ActiveLabel    string `json:"activeLabel"`
	ActiveValue    string `json:"activeValue"`
	PeakLabel      string `json:"peakLabel"`
	PeakValue      string `json:"peakValue"`
	ThresholdLabel string `json:"thresholdLabel"`
	ThresholdValue string `json:"thresholdValue"`
}

// renderActivityPage publishes the development-activity page and its mirror.
//
// The window ends on the DAY buildClock answers and the page states no time of
// day, so a second build over an unchanged history writes the same bytes. The
// clock is the seam the footer stamp already reads, so the two cannot disagree
// about the date, and a test fixes both at once.
func renderActivityPage(paths Paths) ([]string, error) {
	window, err := sourcerewrite.MeasureActivity(paths.Repository, buildClock())
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}

	shell := pageShell{
		Title: "Development Activity - Ze",
		Description: "A year of Ze's commit and added-line history, visualized as a " +
			"calendar heatmap. Live data, regenerated from git history.",
		Root:      activityRoot,
		Path:      activityDest,
		ExtraHead: activityStyle,
		Sidebar:   pageSidebar(activityRoot, activityDest, links),
	}
	page := shell.render(activityBody(&window))
	body, err := extractMain(page)
	if err != nil {
		return nil, err
	}

	// The mirror is converted back from the page this build just wrote. The
	// page has no Markdown source, and a heatmap has no Markdown form, so what
	// a reader of index.md gets is the text the panels beside the grid state:
	// the totals, the range and the Go inventory.
	mirror, err := htmlToMarkdown(body, pageCanonicalURL(activityDest))
	if err != nil {
		return nil, err
	}
	if err := writePublishedPage(paths.Output, activityDest, page, mirror); err != nil {
		return nil, err
	}
	return []string{activityRoute}, nil
}

// activityBody renders the page under <main>: the hero, then the widget that
// holds the summary cards, the calendar, the Go inventory and the tooltip.
func activityBody(window *sourcerewrite.ActivityWindow) string {
	lines := activityLineMetric(window)
	commits := activityCommitMetric(window)

	var body strings.Builder
	body.WriteString(`            <section class="activity-page" aria-labelledby="activity-title">` + "\n")
	body.WriteString(`                <div class="activity-hero journey-hero reveal">` + "\n")
	body.WriteString(`                    <span class="activity-eyebrow journey-eyebrow">Git telemetry</span>` + "\n")
	body.WriteString(`                    <h1 id="activity-title">Development activity</h1>` + "\n")
	body.WriteString(`                    <p>A year of commits, added lines, and Go composition ` +
		`regenerated from the repository.</p>` + "\n")
	body.WriteString("                </div>\n")
	body.WriteString(`                <div class="activity-widget reveal" aria-label="Activity heatmap">` + "\n")
	body.WriteString(activitySummaryHTML(lines, activityRangeText(window), activityDaysShown(window)))
	body.WriteString(`                    <div class="dashboard-grid">` + "\n")
	body.WriteString(`                        <div class="left-stack">` + "\n")
	body.WriteString(activityChartHTML(window))
	body.WriteString(activityGoPanelHTML(window.Go))
	body.WriteString("                        </div>\n")
	body.WriteString("                    </div>\n")
	body.WriteString(activityTooltipHTML)
	body.WriteString("                </div>\n")
	body.WriteString("            </section>\n")
	body.WriteString(activityScriptHTML(lines, commits))
	return body.String()
}

// activityLineMetric labels the added-line series, which the page opens on.
func activityLineMetric(window *sourcerewrite.ActivityWindow) activityMetric {
	return activityMetric{
		TotalLabel: "Total added lines", TotalValue: groupThousands(window.Lines.Total),
		ActiveLabel: "Days with added lines", ActiveValue: groupThousands(window.Lines.ActiveDays),
		PeakLabel: "Peak line day", PeakValue: groupThousands(window.Lines.Peak),
		ThresholdLabel: "Line thresholds", ThresholdValue: activityThresholdText(window.Lines),
	}
}

// activityCommitMetric labels the commit series, which the metric switch moves
// the same four cards onto.
func activityCommitMetric(window *sourcerewrite.ActivityWindow) activityMetric {
	return activityMetric{
		TotalLabel: "Total commits", TotalValue: groupThousands(window.Commits.Total),
		ActiveLabel: "Days with commits", ActiveValue: groupThousands(window.Commits.ActiveDays),
		PeakLabel: "Peak commit day", PeakValue: groupThousands(window.Commits.Peak),
		ThresholdLabel: "Commit thresholds", ThresholdValue: activityThresholdText(window.Commits),
	}
}

// activityThresholdText lists the four heat-level boundaries the way the page
// prints every other number on it.
func activityThresholdText(series sourcerewrite.ActivitySeries) string {
	text := make([]string, len(series.Thresholds))
	for index, value := range series.Thresholds {
		text[index] = groupThousands(value)
	}
	return strings.Join(text, ", ")
}

// activityRangeText names the measured window the way the pill beside the
// metric switch shows it.
func activityRangeText(window *sourcerewrite.ActivityWindow) string {
	const dayLayout = "2006-01-02"
	return window.Start.Format(dayLayout) + " to " + window.End.Format(dayLayout)
}

// activityDaysShown counts the measured days the way the summary card states
// them.
//
// It is derived from the window rather than restated as a constant here,
// because the measurement chooses the span: a number written twice is a page
// that can disagree with the grid beside it. Both bounds are inclusive, so the
// count is one more than the days between them.
func activityDaysShown(window *sourcerewrite.ActivityWindow) int {
	const day = 24 * time.Hour
	return int(window.End.Sub(window.Start)/day) + 1
}

// activitySummaryHTML renders the four cards and the metric switch.
//
// The cards carry the ids the script writes into, and they open on the values
// of the added-line metric, so a reader with no JavaScript still reads a
// complete summary of the grid the page draws.
func activitySummaryHTML(lines activityMetric, rangeText string, days int) string {
	var out strings.Builder
	out.WriteString(`<section class="stats" aria-label="Summary">` + "\n")
	out.WriteString(activityStatHTML("total", lines.TotalLabel, lines.TotalValue))
	out.WriteString(activityStatHTML("active", lines.ActiveLabel, lines.ActiveValue))
	out.WriteString(activityStatHTML("peak", lines.PeakLabel, lines.PeakValue))
	out.WriteString(`        <div class="stat"><span>Days shown</span><strong>` +
		groupThousands(days) + "</strong></div>\n")
	out.WriteString(`        <div class="metric-control">` + "\n")
	out.WriteString(`            <div class="metric-switch" aria-label="Activity metric">` + "\n")
	out.WriteString(`                <button type="button" data-metric="lines" aria-pressed="true">Added Lines</button>` + "\n")
	out.WriteString(`                <button type="button" data-metric="commits" aria-pressed="false">Commits</button>` + "\n")
	out.WriteString("            </div>\n")
	out.WriteString(`            <div class="pill">` + html.EscapeString(rangeText) + "</div>\n")
	out.WriteString("        </div>\n")
	out.WriteString("    </section>\n")
	return out.String()
}

// activityStatHTML renders one summary card. The name is the id prefix the
// script addresses the card's two halves by.
func activityStatHTML(name, label, value string) string {
	return `        <div class="stat"><span id="` + name + `-label">` + html.EscapeString(label) +
		`</span><strong id="` + name + `-value">` + html.EscapeString(value) + "</strong></div>\n"
}

// activityChartHTML renders the calendar: the month row, the weekday column,
// the grid of day cells, and the level legend under them.
//
// The grid is one image to a screen reader rather than a year of focusable
// squares with no shared meaning, so it states role="img" and a label naming
// the span it covers.
func activityChartHTML(window *sourcerewrite.ActivityWindow) string {
	var out strings.Builder
	out.WriteString(`<div class="chart-scroll">` + "\n")
	out.WriteString(`            <div class="chart">` + "\n")
	out.WriteString(`                <div class="months" aria-hidden="true">` + "\n")
	out.WriteString(window.MonthLabels)
	out.WriteString("                </div>\n")
	out.WriteString(`                <div class="chart-body">` + "\n")
	out.WriteString(`                    <div class="weekday-labels" aria-hidden="true">` +
		`<span></span><span>Mon</span><span></span><span>Wed</span><span></span>` +
		`<span>Fri</span><span></span></div>` + "\n")
	out.WriteString(`                    <div class="activity-grid" role="img" aria-label="Daily activity from ` +
		html.EscapeString(activityRangeText(window)) + `">` + "\n")
	out.WriteString(window.Cells)
	out.WriteString("                    </div>\n")
	out.WriteString("                </div>\n")
	out.WriteString("            </div>\n")
	out.WriteString("        </div>\n")
	out.WriteString(activityLegendHTML)
	return out.String()
}

// activityLegendHTML is the five-step color key under the grid. Its squares
// are decoration beside the words that bound them, so they carry no data.
const activityLegendHTML = `        <div class="legend"><span>Less</span>` +
	`<span class="day-cell" data-level="0"></span><span class="day-cell" data-level="1"></span>` +
	`<span class="day-cell" data-level="2"></span><span class="day-cell" data-level="3"></span>` +
	`<span class="day-cell" data-level="4"></span><span class="day-cell" data-level="5"></span>` +
	`<span>More</span></div>` + "\n"

// activityGoPanelHTML renders the four buckets of the Go source inventory.
func activityGoPanelHTML(inventory sourcerewrite.ActivityGo) string {
	var out strings.Builder
	out.WriteString(`<section class="panel go-panel" aria-label="Go code stats">` + "\n")
	out.WriteString("        <h2>Go code composition</h2>\n")
	out.WriteString(`        <div class="go-breakdown">` + "\n")
	out.WriteString(activityGoBucketHTML("Total Code", inventory.Total, ""))
	out.WriteString(activityGoBucketHTML("Production", inventory.Code, ""))
	out.WriteString(activityGoBucketHTML("Test", inventory.Tests, ""))
	out.WriteString(activityGoBucketHTML("Dependencies", inventory.Vendor,
		`                <div class="stat"><span>Modules</span><strong>`+
			groupThousands(inventory.VendorModules)+"</strong></div>\n"))
	out.WriteString("        </div>\n")
	out.WriteString("    </section>\n")
	return out.String()
}

// activityGoBucketHTML renders one bucket: five counts, then whatever extra
// card that bucket alone carries. Only the vendored bucket has one, the module
// count, so extraCard is empty for the other three.
func activityGoBucketHTML(title string, bucket sourcerewrite.ActivityGoBucket, extraCard string) string {
	var out strings.Builder
	out.WriteString(`<div class="go-bucket">` + "\n")
	out.WriteString("            <h3>" + html.EscapeString(title) + "</h3>\n")
	out.WriteString(`            <div class="go-stats">` + "\n")
	out.WriteString(activityGoStatHTML("Files", bucket.Files))
	out.WriteString(activityGoStatHTML("Total lines", bucket.TotalLines))
	out.WriteString(activityGoStatHTML("Code", bucket.CodeLines))
	out.WriteString(activityGoStatHTML("Blank", bucket.BlankLines))
	out.WriteString(activityGoStatHTML("Comments", bucket.CommentLines))
	out.WriteString(extraCard)
	out.WriteString("            </div>\n")
	out.WriteString("        </div>\n")
	return out.String()
}

// activityGoStatHTML renders one count of one bucket.
func activityGoStatHTML(label string, count int) string {
	return `<div class="stat"><span>` + html.EscapeString(label) + "</span><strong>" +
		groupThousands(count) + "</strong></div>\n"
}

// activityTooltipHTML is the hover and focus readout. It ships hidden and
// empty: the script fills it from the cell the reader is on, so a reader with
// no JavaScript meets nothing rather than an empty box.
const activityTooltipHTML = `<div id="activity-tooltip" class="activity-tooltip" role="tooltip" hidden>
    <span class="tooltip-date"></span>
    <div class="tooltip-gap"></div>
    <span class="tooltip-value tooltip-primary"></span>
    <span class="tooltip-value tooltip-secondary"></span>
</div>
`

// activityScriptHTML writes this page's two metrics, then the script that reads
// them.
//
// The numbers are the page's own and the code is the same on every build, so
// the code is a constant and only the numbers are composed here. The encoder is
// given the same escaping the config browser's payload takes, because a
// "</script>" inside the JSON would close the element early.
func activityScriptHTML(lines, commits activityMetric) string {
	payload, err := json.Marshal(map[string]activityMetric{"lines": lines, "commits": commits})
	if err != nil {
		// Every field of activityMetric is a string, so the encoder has no
		// value it can refuse. A failure here is a Ze defect rather than an
		// operating error.
		panic("BUG: site.activityScriptHTML: the metric summaries do not encode: " + err.Error())
	}
	return "<script>\nconst metricSummaries = " + escapeEmbeddedJSON(payload) + ";\n" + activityPageScript
}
