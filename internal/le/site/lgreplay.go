// Design: docs/contributing/gh-pages.md -- the looking glass replay: what the site publishes from the recording
// Related: authored.go -- registerAuthoredBlock, through which the labs page names the replay
// Related: lgreplay/recording.go -- the Recording this renders

package site

import (
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

const (
	// lgReplayBlock is the block an authored page names to carry the replay.
	lgReplayBlock = "lg-replay"
	// lgReplayDirectory holds the replay's files, relative to the page that
	// carries the block.
	lgReplayDirectory = "replay"
	// lgReplayFrame is the document the page's frame loads. It is no route:
	// only index.html makes one (pageRegistry).
	lgReplayFrame      = "frame.html"
	lgReplayTranscript = "transcript.txt"
	// lgReplayPage is the page the replay belongs to.
	lgReplayPage = "labs/looking-glass-graph/" + pageIndexFile
	// lgReplayScriptSource is the player, one file with no build step. The
	// site publishes it at assets/lg-replay.js for the page, and the frame
	// document carries a copy inline.
	lgReplayScriptSource = "assets/js/lg-replay.js"
	lgReplayScript       = "assets/lg-replay.js"

	// lgReplayPolicy is the frame document's content security policy. Nothing
	// is allowed but the inline player, the looking glass's inline stylesheet
	// and scripts, and images carried as data: no connection, so the browser
	// itself refuses a request the player did not answer.
	lgReplayPolicy = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:"

	lgReplayUnrecorded = "Replay not yet available. The looking glass has not been recorded for the published site yet."

	// lgReplaySimulated tells a reader of the page and of the transcript that
	// no address or AS number shown belongs to a real network. The prefixes
	// are the simulated network's (topology_generate.go in
	// internal/chaos/scenario): 198.18.0.0/15 is reserved for benchmarking and
	// is routed nowhere on the Internet.
	lgReplaySimulated = "None of the networks shown is a real one: the prefixes come from 198.18.0.0/15, " +
		"the range RFC 2544 reserves for benchmarking, the speakers use documentation addresses (RFC 5737), " +
		"and the AS numbers are private or documentation numbers (RFC 6996, RFC 5398)."

	// lgReplayCommitShown is how many characters of the commit a reader sees.
	lgReplayCommitShown = 10
)

func init() {
	registerProducer(Producer{Name: lgReplayBlock, Render: renderLGReplay})
	registerAuthoredBlock(lgReplayBlock, lgReplayBlockHTML)
}

// readLGReplay answers the checkout's recording. recorded is false when the
// checkout holds none, which is a state the site publishes, not a failure: a
// site build renders data and never records.
func readLGReplay(paths Paths) (recording sitelgreplay.Recording, recorded bool, err error) {
	recording, err = sitelgreplay.Read(filepath.Join(paths.Repository, filepath.FromSlash(sitelgreplay.File)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return sitelgreplay.Recording{}, false, nil
		}
		return sitelgreplay.Recording{}, false, err
	}
	// The page says which commit answered. A recording of a tree with
	// uncommitted changes names a commit that did not, so it is a trial of the
	// recorder and never a thing to publish.
	if recording.Provenance.TreeDirty {
		commit := recording.Provenance.Commit
		if len(commit) > lgReplayCommitShown {
			commit = commit[:lgReplayCommitShown]
		}
		return sitelgreplay.Recording{}, false, opererr.New(
			sitelgreplay.File+" was recorded from a checkout with uncommitted changes, so the site does not publish it",
			"The replay's page would name commit "+commit+" as the code that answered, and that commit did not produce these answers. "+
				"Commit the changes, then record again from the clean checkout",
			sitelgreplay.RecorderCommand)
	}
	return recording, true, nil
}

// renderLGReplay writes the frame document and the transcript beside the page
// that carries the replay. It answers no route: neither file is a page.
func renderLGReplay(paths Paths) ([]string, error) {
	recording, recorded, err := readLGReplay(paths)
	if err != nil {
		return nil, err
	}
	if !recorded {
		return nil, nil
	}
	script, err := os.ReadFile(filepath.Join(paths.Source, filepath.FromSlash(lgReplayScriptSource))) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return nil, err
	}
	frame, err := lgReplayFrameDocument(recording, string(script))
	if err != nil {
		return nil, err
	}
	transcript, err := lgReplayTranscriptText(recording)
	if err != nil {
		return nil, err
	}
	directory := filepath.ToSlash(filepath.Join(filepath.Dir(lgReplayPage), lgReplayDirectory))
	if err := writeNamedArtifact(paths.Output, directory+"/"+lgReplayFrame, frame); err != nil {
		return nil, err
	}
	if err := writeNamedArtifact(paths.Output, directory+"/"+lgReplayTranscript, transcript); err != nil {
		return nil, err
	}
	// The page that carries the replay loads the player and the replay's
	// styles from the shared assets. A producer run renders no asset, so a
	// run of this producer would publish a frame its page cannot mount. The
	// same function a full build calls writes them, so the bytes are its own.
	if err := RenderAssets(paths); err != nil {
		return nil, err
	}
	return nil, nil
}

// lgReplayFrameDocument answers the document the replay's frame loads: the
// policy, the recording, and the player, in one file that fetches nothing.
//
// The document is a file of the site, so a browser can open it with no frame
// around it, and the recorded scripts would then run in the site's own origin.
// The sandbox is an attribute of the page's iframe and a policy in a meta
// element cannot state one, so the player decides: it puts no recorded markup
// or script in place unless the document is framed and its origin is the
// opaque one a sandbox gives (frameRefusal in lg-replay.js). The paragraph
// below is what a reader sees then.
func lgReplayFrameDocument(recording sitelgreplay.Recording, script string) (string, error) {
	// The recording's own encoding escapes <, > and &, so it cannot end the
	// element that holds it. The player is source text, so it is checked.
	if strings.Contains(strings.ToLower(script), "</script") {
		return "", fmt.Errorf("%s holds the text </script, which would end the frame document's script element early", lgReplayScriptSource)
	}
	encoded, err := recording.Encode()
	if err != nil {
		return "", err
	}
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="` + lgReplayPolicy + `">
<meta name="robots" content="noindex">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Ze looking glass replay</title>
</head>
<body>
<p id="lg-replay-notice">This document replays a recorded Ze looking glass, and only inside the frame of <a href="../">its page</a>, with JavaScript.</p>
<script type="application/json" id="lg-replay-recording">` + strings.TrimSpace(string(encoded)) + `</script>
<script>` + script + `</script>
</body>
</html>
`, nil
}

// lgReplayTranscriptText answers the tour as text: each step's caption, its
// request, and the answer the looking glass gave.
func lgReplayTranscriptText(recording sitelgreplay.Recording) (string, error) {
	var out strings.Builder
	out.WriteString("Ze looking glass replay: transcript of the tour\n\n")
	out.WriteString(lgReplayProvenance(recording) + "\n")
	for index, step := range recording.Tour {
		exchange, found := recording.Exchange(step.Exchange)
		if !found {
			return "", fmt.Errorf("the looking glass recording's tour step %d names %s, which the recording does not hold", index+1, step.Exchange)
		}
		answer, err := lgReplayAnswerText(exchange)
		if err != nil {
			return "", fmt.Errorf("tour step %d: %w", index+1, err)
		}
		out.WriteString("\n" + strconv.Itoa(index+1) + ". " + step.Caption + "\n")
		out.WriteString("   " + lgReplayRequestText(exchange) + "\n")
		out.WriteString("   " + strconv.Itoa(exchange.Status) + " " + exchange.ContentType + "\n\n")
		out.WriteString(strings.TrimSpace(answer) + "\n")
	}
	return out.String(), nil
}

// lgReplayRequestText answers one request as a reader types it.
func lgReplayRequestText(exchange sitelgreplay.Exchange) string {
	var text strings.Builder
	text.WriteString(exchange.Method + " " + exchange.Path)
	separator := "?"
	for _, field := range exchange.Query {
		text.WriteString(separator + field.Name + "=" + field.Value)
		separator = "&"
	}
	for _, field := range exchange.Form {
		text.WriteString(" " + field.Name + "=" + field.Value)
	}
	if exchange.HTMX {
		text.WriteString(" (asked by the page, answered with a fragment)")
	}
	return text.String()
}

// lgReplayAnswerText answers one recorded answer as text: markup through the
// converter every page mirror uses, text as it is, and a drawing by its size,
// because the converter leaves drawings out.
func lgReplayAnswerText(exchange sitelgreplay.Exchange) (string, error) {
	if strings.HasPrefix(exchange.ContentType, "text/plain") {
		return exchange.Body, nil
	}
	if !strings.HasPrefix(exchange.ContentType, "text/html") {
		return "(" + exchange.ContentType + ", " + strconv.Itoa(len(exchange.Body)) + " bytes)", nil
	}
	if strings.Contains(exchange.Body, "<svg") {
		return "(a drawing, " + strconv.Itoa(len(exchange.Body)) + " bytes of SVG)", nil
	}
	markup := exchange.Body
	if start := strings.Index(markup, "<body"); start >= 0 {
		end := strings.LastIndex(markup, "</body>")
		if end < start {
			return "", fmt.Errorf("the answer to %s opens a body and never closes it", exchange.Key)
		}
		opened := strings.Index(markup[start:], ">")
		markup = markup[start+opened+1 : end]
	}
	return htmlToMarkdown(markup, pageCanonicalURL(lgReplayPage))
}

// lgReplayProvenance answers where the recording comes from, in one paragraph
// the page and the transcript share.
func lgReplayProvenance(recording sitelgreplay.Recording) string {
	provenance := recording.Provenance
	commit := provenance.Commit
	if len(commit) > lgReplayCommitShown {
		commit = commit[:lgReplayCommitShown]
	}
	date, _, _ := strings.Cut(provenance.CommitDate, "T")
	// readLGReplay refuses a recording of a tree with uncommitted changes, so
	// the commit named here is the code that answered.
	text := "Recorded by " + provenance.Recorder + " from commit " + commit + " (" + date + ")"
	text += ": a real ze, " + strconv.Itoa(provenance.Speakers) + " simulated BGP speakers generated from seed " +
		strconv.FormatUint(provenance.Seed, 10) + ", " + strconv.Itoa(len(recording.Exchanges)) + " recorded answers."
	replaced := 0
	for _, normalization := range provenance.Normalized {
		replaced += normalization.Count
	}
	text += " " + strconv.Itoa(replaced) + " values that change from run to run (session uptimes, state-change times, the listener address) are fixed values the recording declares."
	if len(provenance.Varies) > 0 {
		text += " " + strconv.Itoa(len(provenance.Varies)) + " answers still differ from one run to the next, and the recording names them."
	}
	return text + " " + lgReplaySimulated
}

// lgReplayBlockHTML answers the replay as the page that names the block
// carries it: the mount the player fills, the tour as a list, and the
// transcript link. Without JavaScript the list and the link are the page.
func lgReplayBlockHTML(paths Paths, name string) (string, error) {
	recording, recorded, err := readLGReplay(paths)
	if err != nil {
		return "", err
	}
	if !recorded {
		return `<div class="lg-replay">
    <h2 id="lg-replay-title">The looking glass, replayed</h2>
    <p class="lg-replay__unrecorded">` + lgReplayUnrecorded + `</p>
</div>`, nil
	}
	var steps strings.Builder
	for index, step := range recording.Tour {
		exchange, found := recording.Exchange(step.Exchange)
		if !found {
			return "", fmt.Errorf("the looking glass recording's tour step %d names %s, which the recording does not hold", index+1, step.Exchange)
		}
		steps.WriteString(`        <li data-lg-replay-step="` + strconv.Itoa(index) + `"><span data-lg-replay-caption>` +
			html.EscapeString(step.Caption) + `</span> <code>` + html.EscapeString(lgReplayRequestText(exchange)) + "</code></li>\n")
	}
	transcript := lgReplayDirectory + "/" + lgReplayTranscript
	return `<div class="lg-replay">
    <h2 id="lg-replay-title">The looking glass, replayed</h2>
    <p>
        Below is the looking glass itself, not a picture of it: its own pages,
        stylesheet and scripts, answering from the answers a real ze gave while
        it served the simulated network. Click a peer, a route or a graph
        button, or play the tour. Only the recorded questions have an answer,
        so a search for anything else shows a notice that names the request.
    </p>
    <div class="lg-replay__player" data-lg-replay data-lg-replay-frame="` + lgReplayDirectory + "/" + lgReplayFrame + `">
        <p class="lg-replay__fallback">
            The replay runs in your browser and needs JavaScript.
            <a href="` + transcript + `">Read the transcript of the tour</a>.
        </p>
    </div>
    <h3>The tour</h3>
    <ol class="lg-replay__tour">
` + steps.String() + `    </ol>
    <p class="lg-replay__provenance">
        ` + html.EscapeString(lgReplayProvenance(recording)) + `
        <a href="` + transcript + `">Transcript of the tour</a>.
    </p>
    <script src="` + pageRoot(name) + lgReplayScript + `" defer></script>
</div>`, nil
}
