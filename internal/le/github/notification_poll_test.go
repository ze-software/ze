// Design: docs/contributing/github-notifier.md -- bounded polling preserves delivery and scan history separately.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

type notificationPollFixture struct {
	activation             notificationActivationFixture
	config                 string
	rows                   []apiRun
	pages                  map[string][]apiRun
	changed                []apiRun
	details                map[uint64]apiRun
	jobs                   map[uint64][]apiJob
	posts, lists, requests int
	ambiguous              bool
	unrelated              []apiRun
	repositoryLists        int
	declaredTotal          int
}

func notificationPollTestFixture(t *testing.T) *notificationPollFixture {
	t.Helper()
	activation := notificationActivationTestFixture(t)
	notificationActivationTestReady(t, activation)
	fixture := &notificationPollFixture{activation: activation, config: filepath.Join(filepath.Dir(activation.config), "poll.json"),
		details: map[uint64]apiRun{}, jobs: map[uint64][]apiJob{}}
	fixture.writeConfig(t, map[string]any{"version": 1, "start-run-id": 101, "activation-configs": []string{activation.config}})
	return fixture
}

func (fixture *notificationPollFixture) writeConfig(t *testing.T, data map[string]any) {
	t.Helper()
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.config, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (fixture *notificationPollFixture) add(id uint64, conclusion string) apiRun {
	run := runFixture(id)
	run.WorkflowID, run.RunAttempt = 7, 1
	run.Conclusion = new(conclusion)
	job := jobFixture(id + 1000)
	job.RunID, job.RunAttempt, job.Name = id, 1, "verify shard 1"
	job.Conclusion = new(conclusion)
	job.Steps = []apiStep{{Number: 1, Name: "Verify", Status: "completed", Conclusion: new(conclusion)}}
	fixture.details[id], fixture.jobs[id] = run, []apiJob{job}
	return run
}

func (fixture *notificationPollFixture) transport(t *testing.T) http.RoundTripper {
	t.Helper()
	return notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
		fixture.requests++
		if request.URL.Host == "discord.com" {
			if request.Method == http.MethodGet {
				return notificationHTTPResponse(t, 200, `{"id":"456","channel_id":"123"}`), nil
			}
			fixture.posts++
			if fixture.ambiguous {
				return nil, errors.New("fixture lost acknowledgement")
			}
			return notificationHTTPResponse(t, 200, `{"id":"900","channel_id":"123"}`), nil
		}
		if request.URL.Host != "api.github.com" {
			t.Fatalf("unexpected origin %s", request.URL.Host)
		}
		if request.Method != http.MethodGet {
			t.Fatal("poller mutates GitHub")
		}
		prefix := "/repos/" + Repository + "/actions/runs"
		path := request.URL.Path
		var value any
		workflowPrefix := "/repos/" + Repository + "/actions/workflows/"
		isWorkflowList := strings.HasPrefix(path, workflowPrefix) && strings.HasSuffix(path, "/runs")
		if path == prefix || isWorkflowList {
			fixture.lists++
			if request.URL.Query().Get("branch") != "main" {
				t.Error("poll lost trusted branch filter")
			}
			if request.URL.Query().Get("per_page") != "100" {
				t.Error("poll lost page bound")
			}
			rows := fixture.rows
			if isWorkflowList {
				identity := strings.TrimSuffix(strings.TrimPrefix(path, workflowPrefix), "/runs")
				workflowID, err := strconv.ParseUint(identity, 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				rows = []apiRun{}
				for index := range fixture.rows {
					row := &fixture.rows[index]
					if row.WorkflowID == workflowID {
						rows = append(rows, *row)
					}
				}
			} else {
				fixture.repositoryLists++
				rows = append(append([]apiRun{}, fixture.unrelated...), rows...)
			}
			if fixture.changed != nil && fixture.lists > 1 {
				rows = fixture.changed
			}
			total := len(rows)
			if fixture.pages != nil {
				total = 0
				for _, page := range fixture.pages {
					total += len(page)
				}
				rows = fixture.pages[request.URL.Query().Get("page")]
			}
			if fixture.declaredTotal > 0 {
				total = fixture.declaredTotal
			}
			value = map[string]any{"total_count": total, "workflow_runs": rows}
		} else {
			parts := strings.Split(strings.TrimPrefix(path, prefix+"/"), "/")
			id, err := strconv.ParseUint(parts[0], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if len(parts) == 1 {
				value = fixture.details[id]
			} else {
				if len(parts) != 4 {
					t.Fatalf("unexpected path %s", path)
				}
				value = map[string]any{"total_count": len(fixture.jobs[id]), "jobs": fixture.jobs[id]}
			}
		}
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return notificationHTTPResponse(t, 200, string(body)), nil
	})
}

func (fixture *notificationPollFixture) answer(t *testing.T) int {
	t.Helper()
	action := leaction.New(area, notificationPollActions(fixture.transport(t))...)
	_, code := action.Answer([]string{"poll", "config", fixture.config})
	return code
}

func (fixture *notificationPollFixture) receipt(t *testing.T) notificationReceipt {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(fixture.activation.state, notificationStateFile))
	if err != nil {
		t.Fatal(err)
	}
	var receipt notificationReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		t.Fatal(err)
	}
	return receipt
}

// TestNotificationPollRegisteredCatchupSurvivesReopen proves the registered
// action catches up oldest first, ignores unchanged failure and terminal skip,
// delivers recovery once, and persists both histories through store reopening.
func TestNotificationPollRegisteredCatchupSurvivesReopen(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	fixture.rows = []apiRun{fixture.add(105, "success"), fixture.add(104, "skipped"), fixture.add(103, "failure"), fixture.add(102, "failure"), fixture.add(100, "success")}
	if code := fixture.answer(t); code != 0 {
		t.Fatalf("catch-up code = %d", code)
	}
	if fixture.posts != 2 {
		t.Fatalf("failure/recovery posts = %d, want 2", fixture.posts)
	}
	receipt := fixture.receipt(t)
	if receipt.ScanRunID != 105 || receipt.ScanAttempt != 1 {
		t.Errorf("scan cursor = %d/%d", receipt.ScanRunID, receipt.ScanAttempt)
	}
	if receipt.SeenRunID != 105 {
		t.Error("recovery lost delivered history")
	}
	if receipt.FailureDigest != "" {
		t.Error("recovery retained failure")
	}
	if code := fixture.answer(t); code != 0 {
		t.Fatalf("reopened catch-up code = %d", code)
	}
	if fixture.posts != 2 {
		t.Error("reopened polling repeated confirmed messages")
	}
}

// TestNotificationPollActiveEarlierRunBlocksLaterTransition requires a later
// completed run to wait until the earlier run's evidence is terminal and usable.
func TestNotificationPollActiveEarlierRunBlocksLaterTransition(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	later := fixture.add(103, "success")
	earlier := fixture.add(102, "failure")
	earlier.Status, earlier.Conclusion = "in_progress", nil
	fixture.details[102] = earlier
	fixture.rows = []apiRun{later, earlier, fixture.add(100, "success")}
	if code := fixture.answer(t); code == 0 {
		t.Error("pending earlier run reported completed catch-up")
	}
	if fixture.posts != 0 {
		t.Error("later run overtook pending earlier evidence")
	}
	fixture.rows[1] = fixture.add(102, "failure")
	if code := fixture.answer(t); code != 0 {
		t.Fatalf("completed catch-up code = %d", code)
	}
	if fixture.posts != 2 {
		t.Errorf("completed sequence posted %d messages, want 2", fixture.posts)
	}
}

// TestNotificationPollSkippedResultAdvancesOnlyScan preserves the delivered
// failure across every nonactionable completed result in the API vocabulary.
func TestNotificationPollSkippedResultAdvancesOnlyScan(t *testing.T) {
	var states apiStates
	if err := json.Unmarshal(apiStatesData, &states); err != nil {
		t.Fatal(err)
	}
	covered := 0
	for _, conclusion := range states.Conclusion {
		switch conclusion {
		case "success", "failure", "timed_out":
			continue
		}
		covered++
		t.Run(conclusion, func(t *testing.T) {
			fixture := notificationPollTestFixture(t)
			fixture.rows = []apiRun{fixture.add(101, "failure"), fixture.add(100, "success")}
			if code := fixture.answer(t); code != 0 {
				t.Fatal(code)
			}
			fixture.rows = []apiRun{fixture.add(102, conclusion), fixture.add(101, "failure"), fixture.add(100, "success")}
			if code := fixture.answer(t); code != 0 {
				t.Fatal(code)
			}
			receipt := fixture.receipt(t)
			if receipt.ScanRunID != 102 {
				t.Error("terminal result stranded scan cursor")
			}
			if receipt.SeenRunID != 101 {
				t.Error("terminal result changed delivered observation")
			}
			if receipt.FailureDigest == "" {
				t.Error("terminal result erased delivered failure")
			}
			if fixture.posts != 1 {
				t.Error("terminal result emitted an alert")
			}
		})
	}
	if covered == 0 {
		t.Fatal("API vocabulary supplied no nonactionable terminal cases")
	}
}

// TestNotificationPollDiscoveryRefusesBeforeDelivery denies inconsistent list
// evidence before any message or scan mutation, including snapshot changes.
func TestNotificationPollDiscoveryRefusesBeforeDelivery(t *testing.T) {
	for _, mode := range []string{"duplicate", "order", "identity", "snapshot", "attempt race", "missing boundary"} {
		t.Run(mode, func(t *testing.T) {
			fixture := notificationPollTestFixture(t)
			run := fixture.add(102, "failure")
			old := fixture.add(100, "success")
			fixture.rows = []apiRun{run, old}
			switch mode {
			case "duplicate":
				fixture.rows = []apiRun{run, run, old}
			case "order":
				fixture.rows = []apiRun{old, run}
			case "identity":
				fixture.rows[0].HeadSHA = "wrong"
			case "snapshot":
				fixture.changed = []apiRun{fixture.add(103, "failure"), run, old}
			case "attempt race":
				changed := run
				changed.RunAttempt = 2
				fixture.details[102] = changed
			case "missing boundary":
				fixture.declaredTotal = 1001
				fixture.pages = map[string][]apiRun{}
				for page := 1; page <= 10; page++ {
					fixture.pages[strconv.Itoa(page)] = []apiRun{fixture.add(uint64(200-page), "failure")}
				}
			}
			if code := fixture.answer(t); code == 0 {
				t.Error("unsafe discovery succeeded")
			}
			if fixture.posts != 0 {
				t.Error("unsafe discovery reached message POST")
			}
			if fixture.receipt(t).ScanRunID != 0 {
				t.Error("unsafe discovery advanced scan")
			}
		})
	}
}

// TestNotificationPollUnusableStateRefusesBeforeAPI proves polling never
// bootstraps missing state or hides an ambiguous attempted delivery.
func TestNotificationPollUnusableStateRefusesBeforeAPI(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt", "blocked"} {
		t.Run(mode, func(t *testing.T) {
			fixture := notificationPollTestFixture(t)
			path := filepath.Join(fixture.activation.state, notificationStateFile)
			switch mode {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "blocked":
				fixture.ambiguous = true
				fixture.rows = []apiRun{fixture.add(102, "failure"), fixture.add(100, "success")}
				if code := fixture.answer(t); code == 0 {
					t.Error("ambiguous send succeeded")
				}
				fixture.requests = 0
			}
			if code := fixture.answer(t); code == 0 {
				t.Error("unusable receipt succeeded")
			}
			if fixture.requests != 0 {
				t.Error("unusable receipt reached network")
			}
		})
	}
}

// TestNotificationPollPrivateConfigRefusesUnknownAndUnboundedInputs uses the
// actual parser/private-file boundary, requiring refusal before any API read.
func TestNotificationPollPrivateConfigRefusesUnknownAndUnboundedInputs(t *testing.T) {
	for _, mode := range []string{"zero start", "unknown key", "no workflows", "duplicate config", "too many", "relative", "public"} {
		t.Run(mode, func(t *testing.T) {
			fixture := notificationPollTestFixture(t)
			data := map[string]any{"version": 1, "start-run-id": 101, "activation-configs": []string{fixture.activation.config}}
			switch mode {
			case "zero start":
				data["start-run-id"] = 0
			case "unknown key":
				data["approved"] = true
			case "no workflows":
				data["activation-configs"] = []string{}
			case "duplicate config":
				data["activation-configs"] = []string{fixture.activation.config, fixture.activation.config}
			case "too many":
				data["activation-configs"] = make([]string, 7)
			case "relative":
				data["activation-configs"] = []string{"relative.json"}
			}
			fixture.writeConfig(t, data)
			if mode == "public" {
				if err := os.Chmod(fixture.config, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if code := fixture.answer(t); code == 0 {
				t.Error("invalid poll configuration succeeded")
			}
			if fixture.requests != 0 {
				t.Error("invalid configuration reached network")
			}
		})
	}
}

// TestNotificationPollProductionRegistration pins the real command table.
func TestNotificationPollProductionRegistration(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Verb == "poll" {
			return
		}
	}
	t.Fatal("production GitHub grammar does not register poll")
}

// TestNotificationPollQuietWorkflowDoesNotPinUnrelatedHistory drives two
// successful polls while a quiet workflow has no new event and more than1000
// other workflow runs accumulate. Its small own feed must not block recovery.
func TestNotificationPollQuietWorkflowDoesNotPinUnrelatedHistory(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	second := notificationActivationTestFixture(t)
	quietReport := notificationTestReport(99, false)
	quietReport.Runs[0].WorkflowID = 19
	quietReport.Runs[0].WorkflowPath = ".github/workflows/codeql.yml"
	activation, err := loadNotificationActivation(second.config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := activation.initialize(t.Context(), func(context.Context, uint64) (Report, error) { return quietReport, nil }, 99); err != nil {
		t.Fatal(err)
	}
	fixture.writeConfig(t, map[string]any{"version": 1, "start-run-id": 101, "activation-configs": []string{fixture.activation.config, second.config}})
	quiet := fixture.add(99, "success")
	quiet.WorkflowID, quiet.Path = 19, ".github/workflows/codeql.yml"
	fixture.details[99] = quiet
	fixture.rows = []apiRun{fixture.add(102, "failure"), fixture.add(100, "success"), quiet}
	if code := fixture.answer(t); code != 0 {
		t.Fatalf("first poll = %d", code)
	}
	for index := range 1001 {
		other := runFixture(2100 - uint64(index))
		other.WorkflowID, other.Path = 77, ".github/workflows/unrelated.yml"
		fixture.unrelated = append(fixture.unrelated, other)
	}
	fixture.rows = append([]apiRun{fixture.add(103, "success")}, fixture.rows...)
	if code := fixture.answer(t); code != 0 {
		t.Fatalf("poll after unrelated growth = %d", code)
	}
	if fixture.posts != 2 {
		t.Errorf("failure/recovery messages = %d, want 2", fixture.posts)
	}
	if fixture.repositoryLists != 0 {
		t.Error("polling scanned the unrelated repository-wide population")
	}
}

// TestNotificationPollLatestWorkflowAttemptRemainsEligible preserves a higher
// attempt of the latest real workflow run rather than advancing to unrelated IDs.
func TestNotificationPollLatestWorkflowAttemptRemainsEligible(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	fixture.rows = []apiRun{fixture.add(102, "failure"), fixture.add(100, "success")}
	if code := fixture.answer(t); code != 0 {
		t.Fatal(code)
	}
	rerun := fixture.add(102, "success")
	rerun.RunAttempt = 2
	fixture.details[102] = rerun
	fixture.jobs[102][0].RunAttempt = 2
	fixture.rows[0] = rerun
	if code := fixture.answer(t); code != 0 {
		t.Fatal(code)
	}
	if fixture.posts != 2 {
		t.Error("higher attempt of latest real run did not deliver recovery")
	}
	if fixture.receipt(t).ScanAttempt != 2 {
		t.Error("higher attempt lost scan identity")
	}
}

// TestNotificationPollWorkflowFeedRefusesAnotherIdentity binds the new feed
// endpoint to its initialized workflow ID/path before any candidate can send.
func TestNotificationPollWorkflowFeedRefusesAnotherIdentity(t *testing.T) {
	for _, mode := range []string{"id", "path"} {
		t.Run(mode, func(t *testing.T) {
			fixture := notificationPollTestFixture(t)
			raw := fixture.add(102, "failure")
			if mode == "id" {
				raw.WorkflowID = 19
			} else {
				raw.Path = ".github/workflows/codeql.yml"
			}
			fixture.pages = map[string][]apiRun{"1": {raw, fixture.add(100, "success")}}
			if code := fixture.answer(t); code == 0 {
				t.Error("foreign workflow feed succeeded")
			}
			if fixture.posts != 0 {
				t.Error("foreign workflow feed sent a message")
			}
		})
	}
}

// TestNotificationPollPaginatedSuffixKeepsOldestFailure proves the second
// discovery page is read before a failure from the first page is delivered.
func TestNotificationPollPaginatedSuffixKeepsOldestFailure(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	fixture.pages = map[string][]apiRun{"1": {fixture.add(102, "failure")}, "2": {fixture.add(100, "success")}}
	if code := fixture.answer(t); code != 0 {
		t.Fatalf("paginated poll code = %d", code)
	}
	if fixture.lists != 3 {
		t.Errorf("list reads = %d, want two pages and snapshot recheck", fixture.lists)
	}
	if fixture.posts != 1 {
		t.Error("paginated suffix lost its failure")
	}
}

// TestNotificationPollIncompleteWorkflowDoesNotBlockAnother requires an
// evidence gap to preserve its workflow cursor while an independent one sends.
func TestNotificationPollIncompleteWorkflowDoesNotBlockAnother(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	second := notificationActivationTestFixture(t)
	report := notificationTestReport(150, true)
	report.Runs[0].WorkflowID = 19
	report.Runs[0].WorkflowPath = ".github/workflows/codeql.yml"
	activation, err := loadNotificationActivation(second.config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := activation.initialize(t.Context(), func(context.Context, uint64) (Report, error) { return report, nil }, 150); err != nil {
		t.Fatal(err)
	}
	fixture.writeConfig(t, map[string]any{"version": 1, "start-run-id": 101, "activation-configs": []string{fixture.activation.config, second.config}})
	other := fixture.add(151, "failure")
	other.WorkflowID, other.Path = 19, ".github/workflows/codeql.yml"
	fixture.details[151] = other
	fixture.rows = []apiRun{other, fixture.add(102, "failure"), fixture.add(100, "success")}
	fixture.jobs[102] = []apiJob{}
	if code := fixture.answer(t); code == 0 {
		t.Error("incomplete workflow reported full catch-up")
	}
	if fixture.posts != 1 {
		t.Errorf("independent workflow posts = %d, want 1", fixture.posts)
	}
	if fixture.receipt(t).ScanRunID != 0 {
		t.Error("incomplete failure advanced scan")
	}
}

// TestNotificationPollContinuedErrorAdvancesScanWithoutRecovery preserves the
// original recovery rule when GitHub calls a continued-error workflow success.
func TestNotificationPollContinuedErrorAdvancesScanWithoutRecovery(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	fixture.rows = []apiRun{fixture.add(101, "failure"), fixture.add(100, "success")}
	if code := fixture.answer(t); code != 0 {
		t.Fatal(code)
	}
	continued := fixture.add(102, "failure")
	continued.Conclusion = new("success")
	fixture.details[102] = continued
	fixture.rows = []apiRun{continued, fixture.add(101, "failure"), fixture.add(100, "success")}
	if code := fixture.answer(t); code != 0 {
		t.Fatal(code)
	}
	receipt := fixture.receipt(t)
	if receipt.ScanRunID != 102 {
		t.Error("continued-error result stranded scan")
	}
	if receipt.SeenRunID != 101 {
		t.Error("continued-error result changed delivery history")
	}
	if receipt.FailureDigest == "" {
		t.Error("continued-error result erased failure")
	}
	if fixture.posts != 1 {
		t.Error("continued-error result falsely delivered recovery")
	}
}

// TestNotificationPollConfirmedSendBeforeScanDoesNotRepeat simulates a crash
// after confirmation persistence and before scan persistence, then reopens.
func TestNotificationPollConfirmedSendBeforeScanDoesNotRepeat(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	fixture.rows = []apiRun{fixture.add(102, "failure"), fixture.add(100, "success")}
	if code := fixture.answer(t); code != 0 {
		t.Fatal(code)
	}
	store, err := newFileNotificationStore(fixture.activation.state)
	if err != nil {
		t.Fatal(err)
	}
	writeErr := store.withLock(t.Context(), func(ledger notificationLedger) error {
		receipt, err := ledger.load()
		if err != nil {
			return err
		}
		receipt.ScanRunID, receipt.ScanAttempt = 0, 0
		return ledger.save(receipt)
	})
	closeErr := store.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if code := fixture.answer(t); code != 0 {
		t.Fatal(code)
	}
	if fixture.posts != 1 {
		t.Error("scan recovery repeated a confirmed POST")
	}
	if fixture.receipt(t).ScanRunID != 102 {
		t.Error("restart did not complete scan persistence")
	}
}

// TestNotificationPollReceiptRejectsPartialScanIdentity verifies the optional
// boundary accepts old receipts but never half of a new scan identity.
func TestNotificationPollReceiptRejectsPartialScanIdentity(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	for _, scan := range []struct{ id, attempt uint64 }{{0, 1}, {102, 0}} {
		receipt := fixture.receipt(t)
		receipt.ScanRunID, receipt.ScanAttempt = scan.id, scan.attempt
		if _, err := decodeNotificationReceipt(receipt); err == nil {
			t.Error("partial scan identity passed receipt validation")
		}
	}
}
