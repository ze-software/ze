//go:build integration

// Design: docs/architecture/testing/runner-architecture.md -- finite capture work.
package webtesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestBrowserScreenshotNativeHTMXLifecycle reaches the bundled HTMX producer,
// native capture predicate and real Screenshot across each finite completion
// rail. No test manufactures a request-completion event or changes a counter.
// MUTATION: omit finite HTMX ownership, or fail to discharge one of its actual
// error, abort, cancellation or disconnected-source completion paths.
func TestBrowserScreenshotNativeHTMXLifecycle(t *testing.T) {
	for _, mode := range []string{
		"awaited-swap", "request-failure", "body-failure", "abort-request",
		"abort-body", "cancel-request", "cancel-response", "removed-source",
	} {
		t.Run(mode, func(t *testing.T) {
			fixture := newHTMXCaptureLifecycle(t, mode)
			fixture.click()
			swapped, headers, failures := false, 0, 0
			switch mode {
			case "awaited-swap":
				fixture.wait(`p.swapWait===1&&p.swapDelegated===0&&p.finished===0&&document.getElementById('proof').dataset.state==='initial'`, "the real swap awaiting its exact public-timeout barrier")
				fixture.captureWhilePending()
				fixture.eval(`window.__zeHTMXLifecycle.releaseSwap();true`)
				fixture.joinCapture()
				fixture.wait(`p.swapDelegated===1&&p.swapped===1&&p.finished===1`, "the delegated original timeout, actual swap and request completion")
				swapped, headers = true, 1
			case "request-failure":
				fixture.wait(`p.errors===1&&p.finished===1`, "the real fetch failure and its final request hook")
				failures = 1
			case "body-failure":
				fixture.waitHeaders()
				fixture.release()
				fixture.wait(`p.errors===1&&p.finished===1`, "the actual truncated-body rejection and final request hook")
				headers, failures = 1, 1
			case "abort-request":
				fixture.waitRequested()
				fixture.abort()
				fixture.waitCanceled()
				fixture.wait(`p.errors===1&&p.finished===1`, "the real pre-header abort and final request hook")
				failures = 1
			case "abort-body":
				fixture.waitHeaders()
				fixture.abort()
				fixture.waitCanceled()
				fixture.wait(`p.errors===1&&p.finished===1`, "the real held-body abort and final request hook")
				headers, failures = 1, 1
			case "cancel-request":
				fixture.wait(`p.finished===1`, "the actual before-request cancellation and finally hook")
			case "cancel-response":
				fixture.wait(`p.headers===1&&p.finished===1`, "the actual before-response cancellation and finally hook")
				headers = 1
			case "removed-source":
				fixture.waitHeaders()
				fixture.eval(`document.getElementById('load-proof').remove();true`)
				fixture.wait(`!document.getElementById('load-proof')&&p.finished===0`, "source removal while its actual body remains held")
				fixture.captureWhilePending()
				fixture.release()
				fixture.joinCapture()
				fixture.wait(`p.finished===1&&p.finalTarget==='document'&&p.swapped===1`, "real disconnected-source completion rehomed by bundled HTMX")
				swapped, headers = true, 1
			default:
				t.Fatalf("unhandled lifecycle fixture %q", mode)
			}
			fixture.assertFinished(headers, failures, swapped)
			fixture.waitIdle()
			if fixture.capture == "" {
				fixture.capture = filepath.Join(t.TempDir(), "htmx-lifecycle.png")
				if err := fixture.browser.Screenshot(fixture.capture); err != nil {
					t.Fatal(err)
				}
			}
			fixture.assertPixels(swapped)
			if mode == "cancel-request" && fixture.requests.Load() != 0 {
				t.Errorf("before-request cancellation still sent %d HTTP requests", fixture.requests.Load())
			}
			if mode != "cancel-request" && fixture.requests.Load() == 0 {
				t.Error("the lifecycle control never reached the real HTTP request")
			}
		})
	}
}

type htmxCaptureLifecycle struct {
	t         *testing.T
	ctx       context.Context
	browser   *Browser
	observer  *renderObserver
	mode      string
	body      chan struct{}
	bodyOnce  sync.Once
	requested chan struct{}
	headers   chan struct{}
	canceled  chan struct{}
	requests  atomic.Int32
	capture   string
	result    chan error
	joined    bool
}

func newHTMXCaptureLifecycle(t *testing.T, mode string) *htmxCaptureLifecycle {
	t.Helper()
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("native HTMX lifecycle proof requires agent-browser: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the actual bundled HTMX asset")
	}
	asset, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "assets", "htmx.min.js"))
	if err != nil || len(asset) == 0 {
		t.Fatalf("read actual bundled HTMX: %v, bytes=%d", err, len(asset))
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	fixture := &htmxCaptureLifecycle{t: t, ctx: ctx, mode: mode,
		body: make(chan struct{}), requested: make(chan struct{}, 1),
		headers: make(chan struct{}, 1), canceled: make(chan struct{}, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("/htmx.min.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		if _, err := w.Write(asset); err != nil {
			t.Errorf("serve real HTMX: %v", err)
		}
	})
	mux.HandleFunc("/work", fixture.serveWork)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		swap := "outerHTML"
		if mode == "awaited-swap" {
			swap += " swap:1000ms"
		}
		page := `<!doctype html><html><head><style>
html,body{margin:0;width:100%;height:100%;background:red}
#proof{position:fixed;inset:0;background:red}#load-proof{position:relative;z-index:1}
</style><script src="/htmx.min.js"></script><script>
(function(){
  var p=window.__zeHTMXLifecycle={mode:` + strconv.Quote(mode) + `,started:0,headers:0,
    swapped:0,finished:0,errors:0,swapWait:0,swapDelegated:0,finalTarget:'',ctx:null};
  var release;
  var gate=new Promise(function(resolve){release=resolve;});
  p.releaseSwap=function(){release();};
  if(p.mode==='awaited-swap'){
    var originalTimeout=htmx.timeout.bind(htmx);
    htmx.timeout=function(delay){
      if(delay==='1000ms'){
        p.swapWait++;
        return gate.then(function(){p.swapDelegated++;return originalTimeout(delay);});
      }
      return originalTimeout(delay);
    };
  }
  function ours(event){return event.detail&&event.detail.ctx===p.ctx;}
  document.addEventListener('htmx:before:request',function(event){
    var ctx=event.detail&&event.detail.ctx;
    if(ctx&&ctx.sourceElement.id==='load-proof'){
      p.ctx=ctx;p.started++;
      if(p.mode==='cancel-request'){event.preventDefault();}
    }
  });
  document.addEventListener('htmx:before:response',function(event){
    if(ours(event)){
      p.headers++;
      if(p.mode==='cancel-response'){event.preventDefault();}
    }
  });
  document.addEventListener('htmx:after:swap',function(event){if(ours(event)){p.swapped++;}});
  document.addEventListener('htmx:error',function(event){if(ours(event)){p.errors++;}});
  document.addEventListener('htmx:finally:request',function(event){
    if(ours(event)){p.finished++;p.finalTarget=event.target===document?'document':event.target.id;}
  });
})();
</script></head><body><div id="proof" data-state="initial">Initial body</div>
<button id="load-proof" hx-get="/work" hx-target="#proof" hx-swap="` + swap + `">Load proof</button>
</body></html>`
		if _, err := w.Write([]byte(page)); err != nil {
			t.Errorf("serve lifecycle fixture: %v", err)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	fixture.browser = newBrowser(server.URL)
	fixture.browser.ctx = ctx
	t.Cleanup(func() {
		fixture.release()
		cancel()
		if fixture.result != nil && !fixture.joined {
			<-fixture.result
		}
		if err := fixture.browser.Close(); err != nil {
			t.Errorf("close owned lifecycle browser: %v", err)
		}
	})
	if err := fixture.browser.setViewport(320, 240); err != nil {
		t.Fatal(err)
	}
	if err := fixture.browser.Open("/"); err != nil {
		t.Fatal(err)
	}
	fixture.observer, err = newRenderObserverAddress(fixture.browser.session)
	if err != nil {
		t.Fatal(err)
	}
	fixture.wait(`typeof htmx==='object'&&document.getElementById('load-proof').getAttribute('data-htmx-powered')==='true'`, "real bundled HTMX initialization")
	return fixture
}

func (f *htmxCaptureLifecycle) serveWork(w http.ResponseWriter, r *http.Request) {
	f.requests.Add(1)
	if r.Header.Get("HX-Request") != "true" {
		http.Error(w, "expected the real HTMX request", http.StatusBadRequest)
		return
	}
	select {
	case f.requested <- struct{}{}:
	default:
	}
	if f.mode == "request-failure" {
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			f.t.Errorf("hijack the owned failing request: %v", err)
			return
		}
		if err := connection.Close(); err != nil {
			f.t.Errorf("close the owned failed request: %v", err)
		}
		return
	}
	if f.mode == "abort-request" {
		select {
		case <-f.body:
		case <-r.Context().Done():
			f.canceled <- struct{}{}
		}
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if f.mode == "body-failure" {
		// Returning less than this declared length makes the actual browser
		// response.text reject; no test-produced completion event is involved.
		w.Header().Set("Content-Length", "4096")
	}
	if f.mode == "body-failure" || f.mode == "abort-body" || f.mode == "removed-source" {
		w.WriteHeader(http.StatusOK)
		if err := http.NewResponseController(w).Flush(); err != nil {
			f.t.Errorf("flush lifecycle headers: %v", err)
			return
		}
		f.headers <- struct{}{}
		select {
		case <-f.body:
		case <-r.Context().Done():
			f.canceled <- struct{}{}
			return
		}
	}
	body := `<div id="proof" data-state="swapped" style="background:rgb(0,255,0)">Swapped body</div>`
	if f.mode == "body-failure" {
		body = "<div"
	}
	if _, err := w.Write([]byte(body)); err != nil {
		f.t.Errorf("serve lifecycle response: %v", err)
	}
}

func (f *htmxCaptureLifecycle) eval(script string) json.RawMessage {
	f.t.Helper()
	data, err := f.observer.command(f.ctx, "evaluate", script)
	if err != nil {
		f.t.Fatal(err)
	}
	var response struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &response); err != nil || len(response.Result) == 0 {
		f.t.Fatalf("decode native lifecycle observation: %v, data=%s", err, data)
	}
	return response.Result
}

func (f *htmxCaptureLifecycle) wait(expression, description string) {
	f.t.Helper()
	ticker := time.NewTicker(expectPoll)
	defer ticker.Stop()
	for {
		if string(f.eval(`(function(){var p=window.__zeHTMXLifecycle;return (`+expression+`);})()`)) == "true" {
			return
		}
		select {
		case <-f.ctx.Done():
			f.t.Fatalf("wait for %s: %v", description, context.Cause(f.ctx))
		case <-ticker.C:
		}
	}
}

func (f *htmxCaptureLifecycle) click() {
	f.t.Helper()
	if string(f.eval(`document.getElementById('load-proof').click();true`)) != "true" {
		f.t.Fatal("the native request click did not execute")
	}
}

func (f *htmxCaptureLifecycle) abort() {
	f.t.Helper()
	// htmx:abort is the bundled producer's actual public abort control. It
	// cancels the owned request; it is not a fabricated finally/error event.
	if string(f.eval(`htmx.trigger(document.getElementById('load-proof'),'htmx:abort');true`)) != "true" {
		f.t.Fatal("the real HTMX abort control did not execute")
	}
}

func (f *htmxCaptureLifecycle) waitRequested() {
	f.t.Helper()
	select {
	case <-f.requested:
	case <-f.ctx.Done():
		f.t.Fatal("the actual request never reached its server-side gate")
	}
}

func (f *htmxCaptureLifecycle) waitHeaders() {
	f.t.Helper()
	select {
	case <-f.headers:
	case <-f.ctx.Done():
		f.t.Fatal("the actual response never flushed its headers")
	}
	f.wait(`p.headers===1&&p.finished===0&&document.getElementById('proof').dataset.state==='initial'`, "the browser-side pre-body response hook")
}

func (f *htmxCaptureLifecycle) waitCanceled() {
	f.t.Helper()
	select {
	case <-f.canceled:
	case <-f.ctx.Done():
		f.t.Fatal("the real browser abort never canceled the owned HTTP request")
	}
}

func (f *htmxCaptureLifecycle) release() {
	f.bodyOnce.Do(func() { close(f.body) })
}

func (f *htmxCaptureLifecycle) captureWhilePending() {
	f.t.Helper()
	const arm = "window.__zeInflight===0"
	if strings.Count(inflightIdleExpr, arm) != 1 {
		f.t.Fatal("the production quiet-window probe no longer identifies its request-count arm")
	}
	f.wait(strings.Replace(inflightIdleExpr, arm, "true", 1), "the existing quiet window while finite HTMX work remains held")
	if string(f.eval(captureReadyExpr)) == "true" {
		f.t.Error("capture readiness accepted the actual pending HTMX body/swap")
	}
	f.eval(`window.__zeCaptureFrames=null;true`)
	f.capture = filepath.Join(f.t.TempDir(), "pending-htmx.png")
	f.result = make(chan error, 1)
	go func() { f.result <- f.browser.Screenshot(f.capture) }()
	f.wait(`!!window.__zeCaptureFrames&&window.__zeCaptureFrames.completed===true`, "Screenshot's actual completed render-frame receipt")
	select {
	case err := <-f.result:
		f.joined = true
		f.t.Errorf("Screenshot completed while real HTMX work was held: %v", err)
	case <-time.After(300 * time.Millisecond):
	case <-f.ctx.Done():
		f.t.Fatal("capture did not remain live during its held-work observation")
	}
	if _, err := os.Stat(f.capture); !os.IsNotExist(err) {
		f.t.Errorf("held HTMX work produced a capture artifact: %v", err)
	}
}

func (f *htmxCaptureLifecycle) joinCapture() {
	f.t.Helper()
	if !f.joined {
		if err := <-f.result; err != nil {
			f.t.Error(err)
		}
		f.joined = true
	}
}

func (f *htmxCaptureLifecycle) assertFinished(headers, failures int, swapped bool) {
	f.t.Helper()
	wantSwaps := 0
	if swapped {
		wantSwaps = 1
	}
	expression := fmt.Sprintf(`p.started===1&&p.headers===%d&&p.finished===1&&p.errors===%d&&p.swapped===%d`, headers, failures, wantSwaps)
	if string(f.eval(`(function(){var p=window.__zeHTMXLifecycle;return (`+expression+`);})()`)) != "true" {
		f.t.Errorf("actual lifecycle counts differ from %s: %s", expression, f.eval(`JSON.stringify(window.__zeHTMXLifecycle,function(key,value){return key==='ctx'?undefined:value;})`))
	}
	if f.mode == "awaited-swap" {
		if string(f.eval(`window.__zeHTMXLifecycle.swapWait===1&&window.__zeHTMXLifecycle.swapDelegated===1`)) != "true" {
			f.t.Error("the exact awaited-swap barrier did not enter and delegate the original timeout once")
		}
	}
}

func (f *htmxCaptureLifecycle) waitIdle() {
	f.t.Helper()
	f.wait(inflightIdleExpr, "balanced real request ownership and its existing quiet window")
	if verdict := f.eval(captureReadyExpr); string(verdict) != "true" {
		f.t.Errorf("completed finite HTMX work did not restore actual capture readiness: %s", verdict)
	}
}

func (f *htmxCaptureLifecycle) assertPixels(swapped bool) {
	f.t.Helper()
	file, err := os.Open(f.capture)
	if err != nil {
		f.t.Fatal(err)
	}
	image, decodeErr := png.Decode(file)
	if err := errors.Join(decodeErr, file.Close()); err != nil {
		f.t.Fatal(err)
	}
	if image.Bounds().Dx() != 320 || image.Bounds().Dy() != 240 {
		f.t.Fatalf("actual capture dimensions = %v, want 320x240", image.Bounds())
	}
	wantRed, wantGreen := uint32(65535), uint32(0)
	if swapped {
		wantRed, wantGreen = 0, 65535
	}
	red, green, blue, alpha := image.At(100, 100).RGBA()
	if red != wantRed || green != wantGreen || blue != 0 || alpha != 65535 {
		f.t.Errorf("capture did not preserve the exact resulting painted state: R=%d G=%d B=%d A=%d", red, green, blue, alpha)
	}
}
