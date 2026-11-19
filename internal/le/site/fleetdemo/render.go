//go:build ze_web && ze_fleet_ui

// Design: docs/contributing/gh-pages.md -- the fleet interface demonstration: what a render records and writes
// Related: actions.go -- the action that calls render, and the demonstration it is given
// Related: render_absent.go -- render in a le built without the fleet interface
// Related: tour.go -- the requests a render records, its chunks and its bounds
// Detail: host.go -- the host page that mounts the frame

package sitefleetdemo

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/component/web/fleetui/sample"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/site"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

const (
	// demoAccount is the account name the recorded pages show in their
	// header.
	demoAccount = "demo"

	// entryPath is the estate dashboard, the page the frame opens on.
	entryPath    = "/fleet/"
	entryCaption = "The estate dashboard"

	// The prefixes of the files a fleet page loads: the fleet package's own
	// styles and scripts, and the web package's shared ones (htmx, uPlot).
	fleetAssetsPrefix = "/fleet/assets/"
	webAssetsPrefix   = "/assets/"

	// answerBytesMax bounds one recorded answer. The largest fleet page and
	// the largest shared script are well under a megabyte.
	answerBytesMax = 16 << 20

	// The files of the demonstration, in demoDirectory, beside one file per
	// chunk.
	hostFile      = "index.html"
	frameFile     = "frame.html"
	recordingFile = "recording.json"

	// What the player shows for a request outside the recording, in the
	// fleet stylesheet's own banner.
	playerTitle   = "Ze fleet interface, recorded demonstration"
	playerSubject = "Ze fleet interface"
	noticeText    = "This is not part of the recorded demonstration: " + sitelgreplay.NoticeRequestToken
	noticeClass   = "fleet-banner"

	// actionNotSent is the toast a posted action answers, and
	// consoleNotSent the terminal's answer to a command it does not suggest.
	actionNotSent  = "Recorded demonstration: this action was not sent"
	consoleNotSent = "This is not part of the recorded demonstration: the command was not sent. Run one of the suggested commands."

	htmlContentType = "text/html; charset=utf-8"
	// viaFallback marks a fallback's answer (Exchange.Via).
	viaFallback = "fallback"
)

// plainSource exposes the sample estate as a fleetui.Source that says it is
// sample data, and nothing more: the prototype's stepping, which the shell
// looks for beside it, is hidden by it.
type plainSource struct {
	fleetui.Source
}

// SampleData keeps the header's sample-data tag (fleetui.SampleSource): the
// demonstration says it shows sample data.
func (plainSource) SampleData() {}

// recorded is one recording of the tour, the bytes of each of its chunk
// files by chunk name, and what each chunk and the whole hold by kind of
// request: by chunk name, and the whole's under wholeTour.
type recorded struct {
	recording sitelgreplay.Recording
	contents  map[string][]byte
	sizes     map[string][]kindSize
}

// render records the tour over the sample estate at the demonstration's
// clock, and writes the host page, the frame, the recording's index and one
// file per chunk into the output's demoDirectory. It writes nowhere else, and
// nothing when a bound is passed.
func render(ctx context.Context, demo demonstration) (rendered, error) {
	directory, err := demoDirectoryIn(demo.output)
	if err != nil {
		return rendered{}, err
	}
	player, err := os.ReadFile(filepath.Join(demo.repository, filepath.FromSlash(site.ReplayPlayerSource))) //nolint:gosec // the player of the checkout lepath.Root found
	if err != nil {
		return rendered{}, opererr.Wrap(err, "The replay player "+site.ReplayPlayerSource+" cannot be read",
			"The frame carries the player inline, as the looking glass replay's does, so no frame can be written without it.",
			"git -C "+demo.repository+" status -- "+site.ReplayPlayerSource)
	}
	made, err := recordTour(ctx, demo)
	if err != nil {
		return rendered{}, err
	}
	recording := made.recording

	frame, err := site.ReplayFrameDocument(recording, string(player))
	if err != nil {
		return rendered{}, opererr.Wrap(err, "The frame of the demonstration could not be written",
			"The recording was made, and the frame writer refused it, so nothing was written.",
			RenderCommand)
	}
	if err := tourBounds.check(recording.Chunks, len(frame), made.sizes); err != nil {
		return rendered{}, err
	}
	encoded, err := recording.Encode()
	if err != nil {
		return rendered{}, err
	}
	chunkFiles, err := sitelgreplay.ChunkFiles(recording.Chunks)
	if err != nil {
		return rendered{}, err
	}

	type demoFile struct {
		name    string
		content []byte
	}
	files := []demoFile{
		{name: hostFile, content: hostPage(demo.provenance, chunkFiles)},
		{name: frameFile, content: frame},
		{name: recordingFile, content: encoded},
	}
	written := rendered{exchanges: len(recording.Exchanges), assets: len(recording.Assets), kinds: made.sizes[wholeTour]}
	for _, chunk := range recording.Chunks {
		files = append(files, demoFile{name: chunk.File, content: made.contents[chunk.Name]})
		written.chunks = append(written.chunks, writtenChunk{
			name: chunk.Name, exchanges: len(chunk.Keys), size: chunk.Size, kinds: made.sizes[chunk.Name],
		})
		written.exchanges += len(chunk.Keys)
	}
	for _, file := range files {
		target := filepath.Join(directory, file.name)
		if err := os.WriteFile(target, file.content, 0o644); err != nil { //nolint:gosec // a page of the website output's unpublished tmp/
			return rendered{}, writeFailure(err, target)
		}
		written.files = append(written.files, writtenFile{path: demoDirectory + "/" + file.name, size: len(file.content)})
	}

	return written, nil
}

// demoDirectoryIn answers the demonstration's directory in output, made if
// absent. It refuses an output that is no built website output, since the
// host page loads the site's player and stylesheet from it, and a directory
// on the way that is a symbolic link, which could lead the files out of the
// output.
func demoDirectoryIn(output string) (string, error) {
	for _, built := range []string{"index.html", "assets/site.css", "assets/lg-replay.js"} {
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(built))); err != nil {
			return "", opererr.Wrap(err, output+" is not a built website output: it has no "+built,
				"The demonstration is written into a website output, and its host page loads the site's player and stylesheet from the output's assets.",
				"./le site build output "+output)
		}
	}

	directory := output
	for _, segment := range strings.Split(demoDirectory, "/") {
		directory = filepath.Join(directory, segment)
		info, err := os.Lstat(directory)
		if errors.Is(err, fs.ErrNotExist) {
			if err := os.Mkdir(directory, 0o755); err != nil { //nolint:gosec // the website output's unpublished tmp/
				return "", writeFailure(err, directory)
			}
			continue
		}
		if err != nil {
			return "", writeFailure(err, directory)
		}
		if !info.IsDir() {
			return "", opererr.New(directory+" is not a directory of the website output",
				"The demonstration is written only under "+demoDirectory+"/ of the output, and a link or a file there could lead it elsewhere.",
				"ls -l "+directory)
		}
	}

	return directory, nil
}

// writeFailure answers the operator error of a demonstration file that could
// not be written.
func writeFailure(cause error, path string) *opererr.Error {
	return opererr.Wrap(cause, "The demonstration could not be written to "+path,
		"The recording was made in memory and is lost; the files already in "+demoDirectory+"/ may be from an earlier render.",
		RenderCommand)
}

// recordTour serves the fleet pages over a new sample estate at the
// demonstration's clock, records the tour with every file its pages load, and
// splits the answers into one chunk per object. The estate's script is held,
// so no event changes it meanwhile.
func recordTour(ctx context.Context, demo demonstration) (recorded, error) {
	source := sample.New(demo.clock)
	defer source.Close()
	source.Hold()

	renderer, err := web.NewRenderer()
	if err != nil {
		return recorded{}, err
	}
	// The routes see the estate through plainSource only, so the shell
	// draws no prototype control and mounts no step routes: in the
	// recording they would do nothing.
	served := plainSource{Source: source}
	mux := http.NewServeMux()
	mux.Handle(entryPath, fleetui.NewRoutes(served, demoAccount))
	mux.Handle(webAssetsPrefix, http.StripPrefix(webAssetsPrefix, renderer.AssetHandler()))
	server := httptest.NewServer(mux)
	defer server.Close()

	plan, err := planTour(ctx, served, demoAccount, demo.scope)
	if err != nil {
		return recorded{}, err
	}
	crawler := sitelgreplay.Crawler{
		Base: server.URL, Client: server.Client(),
		AssetPrefixes: []string{fleetAssetsPrefix, webAssetsPrefix}, BodyBytesMax: answerBytesMax,
	}
	crawled, err := crawlTour(ctx, crawler, &plan)
	if err != nil {
		return recorded{}, err
	}

	entry := sitelgreplay.ExchangeKey(http.MethodGet, entryPath, nil, nil, false)
	names := make(map[string]string, len(crawled.Exchanges))
	byChunk := map[string][]sitelgreplay.Exchange{}
	for index := range crawled.Exchanges {
		exchange := crawled.Exchanges[index]
		if exchange.Key == entry {
			continue
		}
		name, err := chunkOf(exchange)
		if err != nil {
			return recorded{}, err
		}
		names[exchange.Key] = name
		byChunk[name] = append(byChunk[name], exchange)
	}
	sizes := map[string][]kindSize{wholeTour: sizeByKind(crawled.Exchanges)}
	for name, exchanges := range byChunk {
		sizes[name] = sizeByKind(exchanges)
	}
	split, err := sitelgreplay.SplitChunks(crawled.Exchanges, func(exchange sitelgreplay.Exchange) string { return names[exchange.Key] })
	if err != nil {
		return recorded{}, err
	}
	fallbacks, err := demoFallbacks(ctx)
	if err != nil {
		return recorded{}, err
	}

	return recorded{
		recording: sitelgreplay.Recording{
			Schema:     sitelgreplay.Schema,
			Provenance: demo.provenance,
			Entry:      entry,
			Tour:       []sitelgreplay.TourStep{{Caption: entryCaption, Exchange: entry}},
			Exchanges:  split.Inline,
			Assets:     crawled.Assets,
			Player: &sitelgreplay.Player{
				Title: playerTitle, Subject: playerSubject, HostPage: hostFile,
				AssetPrefixes: []string{fleetAssetsPrefix, webAssetsPrefix},
				NoticeText:    noticeText, NoticeClass: noticeClass,
				// The page's stream client sees a quiet stream; the shell
				// script resolves its addresses against the pushed one; a
				// dialog form closes as the browser closes it.
				SilentEventSource: true, KeepPushedAddress: true, PlainForms: true,
			},
			Chunks:    split.Chunks,
			Fallbacks: fallbacks,
		},
		contents: split.Contents,
		sizes:    sizes,
	}, nil
}

// crawlTour asks the tour's steps and what their answers name, then the
// requests the fleet scripts send from those answers outside hx-get, round
// after round until a round names nothing new. Each request is asked once.
func crawlTour(ctx context.Context, crawler sitelgreplay.Crawler, plan *tour) (sitelgreplay.Crawled, error) {
	var whole sitelgreplay.Crawled
	held := map[string]bool{}
	fetched := map[string]bool{}
	crawler.Follow = func(parent sitelgreplay.Exchange, asked sitelgreplay.Request) bool {
		if held[requestKey(asked)] {
			return false
		}
		return plan.follow(parent, asked)
	}

	steps := plan.steps
	for round := 1; len(steps) > 0; round++ {
		if round > discoveryRoundsMax {
			return sitelgreplay.Crawled{}, boundFailure("discoveryRoundsMax", discoveryRoundsMax, "round "+strconv.Itoa(round)+" still names new requests")
		}
		crawler.ExchangesMax = tourExchangesMax - len(whole.Exchanges)
		if crawler.ExchangesMax < len(steps) {
			return sitelgreplay.Crawled{}, boundFailure("tourExchangesMax", tourExchangesMax,
				"the tour asks "+strconv.Itoa(len(whole.Exchanges)+len(steps))+" requests")
		}
		crawled, err := crawler.Crawl(ctx, steps)
		if err != nil {
			return sitelgreplay.Crawled{}, opererr.Wrap(err, "The fleet interface could not be recorded",
				"The demonstration holds the real fleet handlers' answers over the sample estate, so nothing was written. "+
					"A crawl past its requests names tourExchangesMax ("+strconv.Itoa(tourExchangesMax)+") of internal/le/site/fleetdemo/tour.go; "+
					"any other failure is the fleet package's, and its tests show it",
				"./le test unit pkg path ./internal/component/web/fleetui/sample/")
		}
		var fresh []sitelgreplay.Exchange
		for index := range crawled.Exchanges {
			exchange := crawled.Exchanges[index]
			if held[exchange.Key] {
				continue
			}
			held[exchange.Key] = true
			fresh = append(fresh, exchange)
		}
		whole.Exchanges = append(whole.Exchanges, fresh...)
		for _, asset := range crawled.Assets {
			if !fetched[asset.Path] {
				fetched[asset.Path] = true
				whole.Assets = append(whole.Assets, asset)
			}
		}

		found, err := discover(fresh)
		if err != nil {
			return sitelgreplay.Crawled{}, err
		}
		queued := map[string]bool{}
		steps = nil
		for _, request := range found {
			key := requestKey(request)
			if held[key] || queued[key] {
				continue
			}
			queued[key] = true
			steps = append(steps, request)
		}
	}
	return whole, nil
}

// requestKey answers the key the recording holds request's answer under.
func requestKey(request sitelgreplay.Request) string {
	return sitelgreplay.ExchangeKey(request.Method, request.Path, request.Query, request.Form, request.HTMX)
}

// demoFallbacks answers what the player shows for a post the recording does
// not hold: a command the console does not suggest answers as the terminal
// answers one, and any action answers the fleet package's own informational
// toast, so the view it was posted from stays as it was.
func demoFallbacks(ctx context.Context) ([]sitelgreplay.Fallback, error) {
	var refusal bytes.Buffer
	if err := fleetui.TerminalEntry(fleetui.TerminalEntryView{Output: consoleNotSent}).Render(ctx, &refusal); err != nil {
		return nil, err
	}
	var toast bytes.Buffer
	if err := fleetui.ToastOOB(fleetui.ToastView{Level: fleetui.ToastInformational, Message: actionNotSent}).Render(ctx, &toast); err != nil {
		return nil, err
	}
	fallback := func(prefix, body string, htmx bool) sitelgreplay.Fallback {
		return sitelgreplay.Fallback{Method: http.MethodPost, PathPrefix: prefix, Exchange: sitelgreplay.Exchange{
			Key:    sitelgreplay.ExchangeKey(http.MethodPost, prefix, nil, nil, htmx),
			Method: http.MethodPost, Path: prefix, HTMX: htmx, Via: viaFallback,
			Status: http.StatusOK, ContentType: htmlContentType, Body: body,
		}}
	}
	// The terminal posts with fetch, and every action form through htmx.
	return []sitelgreplay.Fallback{
		fallback(consolePrefix, refusal.String(), false),
		fallback(actionPrefix, toast.String(), true),
	}, nil
}
