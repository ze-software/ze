package browsertest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// rfcCompliancePagesGolden is internal/le/site/testdata/rfc-compliance-pages.json:
// one RFC detail page and the compliance index the site producer renders,
// and the RFC data they read, which TestTheCompliancePagesGoldenIsCurrent
// holds to the producer.
type rfcCompliancePagesGolden struct {
	Detail string            `json:"detail"`
	List   string            `json:"list"`
	Files  map[string]string `json:"files"`
}

// rfcCompliancePagesSite serves the golden's files, the real rfc-figures.js
// and the site's CSS, seeds every marked element of both pages with the text
// "fallback" (the build's fallback equals what the script writes, so a page
// whose script never ran would otherwise pass), and answers the site's
// directory, the golden and the server's base URL.
func rfcCompliancePagesSite(t *testing.T) (string, rfcCompliancePagesGolden, string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "testdata", "rfc-compliance-pages.json"))
	if err != nil {
		t.Fatalf("read the compliance pages golden: %v. Write it with: ./le job run label unit-pkg command go test -run TestTheCompliancePagesGoldenIsCurrent ./internal/le/site/ -args -update", err)
	}
	var golden rfcCompliancePagesGolden
	if err := json.Unmarshal(body, &golden); err != nil {
		t.Fatalf("the compliance pages golden: %v", err)
	}
	dir := t.TempDir()
	site := &rfcFixtureSite{dir: dir, t: t}
	seed := regexp.MustCompile(`(data-ze-rfc(?:-bar)?="[^"]+">)[^<]*(<)`)
	for name, content := range golden.Files {
		if name == golden.Detail || name == golden.List {
			content = seed.ReplaceAllString(content, "${1}fallback${2}")
		}
		site.write(name, content)
	}
	assets := filepath.Join("..", "..", "..", "..", "website", "assets")
	for rel, target := range map[string]string{"js/rfc-figures.js": "assets/rfc-figures.js",
		"css/00-tokens.css": "assets/css/00-tokens.css", "css/30-components.css": "assets/css/30-components.css"} {
		content, err := os.ReadFile(filepath.Join(assets, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		site.write(target, string(content))
	}
	files := http.FileServer(http.Dir(dir))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return dir, golden, server.URL + "/"
}

// rfcNoFallbackLeft answers true once no marked element still shows the
// seeded fallback, so a wait covers every element the test asserts on.
func rfcNoFallbackLeft(state *rfcProfilePageState) bool {
	for _, value := range state.Values {
		if value == "fallback" {
			return false
		}
	}
	for _, segments := range state.Bars {
		if strings.Join(segments, "") == "fallback" {
			return false
		}
	}
	return true
}

// TestTheBrowserDrawsAnRFCDetailPageFromItsFile proves an RFC detail page
// draws its MUST and overview bars and fills its figure table in a real
// browser from data/rfc/<stem>.json (AC-11), in the AC-12 segment order, and
// shows a republished file's counts with no page file changed (AC-17).
//
// Method: serve the golden page with every marked element seeded "fallback",
// wait until none is left, read every bar and value; rewrite rfc1.json with
// other counts and wait for them.
// PREVENTS: a detail page whose markers the script cannot read, or that
// prints the build's figures forever.
func TestTheBrowserDrawsAnRFCDetailPageFromItsFile(t *testing.T) {
	dir, golden, base := rfcCompliancePagesSite(t)
	ctx := newRFCBrowser(t)
	pageURL := base + strings.TrimSuffix(golden.Detail, "index.html")
	got := loadRFCProfilePage(ctx, t, pageURL, rfcNoFallbackLeft)
	for key, want := range map[string][]string{
		"rfc1:must": {"ze-proven=Ze proven 3", "os-verified=OS verified 2", "os-gap=OS testing gap 1",
			"ze-gap=Ze correctness gap 1,200"},
		"rfc1:overview": {"ze-proven=Ze MUST proven 3", "os-verified=OS verified 3",
			"should-proven=SHOULD proven 1", "not-owed=Not owed 12", "not-ever=SHOULD not ever 1",
			"not-yet=SHOULD not yet 2", "not-decided=SHOULD not decided 2", "os-gap=OS testing gap 2",
			"ze-gap=Ze correctness gap 1,200"},
	} {
		if !reflect.DeepEqual(got.Bars[key], want) {
			t.Errorf("bar %s draws %q, want %q", key, got.Bars[key], want)
		}
	}
	for key, want := range map[string]string{
		"rfc1:must.total": "1,216", "rfc1:must.kept": "1,206", "rfc1:must.not-owed": "10",
		"rfc1:must.not-owed-by-kind.feature-declined": "6", "rfc1:must.not-owed-by-kind.linux-limit": "0",
		"rfc1:should.not-decided": "2", "rfc1:should.os-verified": "1",
	} {
		if got.Values[key] != want {
			t.Errorf("value %s shows %q, want %q", key, got.Values[key], want)
		}
	}
	for key := range got.Values {
		if !strings.HasPrefix(key, "rfc1:") {
			t.Errorf("the rfc1 page marks %s, a subject other than its own RFC", key)
		}
	}

	page := filepath.Join(dir, filepath.FromSlash(golden.Detail))
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
		return state.Values["rfc1:must.ze-proven"] == "103" && state.Values["rfc1:must.ze-gap"] == "1,100" &&
			len(state.Bars["rfc1:must"]) == 4 && state.Bars["rfc1:must"][3] == "ze-gap=Ze correctness gap 1,100"
	})
	if want := "ze-proven=Ze proven 103"; got.Bars["rfc1:must"][0] != want {
		t.Errorf("after republishing rfc1.json the MUST bar is %q, want its first segment %q", got.Bars["rfc1:must"], want)
	}
	after, err := os.ReadFile(page) //nolint:gosec // the test's own temporary site
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("the page file changed: the republished counts must reach it through the files alone")
	}
	if _, err := chromedp.Run(ctx, chromedp.Navigate("about:blank")); err != nil {
		t.Fatal(err)
	}
}
