// Design: website/AI.md -- the activity page's size view, drawn from the committed size history
// Related: activityhistory.go -- the history markers these views carry and the fill that writes them
// Related: activity.go -- the calendar view the Go composition and the cumulative curve sit in
// Related: activitygrowthstyle.go -- places each date control's track by the plot bounds below
package site

import (
	"html"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The charts' horizontal coordinates, in viewBox units. Every chart is
// growthViewWidth wide and plots its first day at growthPlotLeft and its last
// at growthPlotRight; the space before growthPlotLeft holds the axis figures.
// The script places points and cursors by them, and the stylesheet places each
// date control's track by them, so the control's handle stands on the charts'
// cursor line on every day. They are written once here because both depend on
// the same numbers.
const (
	growthViewWidth = "720"
	growthPlotLeft  = "72"
	growthPlotRight = "704"
)

// activityTabsHTML switches the page between its two views. It ships hidden,
// because a reader with no JavaScript meets both views one after the other;
// the script shows it and opens the size view, which is why the size tab is
// the selected one.
const activityTabsHTML = `<div class="activity-tabs" role="tablist" aria-label="Activity view" hidden>
    <button type="button" id="activity-tab-size" role="tab" aria-controls="activity-size" aria-selected="true" tabindex="0">Repository size</button>
    <button type="button" id="activity-tab-calendar" role="tab" aria-controls="activity-calendar" aria-selected="false" tabindex="-1">Commit calendar</button>
</div>
`

// activitySizeMeasureText says what every size figure measures.
const activitySizeMeasureText = "The figures count the files committed as of that day. Lines include comments and blank lines, " +
	"and a binary file counts as a file with no lines. Each day is a UTC day by author date, measured at the last commit of that day."

// activityHistoryGoBuckets names the buckets of the Go composition, in the
// order the page shows them: first-party Go, its production and test halves,
// then third-party Go with the module count. The titles are the file's own
// section labels, so the subjects are the one thing named here.
var activityHistoryGoBuckets = []struct {
	// subject is the history subject the bucket states.
	subject string
	// modules says the bucket also carries the module count.
	modules bool
}{
	{subject: "go-production" + historySubjectJoin + "go-tests"},
	{subject: "go-production"},
	{subject: "go-tests"},
	{subject: "third-party-go", modules: true},
}

// activitySizeHTML renders the size view: the newest recorded day's total and
// date, one figure for each section of the file, what the figures measure, the
// charts and their date control, and the readout. Every figure is a history
// marker the build fills; the charts are drawn by the page script from the
// published file, whose address the view carries. The Go composition of the
// same day sits in the calendar view (activityCalendarViewHTML).
func activitySizeHTML(figures historyFigures) string {
	var out textbuf.Buffer
	out.Reset().Str(`<section class="activity-view activity-size panel" id="activity-size" role="tabpanel" aria-labelledby="activity-tab-size" data-history="`).
		Str(html.EscapeString(activityRoot + historyDataFile)).Str(`">`).Byte('\n')
	out.Str("    <h2>Repository size</h2>\n")
	out.Str(`    <p class="size-headline"><strong>`).Str(historyValueHTML(historySubjectTotal, historyFieldLines)).
		Str(`</strong> lines on <strong>`).Str(historyValueHTML(historySubjectDay, historyFieldDate)).Str("</strong></p>\n")
	out.Str("    <p>").Str(activitySizeMeasureText).Str("</p>\n")
	out.Str(`    <div class="size-sections">`).Byte('\n')
	for _, section := range figures.sections {
		out.Str(`        <div class="stat"><span>`).Str(html.EscapeString(section.Label)).Str("</span><strong>").
			Str(historyValueHTML(section.ID, historyFieldLines)).Str("</strong></div>\n")
	}
	out.Str("    </div>\n")
	out.Str(activitySizeExplorerHTML)
	out.Str("</section>\n")
	return out.String()
}

// activitySizeExplorerHTML holds the two charts, the section legend the script
// fills, the date control and the readout. The charts ship hidden: without the
// script they hold nothing, and the readout then stays empty, or says why the
// file could not be read. The date control sits under both charts, and the
// stylesheet gives it their horizontal inset, so its track spans their time
// line.
const activitySizeExplorerHTML = `    <div class="size-explorer" hidden>
        <div class="size-legend" role="group" aria-label="Sections drawn in the charts"></div>
        <div class="growth-charts">
            <div><h3>Lines per section</h3><svg id="activity-size-chart" class="growth-chart" viewBox="0 0 ` + growthViewWidth + ` 260" tabindex="0" role="img" aria-label="Lines per section on each recorded day. Use arrow keys to select a day, or Home and End."></svg></div>
            <div><h3>Lines gained above the axis, lines lost below</h3><svg id="activity-change-chart" class="growth-chart" viewBox="0 0 ` + growthViewWidth + ` 260" tabindex="0" role="img" aria-label="Lines the selected sections gained and lost on each recorded day. Use arrow keys to select a day, or Home and End."></svg></div>
        </div>
        <label class="growth-date-control" for="activity-size-day">Select a recorded day
            <input id="activity-size-day" type="range" min="0" max="0" value="0" step="1" aria-describedby="activity-size-readout">
        </label>
    </div>
    <div id="activity-size-readout" class="growth-readout size-readout" aria-live="polite" aria-atomic="true"></div>
`

// activityHistoryGoPanelHTML renders the Go composition of the newest recorded
// day: the lines of all Go, third-party included, then four buckets of five
// counts each, and the module count on the third-party bucket. It sits in the
// calendar view after the calendar, where the page carried it before it had a
// size view, and it uses the calendar panel's classes, so it reads the way the
// deck's composition of the working tree does. Every count is a history marker
// of the committed size history, never the working tree.
func activityHistoryGoPanelHTML(figures historyFigures) (string, error) {
	allGo, err := figures.goSubject()
	if err != nil {
		return "", err
	}
	var out textbuf.Buffer
	out.Reset().Str(`<section class="panel go-panel" aria-label="Go code stats">`).Byte('\n')
	out.Str("        <h2>Go code composition</h2>\n")
	out.Str("        <p>As committed on ").Str(historyValueHTML(historySubjectDay, historyFieldDate)).Str(".</p>\n")
	out.Str("        <p>All Go files, third-party included, hold <strong>").Str(historyValueHTML(allGo, historyFieldLines)).
		Str("</strong> lines.</p>\n")
	out.Str(`        <div class="go-breakdown">`).Byte('\n')
	for _, bucket := range activityHistoryGoBuckets {
		title, err := figures.label(bucket.subject)
		if err != nil {
			return "", err
		}
		out.Str(`<div class="go-bucket">`).Byte('\n')
		out.Str("            <h3>").Str(html.EscapeString(title)).Str("</h3>\n")
		out.Str(`            <div class="go-stats">`).Byte('\n')
		out.Str(activityHistoryStatHTML("Files", bucket.subject, historyFieldFiles))
		out.Str(activityHistoryStatHTML("Total lines", bucket.subject, historyFieldLines))
		out.Str(activityHistoryStatHTML("Code", bucket.subject, historyFieldCode))
		out.Str(activityHistoryStatHTML("Blank", bucket.subject, historyFieldBlank))
		out.Str(activityHistoryStatHTML("Comments", bucket.subject, historyFieldComment))
		if bucket.modules {
			out.Str(activityHistoryStatHTML("Modules", historySubjectDay, historyFieldModules))
		}
		out.Str("            </div>\n")
		out.Str("        </div>\n")
	}
	out.Str("        </div>\n")
	out.Str("    </section>\n")
	return out.String(), nil
}

// activityHistoryStatHTML renders one count of one bucket as a history marker.
func activityHistoryStatHTML(label, subject, field string) string {
	return `<div class="stat"><span>` + html.EscapeString(label) + "</span><strong>" +
		historyValueHTML(subject, field) + "</strong></div>\n"
}

// activityCumulativeWording is what the cumulative curve says while the
// calendar's switch shows one metric: the sentence above it, the readout's
// label, the chart's accessible name and the unit the date control reads out.
// The curve carries no heading: the sentence says what it draws.
type activityCumulativeWording struct {
	note  string
	label string
	chart string
	unit  string
}

// activityCumulativeLines is the curve's wording under Added Lines. It names
// the lines as added by commits, so the curve reads as activity and never as
// the repository's size.
var activityCumulativeLines = activityCumulativeWording{
	note:  "Lines added by commits, accumulated since the calendar's first day.",
	label: "Lines added",
	chart: "Cumulative lines added by commits. Use arrow keys to select a date, or Home and End.",
	unit:  "lines added",
}

// activityCumulativeCommits is the curve's wording under Commits.
var activityCumulativeCommits = activityCumulativeWording{
	note:  "Commits accumulated since the calendar's first day.",
	label: "Commits",
	chart: "Cumulative commits. Use arrow keys to select a date, or Home and End.",
	unit:  "commits",
}

// activityCumulativeCurveHTML renders the cumulative curve under the calendar
// it is drawn from. It follows the calendar's switch: lines added by commits
// under Added Lines, commits under Commits, both accumulated over the
// calendar's window. It ships stating the Added Lines wording, the metric the
// cards open on, and every element whose words change carries both wordings
// as data attributes, which the script swaps when the switch moves. It keeps
// its own date control and readout.
func activityCumulativeCurveHTML() string {
	lines, commits := activityCumulativeLines, activityCumulativeCommits
	var out textbuf.Buffer
	out.Reset().Str(`<section class="activity-cumulative panel">`).Byte('\n')
	out.Str("    <p").Str(activityCumulativeTextAttributes(lines.note, commits.note)).Str(">").
		Str(html.EscapeString(lines.note)).Str("</p>\n")
	out.Str(`    <svg id="activity-cumulative-chart" class="growth-chart" viewBox="0 0 `).Str(growthViewWidth).
		Str(` 220" tabindex="0" role="img" aria-label="`).Str(html.EscapeString(lines.chart)).
		Str(`" data-aria-lines="`).Str(html.EscapeString(lines.chart)).
		Str(`" data-aria-commits="`).Str(html.EscapeString(commits.chart)).Str(`"></svg>`).Byte('\n')
	out.Str(`    <label class="growth-date-control" for="activity-cumulative-day">Select a date`).Byte('\n')
	out.Str(`        <input id="activity-cumulative-day" type="range" min="0" max="0" value="0" step="1" aria-describedby="activity-cumulative-readout"`).
		Str(` data-unit-lines="`).Str(html.EscapeString(lines.unit)).
		Str(`" data-unit-commits="`).Str(html.EscapeString(commits.unit)).Str(`">`).Byte('\n')
	out.Str("    </label>\n")
	out.Str(`    <div id="activity-cumulative-readout" class="growth-readout" aria-live="polite" aria-atomic="true">`).Byte('\n')
	out.Str(`        <strong id="activity-cumulative-date"></strong>`).Byte('\n')
	out.Str("        <span><span").Str(activityCumulativeTextAttributes(lines.label, commits.label)).Str(">").
		Str(html.EscapeString(lines.label)).Str(`</span>: <strong id="activity-cumulative-value"></strong></span>`).Byte('\n')
	out.Str("    </div>\n")
	out.Str("</section>\n")
	return out.String()
}

// activityCumulativeTextAttributes writes the two wordings of one element of
// the curve, which the script reads when the switch moves.
func activityCumulativeTextAttributes(lines, commits string) string {
	return ` data-text-lines="` + html.EscapeString(lines) + `" data-text-commits="` + html.EscapeString(commits) + `"`
}

// activityGrowthScript drives the page's two views. It opens the size view,
// reads the published size history, refills every history marker of the
// widget from it (the size view's and the Go composition's in the calendar
// view), and draws the size and daily-change charts; it draws the cumulative
// curve from the calendar's cells and redraws it when the calendar's switch
// moves. Every label and figure enters the page as text content.
//
// The switch belongs to activityPageScript, which the deck shares, so this
// script does not call into it: it observes the body's data-metric attribute,
// which setMetric writes, and keeps the selected date across a change.
//
// Both size charts scale to the largest value of every recorded day of the
// sections they draw, so an earlier peak and a later decline stay inside the
// chart. A section's color comes from its position in the file's list, and
// the legend names each color in text. The readout's table is built once,
// with one row for every section of the file whatever the legend selects, and
// a day's selection rewrites only its figures, so its layout never moves.
const activityGrowthScript = `<script>
(() => {
    const left = ` + growthPlotLeft + `, right = ` + growthPlotRight + `, viewWidth = ` + growthViewWidth + `;
    const format = value => Number(value).toLocaleString('en-US');
    const x = (index, count) => left + index / Math.max(1, count - 1) * (right - left);
    function svgElement(name, attributes, text) {
        const node = document.createElementNS('http://www.w3.org/2000/svg', name);
        for (const [key, value] of Object.entries(attributes)) node.setAttribute(key, value);
        if (text !== undefined) node.textContent = text;
        return node;
    }
    function htmlElement(name, className, text) {
        const node = document.createElement(name);
        if (className) node.className = className;
        if (text !== undefined) node.textContent = text;
        return node;
    }
    function scale(low, high, top, bottom) {
        const span = Math.max(1, high - low);
        return value => bottom - (value - low) / span * (bottom - top);
    }
    function frame(chart, levels, y, top, bottom, first, last) {
        for (const level of new Set(levels)) {
            chart.append(svgElement('path', {class:'growth-grid', d:'M' + left + ' ' + y(level) + 'H' + right}));
            chart.append(svgElement('text', {class:'growth-axis', x:left - 10, y:y(level) + 4, 'text-anchor':'end'}, format(level)));
        }
        chart.append(svgElement('path', {class:'growth-cursor', d:'M' + left + ' ' + top + 'V' + bottom}));
        chart.append(svgElement('text', {class:'growth-axis', x:left, y:bottom + 27}, first));
        chart.append(svgElement('text', {class:'growth-axis', x:right, y:bottom + 27, 'text-anchor':'end'}, last));
    }
    function placeCursor(chart, index, count, top, bottom) {
        chart.querySelector('.growth-cursor').setAttribute('d', 'M' + x(index, count) + ' ' + top + 'V' + bottom);
    }
    function selector(charts, slider, count, show) {
        let selected = count - 1;
        function select(index) {
            if (!count) return;
            selected = Math.max(0, Math.min(count - 1, index));
            slider.value = selected;
            show(selected);
        }
        slider.max = Math.max(0, count - 1);
        slider.disabled = count === 0;
        slider.addEventListener('input', () => select(Number(slider.value)));
        for (const chart of charts) {
            chart.addEventListener('pointermove', event => {
                const bounds = chart.getBoundingClientRect();
                const position = (event.clientX - bounds.left) / bounds.width * viewWidth;
                select(Math.round((position - left) / (right - left) * (count - 1)));
            });
            chart.addEventListener('keydown', event => {
                let index;
                if (event.key === 'ArrowLeft') index = selected - 1;
                else if (event.key === 'ArrowRight') index = selected + 1;
                else if (event.key === 'Home') index = 0;
                else if (event.key === 'End') index = count - 1;
                else return;
                event.preventDefault(); select(index);
            });
        }
        return select;
    }

    const tabs = Array.from(document.querySelectorAll('.activity-tabs [role="tab"]'));
    function selectTab(tab) {
        document.getElementById('activity-tooltip').hidden = true;
        for (const candidate of tabs) {
            const active = candidate === tab;
            candidate.setAttribute('aria-selected', String(active));
            candidate.tabIndex = active ? 0 : -1;
            document.getElementById(candidate.getAttribute('aria-controls')).hidden = !active;
        }
    }
    tabs.forEach((tab, index) => {
        tab.addEventListener('click', () => selectTab(tab));
        tab.addEventListener('keydown', event => {
            let next;
            if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = (index + tabs.length - 1) % tabs.length;
            else if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = (index + 1) % tabs.length;
            else if (event.key === 'Home') next = 0;
            else if (event.key === 'End') next = tabs.length - 1;
            else return;
            event.preventDefault(); selectTab(tabs[next]); tabs[next].focus();
        });
    });
    document.querySelector('.activity-tabs').hidden = false;
    selectTab(document.getElementById('activity-tab-size'));

    // The cumulative curve accumulates both metrics of the calendar's dated
    // cells, oldest first, and draws the one the switch shows. The wording of
    // each metric is written on the elements it changes.
    const cumulativeKeys = {lines: 'Lines', commits: 'Commits'};
    const cells = Array.from(document.querySelectorAll('.activity-grid .day-cell:not(.outside)'))
        .sort((a, b) => a.dataset.date.localeCompare(b.dataset.date));
    let addedLines = 0, commits = 0;
    const windowDays = cells.map(cell => {
        addedLines += Number(cell.dataset.lines);
        commits += Number(cell.dataset.commits);
        return {date: cell.dataset.date, lines: addedLines, commits};
    });
    const curve = document.querySelector('.activity-cumulative');
    const curveChart = document.getElementById('activity-cumulative-chart');
    const curveSlider = document.getElementById('activity-cumulative-day');
    const curveTop = 20, curveBottom = 184;
    let curveMetric, curveY = scale(0, 0, curveTop, curveBottom);
    let curveSelected = windowDays.length - 1;
    function showCurveDay(index) {
        curveSelected = index;
        const day = windowDays[index], value = day[curveMetric];
        document.getElementById('activity-cumulative-date').textContent = day.date;
        document.getElementById('activity-cumulative-value').textContent = format(value);
        curveSlider.setAttribute('aria-valuetext', day.date + ': ' + format(value) + ' ' + curveSlider.dataset['unit' + cumulativeKeys[curveMetric]]);
        placeCursor(curveChart, index, windowDays.length, curveTop, curveBottom);
        const point = curveChart.querySelector('.growth-point');
        point.setAttribute('cx', x(index, windowDays.length));
        point.setAttribute('cy', curveY(value));
    }
    const selectCurveDay = selector([curveChart], curveSlider, windowDays.length, showCurveDay);
    function drawCurve(metric) {
        const key = cumulativeKeys[metric];
        curveMetric = metric;
        for (const node of curve.querySelectorAll('[data-text-lines]')) node.textContent = node.dataset['text' + key];
        curveChart.setAttribute('aria-label', curveChart.dataset['aria' + key]);
        curveChart.replaceChildren();
        if (!windowDays.length) {
            document.getElementById('activity-cumulative-date').textContent = 'No daily values in this window.';
            return;
        }
        const high = windowDays.reduce((most, day) => Math.max(most, day[metric]), 0);
        curveY = scale(0, high, curveTop, curveBottom);
        frame(curveChart, [0, Math.ceil(high / 2), high], curveY, curveTop, curveBottom,
            windowDays[0].date, windowDays[windowDays.length - 1].date);
        curveChart.append(svgElement('path', {class:'growth-series cumulative-series',
            d:'M' + left + ' ' + curveBottom + windowDays.map((day, index) => 'L' + x(index, windowDays.length) + ' ' + curveY(day[metric])).join('')}));
        curveChart.append(svgElement('circle', {class:'growth-point', cx:left, cy:curveBottom, r:4}));
        selectCurveDay(curveSelected);
    }
    // The curve opens on the metric the switch shows, or on Added Lines, the
    // wording its markup ships with, when the switch has not run. A metric the
    // curve has no wording for leaves it as it is.
    const switchMetric = () => Object.hasOwn(cumulativeKeys, document.body.dataset.metric) ? document.body.dataset.metric : undefined;
    drawCurve(switchMetric() || 'lines');
    new MutationObserver(() => {
        const metric = switchMetric();
        if (metric && metric !== curveMetric) drawCurve(metric);
    }).observe(document.body, {attributes: true, attributeFilter: ['data-metric']});

    const sizeView = document.getElementById('activity-size');
    const widget = sizeView.closest('.activity-widget');
    const sizeReadout = document.getElementById('activity-size-readout');
    const valueFields = ['files', 'lines', 'gained', 'lost'];
    const goFields = ['code', 'blank', 'comment'];
    const readoutFields = ['lines', 'gained', 'lost'];
    function sectionValues(day, id) {
        const values = day.sections[id] || {};
        return {files: values.files || 0, lines: values.lines || 0, gained: values.gained || 0, lost: values.lost || 0, go: values.go || {}};
    }
    function figure(file, subject, field) {
        const day = file.days[file.days.length - 1];
        if (subject === 'day') {
            if (field === 'date') return day.date;
            if (field === 'modules') return format(day.modules);
            return undefined;
        }
        const parts = subject === 'total' ? file.sections
            : subject.split('+').map(id => file.sections.find(section => section.id === id));
        let sum = 0;
        for (const section of parts) {
            if (!section) return undefined;
            const values = sectionValues(day, section.id);
            if (valueFields.includes(field)) sum += values[field];
            else if (goFields.includes(field) && section['go-split']) sum += values.go[field] || 0;
            else return undefined;
        }
        return format(sum);
    }
    function fillMarkers(file) {
        for (const marked of widget.querySelectorAll('[data-ze-history]')) {
            const mark = marked.getAttribute('data-ze-history');
            const at = mark.indexOf(':');
            const text = figure(file, mark.slice(0, at), mark.slice(at + 1));
            if (text !== undefined) marked.textContent = text;
        }
    }
    // readoutTable builds the readout once: one row per section of the file and
    // the sum, with fixed columns. The site script gives a table a Columns
    // control unless it opts out; this one does, because hiding a column of
    // four gains nothing and the control would change the readout's height.
    function readoutTable(sections) {
        const table = htmlElement('table', 'size-table');
        table.setAttribute('data-column-controls', 'off');
        const columns = document.createElement('colgroup');
        columns.append(htmlElement('col', 'size-column-section'));
        for (const field of readoutFields) columns.append(htmlElement('col', 'size-column-figure size-column-' + field));
        table.append(columns);
        const head = table.createTHead().insertRow();
        for (const name of ['Section', 'Lines', 'Gained', 'Lost']) head.append(htmlElement('th', '', name));
        const body = table.createTBody();
        function figureRow(label) {
            const row = body.insertRow();
            row.append(htmlElement('th', '', label));
            return readoutFields.map(() => row.appendChild(htmlElement('td', '')));
        }
        const rows = sections.map(section => figureRow(section.label));
        const total = figureRow('All sections');
        const date = htmlElement('strong', 'size-readout-date');
        const wrap = htmlElement('div', 'size-table-wrap');
        wrap.append(table);
        sizeReadout.replaceChildren(date, wrap);
        return {date, rows, total};
    }
    function drawSize(file) {
        const sections = file.sections, days = file.days, count = days.length;
        const sizeChart = document.getElementById('activity-size-chart');
        const changeChart = document.getElementById('activity-change-chart');
        const slider = document.getElementById('activity-size-day');
        const legend = sizeView.querySelector('.size-legend');
        const top = 20, bottom = 224;
        const shown = new Set(sections.map(section => section.id));
        const readout = readoutTable(sections);
        let sizeY = scale(0, 0, top, bottom), selected = count - 1;
        function show(index) {
            selected = index;
            const day = days[index];
            placeCursor(sizeChart, index, count, top, bottom);
            placeCursor(changeChart, index, count, top, bottom);
            for (const point of sizeChart.querySelectorAll('.size-point')) {
                point.setAttribute('cx', x(index, count));
                point.setAttribute('cy', sizeY(sectionValues(day, point.dataset.section).lines));
            }
            readout.date.textContent = day.date;
            const total = {lines: 0, gained: 0, lost: 0};
            sections.forEach((section, row) => {
                const values = sectionValues(day, section.id);
                readoutFields.forEach((field, column) => {
                    readout.rows[row][column].textContent = format(values[field]);
                    total[field] += values[field];
                });
            });
            readoutFields.forEach((field, column) => { readout.total[column].textContent = format(total[field]); });
            slider.setAttribute('aria-valuetext', day.date + ': ' + format(total.lines) + ' lines');
        }
        const select = selector([sizeChart, changeChart], slider, count, show);
        function redraw() {
            sizeChart.replaceChildren();
            changeChart.replaceChildren();
            const drawn = sections.map((section, index) => ({section, series: String(index % 12 + 1)}))
                .filter(entry => shown.has(entry.section.id));
            let high = 0, gainedHigh = 0, lostHigh = 0;
            const change = days.map(day => {
                let gained = 0, lost = 0;
                for (const entry of drawn) {
                    const values = sectionValues(day, entry.section.id);
                    high = Math.max(high, values.lines);
                    gained += values.gained; lost += values.lost;
                }
                gainedHigh = Math.max(gainedHigh, gained); lostHigh = Math.max(lostHigh, lost);
                return {gained, lost};
            });
            sizeY = scale(0, high, top, bottom);
            frame(sizeChart, [0, Math.ceil(high / 2), high], sizeY, top, bottom, days[0].date, days[count - 1].date);
            for (const entry of drawn) {
                const d = days.map((day, index) => (index ? 'L' : 'M') + x(index, count) + ' ' + sizeY(sectionValues(day, entry.section.id).lines)).join('');
                const line = svgElement('path', {class:'growth-series size-line', d, 'data-series':entry.series, 'data-section':entry.section.id});
                line.append(svgElement('title', {}, entry.section.label));
                sizeChart.append(line);
                sizeChart.append(svgElement('circle', {class:'growth-point size-point', cx:left, cy:bottom, r:4,
                    'data-series':entry.series, 'data-section':entry.section.id}));
            }
            const changeY = scale(-lostHigh, gainedHigh, top, bottom);
            frame(changeChart, [-lostHigh, 0, gainedHigh], changeY, top, bottom, days[0].date, days[count - 1].date);
            const width = Math.max(1, (right - left) / count * 0.8);
            const bars = sign => change.map((entry, index) => {
                const base = changeY(0), end = changeY(sign > 0 ? entry.gained : -entry.lost);
                const from = x(index, count) - width / 2, to = from + width;
                return 'M' + from + ' ' + base + 'L' + from + ' ' + end + 'L' + to + ' ' + end + 'L' + to + ' ' + base + 'Z';
            }).join('');
            const gained = svgElement('path', {class:'change-gained', d:bars(1)});
            gained.append(svgElement('title', {}, 'Lines gained'));
            const lost = svgElement('path', {class:'change-lost', d:bars(-1)});
            lost.append(svgElement('title', {}, 'Lines lost'));
            changeChart.append(gained, lost);
            show(selected);
        }
        sections.forEach((section, index) => {
            const button = htmlElement('button', 'size-toggle');
            button.type = 'button';
            button.dataset.series = String(index % 12 + 1);
            button.dataset.section = section.id;
            button.setAttribute('aria-pressed', 'true');
            button.append(htmlElement('span', 'size-swatch'), document.createTextNode(section.label));
            button.addEventListener('click', () => {
                if (shown.has(section.id)) shown.delete(section.id);
                else shown.add(section.id);
                button.setAttribute('aria-pressed', String(shown.has(section.id)));
                redraw();
            });
            legend.append(button);
        });
        redraw();
        select(count - 1);
    }
    fetch(sizeView.dataset.history).then(response => {
        if (!response.ok) throw new Error('the server answered ' + response.status);
        return response.json();
    }).then(file => {
        if (!Array.isArray(file.sections) || !Array.isArray(file.days) || !file.days.length) throw new Error('the file holds no recorded day');
        fillMarkers(file);
        sizeView.querySelector('.size-explorer').hidden = false;
        drawSize(file);
    }).catch(error => {
        sizeView.querySelector('.size-explorer').hidden = true;
        sizeReadout.textContent = 'The size history could not be read (' + error.message +
            '), so the figures above are those of the last build and no chart is drawn.';
    });
})();
</script>
`
