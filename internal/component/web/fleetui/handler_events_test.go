package fleetui

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The frames a stream writes for each set of regions the tests meet.
const (
	frameTree        = "event: stale\ndata: tree\n\n"
	frameTreeContent = "event: stale\ndata: tree,content\n\n"
	frameEverything  = "event: stale\ndata: tree,content,tasks,header\n\n"
)

// liveStream is one open event stream of a test: the source the events are
// published into, the tick that makes the stream write, and the frames it
// wrote. No test sleeps: a stream writes only when the test ticks.
type liveStream struct {
	source *testSource
	ticks  chan time.Time
	server *httptest.Server
	body   io.ReadCloser
	reader *bufio.Reader
}

// openStream starts a server over the stream handler and opens the stream of
// the page at scope. It answers once the stream holds its subscription, which
// the stream's opening comment says.
func openStream(t *testing.T, scope string) *liveStream {
	t.Helper()

	live := &liveStream{source: newTestSource(), ticks: make(chan time.Time)}

	handler := newEventStream(live.source)
	handler.ticker = func() (<-chan time.Time, func()) { return live.ticks, func() {} }
	live.server = httptest.NewServer(handler)
	t.Cleanup(live.close)

	response := live.get(t, scope)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))

	live.body = response.Body
	live.reader = bufio.NewReader(response.Body)
	require.Equal(t, frameOpen, live.frame(t), "the stream opens with a comment once it is subscribed")

	return live
}

// get asks the server for the stream of scope.
func (live *liveStream) get(t *testing.T, scope string) *http.Response {
	t.Helper()

	query := url.Values{}
	query.Set(scopeParam, scope)

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		live.server.URL+EventsPath+"?"+query.Encode(), http.NoBody)
	require.NoError(t, err)

	response, err := live.server.Client().Do(request)
	require.NoError(t, err)

	return response
}

// frame reads the next frame of the stream: every line up to the empty one.
func (live *liveStream) frame(t *testing.T) string {
	t.Helper()

	var frame strings.Builder
	for {
		line, err := live.reader.ReadString('\n')
		require.NoError(t, err, "the stream ended inside a frame: %q", frame.String())

		frame.WriteString(line)
		if line == "\n" {
			return frame.String()
		}
	}
}

// tick makes the stream write what went stale since the last tick.
func (live *liveStream) tick() {
	live.ticks <- time.Time{}
}

// close ends the stream from the browser's side and waits for its handler.
func (live *liveStream) close() {
	if live.body != nil {
		live.body.Close() //nolint:errcheck // the test is over; nothing reads the stream again
	}
	live.server.Close()
}

// publish hands one event of kind about device to the source's feed.
func (live *liveStream) publish(kind EventKind, device string) {
	live.source.feed.Publish(Event{Kind: kind, At: testNow, Device: device})
}

// TestEventStreamScopesTheContent publishes changes inside and outside the
// scope of a device page, a group page and the estate page.
//
// VALIDATES: a change of a device inside the scope names the content region
// and the tree; a change outside it names the tree only, which shows every
// device on every page, and leaves the content alone; a task event names
// every region.
// PREVENTS: a device page refetching its content for every change in the
// estate, and a tree that shows a stale state as live.
func TestEventStreamScopesTheContent(t *testing.T) {
	cases := []struct {
		name    string
		scope   string
		device  string
		expects string
	}{
		{"device page, the device itself", pathDevice, edgeLon01, frameTreeContent},
		{"device page, another device", pathDevice, coreAms01, frameTree},
		{"group page, a member", pathGroup, edgeLon02, frameTreeContent},
		{"group page, a device of another group", pathGroup, coreAms01, frameTree},
		{"estate page, any device", pathEstate, coreAms01, frameTreeContent},
		{"task list, any device", "/fleet/tasks/", coreAms01, frameTreeContent},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			live := openStream(t, test.scope)

			live.publish(EventDeviceChanged, test.device)
			live.tick()
			assert.Equal(t, test.expects, live.frame(t))

			live.publish(EventTaskProgressed, "")
			live.tick()
			assert.Equal(t, frameEverything, live.frame(t))
		})
	}
}

// TestEventStreamCoalescesABurst publishes several changes between two
// ticks.
//
// VALIDATES: a burst of events inside one interval is one frame naming each
// stale region once.
// PREVENTS: one refetch per event, which a flapping device would turn into a
// request storm.
func TestEventStreamCoalescesABurst(t *testing.T) {
	live := openStream(t, pathDevice)

	for range 5 {
		live.publish(EventDeviceChanged, edgeLon01)
	}
	live.publish(EventAlarmOpened, coreAms01)
	live.tick()
	assert.Equal(t, frameTreeContent, live.frame(t))

	// Had the burst been written as more than one frame, the next frame
	// read here would be another of them.
	live.publish(EventTaskEnded, "")
	live.tick()
	assert.Equal(t, frameEverything, live.frame(t))
}

// TestEventStreamEndsWithItsBrowser closes a stream from the browser's side.
//
// VALIDATES: a stream whose browser left gives its subscription back.
// PREVENTS: closed pages holding the feed's subscriptions until it refuses
// new ones.
func TestEventStreamEndsWithItsBrowser(t *testing.T) {
	live := openStream(t, pathEstate)

	live.source.feed.mu.Lock()
	open := len(live.source.feed.subscriptions)
	live.source.feed.mu.Unlock()
	require.Equal(t, 1, open)

	// Closing the server waits for the handler, which returns once the
	// request's context ends.
	live.close()

	live.source.feed.mu.Lock()
	left := len(live.source.feed.subscriptions)
	live.source.feed.mu.Unlock()
	assert.Zero(t, left)
}

// TestEventStreamTellsAnEndedPageToFetchAgain ends a stream's subscription
// from the feed's side, which is what the feed does to a subscription that
// falls behind and to every subscription when the source stops.
//
// VALIDATES: a stream whose subscription the feed ended names every region
// before it ends, so the page fetches them again and subscribes anew.
// PREVENTS: a page that lost events showing what it holds as current.
func TestEventStreamTellsAnEndedPageToFetchAgain(t *testing.T) {
	live := openStream(t, pathEstate)

	live.source.feed.Close()

	assert.Equal(t, frameEverything, live.frame(t))

	_, err := live.reader.ReadString('\n')
	assert.ErrorIs(t, err, io.EOF, "the stream ends after it said so")
}

// TestEventStreamRefusals asks for streams that cannot start.
//
// VALIDATES: a scope that names no page answers 400, a group no device is in
// answers 404, and a feed that already serves its maximum answers 503 with
// the data layer's own operator error.
// PREVENTS: an unbounded number of open streams.
func TestEventStreamRefusals(t *testing.T) {
	source := newTestSource()
	stream := newEventStream(source)

	bad := getFleet(t, stream, EventsPath+"?scope=/elsewhere/")
	assert.Equal(t, http.StatusBadRequest, bad.Code)

	none := getFleet(t, stream, EventsPath)
	assert.Equal(t, http.StatusBadRequest, none.Code)

	group := getFleet(t, stream, EventsPath+"?scope=/fleet/group/site=mars/dashboard/")
	assert.Equal(t, http.StatusNotFound, group.Code)

	for range EventSubscriptionsMax {
		_, err := source.feed.Subscribe()
		require.NoError(t, err)
	}
	full := getFleet(t, stream, EventsPath+"?scope="+pathEstate)
	assert.Equal(t, http.StatusServiceUnavailable, full.Code)
	requireRefusalText(t, full.Body.String())
}

// requireRefusalText asserts that an answer is an operator error: it ends
// with the line that names the fix.
func requireRefusalText(t *testing.T, body string) {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(body), "\n")
	require.NotEmpty(t, lines)
	assert.True(t, strings.HasPrefix(lines[len(lines)-1], "fix: "), "the last line names the fix, got %q", lines[len(lines)-1])
}

// TestEventScopeFollowsAGroupThatChanges changes which devices a group
// holds while a stream follows it.
//
// VALIDATES: a change of a device outside a group makes the group's content
// stale when the device joined the group since, and only then.
// PREVENTS: a group page that never learns of a device that moved into it.
func TestEventScopeFollowsAGroupThatChanges(t *testing.T) {
	ctx := context.Background()
	source := newTestSource()
	path := []GroupLevel{{Key: "site", Value: "lon"}}

	scope, err := newEventScope(ctx, source, Address{Kind: ObjectGroup, Group: path, Tab: TabDashboard})
	require.NoError(t, err)

	assert.Equal(t, regionTree, scope.stale(Event{Kind: EventDeviceChanged, Device: coreAms01}))
	assert.Zero(t, scope.settle(ctx, source), "the group holds the devices it held")
	assert.Zero(t, scope.settle(ctx, source), "with no change outside the group nothing is read again")

	// The stream believed the group smaller than it is: a device outside
	// its members changed, and reading the group again finds it inside.
	delete(scope.members, coreLon01)
	assert.Equal(t, regionTree, scope.stale(Event{Kind: EventDeviceChanged, Device: coreLon01}))
	assert.Equal(t, regionContent, scope.settle(ctx, source))
	assert.Contains(t, scope.members, coreLon01)
}

// stepScript is a Stepper over a fixed script of events. Past its end it
// refuses, as a source that stopped does.
type stepScript struct {
	events []Event
	held   bool
}

func (script *stepScript) Hold() {
	script.held = true
}

func (script *stepScript) Step() (Event, error) {
	if len(script.events) == 0 {
		return Event{}, opererr.Wrap(ErrFeedClosed, "The script has stopped.",
			"It holds no more events, so nothing was published.", "restart the review build")
	}

	next := script.events[0]
	script.events = script.events[1:]

	return next, nil
}

// TestStepRouteAnswersHtmxAToast posts to the step and hold routes as htmx
// does, from the states page.
//
// VALIDATES: asked by htmx, the step route answers the event's line and an
// out-of-band toast naming it, and the hold route a toast saying the script
// is held.
// PREVENTS: a states page whose controls change the estate with nothing on
// the page saying so.
func TestStepRouteAnswersHtmxAToast(t *testing.T) {
	script := &stepScript{events: []Event{{Kind: EventDeviceChanged, Device: edgeLon01}}}

	request := httptest.NewRequest(http.MethodPost, stepPath, http.NoBody)
	request.Header.Set(headerHTMXRequest, htmxTrue)
	stepped := httptest.NewRecorder()
	newStepHandler(script).ServeHTTP(stepped, request)

	assert.Equal(t, http.StatusOK, stepped.Code)
	assert.Contains(t, stepped.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, stepped.Body.String(), "device-changed "+edgeLon01)
	assert.Contains(t, stepped.Body.String(), "hx-swap-oob")
	assert.Contains(t, stepped.Body.String(), "The sample script published device-changed "+edgeLon01+".")

	request = httptest.NewRequest(http.MethodPost, holdPath, http.NoBody)
	request.Header.Set(headerHTMXRequest, htmxTrue)
	held := httptest.NewRecorder()
	newHoldHandler(script).ServeHTTP(held, request)

	assert.Contains(t, held.Body.String(), "The sample script is held.")
	assert.True(t, script.held)
}

// TestStepRouteAdvancesTheScript posts to the prototype-only step route.
//
// VALIDATES: each step request advances the script by one event and names
// the event it published; a script that stopped answers its operator error;
// the hold request holds the script and changes nothing else.
// PREVENTS: a browser test that waits on the wall clock for the estate to
// change.
func TestStepRouteAdvancesTheScript(t *testing.T) {
	script := &stepScript{events: []Event{
		{Kind: EventDeviceChanged, Device: edgeLon01},
		{Kind: EventTaskEnded},
	}}
	handler := newStepHandler(script)

	post := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, stepPath, http.NoBody))

		return recorder
	}

	first := post()
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, "device-changed "+edgeLon01+"\n", first.Body.String())

	second := post()
	assert.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, "task-ended\n", second.Body.String())

	stopped := post()
	assert.Equal(t, http.StatusServiceUnavailable, stopped.Code)
	requireRefusalText(t, stopped.Body.String())

	require.False(t, script.held)
	held := httptest.NewRecorder()
	newHoldHandler(script).ServeHTTP(held, httptest.NewRequest(http.MethodPost, holdPath, http.NoBody))
	assert.Equal(t, http.StatusOK, held.Code)
	assert.Equal(t, holdAnswer, held.Body.String())
	assert.True(t, script.held, "the hold route takes the script away from its timer")
}
