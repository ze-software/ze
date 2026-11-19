// Design: docs/architecture/fleet-interface.md -- the side panel, task words, rollout results and state cells

package fleetui

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderMarkup answers the markup a component draws.
func renderMarkup(t *testing.T, component templ.Component) string {
	t.Helper()

	var markup bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &markup))

	return markup.String()
}

// sidePanelTag answers the opening tag of the side panel region of a page
// that has a side panel builder, at target.
func sidePanelTag(t *testing.T, target string) (tag, body string) {
	t.Helper()

	handler, registry := testHandler()
	registerTestPage(t, registry, Page{
		Kind: ObjectEstate, Tab: TabDashboard, Content: sentencePage("A page"), SidePanel: sentencePage("The details"),
	})
	answer := getFleet(t, handler, target)
	require.Equal(t, http.StatusOK, answer.Code)
	body = answer.Body.String()
	tag = regexp.MustCompile(`<aside class="fleet-side-panel"[^>]*>`).FindString(body)
	require.NotEmpty(t, tag, "the region is in the shell")

	return tag, body
}

// TestSidePanelWithoutRowIsTheEmptyRegion pins the markup the stylesheet
// draws only while the panel is pinned: with no row open the region is not
// hidden, says it is empty, and holds the sentence.
func TestSidePanelWithoutRowIsTheEmptyRegion(t *testing.T) {
	tag, body := sidePanelTag(t, pathEstate)

	assert.Contains(t, tag, `data-fleet-panel="empty"`)
	assert.NotRegexp(t, `\shidden[\s>=]`, tag, "the stylesheet, not the hidden attribute, decides whether it is drawn")
	assert.Contains(t, body, "Click the eye to see details")
	assert.NotContains(t, body, "The details")
}

// TestSidePanelWithRowIsTheOpenRegion pins the region a row opens: the
// details, and no sentence.
func TestSidePanelWithRowIsTheOpenRegion(t *testing.T) {
	tag, body := sidePanelTag(t, pathEstate+"?id="+coreLon01)

	assert.Contains(t, tag, `data-fleet-panel="open"`)
	assert.Contains(t, body, "The details")
	assert.NotContains(t, body, "Click the eye to see details")
}

// TestStylesheetDrawsTheEmptyPanelOnlyWhenPinned pins the stylesheet half of
// the contract: the empty region is not drawn, unless the root carries the
// pinned class.
func TestStylesheetDrawsTheEmptyPanelOnlyWhenPinned(t *testing.T) {
	sheet, err := os.ReadFile(filepath.Join("assets", "fleet-shell.css"))
	require.NoError(t, err)

	assert.Regexp(t, `(?m)^\.fleet-side-panel\[data-fleet-panel="empty"\] \{\s*display: none;`, string(sheet))
	assert.Regexp(t,
		`(?m)^\.fleet-panel-pinned \.fleet-side-panel\[data-fleet-panel="empty"\] \{\s*display: block;`, string(sheet))
}

// TestPanelPinControlCarriesBothIcons pins the pin control: both icons, the
// pressed state, and the name of what a press does.
func TestPanelPinControlCarriesBothIcons(t *testing.T) {
	cases := []struct {
		name   string
		pinned bool
		holds  []string
	}{
		{"unpinned", false, []string{`aria-pressed="false"`, `aria-label="Pin the side panel"`}},
		{"pinned", true, []string{`aria-pressed="true"`, `aria-label="Unpin the side panel"`}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			markup := renderMarkup(t, PanelContent(PanelView{Title: coreLon01, Pinned: test.pinned}))

			assert.Contains(t, markup, `data-fleet-panel-pin="true"`)
			assert.Contains(t, markup, "fleet-pin-icon--pin")
			assert.Contains(t, markup, "fleet-pin-icon--unpin")
			for _, text := range test.holds {
				assert.Contains(t, markup, text)
			}
		})
	}
}

// TestTaskKindWordsShowsAnUnknownKindByItsName pins the open vocabulary: a
// kind the table does not hold is shown as the hub names it.
func TestTaskKindWordsShowsAnUnknownKindByItsName(t *testing.T) {
	assert.Equal(t, "Config push", TaskKindWords("config-push"))
	assert.Equal(t, "firmware-flash", TaskKindWords("firmware-flash"))
}

// TestTaskRowShowsKindWordsAndTarget pins a task row: its name is the kind
// in words, and the devices it runs on follow as the target.
func TestTaskRowShowsKindWordsAndTarget(t *testing.T) {
	request := testRequest(t, pathEstate)
	task := Task{
		ID: "t1", Kind: "config-push", Target: "role:edge", State: TaskStateRunning,
		Devices: []string{edgeLon01, edgeLon02}, Started: testNow.Add(-time.Minute),
	}

	row, err := activityTaskRow(context.Background(), request, &task, "")
	require.NoError(t, err)

	assert.Equal(t, "Config push", row.Name)
	assert.Equal(t, "role:edge", row.Target)
	assert.Contains(t, renderMarkup(t, TaskRow(row)), "fleet-task-target")
}

// TestResultVocabulariesHaveATonePerValue pins every result vocabulary: each
// value has its own word and a tone, so none is drawn as a bare word.
func TestResultVocabulariesHaveATonePerValue(t *testing.T) {
	assert.Len(t, rolloutResultTones, len(rolloutResultWords))
	assert.Len(t, configVersionResultTones, len(configVersionResultWords))
	assert.Len(t, auditResultTones, len(auditResultWords))
	assert.Len(t, auditResultKeys, len(auditResultWords))

	for result := RolloutResultWaiting; int(result) < len(rolloutResultWords); result++ {
		status := RolloutResultStatus(result)
		assert.NotEqual(t, enumWordUnspecified, status.Word, "rollout result %d", result)
		assert.NotEqual(t, ToneUnspecified, status.Tone, "rollout result %d", result)
	}
	for result := ConfigVersionResultApplied; int(result) < len(configVersionResultWords); result++ {
		status := ConfigVersionResultStatus(result)
		assert.NotEqual(t, enumWordUnspecified, status.Word, "version result %d", result)
		assert.NotEqual(t, ToneUnspecified, status.Tone, "version result %d", result)
	}
	for result := AuditResultRecorded; int(result) < len(auditResultWords); result++ {
		status := AuditResultStatus(result)
		assert.NotEqual(t, enumWordUnspecified, status.Word, "audit result %d", result)
		assert.NotEqual(t, ToneUnspecified, status.Tone, "audit result %d", result)
	}
}

// TestResultTones pins the tone of each result a reader must tell apart.
func TestResultTones(t *testing.T) {
	cases := []struct {
		name string
		got  Status
		tone Tone
	}{
		{"rollout waiting", RolloutResultStatus(RolloutResultWaiting), ToneNeutral},
		{"rollout applied", RolloutResultStatus(RolloutResultApplied), ToneSuccess},
		{"rollout rejected", RolloutResultStatus(RolloutResultRejected), ToneError},
		{"version applied", ConfigVersionResultStatus(ConfigVersionResultApplied), ToneSuccess},
		{"version rejected", ConfigVersionResultStatus(ConfigVersionResultRejected), ToneError},
		{"version not fetched", ConfigVersionResultStatus(ConfigVersionResultNotFetched), ToneMuted},
		{"audit recorded", AuditResultStatus(AuditResultRecorded), ToneSuccess},
		{"audit paused", AuditResultStatus(AuditResultPaused), ToneWarning},
		{"audit failed", AuditResultStatus(AuditResultFailed), ToneError},
		{"audit partly failed", AuditResultStatus(AuditResultPartlyFailed), ToneWarning},
		{"adjacency up", AdjacencyStatus(&Adjacency{State: "full", Up: true}), ToneSuccess},
		{"adjacency down", AdjacencyStatus(&Adjacency{State: "down"}), ToneWarning},
		{"adjacency unknown", AdjacencyStatus(&Adjacency{}), ToneNeutral},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.tone, test.got.Tone)
		})
	}
}

// TestAuditResultCellIsAState pins the audit table: the result is drawn as
// a state, its word beside its mark.
func TestAuditResultCellIsAState(t *testing.T) {
	request := testRequest(t, pathEstate)
	entry := AuditEntry{ID: "A-1", At: testNow, Actor: "alice", Event: "config-push", Target: "role:edge", Result: AuditResultFailed}

	row := auditRow(request, &entry)

	require.Len(t, row.Cells, 6)
	assert.Equal(t, StatusCell(AuditResultStatus(AuditResultFailed)), row.Cells[4])
}

// TestRolloutPauseNamesTheOneDevice pins the callout of a pause one device
// caused: its link opens that device.
func TestRolloutPauseNamesTheOneDevice(t *testing.T) {
	request := testRequest(t, pathEstate)

	banner := rolloutPauseBanner(request, &Rollout{Name: "edge", PauseReason: "a device failed"}, []string{edgeLon01})

	require.NotNil(t, banner.Link)
	assert.Equal(t, edgeLon01, banner.Link.Label)
	assert.Equal(t, deviceTabHref(request, edgeLon01, tabDeviceDashboard), banner.Link.Href)
}

// TestRolloutPauseShowsTheDevicesThatCausedIt pins the callout of a pause
// several devices caused: its link opens the devices table holding exactly
// those devices.
func TestRolloutPauseShowsTheDevicesThatCausedIt(t *testing.T) {
	request := testRequest(t, pathEstate)
	causes := []string{edgeLon01, coreLon01}

	banner := rolloutPauseBanner(request, &Rollout{Name: "edge", PauseReason: "devices failed"}, causes)

	require.NotNil(t, banner.Link)
	assert.Equal(t, "Show them", banner.Link.Label)
	address := testAddress(t, banner.Link.Href)
	assert.Equal(t, ObjectEstate, address.Kind)
	answer, err := QueryDevices(testViews(), DeviceQuery{Filter: address.View.Filter, Page: 1, Size: 100})
	require.NoError(t, err)
	assert.ElementsMatch(t, causes, namesOf(answer.Rows))
}

// TestEstatePagesShowAddDevice pins the primary action of the estate's
// pages, and its absence on a group's.
func TestEstatePagesShowAddDevice(t *testing.T) {
	estate := testRequest(t, pathEstate)
	group := testRequest(t, pathDashboardGroup)

	for _, tab := range []string{TabDashboard, "devices", "tasks", "alarms"} {
		page, found := registeredPages.lookup(ObjectEstate, tab)
		require.True(t, found, "the estate has a %s page", tab)
		require.NotNil(t, page.Primary, "the estate %s page has a primary action", tab)
		action, shown := page.Primary(estate)
		assert.True(t, shown, tab)
		assert.Equal(t, "Add device", action.Label, tab)
	}

	page, found := registeredPages.lookup(ObjectGroup, TabDashboard)
	require.True(t, found)
	if page.Primary != nil {
		_, shown := page.Primary(group)
		assert.False(t, shown, "a group adds no device")
	}
}

// TestShortTagIsDrawnWhole pins the chip of a label of 18 characters: its
// whole text, with no truncation class or attribute. Whether the chip is
// wide enough for it is the stylesheet's, checked in a browser.
func TestShortTagIsDrawnWhole(t *testing.T) {
	markup := renderMarkup(t, Tag(TagView{Key: "site", Value: "london-dc-001"}))

	assert.Equal(t,
		`<span class="fleet-chip"><span class="fleet-chip-text" title="site=london-dc-001">site=london-dc-001</span> </span>`,
		markup)
}
