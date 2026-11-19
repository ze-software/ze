package sitelgreplay

import (
	"bytes"
	"encoding/json"
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

// chunkedRecording is sampleRecording with every optional field filled: the
// player's settings, two chunks and two fallbacks whose prefixes overlap, so
// their order decides which one answers.
func chunkedRecording() Recording {
	recording := sampleRecording()
	recording.Player = &Player{
		Title: "Ze fleet interface, recorded", Subject: "Ze fleet interface", HostPage: "./",
		AssetPrefixes: []string{"/fleet/assets/", "/assets/"},
		NoticeText:    "This is not part of the recorded demonstration: " + NoticeRequestToken + ".",
		NoticeClass:   "fleet-toast", SilentEventSource: true, KeepPushedAddress: true, PlainForms: true,
	}
	recording.Chunks = []Chunk{
		{Name: "estate", File: "estate.json", Size: 120, Keys: []string{"GET /fleet/estate htmx"}},
		{Name: "device-r1", File: "device-r1.json", Size: 80, Keys: []string{"GET /fleet/device/r1", "GET /fleet/device/r1 htmx"}},
	}
	toast := Exchange{
		Key: "POST /fleet/action/", Method: "POST", Path: "/fleet/action/", HTMX: true, Via: viaTour, Status: 200,
		ContentType: "text/html; charset=utf-8", Body: "<div>Recorded demonstration: this action was not sent</div>",
	}
	recording.Fallbacks = []Fallback{
		{Method: "POST", PathPrefix: "/fleet/action/reboot", Exchange: toast},
		{Method: "POST", PathPrefix: "/fleet/action/", Exchange: toast},
	}
	return recording
}

// writeAndRead encodes recording to a file and answers what Read makes of it.
func writeAndRead(t *testing.T, recording Recording) (Recording, []byte, error) {
	t.Helper()
	content, err := recording.Encode()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "recording.json")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := Read(path)
	return read, content, err
}

// TestRecordingOptionalFieldsRoundTrip proves the fields a second product
// needs are optional: a recording without them encodes none of their keys and
// reads back equal, one with every one of them reads back equal, and the
// committed looking glass recording re-encodes to its own bytes.
//
// Method: encode, read back through Read, compare whole values; then read the
// committed file and compare its re-encoding byte for byte.
func TestRecordingOptionalFieldsRoundTrip(t *testing.T) {
	plain := sampleRecording()
	read, content, err := writeAndRead(t, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, plain) {
		t.Fatalf("a recording without the optional fields read back:\n got %+v\nwant %+v", read, plain)
	}
	for _, key := range []string{`"player"`, `"chunks"`, `"fallbacks"`} {
		if bytes.Contains(content, []byte(key)) {
			t.Errorf("a recording without the optional fields encodes the key %s:\n%s", key, content)
		}
	}

	chunked := chunkedRecording()
	read, content, err = writeAndRead(t, chunked)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, chunked) {
		t.Fatalf("a recording with the optional fields read back:\n got %+v\nwant %+v", read, chunked)
	}
	for _, key := range []string{`"asset-prefixes"`, `"notice-text"`, `"silent-event-source"`, `"path-prefix"`, `"keys"`} {
		if !bytes.Contains(content, []byte(key)) {
			t.Errorf("the encoded recording has no key %s:\n%s", key, content)
		}
	}

	committed := filepath.Join("..", "..", "..", "..", filepath.FromSlash(File))
	original, err := os.ReadFile(committed) //nolint:gosec // the committed recording of this checkout
	if err != nil {
		t.Fatal(err)
	}
	recording, err := Read(committed)
	if err != nil {
		t.Fatal(err)
	}
	if recording.Player != nil || recording.Chunks != nil || recording.Fallbacks != nil {
		t.Fatalf("the looking glass recording holds a field of another product: player %v, %d chunks, %d fallbacks",
			recording.Player, len(recording.Chunks), len(recording.Fallbacks))
	}
	again, err := recording.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, original) {
		t.Fatalf("%s re-encodes to other bytes: %d bytes read, %d written", File, len(original), len(again))
	}
}

// TestFallbacksKeepTheirOrder proves the fallbacks read back in the order they
// were written: the player answers with the first that matches, so a narrower
// prefix written first must stay first.
func TestFallbacksKeepTheirOrder(t *testing.T) {
	read, _, err := writeAndRead(t, chunkedRecording())
	if err != nil {
		t.Fatal(err)
	}
	var prefixes []string
	for _, fallback := range read.Fallbacks {
		prefixes = append(prefixes, fallback.PathPrefix)
	}
	want := []string{"/fleet/action/reboot", "/fleet/action/"}
	if !reflect.DeepEqual(prefixes, want) {
		t.Fatalf("fallback order: got %q, want %q", prefixes, want)
	}
}

// TestSplitChunksGroupsTheExchanges proves a split keeps an ungrouped
// exchange in the frame, gives each chunk its exchanges in their order, names
// its file after it, states the file's size and keys, and that the chunk file
// reads back as exactly those exchanges.
func TestSplitChunksGroupsTheExchanges(t *testing.T) {
	exchange := func(path string) Exchange {
		return Exchange{
			Key: "GET " + path, Method: "GET", Path: path, Via: viaLink, Status: 200,
			ContentType: "text/html; charset=utf-8", Body: "<p>" + path + "</p>",
		}
	}
	exchanges := []Exchange{exchange("/fleet/"), exchange("/fleet/device/r1"), exchange("/fleet/device/r2"), exchange("/fleet/device/r1/tab")}
	groups := map[string]string{"/fleet/device/r1": "device-r1", "/fleet/device/r1/tab": "device-r1", "/fleet/device/r2": "device-r2"}
	split, err := SplitChunks(exchanges, func(exchange Exchange) string { return groups[exchange.Path] })
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(split.Inline, exchanges[:1]) {
		t.Fatalf("inline exchanges: got %+v, want the entry page alone", split.Inline)
	}
	var names []string
	for _, chunk := range split.Chunks {
		names = append(names, chunk.Name+"="+chunk.File+":"+strings.Join(chunk.Keys, ","))
		if chunk.Size != len(split.Contents[chunk.Name]) {
			t.Errorf("chunk %s states %d bytes, and its file has %d", chunk.Name, chunk.Size, len(split.Contents[chunk.Name]))
		}
	}
	want := []string{
		"device-r1=device-r1.json:GET /fleet/device/r1,GET /fleet/device/r1/tab",
		"device-r2=device-r2.json:GET /fleet/device/r2",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("chunks: got %q, want %q", names, want)
	}
	var held []Exchange
	if err := json.Unmarshal(split.Contents["device-r1"], &held); err != nil {
		t.Fatalf("the chunk file does not parse as a list of exchanges: %v", err)
	}
	if !reflect.DeepEqual(held, []Exchange{exchanges[1], exchanges[3]}) {
		t.Fatalf("chunk device-r1 holds %+v", held)
	}

	recording := sampleRecording()
	recording.Exchanges = split.Inline
	recording.Chunks = split.Chunks
	if _, _, err := writeAndRead(t, recording); err != nil {
		t.Fatalf("a recording of the split is refused: %v", err)
	}
	files, err := ChunkFiles(split.Chunks)
	if err != nil {
		t.Fatal(err)
	}
	if files != `{"device-r1":"device-r1.json","device-r2":"device-r2.json"}` {
		t.Fatalf("the host's chunk files: %s", files)
	}
}

// TestSplitChunksRefusesWhatTheHostCouldNotServe proves a chunk name that is
// not a plain file name, and a key two exchanges share, are refused.
func TestSplitChunksRefusesWhatTheHostCouldNotServe(t *testing.T) {
	page := Exchange{Key: "GET /fleet/", Method: "GET", Path: "/fleet/", Status: 200, ContentType: "text/html"}
	for name, tc := range map[string]struct {
		exchanges []Exchange
		group     string
	}{
		"a name that leaves the directory": {exchanges: []Exchange{page}, group: "../estate"},
		"a name with a capital":            {exchanges: []Exchange{page}, group: "Estate"},
		"a key twice":                      {exchanges: []Exchange{page, page}, group: "estate"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := SplitChunks(tc.exchanges, func(Exchange) string { return tc.group }); err == nil {
				t.Fatal("the split was accepted")
			}
		})
	}
}

// TestReadRefusesChunksAndFallbacksThatContradict proves Read refuses a
// recording whose chunks or fallbacks the player could not follow honestly.
func TestReadRefusesChunksAndFallbacksThatContradict(t *testing.T) {
	for name, change := range map[string]func(*Recording){
		"a key in the frame and in a chunk": func(r *Recording) { r.Chunks[0].Keys = append(r.Chunks[0].Keys, "GET /lg/peers") },
		"a key in two chunks":               func(r *Recording) { r.Chunks[1].Keys = append(r.Chunks[1].Keys, "GET /fleet/estate htmx") },
		"two chunks of one name":            func(r *Recording) { r.Chunks[1].Name = "estate"; r.Chunks[1].File = "estate.json" },
		"a file that is not the chunk's":    func(r *Recording) { r.Chunks[0].File = "../frame.html" },
		"a chunk name that is no file name": func(r *Recording) { r.Chunks[0].Name = "a/b"; r.Chunks[0].File = "a/b.json" },
		"a fallback with no method":         func(r *Recording) { r.Fallbacks[0].Method = "" },
		"a fallback prefix that is no path": func(r *Recording) { r.Fallbacks[0].PathPrefix = "fleet/action/" },
	} {
		t.Run(name, func(t *testing.T) {
			recording := chunkedRecording()
			change(&recording)
			if _, _, err := writeAndRead(t, recording); err == nil {
				t.Fatal("Read accepted the recording")
			}
		})
	}
}
