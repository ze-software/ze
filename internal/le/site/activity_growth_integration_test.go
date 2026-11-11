// Design: website/AI.md -- the growth tab reuses the calendar's daily values.
//go:build integration

package site

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	reporewrite "github.com/ze-software/ze/internal/le/repo/rewrite"
)

// TestActivityGrowthUsesExistingDays drives both representations in Chromium.
// The fixture deliberately includes padding with activity, an idle day and
// cells out of date order, so a cumulative plot must read the dated window.
func TestActivityGrowthUsesExistingDays(t *testing.T) {
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
	window := reporewrite.ActivityWindow{
		Start:   time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		End:     time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
		Lines:   reporewrite.ActivitySeries{Total: 17, ActiveDays: 2, Peak: 10},
		Commits: reporewrite.ActivitySeries{Total: 3, ActiveDays: 2, Peak: 2},
		Cells: `<div class="day-cell outside" data-date="2026-07-31" data-lines="999" data-commits="99"></div>
<div class="day-cell" data-date="2026-08-03" data-date-label="Mon 03 Aug 2026" data-lines="7" data-lines-display="7" data-lines-level="4" data-commits="1" data-commits-display="1" data-commits-level="4"></div>
<div class="day-cell" data-date="2026-08-01" data-date-label="Sat 01 Aug 2026" data-lines="10" data-lines-display="10" data-lines-level="5" data-commits="2" data-commits-display="2" data-commits-level="5"></div>
<div class="day-cell" data-date="2026-08-02" data-date-label="Sun 02 Aug 2026" data-lines="0" data-lines-display="0" data-lines-level="0" data-commits="0" data-commits-display="0" data-commits-level="0"></div>`,
		Go: reporewrite.ActivityGo{
			Total:  reporewrite.ActivityGoBucket{TotalLines: 30},
			Code:   reporewrite.ActivityGoBucket{TotalLines: 10},
			Tests:  reporewrite.ActivityGoBucket{TotalLines: 20},
			Vendor: reporewrite.ActivityGoBucket{TotalLines: 40},
		},
	}
	page := pageShell{Title: "Activity growth", Root: activityRoot, Path: activityDest, ExtraHead: activityStyle}.
		render(activityBody(&window, activitySurfacePage) + activityGrowthObservation)
	observations := make(chan []string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+activityRoute, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := io.WriteString(w, page); err != nil {
			t.Errorf("serve growth: %v", err)
		}
	})
	mux.HandleFunc("POST /growth-observation", func(w http.ResponseWriter, r *http.Request) {
		var failures []string
		if err := json.NewDecoder(io.LimitReader(r.Body, 32*1024)).Decode(&failures); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		select {
		case observations <- failures:
		default:
			t.Error("duplicate growth observation")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", http.FileServer(http.Dir(paths.Output)))
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	session := "activity-growth-" + rand.Text()
	defer func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer stop()
		closeBrowser := exec.CommandContext(cleanup, "agent-browser", "--session", session, "close")
		closeBrowser.WaitDelay = 2 * time.Second
		if output, err := closeBrowser.CombinedOutput(); err != nil {
			t.Errorf("close owned growth browser: %v: %s", err, output)
		}
	}()
	// This static page reports every completed interaction through the POST
	// below; no request-idle observer is needed before its first navigation.
	openBrowser := exec.CommandContext(ctx, "agent-browser", "--session", session, "open", server.URL+activityRoute)
	openBrowser.WaitDelay = 2 * time.Second
	if output, err := openBrowser.CombinedOutput(); err != nil {
		t.Fatalf("open growth page in Chromium: %v: %s", err, output)
	}
	select {
	case failures := <-observations:
		for _, failure := range failures {
			t.Error(failure)
		}
	case <-ctx.Done():
		t.Fatal("Chromium did not report growth behavior")
	}
}

const activityGrowthObservation = `<script>
window.addEventListener('load', async () => {
  const failures = [];
  function check(ok, message) { if (!ok) failures.push(message); }
  function text(id) { return document.getElementById(id).textContent.trim(); }
  function key(el, value) { el.dispatchEvent(new KeyboardEvent('keydown', {key:value, bubbles:true})); }
  try {
    const calendar = document.getElementById('activity-tab-calendar');
    const growth = document.getElementById('activity-tab-growth');
    check(!!calendar && !!growth, 'Calendar and Growth tabs must exist');
    if (calendar && growth) {
      const calendarPanel = document.getElementById(calendar.getAttribute('aria-controls'));
      const growthPanel = document.getElementById(growth.getAttribute('aria-controls'));
      check(calendar.getAttribute('aria-selected') === 'true' && !calendarPanel.hidden && growthPanel.hidden,
        'Calendar must be the initial visible representation');
      calendar.focus(); key(calendar, 'ArrowRight');
      check(document.activeElement === growth && growth.getAttribute('aria-selected') === 'true',
        'ArrowRight must focus and activate Growth');
      check(calendarPanel.hidden && !growthPanel.hidden, 'Growth must replace the calendar panel');
      function metricVisible(metric) {
        for (const name of ['lines', 'commits']) {
          const graph = document.getElementById('activity-growth-' + name + '-chart');
          const shown = graph.getClientRects().length > 0 && getComputedStyle(graph).visibility !== 'hidden';
          check(shown === (name === metric), 'Growth must display only the selected ' + metric + ' chart; ' + name + ' visibility was ' + shown);
        }
      }
      metricVisible('lines');
      const day = document.getElementById('activity-growth-day');
      function values(date, lines, commits) {
        check(text('activity-growth-date').includes(date), 'Selected date must be ' + date);
        check(text('activity-growth-lines') === String(lines), 'Cumulative added lines must be ' + lines);
        check(text('activity-growth-commits') === String(commits), 'Cumulative commits must be ' + commits);
      }
      values('2026-08-03', 17, 3);
      document.querySelector('[data-metric="commits"]').click();
      metricVisible('commits');
      values('2026-08-03', 17, 3);
      document.querySelector('[data-metric="lines"]').click();
      metricVisible('lines');
      day.value = '0'; day.dispatchEvent(new Event('input', {bubbles:true}));
      values('2026-08-01', 10, 2);
      const chart = document.getElementById('activity-growth-lines-chart');
      chart.focus();
      key(chart, 'ArrowRight');
      values('2026-08-02', 10, 2);
      key(chart, 'End'); values('2026-08-03', 17, 3);
      const rect = chart.getBoundingClientRect();
      chart.dispatchEvent(new PointerEvent('pointermove', {clientX:rect.left + rect.width * 0.07, clientY:rect.top + rect.height / 2}));
      values('2026-08-01', 10, 2);
      check(text('activity-growth-code') === '10', 'Current code total must remain 10');
      check(text('activity-growth-tests') === '20', 'Current tests total must remain 20');
      check(text('activity-growth-vendor') === '40', 'Current third-party total must remain 40');
      check(text('activity-growth-overall') === '70', 'Overall must include code, tests and third party');
      for (const id of ['activity-growth-lines-chart', 'activity-growth-commits-chart']) {
        const svg = document.getElementById(id);
        const series = svg.querySelector('.growth-series');
        check(!!series && series.getAttribute('d').length > 12, id + ' must plot existing daily values');
        check(svg.getAttribute('viewBox') !== null, id + ' must scale responsively');
      }
      document.documentElement.dataset.theme = 'light';
      const light = getComputedStyle(chart.querySelector('.growth-series')).stroke;
      document.documentElement.dataset.theme = 'dark';
      const dark = getComputedStyle(chart.querySelector('.growth-series')).stroke;
      check(light !== dark, 'Growth series must respond to the selected theme');
      key(growth, 'Home');
      check(document.activeElement === calendar && !calendarPanel.hidden && growthPanel.hidden,
        'Home must return to Calendar');
      document.querySelector('[data-metric="commits"]').click();
      check(text('total-value') === '3', 'Calendar commit metric must still work');
      growth.click();
      metricVisible('commits');
      values('2026-08-01', 10, 2);
      calendar.click();
      document.querySelector('[data-metric="lines"]').click();
      growth.click();
      metricVisible('lines');
      values('2026-08-01', 10, 2);
    }
  } catch (error) { failures.push(String(error)); }
  await fetch('/growth-observation', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(failures)});
  const done = document.createElement('p'); done.textContent='growth-proof-complete'; document.body.append(done);
});
</script>`
