// Design: docs/contributing/gh-pages.md -- the replay of a second product: chunks through the host, fallbacks, a silent stream.
package browsertest

import (
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/ze-software/ze/internal/le/site"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

// replayHostSite serves a host page that mounts recording with the real
// player and the real frame writer, with no tour: the frame at
// replay/frame.html, each chunk file of files beside it, and a mount carrying
// mountAttributes. The host page keeps every message whose data has a probe
// field in window.replayProbes, by that field, so a script of the recording
// can report what it saw. It answers the address of the host page and the log
// of every path the server was asked for.
func replayHostSite(t *testing.T, recording sitelgreplay.Recording, files map[string][]byte, mountAttributes string) (string, func() []string) {
	t.Helper()
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	player, err := os.ReadFile(filepath.Join(repository, filepath.FromSlash(site.ReplayPlayerSource)))
	if err != nil {
		t.Fatal(err)
	}
	frame, err := site.ReplayFrameDocument(recording, string(player))
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	write := func(name string, content []byte) {
		t.Helper()
		path := filepath.Join(output, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("demo/replay/frame.html", frame)
	write("demo/player.js", player)
	for name, content := range files {
		write("demo/replay/"+name, content)
	}
	chunks, err := sitelgreplay.ChunkFiles(recording.Chunks)
	if err != nil {
		t.Fatal(err)
	}
	host := `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Replay host</title><script>` +
		`window.replayHostErrors = []; window.replayProbes = {};` +
		`window.addEventListener('error', function (e) { window.replayHostErrors.push(e.message); });` +
		`window.addEventListener('message', function (e) { if (e.data && e.data.probe) window.replayProbes[e.data.probe] = e.data.value; });` +
		`</script></head><body><div data-lg-replay data-lg-replay-frame="replay/frame.html" data-lg-replay-chunks="` +
		html.EscapeString(chunks) + `" ` + mountAttributes + `></div><script src="player.js" defer></script></body></html>`
	write("demo/index.html", []byte(host))

	var mu sync.Mutex
	var asked []string
	served := http.FileServer(http.Dir(output))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path)
		mu.Unlock()
		served.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL + "/demo/", func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string{}, asked...)
	}
}

// probeRecording is a recording of a product that is not the looking glass,
// with every optional field: its entry page loads a classic script and, before
// it, a deferred one that asks the player what the fleet pages ask of it and
// reports each answer to the host page. Chunk "device" is written; chunk
// "broken" is named and never written, so its file cannot load.
func probeRecording(t *testing.T) (sitelgreplay.Recording, map[string][]byte) {
	t.Helper()
	const markup = "text/html; charset=utf-8"
	exchange := func(method, path string, htmx bool, body string) sitelgreplay.Exchange {
		return sitelgreplay.Exchange{
			Key: sitelgreplay.ExchangeKey(method, path, nil, nil, htmx), Method: method, Path: path, HTMX: htmx,
			Via: "link", Status: 200, ContentType: markup, Body: body,
		}
	}
	probe := `window.order.push('deferred');
function report(name, value) { window.parent.postMessage({ probe: name, value: String(value) }, '*'); }
report('order', window.order.join(','));
var htmx = { headers: { 'HX-Request': 'true' } };
function ask(name, address, init) {
  return fetch(address, init || htmx).then(function (answer) { return answer.text(); }).then(function (text) { report(name, text); });
}
var stream = new EventSource('/probe/events');
var opened = new Promise(function (resolve) {
  stream.addEventListener('open', function () { report('stream', stream.readyState + ' ' + (stream.onmessage === null)); resolve(); });
});
history.pushState({}, '', '/probe/sub/');
Promise.all([
  ask('chunk', '/probe/device'), ask('chunk-again', '/probe/device'),
  ask('fallback', '/probe/action/reboot', { method: 'POST', headers: htmx.headers }),
  ask('notice', '/probe/elsewhere'), ask('broken', '/probe/broken'), ask('relative', 'relative'), opened
]).then(function () {
  report('done', 'yes');
  document.getElementById('list').requestSubmit();
});`
	entry := exchange("GET", "/probe/", false, `<!DOCTYPE html><html lang="en"><head><title>Probe</title>`+
		`<script src="/probe/assets/probe.js" defer></script><script src="/probe/assets/first.js"></script></head>`+
		`<body><form id="list" method="get" action="/probe/list"><input name="q" value="a"><input name="empty" value=""></form></body></html>`)
	list := exchange("GET", "/probe/list", false, `<!DOCTYPE html><html><head><title>List</title></head><body><p>the list</p></body></html>`)
	list.Query = []sitelgreplay.Field{{Name: "q", Value: "a"}}
	list.Key = sitelgreplay.ExchangeKey("GET", "/probe/list", list.Query, nil, false)
	held := []sitelgreplay.Exchange{
		exchange("GET", "/probe/device", true, `<p>from the chunk</p>`),
		exchange("GET", "/probe/sub/relative", true, `<p>relative to the pushed address</p>`),
	}
	device, err := sitelgreplay.EncodeChunk(held)
	if err != nil {
		t.Fatal(err)
	}
	return sitelgreplay.Recording{
		Schema:     sitelgreplay.Schema,
		Provenance: sitelgreplay.Provenance{Recorder: "browser test", Normalized: []sitelgreplay.Normalization{}, Varies: []string{}},
		Entry:      entry.Key,
		Exchanges:  []sitelgreplay.Exchange{entry, list},
		Assets: []sitelgreplay.Asset{
			{Path: "/probe/assets/first.js", ContentType: "text/javascript", Body: "window.order = ['classic'];"},
			{Path: "/probe/assets/probe.js", ContentType: "text/javascript", Body: probe},
		},
		Player: &sitelgreplay.Player{
			Title: "Probe replay", Subject: "probe", HostPage: "../", AssetPrefixes: []string{"/probe/assets/"},
			NoticeText: "Not recorded here: " + sitelgreplay.NoticeRequestToken + ".", NoticeClass: "probe-notice",
			SilentEventSource: true, KeepPushedAddress: true, PlainForms: true,
		},
		Chunks: []sitelgreplay.Chunk{
			{Name: "device", File: "device.json", Size: len(device), Keys: []string{held[0].Key, held[1].Key}},
			{Name: "broken", File: "broken.json", Size: 1, Keys: []string{"GET /probe/broken htmx"}},
		},
		Fallbacks: []sitelgreplay.Fallback{{
			Method: "POST", PathPrefix: "/probe/action/",
			Exchange: exchange("POST", "/probe/action/", true, `<div>Recorded demonstration: this action was not sent</div>`),
		}},
	}, map[string][]byte{"device.json": device}
}

// TestReplayPlaysAProductFromChunksFallbacksAndASilentStream proves, in a
// browser, what the player does for a recording that asks for it: deferred
// scripts run after classic ones; a key in a chunk is answered once the host
// hands the chunk over, two requests for it share one fetch of its file; a
// posted action is answered by its fallback; an unrecorded request gets the
// recording's notice and class; a chunk that cannot load answers with the
// notice naming it; the stream opens and stays quiet; a relative address
// resolves against the pushed one; a plain GET form opens its recorded page;
// and a mount with no tour fills its box.
//
// Method: the recording's own script reports each answer to the host page by
// message, and the test reads those reports, the mount and the server's log.
func TestReplayPlaysAProductFromChunksFallbacksAndASilentStream(t *testing.T) {
	recording, files := probeRecording(t)
	base, asked := replayHostSite(t, recording, files, `data-lg-replay-fill style="height: 300px"`)
	ctx := newRFCBrowser(t)
	if _, err := chromedp.Run(ctx, chromedp.Navigate(base)); err != nil {
		t.Fatal(err)
	}
	const mount = `document.querySelector('[data-lg-replay]')`
	seen := func() string {
		value, err := chromedp.Run(ctx, chromedp.Evaluate[string](`JSON.stringify({ host: window.replayHostErrors, probes: window.replayProbes,
            mount: Array.from(`+mount+`.attributes).map(a => a.name + '=' + a.value) })`))
		if err != nil {
			return "the page could not be read: " + err.Error()
		}
		return value
	}
	if _, err := chromedp.Run(ctx, chromedp.Poll[bool](
		mount+`.getAttribute('data-lg-replay-page') === 'GET /probe/list?q=a'`,
		chromedp.WithPollingTimeout(15*time.Second))); err != nil {
		t.Fatalf("waited for the probe to end on the form's page: %v\n%s", err, seen())
	}
	probes, err := chromedp.Run(ctx, chromedp.Evaluate[map[string]string](`window.replayProbes`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"order":       "classic,deferred",
		"chunk":       "<p>from the chunk</p>",
		"chunk-again": "<p>from the chunk</p>",
		"fallback":    "<div>Recorded demonstration: this action was not sent</div>",
		"notice":      `<div class="probe-notice" role="status">Not recorded here: GET /probe/elsewhere.</div>`,
		"relative":    "<p>relative to the pushed address</p>",
		"stream":      "1 true",
		"done":        "yes",
	}
	for name, value := range want {
		if probes[name] != value {
			t.Errorf("probe %s: got %q, want %q\n%s", name, probes[name], value, seen())
		}
	}
	if broken := probes["broken"]; !strings.Contains(broken, "Not recorded here: GET /probe/broken.") ||
		!strings.Contains(broken, "The part of the recording named broken could not be loaded") {
		t.Errorf("probe broken: got %q, want the notice naming the chunk", broken)
	}

	errors, err := chromedp.Run(ctx, chromedp.Evaluate[string](mount+`.getAttribute('data-lg-replay-errors') || ''`))
	if err != nil {
		t.Fatal(err)
	}
	if errors != "" {
		t.Errorf("the frame reported: %s", errors)
	}
	height, err := chromedp.Run(ctx, chromedp.Evaluate[float64](mount+`.querySelector('iframe').getBoundingClientRect().height`))
	if err != nil {
		t.Fatal(err)
	}
	if height != 300 {
		t.Errorf("the frame of a filling mount is %v pixels high, and its mount 300", height)
	}

	fetched := map[string]int{}
	for _, path := range asked() {
		if strings.HasPrefix(path, "/probe/") {
			t.Errorf("a request of the recorded product left the frame: %s", path)
		}
		fetched[path]++
	}
	if count := fetched["/demo/replay/device.json"]; count != 1 {
		t.Errorf("the host fetched the device chunk %d times, want once for the two requests that asked it", count)
	}
	if count := fetched["/demo/replay/broken.json"]; count != 1 {
		t.Errorf("the host fetched the broken chunk %d times, want once", count)
	}
}
