package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

// lgReplaySample is a small recording: a page, a fragment whose text would
// end a script element, and a text answer.
func lgReplaySample() sitelgreplay.Recording {
	return sitelgreplay.Recording{
		Schema: sitelgreplay.Schema,
		Provenance: sitelgreplay.Provenance{
			Recorder: "./le site lg-replay record", Commit: "0123456789abcdef0123", CommitDate: "2026-11-18T20:00:00Z",
			Seed: 7, Speakers: 11,
			Normalized: []sitelgreplay.Normalization{{Name: "uptime", What: "uptime", Replacement: "1m 0s", Count: 11}},
			Varies:     []string{},
		},
		Entry: "GET /lg/peers",
		Tour: []sitelgreplay.TourStep{
			{Caption: "The peers page.", Exchange: "GET /lg/peers"},
			{Caption: "Search <by> prefix.", Exchange: "POST /lg/search form prefix=198.18.0.0/24 htmx"},
			{Caption: "The graph as text.", Exchange: "GET /lg/graph?format=text&prefix=198.18.0.0/24"},
		},
		Exchanges: []sitelgreplay.Exchange{
			{
				Key: "GET /lg/peers", Method: "GET", Path: "/lg/peers", Via: "tour", Status: 200,
				ContentType: "text/html; charset=utf-8",
				Body:        "<html><head><title>Peers</title></head><body><main><p>eleven peers</p></main></body></html>",
			},
			{
				Key: "POST /lg/search form prefix=198.18.0.0/24 htmx", Method: "POST", Path: "/lg/search",
				Form: []sitelgreplay.Field{{Name: "prefix", Value: "198.18.0.0/24"}}, HTMX: true, Via: "tour",
				Status: 200, ContentType: "text/html; charset=utf-8",
				Body: "<p>seven routes</p><script>var done;</script>",
			},
			{
				Key: "GET /lg/graph?format=text&prefix=198.18.0.0/24", Method: "GET", Path: "/lg/graph",
				Query: []sitelgreplay.Field{{Name: "prefix", Value: "198.18.0.0/24"}, {Name: "format", Value: "text"}},
				Via:   "tour", Status: 200, ContentType: "text/plain; charset=utf-8", Body: "AS65000 -> AS64501\n",
			},
		},
		Assets: []sitelgreplay.Asset{{Path: "/lg/assets/style.css", ContentType: "text/css", Body: "body{}"}},
	}
}

// lgReplayCheckout answers the paths of a checkout that holds the player
// script and, when recorded is set, the sample recording.
func lgReplayCheckout(t *testing.T, recorded bool) Paths {
	t.Helper()
	repository := t.TempDir()
	paths := Paths{Repository: repository, Source: filepath.Join(repository, "website"), Output: t.TempDir()}
	write := func(name string, content []byte) {
		t.Helper()
		path := filepath.Join(repository, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("website/"+lgReplayScriptSource, []byte("/* the player */\n"))
	// The two files every asset render requires of a checkout.
	write("website/assets/js/site.js", []byte("/* the site */\n"))
	write("website/assets/css/site.css", []byte(".lg-replay__frame { width: 100%; }\n"))
	if recorded {
		content, err := lgReplaySample().Encode()
		if err != nil {
			t.Fatal(err)
		}
		write(sitelgreplay.File, content)
	}
	return paths
}

// TestLGReplayFrameCarriesTheRecordingAndAllowsNoConnection proves the frame
// document holds the recording whole, the player, and a policy under which the
// browser refuses every connection, and that recorded markup cannot end the
// element that carries it.
func TestLGReplayFrameCarriesTheRecordingAndAllowsNoConnection(t *testing.T) {
	recording := lgReplaySample()
	document, err := lgReplayFrameDocument(recording, "/* the player */")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(document, `http-equiv="Content-Security-Policy" content="default-src 'none';`) {
		t.Fatalf("the frame document states no policy that refuses by default:\n%s", document)
	}
	for _, allowed := range []string{"connect-src", "'self'", "http:", "https:"} {
		if strings.Contains(lgReplayPolicy, allowed) {
			t.Errorf("the policy allows %s, so a request could leave the frame", allowed)
		}
	}
	// Two script elements, so two closers: the recorded "</script>" is escaped.
	if got := strings.Count(document, "</script>"); got != 2 {
		t.Fatalf("the frame document closes %d script elements, want 2", got)
	}
	opening := `<script type="application/json" id="lg-replay-recording">`
	_, afterOpening, opened := strings.Cut(document, opening)
	if !opened {
		t.Fatal("the frame document carries no recording element")
	}
	embedded, _, closed := strings.Cut(afterOpening, "</script>")
	if !closed {
		t.Fatal("the frame document does not close its recording element")
	}
	var read sitelgreplay.Recording
	if err := json.Unmarshal([]byte(embedded), &read); err != nil {
		t.Fatalf("the embedded recording does not parse: %v", err)
	}
	if !reflect.DeepEqual(read, recording) {
		t.Fatalf("the embedded recording differs:\n got %+v\nwant %+v", read, recording)
	}
	if !strings.Contains(document, "<script>/* the player */</script>") {
		t.Fatal("the frame document does not carry the player inline")
	}
	if _, err := lgReplayFrameDocument(recording, "var s = '</SCRIPT>';"); err == nil {
		t.Fatal("a player holding </script was accepted")
	}
}

// TestLGReplayBlockListsTheTourAndTheTranscript proves the block a page
// carries holds the mount, every step of the tour, the provenance and the
// transcript link, and that the producer writes the frame and the transcript.
func TestLGReplayBlockListsTheTourAndTheTranscript(t *testing.T) {
	paths := lgReplayCheckout(t, true)
	block, err := lgReplayBlockHTML(paths, lgReplayPage)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-lg-replay data-lg-replay-frame="replay/frame.html"`,
		`<a href="replay/transcript.txt">Read the transcript of the tour</a>`,
		`<li data-lg-replay-step="0"><span data-lg-replay-caption>The peers page.</span> <code>GET /lg/peers</code></li>`,
		`<span data-lg-replay-caption>Search &lt;by&gt; prefix.</span>`,
		`<code>GET /lg/graph?prefix=198.18.0.0/24&amp;format=text</code>`,
		"from commit 0123456789 (2026-11-18)",
		"11 simulated BGP speakers generated from seed 7, 3 recorded answers.",
		`<script src="../../assets/lg-replay.js" defer></script>`,
	} {
		if !strings.Contains(block, want) {
			t.Errorf("the block lacks %q:\n%s", want, block)
		}
	}

	if routes, err := renderLGReplay(paths); err != nil || len(routes) != 0 {
		t.Fatalf("producer: routes=%v err=%v, want no route and no error", routes, err)
	}
	directory := filepath.Join(paths.Output, "labs", "looking-glass-graph", "replay")
	if _, err := os.Stat(filepath.Join(directory, lgReplayFrame)); err != nil {
		t.Fatalf("the frame document was not written: %v", err)
	}
	// A run of this producer alone publishes what its page loads.
	for _, asset := range []string{lgReplayScript, "assets/site.css"} {
		info, err := os.Stat(filepath.Join(paths.Output, filepath.FromSlash(asset)))
		if err != nil {
			t.Fatalf("the producer did not publish %s: %v", asset, err)
		}
		if info.Size() == 0 {
			t.Fatalf("the producer published an empty %s", asset)
		}
	}
	transcript, err := os.ReadFile(filepath.Join(directory, lgReplayTranscript))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"1. The peers page.\n   GET /lg/peers\n   200 text/html; charset=utf-8",
		"eleven peers",
		"2. Search <by> prefix.\n   POST /lg/search prefix=198.18.0.0/24 (asked by the page, answered with a fragment)",
		"seven routes",
		"3. The graph as text.",
		"AS65000 -> AS64501",
	} {
		if !strings.Contains(string(transcript), want) {
			t.Errorf("the transcript lacks %q:\n%s", want, transcript)
		}
	}
	if strings.Contains(string(transcript), "var done") {
		t.Errorf("the transcript carries a script's text:\n%s", transcript)
	}
}

// TestLGReplayBlockWithoutARecording proves a checkout that never recorded
// publishes a block that says so, and that the producer writes nothing.
func TestLGReplayBlockWithoutARecording(t *testing.T) {
	paths := lgReplayCheckout(t, false)
	block, err := lgReplayBlockHTML(paths, lgReplayPage)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block, lgReplayUnrecorded) {
		t.Fatalf("the block does not say the looking glass is unrecorded:\n%s", block)
	}
	if strings.Contains(block, "data-lg-replay") {
		t.Fatalf("the block mounts a player with nothing to play:\n%s", block)
	}
	if _, err := renderLGReplay(paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.Output, "labs")); !os.IsNotExist(err) {
		t.Fatalf("the producer wrote under labs/ with no recording: %v", err)
	}
}

// lgReplayWriteRecording writes recording as the checkout's recording.
func lgReplayWriteRecording(t *testing.T, paths Paths, recording sitelgreplay.Recording) {
	t.Helper()
	content, err := recording.Encode()
	if err != nil {
		t.Fatal(err)
	}
	recorded := filepath.Join(paths.Repository, filepath.FromSlash(sitelgreplay.File))
	if err := os.MkdirAll(filepath.Dir(recorded), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recorded, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestLGReplayRendersATrialRecordingWithItsNotice proves the build renders a
// recording whose provenance says the checkout held uncommitted changes, and
// refuses nothing: building the site renders data and never runs a check
// (ai/META-DECISIONS.md, "Website design"). The block, the transcript and the
// frame document each say it is a trial that is not for publication, and the
// block states the mark ./le site check refuses. The refusal this test
// asserted of the build until 2026-11-19 is asserted of the check, in
// TestSiteCheckRefusesATrialRecording.
func TestLGReplayRendersATrialRecordingWithItsNotice(t *testing.T) {
	paths := lgReplayCheckout(t, false)
	trial := lgReplaySample()
	trial.Provenance.TreeDirty = true
	lgReplayWriteRecording(t, paths, trial)

	block, err := lgReplayBlockHTML(paths, lgReplayPage)
	if err != nil {
		t.Fatalf("the block refused a trial recording, and a build renders one: %v", err)
	}
	said := []string{
		"Trial recording, not for publication.",
		"made from a checkout with uncommitted changes, so commit 0123456789 alone did not produce these answers",
		"./le site check refuses a site that carries it",
		"record again with " + sitelgreplay.RecorderCommand,
	}
	for _, want := range append([]string{
		`<div class="lg-replay" data-lg-replay-commit="0123456789abcdef0123" data-lg-replay-trial>`,
		`<div class="section-note" role="note">`,
		`<strong>Trial recording, not for publication.</strong>`,
		// The replay itself is rendered whole.
		`data-lg-replay data-lg-replay-frame="replay/frame.html"`,
		`<li data-lg-replay-step="0">`,
		"from commit 0123456789 (2026-11-18)",
	}, said[1:]...) {
		if !strings.Contains(block, want) {
			t.Errorf("the block of a trial recording lacks %q:\n%s", want, block)
		}
	}

	if routes, err := renderLGReplay(paths); err != nil || len(routes) != 0 {
		t.Fatalf("producer: routes=%v err=%v, want a trial recording rendered with no route and no error", routes, err)
	}
	directory := filepath.Join(paths.Output, "labs", "looking-glass-graph", "replay")
	transcript, err := os.ReadFile(filepath.Join(directory, lgReplayTranscript))
	if err != nil {
		t.Fatal(err)
	}
	header, _, _ := strings.Cut(string(transcript), "\n1. ")
	frame, err := os.ReadFile(filepath.Join(directory, lgReplayFrame))
	if err != nil {
		t.Fatal(err)
	}
	_, notice, found := strings.Cut(string(frame), `<p id="lg-replay-notice">`)
	if !found {
		t.Fatalf("the frame document carries no notice paragraph:\n%s", frame)
	}
	notice, _, _ = strings.Cut(notice, "</p>")
	for name, text := range map[string]string{"the transcript's header": header, "the frame document's notice": notice} {
		for _, want := range said {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q:\n%s", name, want, text)
			}
		}
	}
}

// TestLGReplayOfACommittedTreeCarriesNoTrialNotice proves a recording made
// from a committed tree states its commit for the check, and that neither the
// block, the transcript nor the frame document calls it a trial.
func TestLGReplayOfACommittedTreeCarriesNoTrialNotice(t *testing.T) {
	paths := lgReplayCheckout(t, true)
	block, err := lgReplayBlockHTML(paths, lgReplayPage)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(block, `<div class="lg-replay" data-lg-replay-commit="0123456789abcdef0123">`) {
		t.Errorf("the block does not state its recording's commit and nothing else on its root:\n%s", block)
	}
	recording := lgReplaySample()
	transcript, err := lgReplayTranscriptText(recording)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := lgReplayFrameDocument(recording, "/* the player */")
	if err != nil {
		t.Fatal(err)
	}
	// The frame document is judged without the recording it embeds.
	frame, _, _ = strings.Cut(frame, `<script type="application/json"`)
	for name, text := range map[string]string{"the block": block, "the transcript": transcript, "the frame document": frame} {
		for _, unwanted := range []string{"Trial recording", "data-lg-replay-trial", "section-note"} {
			if strings.Contains(text, unwanted) {
				t.Errorf("%s of a committed recording carries %q:\n%s", name, unwanted, text)
			}
		}
	}
}

// TestLGReplayProvenanceSaysTheNetworksAreNotReal proves the paragraph the
// page and the transcript share tells a reader the prefixes, addresses and AS
// numbers shown belong to no real network, and names the benchmarking range
// the prefixes come from.
func TestLGReplayProvenanceSaysTheNetworksAreNotReal(t *testing.T) {
	recording := lgReplaySample()
	transcript, err := lgReplayTranscriptText(recording)
	if err != nil {
		t.Fatal(err)
	}
	header, _, _ := strings.Cut(transcript, "\n1. ")
	for _, text := range []string{lgReplayProvenance(recording), header} {
		for _, want := range []string{"None of the networks shown is a real one", "198.18.0.0/15", "RFC 2544", "RFC 5737"} {
			if !strings.Contains(text, want) {
				t.Errorf("the provenance lacks %q:\n%s", want, text)
			}
		}
	}
}

// TestAuthoredPageExpandsARegisteredBlock proves an authored page's block
// comment is replaced by the registered block, with the page around it kept,
// and that a comment naming no block is refused by name.
func TestAuthoredPageExpandsARegisteredBlock(t *testing.T) {
	paths := lgReplayCheckout(t, false)
	expanded, err := expandAuthoredBlocks(paths, lgReplayPage, "<p>before</p>\n<!-- site-block: lg-replay -->\n<p>after</p>")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(expanded, "<p>before</p>\n<div class=\"lg-replay\">") {
		t.Fatalf("the block did not replace its comment:\n%s", expanded)
	}
	if !strings.HasSuffix(expanded, "</div>\n<p>after</p>") {
		t.Fatalf("the page after the block was lost:\n%s", expanded)
	}
	_, err = expandAuthoredBlocks(paths, lgReplayPage, "<!-- site-block: no-such-block -->")
	if err == nil {
		t.Fatal("a comment naming an unregistered block was published")
	}
	if !strings.Contains(err.Error(), "no-such-block") {
		t.Fatalf("the refusal does not name the block: %v", err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "website", filepath.FromSlash(lgReplayPage)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "<!-- site-block: "+lgReplayBlock+" -->") {
		t.Fatalf("website/%s no longer names the %s block", lgReplayPage, lgReplayBlock)
	}
}

// TestReplayFrameDocumentTakesThePlayerSettings proves the frame document of a
// recording with player settings carries that recording's title, subject and
// host page, escaped, under the same policy, and that a recording without them
// gets the looking glass's.
func TestReplayFrameDocumentTakesThePlayerSettings(t *testing.T) {
	looking, err := ReplayFrameDocument(lgReplaySample(), "/* the player */")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<title>Ze looking glass replay</title>",
		`This document replays a recorded Ze looking glass, and only inside the frame of <a href="../">its page</a>, with JavaScript.`,
	} {
		if !strings.Contains(string(looking), want) {
			t.Errorf("the looking glass frame lacks %q", want)
		}
	}

	recording := lgReplaySample()
	recording.Player = &sitelgreplay.Player{Title: "Fleet <recorded>", Subject: "Ze fleet & co", HostPage: "./"}
	fleet, err := ReplayFrameDocument(recording, "/* the player */")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<title>Fleet &lt;recorded&gt;</title>",
		`This document replays a recorded Ze fleet &amp; co, and only inside the frame of <a href="./">its page</a>, with JavaScript.`,
		`content="` + lgReplayPolicy + `"`,
	} {
		if !strings.Contains(string(fleet), want) {
			t.Errorf("the frame of a recording with player settings lacks %q:\n%s", want, fleet)
		}
	}
}
