// Design: website/AI.md -- the commit calendar, measured once and drawn here
// Detail: activitystyle.go dresses the published page, activityslidestyle.go
// the deck embed, and activityscript.go drives both.
// Related: internal/le/repo/rewrite/activitymeasure.go measures the history.
package site

import (
	"encoding/json"
	"errors"
	"html"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	reporewrite "github.com/ze-software/ze/internal/le/repo/rewrite"
	sitehistory "github.com/ze-software/ze/internal/le/site/history"
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
// presentations.go draws the same widget as a standalone document a talk deck
// embeds, under the deck's own directory and behind no route.
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
//
// Repository size comes from the committed size history alone, which
// `./le site history update` writes: the build reads that file and never
// measures size. The calendar beside it is still measured from git here.
func renderActivityPage(paths Paths) ([]string, error) {
	history, err := sitehistory.ReadHistory(paths.Repository)
	if errors.Is(err, sitehistory.ErrNoHistory) {
		return nil, opererr.Wrap(sitehistory.ErrNoHistory,
			"the size history has not been recorded: "+sitehistory.HistoryFile+" does not exist",
			"the activity page publishes repository size from that file alone, and the site build never measures it",
			"./le site history update")
	}
	if err != nil {
		return nil, err
	}
	figures, err := newHistoryFigures(history)
	if err != nil {
		return nil, err
	}
	window, err := reporewrite.MeasureActivity(paths.Repository, reporewrite.ActivityDaysDefault, "", buildClock())
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	content, err := activityPageBody(&window, figures)
	if err != nil {
		return nil, err
	}

	shell := pageShell{
		Title: "Repository Size and Activity - Ze",
		Description: "How many lines each part of Ze's repository held on every recorded day, " +
			"read from the committed size history, beside a year of commits and added lines as a calendar heatmap.",
		Root:      activityRoot,
		Path:      activityDest,
		ExtraHead: activityStyle,
		Sidebar:   pageSidebar(activityRoot, activityDest, links),
	}
	page, err := figures.fillHTML(shell.render(content))
	if err != nil {
		return nil, err
	}
	body, err := extractMain(page)
	if err != nil {
		return nil, err
	}

	// The mirror is converted back from the page this build just wrote, after
	// the fill, so it states the size figures the page shows. The page has no
	// Markdown source, and a heatmap has no Markdown form, so what a reader of
	// index.md gets is the text the panels state: the newest recorded day, each
	// section's lines, the Go composition, the calendar's totals and its range.
	mirror, err := htmlToMarkdown(body, pageCanonicalURL(activityDest))
	if err != nil {
		return nil, err
	}
	if err := writePublishedPage(paths.Output, activityDest, page, mirror); err != nil {
		return nil, err
	}
	return []string{activityRoute}, nil
}

// activitySurface names which of the two renderings of one measurement a body
// is drawn for. The measurement, the grid and the numbers are the same on both;
// the shell around them, and the stylesheet that dresses them, are not.
//
// Zero is unspecified, so a caller that names no surface is caught rather than
// given the page by accident.
type activitySurface int

const (
	activitySurfaceUnspecified activitySurface = iota
	// activitySurfacePage is /project/activity/: the site shell, the hero, and
	// the light site stylesheet the rest of the website is drawn in.
	activitySurfacePage
	// activitySurfaceSlide is talks/<slug>/activity.html: one standalone dark
	// document sized to the iframe a deck shows it in, with no hero to spend
	// the slide's height on.
	activitySurfaceSlide
)

// activityPageBody renders the published page's body: the hero, then one
// widget holding the view tabs, the size view and the calendar view, then the
// script that draws the size view and the commits curve.
//
// The shipped markup shows both views, the size view first, and hides the
// tabs, so a reader with no JavaScript reads everything in order; the script
// shows the tabs and opens the size view. The size figures are history
// markers, which renderActivityPage fills after the shell is drawn.
func activityPageBody(window *reporewrite.ActivityWindow, figures historyFigures) (string, error) {
	size, err := activitySizeHTML(figures)
	if err != nil {
		return "", err
	}
	var body textbuf.Buffer
	body.Reset().Str(`            <section class="activity-page" aria-labelledby="activity-title">`).Byte('\n')
	body.Str(`                <div class="activity-hero journey-hero reveal">`).Byte('\n')
	body.Str(`                    <span class="activity-eyebrow journey-eyebrow">Git telemetry</span>`).Byte('\n')
	body.Str(`                    <h1 id="activity-title">Repository size and activity</h1>`).Byte('\n')
	body.Str(`                    <p>How many lines each part of the repository held on every recorded day, `).
		Str(`and a year of commits and added lines, both read from the repository's history.</p>`).Byte('\n')
	body.Str("                </div>\n")
	// The site script fades the widget in as the reader scrolls to it, which
	// is what the reveal class marks.
	body.Str(`                <div class="activity-widget reveal" aria-label="Repository size and commit activity">`).Byte('\n')
	body.Str(activityTabsHTML)
	body.Str(size)
	body.Str(activityBody(window, activitySurfacePage))
	body.Str("                </div>\n")
	body.Str("            </section>\n")
	body.Str(activityGrowthScript)
	return body.String(), nil
}

// activityBody renders what each surface draws from the calendar measurement.
//
// On the slide that is the whole document body: the widget with the summary
// cards, the calendar, the working tree's Go inventory and the tooltip. On the
// page it is the calendar view alone, which activityPageBody places after the
// size view inside the page's widget: the summary cards, the metric switch, the
// calendar and the commits curve, then the tooltip. Both close with the script
// that switches the cards and the grid between the two metrics.
func activityBody(window *reporewrite.ActivityWindow, surface activitySurface) string {
	var body textbuf.Buffer
	body.Reset()
	lines := activityLineMetric(window)
	switch surface {
	case activitySurfacePage:
		lines = activityPageLineMetric(window)
		body.Str(activityCalendarViewHTML(window, lines))
		body.Str(activityTooltipHTML)
	case activitySurfaceSlide:
		body.Str(`        <main class="activity-slide">`).Byte('\n')
		body.Str(activitySlideWidgetHTML(window))
		body.Str("        </main>\n")
	case activitySurfaceUnspecified:
		panic("BUG: site.activityBody: the caller named no activity surface")
	default:
		panic("BUG: site.activityBody: invalid activity surface")
	}
	body.Str(activityScriptHTML(lines, activityCommitMetric(window)))
	return body.String()
}

// activitySlideWidgetHTML renders the deck's widget: the summary cards, the
// calendar, the working tree's Go inventory and the tooltip. The slide
// carries neither the site's reveal script nor a scroll, so it does not claim
// the reveal class.
func activitySlideWidgetHTML(window *reporewrite.ActivityWindow) string {
	var out textbuf.Buffer
	out.Reset().Str(`                <div class="activity-widget" aria-label="Activity heatmap">`).Byte('\n')
	out.Str(activitySummaryHTML(activityLineMetric(window), activityRangeText(window), activityDaysShown(window), activitySurfaceSlide))
	out.Str(`                    <div class="dashboard-grid">`).Byte('\n')
	out.Str(`                        <div class="left-stack">`).Byte('\n')
	out.Str(activityChartHTML(window, activitySurfaceSlide))
	out.Str(activityGoPanelHTML(window.Go))
	out.Str("                        </div>\n")
	out.Str("                    </div>\n")
	out.Str(activityTooltipHTML)
	out.Str("                </div>\n")
	return out.String()
}

// activityCalendarViewHTML renders the page's calendar view: the four cards
// and the metric switch, the note on what added lines count, the calendar, and
// the cumulative commits curve drawn from the calendar's own cells. The cards
// and the switch live inside the view, so they show only with it, while the
// tabs that leave it sit outside.
func activityCalendarViewHTML(window *reporewrite.ActivityWindow, lines activityMetric) string {
	var out textbuf.Buffer
	out.Reset().Str(`<div class="activity-view" id="activity-calendar" role="tabpanel" aria-labelledby="activity-tab-calendar">`).Byte('\n')
	out.Str(activitySummaryHTML(lines, activityRangeText(window), activityDaysShown(window), activitySurfacePage))
	out.Str(`    <p class="activity-note">Added lines count additions, not net repository size.</p>`).Byte('\n')
	out.Str(`                    <div class="dashboard-grid">`).Byte('\n')
	out.Str(`                        <div class="left-stack">`).Byte('\n')
	out.Str(activityChartHTML(window, activitySurfacePage))
	out.Str("                        </div>\n")
	out.Str("                    </div>\n")
	out.Str(activityCommitsCurveHTML)
	out.Str("</div>\n")
	return out.String()
}

// activityPageLineMetric labels the added-line series on the published page,
// where the size view beside it states the repository's size: the first card
// says the lines were added by commits in the window, so it never reads as a
// size. The slide keeps activityLineMetric's labels.
func activityPageLineMetric(window *reporewrite.ActivityWindow) activityMetric {
	metric := activityLineMetric(window)
	metric.TotalLabel = "Lines added by commits in this window"
	return metric
}

// activityLineMetric labels the added-line series, which the calendar opens on.
func activityLineMetric(window *reporewrite.ActivityWindow) activityMetric {
	return activityMetric{
		TotalLabel: "Total added lines", TotalValue: groupThousands(window.Lines.Total),
		ActiveLabel: "Days with added lines", ActiveValue: groupThousands(window.Lines.ActiveDays),
		PeakLabel: "Peak line day", PeakValue: groupThousands(window.Lines.Peak),
		ThresholdLabel: "Line thresholds", ThresholdValue: activityThresholdText(window.Lines),
	}
}

// activityCommitMetric labels the commit series, which the metric switch moves
// the same four cards onto.
func activityCommitMetric(window *reporewrite.ActivityWindow) activityMetric {
	return activityMetric{
		TotalLabel: "Total commits", TotalValue: groupThousands(window.Commits.Total),
		ActiveLabel: "Days with commits", ActiveValue: groupThousands(window.Commits.ActiveDays),
		PeakLabel: "Peak commit day", PeakValue: groupThousands(window.Commits.Peak),
		ThresholdLabel: "Commit thresholds", ThresholdValue: activityThresholdText(window.Commits),
	}
}

// activityThresholdText lists the four heat-level boundaries the way the page
// prints every other number on it.
func activityThresholdText(series reporewrite.ActivitySeries) string {
	text := make([]string, len(series.Thresholds))
	for index, value := range series.Thresholds {
		text[index] = groupThousands(value)
	}
	return strings.Join(text, ", ")
}

// activityRangeText names the measured window the way the pill beside the
// metric switch shows it.
func activityRangeText(window *reporewrite.ActivityWindow) string {
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
func activityDaysShown(window *reporewrite.ActivityWindow) int {
	const day = 24 * time.Hour
	return int(window.End.Sub(window.Start)/day) + 1
}

// activitySummaryHTML renders the four cards and the metric switch. The slide
// places the window's range beside the switch; the page places it under the
// calendar.
//
// The cards carry the ids the script writes into, and they open on the values
// of the added-line metric, so a reader with no JavaScript still reads a
// complete summary of the grid the page draws.
func activitySummaryHTML(lines activityMetric, rangeText string, days int, surface activitySurface) string {
	var out textbuf.Buffer
	out.Reset().Str(`<section class="stats" aria-label="Summary">`).Byte('\n')
	out.Str(activityStatHTML("total", lines.TotalLabel, lines.TotalValue))
	out.Str(activityStatHTML("active", lines.ActiveLabel, lines.ActiveValue))
	out.Str(activityStatHTML("peak", lines.PeakLabel, lines.PeakValue))
	out.Str(`        <div class="stat"><span>Days shown</span><strong>`).
		Str(groupThousands(days)).Str("</strong></div>\n")
	out.Str(`        <div class="metric-control">`).Byte('\n')
	out.Str(`            <div class="metric-switch" aria-label="Activity metric">`).Byte('\n')
	out.Str(`                <button type="button" data-metric="lines" aria-pressed="true">Added Lines</button>`).Byte('\n')
	out.Str(`                <button type="button" data-metric="commits" aria-pressed="false">Commits</button>`).Byte('\n')
	out.Str("            </div>\n")
	if surface == activitySurfaceSlide {
		out.Str(`            <div class="pill">`).Str(html.EscapeString(rangeText)).Str("</div>\n")
	}
	out.Str("        </div>\n")
	out.Str("    </section>\n")
	return out.String()
}

// activityStatHTML renders one summary card. The name is the id prefix the
// script addresses the card's two halves by.
func activityStatHTML(name, label, value string) string {
	return `        <div class="stat"><span id="` + name + `-label">` + html.EscapeString(label) +
		`</span><strong id="` + name + `-value">` + html.EscapeString(value) + "</strong></div>\n"
}

// activityChartHTML renders the calendar: the month row, the weekday column,
// the grid of day cells, and the level legend under them. The published page
// places the window's date range to the left of that legend.
//
// The grid is one image to a screen reader rather than a year of focusable
// squares with no shared meaning, so it states role="img" and a label naming
// the span it covers.
func activityChartHTML(window *reporewrite.ActivityWindow, surface activitySurface) string {
	var out textbuf.Buffer
	out.Reset().Str(`<div class="chart-scroll">`).Byte('\n')
	out.Str(`            <div class="chart">`).Byte('\n')
	out.Str(`                <div class="months" aria-hidden="true">`).Byte('\n')
	out.Str(window.MonthLabels)
	out.Str("                </div>\n")
	out.Str(`                <div class="chart-body">`).Byte('\n')
	out.Str(`                    <div class="weekday-labels" aria-hidden="true">`).
		Str(`<span></span><span>Mon</span><span></span><span>Wed</span><span></span>`).
		Str(`<span>Fri</span><span></span></div>`).Byte('\n')
	out.Str(`                    <div class="activity-grid" role="img" aria-label="Daily activity from `).
		Str(html.EscapeString(activityRangeText(window))).Str(`">`).Byte('\n')
	out.Str(window.Cells)
	out.Str("                    </div>\n")
	out.Str("                </div>\n")
	out.Str("            </div>\n")
	out.Str("        </div>\n")
	if surface == activitySurfacePage {
		out.Str(`<div class="activity-calendar-footer">`).Byte('\n')
		out.Str(`<div class="pill">`).Str(html.EscapeString(activityRangeText(window))).Str("</div>\n")
	}
	out.Str(activityLegendHTML)
	if surface == activitySurfacePage {
		out.Str("</div>\n")
	}
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
func activityGoPanelHTML(inventory reporewrite.ActivityGo) string {
	var out textbuf.Buffer
	out.Reset().Str(`<section class="panel go-panel" aria-label="Go code stats">`).Byte('\n')
	out.Str("        <h2>Go code composition</h2>\n")
	out.Str(`        <div class="go-breakdown">`).Byte('\n')
	out.Str(activityGoBucketHTML("Total Code", inventory.Total, ""))
	out.Str(activityGoBucketHTML("Production", inventory.Code, ""))
	out.Str(activityGoBucketHTML("Test", inventory.Tests, ""))
	out.Str(activityGoBucketHTML("Dependencies", inventory.Vendor,
		`                <div class="stat"><span>Modules</span><strong>`+
			groupThousands(inventory.VendorModules)+"</strong></div>\n"))
	out.Str("        </div>\n")
	out.Str("    </section>\n")
	return out.String()
}

// activityGoBucketHTML renders one bucket: five counts, then whatever extra
// card that bucket alone carries. Only the vendored bucket has one, the module
// count, so extraCard is empty for the other three.
func activityGoBucketHTML(title string, bucket reporewrite.ActivityGoBucket, extraCard string) string {
	var out textbuf.Buffer
	out.Reset().Str(`<div class="go-bucket">`).Byte('\n')
	out.Str("            <h3>").Str(html.EscapeString(title)).Str("</h3>\n")
	out.Str(`            <div class="go-stats">`).Byte('\n')
	out.Str(activityGoStatHTML("Files", bucket.Files))
	out.Str(activityGoStatHTML("Total lines", bucket.TotalLines))
	out.Str(activityGoStatHTML("Code", bucket.CodeLines))
	out.Str(activityGoStatHTML("Blank", bucket.BlankLines))
	out.Str(activityGoStatHTML("Comments", bucket.CommentLines))
	out.Str(extraCard)
	out.Str("            </div>\n")
	out.Str("        </div>\n")
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
