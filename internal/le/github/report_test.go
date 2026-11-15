// Design: docs/contributing/github-integration.md -- bounded, identified GitHub observations.

package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func runFixture(id uint64) apiRun {
	return apiRun{ID: id, WorkflowID: 19, Name: "verify", HeadSHA: testRevision, RunAttempt: 2,
		Event: "push", HeadBranch: "main", Path: ".github/workflows/verify.yml", Status: "completed", Conclusion: new("success"),
		Repository: &apiRepository{FullName: Repository}, HeadRepository: &apiRepository{FullName: Repository}}
}

func jobFixture(id uint64) apiJob {
	return apiJob{ID: id, RunID: 7, RunAttempt: 2, HeadSHA: testRevision, Name: "verify shard", Status: "completed", Conclusion: new("success"),
		Steps: []apiStep{{Number: 1, Name: "Run native actions", Status: "completed", Conclusion: new("success")}}}
}

func jsonResponse(t *testing.T, request *http.Request, value any) *http.Response {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body)), Request: request}
}

func fixtureClient(t *testing.T, body func(*http.Request) (*http.Response, error)) *Client {
	t.Helper()
	client, err := NewClient(fixtureTransport(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet {
			t.Errorf("method = %s", request.Method)
		}
		if request.URL.Scheme != "https" {
			t.Errorf("scheme = %s", request.URL.Scheme)
		}
		if request.URL.Host != "api.github.com" {
			t.Errorf("host = %s", request.URL.Host)
		}
		if !strings.HasPrefix(request.URL.Path, "/repos/"+Repository+"/") {
			t.Errorf("path = %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "" {
			t.Error("public reporting acquired credentials")
		}
		return body(request)
	}))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// TestGitHubRevisionReadsEveryRunAndAttemptJobPage proves nested pagination
// includes failed jobs and pending runs without equating complete reads to passes.
func TestGitHubRevisionReadsEveryRunAndAttemptJobPage(t *testing.T) {
	requests := []string{}
	client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.RequestURI())
		page := request.URL.Query().Get("page")
		if request.URL.Query().Get("per_page") != "100" {
			t.Error("page size is not bounded")
		}
		switch request.URL.Path {
		case "/repos/" + Repository + "/actions/runs":
			if request.URL.Query().Get("head_sha") != testRevision {
				t.Error("revision query lost its SHA")
			}
			row := runFixture(7)
			if page == "2" {
				row = runFixture(8)
				row.Status, row.Conclusion = "waiting", nil
			}
			return jsonResponse(t, request, map[string]any{"total_count": 2, "workflow_runs": []apiRun{row}}), nil
		case "/repos/" + Repository + "/actions/runs/7/attempts/2/jobs":
			row := jobFixture(41)
			if page == "2" {
				row.ID = 42
				row.Conclusion = new("failure")
			}
			return jsonResponse(t, request, map[string]any{"total_count": 2, "jobs": []apiJob{row}}), nil
		case "/repos/" + Repository + "/actions/runs/8/attempts/2/jobs":
			return jsonResponse(t, request, map[string]any{"total_count": 0, "jobs": []apiJob{}}), nil
		default:
			t.Fatalf("unexpected API path %s", request.URL)
			return nil, errors.New("unexpected path")
		}
	})
	report, err := client.reportRevision(t.Context(), testRevision)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Complete {
		t.Fatalf("report incomplete: %+v", report)
	}
	if len(report.Runs) != 2 {
		t.Fatalf("runs = %+v", report.Runs)
	}
	if len(report.Runs[0].Jobs) != 2 {
		t.Fatalf("jobs = %+v", report.Runs[0].Jobs)
	}
	if report.Runs[0].Jobs[1].Conclusion != "failure" {
		t.Error("failed shard was hidden")
	}
	if report.Runs[1].Status != "waiting" {
		t.Error("waiting run was hidden")
	}
	if report.Qualification != reportQualification {
		t.Error("report lost its evidence qualification")
	}
	if len(requests) != 5 {
		t.Errorf("requests = %v", requests)
	}
}

// TestGitHubRunIdentityAndStateFailures retains observations while refusing a
// different identity or unknown API semantics. Fork source repositories remain valid.
func TestGitHubRunIdentityAndStateFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*apiRun)
		want   string
	}{
		{name: "wrong run", change: func(run *apiRun) { run.ID = 8 }, want: "different run ID"},
		{name: "bad head", change: func(run *apiRun) { run.HeadSHA = "short" }, want: "full head"},
		{name: "wrong host repository", change: func(run *apiRun) { run.Repository.FullName = "someone/ze" }, want: "hosting repository"},
		{name: "missing attempt", change: func(run *apiRun) { run.RunAttempt = 0 }, want: "attempt"},
		{name: "missing workflow", change: func(run *apiRun) { run.WorkflowID = 0 }, want: "workflow ID"},
		{name: "unknown state", change: func(run *apiRun) { run.Status = "future" }, want: "unknown API status"},
		{name: "unknown verdict", change: func(run *apiRun) { run.Conclusion = new("future") }, want: "unknown API conclusion"},
		{name: "missing verdict", change: func(run *apiRun) { run.Conclusion = nil }, want: "no conclusion"},
		{name: "wrong URL", change: func(run *apiRun) { run.URL = "https://api.github.com/repos/other/ze/actions/runs/7" }, want: "different identity"},
	} {
		t.Run(test.name, func(t *testing.T) {
			run := runFixture(7)
			test.change(&run)
			client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
				if strings.HasSuffix(request.URL.Path, "/jobs") {
					return jsonResponse(t, request, map[string]any{"total_count": 0, "jobs": []apiJob{}}), nil
				}
				return jsonResponse(t, request, run), nil
			})
			report, err := client.reportRun(t.Context(), 7)
			if err == nil {
				t.Fatal("invalid API identity/state passed")
			}
			if report.Complete {
				t.Fatal("invalid report claims completeness")
			}
			if !strings.Contains(strings.Join(report.Problems, ";"), test.want) {
				t.Fatalf("problems = %v", report.Problems)
			}
		})
	}
	client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/jobs") {
			return jsonResponse(t, request, map[string]any{"total_count": 0, "jobs": []apiJob{}}), nil
		}
		run := runFixture(7)
		run.Event = "pull_request"
		run.HeadRepository = &apiRepository{FullName: "someone/ze"}
		return jsonResponse(t, request, run), nil
	})
	report, err := client.reportRun(t.Context(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if report.Runs[0].HeadRepository == nil {
		t.Fatal("fork source identity was lost")
	}
	if *report.Runs[0].HeadRepository != "someone/ze" {
		t.Error("fork source was replaced by the host repository")
	}
}

// TestGitHubJobsRefuseDifferentInputs checks run/attempt/SHA and step semantics
// where job data joins its selected parent, not only at the API-list caller.
func TestGitHubJobsRefuseDifferentInputs(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*apiJob)
		want   string
	}{
		{name: "wrong run", change: func(job *apiJob) { job.RunID = 8 }, want: "different run"},
		{name: "wrong attempt", change: func(job *apiJob) { job.RunAttempt = 1 }, want: "different attempt"},
		{name: "wrong head", change: func(job *apiJob) { job.HeadSHA = strings.Repeat("f", 40) }, want: "different head"},
		{name: "unknown job status", change: func(job *apiJob) { job.Status = "future" }, want: "unknown API status"},
		{name: "unknown step status", change: func(job *apiJob) { job.Steps[0].Status = "future" }, want: "unknown API status"},
		{name: "repeated step", change: func(job *apiJob) { job.Steps = append(job.Steps, job.Steps[0]) }, want: "repeated a step"},
		{name: "zero step", change: func(job *apiJob) { job.Steps[0].Number = 0 }, want: "positive number"},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := jobFixture(41)
			test.change(&job)
			client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
				if strings.HasSuffix(request.URL.Path, "/jobs") {
					return jsonResponse(t, request, map[string]any{"total_count": 1, "jobs": []apiJob{job}}), nil
				}
				return jsonResponse(t, request, runFixture(7)), nil
			})
			report, err := client.reportRun(t.Context(), 7)
			if err == nil {
				t.Fatal("foreign or unreadable job data passed")
			}
			if report.Runs[0].JobsComplete {
				t.Error("incomplete job input claimed completeness")
			}
			if !strings.Contains(strings.Join(report.Problems, ";"), test.want) {
				t.Fatalf("problems = %v", report.Problems)
			}
		})
	}
}

// TestGitHubAPIUnavailabilityAndBounds proves an unsuccessful or malformed
// response is not decoded as an empty passing collection or followed elsewhere.
func TestGitHubAPIUnavailabilityAndBounds(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		link   string
		want   string
	}{
		{name: "rate limited", status: 403, body: `{"secret":"must-not-appear"}`, want: "HTTP 403"},
		{name: "redirect", status: 302, want: "HTTP 302"},
		{name: "malformed", status: 200, body: "{", want: "decode"},
		{name: "too many", status: 200, body: `{"total_count":1001,"workflow_runs":[]}`, want: "bounded page"},
		{name: "missing array", status: 200, body: `{"total_count":0}`, want: "workflow_runs"},
		{name: "foreign next", status: 200, body: `{"total_count":0,"workflow_runs":[]}`, link: `<https://example.com/next>; rel="next"`, want: "leaves the API origin"},
		{name: "oversized", status: 200, body: strings.Repeat(" ", pageBytesMax+1), want: "page byte bound"},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
				requests++
				header := http.Header{"Location": {"https://example.com/not-read"}}
				if test.link != "" {
					header.Set("Link", test.link)
				}
				return &http.Response{StatusCode: test.status, Header: header, Body: io.NopCloser(strings.NewReader(test.body)), Request: request}, nil
			})
			report, err := client.reportRevision(t.Context(), testRevision)
			if err == nil {
				t.Fatal("unavailable evidence passed")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
			if strings.Contains(err.Error(), "must-not-appear") {
				t.Error("API body leaked into error")
			}
			if report.Complete {
				t.Error("failed read claimed completeness")
			}
			if requests != 1 {
				t.Errorf("requests = %d; redirect or extra page was followed", requests)
			}
		})
	}
}

// TestGitHubPaginationFailuresPreserveQualifiedRows refuses truncated,
// inconsistent and repeated pages while keeping the first observed identity.
func TestGitHubPaginationFailuresPreserveQualifiedRows(t *testing.T) {
	for _, kind := range []string{"empty", "count changed", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
				if strings.HasSuffix(request.URL.Path, "/jobs") {
					return jsonResponse(t, request, map[string]any{"total_count": 0, "jobs": []apiJob{}}), nil
				}
				count, rows := 2, []apiRun{runFixture(7)}
				if request.URL.Query().Get("page") == "2" {
					if kind == "empty" {
						rows = []apiRun{}
					}
					if kind == "count changed" {
						count = 3
					}
				}
				return jsonResponse(t, request, map[string]any{"total_count": count, "workflow_runs": rows}), nil
			})
			report, err := client.reportRevision(t.Context(), testRevision)
			if err == nil {
				t.Fatal("pagination lost its completeness qualification")
			}
			if len(report.Runs) != 1 {
				t.Fatalf("partial rows = %+v", report.Runs)
			}
			if report.Runs[0].ID != 7 {
				t.Error("known row identity was replaced")
			}
			if report.Complete {
				t.Error("partial pages claimed complete")
			}
		})
	}
}

// TestGitHubBoundsCoverTheOperation proves the shared budgets remain in force
// across page calls and a canceled context is never interpreted as no runs.
func TestGitHubBoundsCoverTheOperation(t *testing.T) {
	client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
		if err := request.Context().Err(); err != nil {
			return nil, err
		}
		return jsonResponse(t, request, map[string]any{"ok": true}), nil
	})
	read, cancel, err := client.start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	read.requests = requestsMax
	var result map[string]any
	if err := read.get("actions/runs", nil, &result); err == nil {
		t.Error("request bound was ignored")
	}
	read.requests, read.bytes = 0, operationBytesMax
	if err := read.get("actions/runs", nil, &result); err == nil {
		t.Error("operation byte bound was ignored")
	}
	ctx, stop := context.WithCancel(t.Context())
	stop()
	report, err := client.reportRun(ctx, 7)
	if err == nil {
		t.Fatal("canceled context returned evidence")
	}
	if report.Complete {
		t.Error("canceled context certified completeness")
	}
}

// TestGitHubReportUsesSharedFormats drives the real action factory and root
// renderer together. JSON, YAML and table retain the same qualified payload.
func TestGitHubReportUsesSharedFormats(t *testing.T) {
	client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/jobs") {
			return jsonResponse(t, request, map[string]any{"total_count": 0, "jobs": []apiJob{}}), nil
		}
		return jsonResponse(t, request, runFixture(7)), nil
	})
	handler := func(args command.ValidatedArgs) command.LocalAnswer {
		payload, code := reportArea(func(parsed leaction.Arguments) (any, int) { return answerWith(client, parsed) }).Answer(args.Tokens())
		return command.LocalAnswer{Payload: payload, Code: code}
	}
	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := leroot.Run(area, handler, []string{"report", "run", strconv.Itoa(7), "|", format}, &out, &errOut)
			if code != 0 {
				t.Fatalf("%s returned %d: %s", format, code, errOut.String())
			}
			if !strings.Contains(out.String(), Repository) {
				t.Errorf("%s lost repository: %s", format, out.String())
			}
			if !strings.Contains(out.String(), testRevision) {
				t.Errorf("%s lost revision: %s", format, out.String())
			}
		})
	}
}
