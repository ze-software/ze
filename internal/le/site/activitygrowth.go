// Design: website/AI.md -- alternate views of the calendar's existing data.
package site

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	reporewrite "github.com/ze-software/ze/internal/le/repo/rewrite"
)

const activityTabsHTML = `<div class="activity-tabs" role="tablist" aria-label="Activity representation" aria-orientation="vertical" hidden>
    <button type="button" id="activity-tab-calendar" role="tab" aria-controls="activity-calendar" aria-selected="true" tabindex="0">Calendar</button>
    <button type="button" id="activity-tab-growth" role="tab" aria-controls="activity-growth" aria-selected="false" tabindex="-1">Growth</button>
</div>
`

// activityGrowthHTML exposes only the checkout inventory; the browser derives
// the history curves from the dated cells the calendar already carries.
func activityGrowthHTML(inventory reporewrite.ActivityGo) string {
	var out textbuf.Buffer
	out.Reset().Str(`<section class="activity-view activity-growth panel" id="activity-growth" role="tabpanel" aria-labelledby="activity-tab-growth" hidden>
    <h2>Growth within this window</h2>
    <p>Commits and added lines accumulated since the displayed window began. Added lines count additions, not net repository size.</p>
    <div class="growth-charts">
        <div data-growth-metric="lines"><h3>Cumulative added lines</h3><svg id="activity-growth-lines-chart" class="growth-chart" viewBox="0 0 720 220" tabindex="0" role="img" aria-label="Cumulative added lines. Use arrow keys to select a date, or Home and End."></svg></div>
        <div data-growth-metric="commits"><h3>Cumulative commits</h3><svg id="activity-growth-commits-chart" class="growth-chart" viewBox="0 0 720 220" tabindex="0" role="img" aria-label="Cumulative commits. Use arrow keys to select a date, or Home and End."></svg></div>
    </div>
    <label class="growth-date-control" for="activity-growth-day">Select a date
        <input id="activity-growth-day" type="range" min="0" max="0" value="0" step="1" aria-describedby="activity-growth-readout">
    </label>
    <div id="activity-growth-readout" class="growth-readout" aria-live="polite" aria-atomic="true">
        <strong id="activity-growth-date"></strong>
        <span data-growth-metric="lines">Added lines: <strong id="activity-growth-lines"></strong></span>
        <span data-growth-metric="commits">Commits: <strong id="activity-growth-commits"></strong></span>
    </div>
    <h2>Current Go lines</h2>
    <p>Current snapshot, including blank lines and comments. Historical category totals are unavailable in this data.</p>
    <div class="growth-snapshot">
`)
	for _, bucket := range []struct {
		name, label string
		lines       int
	}{
		{"code", "Code", inventory.Code.TotalLines},
		{"tests", "Tests", inventory.Tests.TotalLines},
		{"vendor", "Third party", inventory.Vendor.TotalLines},
		{"overall", "Overall", inventory.Code.TotalLines + inventory.Tests.TotalLines + inventory.Vendor.TotalLines},
	} {
		out.Str(`<div class="stat"><span>`).Str(bucket.label).
			Str(`</span><strong id="activity-growth-`).Str(bucket.name).Str(`">`).
			Str(groupThousands(bucket.lines)).Str("</strong></div>\n")
	}
	out.Str("    </div>\n</section>\n")
	return out.String()
}

const activityGrowthScript = `<script>
(() => {
    const tabs = Array.from(document.querySelectorAll('.activity-tabs [role="tab"]'));
    const points = Array.from(document.querySelectorAll('.activity-grid .day-cell:not(.outside)'))
        .sort((a, b) => a.dataset.date.localeCompare(b.dataset.date));
    let lines = 0, commits = 0;
    const history = points.map(cell => {
        lines += Number(cell.dataset.lines);
        commits += Number(cell.dataset.commits);
        return {date: cell.dataset.date, lines, commits};
    });
    const slider = document.getElementById('activity-growth-day');
    const charts = [document.getElementById('activity-growth-lines-chart'), document.getElementById('activity-growth-commits-chart')];
    const left = 72, right = 704, top = 20, bottom = 184;
    const format = value => value.toLocaleString('en-US');
    const x = index => left + index / Math.max(1, history.length - 1) * (right - left);
    function svgElement(name, attributes, text) {
        const element = document.createElementNS('http://www.w3.org/2000/svg', name);
        for (const [key, value] of Object.entries(attributes)) element.setAttribute(key, value);
        if (text !== undefined) element.textContent = text;
        return element;
    }
    function draw(chart, metric) {
        const maximum = history.length ? history[history.length - 1][metric] : 0;
        const y = value => bottom - value / Math.max(1, maximum) * (bottom - top);
        for (const level of new Set([0, Math.ceil(maximum / 2), maximum])) {
            chart.append(svgElement('path', {class:'growth-grid', d:'M' + left + ' ' + y(level) + 'H' + right}));
            chart.append(svgElement('text', {class:'growth-axis', x:left - 10, y:y(level) + 4, 'text-anchor':'end'}, format(level)));
        }
        if (!history.length) return;
        const path = 'M' + left + ' ' + bottom + history.map((point, index) => 'L' + x(index) + ' ' + y(point[metric])).join('');
        chart.append(svgElement('path', {class:'growth-series', d:path}));
        chart.append(svgElement('path', {class:'growth-cursor', d:'M' + left + ' ' + top + 'V' + bottom}));
        chart.append(svgElement('circle', {class:'growth-point', cx:left, cy:bottom, r:4}));
        chart.append(svgElement('text', {class:'growth-axis', x:left, y:211}, history[0].date));
        chart.append(svgElement('text', {class:'growth-axis', x:right, y:211, 'text-anchor':'end'}, history[history.length - 1].date));
    }
    draw(charts[0], 'lines'); draw(charts[1], 'commits');
    let selected = history.length - 1;
    function selectDay(index) {
        if (!history.length) return;
        selected = Math.max(0, Math.min(history.length - 1, index));
        slider.value = selected;
        const point = history[selected];
        document.getElementById('activity-growth-date').textContent = point.date;
        document.getElementById('activity-growth-lines').textContent = format(point.lines);
        document.getElementById('activity-growth-commits').textContent = format(point.commits);
        slider.setAttribute('aria-valuetext', point.date + ': ' + format(point.lines) + ' added lines, ' + format(point.commits) + ' commits');
        charts.forEach((chart, index) => {
            const metric = index === 0 ? 'lines' : 'commits';
            const maximum = history[history.length - 1][metric];
            chart.querySelector('.growth-cursor').setAttribute('d', 'M' + x(selected) + ' ' + top + 'V' + bottom);
            const dot = chart.querySelector('.growth-point');
            dot.setAttribute('cx', x(selected));
            dot.setAttribute('cy', bottom - point[metric] / Math.max(1, maximum) * (bottom - top));
        });
    }
    slider.max = Math.max(0, history.length - 1);
    slider.disabled = history.length === 0;
    if (history.length) selectDay(selected);
    else document.getElementById('activity-growth-date').textContent = 'No daily values in this window.';
    slider.addEventListener('input', () => selectDay(Number(slider.value)));
    for (const chart of charts) {
        chart.addEventListener('pointermove', event => {
            const bounds = chart.getBoundingClientRect();
            const position = (event.clientX - bounds.left) / bounds.width * 720;
            selectDay(Math.round((position - left) / (right - left) * (history.length - 1)));
        });
        chart.addEventListener('keydown', event => {
            let index;
            if (event.key === 'ArrowLeft') index = selected - 1;
            else if (event.key === 'ArrowRight') index = selected + 1;
            else if (event.key === 'Home') index = 0;
            else if (event.key === 'End') index = history.length - 1;
            else return;
            event.preventDefault(); selectDay(index);
        });
    }
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
})();
</script>
`
