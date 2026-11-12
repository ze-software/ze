// Design: docs/architecture/appliance/builder.md -- embedded builder failures.

package appliance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const gokPackingFailureChild = "ZE_TEST_GOK_PACKING_FAILURE_CHILD"

// This uses the actual embedded gok packer, not gokBuildFn's error seam. A
// process exit inside the dependency must fail the parent test rather than
// terminate the package suite or let testing's own cleanup hide a leaked key.
func TestGokPackingFailureReturnsAndCleansIdentity(t *testing.T) {
	if os.Getenv(gokPackingFailureChild) == "1" {
		runGokPackingFailureChild(t)
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGokPackingFailureReturnsAndCleansIdentity$", "-test.v") //nolint:gosec // Rerun this test's own binary.
	command.Env = append(os.Environ(), gokPackingFailureChild+"=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("embedded builder did not return through the cleanup path: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), `unknown device slug "ze-test-invalid-device"`) {
		t.Fatalf("fixture never reached the packer's actual failure: %s", output)
	}
	if !strings.Contains(string(output), "PACKING-FAILURE-RETURNED-AND-CLEANED") {
		t.Fatalf("fixture did not assert immediate prepared-instance cleanup: %s", output)
	}
}

func runGokPackingFailureChild(t *testing.T) {
	t.Helper()
	root := writeGokrazyFixture(t)
	for _, dir := range []string{filepath.Join(root, "gokrazy", "modcache")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, contents := range map[string]string{
		filepath.Join(root, "go.mod"):                       "module github.com/ze-software/ze\n\ngo 1.26\n",
		filepath.Join(root, "gokrazy", "ze", "config.json"): `{"Hostname":"ze","DeviceType":"ze-test-invalid-device"}`,
		filepath.Join(root, "gokrazy", "ze", "ze.conf"):     "set system host fixture\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	underPatchedTree(t, patchedTreeFixture(t, true))
	initTestAppliance(t, "failure-device", nil)
	oldMkfs, oldDebugfs := e2fsMkfs, e2fsDebugfs
	e2fsMkfs, e2fsDebugfs = "fixture-mkfs", "fixture-debugfs"
	t.Cleanup(func() { e2fsMkfs, e2fsDebugfs = oldMkfs, oldDebugfs })
	if code := runBuild([]string{"failure-device"}); code != exitError {
		t.Fatalf("appliance build code=%d, want packing failure", code)
	}
	prepared, err := filepath.Glob(filepath.Join(root, "tmp", "appliance-build-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 0 {
		t.Fatalf("failed packer left prepared credentials in %v", prepared)
	}
	// Check before testing's own TempDir cleanup runs: that cleanup is not the
	// product's guarantee and would otherwise hide a missing build defer.
	t.Log("PACKING-FAILURE-RETURNED-AND-CLEANED")
}
