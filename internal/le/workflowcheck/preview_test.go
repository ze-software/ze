package workflowcheck

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: the workflow gate resolves the public named-launcher grammar.
// PREVENTS: a named build being read as the unregistered command --name.
func TestNativeActionExtractorHandlesNamedLauncherBuilds(t *testing.T) {
	for _, prefix := range []string{"./le --name preview-ci", "./le --name=preview-ci", "./le --name Preview.CI_1", "./le --name 'preview-ci'", "./le --name=\"preview-ci\""} {
		source := prefix + " build preview run output /tmp/new-bundle images && " + prefix + " build preview check path /tmp/new-bundle\n"
		want := []string{"build preview/run", "build preview/check"}
		if got := nativeActionsIn(source); !slices.Equal(got, want) {
			t.Fatalf("%s: actions = %v, want %v", prefix, got, want)
		}
	}
}

// VALIDATES: previews can be downloaded from a manually started CI run.
// PREVENTS: preview preparation activating a stable release or registry push.
func TestPreviewWorkflowOnlyPreparesDownloadableArtifacts(t *testing.T) {
	source := workflowSource(t, "preview.yml")
	for _, required := range []string{"workflow_dispatch:", "contents: read", "build preview run output", "build preview check path", "actions/upload-artifact@", "images", "retention-days:"} {
		if !strings.Contains(source, required) {
			t.Errorf("preview workflow omits %q", required)
		}
	}
	for _, forbidden := range []string{"packages: write", "contents: write", "docker push", "gh release", "schedule:", "pull_request:", "secrets."} {
		if strings.Contains(source, forbidden) {
			t.Errorf("preview workflow contains %q", forbidden)
		}
	}
}
