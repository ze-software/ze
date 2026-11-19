//go:build ze_web && ze_fleet_ui

// Design: docs/contributing/gh-pages.md -- the fleet demonstration the action writes, played in a browser.
// Related: replayproduct_test.go -- the player's product settings over a probe recording

package browsertest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/component/web/fleetui/sample"
	"github.com/ze-software/ze/internal/le/site"
	sitefleetdemo "github.com/ze-software/ze/internal/le/site/fleetdemo"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

const (
	// fleetDemoFiles is where the action writes the demonstration in an
	// output, and fleetDemoPage the host page the owner opens there.
	fleetDemoFiles = "/tmp/fleet-demo/"
	fleetDemoPage  = fleetDemoFiles + "index.html"
	fleetDemoFrame = fleetDemoFiles + "frame.html"
	fleetDemoIndex = "tmp/fleet-demo/recording.json"

	// fleetDemoRenderWait bounds the one render the tests share: the action's
	// own bound for the whole tour.
	fleetDemoRenderWait = 10 * time.Minute
	// fleetDemoWait bounds each wait for a state of the page, and
	// fleetDemoPoll is how often the state is read meanwhile.
	fleetDemoWait = 20 * time.Second
	fleetDemoPoll = 100 * time.Millisecond

	// fleetDemoCommitShown is how much of the commit the host bar shows, as
	// the looking glass replay's provenance line does.
	fleetDemoCommitShown = 10
	// fleetDemoTreeDepthMax bounds the walk down the estate tree to a device:
	// the default tree view's site and role levels, with room for a root row.
	fleetDemoTreeDepthMax = 4

	// What the demonstration shows for a request it did not record, and for
	// a posted action (the render's player settings and fallbacks).
	fleetDemoNotice = "This is not part of the recorded demonstration"
	fleetDemoToast  = "Recorded demonstration: this action was not sent"
	// fleetDemoFilter is a filter of the devices table no tour records.
	fleetDemoFilter = "zz-not-recorded"

	fleetDemoMount = `document.querySelector('[data-lg-replay]')`
	// fleetDemoPath is the path of the page the frame shows, which every
	// answer of the shell carries on its content region.
	fleetDemoPath = `document.getElementById('fleet-content-body')?.getAttribute('data-fleet-path')`

	// fleetDemoHTMXRequests are the attributes by which a control sends an
	// htmx request, and fleetDemoHTMXBound the attribute htmx puts on an
	// element once it listens to it (the vendored htmx 4, initialization).
	fleetDemoHTMXRequests = `['hx-get', 'hx-post', 'hx-put', 'hx-patch', 'hx-delete']`
	fleetDemoHTMXBound    = "data-htmx-powered"
)

// fleetDemoRender is the one render the fleet demonstration tests share. The
// whole tour takes minutes, so it is recorded once per run, by the first test
// that asks for it, into a scratch output that TestMain removes.
var fleetDemoRender struct {
	once       sync.Once
	output     string
	provenance sitelgreplay.Provenance
	err        error
}

// TestMain removes the shared render's scratch output once every test ran.
func TestMain(m *testing.M) {
	code := m.Run()
	if fleetDemoRender.output != "" {
		if err := os.RemoveAll(fleetDemoRender.output); err != nil {
			fmt.Fprintln(os.Stderr, "the fleet demonstration's scratch output was not removed:", err)
			code = 1
		}
	}
	os.Exit(code)
}

// fleetDemoOutput answers the scratch output holding the demonstration, and
// the provenance its recording names, rendering it on the first call.
func fleetDemoOutput(t *testing.T) (string, sitelgreplay.Provenance) {
	t.Helper()
	fleetDemoRender.once.Do(func() {
		fleetDemoRender.provenance, fleetDemoRender.err = renderFleetDemo()
	})
	if fleetDemoRender.err != nil {
		t.Fatalf("render the fleet demonstration: %v", fleetDemoRender.err)
	}
	return fleetDemoRender.output, fleetDemoRender.provenance
}

// renderFleetDemo lays down what the action asks of a built output (its
// index page, the site's stylesheet and the player), then records the whole
// tour into it through the action's own render. The output is kept in
// fleetDemoRender.output as soon as it exists, so TestMain removes it even
// when the render fails.
func renderFleetDemo() (sitelgreplay.Provenance, error) {
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		return sitelgreplay.Provenance{}, err
	}
	output, err := os.MkdirTemp("", "fleet-demo-browser-")
	if err != nil {
		return sitelgreplay.Provenance{}, err
	}
	fleetDemoRender.output = output
	player, err := os.ReadFile(filepath.Join(repository, filepath.FromSlash(site.ReplayPlayerSource)))
	if err != nil {
		return sitelgreplay.Provenance{}, err
	}
	for name, content := range map[string][]byte{
		"index.html":          []byte("<!doctype html><title>Scratch output</title>\n"),
		"assets/site.css":     []byte("/* the site */\n"),
		"assets/lg-replay.js": player,
	} {
		path := filepath.Join(output, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return sitelgreplay.Provenance{}, err
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return sitelgreplay.Provenance{}, err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), fleetDemoRenderWait)
	defer cancel()
	return sitefleetdemo.RenderInto(ctx, repository, output)
}

// fleetDemo is the demonstration open in a browser: the host page, the frame
// as the test reaches it, and the review server's log.
type fleetDemo struct {
	t          *testing.T
	output     string
	provenance sitelgreplay.Provenance
	asked      func() []string
	host       context.Context
	// frame runs actions in the frame's document: a context attached to the
	// frame's own target when the browser runs the sandboxed frame in a
	// process of its own, else the host's, with inFrame pointing each
	// predicate at the frame.
	frame   context.Context
	inFrame []chromedp.PollOption
}

// openFleetDemo serves the shared render as the owner's review server does,
// opens its host page, and waits for the frame to show the entry page.
func openFleetDemo(t *testing.T) *fleetDemo {
	t.Helper()
	output, provenance := fleetDemoOutput(t)
	files := http.FileServer(http.Dir(output))
	var mu sync.Mutex
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path)
		mu.Unlock()
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	demo := &fleetDemo{t: t, output: output, provenance: provenance, host: newRFCBrowser(t), asked: func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(asked)
	}}
	if _, err := chromedp.Run(demo.host, chromedp.Navigate(server.URL+fleetDemoPage)); err != nil {
		t.Fatal(err)
	}
	if _, err := chromedp.Run(demo.host, chromedp.Poll[bool](
		fleetDemoMount+` !== null && `+fleetDemoMount+`.getAttribute('data-lg-replay-state') === 'ready'`,
		chromedp.WithPollingInterval(fleetDemoPoll), chromedp.WithPollingTimeout(fleetDemoWait))); err != nil {
		t.Fatalf("waited for the frame to show the entry page: %v\n%s", err, demo.seen())
	}
	demo.reachFrame()
	demo.await("the shell's content region", fleetDemoPath+` != null`)
	return demo
}

// reachFrame finds the frame's document: the browser's target for the
// frame, or the frame element of the host page when the frame runs in the
// host's process.
func (demo *fleetDemo) reachFrame() {
	demo.t.Helper()
	targets, err := chromedp.Targets(demo.host)
	if err != nil {
		demo.t.Fatal(err)
	}
	for _, info := range targets {
		if info.Type == "iframe" && strings.Contains(info.URL, fleetDemoFrame) {
			frame, cancel := chromedp.NewContext(demo.host, chromedp.WithTargetID(info.TargetID))
			demo.t.Cleanup(cancel)
			demo.frame = frame
			return
		}
	}
	nodes, err := chromedp.Run(demo.host, chromedp.Nodes(chromedp.CSS("[data-lg-replay] iframe")))
	if err != nil {
		demo.t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ContentDocument == nil {
		demo.t.Fatalf("the frame is neither a target of its own nor a frame of the host page: %d frame elements\n%s", len(nodes), demo.seen())
	}
	demo.frame = demo.host
	demo.inFrame = []chromedp.PollOption{chromedp.WithPollingInFrame(nodes[0])}
}

// polling answers the options of every wait in the frame.
func (demo *fleetDemo) polling() []chromedp.PollOption {
	return append([]chromedp.PollOption{chromedp.WithPollingInterval(fleetDemoPoll), chromedp.WithPollingTimeout(fleetDemoWait)}, demo.inFrame...)
}

// await waits for condition, a boolean expression over the frame's
// document, and on a timeout says what was awaited and what the page shows.
func (demo *fleetDemo) await(what, condition string) {
	demo.t.Helper()
	if _, err := chromedp.Run(demo.frame, chromedp.Poll[bool](condition, demo.polling()...)); err != nil {
		demo.t.Fatalf("waited for %s (%s): %v\n%s", what, condition, err, demo.seen())
	}
}

// read waits until expression answers a value in the frame, neither null nor
// undefined, and decodes it into into.
func (demo *fleetDemo) read(what, expression string, into any) {
	demo.t.Helper()
	answer, err := chromedp.Run(demo.frame, chromedp.Poll[string](
		`(() => { const value = `+expression+`; return value === undefined || value === null ? false : JSON.stringify(value); })()`,
		demo.polling()...))
	if err != nil {
		demo.t.Fatalf("waited to read %s (%s): %v\n%s", what, expression, err, demo.seen())
	}
	if err := json.Unmarshal([]byte(answer), into); err != nil {
		demo.t.Fatalf("read %s: %v", what, err)
	}
}

// click waits until element, an expression naming one element of the frame,
// is there, enabled and, when it sends an htmx request, bound by htmx, and
// clicks it once. htmx binds what a swap put in place only after its settle
// delay when the swap kept an element of the same id (the side panel's body),
// and the recording answers at once, so the wait that saw the swap can pass
// before htmx listens: a link clicked then follows its href, and the frame
// leaves its document.
func (demo *fleetDemo) click(what, element string) {
	demo.t.Helper()
	demo.await(what, `(() => { const element = `+element+`; if (!element || element.disabled) return false;
        if (`+fleetDemoHTMXRequests+`.some(name => element.hasAttribute(name)) && !element.hasAttribute(`+strconv.Quote(fleetDemoHTMXBound)+`)) return false;
        element.click(); return true; })()`)
}

// hostText answers the text of the host page's element selector names.
func (demo *fleetDemo) hostText(selector string) string {
	demo.t.Helper()
	text, err := chromedp.Run(demo.host, chromedp.Evaluate[string](`document.querySelector(`+strconv.Quote(selector)+`)?.textContent ?? ''`))
	if err != nil {
		demo.t.Fatal(err)
	}
	return text
}

// mountAttribute answers what the player recorded on the mount under name.
func (demo *fleetDemo) mountAttribute(name string) string {
	demo.t.Helper()
	value, err := chromedp.Run(demo.host, chromedp.Evaluate[string](fleetDemoMount+`?.getAttribute(`+strconv.Quote(name)+`) ?? ''`))
	if err != nil {
		demo.t.Fatal(err)
	}
	return value
}

// seen answers what the browser shows, for a failure message: what the
// player recorded on the mount (but the chunk list), and the frame's page
// path and the start of its text.
func (demo *fleetDemo) seen() string {
	host, err := chromedp.Run(demo.host, chromedp.Evaluate[string](`(() => {
        const mount = `+fleetDemoMount+`;
        if (!mount) return 'the host page holds no [data-lg-replay] mount';
        return Array.from(mount.attributes).filter(a => a.name !== 'data-lg-replay-chunks').map(a => a.name + '=' + a.value).join('\n');
    })()`))
	if err != nil {
		host = "the host page could not be read: " + err.Error()
	}
	if demo.frame == nil {
		return host
	}
	frame, err := chromedp.Run(demo.frame, chromedp.Poll[string](
		`'path: ' + (`+fleetDemoPath+` ?? 'none') + '\n' + (document.body ? document.body.innerText.slice(0, 800) : 'no body')`,
		append([]chromedp.PollOption{chromedp.WithPollingTimeout(2 * time.Second)}, demo.inFrame...)...))
	if err != nil {
		frame = "the frame could not be read: " + err.Error()
	}
	return host + "\nframe " + frame
}

// openTab clicks the object's tab titled title and waits for its page,
// whose path it answers.
func (demo *fleetDemo) openTab(title string) string {
	demo.t.Helper()
	link := `Array.from(document.querySelectorAll('#fleet-object .fleet-tab-row a')).find(a => a.textContent.trim() === ` + strconv.Quote(title) + `)`
	var href string
	demo.read("the link of the "+title+" tab", `(`+link+`)?.getAttribute('href')`, &href)
	demo.click("the "+title+" tab", link)
	path, _, _ := strings.Cut(href, "?")
	demo.await("the page of the "+title+" tab", fleetDemoPath+` === `+strconv.Quote(path))
	return href
}

// chunkFileOf answers the path the host asks the review server for when it
// loads the chunk holding key, read from the recording's index.
func (demo *fleetDemo) chunkFileOf(key string) string {
	demo.t.Helper()
	recording, err := sitelgreplay.Read(filepath.Join(demo.output, filepath.FromSlash(fleetDemoIndex)))
	if err != nil {
		demo.t.Fatal(err)
	}
	var files []string
	for _, chunk := range recording.Chunks {
		if slices.Contains(chunk.Keys, key) {
			files = append(files, fleetDemoFiles+chunk.File)
		}
	}
	if len(files) != 1 {
		demo.t.Fatalf("the recording's index puts %s in %d chunks %v, want one", key, len(files), files)
	}
	return files[0]
}

// fleetDemoPageKey answers the recording's key of the request a link of the
// shell sends for href: an htmx GET.
func fleetDemoPageKey(t *testing.T, href string) string {
	t.Helper()
	address, err := url.Parse(href)
	if err != nil {
		t.Fatal(err)
	}
	query := address.Query()
	names := make([]string, 0, len(query))
	for name := range query {
		names = append(names, name)
	}
	slices.Sort(names)
	var fields []sitelgreplay.Field
	for _, name := range names {
		for _, value := range query[name] {
			fields = append(fields, sitelgreplay.Field{Name: name, Value: value})
		}
	}
	return sitelgreplay.ExchangeKey(http.MethodGet, address.Path, fields, nil, true)
}

// fetchCount answers how many times asked names path.
func fetchCount(asked []string, path string) int {
	count := 0
	for _, each := range asked {
		if each == path {
			count++
		}
	}
	return count
}

// TestFleetDemoPlaysTheEstateDashboard proves, in a browser, that the
// demonstration the action writes opens on the sample estate's dashboard,
// under a host bar that names it a recorded demonstration on sample data and
// names the commit, with the event stream open and silent, and with no
// control that steps the estate.
//
// Method: render once into a scratch output, serve it, open the host page,
// and read the frame's own document.
//
// Red when: the host bar does not carry the recording's provenance
// (hostBarText); the frame does not show the estate dashboard from the
// recording, or asks a request the recording lacks while it opens; the
// event stream fails, so fleet.js shows "Live updates stopped"; or the
// header draws Step or Hold, because the routes see the sample's stepper
// (plainSource).
func TestFleetDemoPlaysTheEstateDashboard(t *testing.T) {
	demo := openFleetDemo(t)

	bar := demo.hostText(".fleet-demo__bar")
	commit := demo.provenance.Commit
	if len(commit) > fleetDemoCommitShown {
		commit = commit[:fleetDemoCommitShown]
	}
	for _, want := range []string{"recorded demonstration", "Sample data", "commit " + commit} {
		if !strings.Contains(bar, want) {
			t.Errorf("the host bar %q does not say %q", bar, want)
		}
	}

	source := sample.New(time.Now)
	defer source.Close()
	snapshot, err := fleetui.ReadSnapshot(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}
	demo.await("the estate dashboard", fleetDemoPath+` === '/fleet/' && document.querySelector('.fleet-dashboard') !== null`)
	var estate string
	demo.read("the estate's name in the header", `document.querySelector('.fleet-hub-name')?.textContent.trim()`, &estate)
	if estate != snapshot.Estate.Name {
		t.Errorf("the header names the estate %q, and the sample estate is %q", estate, snapshot.Estate.Name)
	}
	// The first row of the estate dashboard, each card drawn and none failed.
	demo.await("four cards on the dashboard", `document.querySelectorAll('.fleet-dashboard-slot section.fleet-card').length >= 4`)
	var cards []string
	demo.read("the first row of cards", `Array.from(document.querySelectorAll('.fleet-dashboard-slot')).slice(0, 4).map(slot =>
        (slot.querySelector('section.fleet-card')?.getAttribute('aria-label') ?? 'no card') +
        (slot.querySelector('.fleet-dashboard-retry') ? ' (failed)' : ''))`, &cards)
	if want := []string{"Devices", "Configuration", "Software", "Estate"}; !slices.Equal(cards, want) {
		t.Errorf("the first row of the estate dashboard is %q, want %q\n%s", cards, want, demo.seen())
	}

	// The stream opens, and nothing it says makes the page stale.
	demo.await("the event stream to open", `document.documentElement.getAttribute('data-fleet-stream') === 'open'`)
	var stale bool
	demo.read("the stale banner", `document.getElementById('fleet-stale')?.hidden === false`, &stale)
	if stale {
		t.Errorf("the page shows %q", "Live updates stopped")
	}
	var steppers int
	demo.read("the stepping controls", `document.querySelectorAll('#fleet-proto-step, #fleet-proto-hold, #fleet-proto-step-form, #fleet-proto-hold-form').length`, &steppers)
	if steppers != 0 {
		t.Errorf("the header draws %d Step or Hold controls, and the recording hides stepping", steppers)
	}

	if missing := demo.mountAttribute("data-lg-replay-missing"); missing != "" {
		t.Errorf("the dashboard asked a request the recording lacks: %s", missing)
	}
	if reported := demo.mountAttribute("data-lg-replay-errors"); reported != "" {
		t.Errorf("the frame reported: %s", reported)
	}
}

// TestFleetDemoOpensADeviceFromItsChunk proves, in a browser, that a device
// opened through the estate tree shows its dashboard and its System and
// Interfaces tabs from the device's own chunk, which the host fetches when
// the device is first opened, and once.
//
// Method: walk the tree down its first branches (a site, then a role) to its
// first device and click it, then its two tabs, and read the frame's
// document and the review server's log, against the chunk the recording's
// index names for the device's page.
//
// Red when: a tree branch or the device's pages are not recorded, so the
// frame shows the notice; the host fetches the device's chunk before the
// device is opened, or not at all (the frame answered from elsewhere); or it
// fetches the chunk again for each tab.
func TestFleetDemoOpensADeviceFromItsChunk(t *testing.T) {
	demo := openFleetDemo(t)

	list := `document.querySelector('#fleet-tree ul[role=tree]')`
	var name, href, link string
	for depth := 0; depth < fleetDemoTreeDepthMax && link == ""; depth++ {
		level := "tree level " + strconv.Itoa(depth+1)
		demo.await("the rows of "+level, `(`+list+`)?.querySelector(':scope > li.fleet-tree-item') != null`)
		var key string
		demo.read("the first branch of "+level, `(`+list+`).querySelector(':scope > li.fleet-tree-item[aria-expanded]')?.getAttribute('data-fleet-key') ?? ''`, &key)
		if key == "" {
			link = `(` + list + `).querySelector(':scope > li.fleet-tree-item:not([aria-expanded]) a.fleet-tree-link')`
			break
		}
		item := `document.querySelector('#fleet-tree li[data-fleet-key="' + CSS.escape(` + strconv.Quote(key) + `) + '"]')`
		demo.await("the branch "+key+" to open", `(() => { const item = `+item+`; if (!item) return false;
            if (item.getAttribute('aria-expanded') !== 'true') item.querySelector(':scope > .fleet-tree-row > .fleet-tree-toggle')?.click();
            return item.getAttribute('aria-expanded') === 'true'; })()`)
		list = `(` + item + `)?.querySelector(':scope > .fleet-tree-children')`
	}
	if link == "" {
		t.Fatalf("no device within %d levels of the estate tree\n%s", fleetDemoTreeDepthMax, demo.seen())
	}
	demo.read("the device's link", `(`+link+`)?.getAttribute('href')`, &href)
	demo.read("the device's name", `(`+link+`)?.querySelector('.fleet-tree-name')?.textContent.trim()`, &name)
	chunk := demo.chunkFileOf(fleetDemoPageKey(t, href))
	if count := fetchCount(demo.asked(), chunk); count != 0 {
		t.Fatalf("the host fetched %s %d times before the device was opened", chunk, count)
	}

	demo.click("the device "+name+" in the tree", link)
	path, _, _ := strings.Cut(href, "?")
	demo.await("the dashboard of "+name, fleetDemoPath+` === `+strconv.Quote(path)+` && document.querySelector('.fleet-dashboard') !== null &&
        (document.getElementById('fleet-object')?.textContent.includes(`+strconv.Quote(name)+`) ?? false)`)
	if count := fetchCount(demo.asked(), chunk); count != 1 {
		t.Errorf("the host fetched %s %d times once the device was opened, want once", chunk, count)
	}

	for _, tab := range []string{"System", "Interfaces"} {
		demo.openTab(tab)
		demo.await("the "+tab+" tab to name "+name, `document.getElementById('fleet-object')?.textContent.includes(`+strconv.Quote(name)+`) ?? false`)
		var notice bool
		demo.read("the notice", `document.body.innerText.includes(`+strconv.Quote(fleetDemoNotice)+`)`, &notice)
		if notice {
			t.Errorf("the %s tab of %s shows the notice\n%s", tab, name, demo.seen())
		}
	}
	if count := fetchCount(demo.asked(), chunk); count != 1 {
		t.Errorf("the host fetched %s %d times over the device's tabs, want once", chunk, count)
	}
}

// TestFleetDemoSortsAndOpensAPanel proves, in a browser, that the Devices tab
// of the estate plays from its own chunk (estate-devices): a row's eye opens
// the side panel of that device and its close control closes it, and a
// sortable header reorders the rows.
//
// Method: open the tab, read the review server's log against the chunk the
// recording's index names for the tab's page, then work the table and read
// the frame's document.
//
// Red when: the tab's views are not in a chunk the host loads when the tab
// opens; the side panel of a row or its close is not recorded, or opens
// another row's details; or the sorted view is not recorded, or answers the
// rows in the order they were.
func TestFleetDemoSortsAndOpensAPanel(t *testing.T) {
	demo := openFleetDemo(t)

	devices := demo.openTab("Devices")
	chunk := demo.chunkFileOf(fleetDemoPageKey(t, devices))
	if count := fetchCount(demo.asked(), chunk); count != 1 {
		t.Errorf("the host fetched %s %d times once the Devices tab opened, want once", chunk, count)
	}

	eye := `document.querySelector('#fleet-content tr.fleet-table-row a[data-fleet-eye]')`
	var label string
	demo.read("the first row's eye", `(`+eye+`)?.getAttribute('aria-label')`, &label)
	name, found := strings.CutPrefix(label, "Show the details of ")
	if !found {
		t.Fatalf("the eye of the first row is labeled %q", label)
	}
	demo.click("the eye of "+name, eye)
	demo.await("the side panel of "+name, `document.getElementById('fleet-side-panel')?.getAttribute('data-fleet-panel') === 'open' &&
        (document.querySelector('#fleet-side-panel .fleet-panel-title')?.textContent.includes(`+strconv.Quote(name)+`) ?? false)`)
	demo.click("the close control of the side panel", `document.querySelector('#fleet-side-panel a[aria-label="Close the details"]')`)
	demo.await("the side panel to close", `document.getElementById('fleet-side-panel')?.getAttribute('data-fleet-panel') === 'empty'`)

	order := `JSON.stringify(Array.from(document.querySelectorAll('#fleet-content tr.fleet-table-row [data-fleet-select-row]')).map(box => box.value))`
	var before string
	demo.read("the order of the rows", order, &before)
	// The column the table is sorted by reverses when its header is
	// clicked, so the order cannot stay as it was.
	header := `(document.querySelector('#fleet-content th[aria-sort="ascending"] a.fleet-table-sort, #fleet-content th[aria-sort="descending"] a.fleet-table-sort') ??
        document.querySelector('#fleet-content a.fleet-table-sort'))`
	var column string
	demo.read("a sortable header", `(`+header+`)?.textContent.trim()`, &column)
	demo.click("the header "+column, header)
	demo.await("the rows sorted by "+column, `(`+order+`) !== `+strconv.Quote(before)+` && document.querySelectorAll('#fleet-content tr.fleet-table-row').length > 0`)
	if missing := demo.mountAttribute("data-lg-replay-missing"); missing != "" {
		t.Errorf("working the table asked a request the recording lacks: %s", missing)
	}
}

// TestFleetDemoAnswersAnUnrecordedRequestWithTheNotice proves, in a browser,
// that what the tour did not record is answered rather than left hanging: a
// typed filter shows the notice and leaves the page usable, and a posted
// action shows the toast that it was not sent.
//
// Method: type a filter no tour records into the devices table and submit
// it, then open another tab; then select a row, post an action from the bulk
// bar through its confirm dialog, and read the frame's document and what the
// player recorded on the mount.
//
// Red when: the player answers an unrecorded request with nothing, or with a
// notice the page cannot leave; or a posted action is not answered by the
// recording's fallback toast.
func TestFleetDemoAnswersAnUnrecordedRequestWithTheNotice(t *testing.T) {
	demo := openFleetDemo(t)

	demo.openTab("Devices")
	demo.await("the filter of the devices table", `(() => { const field = document.querySelector('#fleet-content form.fleet-query input[name="q"]');
        if (!field) return false; field.value = `+strconv.Quote(fleetDemoFilter)+`; return true; })()`)
	demo.click("the filter's Search button", `document.querySelector('#fleet-content form.fleet-query button[type="submit"]')`)
	demo.await("the notice", `document.body.innerText.includes(`+strconv.Quote(fleetDemoNotice)+`)`)
	if missing := demo.mountAttribute("data-lg-replay-missing"); !strings.Contains(missing, fleetDemoFilter) {
		t.Errorf("the player names the unrecorded request %q, and the filter was %q", missing, fleetDemoFilter)
	}
	// The page is still usable: a tab opens its recorded page.
	demo.openTab("Dashboard")
	demo.await("the dashboard in place of the notice", `document.querySelector('.fleet-dashboard') !== null && !document.body.innerText.includes(`+strconv.Quote(fleetDemoNotice)+`)`)

	demo.openTab("Devices")
	demo.click("the first row's checkbox", `document.querySelector('#fleet-content tr.fleet-table-row [data-fleet-select-row]')`)
	demo.click("the Add label action of the bulk bar", `document.querySelector('#fleet-content button[data-fleet-bulk-action][value="label-add"]')`)
	demo.await("the confirm dialog", `document.getElementById('fleet-confirm')?.open === true`)
	demo.click("Confirm", `document.querySelector('#fleet-confirm [data-confirm-slot="confirm"]')`)
	demo.await("the toast", `document.getElementById('fleet-toasts')?.textContent.includes(`+strconv.Quote(fleetDemoToast)+`) ?? false`)
}
