package browsertest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// rfcIndex is data/rfc/index.json as internal/le/site/rfcdata.go writes it:
// the bar layout (rfcBars), then every RFC and profile with its file. The
// layout is restated here because this package cannot reach the site
// package's unexported value; TestPerRFCDataCountsEverySegment and
// TestTheBarLayoutKeepsTheAgreedOrder hold the Go side to the same names.
const rfcIndex = `{
  "bars": {
    "must": [
      {"segment": "ze-proven", "label": "Ze proven", "fields": ["must.ze-proven"]},
      {"segment": "os-verified", "label": "OS verified", "fields": ["must.os-verified"]},
      {"segment": "os-gap", "label": "OS testing gap", "fields": ["must.os-gap"]},
      {"segment": "ze-gap", "label": "Ze correctness gap", "fields": ["must.ze-gap"]}
    ],
    "must-total": ["must.kept"],
    "overview": [
      {"segment": "ze-proven", "label": "Ze MUST proven", "fields": ["must.ze-proven"]},
      {"segment": "os-verified", "label": "OS verified", "fields": ["must.os-verified"]},
      {"segment": "should-proven", "label": "SHOULD proven", "fields": ["should.proven"]},
      {"segment": "not-owed", "label": "Not owed", "fields": ["must.not-owed", "should.not-owed"]},
      {"segment": "not-ever", "label": "SHOULD not ever", "fields": ["should.not-ever"]},
      {"segment": "not-yet", "label": "SHOULD not yet", "fields": ["should.not-yet"]},
      {"segment": "not-decided", "label": "SHOULD not decided", "fields": ["should.not-decided"]},
      {"segment": "os-gap", "label": "OS testing gap", "fields": ["must.os-gap"]},
      {"segment": "ze-gap", "label": "Ze correctness gap", "fields": ["must.ze-gap"]}
    ],
    "overview-total": ["must.total", "should.total"]
  },
  "rfcs": [
    {"stem": "rfc1", "file": "data/rfc/rfc1.json", "walked": true, "enrolment": "enrolled"},
    {"stem": "rfc2", "file": "data/rfc/rfc2.json", "walked": true, "enrolment": "enrolled"},
    {"stem": "rfc3", "file": "data/rfc/rfc3.json", "walked": false, "enrolment": "enrolled"},
    {"stem": "rfc4", "file": "data/rfc/rfc4.json", "walked": true, "enrolment": "enrolled"}
  ],
  "profiles": [{"name": "edge", "file": "data/rfc/profiles/edge.json", "kind": "use-case", "title": "Edge"}]
}`

// rfcStemFile is one per-RFC file with the MUST and SHOULD counts given.
func rfcStemFile(stem string, zeProven, osVerified, osGap, zeGap, mustNotOwed int, should [6]int) string {
	kept := zeProven + osVerified + osGap + zeGap
	body, err := json.Marshal(map[string]any{
		"stem": stem, "walked": true, "unmet": []string{}, "proven-by-kind": map[string]int{},
		"must": map[string]any{"total": kept + mustNotOwed, "kept": kept, "ze-proven": zeProven,
			"os-verified": osVerified, "os-gap": osGap, "ze-gap": zeGap, "not-owed": mustNotOwed,
			"not-owed-by-kind": map[string]int{"not-applicable": mustNotOwed}},
		"should": map[string]any{"total": should[0], "proven": should[1], "not-owed": should[2],
			"not-ever": should[3], "not-yet": should[4], "not-decided": should[5],
			"not-owed-by-kind": map[string]int{"not-applicable": should[2]}},
	})
	if err != nil {
		panic("BUG: a fixed map does not encode: " + err.Error())
	}
	return string(body)
}

const rfcFiguresPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>rfc fixture</title></head>
<body>
<span id="stem" data-ze-rfc="rfc1:must.ze-proven">fallback</span>
<span id="unwalked" data-ze-rfc="rfc3:must.ze-proven">fallback</span>
<span id="profile-gap" data-ze-rfc="profile/edge/required:must.ze-gap">fallback</span>
<span id="profile-unwalked" data-ze-rfc="profile/edge/required:not-walked">fallback</span>
<span id="optional-proven" data-ze-rfc="profile/edge/optional:must.ze-proven">fallback</span>
<div id="must-bar" data-ze-rfc-bar="profile/edge/required:must">fallback</div>
<div id="overview-bar" data-ze-rfc-bar="rfc1:overview">fallback</div>
<script src="assets/rfc-figures.js" defer></script>
</body>
</html>
`

// rfcPageState is what the page shows: the marked texts, and each bar's
// segments left to right as class name and text, with the first segment's
// width.
type rfcPageState struct {
	Texts    map[string]string   `json:"texts"`
	Bars     map[string][]string `json:"bars"`
	Widths   map[string]string   `json:"widths"`
	Fallback bool                `json:"fallback"`
}

const readRFCPage = `(function () {
    var texts = {}, bars = {}, widths = {}, fallback = false;
    ["stem", "unwalked", "profile-gap", "profile-unwalked", "optional-proven"].forEach(function (id) {
        texts[id] = document.getElementById(id).textContent;
        if (texts[id] === "fallback") fallback = true;
    });
    ["must-bar", "overview-bar"].forEach(function (id) {
        var node = document.getElementById(id);
        if (node.textContent === "fallback") fallback = true;
        var spans = node.querySelectorAll(".rfc-seg");
        bars[id] = spans.length ? Array.prototype.map.call(spans, function (span) {
            return span.getAttribute("data-segment") + "=" + span.textContent;
        }) : [node.textContent];
        widths[id] = spans.length ? spans[0].style.width : "";
    });
    return {texts: texts, bars: bars, widths: widths, fallback: fallback};
})()`

// waitForRFCPage polls until no marked element shows its fallback text.
func waitForRFCPage(ctx context.Context, t *testing.T) rfcPageState {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var got rfcPageState
	for time.Now().Before(deadline) {
		var err error
		got, err = chromedp.Run(ctx, chromedp.Evaluate[rfcPageState](readRFCPage))
		if err != nil {
			t.Fatalf("read the page: %v", err)
		}
		if !got.Fallback {
			return got
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the page kept a fallback text: %+v", got)
	return got
}

// TestTheBrowserDrawsRFCFiguresFromTheirFiles proves AC-10's sums, AC-12's
// segment order and text, AC-17 and AC-18 for the renderer: a page shows each
// RFC's counts from its own file, sums each profile tier over its own walked
// members with the unwalked one counted apart, draws both bars in the
// declared order with each segment's name and count as text, shows new counts
// when one RFC's file is republished with no page changed, and names a
// missing file rather than dropping it from a sum.
//
// Method: serve a fixture site carrying the real rfc-figures.js, load the page
// in headless Chrome, read every marked element and segment, then change one
// file, reload and read again, then delete one file, reload and read again.
func TestTheBrowserDrawsRFCFiguresFromTheirFiles(t *testing.T) {
	chrome := findChrome(t)
	dir := t.TempDir()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "website", "assets", "js", "rfc-figures.js"))
	if err != nil {
		t.Fatalf("read the renderer: %v", err)
	}
	write := func(name, content string) {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("assets/rfc-figures.js", string(script))
	write("index.html", rfcFiguresPage)
	write("data/rfc/index.json", rfcIndex)
	write("data/rfc/profiles/edge.json", `{"name":"edge","kind":"use-case","title":"Edge","release":true,"required":["rfc1","rfc2","rfc3"],"optional":["rfc4"]}`)
	write("data/rfc/rfc1.json", rfcStemFile("rfc1", 3, 2, 3, 6, 6, [6]int{10, 1, 2, 1, 2, 4}))
	write("data/rfc/rfc2.json", rfcStemFile("rfc2", 4, 0, 0, 1, 0, [6]int{0, 0, 0, 0, 0, 0}))
	write("data/rfc/rfc4.json", rfcStemFile("rfc4", 9, 0, 0, 0, 0, [6]int{0, 0, 0, 0, 0, 0}))
	write("data/rfc/rfc3.json", `{"stem":"rfc3","walked":false,"must":null,"should":null,"proven-by-kind":null,"unmet":[]}`)

	files := http.FileServer(http.Dir(dir))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	}))
	defer server.Close()

	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(t.Context(), options...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	ctx, cancelTimeout := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancelTimeout()

	if _, err := chromedp.Run(ctx, chromedp.Navigate(server.URL+"/")); err != nil {
		t.Fatalf("load: %v", err)
	}
	got := waitForRFCPage(ctx, t)
	wantTexts := map[string]string{"stem": "3", "unwalked": "not walked", "profile-gap": "7", "profile-unwalked": "1",
		"optional-proven": "9"}
	if !reflect.DeepEqual(got.Texts, wantTexts) {
		t.Errorf("marked values\n got: %v\nwant: %v", got.Texts, wantTexts)
	}
	wantBars := map[string][]string{
		"must-bar": {"ze-proven=Ze proven 7", "os-verified=OS verified 2", "os-gap=OS testing gap 3",
			"ze-gap=Ze correctness gap 7"},
		"overview-bar": {"ze-proven=Ze MUST proven 3", "os-verified=OS verified 2", "should-proven=SHOULD proven 1",
			"not-owed=Not owed 8", "not-ever=SHOULD not ever 1", "not-yet=SHOULD not yet 2",
			"not-decided=SHOULD not decided 4", "os-gap=OS testing gap 3", "ze-gap=Ze correctness gap 6"},
	}
	if !reflect.DeepEqual(got.Bars, wantBars) {
		t.Errorf("bar segments\n got: %v\nwant: %v", got.Bars, wantBars)
	}
	// Widths are counts over the bar's total: 7 of the 19 MUST rows kept, 3 of
	// the 30 rows of RFC 1.
	for bar, want := range map[string]float64{"must-bar": 700.0 / 19, "overview-bar": 10} {
		width, err := strconv.ParseFloat(strings.TrimSuffix(got.Widths[bar], "%"), 64)
		if err != nil || width < want-0.01 || width > want+0.01 {
			t.Errorf("%s first segment width %q, want %.2f%%", bar, got.Widths[bar], want)
		}
	}

	// AC-17: one RFC's file changes and the page, unchanged, shows it.
	write("data/rfc/rfc1.json", rfcStemFile("rfc1", 5, 2, 3, 4, 6, [6]int{10, 1, 2, 1, 2, 4}))
	if _, err := chromedp.Run(ctx, chromedp.Reload()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got = waitForRFCPage(ctx, t)
	if got.Texts["stem"] != "5" || got.Texts["profile-gap"] != "5" {
		t.Errorf("republished counts not shown: %v", got.Texts)
	}

	// AC-18: a file the index names is missing, and the page names it.
	if err := os.Remove(filepath.Join(dir, "data", "rfc", "rfc2.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := chromedp.Run(ctx, chromedp.Reload()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got = waitForRFCPage(ctx, t)
	for _, id := range []string{"profile-gap", "profile-unwalked"} {
		if got.Texts[id] != "missing data/rfc/rfc2.json" {
			t.Errorf("%s shows %q, want the missing file named", id, got.Texts[id])
		}
	}
	if !reflect.DeepEqual(got.Bars["must-bar"], []string{"missing data/rfc/rfc2.json"}) {
		t.Errorf("the MUST bar shows %v, want the missing file named", got.Bars["must-bar"])
	}
	if got.Texts["stem"] != "5" {
		t.Errorf("an RFC whose file is present lost its count: %q", got.Texts["stem"])
	}
}
