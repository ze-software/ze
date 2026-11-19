//go:build ze_web && ze_fleet_ui

package sitefleetdemo

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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

// testDemonstration answers a demonstration over a scratch website output,
// holding the files of a built one, and the player of this checkout.
func testDemonstration(t *testing.T) demonstration {
	t.Helper()

	root, err := lepath.Root()
	require.NoError(t, err)
	output := t.TempDir()
	for name, content := range builtOutput {
		path := filepath.Join(output, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	pinned, err := time.Parse(time.RFC3339, testCommitDate)
	require.NoError(t, err)

	return demonstration{
		repository: root,
		output:     output,
		provenance: sitelgreplay.Provenance{
			Recorder: RenderCommand, Commit: testCommit, CommitDate: testCommitDate,
			Normalized: []sitelgreplay.Normalization{}, Varies: []string{},
		},
		clock: func() time.Time { return pinned },
	}
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

// TestFleetDemoRenderWritesTheDemonstration renders into a scratch website
// output at a pinned clock, then reads what was written.
//
// VALIDATES: AC-1 for the wiring phase -- the render writes the host page, the
// frame and the recording under tmp/fleet-demo/ and nothing else; the
// recording's entry is the estate dashboard of the sample estate; every style
// and script the page loads is in the recording; the host page is marked
// no-index and names the commit and its date.
// PREVENTS: a demonstration written where the site publishes it, and a frame
// whose page loads a file the player cannot answer.
func TestFleetDemoRenderWritesTheDemonstration(t *testing.T) {
	demo := testDemonstration(t)

	written, err := render(t.Context(), demo)
	require.NoError(t, err)

	expected := []string{
		"assets/lg-replay.js", "assets/site.css", "index.html",
		demoDirectory + "/" + frameFile, demoDirectory + "/" + hostFile, demoDirectory + "/" + recordingFile,
	}
	slices.Sort(expected)
	assert.Equal(t, expected, outputFiles(t, demo.output), "the render writes its three files and nothing else")
	for name, content := range builtOutput {
		kept, readErr := os.ReadFile(filepath.Join(demo.output, filepath.FromSlash(name)))
		require.NoError(t, readErr)
		assert.Equal(t, content, string(kept), "the render leaves %s alone", name)
	}
	assert.Len(t, written.files, 3)
	assert.Equal(t, 1, written.exchanges)

	recording, err := sitelgreplay.Read(filepath.Join(demo.output, filepath.FromSlash(demoDirectory), recordingFile))
	require.NoError(t, err)
	entry, found := recording.Exchange(recording.Entry)
	require.True(t, found, "the recording holds its entry")
	assert.Equal(t, entryPath, entry.Path)
	assert.False(t, entry.HTMX, "the entry is the whole page a browser opens")
	assert.Contains(t, entry.Body, sampleEstateName, "the entry is the sample estate's dashboard")
	assert.Equal(t, testCommit, recording.Provenance.Commit)

	linked, err := linkedAssets(entry.Body)
	require.NoError(t, err)
	assert.Contains(t, linked, "/assets/htmx.min.js", "the page's shared scripts are found")
	assert.Contains(t, linked, "/fleet/assets/fleet.js", "the page's own scripts are found")
	held := make([]string, 0, len(recording.Assets))
	for _, asset := range recording.Assets {
		assert.NotEmpty(t, asset.Body, asset.Path)
		held = append(held, asset.Path)
	}
	for _, path := range linked {
		assert.Contains(t, held, path, "the recording holds every file the page loads")
	}

	host, err := os.ReadFile(filepath.Join(demo.output, filepath.FromSlash(demoDirectory), hostFile))
	require.NoError(t, err)
	assert.Contains(t, string(host), `<meta name="robots" content="noindex, nofollow">`)
	assert.Contains(t, string(host), "recorded from commit 0123456789 of 2026-11-18.")
	assert.Contains(t, string(host), `data-lg-replay-frame="frame.html"`)
	assert.Contains(t, string(host), `src="../../assets/lg-replay.js"`, "the player is named relative to the page")

	frame, err := os.ReadFile(filepath.Join(demo.output, filepath.FromSlash(demoDirectory), frameFile))
	require.NoError(t, err)
	assert.Contains(t, string(frame), sampleEstateName, "the frame carries the recording inline")
}

// TestFleetDemoRenderIsDeterministic renders twice, into two scratch outputs,
// at the same pinned clock.
//
// VALIDATES: AC-7 -- every written file is byte-identical.
// PREVENTS: a demonstration that changes on every render, so a review is of
// no known state.
func TestFleetDemoRenderIsDeterministic(t *testing.T) {
	first := testDemonstration(t)
	second := testDemonstration(t)

	_, err := render(t.Context(), first)
	require.NoError(t, err)
	_, err = render(t.Context(), second)
	require.NoError(t, err)

	for _, name := range []string{hostFile, frameFile, recordingFile} {
		one, readErr := os.ReadFile(filepath.Join(first.output, filepath.FromSlash(demoDirectory), name))
		require.NoError(t, readErr)
		two, readErr := os.ReadFile(filepath.Join(second.output, filepath.FromSlash(demoDirectory), name))
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
