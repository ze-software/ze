// Design: docs/architecture/testing/runner-architecture.md -- observer installation before application scripts.
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
	"strings"
	"testing"
	"time"
)

// TestBrowserNativeObserverPrecedesPageScript observes the request wrapper from
// the application's first inline script, not a later test eval. Each navigation
// starts a held fetch immediately; Open must wait, and the registered observer
// must survive into the second document. The final actual PNG must be painted.
func TestBrowserNativeObserverPrecedesPageScript(t *testing.T) {
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("native observer lifecycle proof requires agent-browser: %v", err)
	}
	requested := make(chan string, 2)
	gates := map[string]chan struct{}{"first": make(chan struct{}), "second": make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		phase := strings.TrimPrefix(r.URL.Path, "/")
		if _, ok := gates[phase]; !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		page := `<!doctype html><style>html,body{margin:0;background:red;width:100%;height:100%}</style><script>fetch('/held?phase=` + phase + `&installed='+String(window.__zeWaitInstalled===true)).then(()=>document.body.style.background='lime');</script><body>Early request</body>`
		if _, err := w.Write([]byte(page)); err != nil {
			t.Errorf("serve early observer page: %v", err)
		}
	})
	mux.HandleFunc("/held", func(w http.ResponseWriter, r *http.Request) {
		phase := r.URL.Query().Get("phase")
		gate, ok := gates[phase]
		if !ok {
			http.Error(w, "unknown observer phase", http.StatusBadRequest)
			return
		}
		select {
		case requested <- phase + ":" + r.URL.Query().Get("installed"):
		case <-r.Context().Done():
			return
		}
		select {
		case <-gate:
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	browser := newBrowser(server.URL)
	browser.ctx = ctx
	defer func() {
		if err := browser.Close(); err != nil {
			t.Errorf("close native observer lifecycle browser: %v", err)
		}
	}()
	for _, phase := range []string{"first", "second"} {
		gate := gates[phase]
		released := false
		opened := make(chan error, 1)
		joined := false
		defer func() {
			if !released {
				close(gate)
			}
			if !joined {
				cancel()
				<-opened
			}
		}()
		go func() { opened <- browser.Open("/" + phase) }()
		select {
		case observation := <-requested:
			if observation != phase+":true" {
				t.Errorf("application script ran before observation was installed: %s", observation)
			}
		case err := <-opened:
			joined = true
			t.Fatalf("navigation returned before early-request rendezvous: %v", err)
		case <-ctx.Done():
			t.Fatal("application's early fetch did not rendezvous")
		}
		select {
		case err := <-opened:
			joined = true
			t.Fatalf("Open returned while the application's early fetch was held: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
		close(gate)
		released = true
		err := <-opened
		joined = true
		if err != nil {
			t.Fatal(err)
		}
	}
	capture := filepath.Join(t.TempDir(), "early-observed.png")
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
		t.Fatalf("early observed response was not painted: R=%d G=%d B=%d", red, green, blue)
	}
}

// TestBrowserNativeObserverPropagatesException uses a real native daemon and
// owned blank page. A registration-source exception must reach the caller with
// its original JavaScript message, without publishing an installation capability.
func TestBrowserNativeObserverPropagatesException(t *testing.T) {
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("native observer exception proof requires agent-browser: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	browser := newBrowser("")
	browser.ctx = ctx
	defer func() {
		if err := browser.Close(); err != nil {
			t.Errorf("close native observer exception browser: %v", err)
		}
	}()
	err := browser.renderObserverScript(`throw new Error('owned-observer-exception-proof')`)
	if err == nil {
		t.Fatal("native observer evaluation swallowed a browser exception")
	}
	if !strings.Contains(err.Error(), "evaluate request observer on owned blank page") || !strings.Contains(err.Error(), "owned-observer-exception-proof") || !strings.Contains(err.Error(), "Evaluation error") {
		t.Fatalf("native observer lost the exception's stage or original cause: %v", err)
	}
	if browser.observer != nil {
		t.Fatal("failed observer evaluation published an installation capability")
	}
}
