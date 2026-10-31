package browsertest

import (
	"context"
	"encoding/json"
	"errors"
	"math"
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

// rfcGolden is internal/le/site/testdata/rfc-bars.json, which
// TestTheBarLayoutGoldenIsCurrent holds to the site package's rfcBars and to
// the per-RFC file it writes: this package cannot reach those unexported
// values, so it reads what they marshal to rather than restating them.
type rfcGolden struct {
	Bars json.RawMessage `json:"bars"`
	Stem map[string]any  `json:"stem"`
}

func readRFCGolden(t *testing.T) rfcGolden {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "testdata", "rfc-bars.json"))
	if err != nil {
		t.Fatalf("read the bar layout golden: %v. Write it with: ./le job run label unit-pkg command go test -run TestTheBarLayoutGoldenIsCurrent ./internal/le/site/ -args -update", err)
	}
	var golden rfcGolden
	if err := json.Unmarshal(body, &golden); err != nil {
		t.Fatalf("the bar layout golden: %v", err)
	}
	return golden
}

// rfcIndexOf is data/rfc/index.json over the golden's bar layout: four RFCs,
// one of them unwalked, and two profiles.
func rfcIndexOf(t *testing.T, golden *rfcGolden) string {
	t.Helper()
	rfcs := []map[string]any{}
	for _, one := range []struct {
		stem   string
		walked bool
	}{{"rfc1", true}, {"rfc2", true}, {"rfc3", false}, {"rfc4", true}} {
		rfcs = append(rfcs, map[string]any{"stem": one.stem, "file": "data/rfc/" + one.stem + ".json",
			"walked": one.walked, "enrolment": "enrolled"})
	}
	body, err := json.Marshal(map[string]any{"bars": golden.Bars, "rfcs": rfcs, "profiles": []map[string]string{
		{"name": "edge", "file": "data/rfc/profiles/edge.json", "kind": "use-case", "title": "Edge"},
		{"name": "core", "file": "data/rfc/profiles/core.json", "kind": "use-case", "title": "Core"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// fieldPaths answers the dotted keys of a per-RFC file, not descending into a
// by-kind map, whose keys are data rather than shape.
func fieldPaths(prefix string, value map[string]any, into map[string]bool) {
	for key, child := range value {
		path := prefix + key
		into[path] = true
		nested, isMap := child.(map[string]any)
		if !isMap || strings.HasSuffix(key, "-by-kind") {
			continue
		}
		fieldPaths(path+".", nested, into)
	}
}

// rfcStemFile is one per-RFC file with the MUST and SHOULD counts given.
func rfcStemFile(stem string, zeProven, osVerified, osGap, zeGap, mustNotOwed int, should [6]int) string {
	kept := zeProven + osVerified + osGap + zeGap
	body, err := json.Marshal(map[string]any{
		"stem": stem, "title": "RFC " + stem, "walked": true, "unmet": []string{}, "proven-by-kind": map[string]int{},
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
<span id="empty-optional" data-ze-rfc="profile/core/optional:must.ze-proven">fallback</span>
<span id="by-kind" data-ze-rfc="rfc1:must.not-owed-by-kind.not-applicable">fallback</span>
<span id="by-kind-absent" data-ze-rfc="rfc1:must.not-owed-by-kind.feature-declined">fallback</span>
<span id="bad-field" data-ze-rfc="rfc1:must.no-such-count">fallback</span>
<div id="empty-bar" data-ze-rfc-bar="profile/core/optional:must">fallback</div>
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
	Legends  map[string][]string `json:"legends"`
	Widths   map[string]string   `json:"widths"`
	Fallback bool                `json:"fallback"`
}

const readRFCPage = `(function () {
    var texts = {}, bars = {}, legends = {}, widths = {}, fallback = false;
    ["stem", "unwalked", "profile-gap", "profile-unwalked", "optional-proven", "empty-optional", "by-kind",
        "by-kind-absent", "bad-field"].forEach(function (id) {
        texts[id] = document.getElementById(id).textContent;
        if (texts[id] === "fallback") fallback = true;
    });
    ["must-bar", "overview-bar", "empty-bar"].forEach(function (id) {
        var node = document.getElementById(id);
        if (node.textContent === "fallback") fallback = true;
        var spans = node.querySelectorAll(".rfc-seg");
        bars[id] = spans.length ? Array.prototype.map.call(spans, function (span) {
            return span.getAttribute("data-segment") + "=" + span.textContent;
        }) : [node.textContent];
        widths[id] = spans.length ? spans[0].style.width : "";
        legends[id] = Array.prototype.map.call(node.querySelectorAll(".rfc-seg-legend li"), function (item) {
            var swatch = item.querySelector(".rfc-seg-swatch");
            return item.getAttribute("data-segment") + "=" + item.textContent +
                (swatch && swatch.classList.contains("rfc-seg-" + item.getAttribute("data-segment")) ? "" : " (no swatch)");
        });
    });
    return {texts: texts, bars: bars, legends: legends, widths: widths, fallback: fallback};
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

// rfcFixtureSite is a served fixture site carrying the real rfc-figures.js
// and the site's CSS, with the index built from the golden layout.
type rfcFixtureSite struct {
	dir string
	url string
	t   *testing.T
}

func (site *rfcFixtureSite) write(name, content string) {
	site.t.Helper()
	path := filepath.Join(site.dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		site.t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		site.t.Fatalf("write %s: %v", name, err)
	}
}

// newRFCFixtureSite serves the fixture: RFC 1 holds a different count in
// every segment, RFC 2 and RFC 4 only MUST rows, RFC 3 is unwalked; profile
// edge requires RFC 1 to 3 and lists RFC 4 as optional, and profile core has
// an empty optional tier.
func newRFCFixtureSite(t *testing.T) *rfcFixtureSite {
	t.Helper()
	site := &rfcFixtureSite{dir: t.TempDir(), t: t}
	golden := readRFCGolden(t)
	assets := filepath.Join("..", "..", "..", "..", "website", "assets")
	for _, rel := range []string{"js/rfc-figures.js", "css/00-tokens.css", "css/30-components.css"} {
		body, err := os.ReadFile(filepath.Join(assets, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		target := rel
		if rel == "js/rfc-figures.js" {
			target = "rfc-figures.js"
		}
		site.write("assets/"+target, string(body))
	}
	site.write("index.html", rfcFiguresPage)
	site.write("data/rfc/index.json", rfcIndexOf(t, &golden))
	site.write("data/rfc/profiles/edge.json", `{"name":"edge","kind":"use-case","title":"Edge","release":true,"required":["rfc1","rfc2","rfc3"],"optional":["rfc4"]}`)
	site.write("data/rfc/profiles/core.json", `{"name":"core","kind":"use-case","title":"Core","release":false,"required":["rfc2"],"optional":[]}`)
	site.write("data/rfc/rfc1.json", rfcStemFile("rfc1", 3, 2, 3, 6, 6, [6]int{10, 1, 2, 1, 2, 4}))
	site.write("data/rfc/rfc2.json", rfcStemFile("rfc2", 4, 0, 0, 1, 0, [6]int{0, 0, 0, 0, 0, 0}))
	site.write("data/rfc/rfc4.json", rfcStemFile("rfc4", 9, 0, 0, 0, 0, [6]int{0, 0, 0, 0, 0, 0}))
	site.write("data/rfc/rfc3.json", `{"stem":"rfc3","walked":false,"must":null,"should":null,"proven-by-kind":null,"unmet":[]}`)

	files := http.FileServer(http.Dir(site.dir))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	site.url = server.URL
	return site
}

// newRFCBrowser answers a headless Chrome context that ends with the test.
func newRFCBrowser(t *testing.T) context.Context {
	t.Helper()
	chrome := findChrome(t)
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(t.Context(), options...)
	t.Cleanup(cancelAlloc)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	t.Cleanup(cancelBrowser)
	ctx, cancelTimeout := context.WithTimeout(browserCtx, 60*time.Second)
	t.Cleanup(cancelTimeout)
	return ctx
}

// TestTheGoldenStemFileHasTheFixtureShape proves the per-RFC fixtures this
// package serves carry exactly the fields the site package writes, read from
// the golden, so the renderer is tested against the published shape.
//
// Method: compare the dotted field paths of the golden's sample file with
// those of a fixture file.
func TestTheGoldenStemFileHasTheFixtureShape(t *testing.T) {
	golden := readRFCGolden(t)
	var fixture map[string]any
	if err := json.Unmarshal([]byte(rfcStemFile("rfc1", 1, 1, 1, 1, 1, [6]int{5, 1, 1, 1, 1, 1})), &fixture); err != nil {
		t.Fatal(err)
	}
	want, got := map[string]bool{}, map[string]bool{}
	fieldPaths("", golden.Stem, want)
	fieldPaths("", fixture, got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the fixture's per-RFC fields differ from the published file's\n got: %v\nwant: %v", got, want)
	}
}

// TestTheBrowserDrawsRFCFiguresFromTheirFiles proves AC-8, AC-10's sums,
// AC-12's segment order and text, AC-17 and AC-18 for the renderer: a page
// shows each RFC's counts from its own file, sums each profile tier over its
// own walked members with the unwalked one counted apart, shows `no rows` for
// an empty tier, draws both bars in the declared order with each segment's
// name and count as text and again in a legend beside the bar, shows 0 for a
// kind no row was excluded by, shows new counts when one RFC's file is
// republished with no page changed, and names a missing file, the index
// included, with the command that publishes it.
//
// Method: serve a fixture site carrying the real rfc-figures.js and the bar
// layout golden, load the page in headless Chrome, read every marked element,
// segment and legend item, then change one file, reload and read again, then
// delete one file, then the index, reloading and reading after each.
func TestTheBrowserDrawsRFCFiguresFromTheirFiles(t *testing.T) {
	site := newRFCFixtureSite(t)
	ctx := newRFCBrowser(t)

	if _, err := chromedp.Run(ctx, chromedp.Navigate(site.url+"/")); err != nil {
		t.Fatalf("load: %v", err)
	}
	got := waitForRFCPage(ctx, t)
	wantTexts := map[string]string{"stem": "3", "unwalked": "not walked", "profile-gap": "7", "profile-unwalked": "1",
		"optional-proven": "9", "empty-optional": "no rows", "by-kind": "6", "by-kind-absent": "0",
		"bad-field": "must.no-such-count is not a field of the RFC data, so nothing can be shown here. " +
			"Fix the data-ze-rfc marker: docs/architecture/site-facts.md lists the fields."}
	if !reflect.DeepEqual(got.Texts, wantTexts) {
		t.Errorf("marked values\n got: %v\nwant: %v", got.Texts, wantTexts)
	}
	wantBars := map[string][]string{
		"must-bar": {"ze-proven=Ze proven 7", "os-verified=OS verified 2", "os-gap=OS testing gap 3",
			"ze-gap=Ze correctness gap 7"},
		"overview-bar": {"ze-proven=Ze MUST proven 3", "os-verified=OS verified 2", "should-proven=SHOULD proven 1",
			"not-owed=Not owed 8", "not-ever=SHOULD not ever 1", "not-yet=SHOULD not yet 2",
			"not-decided=SHOULD not decided 4", "os-gap=OS testing gap 3", "ze-gap=Ze correctness gap 6"},
		"empty-bar": {"no rows"},
	}
	if !reflect.DeepEqual(got.Bars, wantBars) {
		t.Errorf("bar segments\n got: %v\nwant: %v", got.Bars, wantBars)
	}
	// AC-12: a segment too narrow for its words keeps them in the legend, which
	// names every non-zero segment in the bar's order with its colour swatch.
	wantLegends := map[string][]string{"must-bar": wantBars["must-bar"], "overview-bar": wantBars["overview-bar"],
		"empty-bar": {}}
	if !reflect.DeepEqual(got.Legends, wantLegends) {
		t.Errorf("bar legends\n got: %v\nwant: %v", got.Legends, wantLegends)
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
	site.write("data/rfc/rfc1.json", rfcStemFile("rfc1", 5, 2, 3, 4, 6, [6]int{10, 1, 2, 1, 2, 4}))
	if _, err := chromedp.Run(ctx, chromedp.Reload()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got = waitForRFCPage(ctx, t)
	if got.Texts["stem"] != "5" || got.Texts["profile-gap"] != "5" {
		t.Errorf("republished counts not shown: %v", got.Texts)
	}

	// AC-18: a file the index names is missing, and the page names it.
	if err := os.Remove(filepath.Join(site.dir, "data", "rfc", "rfc2.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := chromedp.Run(ctx, chromedp.Reload()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got = waitForRFCPage(ctx, t)
	const missing = "data/rfc/rfc2.json could not be read, so this figure would leave that RFC out. " +
		"Run ./le site build to publish data/rfc/."
	for _, id := range []string{"profile-gap", "profile-unwalked"} {
		if got.Texts[id] != missing {
			t.Errorf("%s shows %q, want the missing file named", id, got.Texts[id])
		}
	}
	if !reflect.DeepEqual(got.Bars["must-bar"], []string{missing}) {
		t.Fatalf("the MUST bar shows %v, want the missing file named", got.Bars["must-bar"])
	}
	if got.Texts["stem"] != "5" {
		t.Errorf("an RFC whose file is present lost its count: %q", got.Texts["stem"])
	}

	// The index itself is missing: every figure names it and the command.
	if err := os.Remove(filepath.Join(site.dir, "data", "rfc", "index.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := chromedp.Run(ctx, chromedp.Reload()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got = waitForRFCPage(ctx, t)
	const noIndex = "data/rfc/index.json could not be read, so no RFC figure on this page can be shown. " +
		"Run ./le site build to publish data/rfc/."
	for _, id := range []string{"stem", "profile-gap"} {
		if got.Texts[id] != noIndex {
			t.Errorf("%s shows %q, want the missing index named", id, got.Texts[id])
		}
	}
}

// rfcColours is each overview segment's text colour and fills, and the bar's
// own background, as the browser computes them.
type rfcColours struct {
	Bar      string              `json:"bar"`
	Text     map[string]string   `json:"text"`
	Fills    map[string][]string `json:"fills"`
	Segments []string            `json:"segments"`
}

const readRFCColours = `(function () {
    var bar = document.querySelector("#overview-bar .rfc-seg-bar");
    var out = {bar: bar ? getComputedStyle(bar).backgroundColor : "", text: {}, fills: {}, segments: []};
    Array.prototype.forEach.call(document.querySelectorAll("#overview-bar .rfc-seg"), function (span) {
        var name = span.getAttribute("data-segment"), style = getComputedStyle(span);
        var fills = (style.backgroundImage.match(/rgba?\([^)]*\)/g) || []);
        if (!fills.length) fills = [style.backgroundColor];
        out.segments.push(name);
        out.text[name] = style.color;
        out.fills[name] = fills;
    });
    return out;
})()`

// parseCSSColour answers an opaque rgb() or rgba() colour as three channels;
// a colour that is not opaque cannot be judged and is an error.
func parseCSSColour(value string) ([3]float64, error) {
	var out [3]float64
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(value, "rgba("), "rgb("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) == 4 && strings.TrimSpace(parts[3]) != "1" {
		return out, errors.New(value + " is not opaque")
	}
	if len(parts) != 3 && len(parts) != 4 {
		return out, errors.New(value + " is not an rgb() colour")
	}
	for index := range 3 {
		channel, err := strconv.ParseFloat(strings.TrimSpace(parts[index]), 64)
		if err != nil {
			return out, err
		}
		out[index] = channel / 255
	}
	return out, nil
}

// relativeLuminance is WCAG 2.2's relative luminance of an sRGB colour.
func relativeLuminance(rgb [3]float64) float64 {
	linear := func(channel float64) float64 {
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(rgb[0]) + 0.7152*linear(rgb[1]) + 0.0722*linear(rgb[2])
}

// contrastRatio is WCAG 2.2's contrast ratio of two luminances.
func contrastRatio(first, second float64) float64 {
	return (math.Max(first, second) + 0.05) / (math.Min(first, second) + 0.05)
}

// TestRFCSegmentColoursAreReadableInBothThemes proves every segment's words
// meet WCAG 2.2 AA for small text (4.5:1) against every colour of its fill in
// the light and the dark theme; that OS verified, "light green", is lighter
// than Ze proven in both; that in the light theme it stands apart from SHOULD
// proven's teal; and that not decided has a fill of its own, apart from the
// bar's background and every other segment.
//
// Method: load the fixture page with the site's real tokens and component CSS
// under each data-theme, read each segment's computed colours, and compute the
// ratios here.
func TestRFCSegmentColoursAreReadableInBothThemes(t *testing.T) {
	site := newRFCFixtureSite(t)
	for _, theme := range []string{"light", "dark"} {
		site.write(theme+".html", strings.Replace(strings.Replace(rfcFiguresPage, `<html lang="en">`,
			`<html lang="en" data-theme="`+theme+`">`, 1), `<title>rfc fixture</title>`,
			`<title>rfc fixture</title><link rel="stylesheet" href="assets/css/00-tokens.css">`+
				`<link rel="stylesheet" href="assets/css/30-components.css">`, 1))
	}
	ctx := newRFCBrowser(t)
	for _, theme := range []string{"light", "dark"} {
		if _, err := chromedp.Run(ctx, chromedp.Navigate(site.url+"/"+theme+".html")); err != nil {
			t.Fatalf("load %s: %v", theme, err)
		}
		waitForRFCPage(ctx, t)
		colours, err := chromedp.Run(ctx, chromedp.Evaluate[rfcColours](readRFCColours))
		if err != nil {
			t.Fatalf("%s: read the colours: %v", theme, err)
		}
		if len(colours.Segments) != 9 {
			t.Fatalf("%s: %d overview segments drawn, want 9: %v", theme, len(colours.Segments), colours.Segments)
		}
		luminance := func(value string) float64 {
			rgb, err := parseCSSColour(value)
			if err != nil {
				t.Fatalf("%s: %v", theme, err)
			}
			return relativeLuminance(rgb)
		}
		fills := map[string]string{}
		for _, segment := range colours.Segments {
			text := luminance(colours.Text[segment])
			for _, fill := range colours.Fills[segment] {
				if ratio := contrastRatio(text, luminance(fill)); ratio < 4.5 {
					t.Errorf("%s %s: text %s on %s is %.2f:1, under 4.5:1", theme, segment, colours.Text[segment], fill, ratio)
				}
			}
			fills[segment] = colours.Fills[segment][0]
		}
		if luminance(fills["os-verified"]) <= luminance(fills["ze-proven"]) {
			t.Errorf("%s: OS verified %s is not lighter than Ze proven %s", theme, fills["os-verified"], fills["ze-proven"])
		}
		if theme == "light" {
			if ratio := contrastRatio(luminance(fills["os-verified"]), luminance(fills["should-proven"])); ratio < 1.4 {
				t.Errorf("light: OS verified %s and SHOULD proven %s differ by %.2f:1 in luminance, under 1.4:1",
					fills["os-verified"], fills["should-proven"], ratio)
			}
		}
		if ratio := contrastRatio(luminance(fills["not-decided"]), luminance(colours.Bar)); ratio < 1.4 {
			t.Errorf("%s: not decided %s is %.2f:1 against the bar's background %s, under 1.4:1",
				theme, fills["not-decided"], ratio, colours.Bar)
		}
		for _, segment := range colours.Segments {
			if segment != "not-decided" && fills[segment] == fills["not-decided"] {
				t.Errorf("%s: not decided shares its fill %s with %s", theme, fills["not-decided"], segment)
			}
		}
	}
}
