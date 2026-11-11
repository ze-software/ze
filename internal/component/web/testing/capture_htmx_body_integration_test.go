//go:build integration

// Design: docs/architecture/testing/runner-architecture.md -- settled browser captures.
package webtesting

import (
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestBrowserScreenshotNativeWaitsForHTMXBody holds a real bundled-HTMX response
// after its headers reach Chromium. Capture must retain the finite request until
// the body is read, swapped and painted. The response hook and capture-frame
// receipt establish the interleaving; the bounded non-completion observation is
// not a sleep used to guess when the request or screenshot started.
// MUTATION: end request observation when fetch resolves its response headers,
// without retaining the finite HTMX response-body and swap lifetime.
func TestBrowserScreenshotNativeWaitsForHTMXBody(t *testing.T) {
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("native HTMX body capture proof requires agent-browser: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the real bundled HTMX asset")
	}
	htmx, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "assets", "htmx.min.js"))
	if err != nil {
		t.Fatal(err)
	}
	if len(htmx) == 0 {
		t.Fatal("the real bundled HTMX asset is empty")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	releaseBody := make(chan struct{})
	headersFlushed := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/htmx.min.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		if _, err := w.Write(htmx); err != nil {
			t.Errorf("serve bundled HTMX: %v", err)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		page := `<!doctype html><html><head><style>
html,body{margin:0;width:100%;height:100%;background:red}
#proof{position:fixed;inset:0;background:red}
#load-proof{position:relative;z-index:1}
</style><script src="/htmx.min.js"></script><script>
window.__zeHTMXBodyProof={headers:0,finished:0};
document.addEventListener('htmx:before:response',function(event){
  var ctx=event.detail&&event.detail.ctx;
  if(ctx&&ctx.sourceElement.id==='load-proof'&&ctx.response.status===200){
    window.__zeHTMXBodyProof.headers++;
  }
});
document.addEventListener('htmx:finally:request',function(event){
  var ctx=event.detail&&event.detail.ctx;
  if(ctx&&ctx.sourceElement.id==='load-proof'){
    window.__zeHTMXBodyProof.finished++;
  }
});
</script></head><body><div id="proof" data-state="initial">Held body</div>
<button id="load-proof" hx-get="/body" hx-target="#proof" hx-swap="outerHTML">Load proof</button>
</body></html>`
		if _, err := w.Write([]byte(page)); err != nil {
			t.Errorf("serve HTMX capture page: %v", err)
		}
	})
	mux.HandleFunc("/body", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("HX-Request") != "true" {
			http.Error(w, "expected the bundled HTMX request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("flush held HTMX response headers: %v", err)
			return
		}
		select {
		case headersFlushed <- struct{}{}:
		case <-r.Context().Done():
			return
		}
		select {
		case <-releaseBody:
			if _, err := w.Write([]byte(`<div id="proof" data-state="swapped" style="background:rgb(0,255,0)">Swapped body</div>`)); err != nil {
				t.Errorf("release HTMX response body: %v", err)
			}
		case <-r.Context().Done():
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	browser := newBrowser(server.URL)
	browser.ctx = ctx
	result := make(chan error, 1)
	started, joined, released := false, false, false
	defer func() {
		if !released {
			close(releaseBody)
		}
		cancel()
		if started && !joined {
			<-result
		}
		if err := browser.Close(); err != nil {
			t.Errorf("close native HTMX capture browser: %v", err)
		}
	}()
	if err := browser.setViewport(320, 240); err != nil {
		t.Fatal(err)
	}
	if err := browser.Open("/"); err != nil {
		t.Fatal(err)
	}
	// This controller owns separate SDK connections, not Browser's mutable
	// per-operation context. Browser remains single-owner during Screenshot.
	observer, err := newRenderObserverAddress(browser.session)
	if err != nil {
		t.Fatal(err)
	}
	evaluate := func(script string) json.RawMessage {
		t.Helper()
		data, err := observer.command(ctx, "evaluate", script)
		if err != nil {
			t.Fatalf("observe native HTMX capture state: %v", err)
		}
		var response struct {
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Result) == 0 {
			t.Fatal("native evaluation returned no result")
		}
		return response.Result
	}
	waitFor := func(script, description string) {
		t.Helper()
		ticker := time.NewTicker(expectPoll)
		defer ticker.Stop()
		for {
			if string(evaluate(script)) == "true" {
				return
			}
			select {
			case <-ctx.Done():
				t.Fatalf("wait for %s: %v", description, context.Cause(ctx))
			case <-ticker.C:
			}
		}
	}
	waitFor(`typeof htmx==='object'&&document.getElementById('load-proof').getAttribute('data-htmx-powered')==='true'`, "real bundled HTMX initialization")
	if string(evaluate(`document.getElementById('load-proof').click();true`)) != "true" {
		t.Fatal("native click did not execute")
	}
	select {
	case <-headersFlushed:
	case <-ctx.Done():
		t.Fatal("the real HTMX request never reached the held response")
	}
	waitFor(`window.__zeHTMXBodyProof.headers===1&&window.__zeHTMXBodyProof.finished===0&&document.getElementById('proof').dataset.state==='initial'`, "HTMX's browser-side header receipt before its body and swap")
	// Derive the existing quiet-window predicate rather than copying its
	// duration. The outstanding-count arm is intentionally omitted only from
	// this observation: the response body is deliberately still outstanding.
	const pendingArm = "window.__zeInflight===0"
	if strings.Count(inflightIdleExpr, pendingArm) != 1 {
		t.Fatal("the quiet-window probe no longer identifies its pending-request arm")
	}
	quietWindow := strings.Replace(inflightIdleExpr, pendingArm, "true", 1)
	waitFor(quietWindow, "the existing request quiet window while the body is held")
	if verdict := evaluate(captureReadyExpr); string(verdict) == "true" {
		t.Error("the real capture predicate accepted headers before the finite HTMX body and swap completed")
	}
	if string(evaluate(`window.__zeCaptureFrames=null;true`)) != "true" {
		t.Fatal("reset the diagnostic capture-frame receipt")
	}
	capture := filepath.Join(t.TempDir(), "htmx-body.png")
	started = true
	go func() { result <- browser.Screenshot(capture) }()
	waitFor(`!!window.__zeCaptureFrames&&window.__zeCaptureFrames.completed===true`, "Screenshot's actual completed render-frame observation")
	// The request/header/frame rendezvous above establishes execution. This
	// bounded negative observation only checks non-completion while the real
	// response body remains held; it does not schedule the interleaving.
	select {
	case err := <-result:
		joined = true
		t.Errorf("Screenshot returned before the held HTMX body was released: %v", err)
	case <-time.After(300 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal("capture did not remain live for its held-body observation")
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Errorf("held HTMX body left a screenshot artifact: %v", err)
	}
	close(releaseBody)
	released = true
	if !joined {
		if err := <-result; err != nil {
			t.Error(err)
		}
		joined = true
	}
	waitFor(`window.__zeHTMXBodyProof.headers===1&&window.__zeHTMXBodyProof.finished===1&&document.getElementById('proof').dataset.state==='swapped'`, "the real HTMX response-body swap and final request hook")
	imageFile, err := os.Open(capture)
	if err != nil {
		t.Fatal(err)
	}
	image, decodeErr := png.Decode(imageFile)
	if err := errors.Join(decodeErr, imageFile.Close()); err != nil {
		t.Fatal(err)
	}
	if image.Bounds().Dx() != 320 || image.Bounds().Dy() != 240 {
		t.Fatalf("native screenshot dimensions = %v, want 320x240", image.Bounds())
	}
	red, green, blue, alpha := image.At(100, 100).RGBA()
	if red != 0 || green != 65535 || blue != 0 || alpha != 65535 {
		t.Errorf("Screenshot did not capture the released, swapped and painted body: R=%d G=%d B=%d A=%d", red, green, blue, alpha)
	}
}
