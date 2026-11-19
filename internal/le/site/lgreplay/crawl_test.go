package sitelgreplay

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// fakeFleet answers like a product other than the looking glass where the
// crawl depends on it: a page that loads files under two asset prefixes and
// links one that is under neither, and fragments that name further fragments.
func fakeFleet(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	answer := func(contentType, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", contentType)
			fmt.Fprint(w, body) //nolint:errcheck // a test server's fixed answer
		}
	}
	const markup = "text/html; charset=utf-8"
	mux.HandleFunc("GET /fleet/", answer(markup, `<html><head><link rel="stylesheet" href="/fleet/assets/shell.css">`+
		`<script src="/assets/htmx.min.js" defer></script></head><body><a href="/">home</a>`+
		`<nav hx-get="/fleet/tree?view=site"></nav></body></html>`))
	mux.HandleFunc("GET /fleet/tree", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerHTMX) != "true" {
			http.Error(w, "the crawl asked for the page, not the fragment", http.StatusTeapot)
			return
		}
		answer(markup, `<li hx-get="/fleet/device/r1"></li><li hx-get="/fleet/device/r2"></li>`)(w, r)
	})
	mux.HandleFunc("GET /fleet/device/{name}", answer(markup, `<p>a device</p>`))
	mux.HandleFunc("GET /fleet/assets/shell.css", answer("text/css; charset=utf-8", "body{}"))
	mux.HandleFunc("GET /assets/htmx.min.js", answer("text/javascript; charset=utf-8", "var htmx;"))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// TestCrawlerFollowsTheAnswersWithinItsBounds proves a crawl of a second
// product asks its steps, follows every hx-get its answers name, fetches the
// files under each asset prefix and nothing outside them, and changes no
// answer; and that a walk past the exchange bound is refused, naming it.
func TestCrawlerFollowsTheAnswersWithinItsBounds(t *testing.T) {
	server := fakeFleet(t)
	crawler := Crawler{
		Base: server.URL, Client: server.Client(), AssetPrefixes: []string{"/fleet/assets/", "/assets/"},
		ExchangesMax: 4, BodyBytesMax: 1 << 10,
	}
	crawled, err := crawler.Crawl(t.Context(), []Request{{Method: "get", Path: "/fleet/"}})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, exchange := range crawled.Exchanges {
		keys = append(keys, exchange.Key+" via "+exchange.Via)
	}
	want := []string{
		"GET /fleet/ via tour", "GET /fleet/tree?view=site htmx via link",
		"GET /fleet/device/r1 htmx via link", "GET /fleet/device/r2 htmx via link",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("crawled exchanges:\n got %q\nwant %q", keys, want)
	}
	if body := crawled.Exchanges[0].Body; !strings.Contains(body, `<a href="/">home</a>`) {
		t.Fatalf("the crawl changed the answer: %s", body)
	}
	var assets []string
	for _, asset := range crawled.Assets {
		assets = append(assets, asset.Path+"="+asset.Body)
	}
	if want := []string{"/fleet/assets/shell.css=body{}", "/assets/htmx.min.js=var htmx;"}; !reflect.DeepEqual(assets, want) {
		t.Fatalf("crawled assets: got %q, want %q", assets, want)
	}

	crawler.ExchangesMax = 3
	if _, err := crawler.Crawl(t.Context(), []Request{{Method: "GET", Path: "/fleet/"}}); err == nil ||
		!strings.Contains(err.Error(), "reached 3 requests") {
		t.Fatalf("a crawl past its bound: got %v, want a refusal naming the bound", err)
	}
	crawler.ExchangesMax = 4
	crawler.BodyBytesMax = 8
	if _, err := crawler.Crawl(t.Context(), []Request{{Method: "GET", Path: "/fleet/"}}); err == nil ||
		!strings.Contains(err.Error(), "larger than 8 bytes") {
		t.Fatalf("an answer past the size bound: got %v, want a refusal naming the bound", err)
	}
}

// TestCrawlerRefusesSettingsThatCannotDriveACrawl proves a crawl with no
// client, no bound, a prefix that is no path, no step or a repeated step is
// refused before or at its first request.
func TestCrawlerRefusesSettingsThatCannotDriveACrawl(t *testing.T) {
	server := fakeFleet(t)
	valid := Crawler{Base: server.URL, Client: server.Client(), ExchangesMax: 10, BodyBytesMax: 1 << 10}
	page := Request{Method: "GET", Path: "/fleet/"}
	for name, tc := range map[string]struct {
		change func(*Crawler)
		steps  []Request
	}{
		"no client":           {change: func(c *Crawler) { c.Client = nil }, steps: []Request{page}},
		"no exchange bound":   {change: func(c *Crawler) { c.ExchangesMax = 0 }, steps: []Request{page}},
		"no size bound":       {change: func(c *Crawler) { c.BodyBytesMax = 0 }, steps: []Request{page}},
		"a prefix not a path": {change: func(c *Crawler) { c.AssetPrefixes = []string{"assets/"} }, steps: []Request{page}},
		"no step":             {change: func(*Crawler) {}, steps: nil},
		"a step twice":        {change: func(*Crawler) {}, steps: []Request{page, page}},
	} {
		t.Run(name, func(t *testing.T) {
			crawler := valid
			tc.change(&crawler)
			if _, err := crawler.Crawl(t.Context(), tc.steps); err == nil {
				t.Fatal("the crawl was accepted")
			}
		})
	}
}
