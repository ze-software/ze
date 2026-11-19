package sitelgreplay

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// testListener is the address the looking glass of a test run listened on.
const testListener = "127.0.0.1:28471"

// normalizedCounts answers each rule's count by its name.
func normalizedCounts(n *normalizer) map[string]int {
	counts := map[string]int{}
	for _, each := range n.declared() {
		counts[each.Name] = each.Count
	}
	return counts
}

// TestNormalizeFixesWhatVariesRunToRun proves the values that depend on when a
// run was made become the fixed values the file declares, each counted.
//
// Method: one page holding two uptimes in two of formatUptime's forms, two
// state-change cells, one peer header and the listener address, compared with
// the whole expected text.
func TestNormalizeFixesWhatVariesRunToRun(t *testing.T) {
	n := newNormalizer(testListener)
	page := `<tr><td>198.51.100.1</td><td class="state">established</td><td>6m 10s</td>` +
		`<td class="state-changed">2026-11-18T21:44:03Z</td><td>9</td></tr>` +
		`<tr><td>203.0.113.1</td><td>1h 2m 3s</td><td class="state-changed">2026-11-18T21:44:05Z</td></tr>` +
		`<span class="peer-info-state-changed">Last change <time>2026-11-18T21:44:03Z</time></span>` +
		`<a href="http://127.0.0.1:28471/lg/peers">peers</a>` +
		`<td class="route-age" data-seconds="1">1s</td><td class="route-age"></td>` +
		`<dt>Age</dt><dd class="route-age" data-seconds="4000">1h 6m 40s</dd>`
	// A known age becomes the fixed one in both of its forms, in a table cell
	// and in a route's detail. A cell with no known age stays empty.
	ages := `<td class="route-age" data-seconds="192">3m 12s</td><td class="route-age"></td>` +
		`<dt>Age</dt><dd class="route-age" data-seconds="192">3m 12s</dd>`
	want := `<tr><td>198.51.100.1</td><td class="state">established</td><td>1m 0s</td>` +
		`<td class="state-changed">2026-01-01T00:00:00Z</td><td>9</td></tr>` +
		`<tr><td>203.0.113.1</td><td>1m 0s</td><td class="state-changed">2026-01-01T00:00:00Z</td></tr>` +
		`<span class="peer-info-state-changed">Last change <time>2026-01-01T00:00:00Z</time></span>` +
		`<a href="http://looking-glass.example/lg/peers">peers</a>` + ages
	if got := n.apply(page, true); got != want {
		t.Fatalf("normalized page:\n got %s\nwant %s", got, want)
	}
	wantCounts := map[string]int{"uptime": 2, "state-changed": 2, "route-age": 2, "peer-state-changed": 1, "listener": 1}
	if got := normalizedCounts(n); !reflect.DeepEqual(got, wantCounts) {
		t.Fatalf("declared counts: got %v, want %v", got, wantCounts)
	}
	// The second read of a run is normalized and not counted.
	if got := n.apply(page, false); got != want {
		t.Fatalf("uncounted read differs: %s", got)
	}
	if got := normalizedCounts(n); !reflect.DeepEqual(got, wantCounts) {
		t.Fatalf("an uncounted read changed the counts: %v", got)
	}
}

// TestNormalizeLeavesStableTextAlone proves a rule touches only the element
// that holds a run-dependent value: a route count, an AS number, a prefix and
// a state keep their text, and every rule declares a count of zero.
func TestNormalizeLeavesStableTextAlone(t *testing.T) {
	n := newNormalizer(testListener)
	page := `<td>9</td><td>65001</td><td>198.18.0.0/24</td><td class="state">established</td>` +
		`<td>1,024</td><td class="connections-dropped">2</td><td>30s ago</td>`
	if got := n.apply(page, true); got != page {
		t.Fatalf("stable text changed:\n got %s\nwant %s", got, page)
	}
	for name, count := range normalizedCounts(n) {
		if count != 0 {
			t.Errorf("rule %s declares %d replacements on a page that holds none", name, count)
		}
	}
}

// TestExchangeKeyIgnoresOrderEncodingAndEmptyFields proves the key of a
// request as the browser sends it equals the key of the request as the
// recorder asked it, so the player finds the recorded answer.
func TestExchangeKeyIgnoresOrderEncodingAndEmptyFields(t *testing.T) {
	key := func(method, target string, form []Field, htmx bool) string {
		t.Helper()
		path, query, err := splitTarget(target)
		if err != nil {
			t.Fatal(err)
		}
		return ExchangeKey(method, path, query, form, htmx)
	}
	// The markup writes the prefix raw; theme.js sends it escaped, mode first.
	markup := key("GET", "/lg/graph?prefix=198.18.0.0/24&mode=aspath", nil, true)
	script := key("get", "/lg/graph?mode=aspath&prefix=198.18.0.0%2F24", nil, true)
	if markup != script {
		t.Fatalf("one request, two keys: %q and %q", markup, script)
	}
	if want := "GET /lg/graph?mode=aspath&prefix=198.18.0.0/24 htmx"; markup != want {
		t.Fatalf("key: got %q, want %q", markup, want)
	}
	// The tour posts one field; a browser posts every field of the form.
	asked := key("POST", "/lg/search", []Field{{Name: "prefix", Value: "198.18.0.0/24"}}, true)
	sent := key("POST", "/lg/search", []Field{
		{Name: "prefix", Value: "198.18.0.0/24"}, {Name: "aspath", Value: ""},
		{Name: "community", Value: ""}, {Name: "family", Value: ""},
	}, true)
	if asked != sent {
		t.Fatalf("one search, two keys: %q and %q", asked, sent)
	}
	if want := "POST /lg/search form prefix=198.18.0.0/24 htmx"; asked != want {
		t.Fatalf("key: got %q, want %q", asked, want)
	}
	// A plain request gets the whole page and an htmx one a fragment.
	if plain := key("GET", "/lg/peers", nil, false); plain == key("GET", "/lg/peers", nil, true) {
		t.Fatalf("the page and its fragment share the key %q", plain)
	}
	if _, _, err := splitTarget("lg/peers"); err == nil {
		t.Fatal("a target with no leading slash was accepted")
	}
}

// TestFollowUpRequestsOfAnAnswer proves every hx-get of an answer is found
// once, in order, with its character references decoded.
func TestFollowUpRequestsOfAnAnswer(t *testing.T) {
	markup := `<tr hx-get="/lg/peer/203.0.113.1"></tr>` +
		`<tr hx-get="/lg/route/detail?prefix=198.18.0.0/24&amp;peer=203.0.113.1"></tr>` +
		`<tr hx-get="/lg/peer/203.0.113.1"></tr><a href="/lg/peer/203.0.113.1/download">csv</a>`
	want := []string{"/lg/peer/203.0.113.1", "/lg/route/detail?prefix=198.18.0.0/24&peer=203.0.113.1"}
	if got := attributeValues(followUpAttribute, markup); !reflect.DeepEqual(got, want) {
		t.Fatalf("follow-up requests: got %q, want %q", got, want)
	}
}

// sampleRecording is a small recording with every part filled.
func sampleRecording() Recording {
	return Recording{
		Schema: Schema,
		Provenance: Provenance{
			Recorder: RecorderCommand, Commit: "0123456789abcdef", CommitDate: "2026-11-18T20:00:00Z",
			Seed: 7, Speakers: 11,
			Normalized: []Normalization{{Name: "uptime", What: "uptime", Replacement: fixedUptime, Count: 11}},
			Varies:     []string{},
		},
		Entry: "GET /lg/peers",
		Tour:  []TourStep{{Caption: "The peers page.", Exchange: "GET /lg/peers"}},
		Exchanges: []Exchange{{
			Key: "GET /lg/peers", Method: "GET", Path: "/lg/peers", Via: viaTour, Status: 200,
			ContentType: "text/html; charset=utf-8", Body: "<html><script>var a = 1 < 2;</script></html>",
		}},
		Assets: []Asset{{Path: "/lg/assets/style.css", ContentType: "text/css", Body: "body{}"}},
		Stream: &Stream{Path: "/lg/events", ContentType: "text/event-stream", Messages: []string{"data: <tr></tr>"}},
	}
}

// TestRecordingBytesAreStable proves one recording always encodes to the same
// bytes, that those bytes hold no raw angle bracket (they go inside a script
// element), and that the file reads back equal.
func TestRecordingBytesAreStable(t *testing.T) {
	recording := sampleRecording()
	first, err := recording.Encode()
	if err != nil {
		t.Fatal(err)
	}
	second, err := recording.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("one recording encoded to two different files")
	}
	if strings.ContainsAny(string(first), "<>") {
		t.Fatalf("the file holds a raw angle bracket:\n%s", first)
	}

	path := filepath.Join(t.TempDir(), "lg-replay.json")
	if unchanged, err := writeRecording(path, recording); err != nil || unchanged {
		t.Fatalf("first write: unchanged=%v err=%v", unchanged, err)
	}
	if unchanged, err := writeRecording(path, recording); err != nil || !unchanged {
		t.Fatalf("second write of the same recording: unchanged=%v err=%v", unchanged, err)
	}
	read, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, recording) {
		t.Fatalf("read back:\n got %+v\nwant %+v", read, recording)
	}

	if _, err := Read(filepath.Join(t.TempDir(), "absent.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a missing file: got %v, want fs.ErrNotExist", err)
	}
	recording.Schema = Schema + 1
	later, err := recording.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, later, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("a recording of another schema was read")
	}
}
