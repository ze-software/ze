// Design: website/AI.md -- the size view draws the committed size history; the commits curve the calendar's days.
//go:build integration

package site

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	reporewrite "github.com/ze-software/ze/internal/le/repo/rewrite"
)

// TestActivitySizeChartFollowsTheHistoryFile drives the page in Chromium at a
// 320px viewport. The page fetches the hand-written fixture history from the
// artifact's data directory, as the published page does; the fixture holds five
// sections, one of which (Go production) peaks on its third day and declines,
// and one of which (Plans) shrinks. The calendar fixture carries padding with
// activity, an idle day and cells out of date order, so the commits curve must
// read the dated window.
//
// The page is served with its total marker stating a figure the file
// contradicts, as a page built before the file was last updated would, so the
// file's total can reach the browser only through the script's refill.
//
// VALIDATES: AC-11: date selection by pointer, keyboard and scrubber on the
// size charts and on the commits curve, section selection, both views
// reachable both ways, the decline inside the chart, the readout agreeing with
// the file, the commits curve drawn, every chart scaling by its viewBox, both
// themes, nothing overflowing at 320px; AC-10: the page opens on the size view,
// and the script replaces a marker's stale built figure with the file's value.
func TestActivitySizeChartFollowsTheHistoryFile(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	paths := Paths{Source: filepath.Join(root, "website"), Output: t.TempDir()}
	if err := renderCSS(paths.Source, paths.Output); err != nil {
		t.Fatal(err)
	}
	if err := renderJS(paths.Source, paths.Output); err != nil {
		t.Fatal(err)
	}
	published := filepath.Join(paths.Output, filepath.FromSlash(historyDataFile))
	if err := os.MkdirAll(filepath.Dir(published), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(published, []byte(readFixture(t, "repo-history.json")), 0o600); err != nil {
		t.Fatal(err)
	}
	window := reporewrite.ActivityWindow{
		Start:   time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		End:     time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
		Lines:   reporewrite.ActivitySeries{Total: 17, ActiveDays: 2, Peak: 10},
		Commits: reporewrite.ActivitySeries{Total: 3, ActiveDays: 2, Peak: 2},
		Cells: `<div class="day-cell outside" data-date="2026-07-31" data-lines="999" data-commits="99"></div>
<div class="day-cell" data-date="2026-08-03" data-date-label="Mon 03 Aug 2026" data-lines="7" data-lines-display="7" data-lines-level="4" data-commits="1" data-commits-display="1" data-commits-level="4"></div>
<div class="day-cell" data-date="2026-08-01" data-date-label="Sat 01 Aug 2026" data-lines="10" data-lines-display="10" data-lines-level="5" data-commits="2" data-commits-display="2" data-commits-level="5"></div>
<div class="day-cell" data-date="2026-08-02" data-date-label="Sun 02 Aug 2026" data-lines="0" data-lines-display="0" data-lines-level="0" data-commits="0" data-commits-display="0" data-commits-level="0"></div>`,
	}
	figures := activityHistoryFigures(t)
	content, err := activityPageBody(&window, figures)
	if err != nil {
		t.Fatal(err)
	}
	// The body is filled before the observation joins it: the observation's
	// selectors name the marker attribute, which the fill refuses as a stray
	// marker. The shell carries no marker, so this is the producer's fill.
	filled, err := figures.fillHTML(content)
	if err != nil {
		t.Fatal(err)
	}
	page := pageShell{Title: "Activity size", Root: activityRoot, Path: activityDest, ExtraHead: activityStyle}.
		render(filled + activitySizeObservation)
	// The build filled the total with the fixture's newest value. Serving a
	// stale one instead leaves the script's refill as the only way the
	// observation below can read 2,344.
	const builtTotal = `<span data-ze-history="total:lines">2,344</span>`
	if count := strings.Count(page, builtTotal); count != 1 {
		t.Fatalf("the built page states %s %d times, want once", builtTotal, count)
	}
	page = strings.Replace(page, builtTotal, `<span data-ze-history="total:lines">1,000</span>`, 1)
	observations := make(chan []string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+activityRoute, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := io.WriteString(w, page); err != nil {
			t.Errorf("serve size view: %v", err)
		}
	})
	mux.HandleFunc("POST /size-observation", func(w http.ResponseWriter, r *http.Request) {
		var failures []string
		if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&failures); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		select {
		case observations <- failures:
		default:
			t.Error("duplicate size observation")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", http.FileServer(http.Dir(paths.Output)))
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	session := "activity-size-" + rand.Text()
	defer func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer stop()
		closeBrowser := exec.CommandContext(cleanup, "agent-browser", "--session", session, "close")
		closeBrowser.WaitDelay = 2 * time.Second
		if output, err := closeBrowser.CombinedOutput(); err != nil {
			t.Errorf("close owned size browser: %v: %s", err, output)
		}
	}()
	// The viewport is set before the first navigation, so the page lays out
	// at the narrowest width the site supports from its first paint.
	viewport := exec.CommandContext(ctx, "agent-browser", "--session", session, "set", "viewport", "320", "800")
	viewport.WaitDelay = 2 * time.Second
	if output, err := viewport.CombinedOutput(); err != nil {
		t.Fatalf("set a 320px viewport in Chromium: %v: %s", err, output)
	}
	openBrowser := exec.CommandContext(ctx, "agent-browser", "--session", session, "open", server.URL+activityRoute)
	openBrowser.WaitDelay = 2 * time.Second
	if output, err := openBrowser.CombinedOutput(); err != nil {
		t.Fatalf("open the activity page in Chromium: %v: %s", err, output)
	}
	select {
	case failures := <-observations:
		for _, failure := range failures {
			t.Error(failure)
		}
	case <-ctx.Done():
		t.Fatal("Chromium did not report the size view's behavior")
	}
}

const activitySizeObservation = `<script>
window.addEventListener('load', async () => {
  const failures = [];
  function check(ok, message) { if (!ok) failures.push(message); }
  function text(id) { return document.getElementById(id).textContent.trim(); }
  function key(el, value) { el.dispatchEvent(new KeyboardEvent('keydown', {key:value, bubbles:true})); }
  function shown(el) { return el.getClientRects().length > 0 && getComputedStyle(el).visibility !== 'hidden'; }
  function fits(view) { check(document.documentElement.scrollWidth <= window.innerWidth + 1, view + ' overflows a ' + window.innerWidth + 'px viewport: ' + document.documentElement.scrollWidth + 'px'); }
  function row(label) {
    for (const tr of document.querySelectorAll('#activity-size-readout tbody tr')) {
      if (tr.querySelector('th').textContent === label) return Array.from(tr.querySelectorAll('td')).map(td => td.textContent);
    }
    return null;
  }
  function selected(date, label, lines, gained, lost) {
    check(document.querySelector('#activity-size-readout strong').textContent === date, 'Selected day must be ' + date);
    const values = row(label);
    check(!!values && values[0] === lines && values[1] === gained && values[2] === lost,
      label + ' on ' + date + ' must read ' + [lines, gained, lost] + ', read ' + values);
  }
  function ys(path) {
    return (path.getAttribute('d').match(/-?[\d.]+ -?[\d.]+/g) || []).map(pair => Number(pair.split(' ')[1]));
  }
  try {
    const explorer = document.querySelector('.size-explorer');
    for (let wait = 0; explorer.hidden && wait < 50; wait++) await new Promise(resolve => setTimeout(resolve, 100));
    check(!explorer.hidden, 'The charts must show once the history file is read: ' + text('activity-size-readout'));
    const sizeTab = document.getElementById('activity-tab-size');
    const calendarTab = document.getElementById('activity-tab-calendar');
    const sizePanel = document.getElementById('activity-size');
    const calendarPanel = document.getElementById('activity-calendar');
    check(shown(sizeTab) && shown(calendarTab), 'Both tabs must show once the script runs');
    check(sizeTab.getAttribute('aria-selected') === 'true' && !sizePanel.hidden && calendarPanel.hidden,
      'The size view must be the view the page opens on');
    check(!shown(document.getElementById('total-value')), 'The calendar cards must hide with the calendar view');
    check(document.querySelector('[data-ze-history="total:lines"]').textContent === '2,344', 'The script must refill the stale built total from the file');
    for (const id of ['activity-size-chart', 'activity-change-chart', 'activity-commits-chart']) {
      check(document.getElementById(id).getAttribute('viewBox') !== null, id + ' must scale responsively');
    }
    check(document.querySelector('[data-ze-history="go-production+go-tests:lines"]').textContent === '365', 'First-party Go must sum production and tests');
    const legend = Array.from(document.querySelectorAll('.size-legend button'));
    check(legend.map(button => button.textContent).join('|') === 'Third-party Go|Go tests|Go production|Plans|Other',
      'The legend must name the file sections in file order: ' + legend.map(button => button.textContent));
    const chart = document.getElementById('activity-size-chart');
    const lines = Array.from(chart.querySelectorAll('.size-line'));
    check(lines.length === 5, 'Every section must be drawn: ' + lines.length);
    for (const line of lines) {
      for (const y of ys(line)) check(y >= 19.5 && y <= 224.5, line.dataset.section + ' leaves the chart at y=' + y);
    }
    const production = chart.querySelector('.size-line[data-section="go-production"]');
    const productionY = ys(production);
    check(productionY.length === 5 && Math.min(...productionY) === productionY[2] && productionY[4] > productionY[2],
      'Go production must peak on its third day and decline after it: ' + productionY);
    const stroke = getComputedStyle(production).stroke;
    check(stroke !== getComputedStyle(chart.querySelector('.size-line[data-section="go-tests"]')).stroke, 'Two sections must not share a color');
    const change = document.getElementById('activity-change-chart');
    const zero = Array.from(change.querySelectorAll('.growth-axis')).find(label => label.textContent === '0');
    const zeroY = Number(zero.getAttribute('y')) - 4;
    check(ys(change.querySelector('.change-gained')).every(y => y <= zeroY + 0.5), 'Gained lines must stand above the axis');
    check(ys(change.querySelector('.change-lost')).some(y => y > zeroY + 0.5), 'Lost lines must stand below the axis');
    selected('2026-08-23', 'Go production', '190', '15', '5');
    selected('2026-08-23', 'All sections', '2,344', '1,294', '15');
    fits('The size view');
    chart.focus(); key(chart, 'Home'); selected('2026-08-19', 'Go production', '120', '120', '0');
    key(chart, 'ArrowRight'); selected('2026-08-20', 'Go production', '210', '100', '10');
    key(chart, 'End'); selected('2026-08-23', 'Plans', '95', '40', '5');
    const day = document.getElementById('activity-size-day');
    day.value = '3'; day.dispatchEvent(new Event('input', {bubbles:true}));
    selected('2026-08-22', 'Plans', '60', '0', '80');
    const rect = chart.getBoundingClientRect();
    chart.dispatchEvent(new PointerEvent('pointermove', {clientX:rect.left + rect.width * 0.55, clientY:rect.top + rect.height / 2}));
    selected('2026-08-21', 'Go production', '300', '120', '30');
    const other = legend.find(button => button.textContent === 'Other');
    other.click();
    check(other.getAttribute('aria-pressed') === 'false' && chart.querySelectorAll('.size-line').length === 4, 'Deselecting a section must remove its line');
    const levels = Array.from(chart.querySelectorAll('.growth-axis')).map(label => label.textContent);
    check(levels.includes('650') && !levels.includes('1,234'), 'The scale must follow the drawn sections: ' + levels);
    selected('2026-08-21', 'Go production', '300', '120', '30');
    other.click();
    check(chart.querySelectorAll('.size-line').length === 5, 'Reselecting a section must draw it again');
    document.documentElement.dataset.theme = 'light';
    const light = getComputedStyle(chart.querySelector('.size-line')).stroke;
    document.documentElement.dataset.theme = 'dark';
    const dark = getComputedStyle(chart.querySelector('.size-line')).stroke;
    check(light !== dark, 'The section colors must follow the theme');
    calendarTab.click();
    check(calendarPanel.hidden === false && sizePanel.hidden, 'The calendar tab must open the calendar view');
    check(shown(document.getElementById('total-value')), 'The calendar cards must show with the calendar view');
    const commits = document.getElementById('activity-commits-chart');
    check(shown(commits), 'The commits curve must show under the calendar');
    const commitsSeries = commits.querySelector('.growth-series');
    check(!!commitsSeries && commitsSeries.getAttribute('d').length > 12, 'The commits curve must plot the calendar days');
    check(text('activity-commits-date') === '2026-08-03' && text('activity-commits-count') === '3', 'The commits curve must total the window');
    document.querySelector('[data-metric="lines"]').click();
    check(shown(commits), 'The commits curve must show while the switch says Added Lines');
    document.querySelector('[data-metric="commits"]').click();
    check(shown(commits) && text('total-value') === '3', 'The commits curve and the commit cards must show while the switch says Commits');
    const commitsDay = document.getElementById('activity-commits-day');
    commitsDay.value = '0'; commitsDay.dispatchEvent(new Event('input', {bubbles:true}));
    check(text('activity-commits-date') === '2026-08-01' && text('activity-commits-count') === '2', 'The commits scrubber must select a day');
    commits.focus(); key(commits, 'ArrowRight');
    check(text('activity-commits-date') === '2026-08-02' && text('activity-commits-count') === '2', 'The commits curve must answer the keyboard');
    const commitsRect = commits.getBoundingClientRect();
    commits.dispatchEvent(new PointerEvent('pointermove', {clientX:commitsRect.left + commitsRect.width * 0.07, clientY:commitsRect.top + commitsRect.height / 2}));
    check(text('activity-commits-date') === '2026-08-01' && text('activity-commits-count') === '2', 'The commits curve must answer the pointer');
    fits('The calendar view');
    calendarTab.focus(); key(calendarTab, 'Home');
    check(document.activeElement === sizeTab && !sizePanel.hidden && calendarPanel.hidden, 'Home must return to the size view');
    key(sizeTab, 'ArrowRight');
    check(document.activeElement === calendarTab && calendarTab.getAttribute('aria-selected') === 'true' && !calendarPanel.hidden && sizePanel.hidden, 'ArrowRight must open the calendar view');
    sizeTab.click();
    check(!sizePanel.hidden && calendarPanel.hidden, 'The size tab must reopen the size view');
    selected('2026-08-21', 'Go production', '300', '120', '30');
  } catch (error) { failures.push(String(error)); }
  await fetch('/size-observation', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(failures)});
  const done = document.createElement('p'); done.textContent = 'size-proof-complete'; document.body.append(done);
});
</script>`
