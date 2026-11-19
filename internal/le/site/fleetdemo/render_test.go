//go:build ze_web && ze_fleet_ui

package sitefleetdemo

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

// The commit a test render names. The clock is pinned to its date, as a
// render pins it to the commit date of HEAD.
const (
	testCommit     = "0123456789abcdef0123456789abcdef01234567"
	testCommitDate = "2026-11-18T20:00:00Z"
	// sampleEstateName is the sample hub's name, which the estate dashboard
	// shows (estateName in the sample package).
	sampleEstateName = "hub-lon-01"
)

// builtOutput is what a website output holds before a render: the files
// demoDirectoryIn requires of a built output.
var builtOutput = map[string]string{
	"index.html":          "<!DOCTYPE html><title>site</title>",
	"assets/site.css":     "body {}",
	"assets/lg-replay.js": "/* player */",
}

// testDemonstration answers a demonstration of the sampled tour over a
// scratch website output, holding the files of a built one, and the player of
// this checkout. A test that needs the whole tour reads the shared render.
func testDemonstration(t *testing.T) demonstration {
	t.Helper()

	demo, err := newTestDemonstration(t.TempDir())
	require.NoError(t, err)
	demo.scope = tourScope{sampled: true}
	return demo
}

// newTestDemonstration answers a demonstration of the whole tour over output,
// after writing into it the files of a built website output.
func newTestDemonstration(output string) (demonstration, error) {
	root, err := lepath.Root()
	if err != nil {
		return demonstration{}, err
	}
	for name, content := range builtOutput {
		path := filepath.Join(output, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return demonstration{}, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // a scratch output
			return demonstration{}, err
		}
	}
	pinned, err := time.Parse(time.RFC3339, testCommitDate)
	if err != nil {
		return demonstration{}, err
	}

	return demonstration{
		repository: root,
		output:     output,
		provenance: sitelgreplay.Provenance{
			Recorder: RenderCommand, Commit: testCommit, CommitDate: testCommitDate,
			Normalized: []sitelgreplay.Normalization{}, Varies: []string{},
		},
		clock: func() time.Time { return pinned },
	}, nil
}

// outputFiles answers every file under output, relative and slash-separated.
func outputFiles(t *testing.T, output string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(output, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	require.NoError(t, err)
	slices.Sort(files)

	return files
}

// TestFleetDemoRenderWritesTheDemonstration reads what the shared render of
// the whole tour wrote into a scratch website output at a pinned clock.
//
// VALIDATES: AC-1 -- the render writes the host page, the frame, the
// recording's index and one chunk per object under tmp/fleet-demo/ and nothing
// else, and reports every chunk; the
// recording's entry is the estate dashboard of the sample estate; every style
// and script the page loads is in the recording; the host page is marked
// no-index and names the commit and its date.
// PREVENTS: a demonstration written where the site publishes it, and a frame
// whose page loads a file the player cannot answer.
func TestFleetDemoRenderWritesTheDemonstration(t *testing.T) {
	sharedRender(t)
	demo, written := shared.demo, shared.written

	recording, err := sitelgreplay.Read(filepath.Join(demo.output, filepath.FromSlash(demoDirectory), recordingFile))
	require.NoError(t, err)
	require.NotEmpty(t, recording.Chunks, "the answers of each object go to a chunk of their own")

	expected := []string{
		"assets/lg-replay.js", "assets/site.css", "index.html",
		demoDirectory + "/" + frameFile, demoDirectory + "/" + hostFile, demoDirectory + "/" + recordingFile,
	}
	exchanges := len(recording.Exchanges)
	for _, chunk := range recording.Chunks {
		expected = append(expected, demoDirectory+"/"+chunk.File)
		exchanges += len(chunk.Keys)
	}
	slices.Sort(expected)
	assert.Equal(t, expected, outputFiles(t, demo.output), "the render writes its files and its chunks and nothing else")
	for name, content := range builtOutput {
		kept, readErr := os.ReadFile(filepath.Join(demo.output, filepath.FromSlash(name)))
		require.NoError(t, readErr)
		assert.Equal(t, content, string(kept), "the render leaves %s alone", name)
	}
	assert.Len(t, written.files, 3+len(recording.Chunks))
	assert.Len(t, written.chunks, len(recording.Chunks), "the report names every chunk")
	assert.Equal(t, exchanges, written.exchanges, "the report counts the frame's answers and every chunk's")
	for _, name := range []string{"estate", "admin", "device-edge-lon-01", "group-site-lon"} {
		assert.True(t, slices.ContainsFunc(recording.Chunks, func(chunk sitelgreplay.Chunk) bool { return chunk.Name == name }),
			"the recording has the chunk %s", name)
	}

	entry, found := recording.Exchange(recording.Entry)
	require.True(t, found, "the recording holds its entry")
	assert.Equal(t, entryPath, entry.Path)
	assert.False(t, entry.HTMX, "the entry is the whole page a browser opens")
	assert.Contains(t, entry.Body, sampleEstateName, "the entry is the sample estate's dashboard")
	assert.NotContains(t, entry.Body, `id="fleet-proto-step"`, "the recording shows no prototype control, which would do nothing in it")
	assert.Equal(t, testCommit, recording.Provenance.Commit)

	held := make([]string, 0, len(recording.Assets))
	for _, asset := range recording.Assets {
		assert.NotEmpty(t, asset.Body, asset.Path)
		held = append(held, asset.Path)
	}
	assert.Contains(t, held, "/assets/htmx.min.js", "the page's shared scripts are in the recording")
	assert.Contains(t, held, "/fleet/assets/fleet.js", "the page's own scripts are in the recording")
	require.NotNil(t, recording.Player)
	assert.Equal(t, playerTitle, recording.Player.Title)
	assert.Len(t, recording.Fallbacks, 2, "the console and the actions have their fallback answers")

	host, err := os.ReadFile(filepath.Join(demo.output, filepath.FromSlash(demoDirectory), hostFile))
	require.NoError(t, err)
	assert.Contains(t, string(host), `<meta name="robots" content="noindex, nofollow">`)
	assert.Contains(t, string(host), "recorded from commit 0123456789 of 2026-11-18.")
	assert.Contains(t, string(host), `data-lg-replay-frame="frame.html"`)
	assert.Contains(t, string(host), `data-lg-replay-chunks="{&#34;admin&#34;:&#34;admin.json&#34;`, "the host names the chunk files")
	assert.Contains(t, string(host), `src="../../assets/lg-replay.js"`, "the player is named relative to the page")

	frame, err := os.ReadFile(filepath.Join(demo.output, filepath.FromSlash(demoDirectory), frameFile))
	require.NoError(t, err)
	assert.Contains(t, string(frame), sampleEstateName, "the frame carries the recording inline")
}

// followedOnlyMax is the most answers a device chunk holds when the tour did
// not plan the device but followed a link to it from another recorded page.
// A planned device holds every tab and sub-tab, far above it.
const followedOnlyMax = 5

// TestFleetDemoRenderIsDeterministic renders the sampled tour twice, into two
// scratch outputs, at the same pinned clock. The sampled tour runs the
// whole tour's code over one object of each kind and one featured and one
// ordinary device, so the package renders the whole tour once (the shared
// render) rather than three times.
//
// VALIDATES: AC-7 -- every written file is byte-identical.
// PREVENTS: a demonstration that changes on every render, so a review is of
// no known state.
func TestFleetDemoRenderIsDeterministic(t *testing.T) {
	first := testDemonstration(t)
	second := testDemonstration(t)

	written, err := render(t.Context(), first)
	require.NoError(t, err)
	_, err = render(t.Context(), second)
	require.NoError(t, err)

	// The sampled tour records two devices in full, one featured and one
	// ordinary. A device another recorded page links to (the states page's
	// gallery names one) gets a small chunk of followed answers, which is
	// the crawl working, not a third device of the plan.
	full := 0
	for _, chunk := range written.chunks {
		if !strings.HasPrefix(chunk.name, "device-") {
			continue
		}
		if chunk.exchanges > followedOnlyMax {
			full++
		}
	}
	assert.Equal(t, 2, full, "the sampled tour records one featured and one ordinary device in full: %v", written.chunks)

	files := outputFiles(t, first.output)
	assert.Equal(t, files, outputFiles(t, second.output), "two renders write the same files")
	for _, name := range files {
		one, readErr := os.ReadFile(filepath.Join(first.output, filepath.FromSlash(name)))
		require.NoError(t, readErr)
		two, readErr := os.ReadFile(filepath.Join(second.output, filepath.FromSlash(name)))
		require.NoError(t, readErr)
		assert.True(t, bytes.Equal(one, two), "%s differs between two renders of one commit", name)
	}
}

// TestFleetDemoRefusesAnOutputThatIsNotBuilt renders into an empty directory.
//
// VALIDATES: the render refuses an output with no built site, naming the file
// it lacks, and writes nothing.
// PREVENTS: a host page written where it cannot load the player.
func TestFleetDemoRefusesAnOutputThatIsNotBuilt(t *testing.T) {
	demo := testDemonstration(t)
	demo.output = t.TempDir()

	_, err := render(t.Context(), demo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not a built website output")
	assert.Empty(t, outputFiles(t, demo.output))
}
