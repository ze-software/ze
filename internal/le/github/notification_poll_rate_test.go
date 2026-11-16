// Design: docs/contributing/github-notifier.md -- public polling spends requests only on reachable terminal evidence.
package github

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

func notificationPollRateAnswer(t *testing.T, config string, transport http.RoundTripper) (notificationPollReport, int, []string) {
	t.Helper()
	paths := []string{}
	counted := notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "api.github.com" {
			if request.Method != http.MethodGet {
				t.Fatal("poll attempted a GitHub mutation")
			}
			if request.Header.Get("Authorization") != "" {
				t.Fatal("public poll added authorization")
			}
			paths = append(paths, request.URL.Path)
		}
		return transport.RoundTrip(request)
	})
	action := leaction.New(area, notificationPollActions(counted)...)
	payload, code := action.Answer([]string{"poll", "config", config})
	report, ok := payload.(notificationPollReport)
	if !ok {
		t.Fatalf("poll payload = %T", payload)
	}
	return report, code, paths
}

func notificationPollRateReceipt(t *testing.T, activation notificationActivationFixture) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(activation.state, notificationStateFile))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func notificationPollRateSixFeeds(t *testing.T, active bool) (*notificationPollFixture, []notificationActivationFixture) {
	t.Helper()
	fixture := notificationPollTestFixture(t)
	activations := []notificationActivationFixture{fixture.activation}
	configs := []string{fixture.activation.config}
	paths := []string{"verify", "evidence-nightly", "qemu-nightly", "perf-nightly", "govulncheck", "codeql"}
	for index, path := range paths {
		workflowID := uint64(7 + index)
		if index != 0 {
			activation := notificationActivationTestFixture(t)
			report := notificationTestReport(uint64(90+index), false)
			report.Runs[0].WorkflowID = workflowID
			report.Runs[0].WorkflowPath = ".github/workflows/" + path + ".yml"
			authority, err := loadNotificationActivation(activation.config)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := authority.initialize(t.Context(), func(context.Context, uint64) (Report, error) { return report, nil }, report.RequestedRunID); err != nil {
				t.Fatal(err)
			}
			activations = append(activations, activation)
			configs = append(configs, activation.config)
		}
		id := uint64(90 + index)
		if active {
			id = uint64(110 + index)
		}
		raw := fixture.add(id, "success")
		raw.WorkflowID, raw.Path = workflowID, ".github/workflows/"+path+".yml"
		if active {
			raw.Status, raw.Conclusion = "in_progress", nil
		}
		fixture.rows = append(fixture.rows, raw)
	}
	fixture.writeConfig(t, map[string]any{"version": 1, "start-run-id": 101, "activation-configs": configs})
	return fixture, activations
}

// TestNotificationPollSixFeedsUseOnlyDiscovery proves six quiet or active
// feeds need exactly six public GETs and preserve every receipt byte on reopen.
func TestNotificationPollSixFeedsUseOnlyDiscovery(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(strconv.FormatBool(active), func(t *testing.T) {
			fixture, activations := notificationPollRateSixFeeds(t, active)
			before := make([][]byte, len(activations))
			for index, activation := range activations {
				before[index] = notificationPollRateReceipt(t, activation)
			}
			for range 2 {
				report, code, paths := notificationPollRateAnswer(t, fixture.config, fixture.transport(t))
				if active {
					if code == 0 {
						t.Fatal("active feeds claimed complete catch-up")
					}
					if report.State != "incomplete" {
						t.Fatal("active poll lost its incomplete qualification")
					}
					if len(report.Results) != 6 {
						t.Fatalf("active barriers = %d, want 6", len(report.Results))
					}
				} else {
					if code != 0 {
						t.Fatalf("quiet catch-up = %d", code)
					}
					if report.State != "complete" {
						t.Fatal("quiet feeds lost complete result")
					}
				}
				if len(paths) != 6 {
					t.Fatalf("GitHub GETs = %d, want 6", len(paths))
				}
				for _, path := range paths {
					if !strings.Contains(path, "/actions/workflows/") {
						t.Fatalf("quiet/active feed requested details: %s", path)
					}
				}
				for index, activation := range activations {
					if !bytes.Equal(before[index], notificationPollRateReceipt(t, activation)) {
						t.Error("quiet/active polling changed a receipt")
					}
				}
			}
			if fixture.posts != 0 {
				t.Fatal("quiet/active feeds posted a message")
			}
		})
	}
}

// TestNotificationPollCompletedBehindActiveCostsOnlyDiscovery proves an
// unreachable completed entry causes neither a snapshot recheck nor details.
func TestNotificationPollCompletedBehindActiveCostsOnlyDiscovery(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	active := fixture.add(102, "success")
	active.Status, active.Conclusion = "queued", nil
	fixture.rows = []apiRun{fixture.add(103, "failure"), active, fixture.add(100, "success")}
	before := notificationPollRateReceipt(t, fixture.activation)
	report, code, paths := notificationPollRateAnswer(t, fixture.config, fixture.transport(t))
	if code == 0 {
		t.Fatal("active barrier claimed complete catch-up")
	}
	if len(paths) != 1 {
		t.Fatalf("GitHub GETs = %d, want 1", len(paths))
	}
	if len(report.Results) != 1 {
		t.Fatalf("barrier results = %d, want 1", len(report.Results))
	}
	if report.Results[0].RunID != 102 {
		t.Fatal("later completed entry overtook the active barrier")
	}
	if !bytes.Equal(before, notificationPollRateReceipt(t, fixture.activation)) {
		t.Fatal("active barrier changed history")
	}
	if fixture.posts != 0 {
		t.Fatal("unreachable completion sent a message")
	}
}

// TestNotificationPollDiscoveredActiveBarrierSurvivesConcurrentScan advances
// the first workflow receipt while later feeds are discovered. The original
// poll must retain its barrier; a fresh completed poll must recheck and progress.
func TestNotificationPollDiscoveredActiveBarrierSurvivesConcurrentScan(t *testing.T) {
	fixture, activations := notificationPollRateSixFeeds(t, false)
	active := fixture.add(102, "success")
	active.Status, active.Conclusion = "in_progress", nil
	fixture.rows = append([]apiRun{fixture.add(103, "failure"), active}, fixture.rows...)
	authority, err := loadNotificationActivation(fixture.activation.config)
	if err != nil {
		t.Fatal(err)
	}
	next := fixture.transport(t)
	var advanced []byte
	transport := notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/workflows/12/runs") {
			if advanced == nil {
				store, err := newFileNotificationStore(fixture.activation.state)
				if err != nil {
					t.Fatal(err)
				}
				writeErr := store.withLock(t.Context(), func(ledger notificationLedger) error {
					ready, err := notificationPollReady(ledger, authority)
					if err != nil {
						return err
					}
					return ready.scan(ledger, 102, 1)
				})
				closeErr := store.Close()
				if writeErr != nil {
					t.Fatal(writeErr)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				advanced = notificationPollRateReceipt(t, fixture.activation)
			}
		}
		return next.RoundTrip(request)
	})
	before := make([][]byte, len(activations))
	for index, activation := range activations {
		before[index] = notificationPollRateReceipt(t, activation)
	}
	report, code, paths := notificationPollRateAnswer(t, fixture.config, transport)
	if advanced == nil {
		t.Fatal("controlled concurrent scan did not run")
	}
	if code == 0 {
		t.Fatal("concurrent scan removed the discovered active barrier")
	}
	if len(paths) != 6 {
		t.Fatalf("raced poll GETs = %d, want discovery only", len(paths))
	}
	if len(report.Results) != 1 {
		t.Fatal("raced poll reached an event after the discovered barrier")
	}
	if report.Results[0].RunID != 102 {
		t.Fatal("raced poll did not stop at its discovered active entry")
	}
	if report.Results[0].State != notificationWireBlocked {
		t.Fatal("concurrently scanned active entry became a quiet bypass")
	}
	if fixture.posts != 0 {
		t.Fatal("raced poll sent without a workflow snapshot recheck")
	}
	if !bytes.Equal(advanced, notificationPollRateReceipt(t, fixture.activation)) {
		t.Fatal("raced poll changed subsequent history after the concurrent scan")
	}
	for index := 1; index < len(activations); index++ {
		if !bytes.Equal(before[index], notificationPollRateReceipt(t, activations[index])) {
			t.Fatal("raced poll changed another receipt")
		}
	}
	fixture.rows[1] = fixture.add(102, "success")
	_, code, paths = notificationPollRateAnswer(t, fixture.config, fixture.transport(t))
	if code != 0 {
		t.Fatal("fresh completed poll did not progress")
	}
	if len(paths) != 9 {
		t.Fatalf("fresh poll GETs = %d, want six feeds, recheck, detail and jobs", len(paths))
	}
	if fixture.posts != 1 {
		t.Fatal("fresh completed poll lost the reachable failure")
	}
	if fixture.receipt(t).ScanRunID != 103 {
		t.Fatal("fresh completed poll did not retain scan progress")
	}
}

// TestNotificationPollFailureBeforeActiveThenRecovery proves reachable failure
// evidence still sends before an active barrier and later recovers exactly once.
func TestNotificationPollFailureBeforeActiveThenRecovery(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	active := fixture.add(102, "success")
	active.Status, active.Conclusion = "in_progress", nil
	fixture.rows = []apiRun{fixture.add(103, "success"), active, fixture.add(101, "failure"), fixture.add(100, "success")}
	report, code, paths := notificationPollRateAnswer(t, fixture.config, fixture.transport(t))
	if code == 0 {
		t.Fatal("active suffix claimed complete")
	}
	if len(paths) != 4 {
		t.Fatalf("GitHub GETs = %d, want listing, recheck, failure and jobs", len(paths))
	}
	if len(report.Results) != 2 {
		t.Fatal("reachable failure and active barrier were not both reported")
	}
	if report.Results[0].State != notificationWireDelivered {
		t.Fatal("reachable failure was not delivered")
	}
	if fixture.posts != 1 {
		t.Fatal("failure was lost or recovery overtook active evidence")
	}
	receipt := fixture.receipt(t)
	if receipt.ScanRunID != 101 {
		t.Fatal("scan crossed the active barrier")
	}
	if receipt.FailureDigest == "" {
		t.Fatal("confirmed failure history was lost")
	}
	fixture.rows[1] = fixture.add(102, "success")
	if code := fixture.answer(t); code != 0 {
		t.Fatal("terminal evidence did not resume catch-up")
	}
	if fixture.posts != 2 {
		t.Fatal("failure/recovery did not send exactly two messages")
	}
	receipt = fixture.receipt(t)
	if receipt.ScanRunID != 103 {
		t.Fatal("terminal suffix did not advance scan")
	}
	if receipt.FailureDigest != "" {
		t.Fatal("confirmed recovery retained failure")
	}
}

// TestNotificationPollMalformedActiveRefusesDiscovery ensures skipping details
// cannot let an active entry bypass feed identity, trust or state validation.
func TestNotificationPollMalformedActiveRefusesDiscovery(t *testing.T) {
	for _, mode := range []string{"status", "conclusion", "revision", "attempt", "head missing", "head foreign", "branch", "path", "url", "name"} {
		t.Run(mode, func(t *testing.T) {
			fixture := notificationPollTestFixture(t)
			raw := fixture.add(102, "success")
			raw.Status, raw.Conclusion = "in_progress", nil
			switch mode {
			case "status":
				raw.Status = "unknown"
			case "conclusion":
				raw.Conclusion = new("success")
			case "revision":
				raw.HeadSHA = "wrong"
			case "attempt":
				raw.RunAttempt = 0
			case "head missing":
				raw.HeadRepository = nil
			case "head foreign":
				raw.HeadRepository = &apiRepository{FullName: "other/repository"}
			case "branch":
				raw.HeadBranch = "other"
			case "path":
				raw.Path = ".github/workflows/other.yml"
			case "url":
				raw.HTMLURL = "https://github.com/other/repository/actions/runs/102"
			case "name":
				raw.Name = strings.Repeat("x", 161)
			}
			fixture.rows = []apiRun{raw, fixture.add(100, "success")}
			before := notificationPollRateReceipt(t, fixture.activation)
			report, code, paths := notificationPollRateAnswer(t, fixture.config, fixture.transport(t))
			if code == 0 {
				t.Fatal("malformed active feed succeeded")
			}
			if len(report.Results) != 0 {
				t.Fatal("malformed active feed reached processing")
			}
			if len(paths) != 1 {
				t.Fatalf("malformed feed GETs = %d, want discovery only", len(paths))
			}
			if !bytes.Equal(before, notificationPollRateReceipt(t, fixture.activation)) {
				t.Fatal("malformed active feed changed history")
			}
			if fixture.posts != 0 {
				t.Fatal("malformed active feed sent a message")
			}
		})
	}
}

// TestNotificationPollCompletedDetailBecomesActive refuses a completion race
// before requesting jobs, preserving the unscanned event and every receipt byte.
func TestNotificationPollCompletedDetailBecomesActive(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	raw := fixture.add(102, "failure")
	fixture.rows = []apiRun{raw, fixture.add(100, "success")}
	raw.Status, raw.Conclusion = "in_progress", nil
	fixture.details[102] = raw
	before := notificationPollRateReceipt(t, fixture.activation)
	_, code, paths := notificationPollRateAnswer(t, fixture.config, fixture.transport(t))
	if code == 0 {
		t.Fatal("completion race succeeded")
	}
	if len(paths) != 3 {
		t.Fatalf("GitHub GETs = %d, want listing, recheck and detail only", len(paths))
	}
	if !bytes.Equal(before, notificationPollRateReceipt(t, fixture.activation)) {
		t.Fatal("completion race changed history")
	}
	if fixture.posts != 0 {
		t.Fatal("completion race sent a message")
	}
}

// TestNotificationPollRequestBoundaryRetainsHistory exercises request 30 and
// refusal of request 31 in details or jobs, then resumes without repeating sends.
func TestNotificationPollRequestBoundaryRetainsHistory(t *testing.T) {
	for _, mode := range []string{"exact", "detail", "jobs", "job pagination"} {
		t.Run(mode, func(t *testing.T) {
			fixture := notificationPollTestFixture(t)
			last := uint64(114)
			if mode != "exact" {
				last = 115
			}
			for id := last; id >= 101; id-- {
				conclusion := "failure"
				if id == last {
					conclusion = "success"
				}
				fixture.rows = append(fixture.rows, fixture.add(id, conclusion))
			}
			old := fixture.add(100, "success")
			fixture.rows = append(fixture.rows, old)
			if mode == "jobs" {
				fixture.pages = map[string][]apiRun{"1": fixture.rows[:len(fixture.rows)-2], "2": fixture.rows[len(fixture.rows)-2:]}
			}
			transport := fixture.transport(t)
			if mode == "job pagination" {
				next := transport
				transport = notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
					if strings.HasSuffix(request.URL.Path, "/runs/114/attempts/1/jobs") {
						if request.URL.Query().Get("page") != "1" {
							t.Fatal("job page beyond request 30 reached the network")
						}
						return jsonResponse(t, request, map[string]any{"total_count": 2, "jobs": fixture.jobs[114]}), nil
					}
					return next.RoundTrip(request)
				})
			}
			report, code, paths := notificationPollRateAnswer(t, fixture.config, transport)
			if len(paths) != 30 {
				t.Fatalf("GitHub GETs = %d, want exact cap 30", len(paths))
			}
			receipt := fixture.receipt(t)
			if mode == "exact" {
				if code != 0 {
					t.Fatal("request 30 was refused")
				}
				if receipt.ScanRunID != 114 {
					t.Fatal("exact-boundary completion lost scan")
				}
				if fixture.posts != 2 {
					t.Fatal("exact-boundary failure/recovery lost delivery")
				}
				return
			}
			if code == 0 {
				t.Fatal("request 31 was permitted")
			}
			if report.State != "incomplete" {
				t.Fatal("budget refusal lost incomplete state")
			}
			if !strings.Contains(strings.Join(report.Problems, " "), "request bound") {
				t.Fatal("budget refusal did not name the exhausted request bound")
			}
			wantScan := uint64(114)
			if mode == "jobs" {
				wantScan = 113
			}
			if mode == "job pagination" {
				wantScan = 113
			}
			if receipt.ScanRunID != wantScan {
				t.Fatalf("scan = %d, want %d", receipt.ScanRunID, wantScan)
			}
			if receipt.SeenRunID != wantScan {
				t.Fatal("budget refusal changed confirmed observation")
			}
			if receipt.FailureDigest == "" {
				t.Fatal("budget refusal erased confirmed failure")
			}
			if receipt.FailureMessageID != "900" {
				t.Fatal("budget refusal lost confirmed message identity")
			}
			if receipt.Mode != notificationReceiptReady {
				t.Fatal("unsent candidate became an attempted delivery")
			}
			if fixture.posts != 1 {
				t.Fatal("budget refusal sent incomplete recovery")
			}
			fixture.pages = nil
			if code := fixture.answer(t); code != 0 {
				t.Fatal("budget refusal stranded unscanned history")
			}
			if fixture.posts != 2 {
				t.Fatal("resumption repeated failure or lost recovery")
			}
			if fixture.receipt(t).ScanRunID != last {
				t.Fatal("resumption did not finish retained history")
			}
		})
	}
}

// TestNotificationPollDiscoveryBudgetLeavesAllReceipts proves the 30-request
// budget spans workflow feeds and stops before any partially discovered send.
func TestNotificationPollDiscoveryBudgetLeavesAllReceipts(t *testing.T) {
	fixture, activations := notificationPollRateSixFeeds(t, true)
	before := make([][]byte, len(activations))
	for index, activation := range activations {
		before[index] = notificationPollRateReceipt(t, activation)
	}
	transport := notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
		for _, source := range fixture.rows {
			if !strings.HasSuffix(request.URL.Path, "/workflows/"+strconv.FormatUint(source.WorkflowID, 10)+"/runs") {
				continue
			}
			page, err := strconv.Atoi(request.URL.Query().Get("page"))
			if err != nil {
				t.Fatal(err)
			}
			source.ID = source.WorkflowID*1000 + uint64(10-page)
			source.Status, source.Conclusion = "completed", new("failure")
			if page == 6 {
				source.ID = source.WorkflowID + 80
			}
			return jsonResponse(t, request, map[string]any{"total_count": 6, "workflow_runs": []apiRun{source}}), nil
		}
		t.Fatal("discovery budget reached detail or delivery")
		return nil, errors.New("unexpected request beyond discovery")
	})
	report, code, paths := notificationPollRateAnswer(t, fixture.config, transport)
	if code == 0 {
		t.Fatal("partial discovery succeeded")
	}
	if report.State != "incomplete" {
		t.Fatal("partial discovery lost incomplete state")
	}
	if len(paths) != 30 {
		t.Fatalf("discovery GETs = %d, want 30", len(paths))
	}
	if len(report.Results) != 0 {
		t.Fatal("partial discovery started processing")
	}
	for index, activation := range activations {
		if !bytes.Equal(before[index], notificationPollRateReceipt(t, activation)) {
			t.Fatal("partial discovery changed a receipt")
		}
	}
}

// TestNotificationPollSnapshotBudgetLeavesAllReceipts proves all workflow
// rechecks share discovery's budget before any detail, send or scan mutation.
func TestNotificationPollSnapshotBudgetLeavesAllReceipts(t *testing.T) {
	fixture, activations := notificationPollRateSixFeeds(t, true)
	before := make([][]byte, len(activations))
	for index, activation := range activations {
		before[index] = notificationPollRateReceipt(t, activation)
	}
	transport := notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
		for _, source := range fixture.rows {
			if !strings.HasSuffix(request.URL.Path, "/workflows/"+strconv.FormatUint(source.WorkflowID, 10)+"/runs") {
				continue
			}
			page, err := strconv.Atoi(request.URL.Query().Get("page"))
			if err != nil {
				t.Fatal(err)
			}
			source.ID = source.WorkflowID*1000 + uint64(10-page)
			source.Status, source.Conclusion = "completed", new("failure")
			if page == 4 {
				source.ID = source.WorkflowID + 80
			}
			return jsonResponse(t, request, map[string]any{"total_count": 4, "workflow_runs": []apiRun{source}}), nil
		}
		t.Fatal("snapshot budget allowed details or delivery")
		return nil, errors.New("unexpected request beyond snapshots")
	})
	report, code, paths := notificationPollRateAnswer(t, fixture.config, transport)
	if code == 0 {
		t.Fatal("budget-exhausted processing succeeded")
	}
	if report.State != "incomplete" {
		t.Fatal("snapshot budget lost incomplete state")
	}
	if len(paths) != 30 {
		t.Fatalf("discovery and recheck GETs = %d, want 30", len(paths))
	}
	if len(report.Results) != 6 {
		t.Fatalf("budget-blocked workflows = %d, want 6", len(report.Results))
	}
	for index, activation := range activations {
		if !bytes.Equal(before[index], notificationPollRateReceipt(t, activation)) {
			t.Fatal("snapshot budget changed a receipt")
		}
	}
}

// TestNotificationPollSnapshotRequest31LeavesAllReceipts uses five pages for
// each of six feeds: request 31 is the first snapshot recheck, before processing.
func TestNotificationPollSnapshotRequest31LeavesAllReceipts(t *testing.T) {
	fixture, activations := notificationPollRateSixFeeds(t, true)
	before := make([][]byte, len(activations))
	for index, activation := range activations {
		before[index] = notificationPollRateReceipt(t, activation)
	}
	transport := notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
		for _, source := range fixture.rows {
			if !strings.HasSuffix(request.URL.Path, "/workflows/"+strconv.FormatUint(source.WorkflowID, 10)+"/runs") {
				continue
			}
			page, err := strconv.Atoi(request.URL.Query().Get("page"))
			if err != nil {
				t.Fatal(err)
			}
			source.ID = source.WorkflowID*1000 + uint64(10-page)
			source.Status, source.Conclusion = "completed", new("failure")
			if page == 5 {
				source.ID = source.WorkflowID + 80
			}
			return jsonResponse(t, request, map[string]any{"total_count": 5, "workflow_runs": []apiRun{source}}), nil
		}
		t.Fatal("snapshot refusal reached details or a POST")
		return nil, errors.New("unexpected request after snapshot refusal")
	})
	report, code, paths := notificationPollRateAnswer(t, fixture.config, transport)
	if code == 0 {
		t.Fatal("request 31 snapshot recheck succeeded")
	}
	if report.State != "incomplete" {
		t.Fatal("snapshot refusal lost incomplete state")
	}
	if len(paths) != 30 {
		t.Fatalf("discovery GETs = %d, want 30 before recheck refusal", len(paths))
	}
	if len(report.Results) != 0 {
		t.Fatal("snapshot refusal started processing")
	}
	for index, activation := range activations {
		if !bytes.Equal(before[index], notificationPollRateReceipt(t, activation)) {
			t.Fatal("snapshot refusal changed a receipt")
		}
	}
}

// TestNotificationPollBudgetDoesNotChangeReportBudget proves a poll-specific
// cap does not lower the independent public report operation's existing bound.
func TestNotificationPollBudgetDoesNotChangeReportBudget(t *testing.T) {
	fixture := notificationPollTestFixture(t)
	fixture.rows = []apiRun{fixture.add(102, "success"), fixture.add(100, "success")}
	transport := fixture.transport(t)
	client, err := NewClient(transport)
	if err != nil {
		t.Fatal(err)
	}
	poll, err := loadNotificationPoll(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := poll.run(t.Context(), client, transport); err != nil {
		t.Fatal(err)
	}
	read, cancel, err := client.start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	for range 200 {
		var raw apiRun
		if err := read.get("actions/runs/102", nil, &raw); err != nil {
			t.Fatal("poll changed the independent report request bound")
		}
		if !reflect.DeepEqual(raw, fixture.details[102]) {
			t.Fatal("report request did not return identified evidence")
		}
	}
	requests := fixture.requests
	var raw apiRun
	if err := read.get("actions/runs/102", nil, &raw); err == nil {
		t.Fatal("report lost its existing 200-request cap")
	}
	if fixture.requests != requests {
		t.Fatal("report request 201 reached the network")
	}
}

// TestNotificationPollHourlyTimerKeepsPersistence reads the shipped timer to
// bind request savings to its actual hourly schedule, persistence and jitter.
func TestNotificationPollHourlyTimerKeepsPersistence(t *testing.T) {
	body, err := os.ReadFile("../../../contrib/github-notifier/ze-github-notifier.timer")
	if err != nil {
		t.Fatal(err)
	}
	section := ""
	settings := map[string]string{}
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		if section != "[Timer]" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			if _, repeated := settings[key]; repeated {
				t.Fatalf("timer repeats setting %s", key)
			}
			settings[key] = value
		}
	}
	for key, value := range map[string]string{"OnCalendar": "hourly", "Persistent": "true", "RandomizedDelaySec": "20"} {
		if settings[key] != value {
			t.Fatalf("timer %s = %q, want %q", key, settings[key], value)
		}
	}
}
