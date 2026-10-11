package appliance

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestISOToolFailureNamesTheTool proves an ISO tool that runs out of
// isoBuildTimeout, or fails, is named with its command line, never reported
// as exec's "signal: killed" or "exit status N" alone. A shell script plays
// GRUB's EFI builder: one sleeps past a 300 ms limit, one exits 3.
//
// VALIDATES: round-9 ISSUE 1 (the sweep of the cmd_iso build path).
// PREVENTS: "GRUB EFI builder timed out" with no limit and no next step, and
// a bare "build ISO: exit status 3".
func TestISOToolFailureNamesTheTool(t *testing.T) {
	cases := []struct {
		name   string
		script string
		say    string
		want   error
	}{
		{"out of time", "#!/bin/sh\nexec sleep 60\n", "the GRUB EFI builder did not finish within the 300ms limit Ze gives the ISO tools", context.DeadlineExceeded},
		{"fails", "#!/bin/sh\nexit 3\n", "the GRUB EFI builder failed (exit status 3)", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old := isoBuildTimeout
			isoBuildTimeout = 300 * time.Millisecond
			t.Cleanup(func() { isoBuildTimeout = old })
			// A directory with a space: the command line quotes the tool
			// path (shellArg), or it does not run as written.
			grubDir := filepath.Join(t.TempDir(), "iso tools")
			if err := os.MkdirAll(grubDir, 0o750); err != nil {
				t.Fatal(err)
			}
			grub := filepath.Join(grubDir, "grub-mkstandalone")
			if err := os.WriteFile(grub, []byte(tc.script), 0o755); err != nil { //nolint:gosec // test script must be executable
				t.Fatal(err)
			}
			err := runISOBuilder(isoBuilderCall{
				GRUBPath: grub, XorrisoPath: "/nonexistent/xorriso", OutputPath: filepath.Join(t.TempDir(), "out.iso"),
				StagingDir: t.TempDir(), GRUBTarget: "x86_64-efi", EFIBootFile: "BOOTX64.EFI",
			})
			if err == nil {
				t.Fatal("a failed ISO tool was reported as a build")
			}
			if !strings.Contains(err.Error(), tc.say) {
				t.Errorf("the ISO tool failure does not say %q:\n%v", tc.say, err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("the ISO tool failure does not carry %v:\n%v", tc.want, err)
			}
			if strings.Contains(err.Error(), "signal: killed") {
				t.Errorf("an ISO tool out of time is reported as killed:\n%v", err)
			}
			lines := strings.Split(err.Error(), "\n")
			if last := lines[len(lines)-1]; !strings.HasPrefix(last, shellArg(grub)+" -O x86_64-efi ") || !strings.HasPrefix(last, "'") {
				t.Errorf("the ISO tool failure does not end with its command line:\n%v", err)
			}
		})
	}
}

// TestFirmwareDownloadNamesTheFetch proves a firmware download that fails, or
// runs out of downloadTimeout, names the URL and ends with the curl command
// that fetches the blob to where ensureFirmware looks for it, never net/http's
// error alone. A closed httptest server plays an unreachable mirror; an
// expired context plays the limit.
//
// VALIDATES: round-9 ISSUE 1 (the sweep of the cmd_kernel build path).
// PREVENTS: "download firmware X: context deadline exceeded" with no next step.
func TestFirmwareDownloadNamesTheFetch(t *testing.T) {
	srv := httptest.NewServer(nil)
	url := srv.URL + "/i915/blob.bin"
	srv.Close()
	dest := filepath.Join(t.TempDir(), "i915", "blob.bin")
	fetch := "curl -fL -o " + dest + ".part " + url + " && mv " + dest + ".part " + dest

	err := downloadFirmwareBlob(url, dest)
	if err == nil {
		t.Fatal("a download from a closed server was reported as a download")
	}
	if !strings.Contains(err.Error(), "the download from "+url+" failed (") {
		t.Errorf("the firmware failure does not name the URL:\n%v", err)
	}
	lines := strings.Split(err.Error(), "\n")
	if last := lines[len(lines)-1]; last != fetch {
		t.Errorf("the firmware failure does not end with %q:\n%v", fetch, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	err = firmwareDownloadFailed(ctx, url, dest, ctx.Err())
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "did not finish within the 10-minute limit Ze gives one firmware download") {
		t.Errorf("a firmware download out of time does not name the limit:\n%v", err)
	}
	lines = strings.Split(err.Error(), "\n")
	if last := lines[len(lines)-1]; last != fetch {
		t.Errorf("a firmware download out of time does not end with %q:\n%v", fetch, err)
	}
}
