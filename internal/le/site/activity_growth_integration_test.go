// Design: website/AI.md -- the size view draws the committed size history; the cumulative curve the calendar's days.
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
// activity, an idle day and cells out of date order, so the cumulative curve
// must read the dated window: 10, 0 and 7 lines added and 2, 0 and 1 commits,
// with the padding cell's 999 lines and 99 commits outside it.
//
// The page is served with two markers stating figures the file contradicts,
// the size view's total and one count of the Go composition in the calendar
// view, as a page built before the file was last updated would, so the file's
// figures can reach the browser only through the script's refill.
//
// Geometry is measured in page pixels: a chart's viewBox x converts through
// its bounding box, and a date control's handle center is its input's left
// edge plus half the handle, plus the value's share of the input's width less
// one handle, with the handle's width read from the stylesheet's
// --growth-thumb, which fixes it.
//
// VALIDATES: AC-11: date selection by pointer, keyboard and scrubber on the
// size charts and on the cumulative curve, section selection, both views
// reachable both ways, the decline inside the chart, the readout agreeing with
// the file, the curve drawn, every chart scaling by its viewBox, both themes,
// nothing overflowing at 320px; AC-10: the page opens on the size view, and
// the script replaces a marker's stale built figure with the file's value.
// The owner's four corrections: the Go composition sits in the calendar view
// and its markers are refilled; each date control's track spans its charts'
// time line, from the first day's x to the last day's, and its handle stands
// on every chart's cursor on the first, a middle and the last day; the size
// readout's table keeps its width, position, column widths, row heights and
// row count across days and section selection, and carries no Columns
// control; the cumulative curve shows lines added under Added Lines and
// commits under Commits, says which in its sentence, readout label, slider
// and chart name, carries no heading, and keeps the selected date across the
// switch.
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
	// The build filled both markers with the fixture's newest values. Serving
	// stale ones instead leaves the script's refill as the only way the
	// observation below can read 2,344 and 10.
	for built, stale := range map[string]string{
		`<span data-ze-history="total:lines">2,344</span>`:               `<span data-ze-history="total:lines">1,000</span>`,
		`<span data-ze-history="go-production+go-tests:files">10</span>`: `<span data-ze-history="go-production+go-tests:files">99</span>`,
	} {
		if count := strings.Count(page, built); count != 1 {
			t.Fatalf("the built page states %s %d times, want once", built, count)
		}
		page = strings.Replace(page, built, stale, 1)
	}
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
	// The first agent-browser command of a new session starts Chromium, and how
	// long that takes depends on the machine's load: half a second when it is
	// quiet, more than thirty when other test runs share it. The assertions are
	// about the page, not about how fast a browser starts, so the bound covers a
	// slow start as well as the page; a hang still ends, after two minutes.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	session := "activity-size-" + rand.Text()
	defer func() {
		// WithoutCancel drops ctx's deadline and cancellation, so the close gets
		// its own bound even after ctx expired. A close that follows a start
		// still under way can wait for that start, so its bound allows a slow
		// one: a loaded run killed the viewport command and then this close.
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
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
  const settle = () => new Promise(resolve => setTimeout(resolve, 0));
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
  function xs(path) {
    return (path.getAttribute('d').match(/-?[\d.]+ -?[\d.]+/g) || []).map(pair => Number(pair.split(' ')[0]));
  }
  function pageX(chart, viewX) {
    const box = chart.getBoundingClientRect();
    return box.left + viewX / chart.viewBox.baseVal.width * box.width;
  }
  function cursorX(chart) { return pageX(chart, xs(chart.querySelector('.growth-cursor'))[0]); }
  function thumb(input) { return parseFloat(getComputedStyle(input).getPropertyValue('--growth-thumb')); }
  function handleX(input) {
    const box = input.getBoundingClientRect(), max = Number(input.max);
    return box.left + thumb(input) / 2 + (max ? Number(input.value) / max : 0) * (box.width - thumb(input));
  }
  function near(a, b) { return Math.abs(a - b) <= 1.5; }
  function handleOnCursor(input, charts, what) {
    for (const chart of charts) {
      check(near(handleX(input), cursorX(chart)), what + ': the handle center at ' + handleX(input) + 'px must stand on the cursor of ' + chart.id + ' at ' + cursorX(chart) + 'px');
    }
  }
  // The drawn track is the input less half a handle at each end, the span the
  // handle's center travels.
  function trackOnTimeLine(input, chart, first, last, what) {
    const box = input.getBoundingClientRect(), half = thumb(input) / 2;
    check(near(box.left + half, pageX(chart, first)) && near(box.right - half, pageX(chart, last)),
      what + ': the track runs from ' + (box.left + half) + 'px to ' + (box.right - half) + 'px, the time line from ' + pageX(chart, first) + 'px to ' + pageX(chart, last) + 'px');
  }
  function readoutLayout() {
    const readout = document.getElementById('activity-size-readout').getBoundingClientRect();
    const table = document.querySelector('#activity-size-readout table');
    const box = table.getBoundingClientRect();
    return JSON.stringify({rows: table.rows.length, left: box.left - readout.left, top: box.top - readout.top,
      width: box.width, height: box.height, readout: readout.height,
      columns: Array.from(table.rows[0].cells).map(cell => cell.getBoundingClientRect().width),
      heights: Array.from(table.rows).map(tr => tr.getBoundingClientRect().height)});
  }
  try {
    const explorer = document.querySelector('.size-explorer');
    for (let wait = 0; explorer.hidden && wait < 50; wait++) await new Promise(resolve => setTimeout(resolve, 100));
    check(!explorer.hidden, 'The charts must show once the history file is read: ' + text('activity-size-readout'));
    // Every measurement of the readout's layout is taken once the fonts are
    // in, so a late font cannot read as the table moving with the data.
    await document.fonts.ready;
    await settle();
    const sizeTab = document.getElementById('activity-tab-size');
    const calendarTab = document.getElementById('activity-tab-calendar');
    const sizePanel = document.getElementById('activity-size');
    const calendarPanel = document.getElementById('activity-calendar');
    check(shown(sizeTab) && shown(calendarTab), 'Both tabs must show once the script runs');
    check(sizeTab.getAttribute('aria-selected') === 'true' && !sizePanel.hidden && calendarPanel.hidden,
      'The size view must be the view the page opens on');
    check(!shown(document.getElementById('total-value')), 'The calendar cards must hide with the calendar view');
    check(document.querySelector('[data-ze-history="total:lines"]').textContent === '2,344', 'The script must refill the stale built total from the file');
    const goPanel = document.querySelector('.go-panel');
    check(!!goPanel && calendarPanel.contains(goPanel) && !sizePanel.contains(goPanel), 'The Go composition must sit in the calendar view and not in the size view');
    check(!shown(goPanel), 'The Go composition must hide with the calendar view');
    check(document.querySelector('[data-ze-history="go-production+go-tests:files"]').textContent === '10', 'The script must refill the Go composition from the file');
    for (const id of ['activity-size-chart', 'activity-change-chart', 'activity-cumulative-chart']) {
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
    const chartBox = chart.getBoundingClientRect(), changeBox = change.getBoundingClientRect();
    check(near(chartBox.left, changeBox.left) && near(chartBox.width, changeBox.width),
      'The two size charts must plot over the same span: ' + [chartBox.left, chartBox.width] + ' and ' + [changeBox.left, changeBox.width]);
    const day = document.getElementById('activity-size-day');
    const productionX = xs(production);
    trackOnTimeLine(day, chart, productionX[0], productionX[productionX.length - 1], 'The size view date control');
    handleOnCursor(day, [chart, change], 'The size view on its last day');
    const table = document.querySelector('#activity-size-readout table');
    check(!!table && table.rows.length === 7, 'The readout must hold a heading, one row per section and the sum: ' + (table && table.rows.length));
    check(!document.querySelector('#activity-size-readout .table-column-selector'), 'The readout must carry no Columns control');
    const layout = readoutLayout();
    function sameLayout(what) { const now = readoutLayout(); check(now === layout, 'The readout table must keep its size ' + what + ': ' + now + ' differs from ' + layout); }
    selected('2026-08-23', 'Go production', '190', '15', '5');
    selected('2026-08-23', 'All sections', '2,344', '1,294', '15');
    fits('The size view');
    chart.focus(); key(chart, 'Home'); selected('2026-08-19', 'Go production', '120', '120', '0');
    handleOnCursor(day, [chart, change], 'The size view on its first day');
    sameLayout('on the first day');
    key(chart, 'ArrowRight'); selected('2026-08-20', 'Go production', '210', '100', '10');
    sameLayout('on the second day');
    key(chart, 'End'); selected('2026-08-23', 'Plans', '95', '40', '5');
    handleOnCursor(day, [chart, change], 'The size view on its last day, by keyboard');
    day.value = '3'; day.dispatchEvent(new Event('input', {bubbles:true}));
    selected('2026-08-22', 'Plans', '60', '0', '80');
    sameLayout('on the fourth day');
    const rect = chart.getBoundingClientRect();
    chart.dispatchEvent(new PointerEvent('pointermove', {clientX:rect.left + rect.width * 0.55, clientY:rect.top + rect.height / 2}));
    selected('2026-08-21', 'Go production', '300', '120', '30');
    handleOnCursor(day, [chart, change], 'The size view on its middle day');
    const other = legend.find(button => button.textContent === 'Other');
    other.click();
    check(other.getAttribute('aria-pressed') === 'false' && chart.querySelectorAll('.size-line').length === 4, 'Deselecting a section must remove its line');
    const levels = Array.from(chart.querySelectorAll('.growth-axis')).map(label => label.textContent);
    check(levels.includes('650') && !levels.includes('1,234'), 'The scale must follow the drawn sections: ' + levels);
    selected('2026-08-21', 'Go production', '300', '120', '30');
    selected('2026-08-21', 'Other', '25', '15', '0');
    sameLayout('with a section deselected');
    handleOnCursor(day, [chart, change], 'The size view with a section deselected');
    other.click();
    check(chart.querySelectorAll('.size-line').length === 5, 'Reselecting a section must draw it again');
    sameLayout('with the section reselected');
    document.documentElement.dataset.theme = 'light';
    const light = getComputedStyle(chart.querySelector('.size-line')).stroke;
    document.documentElement.dataset.theme = 'dark';
    const dark = getComputedStyle(chart.querySelector('.size-line')).stroke;
    check(light !== dark, 'The section colors must follow the theme');
    calendarTab.click();
    check(calendarPanel.hidden === false && sizePanel.hidden, 'The calendar tab must open the calendar view');
    check(shown(document.getElementById('total-value')), 'The calendar cards must show with the calendar view');
    check(shown(goPanel), 'The Go composition must show with the calendar view');
    const curve = document.getElementById('activity-cumulative-chart');
    const curveDay = document.getElementById('activity-cumulative-day');
    check(shown(curve), 'The cumulative curve must show under the calendar');
    const curveSeries = curve.querySelector('.growth-series');
    check(!!curveSeries && curveSeries.getAttribute('d').length > 12, 'The cumulative curve must plot the calendar days');
    function curveReads(metric, date, value, what) {
      const lines = metric === 'lines';
      const sentence = lines ? "Lines added by commits, accumulated since the calendar's first day." : "Commits accumulated since the calendar's first day.";
      const label = lines ? 'Lines added' : 'Commits';
      const unit = lines ? 'lines added' : 'commits';
      const name = lines ? 'Cumulative lines added by commits.' : 'Cumulative commits.';
      check(text('activity-cumulative-date') === date && text('activity-cumulative-value') === value,
        what + ': the curve must read ' + date + ' ' + value + ', reads ' + text('activity-cumulative-date') + ' ' + text('activity-cumulative-value'));
      check(document.querySelector('.activity-cumulative h2') === null, what + ': the curve must carry no heading');
      check(document.querySelector('.activity-cumulative p').textContent === sentence, what + ': the curve must say ' + sentence);
      check(document.querySelector('#activity-cumulative-readout span span').textContent === label, what + ': the readout must be labeled ' + label);
      check(curveDay.getAttribute('aria-valuetext') === date + ': ' + value + ' ' + unit, what + ': the slider must read out ' + date + ': ' + value + ' ' + unit + ', reads ' + curveDay.getAttribute('aria-valuetext'));
      check(curve.getAttribute('aria-label').startsWith(name), what + ': the chart must be named ' + name);
    }
    function curveLevels() { return Array.from(curve.querySelectorAll('.growth-axis')).map(label => label.textContent); }
    check(document.body.dataset.metric === 'lines', 'The calendar must open on Added Lines');
    curveReads('lines', '2026-08-03', '17', 'Under Added Lines');
    check(curveLevels().includes('17'), 'The curve must scale to the lines added in the window: ' + curveLevels());
    const curveX = xs(curveSeries);
    trackOnTimeLine(curveDay, curve, curveX[1], curveX[curveX.length - 1], 'The calendar curve date control');
    handleOnCursor(curveDay, [curve], 'The calendar curve on its last date');
    curveDay.value = '1'; curveDay.dispatchEvent(new Event('input', {bubbles:true}));
    curveReads('lines', '2026-08-02', '10', 'The scrubber under Added Lines');
    handleOnCursor(curveDay, [curve], 'The calendar curve on its middle date');
    document.querySelector('[data-metric="commits"]').click();
    await settle();
    check(shown(curve) && text('total-value') === '3', 'The curve and the commit cards must show while the switch says Commits');
    curveReads('commits', '2026-08-02', '2', 'After switching to Commits');
    check(curveLevels().includes('3') && !curveLevels().includes('17'), 'The curve must scale to the commits in the window: ' + curveLevels());
    curveDay.value = '0'; curveDay.dispatchEvent(new Event('input', {bubbles:true}));
    curveReads('commits', '2026-08-01', '2', 'The scrubber under Commits');
    handleOnCursor(curveDay, [curve], 'The calendar curve on its first date');
    curve.focus(); key(curve, 'ArrowRight');
    curveReads('commits', '2026-08-02', '2', 'The keyboard under Commits');
    const curveRect = curve.getBoundingClientRect();
    curve.dispatchEvent(new PointerEvent('pointermove', {clientX:curveRect.left + curveRect.width * 0.07, clientY:curveRect.top + curveRect.height / 2}));
    curveReads('commits', '2026-08-01', '2', 'The pointer under Commits');
    key(curve, 'End'); curveReads('commits', '2026-08-03', '3', 'The End key under Commits'); key(curve, 'Home'); curveReads('commits', '2026-08-01', '2', 'The Home key under Commits');
    document.querySelector('[data-metric="lines"]').click();
    await settle();
    check(shown(curve), 'The curve must show while the switch says Added Lines');
    curveReads('lines', '2026-08-01', '10', 'After switching back to Added Lines');
    key(curve, 'End');
    curveReads('lines', '2026-08-03', '17', 'The End key under Added Lines');
    handleOnCursor(curveDay, [curve], 'The calendar curve on its last date, by keyboard');
    fits('The calendar view');
    calendarTab.focus(); key(calendarTab, 'Home');
    check(document.activeElement === sizeTab && !sizePanel.hidden && calendarPanel.hidden, 'Home must return to the size view');
    key(sizeTab, 'ArrowRight');
    check(document.activeElement === calendarTab && calendarTab.getAttribute('aria-selected') === 'true' && !calendarPanel.hidden && sizePanel.hidden, 'ArrowRight must open the calendar view');
    sizeTab.click();
    check(!sizePanel.hidden && calendarPanel.hidden, 'The size tab must reopen the size view');
    selected('2026-08-21', 'Go production', '300', '120', '30');
    sameLayout('after the views were switched');
  } catch (error) { failures.push(String(error)); }
  await fetch('/size-observation', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(failures)});
  const done = document.createElement('p'); done.textContent = 'size-proof-complete'; document.body.append(done);
});
</script>`
