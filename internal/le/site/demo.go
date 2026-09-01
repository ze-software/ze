// Design: website/AI.md -- a recorded demonstration is published from verified local media
// Detail: docs.go expands the markers of one page; terminaldemo owns the manifest contract.
package site

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/terminaldemo"
)

// demoMarker matches the comment a page source writes where its recorded
// demonstration goes.
var demoMarker = regexp.MustCompile(`<!--\s*terminal-demo:\s*([a-z0-9-]+)\s*-->`)

// The player this site serves for an asciicast. Both files are pinned and
// committed under website/assets/vendor/, so a page fetches nothing from a
// content delivery network.
const (
	demoPlayerScript     = "assets/vendor/asciinema-player.min.js"
	demoPlayerStylesheet = "assets/vendor/asciinema-player.css"
)

// The four assets a recording can publish. Each name is the key its manifest
// records the file under and the word the error messages use.
const (
	demoCast       = "cast"
	demoPoster     = "poster"
	demoTranscript = "transcript"
	demoVideo      = "video"
)

// demoAssetExtensions name the file each kind of demo asset is published as.
var demoAssetExtensions = map[string]string{
	demoCast: ".cast", demoPoster: ".png", demoTranscript: ".txt", demoVideo: ".webm",
}

// demoKindAssets name the asset set each kind of demonstration publishes, and
// nothing else.
//
// A terminal session is a byte stream, so it records an asciicast the player
// replays. A browser recording has no byte stream and stays a video with a
// poster frame. A demo that published both would show a reader two players.
var demoKindAssets = map[string][]string{
	"terminal": {demoCast, demoTranscript},
	"browser":  {demoPoster, demoTranscript, demoVideo},
}

// The nominal monospace cell, used only to reserve the demo box before the
// player loads. They are JetBrains Mono's own metrics, so the reserved box has
// the shape of the recording. The player measures the reader's real font at
// run time and scales into whatever box it is given, so a small disagreement
// costs letterboxing rather than a reflow.
const (
	demoCellAdvanceRatio = 0.6
	demoCellLineRatio    = 1.32
)

// demoCatalog answers what one build publishes for each recorded
// demonstration.
//
// It is LAZY: a build whose pages carry no marker never opens a manifest, so a
// checkout that has not rendered its demonstrations still publishes every page
// that shows none. The first page that carries a marker loads the two
// manifests, and a load that fails fails every page after it the same way,
// because the answer cannot change inside one build.
type demoCatalog struct {
	paths  Paths
	loaded bool
	demos  map[string]terminaldemo.Demo
	built  map[string]terminaldemo.ArtifactEntry
	page   string
	err    error
}

// errDemoMediaAbsent says this checkout holds no rendered demonstrations at
// all, which is a different answer from a recording that fails its digest.
//
// The media is generated rather than authored, and a render is the only thing
// that writes it. So a checkout whose artifact tree holds no recording cannot
// publish the pages that show one, and this error names the command to run
// rather than a path that is missing.
var errDemoMediaAbsent = errors.New("this checkout holds no rendered terminal demonstrations: run `./le terminal-demo render-all` before a site build")

// newDemoCatalog answers a catalog that has read nothing yet.
func newDemoCatalog(paths Paths) *demoCatalog {
	return &demoCatalog{paths: paths}
}

// assetRoot is where a render writes the media this build publishes: the
// ARTIFACT tree, which is where `./le terminal-demo render` and `render-all`
// write by default (renderEngine, internal/le/terminaldemo/actions.go).
//
// It read the website SOURCE tree until 2026-09-01, and nothing ever copied one
// to the other. The staging list comes from `git ls-files --exclude-standard`
// run in `website/`, and `website/.gitignore` ignores `assets/demos`, so the
// media was excluded from staging and the build published whatever the previous
// artifact carried. A page therefore stamped a `?v=` digest read from one
// directory over bytes served from another, and the two were different
// recordings of the same demonstration.
//
// The media survives the rebuild because seedArtifact lays the last published
// artifact back down before any producer reads this (build.go). A recording
// made since that publish is in it, because the render wrote it there.
func (catalog *demoCatalog) assetRoot() string {
	return filepath.Join(catalog.paths.Output, "assets", "demos")
}

// load reads the two manifests once and indexes the demonstrations by id.
func (catalog *demoCatalog) load() error {
	if catalog.loaded {
		return catalog.err
	}
	catalog.loaded = true
	if _, err := os.Stat(catalog.assetRoot()); os.IsNotExist(err) {
		catalog.err = errDemoMediaAbsent
		return catalog.err
	}
	manifest, built, err := terminaldemo.Published(catalog.paths.Repository, catalog.assetRoot())
	if err != nil {
		catalog.err = err
		return err
	}
	catalog.demos = make(map[string]terminaldemo.Demo, len(manifest.Demos))
	for index := range manifest.Demos {
		id := manifest.Demos[index].ID
		if _, repeated := catalog.demos[id]; repeated {
			catalog.err = fmt.Errorf("duplicate terminal demo id: %s", id)
			return catalog.err
		}
		catalog.demos[id] = manifest.Demos[index]
	}
	catalog.built = built
	catalog.page = manifest.GalleryPage
	return nil
}

// expand replaces every terminal-demo marker of one page with the player and
// with the Markdown a mirror shows, and answers the head fragment the page
// needs.
//
// A page with no marker links neither the player's stylesheet nor its script,
// so the other seven hundred published pages download neither.
//
// docRel names the page inside docs/, and is empty for a page the manifest
// does not name. When it is set, a demonstration may appear only on its own
// page or on the gallery, so a marker copied to a second page is refused
// rather than published twice.
func (catalog *demoCatalog) expand(body, mirror, root, docRel string) (string, string, string, error) {
	inBody := demoMarker.FindAllStringSubmatch(body, -1)
	inMirror := demoMarker.FindAllStringSubmatch(mirror, -1)
	if len(inBody) == 0 && len(inMirror) == 0 {
		return body, mirror, "", nil
	}
	if !sameDemoOrder(inBody, inMirror) {
		return "", "", "", fmt.Errorf("terminal demo markers changed during Markdown rendering")
	}
	if err := catalog.load(); err != nil {
		return "", "", "", err
	}
	bodyByID := map[string]string{}
	mirrorByID := map[string]string{}
	needsPlayer := false
	for _, marker := range inMirror {
		id := marker[1]
		if _, done := bodyByID[id]; done {
			continue
		}
		demo, known := catalog.demos[id]
		if !known {
			return "", "", "", fmt.Errorf("unknown terminal demo marker: %s", id)
		}
		if docRel != "" && docRel != demo.Page && docRel != catalog.page {
			return "", "", "", fmt.Errorf("terminal demo %s belongs on %s, not %s", id, demo.Page, docRel)
		}
		entry, generated := catalog.built[id]
		if !generated {
			return "", "", "", fmt.Errorf("terminal demo %s has no generated artifacts", id)
		}
		assets, err := catalog.verifyAssets(id, demo.Kind, entry)
		if err != nil {
			return "", "", "", err
		}
		transcript, err := os.ReadFile(assets[demoTranscript]) //nolint:gosec // the path is one this build verified against the artifact manifest
		if err != nil {
			return "", "", "", err
		}
		text := strings.TrimRight(string(transcript), " \t\r\n")
		duration := demo.Duration
		var facts *castFacts
		if cast, replayed := assets[demoCast]; replayed {
			needsPlayer = true
			read, err := readCastFacts(cast)
			if err != nil {
				return "", "", "", err
			}
			facts = &read
			duration = durationPhrase(read.Seconds)
		}
		label, err := demoPlatformLabel(demo.Platform)
		if err != nil {
			return "", "", "", err
		}
		sentence, err := demoPlatformSentence(demo.Platform)
		if err != nil {
			return "", "", "", err
		}
		render := &demoRender{
			ID: id, Demo: demo, Entry: entry, Root: root, Duration: duration,
			Facts: facts, Transcript: text, PlatformLabel: label, PlatformSentence: sentence,
		}
		bodyByID[id] = renderDemoHTML(render)
		mirrorByID[id] = renderDemoMarkdown(render)
	}
	head := ""
	if needsPlayer {
		head = demoPlayerHead(root)
	}
	return substituteDemoMarkers(body, bodyByID), substituteDemoMarkers(mirror, mirrorByID), head, nil
}

// heroMount answers the player element the homepage hero replays, bound to the
// same manifest every other page reads.
//
// The hero used to spell the recording's asset paths and digests by hand, which
// made it a fourth place the asset set was written down and the only one no
// render could correct. A recording that is not a terminal one is refused: the
// hero frame is a terminal window, so a video in it would be a frame around the
// wrong thing.
func (catalog *demoCatalog) heroMount(id, root, label string) (string, error) {
	if err := catalog.load(); err != nil {
		return "", err
	}
	demo, known := catalog.demos[id]
	if !known {
		return "", fmt.Errorf("unknown terminal demo: %s", id)
	}
	if demo.Kind != "terminal" {
		return "", fmt.Errorf("hero demo %s is a %s recording, and the hero frame replays a terminal", id, demo.Kind)
	}
	entry, generated := catalog.built[id]
	if !generated {
		return "", fmt.Errorf("terminal demo %s has no generated artifacts", id)
	}
	assets, err := catalog.verifyAssets(id, demo.Kind, entry)
	if err != nil {
		return "", err
	}
	facts, err := readCastFacts(assets[demoCast])
	if err != nil {
		return "", err
	}
	return playerMount(html.EscapeString(demoAssetURL(root, id, entry, demoCast)),
		html.EscapeString(demoAssetURL(root, id, entry, demoTranscript)),
		facts, html.EscapeString(label)), nil
}

// sameDemoOrder reports whether two marker runs name the same demonstrations
// in the same order.
func sameDemoOrder(left, right [][]string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index][1] != right[index][1] {
			return false
		}
	}
	return true
}

// substituteDemoMarkers replaces each marker with what its demonstration
// renders to.
func substituteDemoMarkers(text string, rendered map[string]string) string {
	return demoMarker.ReplaceAllStringFunc(text, func(marker string) string {
		return rendered[demoMarker.FindStringSubmatch(marker)[1]]
	})
}

// verifyAssets answers the file behind each asset one demonstration kind
// publishes, having checked every one against the size and digest its manifest
// states.
//
// A demonstration that publishes an asset its kind does not name is refused,
// which is what keeps a half-converted recording from showing a player and a
// video at once.
func (catalog *demoCatalog) verifyAssets(id, kind string, entry terminaldemo.ArtifactEntry) (map[string]string, error) {
	names, known := demoKindAssets[kind]
	if !known {
		return nil, fmt.Errorf("terminal demo %s has an unknown kind: %q", id, kind)
	}
	published := make(map[string]string, len(names))
	for _, name := range names {
		path, err := catalog.verifyAsset(id, name, entry)
		if err != nil {
			return nil, err
		}
		published[name] = path
	}
	for name := range entry.Assets {
		if _, expected := published[name]; !expected {
			return nil, fmt.Errorf("terminal demo %s is a %s demo and must not publish %s", id, kind, name)
		}
	}
	return published, nil
}

// verifyAsset answers the file behind one asset, refusing a path that leaves
// the media root and a file whose size or digest disagrees with the manifest.
func (catalog *demoCatalog) verifyAsset(id, name string, entry terminaldemo.ArtifactEntry) (string, error) {
	metadata, present := entry.Assets[name]
	if !present || metadata.Path == "" {
		return "", fmt.Errorf("terminal demo %s is missing its %s artifact", id, name)
	}
	clean := filepath.FromSlash(metadata.Path)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(filepath.Clean(clean), ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("terminal demo %s %s path leaves the demo root: %s", id, name, metadata.Path)
	}
	path := filepath.Join(catalog.assetRoot(), clean)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("terminal demo %s %s artifact is missing: %s", id, name, path)
	}
	if info.Size() != metadata.Bytes {
		return "", fmt.Errorf("terminal demo %s %s artifact is %d bytes, and its manifest states %d", id, name, info.Size(), metadata.Bytes)
	}
	digest, err := fileDigest(path)
	if err != nil {
		return "", err
	}
	if digest != metadata.SHA256 {
		return "", fmt.Errorf("terminal demo %s %s artifact digest does not match its manifest", id, name)
	}
	return path, nil
}

// fileDigest answers the SHA-256 of one file, read in bounded chunks so a
// large recording never sits in memory whole.
func fileDigest(path string) (string, error) {
	handle, err := os.Open(path) //nolint:gosec // the path is one this build resolved inside its own media root
	if err != nil {
		return "", err
	}
	defer handle.Close() //nolint:errcheck // the read side has nothing to report on close

	digest := sha256.New()
	if _, err := io.Copy(digest, handle); err != nil {
		return "", fmt.Errorf("digest %s: %w", path, err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// castFacts is what a recording says about itself: the grid it was recorded on
// and how long it runs.
type castFacts struct {
	Columns int
	Rows    int
	Seconds float64
}

// readCastFacts reads the grid and the running time out of an asciicast v2
// file, whose first line is a JSON header and whose remaining lines are the
// timed events.
func readCastFacts(path string) (castFacts, error) {
	content, err := os.ReadFile(path) //nolint:gosec // the path is one this build verified against the artifact manifest
	if err != nil {
		return castFacts{}, err
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	var header struct {
		Version int `json:"version"`
		Width   int `json:"width"`
		Height  int `json:"height"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil || header.Version != 2 {
		return castFacts{}, fmt.Errorf("not an asciicast v2 recording: %s", path)
	}
	if header.Width < 1 || header.Height < 1 {
		return castFacts{}, fmt.Errorf("asciicast %s records a %dx%d grid", path, header.Width, header.Height)
	}
	facts := castFacts{Columns: header.Width, Rows: header.Height}
	for index := len(lines) - 1; index >= 1; index-- {
		if strings.TrimSpace(lines[index]) == "" {
			continue
		}
		var event []json.RawMessage
		if err := json.Unmarshal([]byte(lines[index]), &event); err != nil || len(event) == 0 {
			return castFacts{}, fmt.Errorf("asciicast %s ends with an event it does not state a time for", path)
		}
		if err := json.Unmarshal(event[0], &facts.Seconds); err != nil {
			return castFacts{}, fmt.Errorf("asciicast %s ends with an event it does not state a time for", path)
		}
		break
	}
	if facts.Seconds < 0 {
		return castFacts{}, fmt.Errorf("asciicast %s ends at %v seconds", path, facts.Seconds)
	}
	return facts, nil
}

// durationPhrase spells a running time the way the demo catalog spells one.
func durationPhrase(seconds float64) string {
	total := int(seconds + 0.5)
	if total < 60 {
		return plural(total, "second")
	}
	phrase := plural(total/60, "minute")
	if rest := total % 60; rest != 0 {
		phrase += " " + plural(rest, "second")
	}
	return phrase
}

// plural answers a count and its unit, with the unit made plural when the
// count is not one.
func plural(count int, unit string) string {
	if count == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(count) + " " + unit + "s"
}

// demoRatio spells one dimension of the reserved box, with the trailing zeros
// a browser does not need removed.
func demoRatio(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// reservedBox answers the aspect ratio a page reserves for a recording of one
// grid, so the demo does not reflow when the player loads.
func reservedBox(facts castFacts) string {
	return demoRatio(float64(facts.Columns)*demoCellAdvanceRatio) + " / " +
		demoRatio(float64(facts.Rows)*demoCellLineRatio)
}

// demoPlayerHead answers the player's own stylesheet and script, for the head
// of a page that carries an asciicast.
func demoPlayerHead(root string) string {
	return `        <link rel="stylesheet" href="` + root + demoPlayerStylesheet + "\" />\n" +
		`        <script src="` + root + demoPlayerScript + "\" defer></script>\n"
}

// demoAssetURL answers the URL one asset is published at, with the first ten
// characters of its digest as a cache-busting version.
func demoAssetURL(root, id string, entry terminaldemo.ArtifactEntry, name string) string {
	digest := entry.Assets[name].SHA256
	if len(digest) > 10 {
		digest = digest[:10]
	}
	return root + "assets/demos/" + id + demoAssetExtensions[name] + "?v=" + digest
}

// demoPlatformLabel and demoPlatformSentence name where a demonstration was
// recorded, as a caption cell and as a sentence.
func demoPlatformLabel(platform string) (string, error) {
	switch platform {
	case "linux":
		return "Linux namespace lab", nil
	case "portable":
		return "macOS and Linux", nil
	}
	return "", fmt.Errorf("unknown terminal demo platform: %q", platform)
}

func demoPlatformSentence(platform string) (string, error) {
	switch platform {
	case "linux":
		return "in a Linux namespace lab", nil
	case "portable":
		return "on macOS and Linux", nil
	}
	return "", fmt.Errorf("unknown terminal demo platform: %q", platform)
}

// demoRelease answers the release a recording states, or the word the retired
// renderer published when an artifact stated none.
func demoRelease(entry terminaldemo.ArtifactEntry) string {
	if entry.Release == "" {
		return "unknown"
	}
	return entry.Release
}

// playerMount answers the element the page's own script turns into a player,
// sized before that script runs.
func playerMount(castURL, transcriptURL string, facts castFacts, label string) string {
	return `<div
      class="terminal-demo__player"
      data-terminal-demo-player
      data-cast-src="` + castURL + `"
      data-cols="` + strconv.Itoa(facts.Columns) + `"
      data-rows="` + strconv.Itoa(facts.Rows) + `"
      style="--demo-aspect: ` + reservedBox(facts) + `"
      aria-label="` + label + `"
    ></div>
    <noscript>
      <p class="terminal-demo__noscript">This recording is replayed by the
      page's own player. <a href="` + castURL + `">Download the asciicast</a> or
      <a href="` + transcriptURL + `">read the transcript</a>.</p>
    </noscript>`
}

// demoRender is one verified demonstration, ready to publish. Every field is
// settled before either renderer runs, so neither has a decision left to make
// and neither can fail.
type demoRender struct {
	ID   string
	Demo terminaldemo.Demo
	// Entry is the artifact manifest's record of the media this publishes.
	Entry terminaldemo.ArtifactEntry
	// Root is the relative path from the page back to the site root.
	Root string
	// Duration is the running time as a reader reads it, taken from the
	// recording itself when it carries one.
	Duration string
	// Facts is the recording's own grid and length, nil for a browser demo,
	// which has no byte stream and stays a video.
	Facts *castFacts
	// Transcript is the recorded session as text, with its trailing blank
	// lines removed.
	Transcript string
	// PlatformLabel and PlatformSentence name where the recording ran, as a
	// caption cell and as a sentence.
	PlatformLabel    string
	PlatformSentence string
}

// renderDemoHTML answers the figure one demonstration publishes as.
func renderDemoHTML(render *demoRender) string {
	id, demo, entry, root := render.ID, render.Demo, render.Entry, render.Root
	facts, duration, transcript := render.Facts, render.Duration, render.Transcript
	label := html.EscapeString(demo.Title + " demonstration")
	transcriptURL := html.EscapeString(demoAssetURL(root, id, entry, demoTranscript))
	kindLabel := "Terminal"
	if demo.Kind == "browser" {
		kindLabel = "Browser"
	}

	// A demonstration with no cast facts is a browser recording, and it keeps
	// the video element, the poster frame and the WEBM label it has always had.
	var player, format string
	if facts == nil {
		poster := html.EscapeString(demoAssetURL(root, id, entry, demoPoster))
		video := html.EscapeString(demoAssetURL(root, id, entry, demoVideo))
		format = "WEBM"
		player = `<video controls playsinline preload="metadata" poster="` + poster + `" ` +
			`aria-label="` + label + "\">\n" +
			`      <source src="` + video + "\" type=\"video/webm\">\n" +
			"      Your browser cannot play WebM video. " +
			`<a href="` + video + "\">Download the recording</a>.\n" +
			"    </video>"
	} else {
		format = "CAST"
		player = playerMount(html.EscapeString(demoAssetURL(root, id, entry, demoCast)), transcriptURL, *facts, label)
	}

	return `<figure class="terminal-demo" data-terminal-demo="` + html.EscapeString(id) + `">
  <div class="terminal-demo__intro">
    <div>
      <span class="terminal-demo__eyebrow">` + html.EscapeString("Replayable Ze "+strings.ToLower(demo.Kind)+" lab") + `</span>
      <h3>` + html.EscapeString(demo.Title) + `</h3>
      <p>` + html.EscapeString(demo.Description) + `</p>
    </div>
    <span class="terminal-demo__status"><i aria-hidden="true"></i> Reproducible</span>
  </div>
  <div class="terminal-demo__frame">
    <div class="terminal-demo__bar" aria-hidden="true">
      <span class="terminal-demo__dots"><i></i><i></i><i></i></span>
      <span>` + html.EscapeString(id+"."+strings.ToLower(demo.Kind)) + `</span>
      <span>` + format + `</span>
    </div>
    ` + player + `
  </div>
  <figcaption>
    <span>Ze ` + html.EscapeString(demoRelease(entry)) + `</span><span>` + html.EscapeString(duration) +
		`</span><span>` + html.EscapeString(render.PlatformLabel) + `</span><span>` + kindLabel +
		`</span><span>` + html.EscapeString(demo.Engine) + `</span>
    <a href="` + transcriptURL + `">Plain-text transcript</a>
  </figcaption>
  <details class="terminal-demo__transcript">
    <summary>Read the demonstration transcript</summary>
    <pre><code>` + html.EscapeString(transcript) + `</code></pre>
  </details>
</figure>`
}

// renderDemoMarkdown answers the section a page's Markdown mirror shows in
// place of the player: the same facts, and links to the media itself.
func renderDemoMarkdown(render *demoRender) string {
	id, demo, entry, root := render.ID, render.Demo, render.Entry, render.Root
	facts, duration, transcript := render.Facts, render.Duration, render.Transcript
	transcriptURL := demoAssetURL(root, id, entry, demoTranscript)
	links := "[Download the asciicast recording](" + demoAssetURL(root, id, entry, demoCast) +
		") · [Plain-text transcript](" + transcriptURL + ")"
	if facts == nil {
		links = "[Play the WebM recording](" + demoAssetURL(root, id, entry, demoVideo) +
			") · [View the poster](" + demoAssetURL(root, id, entry, demoPoster) +
			") · [Plain-text transcript](" + transcriptURL + ")"
	}
	return "### Demo: " + demo.Title + "\n\n" +
		demo.Description + "\n\n" +
		links + "\n\n" +
		"Recorded with Ze " + demoRelease(entry) + " " + render.PlatformSentence + " using " + demo.Engine +
		". Duration: " + duration + ".\n\n" +
		"```console\n" + transcript + "\n```\n"
}
