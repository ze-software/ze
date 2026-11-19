//go:build ze_web && ze_fleet_ui

// Design: docs/contributing/gh-pages.md -- the fleet interface demonstration: what a render records and writes
// Related: actions.go -- the action that calls render, and the demonstration it is given
// Related: render_absent.go -- render in a le built without the fleet interface
// Detail: host.go -- the host page that mounts the frame

package sitefleetdemo

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/html"

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

	// The files of the demonstration, in demoDirectory.
	hostFile      = "index.html"
	frameFile     = "frame.html"
	recordingFile = "recording.json"

	// viaTour marks the declared question in a recording (Exchange.Via).
	viaTour = "tour"
)

// render records the estate dashboard of the sample estate at the
// demonstration's clock, with every file it loads, and writes the host page,
// the frame and the recording into the output's demoDirectory. It writes
// nowhere else.
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
	recording, err := recordEstate(ctx, demo)
	if err != nil {
		return rendered{}, err
	}

	encoded, err := recording.Encode()
	if err != nil {
		return rendered{}, err
	}
	frame, err := site.ReplayFrameDocument(recording, string(player))
	if err != nil {
		return rendered{}, opererr.Wrap(err, "The frame of the demonstration could not be written",
			"The recording was made, and the frame writer refused it, so nothing was written.",
			RenderCommand)
	}
	host := hostPage(demo.provenance)

	files := []struct {
		name    string
		content []byte
	}{
		{name: hostFile, content: host},
		{name: frameFile, content: frame},
		{name: recordingFile, content: encoded},
	}
	written := rendered{exchanges: len(recording.Exchanges), assets: len(recording.Assets)}
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

// recordEstate serves the fleet pages over a new sample estate at the
// demonstration's clock, and records the estate dashboard with every file it
// loads. The estate's script is held, so no event changes it meanwhile.
func recordEstate(ctx context.Context, demo demonstration) (sitelgreplay.Recording, error) {
	source := sample.New(demo.clock)
	defer source.Close()
	source.Hold()

	renderer, err := web.NewRenderer()
	if err != nil {
		return sitelgreplay.Recording{}, err
	}
	mux := http.NewServeMux()
	mux.Handle(entryPath, fleetui.NewRoutes(source, demoAccount))
	mux.Handle(webAssetsPrefix, http.StripPrefix(webAssetsPrefix, renderer.AssetHandler()))
	server := httptest.NewServer(mux)
	defer server.Close()

	entry, err := ask(ctx, server, entryPath)
	if err != nil {
		return sitelgreplay.Recording{}, err
	}
	key := sitelgreplay.ExchangeKey(http.MethodGet, entryPath, nil, nil, false)
	exchange := sitelgreplay.Exchange{
		Key: key, Method: http.MethodGet, Path: entryPath, Via: viaTour,
		Status: entry.status, ContentType: entry.contentType, Body: entry.body,
	}

	linked, err := linkedAssets(entry.body)
	if err != nil {
		return sitelgreplay.Recording{}, err
	}
	assets := make([]sitelgreplay.Asset, 0, len(linked))
	for _, path := range linked {
		answer, err := ask(ctx, server, path)
		if err != nil {
			return sitelgreplay.Recording{}, err
		}
		assets = append(assets, sitelgreplay.Asset{Path: path, ContentType: answer.contentType, Body: answer.body})
	}

	return sitelgreplay.Recording{
		Schema:     sitelgreplay.Schema,
		Provenance: demo.provenance,
		Entry:      key,
		Tour:       []sitelgreplay.TourStep{{Caption: entryCaption, Exchange: key}},
		Exchanges:  []sitelgreplay.Exchange{exchange},
		Assets:     assets,
	}, nil
}

// answer is one recorded answer of the fleet routes.
type answer struct {
	status      int
	contentType string
	body        string
}

// ask requests path from server as a browser opening it does, and answers
// the answer. Anything but 200 is refused: a recorded page that failed would
// show the failure as the interface.
func ask(ctx context.Context, server *httptest.Server, path string) (answer, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, http.NoBody)
	if err != nil {
		return answer{}, err
	}
	response, err := server.Client().Do(request)
	if err != nil {
		return answer{}, askFailure(err, path)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, answerBytesMax+1))
	closeErr := response.Body.Close()
	if readErr != nil {
		return answer{}, askFailure(readErr, path)
	}
	if closeErr != nil {
		return answer{}, askFailure(closeErr, path)
	}
	if len(body) > answerBytesMax {
		return answer{}, askFailure(errors.New("the answer is longer than "+strconv.Itoa(answerBytesMax)+" bytes"), path)
	}
	if response.StatusCode != http.StatusOK {
		return answer{}, askFailure(errors.New("status "+strconv.Itoa(response.StatusCode)+": "+string(body)), path)
	}

	return answer{status: response.StatusCode, contentType: response.Header.Get("Content-Type"), body: string(body)}, nil
}

// askFailure answers the operator error of a request the fleet routes did
// not answer.
func askFailure(cause error, path string) *opererr.Error {
	return opererr.Wrap(cause, "The fleet interface did not answer "+path+" while it was recorded",
		"The demonstration holds the answers of the real fleet handlers over the sample estate, so nothing was written. "+
			"The fleet package's tests show the same failure",
		"./le test unit pkg path ./internal/component/web/fleetui/sample/")
}

// linkedAssets answers the path of every style and script the page loads
// from the fleet assets or the shared web assets, in document order, once
// each. The page is the server's own markup, bounded by answerBytesMax.
func linkedAssets(page string) ([]string, error) {
	var paths []string
	seen := map[string]bool{}
	tokens := html.NewTokenizer(strings.NewReader(page))

	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			if errors.Is(tokens.Err(), io.EOF) {
				return paths, nil
			}
			return nil, askFailure(tokens.Err(), entryPath)
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		_, more := tokens.TagName()
		for more {
			var name, value []byte
			name, value, more = tokens.TagAttr()
			path := string(value)
			if !assetAttribute(string(name), path) {
				continue
			}
			if seen[path] {
				continue
			}
			seen[path] = true
			paths = append(paths, path)
		}
	}
}

// assetAttribute reports whether an attribute names a file the player must
// answer from the recording's assets.
func assetAttribute(name, value string) bool {
	if name != "href" && name != "src" {
		return false
	}
	if strings.HasPrefix(value, fleetAssetsPrefix) {
		return true
	}

	return strings.HasPrefix(value, webAssetsPrefix)
}
