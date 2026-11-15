// Design: docs/contributing/github-integration.md -- positive run action and unfinished states.

package github

import (
	"net/http"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// TestGitHubReportRunReachesAPI supplies a real selected run and drives the
// production action table, including the structured successful answer.
func TestGitHubReportRunReachesAPI(t *testing.T) {
	requests := 0
	client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if strings.HasSuffix(request.URL.Path, "/jobs") {
			return jsonResponse(t, request, map[string]any{"total_count": 1, "jobs": []apiJob{jobFixture(41)}}), nil
		}
		if !strings.HasSuffix(request.URL.Path, "/actions/runs/7") {
			t.Fatalf("run selector became %s", request.URL)
		}
		return jsonResponse(t, request, runFixture(7)), nil
	})
	payload, code := reportArea(func(args leaction.Arguments) (any, int) { return answerWith(client, args) }).Answer([]string{"report", "run", "7"})
	if code != 0 {
		t.Fatalf("run action returned %d", code)
	}
	report, ok := payload.(Report)
	if !ok {
		t.Fatalf("payload = %T", payload)
	}
	if !report.Complete {
		t.Fatalf("run action was incomplete: %+v", report)
	}
	if report.RequestedRunID != 7 {
		t.Error("selected run was lost")
	}
	if requests != 2 {
		t.Errorf("API requests = %d, want run then attempt jobs", requests)
	}
}

// TestGitHubUnfinishedStatesRemainDistinct reads each known unfinished state
// without converting it to a completed verdict or native proof.
func TestGitHubUnfinishedStatesRemainDistinct(t *testing.T) {
	for _, state := range []string{"queued", "in_progress", "requested", "waiting", "pending"} {
		t.Run(state, func(t *testing.T) {
			client := fixtureClient(t, func(request *http.Request) (*http.Response, error) {
				if strings.HasSuffix(request.URL.Path, "/jobs") {
					return jsonResponse(t, request, map[string]any{"total_count": 0, "jobs": []apiJob{}}), nil
				}
				run := runFixture(7)
				run.Status, run.Conclusion = state, nil
				return jsonResponse(t, request, run), nil
			})
			report, err := client.reportRun(t.Context(), 7)
			if err != nil {
				t.Fatal(err)
			}
			if report.Runs[0].Status != state {
				t.Error("unfinished state was flattened")
			}
			if report.Runs[0].Conclusion != "" {
				t.Error("unfinished run acquired a verdict")
			}
			if report.Qualification != reportQualification {
				t.Error("API observation lost its qualification")
			}
		})
	}
}
