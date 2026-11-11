// Design: website/AI.md -- all homepage project figures refresh from one snapshot.
package browsertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestTheBrowserFillsHomeProjectMetrics reloads the actual site script with two
// snapshots and checks all five marked values, including nested compact strings.
func TestTheBrowserFillsHomeProjectMetrics(t *testing.T) {
	dir := t.TempDir()
	writeFixtureSite(t, dir)
	const metrics = `<section id="project-metrics">
<span data-ze-stat="repo.go_source_lines_display">fallback</span>
<span data-ze-stat="repo.go_test_lines_display">fallback</span>
<span data-ze-stat="repo.go_vendor_lines_display">fallback</span>
<span data-ze-stat="plugin_registrations">fallback</span>
<span data-ze-stat="cli_commands">fallback</span>
</section>`
	page := strings.Replace(fixturePage(""), "<main>", "<main>"+metrics, 1)
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	versions := []struct{ body, text string }{
		{`{"repo":{"go_source_lines_display":"1.57 million","go_test_lines_display":"1.84 million","go_vendor_lines_display":"2.00 million"},"plugin_registrations":96,"cli_commands":499}`, "1.57 million|1.84 million|2.00 million|96|499"},
		{`{"repo":{"go_source_lines_display":"2.35 million","go_test_lines_display":"3.46 million","go_vendor_lines_display":"4.57 million"},"plugin_registrations":87,"cli_commands":612}`, "2.35 million|3.46 million|4.57 million|87|612"},
	}
	var version atomic.Int32
	var requests atomic.Int32
	files := http.FileServer(http.Dir(dir))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == factsPath {
			requests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(versions[version.Load()].body)); err != nil {
				t.Errorf("serve snapshot: %v", err)
			}
			return
		}
		files.ServeHTTP(w, r)
	}))
	defer server.Close()
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(findChrome(t)))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(t.Context(), options...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	ctx, cancelTimeout := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancelTimeout()
	for index, snapshot := range versions {
		version.Store(int32(index))
		requests.Store(0)
		load := chromedp.Navigate(server.URL)
		if index > 0 {
			load = chromedp.Reload()
		}
		if _, err := chromedp.Run(ctx, load); err != nil {
			t.Fatal(err)
		}
		expression := `Array.from(document.querySelectorAll('#project-metrics [data-ze-stat]')).map(node => node.textContent).join('|') === ` + strconv.Quote(snapshot.text)
		if _, err := chromedp.Run(ctx, chromedp.Poll[bool](expression, chromedp.WithPollingTimeout(10*time.Second))); err != nil {
			t.Fatalf("snapshot version %d did not fill every project metric: %v", index, err)
		}
		if got := requests.Load(); got != 1 {
			t.Errorf("snapshot version %d fetched %d times, want once", index, got)
		}
	}
}
