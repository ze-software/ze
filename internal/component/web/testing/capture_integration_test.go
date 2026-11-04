// Design: docs/architecture/testing/runner-architecture.md -- real settled captures.
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
	"runtime"
	"testing"
	"time"
)

// TestBrowserScreenshotNativeSettling judges captured pixels from real Chromium,
// independently holding a font, a fetch, a finite animation and two render
// frames. A still-animated page at the case deadline must produce no artifact.
func TestBrowserScreenshotNativeSettling(t *testing.T) {
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("real capture proof requires agent-browser: %v", err)
	}
	for _, mode := range []string{"font", "network", "animation", "frames", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			requested := make(chan struct{}, 2)
			release := make(chan struct{})
			defer close(release)
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, err := w.Write([]byte(`<!doctype html><style>html,body{margin:0;width:100%;height:100%;background:red}body{font-size:20px}</style><body>Capture proof</body>`))
				if err != nil {
					t.Errorf("serve capture page: %v", err)
				}
			})
			mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
				requested <- struct{}{}
				select {
				case <-release:
					w.WriteHeader(http.StatusNoContent)
				case <-r.Context().Done():
				}
			})
			mux.HandleFunc("/font", func(w http.ResponseWriter, r *http.Request) {
				requested <- struct{}{}
				select {
				case <-release:
					_, file, _, ok := runtime.Caller(0)
					if !ok {
						t.Error("locate vendored capture font")
						return
					}
					font := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "demos", "terminal", "fonts", "fira-code", "FiraCode-Regular.ttf")
					w.Header().Set("Content-Type", "font/ttf")
					http.ServeFile(w, r, font)
				case <-r.Context().Done():
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
			var script string
			switch mode {
			case "font":
				script = `var face=new FontFace('CaptureFont','url(/font)');document.fonts.add(face);document.body.style.fontFamily='CaptureFont';document.fonts.load('20px CaptureFont').then(()=>{document.body.style.background='lime';});true`
			case "network":
				script = `fetch('/ready').then(()=>{document.body.style.background='lime';});true`
			case "animation":
				script = `document.body.animate([{backgroundColor:'red'},{backgroundColor:'lime'}],{duration:1200,fill:'forwards'});true`
			case "frames":
				script = `requestAnimationFrame(()=>requestAnimationFrame(()=>{document.body.style.background='lime';}));true`
			case "deadline":
				script = `document.body.animate([{backgroundColor:'red'},{backgroundColor:'lime'}],{duration:30000,fill:'forwards'});true`
			}
			if _, err := browser.runAgentOutput("eval", script); err != nil {
				t.Fatal(err)
			}
			if mode == "font" || mode == "network" {
				select {
				case <-requested:
				case <-ctx.Done():
					t.Fatal("delayed render resource was never requested")
				}
			}
			if mode == "deadline" {
				limited, finish := context.WithTimeout(ctx, 750*time.Millisecond)
				defer finish()
				browser.ctx = limited
			}
			capture := filepath.Join(t.TempDir(), "capture.png")
			result := make(chan error, 1)
			go func() { result <- browser.Screenshot(capture) }()
			if mode == "font" || mode == "network" {
				// This is a negative observation of the held resource, not a
				// settling delay: release happens only after early capture is refused.
				select {
				case err := <-result:
					t.Fatalf("capture returned while its %s was held: %v", mode, err)
				case <-time.After(300 * time.Millisecond):
				}
				release <- struct{}{}
			}
			err := <-result
			if mode == "deadline" {
				if err == nil {
					t.Fatal("captured an animation that outlives the case")
				}
				if _, statErr := os.Stat(capture); !os.IsNotExist(statErr) {
					t.Fatalf("deadline left a capture artifact: %v", statErr)
				}
				return
			}
			if err != nil {
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
				t.Fatalf("captured unsettled %s pixels: R=%d G=%d B=%d", mode, red, green, blue)
			}
		})
	}
}
