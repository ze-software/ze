// Design: docs/architecture/testing/runner-architecture.md -- in-page wait.
// Related: wait_html_test.go -- the grammar refusals, with no browser.
//go:build integration

package webtesting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// waitHTMLPage serves a page whose #commit-bar carries "commit-bar visible"
// and, when clearAfter is positive, a script that drops the class after that
// delay, the way the commit answer's out-of-band swap does. The script sits in
// the head: getHTML reads the body, so the text being waited on lives in the
// markup alone. Every load stamps window.zeLoadMark with a fresh value.
func waitHTMLPage(clearAfter time.Duration) string {
	var page strings.Builder
	page.WriteString(`<!doctype html><html><head><script>`)
	page.WriteString(`window.zeLoadMark=String(Math.random());`)
	if clearAfter > 0 {
		page.WriteString(`setTimeout(function(){document.getElementById('commit-bar').className='commit-bar'},`)
		page.WriteString(strconv.Itoa(int(clearAfter.Milliseconds())))
		page.WriteString(`);`)
	}
	page.WriteString(`</script></head><body><div id="commit-bar" class="commit-bar visible">pending</div></body></html>`)
	return page.String()
}

// TestBrowserWaitHTMLPollsInPlace proves `action=wait-html` judges the page in
// hand and never navigates. Method: a real browser opens a page whose bar
// class clears after a delay, and a second page whose class never clears. The
// first wait must pass after the text was observed present; the second must
// fail with the named message once its deadline passes. Both pages must be
// served exactly once, and the load mark the page stamped and the URL must be
// unchanged after the wait, so a reload or a re-open cannot pass.
// MUTATION: making waitHTMLState call b.Open (as waitUntil does) serves each
// page again and changes the load mark; making it return nil without reading
// the page passes the never-clearing page, which must fail.
func TestBrowserWaitHTMLPollsInPlace(t *testing.T) {
	if _, err := exec.LookPath(agentBrowserBin); err != nil {
		t.Fatalf("in-page wait proof requires agent-browser: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	var clearsServed, staysServed atomic.Int64
	mux := http.NewServeMux()
	serve := func(counter *atomic.Int64, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			counter.Add(1)
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Cache-Control", "no-store")
			if _, err := w.Write([]byte(body)); err != nil {
				t.Errorf("serve wait-html page: %v", err)
			}
		}
	}
	mux.HandleFunc("/clears", serve(&clearsServed, waitHTMLPage(2500*time.Millisecond)))
	mux.HandleFunc("/stays", serve(&staysServed, waitHTMLPage(0)))
	server := httptest.NewServer(mux)
	defer server.Close()

	browser := newBrowser(server.URL)
	browser.ctx = ctx
	defer func() {
		if err := browser.Close(); err != nil {
			t.Errorf("close wait-html browser: %v", err)
		}
	}()

	const barVisible = "commit-bar visible"
	condition := map[string]string{"not-contains": barVisible}

	// The bar clears in place: the wait passes, and only after the text was there.
	if err := browser.Open("/clears"); err != nil {
		t.Fatal(err)
	}
	before := waitHTMLObserve(t, browser)
	html, err := browser.getHTML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, barVisible) {
		t.Fatalf("the clearing page lost %q before the wait began, so the wait proves nothing: %s", barVisible, html)
	}
	if err := browser.waitHTML(condition); err != nil {
		t.Fatalf("wait-html on a bar that clears in place: %v", err)
	}
	after := waitHTMLObserve(t, browser)
	if after != before {
		t.Fatalf("wait-html changed the page: before %q, after %q", before, after)
	}
	if served := clearsServed.Load(); served != 1 {
		t.Fatalf("/clears served %d times, want 1: the wait re-opened the page", served)
	}

	// The bar never clears: the wait fails at its deadline and names the text.
	if err := browser.Open("/stays"); err != nil {
		t.Fatal(err)
	}
	before = waitHTMLObserve(t, browser)
	parent := browser.ctx
	short, cancelShort := context.WithTimeout(parent, 3*time.Second)
	browser.ctx = short
	err = browser.waitHTML(condition)
	cancelShort()
	browser.ctx = parent
	if err == nil {
		t.Fatalf("wait-html passed on a bar that never clears")
	}
	if !strings.Contains(err.Error(), `current page still contains "commit-bar visible"`) {
		t.Fatalf("wait-html failed without naming the text it waited on: %v", err)
	}
	after = waitHTMLObserve(t, browser)
	if after != before {
		t.Fatalf("wait-html changed the page while failing: before %q, after %q", before, after)
	}
	if served := staysServed.Load(); served != 1 {
		t.Fatalf("/stays served %d times, want 1: the wait re-opened the page", served)
	}
}

// waitHTMLObserve returns the address and the load mark of the page in hand.
// A navigation or reload changes the mark even when the URL stays the same.
func waitHTMLObserve(t *testing.T, browser *Browser) string {
	t.Helper()
	url, err := browser.getURL()
	if err != nil {
		t.Fatal(err)
	}
	mark, err := browser.runAgentOutput("eval", "window.zeLoadMark")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(mark) == "" {
		t.Fatalf("the page carries no load mark, so a reload could not be seen")
	}
	return strings.TrimSpace(url) + " " + strings.TrimSpace(mark)
}
