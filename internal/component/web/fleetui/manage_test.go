package fleetui

import (
	"bytes"
	"context"
	"encoding/csv"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The configuration texts of the manage tests: the version before, the one
// the hub holds, and the device's own edit of a diverged device.
const (
	manageVersionOld    = "a3f9"
	manageVersionNew    = "b2c4"
	manageVersionDevice = "c71e"
	manageConfigOld     = "bgp {\n    peer core {\n        hold-time 90;\n    }\n}\n"
	manageConfigNew     = "bgp {\n    peer core {\n        hold-time 180;\n    }\n}\n"
	manageConfigDevice  = "bgp {\n    peer core {\n        hold-time 30;\n        passive;\n    }\n}\n"
	manageRolloutPaused = "edge-refresh"
	manageRolloutDone   = "core-timers"
	managePauseReason   = "Batch 2 crossed its failure threshold: 1 of 2 devices failed."
	manageSummaryNew    = "Raise the hold time to 180 seconds"
	manageSummaryOld    = "First configuration"
	manageSummaryStart  = "Start edge-refresh"
)

// manageSource is the test source of fixture_source_test.go, answering also
// the configuration, rollout, audit and administration questions.
type manageSource struct {
	*testSource
}

// newManageSource answers a manage source over the test estate.
func newManageSource() *manageSource {
	return &manageSource{testSource: newTestSource()}
}

func (source *manageSource) DeviceConfig(_ context.Context, device string) (DeviceConfig, error) {
	return DeviceConfig{Device: device, Version: manageVersionNew, Text: manageConfigNew}, nil
}

func (source *manageSource) ConfigVersions(context.Context, string) ([]ConfigVersion, error) {
	return []ConfigVersion{
		{Version: manageVersionNew, At: testNow.Add(-time.Hour), Actor: "alice", Summary: manageSummaryNew, Result: ConfigVersionResultApplied},
		{Version: manageVersionOld, At: testNow.Add(-48 * time.Hour), Actor: "bob", Summary: manageSummaryOld, Result: ConfigVersionResultApplied},
	}, nil
}

func (source *manageSource) ConfigText(_ context.Context, device, version string) (string, error) {
	switch version {
	case manageVersionNew:
		return manageConfigNew, nil
	case manageVersionOld:
		return manageConfigOld, nil
	}

	return "", versionUnknown(device, version)
}

func (source *manageSource) Divergence(_ context.Context, device string) (Divergence, error) {
	if device != rsAms01 {
		return Divergence{}, DeviceUnknownError(device)
	}

	return Divergence{
		Device: rsAms01, HubVersion: manageVersionOld, HubText: manageConfigOld,
		DeviceVersion: manageVersionDevice, DeviceText: manageConfigDevice,
		Changes: 3, EditedAt: time.Date(2026, 11, 18, 2, 14, 0, 0, time.UTC),
	}, nil
}

func (source *manageSource) Rollouts(context.Context) ([]Rollout, error) {
	return []Rollout{manageRollout(manageRolloutPaused), manageRollout(manageRolloutDone)}, nil
}

// manageRollout answers one of the two rollouts of the tests.
func manageRollout(name string) Rollout {
	if name == manageRolloutPaused {
		return Rollout{
			Name: name, Target: "role:edge", State: RolloutStatePaused, Actor: "alice",
			Started: testNow.Add(-2 * time.Hour), BatchesTotal: 3, BatchesDone: 1, DevicesTotal: 3, DevicesDone: 1,
			PauseReason: managePauseReason,
		}
	}

	return Rollout{
		Name: name, Target: "role:core", State: RolloutStateComplete, Actor: "bob",
		Started: testNow.Add(-24 * time.Hour), BatchesTotal: 1, BatchesDone: 1, DevicesTotal: 1, DevicesDone: 1,
	}
}

func (source *manageSource) Rollout(_ context.Context, name string) (RolloutDetail, error) {
	if name == manageRolloutDone {
		return RolloutDetail{
			Rollout: manageRollout(name),
			Batches: []RolloutBatch{{Number: 1, State: RolloutStateComplete}},
			Devices: []RolloutDevice{{Device: coreLon01, Batch: 1, Result: RolloutResultApplied, Answer: "commit 7", Answered: true, At: testNow.Add(-23 * time.Hour)}},
		}, nil
	}
	if name != manageRolloutPaused {
		return RolloutDetail{}, DeviceUnknownError(name)
	}

	return RolloutDetail{
		Rollout: manageRollout(name),
		Batches: []RolloutBatch{
			{Number: 1, State: RolloutStateComplete}, {Number: 2, State: RolloutStatePaused}, {Number: 3, State: RolloutStatePending},
		},
		Devices: []RolloutDevice{
			{Device: edgeLon01, Batch: 1, Result: RolloutResultApplied, Answer: "commit 12", Answered: true, At: testNow.Add(-time.Hour)},
			{Device: cpe0113, Batch: 2, Result: RolloutResultRejected, Answer: "commit refused: unknown leaf", Answered: true, At: testNow.Add(-30 * time.Minute)},
			{Device: edgeLon02, Batch: 3, Result: RolloutResultWaiting},
		},
		PauseDevices: []string{cpe0113},
	}, nil
}

func (source *manageSource) AuditEntries(context.Context) ([]AuditEntry, error) {
	return []AuditEntry{
		{ID: "A-3", At: testNow.Add(-time.Hour), Actor: "alice", Event: "config-push", Target: edgeLon01, Devices: 1, Result: AuditResultSucceeded, Summary: manageSummaryNew, TaskID: "T-1"},
		{ID: "A-2", At: testNow.Add(-3 * time.Hour), Actor: "bob", Event: "rollout-start", Target: "role:edge", Devices: 3, Result: AuditResultStarted, Summary: manageSummaryStart},
		{ID: "A-1", At: testNow.Add(-48 * time.Hour), Actor: "bob", Event: "config-push", Target: edgeLon01, Devices: 1, Result: AuditResultSucceeded, Summary: manageSummaryOld},
	}, nil
}

func (source *manageSource) Templates(context.Context) ([]Template, error) {
	return []Template{{Name: "edge-base", Groups: []string{"edge"}, Devices: 2, Changed: testNow.Add(-24 * time.Hour), Actor: "alice"}}, nil
}

func (source *manageSource) AccessProfiles(context.Context) ([]AccessProfile, error) {
	return []AccessProfile{
		{Name: "admin", FleetMode: true, Console: true, Diagnostics: true, ConfigPush: true, Rollouts: true},
		{Name: "viewer", FleetMode: true},
	}, nil
}

func (source *manageSource) Settings(context.Context) (Settings, error) {
	return Settings{
		OfflineAlarmDelay: 7 * 24 * time.Hour, ResourceAlarmPercent: 90, ResourceAlarmDelay: 5 * time.Minute,
		FinishedTasksKept: 13, EventStreamsMax: 64,
	}, nil
}

func (source *manageSource) HistoryUsage(context.Context) (HistoryUsage, error) {
	return HistoryUsage{
		Retention: 30 * 24 * time.Hour, SizeBytes: 512 << 20, CapBytes: 2 << 30, Oldest: testNow.Add(-30 * 24 * time.Hour),
		Largest: []HistoryDeviceUsage{{Device: edgeLon01, SizeBytes: 64 << 20}},
	}, nil
}

func (source *manageSource) SavedFilters(context.Context, string) ([]SavedFilter, error) {
	return []SavedFilter{{Name: "London edges", Filter: "site:lon role:edge", Default: true}}, nil
}

// manageHandler answers the page handler over the manage source, with the
// pages this build registered.
func manageHandler() *fleetHandler {
	return &fleetHandler{source: newManageSource(), pages: &registeredPages}
}

// manageRequest builds the request of the page at target over the manage
// source.
func manageRequest(t *testing.T, target string) *PageRequest {
	t.Helper()

	request, err := NewPageRequest(context.Background(), newManageSource(), testAddress(t, target), testUser)
	require.NoError(t, err)

	return request
}

// manageBody asks the page handler for target and answers the body, which
// MUST come with 200.
func manageBody(t *testing.T, target string) string {
	t.Helper()

	recorder := getFleet(t, manageHandler(), target)
	require.Equal(t, http.StatusOK, recorder.Code, "page %s", target)

	return recorder.Body.String()
}

// TestManageConfigTabShowsStateAndHistory asks for an in-sync device's
// Config tab.
//
// VALIDATES: the state line, the configuration text and the history table
// with every version the hub holds.
// PREVENTS: a Config tab that renders the shell with an empty content.
func TestManageConfigTabShowsStateAndHistory(t *testing.T) {
	body := manageBody(t, "/fleet/device/"+edgeLon01+"/config/")

	assert.Contains(t, body, ConfigStateInSync.String())
	assert.Contains(t, body, "hold-time 180;")
	assert.Contains(t, body, manageSummaryNew)
	assert.Contains(t, body, manageSummaryOld)
	assert.Contains(t, body, "Roll back to this version")
}

// TestManageDivergenceRestoreWaitsForTheDiff builds the divergence of a
// diverged device with and without the opened difference.
//
// VALIDATES: AC-3 -- the banner, the diff with the hub on the left, and the
// destructive resolution disabled until the address carries the opened
// divergence row.
// PREVENTS: a control that discards the device's edit before anyone saw it.
func TestManageDivergenceRestoreWaitsForTheDiff(t *testing.T) {
	path := "/fleet/device/" + rsAms01 + "/config/"
	assert.Contains(t, manageBody(t, path), rsAms01+" was edited locally while out of the fleet")

	source := newManageSource()
	divergence, err := source.Divergence(context.Background(), rsAms01)
	require.NoError(t, err)

	closed := divergenceView(manageRequest(t, path), &divergence)
	assert.True(t, closed.Restore.Disabled, "restore is offered before the diff was opened")
	assert.NotEmpty(t, closed.Restore.Reason)
	assert.False(t, closed.Keep.Disabled)
	assert.Empty(t, closed.Diff.Rows)

	opened := divergenceView(manageRequest(t, path+"?sub="+divergenceSub), &divergence)
	assert.False(t, opened.Restore.Disabled)
	require.NotNil(t, opened.Restore.Confirm)
	assert.Equal(t, rsAms01, opened.Restore.Confirm.Expect)
	assert.Contains(t, opened.Diff.LeftTitle, manageVersionOld)
	assert.Contains(t, opened.Diff.RightTitle, manageVersionDevice)
	assert.NotEmpty(t, opened.Diff.Rows)
}

// TestManageOfflineConfigIsReadOnly asks for an offline device's Config tab.
//
// VALIDATES: AC-2 -- the banner says why the configuration is read-only and
// when the device was last seen, Edit is disabled with the reason, and the
// history stays readable.
// PREVENTS: an editor offered for a device the hub cannot reach.
func TestManageOfflineConfigIsReadOnly(t *testing.T) {
	body := manageBody(t, "/fleet/device/"+coreAms01+"/config/")

	assert.Contains(t, body, coreAms01+" is offline, so its configuration is read-only.")
	assert.Contains(t, body, "6 days ago")
	assert.Contains(t, body, "the hub cannot change its configuration until it connects again")
	assert.Contains(t, body, manageSummaryOld)
}

// TestManageRolloutsTableListsBoth asks for the estate's Rollouts tab.
//
// VALIDATES: the table lists both rollouts with their state words.
// PREVENTS: a table that drops a finished rollout.
func TestManageRolloutsTableListsBoth(t *testing.T) {
	body := manageBody(t, "/fleet/estate/rollouts/")

	assert.Contains(t, body, manageRolloutPaused)
	assert.Contains(t, body, manageRolloutDone)
	assert.Contains(t, body, RolloutStatePaused.String())
	assert.Contains(t, body, RolloutStateComplete.String())
	assert.Contains(t, body, "2 of 2 rollouts")
}

// TestManagePausedRolloutNamesTheCause asks for the paused rollout's page.
//
// VALIDATES: AC-5 -- the page states why it paused, links the device that
// caused it, and offers Resume and Abort.
// PREVENTS: a paused rollout whose reason the operator must look for.
func TestManagePausedRolloutNamesTheCause(t *testing.T) {
	body := manageBody(t, "/fleet/estate/rollouts/?id="+manageRolloutPaused)

	assert.Contains(t, body, "Batch 2 crossed its failure threshold")
	assert.Contains(t, body, "/fleet/device/"+cpe0113+"/dashboard/")
	assert.Contains(t, body, "Resume")
	assert.Contains(t, body, "Abort")
}

// TestManageRolloutPreviewCounts resolves two targets over the test estate.
//
// VALIDATES: AC-4 -- the connected count a target changes, the offline count
// it skips, and the batches of the connected devices.
// PREVENTS: a rollout that counts an offline device it cannot change.
func TestManageRolloutPreviewCounts(t *testing.T) {
	ams, err := rolloutPreview(testViews(), "site:ams", 50)
	require.NoError(t, err)
	assert.Equal(t, 1, ams.Connected)
	assert.Equal(t, 1, ams.Offline)
	require.Len(t, ams.Batches, 1)
	assert.Equal(t, rsAms01, ams.Batches[0].Devices)

	estate, err := rolloutPreview(testViews(), "", 50)
	require.NoError(t, err)
	assert.Equal(t, 6, estate.Connected)
	assert.Equal(t, 1, estate.Offline)
	assert.Len(t, estate.Batches, 2)
}

// TestManageAuditFilterNarrows asks for the audit page with a filter.
//
// VALIDATES: AC-6 -- exactly the matching entries, from the address.
// PREVENTS: a filter the audit page ignores.
func TestManageAuditFilterNarrows(t *testing.T) {
	body := manageBody(t, "/fleet/admin/audit/?q=actor%3Abob")

	assert.Contains(t, body, manageSummaryStart)
	assert.Contains(t, body, manageSummaryOld)
	assert.NotContains(t, body, manageSummaryNew)
	assert.Contains(t, body, "2 of 3 entries")
}

// TestManageAuditExportCSV asks the export for the filtered entries as CSV.
//
// VALIDATES: AC-6 -- the export has its content type, a header row, and the
// filtered entries newest first.
// PREVENTS: an export of the whole trail under a filtered page.
func TestManageAuditExportCSV(t *testing.T) {
	recorder := getFleet(t, newAuditExportHandler(newManageSource()), AuditExportPath+"?format=csv&q=actor%3Abob")
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/csv; charset=utf-8", recorder.Header().Get("Content-Type"))

	records, err := csv.NewReader(strings.NewReader(recorder.Body.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, auditColumns, records[0])
	assert.Equal(t, "A-2", records[1][0])
	assert.Equal(t, "A-1", records[2][0])
}

// TestManageAuditExportCSVRunsNoFormula exports entries whose text starts a
// spreadsheet formula.
//
// VALIDATES: each cell that starts with = + - @, a tab or a carriage return
// is written as text, prefixed with a single quote; other cells are kept as
// they are.
// PREVENTS: CSV injection, where a summary a device or an operator wrote
// runs as a formula on the machine of whoever opens the export.
func TestManageAuditExportCSVRunsNoFormula(t *testing.T) {
	entries := []AuditEntry{{
		ID: "A-9", At: testNow, Actor: "@bob", Event: "+event", Target: "-target",
		Devices: 1, Result: AuditResultSucceeded, Summary: `=HYPERLINK("http://example.com","x")`, TaskID: "\tT-1",
	}, {
		ID: "A-8", At: testNow, Actor: "alice", Event: "config-push", Target: edgeLon01,
		Devices: 1, Result: AuditResultSucceeded, Summary: "\rcarried", TaskID: "T-2",
	}}

	var body bytes.Buffer
	require.NoError(t, writeAuditCSV(&body, entries))

	records, err := csv.NewReader(&body).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, "'@bob", records[1][2])
	assert.Equal(t, "'+event", records[1][3])
	assert.Equal(t, "'-target", records[1][4])
	assert.Equal(t, `'=HYPERLINK("http://example.com","x")`, records[1][7])
	assert.Equal(t, "'\tT-1", records[1][8])
	assert.Equal(t, "alice", records[2][2], "a cell that starts no formula is kept")
	assert.Equal(t, "'\rcarried", records[2][7])
}

// TestManageAdminPagesAnswer asks for every administration page of the
// family.
//
// VALIDATES: each page answers 200 with its table's own content.
// PREVENTS: an administration page that is registered and fails to build.
func TestManageAdminPagesAnswer(t *testing.T) {
	pages := map[string]string{
		tabAdminDevices:   "Declared devices",
		tabAdminLabels:    "route-server",
		tabAdminTreeViews: "Levels, in order",
		tabAdminFilters:   "London edges",
		tabAdminTemplates: "edge-base",
		tabAdminAccess:    "Relayed diagnostics",
		tabAdminHistory:   "Devices using most",
		tabAdminSettings:  "Offline alarm delay",
		tabAdminAudit:     "Audit trail",
	}
	for tab, content := range pages {
		t.Run(tab, func(t *testing.T) {
			assert.Contains(t, manageBody(t, "/fleet/admin/"+tab+"/"), content)
		})
	}
}

// TestManageDeviceJoinHidesTheSecret asks for the panel of a device being
// added.
//
// VALIDATES: AC-8 -- the panel shows the block the device needs to join, and
// the secret only as a placeholder.
// PREVENTS: a secret shown again once it is set.
func TestManageDeviceJoinHidesTheSecret(t *testing.T) {
	body := manageBody(t, "/fleet/admin/devices/?id=edge-new-01")

	assert.Contains(t, body, "client edge-new-01 {")
	assert.Contains(t, body, "the secret set for edge-new-01 on the hub")
}

// TestManageActionAnswersThePrototypeToast posts a roll back to the manage
// action.
//
// VALIDATES: an action answers the out-of-band toast that names it, and an
// action nobody declared answers an error toast.
// PREVENTS: an action that looks done in a prototype that changes nothing.
func TestManageActionAnswersThePrototypeToast(t *testing.T) {
	target := actionPath + manageActionName + "/device/" + edgeLon01 + "/config/"

	done := postAction(t, newManageSource(), target, url.Values{paramItem: {"rollback" + manageActionSep + edgeLon01}})
	assert.Contains(t, done, prototypeNothing)
	assert.Contains(t, done, "hx-swap-oob")
	assert.Contains(t, done, "Roll back to this version on "+edgeLon01)

	unknown := postAction(t, newManageSource(), target, url.Values{paramItem: {"explode" + manageActionSep + "all"}})
	assert.Contains(t, unknown, "has no action named")
}

// TestManageDiffLines diffs two short texts.
//
// VALIDATES: common lines are same, a replaced line is changed, an extra
// line is added, each with its line numbers.
// PREVENTS: a diff that misaligns the two columns.
func TestManageDiffLines(t *testing.T) {
	rows := DiffLines("a\nb\nc\n", "a\nx\nc\nd\n")

	require.Len(t, rows, 4)
	assert.Equal(t, DiffRowView{Left: "a", Right: "a", LeftNumber: 1, RightNumber: 1, Kind: DiffSame}, rows[0])
	assert.Equal(t, DiffRowView{Left: "b", Right: "x", LeftNumber: 2, RightNumber: 2, Kind: DiffChanged}, rows[1])
	assert.Equal(t, DiffRowView{Left: "c", Right: "c", LeftNumber: 3, RightNumber: 3, Kind: DiffSame}, rows[2])
	assert.Equal(t, DiffRowView{Right: "d", RightNumber: 4, Kind: DiffAdded}, rows[3])
}

// TestManageActionQuotesAtMostAnEcho posts a manage action item whose key
// and target are far longer than a toast quotes.
//
// VALIDATES: the toast quotes the key and the target cut by echoText.
// PREVENTS: a toast that echoes the whole posted item.
func TestManageActionQuotesAtMostAnEcho(t *testing.T) {
	long := strings.Repeat("x", 4096)
	echo := strings.Repeat("x", echoMax) + echoEllipsis

	unknown, err := manageActionAnswer(t.Context(), nil, url.Values{paramItem: {long}})
	require.NoError(t, err)
	assert.Contains(t, unknown.Message, echo)
	assert.Less(t, len(unknown.Message), 200)

	for key := range manageActions {
		known, err := manageActionAnswer(t.Context(), nil, url.Values{paramItem: {key + manageActionSep + long}})
		require.NoError(t, err)
		assert.Contains(t, known.Detail, echo)
		assert.NotContains(t, known.Detail, echo[:echoMax]+"x")

		break
	}
}
