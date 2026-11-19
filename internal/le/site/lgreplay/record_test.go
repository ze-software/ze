package sitelgreplay

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeLookingGlass answers like the looking glass does where the recorder
// depends on it: a whole page to a plain request and a fragment to an htmx
// one, an uptime that grows with every read, one answer that changes with
// every read and that no rule covers, two assets, and an event stream.
func fakeLookingGlass(t *testing.T) *httptest.Server {
	t.Helper()
	var reads atomic.Int64
	mux := http.NewServeMux()
	markup := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write the answer: %v", err)
		}
	}
	mux.HandleFunc("GET /lg/peers", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerHTMX) == "true" {
			markup(w, `<p>the peers fragment</p>`)
			return
		}
		markup(w, fmt.Sprintf(`<html><head><link rel="stylesheet" href="/lg/assets/style.css">`+
			`<script src="/lg/assets/app.js"></script></head><body><table>`+
			`<tbody hx-sse:connect="/lg/events"><tr hx-get="/lg/peer/203.0.113.1">`+
			`<td>203.0.113.1</td><td>%ds</td></tr></tbody></table></body></html>`, reads.Add(1)))
	})
	mux.HandleFunc("GET /lg/peer/203.0.113.1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerHTMX) != "true" {
			http.Error(w, "the recorder asked for the page, not the fragment", http.StatusTeapot)
			return
		}
		markup(w, `<tr hx-get="/lg/route/detail?prefix=198.18.0.0/24&amp;peer=203.0.113.1"><td>198.18.0.0/24</td></tr>`)
	})
	mux.HandleFunc("GET /lg/route/detail", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("prefix") != "198.18.0.0/24" {
			http.Error(w, "the prefix did not arrive decoded", http.StatusBadRequest)
			return
		}
		markup(w, fmt.Sprintf(`<dl><dt>Read</dt><dd>number %d</dd></dl>`, reads.Add(1)))
	})
	mux.HandleFunc("GET /lg/assets/style.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		fmt.Fprint(w, "body{}") //nolint:errcheck // a test server's fixed answer
	})
	mux.HandleFunc("GET /lg/assets/app.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		fmt.Fprint(w, "var app;") //nolint:errcheck // a test server's fixed answer
	})
	mux.HandleFunc("GET /lg/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", streamContentType)
		for index := 1; index <= streamMessages+1; index++ {
			fmt.Fprintf(w, "data: <tr><td>203.0.113.1</td>\ndata: <td>%ds</td></tr>\n\n", index) //nolint:errcheck // a test server's fixed answer
			if err := http.NewResponseController(w).Flush(); err != nil {
				return
			}
		}
		<-r.Context().Done()
	})
	mux.HandleFunc("GET /lg/binary", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		if _, err := w.Write([]byte{0x1f, 0x8b, 0xff, 0xfe}); err != nil {
			t.Errorf("write the answer: %v", err)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// TestRecordAsksTheTourAndDeclaresWhatVaries proves one run records the tour,
// the requests its answers name, the assets and the stream sample, fixes what
// a rule covers, and names the answer no rule covers.
//
// Method: a run against fakeLookingGlass with a wait that returns at once and
// records the gap it was asked for.
func TestRecordAsksTheTourAndDeclaresWhatVaries(t *testing.T) {
	server := fakeLookingGlass(t)
	var waited []time.Duration
	lookingGlass := lookingGlass{base: server.URL, wait: func(_ context.Context, d time.Duration) error {
		waited = append(waited, d)
		return nil
	}}
	steps := []Step{
		{Caption: "The peers page.", Method: methodGet, Target: "/lg/peers"},
		{Caption: "One peer.", Method: methodGet, Target: "/lg/peer/203.0.113.1", HTMX: true},
	}
	recording, err := record(t.Context(), lookingGlass, steps, Provenance{Recorder: RecorderCommand, Seed: 7})
	if err != nil {
		t.Fatal(err)
	}

	detail := "GET /lg/route/detail?peer=203.0.113.1&prefix=198.18.0.0/24 htmx"
	var keys, via []string
	for _, exchange := range recording.Exchanges {
		keys = append(keys, exchange.Key)
		via = append(via, exchange.Via)
	}
	if want := []string{"GET /lg/peers", "GET /lg/peer/203.0.113.1 htmx", detail}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("recorded requests: got %q, want %q", keys, want)
	}
	if want := []string{viaTour, viaTour, viaLink}; !reflect.DeepEqual(via, want) {
		t.Fatalf("how each request was found: got %q, want %q", via, want)
	}
	if recording.Entry != "GET /lg/peers" {
		t.Fatalf("entry: got %q", recording.Entry)
	}
	if want := []TourStep{{Caption: "The peers page.", Exchange: "GET /lg/peers"}, {Caption: "One peer.", Exchange: "GET /lg/peer/203.0.113.1 htmx"}}; !reflect.DeepEqual(recording.Tour, want) {
		t.Fatalf("tour: got %+v, want %+v", recording.Tour, want)
	}
	if !strings.Contains(recording.Exchanges[0].Body, "<td>"+fixedUptime+"</td>") {
		t.Fatalf("the uptime of the page was not fixed: %s", recording.Exchanges[0].Body)
	}

	var assets []string
	for _, asset := range recording.Assets {
		assets = append(assets, asset.Path+" "+asset.Body)
	}
	if want := []string{"/lg/assets/style.css body{}", "/lg/assets/app.js var app;"}; !reflect.DeepEqual(assets, want) {
		t.Fatalf("assets: got %q, want %q", assets, want)
	}

	if recording.Stream == nil {
		t.Fatal("the page names a stream and the recording holds none")
	}
	message := "data: <tr><td>203.0.113.1</td>\ndata: <td>" + fixedUptime + "</td></tr>"
	if want := []string{message, message}; !reflect.DeepEqual(recording.Stream.Messages, want) {
		t.Fatalf("stream sample: got %q, want %q", recording.Stream.Messages, want)
	}

	// The page's uptime grew between the two reads and a rule fixed it. The
	// route detail's read number grew and no rule covers it.
	if want := []string{detail}; !reflect.DeepEqual(recording.Provenance.Varies, want) {
		t.Fatalf("varies: got %q, want %q", recording.Provenance.Varies, want)
	}
	if want := []time.Duration{secondReadGap}; !reflect.DeepEqual(waited, want) {
		t.Fatalf("wait before the second read: got %v, want %v", waited, want)
	}
	counts := map[string]int{}
	for _, each := range recording.Provenance.Normalized {
		counts[each.Name] = each.Count
	}
	// One uptime in the page and one in each of the two stream messages.
	if counts["uptime"] != 3 {
		t.Fatalf("uptime replacements declared: got %d, want 3 (%v)", counts["uptime"], counts)
	}
	if recording.Provenance.Seed != 7 {
		t.Fatalf("the caller's provenance was lost: %+v", recording.Provenance)
	}
}

// TestRecordRefusesAnAnswerThatIsNotText proves a run that meets a binary
// answer, a refused question or a repeated step ends with an error naming it,
// and answers no recording.
func TestRecordRefusesAnAnswerThatIsNotText(t *testing.T) {
	server := fakeLookingGlass(t)
	lookingGlass := lookingGlass{base: server.URL, wait: func(context.Context, time.Duration) error { return nil }}
	cases := []struct {
		name  string
		steps []Step
		want  string
	}{
		{"binary answer", []Step{{Method: methodGet, Target: "/lg/binary"}}, "is not text"},
		{"refused question", []Step{{Method: methodGet, Target: "/lg/absent"}}, "404"},
		{"repeated step", []Step{{Method: methodGet, Target: "/lg/peers"}, {Method: methodGet, Target: "/lg/peers"}}, "already asks"},
		{"empty tour", nil, "tour is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recording, err := record(t.Context(), lookingGlass, tc.steps, Provenance{})
			if err == nil {
				t.Fatalf("recorded %d answers where a refusal was owed", len(recording.Exchanges))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not say %q", err, tc.want)
			}
		})
	}
}

// TestTourStepsAreDistinctQuestions proves the declared tour resolves to
// requests with distinct keys and opens on a plain page, which is what record
// requires of it before any network runs.
func TestTourStepsAreDistinctQuestions(t *testing.T) {
	values := strings.NewReplacer("{peer}", "203.0.113.1", "{announcer}", "203.0.113.1",
		"{prefix}", "198.18.0.0/24", "{address}", "198.18.0.1", "{origin}", "64501",
		"{router}", "198.51.100.3")
	seen := map[string]bool{}
	for index, step := range resolve(tour, values) {
		if strings.ContainsAny(step.Caption+step.Target, "{}") {
			t.Errorf("step %d keeps an unfilled subject: %q %q", index+1, step.Caption, step.Target)
		}
		path, query, err := splitTarget(step.Target)
		if err != nil {
			t.Fatalf("step %d: %v", index+1, err)
		}
		key := ExchangeKey(step.Method, path, query, step.Form, step.HTMX)
		if seen[key] {
			t.Errorf("step %d repeats %s", index+1, key)
		}
		seen[key] = true
	}
	if tour[0].HTMX {
		t.Error("the tour opens on a fragment, and a replay opens on a page")
	}
}
