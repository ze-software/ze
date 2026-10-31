package browsertest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// rfcProfilePageGolden is internal/le/site/testdata/rfc-profile-page.json:
// the edge profile page the site producer renders and the RFC data it reads,
// which TestTheProfilePageGoldenIsCurrent holds to the producer.
type rfcProfilePageGolden struct {
	Page  string            `json:"page"`
	Files map[string]string `json:"files"`
}

// rfcProfilePageSite serves the golden's files, the real rfc-figures.js and
// the site's CSS, and answers the page's URL.
func rfcProfilePageSite(t *testing.T) (dir, pageURL string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "testdata", "rfc-profile-page.json"))
	if err != nil {
		t.Fatalf("read the profile page golden: %v. Write it with: ./le job run label unit-pkg command go test -run TestTheProfilePageGoldenIsCurrent ./internal/le/site/ -args -update", err)
	}
	var golden rfcProfilePageGolden
	if err := json.Unmarshal(body, &golden); err != nil {
		t.Fatalf("the profile page golden: %v", err)
	}
	dir = t.TempDir()
	site := &rfcFixtureSite{dir: dir, t: t}
	for name, content := range golden.Files {
		site.write(name, content)
	}
	assets := filepath.Join("..", "..", "..", "..", "website", "assets")
	for _, rel := range []string{"js/rfc-figures.js", "css/00-tokens.css", "css/30-components.css"} {
		content, err := os.ReadFile(filepath.Join(assets, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		target := "assets/" + rel
		if rel == "js/rfc-figures.js" {
			target = "assets/rfc-figures.js"
		}
		site.write(target, string(content))
	}
	files := http.FileServer(http.Dir(dir))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return dir, server.URL + "/" + strings.TrimSuffix(golden.Page, "index.html")
}

// rfcProfilePageState is what the page shows once the script ran: each bar's
// segments and legend items as segment=text, and each marked value. Values
// keys a value by its marker alone, and where two blocks of one page mark the
// same value the filtered list's #rfc-overview wins; Scoped keys each value
// inside an identified .rfc-overview block as "<block id>|<marker>", so a
// test reads a block the filter never narrows apart from the list.
type rfcProfilePageState struct {
	Bars    map[string][]string `json:"bars"`
	Legends map[string][]string `json:"legends"`
	Values  map[string]string   `json:"values"`
	Scoped  map[string]string   `json:"scoped"`
	Drawn   bool                `json:"drawn"`
}

const readRFCProfilePage = `(function () {
    var bars = {}, legends = {}, values = {}, scoped = {}, drawn = true;
    Array.prototype.forEach.call(document.querySelectorAll("[data-ze-rfc-bar]"), function (node) {
        var key = node.getAttribute("data-ze-rfc-bar");
        var spans = node.querySelectorAll(".rfc-seg");
        if (!spans.length && !node.classList.contains("rfc-figure-error") &&
            node.textContent !== "not walked" && node.textContent !== "no rows") drawn = false;
        bars[key] = spans.length ? Array.prototype.map.call(spans, function (span) {
            return span.getAttribute("data-segment") + "=" + span.textContent;
        }) : [node.textContent];
        legends[key] = Array.prototype.map.call(node.querySelectorAll(".rfc-seg-legend li"), function (item) {
            var swatch = item.querySelector(".rfc-seg-swatch");
            return item.getAttribute("data-segment") + "=" + item.textContent +
                (swatch && swatch.classList.contains("rfc-seg-" + item.getAttribute("data-segment")) ? "" : " (no swatch)");
        });
    });
    Array.prototype.forEach.call(document.querySelectorAll("[data-ze-rfc]"), function (node) {
        var key = node.getAttribute("data-ze-rfc");
        var block = node.closest(".rfc-overview[id]");
        if (block) {
            scoped[block.id + "|" + key] = node.textContent;
        }
        if (!(key in values) || (block && block.id === "rfc-overview")) {
            values[key] = node.textContent;
        }
    });
    return {bars: bars, legends: legends, values: values, scoped: scoped, drawn: drawn};
})()`

// loadRFCProfilePage opens the page and polls until every bar is drawn and
// done answers true over what it shows.
func loadRFCProfilePage(ctx context.Context, t *testing.T, pageURL string, done func(*rfcProfilePageState) bool) rfcProfilePageState {
	t.Helper()
	if _, err := chromedp.Run(ctx, chromedp.Navigate(pageURL)); err != nil {
		t.Fatalf("open %s: %v", pageURL, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	var got rfcProfilePageState
	for time.Now().Before(deadline) {
		var err error
		got, err = chromedp.Run(ctx, chromedp.Evaluate[rfcProfilePageState](readRFCProfilePage))
		if err != nil {
			t.Fatalf("read the page: %v", err)
		}
		if got.Drawn && done(&got) {
			return got
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the page did not reach the expected state: %+v", got)
	return got
}

// TestTheBrowserDrawsAProfilePageFromItsFiles proves the profile page the
// build renders draws both tiers' bars and legends in a real browser from the
// RFC data, shows a republished file's counts with no page changed (AC-17),
// and names a missing file rather than dropping it from a sum (AC-18).
//
// Method: serve the golden page and its files, read every bar and value;
// rewrite rfc1.json with other counts and reload; remove rfc2.json and reload.
// PREVENTS: a profile page whose markers the script cannot read, a page that
// prints the build's figures forever, and a sum that silently loses an RFC.
func TestTheBrowserDrawsAProfilePageFromItsFiles(t *testing.T) {
	dir, pageURL := rfcProfilePageSite(t)
	ctx := newRFCBrowser(t)
	// The build's fallback text equals what the script writes, so a page
	// whose script never ran would pass every check: seed a fallback that
	// differs in every marked element first.
	page := filepath.Join(dir, "quality", "rfc-compliance", "profiles", "edge", "index.html")
	built, err := os.ReadFile(page) //nolint:gosec // the test's own temporary site
	if err != nil {
		t.Fatal(err)
	}
	seeded := regexp.MustCompile(`(data-ze-rfc(?:-bar)?="[^"]+">)[^<]*(<)`).ReplaceAllString(string(built), "${1}fallback${2}")
	if err := os.WriteFile(page, []byte(seeded), 0o600); err != nil {
		t.Fatal(err)
	}

	got := loadRFCProfilePage(ctx, t, pageURL, func(state *rfcProfilePageState) bool {
		for _, value := range state.Values {
			if value == "fallback" {
				return false
			}
		}
		return true
	})
	mustBar := []string{"ze-proven=Ze proven 7", "os-verified=OS verified 2", "os-gap=OS testing gap 1",
		"ze-gap=Ze correctness gap 1,200"}
	for key, want := range map[string][]string{
		"profile/edge/required:must": mustBar,
		"profile/edge/required:overview": {"ze-proven=Ze MUST proven 7", "os-verified=OS verified 3",
			"should-proven=SHOULD proven 1", "not-owed=Not owed 12", "not-ever=SHOULD not ever 1",
			"not-yet=SHOULD not yet 2", "not-decided=SHOULD not decided 2", "os-gap=OS testing gap 2",
			"ze-gap=Ze correctness gap 1,200"},
		"profile/edge/optional:must": {"ze-proven=Ze proven 9"},
		"rfc3:must":                  {"not walked"},
		"rfc4:overview":              {"ze-proven=Ze MUST proven 9"},
	} {
		if !reflect.DeepEqual(got.Bars[key], want) {
			t.Errorf("bar %s draws %q, want %q", key, got.Bars[key], want)
		}
	}
	if !reflect.DeepEqual(got.Legends["profile/edge/required:must"], mustBar) {
		t.Errorf("the required MUST legend is %q, want %q", got.Legends["profile/edge/required:must"], mustBar)
	}
	for key, want := range map[string]string{
		"profile/edge/required:register": "not complete", "profile/edge/optional:register": "complete",
		"profile/edge/required:must.ze-gap": "1,200", "profile/edge/required:not-walked": "1",
		"profile/edge/required:must.not-owed-by-kind.linux-limit": "0",
	} {
		if got.Values[key] != want {
			t.Errorf("value %s shows %q, want %q", key, got.Values[key], want)
		}
	}

	// Registers each kept open by one clause alone, drawn into elements added
	// here: lab (a gapless RFC and an unwalked one), rfc5 (only an OS gap),
	// rfc6 (only a Ze gap); rfc2, with neither, is complete.
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(function () {
        ["profile/lab/required", "rfc5", "rfc6", "rfc2"].forEach(function (subject) {
            var node = document.createElement("span");
            node.setAttribute("data-ze-rfc", subject + ":register");
            node.textContent = "fallback";
            document.body.appendChild(node);
        });
        window.zeRFCFigures.fill(document.body);
        return true;
    })()`)); err != nil {
		t.Fatalf("add the register probes: %v", err)
	}
	// Each probe fills when its own subject's files arrive, and rfc2's are
	// already loaded, so the wait holds until every probe left its fallback.
	registers := map[string]string{"profile/lab/required": "not complete", "rfc5": "not complete",
		"rfc6": "not complete", "rfc2": "complete"}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		got, err = chromedp.Run(ctx, chromedp.Evaluate[rfcProfilePageState](readRFCProfilePage))
		if err != nil {
			t.Fatalf("read the page: %v", err)
		}
		filled := 0
		for subject := range registers {
			if value := got.Values[subject+":register"]; value != "" && value != "fallback" {
				filled++
			}
		}
		if filled == len(registers) {
			break
		}
	}
	for subject, want := range registers {
		if got.Values[subject+":register"] != want {
			t.Errorf("%s:register shows %q, want %q", subject, got.Values[subject+":register"], want)
		}
	}

	before, err := os.ReadFile(page) //nolint:gosec // the test's own temporary site
	if err != nil {
		t.Fatal(err)
	}
	stem := filepath.Join(dir, "data", "rfc", "rfc1.json")
	original, err := os.ReadFile(stem) //nolint:gosec // the test's own temporary site
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(original, &file); err != nil {
		t.Fatal(err)
	}
	must, isMap := file["must"].(map[string]any)
	if !isMap {
		t.Fatalf("rfc1.json holds no must counts: %s", original)
	}
	must["ze-proven"], must["ze-gap"] = float64(103), float64(1100)
	republished, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stem, republished, 0o600); err != nil {
		t.Fatal(err)
	}
	got = loadRFCProfilePage(ctx, t, pageURL, func(state *rfcProfilePageState) bool {
		return state.Values["profile/edge/required:must.ze-proven"] == "107"
	})
	if want := "ze-gap=Ze correctness gap 1,100"; got.Bars["profile/edge/required:must"][3] != want {
		t.Errorf("after republishing rfc1.json the required MUST bar is %q, want its last segment %q",
			got.Bars["profile/edge/required:must"], want)
	}
	after, err := os.ReadFile(page) //nolint:gosec // the test's own temporary site
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("the page file changed: the republished counts must reach it through the files alone")
	}

	if err := os.Remove(filepath.Join(dir, "data", "rfc", "rfc2.json")); err != nil {
		t.Fatal(err)
	}
	got = loadRFCProfilePage(ctx, t, pageURL, func(state *rfcProfilePageState) bool {
		return strings.Contains(strings.Join(state.Bars["profile/edge/required:must"], ""), "data/rfc/rfc2.json")
	})
	if text := got.Values["profile/edge/required:rfcs"]; !strings.Contains(text, "data/rfc/rfc2.json") ||
		!strings.Contains(text, "./le site build") {
		t.Errorf("with rfc2.json missing the RFC count shows %q, want the file and the command named", text)
	}
	if want := []string{"ze-proven=Ze proven 9"}; !reflect.DeepEqual(got.Bars["profile/edge/optional:must"], want) {
		t.Errorf("the optional tier, which does not count rfc2, draws %q, want %q", got.Bars["profile/edge/optional:must"], want)
	}
}
