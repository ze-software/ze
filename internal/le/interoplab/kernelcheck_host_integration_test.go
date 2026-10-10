//go:build integration

// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
package interoplab

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// TestDockerKernelCheckOnThisHost points the check at the Docker daemon this
// machine uses, with a ze staged for that daemon exactly as a lab stages it.
// It starts the probe container and nothing else: no image build, no lab.
//
// The verdict is judged against the probe's own rows, read a second time
// through the same container, so the test holds on any host: a kernel with
// every feature proceeds, and any other is refused naming every row that is not
// present, by subsystem and CONFIG_ symbol, and nothing that is. On colima's
// 6.8.0-117-generic (2026-10-10) that is ipsec-mobike, l2tp, l2tp-ppp and mpls
// absent and mpls-transit-mtu unknown. The refusal is logged so a run records
// which kernel it judged.
func TestDockerKernelCheckOnThisHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()
	docker := NewDocker()
	if err := docker.Probe(ctx); err != nil {
		t.Skipf("no Docker daemon answers here, so there is no kernel to check: %v", err)
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	// The staged ze sits in the checkout, not in t.TempDir: a Docker VM that
	// mounts only the home directory cannot bind-mount the system temp.
	output := filepath.Join("tmp", "interoplab-kernel-host", filepath.Base(t.TempDir()), "ze-linux")
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(filepath.Join(root, filepath.Dir(output))); removeErr != nil {
			t.Error(removeErr)
		}
	})
	stage := StageBinaries(root, false, LabBinary{Name: "ze", Base: repofeaturetags.DaemonBase, Output: output})
	if stageErr := stage(ctx, docker); stageErr != nil {
		t.Fatalf("stage ze for the daemon: %v", stageErr)
	}
	staged := StagedZePath(root, []LabBinary{{Name: "ze", Output: output}})

	verdict := DockerKernel(staged)(ctx, docker)

	appArmor, securityErr := dockerAppArmor(ctx, docker)
	if securityErr != nil {
		t.Fatalf("read the daemon's security options: %v", securityErr)
	}
	result, runErr := docker.runner.Run(ctx, processCommand{Arguments: kernelProbeArgv(staged, appArmor), Timeout: kernelProbeTimeout})
	if runErr != nil {
		t.Fatalf("read the rows a second time: %v", runErr)
	}
	var answer kernelAnswer
	if jsonErr := json.Unmarshal([]byte(result.Stdout), &answer); jsonErr != nil {
		t.Fatalf("the probe answered no JSON: %v; stdout %q; stderr %q", jsonErr, result.Stdout, result.Stderr)
	}
	if len(answer.Capabilities) == 0 {
		t.Fatal("the probe answered no capability")
	}

	var missing []kernelRow
	for _, row := range answer.Capabilities {
		if row.State != "present" {
			missing = append(missing, row)
		}
	}
	if len(missing) == 0 {
		if verdict != nil {
			t.Fatalf("every row is present and the check refused: %v", verdict)
		}
		t.Logf("this Docker host passes: %d capabilities present", len(answer.Capabilities))
		return
	}
	if verdict == nil {
		t.Fatalf("%d rows are not present and the check proceeded: %+v", len(missing), missing)
	}
	t.Logf("this Docker host is refused:\n%v", verdict)
	refusal := verdict.Error()
	for _, row := range missing {
		if !strings.Contains(refusal, row.Subsystem+" ("+row.Kernel+"): "+row.State) {
			t.Errorf("the refusal does not name %s (%s): %s", row.Subsystem, row.Kernel, row.State)
		}
	}
	for _, row := range answer.Capabilities {
		if row.State == "present" && strings.Contains(refusal, "\n  "+row.Subsystem+" (") {
			t.Errorf("the refusal names present %s", row.Subsystem)
		}
	}
}
