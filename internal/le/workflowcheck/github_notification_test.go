// Design: docs/contributing/github-integration.md -- previews use trusted code and no delivery credential.
package workflowcheck

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	_ "github.com/ze-software/ze/internal/le/github" // Register the workflow's actual native owner in this isolated test binary.
)

// TestNotificationWorkflowReadsTrustedResultsOnly reads the real workflow
// structure and requires manual selection, default-branch code and a retained
// preview. The reported run is data and never selects executable code.
func TestNotificationWorkflowReadsTrustedResultsOnly(t *testing.T) {
	source := workflowSource(t, "notification-preview.yml")
	var document struct {
		On          map[string]yaml.Node `yaml:"on"`
		Permissions map[string]string    `yaml:"permissions"`
		Jobs        map[string]struct {
			If    string `yaml:"if"`
			Steps []struct {
				Uses string            `yaml:"uses"`
				With map[string]string `yaml:"with"`
				Run  string            `yaml:"run"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(source), &document); err != nil {
		t.Fatal(err)
	}
	if len(document.On) != 1 {
		t.Fatal("notification preview must have only the manual dispatch trigger")
	}
	if _, ok := document.On["workflow_dispatch"]; !ok {
		t.Fatal("notification preview has no manual dispatch")
	}
	for key, value := range document.Permissions {
		if value != "read" {
			t.Errorf("notification preview permission %s = %s, want read", key, value)
		}
	}
	if strings.Contains(source, "secrets.") {
		t.Fatal("preview workflow references a secret; Discord delivery is inactive")
	}
	job, ok := document.Jobs["preview"]
	if !ok {
		t.Fatal("notification preview has no preview job")
	}
	if job.If != "${{ github.ref == format('refs/heads/{0}', github.event.repository.default_branch) }}" {
		t.Error("preview workflow does not require dispatch from its default branch")
	}
	var checkout, rendered, retained bool
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") {
			checkout = true
			if step.With["ref"] != "${{ github.event.repository.default_branch }}" {
				t.Error("preview checks out a revision other than its trusted default branch")
			}
		}
		if strings.Contains(step.Run, "./le github preview") {
			rendered = true
			if !strings.Contains(step.Run, `./le github preview run "$SELECTED_RUN_ID" destination "$DESTINATION" '|' json > "$RUNNER_TEMP/notification-preview.json"`) {
				t.Error("preview command does not treat the selected run and destination as quoted data")
			}
		}
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			retained = true
			if step.With["retention-days"] != "14" {
				t.Error("preview retention must be 14 days")
			}
			if step.With["path"] != "${{ runner.temp }}/notification-preview.json" {
				t.Error("preview artifact is not the rendered candidate document")
			}
		}
	}
	if !checkout {
		t.Error("preview does not select trusted default-branch code")
	}
	if !rendered {
		t.Error("preview never reaches the registered renderer")
	}
	if !retained {
		t.Error("preview retains no review artifact")
	}
}
