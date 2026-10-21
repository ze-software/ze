package interoplab

import (
	"slices"
	"testing"
)

// TestBuildToolDockerKeepsTheProcessEnvironment builds the build-tool client
// with a marker variable set and DOCKER_HOST unset, and checks it carries
// this process's environment and no endpoint refusal.
//
// VALIDATES: NewBuildToolDocker hands the docker CLI the process's own
// environment, so a build container runs on the daemon DOCKER_HOST or the
// current context names, even on a Mac with no `ze` profile yet.
// PREVENTS: the docker-host image build refused for want of the `ze`
// profile that the build itself produces.
func TestBuildToolDockerKeepsTheProcessEnvironment(t *testing.T) {
	t.Setenv("ZE_BUILD_TOOL_MARKER", "kept")
	t.Setenv("DOCKER_HOST", "")
	docker := NewBuildToolDocker()
	if docker.endpointRefusal != nil {
		t.Fatalf("build-tool client refuses: %v", docker.endpointRefusal)
	}
	if !slices.Contains(docker.environ, "ZE_BUILD_TOOL_MARKER=kept") {
		t.Errorf("build-tool client environment lacks the process's ZE_BUILD_TOOL_MARKER")
	}
	if docker.oneShotName == nil {
		t.Errorf("build-tool client has no one-shot namer, so RunOneShot cannot name its container")
	}
}
