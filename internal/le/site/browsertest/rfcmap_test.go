// Design: website/AI.md -- the RFC map reads recorded inputs without changing release obligations.
// Related: ../rfcmap.go publishes the page; ../rfcmap_model.go projects its recorded inputs.
package browsertest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// TestRFCMapBrowserInteraction drives the canonical generated page in Chrome.
// It checks persistent scope, keyboard selection, declared layer/profile
// membership, exact keyword filters, phone layout and useful load failure.
// Expected identities come from the published input, not graph internals.
func TestRFCMapBrowserInteraction(t *testing.T) {
	base, model, responses := rfcMapBrowserSite(t)
	ctx := newRFCBrowser(t)
	pageURL := base + "/quality/rfc-map/"

	t.Run("all_scope_survives_reload", func(t *testing.T) {
		rfcMapBrowserOpen(ctx, t, pageURL)
		if _, err := chromedp.Run(ctx, chromedp.Click(chromedp.CSS(".rfc-map-scope-row button"))); err != nil {
			t.Fatal(err)
		}
		rfcMapBrowserMembers(ctx, t, model.nodeLabels())
		url, err := chromedp.Run(ctx, chromedp.Evaluate[string]("location.href"))
		if err != nil {
			t.Fatal(err)
		}
		rfcMapBrowserOpen(ctx, t, url)
		rfcMapBrowserMembers(ctx, t, model.nodeLabels())
	})

	t.Run("keyboard_selection_keeps_focus", func(t *testing.T) {
		rfcMapBrowserOpen(ctx, t, pageURL)
		if _, err := chromedp.Run(ctx, chromedp.Click(chromedp.CSS(".rfc-map-scope-row button"))); err != nil {
			t.Fatal(err)
		}
		if _, err := chromedp.Run(ctx, chromedp.SendKeys(chromedp.CSS("#rfc-map-search"), "RFC 2545")); err != nil {
			t.Fatal(err)
		}
		rfcMapBrowserMembers(ctx, t, []string{"RFC 2545"})
		if _, err := chromedp.Run(ctx, chromedp.Focus(chromedp.CSS(".rfc-map-document"))); err != nil {
			t.Fatal(err)
		}
		if _, err := chromedp.Run(ctx, chromedp.KeyEvent(kb.Enter)); err != nil {
			t.Fatal(err)
		}
		rfcMapBrowserWait(ctx, t, `document.querySelector('#rfc-map-inspector h2')?.textContent === 'RFC 2545'`)
		focused, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
            const active = document.activeElement;
            return active.matches('.rfc-map-document[aria-pressed="true"], .rfc-map-node[aria-pressed="true"]') ||
                active.closest('#rfc-map-inspector') !== null;
        })()`))
		if err != nil {
			t.Fatal(err)
		}
		if !focused {
			t.Fatal("keyboard selection discarded focus instead of retaining the selected control or its inspector")
		}
	})

	t.Run("declared_scopes_and_keyword_filters", func(t *testing.T) {
		rfcMapBrowserOpen(ctx, t, pageURL)
		rfcMapBrowserMembers(ctx, t, []string{"RFC 4271"})
		rfcMapBrowserChoose(ctx, t, "rfc-map-layer", "ipv6-unicast")
		rfcMapBrowserMembers(ctx, t, []string{"RFC 2545", "RFC 4760"})
		profileFound := false
		for _, profile := range model.Profiles {
			if profile.Kind != "use-case" {
				continue
			}
			if len(profile.Required) == 0 {
				continue
			}
			if len(profile.Optional) == 0 {
				continue
			}
			rfcMapBrowserChoose(ctx, t, "rfc-map-profile", profile.ID)
			rfcMapBrowserMembers(ctx, t, rfcMapBrowserLabels(profile.Required))
			profileFound = true
			break
		}
		if !profileFound {
			t.Fatal("published model has no use-case profile with required and optional members to distinguish")
		}
		rfcMapBrowserOpen(ctx, t, pageURL)
		rfcMapBrowserWait(ctx, t, `document.querySelector('.rfc-map-proof-value') !== null`)
		proof, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('.rfc-map-proof-card').textContent`))
		if err != nil {
			t.Fatal(err)
		}
		for _, level := range []string{"MUST", "SHOULD", "MAY"} {
			rfcMapBrowserChoose(ctx, t, "rfc-map-level", level)
			var want []string
			for _, row := range model.Requirements["rfc4271"] {
				if row.Level == level {
					want = append(want, row.RID)
				}
			}
			if len(want) == 0 {
				t.Fatalf("base BGP has no %s rows, so the published input cannot exercise this keyword", level)
			}
			got, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](`Array.from(document.querySelectorAll('.rfc-map-statement-heading a'), node => node.textContent)`))
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(want)
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("%s keyword shows %v, want exactly %v", level, got, want)
			}
			after, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('.rfc-map-proof-card').textContent`))
			if err != nil {
				t.Fatal(err)
			}
			if after != proof {
				t.Errorf("%s keyword changed the full mandatory proof card", level)
			}
		}
	})

	t.Run("phone_layout_and_menu", func(t *testing.T) {
		if _, err := chromedp.Run(ctx, chromedp.EmulateViewport(375, 812)); err != nil {
			t.Fatal(err)
		}
		rfcMapBrowserOpen(ctx, t, pageURL)
		rfcMapBrowserWait(ctx, t, `document.fonts.status === 'loaded' && document.querySelector('header.site-header a[href$="quality/rfc-map/"]') !== null`)
		links, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
            const nav = document.querySelector('nav[aria-label="Related standards pages"]');
            if (!nav || nav.closest('#rfc-map')) return false;
            return ['../rfc-compliance/', '../rfc-compliance/profiles/', '../../project/roadmap/'].every(href =>
                Array.from(nav.querySelectorAll('a')).some(link => link.getAttribute('href') === href));
        })()`))
		if err != nil {
			t.Fatal(err)
		}
		if !links {
			t.Fatal("the enhanced map lost its persistent RFC evidence, profile or roadmap navigation")
		}
		overflow, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`document.documentElement.scrollWidth > innerWidth + 1`))
		if err != nil {
			t.Fatal(err)
		}
		if overflow {
			t.Fatal("the 375px page overflows horizontally instead of containing graph scrolling")
		}
	})

	t.Run("proof_states_remain_distinct", func(t *testing.T) {
		for _, state := range []struct {
			name  string
			mode  int32
			value string
			note  string
		}{
			{"unwalked", 1, "Unknown", "The requirement extraction is not signed off"},
			{"no_mandatory_rows", 2, "No MUST rows", "no mandatory requirements"},
			{"all_mandatory_rows_excluded", 3, "None owed", "not a 100% proof result"},
		} {
			t.Run(state.name, func(t *testing.T) {
				responses.Figure.Store(state.mode)
				rfcMapBrowserOpen(ctx, t, pageURL)
				rfcMapBrowserWait(ctx, t, `document.querySelector('.rfc-map-proof-value') !== null`)
				got, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('.rfc-map-proof-value').textContent`))
				if err != nil {
					t.Fatal(err)
				}
				if got != state.value {
					t.Errorf("proof state is %q, want %q", got, state.value)
				}
				text, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('.rfc-map-proof-card').textContent`))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(text, state.note) {
					t.Errorf("proof card does not explain %q: %s", state.note, text)
				}
			})
		}
		responses.Figure.Store(0)
	})

	t.Run("document_without_summary_stays_unknown", func(t *testing.T) {
		if responses.NoSummary == "" {
			t.Fatal("the canonical graph has no text-only document to exercise unknown proof")
		}
		responses.NoSummaryRequests.Store(0)
		rfcMapBrowserOpen(ctx, t, pageURL+"?rfc="+responses.NoSummary)
		rfcMapBrowserWait(ctx, t, `document.querySelector('.rfc-map-proof-value') !== null`)
		got, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('.rfc-map-proof-value').textContent`))
		if err != nil {
			t.Fatal(err)
		}
		if got != "Unknown" {
			t.Errorf("a document without a summary shows %q, want Unknown", got)
		}
		if got := responses.NoSummaryRequests.Load(); got != 0 {
			t.Errorf("a document without a summary fetched its nonexistent proof file %d times", got)
		}
	})

	t.Run("missing_input_keeps_static_navigation", func(t *testing.T) {
		responses.Missing.Store(true)
		if _, err := chromedp.Run(ctx, chromedp.Navigate(pageURL)); err != nil {
			t.Fatal(err)
		}
		rfcMapBrowserWait(ctx, t, `document.querySelector('#rfc-map [role="alert"]') !== null`)
		useful, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
            const root = document.querySelector('#rfc-map');
            const alert = root.querySelector('[role="alert"]');
            return alert.textContent.includes('HTTP 503') &&
                root.querySelector('.rfc-map-fallback a[href="../rfc-compliance/"]') !== null &&
                root.querySelector('.rfc-map-fallback a[href="../rfc-compliance/profiles/"]') !== null &&
                root.querySelector('.rfc-map-fallback a[href="../../project/roadmap/"]') !== null;
        })()`))
		if err != nil {
			t.Fatal(err)
		}
		if !useful {
			t.Fatal("a failed map input lost its named HTTP error or static evidence, profile and roadmap links")
		}
	})
}

// rfcMapBrowserModel reads identities and declarations from the published
// projection. Its zero value carries no valid test population.
type rfcMapBrowserModel struct {
	Nodes []struct {
		Stem    string `json:"stem"`
		Summary bool   `json:"summary"`
	} `json:"nodes"`
	Profiles []struct {
		ID       string   `json:"id"`
		Kind     string   `json:"kind"`
		Required []string `json:"required"`
		Optional []string `json:"optional"`
	} `json:"profiles"`
	Requirements map[string][]struct {
		RID   string `json:"rid"`
		Level string `json:"level"`
	} `json:"requirements"`
}

func (m *rfcMapBrowserModel) nodeLabels() []string {
	stems := make([]string, 0, len(m.Nodes))
	for _, node := range m.Nodes {
		stems = append(stems, node.Stem)
	}
	return rfcMapBrowserLabels(stems)
}

func rfcMapBrowserLabels(stems []string) []string {
	labels := make([]string, 0, len(stems))
	for _, stem := range stems {
		label := stem
		if strings.HasPrefix(stem, "rfc") {
			label = "RFC " + strings.TrimPrefix(stem, "rfc")
		}
		labels = append(labels, label)
	}
	slices.Sort(labels)
	return labels
}

// rfcMapBrowserResponses controls only test HTTP replies. Figure zero serves
// the canonical file; the nonzero modes cover unavailable proof populations.
type rfcMapBrowserResponses struct {
	Missing           atomic.Bool
	Figure            atomic.Int32
	NoSummaryRequests atomic.Int32
	NoSummary         string
}

// rfcMapBrowserSite serves the canonical artifact without copying it or
// rebuilding unrelated pages. The HTTP failure switch changes no source file.
func rfcMapBrowserSite(t *testing.T) (string, rfcMapBrowserModel, *rfcMapBrowserResponses) {
	t.Helper()
	output, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "gh-pages"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(output, "data", "rfc-map.json"))
	if err != nil {
		t.Fatalf("read the canonical RFC map (render it with ./le --name session site build producer rfc-map): %v", err)
	}
	var model rfcMapBrowserModel
	if err := json.Unmarshal(body, &model); err != nil {
		t.Fatalf("read the published RFC map model: %v", err)
	}
	if len(model.Nodes) < 2 {
		t.Fatal("the published graph has too few nodes to test changes of scope")
	}
	responses := new(rfcMapBrowserResponses)
	for _, node := range model.Nodes {
		if !node.Summary {
			responses.NoSummary = node.Stem
			break
		}
	}
	files := http.FileServer(http.Dir(output))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/data/rfc-map.json" {
			if responses.Missing.Load() {
				http.Error(w, "map input deliberately unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		if r.URL.Path == "/data/rfc/"+responses.NoSummary+".json" {
			responses.NoSummaryRequests.Add(1)
		}
		if r.URL.Path == "/data/rfc/rfc4271.json" {
			if payload := rfcMapBrowserFigure(responses.Figure.Load()); payload != "" {
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write([]byte(payload)); err != nil {
					t.Errorf("write the synthetic proof response: %v", err)
				}
				return
			}
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL, model, responses
}

// rfcMapBrowserFigure uses the canonical field shape to distinguish missing
// extraction, an empty register and a register with every obligation excluded.
func rfcMapBrowserFigure(mode int32) string {
	if mode == 1 {
		return `{"stem":"rfc4271","walked":false}`
	}
	if mode == 2 {
		return `{"stem":"rfc4271","walked":true,"must":{"kept":0,"total":0,"ze-proven":0,"os-verified":0,"proof-pending":0,"not-built":0,"not-assessed":0,"os-gap":0,"exclusion-unchecked":0,"not-owed":0}}`
	}
	if mode == 3 {
		return `{"stem":"rfc4271","walked":true,"must":{"kept":0,"total":3,"ze-proven":0,"os-verified":0,"proof-pending":0,"not-built":0,"not-assessed":0,"os-gap":0,"exclusion-unchecked":0,"not-owed":3}}`
	}
	return ""
}

func rfcMapBrowserOpen(ctx context.Context, t *testing.T, url string) {
	t.Helper()
	if _, err := chromedp.Run(ctx, chromedp.Navigate(url)); err != nil {
		t.Fatal(err)
	}
	rfcMapBrowserWait(ctx, t, `document.querySelector('#rfc-map .rfc-map-app') !== null`)
}

func rfcMapBrowserWait(ctx context.Context, t *testing.T, condition string) {
	t.Helper()
	if _, err := chromedp.Run(ctx, chromedp.Poll[bool](condition, chromedp.WithPollingTimeout(10*time.Second))); err != nil {
		t.Fatalf("the RFC map did not reach %s: %v", condition, err)
	}
}

func rfcMapBrowserMembers(ctx context.Context, t *testing.T, want []string) {
	t.Helper()
	got, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](`Array.from(document.querySelectorAll('.rfc-map-document strong'), node => node.textContent)`))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("documents in view = %v, want exactly %v", got, want)
	}
}

func rfcMapBrowserChoose(ctx context.Context, t *testing.T, id, value string) {
	t.Helper()
	changed, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
        const select = document.getElementById(`+strconv.Quote(id)+`);
        select.value = `+strconv.Quote(value)+`;
        if (select.value !== `+strconv.Quote(value)+`) return false;
        select.dispatchEvent(new Event('change', {bubbles: true}));
        return true;
    })()`))
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatalf("%s does not offer %q", id, value)
	}
}
