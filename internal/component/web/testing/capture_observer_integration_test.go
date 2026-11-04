// Design: docs/architecture/testing/runner-architecture.md -- capture requires request observation.
//go:build integration

package webtesting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBrowserScreenshotNativeMissingObserver starts a held fetch in real
// Chromium, then deliberately removes its request-observation state.
// Missing observation must refuse capture without publishing unsettled pixels.
func TestBrowserScreenshotNativeMissingObserver(t *testing.T) {
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("real missing-observer proof requires agent-browser: %v", err)
	}
	requested := make(chan struct{}, 1)
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(`<!doctype html><style>html,body{margin:0;background:red}</style><body>Held fetch</body>`)); err != nil {
			t.Errorf("serve held-fetch page: %v", err)
		}
	})
	mux.HandleFunc("/held", func(w http.ResponseWriter, r *http.Request) {
		requested <- struct{}{}
		select {
		case <-release:
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	browser := newBrowser(server.URL)
	browser.ctx = ctx
	defer func() {
		if err := browser.Close(); err != nil {
			t.Errorf("close missing-observer browser: %v", err)
		}
	}()
	if err := browser.Open("/"); err != nil {
		t.Fatal(err)
	}
	observed, err := browser.runAgentOutput("eval", `fetch('/held').then(()=>document.body.style.background='lime');delete window.__zeWaitInstalled;delete window.__zeInflight;delete window.__zeLastChange;typeof window.__zeWaitInstalled==='undefined'&&typeof window.__zeInflight==='undefined'&&typeof window.__zeLastChange==='undefined'`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(observed) != "true" {
		t.Fatalf("fixture did not reach absent request observation: %s", observed)
	}
	select {
	case <-requested:
	case <-ctx.Done():
		t.Fatal("missing-observer fetch never rendezvoused")
	}
	limited, finish := context.WithTimeout(ctx, time.Second)
	defer finish()
	browser.ctx = limited
	capture := filepath.Join(t.TempDir(), "unobserved.png")
	if err := browser.Screenshot(capture); err == nil {
		t.Error("captured while request observation was absent and a real fetch remained held")
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Errorf("missing observation published an unsettled capture artifact: %v", err)
	}
}
