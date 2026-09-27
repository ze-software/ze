package rfc

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// VALIDATES: a tagged Go unit is placed where its own build constraints hold: on the
// host when the host run compiles it, in the Linux guest when only the guest does, and
// nowhere, as an error, when neither does.
// METHOD: fixture files carrying each constraint shape the tree uses, asked of a
// darwin host context and the guest context placementContexts builds.
// PREVENTS: the gap journal row 155 records -- an `integration && linux` unit run by host
// `go test`, which never compiles it, so no break could ever be observed on it -- and the
// opposite error, a unit that compiles nowhere being reported as green or as red.
func TestUnitPlacementFollowsBuildConstraints(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"plain_test.go":        "package widget\n",
		"integration_test.go":  "//go:build integration && linux\n\npackage widget\n",
		"linuxonly_test.go":    "//go:build linux\n\npackage widget\n",
		"widget_linux_test.go": "package widget\n",
		"feature_test.go":      "//go:build ze_widget\n\npackage widget\n",
		"windows_test.go":      "//go:build windows\n\npackage widget\n",
		"notlinux_test.go":     "//go:build !linux\n\npackage widget\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	host, guest := placementContexts([]string{"ze_widget"})
	// A darwin host, whatever this test runs on, so the linux cases are asked of
	// the guest on every machine.
	host.GOOS = "darwin"

	cases := []struct {
		file  string
		guest bool
		fails bool
	}{
		{file: "plain_test.go"},
		{file: "feature_test.go"},
		{file: "notlinux_test.go"},
		{file: "integration_test.go", guest: true},
		{file: "linuxonly_test.go", guest: true},
		{file: "widget_linux_test.go", guest: true},
		{file: "windows_test.go", fails: true},
	}
	for _, c := range cases {
		inGuest, err := unitNeedsGuest(&host, &guest, filepath.Join(directory, c.file))
		if c.fails {
			if err == nil {
				t.Errorf("%s: placed (guest=%v), want the refusal for a file nothing compiles", c.file, inGuest)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.file, err)
			continue
		}
		if inGuest != c.guest {
			t.Errorf("%s: guest=%v, want %v", c.file, inGuest, c.guest)
		}
	}
	if !slices.Contains(guest.BuildTags, testqemu.IntegrationTag) {
		t.Errorf("guest tags %v lack %q", guest.BuildTags, testqemu.IntegrationTag)
	}
	if guest.GOOS != "linux" {
		t.Errorf("guest GOOS %q, want linux", guest.GOOS)
	}
}

// VALIDATES: a guest observation demands Ze's runtime kernel, and a host observation
// refuses one.
// METHOD: requireGuestKernel over the four combinations.
// PREVENTS: a guest proof recorded over the Alpine ISO's own kernel, which
// ai/rules/platform-linux.md forbids, and a kernel argument silently read by nothing.
func TestRequireGuestKernel(t *testing.T) {
	const unit = "internal/widget/widget_linux_test.go::TestWidget"
	if err := requireGuestKernel(unit, true, ""); err == nil || !strings.Contains(err.Error(), keyKernel) {
		t.Errorf("guest with no kernel: %v, want a refusal naming %q", err, keyKernel)
	}
	if err := requireGuestKernel(unit, true, "tmp/kernel/build/vmlinuz"); err != nil {
		t.Errorf("guest with a kernel: %v", err)
	}
	if err := requireGuestKernel(unit, false, "tmp/kernel/build/vmlinuz"); err == nil {
		t.Error("host unit with a kernel: accepted, want a refusal")
	}
	if err := requireGuestKernel(unit, false, ""); err != nil {
		t.Errorf("host unit with no kernel: %v", err)
	}
}

// VALIDATES: a guest observation compiles the unit's package for the guest on the host,
// under the integration tag and the overlay, and runs the ONE unit inside the guest on
// the named kernel, verbose and with the coverage profile written through the checkout.
// METHOD: the three commands a guest run is made of, over a fixture runner.
// PREVENTS: a guest run that selects every test in the package (no attribution), that
// drops the overlay (a green that never saw the break), or whose profile the host cannot
// read (a reachability judgement over nothing).
func TestGuestObservationCommands(t *testing.T) {
	tree := t.TempDir()
	runner := &observationRunner{
		tree: tree, toolchain: gotoolchain.Toolchain{Timeout: "20m"},
		carrier: Carrier{Kind: kindUnit}, tag: Tag{File: "internal/widget/widget_linux_test.go"},
		record:   DiscriminationRecord{Producer: "internal/widget/widget.go::SendWidget"},
		unitName: "TestWidget", self: "/bin/le", kernel: "tmp/kernel/build/vmlinuz", guest: true,
	}
	scratch := filepath.Join(tree, "tmp", "s")
	binary := filepath.Join(scratch, "guest-1.test")
	profile := filepath.Join(scratch, "cover-1.out")

	build := runner.guestBuildArgv(binary, filepath.Join(scratch, "overlay.json"), profile)
	joined := strings.Join(build, " ")
	for _, want := range []string{" test ", " -c -o " + binary, " -overlay " + filepath.Join(scratch, "overlay.json"),
		" -cover -coverpkg ./internal/widget"} {
		if !strings.Contains(joined, want) {
			t.Errorf("build argv %q lacks %q", joined, want)
		}
	}
	if tags := build[slices.Index(build, "-tags")+1]; !slices.Contains(strings.Fields(tags), testqemu.IntegrationTag) {
		t.Errorf("build tags %q lack %q", tags, testqemu.IntegrationTag)
	}
	if build[len(build)-1] != "./internal/widget" {
		t.Errorf("build argv ends %q, want the unit's package", build[len(build)-1])
	}

	script := runner.guestScript(binary, profile)
	want := "cd '/workspace/internal/widget' && exec '/workspace/tmp/s/guest-1.test' -test.run '^TestWidget$'" +
		" -test.count=1 -test.v -test.timeout 20m -test.coverprofile '/workspace/tmp/s/cover-1.out'\n"
	if script != want {
		t.Errorf("guest script\n got %q\nwant %q", script, want)
	}

	run := runner.guestRunArgv(filepath.Join(scratch, "guest-1.sh"))
	wantRun := []string{"/bin/le", "test", "qemu", "run", "kernel", "tmp/kernel/build/vmlinuz",
		"command", "sh '/workspace/tmp/s/guest-1.sh'"}
	if !slices.Equal(run, wantRun) {
		t.Errorf("guest run argv\n got %q\nwant %q", run, wantRun)
	}
}
