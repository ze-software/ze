package testqemu

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGuestBootstrapContracts executes the generated setup in a confined shell
// fixture. A required mount failure must stop all later downloads and builds.
func TestGuestBootstrapContracts(t *testing.T) {
	for _, failed := range []string{"workspace", cacheShareTag} {
		t.Run(failed, func(t *testing.T) {
			run := fixtureRun(t, ArchAMD64)
			cache := filepath.Join(run.Tree, "cache")
			if err := os.Rename(cache, cache+"-target"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(cache+"-target", cache); err != nil {
				t.Fatal(err)
			}
			setup, err := run.setupCommand(fixtureGoRelease)
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			for _, dir := range []string{"etc/apk", "proc/sys/net/netfilter", "workspace", "usr/local", "root"} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
					t.Fatal(err)
				}
			}
			setup = strings.NewReplacer("/etc/", root+"/etc/", "/proc/", root+"/proc/",
				"/workspace", root+"/workspace", "/usr/local", root+"/usr/local", "\"/root\"", "\""+root+"/root\"",
				guestRunJobParent, filepath.Join(root, "host-job")).Replace(setup)
			prelude := `apk() { :; }
modprobe() { :; }
nft() { :; }
git() { :; }
mount() { case "$*" in *" ` + failed + ` "*) echo REQUIRED_MOUNT_FAILED; return 71;; esac; }
curl() { echo FORBIDDEN_DOWNLOAD; }
tar() { echo FORBIDDEN_UNPACK; }
go() { echo FORBIDDEN_BUILD >&2; }
`
			command := exec.CommandContext(t.Context(), "sh", "-c", prelude+setup)
			output, err := command.CombinedOutput()
			if !strings.Contains(string(output), "REQUIRED_MOUNT_FAILED") {
				t.Fatalf("fixture did not reach the required mount: %v\n%s", err, output)
			}
			if err == nil {
				t.Errorf("failed required mount reported success:\n%s", output)
			}
			for _, forbidden := range []string{"FORBIDDEN_DOWNLOAD", "FORBIDDEN_UNPACK", "FORBIDDEN_BUILD"} {
				if strings.Contains(string(output), forbidden) {
					t.Errorf("failed required mount started %s:\n%s", forbidden, output)
				}
			}
		})
	}
}

// TestNativeARM64LinuxUsesKVM exercises host planning's accelerator choice; a
// Linux KVM host must not be sent to Apple's HVF or software-only fallback.
func TestNativeARM64LinuxUsesKVM(t *testing.T) {
	run := fixtureRun(t, ArchARM64)
	run.ops.Accelerator = func() string { return acceleratorKVM }
	acceleration, err := run.acceleration()
	if err != nil {
		t.Fatal(err)
	}
	if acceleration != "kvm:tcg" {
		t.Fatalf("native Linux arm64 acceleration = %q, want kvm:tcg", acceleration)
	}
}
