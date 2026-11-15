// Design: docs/contributing/github-integration.md -- entry-point wiring before API implementation.

package github

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

const testRevision = "0123456789abcdef0123456789abcdef01234567"

type fixtureTransport func(*http.Request) (*http.Response, error)

func (transport fixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

// TestGitHubReportRevisionReachesAPI drives the production action factory with
// a fixture transport and requires the selected revision to reach the reader.
// It is expected red while the wiring phase still returns its missing-reader error.
func TestGitHubReportRevisionReachesAPI(t *testing.T) {
	if leroot.LookupCommand(area) == nil {
		t.Fatal("the GitHub report area is not registered")
	}
	requests := 0
	client, err := NewClient(fixtureTransport(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.Host != "api.github.com" {
			t.Errorf("API host = %s", request.URL.Host)
		}
		if request.URL.Query().Get("head_sha") != testRevision {
			t.Errorf("selected revision did not reach the runs query: %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{},
			Body: io.NopCloser(strings.NewReader(`{"total_count":0,"workflow_runs":[]}`)), Request: request}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	payload, code := reportArea(func(args leaction.Arguments) (any, int) {
		return answerWith(client, args)
	}).Answer([]string{"report", "revision", testRevision})
	if requests == 0 {
		t.Fatal("registered revision selection never reached the GitHub API reader")
	}
	if code == 0 {
		t.Fatal("no matching runs must remain missing evidence, not success")
	}
	report, ok := payload.(Report)
	if !ok {
		t.Fatalf("payload = %T, want the structured report", payload)
	}
	if report.Revision != testRevision {
		t.Errorf("revision = %s", report.Revision)
	}
	if report.Complete {
		t.Error("an empty run set claimed complete evidence")
	}
}

// TestGitHubReportSelectorRequiresOneChoice proves the grammar does not choose
// a request on the operator's behalf when selectors are absent or combined.
func TestGitHubReportSelectorRequiresOneChoice(t *testing.T) {
	for _, args := range [][]string{{"report"}, {"report", "revision", testRevision, "run", "1"}, {"report", "run", "0"}} {
		_, code := Answer(args)
		if code != 2 {
			t.Errorf("%v answered %d, want usage refusal 2", args, code)
		}
	}
}

// TestGitHubReportActionErrorPreservesProducerGuidance checks the writer
// boundary retains existing guidance and qualifies a plain failure with a fix.
func TestGitHubReportActionErrorPreservesProducerGuidance(t *testing.T) {
	producer := opererr.New("selected run is unavailable", "Its result cannot be read.", "./le github report run 7")
	if got := reportActionError(producer, "fallback problem", "fallback reason", "./le help github report"); got != producer {
		t.Fatal("existing operator guidance was replaced or wrapped again")
	}
	cause := errors.New("reader configuration is unavailable")
	got := reportActionError(cause, "github report could not initialize its API reader",
		"The selected workflow evidence cannot be read.", "./le help github report")
	if !errors.Is(got, cause) {
		t.Error("plain failure lost its evidence")
	}
	if got.Problem() != "github report could not initialize its API reader" {
		t.Errorf("problem = %q", got.Problem())
	}
	if got.Why() != "The selected workflow evidence cannot be read." {
		t.Errorf("reason = %q", got.Why())
	}
	if got.Fix() != "./le help github report" {
		t.Errorf("fix = %q", got.Fix())
	}
	if !strings.HasSuffix(got.Error(), "\nfix: ./le help github report") {
		t.Errorf("fix is not the final line: %s", got)
	}
}
