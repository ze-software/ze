// Design: docs/architecture/testing/runner-architecture.md -- real settled captures.
// Related: capture_integration_test.go -- font, request, animation and frame settling.
//go:build integration

package webtesting

import (
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestBrowserScreenshotSettlesCSSTransition judges a real Chromium capture
// taken while a CSS transition runs. The web UI gives `.config-input` and
// `.btn` border and background transitions (internal/component/web/assets/style.css),
// and a poster captured inside one differs in the edge pixels the
// 2026-08-24 web-config poster row in plan/journal/output-not-byte-stable.md
// recorded. A CSS transition is a finite document animation, so the capture
// must wait for it to end and see only the final color.
// MUTATION: dropping the finite-animation clause from captureReadyExpr captures
// mid-transition and fails the pixel assertion.
func TestBrowserScreenshotSettlesCSSTransition(t *testing.T) {
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("real capture proof requires agent-browser: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(`<!doctype html><style>html,body{margin:0;width:100%;height:100%;background-color:red}</style><body></body>`)); err != nil {
			t.Errorf("serve capture page: %v", err)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	browser := newBrowser(server.URL)
	browser.ctx = ctx
	defer func() {
		if err := browser.Close(); err != nil {
			t.Errorf("close native capture browser: %v", err)
		}
	}()
	if err := browser.setViewport(320, 240); err != nil {
		t.Fatal(err)
	}
	if err := browser.Open("/"); err != nil {
		t.Fatal(err)
	}
	// Reading the computed color flushes style, so the change below starts a
	// transition rather than replacing the color before the first frame.
	script := `document.body.style.transition='background-color 1200ms linear';` +
		`getComputedStyle(document.body).backgroundColor;` +
		`document.body.style.backgroundColor='lime';true`
	if _, err := browser.runAgentOutput("eval", script); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "capture.png")
	if err := browser.Screenshot(capture); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(capture)
	if err != nil {
		t.Fatal(err)
	}
	image, decodeErr := png.Decode(file)
	closeErr := file.Close()
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	red, green, blue, _ := image.At(100, 100).RGBA()
	if red != 0 || green != 65535 || blue != 0 {
		t.Fatalf("captured inside the CSS transition: R=%d G=%d B=%d, want the settled lime", red, green, blue)
	}
}
