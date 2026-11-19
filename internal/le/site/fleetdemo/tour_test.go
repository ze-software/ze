//go:build ze_web && ze_fleet_ui

package sitefleetdemo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/component/web/fleetui/sample"
	sitelgreplay "github.com/ze-software/ze/internal/le/site/lgreplay"
)

// shared is the one render of the whole tour every test of the package that
// only reads it shares, since a render crawls the whole tour. TestMain
// removes its output.
var shared struct {
	once   sync.Once
	output string
	err    error
	// demo is what was rendered, and written what the render reported.
	demo      demonstration
	written   rendered
	recording sitelgreplay.Recording
	// exchanges are the frame's and every chunk's, by key.
	exchanges map[string]sitelgreplay.Exchange
	// chunks are each chunk file's exchanges, by chunk name.
	chunks map[string][]sitelgreplay.Exchange
}

func TestMain(m *testing.M) {
	code := m.Run()
	if shared.output != "" {
		os.RemoveAll(shared.output) //nolint:errcheck // a scratch directory of the test run
	}
	os.Exit(code)
}

// sharedRender answers the shared render, made on first use over a scratch
// website output at the test commit's clock.
func sharedRender(t *testing.T) {
	t.Helper()
	shared.once.Do(func() {
		shared.err = func() error {
			output, err := os.MkdirTemp("", "fleetdemo-")
			if err != nil {
				return err
			}
			shared.output = output
			shared.demo, err = newTestDemonstration(output)
			if err != nil {
				return err
			}
			// The render outlives the test that first asks for it, so it
			// runs under no test's context.
			shared.written, err = render(context.Background(), shared.demo)
			if err != nil {
				return err
			}
			directory := filepath.Join(output, filepath.FromSlash(demoDirectory))
			shared.recording, err = sitelgreplay.Read(filepath.Join(directory, recordingFile))
			if err != nil {
				return err
			}
			shared.exchanges = map[string]sitelgreplay.Exchange{}
			shared.chunks = map[string][]sitelgreplay.Exchange{}
			for _, exchange := range shared.recording.Exchanges {
				shared.exchanges[exchange.Key] = exchange
			}
			for _, chunk := range shared.recording.Chunks {
				content, err := os.ReadFile(filepath.Join(directory, chunk.File)) //nolint:gosec // a file the render wrote
				if err != nil {
					return err
				}
				var held []sitelgreplay.Exchange
				if err := json.Unmarshal(content, &held); err != nil {
					return err
				}
				shared.chunks[chunk.Name] = held
				for _, exchange := range held {
					shared.exchanges[exchange.Key] = exchange
				}
			}
			return nil
		}()
	})
	require.NoError(t, shared.err)
}

// holds reports whether the shared render holds an answer under key.
func holds(key string) bool {
	_, found := shared.exchanges[key]
	return found
}

// testTour answers the tour over a new sample estate at the test clock, as a
// render plans it.
func testTour(t *testing.T) tour {
	t.Helper()
	pinned, err := time.Parse(time.RFC3339, testCommitDate)
	require.NoError(t, err)
	source := sample.New(func() time.Time { return pinned })
	t.Cleanup(source.Close)
	source.Hold()
	plan, err := planTour(t.Context(), plainSource{Source: source}, demoAccount, tourScope{})
	require.NoError(t, err)
	return plan
}

// TestFleetDemoTourCoversEveryRegisteredPage reads the render's every answer
// against the fleet package's tab registry.
//
// VALIDATES: AC-3 -- every registered tab of the estate, of each group node of
// the default tree view, of each device and of the administration, and the
// task list, holds an exchange as its tab link asks for it; and a tab added to
// the registry is reported missing until the tour reaches it.
// PREVENTS: a page registered later that the demonstration silently lacks.
func TestFleetDemoTourCoversEveryRegisteredPage(t *testing.T) {
	sharedRender(t)
	plan := testTour(t)

	kinds := map[fleetui.ObjectKind]int{}
	for _, object := range plan.objects {
		kinds[object.Kind]++
	}
	for _, kind := range []fleetui.ObjectKind{
		fleetui.ObjectEstate, fleetui.ObjectGroup, fleetui.ObjectDevice, fleetui.ObjectTasks, fleetui.ObjectAdmin,
	} {
		assert.Positive(t, kinds[kind], "the tour opens an object of kind %s", kind)
	}
	assert.Empty(t, unreached(plan.objects, fleetui.Tabs, holds), "every registered page is in the recording")

	added := func(kind fleetui.ObjectKind) []fleetui.Tab {
		tabs := fleetui.Tabs(kind)
		if kind == fleetui.ObjectDevice {
			tabs = append(tabs, fleetui.Tab{Name: "added", Title: "Added"})
		}
		return tabs
	}
	missing := unreached(plan.objects, added, holds)
	assert.Len(t, missing, kinds[fleetui.ObjectDevice], "a tab registered for devices is missing on every device")
	for _, key := range missing {
		assert.Contains(t, key, "/added/ htmx")
	}
}

// TestFleetDemoBoundsAreEnforced checks chunks at and past each bound.
//
// VALIDATES: R-6 -- a chunk past its exchanges or its bytes, and a whole past
// its total, is refused naming the bound, the chunk and the figure; one at
// each bound passes.
// PREVENTS: a tour that grows without bound into the website output.
func TestFleetDemoBoundsAreEnforced(t *testing.T) {
	limit := bounds{chunkExchanges: 2, chunkBytes: 10, totalBytes: 30}
	chunk := func(name string, keys, size int) sitelgreplay.Chunk {
		held := make([]string, keys)
		for index := range held {
			held[index] = name + "-" + string(rune('a'+index))
		}
		return sitelgreplay.Chunk{Name: name, File: name + ".json", Size: size, Keys: held}
	}

	sizes := map[string][]kindSize{
		"device-b": {{kind: kindSidePanel, exchanges: 1, bytes: 11}},
		wholeTour:  {{kind: kindPartialPage, exchanges: 4, bytes: 20}, {kind: kindFullPage, exchanges: 1, bytes: 11}},
	}
	require.NoError(t, limit.check([]sitelgreplay.Chunk{chunk("estate", 2, 10), chunk("admin", 2, 10)}, 10, sizes), "at every bound")

	err := limit.check([]sitelgreplay.Chunk{chunk("device-a", 3, 5)}, 0, sizes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chunkExchangesMax of 2: chunk device-a holds 3 exchanges")

	err = limit.check([]sitelgreplay.Chunk{chunk("device-b", 1, 11)}, 0, sizes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chunkBytesMax of 10: chunk device-b is 11 bytes; by kind of request: side panel fragment 1 answers 11 bytes",
		"the failure says what the chunk holds")

	err = limit.check([]sitelgreplay.Chunk{chunk("estate", 2, 10), chunk("admin", 2, 10)}, 11, sizes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "totalBytesMax of 30: the frame and its chunks are 31 bytes; "+
		"by kind of request: htmx partial page 4 answers 20 bytes, full page 1 answers 11 bytes", "the failure says what the whole holds")
}

// TestFleetDemoChunksMatchTheIndex reads every chunk file the render wrote
// against the index the frame carries.
//
// VALIDATES: every key a chunk lists is in its file, in order, every answer
// in the file is listed, and the listed size is the file's.
// PREVENTS: a frame that asks the host for a chunk that does not hold the
// answer, and shows the notice for a recorded page.
func TestFleetDemoChunksMatchTheIndex(t *testing.T) {
	sharedRender(t)
	directory := filepath.Join(shared.output, filepath.FromSlash(demoDirectory))

	for _, chunk := range shared.recording.Chunks {
		keys := make([]string, 0, len(shared.chunks[chunk.Name]))
		for _, exchange := range shared.chunks[chunk.Name] {
			keys = append(keys, exchange.Key)
		}
		assert.Equal(t, chunk.Keys, keys, "chunk %s lists exactly the answers of its file", chunk.Name)
		info, err := os.Stat(filepath.Join(directory, chunk.File))
		require.NoError(t, err)
		assert.Equal(t, int64(chunk.Size), info.Size(), "chunk %s names its file's size", chunk.Name)
	}
}

// TestFleetDemoRecordsTheHTMXFlagEachControlSends reads a sample of recorded
// requests against how the fleet pages' own controls send them.
//
// VALIDATES: a tab link, a tree branch and a side panel (the fragment its
// eye asks) are recorded as htmx asks them (HX-Request), a chart's data and a console command as the chart
// and terminal scripts fetch them (no HX-Request), and none under the other
// flag.
// PREVENTS: a recorded answer the player never matches, because the key holds
// the flag.
func TestFleetDemoRecordsTheHTMXFlagEachControlSends(t *testing.T) {
	sharedRender(t)
	entry, found := shared.exchanges[shared.recording.Entry]
	require.True(t, found)

	bothWays := func(what string, request sitelgreplay.Request) {
		t.Helper()
		assert.True(t, holds(requestKey(request)), "%s is recorded as its control sends it: %s", what, requestKey(request))
		request.HTMX = !request.HTMX
		assert.False(t, holds(requestKey(request)), "%s is not recorded under the other flag: %s", what, requestKey(request))
	}

	device := fleetui.Address{Kind: fleetui.ObjectDevice, Device: "edge-lon-01", Tab: fleetui.Tabs(fleetui.ObjectDevice)[1].Name}
	bothWays("a device's tab", pageRequest(device))

	branches := attributeTargets(lazyAttribute, entry.Body)
	require.NotEmpty(t, branches, "the entry's tree has a branch that loads on expansion")
	branch, err := targetRequest(branches[0], true)
	require.NoError(t, err)
	bothWays("a tree branch", branch)

	charts := attributeTargets(chartAttribute, entry.Body)
	require.NotEmpty(t, charts, "the estate dashboard has a chart")
	chart, err := targetRequest(charts[0], false)
	require.NoError(t, err)
	bothWays("a chart's data", chart)

	panel := fleetui.Address{Kind: fleetui.ObjectEstate, Tab: devicesTabName, View: fleetui.ViewParams{Row: "edge-lon-01"}}
	eye, err := targetRequest(fleetui.FragmentLink(fleetui.SidePanelFragment, panel, ""), true)
	require.NoError(t, err)
	bothWays("a device's side panel, as its eye asks it", eye)

	posted := 0
	for _, exchange := range shared.chunks["device-edge-lon-01"] {
		if exchange.Method == http.MethodPost && strings.HasPrefix(exchange.Path, consolePrefix) {
			posted++
			assert.False(t, exchange.HTMX, "the terminal posts a command with fetch, not htmx")
		}
	}
	assert.Positive(t, posted, "the console's suggested commands are recorded")
}

// TestFleetDemoRecordsPagesAsTheirPartialAnswer reads the entry and a device's
// tab out of the shared render.
//
// VALIDATES: a navigation is recorded as the region htmx asks for (the
// content and its out-of-band regions, with no header), and only the entry
// is the whole page, which a browser opens.
// PREVENTS: a recording that holds the whole shell for every navigation, so
// the frame swaps a second shell into the content region, and the header,
// the sidebar and the task panel are recorded thousands of times.
func TestFleetDemoRecordsPagesAsTheirPartialAnswer(t *testing.T) {
	sharedRender(t)
	const shellHeader = `<header class="fleet-header">`

	entry, found := shared.exchanges[shared.recording.Entry]
	require.True(t, found)
	assert.Contains(t, entry.Body, shellHeader, "the entry is the whole shell")

	device := fleetui.Address{Kind: fleetui.ObjectDevice, Device: "edge-lon-01", Tab: fleetui.Tabs(fleetui.ObjectDevice)[1].Name}
	tab, found := shared.exchanges[requestKey(pageRequest(device))]
	require.True(t, found, "a device's tab is recorded")
	assert.NotContains(t, tab.Body, shellHeader, "a tab is the region htmx swaps, not the whole shell")
	assert.Contains(t, tab.Body, `hx-swap-oob`, "a tab carries the regions the shell updates beside the content")

	for _, size := range shared.written.kinds {
		if size.kind == kindFullPage {
			assert.Equal(t, 1, size.exchanges, "only the entry is recorded as a whole page")
		}
	}
}

// TestFleetDemoReportsItsSizeByKind reads the shared render's report against
// the answers it recorded, and logs the figures.
//
// VALIDATES: the report counts every recorded answer and its bytes once, by
// kind of request, the largest first, in the whole and in each chunk; and it
// tells a page, a side panel, a tree branch, a chart's data and a console
// command apart.
// PREVENTS: a bound failure or a size that cannot be traced to what produced
// it, so a narrowing is guessed rather than measured.
func TestFleetDemoReportsItsSizeByKind(t *testing.T) {
	sharedRender(t)

	bodies := 0
	for _, exchange := range shared.exchanges {
		bodies += len(exchange.Body)
	}
	counted, bytes := 0, 0
	kinds := map[string]bool{}
	for index, size := range shared.written.kinds {
		t.Logf("%-28s %6d answers %10d bytes", size.kind, size.exchanges, size.bytes)
		counted += size.exchanges
		bytes += size.bytes
		kinds[size.kind] = true
		if index > 0 {
			assert.LessOrEqual(t, size.bytes, shared.written.kinds[index-1].bytes, "the kinds are the largest first")
		}
	}
	assert.Equal(t, len(shared.exchanges), counted, "every answer is counted once")
	assert.Equal(t, bodies, bytes, "every answer's body is counted once")
	for _, kind := range []string{kindFullPage, kindPartialPage, kindSidePanel, kindChart, kindConsole, kindFragment + "tree-branch"} {
		assert.True(t, kinds[kind], "the report counts the kind %q", kind)
	}

	for _, chunk := range shared.written.chunks {
		inChunk := 0
		for _, size := range chunk.kinds {
			inChunk += size.exchanges
		}
		assert.Equal(t, chunk.exchanges, inChunk, "chunk %s counts each of its answers once", chunk.name)
		t.Logf("chunk %-28s %6d answers %10d bytes", chunk.name, chunk.exchanges, chunk.size)
	}
}

// TestFleetDemoNarrowsOrdinaryDevices reads a featured and an ordinary
// device's chunks, and the estate's devices table, out of the shared render.
//
// VALIDATES: spec A-4 -- a featured device is recorded with its side panels
// and its table sorts, an ordinary one with neither; no page of the devices
// table is recorded at a page size other than the default.
// PREVENTS: a tour that records every view of every device and every page of
// every page size, past the size a static site carries.
func TestFleetDemoNarrowsOrdinaryDevices(t *testing.T) {
	sharedRender(t)
	plan := testTour(t)
	plain := fleetui.ParseViewParams(url.Values{})

	var featured, ordinary string
	for _, object := range plan.objects {
		if object.Kind != fleetui.ObjectDevice {
			continue
		}
		if plan.featured[object.Device] && featured == "" {
			featured = object.Device
		}
		if !plan.featured[object.Device] && ordinary == "" {
			ordinary = object.Device
		}
	}
	require.NotEmpty(t, featured)
	require.NotEmpty(t, ordinary)

	views := func(device string) (panels, sorts int) {
		name, err := chunkName("device", device)
		require.NoError(t, err)
		require.NotEmpty(t, shared.chunks[name], "the device %s is recorded", device)
		for _, exchange := range shared.chunks[name] {
			if exchangeKind(exchange) == kindSidePanel {
				panels++
			}
			if exchangeKind(exchange) != kindPartialPage {
				continue
			}
			page, err := addressOf(exchange.Path, exchange.Query)
			require.NoError(t, err)
			if page.View.Sort != plain.Sort {
				sorts++
			}
		}
		return panels, sorts
	}
	panels, sorts := views(featured)
	assert.Positive(t, panels, "the featured device %s shows its side panels", featured)
	assert.Positive(t, sorts, "the featured device %s shows its table sorts", featured)
	panels, sorts = views(ordinary)
	assert.Zero(t, panels, "the ordinary device %s shows no side panel", ordinary)
	assert.Zero(t, sorts, "the ordinary device %s shows no table sort", ordinary)

	for _, exchange := range shared.chunks[estateDevicesChunk] {
		if exchangeKind(exchange) != kindPartialPage {
			continue
		}
		page, err := addressOf(exchange.Path, exchange.Query)
		require.NoError(t, err)
		crossed := page.View.Page > 1 && page.View.Size != 0 && page.View.Size != fleetui.PageSizeDefault
		assert.False(t, crossed, "no page past the first is recorded at another page size: %s", exchange.Key)
	}
}

// TestFleetDemoRecordsStepsInsideAnObjectWithoutTheTree reads a featured
// device's recorded views out of the shared render.
//
// VALIDATES: a view the crawl followed from a view of the same device (a
// table sort) is recorded as the shell answers a navigation that stays on its
// object, without the tree rows, while a tab the tour asks from nowhere
// carries them.
// PREVENTS: a recording that holds the sidebar again for every sort, page
// and sub-tab of one object.
func TestFleetDemoRecordsStepsInsideAnObjectWithoutTheTree(t *testing.T) {
	sharedRender(t)
	plan := testTour(t)
	plain := fleetui.ParseViewParams(url.Values{})
	const treeRows = `id="fleet-tree" hx-swap-oob`

	featured := ""
	for _, object := range plan.objects {
		if object.Kind == fleetui.ObjectDevice && plan.featured[object.Device] {
			featured = object.Device
			break
		}
	}
	require.NotEmpty(t, featured)
	name, err := chunkName("device", featured)
	require.NoError(t, err)

	sorted := 0
	for _, exchange := range shared.chunks[name] {
		if exchangeKind(exchange) != kindPartialPage {
			continue
		}
		page, err := addressOf(exchange.Path, exchange.Query)
		require.NoError(t, err)
		if page.View.Sort == plain.Sort {
			continue
		}
		sorted++
		assert.NotContains(t, exchange.Body, treeRows, "the sort %s stays on its device", exchange.Key)
	}
	assert.Positive(t, sorted, "the featured device %s shows its table sorts", featured)

	device := fleetui.Address{Kind: fleetui.ObjectDevice, Device: featured, Tab: fleetui.Tabs(fleetui.ObjectDevice)[1].Name}
	tab, found := shared.exchanges[requestKey(pageRequest(device))]
	require.True(t, found, "the device's tab is recorded")
	assert.Contains(t, tab.Body, treeRows, "a tab asked from nowhere carries the tree")
}
