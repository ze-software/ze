// Design: (none -- no architecture doc covers the finder's push-URL contract)
// Related: test/web/history-full-page.wb — the browser half of AC-7

package web

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// pushedURL reads what the markup asks the browser to put in its address bar.
// The finder writes it on every item, every path segment and every table row.
var pushedURL = regexp.MustCompile(`hx-push-url="([^"]+)"`)

// pushedURLFloor guards this check against measuring nothing. The finder walks
// the YANG schema, so the population moves with the schema and a number typed
// here would be wrong by the next leaf. What must never happen is the crawl
// finding a handful: that is a renderer that stopped pushing, and every
// assertion below would then pass over an empty set.
const pushedURLFloor = 50

// VALIDATES: AC-7 — every URL the finder pushes answers a COMPLETE page on a
// direct GET, not the fragment an htmx swap would have received.
// PREVENTS: the failure htmx 4 makes reachable. htmx 2 kept a history cache and
// restored the previous DOM on back, so a pushed URL that answers only a
// fragment was never noticed. htmx 4 keeps no cache and traverses through the
// Navigation API, so the server's answer is what the operator sees. The .wb
// proves the browser half on two URLs; this holds the whole population.
func TestPushedURLsAnswerAFullPage(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	schema, schemaErr := config.YANGSchema()
	if schemaErr != nil {
		t.Fatalf("YANGSchema: %v", schemaErr)
	}

	handler := HandleFragment(renderer, schema, config.NewTree(), nil, false)

	// The crawl starts where the operator starts and follows what the markup
	// pushes, so the population is DERIVED. A list typed here would hold what
	// somebody believed the finder renders on the day they wrote it.
	fetched := make(map[string]bool)
	pushedBy := make(map[string]string)
	queue := []string{"/show/"}

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		if fetched[path] {
			continue
		}
		fetched[path] = true

		body := fullPageBody(t, handler, path, pushedBy[path])

		for _, m := range pushedURL.FindAllStringSubmatch(body, -1) {
			url := m[1]
			if _, known := pushedBy[url]; !known {
				pushedBy[url] = path
			}
			if strings.HasPrefix(url, "/show/") && !fetched[url] {
				queue = append(queue, url)
			}
		}
	}

	if len(pushedBy) < pushedURLFloor {
		t.Errorf("the finder pushed %d URLs from %d pages; below %d this check is measuring a tree that did not render",
			len(pushedBy), len(fetched), pushedURLFloor)
	}

	// Every pushed URL must have been fetched. One outside /show/ would leave
	// the loop above without a fetch, and its answer would go unread.
	for url, from := range pushedBy {
		if !fetched[url] {
			t.Errorf("%s pushes %s, which this check never fetched", from, url)
		}
	}
}

// fullPageBody fetches one URL the way the browser fetches a restored history
// entry -- a plain GET with no HX-Request header -- and fails when the answer is
// anything less than a whole page.
func fullPageBody(t *testing.T, handler http.HandlerFunc, path, from string) string {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))

	if rec.Code != http.StatusOK {
		t.Errorf("GET %s (pushed by %s) answered %d, want 200", path, from, rec.Code)
		return ""
	}

	body := rec.Body.String()

	// A fragment carries the split alone. A page carries the document around
	// it, and the head that loads the library the swap depends on. Each of the
	// three tells a different way of answering less than a page.
	for _, needle := range []string{"<html", "htmx.min.js", "main-split"} {
		if !strings.Contains(body, needle) {
			t.Errorf("GET %s (pushed by %s) answered %d bytes without %q: back or forward would render this",
				path, from, len(body), needle)
		}
	}

	return body
}
