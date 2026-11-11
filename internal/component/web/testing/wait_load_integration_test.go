// Design: docs/architecture/testing/runner-architecture.md -- the click settle wait.
// Related: inflight.go -- the request observer the wait reads.
//go:build integration

package webtesting

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// waitLoadSlowAnswer is how long /slow takes to answer. It is the subject of
// the test, not a guess: it lies above the fixed 5s settle bound waitLoad once
// carried, the bound a commit's 5.1s durable store write outlasted.
const waitLoadSlowAnswer = 6 * time.Second

// waitLoadPage serves two buttons, each fetching its own path and writing the
// answer into #answer once the response arrives.
const waitLoadPage = `<!doctype html><html><head><script>
function zeAsk(path){fetch(path).then(function(r){return r.text();}).then(function(t){document.getElementById('answer').textContent=t;});}
</script></head><body>
<button id="slow" onclick="zeAsk('/slow')">slow</button>
<button id="never" onclick="zeAsk('/never')">never</button>
<p id="answer">none</p></body></html>`

// TestBrowserClickWaitsForSlowRequestUntilCaseDeadline proves a click's settle
// wait is bounded by the case deadline and by nothing shorter. Method: a real
// browser clicks a button whose request the server answers only after
// waitLoadSlowAnswer, under a case context of 30s; the click must return
// without error, after the server answered, with the answer on the page. A
// second button's request is never answered, under a 4s case context; the
// click must fail with the deadline and name the request still pending.
// MUTATION: bounding waitLoad by a fixed 5s (the old waitLoadDeadline) fails
// the slow click with "browser requests did not settle"; dropping
// describePending from the deadline error loses "fetch GET /never pending".
func TestBrowserClickWaitsForSlowRequestUntilCaseDeadline(t *testing.T) {
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("click settle proof requires agent-browser: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	release := make(chan struct{})
	var slowAnswered atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Cache-Control", "no-store")
		if _, err := w.Write([]byte(waitLoadPage)); err != nil {
			t.Errorf("serve click settle page: %v", err)
		}
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(waitLoadSlowAnswer)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-r.Context().Done():
			return
		}
		slowAnswered.Store(true)
		if _, err := w.Write([]byte("slow answered")); err != nil {
			t.Errorf("answer /slow: %v", err)
		}
	})
	mux.HandleFunc("/never", func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	defer close(release)

	browser := newBrowser(server.URL)
	browser.ctx = ctx
	defer func() {
		if err := browser.Close(); err != nil {
			t.Errorf("close click settle browser: %v", err)
		}
	}()
	if err := browser.Open("/"); err != nil {
		t.Fatal(err)
	}

	if err := browser.clickID("slow"); err != nil {
		t.Fatalf("click on a request answered after %s failed under a 30s case: %v", waitLoadSlowAnswer, err)
	}
	if !slowAnswered.Load() {
		t.Fatal("the click returned before the server answered /slow, so the wait did not wait for it")
	}
	html, err := browser.getHTML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "slow answered") {
		t.Fatalf("the slow answer never reached the page after the click settled: %s", html)
	}

	parent := browser.ctx
	short, cancelShort := context.WithTimeout(parent, 4*time.Second)
	browser.ctx = short
	err = browser.clickID("never")
	cancelShort()
	browser.ctx = parent
	if err == nil {
		t.Fatal("a click whose request never answers passed")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a click whose request never answers did not fail at the case deadline: %v", err)
	}
	if !strings.Contains(err.Error(), "fetch GET /never pending") {
		t.Fatalf("the deadline error does not name the pending request: %v", err)
	}
}
