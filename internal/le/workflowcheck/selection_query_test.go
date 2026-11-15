// Design: docs/architecture/testing/ci-workflows.md -- evidence-preserving event selection

package workflowcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestVerifySelectionQueryRequiresUsableMatchingEvidence feeds actual API
// fixtures through gh's gojq engine and the exact query read from verify.yml.
// The only HTTP endpoint is this test's loopback server; real configuration
// and tokens are replaced for the subprocess. The complete response inventory
// lives in testdata/verify-pr-runs.json, preserving GitHub's wire vocabulary.
func TestVerifySelectionQueryRequiresUsableMatchingEvidence(t *testing.T) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		t.Fatal("GitHub CLI is required to test the workflow's query engine; install gh before rerunning this package")
	}
	script := workflowStepRun(t, "verify.yml", "select", "Select the verification event")
	queries := regexp.MustCompile(`--jq '([^']+)'`).FindAllStringSubmatch(script, -1)
	if len(queries) != 2 {
		t.Fatalf("event selection has %d queries, want the PR and verification queries", len(queries))
	}
	var fixtures []struct {
		Name     string          `json:"name"`
		OpenPRs  json.RawMessage `json:"open_prs"`
		Want     string          `json:"want"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join("testdata", "verify-pr-runs.json"))), &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("the PR verification response inventory is empty")
	}
	for _, test := range fixtures {
		t.Run(test.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				if _, err := writer.Write(test.Response); err != nil {
					t.Errorf("serve GitHub query fixture: %v", err)
				}
			}))
			t.Cleanup(server.Close)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, gh, "api", server.URL, "--jq", queries[1][1]) // #nosec G204 -- The installed CLI queries only this test's loopback fixture.
			command.Env = append(os.Environ(), "GH_CONFIG_DIR="+t.TempDir(), "GH_HOST=github.com",
				"GH_TOKEN=workflow-fixture", "GH_ENTERPRISE_TOKEN=workflow-fixture",
				"GH_NO_UPDATE_NOTIFIER=1", "GH_NO_EXTENSION_UPDATE_NOTIFIER=1", "GH_PROMPT_DISABLED=1", "GH_DEBUG=",
				"NO_PROXY=127.0.0.1,localhost", "no_proxy=127.0.0.1,localhost",
				"GITHUB_REPOSITORY=ze-software/ze", "GITHUB_SHA=abc123", "PR_NUMBERS="+string(test.OpenPRs))
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("GitHub CLI could not evaluate the real workflow query: %v\n%s", err, output)
			}
			if got := strings.TrimSpace(string(output)); got != test.Want {
				t.Errorf("matching verification count = %q, want %q", got, test.Want)
			}
		})
	}
}
