package testqemu

// The appliance environment of the hugepage, crash-capture and MPLS boot runs.
//
// Goal: `ze appliance build` runs Docker and inherits this environment, so it
// must carry the daemon dockerhost selects, never whatever the Docker client's
// context points at. Method: set this process's DOCKER_HOST and HOME, call
// applianceEnv, and read the result. The selection's own conditions are tested
// in internal/le/dockerhost; this proves the run asks it.

import (
	"runtime"
	"strings"
	"testing"
)

// VALIDATES: an operator's DOCKER_HOST reaches `ze appliance build` unchanged,
// beside the appliance directory and the SSH password.
// PREVENTS: the run replacing the daemon the operator named.
func TestApplianceEnvKeepsTheOperatorsDaemon(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://docker")
	run := fixtureHugepages(t)

	got, err := run.applianceEnv("/work/appliances")
	if err != nil {
		t.Fatalf("refused an explicit DOCKER_HOST: %v", err)
	}
	for _, want := range []string{"DOCKER_HOST=tcp://docker", "ZE_APPLIANCE_DIR=/work/appliances", "ze.appliance.ssh.password=" + DefaultSSHPass} {
		if !envHas(got, want) {
			t.Fatalf("env=%q, want %q", got, want)
		}
	}
}

// VALIDATES: on macOS, with no DOCKER_HOST and no `ze` Colima profile socket,
// the run is refused naming `./le setup docker-host start`; elsewhere it runs.
// PREVENTS: `ze appliance build` running Docker on the client's current
// context, which owner decision M10 rules out (it ran on os.Environ() before).
func TestApplianceEnvRefusesWithoutTheZeProfile(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("HOME", t.TempDir())
	run := fixtureHugepages(t)

	got, err := run.applianceEnv("/work/appliances")
	if runtime.GOOS == "darwin" {
		if err == nil || !strings.Contains(err.Error(), "./le setup docker-host start") {
			t.Fatalf("env=%q err=%v, want the refusal naming the docker-host start", got, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("refused on %s, which has no Colima profile to select: %v", runtime.GOOS, err)
	}
}
