package testqemu

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeFakeHost writes a stand-in for the host ze binary. `appliance init`
// exits initCode, writing the appliance.json a real init writes when initCode
// is zero, and `appliance build` exits buildCode. Neither prints anything,
// which is the shape of the failure the harness hid: a build that ends with a
// status and no output.
func writeFakeHost(t *testing.T, initCode, buildCode int) string {
	t.Helper()
	initStatus := strconv.Itoa(initCode)
	script := "#!/bin/sh\n" +
		"case \"$2\" in\n" +
		"init)\n" +
		"  [ " + initStatus + " -ne 0 ] && exit " + initStatus + "\n" +
		"  mkdir -p \"$ZE_APPLIANCE_DIR/$3\"\n" +
		"  echo '{\"image\":{}}' > \"$ZE_APPLIANCE_DIR/$3/appliance.json\"\n" +
		"  exit 0;;\n" +
		"build) exit " + strconv.Itoa(buildCode) + ";;\n" +
		"esac\n" +
		"exit 99\n"
	host := filepath.Join(t.TempDir(), "ze")
	if err := os.WriteFile(host, []byte(script), 0o700); err != nil { //nolint:gosec // an executable the test runs
		t.Fatal(err)
	}
	return host
}

// TestApplianceStepFailureNamesExitStatus proves that each proof that builds an
// appliance image reports how `ze appliance init` or `ze appliance build`
// ended, not only what it printed.
//
// VALIDATES: a failed step names its exit status.
// PREVENTS: "ze appliance build failed:" followed by nothing, which is all
// mpls-boot-test printed when the build exited 1 with an empty output on
// 2026-10-10: the harness dropped the error that carried the status.
//
// Method: a fake host binary fails one step with a distinct status and prints
// nothing, and the test asserts the status is in the error the proof returns.
func TestApplianceStepFailureNamesExitStatus(t *testing.T) {
	cases := []struct {
		name  string
		build func(host, work string) error
		init  int
		image int
		want  string
	}{
		{"hugepages init", func(host, work string) error {
			_, err := newHugepages(t.TempDir()).buildImage(host, work)
			return err
		}, 3, 0, "ze appliance init failed (exit status 3)"},
		{"hugepages build", func(host, work string) error {
			_, err := newHugepages(t.TempDir()).buildImage(host, work)
			return err
		}, 0, 4, "ze appliance build failed (exit status 4)"},
		{"mpls boot init", func(host, work string) error {
			_, err := newMPLSBoot(t.TempDir()).buildImage(host, work, "")
			return err
		}, 5, 0, "ze appliance init failed (exit status 5)"},
		{"mpls boot build", func(host, work string) error {
			_, err := newMPLSBoot(t.TempDir()).buildImage(host, work, "")
			return err
		}, 0, 6, "ze appliance build failed (exit status 6)"},
		{"crash capture init", func(host, work string) error {
			_, err := newCrashCapture(t.TempDir(), CrashLab(0)).buildImage(host, work)
			return err
		}, 7, 0, "ze appliance init failed (exit status 7)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host := writeFakeHost(t, tc.init, tc.image)
			err := tc.build(host, t.TempDir())
			if err == nil {
				t.Fatal("buildImage succeeded over a failing step")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

// TestPrerequisitesFindKegOnlyE2fsprogs proves the proofs look for e2fsprogs
// where `ze appliance build` does, not only on PATH.
//
// VALIDATES: a machine whose e2fsprogs is installed off PATH runs the proof.
// PREVENTS: mpls-boot-test answering "SKIP e2fsprogs not found" and exit 0 on a
// Mac where Homebrew's keg-only e2fsprogs serves the build perfectly well; the
// proof then never ran and the run read as success.
//
// Method: PATH holds every other prerequisite and no e2fsprogs tool, and the
// resolver the build uses answers a path for both. The check must report
// nothing missing.
func TestPrerequisitesFindKegOnlyE2fsprogs(t *testing.T) {
	bin := t.TempDir()
	h := newHugepages(t.TempDir())
	for _, tool := range []string{"go", qemuBinary(h.Arch), "sshpass"} {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // an executable the lookup finds
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	keg := t.TempDir()
	saved := e2fsToolFn
	t.Cleanup(func() { e2fsToolFn = saved })
	e2fsToolFn = func(name string) string { return filepath.Join(keg, name) }

	if reason := h.missingPrerequisite(); reason != "" {
		t.Fatalf("missingPrerequisite = %q, want nothing missing when the build's resolver finds e2fsprogs", reason)
	}

	e2fsToolFn = func(string) string { return "" }
	if reason := h.missingPrerequisite(); !strings.Contains(reason, "e2fsprogs") {
		t.Fatalf("missingPrerequisite = %q, want it to name e2fsprogs when the resolver finds nothing", reason)
	}
}
