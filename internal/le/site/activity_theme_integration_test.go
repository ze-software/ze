// Design: website/AI.md -- activity styling follows the shared site theme.
//go:build integration

package site

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	webtesting "github.com/ze-software/ze/internal/component/web/testing"
)

type activityThemeSample struct {
	Theme          string `json:"theme"`
	Card           string `json:"card"`
	Panel          string `json:"panel"`
	Ink            string `json:"ink"`
	Border         string `json:"border"`
	ExpectedCard   string `json:"expectedCard"`
	ExpectedPanel  string `json:"expectedPanel"`
	ExpectedInk    string `json:"expectedInk"`
	ExpectedBorder string `json:"expectedBorder"`
	Idle           string `json:"idle"`
	Active         string `json:"active"`
}

// TestActivityStyleUsesTheme drives the generated activity page and deployed
// stylesheet through real Chromium. It reads computed styles in both themes,
// including zero-activity and active cells; source-token matches cannot pass it.
// MUTATION: Replacing --activity-card/ink/border with light-only literals must
// fail the dark sample even when the page still contains its dark-theme rule.
func TestActivityStyleUsesTheme(t *testing.T) {
	paths := activityFixturePaths(t)
	if _, err := renderActivityPage(paths); err != nil {
		t.Fatal(err)
	}
	if err := renderCSS(paths.Source, paths.Output); err != nil {
		t.Fatal(err)
	}
	if err := renderJS(paths.Source, paths.Output); err != nil {
		t.Fatal(err)
	}
	pagePath := filepath.Join(paths.Output, filepath.FromSlash(activityDest))
	page, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	page = []byte(strings.Replace(string(page), "</body>", activityThemeObservation+"</body>", 1))
	samples := make(chan []activityThemeSample, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+activityRoute, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write(page); err != nil {
			t.Errorf("serve activity: %v", err)
		}
	})
	mux.HandleFunc("POST /theme-observation", func(w http.ResponseWriter, r *http.Request) {
		var observed []activityThemeSample
		if err := json.NewDecoder(io.LimitReader(r.Body, 32*1024)).Decode(&observed); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		select {
		case samples <- observed:
		default:
			t.Error("duplicate theme observation")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", http.FileServer(http.Dir(paths.Output)))
	server := httptest.NewServer(mux)
	defer server.Close()
	journey, err := webtesting.ParseWBFile("action=open:path=" + activityRoute + "\nexpect=html:contains=theme-proof-complete\n")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	result := webtesting.RunWBCase(ctx, journey, server.URL, "")
	if !result.Passed {
		t.Fatalf("activity Chromium journey: %s", result.Error)
	}
	var observed []activityThemeSample
	select {
	case observed = <-samples:
	case <-ctx.Done():
		t.Fatal("Chromium did not return computed styles")
	}
	if len(observed) != 2 {
		t.Fatalf("theme samples = %d, want light and dark", len(observed))
	}
	for idx, sample := range observed {
		if sample.Theme != []string{"light", "dark"}[idx] {
			t.Errorf("theme sample %d = %q", idx, sample.Theme)
		}
		if sample.Card != sample.ExpectedCard {
			t.Errorf("%s card = %s, theme surface %s", sample.Theme, sample.Card, sample.ExpectedCard)
		}
		if sample.Panel != sample.ExpectedPanel {
			t.Errorf("%s panel = %s, theme surface %s", sample.Theme, sample.Panel, sample.ExpectedPanel)
		}
		if sample.Ink != sample.ExpectedInk {
			t.Errorf("%s ink = %s, theme text %s", sample.Theme, sample.Ink, sample.ExpectedInk)
		}
		if sample.Border != sample.ExpectedBorder {
			t.Errorf("%s border = %s, theme border %s", sample.Theme, sample.Border, sample.ExpectedBorder)
		}
		if sample.Idle == sample.Active {
			t.Errorf("%s active and idle cells are indistinguishable", sample.Theme)
		}
	}
	if observed[0].Card == observed[1].Card {
		t.Error("activity card did not respond to theme change")
	}
	if observed[0].Ink == observed[1].Ink {
		t.Error("activity ink did not respond to theme change")
	}
	if observed[0].Active == observed[1].Active {
		t.Error("activity heatmap did not respond to theme change")
	}
}

// The observer selects the actual CSS theme and reports browser-resolved role
// values. It does not replace activity markup or any stylesheet declaration.
const activityThemeObservation = `<script>
window.addEventListener('load', async () => {
  const root = document.documentElement;
  const widget = document.querySelector('.activity-widget');
  const probe = document.createElement('span');
  widget.append(probe);
  function role(name) {
    probe.style.color = 'var(' + name + ')';
    return getComputedStyle(probe).color;
  }
  const idle = document.createElement('span');
  idle.className = 'day-cell'; idle.dataset.level = '0'; widget.append(idle);
  const active = document.createElement('span');
  active.className = 'day-cell'; active.dataset.level = '5'; widget.append(active);
  const samples = [];
  for (const theme of ['light', 'dark']) {
    root.dataset.theme = theme;
    const card = getComputedStyle(widget.querySelector('.stat'));
    const panel = getComputedStyle(widget.querySelector('.panel'));
    const ink = getComputedStyle(widget.querySelector('.stat strong'));
    samples.push({theme, card:card.backgroundColor, panel:panel.backgroundColor,
      ink:ink.color, border:card.borderTopColor,
      expectedCard:role('--panel'), expectedPanel:role('--panel-strong'),
      expectedInk:role('--text'), expectedBorder:role('--line-strong'),
      idle:getComputedStyle(idle).backgroundColor, active:getComputedStyle(active).backgroundColor});
  }
  await fetch('/theme-observation', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(samples)});
  const done = document.createElement('p'); done.textContent='theme-proof-complete'; document.body.append(done);
});
</script>`
