package fleetui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// testEstateName is the name of the test estate.
const testEstateName = "hub-lon-01"

// testUser is the account the page requests of the tests run as.
const testUser = "operator"

// testSource is a data source over the test estate of fixture_test.go. It
// answers the questions the shell asks: the clock, the estate, the devices,
// the declared tree views, the alarms, the tasks and the event subscription.
// It embeds the interface to stand in for a whole source, so a question no
// test planned for panics on the nil embedded value instead of answering
// something empty.
//
// It holds a Feed, so it is used through a pointer.
type testSource struct {
	Source
	feed  Feed
	tasks []Task
}

// newTestSource answers a source over the test estate.
func newTestSource() *testSource {
	return &testSource{tasks: testTasks()}
}

func (source *testSource) Now(context.Context) (time.Time, error) {
	return testNow, nil
}

func (source *testSource) Estate(context.Context) (Estate, error) {
	return Estate{Name: testEstateName}, nil
}

func (source *testSource) Devices(context.Context) ([]Device, error) {
	return testDevices(), nil
}

func (source *testSource) TreeViews(context.Context) ([]TreeView, error) {
	return []TreeView{}, nil
}

func (source *testSource) Alarms(context.Context) ([]Alarm, error) {
	return testAlarms(), nil
}

func (source *testSource) Tasks(context.Context) ([]Task, error) {
	return source.tasks, nil
}

// Task answers one task of the test estate, with no per-device result: the
// shell's task panel asks it of every row it lists.
func (source *testSource) Task(_ context.Context, id string) (TaskDetail, error) {
	for index := range source.tasks {
		if source.tasks[index].ID == id {
			return TaskDetail{Task: source.tasks[index]}, nil
		}
	}

	return TaskDetail{}, opererr.Wrap(ErrObjectUnknown, "No task is named "+id+".",
		"The test estate keeps no such task.", "ask for a task of testTasks")
}

func (source *testSource) Subscribe(context.Context) (*Subscription, error) {
	return source.feed.Subscribe()
}

// testAddress parses the address of a fleet page, which MUST name one.
func testAddress(t *testing.T, target string) Address {
	t.Helper()

	parsed, err := url.Parse(target)
	require.NoError(t, err)

	address, err := ParseAddress(parsed)
	require.NoError(t, err, "the test address %s must name a page", target)

	return address
}

// testRequest builds the request of the page at target over the test estate,
// with a page registry of its own, which starts empty.
func testRequest(t *testing.T, target string) *PageRequest {
	t.Helper()

	request, err := NewPageRequest(context.Background(), newTestSource(), testAddress(t, target), testUser)
	require.NoError(t, err)
	request.pages = &pageRegistry{}

	return request
}

// registerTestPage adds a page to a registry of a test. The global registry
// is never written by a test: a page family may hold the same kind and tab.
func registerTestPage(t *testing.T, registry *pageRegistry, page Page) {
	t.Helper()

	if registry.pages == nil {
		registry.pages = make(map[pageKey]Page)
	}
	registry.pages[pageKey{kind: page.Kind, tab: page.Tab}] = page
}

// sentencePage answers a page builder whose content is one sentence, drawn
// by the state hero, so a test finds the sentence in the answer.
func sentencePage(sentence string) PageBuilder {
	return func(context.Context, *PageRequest) (templ.Component, error) {
		return StateHero(StateHeroView{Kind: HeroNoData, Sentence: sentence}), nil
	}
}

// failingPage answers a page builder that fails with err.
func failingPage(err error) PageBuilder {
	return func(context.Context, *PageRequest) (templ.Component, error) {
		return nil, err
	}
}

// getFleet asks handler for target and answers the recorded response. header
// pairs are set on the request.
func getFleet(t *testing.T, handler http.Handler, target string, header ...string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	for index := 0; index+1 < len(header); index += 2 {
		request.Header.Set(header[index], header[index+1])
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}
